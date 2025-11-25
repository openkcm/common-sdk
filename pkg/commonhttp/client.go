package commonhttp

import (
	"crypto/tls"
	"errors"
	"fmt"
	"net/http"

	"github.com/openkcm/common-sdk/pkg/commoncfg"
	"github.com/openkcm/common-sdk/pkg/pointers"
)

func NewClient(cfg *commoncfg.HTTPClient) (*http.Client, error) {
	if cfg == nil {
		return nil, errors.New("HTTPClient config is nil")
	}

	c := &http.Client{Timeout: cfg.Timeout}

	tlsConfig := &tls.Config{
		InsecureSkipVerify: cfg.InsecureSkipVerify,
		MinVersion:         tls.VersionTLS12,
	}

	// override min TLS version if specified
	if cfg.MinVersion != 0 {
		tlsConfig.MinVersion = cfg.MinVersion
	}

	// load optional root CAs
	if !cfg.InsecureSkipVerify && cfg.RootCAs != nil {
		certPool, err := commoncfg.LoadCACertPool(cfg.RootCAs)
		if err != nil {
			return nil, err
		}

		tlsConfig.RootCAs = certPool
	}

	// load optional mTLS certificates
	if cfg.Cert != nil && cfg.CertKey != nil {
		cert, err := commoncfg.LoadClientCertificate(cfg.Cert, cfg.CertKey)
		if err != nil {
			return nil, err
		}

		tlsConfig.Certificates = []tls.Certificate{*cert}
	}

	c.Transport = &http.Transport{
		TLSClientConfig: tlsConfig,
	}

	return c, nil
}

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

	// ---- Load mTLS if configured ----
	if clientAuth.MTLS != nil {
		tlsConfig, err := commoncfg.LoadMTLSConfig(clientAuth.MTLS)
		if err != nil {
			return nil, fmt.Errorf("loading mTLS config: %w", err)
		}

		rt.Next = &http.Transport{
			TLSClientConfig: tlsConfig,
		}
	}

	// ---- Load client_secret authentication ----
	if clientAuth.Credentials.ClientSecret != nil {
		secret, _ := commoncfg.ExtractValueFromSourceRef(clientAuth.Credentials.ClientSecret)
		if secret != nil && string(secret) != "" {
			rt.ClientSecret = pointers.To(string(secret))
		}
	}

	// ---- Load private_key_jwt authentication ----
	if clientAuth.Credentials.ClientAssertion != nil {
		assertion, _ := commoncfg.ExtractValueFromSourceRef(clientAuth.Credentials.ClientAssertion)
		if assertion != nil && string(assertion) != "" {
			rt.ClientAssertion = pointers.To(string(assertion))
		}
	}

	if clientAuth.Credentials.ClientAssertionType != nil &&
		*clientAuth.Credentials.ClientAssertionType != "" {
		rt.ClientAssertionType = clientAuth.Credentials.ClientAssertionType
	}

	// ---- Validation: must not mix auth methods ----
	if rt.ClientSecret != nil && rt.ClientAssertion != nil {
		return nil, errors.New("invalid OAuth2 config: both clientSecret and clientAssertion provided")
	}

	return &http.Client{
		Transport: rt,
	}, nil
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
