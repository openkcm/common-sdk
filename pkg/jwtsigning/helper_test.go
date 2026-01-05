package jwtsigning_test

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"testing"

	"github.com/stretchr/testify/assert"

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
