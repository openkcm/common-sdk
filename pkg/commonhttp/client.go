package commonhttp

import (
	"crypto/tls"
	"errors"
	"fmt"
	"net/http"

	"github.com/openkcm/common-sdk/pkg/commoncfg"
)

// NewClient creates an *http.Client using the full HTTPClient configuration.
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
//
// Important behaviour:
//   - If an authentication method is used, the factory returns a client
//     whose Transport is a wrapped RoundTripper (e.g., OAuth2, BasicAuth).
//   - This function **preserves** that RoundTripper and wraps it with
//     a proper `http.Transport` when TLS or transport attributes must be applied.
//   - This avoids overwriting authentication transport logic.
func NewClient(cfg *commoncfg.HTTPClient) (*http.Client, error) {
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
		TLSClientConfig:       tlsConfig,
		TLSHandshakeTimeout:   cfg.TransportAttributes.TLSHandshakeTimeout,
		DisableKeepAlives:     cfg.TransportAttributes.DisableKeepAlives,
		DisableCompression:    cfg.TransportAttributes.DisableCompression,
		MaxIdleConns:          cfg.TransportAttributes.MaxIdleConns,
		MaxIdleConnsPerHost:   cfg.TransportAttributes.MaxIdleConnsPerHost,
		MaxConnsPerHost:       cfg.TransportAttributes.MaxConnsPerHost,
		IdleConnTimeout:       cfg.TransportAttributes.IdleConnTimeout,
		ResponseHeaderTimeout: cfg.TransportAttributes.ResponseHeaderTimeout,
		ExpectContinueTimeout: cfg.TransportAttributes.ExpectContinueTimeout,
	}

	// Authentication-aware clients already set their own custom RoundTrippers.
	//    We must wrap the existing one with our transport (do NOT overwrite it).
	switch t := client.Transport.(type) {
	// OAuth2 wrapper: set its Next transport
	case *clientOAuth2RoundTripper:
		t.Next = baseTransport

	// API Token wrapper
	case *clientAPITokenRoundTripper:
		t.Next = baseTransport

	// Basic Auth wrapper
	case *clientBasicRoundTripper:
		t.Next = baseTransport

	// Custom transports: do a safe replacement
	case *http.Transport:
		// No custom wrapper → replace directly
		client.Transport = baseTransport

	default:
		// Fallback: wrap unknown transport type
		client.Transport = baseTransport
	}

	// Set global timeout
	client.Timeout = cfg.Timeout

	return client, nil
}
