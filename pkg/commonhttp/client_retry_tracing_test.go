package commonhttp

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/metric/exemplar"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"

	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"

	"github.com/openkcm/common-sdk/pkg/commoncfg"
)

func setupRetryTracing(t *testing.T) (context.Context, *tracetest.InMemoryExporter, *sdkmetric.ManualReader) {
	t.Helper()

	previousTrace, previousMeter, previousPropagation := otel.GetTracerProvider(), otel.GetMeterProvider(), otel.GetTextMapPropagator()
	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	reader := sdkmetric.NewManualReader()
	mp := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader), sdkmetric.WithExemplarFilter(exemplar.AlwaysOnFilter))

	otel.SetTracerProvider(tp)
	otel.SetMeterProvider(mp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}))
	t.Cleanup(func() {
		require.NoError(t, tp.Shutdown(context.Background()))
		require.NoError(t, mp.Shutdown(context.Background()))
		otel.SetTracerProvider(previousTrace)
		otel.SetMeterProvider(previousMeter)
		otel.SetTextMapPropagator(previousPropagation)
	})
	ctx := otel.GetTextMapPropagator().Extract(t.Context(), propagation.MapCarrier{
		"traceparent": "00-aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa-bbbbbbbbbbbbbbbb-01",
		"tracestate":  "vendor=value", "baggage": "vendor=opaque",
	})

	return ctx, exporter, reader
}

// Double otelhttp wrapping would create an extra CLIENT span and an extra HTTP
// metric observation. Missing inner instrumentation would lose attempt parents.
func TestHTTPClientRetrySpanHierarchyAndMetrics(t *testing.T) {
	for _, tc := range []struct {
		name       string
		statuses   []int
		retry      bool
		wantStatus codes.Code
		wantErrors int64
	}{
		{name: "success after retries", statuses: []int{503, 503, 200}, retry: true, wantErrors: 2},
		{name: "exhausted retries", statuses: []int{503, 503, 503}, retry: true, wantStatus: codes.Error, wantErrors: 3},
		{name: "first attempt succeeds", statuses: []int{200}, retry: true},
		{name: "non retryable failure", statuses: []int{400}, retry: true, wantStatus: codes.Error, wantErrors: 1},
		{name: "retry disabled", statuses: []int{200}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, exporter, reader := setupRetryTracing(t)
			received := make(chan trace.SpanContext, 4)

			var calls atomic.Int32

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				received <- trace.SpanContextFromContext(otel.GetTextMapPropagator().Extract(r.Context(), propagation.HeaderCarrier(r.Header)))

				assert.Equal(t, "vendor=value", r.Header.Get("Tracestate"))
				assert.Equal(t, "vendor=opaque", r.Header.Get("Baggage"))

				i := int(calls.Add(1)) - 1
				w.WriteHeader(tc.statuses[min(i, len(tc.statuses)-1)])
				_, _ = io.WriteString(w, "response body")
			}))
			defer server.Close()

			cfg := &commoncfg.HTTPClient{Timeout: 3 * time.Second}
			if tc.retry {
				cfg.Retry = commoncfg.HTTPRetry{MaxAttempts: 3, InitialBackoff: 10 * time.Millisecond, MaxBackoff: 20 * time.Millisecond, RetryableStatusCodes: []int{503}}
			}

			client, err := NewHTTPClient(cfg)
			require.NoError(t, err)

			defer client.CloseIdleConnections()

			req, err := http.NewRequestWithContext(ctx, http.MethodGet, server.URL, nil)
			require.NoError(t, err)
			resp, err := client.Do(req)
			require.NoError(t, err)

			defer resp.Body.Close()

			assert.Equal(t, tc.statuses[len(tc.statuses)-1], resp.StatusCode)
			assert.EqualValues(t, len(tc.statuses), calls.Load())

			// The operation ends at final headers. The final CLIENT span still
			// belongs to the caller's response-body lifetime.
			var operations []tracetest.SpanStub

			for _, span := range exporter.GetSpans() {
				if span.SpanKind == trace.SpanKindInternal {
					operations = append(operations, span)
				}
			}

			parent := trace.SpanContextFromContext(ctx)

			if tc.retry {
				require.Len(t, operations, 1)
				operation := operations[0]
				assert.Empty(t, operation.Events, "network events belong to individual attempts")
				assert.Equal(t, parent.SpanID(), operation.Parent.SpanID())
				assert.Equal(t, tc.wantStatus, operation.Status.Code)
				attrs := attribute.NewSet(operation.Attributes...)
				attempts, ok := attrs.Value("retry.attempts")
				require.True(t, ok)
				assert.EqualValues(t, len(tc.statuses), attempts.AsInt64())

				parent = operation.SpanContext

				assert.Len(t, exporter.GetSpans(), len(tc.statuses))
			} else {
				assert.Empty(t, operations)
				assert.Empty(t, exporter.GetSpans())
			}

			body, err := io.ReadAll(resp.Body)
			require.NoError(t, err)
			assert.Equal(t, "response body", string(body), "the final response must remain readable even on exhaustion")
			require.NoError(t, resp.Body.Close())

			var attempts []tracetest.SpanStub

			for _, span := range exporter.GetSpans() {
				if span.SpanKind == trace.SpanKindClient {
					attempts = append(attempts, span)
				}
			}

			require.Len(t, attempts, len(tc.statuses))
			slices.SortFunc(attempts, func(a, b tracetest.SpanStub) int { return a.StartTime.Compare(b.StartTime) })

			for i, span := range attempts {
				var eventNames []string
				for _, event := range span.Events {
					eventNames = append(eventNames, event.Name)
				}

				assert.Contains(t, eventNames, "http.wrote_request")
				assert.Contains(t, eventNames, "http.got_first_response_byte")

				remote := <-received
				assert.Equal(t, parent.TraceID(), remote.TraceID())
				assert.Equal(t, parent.SpanID(), span.Parent.SpanID())
				assert.Equal(t, remote.SpanID(), span.SpanContext.SpanID())

				if i > 0 {
					assert.NotEqual(t, attempts[i-1].SpanContext.SpanID(), span.SpanContext.SpanID())
					assert.GreaterOrEqual(t, span.StartTime.Sub(attempts[i-1].EndTime), cfg.Retry.InitialBackoff,
						"discarded response spans must end before backoff")
				}
			}

			var metrics metricdata.ResourceMetrics
			require.NoError(t, reader.Collect(t.Context(), &metrics))

			var (
				observations uint64
				errors       int64
			)

			for _, scope := range metrics.ScopeMetrics {
				for _, m := range scope.Metrics {
					if m.Name == "http.client.request.errors" {
						assert.Equal(t, "github.com/openkcm/common-sdk/pkg/commonhttp", scope.Scope.Name)

						sum, ok := m.Data.(metricdata.Sum[int64])
						require.True(t, ok)

						for _, point := range sum.DataPoints {
							errors += point.Value
							require.NotEmpty(t, point.Exemplars)

							for _, exemplar := range point.Exemplars {
								var attemptIDs [][]byte

								for _, attempt := range attempts {
									if attempt.Status.Code == codes.Error {
										spanID := attempt.SpanContext.SpanID()
										attemptIDs = append(attemptIDs, spanID[:])
									}
								}

								assert.Contains(t, attemptIDs, exemplar.SpanID, "counter exemplars must identify a failed CLIENT span")
							}
						}
					}

					if m.Name == "http.client.request.duration" {
						assert.Equal(t, "go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp", scope.Scope.Name)

						hist, ok := m.Data.(metricdata.Histogram[float64])
						require.True(t, ok)

						for _, point := range hist.DataPoints {
							observations += point.Count
						}
					}
				}
			}

			assert.EqualValues(t, len(tc.statuses), observations, "HTTP metrics count attempts, not the enclosing operation")
			assert.Equal(t, tc.wantErrors, errors)
		})
	}
}

type cancelOnCloseBody struct {
	io.Reader

	cancel context.CancelFunc
	closed bool
}

func (b *cancelOnCloseBody) Close() error {
	b.closed = true
	b.cancel()

	return nil
}

// A discarded response must be closed before waiting. Cancelling on Close also
// proves that cancellation returns an error instead of a stale retryable response.
func TestHTTPClientRetryClosesResponseBeforeBackoff(t *testing.T) {
	ctx, exporter, _ := setupRetryTracing(t)

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	body := &cancelOnCloseBody{Reader: strings.NewReader("retry response"), cancel: cancel}
	calls := 0
	transport := &retryableClient{
		Next: newTestTelemetryTransport(t, roundTripFunc(func(*http.Request) (*http.Response, error) {
			calls++
			return &http.Response{StatusCode: http.StatusServiceUnavailable, Header: http.Header{}, Body: body}, nil
		})),
		Retry: commoncfg.HTTPRetry{MaxAttempts: 2, InitialBackoff: time.Millisecond, MaxBackoff: time.Millisecond, RetryableStatusCodes: []int{503}},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://example.test", nil)
	require.NoError(t, err)

	resp, err := transport.RoundTrip(req)
	if resp != nil {
		_ = resp.Body.Close()
	}

	require.ErrorIs(t, err, context.Canceled)
	assert.Nil(t, resp)
	assert.Equal(t, 1, calls)
	assert.True(t, body.closed)

	spans := exporter.GetSpans()
	require.Len(t, spans, 2)
	assert.Equal(t, trace.SpanKindInternal, spans[1].SpanKind)
	assert.Equal(t, codes.Error, spans[1].Status.Code)
}

func TestHTTPClientRetryDoesNotReturnDiscardedResponseOnNetworkFailure(t *testing.T) {
	ctx, exporter, _ := setupRetryTracing(t)
	networkErr := errors.New("connection failed")
	calls := 0
	transport := &retryableClient{
		Next: newTestTelemetryTransport(t, roundTripFunc(func(*http.Request) (*http.Response, error) {
			calls++
			if calls == 1 {
				return &http.Response{StatusCode: http.StatusServiceUnavailable, Header: http.Header{}, Body: io.NopCloser(strings.NewReader("discarded"))}, nil
			}

			return nil, networkErr
		})),
		Retry: commoncfg.HTTPRetry{MaxAttempts: 2, InitialBackoff: time.Millisecond, MaxBackoff: time.Millisecond, RetryableStatusCodes: []int{503}},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://example.test", nil)
	require.NoError(t, err)

	resp, err := transport.RoundTrip(req)
	if resp != nil {
		_ = resp.Body.Close()
	}

	require.ErrorIs(t, err, networkErr)
	assert.Nil(t, resp)

	spans := exporter.GetSpans()
	require.Len(t, spans, 3)
	assert.Equal(t, trace.SpanKindInternal, spans[2].SpanKind)
	assert.Equal(t, codes.Error, spans[2].Status.Code)
}
