package otlpaudit

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"

	"go.opentelemetry.io/collector/pdata/plog"
)

func (o *otlpClient) send(ctx context.Context, payload string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, o.Endpoint, bytes.NewBufferString(payload))
	if err != nil {
		return errors.Join(errCreateReqFailed, err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	if o.BasicAuth != nil {
		req.SetBasicAuth(o.BasicAuth.username, o.BasicAuth.password)
	}

	resp, err := o.Client.Do(req)
	if err != nil {
		return errors.Join(errReqFailed, err)
	}

	defer func() {
		err = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("%w %d", errStatusNotOK, resp.StatusCode)
	}

	return err
}

func (auditLogger *AuditLogger) SendEvent(ctx context.Context, logs plog.Logs) error {
	err := auditLogger.enrichLogs(&logs)
	if err != nil {
		return err
	}

	marshaller := plog.JSONMarshaler{}

	marshaledLogs, err := marshaller.MarshalLogs(logs)
	if err != nil {
		return errors.Join(errMarshalingFailed, err)
	}

	err = auditLogger.client.send(ctx, string(marshaledLogs))
	if err != nil {
		return err
	}

	return nil
}

func (auditLogger *AuditLogger) enrichLogs(logs *plog.Logs) error {
	logRecord, err := getFirstLogRecord(*logs)
	if err != nil {
		return err
	}

	for k, v := range auditLogger.additionalProps {
		logRecord.Attributes().PutStr(k, v)
	}

	return nil
}

func getFirstLogRecord(ld plog.Logs) (plog.LogRecord, error) {
	if ld.ResourceLogs().Len() > 0 && ld.ResourceLogs().At(0).ScopeLogs().Len() > 0 && ld.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().Len() > 0 {
		return ld.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0), nil
	} else {
		return plog.LogRecord{}, errNoLogRecord
	}
}
