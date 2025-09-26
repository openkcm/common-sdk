package commoncfg_test

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"log/slog"
	"math/big"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/openkcm/common-sdk/pkg/commoncfg"
)

func generateValidTestCertificates(t *testing.T) ([]byte, []byte) {
	t.Helper()
	// Generate an RSA private key
	privKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	// Create certificate template
	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Organization: []string{"Test Org"},
			Country:      []string{"Test Country"},
		},
		NotBefore: time.Now(),
		NotAfter:  time.Now().Add(365 * 24 * time.Hour),
		KeyUsage:  x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
	}

	// Create certificate
	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &privKey.PublicKey, privKey)
	require.NoError(t, err)

	// Encode certificate to PEM
	certPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: certDER,
	})

	// Encode private key to PEM using PKCS#1 format (traditional RSA format)
	keyDER := x509.MarshalPKCS1PrivateKey(privKey)
	keyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: keyDER,
	})

	// Verify the generated certificate and key can be loaded
	_, err = tls.X509KeyPair(certPEM, keyPEM)
	require.NoError(t, err, "Generated certificate and key pair should be valid")

	return certPEM, keyPEM
}

func createTempCertFile(t *testing.T, content []byte, filename string) string {
	t.Helper()
	tmpFile, err := os.CreateTemp(t.TempDir(), filename)
	require.NoError(t, err)

	_, err = tmpFile.Write(content)
	require.NoError(t, err)
	require.NoError(t, tmpFile.Close())

	return tmpFile.Name()
}

func createTestCertFiles(t *testing.T) (string, string, string) {
	t.Helper()
	certPEM, keyPEM := generateValidTestCertificates(t)

	cert := createTempCertFile(t, certPEM, "cert-*.pem")
	key := createTempCertFile(t, keyPEM, "key-*.pem")
	ca := createTempCertFile(t, certPEM, "ca-*.pem")

	return cert, key, ca
}

func TestCertWatcherReloadAndCallback(t *testing.T) {
	cert, key, ca := createTestCertFiles(t)

	// Clean up files
	defer func() {
		err := os.Remove(cert)
		assert.NoError(t, err)
		err = os.Remove(key)
		assert.NoError(t, err)
		err = os.Remove(ca)
		assert.NoError(t, err)
	}()

	mtls := commoncfg.MTLS{
		Cert: commoncfg.SourceRef{
			Source: commoncfg.FileSourceValue,
			File:   commoncfg.CredentialFile{Path: cert, Format: commoncfg.BinaryFileFormat},
		},
		CertKey: commoncfg.SourceRef{
			Source: commoncfg.FileSourceValue,
			File:   commoncfg.CredentialFile{Path: key, Format: commoncfg.BinaryFileFormat},
		},
		ServerCA: commoncfg.SourceRef{
			Source: commoncfg.FileSourceValue,
			File:   commoncfg.CredentialFile{Path: ca, Format: commoncfg.BinaryFileFormat},
		},
	}

	logger := slog.Default()
	cw, err := commoncfg.NewCertWatcher(mtls, logger, nil)
	require.NoError(t, err)
	require.NotNil(t, cw)

	defer func() {
		err = cw.Close()
		assert.NoError(t, err)
	}()

	var (
		mu            sync.Mutex
		callbackCount int
		lastConfig    *tls.Config
		lastError     error
	)

	cw.RegisterCallback(func(cfg *tls.Config, err error) {
		mu.Lock()
		defer mu.Unlock()

		callbackCount++
		lastConfig = cfg
		lastError = err
	})

	mu.Lock()

	initialCount := callbackCount

	mu.Unlock()

	// Generate new certificates and update files
	newCertPEM, newKeyPEM := generateValidTestCertificates(t)
	require.NoError(t, os.WriteFile(cert, newCertPEM, 0644))
	require.NoError(t, os.WriteFile(key, newKeyPEM, 0600))

	// Wait for file watcher to detect changes
	time.Sleep(200 * time.Millisecond)

	mu.Lock()

	finalCount := callbackCount
	finalConfig := lastConfig
	finalError := lastError

	mu.Unlock()

	assert.GreaterOrEqual(t, finalCount, initialCount, "Callback should be called")

	if finalCount > initialCount {
		assert.NotNil(t, finalConfig, "TLS config should not be nil after reload")
		assert.NoError(t, finalError, "Callback error should be nil on successful reload")
	}
}
