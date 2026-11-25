package commonhttp_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/openkcm/common-sdk/pkg/commoncfg"
	"github.com/openkcm/common-sdk/pkg/commonhttp"
)

// Public discovery and dynamic OAuth2 clients must retain default proxy behavior
// and share idle connections when no transport overrides were requested.
func TestHTTPClientRetainsDefaultTransport(t *testing.T) {
	connections := make(chan string, 4)

	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "127.0.0.1:1", r.URL.Host)

		connections <- r.RemoteAddr

		_, _ = io.WriteString(w, "proxied")
	}))
	defer proxy.Close()

	proxyURL, err := url.Parse(proxy.URL)
	require.NoError(t, err)

	previous := http.DefaultTransport
	shared := &http.Transport{Proxy: http.ProxyURL(proxyURL)}
	http.DefaultTransport = shared

	t.Cleanup(func() {
		http.DefaultTransport = previous

		shared.CloseIdleConnections()
	})

	for _, tc := range []struct {
		name string
		cfg  commoncfg.HTTPClient
	}{
		{name: "public"},
		{name: "OAuth2", cfg: commoncfg.HTTPClient{OAuth2Auth: &commoncfg.OAuth2{
			Credentials: commoncfg.OAuth2Credentials{
				ClientID:   commoncfg.SourceRef{Source: commoncfg.EmbeddedSourceValue, Value: "client"},
				AuthMethod: commoncfg.OAuth2None,
			},
		}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var firstConnection string

			for i := range 2 {
				client, err := commonhttp.NewHTTPClient(&tc.cfg)
				require.NoError(t, err)
				req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, "http://127.0.0.1:1/resource", nil)
				require.NoError(t, err)
				resp, err := client.Do(req)
				require.NoError(t, err, "request must use the default transport's proxy")
				body, err := io.ReadAll(resp.Body)
				require.NoError(t, resp.Body.Close())
				require.NoError(t, err)
				assert.Equal(t, "proxied", string(body))

				connection := <-connections
				if i == 0 {
					firstConnection = connection
				} else {
					assert.Equal(t, firstConnection, connection, "separately constructed clients must share the default pool")
				}
			}
		})
	}
}
