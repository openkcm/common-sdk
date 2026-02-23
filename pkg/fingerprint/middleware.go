package fingerprint

import (
	"log/slog"
	"net/http"
)

func FingerprintCtxMiddleware(next http.Handler) http.Handler {
	builder := NewBuilder()

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fp, err := builder.FromHTTPRequest(r)
		if err != nil {
			slog.Error("failed to generate fingerprint", "error", err)
			w.WriteHeader(http.StatusInternalServerError)

			return
		}

		ctxWithFP := WithFingerprint(r.Context(), fp)
		next.ServeHTTP(w, r.WithContext(ctxWithFP))
	})
}
