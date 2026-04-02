package oidc

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIntrospectToken(t *testing.T) {
	t.Run("successfully introspects token", func(t *testing.T) {
		introspectionResult := Introspection{
			Active: true,
			Groups: []string{"admin", "users"},
		}

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/introspect" {
				assert.Equal(t, http.MethodPost, r.Method)
				assert.Equal(t, "application/x-www-form-urlencoded", r.Header.Get("Content-Type"))
				assert.Equal(t, "application/json", r.Header.Get("Accept"))
				assert.Equal(t, "test-token", r.PostFormValue("token"))

				w.Header().Set("Content-Type", "application/json")
				err := json.NewEncoder(w).Encode(introspectionResult)
				assert.NoError(t, err)

				return
			}
		}))
		defer server.Close()

		provider, err := NewProvider(server.URL, []string{"aud1"}, WithAllowHttpScheme(true))
		require.NoError(t, err)

		provider.config = &Configuration{
			Issuer:                server.URL,
			IntrospectionEndpoint: server.URL + "/introspect",
		}

		result, err := provider.IntrospectToken(context.Background(), "test-token")
		require.NoError(t, err)
		assert.True(t, result.Active)
		assert.Equal(t, []string{"admin", "users"}, result.Groups)
	})

	t.Run("sends additional query parameters", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/introspect" {
				assert.Equal(t, "test-token", r.PostFormValue("token"))
				assert.Equal(t, "value1", r.PostFormValue("key1"))
				assert.Equal(t, "value2", r.PostFormValue("key2"))

				w.Header().Set("Content-Type", "application/json")
				err := json.NewEncoder(w).Encode(Introspection{Active: true})
				assert.NoError(t, err)

				return
			}
		}))
		defer server.Close()

		provider, err := NewProvider(server.URL, []string{"aud1"},
			WithAllowHttpScheme(true),
			WithIntrospectQueryParameters(map[string]string{
				"key1": "value1",
				"key2": "value2",
			}))

		require.NoError(t, err)

		provider.config = &Configuration{
			IntrospectionEndpoint: server.URL + "/introspect",
		}

		result, err := provider.IntrospectToken(context.Background(), "test-token")
		require.NoError(t, err)
		assert.True(t, result.Active)
	})

	t.Run("fails when no introspection endpoint", func(t *testing.T) {
		provider, err := NewProvider("https://issuer.example.com", []string{"aud1"})
		require.NoError(t, err)

		provider.config = &Configuration{
			Issuer:                "https://issuer.example.com",
			IntrospectionEndpoint: "", // empty
		}

		_, err = provider.IntrospectToken(context.Background(), "test-token")
		require.Error(t, err)
		assert.ErrorIs(t, err, ErrNoIntrospectionEndpoint)
	})

	t.Run("handles non-200 response", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte("unauthorized"))
		}))
		defer server.Close()

		provider, err := NewProvider(server.URL, []string{"aud1"}, WithAllowHttpScheme(true))

		require.NoError(t, err)

		provider.config = &Configuration{
			IntrospectionEndpoint: server.URL + "/introspect",
		}

		_, err = provider.IntrospectToken(context.Background(), "test-token")
		require.Error(t, err)

		var non200Err ProviderRespondedNon200Error
		assert.ErrorAs(t, err, &non200Err)
		assert.Equal(t, http.StatusUnauthorized, non200Err.Code)
	})

	t.Run("handles invalid JSON response", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte("invalid json"))
		}))
		defer server.Close()

		provider, err := NewProvider(server.URL, []string{"aud1"}, WithAllowHttpScheme(true))
		require.NoError(t, err)

		provider.config = &Configuration{
			IntrospectionEndpoint: server.URL + "/introspect",
		}

		_, err = provider.IntrospectToken(context.Background(), "test-token")
		require.Error(t, err)

		var decodeErr CouldNotUnmarshallResponseError
		assert.ErrorAs(t, err, &decodeErr)
	})

	t.Run("handles configuration fetch error", func(t *testing.T) {
		provider, err := NewProvider("http://localhost:99999", []string{"aud1"}, WithAllowHttpScheme(true))
		require.NoError(t, err)

		_, err = provider.IntrospectToken(context.Background(), "test-token")
		require.Error(t, err)
	})

	t.Run("handles HTTP request error during introspection", func(t *testing.T) {
		provider, err := NewProvider("http://localhost:99999", []string{"aud1"}, WithAllowHttpScheme(true))
		require.NoError(t, err)

		provider.config = &Configuration{
			IntrospectionEndpoint: "http://localhost:99999/introspect",
		}

		_, err = provider.IntrospectToken(context.Background(), "test-token")
		require.Error(t, err)
		assert.ErrorIs(t, err, ErrCouldNotDoHTTPRequest)
	})

	t.Run("returns inactive token response", func(t *testing.T) {
		introspectionResult := Introspection{
			Active: false,
		}

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			err := json.NewEncoder(w).Encode(introspectionResult)
			assert.NoError(t, err)
		}))
		defer server.Close()

		provider, err := NewProvider(server.URL, []string{"aud1"}, WithAllowHttpScheme(true))
		require.NoError(t, err)

		provider.config = &Configuration{
			IntrospectionEndpoint: server.URL + "/introspect",
		}

		result, err := provider.IntrospectToken(context.Background(), "expired-token")
		require.NoError(t, err)
		assert.False(t, result.Active)
	})

	t.Run("returns error response fields", func(t *testing.T) {
		introspectionResult := Introspection{
			Active:           false,
			Error:            "invalid_token",
			ErrorDescription: "The token has expired",
		}

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			err := json.NewEncoder(w).Encode(introspectionResult)
			assert.NoError(t, err)
		}))
		defer server.Close()

		provider, err := NewProvider(server.URL, []string{"aud1"}, WithAllowHttpScheme(true))
		require.NoError(t, err)

		provider.config = &Configuration{
			IntrospectionEndpoint: server.URL + "/introspect",
		}

		result, err := provider.IntrospectToken(context.Background(), "expired-token")
		require.NoError(t, err)
		assert.False(t, result.Active)
		assert.Equal(t, "invalid_token", result.Error)
		assert.Equal(t, "The token has expired", result.ErrorDescription)
	})

	t.Run("handles context cancellation", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			<-r.Context().Done()
		}))
		defer server.Close()

		provider, err := NewProvider(server.URL, []string{"aud1"}, WithAllowHttpScheme(true))
		require.NoError(t, err)

		provider.config = &Configuration{
			IntrospectionEndpoint: server.URL + "/introspect",
		}

		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		_, err = provider.IntrospectToken(ctx, "test-token")
		require.Error(t, err)
		assert.ErrorIs(t, err, ErrCouldNotDoHTTPRequest)
	})

	t.Run("returns error when token introspection is disabled", func(t *testing.T) {
		provider, err := NewProvider("https://issuer.example.com", []string{"aud1"},
			WithDisableTokenIntrospection(true))
		require.NoError(t, err)

		provider.config = &Configuration{
			IntrospectionEndpoint: "https://issuer.example.com/introspect",
		}

		_, err = provider.IntrospectToken(context.Background(), "test-token")
		require.Error(t, err)
		assert.ErrorIs(t, err, ErrTokenIntrospectionDisabled)
	})
}
