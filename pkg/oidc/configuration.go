package oidc

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
)

const wellKnownOpenIDConfigPath = "/.well-known/openid-configuration"

// Configuration is the meta data describing the configuration of an OpenID Provider.
// It can be onbtained from the .well-known/openid-configuration endpoint.
// See https://openid.net/specs/openid-connect-discovery-1_0.html#ProviderMetadata for details.
type Configuration struct {
	Issuer                            string   `json:"issuer,omitempty"`
	AuthorizationEndpoint             string   `json:"authorization_endpoint,omitempty"`
	TokenEndpoint                     string   `json:"token_endpoint,omitempty"`
	UserinfoEndpoint                  string   `json:"userinfo_endpoint,omitempty"`
	JwksURI                           string   `json:"jwks_uri,omitempty"`
	ResponseTypesSupported            []string `json:"response_types_supported,omitempty"`
	GrantTypesSupported               []string `json:"grant_types_supported,omitempty"`
	SubjectTypesSupported             []string `json:"subject_types_supported,omitempty"`
	IDTokenSigningAlgValuesSupported  []string `json:"id_token_signing_alg_values_supported,omitempty"`
	ScopesSupported                   []string `json:"scopes_supported,omitempty"`
	TokenEndpointAuthMethodsSupported []string `json:"token_endpoint_auth_methods_supported,omitempty"`
	ClaimsSupported                   []string `json:"claims_supported,omitempty"`

	// From https://datatracker.ietf.org/doc/html/rfc7662
	IntrospectionEndpoint string `json:"introspection_endpoint,omitempty"`

	// From https://openid.net/specs/openid-connect-rpinitiated-1_0.html#OPMetadata
	EndSessionEndpoint string `json:"end_session_endpoint,omitempty"`
}

// GetConfiguration fetches and stores the OpenID configuration for the provider.
func (p *Provider) GetConfiguration(ctx context.Context) (*Configuration, error) {
	// Fast path: check if config is already set with read lock
	p.configMu.RLock()

	if p.config != nil {
		defer p.configMu.RUnlock()
		return p.config, nil
	}

	p.configMu.RUnlock()

	// Slow path: acquire write lock and check again (double-checked locking)
	p.configMu.Lock()
	defer p.configMu.Unlock()

	// Check again in case another goroutine set it while we were waiting
	if p.config != nil {
		return p.config, nil
	}

	u, err := url.JoinPath(p.issuerURI, wellKnownOpenIDConfigPath)
	if err != nil {
		return nil, errors.Join(ErrCouldNotBuildURL, err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, errors.Join(ErrCouldNotCreateHTTPRequest, err)
	}

	resp, err := p.publicHttpClient.Do(req)
	if err != nil {
		return nil, errors.Join(ErrCouldNotDoHTTPRequest, err)
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

	var conf Configuration

	err = json.Unmarshal(body, &conf)
	if err != nil {
		return nil, CouldNotUnmarshallResponseError{
			Err:  err,
			Body: string(body),
		}
	}

	p.config = &conf

	return p.config, nil
}
