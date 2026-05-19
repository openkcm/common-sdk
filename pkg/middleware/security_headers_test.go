package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/openkcm/common-sdk/pkg/middleware"
)

func TestSecurityHeadersMiddleware(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := middleware.SecurityHeadersMiddleware(next, nil)

	t.Run("sets all required security headers", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/", nil)

		handler.ServeHTTP(rec, req)

		assert.Equal(t, "nosniff", rec.Header().Get("X-Content-Type-Options"))
		assert.Equal(t, "DENY", rec.Header().Get("X-Frame-Options"))
		assert.Equal(t, "max-age=31536000; includeSubDomains", rec.Header().Get("Strict-Transport-Security"))
		assert.Equal(t, "strict-origin-when-cross-origin", rec.Header().Get("Referrer-Policy"))
		assert.Equal(t, "no-store", rec.Header().Get("Cache-Control"))
		assert.Equal(t, "same-origin", rec.Header().Get("Cross-Origin-Opener-Policy"))
		assert.Equal(t, "same-site", rec.Header().Get("Cross-Origin-Resource-Policy"))
	})

	t.Run("calls the next handler", func(t *testing.T) {
		var calledNextHandler bool

		nextWithFlag := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			calledNextHandler = true
		})

		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/", nil)

		middleware.SecurityHeadersMiddleware(nextWithFlag, nil).ServeHTTP(rec, req)

		assert.True(t, calledNextHandler, "next handler was not called")
	})

	t.Run("next handler can override Cache-Control", func(t *testing.T) {
		nextWithOverride := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Cache-Control", "public, max-age=3600")
		})

		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/", nil)

		middleware.SecurityHeadersMiddleware(nextWithOverride, nil).ServeHTTP(rec, req)

		assert.Equal(t, "public, max-age=3600", rec.Header().Get("Cache-Control"))
	})

	t.Run("custom headers override defaults", func(t *testing.T) {
		customHeaders := map[string]string{
			"X-Frame-Options":           "SAMEORIGIN",
			"Strict-Transport-Security": "max-age=63072000; includeSubDomains; preload",
			"Referrer-Policy":           "no-referrer",
		}

		handler := middleware.SecurityHeadersMiddleware(next, customHeaders)

		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/", nil)

		handler.ServeHTTP(rec, req)

		// Verify overridden headers
		assert.Equal(t, "SAMEORIGIN", rec.Header().Get("X-Frame-Options"))
		assert.Equal(t, "max-age=63072000; includeSubDomains; preload", rec.Header().Get("Strict-Transport-Security"))
		assert.Equal(t, "no-referrer", rec.Header().Get("Referrer-Policy"))

		// Verify non-overridden defaults remain
		assert.Equal(t, "nosniff", rec.Header().Get("X-Content-Type-Options"))
		assert.Equal(t, "no-store", rec.Header().Get("Cache-Control"))
	})

	t.Run("custom headers add additional headers", func(t *testing.T) {
		customHeaders := map[string]string{
			"X-Custom-Header":         "custom-value",
			"Permissions-Policy":      "geolocation=(), microphone=()",
			"Content-Security-Policy": "default-src 'self'",
		}

		handler := middleware.SecurityHeadersMiddleware(next, customHeaders)

		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/", nil)

		handler.ServeHTTP(rec, req)

		// Verify custom headers are added
		assert.Equal(t, "custom-value", rec.Header().Get("X-Custom-Header"))
		assert.Equal(t, "geolocation=(), microphone=()", rec.Header().Get("Permissions-Policy"))
		assert.Equal(t, "default-src 'self'", rec.Header().Get("Content-Security-Policy"))

		// Verify defaults remain
		assert.Equal(t, "nosniff", rec.Header().Get("X-Content-Type-Options"))
		assert.Equal(t, "DENY", rec.Header().Get("X-Frame-Options"))
	})

	t.Run("empty custom headers map does not affect defaults", func(t *testing.T) {
		customHeaders := map[string]string{}

		handler := middleware.SecurityHeadersMiddleware(next, customHeaders)

		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/", nil)

		handler.ServeHTTP(rec, req)

		// Verify all defaults are still applied
		assert.Equal(t, "nosniff", rec.Header().Get("X-Content-Type-Options"))
		assert.Equal(t, "DENY", rec.Header().Get("X-Frame-Options"))
		assert.Equal(t, "max-age=31536000; includeSubDomains", rec.Header().Get("Strict-Transport-Security"))
		assert.Equal(t, "strict-origin-when-cross-origin", rec.Header().Get("Referrer-Policy"))
		assert.Equal(t, "no-store", rec.Header().Get("Cache-Control"))
		assert.Equal(t, "same-origin", rec.Header().Get("Cross-Origin-Opener-Policy"))
		assert.Equal(t, "same-site", rec.Header().Get("Cross-Origin-Resource-Policy"))
	})

	t.Run("custom headers with empty values are set", func(t *testing.T) {
		customHeaders := map[string]string{
			"X-Frame-Options": "",
		}

		handler := middleware.SecurityHeadersMiddleware(next, customHeaders)

		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/", nil)

		handler.ServeHTTP(rec, req)

		// Verify empty value overrides default
		assert.Empty(t, rec.Header().Get("X-Frame-Options"))

		// Verify other defaults remain
		assert.Equal(t, "nosniff", rec.Header().Get("X-Content-Type-Options"))
	})

	t.Run("preserves original defaults map", func(t *testing.T) {
		// Store original default value
		originalXFrameOptions := middleware.SecurityHeaderDefaults["X-Frame-Options"]

		customHeaders := map[string]string{
			"X-Frame-Options": "SAMEORIGIN",
		}

		handler := middleware.SecurityHeadersMiddleware(next, customHeaders)

		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/", nil)

		handler.ServeHTTP(rec, req)

		// Verify the original defaults map is unchanged
		assert.Equal(t, originalXFrameOptions, middleware.SecurityHeaderDefaults["X-Frame-Options"])

		// Verify the response has the custom value
		assert.Equal(t, "SAMEORIGIN", rec.Header().Get("X-Frame-Options"))
	})
}
