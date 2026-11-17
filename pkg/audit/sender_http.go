package audit

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"time"

	"go.opentelemetry.io/collector/pdata/plog"

	"github.com/openkcm/common-sdk/pkg/commoncfg"
	"github.com/openkcm/common-sdk/pkg/commonhttp"
)

const (
	// defaultHTTPRequestTimeout is the default timeout for HTTP requests when no context deadline is set.
	defaultHTTPRequestTimeout = 30 * time.Second
)

// httpSender implements Sender using HTTP transport.
type httpSender struct {
	endpoint string
	client   *http.Client
}

// newHTTPSender creates a new HTTP-based sender.
func newHTTPSender(cfg *commoncfg.Audit) (*httpSender, error) {
	httpClient, err := commonhttp.NewHTTPClient(&cfg.HTTPClient)
	if err != nil {
		return nil, fmt.Errorf("audit: failed to create HTTP client: %w", err)
	}

	return &httpSender{
		endpoint: cfg.Endpoint,
		client:   httpClient,
	}, nil
}

// Send converts logs to OTLP JSON and sends via HTTP POST.
func (s *httpSender) Send(ctx context.Context, logs plog.Logs) error {
	// Ensure we have a timeout
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		var cancel context.CancelFunc

		ctx, cancel = context.WithTimeout(ctx, defaultHTTPRequestTimeout)
		defer cancel()
	}

	// Marshal to OTLP JSON format
	marshaler := plog.JSONMarshaler{}

	data, err := marshaler.MarshalLogs(logs)
	if err != nil {
		return fmt.Errorf("audit: failed to marshal OTLP logs: %w", err)
	}

	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.endpoint, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("audit: failed to create HTTP request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	// Send request
	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("audit: HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}

	// 4xx errors are permanent (bad request, auth, etc.) - do not retry
	if resp.StatusCode >= 400 && resp.StatusCode < 500 {
		return fmt.Errorf("%w: status %d", ErrPermanent, resp.StatusCode)
	}

	// 5xx errors and other unexpected statuses - will retry
	return fmt.Errorf("audit: transient error, status: %d", resp.StatusCode)
}

// Close is a no-op for HTTP sender as http.Client doesn't need explicit cleanup.
func (s *httpSender) Close() error {
	return nil
}
