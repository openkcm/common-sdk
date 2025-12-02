package jwtsigning

import (
	"context"
	"crypto/rsa"
)

// KeyMetadata describes the logical identity of a signing key.
// It maps directly to the JWT "iss" (issuer) and "kid" (key ID) used by
// verifiers to look up the matching public key via the configured trust mechanism.
type KeyMetadata struct {
	Iss string // typically the cluster URL that exposes .well-known/jwks.json
	Kid string // uniquely identifies the signing key under that issuer
}

// PrivateKeyProvider supplies the current RSA private key and its metadata for signing outgoing messages.
type PrivateKeyProvider interface {
	// CurrentSigningKey returns the key+metadata to use for signing.
	CurrentSigningKey(ctx context.Context) (*rsa.PrivateKey, KeyMetadata, error)
}

// PublicKeyProvider resolves RSA public keys for verification of incoming message signatures.
type PublicKeyProvider interface {
	// VerificationKey returns the public key for given issuer and kid.
	VerificationKey(ctx context.Context, iss, kid string) (*rsa.PublicKey, error)
}

type Hasher interface {
	HashMessage(body []byte) string
	ToString() string
}
