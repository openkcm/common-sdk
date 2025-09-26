package certs_test

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/openkcm/common-sdk/pkg/commonfs/certs"
	"github.com/openkcm/common-sdk/pkg/storage/keyvalue"
)

// generateCertKeyPEM returns a valid matching certificate and private key PEM.
func generateCertKeyPEM(t *testing.T) ([]byte, []byte) {
	t.Helper()

	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	template := &x509.Certificate{
		SerialNumber:          bigInt(t),
		Subject:               pkix.Name{CommonName: "test"},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &priv.PublicKey, priv)
	require.NoError(t, err)

	certPEMBlock := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})
	keyBytes, err := x509.MarshalECPrivateKey(priv)
	require.NoError(t, err)

	keyPEMBlock := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyBytes})

	return certPEMBlock, keyPEMBlock
}

func bigInt(t *testing.T) *big.Int {
	t.Helper()

	n, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	require.NoError(t, err)

	return n
}

// --- Tests ---

func TestTLSConfigWatcherGet(t *testing.T) {
	tests := []struct {
		name      string
		setup     func(st *keyvalue.MemoryStorage[string, []byte])
		expectErr string
	}{
		{
			name: "Positive valid cert+key",
			setup: func(st *keyvalue.MemoryStorage[string, []byte]) {
				certPEM, keyPEM := generateCertKeyPEM(t)
				st.Store(certs.DefaultCertFilename, certPEM)
				st.Store(certs.DefaultKeyFilename, keyPEM)
			},
			expectErr: "",
		},
		{
			name: "Missing cert",
			setup: func(st *keyvalue.MemoryStorage[string, []byte]) {
				certPEM, _ := generateCertKeyPEM(t)
				st.Store(certs.DefaultKeyFilename, certPEM)
			},
			expectErr: "no value found for tls.crt",
		},
		{
			name: "Missing key",
			setup: func(st *keyvalue.MemoryStorage[string, []byte]) {
				certPEM, _ := generateCertKeyPEM(t)
				st.Store(certs.DefaultCertFilename, certPEM)
			},
			expectErr: "no value found for tls.key",
		},
		{
			name: "Invalid key",
			setup: func(st *keyvalue.MemoryStorage[string, []byte]) {
				certPEM, _ := generateCertKeyPEM(t)
				st.Store(certs.DefaultCertFilename, certPEM)
				st.Store(certs.DefaultKeyFilename, []byte("invalidkey"))
			},
			expectErr: "failed to find any PEM data",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			w, err := certs.NewTLSConfigWatcher(dir)
			require.NoError(t, err)

			st, ok := w.Storage().(*keyvalue.MemoryStorage[string, []byte])
			require.True(t, ok)
			tc.setup(st)

			cfg, err := w.Get()
			if tc.expectErr != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), tc.expectErr)
				require.Nil(t, cfg)
			} else {
				require.NoError(t, err)
				require.NotNil(t, cfg)
				require.Len(t, cfg.Certificates, 1)
				require.IsType(t, &x509.CertPool{}, cfg.RootCAs)
				require.Equal(t, uint16(tls.VersionTLS12), cfg.MinVersion)
				require.Equal(t, uint16(tls.VersionTLS13), cfg.MaxVersion)
			}
		})
	}
}

func TestTLSConfigWatcherIntegrationFileWatcher(t *testing.T) {
	dir := t.TempDir()

	certPEM, keyPEM := generateCertKeyPEM(t)
	caPEM := certPEM

	// Write files to temp directory
	err := os.WriteFile(filepath.Join(dir, certs.DefaultCertFilename), certPEM, 0600)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(dir, certs.DefaultKeyFilename), keyPEM, 0600)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(dir, certs.DefaultCAFilename), caPEM, 0600)
	require.NoError(t, err)

	// Create TLSConfigWatcher
	w, err := certs.NewTLSConfigWatcher(dir)
	require.NoError(t, err)

	// Start watching (loader will pick up existing files)
	err = w.StartWatching()
	require.NoError(t, err)

	defer func() {
		require.NoError(t, w.StopWatching())
	}()

	// Wait a short moment for watcher to load files
	time.Sleep(200 * time.Millisecond)

	cfg, err := w.Get()
	require.NoError(t, err)
	require.NotNil(t, cfg)
	require.Len(t, cfg.Certificates, 1)
	require.IsType(t, &x509.CertPool{}, cfg.RootCAs)
	require.Equal(t, uint16(tls.VersionTLS12), cfg.MinVersion)
	require.Equal(t, uint16(tls.VersionTLS13), cfg.MaxVersion)
}
