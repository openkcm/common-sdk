package jwtsigning_test

import (
	"errors"
	"testing"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"

	"github.com/openkcm/common-sdk/pkg/jwtsigning"
)

func TestVerifier(t *testing.T) {
	defaultTrustMap := map[string]struct{}{
		"issuer":                 {},
		"https://issuer.example": {},
		"trusted-issuer":         {},
	}

	t.Run("NewVerifier", func(t *testing.T) {
		key := generateRSAKey(t, 3072)

		tests := []struct {
			name        string
			pubProvider jwtsigning.PublicKeyProvider
			hasher      jwtsigning.Hasher
			expectError error
		}{
			{
				name:        "nil public key provider returns ErrNilPublicKeyProvider",
				pubProvider: nil,
				hasher:      &hasherStub{},
				expectError: jwtsigning.ErrNilPublicKeyProvider,
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
				verifier, err := jwtsigning.NewVerifier(tt.pubProvider, tt.hasher, defaultTrustMap)

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
		meta := jwtsigning.KeyMetadata{
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

		trustMap := map[string]struct{}{meta.Iss: {}}
		verifier, err := jwtsigning.NewVerifier(pubProvider, nil, trustMap)
		assert.NoError(t, err)

		err = verifier.Verify(t.Context(), token, body)
		assert.NoError(t, err)

		assert.Equal(t, meta.Iss, pubProvider.lastIss)
		assert.Equal(t, meta.Kid, pubProvider.lastKid)
	})

	t.Run("Verify_TrustedIssuersBehavior", func(t *testing.T) {
		key := generateRSAKey(t, 3072)
		meta := jwtsigning.KeyMetadata{
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
				name:           "issuer in trust list is accepted",
				trustedIssuers: map[string]struct{}{meta.Iss: {}},
				expectErr:      nil,
			},
			{
				name:           "issuer not in trust list is rejected",
				trustedIssuers: map[string]struct{}{"other-issuer": {}},
				expectErr:      jwtsigning.ErrUntrustedIssuer,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				verifier, err := jwtsigning.NewVerifier(pubProvider, nil, tt.trustedIssuers)
				assert.NoError(t, err)

				err = verifier.Verify(t.Context(), token, body)

				if tt.expectErr == nil {
					assert.NoError(t, err)
				} else {
					assert.Error(t, err)
					assert.ErrorIs(t, err, jwtsigning.ErrJWTParseFailed)
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

		verifier, err := jwtsigning.NewVerifier(pubProvider, nil, defaultTrustMap)
		assert.NoError(t, err)

		err = verifier.Verify(t.Context(), "not-a-jwt", []byte("body"))
		assert.Error(t, err)
		assert.ErrorIs(t, err, jwtsigning.ErrJWTParseFailed)
	})

	t.Run("Verify_MissingIssOrKid", func(t *testing.T) {
		key := generateRSAKey(t, 3072)
		body := []byte("body")

		tests := []struct {
			name string
			meta jwtsigning.KeyMetadata
		}{
			{
				name: "missing iss",
				meta: jwtsigning.KeyMetadata{
					Iss: "",
					Kid: "kid",
				},
			},
			{
				name: "missing kid",
				meta: jwtsigning.KeyMetadata{
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

				// FIX: Pass valid map
				verifier, err := jwtsigning.NewVerifier(pubProvider, nil, defaultTrustMap)
				assert.NoError(t, err)

				err = verifier.Verify(t.Context(), token, body)
				assert.Error(t, err)
				assert.ErrorIs(t, err, jwtsigning.ErrJWTParseFailed)
				assert.ErrorIs(t, err, jwtsigning.ErrMissingIssOrKid)
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

		verifier, err := jwtsigning.NewVerifier(&stubPublicKeyProvider{}, nil, defaultTrustMap)
		assert.NoError(t, err)

		err = verifier.Verify(t.Context(), tokenStr, []byte("body"))
		assert.Error(t, err)
		assert.ErrorIs(t, err, jwtsigning.ErrJWTParseFailed)
		assert.ErrorIs(t, err, jwtsigning.ErrUnexpectedSigningMethod)
	})

	t.Run("Verify_PublicKeyProviderErrorIsPropagated", func(t *testing.T) {
		key := generateRSAKey(t, 3072)
		meta := jwtsigning.KeyMetadata{
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

		trustMap := map[string]struct{}{meta.Iss: {}}
		verifier, err := jwtsigning.NewVerifier(pubProvider, nil, trustMap)
		assert.NoError(t, err)

		err = verifier.Verify(t.Context(), token, body)
		assert.Error(t, err)
		assert.ErrorIs(t, err, jwtsigning.ErrJWTParseFailed)
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

		verifier, err := jwtsigning.NewVerifier(pubProvider, nil, defaultTrustMap)
		assert.NoError(t, err)

		err = verifier.Verify(t.Context(), tokenStr, []byte("body"))
		assert.Error(t, err)
		assert.ErrorIs(t, err, jwtsigning.ErrJWTParseFailed)
		assert.ErrorIs(t, err, jwtsigning.ErrRSAKeyLength)
	})

	t.Run("Verify_HashAndAlgFailures", func(t *testing.T) {
		key := generateRSAKey(t, 3072)
		meta := jwtsigning.KeyMetadata{
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
				expectedErr: jwtsigning.ErrUnsupportedHashAlgorithm,
			},
			{
				name: "hash claim missing",
				hasher: &hasherStub{
					hash: "",
					alg:  "SHA256",
				},
				expectedErr: jwtsigning.ErrHashClaimMissing,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				token := signMessage(t, &stubPrivateKeyProvider{
					key:  key,
					meta: meta,
				}, tt.hasher, body)

				trustMap := map[string]struct{}{meta.Iss: {}}
				verifier, err := jwtsigning.NewVerifier(pubProvider, nil, trustMap)
				assert.NoError(t, err)

				err = verifier.Verify(t.Context(), token, body)
				assert.Error(t, err)
				assert.ErrorIs(t, err, tt.expectedErr)
			})
		}
	})

	t.Run("Verify_HashMismatch", func(t *testing.T) {
		key := generateRSAKey(t, 3072)
		meta := jwtsigning.KeyMetadata{
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

		trustMap := map[string]struct{}{meta.Iss: {}}
		verifier, err := jwtsigning.NewVerifier(pubProvider, nil, trustMap)
		assert.NoError(t, err)

		err = verifier.Verify(t.Context(), token, tamperedBody)
		assert.Error(t, err)
		assert.ErrorIs(t, err, jwtsigning.ErrMessageHashMismatch)
	})
}
