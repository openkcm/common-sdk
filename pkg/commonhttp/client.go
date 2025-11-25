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
