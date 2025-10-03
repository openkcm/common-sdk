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

// generateSelfSignedCert creates a temporary self-signed certificate and key for testing.
func generateSelfSignedCert(t *testing.T) ([]byte, []byte) {
	t.Helper()

	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("failed to generate key: %v", err)
	}

	certTmpl := x509.Certificate{
		SerialNumber: big.NewInt(1),
		NotBefore:    time.Now(),
		NotAfter:     time.Now().Add(time.Hour),
		Subject:      pkix.Name{CommonName: "localhost"},
		KeyUsage:     x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, &certTmpl, &certTmpl, &priv.PublicKey, priv)
	if err != nil {
		t.Fatalf("failed to create certificate: %v", err)
	}

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: derBytes})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(priv)})

	return certPEM, keyPEM
}

// writeTempFile writes content to a temporary file and returns its path.
func writeTempFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatalf("failed to write file %s: %v", path, err)
	}
	return path
}

// setupGRPCClientConfig prepares a GRPCClient configuration with optional mTLS.
func setupGRPCClientConfig(t *testing.T, tmpDir string, withSecretRef bool) *commoncfg.GRPCClient {
	t.Helper()
	cfg := &commoncfg.GRPCClient{
		Address: "localhost:12345",
	}

	if !withSecretRef {
		return cfg
	}

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

	return cfg
}

func TestDynamicClientConn(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		withSecretRef bool
		expectError   bool
	}{
		{"without SecretRef", false, false},
		{"with SecretRef", true, false},
	}

	for _, tt := range tests {
		tt := tt // capture range variable
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			tmpDir := t.TempDir()
			cfg := setupGRPCClientConfig(t, tmpDir, tt.withSecretRef)

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
	cfg := setupGRPCClientConfig(t, tmpDir, true)
	conn, err := commongrpc.NewDynamicClientConn(cfg, 50*time.Millisecond)
	if err != nil {
		t.Fatalf("failed to create DynamicClientConn: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })

	oldConn := conn.ClientConn

	// Overwrite certificate and key files
	certPEM, keyPEM := generateSelfSignedCert(t)
	writeTempFile(t, tmpDir, "tls.crt", string(certPEM))
	writeTempFile(t, tmpDir, "tls.key", string(keyPEM))

	time.Sleep(200 * time.Millisecond) // allow refresh

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

	for i := 0; i < 2; i++ {
		if err := conn.Close(); err != nil {
			t.Errorf("close #%d failed: %v", i+1, err)
		}
	}
}
