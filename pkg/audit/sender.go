package audit

import (
	"context"

	"go.opentelemetry.io/collector/pdata/plog"
)

// Sender abstracts the transport layer for sending OTLP logs.
type Sender interface {
	// Send transmits logs to the configured endpoint.
	// Returns ErrPermanent for errors that should not be retried (e.g., 4xx HTTP, INVALID_ARGUMENT gRPC).
	Send(ctx context.Context, logs plog.Logs) error

	// Close releases any resources held by the sender.
	Close() error
}
