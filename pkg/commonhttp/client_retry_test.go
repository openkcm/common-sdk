package commonhttp_test

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/openkcm/common-sdk/pkg/commoncfg"
	"github.com/openkcm/common-sdk/pkg/commonhttp"
)

func TestHTTPClientRetry(t *testing.T) {
	tests := []struct {
		name         string
		responses    []int
		retryOn      []int
		maxAttempts  int
		wantAttempts int
		wantStatus   int
	}{
		{
			name:         "no retry on 200",
			responses:    []int{200},
			retryOn:      []int{429, 500},
			maxAttempts:  3,
			wantAttempts: 1,
			wantStatus:   200,
		},
		{
			name:         "retries on 500 then succeeds",
			responses:    []int{500, 500, 200},
			retryOn:      []int{500},
			maxAttempts:  3,
			wantAttempts: 3,
			wantStatus:   200,
		},
		{
			name:         "exhausts retries and returns last response",
			responses:    []int{429, 429, 429},
			retryOn:      []int{429},
			maxAttempts:  3,
			wantAttempts: 3,
			wantStatus:   429,
		},
		{
			name:         "retries on 429 then succeeds",
			responses:    []int{429, 200},
			retryOn:      []int{429, 500},
			maxAttempts:  3,
			wantAttempts: 2,
			wantStatus:   200,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0

			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if calls < len(tc.responses) {
					w.WriteHeader(tc.responses[calls])
				} else {
					w.WriteHeader(http.StatusInternalServerError)
				}

				calls++
			}))
			defer srv.Close()

			client, err := commonhttp.NewHTTPClient(&commoncfg.HTTPClient{
				Timeout: 5 * time.Second,
				Retry: commoncfg.HTTPRetry{
					MaxAttempts:          tc.maxAttempts,
					InitialBackoff:       time.Millisecond,
					MaxBackoff:           5 * time.Millisecond,
					RetryableStatusCodes: tc.retryOn,
				},
			})
			require.NoError(t, err)

			resp, err := client.Get(srv.URL)
			require.NoError(t, err)

			defer resp.Body.Close()

			assert.Equal(t, tc.wantAttempts, calls)
			assert.Equal(t, tc.wantStatus, resp.StatusCode)
		})
	}
}

func TestHTTPClientRetryWithBody(t *testing.T) {
	calls := 0

	var bodies []string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		bodies = append(bodies, string(b))

		calls++
		if calls < 3 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	client, err := commonhttp.NewHTTPClient(&commoncfg.HTTPClient{
		Timeout: 5 * time.Second,
		Retry: commoncfg.HTTPRetry{
			MaxAttempts:          3,
			InitialBackoff:       time.Millisecond,
			MaxBackoff:           5 * time.Millisecond,
			RetryableStatusCodes: []int{500},
		},
	})
	require.NoError(t, err)

	resp, err := client.Post(srv.URL, "application/json", strings.NewReader(`{"hello":"world"}`))
	require.NoError(t, err)

	defer resp.Body.Close()

	assert.Equal(t, 3, calls)
	assert.Equal(t, 200, resp.StatusCode)

	for i, b := range bodies {
		assert.JSONEq(t, `{"hello":"world"}`, b, "attempt %d body mismatch", i+1)
	}
}

// TestHTTPClientRetryWithOAuth2 verifies that the retry layer re-runs the full
// OAuth2 RoundTripper on each attempt, so credentials are injected correctly
// even when the body must be replayed from the retry buffer.
func TestHTTPClientRetryWithOAuth2(t *testing.T) {
	calls := 0

	var bodies []string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		bodies = append(bodies, string(b))

		calls++
		if calls < 3 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	client, err := commonhttp.NewHTTPClient(&commoncfg.HTTPClient{
		Timeout: 5 * time.Second,
		OAuth2Auth: &commoncfg.OAuth2{
			Credentials: commoncfg.OAuth2Credentials{
				ClientID:   commoncfg.SourceRef{Source: commoncfg.EmbeddedSourceValue, Value: "my-client"},
				AuthMethod: commoncfg.OAuth2ClientSecretBasic,
				ClientSecret: &commoncfg.SourceRef{
					Source: commoncfg.EmbeddedSourceValue,
					Value:  "my-secret",
				},
			},
		},
		Retry: commoncfg.HTTPRetry{
			MaxAttempts:          3,
			InitialBackoff:       time.Millisecond,
			MaxBackoff:           5 * time.Millisecond,
			RetryableStatusCodes: []int{500},
		},
	})
	require.NoError(t, err)

	pr, pw := io.Pipe()
	go func() {
		_, _ = io.WriteString(pw, `{"hello":"world"}`)
		pw.Close()
	}()

	req, err := http.NewRequest(http.MethodPost, srv.URL, pr)
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	require.NoError(t, err)

	defer resp.Body.Close()

	assert.Equal(t, 3, calls)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	for i, b := range bodies {
		assert.JSONEq(t, `{"hello":"world"}`, b, "attempt %d body mismatch", i+1)
	}
}

// TestHTTPClientRetryGetBodyFails verifies that a GetBody failure is treated as
// unrecoverable: the error is returned immediately with no further attempts.
func TestHTTPClientRetryGetBodyFails(t *testing.T) {
	calls := 0

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++

		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	client, err := commonhttp.NewHTTPClient(&commoncfg.HTTPClient{
		Timeout: 5 * time.Second,
		Retry: commoncfg.HTTPRetry{
			MaxAttempts:          3,
			InitialBackoff:       time.Millisecond,
			MaxBackoff:           5 * time.Millisecond,
			RetryableStatusCodes: []int{500},
		},
	})
	require.NoError(t, err)

	bodyErr := errors.New("body unavailable")

	req, err := http.NewRequest(http.MethodPost, srv.URL, strings.NewReader("data"))
	require.NoError(t, err)

	req.GetBody = func() (io.ReadCloser, error) { return nil, bodyErr }

	_, err = client.Do(req) //nolint:bodyclose

	require.ErrorIs(t, err, bodyErr)
	assert.Equal(t, 0, calls)
}

// the retryable transport must buffer it itself.
func TestHTTPClientRetryWithBodyNoGetBody(t *testing.T) {
	calls := 0

	var bodies []string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		bodies = append(bodies, string(b))

		calls++
		if calls < 3 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	client, err := commonhttp.NewHTTPClient(&commoncfg.HTTPClient{
		Timeout: 5 * time.Second,
		Retry: commoncfg.HTTPRetry{
			MaxAttempts:          3,
			InitialBackoff:       time.Millisecond,
			MaxBackoff:           5 * time.Millisecond,
			RetryableStatusCodes: []int{500},
		},
	})
	require.NoError(t, err)

	// io.NopCloser wraps a plain bytes.Reader but does not implement WriteTo,
	// so net/http cannot detect the size and leaves GetBody nil.
	pr, pw := io.Pipe()
	go func() {
		_, _ = io.WriteString(pw, `{"hello":"world"}`)
		pw.Close()
	}()

	req, err := http.NewRequest(http.MethodPost, srv.URL, pr)
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	require.NoError(t, err)

	defer resp.Body.Close()

	assert.Equal(t, 3, calls)
	assert.Equal(t, 200, resp.StatusCode)

	for i, b := range bodies {
		assert.JSONEq(t, `{"hello":"world"}`, b, "attempt %d body mismatch", i+1)
	}
}
