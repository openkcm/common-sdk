package commonhttp_test

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/openkcm/common-sdk/pkg/commoncfg"
	"github.com/openkcm/common-sdk/pkg/commonhttp"
)

func TestHTTPClientLogging(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		wantLevel  string
	}{
		{name: "200 logs info", statusCode: 200, wantLevel: "INFO"},
		{name: "500 logs info", statusCode: 500, wantLevel: "INFO"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tc.statusCode)
			}))
			defer srv.Close()

			var buf bytes.Buffer
			slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, nil)))

			client, err := commonhttp.NewHTTPClient(&commoncfg.HTTPClient{
				Timeout: 5 * time.Second,
				Log:     true,
			})
			require.NoError(t, err)

			resp, err := client.Get(srv.URL + "/test")
			require.NoError(t, err)
			resp.Body.Close()

			var entry map[string]any
			require.NoError(t, json.Unmarshal(buf.Bytes(), &entry))

			assert.Equal(t, tc.wantLevel, entry["level"])
			assert.Equal(t, "outgoing HTTP request", entry["msg"])
			assert.Equal(t, "GET", entry["http.method"])
			assert.Contains(t, entry["http.url"], "/test")
			assert.EqualValues(t, tc.statusCode, entry["http.status_code"])
			assert.NotEmpty(t, entry["http.duration"])
		})
	}
}

func TestHTTPClientLoggingDisabled(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	var buf bytes.Buffer
	slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, nil)))

	client, err := commonhttp.NewHTTPClient(&commoncfg.HTTPClient{
		Timeout: 5 * time.Second,
	})
	require.NoError(t, err)

	resp, err := client.Get(srv.URL)
	require.NoError(t, err)
	resp.Body.Close()

	assert.Empty(t, buf.String())
}
