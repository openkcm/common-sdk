package otlpaudit

import (
	"net/http"

	"gopkg.in/yaml.v3"

	"github.com/openkcm/common-sdk/pkg/commoncfg"
	"github.com/openkcm/common-sdk/pkg/commonhttp"
)

type AuditLogger struct {
	client          otlpClient
	additionalProps map[string]string
}

type otlpClient struct {
	Endpoint string
	Client   *http.Client
}

func NewLogger(config *commoncfg.Audit) (*AuditLogger, error) {
	client, err := commonhttp.NewClient(&config.HTTPClient)
	if err != nil {
		return nil, err
	}

	var m map[string]string

	err = yaml.Unmarshal([]byte(config.AdditionalProperties), &m)
	if err != nil {
		return nil, err
	}

	return &AuditLogger{
		client: otlpClient{
			Endpoint: config.Endpoint,
			Client:   client,
		},
		additionalProps: m,
	}, nil
}
