package commonhttp

import (
	"crypto/tls"
	"errors"
	"fmt"
	"net/http"

	"github.com/openkcm/common-sdk/pkg/commoncfg"
)

// NewClient creates an *http.Client configured with optional TLS/mTLS and custom settings.
//
// Supports:
//   - Timeout
//   - TLS minimum version (default TLS1.2)
//   - InsecureSkipVerify
//   - Custom root CAs
//   - Optional client certificates (mTLS)
//
// Deprecated [to be replaced with NewHTTPClient]
func NewClient(cfg *commoncfg.HTTPClient) (*http.Client, error) {
	if cfg == nil {
		return nil, errors.New("HTTPClient config is nil")
	}

	// Base HTTP client with timeout
	client := &http.Client{
		Timeout: cfg.Timeout,
	}

	// Prepare TLS configuration
	tlsConfig := &tls.Config{
		InsecureSkipVerify: cfg.InsecureSkipVerify,
		MinVersion:         tls.VersionTLS12,
	}

	// Override minimum TLS version if provided
	if cfg.MinVersion >= tlsConfig.MinVersion {
		tlsConfig.MinVersion = cfg.MinVersion
	}

	// Load custom root CAs if provided and not skipping verification
	if !cfg.InsecureSkipVerify && cfg.RootCAs != nil {
		certPool, err := commoncfg.LoadCACertPool(cfg.RootCAs)
		if err != nil {
			return nil, fmt.Errorf("failed to load root CAs: %w", err)
		}

		tlsConfig.RootCAs = certPool
	}

	// Load client certificate for mTLS if both Cert and CertKey are provided
	if cfg.Cert != nil && cfg.CertKey != nil {
		cert, err := commoncfg.LoadClientCertificate(cfg.Cert, cfg.CertKey)
		if err != nil {
			return nil, fmt.Errorf("failed to load client certificate: %w", err)
		}

		tlsConfig.Certificates = []tls.Certificate{*cert}
	}

	// Assign custom transport with TLS configuration
	client.Transport = &http.Transport{
		TLSClientConfig: tlsConfig,
	}

	return client, nil
}

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
//
// Important behaviour:
//   - If an authentication method is used, the factory returns a client
//     whose Transport is a wrapped RoundTripper (e.g., OAuth2, BasicAuth).
//   - This function **preserves** that RoundTripper and wraps it with
//     a proper `http.Transport` when TLS or transport attributes must be applied.
//   - This avoids overwriting authentication transport logic.
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
