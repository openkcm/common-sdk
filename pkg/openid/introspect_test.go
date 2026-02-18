package openid

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIntrospectToken(t *testing.T) {
	t.Parallel()

	t.Run("success with active token", func(t *testing.T) {
		t.Parallel()

		expectedResponse := IntrospectResponse{
			Active: true,
			Groups: []string{"admin", "users"},
		}

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodPost, r.Method)
			assert.Equal(t, "application/x-www-form-urlencoded", r.Header.Get("Content-Type"))
			assert.Equal(t, "application/json", r.Header.Get("Accept"))
			assert.Equal(t, "test-token", r.URL.Query().Get("token"))

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(expectedResponse)
		}))
		defer server.Close()

		cfg := Configuration{
			IntrospectionEndpoint: server.URL,
		}

		resp, err := cfg.IntrospectToken(context.Background(), "test-token", nil)

		require.NoError(t, err)
		assert.True(t, resp.Active)
		assert.Equal(t, []string{"admin", "users"}, resp.Groups)
	})

	t.Run("success with inactive token", func(t *testing.T) {
		t.Parallel()

		expectedResponse := IntrospectResponse{
			Active: false,
		}

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(expectedResponse)
		}))
		defer server.Close()

		cfg := Configuration{
			IntrospectionEndpoint: server.URL,
		}

		resp, err := cfg.IntrospectToken(context.Background(), "invalid-token", nil)

		require.NoError(t, err)
		assert.False(t, resp.Active)
	})

	t.Run("success with additional query parameters", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "test-token", r.URL.Query().Get("token"))
			assert.Equal(t, "hint-value", r.URL.Query().Get("token_type_hint"))
			assert.Equal(t, "custom-value", r.URL.Query().Get("custom_param"))

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(IntrospectResponse{Active: true})
		}))
		defer server.Close()

		cfg := Configuration{
			IntrospectionEndpoint: server.URL,
		}

		additionalParams := map[string]string{
			"token_type_hint": "hint-value",
			"custom_param":    "custom-value",
		}

		resp, err := cfg.IntrospectToken(context.Background(), "test-token", additionalParams)

		require.NoError(t, err)
		assert.True(t, resp.Active)
	})

	t.Run("success with error response fields", func(t *testing.T) {
		t.Parallel()

		expectedResponse := IntrospectResponse{
			Active:           false,
			Error:            "invalid_client",
			ErrorDescription: "Client authentication failed",
		}

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(expectedResponse)
		}))
		defer server.Close()

		cfg := Configuration{
			IntrospectionEndpoint: server.URL,
		}

		resp, err := cfg.IntrospectToken(context.Background(), "test-token", nil)

		require.NoError(t, err)
		assert.False(t, resp.Active)
		assert.Equal(t, "invalid_client", resp.Error)
		assert.Equal(t, "Client authentication failed", resp.ErrorDescription)
	})

	t.Run("error when no introspection endpoint configured", func(t *testing.T) {
		t.Parallel()

		cfg := Configuration{
			IntrospectionEndpoint: "",
		}

		resp, err := cfg.IntrospectToken(context.Background(), "test-token", nil)

		require.ErrorIs(t, err, ErrNoIntrospectionEndpoint)
		assert.Equal(t, IntrospectResponse{}, resp)
	})

	t.Run("error when introspection endpoint is invalid URL", func(t *testing.T) {
		t.Parallel()

		cfg := Configuration{
			IntrospectionEndpoint: "://invalid-url",
		}

		resp, err := cfg.IntrospectToken(context.Background(), "test-token", nil)

		require.Error(t, err)
		require.ErrorIs(t, err, ErrCouldNotCreateHTTPRequest)
		assert.Equal(t, IntrospectResponse{}, resp)
	})

	t.Run("error when HTTP request fails", func(t *testing.T) {
		t.Parallel()

		cfg := Configuration{
			IntrospectionEndpoint: "http://localhost:1", // non-existent server
		}

		resp, err := cfg.IntrospectToken(context.Background(), "test-token", nil)

		require.Error(t, err)
		require.ErrorIs(t, err, ErrCouldNotDoHTTPRequest)
		assert.Equal(t, IntrospectResponse{}, resp)
	})

	t.Run("error when provider responds with non-200 status", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"error": "unauthorized"}`))
		}))
		defer server.Close()

		cfg := Configuration{
			IntrospectionEndpoint: server.URL,
		}

		resp, err := cfg.IntrospectToken(context.Background(), "test-token", nil)

		require.Error(t, err)

		var errNon200 ProviderRespondedNon200Error
		require.ErrorAs(t, err, &errNon200)
		assert.Equal(t, http.StatusUnauthorized, errNon200.Code)
		assert.JSONEq(t, `{"error": "unauthorized"}`, errNon200.Body)
		assert.Equal(t, IntrospectResponse{}, resp)
	})

	t.Run("error when provider responds with 500 status", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte("internal server error"))
		}))
		defer server.Close()

		cfg := Configuration{
			IntrospectionEndpoint: server.URL,
		}

		resp, err := cfg.IntrospectToken(context.Background(), "test-token", nil)

		require.Error(t, err)

		var errNon200 ProviderRespondedNon200Error
		require.ErrorAs(t, err, &errNon200)
		assert.Equal(t, http.StatusInternalServerError, errNon200.Code)
		assert.Equal(t, IntrospectResponse{}, resp)
	})

	t.Run("error when response body is invalid JSON", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("not valid json"))
		}))
		defer server.Close()

		cfg := Configuration{
			IntrospectionEndpoint: server.URL,
		}

		resp, err := cfg.IntrospectToken(context.Background(), "test-token", nil)

		require.Error(t, err)

		var errDecode CouldNotDecodeResponseError
		require.ErrorAs(t, err, &errDecode)
		assert.Equal(t, "not valid json", errDecode.Body)
		assert.Equal(t, IntrospectResponse{}, resp)
	})

	t.Run("error when response body has invalid JSON structure", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"active": "not-a-bool"}`))
		}))
		defer server.Close()

		cfg := Configuration{
			IntrospectionEndpoint: server.URL,
		}

		resp, err := cfg.IntrospectToken(context.Background(), "test-token", nil)

		require.Error(t, err)

		var errDecode CouldNotDecodeResponseError
		require.ErrorAs(t, err, &errDecode)
		assert.Equal(t, IntrospectResponse{}, resp)
	})

	t.Run("success with custom HTTP client", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(IntrospectResponse{Active: true})
		}))
		defer server.Close()

		customClient := &http.Client{
			Transport: &customRoundTripper{
				base:        http.DefaultTransport,
				headerKey:   "X-Custom-Header",
				headerValue: "custom-value",
			},
		}

		cfg := Configuration{
			IntrospectionEndpoint: server.URL,
			HTTPClient:            customClient,
		}

		resp, err := cfg.IntrospectToken(context.Background(), "test-token", nil)

		require.NoError(t, err)
		assert.True(t, resp.Active)
	})

	t.Run("uses default HTTP client when HTTPClient is nil", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(IntrospectResponse{Active: true})
		}))
		defer server.Close()

		cfg := Configuration{
			IntrospectionEndpoint: server.URL,
			HTTPClient:            nil,
		}

		resp, err := cfg.IntrospectToken(context.Background(), "test-token", nil)

		require.NoError(t, err)
		assert.True(t, resp.Active)
	})

	t.Run("context cancellation", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Simulate slow response
			<-r.Context().Done()
		}))
		defer server.Close()

		cfg := Configuration{
			IntrospectionEndpoint: server.URL,
		}

		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		resp, err := cfg.IntrospectToken(ctx, "test-token", nil)

		require.Error(t, err)
		require.ErrorIs(t, err, ErrCouldNotDoHTTPRequest)
		assert.Equal(t, IntrospectResponse{}, resp)
	})

	t.Run("empty response body with 200 status", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			// Empty body
		}))
		defer server.Close()

		cfg := Configuration{
			IntrospectionEndpoint: server.URL,
		}

		resp, err := cfg.IntrospectToken(context.Background(), "test-token", nil)

		require.Error(t, err)

		var errDecode CouldNotDecodeResponseError
		require.ErrorAs(t, err, &errDecode)
		assert.Equal(t, IntrospectResponse{}, resp)
	})

	t.Run("token with special characters is properly encoded", func(t *testing.T) {
		t.Parallel()

		specialToken := "token+with/special=chars&more"

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Verify the token is properly URL-encoded and decoded
			assert.Equal(t, specialToken, r.URL.Query().Get("token"))

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(IntrospectResponse{Active: true})
		}))
		defer server.Close()

		cfg := Configuration{
			IntrospectionEndpoint: server.URL,
		}

		resp, err := cfg.IntrospectToken(context.Background(), specialToken, nil)

		require.NoError(t, err)
		assert.True(t, resp.Active)
	})

	t.Run("error reading response body", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Set content length but don't write full body to cause read error
			w.Header().Set("Content-Length", "1000")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("partial"))
			// Close connection prematurely by hijacking
			if hj, ok := w.(http.Hijacker); ok {
				conn, _, _ := hj.Hijack()
				conn.Close()
			}
		}))
		defer server.Close()

		cfg := Configuration{
			IntrospectionEndpoint: server.URL,
		}

		resp, err := cfg.IntrospectToken(context.Background(), "test-token", nil)

		require.Error(t, err)
		require.ErrorIs(t, err, ErrCouldNotReadResponseBody)
		assert.Equal(t, IntrospectResponse{}, resp)
	})
}

// customRoundTripper is a helper for testing custom HTTP clients.
type customRoundTripper struct {
	base        http.RoundTripper
	headerKey   string
	headerValue string
}

func (c *customRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	req.Header.Set(c.headerKey, c.headerValue)
	return c.base.RoundTrip(req)
}

func TestIntrospectResponse(t *testing.T) {
	t.Parallel()

	t.Run("unmarshal complete response", func(t *testing.T) {
		t.Parallel()

		jsonData := `{
			"active": true,
			"groups": ["admin", "users"],
			"error": "some_error",
			"error_description": "Some error description"
		}`

		var resp IntrospectResponse

		err := json.Unmarshal([]byte(jsonData), &resp)

		require.NoError(t, err)
		assert.True(t, resp.Active)
		assert.Equal(t, []string{"admin", "users"}, resp.Groups)
		assert.Equal(t, "some_error", resp.Error)
		assert.Equal(t, "Some error description", resp.ErrorDescription)
	})

	t.Run("unmarshal minimal response", func(t *testing.T) {
		t.Parallel()

		jsonData := `{"active": false}`

		var resp IntrospectResponse

		err := json.Unmarshal([]byte(jsonData), &resp)

		require.NoError(t, err)
		assert.False(t, resp.Active)
		assert.Nil(t, resp.Groups)
		assert.Empty(t, resp.Error)
		assert.Empty(t, resp.ErrorDescription)
	})

	t.Run("unmarshal with empty groups", func(t *testing.T) {
		t.Parallel()

		jsonData := `{"active": true, "groups": []}`

		var resp IntrospectResponse

		err := json.Unmarshal([]byte(jsonData), &resp)

		require.NoError(t, err)
		assert.True(t, resp.Active)
		assert.Empty(t, resp.Groups)
	})
}

func TestErrProviderRespondedNon200_Error(t *testing.T) {
	t.Parallel()

	err := ProviderRespondedNon200Error{
		Code: 401,
		Body: "unauthorized",
	}

	assert.Equal(t, "provider responded with non-200 status code: 401", err.Error())
}

func TestErrCouldNotDecodeResponse_Error(t *testing.T) {
	t.Parallel()

	err := CouldNotDecodeResponseError{
		Err:  io.EOF,
		Body: "invalid json",
	}

	assert.Contains(t, err.Error(), "could not decode provider response")
	assert.Contains(t, err.Error(), "EOF")
}
