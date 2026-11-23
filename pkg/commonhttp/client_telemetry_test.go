package commonhttp_test

import (
	"context"
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

// A second recorder would double the observation count across scopes. The
// standard attributes must support both HTTP-status and transport-error rates.
func TestHTTPClientTelemetryMetrics(t *testing.T) {
	for _, tc := range []struct {
		name           string
		status         int
		networkFailure bool
		wantErrors     int64
	}{
		{name: "success", status: 200},
		{name: "client error", status: 404, wantErrors: 1},
		{name: "server error", status: 503, wantErrors: 1},
		{name: "transport error", networkFailure: true, wantErrors: 1},
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

			var metrics metricdata.ResourceMetrics
			require.NoError(t, reader.Collect(t.Context(), &metrics))

			target, err := url.Parse(server.URL)
			require.NoError(t, err)
			port, err := strconv.Atoi(target.Port())
			require.NoError(t, err)

			var (
				observations uint64
				errors       int64
			)

			for _, scope := range metrics.ScopeMetrics {
				for _, metric := range scope.Metrics {
					if metric.Name == "http.client.request.errors" {
						assert.Equal(t, "github.com/openkcm/common-sdk/pkg/commonhttp", scope.Scope.Name)

						sum, ok := metric.Data.(metricdata.Sum[int64])
						require.True(t, ok)
						assert.True(t, sum.IsMonotonic)

						for _, point := range sum.DataPoints {
							errors += point.Value
							assert.Equal(t, 3, point.Attributes.Len())
							assertAttr(t, point.Attributes, "http.request.method", "GET")
							assertAttr(t, point.Attributes, "server.address", target.Host)
							assertAttr(t, point.Attributes, "url.path", "/test/path")
						}
					}

					if metric.Name != "http.client.request.duration" {
						continue
					}

					assert.Equal(t, otelhttp.ScopeName, scope.Scope.Name)

					histogram, ok := metric.Data.(metricdata.Histogram[float64])
					require.True(t, ok)

					for _, point := range histogram.DataPoints {
						observations += point.Count
						assertAttr(t, point.Attributes, "http.request.method", "GET")
						assertAttr(t, point.Attributes, "server.address", target.Hostname())
						assertAttr(t, point.Attributes, "url.path", "/test/path")
						serverPort, ok := point.Attributes.Value("server.port")
						require.True(t, ok)
						assert.EqualValues(t, port, serverPort.AsInt64())

						status, hasStatus := point.Attributes.Value("http.response.status_code")
						errorType, hasError := point.Attributes.Value("error.type")

						if tc.networkFailure {
							assert.False(t, hasStatus)
							assert.True(t, hasError)
							assert.NotEmpty(t, errorType.AsString())
						} else {
							assert.True(t, hasStatus)
							assert.EqualValues(t, tc.status, status.AsInt64())
						}

						for _, attr := range point.Attributes.ToSlice() {
							assert.NotContains(t, attr.Value.AsString(), "not-a-metric-label")
						}
					}
				}
			}

			assert.EqualValues(t, 1, observations, "each attempt records duration exactly once across all scopes")
			assert.Equal(t, tc.wantErrors, errors, "the custom error counter counts each failed attempt once")
		})
	}
}

// Request-local labels must not mutate an incoming handler's Labeler or leak
// into concurrent outgoing requests which share that context.
func TestHTTPClientTelemetryIsolatesLabels(t *testing.T) {
	reader := setupMetrics(t)
	inherited := &otelhttp.Labeler{}
	inherited.Add(attribute.String("operation", "introspection"), attribute.String("url.path", "/incoming"))
	original := inherited.Get()
	ctx := otelhttp.ContextWithLabeler(t.Context(), inherited)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client, err := commonhttp.NewHTTPClient(&commoncfg.HTTPClient{Timeout: time.Second})
	require.NoError(t, err)

	errs := make(chan error, 2)

	var wg sync.WaitGroup
	for _, path := range []string{"/first", "/second"} {
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

	assert.Equal(t, original, inherited.Get(), "outbound labels must not modify the caller's labeler")

	var metrics metricdata.ResourceMetrics
	require.NoError(t, reader.Collect(t.Context(), &metrics))

	byPath := make(map[string]uint64)

	for _, scope := range metrics.ScopeMetrics {
		if scope.Scope.Name != otelhttp.ScopeName {
			continue
		}

		for _, metric := range scope.Metrics {
			if metric.Name != "http.client.request.duration" {
				continue
			}

			histogram, ok := metric.Data.(metricdata.Histogram[float64])
			require.True(t, ok)

			for _, point := range histogram.DataPoints {
				assertAttr(t, point.Attributes, "operation", "introspection")
				path, ok := point.Attributes.Value("url.path")
				require.True(t, ok)

				byPath[path.AsString()] += point.Count
			}
		}
	}

	assert.Equal(t, map[string]uint64{"/first": 1, "/second": 1}, byPath)
}

func assertAttr(t *testing.T, set attribute.Set, key, want string) {
	t.Helper()

	value, ok := set.Value(attribute.Key(key))
	require.True(t, ok, "expected attribute %q", key)
	assert.Equal(t, want, value.AsString())
}
