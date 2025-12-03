package jwtsigning

import (
	"context"
	"crypto/subtle"
	"errors"
	"fmt"

	"github.com/golang-jwt/jwt/v5"
)

var (
	ErrUnexpectedClaimsType     = errors.New("unexpected claims type")
	ErrUnexpectedSigningMethod  = errors.New("unexpected signing method")
	ErrMissingIssOrKid          = errors.New("missing iss or kid in token")
	ErrUntrustedIssuer          = errors.New("untrusted issuer")
	ErrJWTParseFailed           = errors.New("jwt parse failed")
	ErrUnsupportedHashAlgorithm = errors.New("unsupported hash algorithm")
	ErrSignatureInvalid         = errors.New("jwt signature invalid")
	ErrHashClaimMissing         = errors.New("missing hash claim")
	ErrMessageHashMismatch      = errors.New("message hash mismatch")
	ErrNilPublicKeyProvider     = errors.New("publicKeyProvider cannot be nil")
	ErrNoTrustedIssuers         = errors.New("trusted issuers cannot be nil")
)

// Verifier verifies signed messages represented as compact JWS (JWT) tokens.
type Verifier struct {
	Hasher

	// keys resolves public keys for given (iss, kid) pairs. It must be non-nil.
	keys PublicKeyProvider

	// trustedIssuers optionally restricts which issuers are accepted. If the
	// map is non-empty, the verifier will only accept tokens whose "iss"
	// claim appears as a key in this map. If it is nil or empty, all issuers
	// resolved by Keys are accepted.
	trustedIssuers map[string]struct{}
}

func NewVerifier(publicKeyProvider PublicKeyProvider, hasher Hasher, trustedIssuers map[string]struct{}) (*Verifier, error) {
	if publicKeyProvider == nil {
		return nil, ErrNilPublicKeyProvider
	}

	if hasher == nil {
		hasher = &SHA256Hasher{}
	}

	if len(trustedIssuers) == 0 {
		return nil, ErrNoTrustedIssuers
	}

	return &Verifier{
		Hasher:         hasher,
		keys:           publicKeyProvider,
		trustedIssuers: trustedIssuers,
	}, nil
}

// Verify checks that the given compact JWS (JWT) token is a valid signature
// for the provided message body.
//
// Verify performs the following steps:
//  1. Parses the token and enforces the PS256 signing method.
//  2. Extracts "iss" and "kid" claims and validates "iss" against
//     TrustedIssuers if configured.
//  3. Resolves the corresponding RSA public key via the PublicKeyProvider.
//  4. Enforces the minimum RSA key size.
//  5. Verifies the JWS signature using the resolved public key.
//  6. Validates that the "hash-alg" claim is "SHA256".
//  7. Recomputes base64url(SHA-256(body)) and compares it to the "hash"
//     claim in constant time.
//
// If any step fails, Verify returns a non-nil error and the caller should
// treat the message as untrusted.
func (v *Verifier) Verify(ctx context.Context, tokenStr string, body []byte) error {
	keyFunc := func(t *jwt.Token) (any, error) {
		if t.Method != jwt.SigningMethodPS256 {
			return nil, fmt.Errorf("%w: %s", ErrUnexpectedSigningMethod, t.Method.Alg())
		}

		claims, ok := t.Claims.(jwt.MapClaims)
		if !ok {
			return nil, ErrUnexpectedClaimsType
		}

		iss, _ := claims[jwtMapClaimIss].(string)
		kid, _ := claims[jwtMapClaimKid].(string)

		if iss == "" || kid == "" {
			return nil, ErrMissingIssOrKid
		}

		if len(v.trustedIssuers) == 0 {
			return nil, ErrNoTrustedIssuers
		}

		if _, trusted := v.trustedIssuers[iss]; !trusted {
			return nil, ErrUntrustedIssuer
		}

		pub, err := v.keys.VerificationKey(ctx, iss, kid)
		if err != nil {
			return nil, err
		}

		if pub.N.BitLen() < 3072 {
			return nil, fmt.Errorf("%w: %d bits", ErrRSAKeyLength, pub.N.BitLen())
		}

		return pub, nil
	}

	parsed, err := jwt.Parse(tokenStr, keyFunc)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrJWTParseFailed, err)
	}

	if !parsed.Valid {
		return ErrSignatureInvalid
	}

	claims, ok := parsed.Claims.(jwt.MapClaims)
	if !ok {
		return ErrUnexpectedClaimsType
	}

	hashAlg, _ := claims[jwtMapClaimHashAlgorithm].(string)
	if hashAlg != v.ToString() {
		return fmt.Errorf("%w: %s", ErrUnsupportedHashAlgorithm, hashAlg)
	}

	hashClaim, _ := claims[jwtMapClaimHash].(string)
	if hashClaim == "" {
		return ErrHashClaimMissing
	}

	calc := v.HashMessage(body)
	if subtle.ConstantTimeCompare([]byte(hashClaim), []byte(calc)) != 1 {
		return ErrMessageHashMismatch
	}

	return nil
}
