package otlpaudit

import (
	"errors"
)

var errCreateReqFailed = errors.New("request creation failed")
var errReqFailed = errors.New("request failed")
var errStatusNotOK = errors.New("response status not OK, got: ")
var errLoadValue = errors.New("failed to load value from ref: ")
var errMarshalingFailed = errors.New("marshaling failed: ")
var errEventCreation = errors.New("event creation failed")
var errLoadMTLSConfigFailed = errors.New("load mtls config failed: ")
var errNoLogRecord = errors.New("no log record present in the plog.Logs struct")
