package jwtsigning_test

import (
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"testing"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"

	"github.com/openkcm/common-sdk/pkg/jwtsigning"
)

func TestNewSigner(t *testing.T) {
	key := generateRSAKey(t, 3072)

	tests := []struct {
		name        string
		provider    jwtsigning.PrivateKeyProvider
		hasher      jwtsigning.Hasher
		expectError error
	}{
		{
			name:        "nil key provider returns ErrNilKeyProvider",
			provider:    nil,
			hasher:      &hasherStub{},
			expectError: jwtsigning.ErrNilKeyProvider,
		},
		{
			name: "nil hasher uses default SHA256 hasher",
			provider: &stubPrivateKeyProvider{
				key: key,
				meta: jwtsigning.KeyMetadata{
					Iss: "iss",
					Kid: "kid",
				},
			},
			hasher:      nil,
			expectError: nil,
		},
		{
			name: "custom hasher is accepted",
			provider: &stubPrivateKeyProvider{
				key:  key,
				meta: jwtsigning.KeyMetadata{},
			},
			hasher:      &hasherStub{hash: "ignored", alg: "IGNORED"},
			expectError: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			signer, err := jwtsigning.NewSigner(tt.provider, tt.hasher)

			if tt.expectError != nil {
				assert.Error(t, err)
				assert.Nil(t, signer)
				assert.ErrorIs(t, err, tt.expectError)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, signer)
			}
		})
	}
}

func TestSignerSign_SuccessClaimsAndHeaders(t *testing.T) {
	key := generateRSAKey(t, 3072)
	meta := jwtsigning.KeyMetadata{
		Iss: "https://issuer.example",
		Kid: "key-id-1",
	}

	hasher := &hasherStub{
		hash: "fixed-hash-value",
		alg:  "TEST-ALG",
	}

	provider := &stubPrivateKeyProvider{
		key:  key,
		meta: meta,
	}

	signer, err := jwtsigning.NewSigner(provider, hasher)
	assert.NoError(t, err)

	body := []byte("message-body")
	tokenStr, err := signer.Sign(t.Context(), body)
	assert.NoError(t, err)
	assert.NotEmpty(t, tokenStr)

	parsed, err := jwt.Parse(tokenStr, func(tkn *jwt.Token) (any, error) {
		return &key.PublicKey, nil
	})
	assert.NoError(t, err)
	assert.True(t, parsed.Valid)

	assert.Equal(t, "JWT", parsed.Header["typ"])
	assert.Equal(t, jwt.SigningMethodPS256.Alg(), parsed.Header["alg"])

	claims, ok := parsed.Claims.(jwt.MapClaims)
	if assert.True(t, ok) {
		assert.Equal(t, meta.Iss, claims["iss"])
		assert.Equal(t, meta.Kid, claims["kid"])
		assert.Equal(t, hasher.hash, claims["hash"])
		assert.Equal(t, hasher.alg, claims["hash-alg"])
	}
}

func TestSignerSign_KeyProviderErrorIsPropagated(t *testing.T) {
	expectedErr := errors.New("provider failed")

	provider := &stubPrivateKeyProvider{
		err: expectedErr,
	}

	signer, err := jwtsigning.NewSigner(provider, nil)
	assert.NoError(t, err)

	tokenStr, err := signer.Sign(t.Context(), []byte("body"))
	assert.Empty(t, tokenStr)
	assert.Error(t, err)
	assert.ErrorIs(t, err, expectedErr)
}

func TestSignerSign_FailsForTooSmallKey(t *testing.T) {
	smallKey := generateRSAKey(t, 1024) // < 3072 bits

	provider := &stubPrivateKeyProvider{
		key: smallKey,
		meta: jwtsigning.KeyMetadata{
			Iss: "issuer",
			Kid: "kid",
		},
	}

	signer, err := jwtsigning.NewSigner(provider, nil)
	assert.NoError(t, err)

	tokenStr, err := signer.Sign(t.Context(), []byte("body"))
	assert.Empty(t, tokenStr)
	assert.Error(t, err)
	assert.ErrorIs(t, err, jwtsigning.ErrRSAKeyLength)
}

func TestSignerSign_UsesDefaultSHA256Hasher(t *testing.T) {
	key := generateRSAKey(t, 3072)
	meta := jwtsigning.KeyMetadata{
		Iss: "default-iss",
		Kid: "default-kid",
	}
	body := []byte("some-body-data")

	provider := &stubPrivateKeyProvider{
		key:  key,
		meta: meta,
	}

	signer, err := jwtsigning.NewSigner(provider, nil)
	assert.NoError(t, err)

	tokenStr, err := signer.Sign(t.Context(), body)
	assert.NoError(t, err)
	assert.NotEmpty(t, tokenStr)

	parsed, err := jwt.Parse(tokenStr, func(tkn *jwt.Token) (any, error) {
		return &key.PublicKey, nil
	})
	assert.NoError(t, err)
	assert.True(t, parsed.Valid)

	claims, ok := parsed.Claims.(jwt.MapClaims)
	if assert.True(t, ok) {
		sum := sha256.Sum256(body)
		expectedHash := base64.RawURLEncoding.EncodeToString(sum[:])
		assert.Equal(t, expectedHash, claims["hash"])
		assert.Equal(t, signer.ToString(), claims["hash-alg"])
	}
}
