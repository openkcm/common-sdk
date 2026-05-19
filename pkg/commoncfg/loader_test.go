package commoncfg_test

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/openkcm/common-sdk/pkg/commoncfg"
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

// generateTestCert creates a self-signed certificate and returns certPEM, keyPEM.
func generateTestCert(t *testing.T) ([]byte, []byte) {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "test"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	require.NoError(t, err)

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})

	keyDER, err := x509.MarshalECPrivateKey(key)
	require.NoError(t, err)

	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})

	return certPEM, keyPEM
}

func TestDisableViperErrorUnused(t *testing.T) {
	tmpdir := t.TempDir()
	cfg := &MyConfig{}

	// Config has an extra key not present in MyConfig — with ErrorUnused disabled this should succeed.
	err := os.WriteFile(filepath.Join(tmpdir, "config.yaml"), []byte("key1: foo\nextra: ignored"), 0o644)
	require.NoError(t, err)

	loader := commoncfg.NewLoader(cfg,
		commoncfg.WithPaths(tmpdir),
		commoncfg.DisableViperErrorUnused(),
	)
	assert.NoError(t, loader.LoadConfig())
}

func TestLoadValueFromSourceRef_DelegatesToExtract(t *testing.T) {
	t.Run("delegates to ExtractValueFromSourceRef", func(t *testing.T) {
		ref := commoncfg.SourceRef{
			Source: commoncfg.EmbeddedSourceValue,
			Value:  "hello",
		}
		val, err := commoncfg.LoadValueFromSourceRef(ref)
		require.NoError(t, err)
		assert.Equal(t, []byte("hello"), val)
	})
}

func TestExtractValueFromSourceRef_NilCredential(t *testing.T) {
	_, err := commoncfg.ExtractValueFromSourceRef(nil)
	assert.Error(t, err)
}

func TestExtractValueFromSourceRef_YAMLFile(t *testing.T) {
	tmpFile := createTempFile(t, "key: value")

	ref := commoncfg.SourceRef{
		Source: commoncfg.FileSourceValue,
		File: commoncfg.CredentialFile{
			Path:   tmpFile,
			Format: commoncfg.YAMLFileFormat,
		},
	}
	val, err := commoncfg.ExtractValueFromSourceRef(&ref)
	require.NoError(t, err)
	assert.Equal(t, []byte("key: value"), val)
}

func TestExtractValueFromSourceRef_JSONFileNoPath(t *testing.T) {
	tmpFile := createTempFile(t, `{"token": "abc"}`)

	ref := commoncfg.SourceRef{
		Source: commoncfg.FileSourceValue,
		File: commoncfg.CredentialFile{
			Path:   tmpFile,
			Format: commoncfg.JSONFileFormat,
		},
	}
	val, err := commoncfg.ExtractValueFromSourceRef(&ref)
	require.NoError(t, err)
	assert.JSONEq(t, `{"token": "abc"}`, string(val))
}

func TestExtractValueFromSourceRef_JSONFileInvalidJSON(t *testing.T) {
	tmpFile := createTempFile(t, "not-json")

	ref := commoncfg.SourceRef{
		Source: commoncfg.FileSourceValue,
		File: commoncfg.CredentialFile{
			Path:     tmpFile,
			Format:   commoncfg.JSONFileFormat,
			JSONPath: "$.token",
		},
	}
	_, err := commoncfg.ExtractValueFromSourceRef(&ref)
	assert.Error(t, err)
}

func TestExtractValueFromSourceRef_JSONFileNonStringResult(t *testing.T) {
	tmpFile := createTempFile(t, `{"count": 42}`)

	ref := commoncfg.SourceRef{
		Source: commoncfg.FileSourceValue,
		File: commoncfg.CredentialFile{
			Path:     tmpFile,
			Format:   commoncfg.JSONFileFormat,
			JSONPath: "$.count",
		},
	}
	_, err := commoncfg.ExtractValueFromSourceRef(&ref)
	assert.Error(t, err)
}

func TestLoadMTLSClientCertificate(t *testing.T) {
	certPEM, keyPEM := generateTestCert(t)

	t.Run("nil config returns error", func(t *testing.T) {
		_, err := commoncfg.LoadMTLSClientCertificate(nil)
		assert.ErrorIs(t, err, commoncfg.ErrMTLSIsNil)
	})

	t.Run("valid cert and key", func(t *testing.T) {
		cert, err := commoncfg.LoadMTLSClientCertificate(&commoncfg.MTLS{
			Cert:    commoncfg.SourceRef{Source: commoncfg.EmbeddedSourceValue, Value: string(certPEM)},
			CertKey: commoncfg.SourceRef{Source: commoncfg.EmbeddedSourceValue, Value: string(keyPEM)},
		})
		require.NoError(t, err)
		assert.NotNil(t, cert)
	})

	t.Run("cert extraction error", func(t *testing.T) {
		_, err := commoncfg.LoadMTLSClientCertificate(&commoncfg.MTLS{
			Cert:    commoncfg.SourceRef{Source: "unknown"},
			CertKey: commoncfg.SourceRef{Source: commoncfg.EmbeddedSourceValue, Value: string(keyPEM)},
		})
		assert.Error(t, err)
	})

	t.Run("key extraction error", func(t *testing.T) {
		_, err := commoncfg.LoadMTLSClientCertificate(&commoncfg.MTLS{
			Cert:    commoncfg.SourceRef{Source: commoncfg.EmbeddedSourceValue, Value: string(certPEM)},
			CertKey: commoncfg.SourceRef{Source: "unknown"},
		})
		assert.Error(t, err)
	})

	t.Run("invalid key pair", func(t *testing.T) {
		otherCertPEM, _ := generateTestCert(t)
		_, err := commoncfg.LoadMTLSClientCertificate(&commoncfg.MTLS{
			Cert:    commoncfg.SourceRef{Source: commoncfg.EmbeddedSourceValue, Value: string(otherCertPEM)},
			CertKey: commoncfg.SourceRef{Source: commoncfg.EmbeddedSourceValue, Value: string(keyPEM)},
		})
		assert.Error(t, err)
	})
}

func TestLoadMTLSCACertPool(t *testing.T) {
	certPEM, _ := generateTestCert(t)

	t.Run("nil config returns error", func(t *testing.T) {
		_, err := commoncfg.LoadMTLSCACertPool(nil)
		assert.ErrorIs(t, err, commoncfg.ErrMTLSIsNil)
	})

	t.Run("empty CAs returns nil pool", func(t *testing.T) {
		pool, err := commoncfg.LoadMTLSCACertPool(&commoncfg.MTLS{})
		require.NoError(t, err)
		assert.Nil(t, pool)
	})

	t.Run("with ServerCA", func(t *testing.T) {
		serverCA := commoncfg.SourceRef{Source: commoncfg.EmbeddedSourceValue, Value: string(certPEM)}
		pool, err := commoncfg.LoadMTLSCACertPool(&commoncfg.MTLS{
			ServerCA: &serverCA,
		})
		require.NoError(t, err)
		assert.NotNil(t, pool)
	})

	t.Run("with RootCAs", func(t *testing.T) {
		pool, err := commoncfg.LoadMTLSCACertPool(&commoncfg.MTLS{
			RootCAs: []commoncfg.SourceRef{
				{Source: commoncfg.EmbeddedSourceValue, Value: string(certPEM)},
			},
		})
		require.NoError(t, err)
		assert.NotNil(t, pool)
	})

	t.Run("invalid CA source returns error", func(t *testing.T) {
		serverCA := commoncfg.SourceRef{Source: "unknown"}
		_, err := commoncfg.LoadMTLSCACertPool(&commoncfg.MTLS{
			ServerCA: &serverCA,
		})
		assert.Error(t, err)
	})
}

func TestLoadCACertPool(t *testing.T) {
	certPEM, _ := generateTestCert(t)

	t.Run("nil ref returns nil pool", func(t *testing.T) {
		pool, err := commoncfg.LoadCACertPool(nil)
		require.NoError(t, err)
		assert.Nil(t, pool)
	})

	t.Run("valid cert ref", func(t *testing.T) {
		ref := &commoncfg.SourceRef{Source: commoncfg.EmbeddedSourceValue, Value: string(certPEM)}
		pool, err := commoncfg.LoadCACertPool(ref)
		require.NoError(t, err)
		assert.NotNil(t, pool)
	})

	t.Run("extraction error", func(t *testing.T) {
		ref := &commoncfg.SourceRef{Source: "unknown"}
		_, err := commoncfg.LoadCACertPool(ref)
		assert.Error(t, err)
	})
}

func TestLoadCAsCertPool(t *testing.T) {
	certPEM, _ := generateTestCert(t)

	t.Run("empty slice returns nil pool", func(t *testing.T) {
		pool, err := commoncfg.LoadCAsCertPool(nil)
		require.NoError(t, err)
		assert.Nil(t, pool)
	})

	t.Run("valid certs", func(t *testing.T) {
		refs := []commoncfg.SourceRef{
			{Source: commoncfg.EmbeddedSourceValue, Value: string(certPEM)},
		}
		pool, err := commoncfg.LoadCAsCertPool(refs)
		require.NoError(t, err)
		assert.NotNil(t, pool)
	})

	t.Run("extraction error", func(t *testing.T) {
		refs := []commoncfg.SourceRef{
			{Source: "unknown"},
		}
		_, err := commoncfg.LoadCAsCertPool(refs)
		assert.Error(t, err)
	})
}

func TestLoadClientCertificate(t *testing.T) {
	certPEM, keyPEM := generateTestCert(t)

	t.Run("nil certRef returns error", func(t *testing.T) {
		keyRef := &commoncfg.SourceRef{Source: commoncfg.EmbeddedSourceValue, Value: string(keyPEM)}
		_, err := commoncfg.LoadClientCertificate(nil, keyRef)
		assert.ErrorIs(t, err, commoncfg.ErrCertificateIsNil)
	})

	t.Run("nil keyRef returns error", func(t *testing.T) {
		certRef := &commoncfg.SourceRef{Source: commoncfg.EmbeddedSourceValue, Value: string(certPEM)}
		_, err := commoncfg.LoadClientCertificate(certRef, nil)
		assert.ErrorIs(t, err, commoncfg.ErrKeyCertificateIsNil)
	})

	t.Run("valid cert and key", func(t *testing.T) {
		certRef := &commoncfg.SourceRef{Source: commoncfg.EmbeddedSourceValue, Value: string(certPEM)}
		keyRef := &commoncfg.SourceRef{Source: commoncfg.EmbeddedSourceValue, Value: string(keyPEM)}
		cert, err := commoncfg.LoadClientCertificate(certRef, keyRef)
		require.NoError(t, err)
		assert.NotNil(t, cert)
	})

	t.Run("cert extraction error", func(t *testing.T) {
		certRef := &commoncfg.SourceRef{Source: "unknown"}
		keyRef := &commoncfg.SourceRef{Source: commoncfg.EmbeddedSourceValue, Value: string(keyPEM)}
		_, err := commoncfg.LoadClientCertificate(certRef, keyRef)
		assert.Error(t, err)
	})

	t.Run("key extraction error", func(t *testing.T) {
		certRef := &commoncfg.SourceRef{Source: commoncfg.EmbeddedSourceValue, Value: string(certPEM)}
		keyRef := &commoncfg.SourceRef{Source: "unknown"}
		_, err := commoncfg.LoadClientCertificate(certRef, keyRef)
		assert.Error(t, err)
	})

	t.Run("invalid key pair", func(t *testing.T) {
		otherCertPEM, _ := generateTestCert(t)
		certRef := &commoncfg.SourceRef{Source: commoncfg.EmbeddedSourceValue, Value: string(otherCertPEM)}
		keyRef := &commoncfg.SourceRef{Source: commoncfg.EmbeddedSourceValue, Value: string(keyPEM)}
		_, err := commoncfg.LoadClientCertificate(certRef, keyRef)
		assert.Error(t, err)
	})
}

func TestLoadMTLSConfig(t *testing.T) {
	certPEM, keyPEM := generateTestCert(t)

	validMTLS := &commoncfg.MTLS{
		Cert:    commoncfg.SourceRef{Source: commoncfg.EmbeddedSourceValue, Value: string(certPEM)},
		CertKey: commoncfg.SourceRef{Source: commoncfg.EmbeddedSourceValue, Value: string(keyPEM)},
	}

	t.Run("nil config returns error", func(t *testing.T) {
		_, err := commoncfg.LoadMTLSConfig(nil)
		assert.ErrorIs(t, err, commoncfg.ErrMTLSIsNil)
	})

	t.Run("valid config without CA", func(t *testing.T) {
		tlsCfg, err := commoncfg.LoadMTLSConfig(validMTLS)
		require.NoError(t, err)
		assert.NotNil(t, tlsCfg)
		assert.Nil(t, tlsCfg.RootCAs)
	})

	t.Run("valid config with TLS attributes", func(t *testing.T) {
		mtls := &commoncfg.MTLS{
			Cert:    commoncfg.SourceRef{Source: commoncfg.EmbeddedSourceValue, Value: string(certPEM)},
			CertKey: commoncfg.SourceRef{Source: commoncfg.EmbeddedSourceValue, Value: string(keyPEM)},
			Attributes: &commoncfg.TLSAttributes{
				InsecureSkipVerify:     true,
				ServerName:             "example.com",
				SessionTicketsDisabled: true,
			},
		}
		tlsCfg, err := commoncfg.LoadMTLSConfig(mtls)
		require.NoError(t, err)
		assert.True(t, tlsCfg.InsecureSkipVerify)
		assert.Equal(t, "example.com", tlsCfg.ServerName)
		assert.True(t, tlsCfg.SessionTicketsDisabled)
	})

	t.Run("valid config with CA", func(t *testing.T) {
		caRef := commoncfg.SourceRef{Source: commoncfg.EmbeddedSourceValue, Value: string(certPEM)}
		mtls := &commoncfg.MTLS{
			Cert:     commoncfg.SourceRef{Source: commoncfg.EmbeddedSourceValue, Value: string(certPEM)},
			CertKey:  commoncfg.SourceRef{Source: commoncfg.EmbeddedSourceValue, Value: string(keyPEM)},
			ServerCA: &caRef,
		}
		tlsCfg, err := commoncfg.LoadMTLSConfig(mtls)
		require.NoError(t, err)
		assert.NotNil(t, tlsCfg.RootCAs)
	})

	t.Run("cert load error propagates", func(t *testing.T) {
		mtls := &commoncfg.MTLS{
			Cert:    commoncfg.SourceRef{Source: "unknown"},
			CertKey: commoncfg.SourceRef{Source: commoncfg.EmbeddedSourceValue, Value: string(keyPEM)},
		}
		_, err := commoncfg.LoadMTLSConfig(mtls)
		assert.Error(t, err)
	})

	t.Run("CA load error propagates", func(t *testing.T) {
		invalidCA := commoncfg.SourceRef{Source: "unknown"}
		mtls := &commoncfg.MTLS{
			Cert:     commoncfg.SourceRef{Source: commoncfg.EmbeddedSourceValue, Value: string(certPEM)},
			CertKey:  commoncfg.SourceRef{Source: commoncfg.EmbeddedSourceValue, Value: string(keyPEM)},
			ServerCA: &invalidCA,
		}
		_, err := commoncfg.LoadMTLSConfig(mtls)
		assert.Error(t, err)
	})
}
