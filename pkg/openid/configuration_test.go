package openid

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetConfig(t *testing.T) {
	t.Parallel()

	t.Run("success with full configuration", func(t *testing.T) {
		t.Parallel()

		expectedConfig := Configuration{
			Issuer:                            "https://issuer.example.com",
			AuthorizationEndpoint:             "https://issuer.example.com/authorize",
			TokenEndpoint:                     "https://issuer.example.com/token",
			UserinfoEndpoint:                  "https://issuer.example.com/userinfo",
			JwksURI:                           "https://issuer.example.com/.well-known/jwks.json",
			ResponseTypesSupported:            []string{"code", "token", "id_token"},
			GrantTypesSupported:               []string{"authorization_code", "refresh_token"},
			SubjectTypesSupported:             []string{"public"},
			IDTokenSigningAlgValuesSupported:  []string{"RS256"},
			ScopesSupported:                   []string{"openid", "profile", "email"},
			TokenEndpointAuthMethodsSupported: []string{"client_secret_basic", "client_secret_post"},
			ClaimsSupported:                   []string{"sub", "iss", "aud", "exp", "iat"},
			IntrospectionEndpoint:             "https://issuer.example.com/introspect",
			EndSessionEndpoint:                "https://issuer.example.com/logout",
		}

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodGet, r.Method)
			assert.Equal(t, "/.well-known/openid-configuration", r.URL.Path)

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(expectedConfig)
		}))
		defer server.Close()

		cfg, err := GetConfig(context.Background(), server.URL)

		require.NoError(t, err)
		assert.Equal(t, expectedConfig.Issuer, cfg.Issuer)
		assert.Equal(t, expectedConfig.AuthorizationEndpoint, cfg.AuthorizationEndpoint)
		assert.Equal(t, expectedConfig.TokenEndpoint, cfg.TokenEndpoint)
		assert.Equal(t, expectedConfig.UserinfoEndpoint, cfg.UserinfoEndpoint)
		assert.Equal(t, expectedConfig.JwksURI, cfg.JwksURI)
		assert.Equal(t, expectedConfig.ResponseTypesSupported, cfg.ResponseTypesSupported)
		assert.Equal(t, expectedConfig.GrantTypesSupported, cfg.GrantTypesSupported)
		assert.Equal(t, expectedConfig.SubjectTypesSupported, cfg.SubjectTypesSupported)
		assert.Equal(t, expectedConfig.IDTokenSigningAlgValuesSupported, cfg.IDTokenSigningAlgValuesSupported)
		assert.Equal(t, expectedConfig.ScopesSupported, cfg.ScopesSupported)
		assert.Equal(t, expectedConfig.TokenEndpointAuthMethodsSupported, cfg.TokenEndpointAuthMethodsSupported)
		assert.Equal(t, expectedConfig.ClaimsSupported, cfg.ClaimsSupported)
		assert.Equal(t, expectedConfig.IntrospectionEndpoint, cfg.IntrospectionEndpoint)
		assert.Equal(t, expectedConfig.EndSessionEndpoint, cfg.EndSessionEndpoint)
	})

	t.Run("success with minimal configuration", func(t *testing.T) {
		t.Parallel()

		expectedConfig := Configuration{
			Issuer: "https://issuer.example.com",
		}

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(expectedConfig)
		}))
		defer server.Close()

		cfg, err := GetConfig(context.Background(), server.URL)

		require.NoError(t, err)
		assert.Equal(t, expectedConfig.Issuer, cfg.Issuer)
		assert.Empty(t, cfg.AuthorizationEndpoint)
		assert.Empty(t, cfg.TokenEndpoint)
	})

	t.Run("success with issuer URL having trailing slash", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/.well-known/openid-configuration", r.URL.Path)

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(Configuration{Issuer: "https://issuer.example.com"})
		}))
		defer server.Close()

		cfg, err := GetConfig(context.Background(), server.URL+"/")

		require.NoError(t, err)
		assert.Equal(t, "https://issuer.example.com", cfg.Issuer)
	})

	t.Run("error when URL cannot be built", func(t *testing.T) {
		t.Parallel()

		// url.JoinPath returns error for invalid URLs with certain control characters
		cfg, err := GetConfig(context.Background(), "://invalid\x00url")

		require.Error(t, err)
		require.ErrorIs(t, err, ErrCouldNotBuildURL)
		assert.Equal(t, Configuration{}, cfg)
	})

	t.Run("error when HTTP request fails", func(t *testing.T) {
		t.Parallel()

		cfg, err := GetConfig(context.Background(), "http://localhost:1") // non-existent server

		require.Error(t, err)
		require.ErrorIs(t, err, ErrCouldNotDoHTTPRequest)
		assert.Equal(t, Configuration{}, cfg)
	})

	t.Run("error when provider responds with 404 status", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte("not found"))
		}))
		defer server.Close()

		cfg, err := GetConfig(context.Background(), server.URL)

		require.Error(t, err)

		var errNon200 ProviderRespondedNon200Error
		require.ErrorAs(t, err, &errNon200)
		assert.Equal(t, http.StatusNotFound, errNon200.Code)
		assert.Equal(t, "not found", errNon200.Body)
		assert.Equal(t, Configuration{}, cfg)
	})

	t.Run("error when provider responds with 500 status", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"error": "internal server error"}`))
		}))
		defer server.Close()

		cfg, err := GetConfig(context.Background(), server.URL)

		require.Error(t, err)

		var errNon200 ProviderRespondedNon200Error
		require.ErrorAs(t, err, &errNon200)
		assert.Equal(t, http.StatusInternalServerError, errNon200.Code)
		assert.Equal(t, Configuration{}, cfg)
	})

	t.Run("error when provider responds with 401 status", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte("unauthorized"))
		}))
		defer server.Close()

		cfg, err := GetConfig(context.Background(), server.URL)

		require.Error(t, err)

		var errNon200 ProviderRespondedNon200Error
		require.ErrorAs(t, err, &errNon200)
		assert.Equal(t, http.StatusUnauthorized, errNon200.Code)
		assert.Equal(t, Configuration{}, cfg)
	})

	t.Run("error when response body is invalid JSON", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("not valid json"))
		}))
		defer server.Close()

		cfg, err := GetConfig(context.Background(), server.URL)

		require.Error(t, err)

		var errDecode CouldNotDecodeResponseError
		require.ErrorAs(t, err, &errDecode)
		assert.Equal(t, "not valid json", errDecode.Body)
		assert.Equal(t, Configuration{}, cfg)
	})

	t.Run("error when response body has invalid JSON structure", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			// scopes_supported should be array, not string
			_, _ = w.Write([]byte(`{"issuer": "https://issuer.example.com", "scopes_supported": "not-an-array"}`))
		}))
		defer server.Close()

		cfg, err := GetConfig(context.Background(), server.URL)

		require.Error(t, err)

		var errDecode CouldNotDecodeResponseError
		require.ErrorAs(t, err, &errDecode)
		assert.Equal(t, Configuration{}, cfg)
	})

	t.Run("error when response body is empty", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			// Empty body
		}))
		defer server.Close()

		cfg, err := GetConfig(context.Background(), server.URL)

		require.Error(t, err)

		var errDecode CouldNotDecodeResponseError
		require.ErrorAs(t, err, &errDecode)
		assert.Equal(t, Configuration{}, cfg)
	})

	t.Run("context cancellation", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Simulate slow response
			<-r.Context().Done()
		}))
		defer server.Close()

		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		cfg, err := GetConfig(ctx, server.URL)

		require.Error(t, err)
		require.ErrorIs(t, err, ErrCouldNotDoHTTPRequest)
		assert.Equal(t, Configuration{}, cfg)
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

		cfg, err := GetConfig(context.Background(), server.URL)

		require.Error(t, err)
		require.ErrorIs(t, err, ErrCouldNotReadResponseBody)
		assert.Equal(t, Configuration{}, cfg)
	})

	t.Run("issuer URL with path", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Verify the path is correctly constructed
			assert.Equal(t, "/some/path/.well-known/openid-configuration", r.URL.Path)

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(Configuration{Issuer: "https://issuer.example.com/some/path"})
		}))
		defer server.Close()

		cfg, err := GetConfig(context.Background(), server.URL+"/some/path")

		require.NoError(t, err)
		assert.Equal(t, "https://issuer.example.com/some/path", cfg.Issuer)
	})
}

func TestConfiguration(t *testing.T) {
	t.Parallel()

	t.Run("unmarshal complete configuration", func(t *testing.T) {
		t.Parallel()

		jsonData := `{
			"issuer": "https://issuer.example.com",
			"authorization_endpoint": "https://issuer.example.com/authorize",
			"token_endpoint": "https://issuer.example.com/token",
			"userinfo_endpoint": "https://issuer.example.com/userinfo",
			"jwks_uri": "https://issuer.example.com/.well-known/jwks.json",
			"response_types_supported": ["code", "token"],
			"grant_types_supported": ["authorization_code"],
			"subject_types_supported": ["public"],
			"id_token_signing_alg_values_supported": ["RS256"],
			"scopes_supported": ["openid", "profile"],
			"token_endpoint_auth_methods_supported": ["client_secret_basic"],
			"claims_supported": ["sub", "iss"],
			"introspection_endpoint": "https://issuer.example.com/introspect",
			"end_session_endpoint": "https://issuer.example.com/logout"
		}`

		var cfg Configuration

		err := json.Unmarshal([]byte(jsonData), &cfg)

		require.NoError(t, err)
		assert.Equal(t, "https://issuer.example.com", cfg.Issuer)
		assert.Equal(t, "https://issuer.example.com/authorize", cfg.AuthorizationEndpoint)
		assert.Equal(t, "https://issuer.example.com/token", cfg.TokenEndpoint)
		assert.Equal(t, "https://issuer.example.com/userinfo", cfg.UserinfoEndpoint)
		assert.Equal(t, "https://issuer.example.com/.well-known/jwks.json", cfg.JwksURI)
		assert.Equal(t, []string{"code", "token"}, cfg.ResponseTypesSupported)
		assert.Equal(t, []string{"authorization_code"}, cfg.GrantTypesSupported)
		assert.Equal(t, []string{"public"}, cfg.SubjectTypesSupported)
		assert.Equal(t, []string{"RS256"}, cfg.IDTokenSigningAlgValuesSupported)
		assert.Equal(t, []string{"openid", "profile"}, cfg.ScopesSupported)
		assert.Equal(t, []string{"client_secret_basic"}, cfg.TokenEndpointAuthMethodsSupported)
		assert.Equal(t, []string{"sub", "iss"}, cfg.ClaimsSupported)
		assert.Equal(t, "https://issuer.example.com/introspect", cfg.IntrospectionEndpoint)
		assert.Equal(t, "https://issuer.example.com/logout", cfg.EndSessionEndpoint)
	})

	t.Run("unmarshal minimal configuration", func(t *testing.T) {
		t.Parallel()

		jsonData := `{"issuer": "https://issuer.example.com"}`

		var cfg Configuration

		err := json.Unmarshal([]byte(jsonData), &cfg)

		require.NoError(t, err)
		assert.Equal(t, "https://issuer.example.com", cfg.Issuer)
		assert.Empty(t, cfg.AuthorizationEndpoint)
		assert.Empty(t, cfg.TokenEndpoint)
		assert.Nil(t, cfg.ResponseTypesSupported)
	})

	t.Run("HTTPClient is not serialized", func(t *testing.T) {
		t.Parallel()

		cfg := Configuration{
			Issuer:     "https://issuer.example.com",
			HTTPClient: &http.Client{},
		}

		data, err := json.Marshal(cfg)
		require.NoError(t, err)

		// HTTPClient should not be in the JSON
		assert.NotContains(t, string(data), "HTTPClient")
		assert.NotContains(t, string(data), "http_client")

		// Verify it can be unmarshaled back (without HTTPClient)
		var cfg2 Configuration

		err = json.Unmarshal(data, &cfg2)
		require.NoError(t, err)
		assert.Equal(t, cfg.Issuer, cfg2.Issuer)
		assert.Nil(t, cfg2.HTTPClient)
	})
}
