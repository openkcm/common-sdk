package logger

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	slogctx "github.com/veqryn/slog-context"

	"github.com/openkcm/common-sdk/pkg/commoncfg"
)

// traceSpanIDExtractor correlates logs independently of recording/sampling and
// captures field names per handler instead of modifying process-wide variables.
func traceSpanIDExtractor(fields commoncfg.LoggerOTel) slogctx.AttrExtractor {
	traceKey := strings.TrimSpace(fields.TraceID)
	spanKey := strings.TrimSpace(fields.SpanID)

	return func(ctx context.Context, _ time.Time, level slog.Level, message string) []slog.Attr {
		span := trace.SpanFromContext(ctx)
		if span.IsRecording() && level >= slog.LevelError {
			span.SetStatus(codes.Error, message)
		}

		sc := span.SpanContext()
		if !sc.IsValid() {
			return nil
		}

		return []slog.Attr{
			slog.String(traceKey, sc.TraceID().String()),
			slog.String(spanKey, sc.SpanID().String()),
		}
	}
}
