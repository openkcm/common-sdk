package otlpaudit

import (
	"errors"
)

var domain = "audit-logger:otlp"
var errEventCreation = errors.New("event creation failed")
var errNoLogRecord = errors.New("no log record present in the plog.Logs struct")
var errQueueFull = errors.New("audit event queue is full")
var errLoggerClosed = errors.New("queued audit logger is closed")
