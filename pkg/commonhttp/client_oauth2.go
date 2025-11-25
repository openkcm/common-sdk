// Package commonhttp provides utilities to create HTTP clients
// configured with OAuth2 credentials and optional mutual TLS (mTLS).

package commonhttp

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/openkcm/common-sdk/pkg/commoncfg"
	"github.com/openkcm/common-sdk/pkg/pointers"
)

// NewClientFromOAuth2 creates an *http.Client configured with OAuth2 credentials
// and optional mTLS transport.
//
// The client supports two types of OAuth2 authentication:
//  1. client_secret: requires ClientID and ClientSecret
//  2. private_key_jwt: requires ClientID, ClientAssertion, and ClientAssertionType
//
// Only one authentication method may be provided. If both clientSecret and
// clientAssertion are configured, this function returns an error.
//
// If MTLS is provided, the client's transport will use the TLS configuration.
//
// Parameters:
//   - clientAuth: a pointer to an OAuth2 configuration containing credentials and optional mTLS.
//
// Returns:
//   - *http.Client: the configured HTTP client
//   - error: if the configuration is invalid or mTLS loading fails
func NewClientFromOAuth2(clientAuth *commoncfg.OAuth2) (*http.Client, error) {
	if clientAuth == nil {
		return nil, errors.New("oauth2 config is nil")
	}

	if clientAuth.Credentials.ClientID.Source == "" {
		return nil, errors.New("oauth2.clientID is missing or invalid")
	}

	clientID, err := commoncfg.LoadValueFromSourceRef(clientAuth.Credentials.ClientID)
	if err != nil {
		return nil, fmt.Errorf("OAuth2 credentials missing client ID: %w", err)
	}

	rt := &clientOAuth2RoundTripper{
		ClientID: string(clientID),
		Next:     http.DefaultTransport,
	}

	// Load mTLS if configured
	err = loadMTLS(clientAuth.MTLS, rt)
	if err != nil {
		return nil, err
	}

	// Load OAuth2 credentials
	err = loadOAuth2Credentials(&clientAuth.Credentials, rt)
	if err != nil {
		return nil, err
	}

	return &http.Client{Transport: rt}, nil
}

// loadMTLS configures the HTTP transport to use mutual TLS (mTLS).
//
// Parameters:
//   - mtls: MTLS configuration (cert, key, server CA)
//   - rt: pointer to the clientOAuth2RoundTripper that will hold the transport
//
// Returns:
//   - error: if loading the TLS configuration fails
func loadMTLS(mtls *commoncfg.MTLS, rt *clientOAuth2RoundTripper) error {
	if mtls == nil {
		return nil
	}

	tlsConfig, err := commoncfg.LoadMTLSConfig(mtls)
	if err != nil {
		return fmt.Errorf("loading mTLS config: %w", err)
	}

	rt.Next = &http.Transport{TLSClientConfig: tlsConfig}

	return nil
}

// loadOAuth2Credentials populates the round-tripper with OAuth2 credentials.
//
// Supports two authentication methods:
//   - client_secret: sets ClientSecret if provided
//   - private_key_jwt: sets ClientAssertion and ClientAssertionType if provided
//
// Returns an error if both methods are provided simultaneously.
func loadOAuth2Credentials(creds *commoncfg.OAuth2Credentials, rt *clientOAuth2RoundTripper) error {
	if secret, _ := commoncfg.ExtractValueFromSourceRef(creds.ClientSecret); secret != nil && string(secret) != "" {
		rt.ClientSecret = pointers.To(string(secret))
	}

	if assertion, _ := commoncfg.ExtractValueFromSourceRef(creds.ClientAssertion); assertion != nil && string(assertion) != "" {
		rt.ClientAssertion = pointers.To(string(assertion))
	}

	if creds.ClientAssertionType != nil && *creds.ClientAssertionType != "" {
		rt.ClientAssertionType = creds.ClientAssertionType
	}

	if rt.ClientSecret != nil && rt.ClientAssertion != nil {
		return errors.New("invalid OAuth2 config: both clientSecret and clientAssertion provided")
	}

	return nil
}

// clientOAuth2RoundTripper is a custom HTTP RoundTripper that automatically
// injects OAuth2 credentials into query parameters for every request.
//
// It supports either client_secret authentication or private_key_jwt authentication.
//
// Fields:
//   - ClientID: OAuth2 client ID (required)
//   - ClientSecret: optional client secret
//   - ClientAssertionType: optional JWT assertion type
//   - ClientAssertion: optional JWT assertion
//   - Next: underlying RoundTripper to which requests are forwarded
type clientOAuth2RoundTripper struct {
	ClientID            string
	ClientSecret        *string // optional
	ClientAssertionType *string // optional
	ClientAssertion     *string // optional
	Next                http.RoundTripper
}

// RoundTrip implements the http.RoundTripper interface.
//
// It injects the configured OAuth2 credentials into the request URL's query parameters
// according to the authentication method:
//   - client_secret: adds client_id and client_secret
//   - private_key_jwt: adds client_id, client_assertion_type, and client_assertion
//
// The modified request is then forwarded to the underlying transport.
func (t *clientOAuth2RoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	newReq := *req
	urlCopy := *req.URL
	q := urlCopy.Query()

	q.Set("client_id", t.ClientID)

	switch {
	case t.ClientSecret != nil && *t.ClientSecret != "":
		q.Set("client_secret", *t.ClientSecret)

	case t.ClientAssertion != nil && t.ClientAssertionType != nil:
		q.Set("client_assertion_type", *t.ClientAssertionType)
		q.Set("client_assertion", *t.ClientAssertion)
	}

	urlCopy.RawQuery = q.Encode()
	newReq.URL = &urlCopy

	return t.Next.RoundTrip(&newReq)
}
