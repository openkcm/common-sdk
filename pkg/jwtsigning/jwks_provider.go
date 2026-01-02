package jwtsigning

import (
	"context"
	"crypto/rsa"
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
)

// JWKSProvider fetches JWKS for issuers and provides RSA public keys.
// It maintains a map of issuer to JWKSClientStore which holds a client,
// validator and an in-memory cache of public keys.
type JWKSProvider struct {
	clis map[string]*JWKSClientStore
}

// JWKSClientStore groups a JWKS client, its validator and an in-memory cache
// of parsed RSA public keys for a single issuer. The lock protects access to the cache.
type JWKSClientStore struct {
	Client    *Client
	Validator *Validator
	cache     map[string]*rsa.PublicKey
	lock      *sync.RWMutex
}

// NewJWKSProvider creates a JWKSProvider with an initialized client store map.
func NewJWKSProvider() *JWKSProvider {
	return &JWKSProvider{
		clis: make(map[string]*JWKSClientStore),
	}
}

// AddCli registers a JWKS client and validator for the given issuer and
// initializes the in-memory cache and lock for that issuer.
func (j *JWKSProvider) AddCli(issuer string, cc JWKSClientStore) error {
	if cc.Client == nil {
		return fmt.Errorf("%w %s", ErrNoClientFound, issuer)
	}

	if cc.Validator == nil {
		return fmt.Errorf("%w %s", ErrNoValidatorFound, issuer)
	}

	cc.cache = make(map[string]*rsa.PublicKey)
	cc.lock = &sync.RWMutex{}

	j.clis[issuer] = &cc

	return nil
}

// VerificationKey implements PublicKeyProvider. It returns the RSA public key for
// the supplied issuer and key id (kid). If the key is not present in the cache
// it will fetch and validate the issuer's JWKS and refresh the cache.
func (j *JWKSProvider) VerificationKey(ctx context.Context, iss string, kid string) (*rsa.PublicKey, error) {
	ctx = slogctx.With(ctx, "issuer", iss, "kidToSearch", kid)

	cc, ok := j.clis[iss]
	if !ok {
		slogctx.Error(ctx, "error no client configure")
		return nil, fmt.Errorf("%w %s", ErrNoClientFound, iss)
	}

	key, err := j.readKey(ctx, cc, kid)
	if err == nil {
		return key, nil
	}

	slogctx.Info(ctx, "key not found in cache, refreshing")

	result, err := cc.Client.Get(ctx)
	if err != nil {
		slogctx.Error(ctx, "error while fetching jwks url", "error", err)
		return nil, err
	}

	pubKeys := make(map[string]*rsa.PublicKey, len(result.Keys))

	for _, key := range result.Keys {
		err := cc.Validator.Validate(key)
		if err != nil {
			slogctx.Error(ctx, "error while validating", "kid", key.Kid, "error", err)
			continue
		}

		pubKey, err := parsePublicKey(ctx, key)
		if err != nil {
			continue
		}

		pubKeys[key.Kid] = pubKey
	}

	j.writeKeys(cc, pubKeys)

	return j.readKey(ctx, cc, kid)
}

func (j *JWKSProvider) readKey(ctx context.Context, cc *JWKSClientStore, kid string) (*rsa.PublicKey, error) {
	cc.lock.RLock()
	defer cc.lock.RUnlock()

	key, ok := cc.cache[kid]
	if !ok {
		slogctx.Error(ctx, "error getting public key", "kid", kid)
		return nil, fmt.Errorf("%w %s", ErrKidNoPublicKeyFound, kid)
	}

	return key, nil
}

func (*JWKSProvider) writeKeys(cc *JWKSClientStore, pubKeys map[string]*rsa.PublicKey) {
	if len(pubKeys) > 0 {
		cc.lock.Lock()
		defer cc.lock.Unlock()

		cc.cache = pubKeys
	}
}
