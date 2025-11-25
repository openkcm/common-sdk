package commonhttp

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/openkcm/common-sdk/pkg/commoncfg"
	"github.com/openkcm/common-sdk/pkg/pointers"
)

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

// Helper: load mTLS configuration
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

// Helper: load client secret or client assertion
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

type clientOAuth2RoundTripper struct {
	ClientID            string
	ClientSecret        *string // optional
	ClientAssertionType *string // optional
	ClientAssertion     *string // optional
	Next                http.RoundTripper
}

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
