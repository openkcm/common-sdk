package audit

import "time"

const (
	defaultQueueSize     = 1024
	defaultMaxConcurrent = 10
	defaultMaxRetries    = 5
	defaultBaseDelay     = 100 * time.Millisecond
	defaultMaxDelay      = 30 * time.Second
)

// Option configures an Auditor
type Option func(*Auditor)

// WithQueueSize sets the capacity of the buffered event queue.
// Send drops events (returning ErrQueueFull) once the queue is full.
// Values <= 0 are ignored. Defaults to 1024.
func WithQueueSize(size int) Option {
	return func(a *Auditor) {
		if size > 0 {
			a.queueSize = size
		}
	}
}

// WithMaxConcurrent sets the maximum number of concurrent requests.
// Uses a semaphore to limit concurrency.
// For HTTP: limits concurrent HTTP requests (goroutine-per-event model).
// For gRPC: limits concurrent batch sends.
// Values <= 0 are ignored. Defaults to 10.
func WithMaxConcurrent(n int) Option {
	return func(a *Auditor) {
		if n > 0 {
			a.maxConcurrent = n
		}
	}
}

// WithMaxRetries sets the number of additional delivery attempts after the first
// one fails. Each retry is spaced by an exponential backoff. Once exhausted
// the event (or batch for gRPC) is reported to OnError and dropped.
// Values < 0 are ignored. Defaults to 5.
func WithMaxRetries(n int) Option {
	return func(a *Auditor) {
		if n >= 0 {
			a.maxRetries = n
		}
	}
}

// WithBaseDelay sets the first backoff step. Subsequent steps double it up to
// MaxDelay. Values <= 0 are ignored. Defaults to 100ms.
func WithBaseDelay(d time.Duration) Option {
	return func(a *Auditor) {
		if d > 0 {
			a.baseDelay = d
		}
	}
}

// WithMaxDelay caps the backoff step. Values <= 0 are ignored. Defaults to 30s.
func WithMaxDelay(d time.Duration) Option {
	return func(a *Auditor) {
		if d > 0 {
			a.maxDelay = d
		}
	}
}

// WithOnError receives errors from background delivery. Events dropped because
// the queue was full are not routed here (Send returns ErrQueueFull directly),
// but events dropped after exhausting all retries are. When nil such errors are
// dropped silently.
func WithOnError(fn func(error)) Option {
	return func(a *Auditor) {
		a.onError = fn
	}
}

// WithGRPCBatchSize sets the number of events to batch before sending.
// Only applies when using gRPC protocol.
// Batching improves efficiency by reducing the number of gRPC calls.
// Values <= 0 are ignored. Defaults to 100.
func WithGRPCBatchSize(size int) Option {
	return func(a *Auditor) {
		if size > 0 {
			a.batchSize = size
		}
	}
}

// WithGRPCBatchTimeout sets the maximum time to wait before sending a partial batch.
// Only applies when using gRPC protocol.
// If the batch size isn't reached within this timeout, the partial batch is sent.
// Values <= 0 are ignored. Defaults to 200ms.
func WithGRPCBatchTimeout(d time.Duration) Option {
	return func(a *Auditor) {
		if d > 0 {
			a.batchTimeout = d
		}
	}
}

// WithGRPCRequestTimeout sets the timeout for individual gRPC requests.
// Only applies when using gRPC protocol.
// Values <= 0 are ignored. Defaults to 30s.
func WithGRPCRequestTimeout(d time.Duration) Option {
	return func(a *Auditor) {
		a.grpcSenderOpts = append(a.grpcSenderOpts, WithRequestTimeout(d))
	}
}

// WithGRPCCompression enables or disables gzip compression for gRPC requests.
// Only applies when using gRPC protocol.
// Compression is enabled by default.
func WithGRPCCompression(enabled bool) Option {
	return func(a *Auditor) {
		a.grpcSenderOpts = append(a.grpcSenderOpts, WithCompression(enabled))
	}
}

// WithGRPCConnectionStateCallback sets a callback to be invoked when the gRPC
// connection state changes. Only applies when using gRPC protocol.
func WithGRPCConnectionStateCallback(cb ConnectionStateCallback) Option {
	return func(a *Auditor) {
		a.grpcSenderOpts = append(a.grpcSenderOpts, WithConnectionStateCallback(cb))
	}
}
