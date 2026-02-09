package oidc

import (
	"errors"
	"net/http"
	"net/url"
)

var (
	DefaultIssuerClaims = []string{"iss"}
)

type Provider struct {
	// According to RFC 7519, the issuer identifies the principal that issued the
	// JWT, which is a case-sensitive string or URI.
	issuer string

	// The optional issuer URI specifies where the well known OpenID configuration
	// endpoint is located. This is usually needed when the issuer is not a URI.
	issuerURI string

	// The optional JSON Web Key Set (JWKS) URI specifies the endpoint where the
	// provider's public keys are published. This is usually needed when the
	// issuer is not a URI and we want to lookup the provider by the `jku` claim
	// from the JWT header without fetching the well known OpenID configuration.
	customJWKSURI string

	// The audiences that are expected in the token's `aud` claim.
	audiences []string

	config *Configuration // the well known OpenID configuration

	// Additional query parameters to be sent with the introspection request.
	queryParametersIntrospect map[string]string

	// whether to allow HTTP scheme for issuer and JWKS URIs
	allowHttpScheme bool

	publicHttpClient *http.Client // client to be used for public endpoints
	secureHttpClient *http.Client // client to be used for secured endpoints
}

// UniqueID returns a unique identifier for the provider.
// This allows for providers having the same issuer but different endpoints.
// It can be used for caching and should be unique across different providers.
func (p *Provider) UniqueID() string {
	if p.issuerURI != "" {
		return p.issuerURI
	}

	if p.customJWKSURI != "" {
		return p.customJWKSURI
	}

	return p.issuer
}
func (p *Provider) Issuer() string {
	return p.issuer
}
func (p *Provider) IssuerURI() string {
	return p.issuerURI
}
func (p *Provider) CustomJWKSURI() string {
	return p.customJWKSURI
}
func (p *Provider) Audiences() []string {
	return p.audiences
}

// ProviderOption is used to configure a provider.
type ProviderOption func(*Provider)

// WithCustomIssuerURI configures a custom issuer URI.
func WithCustomIssuerURI(issuerURI string) ProviderOption {
	return func(provider *Provider) {
		provider.issuerURI = issuerURI
	}
}

// WithCustomJWKSURI configures a custom JWKS URI.
func WithCustomJWKSURI(customJWKSURI string) ProviderOption {
	return func(provider *Provider) {
		provider.customJWKSURI = customJWKSURI
	}
}

// WithAllowHttpScheme configures whether to allow HTTP scheme for URIs.
// By default, the HTTPS scheme is enforced.
func WithAllowHttpScheme(allowHttpScheme bool) ProviderOption {
	return func(provider *Provider) {
		provider.allowHttpScheme = allowHttpScheme
	}
}

// WithPublicHTTPClient let's you set the client to be used for public endpoints,
// e.g. the well known OpenID configuration endpoint.
func WithPublicHTTPClient(c *http.Client) ProviderOption {
	return func(provider *Provider) {
		provider.publicHttpClient = c
	}
}

// WithSecureHTTPClient let's you set the client to be used for secured endpoints,
// e.g. the token endpoint.
func WithSecureHTTPClient(c *http.Client) ProviderOption {
	return func(provider *Provider) {
		provider.secureHttpClient = c
	}
}

// WithIntrospectQueryParameters let's you define addition query parameters
// to be sent with the introspection request.
func WithIntrospectQueryParameters(params map[string]string) ProviderOption {
	return func(provider *Provider) {
		provider.queryParametersIntrospect = params
	}
}

// NewProvider creates a new provider and applies the given options.
func NewProvider(issuer string, audiences []string, opts ...ProviderOption) (*Provider, error) {
	provider := &Provider{
		issuer:           issuer,
		audiences:        audiences,
		publicHttpClient: http.DefaultClient,
		secureHttpClient: http.DefaultClient,
	}

	for _, opt := range opts {
		if opt == nil {
			continue
		}

		opt(provider)
	}

	if provider.issuerURI == "" {
		provider.issuerURI = provider.issuer
	}

	for _, uri := range []string{provider.issuerURI, provider.customJWKSURI} {
		if uri == "" {
			continue
		}

		parsedURL, err := url.Parse(uri)
		if err != nil {
			return nil, errors.Join(ErrInvalidURI, err)
		}

		if !provider.allowHttpScheme {
			if parsedURL.Scheme != "https" {
				return nil, errors.Join(ErrInvalidURI, ErrInvalidURLScheme)
			}
		}
	}

	return provider, nil
}
