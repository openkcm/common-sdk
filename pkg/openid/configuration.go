package openid

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
)

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

	// HTTPClient is the HTTP client to use for certain requests to this OpenID Provider.
	// If nil, http.DefaultClient is used.
	HTTPClient *http.Client `json:"-"`
}

// GetConfig fetches the OpenID Provider configuration from the given issuer URL.
// Note that the issuer URL may be different from the "issuer" field in the returned
// configuration.
func GetConfig(ctx context.Context, issuerURL string) (Configuration, error) {
	const wellKnownOpenIDConfigPath = "/.well-known/openid-configuration"

	u, err := url.JoinPath(issuerURL, wellKnownOpenIDConfigPath)
	if err != nil {
		return Configuration{}, errors.Join(ErrCouldNotBuildURL, err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return Configuration{}, errors.Join(ErrCouldNotCreateHTTPRequest, err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return Configuration{}, errors.Join(ErrCouldNotDoHTTPRequest, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return Configuration{}, errors.Join(ErrCouldNotReadResponseBody, err)
	}

	if resp.StatusCode != http.StatusOK {
		return Configuration{}, ProviderRespondedNon200Error{
			Code: resp.StatusCode,
			Body: string(body),
		}
	}

	var conf Configuration

	err = json.Unmarshal(body, &conf)
	if err != nil {
		return Configuration{}, CouldNotDecodeResponseError{
			Err:  err,
			Body: string(body),
		}
	}

	return conf, nil
}
