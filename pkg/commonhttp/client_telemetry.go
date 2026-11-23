package commonhttp

import (
	"context"
	"crypto/tls"
	"net/http"
	"net/http/httptrace"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"

	semconv "go.opentelemetry.io/otel/semconv/v1.43.0"
)

const instrumentationName = "github.com/openkcm/common-sdk/pkg/commonhttp"

// telemetryTransport supplies request-local metric labels and the custom error
// counter. otelhttp owns standard metrics and attempt spans, ending at body EOF/Close.
type telemetryTransport struct {
	transport *otelhttp.Transport
}

// telemetryRoundTripFunc allows error recording inside otelhttp's attempt span
// without introducing a second duration or tracing recorder.
type telemetryRoundTripFunc func(*http.Request) (*http.Response, error)

func (f telemetryRoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func newTelemetryTransport(next http.RoundTripper) (*telemetryTransport, error) {
	provider := otel.GetMeterProvider()

	errors, err := provider.Meter(instrumentationName).Int64Counter(
		"http.client.request.errors",
		metric.WithDescription("Outgoing HTTP requests that resulted in a network error or 4xx/5xx status"),
	)
	if err != nil {
		return nil, err
	}

	if next == nil {
		next = http.DefaultTransport
	}

	attempt := telemetryRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		resp, err := next.RoundTrip(req)
		if err != nil || (resp != nil && resp.StatusCode >= http.StatusBadRequest) {
			// Preserve the counter's original labels, including the port in
			// server.address. Record with the CLIENT span context for exemplars.
			var host, path string
			if req.URL != nil {
				host, path = req.URL.Host, req.URL.Path
			}

			errors.Add(req.Context(), 1, metric.WithAttributes(
				semconv.HTTPRequestMethodKey.String(req.Method),
				semconv.ServerAddress(host),
				semconv.URLPath(path),
			))
		}

		return resp, err
	})

	return &telemetryTransport{transport: otelhttp.NewTransport(attempt,
		otelhttp.WithTracerProvider(otel.GetTracerProvider()),
		otelhttp.WithMeterProvider(provider),
		otelhttp.WithClientTrace(newClientTrace),
	)}, nil
}

func (t *telemetryTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// An incoming server request or another outgoing call may share the caller's
	// labeler. Copy its labels before adding this attempt's path.
	labeler := &otelhttp.Labeler{}
	if inherited, ok := otelhttp.LabelerFromContext(req.Context()); ok {
		labeler.Add(inherited.Get()...)
	}

	if req.URL != nil {
		labeler.Add(semconv.URLPath(req.URL.Path))
	}

	ctx := otelhttp.ContextWithLabeler(req.Context(), labeler)

	return t.transport.RoundTrip(req.WithContext(ctx))
}

// newClientTrace records network milestones on the attempt span. Their timestamps
// distinguish network phases from authentication and logging overhead. No headers,
// bodies or error text are added.
// Hooks may run concurrently, so only the concurrency-safe span is shared.
func newClientTrace(ctx context.Context) *httptrace.ClientTrace {
	span := trace.SpanFromContext(ctx)

	return &httptrace.ClientTrace{
		GetConn: func(string) {
			span.AddEvent("http.get_conn")
		},
		GotConn: func(info httptrace.GotConnInfo) {
			span.AddEvent("http.got_conn", trace.WithAttributes(
				attribute.Bool("http.connection.reused", info.Reused),
			))
		},
		DNSStart: func(httptrace.DNSStartInfo) {
			span.AddEvent("http.dns.start")
		},
		DNSDone: func(httptrace.DNSDoneInfo) {
			span.AddEvent("http.dns.done")
		},
		ConnectStart: func(string, string) {
			span.AddEvent("http.connect.start")
		},
		ConnectDone: func(string, string, error) {
			span.AddEvent("http.connect.done")
		},
		TLSHandshakeStart: func() {
			span.AddEvent("http.tls.start")
		},
		TLSHandshakeDone: func(tls.ConnectionState, error) {
			span.AddEvent("http.tls.done")
		},
		WroteHeaders: func() {
			span.AddEvent("http.wrote_headers")
		},
		WroteRequest: func(httptrace.WroteRequestInfo) {
			span.AddEvent("http.wrote_request")
		},
		GotFirstResponseByte: func() {
			span.AddEvent("http.got_first_response_byte")
		},
	}
}
