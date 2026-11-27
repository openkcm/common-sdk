package commonhttp_test

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"

	sdkmetric "go.opentelemetry.io/otel/sdk/metric"

	"github.com/openkcm/common-sdk/pkg/commoncfg"
	"github.com/openkcm/common-sdk/pkg/commonhttp"
)

const (
	durationMetric = "http.client.request.duration"
	bodySizeMetric = "http.client.request.body.size"
)

func setupMetrics(t *testing.T) *sdkmetric.ManualReader {
	t.Helper()

	reader := sdkmetric.NewManualReader()
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	previous := otel.GetMeterProvider()

	otel.SetMeterProvider(provider)
	t.Cleanup(func() {
		require.NoError(t, provider.Shutdown(context.Background()))
		otel.SetMeterProvider(previous)
	})

	return reader
}

type metricPoint struct {
	scope string
	attrs attribute.Set
	count uint64
}

// collectPoints returns the data points of every histogram, by metric name. The
// client records histograms only, all of them from otelhttp.
func collectPoints(t *testing.T, reader *sdkmetric.ManualReader) map[string][]metricPoint {
	t.Helper()

	var metrics metricdata.ResourceMetrics
	require.NoError(t, reader.Collect(t.Context(), &metrics))

	points := make(map[string][]metricPoint)

	for _, scope := range metrics.ScopeMetrics {
		assert.Equal(t, otelhttp.ScopeName, scope.Scope.Name, "otelhttp records every HTTP client metric")

		for _, m := range scope.Metrics {
			switch data := m.Data.(type) {
			case metricdata.Histogram[float64]:
				for _, point := range data.DataPoints {
					points[m.Name] = append(points[m.Name], metricPoint{scope.Scope.Name, point.Attributes, point.Count})
				}
			case metricdata.Histogram[int64]:
				for _, point := range data.DataPoints {
					points[m.Name] = append(points[m.Name], metricPoint{scope.Scope.Name, point.Attributes, point.Count})
				}
			default:
				t.Fatalf("unexpected data type %T for %s", m.Data, m.Name)
			}
		}
	}

	return points
}

// A second recorder would double the observation count across scopes. The
// standard attributes must support both HTTP-status and transport-error rates.
func TestHTTPClientTelemetryMetrics(t *testing.T) {
	for _, tc := range []struct {
		name           string
		status         int
		networkFailure bool
	}{
		{name: "success", status: 200},
		{name: "client error", status: 404},
		{name: "server error", status: 503},
		{name: "transport error", networkFailure: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			reader := setupMetrics(t)

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tc.status)
			}))
			defer server.Close()

			if tc.networkFailure {
				server.Close()
			}

			client, err := commonhttp.NewHTTPClient(&commoncfg.HTTPClient{Timeout: time.Second})
			require.NoError(t, err)
			req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, server.URL+"/test/path?credential=not-a-metric-label", nil)
			require.NoError(t, err)

			response, err := client.Do(req)
			if tc.networkFailure {
				require.Error(t, err)
				assert.Nil(t, response)
			} else {
				require.NoError(t, err)
				require.NoError(t, response.Body.Close())
			}

			target, err := url.Parse(server.URL)
			require.NoError(t, err)
			port, err := strconv.Atoi(target.Port())
			require.NoError(t, err)

			points := collectPoints(t, reader)
			assert.Len(t, points, 2, "only the duration and request body size histograms")

			for name, metricPoints := range points {
				for _, point := range metricPoints {
					_, hasPath := point.attrs.Value("url.path")
					assert.False(t, hasPath, "%s must not carry url.path", name)

					for _, attr := range point.attrs.ToSlice() {
						assert.NotContains(t, attr.Value.String(), "not-a-metric-label")
						assert.NotContains(t, attr.Value.String(), "/test/path")
					}
				}
			}

			for _, name := range []string{durationMetric, bodySizeMetric} {
				require.Len(t, points[name], 1, name)
				assert.EqualValues(t, 1, points[name][0].count, "each attempt records %s exactly once across all scopes", name)
			}

			duration := points[durationMetric][0].attrs
			assertAttr(t, duration, "http.request.method", "GET")
			assertAttr(t, duration, "server.address", target.Hostname())
			assertAttr(t, duration, "url.scheme", "http")
			serverPort, ok := duration.Value("server.port")
			require.True(t, ok)
			assert.EqualValues(t, port, serverPort.AsInt64())

			status, hasStatus := duration.Value("http.response.status_code")
			errorType, hasError := duration.Value("error.type")

			if tc.networkFailure {
				assert.False(t, hasStatus)
				assert.True(t, hasError)
				assert.NotEmpty(t, errorType.AsString())
			} else {
				assert.True(t, hasStatus)
				assert.EqualValues(t, tc.status, status.AsInt64())
			}
		})
	}
}

// Paths often hold identifiers. Each distinct path would otherwise add a series
// until the SDK's per-instrument limit sends the rest to an overflow series.
func TestHTTPClientMetricsIgnoreURLPath(t *testing.T) {
	reader := setupMetrics(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	client, err := commonhttp.NewHTTPClient(&commoncfg.HTTPClient{Timeout: time.Second})
	require.NoError(t, err)

	const requests = 20

	for i := range requests {
		req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, fmt.Sprintf("%s/keys/%d", server.URL, i), nil)
		require.NoError(t, err)
		resp, err := client.Do(req)
		require.NoError(t, err)
		require.NoError(t, resp.Body.Close())
	}

	points := collectPoints(t, reader)

	for _, name := range []string{durationMetric, bodySizeMetric} {
		require.Len(t, points[name], 1, "distinct paths must not add %s series", name)
		assert.EqualValues(t, requests, points[name][0].count, name)
	}
}

// Labels a caller puts on its client Labeler must reach the client metrics,
// without the client modifying that Labeler. A server-side Labeler (as set by
// an otelhttp handler) must not: its http.route belongs to the incoming request.
func TestHTTPClientMetricsUseCallerLabels(t *testing.T) {
	reader := setupMetrics(t)
	caller := &otelhttp.Labeler{}
	caller.Add(attribute.String("operation", "introspection"))
	original := caller.Get()

	serverSide := &otelhttp.Labeler{}
	serverSide.Add(attribute.String("http.route", "/incoming/{id}"))

	ctx := otelhttp.ContextWithLabeler(t.Context(), serverSide)
	ctx = otelhttp.ContextWithClientLabeler(ctx, caller)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/fail" {
			w.WriteHeader(http.StatusServiceUnavailable)

			return
		}

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client, err := commonhttp.NewHTTPClient(&commoncfg.HTTPClient{Timeout: time.Second})
	require.NoError(t, err)

	errs := make(chan error, 2)

	var wg sync.WaitGroup
	for _, path := range []string{"/ok", "/fail"} {
		wg.Go(func() {
			req, err := http.NewRequestWithContext(ctx, http.MethodGet, server.URL+path, nil)
			if err != nil {
				errs <- err
				return
			}

			resp, err := client.Do(req)
			if err != nil {
				errs <- err
				return
			}

			_, err = io.Copy(io.Discard, resp.Body)
			_ = resp.Body.Close()

			errs <- err
		})
	}

	wg.Wait()
	close(errs)

	for err := range errs {
		require.NoError(t, err)
	}

	assert.Equal(t, original, caller.Get(), "the client must not modify the caller's labeler")

	byStatus := make(map[int64]uint64)

	for _, point := range collectPoints(t, reader)[durationMetric] {
		assertAttr(t, point.attrs, "operation", "introspection")

		_, leaked := point.attrs.Value("http.route")
		assert.False(t, leaked, "server-side labels must not reach client metrics")

		status, ok := point.attrs.Value("http.response.status_code")
		require.True(t, ok)

		byStatus[status.AsInt64()] += point.count
	}

	assert.Equal(t, map[int64]uint64{http.StatusOK: 1, http.StatusServiceUnavailable: 1}, byStatus)
}

func assertAttr(t *testing.T, set attribute.Set, key, want string) {
	t.Helper()

	value, ok := set.Value(attribute.Key(key))
	require.True(t, ok, "expected attribute %q", key)
	assert.Equal(t, want, value.AsString())
}
