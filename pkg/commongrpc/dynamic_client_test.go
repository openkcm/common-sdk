package commongrpc_test

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/openkcm/common-sdk/pkg/commoncfg"
	"github.com/openkcm/common-sdk/pkg/commongrpc"
)

func generateSelfSignedCert(t *testing.T) ([]byte, []byte) {
	t.Helper()

	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("failed to generate key: %v", err)
	}

	tmpl := x509.Certificate{
		SerialNumber: big.NewInt(1),
		NotBefore:    time.Now(),
		NotAfter:     time.Now().Add(time.Hour),
		Subject:      pkix.Name{CommonName: "localhost"},
		KeyUsage:     x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, &tmpl, &tmpl, &priv.PublicKey, priv)
	if err != nil {
		t.Fatalf("failed to create certificate: %v", err)
	}

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: derBytes})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(priv)})

	return certPEM, keyPEM
}

func writeTempFile(t *testing.T, dir, name, content string) string {
	t.Helper()

	path := filepath.Join(dir, name)

	err := os.WriteFile(path, []byte(content), 0600)
	if err != nil {
		t.Fatalf("failed to write file %s: %v", path, err)
	}

	return path
}

func TestDynamicClientConn(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		withSecretRef bool
		expectError   bool
	}{
		{
			name:          "without SecretRef",
			withSecretRef: false,
		},
		{
			name:          "with SecretRef",
			withSecretRef: true,
		},
	}

	for _, tt := range tests {
		// capture range variable
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			tmpDir := t.TempDir()
			cfg := &commoncfg.GRPCClient{
				Address: "localhost:12345",
			}

			if tt.withSecretRef {
				certPEM, keyPEM := generateSelfSignedCert(t)
				certFile := writeTempFile(t, tmpDir, "tls.crt", string(certPEM))
				keyFile := writeTempFile(t, tmpDir, "tls.key", string(keyPEM))

				cfg.SecretRef = &commoncfg.SecretRef{
					Type: commoncfg.MTLSSecretType,
					MTLS: commoncfg.MTLS{
						Cert: commoncfg.SourceRef{
							Source: commoncfg.FileSourceValue,
							File: commoncfg.CredentialFile{
								Path:   certFile,
								Format: commoncfg.BinaryFileFormat,
							},
						},
						CertKey: commoncfg.SourceRef{
							Source: commoncfg.FileSourceValue,
							File: commoncfg.CredentialFile{
								Path:   keyFile,
								Format: commoncfg.BinaryFileFormat,
							},
						},
					},
				}
			}

			conn, err := commongrpc.NewDynamicClientConn(cfg, 50*time.Millisecond)
			if tt.expectError && err == nil {
				t.Fatal("expected error, got none")
			}

			if err != nil && !tt.expectError {
				t.Fatalf("unexpected error: %v", err)
			}

			if conn != nil {
				t.Cleanup(func() { _ = conn.Close() })

				if conn.ClientConn == nil {
					t.Error("expected ClientConn to be initialized")
				}
			}
		})
	}
}

func TestDynamicClientConnRefreshOnCertChange(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	certPEM, keyPEM := generateSelfSignedCert(t)
	certFile := writeTempFile(t, tmpDir, "tls.crt", string(certPEM))
	keyFile := writeTempFile(t, tmpDir, "tls.key", string(keyPEM))

	cfg := &commoncfg.GRPCClient{
		Address: "localhost:12345",
		SecretRef: &commoncfg.SecretRef{
			Type: commoncfg.MTLSSecretType,
			MTLS: commoncfg.MTLS{
				Cert: commoncfg.SourceRef{
					Source: commoncfg.FileSourceValue,
					File: commoncfg.CredentialFile{
						Path:   certFile,
						Format: commoncfg.BinaryFileFormat,
					},
				},
				CertKey: commoncfg.SourceRef{
					Source: commoncfg.FileSourceValue,
					File: commoncfg.CredentialFile{
						Path:   keyFile,
						Format: commoncfg.BinaryFileFormat,
					},
				},
			},
		},
	}

	conn, err := commongrpc.NewDynamicClientConn(cfg, 50*time.Millisecond)
	if err != nil {
		t.Fatalf("failed to create DynamicClientConn: %v", err)
	}

	t.Cleanup(func() { _ = conn.Close() })

	oldConn := conn.ClientConn

	certPEM, keyPEM = generateSelfSignedCert(t)
	_ = writeTempFile(t, tmpDir, "tls.crt", string(certPEM))
	_ = writeTempFile(t, tmpDir, "tls.key", string(keyPEM))

	time.Sleep(200 * time.Millisecond) // let refresh trigger

	if conn.ClientConn == nil {
		t.Fatal("expected ClientConn to be refreshed")
	}

	if conn.ClientConn == oldConn {
		t.Error("expected ClientConn to be different after refresh")
	}
}

func TestDynamicClientConnCloseIdempotent(t *testing.T) {
	t.Parallel()

	cfg := &commoncfg.GRPCClient{Address: "localhost:12345"}

	conn, err := commongrpc.NewDynamicClientConn(cfg, 50*time.Millisecond)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	err = conn.Close()
	if err != nil {
		t.Errorf("first close failed: %v", err)
	}

	err = conn.Close()
	if err != nil {
		t.Errorf("second close failed: %v", err)
	}
}
