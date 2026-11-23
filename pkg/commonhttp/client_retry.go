package commonhttp

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"slices"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	retry "github.com/avast/retry-go/v5"

	"github.com/openkcm/common-sdk/pkg/commoncfg"
)

type retryableClient struct {
	Next  http.RoundTripper
	Retry commoncfg.HTTPRetry
}

// RoundTrip traces the retry operation through the final response headers. The
// inner HTTP transport owns each attempt's span, metrics and response body.
func (c *retryableClient) RoundTrip(r *http.Request) (*http.Response, error) {
	ctx, span := otel.Tracer(instrumentationName).Start(r.Context(), "http.retry", trace.WithSpanKind(trace.SpanKindInternal))
	defer span.End()

	response, attempts, err := c.roundTrip(r.Clone(ctx))
	span.SetAttributes(attribute.Int("retry.attempts", attempts))

	if response != nil {
		span.SetAttributes(attribute.Int("http.response.status_code", response.StatusCode))
	}

	if err != nil || (response != nil && response.StatusCode >= http.StatusBadRequest) {
		// Keep arbitrary transport/authentication error text out of this span.
		span.SetStatus(codes.Error, "HTTP request failed")
	}

	return response, err
}

func (c *retryableClient) roundTrip(r *http.Request) (*http.Response, int, error) {
	attempts := 0
	// Buffer body upfront so it can be replayed on each attempt.
	if r.Body != nil && r.Body != http.NoBody && r.GetBody == nil {
		buf, err := io.ReadAll(r.Body)
		r.Body.Close()

		if err != nil {
			return nil, attempts, fmt.Errorf("retry: failed to buffer request body: %w", err)
		}

		r.GetBody = func() (io.ReadCloser, error) {
			return io.NopCloser(bytes.NewReader(buf)), nil
		}
		r.Body, _ = r.GetBody()
	}

	var lastResp *http.Response

	err := retry.New(
		retry.Attempts(uint(max(0, c.Retry.MaxAttempts))),
		retry.Delay(c.Retry.InitialBackoff),
		retry.MaxDelay(c.Retry.MaxBackoff),
		retry.DelayType(retry.BackOffDelay),
		retry.Context(r.Context()),
	).Do(func() error {
		// Reset the body for each attempt.
		if r.GetBody != nil {
			var err error

			r.Body, err = r.GetBody()
			if err != nil {
				return retry.Unrecoverable(err)
			}
		}

		attempts++

		resp, err := c.Next.RoundTrip(r)
		if err != nil {
			if resp != nil {
				resp.Body.Close()
			}

			return err
		}

		lastResp = resp

		if slices.Contains(c.Retry.RetryableStatusCodes, resp.StatusCode) {
			if attempts < c.Retry.MaxAttempts {
				// End this attempt before backoff. Only the final response is
				// kept open for the caller, including when retries are exhausted.
				_ = resp.Body.Close()
				lastResp = nil
			}

			return retryableStatusError(resp.StatusCode)
		}

		return nil
	})
	if err != nil {
		// Only return the last response if all attempts were exhausted on a
		// retryable status code. For any other error (network, GetBody, context
		// cancelled) surface the error regardless.
		var statusErr retryableStatusError
		if lastResp != nil && errors.As(err, &statusErr) {
			return lastResp, attempts, nil
		}

		if lastResp != nil {
			lastResp.Body.Close()
		}

		return nil, attempts, err
	}

	return lastResp, attempts, nil
}

// retryableStatusError signals retry-go to retry for a given HTTP status code.
type retryableStatusError int

func (e retryableStatusError) Error() string {
	return http.StatusText(int(e))
}
