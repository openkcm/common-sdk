package commonhttp

import (
	"net/http"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

const meterName = "github.com/openkcm/common-sdk/pkg/commonhttp"

// metricsClient is a RoundTripper that records latency and error rate
// for every outgoing HTTP request using OTel metrics.
type metricsClient struct {
	Next    http.RoundTripper
	latency metric.Float64Histogram
	errors  metric.Int64Counter
}

func newMetricsClient(next http.RoundTripper) (*metricsClient, error) {
	meter := otel.GetMeterProvider().Meter(meterName)

	latency, err := meter.Float64Histogram(
		"http.client.request.duration",
		metric.WithDescription("Outgoing HTTP request duration in seconds"),
		metric.WithUnit("s"),
	)
	if err != nil {
		return nil, err
	}

	errors, err := meter.Int64Counter(
		"http.client.request.errors",
		metric.WithDescription("Outgoing HTTP requests that resulted in a network error or 4xx/5xx status"),
	)
	if err != nil {
		return nil, err
	}

	return &metricsClient{
		Next:    next,
		latency: latency,
		errors:  errors,
	}, nil
}

func (m *metricsClient) RoundTrip(req *http.Request) (*http.Response, error) {
	start := time.Now()

	resp, err := m.Next.RoundTrip(req)

	elapsed := time.Since(start).Seconds()
	attrs := metric.WithAttributes(
		attribute.String("http.request.method", req.Method),
		attribute.String("server.address", req.URL.Host),
		attribute.String("url.path", req.URL.Path),
	)

	m.latency.Record(req.Context(), elapsed, attrs)

	if err != nil || (resp != nil && resp.StatusCode >= 400) {
		m.errors.Add(req.Context(), 1, attrs)
	}

	return resp, err
}
