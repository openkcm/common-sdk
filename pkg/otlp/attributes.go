package otlp

import (
	"go.opentelemetry.io/otel/attribute"

	"github.com/openkcm/common-sdk/pkg/commoncfg"
)

// CreateAttributesFrom builds a slice of OTEL attributes from the application config and optional extra attributes.
func CreateAttributesFrom(appCfg commoncfg.Application, attrs ...attribute.KeyValue) []attribute.KeyValue {
	attributes := make([]attribute.KeyValue, 0)
	attributes = append(attributes,
		attribute.String(commoncfg.AttrEnvironment, appCfg.Environment),
		attribute.String(commoncfg.AttrService, appCfg.Name),
	)
	for k, v := range appCfg.Labels {
		attributes = append(attributes, attribute.String(k, v))
	}
	attributes = append(attributes, attrs...)

	return attributes
}
