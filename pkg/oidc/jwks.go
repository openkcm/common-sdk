package oidc

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/go-jose/go-jose/v4"
)

// GetSigningKey returns the key for the given key.
func (p *Provider) GetSigningKey(ctx context.Context, keyID string) (*jose.JSONWebKey, error) {
	// If the provider was configured with a custom JWKS URI, use it.
	// Otherwise get the JWKS URI from the provider's configuration.
	jwksURI := p.customJWKSURI
	if jwksURI == "" {
		cfg, err := p.GetConfiguration(ctx)
		if err != nil {
			return nil, errors.Join(ErrCouldNotGetWellKnownConfig, err)
		}

		jwksURI = cfg.JwksURI
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, jwksURI, nil)
	if err != nil {
		return nil, errors.Join(ErrCouldNotCreateHTTPRequest, err)
	}

	resp, err := p.secureHttpClient.Do(request)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.Join(ErrCouldNotReadResponseBody, err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, ProviderRespondedNon200Error{
			Code: resp.StatusCode,
			Body: string(body),
		}
	}

	var jwks jose.JSONWebKeySet

	err = json.Unmarshal(body, &jwks)
	if err != nil {
		return nil, CouldNotUnmarshallResponseError{
			Err:  err,
			Body: string(body),
		}
	}

	// find the key for the given key ID, cache it and return it
	for _, k := range jwks.Keys {
		if k.Use == "sig" && k.KeyID == keyID {
			return &k, nil
		}
	}

	// return an error if the key was not found
	return nil, CouldNotFindKeyForKeyIDError{KeyID: keyID}
}
