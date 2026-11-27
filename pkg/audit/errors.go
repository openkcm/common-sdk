package audit

import "errors"

var (
	// ErrQueueFull is returned when the event queue is at capacity
	ErrQueueFull = errors.New("audit event queue is full")

	// ErrClosed is returned when attempting to send events after Close
	ErrClosed = errors.New("auditor is closed")

	// ErrPermanent is returned for 4xx errors that should not be retried
	ErrPermanent = errors.New("audit: permanent error (will not retry)")
)
