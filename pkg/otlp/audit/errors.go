package otlpaudit

import (
	"errors"
)

var domain = "audit-logger:otlp"
var errEventCreation = errors.New("event creation failed")
var errNoLogRecord = errors.New("no log record present in the plog.Logs struct")
var ErrQueueFull = errors.New("audit event queue is full")
var ErrLoggerClosed = errors.New("queued audit logger is closed")

type httpStatusError struct {
	statusCode int
}

func (e *httpStatusError) Error() string {
	return "response status not OK"
}
