package utils_test

import (
	"encoding/base64"
	"reflect"
	"testing"

	"github.com/openkcm/common-sdk/pkg/utils"
)

func TestExtractFromComplexValue(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		want      string
		wantError bool
	}{
		{
			name:  "plain string returns same",
			input: "hello",
			want:  "hello",
		},
		{
			name: "single base64 decoding",
			input: func() string {
				payload := base64.StdEncoding.EncodeToString([]byte("decoded"))
				return utils.Base64Token + "(" + payload + ")"
			}(),
			want: "decoded",
		},
		{
			name: "nested base64 decoding",
			input: func() string {
				inner := base64.StdEncoding.EncodeToString([]byte("final"))
				mid := utils.Base64Token + "(" + inner + ")"
				outer := utils.Base64Token + "(" + base64.StdEncoding.EncodeToString([]byte(mid)) + ")"

				return outer
			}(),
			want: "final",
		},
		{
			name:  "unknown wrapper returns as is",
			input: "unknown(payload)",
			want:  "unknown(payload)",
		},
		{
			name:      "invalid base64 returns error",
			input:     utils.Base64Token + "(!!invalid!!)",
			wantError: true,
		},
		{
			name:  "empty base64 returns empty string",
			input: utils.Base64Token + "()",
			want:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := utils.ExtractFromComplexValue(tt.input)
			if (err != nil) != tt.wantError {
				t.Fatalf("ExtractFromComplexValue() error = %v, wantError %v", err, tt.wantError)
			}

			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ExtractFromComplexValue() = %v, want %v", got, tt.want)
			}
		})
	}
}
