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

func TestConfiguration(t *testing.T) {
	ctx := t.Context()

	t.Run("fetches and returns configuration", func(t *testing.T) {
		config := Configuration{
			Issuer:                "https://issuer.example.com",
			AuthorizationEndpoint: "https://issuer.example.com/authorize",
			TokenEndpoint:         "https://issuer.example.com/token",
			JwksURI:               "https://issuer.example.com/jwks",
			IntrospectionEndpoint: "https://issuer.example.com/introspect",
		}

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, wellKnownOpenIDConfigPath, r.URL.Path)
			w.Header().Set("Content-Type", "application/json")
			err := json.NewEncoder(w).Encode(config)
			assert.NoError(t, err)
		}))
		defer server.Close()

		provider, err := NewProvider(server.URL, []string{"aud1"}, WithAllowHttpScheme(true))
		require.NoError(t, err)

		result, err := provider.GetConfiguration(ctx)
		require.NoError(t, err)
		assert.Equal(t, config.Issuer, result.Issuer)
		assert.Equal(t, config.AuthorizationEndpoint, result.AuthorizationEndpoint)
		assert.Equal(t, config.TokenEndpoint, result.TokenEndpoint)
		assert.Equal(t, config.JwksURI, result.JwksURI)
		assert.Equal(t, config.IntrospectionEndpoint, result.IntrospectionEndpoint)
	})

	t.Run("returns cached configuration", func(t *testing.T) {
		callCount := 0

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			callCount++

			w.Header().Set("Content-Type", "application/json")
			err := json.NewEncoder(w).Encode(Configuration{Issuer: "test"})
			assert.NoError(t, err)
		}))
		defer server.Close()

		provider, err := NewProvider(server.URL, []string{"aud1"}, WithAllowHttpScheme(true))
		require.NoError(t, err)

		_, err = provider.GetConfiguration(ctx)
		require.NoError(t, err)
		_, err = provider.GetConfiguration(ctx)
		require.NoError(t, err)

		assert.Equal(t, 1, callCount, "should only call the server once")
	})

	t.Run("handles non-200 response", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte("server error"))
		}))
		defer server.Close()

		provider, err := NewProvider(server.URL, []string{"aud1"}, WithAllowHttpScheme(true))
		require.NoError(t, err)

		_, err = provider.GetConfiguration(ctx)
		require.Error(t, err)

		var non200Err ProviderRespondedNon200Error
		assert.ErrorAs(t, err, &non200Err)
		assert.Equal(t, http.StatusInternalServerError, non200Err.Code)
		assert.Equal(t, "server error", non200Err.Body)
	})

	t.Run("handles invalid JSON response", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte("not valid json"))
		}))
		defer server.Close()

		provider, err := NewProvider(server.URL, []string{"aud1"}, WithAllowHttpScheme(true))
		require.NoError(t, err)

		_, err = provider.GetConfiguration(ctx)
		require.Error(t, err)

		var decodeErr CouldNotUnmarshallResponseError
		assert.ErrorAs(t, err, &decodeErr)
		assert.Equal(t, "not valid json", decodeErr.Body)
	})

	t.Run("handles HTTP request error", func(t *testing.T) {
		provider, err := NewProvider("http://localhost:99999", []string{"aud1"}, WithAllowHttpScheme(true))
		require.NoError(t, err)

		_, err = provider.GetConfiguration(ctx)
		require.Error(t, err)
		assert.ErrorIs(t, err, ErrCouldNotDoHTTPRequest)
	})

	t.Run("handles context cancellation", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Don't respond - wait for context to be cancelled
			<-r.Context().Done()
		}))
		defer server.Close()

		provider, err := NewProvider(server.URL, []string{"aud1"}, WithAllowHttpScheme(true))
		require.NoError(t, err)

		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		_, err = provider.GetConfiguration(ctx)
		require.Error(t, err)
		assert.ErrorIs(t, err, ErrCouldNotDoHTTPRequest)
	})
}
