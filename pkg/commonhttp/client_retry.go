package commonhttp

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"slices"

	retry "github.com/avast/retry-go/v5"

	"github.com/openkcm/common-sdk/pkg/commoncfg"
)

type retryableClient struct {
	Next  http.RoundTripper
	Retry commoncfg.HTTPRetry
}

func (c *retryableClient) RoundTrip(r *http.Request) (*http.Response, error) {
	// Buffer body upfront so it can be replayed on each attempt.
	if r.Body != nil && r.Body != http.NoBody && r.GetBody == nil {
		buf, err := io.ReadAll(r.Body)
		r.Body.Close()

		if err != nil {
			return nil, fmt.Errorf("retry: failed to buffer request body: %w", err)
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

		resp, err := c.Next.RoundTrip(r)
		if err != nil {
			if resp != nil {
				resp.Body.Close()
			}

			return err
		}

		// Close the previous retryable response body before replacing it.
		if lastResp != nil {
			lastResp.Body.Close()
		}

		lastResp = resp

		if slices.Contains(c.Retry.RetryableStatusCodes, resp.StatusCode) {
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
			return lastResp, nil
		}

		if lastResp != nil {
			lastResp.Body.Close()
		}

		return nil, err
	}

	return lastResp, nil
}

// retryableStatusError signals retry-go to retry for a given HTTP status code.
type retryableStatusError int

func (e retryableStatusError) Error() string {
	return http.StatusText(int(e))
}
