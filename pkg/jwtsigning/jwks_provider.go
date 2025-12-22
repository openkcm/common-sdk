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

	"github.com/openkcm/common-sdk/pkg/jwks"
)

var (
	_                      PublicKeyProvider = &JWKSProvider{}
	ErrNoClientFound                         = errors.New("no client found for issuer")
	ErrNoValidatorFound                      = errors.New("no validator found for issuer")
	ErrKidNoPublicKeyFound                   = errors.New("no public key found for kid")
)

type JWKSProvider struct {
	clis map[string]*ClientCache
}

type ClientCache struct {
	Client    *jwks.Client
	Validator *jwks.Validator
	cache     map[string]*rsa.PublicKey
	lock      *sync.RWMutex
}

func NewJWKSProvider() *JWKSProvider {
	return &JWKSProvider{
		clis: make(map[string]*ClientCache),
	}
}

func (j *JWKSProvider) AddCli(issuer string, cc ClientCache) error {
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

// VerificationKey implements PublicKeyProvider.
func (j *JWKSProvider) VerificationKey(ctx context.Context, iss string, kid string) (*rsa.PublicKey, error) {
	ctx = slogctx.With(ctx, "issuer", iss, "kid", kid)

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
			slogctx.Error(ctx, "error while validating", "kid", kid, "error", err)
			continue
		}

		leaf := key.X5c[0]

		bDer, err := base64.StdEncoding.DecodeString(leaf)
		if err != nil {
			slogctx.Error(ctx, "error while base64 decoding", "kid", kid, "error", err)
			continue
		}

		cert, err := x509.ParseCertificate(bDer)
		if err != nil {
			slogctx.Error(ctx, "error parse certificate", "kid", kid, "error", err)
			continue
		}

		pubKey, ok := cert.PublicKey.(*rsa.PublicKey)
		if !ok {
			slogctx.Error(ctx, "error getting public key", "kid", kid)
			continue
		}

		pubKeys[key.Kid] = pubKey
	}

	j.writeKeys(cc, pubKeys)

	return j.readKey(ctx, cc, kid)
}

func (j *JWKSProvider) readKey(ctx context.Context, cc *ClientCache, kid string) (*rsa.PublicKey, error) {
	cc.lock.RLock()
	defer cc.lock.RUnlock()

	key, ok := cc.cache[kid]
	if !ok {
		slogctx.Error(ctx, "error getting public key", "kid", kid)
		return nil, fmt.Errorf("%w %s", ErrKidNoPublicKeyFound, kid)
	}

	return key, nil
}

func (*JWKSProvider) writeKeys(cc *ClientCache, pubKeys map[string]*rsa.PublicKey) {
	if len(pubKeys) > 0 {
		cc.lock.Lock()
		defer cc.lock.Unlock()

		cc.cache = pubKeys
	}
}
