package jwtsigning_test

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"math/big"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/openkcm/common-sdk/pkg/jwtsigning"
)

type stubPrivateKeyProvider struct {
	key  *rsa.PrivateKey
	meta jwtsigning.KeyMetadata
	err  error
}

func (s *stubPrivateKeyProvider) CurrentSigningKey(ctx context.Context) (*rsa.PrivateKey, jwtsigning.KeyMetadata, error) {
	return s.key, s.meta, s.err
}

type stubPublicKeyProvider struct {
	key     *rsa.PublicKey
	err     error
	lastIss string
	lastKid string
}

func (s *stubPublicKeyProvider) VerificationKey(ctx context.Context, iss, kid string) (*rsa.PublicKey, error) {
	s.lastIss = iss

	s.lastKid = kid
	if s.err != nil {
		return nil, s.err
	}

	return s.key, nil
}

type hasherStub struct {
	hash string
	alg  string
}

func (h *hasherStub) HashMessage(_ []byte) string {
	return h.hash
}

func (h *hasherStub) ToString() string {
	return h.alg
}

func generateRSAKey(tb testing.TB, bits int) *rsa.PrivateKey {
	tb.Helper()

	key, err := rsa.GenerateKey(rand.Reader, bits)
	assert.NoError(tb, err, "generateRSAKey", err)

	return key
}

func signMessage(tb testing.TB, provider jwtsigning.PrivateKeyProvider, hasher jwtsigning.Hasher, body []byte) string {
	tb.Helper()

	signer, err := jwtsigning.NewSigner(provider, hasher)
	assert.NoError(tb, err, "NewSigner failed", err)

	token, err := signer.Sign(tb.Context(), body)
	assert.NoError(tb, err, "Sign failed", err)

	return token
}

func rootCertificate(t *testing.T, notBefore time.Time, notAfter time.Time) (*rsa.PrivateKey, []byte) {
	t.Helper()

	rootKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	rootTmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "Root CA", Organization: []string{"Root Organization"}},
		NotBefore:             notBefore,
		NotAfter:              notAfter,
		IsCA:                  true,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
	}

	rootDer, err := x509.CreateCertificate(
		rand.Reader,
		rootTmpl,
		rootTmpl,
		rootKey.Public(),
		rootKey)
	require.NoError(t, err)

	return rootKey, rootDer
}

func intermediateCertificate(t *testing.T, rootDer []byte, notBefore time.Time, notAfter time.Time, rootKey *rsa.PrivateKey) (*rsa.PrivateKey, []byte) {
	t.Helper()

	rootCert, err := x509.ParseCertificate(rootDer)
	require.NoError(t, err)

	intKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	intTmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(2),
		Subject:               pkix.Name{CommonName: "Intermediate CA", Organization: []string{"Intermediate Organization"}},
		NotBefore:             notBefore,
		NotAfter:              notAfter,
		IsCA:                  true,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
	}
	intDer, err := x509.CreateCertificate(rand.Reader,
		intTmpl,
		rootCert,
		intKey.Public(),
		rootKey)
	require.NoError(t, err)

	return intKey, intDer
}

func leafCertWithParams(t *testing.T, intDer []byte, notBefore time.Time, notAfter time.Time, intKey *rsa.PrivateKey, subject pkix.Name) []byte {
	t.Helper()

	intCert, err := x509.ParseCertificate(intDer)
	require.NoError(t, err)

	leafKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	leafTmpl := &x509.Certificate{
		SerialNumber: big.NewInt(3),
		Subject:      subject,
		NotBefore:    notBefore,
		NotAfter:     notAfter,
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
	}

	leafDer, err := x509.CreateCertificate(rand.Reader,
		leafTmpl,
		intCert,
		leafKey.Public(),
		intKey)
	require.NoError(t, err)

	return leafDer
}
