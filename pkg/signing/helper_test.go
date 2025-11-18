package signing_test

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"testing"

	"github.com/openkcm/common-sdk/pkg/signing"
)

type stubPrivateKeyProvider struct {
	key  *rsa.PrivateKey
	meta signing.KeyMetadata
	err  error
}

func (s *stubPrivateKeyProvider) CurrentSigningKey(ctx context.Context) (*rsa.PrivateKey, signing.KeyMetadata, error) {
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
	if err != nil {
		tb.Fatalf("generateRSAKey: %v", err)
	}

	return key
}

func signMessage(tb testing.TB, provider signing.PrivateKeyProvider, hasher signing.Hasher, body []byte) string {
	tb.Helper()

	signer, err := signing.NewSigner(provider, hasher)
	if err != nil {
		tb.Fatalf("NewSigner failed: %v", err)
	}

	token, err := signer.Sign(context.Background(), body)
	if err != nil {
		tb.Fatalf("Sign failed: %v", err)
	}

	return token
}
