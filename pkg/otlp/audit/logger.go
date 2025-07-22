package otlpaudit

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"gopkg.in/yaml.v3"
	"net/http"
	"time"

	"github.com/openkcm/common-sdk/pkg/commoncfg"
)

type AuditLogger struct {
	client          otlpClient
	additionalProps map[string]string
}

type otlpClient struct {
	Endpoint  string
	Client    *http.Client
	BasicAuth *basicAuth
}

type basicAuth struct {
	username, password string
}

func NewLogger(config *commoncfg.Audit) (*AuditLogger, error) {
	var b basicAuth
	tr := &http.Transport{
		MaxIdleConns:    10,
		IdleConnTimeout: 30 * time.Second,
	}
	if config.MTLS != nil {
		tlsConfig, err := loadTLSConfig(config)
		if err != nil {
			return nil, err
		}
		tr.TLSClientConfig = tlsConfig
	} else if config.BasicAuth != nil {
		var err error
		b, err = loadBasicAuth(config, b)
		if err != nil {
			return nil, err
		}
	}
	var m map[string]string
	if err := yaml.Unmarshal([]byte(config.AdditionalProperties), &m); err != nil {
		return nil, err
	}

	return &AuditLogger{
		client: otlpClient{
			Endpoint: config.Endpoint,
			Client: &http.Client{
				Transport: tr,
				Timeout:   30 * time.Second,
			},
			BasicAuth: &b,
		},
		additionalProps: m,
	}, nil
}

func loadBasicAuth(config *commoncfg.Audit, b basicAuth) (basicAuth, error) {
	u, err := commoncfg.LoadValueFromSourceRef(config.BasicAuth.Username)
	if err != nil {
		return basicAuth{}, errors.Join(errLoadValue, err)
	}
	p, err := commoncfg.LoadValueFromSourceRef(config.BasicAuth.Password)
	if err != nil {
		return basicAuth{}, errors.Join(errLoadValue, err)
	}
	b = basicAuth{
		username: string(u),
		password: string(p),
	}
	return b, nil
}

func loadTLSConfig(config *commoncfg.Audit) (*tls.Config, error) {
	clientCert, err := commoncfg.LoadValueFromSourceRef(config.MTLS.Cert)
	if err != nil {
		return nil, errors.Join(errLoadMTLSConfigFailed, err)
	}
	clientKey, err := commoncfg.LoadValueFromSourceRef(config.MTLS.CertKey)
	if err != nil {
		return nil, err
	}
	cert, err := tls.X509KeyPair(clientCert, clientKey)
	if err != nil {
		return nil, errors.Join(errLoadMTLSConfigFailed, err)
	}
	serverCA, err := commoncfg.LoadValueFromSourceRef(config.MTLS.ServerCA)
	if err != nil {
		return nil, errors.Join(errLoadMTLSConfigFailed, err)
	}
	rootCAs := x509.NewCertPool()
	rootCAs.AppendCertsFromPEM(serverCA)
	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{cert},
		RootCAs:      rootCAs,
	}
	return tlsConfig, nil
}
