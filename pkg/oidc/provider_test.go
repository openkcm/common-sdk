package oidc

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Provider Tests ---

func TestNewProvider(t *testing.T) {
	t.Run("creates provider with valid issuer and audiences", func(t *testing.T) {
		provider, err := NewProvider("https://issuer.example.com", []string{"aud1", "aud2"})
		require.NoError(t, err)
		assert.NotNil(t, provider)
		assert.Equal(t, "https://issuer.example.com", provider.Issuer())
		assert.Equal(t, "https://issuer.example.com", provider.IssuerURI())
		assert.Equal(t, []string{"aud1", "aud2"}, provider.Audiences())
	})

	t.Run("creates provider with nil option (skipped)", func(t *testing.T) {
		provider, err := NewProvider("https://issuer.example.com", []string{"aud1"}, nil)
		require.NoError(t, err)
		assert.NotNil(t, provider)
	})
}

func TestWithCustomIssuerURI(t *testing.T) {
	t.Run("sets custom issuer URI", func(t *testing.T) {
		provider, err := NewProvider("https://issuer.example.com", []string{"aud1"},
			WithCustomIssuerURI("https://custom.issuer.com"))
		require.NoError(t, err)
		assert.Equal(t, "https://issuer.example.com", provider.Issuer())
		assert.Equal(t, "https://custom.issuer.com", provider.IssuerURI())
	})

	t.Run("does nothing with empty issuer URI", func(t *testing.T) {
		provider, err := NewProvider("https://issuer.example.com", []string{"aud1"},
			WithCustomIssuerURI(""))
		require.NoError(t, err)
		// issuerURI defaults to issuer when empty string is passed
		assert.Equal(t, "https://issuer.example.com", provider.IssuerURI())
	})
}

func TestWithPublicHTTPClient(t *testing.T) {
	t.Run("sets public HTTP client", func(t *testing.T) {
		client := &http.Client{}
		provider, err := NewProvider("https://issuer.example.com", []string{"aud1"},
			WithPublicHTTPClient(client))
		require.NoError(t, err)
		assert.NotNil(t, provider)
	})
}

func TestWithSecureHTTPClient(t *testing.T) {
	t.Run("sets secure HTTP client", func(t *testing.T) {
		client := &http.Client{}
		provider, err := NewProvider("https://issuer.example.com", []string{"aud1"},
			WithSecureHTTPClient(client))
		require.NoError(t, err)
		assert.NotNil(t, provider)
	})
}

func TestWithIntrospectQueryParameters(t *testing.T) {
	t.Run("sets introspect query parameters", func(t *testing.T) {
		params := map[string]string{"key": "value"}
		provider, err := NewProvider("https://issuer.example.com", []string{"aud1"},
			WithIntrospectQueryParameters(params))
		require.NoError(t, err)
		assert.NotNil(t, provider)
	})
}

func TestWithCustomJWKSURI(t *testing.T) {
	t.Run("sets custom JWKS URI", func(t *testing.T) {
		provider, err := NewProvider("https://issuer.example.com", []string{"aud1"},
			WithCustomJWKSURI("https://custom.jwks.example.com/keys"))
		require.NoError(t, err)
		assert.Equal(t, "https://custom.jwks.example.com/keys", provider.CustomJWKSURI())
	})

	t.Run("does nothing with empty JWKS URI", func(t *testing.T) {
		provider, err := NewProvider("https://issuer.example.com", []string{"aud1"},
			WithCustomJWKSURI(""))
		require.NoError(t, err)
		assert.Empty(t, provider.CustomJWKSURI())
	})
}

func TestCustomJWKSURI(t *testing.T) {
	t.Run("returns empty string when not set", func(t *testing.T) {
		provider, err := NewProvider("https://issuer.example.com", []string{"aud1"})
		require.NoError(t, err)
		assert.Empty(t, provider.CustomJWKSURI())
	})

	t.Run("returns custom JWKS URI when set", func(t *testing.T) {
		provider, err := NewProvider("https://issuer.example.com", []string{"aud1"},
			WithCustomJWKSURI("https://custom.jwks.example.com/keys"))
		require.NoError(t, err)
		assert.Equal(t, "https://custom.jwks.example.com/keys", provider.CustomJWKSURI())
	})
}

// --- UniqueID Tests ---

func TestUniqueID(t *testing.T) {
	t.Run("returns issuerURI when set via custom option", func(t *testing.T) {
		provider, err := NewProvider("https://issuer.example.com", []string{"aud1"},
			WithCustomIssuerURI("https://custom.issuer.com"))
		require.NoError(t, err)
		assert.Equal(t, "https://custom.issuer.com", provider.UniqueID())
	})

	t.Run("returns issuer as issuerURI when no custom issuerURI set", func(t *testing.T) {
		provider, err := NewProvider("https://issuer.example.com", []string{"aud1"})
		require.NoError(t, err)
		assert.Equal(t, "https://issuer.example.com", provider.UniqueID())
	})

	t.Run("returns customJWKSURI when issuerURI is empty", func(t *testing.T) {
		// Create provider directly to test edge case where issuerURI is empty
		provider := &Provider{
			issuer:        "issuer-value",
			customJWKSURI: "https://custom.jwks.example.com/keys",
		}
		assert.Equal(t, "https://custom.jwks.example.com/keys", provider.UniqueID())
	})

	t.Run("returns issuer when both issuerURI and customJWKSURI are empty", func(t *testing.T) {
		// Create provider directly to test edge case
		provider := &Provider{
			issuer: "issuer-value",
		}
		assert.Equal(t, "issuer-value", provider.UniqueID())
	})

	t.Run("prioritizes issuerURI over customJWKSURI", func(t *testing.T) {
		provider, err := NewProvider("https://issuer.example.com", []string{"aud1"},
			WithCustomIssuerURI("https://custom.issuer.com"),
			WithCustomJWKSURI("https://custom.jwks.example.com/keys"))
		require.NoError(t, err)
		assert.Equal(t, "https://custom.issuer.com", provider.UniqueID())
	})
}

// --- DefaultIssuerClaims Test ---

func TestDefaultIssuerClaims(t *testing.T) {
	assert.Equal(t, []string{"iss"}, DefaultIssuerClaims)
}
