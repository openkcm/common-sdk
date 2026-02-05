package otlpaudit

import (
	"bytes"
	"context"
	"net/http"

	"github.com/samber/oops"
	"go.opentelemetry.io/collector/pdata/plog"
)

func (o *otlpClient) send(ctx context.Context, payload string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, o.Endpoint, bytes.NewBufferString(payload))
	if err != nil {
		return oops.In(domain).
			Hint("request failed").
			Wrap(err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := o.Client.Do(req)
	if err != nil {
		return oops.In(domain).
			Hint("request failed").
			Wrap(err)
	}

	defer func() {
		err = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return oops.In(domain).
			With("status_code", resp.StatusCode).New("response status not OK")
	}

	return err
}

func (auditLogger *AuditLogger) SendEvent(ctx context.Context, logs plog.Logs) error {
	err := auditLogger.enrichLogs(&logs)
	if err != nil {
		return oops.In(domain).
			Hint("enrich failed").
			Wrap(err)
	}

	marshaller := plog.JSONMarshaler{}

	marshaledLogs, err := marshaller.MarshalLogs(logs)
	if err != nil {
		return oops.In(domain).
			Hint("failed to marshal audit logs").
			Wrap(err)
	}

	err = auditLogger.client.send(ctx, string(marshaledLogs))
	if err != nil {
		return oops.In(domain).
			Hint("failed to send audit logs").
			Wrap(err)
	}

	return nil
}

func (auditLogger *AuditLogger) enrichLogs(logs *plog.Logs) error {
	logRecord, err := firstLogRecord(*logs)
	if err != nil {
		return oops.In(domain).
			Hint("failed to find audit record log").
			Wrap(err)
	}

	for k, v := range auditLogger.additionalProps {
		logRecord.Attributes().PutStr(k, v)
	}

	return nil
}

func firstLogRecord(ld plog.Logs) (*plog.LogRecord, error) {
	exist := ld.ResourceLogs().Len() > 0 &&
		ld.ResourceLogs().At(0).ScopeLogs().Len() > 0 &&
		ld.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().Len() > 0

	if exist {
		record := ld.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0)
		return &record, nil
	}

	return nil, errNoLogRecord
}
