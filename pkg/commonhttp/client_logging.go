package commonhttp

import (
	"log/slog"
	"net/http"
	"time"
)

type loggingClient struct {
	Next http.RoundTripper
}

func (l *loggingClient) RoundTrip(req *http.Request) (*http.Response, error) {
	start := time.Now()

	resp, err := l.Next.RoundTrip(req)

	attrs := []any{
		slog.String("http.method", req.Method),
		slog.String("http.url", req.URL.String()),
		slog.Duration("http.duration", time.Since(start)),
	}

	if err != nil {
		slog.ErrorContext(req.Context(), "outgoing HTTP request failed", append(attrs, slog.Any("error", err))...)
		return resp, err
	}

	attrs = append(attrs, slog.Int("http.status_code", resp.StatusCode))
	slog.InfoContext(req.Context(), "outgoing HTTP request", attrs...)

	return resp, nil
}
