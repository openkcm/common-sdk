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
//   - Automatic retry with exponential backoff (when Retry.MaxAttempts > 0 and RetryableStatusCodes is set)
//
// Important behaviour:
//   - If an authentication method is used, the factory returns a client
//     whose Transport is a wrapped RoundTripper (e.g., OAuth2, BasicAuth).
//   - The authentication wrapper is preserved. Its underlying transport is only
//     replaced when TLS or transport attributes are configured; otherwise default
//     proxy behavior and shared connection pooling are retained.
//   - Each attempt gets a CLIENT span and passes through logging, metrics and auth.
//   - When retries are enabled, an INTERNAL span covers the retry loop and backoff.
//     It ends at the final response headers; attempt spans end at body EOF/Close.
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

	// Retain the authentication factory's transport (normally the shared
	// http.DefaultTransport) unless configuration explicitly overrides it.
	// This preserves proxy behavior and connection pooling for dynamic clients.
	if cfg.MTLS != nil || cfg.TransportAttributes != nil {
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
	}

	// Set global timeout
	client.Timeout = cfg.Timeout

	if cfg.Log {
		client.Transport = &loggingClient{Next: client.Transport}
	}

	// Instrument attempts, including the context used by logging and metrics.
	client.Transport, err = newTelemetryTransport(client.Transport)
	if err != nil {
		return nil, fmt.Errorf("failed to initialise HTTP client telemetry: %w", err)
	}

	// The retry transport owns the operation span without recording HTTP metrics.
	if cfg.Retry.MaxAttempts > 0 && len(cfg.Retry.RetryableStatusCodes) > 0 {
		client.Transport = &retryableClient{
			Next:  client.Transport,
			Retry: cfg.Retry,
		}
	}

	return client, nil
}
