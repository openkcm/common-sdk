package signing_test

import (
	"context"
	"errors"
	"testing"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"

	"github.com/openkcm/common-sdk/pkg/signing"
)

func TestVerifier(t *testing.T) {
	t.Run("NewVerifier", func(t *testing.T) {
		key := generateRSAKey(t, 3072)

		tests := []struct {
			name        string
			pubProvider signing.PublicKeyProvider
			hasher      signing.Hasher
			expectError error
		}{
			{
				name:        "nil public key provider returns ErrNilPublicKeyProvider",
				pubProvider: nil,
				hasher:      &hasherStub{},
				expectError: signing.ErrNilPublicKeyProvider,
			},
			{
				name: "nil hasher uses default SHA256 hasher",
				pubProvider: &stubPublicKeyProvider{
					key: &key.PublicKey,
				},
				hasher:      nil,
				expectError: nil,
			},
			{
				name: "custom hasher is accepted",
				pubProvider: &stubPublicKeyProvider{
					key: &key.PublicKey,
				},
				hasher:      &hasherStub{hash: "ignored", alg: "IGNORED"},
				expectError: nil,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				verifier, err := signing.NewVerifier(tt.pubProvider, tt.hasher, nil)

				if tt.expectError != nil {
					assert.Error(t, err)
					assert.Nil(t, verifier)
					assert.ErrorIs(t, err, tt.expectError)
				} else {
					assert.NoError(t, err)
					assert.NotNil(t, verifier)
				}
			})
		}
	})

	t.Run("Verify_Success_ResolvesKeyAndValidatesHash", func(t *testing.T) {
		key := generateRSAKey(t, 3072)
		meta := signing.KeyMetadata{
			Iss: "https://issuer.example",
			Kid: "kid-123",
		}
		body := []byte("verified-message")

		privProvider := &stubPrivateKeyProvider{
			key:  key,
			meta: meta,
		}

		token := signMessage(t, privProvider, nil, body)

		pubProvider := &stubPublicKeyProvider{
			key: &key.PublicKey,
		}

		verifier, err := signing.NewVerifier(pubProvider, nil, nil)
		assert.NoError(t, err)

		err = verifier.Verify(context.Background(), token, body)
		assert.NoError(t, err)

		assert.Equal(t, meta.Iss, pubProvider.lastIss)
		assert.Equal(t, meta.Kid, pubProvider.lastKid)
	})

	t.Run("Verify_TrustedIssuersBehavior", func(t *testing.T) {
		key := generateRSAKey(t, 3072)
		meta := signing.KeyMetadata{
			Iss: "trusted-issuer",
			Kid: "kid-1",
		}
		body := []byte("body")

		token := signMessage(t, &stubPrivateKeyProvider{
			key:  key,
			meta: meta,
		}, nil, body)

		pubProvider := &stubPublicKeyProvider{
			key: &key.PublicKey,
		}

		tests := []struct {
			name           string
			trustedIssuers map[string]struct{}
			expectErr      error
		}{
			{
				name:           "no trust list accepts any issuer",
				trustedIssuers: nil,
				expectErr:      nil,
			},
			{
				name:           "empty trust list accepts any issuer",
				trustedIssuers: map[string]struct{}{},
				expectErr:      nil,
			},
			{
				name:           "issuer in trust list is accepted",
				trustedIssuers: map[string]struct{}{meta.Iss: {}},
				expectErr:      nil,
			},
			{
				name:           "issuer not in trust list is rejected",
				trustedIssuers: map[string]struct{}{"other-issuer": {}},
				expectErr:      signing.ErrUntrustedIssuer,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				verifier, err := signing.NewVerifier(pubProvider, nil, tt.trustedIssuers)
				assert.NoError(t, err)

				err = verifier.Verify(context.Background(), token, body)

				if tt.expectErr == nil {
					assert.NoError(t, err)
				} else {
					assert.Error(t, err)
					assert.ErrorIs(t, err, signing.ErrJWTParseFailed)
					assert.ErrorIs(t, err, tt.expectErr)
				}
			})
		}
	})

	t.Run("Verify_InvalidTokenString", func(t *testing.T) {
		key := generateRSAKey(t, 3072)

		pubProvider := &stubPublicKeyProvider{
			key: &key.PublicKey,
		}

		verifier, err := signing.NewVerifier(pubProvider, nil, nil)
		assert.NoError(t, err)

		err = verifier.Verify(context.Background(), "not-a-jwt", []byte("body"))
		assert.Error(t, err)
		assert.ErrorIs(t, err, signing.ErrJWTParseFailed)
	})

	t.Run("Verify_MissingIssOrKid", func(t *testing.T) {
		key := generateRSAKey(t, 3072)
		body := []byte("body")

		tests := []struct {
			name string
			meta signing.KeyMetadata
		}{
			{
				name: "missing iss",
				meta: signing.KeyMetadata{
					Iss: "",
					Kid: "kid",
				},
			},
			{
				name: "missing kid",
				meta: signing.KeyMetadata{
					Iss: "issuer",
					Kid: "",
				},
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				token := signMessage(t, &stubPrivateKeyProvider{
					key:  key,
					meta: tt.meta,
				}, nil, body)

				pubProvider := &stubPublicKeyProvider{
					key: &key.PublicKey,
				}

				verifier, err := signing.NewVerifier(pubProvider, nil, nil)
				assert.NoError(t, err)

				err = verifier.Verify(context.Background(), token, body)
				assert.Error(t, err)
				assert.ErrorIs(t, err, signing.ErrJWTParseFailed)
				assert.ErrorIs(t, err, signing.ErrMissingIssOrKid)
			})
		}
	})

	t.Run("Verify_UnexpectedSigningMethod", func(t *testing.T) {
		claims := jwt.MapClaims{
			"iss":      "issuer",
			"kid":      "kid",
			"hash":     "h",
			"hash-alg": "SHA256",
		}

		token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
		tokenStr, err := token.SignedString([]byte("secret"))
		assert.NoError(t, err)

		verifier, err := signing.NewVerifier(&stubPublicKeyProvider{}, nil, nil)
		assert.NoError(t, err)

		err = verifier.Verify(context.Background(), tokenStr, []byte("body"))
		assert.Error(t, err)
		assert.ErrorIs(t, err, signing.ErrJWTParseFailed)
		assert.ErrorIs(t, err, signing.ErrUnexpectedSigningMethod)
	})

	t.Run("Verify_PublicKeyProviderErrorIsPropagated", func(t *testing.T) {
		key := generateRSAKey(t, 3072)
		meta := signing.KeyMetadata{
			Iss: "issuer",
			Kid: "kid",
		}
		body := []byte("body")

		token := signMessage(t, &stubPrivateKeyProvider{
			key:  key,
			meta: meta,
		}, nil, body)

		providerErr := errors.New("verification key lookup failed")
		pubProvider := &stubPublicKeyProvider{
			key: nil,
			err: providerErr,
		}

		verifier, err := signing.NewVerifier(pubProvider, nil, nil)
		assert.NoError(t, err)

		err = verifier.Verify(context.Background(), token, body)
		assert.Error(t, err)
		assert.ErrorIs(t, err, signing.ErrJWTParseFailed)
		assert.ErrorIs(t, err, providerErr)
	})

	t.Run("Verify_FailsForTooSmallPublicKey", func(t *testing.T) {
		smallKey := generateRSAKey(t, 1024)

		claims := jwt.MapClaims{
			"iss":      "issuer",
			"kid":      "kid",
			"hash":     "h",
			"hash-alg": "SHA256",
		}

		token := jwt.NewWithClaims(jwt.SigningMethodPS256, claims)
		tokenStr, err := token.SignedString(smallKey)
		assert.NoError(t, err)

		pubProvider := &stubPublicKeyProvider{
			key: &smallKey.PublicKey,
		}

		verifier, err := signing.NewVerifier(pubProvider, nil, nil)
		assert.NoError(t, err)

		err = verifier.Verify(context.Background(), tokenStr, []byte("body"))
		assert.Error(t, err)
		assert.ErrorIs(t, err, signing.ErrJWTParseFailed)
		assert.ErrorIs(t, err, signing.ErrRSAKeyLength)
	})

	t.Run("Verify_HashAndAlgFailures", func(t *testing.T) {
		key := generateRSAKey(t, 3072)
		meta := signing.KeyMetadata{
			Iss: "issuer",
			Kid: "kid",
		}
		body := []byte("body")

		pubProvider := &stubPublicKeyProvider{
			key: &key.PublicKey,
		}

		tests := []struct {
			name        string
			hasher      *hasherStub
			expectedErr error
		}{
			{
				name: "unsupported hash algorithm",
				hasher: &hasherStub{
					hash: "some-hash",
					alg:  "OTHER-ALG",
				},
				expectedErr: signing.ErrUnsupportedHashAlgorithm,
			},
			{
				name: "hash claim missing",
				hasher: &hasherStub{
					hash: "",
					alg:  "SHA256",
				},
				expectedErr: signing.ErrHashClaimMissing,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				token := signMessage(t, &stubPrivateKeyProvider{
					key:  key,
					meta: meta,
				}, tt.hasher, body)

				verifier, err := signing.NewVerifier(pubProvider, nil, nil)
				assert.NoError(t, err)

				err = verifier.Verify(context.Background(), token, body)
				assert.Error(t, err)
				assert.ErrorIs(t, err, tt.expectedErr)
			})
		}
	})

	t.Run("Verify_HashMismatch", func(t *testing.T) {
		key := generateRSAKey(t, 3072)
		meta := signing.KeyMetadata{
			Iss: "issuer",
			Kid: "kid",
		}

		originalBody := []byte("original-body")
		tamperedBody := []byte("tampered-body")

		token := signMessage(t, &stubPrivateKeyProvider{
			key:  key,
			meta: meta,
		}, nil, originalBody)

		pubProvider := &stubPublicKeyProvider{
			key: &key.PublicKey,
		}

		verifier, err := signing.NewVerifier(pubProvider, nil, nil)
		assert.NoError(t, err)

		err = verifier.Verify(context.Background(), token, tamperedBody)
		assert.Error(t, err)
		assert.ErrorIs(t, err, signing.ErrMessageHashMismatch)
	})
}
