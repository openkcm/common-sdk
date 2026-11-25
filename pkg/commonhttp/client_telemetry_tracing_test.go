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
	"sync"
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
	client := &http.Client{Transport: newTelemetryTransport(base.Transport)}
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

// Unsampled requests get none of our hooks but keep the caller's. httptrace
// copies the caller's hooks into the trace otelhttp gets, so a trace shared
// between requests would race here.
func TestHTTPClientTraceSkipsUnsampledRequests(t *testing.T) {
	assert.Equal(t, httptrace.ClientTrace{}, *newClientTrace(t.Context()), "no hooks without a recording span")
	first, second := newClientTrace(t.Context()), newClientTrace(t.Context())
	assert.NotSame(t, first, second, "each request needs its own trace")

	previous := otel.GetTracerProvider()
	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter), sdktrace.WithSampler(sdktrace.NeverSample()))
	otel.SetTracerProvider(tp)
	t.Cleanup(func() {
		require.NoError(t, tp.Shutdown(context.Background()))
		otel.SetTracerProvider(previous)
	})

	var written atomic.Int32

	ctx := httptrace.WithClientTrace(t.Context(), &httptrace.ClientTrace{
		WroteRequest: func(httptrace.WroteRequestInfo) { written.Add(1) },
	})

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client, err := NewHTTPClient(&commoncfg.HTTPClient{Timeout: time.Second})
	require.NoError(t, err)

	const requests = 4

	var wg sync.WaitGroup
	for range requests {
		wg.Go(func() {
			req, err := http.NewRequestWithContext(ctx, http.MethodGet, server.URL, nil)
			if !assert.NoError(t, err) {
				return
			}

			resp, err := client.Do(req)
			if !assert.NoError(t, err) {
				return
			}

			_, _ = io.Copy(io.Discard, resp.Body)
			assert.NoError(t, resp.Body.Close())
		})
	}

	wg.Wait()

	assert.EqualValues(t, requests, written.Load(), "the caller's hooks must still run")
	assert.Empty(t, exporter.GetSpans())
}

// A failed phase must say so on its event, by error type only: the error text
// may hold addresses or certificate details.
func TestHTTPClientTracePhaseErrors(t *testing.T) {
	tlsServer := httptest.NewTLSServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	defer tlsServer.Close()

	closedServer := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	closedURL := closedServer.URL
	closedServer.Close()

	for _, tc := range []struct {
		name, url, event, wantType, hidden string
	}{
		// The default transport does not trust the test server's certificate.
		{name: "TLS handshake", url: tlsServer.URL, event: "http.tls.done", wantType: "*tls.CertificateVerificationError", hidden: "x509"},
		{name: "connect", url: closedURL, event: "http.connect.done", wantType: "*net.OpError", hidden: "refused"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, exporter, _ := setupRetryTracing(t)

			client, err := NewHTTPClient(&commoncfg.HTTPClient{Timeout: 5 * time.Second})
			require.NoError(t, err)
			req, err := http.NewRequestWithContext(ctx, http.MethodGet, tc.url, nil)
			require.NoError(t, err)

			resp, err := client.Do(req)
			if resp != nil {
				_ = resp.Body.Close()
			}

			require.Error(t, err)

			spans := exporter.GetSpans()
			require.Len(t, spans, 1)

			found := false

			for _, event := range spans[0].Events {
				for _, attr := range event.Attributes {
					assert.NotContains(t, attr.Value.String(), tc.hidden, "events must not carry error text")
				}

				if event.Name != tc.event {
					continue
				}

				found = true
				attrs := attribute.NewSet(event.Attributes...)
				errorType, ok := attrs.Value("error.type")
				require.True(t, ok, "%s must carry error.type", tc.event)
				assert.Equal(t, tc.wantType, errorType.AsString())
			}

			assert.True(t, found, "expected event %q", tc.event)
		})
	}
}
