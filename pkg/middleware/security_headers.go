package middleware

import (
	"maps"
	"net/http"
)

var SecurityHeaderDefaults = map[string]string{
	"X-Content-Type-Options":       "nosniff",
	"X-Frame-Options":              "DENY",
	"Strict-Transport-Security":    "max-age=31536000; includeSubDomains",
	"Referrer-Policy":              "strict-origin-when-cross-origin",
	"Cache-Control":                "no-store",
	"Cross-Origin-Opener-Policy":   "same-origin",
	"Cross-Origin-Resource-Policy": "same-site",
}

// SecurityHeadersMiddleware returns an HTTP middleware that adds baseline security headers to every response.
// customHeaders overrides or extends the defaults. Content-Security-Policy is managed at the
// infrastructure level and is omitted from the defaults.
func SecurityHeadersMiddleware(customHeaders map[string]string) func(http.Handler) http.Handler {
	headers := make(map[string]string)
	maps.Copy(headers, SecurityHeaderDefaults)
	maps.Copy(headers, customHeaders)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			for key, value := range headers {
				w.Header().Set(key, value)
			}

			next.ServeHTTP(w, r)
		})
	}
}
