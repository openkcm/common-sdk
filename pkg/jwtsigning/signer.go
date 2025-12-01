package jwtsigning

import (
	"context"
	"errors"
	"fmt"

	"github.com/golang-jwt/jwt/v5"
)

var (
	ErrRSAKeyLength              = errors.New("RSA key is too small")
	ErrUndefinedHashingAlgorithm = errors.New("undefined hashing algorithm")
	ErrUndefinedSigningAlgorithm = errors.New("undefined signing algorithm")
	ErrNilKeyProvider            = errors.New("keyProvider cannot be nil")
)

const (
	jwtMapClaimIss           = "iss"
	jwtMapClaimKid           = "kid"
	jwtMapClaimHash          = "hash"
	jwtMapClaimHashAlgorithm = "hash-alg"

	tokenHeaderType      = "typ"
	tokenType            = "JWT"
	tokenHeaderAlgorithm = "alg"
)

// Signer signs message bodies into JWS (JWT) tokens.
type Signer struct {
	Hasher

	keys PrivateKeyProvider
}

func NewSigner(keyProvider PrivateKeyProvider, hasher Hasher) (*Signer, error) {
	if keyProvider == nil {
		return nil, ErrNilKeyProvider
	}

	if hasher == nil {
		hasher = &SHA256Hasher{}
	}

	return &Signer{
		Hasher: hasher,
		keys:   keyProvider,
	}, nil
}

// Sign creates a compact JWS (JWT) for the given message body using PS256.
//
// The returned string is suitable for use as a value of an HTTP or message
// header (e.g. "X-Message-Signature"). The token will contain the following
// claims and headers:
//   - claims:
//     iss:      issuer from KeyMetadata
//     kid:      key ID from KeyMetadata
//     hash:     base64url(SHA-256(body))
//     hash-alg: "SHA256"
//   - headers:
//     typ: "JWT"
//     alg: "PS256"
//
// Sign obtains the private key and metadata from the configured
// PrivateKeyProvider and enforces the minimum RSA key size before signing.
// It returns an error if the provider fails, if the key is too small, or if
// the token cannot be signed.
func (s *Signer) Sign(ctx context.Context, body []byte) (string, error) {
	priv, meta, err := s.keys.CurrentSigningKey(ctx)
	if err != nil {
		return "", err
	}

	if priv.N.BitLen() < 3072 {
		return "", fmt.Errorf("%w: %d bits", ErrRSAKeyLength, priv.N.BitLen())
	}

	claims := jwt.MapClaims{
		jwtMapClaimIss:           meta.Iss,
		jwtMapClaimKid:           meta.Kid,
		jwtMapClaimHash:          s.HashMessage(body),
		jwtMapClaimHashAlgorithm: s.ToString(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodPS256, claims)
	token.Header[tokenHeaderType] = tokenType
	token.Header[tokenHeaderAlgorithm] = jwt.SigningMethodPS256.Alg()

	return token.SignedString(priv)
}
