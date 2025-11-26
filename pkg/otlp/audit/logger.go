package otlpaudit

import (
	"errors"
	"net/http"
	"time"

	"gopkg.in/yaml.v3"

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
		tlsConfig, err := commoncfg.LoadMTLSConfig(config.MTLS)
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

	err := yaml.Unmarshal([]byte(config.AdditionalProperties), &m)
	if err != nil {
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
	u, err := commoncfg.ExtractValueFromSourceRef(&config.BasicAuth.Username)
	if err != nil {
		return basicAuth{}, errors.Join(errLoadValue, err)
	}

	p, err := commoncfg.ExtractValueFromSourceRef(&config.BasicAuth.Password)
	if err != nil {
		return basicAuth{}, errors.Join(errLoadValue, err)
	}

	b = basicAuth{
		username: string(u),
		password: string(p),
	}

	return b, nil
}
