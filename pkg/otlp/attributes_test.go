package otlp_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/otel/attribute"

	"github.com/openkcm/common-sdk/v2/pkg/commoncfg"
	"github.com/openkcm/common-sdk/v2/pkg/otlp"
)

func TestCreateAttributesFrom(t *testing.T) {
	t.Run("adds environment and service name", func(t *testing.T) {
		appCfg := commoncfg.Application{
			Environment: "prod",
			Name:        "test-service",
		}

		attrs := otlp.CreateAttributesFrom(appCfg)

		assert.Contains(t, attrs, attribute.String(commoncfg.AttrEnvironment, "prod"))
		assert.Contains(t, attrs, attribute.String(commoncfg.AttrServiceName, "test-service"))
	})

	t.Run("adds labels as attributes", func(t *testing.T) {
		appCfg := commoncfg.Application{
			Environment: "dev",
			Name:        "label-test",
			Labels: map[string]string{
				"team":    "core",
				"version": "v1",
			},
		}

		attrs := otlp.CreateAttributesFrom(appCfg)

		assert.Contains(t, attrs, attribute.String("team", "core"))
		assert.Contains(t, attrs, attribute.String("version", "v1"))
	})

	t.Run("includes additional passed attributes", func(t *testing.T) {
		appCfg := commoncfg.Application{
			Environment: "staging",
			Name:        "extra-test",
		}

		extras := []attribute.KeyValue{
			attribute.String("custom", "value"),
			attribute.Bool("enabled", true),
		}

		attrs := otlp.CreateAttributesFrom(appCfg, extras...)

		assert.Contains(t, attrs, attribute.String("custom", "value"))
		assert.Contains(t, attrs, attribute.Bool("enabled", true))
	})
}
