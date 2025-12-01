package fingerprint

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
)

type ctxKey string

const fingerprintKey ctxKey = "fingerprint"

func FingerprintCtxMiddleware(next http.Handler) http.Handler {
	builder := NewBuilder()

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fp, err := builder.FromHTTPRequest(r)
		if err != nil {
			slog.Error("failed to generate fingerprint", "error", err)
			w.WriteHeader(http.StatusInternalServerError)

			return
		}

		ctxWithFP := context.WithValue(r.Context(), fingerprintKey, fp)
		next.ServeHTTP(w, r.WithContext(ctxWithFP))
	})
}

func ExtractFingerprint(ctx context.Context) (string, error) {
	fp, ok := ctx.Value(fingerprintKey).(string)
	if !ok {
		return "", errors.New("no fingerprint in ctx")
	}

	return fp, nil
}
