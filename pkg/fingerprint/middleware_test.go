package fingerprint

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestFingerprintCtxMiddlewareAndExtractFingerprint(t *testing.T) {
	builder := NewBuilder(WithHeaderKeys([]string{"user-agent"}))

	tests := []struct {
		name           string
		headers        map[string][]string
		expectError    bool
		expectedFPFunc func(*http.Request) (string, error)
	}{
		{
			name:        "no headers",
			headers:     map[string][]string{},
			expectError: false,
			expectedFPFunc: func(r *http.Request) (string, error) {
				return builder.FromHTTPRequest(r)
			},
		},
		{
			name: "with headers",
			headers: map[string][]string{
				"User-Agent": {"Foo"},
				"Accept":     {"Bar"},
			},
			expectError: false,
			expectedFPFunc: func(r *http.Request) (string, error) {
				return builder.FromHTTPRequest(r)
			},
		},
		{
			name: "missing accept header",
			headers: map[string][]string{
				"User-Agent": {"AgentX"},
			},
			expectError: false,
			expectedFPFunc: func(r *http.Request) (string, error) {
				return builder.FromHTTPRequest(r)
			},
		},
		{
			name: "multiple values for headers",
			headers: map[string][]string{
				"User-Agent": {"A", "B"},
				"Accept":     {"C", "D"},
			},
			expectError: false,
			expectedFPFunc: func(r *http.Request) (string, error) {
				return builder.FromHTTPRequest(r)
			},
		},
		{
			name: "case-insensitive header keys",
			headers: map[string][]string{
				"user-agent": {"foo"},
				"ACCEPT":     {"bar"},
			},
			expectError: false,
			expectedFPFunc: func(r *http.Request) (string, error) {
				return builder.FromHTTPRequest(r)
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", nil)

			for k, vs := range tc.headers {
				for _, v := range vs {
					req.Header.Add(k, v)
				}
			}

			rr := httptest.NewRecorder()

			var gotFP string

			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				fp, err := ExtractFingerprint(r.Context())
				if tc.expectError && err == nil {
					t.Error("expected error, got nil")
				}

				if !tc.expectError && err != nil {
					t.Errorf("unexpected error: %v", err)
				}

				gotFP = fp
			})

			mw := FingerprintCtxMiddleware(handler)
			mw.ServeHTTP(rr, req)

			wantFP, _ := tc.expectedFPFunc(req)
			if gotFP != wantFP {
				t.Errorf("expected fingerprint %s, got %s", wantFP, gotFP)
			}
		})
	}
}

func TestExtractFingerprint_NoFingerprint(t *testing.T) {
	ctx := context.Background()

	_, err := ExtractFingerprint(ctx)
	if err == nil {
		t.Error("expected error when fingerprint is missing in context")
	}
}

func TestExtractFingerprint_ManualContext(t *testing.T) {
	ctx := context.WithValue(context.Background(), fingerprintKey, "manual-fingerprint")

	fp, err := ExtractFingerprint(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if fp != "manual-fingerprint" {
		t.Errorf("expected fingerprint 'manual-fingerprint', got %s", fp)
	}
}
