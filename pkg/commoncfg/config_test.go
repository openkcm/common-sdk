package commoncfg_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/openkcm/common-sdk/pkg/commoncfg"
)

func TestRetryMarshal(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input commoncfg.GRPCRetry
		want  map[string]any
	}{
		{
			name: "defaults",
			input: commoncfg.GRPCRetry{
				MaxAttempts:          3,
				InitialBackoff:       100 * time.Millisecond,
				MaxBackoff:           1 * time.Second,
				BackoffMultiplier:    2,
				RetryableStatusCodes: []string{"UNAVAILABLE", "RESOURCE_EXHAUSTED"},
			},
			want: map[string]any{
				"maxAttempts":          float64(3),
				"initialBackoff":       "0.100s",
				"maxBackoff":           "1.000s",
				"backoffMultiplier":    float64(2),
				"retryableStatusCodes": []any{"UNAVAILABLE", "RESOURCE_EXHAUSTED"},
			},
		},
		{
			name: "custom values",
			input: commoncfg.GRPCRetry{
				MaxAttempts:          5,
				InitialBackoff:       200 * time.Millisecond,
				MaxBackoff:           2 * time.Second,
				BackoffMultiplier:    3,
				RetryableStatusCodes: []string{"UNAVAILABLE"},
			},
			want: map[string]any{
				"maxAttempts":          float64(5),
				"initialBackoff":       "0.200s",
				"maxBackoff":           "2.000s",
				"backoffMultiplier":    float64(3),
				"retryableStatusCodes": []any{"UNAVAILABLE"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			b, err := json.Marshal(tt.input)
			require.NoError(t, err)

			var got map[string]any
			require.NoError(t, json.Unmarshal(b, &got))

			assert.Equal(t, tt.want, got)
		})
	}
}
