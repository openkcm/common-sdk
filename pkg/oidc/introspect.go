package oidc

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
)

const (
	urlencoded      = "application/x-www-form-urlencoded"
	applicationJSON = "application/json"
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

	requestBody := make(url.Values, len(p.queryParametersIntrospect)+1)
	requestBody.Set("token", token)

	for k, v := range p.queryParametersIntrospect {
		requestBody.Set(k, v)
	}

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		cfg.IntrospectionEndpoint,
		strings.NewReader(requestBody.Encode()),
	)
	if err != nil {
		return Introspection{}, errors.Join(ErrCouldNotCreateHTTPRequest, err)
	}

	req.Header.Set("Content-Type", urlencoded)
	req.Header.Set("Accept", applicationJSON)

	resp, err := p.secureHttpClient.Do(req)
	if err != nil {
		return Introspection{}, errors.Join(ErrCouldNotDoHTTPRequest, err)
	}
	defer resp.Body.Close()

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return Introspection{}, errors.Join(ErrCouldNotReadResponseBody, err)
	}

	if resp.StatusCode != http.StatusOK {
		return Introspection{}, ProviderRespondedNon200Error{
			Code: resp.StatusCode,
			Body: string(responseBody),
		}
	}

	var intr Introspection

	err = json.Unmarshal(responseBody, &intr)
	if err != nil {
		return Introspection{}, CouldNotUnmarshallResponseError{
			Err:  err,
			Body: string(responseBody),
		}
	}

	return intr, nil
}
