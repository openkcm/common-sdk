package commonhttp

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/openkcm/common-sdk/pkg/commoncfg"
)

// NewClientFromBasic creates an *http.Client that automatically injects
// HTTP Basic Authentication credentials into every outgoing request.
//
// The BasicAuth struct contains two SourceRef fields (Username and Password):
// each can come from literals, environment variables, files, etc.
//
// Each request sent by the returned client is modified to include:
//
//	Authorization: Basic <base64(username:password)>
//
// Parameters:
//   - clientAuth: pointer to BasicAuth config containing username & password.
//
// Returns:
//   - *http.Client configured with a custom RoundTripper
//   - error if configuration is invalid or credentials cannot be loaded
func NewClientFromBasic(clientAuth *commoncfg.BasicAuth) (*http.Client, error) {
	if clientAuth == nil {
		return nil, errors.New("basic auth config is nil")
	}

	usernameBytes, err := commoncfg.ExtractValueFromSourceRef(&clientAuth.Username)
	if err != nil {
		return nil, fmt.Errorf("basic credentials missing username: %w", err)
	}

	passwordBytes, err := commoncfg.ExtractValueFromSourceRef(&clientAuth.Password)
	if err != nil {
		return nil, fmt.Errorf("basic credentials missing password: %w", err)
	}

	if len(usernameBytes) == 0 {
		return nil, errors.New("basic auth username is empty")
	}

	if len(passwordBytes) == 0 {
		return nil, errors.New("basic auth password is empty")
	}

	rt := &clientBasicRoundTripper{
		Username: string(usernameBytes),
		Password: string(passwordBytes),
		Next:     http.DefaultTransport,
	}

	return &http.Client{Transport: rt}, nil
}

// clientBasicRoundTripper injects Basic Auth credentials into all outgoing
// HTTP requests when used as the Transport of an http.Client.
//
// It wraps an underlying RoundTripper (Next) and forwards modified requests.
// To avoid side effects, requests are cloned before modification.
type clientBasicRoundTripper struct {
	// Username is the Basic Auth username.
	Username string

	// Password is the Basic Auth password.
	Password string

	// Next is the underlying transport (defaults to http.DefaultTransport).
	Next http.RoundTripper
}

// RoundTrip implements http.RoundTripper by:
//  1. Cloning the request
//  2. Injecting HTTP Basic Authentication headers
//  3. Forwarding the request to the underlying transport
//
// Parameters:
//   - req: original request
//
// Returns:
//   - *http.Response from the underlying transport
//   - error if the underlying transport fails
func (t *clientBasicRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	// Clone the request including its context, headers, and URL.
	newReq := req.Clone(req.Context())

	// Inject Basic Auth header.
	newReq.SetBasicAuth(t.Username, t.Password)

	// Forward request.
	return t.Next.RoundTrip(newReq)
}
