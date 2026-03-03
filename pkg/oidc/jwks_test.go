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

// Sample JWKS response with RSA public key for testing
const testJWKSResponse = `{
	"keys": [
		{
			"kty": "RSA",
			"use": "sig",
			"kid": "key-1",
			"n": "0vx7agoebGcQSuuPiLJXZptN9nndrQmbXEps2aiAFbWhM78LhWx4cbbfAAtVT86zwu1RK7aPFFxuhDR1L6tSoc_BJECPebWKRXjBZCiFV4n3oknjhMstn64tZ_2W-5JsGY4Hc5n9yBXArwl93lqt7_RN5w6Cf0h4QyQ5v-65YGjQR0_FDW2QvzqY368QQMicAtaSqzs8KJZgnYb9c7d0zgdAZHzu6qMQvRL5hajrn1n91CbOpbISD08qNLyrdkt-bFTWhAI4vMQFh6WeZu0fM4lFd2NcRwr3XPksINHaQ-G_xBniIqbw0Ls1jF44-csFCur-kEgU8awapJzKnqDKgw",
			"e": "AQAB"
		},
		{
			"kty": "RSA",
			"use": "enc",
			"kid": "key-2",
			"n": "0vx7agoebGcQSuuPiLJXZptN9nndrQmbXEps2aiAFbWhM78LhWx4cbbfAAtVT86zwu1RK7aPFFxuhDR1L6tSoc_BJECPebWKRXjBZCiFV4n3oknjhMstn64tZ_2W-5JsGY4Hc5n9yBXArwl93lqt7_RN5w6Cf0h4QyQ5v-65YGjQR0_FDW2QvzqY368QQMicAtaSqzs8KJZgnYb9c7d0zgdAZHzu6qMQvRL5hajrn1n91CbOpbISD08qNLyrdkt-bFTWhAI4vMQFh6WeZu0fM4lFd2NcRwr3XPksINHaQ-G_xBniIqbw0Ls1jF44-csFCur-kEgU8awapJzKnqDKgw",
			"e": "AQAB"
		}
	]
}`

const testJWKSResponseSingleKey = `{
	"keys": [
		{
			"kty": "RSA",
			"use": "sig",
			"kid": "key-1",
			"n": "0vx7agoebGcQSuuPiLJXZptN9nndrQmbXEps2aiAFbWhM78LhWx4cbbfAAtVT86zwu1RK7aPFFxuhDR1L6tSoc_BJECPebWKRXjBZCiFV4n3oknjhMstn64tZ_2W-5JsGY4Hc5n9yBXArwl93lqt7_RN5w6Cf0h4QyQ5v-65YGjQR0_FDW2QvzqY368QQMicAtaSqzs8KJZgnYb9c7d0zgdAZHzu6qMQvRL5hajrn1n91CbOpbISD08qNLyrdkt-bFTWhAI4vMQFh6WeZu0fM4lFd2NcRwr3XPksINHaQ-G_xBniIqbw0Ls1jF44-csFCur-kEgU8awapJzKnqDKgw",
			"e": "AQAB"
		}
	]
}`

const testJWKSResponseOtherKey = `{
	"keys": [
		{
			"kty": "RSA",
			"use": "sig",
			"kid": "other-key",
			"n": "0vx7agoebGcQSuuPiLJXZptN9nndrQmbXEps2aiAFbWhM78LhWx4cbbfAAtVT86zwu1RK7aPFFxuhDR1L6tSoc_BJECPebWKRXjBZCiFV4n3oknjhMstn64tZ_2W-5JsGY4Hc5n9yBXArwl93lqt7_RN5w6Cf0h4QyQ5v-65YGjQR0_FDW2QvzqY368QQMicAtaSqzs8KJZgnYb9c7d0zgdAZHzu6qMQvRL5hajrn1n91CbOpbISD08qNLyrdkt-bFTWhAI4vMQFh6WeZu0fM4lFd2NcRwr3XPksINHaQ-G_xBniIqbw0Ls1jF44-csFCur-kEgU8awapJzKnqDKgw",
			"e": "AQAB"
		}
	]
}`

const testJWKSResponseEncKey = `{
	"keys": [
		{
			"kty": "RSA",
			"use": "enc",
			"kid": "key-1",
			"n": "0vx7agoebGcQSuuPiLJXZptN9nndrQmbXEps2aiAFbWhM78LhWx4cbbfAAtVT86zwu1RK7aPFFxuhDR1L6tSoc_BJECPebWKRXjBZCiFV4n3oknjhMstn64tZ_2W-5JsGY4Hc5n9yBXArwl93lqt7_RN5w6Cf0h4QyQ5v-65YGjQR0_FDW2QvzqY368QQMicAtaSqzs8KJZgnYb9c7d0zgdAZHzu6qMQvRL5hajrn1n91CbOpbISD08qNLyrdkt-bFTWhAI4vMQFh6WeZu0fM4lFd2NcRwr3XPksINHaQ-G_xBniIqbw0Ls1jF44-csFCur-kEgU8awapJzKnqDKgw",
			"e": "AQAB"
		}
	]
}`

func TestGetSigningKey(t *testing.T) {
	t.Run("successfully gets signing key", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/jwks" {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(testJWKSResponse))

				return
			}
		}))
		defer server.Close()

		provider, err := NewProvider(server.URL, []string{"aud1"}, WithAllowHttpScheme(true))
		require.NoError(t, err)

		// Set config with correct JWKS URI
		provider.config = &Configuration{
			Issuer:  server.URL,
			JwksURI: server.URL + "/jwks",
		}

		key, err := provider.GetSigningKey(context.Background(), "key-1")
		require.NoError(t, err)
		assert.NotNil(t, key)
		assert.Equal(t, "key-1", key.KeyID)
		assert.Equal(t, "sig", key.Use)
	})

	t.Run("fetches config when not cached", func(t *testing.T) {
		configCalled := false
		jwksCalled := false

		var serverURL string

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == wellKnownOpenIDConfigPath {
				configCalled = true

				w.Header().Set("Content-Type", "application/json")

				config := Configuration{
					Issuer:  serverURL,
					JwksURI: serverURL + "/jwks",
				}
				err := json.NewEncoder(w).Encode(config)
				assert.NoError(t, err)

				return
			}

			if r.URL.Path == "/jwks" {
				jwksCalled = true

				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(testJWKSResponseSingleKey))

				return
			}
		}))
		defer server.Close()

		serverURL = server.URL

		provider, err := NewProvider(server.URL, []string{"aud1"}, WithAllowHttpScheme(true))
		require.NoError(t, err)

		key, err := provider.GetSigningKey(context.Background(), "key-1")
		require.NoError(t, err)
		assert.NotNil(t, key)
		assert.True(t, configCalled, "config should be fetched")
		assert.True(t, jwksCalled, "jwks should be fetched")
	})

	t.Run("returns error when key not found", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(testJWKSResponseOtherKey))
		}))
		defer server.Close()

		provider, err := NewProvider(server.URL, []string{"aud1"}, WithAllowHttpScheme(true))
		require.NoError(t, err)

		provider.config = &Configuration{
			JwksURI: server.URL + "/jwks",
		}

		key, err := provider.GetSigningKey(context.Background(), "non-existent-key")
		require.Error(t, err)
		assert.Nil(t, key)
		assert.Contains(t, err.Error(), "could not find key for key ID: non-existent-key")
	})

	t.Run("skips keys with different use", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(testJWKSResponseEncKey))
		}))
		defer server.Close()

		provider, err := NewProvider(server.URL, []string{"aud1"}, WithAllowHttpScheme(true))
		require.NoError(t, err)

		provider.config = &Configuration{
			JwksURI: server.URL + "/jwks",
		}

		key, err := provider.GetSigningKey(context.Background(), "key-1")
		require.Error(t, err)
		assert.Nil(t, key)
	})

	t.Run("handles config fetch error", func(t *testing.T) {
		provider, err := NewProvider("http://localhost:99999", []string{"aud1"}, WithAllowHttpScheme(true))
		require.NoError(t, err)

		key, err := provider.GetSigningKey(context.Background(), "key-1")
		require.Error(t, err)
		assert.Nil(t, key)
		assert.Contains(t, err.Error(), "could not get well known OpenID configuration")
	})

	t.Run("handles jwks fetch error", func(t *testing.T) {
		provider, err := NewProvider("http://localhost:99999", []string{"aud1"}, WithAllowHttpScheme(true))
		require.NoError(t, err)

		provider.config = &Configuration{
			JwksURI: "http://localhost:99999/jwks",
		}

		key, err := provider.GetSigningKey(context.Background(), "key-1")
		require.Error(t, err)
		assert.Nil(t, key)
	})

	t.Run("handles invalid jwks JSON", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte("invalid json"))
		}))
		defer server.Close()

		provider, err := NewProvider(server.URL, []string{"aud1"}, WithAllowHttpScheme(true))
		require.NoError(t, err)

		provider.config = &Configuration{
			JwksURI: server.URL + "/jwks",
		}

		key, err := provider.GetSigningKey(context.Background(), "key-1")
		require.Error(t, err)
		assert.Nil(t, key)
		assert.Contains(t, err.Error(), "could not decode provider response")
	})

	t.Run("handles non-200 response from jwks endpoint", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte("jwks error"))
		}))
		defer server.Close()

		provider, err := NewProvider(server.URL, []string{"aud1"}, WithAllowHttpScheme(true))
		require.NoError(t, err)

		provider.config = &Configuration{
			JwksURI: server.URL + "/jwks",
		}

		key, err := provider.GetSigningKey(context.Background(), "key-1")
		require.Error(t, err)
		assert.Nil(t, key)

		var non200Err ProviderRespondedNon200Error
		assert.ErrorAs(t, err, &non200Err)
		assert.Equal(t, http.StatusInternalServerError, non200Err.Code)
	})

	t.Run("uses custom JWKS URI when configured", func(t *testing.T) {
		jwksServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(testJWKSResponseSingleKey))
		}))
		defer jwksServer.Close()

		// Create provider with custom JWKS URI - no need to fetch config
		provider, err := NewProvider("https://issuer.example.com", []string{"aud1"},
			WithAllowHttpScheme(true),
			WithCustomJWKSURI(jwksServer.URL+"/jwks"))
		require.NoError(t, err)

		// Provider should use custom JWKS URI, not try to fetch config
		key, err := provider.GetSigningKey(context.Background(), "key-1")
		require.NoError(t, err)
		assert.NotNil(t, key)
		assert.Equal(t, "key-1", key.KeyID)
	})

	t.Run("returns error for key with matching ID but wrong use", func(t *testing.T) {
		// Test when key-1 exists but has use="enc" instead of "sig"
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(testJWKSResponseEncKey))
		}))
		defer server.Close()

		provider, err := NewProvider(server.URL, []string{"aud1"}, WithAllowHttpScheme(true))
		require.NoError(t, err)

		provider.config = &Configuration{
			JwksURI: server.URL + "/jwks",
		}

		key, err := provider.GetSigningKey(context.Background(), "key-1")
		require.Error(t, err)
		assert.Nil(t, key)

		var keyErr CouldNotFindKeyForKeyIDError
		assert.ErrorAs(t, err, &keyErr)
		assert.Equal(t, "key-1", keyErr.KeyID)
	})

	t.Run("handles context cancellation", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			<-r.Context().Done()
		}))
		defer server.Close()

		provider, err := NewProvider(server.URL, []string{"aud1"}, WithAllowHttpScheme(true))
		require.NoError(t, err)

		provider.config = &Configuration{
			JwksURI: server.URL + "/jwks",
		}

		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		key, err := provider.GetSigningKey(ctx, "key-1")
		require.Error(t, err)
		assert.Nil(t, key)
	})

	t.Run("finds correct key among multiple keys", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(testJWKSResponse)) // Contains key-1 (sig) and key-2 (enc)
		}))
		defer server.Close()

		provider, err := NewProvider(server.URL, []string{"aud1"}, WithAllowHttpScheme(true))
		require.NoError(t, err)

		provider.config = &Configuration{
			JwksURI: server.URL + "/jwks",
		}

		// key-1 has use="sig" so it should be found
		key, err := provider.GetSigningKey(context.Background(), "key-1")
		require.NoError(t, err)
		assert.NotNil(t, key)
		assert.Equal(t, "key-1", key.KeyID)
		assert.Equal(t, "sig", key.Use)
	})

	t.Run("skips keys with matching id but wrong use when finding correct key", func(t *testing.T) {
		// key-2 exists but has use="enc", so it should not be returned
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(testJWKSResponse))
		}))
		defer server.Close()

		provider, err := NewProvider(server.URL, []string{"aud1"}, WithAllowHttpScheme(true))
		require.NoError(t, err)

		provider.config = &Configuration{
			JwksURI: server.URL + "/jwks",
		}

		// key-2 exists but has use="enc" so it should NOT be found
		key, err := provider.GetSigningKey(context.Background(), "key-2")
		require.Error(t, err)
		assert.Nil(t, key)

		var keyErr CouldNotFindKeyForKeyIDError
		assert.ErrorAs(t, err, &keyErr)
		assert.Equal(t, "key-2", keyErr.KeyID)
	})

	t.Run("returns empty JWKS error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"keys": []}`))
		}))
		defer server.Close()

		provider, err := NewProvider(server.URL, []string{"aud1"}, WithAllowHttpScheme(true))
		require.NoError(t, err)

		provider.config = &Configuration{
			JwksURI: server.URL + "/jwks",
		}

		key, err := provider.GetSigningKey(context.Background(), "key-1")
		require.Error(t, err)
		assert.Nil(t, key)

		var keyErr CouldNotFindKeyForKeyIDError
		assert.ErrorAs(t, err, &keyErr)
	})
}
