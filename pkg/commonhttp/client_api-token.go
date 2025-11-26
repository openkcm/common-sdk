package commonhttp

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/openkcm/common-sdk/pkg/commoncfg"
)

// NewClientFromAPIToken creates a new *http.Client that automatically injects
// an API token into the Authorization header of every request.
//
// The function expects a *commoncfg.SourceRef containing the API token.
// A SourceRef may reference a literal value, environment variable, file, or
// any other supported configuration source.
//
// On success, the returned client wraps the default HTTP transport with a
// custom RoundTripper (clientAPITokenRoundTripper) which adds:
//
//	Authorization: Api-Token <token>
//
// Parameters:
//   - value: pointer to a SourceRef pointing to the API token.
//
// Returns:
//   - *http.Client: configured HTTP client
//   - error: if the token reference is nil, unreadable, or empty.
func NewClientFromAPIToken(value *commoncfg.SourceRef) (*http.Client, error) {
	if value == nil {
		return nil, errors.New("api token auth config is nil")
	}

	tokenBytes, err := commoncfg.ExtractValueFromSourceRef(value)
	if err != nil {
		return nil, fmt.Errorf("api token could not be loaded: %w", err)
	}

	if len(tokenBytes) == 0 {
		return nil, errors.New("api token is empty")
	}

	rt := &clientAPITokenRoundTripper{
		token: string(tokenBytes),
		Next:  http.DefaultTransport,
	}

	return &http.Client{Transport: rt}, nil
}

// clientAPITokenRoundTripper is a custom HTTP RoundTripper that automatically
// adds an API-token–based Authorization header to all outgoing requests.
//
// Behavior:
//   - Adds: `Authorization: Api-Token <token>`
//   - Forwards the modified request to the underlying transport (Next)
//
// The token is injected into headers for *every* HTTP request sent by the
// client returned by NewClientFromAPIToken.
type clientAPITokenRoundTripper struct {
	// token is the API token string used to authenticate requests.
	token string

	// Next is the underlying HTTP RoundTripper to which the modified request
	// is forwarded. Defaults to http.DefaultTransport.
	Next http.RoundTripper
}

// RoundTrip implements the http.RoundTripper interface.
//
// This method:
//  1. Copies the incoming request to avoid mutation.
//  2. Injects the Authorization header using the API token.
//  3. Forwards the request to the underlying transport.
//
// Parameters:
//   - req: the outgoing HTTP request.
//
// Returns:
//   - *http.Response: the HTTP response returned by the underlying transport.
//   - error: if the underlying RoundTripper returns an error.
func (t *clientAPITokenRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	// Create a shallow copy to avoid mutating user-provided request.
	newReq := req.Clone(req.Context())

	// Inject API token header.
	newReq.Header.Set("Authorization", "Api-Token "+t.token)

	// Forward the request.
	return t.Next.RoundTrip(newReq)
}
