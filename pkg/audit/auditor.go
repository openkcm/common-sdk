package audit

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"math/rand/v2"
	"sync"
	"time"

	"github.com/goccy/go-yaml"
	"go.opentelemetry.io/collector/pdata/plog"
	"google.golang.org/protobuf/types/known/timestamppb"

	auditlogeventsv1 "github.com/openkcm/api-sdk/proto/kms/api/cmk/auditlog/events/v1"

	"github.com/openkcm/common-sdk/pkg/commoncfg"
)

const (
	// defaultBatchSize is the default number of events to batch before sending (gRPC only).
	defaultBatchSize = 100

	// defaultBatchTimeout is the default maximum time to wait before sending a partial batch (gRPC only).
	defaultBatchTimeout = 200 * time.Millisecond
)

// Auditor wraps async queuing and OTLP delivery for audit events.
// It supports both HTTP and gRPC transports based on configuration.
// It is safe for concurrent use by multiple goroutines.
type Auditor struct {
	sender          Sender
	protocol        commoncfg.Protocol
	additionalProps map[string]string
	queue           chan auditlogeventsv1.AuditEvent
	queueSize       int
	maxConcurrent   int
	semaphore       chan struct{}
	maxRetries      int
	baseDelay       time.Duration
	maxDelay        time.Duration
	onError         func(error)

	// gRPC-specific options
	grpcSenderOpts []GRPCSenderOption
	batchSize      int
	batchTimeout   time.Duration

	stopCancel context.CancelFunc
	wg         sync.WaitGroup
	mu         sync.Mutex
	closed     bool
}

// NewAuditor creates an Auditor from the given config and options.
// It starts background workers immediately.
// The transport protocol is determined by cfg.Protocol:
//   - "grpc" uses gRPC transport (requires cfg.GRPCClient) with batching
//   - "http" or empty uses HTTP transport (uses cfg.HTTPClient) with per-event dispatch
func NewAuditor(cfg *commoncfg.Audit, opts ...Option) (*Auditor, error) {
	// Parse additional properties from YAML string
	var additionalProps map[string]string
	if cfg.AdditionalProperties != "" {
		err := yaml.Unmarshal([]byte(cfg.AdditionalProperties), &additionalProps)
		if err != nil {
			return nil, fmt.Errorf("audit: failed to parse additional properties: %w", err)
		}
	}

	// Create sender based on protocol
	var (
		sender Sender
		err    error
	)

	// Create auditor with defaults first to collect options
	a := &Auditor{
		protocol:        cfg.Protocol,
		additionalProps: additionalProps,
		queueSize:       defaultQueueSize,
		maxConcurrent:   defaultMaxConcurrent,
		maxRetries:      defaultMaxRetries,
		baseDelay:       defaultBaseDelay,
		maxDelay:        defaultMaxDelay,
		batchSize:       defaultBatchSize,
		batchTimeout:    defaultBatchTimeout,
	}

	// Apply options (including gRPC sender options)
	for _, opt := range opts {
		opt(a)
	}

	switch cfg.Protocol {
	case commoncfg.GRPCProtocol:
		sender, err = newGRPCSender(cfg, a.grpcSenderOpts...)
	default:
		// HTTP is the default for backward compatibility
		sender, err = newHTTPSender(cfg)
	}

	if err != nil {
		return nil, err
	}

	// Assign the sender
	a.sender = sender

	// Initialize queue and semaphore
	stopCtx, stopCancel := context.WithCancel(context.Background())
	a.stopCancel = stopCancel
	a.queue = make(chan auditlogeventsv1.AuditEvent, a.queueSize)
	a.semaphore = make(chan struct{}, a.maxConcurrent)

	// Start dispatcher based on protocol
	a.wg.Add(1)

	if cfg.Protocol == commoncfg.GRPCProtocol {
		go a.batchDispatcher(stopCtx)
	} else {
		go a.dispatcher(stopCtx)
	}

	return a, nil
}

// Send enqueues an event for async delivery (non-blocking).
// Returns an error only if the queue is full or the auditor is closed.
func (a *Auditor) Send(event auditlogeventsv1.AuditEvent) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.closed {
		return ErrClosed
	}

	// Non-blocking send
	select {
	case a.queue <- event:
		return nil
	default:
		return ErrQueueFull
	}
}

// NewCommon creates a Common struct with timestamp and additional properties pre-filled.
// Accepts optional CommonOption functions for per-event properties.
func (a *Auditor) NewCommon(opts ...CommonOption) *auditlogeventsv1.Common {
	// Copy additional properties from config
	properties := make(map[string]string, len(a.additionalProps))
	maps.Copy(properties, a.additionalProps)

	common := &auditlogeventsv1.Common{}
	common.SetTimestamp(timestamppb.Now())
	common.SetProperties(properties)

	// Apply per-event options (can override additionalProps)
	for _, opt := range opts {
		opt(common)
	}

	return common
}

// Close stops accepting new events, drains events already queued (with retries),
// and waits for background workers to finish. It is idempotent.
//
// The ctx should have a deadline or timeout. If ctx is cancelled before the queue
// is drained, in-flight sends and pending backoffs are aborted and ctx's error is
// returned; otherwise it returns nil. If ctx has no deadline, Close will block
// until all events are delivered or fail.
func (a *Auditor) Close(ctx context.Context) error {
	a.mu.Lock()
	if a.closed {
		a.mu.Unlock()
		return nil
	}

	a.closed = true
	close(a.queue)
	a.mu.Unlock()

	done := make(chan struct{})

	go func() {
		select {
		case <-ctx.Done():
			a.stopCancel() // deadline hit: stop retrying/draining
		case <-done:
		}
	}()

	a.wg.Wait()
	close(done)
	a.stopCancel() // release the workers' context once they have exited

	// Close the sender to release any resources (e.g., gRPC connections)
	closeErr := a.sender.Close()
	if closeErr != nil && ctx.Err() == nil {
		return closeErr
	}

	return ctx.Err()
}

// dispatcher reads from the queue and spawns a goroutine per event.
// Each goroutine acquires the semaphore before delivering (limiting concurrent requests).
// Used for HTTP transport.
func (a *Auditor) dispatcher(ctx context.Context) {
	defer a.wg.Done()

	for event := range a.queue {
		a.wg.Go(func() {
			// Acquire semaphore (limit concurrent requests)
			select {
			case a.semaphore <- struct{}{}:
				defer func() { <-a.semaphore }()
			case <-ctx.Done():
				return // shutdown
			}

			// Deliver with retries
			a.deliver(ctx, event)
		})
	}
}

// batchDispatcher collects events into batches and sends them together.
// Batches are sent when full OR after a timeout. This is more efficient for gRPC
// and ensures proper retry semantics (retry the whole batch, not individual events).
// Used for gRPC transport.
func (a *Auditor) batchDispatcher(ctx context.Context) {
	defer a.wg.Done()

	batch := make([]auditlogeventsv1.AuditEvent, 0, a.batchSize)
	timer := time.NewTimer(a.batchTimeout)
	timer.Stop() // Don't start until we have events

	// Ensure timer is properly cleaned up on exit
	defer func() {
		if !timer.Stop() {
			select {
			case <-timer.C:
			default:
			}
		}
	}()

	flushBatch := func() {
		if len(batch) == 0 {
			return
		}

		// Copy batch for delivery
		toSend := make([]auditlogeventsv1.AuditEvent, len(batch))
		copy(toSend, batch)
		batch = batch[:0]

		// Stop timer while we're sending
		timer.Stop()

		// Acquire semaphore and deliver
		select {
		case a.semaphore <- struct{}{}:
			a.deliverBatch(ctx, toSend)
			<-a.semaphore
		case <-ctx.Done():
			return
		}
	}

	for {
		select {
		case event, ok := <-a.queue:
			if !ok {
				// Queue closed, flush remaining
				flushBatch()
				return
			}

			batch = append(batch, event)

			if len(batch) >= a.batchSize {
				flushBatch()
			} else if len(batch) == 1 {
				// First event in batch, start timer
				timer.Reset(a.batchTimeout)
			}

		case <-timer.C:
			flushBatch()

		case <-ctx.Done():
			// Forced shutdown, try to flush what we have
			flushBatch()
			return
		}
	}
}

// deliverBatch sends a batch of events, retrying transient failures with exponential backoff.
// On a forced shutdown it abandons the batch; after exhausting all retries it reports
// the error to OnError. Permanent errors are not retried.
func (a *Auditor) deliverBatch(ctx context.Context, events []auditlogeventsv1.AuditEvent) {
	var err error

	for attempt := 0; attempt <= a.maxRetries; attempt++ {
		if attempt > 0 && !a.wait(ctx, a.backoff(attempt)) {
			return // forced shutdown during backoff; abandon this batch
		}

		err = a.sendBatch(ctx, events)
		if err == nil {
			return
		}

		// Don't retry permanent errors
		if errors.Is(err, ErrPermanent) {
			a.reportError(fmt.Errorf("batch of %d events failed: %w", len(events), err))
			return
		}
	}

	a.reportError(fmt.Errorf("batch of %d events failed after retries: %w", len(events), err))
}

// sendBatch converts multiple audit events to OTLP log records and exports them via the sender.
func (a *Auditor) sendBatch(ctx context.Context, events []auditlogeventsv1.AuditEvent) error {
	// Build plog.Logs with all events
	logs := plog.NewLogs()
	rl := logs.ResourceLogs().AppendEmpty()
	sl := rl.ScopeLogs().AppendEmpty()

	for _, event := range events {
		lr := event.ToLogRecord()
		lr.MoveTo(sl.LogRecords().AppendEmpty())
	}

	return a.sender.Send(ctx, logs)
}

// deliver sends a single event, retrying transient failures with exponential backoff.
// On a forced shutdown it abandons the event; after exhausting all retries it reports
// the last error to OnError. Permanent errors are not retried.
// Used for HTTP transport.
func (a *Auditor) deliver(ctx context.Context, event auditlogeventsv1.AuditEvent) {
	var err error

	for attempt := 0; attempt <= a.maxRetries; attempt++ {
		if attempt > 0 && !a.wait(ctx, a.backoff(attempt)) {
			return // forced shutdown during backoff; abandon this event
		}

		err = a.send(ctx, event)
		if err == nil {
			return
		}

		// Don't retry permanent errors
		if errors.Is(err, ErrPermanent) {
			a.reportError(err)
			return
		}
	}

	a.reportError(err)
}

// send converts the audit event to an OTLP log record and exports it via the configured sender.
// Used for HTTP transport.
func (a *Auditor) send(ctx context.Context, event auditlogeventsv1.AuditEvent) error {
	// Convert event to plog.LogRecord using the generated converter
	lr := event.ToLogRecord()

	// Wrap in plog.Logs structure
	logs := plog.NewLogs()
	rl := logs.ResourceLogs().AppendEmpty()
	sl := rl.ScopeLogs().AppendEmpty()
	lr.MoveTo(sl.LogRecords().AppendEmpty())

	return a.sender.Send(ctx, logs)
}

// wait sleeps for d, returning false if a forced shutdown happened first.
func (a *Auditor) wait(ctx context.Context, d time.Duration) bool {
	t := time.NewTimer(d)
	defer t.Stop()

	select {
	case <-ctx.Done():
		return false
	case <-t.C:
		return true
	}
}

// backoff returns the delay before the given retry attempt (attempt >= 1),
// doubling baseDelay up to maxDelay with equal jitter.
func (a *Auditor) backoff(attempt int) time.Duration {
	d := a.maxDelay
	// Guard the shift against overflow for large attempt counts.
	if shift := attempt - 1; shift < 62 {
		if scaled := a.baseDelay << shift; scaled > 0 && scaled < d {
			d = scaled
		}
	}

	// Equal jitter: half fixed, half random in [0, d/2).
	half := d / 2

	return half + time.Duration(rand.Int64N(int64(half)+1))
}

func (a *Auditor) reportError(err error) {
	if err == nil || a.onError == nil {
		return
	}

	a.onError(fmt.Errorf("audit: failed to deliver audit event after retries: %w", err))
}
