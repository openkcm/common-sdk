package otlp_test

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/protobuf/proto"

	collectortrace "go.opentelemetry.io/proto/otlp/collector/trace/v1"

	"github.com/openkcm/common-sdk/pkg/commoncfg"
	"github.com/openkcm/common-sdk/pkg/otlp"
)

func TestInitPropagationAndParentSampling(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		name := "without export"
		if enabled {
			name = "with OTLP export"
		}

		t.Run(name, func(t *testing.T) {
			previousProvider, previousPropagator, previousLogger := otel.GetTracerProvider(), otel.GetTextMapPropagator(), slog.Default()
			requests := make(chan []byte, 8)

			collector := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				body, _ := io.ReadAll(r.Body)
				requests <- body

				w.WriteHeader(http.StatusOK)
			}))
			defer collector.Close()

			ctx, cancel := context.WithCancel(t.Context())
			done := make(chan struct{})
			require.NoError(t, otlp.Init(ctx, &commoncfg.Application{Name: "test"}, &commoncfg.Telemetry{
				Traces: commoncfg.Trace{Enabled: enabled, Protocol: commoncfg.HTTPProtocol, URL: "/v1/traces",
					Host:      commoncfg.SourceRef{Source: commoncfg.EmbeddedSourceValue, Value: strings.TrimPrefix(collector.URL, "http://")},
					SecretRef: commoncfg.SecretRef{Type: commoncfg.InsecureSecretType},
				},
			}, &commoncfg.Logger{}, otlp.WithShutdownComplete(done)))
			t.Cleanup(func() {
				cancel()

				select {
				case <-done:
				case <-time.After(10 * time.Second):
					t.Error("telemetry shutdown did not complete")
				}

				otel.SetTracerProvider(previousProvider)
				otel.SetTextMapPropagator(previousPropagator)
				slog.SetDefault(previousLogger)
			})

			for _, parent := range []string{"", "invalid", "00-00000000000000000000000000000000-bbbbbbbbbbbbbbbb-01", "00-cccccccccccccccccccccccccccccccc-0000000000000000-01", "00-aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa-bbbbbbbbbbbbbbbb-00", "00-aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa-bbbbbbbbbbbbbbbb-01"} {
				requestCtx := otel.GetTextMapPropagator().Extract(ctx, propagation.MapCarrier{
					"traceparent": parent, "tracestate": "vendor=value", "baggage": "vendor=opaque",
				})
				requestCtx, span := otel.Tracer("test").Start(requestCtx, "request")
				carrier := propagation.MapCarrier{}
				otel.GetTextMapPropagator().Inject(requestCtx, carrier)
				span.End()

				sc := trace.SpanContextFromContext(requestCtx)
				require.True(t, sc.IsValid(), "every request needs correlation even without export")
				assert.NotEmpty(t, carrier["traceparent"])
				assert.Equal(t, "vendor=opaque", carrier["baggage"])

				if strings.Contains(parent, "aaaaaaaa") {
					assert.Equal(t, "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", sc.TraceID().String())
					assert.Equal(t, strings.HasSuffix(parent, "-01"), sc.IsSampled())
					assert.Equal(t, "vendor=value", carrier["tracestate"])
				}
			}

			cancel()

			select {
			case <-done:
			case <-time.After(10 * time.Second):
				t.Fatal("telemetry shutdown did not complete")
			}

			if !enabled {
				assert.Empty(t, requests, "disabled export must not make OTLP calls")
				return
			}
			// Shutdown has completed every export, including when the batch size is one.
			close(requests)

			spanCount := 0

			for body := range requests {
				var exported collectortrace.ExportTraceServiceRequest
				require.NoError(t, proto.Unmarshal(body, &exported))

				for _, resource := range exported.GetResourceSpans() {
					for _, scope := range resource.GetScopeSpans() {
						spanCount += len(scope.GetSpans())
					}
				}
			}

			assert.Equal(t, 5, spanCount, "only sampled spans must flush on shutdown")
		})
	}
}
