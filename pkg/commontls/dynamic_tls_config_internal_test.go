package commontls

import (
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/mldsa"
	"crypto/rsa"
	"crypto/x509"
	"math/big"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsFIPSAllowedChain(t *testing.T) {
	t.Parallel()

	// Only the key type and size matter, so the keys need not be usable.
	rsaKey := func(bits uint) *rsa.PublicKey {
		return &rsa.PublicKey{N: new(big.Int).Lsh(big.NewInt(1), bits-1), E: 65537}
	}
	cert := func(key any) *x509.Certificate {
		return &x509.Certificate{PublicKey: key}
	}

	tests := []struct {
		name  string
		chain []*x509.Certificate
		want  bool
	}{
		{name: "RSA 2048", chain: []*x509.Certificate{cert(rsaKey(2048))}, want: true},
		{name: "RSA 4096", chain: []*x509.Certificate{cert(rsaKey(4096))}, want: true},
		{name: "RSA 1024", chain: []*x509.Certificate{cert(rsaKey(1024))}},
		{name: "ECDSA P-256", chain: []*x509.Certificate{cert(&ecdsa.PublicKey{Curve: elliptic.P256()})}, want: true},
		{name: "ECDSA P-384", chain: []*x509.Certificate{cert(&ecdsa.PublicKey{Curve: elliptic.P384()})}, want: true},
		{name: "ECDSA P-521", chain: []*x509.Certificate{cert(&ecdsa.PublicKey{Curve: elliptic.P521()})}, want: true},
		{name: "ECDSA P-224", chain: []*x509.Certificate{cert(&ecdsa.PublicKey{Curve: elliptic.P224()})}},
		{name: "Ed25519", chain: []*x509.Certificate{cert(make(ed25519.PublicKey, ed25519.PublicKeySize))}, want: true},
		{name: "ML-DSA", chain: []*x509.Certificate{cert(&mldsa.PublicKey{})}, want: true},
		{name: "unknown key type", chain: []*x509.Certificate{cert("not a key")}},
		{name: "empty chain"},
		{
			name: "one disallowed certificate in the chain",
			chain: []*x509.Certificate{
				cert(&ecdsa.PublicKey{Curve: elliptic.P256()}),
				cert(rsaKey(1024)),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, isFIPSAllowedChain(tt.chain))
		})
	}
}
