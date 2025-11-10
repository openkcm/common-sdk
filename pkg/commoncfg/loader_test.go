package commoncfg_test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/openkcm/common-sdk/v2/pkg/commoncfg"
)

type MyConfig struct {
	Key1 string      `yaml:"key1"`
	Key2 int         `yaml:"key2"`
	Sub  MySubConfig `yaml:"sub"`
}

type MySubConfig struct {
	Key3 string `yaml:"key3"`
}

func TestLoadConfig(t *testing.T) {
	tmpdir := t.TempDir()

	// create the test cases
	tests := []struct {
		name      string
		cfg       string
		defaults  map[string]any
		paths     []string
		wantError bool
		wantKey1  string
		wantKey2  int
	}{
		{
			name:      "zero values",
			wantError: true,
		}, {
			name:      "invalid config",
			cfg:       "foo: bar",
			paths:     []string{tmpdir},
			wantError: true,
		}, {
			name:      "valid config",
			cfg:       "key1: foo\nkey2: 42",
			paths:     []string{tmpdir},
			wantError: false,
			wantKey1:  "foo",
			wantKey2:  42,
		}, {
			name:      "valid config with default on key2",
			cfg:       "key1: foo",
			defaults:  map[string]any{"key2": 42},
			paths:     []string{tmpdir},
			wantError: false,
			wantKey1:  "foo",
			wantKey2:  42,
		}, {
			name:      "valid config with default on key1",
			cfg:       "key2: 42",
			defaults:  map[string]any{"key1": "foo"},
			paths:     []string{tmpdir},
			wantError: false,
			wantKey1:  "foo",
			wantKey2:  42,
		}, {
			name:      "valid config with defaults on both keys",
			cfg:       "",
			defaults:  map[string]any{"key1": "foo", "key2": 42},
			paths:     []string{tmpdir},
			wantError: false,
			wantKey1:  "foo",
			wantKey2:  42,
		}, {
			name:      "valid config with defaults on both keys and an overwrite on key1",
			cfg:       "key1: bar",
			defaults:  map[string]any{"key1": "foo", "key2": 42},
			paths:     []string{tmpdir},
			wantError: false,
			wantKey1:  "bar",
			wantKey2:  42,
		}, {
			name:      "valid config with defaults on both keys and an overwrite on key2",
			cfg:       "key2: 84",
			defaults:  map[string]any{"key1": "foo", "key2": 42},
			paths:     []string{tmpdir},
			wantError: false,
			wantKey1:  "foo",
			wantKey2:  84,
		}, {
			name:      "valid config with defaults on both keys and an overwrites on both keys",
			cfg:       "key1: bar\nkey2: 84",
			defaults:  map[string]any{"key1": "foo", "key2": 42},
			paths:     []string{tmpdir},
			wantError: false,
			wantKey1:  "bar",
			wantKey2:  84,
		},
	}

	// run the tests
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Arrange
			cfg := &MyConfig{}

			file := filepath.Join(tmpdir, "config.yaml")

			err := os.WriteFile(file, []byte(tc.cfg), 0o644)
			if err != nil {
				t.Fatalf("failed to create config file: %v", err)
			}

			// Act
			err = commoncfg.LoadConfig(cfg, tc.defaults, tc.paths...)

			// Assert
			if tc.wantError {
				if err == nil {
					t.Error("expected error, but got nil")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %s", err)
				} else {
					if cfg.Key1 != tc.wantKey1 {
						t.Errorf("expected Key1 to be %s, but got %s", tc.wantKey1, cfg.Key1)
					}

					if cfg.Key2 != tc.wantKey2 {
						t.Errorf("expected Key2 to be %d, but got %d", tc.wantKey2, cfg.Key2)
					}
				}
			}
		})
	}
}

func TestWithFile(t *testing.T) {
	tmpdir := t.TempDir()
	cfg := &MyConfig{}

	yamlCfg := "key1: foo"
	tests := []struct {
		name   string
		file   string
		format commoncfg.FileFormat
		cfg    string
		isErr  bool
	}{
		{
			name:   "Should use default file",
			file:   commoncfg.DefaultFileName,
			format: commoncfg.DefaultFileFormat,
			cfg:    yamlCfg,
		},
		{
			name:   "Should use file with different name",
			file:   "test",
			format: commoncfg.DefaultFileFormat,
			cfg:    yamlCfg,
		},
		{
			name:   "Should use file with different supported format",
			file:   "test",
			format: commoncfg.JSONFileFormat,
			cfg:    "{\"key1\": \"foo\"}",
		},
		{
			name:   "Should error on file with unsupported format",
			file:   "test",
			format: "test",
			isErr:  true,
		},
		{
			name:   "Should use default name on empty file name",
			file:   "",
			format: commoncfg.DefaultFileFormat,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fileName := "config"
			if tt.file != "" {
				fileName = tt.file
			}

			file := filepath.Join(
				tmpdir,
				fmt.Sprintf("%s.%s", fileName, tt.format),
			)

			err := os.WriteFile(file, []byte(tt.cfg), 0o644)
			assert.NoError(t, err)

			loader := commoncfg.NewLoader(cfg, commoncfg.WithPaths(tmpdir), commoncfg.WithFile(tt.file, tt.format))
			err = loader.LoadConfig()

			if tt.isErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestWithEnvOverride(t *testing.T) {
	// Arrange
	config := "key1: value1\nkey2: 42\nsub:\n  key3: value3"
	tmpdir := t.TempDir()
	cfg := &MyConfig{}

	file := filepath.Join(tmpdir, "config.yaml")

	err := os.WriteFile(file, []byte(config), 0o644)
	if err != nil {
		t.Fatalf("failed to create config file: %v", err)
	}

	tests := []struct {
		name          string
		prefix        string
		envKey        string
		envValue      string
		expectedValue string
	}{
		{
			name:          "env with prefix overrides key3",
			prefix:        "TEST",
			envKey:        "TEST_SUB_KEY3",
			envValue:      "value2",
			expectedValue: "value2",
		},
		{
			name:          "env without defined prefix overrides key3",
			prefix:        "",
			envKey:        "SUB_KEY3",
			envValue:      "value2",
			expectedValue: "value2",
		},
		{
			name:          "env without prefix does not override key3",
			prefix:        "TEST",
			envKey:        "SUB_KEY3",
			envValue:      "value2",
			expectedValue: "value3",
		},
		{
			name:          "env with incorrect prefix does not override key3",
			prefix:        "TEST",
			envKey:        "FOO_SUB_KEY3",
			envValue:      "value2",
			expectedValue: "value3",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// Set environment variable
			t.Setenv(test.envKey, test.envValue)

			// Act
			loader := commoncfg.NewLoader(cfg,
				commoncfg.WithEnvOverride(test.prefix),
				commoncfg.WithPaths(tmpdir))
			err := loader.LoadConfig()

			// Assert
			assert.NoError(t, err)
			assert.Equal(t, test.expectedValue, cfg.Sub.Key3)
		})
	}
}

func TestLoadValueFromSourceRef(t *testing.T) {
	t.Run("loads embedded value", func(t *testing.T) {
		ref := commoncfg.SourceRef{
			Source: commoncfg.EmbeddedSourceValue,
			Value:  "secret-value",
		}
		val, err := commoncfg.ExtractValueFromSourceRef(&ref)
		assert.NoError(t, err)
		assert.Equal(t, []byte("secret-value"), val)
	})

	t.Run("loads value from env", func(t *testing.T) {
		t.Setenv("MY_SECRET", "env-secret")

		ref := commoncfg.SourceRef{
			Source: commoncfg.EnvSourceValue,
			Env:    "MY_SECRET",
		}
		val, err := commoncfg.ExtractValueFromSourceRef(&ref)
		assert.NoError(t, err)
		assert.Equal(t, []byte("env-secret"), val)
	})

	t.Run("errors if env var is not set", func(t *testing.T) {
		ref := commoncfg.SourceRef{
			Source: commoncfg.EnvSourceValue,
			Env:    "UNDEFINED_ENV",
		}
		_, err := commoncfg.ExtractValueFromSourceRef(&ref)
		assert.Error(t, err)
	})

	t.Run("loads JSON value from file", func(t *testing.T) {
		tmpFile := createTempFile(t, `{"token": "json-secret"}`)

		defer func() {
			err := os.Remove(tmpFile)
			require.NoError(t, err)
		}()

		ref := commoncfg.SourceRef{
			Source: commoncfg.FileSourceValue,
			File: commoncfg.CredentialFile{
				Path:     tmpFile,
				Format:   commoncfg.JSONFileFormat,
				JSONPath: "$.token",
			},
		}
		val, err := commoncfg.ExtractValueFromSourceRef(&ref)
		assert.NoError(t, err)
		assert.Equal(t, []byte("json-secret"), val)
	})

	t.Run("errors on invalid JSONPath", func(t *testing.T) {
		ref := commoncfg.SourceRef{
			Source: commoncfg.FileSourceValue,
			File: commoncfg.CredentialFile{
				Path:     "invalidPath",
				Format:   commoncfg.JSONFileFormat,
				JSONPath: "$.missing",
			},
		}
		_, err := commoncfg.ExtractValueFromSourceRef(&ref)
		assert.Error(t, err)
	})

	t.Run("loads binary data from file", func(t *testing.T) {
		tmpFile := createTempFile(t, "binary-data")

		defer func() {
			err := os.Remove(tmpFile)
			require.NoError(t, err)
		}()

		ref := commoncfg.SourceRef{
			Source: commoncfg.FileSourceValue,
			File: commoncfg.CredentialFile{
				Path:   tmpFile,
				Format: commoncfg.BinaryFileFormat,
			},
		}
		val, err := commoncfg.ExtractValueFromSourceRef(&ref)
		assert.NoError(t, err)
		assert.Equal(t, []byte("binary-data"), val)
	})

	t.Run("errors on unknown source", func(t *testing.T) {
		ref := commoncfg.SourceRef{
			Source: "unknown",
		}
		_, err := commoncfg.ExtractValueFromSourceRef(&ref)
		assert.Error(t, err)
	})
}

func createTempFile(t *testing.T, content string) string {
	t.Helper()
	tmpFile, err := os.CreateTemp(t.TempDir(), "cred-*.json")
	require.NoError(t, err)

	_, err = tmpFile.WriteString(content)
	require.NoError(t, err)
	require.NoError(t, tmpFile.Close())

	return tmpFile.Name()
}
