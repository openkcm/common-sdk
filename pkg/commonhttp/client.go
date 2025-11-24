package commonhttp

import (
	"crypto/tls"
	"errors"
	"net/http"

	"github.com/openkcm/common-sdk/v2/pkg/commoncfg"
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
