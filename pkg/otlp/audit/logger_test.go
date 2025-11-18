package otlpaudit

import (
	"encoding/base64"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"

	"github.com/openkcm/common-sdk/pkg/commoncfg"
)

const (
	testFilesDir  = "../../../internal/otlp/audit/testdata"
	testConfigDir = "testConfigs"
	config        = "config.yaml"
	envPrefix     = "AUDIT"

	correctBasicAuthCfg        = "correctBasicAuthConfig.yaml"
	incorrectBasicAuthCfg      = "incorrectBasicAuthConfig.yaml"
	incorrectBasicAuthCfgCreds = "incorrectBasicAuthConfigCreds.yaml"
	correctMTLSCfg             = "correctMTLSConfig.yaml"
	incorrectMTLSCfg           = "incorrectMTLSConfig.yaml"
)

func TestSend(t *testing.T) {
	tests := []struct {
		name        string
		configPath  string
		isBasicAuth bool
		serverError error
		expectError error
	}{
		{
			name:        "T300_BasicAuth_Success",
			configPath:  filepath.Join(testFilesDir, testConfigDir, correctBasicAuthCfg),
			isBasicAuth: true,
			expectError: nil,
		},
		{
			name:        "T301_BasicAuth_Fail",
			configPath:  filepath.Join(testFilesDir, testConfigDir, incorrectBasicAuthCfgCreds),
			isBasicAuth: true,
			expectError: errStatusNotOK,
		},
		{
			name:        "T302_CorrectMTLS_Success",
			configPath:  filepath.Join(testFilesDir, testConfigDir, correctMTLSCfg),
			isBasicAuth: false,
			expectError: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if tt.isBasicAuth {
					auth := r.Header.Get("Authorization")

					expectedAuth := "Basic " + base64.StdEncoding.EncodeToString([]byte("admin:admin"))
					if auth != expectedAuth {
						w.WriteHeader(http.StatusUnauthorized)
						return
					}
				}

				w.WriteHeader(http.StatusOK)
			}))
			defer server.Close()

			err := createTempConfigFrom(tt.configPath)

			defer removeTempConfig()

			cfg, err := loadConfigForTests(t, err)
			if err != nil {
				t.Error(err)
			}

			cfg.Audit.Endpoint = server.URL
			logs := plog.NewLogs()
			logs.ResourceLogs().AppendEmpty().ScopeLogs().AppendEmpty().LogRecords().AppendEmpty()

			auditLogger, _ := NewLogger(&cfg.Audit)
			err = auditLogger.SendEvent(t.Context(), logs)

			if (tt.expectError != nil && !errors.Is(err, tt.expectError)) || (err == nil && tt.expectError != nil) {
				t.Errorf("Expected error '%v', got '%v'", tt.expectError, err)
			}

			if err != nil && tt.expectError == nil {
				t.Errorf("Expected no error, got '%v'", err)
			}
		})
	}
}

func Test_NewLogger(t *testing.T) {
	tests := []struct {
		name        string
		configPath  string
		isBasicAuth bool
		serverError error
		expectError error
	}{
		{
			name:        "T200_BasicAuth_Success",
			configPath:  filepath.Join(testFilesDir, testConfigDir, correctBasicAuthCfg),
			isBasicAuth: true,
			expectError: nil,
		},
		{
			name:        "T201_BasicAuth_Fail",
			configPath:  filepath.Join(testFilesDir, testConfigDir, incorrectBasicAuthCfg),
			isBasicAuth: true,
			expectError: errLoadValue,
		},
		{
			name:        "T202_CorrectMTLS_Success",
			configPath:  filepath.Join(testFilesDir, testConfigDir, correctMTLSCfg),
			isBasicAuth: false,
			expectError: nil,
		},
		{
			name:        "T203_IncorrectMTLS_Fail",
			configPath:  filepath.Join(testFilesDir, testConfigDir, incorrectMTLSCfg),
			isBasicAuth: false,
			expectError: errLoadMTLSConfigFailed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := createTempConfigFrom(tt.configPath)

			defer removeTempConfig()

			cfg, err := loadConfigForTests(t, err)
			if err != nil {
				t.Error(err)
			}

			_, err = NewLogger(&cfg.Audit)
			if (tt.expectError != nil && !errors.Is(err, tt.expectError)) || (err == nil && tt.expectError != nil) {
				t.Errorf("Expected error '%v', got '%v'", tt.expectError, err)
			}

			if err != nil && tt.expectError == nil {
				t.Errorf("Expected no error, got '%v'", err)
			}
		})
	}
}

func Test_EnrichLogs(t *testing.T) {
	auditCfg := commoncfg.Audit{
		Endpoint:             "http://localhost:1234/logs",
		MTLS:                 nil,
		BasicAuth:            nil,
		AdditionalProperties: "Prop1: Val1\nProp2: Val2",
	}
	logs := plog.NewLogs()
	logs.ResourceLogs().AppendEmpty().ScopeLogs().AppendEmpty().LogRecords().AppendEmpty()

	auditLogger, _ := NewLogger(&auditCfg)

	err := auditLogger.enrichLogs(&logs)
	if err != nil {
		t.Errorf("Unexpected error '%v'", err)
	}

	attrs := logs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Attributes()
	if !valuesPresent(attrs, "Prop1", "Prop2") {
		t.Errorf("Missing expected attributes")
	}
}

func valuesPresent(m pcommon.Map, keys ...string) bool {
	for _, k := range keys {
		_, ok := m.Get(k)
		if !ok {
			return false
		}
	}

	return true
}

func loadConfigForTests(t *testing.T, err error) (*commoncfg.BaseConfig, error) {
	t.Helper()

	defaults := map[string]any{}

	if err != nil {
		t.Errorf("createTempConfigFrom() error = %v", err)
	}

	cfg := &commoncfg.BaseConfig{}

	err = commoncfg.LoadConfig(cfg, defaults, envPrefix, testFilesDir)
	if err != nil {
		t.Errorf("Expected no error, got '%v'", err)
	}

	return cfg, err
}

func createTempConfigFrom(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	tempFile, err := os.Create(filepath.Join(testFilesDir, config))
	if err != nil {
		return err
	}

	defer func(tempFile *os.File) {
		err := tempFile.Close()
		if err != nil {
			panic("Failed to close temp file")
		}
	}(tempFile)

	_, err = tempFile.Write(data)
	if err != nil {
		return err
	}

	return nil
}

func removeTempConfig() {
	err := os.Remove(filepath.Join(testFilesDir, config))
	if err != nil {
		return
	}
}
