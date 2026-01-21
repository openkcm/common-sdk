package jwtsigning

import (
	"context"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"errors"
	"fmt"
	"sync"

	slogctx "github.com/veqryn/slog-context"
)

var (
	_ PublicKeyProvider = &JWKSProvider{}

	// ErrNoClientFound indicates no JWKS client configured for the given issuer.
	ErrNoClientFound = errors.New("no client found for issuer")

	// ErrNoValidatorFound indicates no validator configured for the given issuer.
	ErrNoValidatorFound = errors.New("no validator found for issuer")

	// ErrKidNoPublicKeyFound indicates no public key found in cache for the given kid.
	ErrKidNoPublicKeyFound = errors.New("no public key found for kid")

	// ErrIssuerEmpty is returned when the issuer value is empty.
	ErrIssuerEmpty = errors.New("issuer is empty")
)

// JWKSProvider fetches JWKS for issuers and provides RSA public keys.
// It maintains a map of issuer to JWKSClientStore which holds a client,
// validator and an in-memory cache of public keys.
type JWKSProvider struct {
	stores map[string]*jwksClientStore
}

// jwksClientStore groups a JWKS client, its validator and an in-memory cache
// of parsed RSA public keys for a single issuer. The lock protects access to the cache.
type jwksClientStore struct {
	client    *Client
	validator *Validator
	cache     map[string]*rsa.PublicKey
	lock      sync.RWMutex
}

// NewJWKSProvider creates and returns a new JWKSProvider instance
// with initialized in-memory storage for issuer client stores.
func NewJWKSProvider() *JWKSProvider {
	return &JWKSProvider{
		stores: make(map[string]*jwksClientStore),
	}
}

// AddIssuerClientValidator registers a client and validator for a given issuer.
// It initializes the in-memory cache for the issuer. Returns an error if the
// issuer, client or validator is nil.
func (j *JWKSProvider) AddIssuerClientValidator(issuer string, client *Client, validator *Validator) error {
	if issuer == "" {
		return ErrIssuerEmpty
	}

	if client == nil {
		return fmt.Errorf("%w: %s", ErrNoClientFound, issuer)
	}

	if validator == nil {
		return fmt.Errorf("%w: %s", ErrNoValidatorFound, issuer)
	}

	j.stores[issuer] = &jwksClientStore{
		client:    client,
		validator: validator,
		cache:     make(map[string]*rsa.PublicKey),
	}

	return nil
}

// VerificationKey returns the RSA public key for the given issuer (iss) and key ID (kid).
// It first attempts to retrieve the key from the in-memory cache. If the key is not found,
// it refreshes the JWKS cache for the issuer and tries again. Returns an error if the
// issuer is not configured or if the key cannot be found or validated.
func (j *JWKSProvider) VerificationKey(ctx context.Context, iss string, kid string) (*rsa.PublicKey, error) {
	ctx = slogctx.With(ctx, "issuer", iss, "kid", kid)

	store, ok := j.stores[iss]
	if !ok {
		slogctx.Error(ctx, "no client configured")
		return nil, fmt.Errorf("%w: %s", ErrNoClientFound, iss)
	}

	key, err := j.readKeyWithLock(ctx, store, kid)
	if err == nil {
		return key, nil
	}

	return j.rebuildAndReadKey(ctx, store, kid)
}

func (j *JWKSProvider) readKeyWithLock(ctx context.Context, store *jwksClientStore, kid string) (*rsa.PublicKey, error) {
	store.lock.RLock()
	defer store.lock.RUnlock()

	return j.readKey(ctx, store, kid)
}

func (j *JWKSProvider) readKey(ctx context.Context, store *jwksClientStore, kid string) (*rsa.PublicKey, error) {
	key, ok := store.cache[kid]
	if !ok {
		slogctx.Info(ctx, "no public key found in cache")
		return nil, fmt.Errorf("%w: %s", ErrKidNoPublicKeyFound, kid)
	}

	return key, nil
}

func (j *JWKSProvider) rebuildAndReadKey(ctx context.Context, store *jwksClientStore, kid string) (*rsa.PublicKey, error) {
	slogctx.Info(ctx, "key not found in cache, refreshing")

	store.lock.Lock()
	defer store.lock.Unlock()

	// Double-check if another goroutine has already refreshed the cache
	key, err := j.readKey(ctx, store, kid)
	if err == nil {
		return key, nil
	}

	result, err := store.client.Get(ctx)
	if err != nil {
		slogctx.Error(ctx, "failed while fetching jwks url", "error", err)
		return nil, err
	}

	pubKeys := make(map[string]*rsa.PublicKey, len(result.Keys))

	for _, jwk := range result.Keys {
		err := store.validator.Validate(jwk)
		if err != nil {
			slogctx.Error(ctx, "failed certificate validation", "for kid", jwk.Kid, "error", err)
			continue
		}

		pubKey, err := parsePublicKey(ctx, jwk)
		if err != nil {
			slogctx.Error(ctx, "failed while parsing public keys", "for kid", jwk.Kid, "error", err)
			continue
		}

		pubKeys[jwk.Kid] = pubKey
	}

	if len(result.Keys) > 0 && len(pubKeys) == 0 {
		slogctx.Warn(ctx, "jwks returned keys but none validated/parsed; retaining previous cache", "total_keys", len(result.Keys))
	}

	if len(pubKeys) > 0 {
		store.cache = pubKeys
	}

	return j.readKey(ctx, store, kid)
}

func parsePublicKey(ctx context.Context, key Key) (*rsa.PublicKey, error) {
	if len(key.X5c) == 0 {
		return nil, ErrX5cEmpty
	}

	firstCert := key.X5c[0]

	bDer, err := base64.StdEncoding.DecodeString(firstCert)
	if err != nil {
		slogctx.Error(ctx, "while base64 decoding", "kid", key.Kid, "error", err)
		return nil, err
	}

	cert, err := x509.ParseCertificate(bDer)
	if err != nil {
		slogctx.Error(ctx, "parse certificate", "kid", key.Kid, "error", err)
		return nil, err
	}

	switch key.Kty {
	case KeyTypeRSA:
		pubKey, ok := cert.PublicKey.(*rsa.PublicKey)
		if !ok {
			slogctx.Error(ctx, "getting public key", "kid", key.Kid)
			return nil, ErrRSAPublicKeyNotFound
		}

		return pubKey, nil

	default:
		slogctx.Error(ctx, "unsupported key type", "kid", key.Kid)
		return nil, fmt.Errorf("%w: [%s]", ErrKeyTypeUnsupported, key.Kty)
	}
}
