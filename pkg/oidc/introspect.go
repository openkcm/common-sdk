package oidc

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
)

// Introspection represents the response from an introspection request.
type Introspection struct {
	Active bool     `json:"active"`
	Groups []string `json:"groups,omitempty"`

	// Error response fields e.g. bad credentials
	Error            string `json:"error,omitempty"`
	ErrorDescription string `json:"error_description,omitempty"`
}

// IntrospectToken introspects the given token using the OpenID Provider's introspection endpoint.
func (p *Provider) IntrospectToken(ctx context.Context, token string) (Introspection, error) {
	if p.disableTokenIntrospection {
		return Introspection{}, ErrTokenIntrospectionDisabled
	}

	cfg, err := p.GetConfiguration(ctx)
	if err != nil {
		return Introspection{}, errors.Join(ErrCouldNotGetWellKnownConfig, err)
	}

	if cfg.IntrospectionEndpoint == "" {
		return Introspection{}, ErrNoIntrospectionEndpoint
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, cfg.IntrospectionEndpoint, nil)
	if err != nil {
		return Introspection{}, errors.Join(ErrCouldNotCreateHTTPRequest, err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	q := req.URL.Query()
	q.Set("token", token)

	for k, v := range p.queryParametersIntrospect {
		q.Set(k, v)
	}

	req.URL.RawQuery = q.Encode()

	resp, err := p.secureHttpClient.Do(req)
	if err != nil {
		return Introspection{}, errors.Join(ErrCouldNotDoHTTPRequest, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return Introspection{}, errors.Join(ErrCouldNotReadResponseBody, err)
	}

	if resp.StatusCode != http.StatusOK {
		return Introspection{}, ProviderRespondedNon200Error{
			Code: resp.StatusCode,
			Body: string(body),
		}
	}

	var intr Introspection

	err = json.Unmarshal(body, &intr)
	if err != nil {
		return Introspection{}, CouldNotUnmarshallResponseError{
			Err:  err,
			Body: string(body),
		}
	}

	return intr, nil
}
