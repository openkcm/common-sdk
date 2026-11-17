package audit

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/types/known/timestamppb"

	auditlogeventsv1 "github.com/openkcm/api-sdk/proto/kms/api/cmk/auditlog/events/v1"
)

func TestWithProperty(t *testing.T) {
	t.Run("adds property to initialized map", func(t *testing.T) {
		common := &auditlogeventsv1.Common{}
		common.SetTimestamp(timestamppb.Now())
		common.SetProperties(make(map[string]string))

		opt := WithProperty("key", "value")
		opt(common)

		assert.Equal(t, "value", common.GetProperties()["key"])
	})

	t.Run("initializes map if nil", func(t *testing.T) {
		common := &auditlogeventsv1.Common{}
		common.SetTimestamp(timestamppb.Now())
		// Note: Properties map is NOT initialized

		opt := WithProperty("key", "value")
		opt(common) // Should not panic

		assert.Equal(t, "value", common.GetProperties()["key"])
	})
}

func TestWithTraceContext(t *testing.T) {
	common := &auditlogeventsv1.Common{}
	common.SetTimestamp(timestamppb.Now())
	common.SetProperties(make(map[string]string))

	traceID := "4bf92f3577b34da6a3ce929d0e0e4736"
	spanID := "00f067aa0ba902b7"

	opt := WithTraceContext(traceID, spanID)
	opt(common)

	assert.Equal(t, traceID, common.GetTraceId())
	assert.Equal(t, spanID, common.GetSpanId())
}

func TestCommonOptions_Chaining(t *testing.T) {
	common := &auditlogeventsv1.Common{}
	common.SetTimestamp(timestamppb.Now())
	common.SetProperties(make(map[string]string))

	opts := []CommonOption{
		WithProperty("key1", "value1"),
		WithProperty("key2", "value2"),
		WithTraceContext("4bf92f3577b34da6a3ce929d0e0e4736", "00f067aa0ba902b7"),
	}

	for _, opt := range opts {
		opt(common)
	}

	assert.Equal(t, "value1", common.GetProperties()["key1"])
	assert.Equal(t, "value2", common.GetProperties()["key2"])
	assert.Equal(t, "4bf92f3577b34da6a3ce929d0e0e4736", common.GetTraceId())
	assert.Equal(t, "00f067aa0ba902b7", common.GetSpanId())
}
