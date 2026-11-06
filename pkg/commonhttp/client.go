package commonhttp

import (
	"crypto/tls"
	"errors"
	"fmt"
	"net/http"

	"github.com/openkcm/common-sdk/pkg/commoncfg"
)

// NewHTTPClient creates an *http.Client using the full HTTPClient configuration.
//
// It supports the following authentication methods:
//   - Basic Auth
//   - OAuth2 (all supported grant types & auth methods)
//   - API Token authentication
//
// It also configures:
//   - TLS configuration (optional mTLS)
//   - Transport attributes (timeouts, connection pooling)
//   - Global client timeout
//   - Automatic retry with exponential backoff (when Retry.MaxAttemps > 0 and RetryableStatusCodes is set)
//
// Important behaviour:
//   - If an authentication method is used, the factory returns a client
//     whose Transport is a wrapped RoundTripper (e.g., OAuth2, BasicAuth).
//   - This function **preserves** that RoundTripper and wraps it with
//     a proper `http.Transport` when TLS or transport attributes must be applied.
//   - This avoids overwriting authentication transport logic.
//   - The retry layer is applied outermost, so every attempt goes through the full
//     auth and transport chain (e.g. OAuth2 token refresh is applied on each retry).
func NewHTTPClient(cfg *commoncfg.HTTPClient) (*http.Client, error) {
	if cfg == nil {
		return nil, errors.New("HTTPClient config is nil")
	}

	var (
		client *http.Client
		err    error
	)

	// Select authentication mechanism (if any)
	switch {
	case cfg.BasicAuth != nil:
		client, err = NewClientFromBasic(cfg.BasicAuth)
	case cfg.OAuth2Auth != nil:
		client, err = NewClientFromOAuth2(cfg.OAuth2Auth)
	case cfg.APIToken != nil:
		client, err = NewClientFromAPIToken(cfg.APIToken)
	default:
		// No authentication → start with a default client
		client = &http.Client{Transport: http.DefaultTransport}
	}

	if err != nil {
		return nil, err
	}

	// Start building TLS configuration
	var tlsConfig *tls.Config
	if cfg.MTLS != nil {
		tlsConfig, err = commoncfg.LoadMTLSConfig(cfg.MTLS)
		if err != nil {
			return nil, fmt.Errorf("failed to load tls config: %w", err)
		}
	} else {
		// keep default system roots if no mTLS is used
		tlsConfig = &tls.Config{}
	}

	// Build a proper base transport
	baseTransport := &http.Transport{
		TLSClientConfig: tlsConfig,
	}
	if cfg.TransportAttributes != nil {
		baseTransport.TLSHandshakeTimeout = cfg.TransportAttributes.TLSHandshakeTimeout
		baseTransport.DisableKeepAlives = cfg.TransportAttributes.DisableKeepAlives
		baseTransport.DisableCompression = cfg.TransportAttributes.DisableCompression
		baseTransport.MaxIdleConns = cfg.TransportAttributes.MaxIdleConns
		baseTransport.MaxIdleConnsPerHost = cfg.TransportAttributes.MaxIdleConnsPerHost
		baseTransport.MaxConnsPerHost = cfg.TransportAttributes.MaxConnsPerHost
		baseTransport.IdleConnTimeout = cfg.TransportAttributes.IdleConnTimeout
		baseTransport.ResponseHeaderTimeout = cfg.TransportAttributes.ResponseHeaderTimeout
		baseTransport.ExpectContinueTimeout = cfg.TransportAttributes.ExpectContinueTimeout
	}

	// Authentication-aware clients already set their own custom RoundTrippers.
	//    We must wrap the existing one with our transport (do NOT overwrite it).
	switch t := client.Transport.(type) {
	case *clientOAuth2RoundTripper:
		t.Next = baseTransport
	case *clientAPITokenRoundTripper:
		t.Next = baseTransport
	case *clientBasicRoundTripper:
		t.Next = baseTransport
	case *http.Transport:
		client.Transport = baseTransport
	default:
		client.Transport = baseTransport
	}

	// Set global timeout
	client.Timeout = cfg.Timeout

	mc, err := newMetricsClient(client.Transport)
	if err != nil {
		return nil, fmt.Errorf("failed to initialise HTTP client metrics: %w", err)
	}

	client.Transport = mc

	// Wrap with retry outermost so it applies regardless of auth transport.
	if cfg.Retry.MaxAttemps > 0 && len(cfg.Retry.RetryableStatusCodes) > 0 {
		client.Transport = &retryableClient{
			Next:  client.Transport,
			Retry: cfg.Retry,
		}
	}

	return client, nil
}
