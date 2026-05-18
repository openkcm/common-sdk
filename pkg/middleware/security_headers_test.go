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

	handler := middleware.SecurityHeadersMiddleware(next)

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

		middleware.SecurityHeadersMiddleware(nextWithFlag).ServeHTTP(rec, req)

		assert.True(t, calledNextHandler, "next handler was not called")
	})

	t.Run("next handler can override Cache-Control", func(t *testing.T) {
		nextWithOverride := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Cache-Control", "public, max-age=3600")
		})

		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/", nil)

		middleware.SecurityHeadersMiddleware(nextWithOverride).ServeHTTP(rec, req)

		assert.Equal(t, "public, max-age=3600", rec.Header().Get("Cache-Control"))
	})
}
