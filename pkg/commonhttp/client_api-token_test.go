package commonhttp

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/openkcm/common-sdk/pkg/commoncfg"
	"github.com/stretchr/testify/assert"
)

func TestNewClientFromAPIToken(t *testing.T) {
	tests := []struct {
		name       string
		value      *commoncfg.SourceRef
		wantErr    bool
		errMessage string
	}{
		{
			name:       "nil config",
			value:      nil,
			wantErr:    true,
			errMessage: "api token auth config is nil",
		},
		{
			name: "empty token",
			value: &commoncfg.SourceRef{
				Source: commoncfg.EmbeddedSourceValue,
				Value:  "",
			},
			wantErr:    true,
			errMessage: "api token is empty",
		},
		{
			name: "valid token",
			value: &commoncfg.SourceRef{
				Source: commoncfg.EmbeddedSourceValue,
				Value:  "my-secret-token",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := NewClientFromAPIToken(tt.value)
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errMessage != "" {
					assert.Contains(t, err.Error(), tt.errMessage)
				}
				return
			}

			assert.NoError(t, err)
			assert.NotNil(t, client)
			assert.NotNil(t, client.Transport)
		})
	}
}

func TestClientAPITokenRoundTripper(t *testing.T) {
	token := "test-token"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		expectedHeader := "Api-Token " + token
		assert.Equal(t, expectedHeader, authHeader)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client, err := NewClientFromAPIToken(&commoncfg.SourceRef{
		Source: commoncfg.EmbeddedSourceValue,
		Value:  token,
	})
	assert.NoError(t, err)

	req, err := http.NewRequest("GET", server.URL, nil)
	assert.NoError(t, err)

	resp, err := client.Do(req)
	assert.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
}
