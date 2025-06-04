package otlpaudit

import (
	"reflect"
)

type eventProperties map[string]any

type EventMetadata map[string]string

func (m eventProperties) hasValues(keys ...string) bool {
	for _, k := range keys {
		v, ok := m[k]
		if !ok || isZeroVal(v) {
			return false
		}
	}
	return true
}

func hasValues(values ...any) bool {
	for _, value := range values {
		if isZeroVal(value) {
			return false
		}
	}
	return true
}

func isZeroVal[T comparable](v T) bool {
	valueType := reflect.TypeOf(v)
	if valueType == nil {
		return true
	}
	zeroValue := reflect.Zero(valueType).Interface()
	comparableZero, _ := zeroValue.(T)
	return v == comparableZero
}

func NewEventMetadata(userInitiatorID, tenantID, eventCorrelationID string) (EventMetadata, error) {
	if userInitiatorID == "" || tenantID == "" {
		return nil, errEventCreation
	}
	return EventMetadata{
		UserInitiatorIDKey:    userInitiatorID,
		TenantIDKey:           tenantID,
		EventCorrelationIDKey: eventCorrelationID,
	}, nil
}

func newEventProperties(objectID, eventType string, eventMetadata EventMetadata) eventProperties {
	return eventProperties{
		ObjectIDKey:           objectID,
		EventTypeKey:          eventType,
		UserInitiatorIDKey:    eventMetadata[UserInitiatorIDKey],
		TenantIDKey:           eventMetadata[TenantIDKey],
		EventCorrelationIDKey: eventMetadata[EventCorrelationIDKey],
	}
}
