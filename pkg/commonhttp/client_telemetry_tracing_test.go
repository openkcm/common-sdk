package commonhttp

import (
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/http/httptrace"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"

	sdktrace "go.opentelemetry.io/otel/sdk/trace"

	"github.com/openkcm/common-sdk/pkg/commoncfg"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestHTTPClientPropagationAndSpanLifetime(t *testing.T) {
	previousProvider, previousPropagator := otel.GetTracerProvider(), otel.GetTextMapPropagator()
	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}))
	t.Cleanup(func() {
		require.NoError(t, tp.Shutdown(context.Background()))
		otel.SetTracerProvider(previousProvider)
		otel.SetTextMapPropagator(previousPropagator)
	})
	ctx := otel.GetTextMapPropagator().Extract(t.Context(), propagation.MapCarrier{
		"traceparent": "00-aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa-bbbbbbbbbbbbbbbb-01",
		"tracestate":  "vendor=value", "baggage": "vendor=opaque",
	})

	var received trace.SpanContext

	base := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		received = trace.SpanContextFromContext(otel.GetTextMapPropagator().Extract(t.Context(), propagation.HeaderCarrier(r.Header)))
		assert.Equal(t, "vendor=opaque", r.Header.Get("Baggage"))
		assert.Equal(t, "Bearer sensitive-token", r.Header.Get("Authorization"))
		assert.Equal(t, "provider=example", r.URL.RawQuery)
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		require.NoError(t, r.Body.Close())
		assert.Equal(t, "sensitive-request-body", string(body))

		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader("sensitive-response-body")), Header: http.Header{}}, nil
	})}
	client := &http.Client{Transport: newTestTelemetryTransport(t, base.Transport)}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://example.test/introspect?provider=example", strings.NewReader("sensitive-request-body"))
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer sensitive-token")
	resp, err := client.Do(req)
	require.NoError(t, err)
	assert.Empty(t, exporter.GetSpans(), "the HTTP span remains open until the response body is consumed or closed")
	require.NoError(t, resp.Body.Close())
	assert.Empty(t, req.Header.Get("Traceparent"))
	assert.Equal(t, "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", received.TraceID().String())
	assert.Equal(t, "vendor=value", received.TraceState().String())

	spans := exporter.GetSpans()
	require.Len(t, spans, 1)
	assert.Equal(t, trace.SpanKindClient, spans[0].SpanKind)
	assert.Equal(t, received.SpanID(), spans[0].SpanContext.SpanID())
	assert.Equal(t, "bbbbbbbbbbbbbbbb", spans[0].Parent.SpanID().String())

	attributes := make(map[string]any)
	for _, attr := range spans[0].Attributes {
		attributes[string(attr.Key)] = attr.Value.AsInterface()
		assert.NotContains(t, attr.Value.AsString(), "sensitive")
	}

	assert.Equal(t, req.URL.String(), attributes["url.full"])
	assert.Equal(t, http.MethodPost, attributes["http.request.method"])
	assert.EqualValues(t, http.StatusOK, attributes["http.response.status_code"])
}

func newTestTelemetryTransport(t *testing.T, next http.RoundTripper) *telemetryTransport {
	t.Helper()

	transport, err := newTelemetryTransport(next)
	require.NoError(t, err)

	return transport
}

// Missing trace hooks would hide network phases; replacing caller hooks would
// break existing instrumentation. Reused connections must not invent setup work.
func TestHTTPClientTracePhasesAndConnectionReuse(t *testing.T) {
	ctx, exporter, _ := setupRetryTracing(t)
	written := make(chan struct{}, 2)

	var callerWrites, callerFirstBytes atomic.Int32

	ctx = httptrace.WithClientTrace(ctx, &httptrace.ClientTrace{
		WroteRequest: func(httptrace.WroteRequestInfo) {
			callerWrites.Add(1)

			written <- struct{}{}
		},
		GotFirstResponseByte: func() { callerFirstBytes.Add(1) },
	})

	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Make request-written/first-byte ordering deterministic without sleeps.
		select {
		case <-written:
		case <-r.Context().Done():
			return
		}

		w.Header().Set("X-Private-Data", "sensitive-response-header")
		w.WriteHeader(http.StatusEarlyHints)
		_, _ = io.WriteString(w, "sensitive-response-body")
	}))
	defer server.Close()

	target, err := url.Parse(server.URL)
	require.NoError(t, err)

	base, ok := server.Client().Transport.(*http.Transport)
	require.True(t, ok)

	base.TLSClientConfig.ServerName = target.Hostname()
	target.Host = net.JoinHostPort("localhost", target.Port())
	previousTransport := http.DefaultTransport
	http.DefaultTransport = base

	t.Cleanup(func() {
		http.DefaultTransport = previousTransport

		base.CloseIdleConnections()
	})

	client, err := NewHTTPClient(&commoncfg.HTTPClient{Timeout: 5 * time.Second, Log: true})
	require.NoError(t, err)

	for i := range 2 {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, target.String(), nil)
		require.NoError(t, err)
		req.Header.Set("Authorization", "Bearer sensitive-token")
		req.Header.Set("X-Private-Data", "sensitive-request-header")
		resp, err := client.Do(req)
		require.NoError(t, err)

		headersReturned := time.Now()
		body, err := io.ReadAll(resp.Body)
		require.NoError(t, resp.Body.Close())
		require.NoError(t, err)
		assert.Equal(t, "sensitive-response-body", string(body))

		spans := exporter.GetSpans()
		require.Len(t, spans, i+1, "phase timing must use events on the existing attempt span")
		span := spans[i]
		assert.Equal(t, trace.SpanKindClient, span.SpanKind)

		events := make(map[string]sdktrace.Event)
		for _, event := range span.Events {
			events[event.Name] = event
			for _, attr := range event.Attributes {
				assert.NotContains(t, attr.Value.String(), "sensitive", "phase events must not capture headers or bodies")
			}
		}

		for _, name := range []string{"http.get_conn", "http.got_conn", "http.wrote_headers", "http.wrote_request", "http.got_first_response_byte"} {
			require.Contains(t, events, name)
			assert.False(t, events[name].Time.Before(span.StartTime))
			assert.False(t, events[name].Time.After(headersReturned), "network events precede response logging and body consumption")
		}

		assert.False(t, events["http.got_first_response_byte"].Time.Before(events["http.wrote_request"].Time))
		attrs := attribute.NewSet(events["http.got_conn"].Attributes...)
		reused, ok := attrs.Value("http.connection.reused")
		require.True(t, ok)
		assert.Equal(t, i > 0, reused.AsBool())

		for _, name := range []string{"http.dns.start", "http.dns.done", "http.connect.start", "http.connect.done", "http.tls.start", "http.tls.done"} {
			if i == 0 {
				assert.Contains(t, events, name)
			} else {
				assert.NotContains(t, events, name, "reused connection has no new DNS, TCP or TLS setup")
			}
		}
	}

	assert.EqualValues(t, 2, callerWrites.Load())
	assert.EqualValues(t, 2, callerFirstBytes.Load())
}
