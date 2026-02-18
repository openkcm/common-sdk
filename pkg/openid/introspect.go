package openid

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
)

// IntrospectResponse represents the response from an introspection request.
type IntrospectResponse struct {
	Active bool     `json:"active"`
	Groups []string `json:"groups,omitempty"`

	// Error response fields e.g. bad credentials
	Error            string `json:"error,omitempty"`
	ErrorDescription string `json:"error_description,omitempty"`
}

// IntrospectToken introspects the given token using the OpenID Provider's introspection endpoint.
func (cfg Configuration) IntrospectToken(ctx context.Context, token string, additionalQueryParameter map[string]string) (IntrospectResponse, error) {
	if cfg.IntrospectionEndpoint == "" {
		return IntrospectResponse{}, ErrNoIntrospectionEndpoint
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, cfg.IntrospectionEndpoint, nil)
	if err != nil {
		return IntrospectResponse{}, errors.Join(ErrCouldNotCreateHTTPRequest, err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	q := req.URL.Query()
	q.Set("token", token)

	for k, v := range additionalQueryParameter {
		q.Set(k, v)
	}

	req.URL.RawQuery = q.Encode()

	httpClient := cfg.HTTPClient
	if httpClient == nil {
		httpClient = http.DefaultClient
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return IntrospectResponse{}, errors.Join(ErrCouldNotDoHTTPRequest, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return IntrospectResponse{}, errors.Join(ErrCouldNotReadResponseBody, err)
	}

	if resp.StatusCode != http.StatusOK {
		return IntrospectResponse{}, ProviderRespondedNon200Error{
			Code: resp.StatusCode,
			Body: string(body),
		}
	}

	var introresp IntrospectResponse

	err = json.Unmarshal(body, &introresp)
	if err != nil {
		return IntrospectResponse{}, CouldNotDecodeResponseError{
			Err:  err,
			Body: string(body),
		}
	}

	return introresp, nil
}
