package otlpaudit

import (
	"context"
	"math/rand/v2"
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
	// events (returning errQueueFull) once the queue is full. Defaults to 1024.
	QueueSize int

	// Workers is the number of background goroutines delivering events to the
	// underlying logger. Defaults to 1.
	Workers int

	// MaxRetries is the number of additional delivery attempts after the first
	// one fails. Each retry is spaced by an exponential backoff. Once exhausted
	// the event is reported to OnError and dropped. Values <= 0 default to 5.
	MaxRetries int

	// BaseDelay is the first backoff step. Subsequent steps double it up to
	// MaxDelay. Defaults to 100ms.
	BaseDelay time.Duration

	// MaxDelay caps the backoff step. Defaults to 30s.
	MaxDelay time.Duration

	// OnError receives errors from background delivery, which cannot be returned
	// to a caller: events dropped because the queue was full are not routed here
	// (SendEvent returns errQueueFull directly), but events dropped after
	// exhausting all retries are. When nil such errors are dropped silently;
	// supply a handler (e.g. logging) to observe them.
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
}

// QueuedAuditLogger wraps an AuditLogger with a non-blocking, channel-based
// queue and background delivery. It is safe for concurrent use by multiple
// goroutines. Create one with NewQueuedLogger and always Close it to drain
// buffered events.
type QueuedAuditLogger struct {
	sink  *AuditLogger
	opts  QueuedOptions
	queue chan plog.Logs

	// stopCancel cancels the context threaded through the workers to abort
	// in-flight sends and pending backoff waits on a forced shutdown.
	stopCancel context.CancelFunc

	wg     sync.WaitGroup
	mu     sync.Mutex
	closed bool
}

// NewQueuedLogger returns a QueuedAuditLogger delivering events to sink. It
// starts the configured number of background workers immediately.
func NewQueuedLogger(sink *AuditLogger, opts QueuedOptions) *QueuedAuditLogger {
	opts.setDefaults()

	stopCtx, stopCancel := context.WithCancel(context.Background())
	q := &QueuedAuditLogger{
		sink:       sink,
		opts:       opts,
		queue:      make(chan plog.Logs, opts.QueueSize),
		stopCancel: stopCancel,
	}

	q.wg.Add(opts.Workers)

	for range opts.Workers {
		go q.worker(stopCtx)
	}

	return q
}

// SendEvent enqueues logs for background delivery and returns immediately. It
// never blocks: if the queue is full the event is dropped and errQueueFull is
// returned. After Close it returns errLoggerClosed.
func (q *QueuedAuditLogger) SendEvent(logs plog.Logs) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	if q.closed {
		return errLoggerClosed
	}

	// The send and the close(q.queue) in Close are both guarded by q.mu, so a
	// send on a closed channel cannot happen. The default branch keeps this
	// non-blocking even while holding the lock.
	select {
	case q.queue <- logs:
		return nil
	default:
		return errQueueFull
	}
}

// Close stops accepting new events, drains events already queued (with retries),
// and waits for the background workers to finish. It is idempotent. If ctx is
// cancelled before the queue is drained, in-flight sends and pending backoffs
// are aborted and ctx's error is returned; otherwise it returns nil.
func (q *QueuedAuditLogger) Close(ctx context.Context) error {
	q.mu.Lock()
	if q.closed {
		q.mu.Unlock()
		return nil
	}

	q.closed = true
	close(q.queue)
	q.mu.Unlock()

	done := make(chan struct{})

	go func() {
		select {
		case <-ctx.Done():
			q.stopCancel() // deadline hit: stop retrying/draining
		case <-done:
		}
	}()

	q.wg.Wait()
	close(done)
	q.stopCancel() // release the workers' context once they have exited

	return ctx.Err()
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
// backoff. On a forced shutdown it abandons the event; after exhausting all
// retries it reports the last error to OnError.
func (q *QueuedAuditLogger) deliver(ctx context.Context, logs plog.Logs) {
	var err error

	for attempt := 0; attempt <= q.opts.MaxRetries; attempt++ {
		if attempt > 0 && !q.wait(ctx, q.backoff(attempt)) {
			return // forced shutdown during backoff; abandon this event
		}

		err = q.sink.SendEvent(ctx, logs)
		if err == nil {
			return
		}
	}

	q.reportError(err)
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
	// Guard the shift against overflow for large attempt counts.
	if shift := attempt - 1; shift < 62 {
		if scaled := q.opts.BaseDelay << shift; scaled > 0 && scaled < d {
			d = scaled
		}
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
		Hint("failed to deliver audit logs after retries").
		Wrap(err))
}
