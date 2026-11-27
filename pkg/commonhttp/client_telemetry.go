package commonhttp

import (
	"context"
	"crypto/tls"
	"net/http"
	"net/http/httptrace"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	semconv "go.opentelemetry.io/otel/semconv/v1.43.0"
)

const instrumentationName = "github.com/openkcm/common-sdk/pkg/commonhttp"

// newTelemetryTransport traces each attempt and records otelhttp's standard HTTP
// client metrics with the global meter provider. Attempt spans end at
// response-body EOF or Close.
//
// Failed attempts are counted from http.client.request.duration: its
// http.response.status_code label holds 4xx/5xx responses, and its error.type
// label transport errors.
//
// Metrics carry no URL path: paths often hold identifiers, which would exhaust
// the SDK's per-instrument series limit. Callers can add bounded labels, such as
// url.template or an operation name, with an otelhttp.Labeler attached by
// otelhttp.ContextWithClientLabeler. The Labeler is read, never modified. A
// server-side Labeler (otelhttp.ContextWithLabeler) is not used.
func newTelemetryTransport(next http.RoundTripper) *otelhttp.Transport {
	return otelhttp.NewTransport(next,
		// Required: without it, otelhttp takes the tracer from the parent span,
		// and a parent known only from incoming headers has a no-op one, so no
		// CLIENT span would be recorded.
		otelhttp.WithTracerProvider(otel.GetTracerProvider()),
		otelhttp.WithClientTrace(newClientTrace),
		otelhttp.WithMeterProvider(otel.GetMeterProvider()),
	)
}

// newClientTrace records network milestones on the attempt span. Their timestamps
// distinguish network phases from authentication and logging overhead. No headers,
// bodies or error text are added; a failed DNS, connect or TLS phase carries its
// error.type.
//
// Hooks may run concurrently, so only the concurrency-safe span is shared. A dial
// the transport started for this request may finish after the request took an
// idle connection; http.connection.reused on http.got_conn tells which applied.
func newClientTrace(ctx context.Context) *httptrace.ClientTrace {
	span := trace.SpanFromContext(ctx)
	if !span.IsRecording() {
		// No hooks for unsampled requests. otelhttp needs a trace anyway, and
		// httptrace copies the caller's hooks into it, so it must not be shared.
		return &httptrace.ClientTrace{}
	}

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
		DNSDone: func(info httptrace.DNSDoneInfo) {
			addPhaseEvent(span, "http.dns.done", info.Err)
		},
		ConnectStart: func(string, string) {
			span.AddEvent("http.connect.start")
		},
		ConnectDone: func(_, _ string, err error) {
			addPhaseEvent(span, "http.connect.done", err)
		},
		TLSHandshakeStart: func() {
			span.AddEvent("http.tls.start")
		},
		TLSHandshakeDone: func(_ tls.ConnectionState, err error) {
			addPhaseEvent(span, "http.tls.done", err)
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

// addPhaseEvent adds a phase's end event, with the error's type when it failed,
// as the span's own error.type does.
func addPhaseEvent(span trace.Span, name string, err error) {
	if err == nil {
		span.AddEvent(name)

		return
	}

	span.AddEvent(name, trace.WithAttributes(semconv.ErrorType(err)))
}
