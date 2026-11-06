package commonhttp_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
	prev := otel.GetMeterProvider()

	otel.SetMeterProvider(provider)
	t.Cleanup(func() { otel.SetMeterProvider(prev) })

	return reader
}

func collectMetrics(t *testing.T, reader *sdkmetric.ManualReader) []metricdata.Metrics {
	t.Helper()

	var rm metricdata.ResourceMetrics
	require.NoError(t, reader.Collect(t.Context(), &rm))

	if len(rm.ScopeMetrics) == 0 {
		return nil
	}

	return rm.ScopeMetrics[0].Metrics
}

func findMetric(metrics []metricdata.Metrics, name string) *metricdata.Metrics {
	for i := range metrics {
		if metrics[i].Name == name {
			return &metrics[i]
		}
	}

	return nil
}

func TestHTTPClientMetrics(t *testing.T) {
	tests := []struct {
		name           string
		statusCode     int
		wantErrorCount int64
	}{
		{
			name:           "200 records latency, no error",
			statusCode:     200,
			wantErrorCount: 0,
		},
		{
			name:           "404 records latency and error",
			statusCode:     404,
			wantErrorCount: 1,
		},
		{
			name:           "500 records latency and error",
			statusCode:     500,
			wantErrorCount: 1,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			reader := setupMetrics(t)

			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.statusCode)
			}))
			defer srv.Close()

			client, err := commonhttp.NewHTTPClient(&commoncfg.HTTPClient{
				Timeout: 5 * time.Second,
			})
			require.NoError(t, err)

			resp, err := client.Get(srv.URL + "/test/path")
			require.NoError(t, err)
			resp.Body.Close()

			metrics := collectMetrics(t, reader)

			// Assert latency histogram has one data point.
			latency := findMetric(metrics, "http.client.request.duration")
			require.NotNil(t, latency, "expected http.client.request.duration metric")

			hist, ok := latency.Data.(metricdata.Histogram[float64])
			require.True(t, ok, "expected Histogram, got %T", latency.Data)
			require.NotEmpty(t, hist.DataPoints, "expected at least one histogram data point")

			dp := hist.DataPoints[0]
			assert.Equal(t, uint64(1), dp.Count)
			assertAttr(t, dp.Attributes, "http.request.method", "GET")
			assertAttr(t, dp.Attributes, "url.path", "/test/path")

			// Assert error counter.
			errMetric := findMetric(metrics, "http.client.request.errors")
			if tc.wantErrorCount == 0 {
				if errMetric != nil {
					sum, ok := errMetric.Data.(metricdata.Sum[int64])
					if ok {
						for _, edp := range sum.DataPoints {
							assert.Zero(t, edp.Value, "expected no errors")
						}
					}
				}
			} else {
				require.NotNil(t, errMetric, "expected http.client.request.errors metric")
				sum, ok := errMetric.Data.(metricdata.Sum[int64])
				require.True(t, ok, "expected Sum, got %T", errMetric.Data)

				var total int64
				for _, edp := range sum.DataPoints {
					total += edp.Value
				}

				assert.Equal(t, tc.wantErrorCount, total)
			}
		})
	}
}

func assertAttr(t *testing.T, set attribute.Set, key, wantVal string) {
	t.Helper()

	val, ok := set.Value(attribute.Key(key))
	require.True(t, ok, "expected attribute %q, not found", key)
	assert.Equal(t, wantVal, val.AsString())
}
