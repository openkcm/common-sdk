package otlpaudit

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math/rand/v2"
	"net"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/samber/oops"
	"go.opentelemetry.io/collector/pdata/plog"
)

const (
	defaultQueueSize  = 1024
	defaultWorkers    = 1
	defaultMaxRetries = 5
	defaultBaseDelay  = 100 * time.Millisecond
	defaultMaxDelay   = 30 * time.Second
)

// QueuedOptions configures a QueuedAuditLogger. The zero value is valid and uses
// the defaults documented on each field.
type QueuedOptions struct {
	// QueueSize is the capacity of the buffered event queue. SendEvent drops
	// events (returning ErrQueueFull) once the queue is full. Defaults to 1024.
	QueueSize int

	// Workers is the number of background goroutines delivering events to the
	// underlying logger. Defaults to 1, preserving enqueue order. Multiple workers
	// may deliver events out of order and call OnError concurrently.
	Workers int

	// MaxRetries is the number of additional delivery attempts after the first
	// one fails with a retryable error. Each retry is spaced by an exponential
	// backoff. Once exhausted the event is dropped. Values <= 0 default to 5.
	MaxRetries int

	// BaseDelay is the first backoff step. Subsequent steps double it up to
	// MaxDelay. Defaults to 100ms.
	BaseDelay time.Duration

	// MaxDelay caps the backoff step. Defaults to 30s.
	MaxDelay time.Duration

	// OnError receives each failed delivery attempt and each event discarded on
	// shutdown cancellation. It defaults to slog.Error. Queue-full and closed
	// errors are returned by SendEvent instead. The handler must return promptly,
	// must not call Close on this logger, and must be safe for concurrent calls
	// when Workers is greater than 1.
	OnError func(error)
}

func (o *QueuedOptions) setDefaults() {
	if o.QueueSize <= 0 {
		o.QueueSize = defaultQueueSize
	}

	if o.Workers <= 0 {
		o.Workers = defaultWorkers
	}

	if o.MaxRetries <= 0 {
		o.MaxRetries = defaultMaxRetries
	}

	if o.BaseDelay <= 0 {
		o.BaseDelay = defaultBaseDelay
	}

	if o.MaxDelay <= 0 {
		o.MaxDelay = defaultMaxDelay
	}

	if o.OnError == nil {
		o.OnError = func(err error) {
			slog.Error("Failed to deliver audit event", "error", err)
		}
	}
}

// QueuedAuditLogger wraps an AuditLogger with a non-blocking, channel-based
// queue and background delivery. It is safe for concurrent use by multiple
// goroutines. Create one with NewQueuedLogger and always Close it to drain
// buffered events. It must not be copied. The queue is held in memory and does
// not survive a process exit.
type QueuedAuditLogger struct {
	sink  *AuditLogger
	opts  QueuedOptions
	queue chan plog.Logs

	cancel context.CancelCauseFunc
	done   chan struct{}

	wg       sync.WaitGroup
	mu       sync.RWMutex
	closed   bool
	closeErr error
}

// NewQueuedLogger returns a QueuedAuditLogger delivering events to sink. It
// starts the configured number of background workers immediately.
func NewQueuedLogger(sink *AuditLogger, opts QueuedOptions) *QueuedAuditLogger {
	opts.setDefaults()

	ctx, cancel := context.WithCancelCause(context.Background())
	q := &QueuedAuditLogger{
		sink:   sink,
		opts:   opts,
		queue:  make(chan plog.Logs, opts.QueueSize),
		cancel: cancel,
		done:   make(chan struct{}),
	}

	q.wg.Add(opts.Workers)

	for range opts.Workers {
		go q.worker(ctx)
	}

	go func() {
		q.wg.Wait()
		q.cancel(nil)
		close(q.done)
	}()

	return q
}

// SendEvent copies and enqueues logs without waiting for delivery or queue space.
// It returns ErrQueueFull if the queue is full, or ErrLoggerClosed after Close
// begins. Logs must be initialized, for example with plog.NewLogs. Callers may
// modify logs after SendEvent returns, but not concurrently with the copy.
func (q *QueuedAuditLogger) SendEvent(logs plog.Logs) error {
	q.mu.RLock()
	defer q.mu.RUnlock()

	if q.closed {
		return ErrLoggerClosed
	}

	snapshot := plog.NewLogs()
	logs.CopyTo(snapshot)

	select {
	case q.queue <- snapshot:
		return nil
	default:
		return ErrQueueFull
	}
}

// Close stops accepting new events, drains events already queued (with retries),
// and waits for the background workers to finish. Delivery failures are reported
// through OnError, not returned here. If ctx is canceled, Close cancels delivery
// and retry waits and returns the context error; workers report discarded events
// as they exit. Concurrent and repeated calls are safe. Cancellation by any caller
// aborts delivery for all callers, and subsequent calls return that shutdown error.
func (q *QueuedAuditLogger) Close(ctx context.Context) error {
	q.mu.Lock()
	if !q.closed {
		q.closed = true
		close(q.queue)
	}
	q.mu.Unlock()

	// Prefer a completed drain over a canceled context.
	select {
	case <-q.done:
		return q.shutdownError()
	default:
	}

	select {
	case <-q.done:
		return q.shutdownError()
	case <-ctx.Done():
		q.mu.Lock()
		defer q.mu.Unlock()

		select {
		case <-q.done:
		default:
			if q.closeErr == nil {
				q.closeErr = ctx.Err()
				q.cancel(q.closeErr)
			}
		}

		return q.closeErr
	}
}

func (q *QueuedAuditLogger) shutdownError() error {
	q.mu.RLock()
	defer q.mu.RUnlock()

	return q.closeErr
}

// worker delivers queued events until the queue is closed and drained. ctx is
// cancelled on a forced shutdown to abort in-flight sends and backoff waits.
func (q *QueuedAuditLogger) worker(ctx context.Context) {
	defer q.wg.Done()

	for logs := range q.queue {
		q.deliver(ctx, logs)
	}
}

// deliver sends a single event, retrying transient failures with exponential
// backoff. Each failed attempt or shutdown discard is reported to OnError.
func (q *QueuedAuditLogger) deliver(ctx context.Context, logs plog.Logs) {
	for attempt := 0; ; attempt++ {
		if q.reportShutdown(ctx) {
			return
		}

		err := q.sink.SendEvent(ctx, logs)
		if err == nil {
			return
		}

		if q.reportShutdown(ctx) {
			return
		}

		q.reportError(err)

		if !isRetryableAuditError(err) || attempt >= q.opts.MaxRetries {
			return
		}

		if !q.wait(ctx, q.backoff(attempt+1)) {
			q.reportShutdown(ctx)
			return
		}
	}
}

func (q *QueuedAuditLogger) reportShutdown(ctx context.Context) bool {
	cause := context.Cause(ctx)
	if cause != nil {
		q.reportError(fmt.Errorf("audit event discarded during shutdown: %w", cause))
		return true
	}

	return false
}

// wait sleeps for d, returning false if a forced shutdown happened first.
func (q *QueuedAuditLogger) wait(ctx context.Context, d time.Duration) bool {
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
// doubling BaseDelay up to MaxDelay with equal jitter.
func (q *QueuedAuditLogger) backoff(attempt int) time.Duration {
	d := q.opts.MaxDelay
	// Cap before shifting to avoid overflowing time.Duration.
	if shift := attempt - 1; shift < 63 && q.opts.BaseDelay <= d>>shift {
		d = q.opts.BaseDelay << shift
	}

	// Equal jitter: half fixed, half random in [0, d/2).
	half := d / 2

	return half + time.Duration(rand.Int64N(int64(half)+1))
}

func (q *QueuedAuditLogger) reportError(err error) {
	if err == nil || q.opts.OnError == nil {
		return
	}

	q.opts.OnError(oops.In(domain).
		Hint("failed to deliver audit logs").
		Wrap(err))
}

func isRetryableAuditError(err error) bool {
	if statusErr, ok := errors.AsType[*httpStatusError](err); ok {
		return statusErr.statusCode == http.StatusTooManyRequests ||
			statusErr.statusCode == http.StatusBadGateway ||
			statusErr.statusCode == http.StatusServiceUnavailable ||
			statusErr.statusCode == http.StatusGatewayTimeout
	}

	// url.Error also wraps permanent failures such as invalid URLs or
	// certificates. Classify its underlying cause instead.
	if urlErr, ok := errors.AsType[*url.Error](err); ok {
		err = urlErr.Err
	}

	_, networkError := errors.AsType[net.Error](err)

	return networkError || errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF)
}
