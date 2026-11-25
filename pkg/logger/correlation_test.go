package logger_test

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"testing"

	"github.com/creasty/defaults"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"

	slogctx "github.com/veqryn/slog-context"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"

	"github.com/openkcm/common-sdk/pkg/commoncfg"
	"github.com/openkcm/common-sdk/pkg/logger"
)

func TestLogCorrelationForEveryValidSpanContext(t *testing.T) {
	for _, kind := range []string{"recording", "unsampled", "remote", "absent"} {
		t.Run(kind, func(t *testing.T) {
			var cfg commoncfg.Logger
			require.NoError(t, defaults.Set(&cfg))

			var output bytes.Buffer

			handler, err := logger.InitHandlerWithWriter(&output, cfg, commoncfg.Application{Name: "example"})
			require.NoError(t, err)
			ctx := t.Context()
			exporter := tracetest.NewInMemoryExporter()
			tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))

			t.Cleanup(func() { require.NoError(t, tp.Shutdown(context.Background())) })

			if kind != "absent" {
				flags := "01"
				if kind == "unsampled" {
					flags = "00"
				}

				ctx = propagation.TraceContext{}.Extract(ctx, propagation.MapCarrier{
					"traceparent": "00-aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa-bbbbbbbbbbbbbbbb-" + flags,
				})

				if kind != "remote" {
					var span trace.Span

					ctx, span = tp.Tracer("test").Start(ctx, "request")
					defer span.End()
				}
			}

			sc := trace.SpanContextFromContext(ctx)
			ctx = slogctx.Append(ctx, "request_id", "request")
			slog.New(handler).ErrorContext(ctx, "operation failed")

			var record map[string]any
			require.NoError(t, json.Unmarshal(output.Bytes(), &record))

			if kind == "absent" {
				assert.NotContains(t, record, "traceId")
				assert.NotContains(t, record, "spanId")
			} else {
				assert.Equal(t, sc.TraceID().String(), record["traceId"])
				assert.Equal(t, sc.SpanID().String(), record["spanId"])
			}

			assert.Equal(t, 1, bytes.Count(output.Bytes(), []byte("\"request_id\":")))

			if kind == "recording" {
				trace.SpanFromContext(ctx).End()

				spans := exporter.GetSpans()
				require.Len(t, spans, 1)
				assert.Equal(t, codes.Error, spans[0].Status.Code)
				assert.Equal(t, "operation failed", spans[0].Status.Description)
			}
		})
	}
}

func TestLogCorrelationFieldNamesAreHandlerLocal(t *testing.T) {
	var cfg commoncfg.Logger
	require.NoError(t, defaults.Set(&cfg))
	cfg.Formatter.Fields.OTel.TraceID = " trace_id "
	cfg.Formatter.Fields.OTel.SpanID = " span_id "

	var first, second bytes.Buffer

	firstHandler, err := logger.InitHandlerWithWriter(&first, cfg, commoncfg.Application{})
	require.NoError(t, err)

	cfg.Formatter.Fields.OTel.TraceID = "traceId"
	cfg.Formatter.Fields.OTel.SpanID = "spanId"
	secondHandler, err := logger.InitHandlerWithWriter(&second, cfg, commoncfg.Application{})
	require.NoError(t, err)

	tp := sdktrace.NewTracerProvider()

	t.Cleanup(func() { require.NoError(t, tp.Shutdown(context.Background())) })

	ctx, span := tp.Tracer("test").Start(t.Context(), "request")
	defer span.End()

	slog.New(firstHandler).InfoContext(ctx, "first")
	slog.New(secondHandler).InfoContext(ctx, "second")

	var record map[string]any
	require.NoError(t, json.Unmarshal(first.Bytes(), &record))
	assert.Equal(t, span.SpanContext().TraceID().String(), record["trace_id"])
	assert.Equal(t, span.SpanContext().SpanID().String(), record["span_id"])
	assert.NotContains(t, record, "traceId")
	assert.Contains(t, second.String(), "\"traceId\"")
}
