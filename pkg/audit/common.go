package audit

import (
	auditlogeventsv1 "github.com/openkcm/api-sdk/proto/kms/api/cmk/auditlog/events/v1"
)

// CommonOption is a functional option for configuring Common fields.
// CommonOptions should only be used with Auditor.NewCommon() which initializes the Common struct properly.
type CommonOption func(*auditlogeventsv1.Common)

// WithProperty adds a property key-value pair (or overrides one from additionalProps).
func WithProperty(key, value string) CommonOption {
	return func(c *auditlogeventsv1.Common) {
		props := c.GetProperties()
		if props == nil {
			c.SetProperties(map[string]string{key: value})
			return
		}

		props[key] = value
	}
}

// WithTraceContext adds OpenTelemetry trace/span IDs
func WithTraceContext(traceID, spanID string) CommonOption {
	return func(c *auditlogeventsv1.Common) {
		c.SetTraceId(traceID)
		c.SetSpanId(spanID)
	}
}
