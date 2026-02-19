package commonhttp

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/openkcm/common-sdk/pkg/commoncfg"
)

func TestNewClientFromBasic(t *testing.T) {
	tests := []struct {
		name       string
		config     *commoncfg.BasicAuth
		wantErr    bool
		errMessage string
	}{
		{
			name:       "nil config",
			config:     nil,
			wantErr:    true,
			errMessage: "basic auth config is nil",
		},
		{
			name: "missing username",
			config: &commoncfg.BasicAuth{
				Password: commoncfg.SourceRef{Source: commoncfg.EmbeddedSourceValue, Value: "pass"},
			},
			wantErr:    true,
			errMessage: "basic credentials missing username",
		},
		{
			name: "missing password",
			config: &commoncfg.BasicAuth{
				Username: commoncfg.SourceRef{Source: commoncfg.EmbeddedSourceValue, Value: "user"},
			},
			wantErr:    true,
			errMessage: "basic credentials missing password",
		},
		{
			name: "valid basic auth",
			config: &commoncfg.BasicAuth{
				Username: commoncfg.SourceRef{Source: commoncfg.EmbeddedSourceValue, Value: "user"},
				Password: commoncfg.SourceRef{Source: commoncfg.EmbeddedSourceValue, Value: "pass"},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := NewClientFromBasic(tt.config)
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

func TestClientBasicRoundTripper(t *testing.T) {
	user, pass := "testuser", "testpass"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u, p, ok := r.BasicAuth()
		assert.True(t, ok)
		assert.Equal(t, user, u)
		assert.Equal(t, pass, p)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client, err := NewClientFromBasic(&commoncfg.BasicAuth{
		Username: commoncfg.SourceRef{Source: commoncfg.EmbeddedSourceValue, Value: user},
		Password: commoncfg.SourceRef{Source: commoncfg.EmbeddedSourceValue, Value: pass},
	})
	assert.NoError(t, err)

	req, err := http.NewRequest(http.MethodGet, server.URL, nil)
	assert.NoError(t, err)

	resp, err := client.Do(req)
	assert.NoError(t, err)

	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
}
