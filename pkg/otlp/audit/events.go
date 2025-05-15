package otlpaudit

import (
	"fmt"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
)

func NewKeyCreateEvent(metadata EventMetadata, objectID string, l KeyLevel, t KeyCreateActionType, value any, dpp bool) (plog.Logs, error) {
	if !t.IsValid() || !l.IsValid() {
		return plog.Logs{}, errEventCreation
	}

	m := newEventProperties(objectID, KeyCreateEvent, metadata)
	m[ObjectTypeKey] = string(l)
	m[ActionTypeKey] = string(t)
	m[ValueKey] = value
	m[DppKey] = dpp

	return createEvent(m)
}

func NewKeyDeleteEvent(metadata EventMetadata, objectID string, l KeyLevel, value any, dpp bool) (plog.Logs, error) {
	if !l.IsValid() {
		return plog.Logs{}, errEventCreation
	}

	m := newEventProperties(objectID, KeyDeleteEvent, metadata)
	m[ObjectTypeKey] = string(l)
	m[ActionTypeKey] = deleteActionType
	m[ValueKey] = value
	m[DppKey] = dpp

	return createEvent(m)
}

func NewKeyUpdateEvent(metadata EventMetadata, objectID, propertyName string, l KeyLevel, t KeyUpdateActionType, oldValue, newValue any, dpp bool) (plog.Logs, error) {
	if propertyName == "" || !t.IsValid() || !l.IsValid() {
		return plog.Logs{}, errEventCreation
	}

	m := newEventProperties(objectID, KeyUpdateEvent, metadata)
	m[PropertyNameKey] = propertyName
	m[ObjectTypeKey] = string(l)
	m[ActionTypeKey] = string(t)
	m[OldValueKey] = oldValue
	m[NewValueKey] = newValue
	m[DppKey] = dpp

	return createEvent(m)
}

func NewKeyReadEvent(metadata EventMetadata, objectID, channelType, channelID string, l KeyLevel, t KeyReadActionType, value any, dpp bool) (plog.Logs, error) {
	if !l.IsValid() || !t.IsValid() || channelID == "" || channelType == "" {
		return plog.Logs{}, errEventCreation
	}

	m := newEventProperties(objectID, KeyReadEvent, metadata)
	m[ObjectTypeKey] = string(l)
	m[ActionTypeKey] = string(t)
	m[ChannelTypeKey] = channelType
	m[ChannelIDKey] = channelID
	m[ValueKey] = value
	m[DppKey] = dpp

	return createEvent(m)
}

func NewWorkflowStartEvent(metadata EventMetadata, objectID, channelID, channelType string, value any, dpp bool) (plog.Logs, error) {
	if channelID == "" || channelType == "" {
		return plog.Logs{}, errEventCreation
	}

	m := newEventProperties(objectID, WorkflowStartEvent, metadata)
	m[ObjectTypeKey] = workflowObjectType
	m[ChannelTypeKey] = channelType
	m[ChannelIDKey] = channelID
	m[ValueKey] = value
	m[DppKey] = dpp

	return createEvent(m)
}

func NewWorkflowUpdateEvent(metadata EventMetadata, objectID string, oldValue, newValue any, dpp bool) (plog.Logs, error) {
	m := newEventProperties(objectID, WorkflowUpdateEvent, metadata)
	m[ObjectTypeKey] = workflowObjectType
	m[OldValueKey] = oldValue
	m[NewValueKey] = newValue
	m[DppKey] = dpp

	return createEvent(m)
}

func NewWorkflowExecuteEvent(metadata EventMetadata, objectID, channelID, channelType string, value any, dpp bool) (plog.Logs, error) {
	if channelID == "" || channelType == "" {
		return plog.Logs{}, errEventCreation
	}

	m := newEventProperties(objectID, WorkflowExecuteEvent, metadata)
	m[ObjectTypeKey] = workflowObjectType
	m[ChannelTypeKey] = channelType
	m[ChannelIDKey] = channelID
	m[ValueKey] = value
	m[DppKey] = dpp

	return createEvent(m)
}

func NewWorkflowTerminateEvent(metadata EventMetadata, objectID, channelID, channelType string, value any, dpp bool) (plog.Logs, error) {
	if channelID == "" || channelType == "" {
		return plog.Logs{}, errEventCreation
	}

	m := newEventProperties(objectID, WorkflowTerminateEvent, metadata)
	m[ObjectTypeKey] = workflowObjectType
	m[ChannelTypeKey] = channelType
	m[ChannelIDKey] = channelID
	m[ValueKey] = value
	m[DppKey] = dpp

	return createEvent(m)
}

func NewGroupCreateEvent(metadata EventMetadata, objectID string, value any, dpp bool) (plog.Logs, error) {
	m := newEventProperties(objectID, GroupCreateEvent, metadata)
	m[ObjectTypeKey] = groupObjectType
	m[ValueKey] = value
	m[DppKey] = dpp

	return createEvent(m)
}
func NewGroupReadEvent(metadata EventMetadata, objectID, channelID, channelType string, value any, dpp bool) (plog.Logs, error) {
	if channelID == "" || channelType == "" {
		return plog.Logs{}, errEventCreation
	}

	m := newEventProperties(objectID, GroupReadEvent, metadata)
	m[ObjectTypeKey] = groupObjectType
	m[ChannelIDKey] = channelID
	m[ChannelTypeKey] = channelType
	m[ValueKey] = value
	m[DppKey] = dpp

	return createEvent(m)
}
func NewGroupDeleteEvent(metadata EventMetadata, objectID string, value any, dpp bool) (plog.Logs, error) {
	m := newEventProperties(objectID, GroupDeleteEvent, metadata)
	m[ObjectTypeKey] = groupObjectType
	m[ValueKey] = value
	m[DppKey] = dpp

	return createEvent(m)
}
func NewGroupUpdateEvent(metadata EventMetadata, objectID, propertyName string, oldValue, newValue any, dpp bool) (plog.Logs, error) {
	if propertyName == "" {
		return plog.Logs{}, errEventCreation
	}

	m := newEventProperties(objectID, GroupUpdateEvent, metadata)
	m[ObjectTypeKey] = groupObjectType
	m[PropertyNameKey] = propertyName
	m[OldValueKey] = oldValue
	m[NewValueKey] = newValue
	m[DppKey] = dpp

	return createEvent(m)
}
func NewUserLoginSuccessEvent(metadata EventMetadata, objectID string, l LoginMethod, t MfaType, u UserType, value any) (plog.Logs, error) {
	if !l.IsValid() || !t.IsValid() || !u.IsValid() {
		return plog.Logs{}, errEventCreation
	}

	m := newEventProperties(objectID, UserLoginSuccessEvent, metadata)
	m[LoginMethodKey] = string(l)
	m[MfaTypeKey] = string(t)
	m[UserTypeKey] = string(u)
	m[ValueKey] = value

	return createEvent(m)
}

func NewUserLoginFailureEvent(metadata EventMetadata, objectID string, l LoginMethod, f FailReason, value any) (plog.Logs, error) {
	if !l.IsValid() || !f.IsValid() {
		return plog.Logs{}, errEventCreation
	}

	m := newEventProperties(objectID, UserLoginFailureEvent, metadata)
	m[LoginMethodKey] = string(l)
	m[FailureReasonKey] = string(f)
	m[ValueKey] = value

	return createEvent(m)
}
func NewTenantOnboardingEvent(metadata EventMetadata, objectID string, value any) (plog.Logs, error) {
	m := newEventProperties(objectID, TenantOnboardingEvent, metadata)
	m[ObjectTypeKey] = tenantObjectType
	m[ValueKey] = value

	return createEvent(m)
}

func NewTenantOffboardingEvent(metadata EventMetadata, objectID string, value any) (plog.Logs, error) {
	m := newEventProperties(objectID, TenantOffboardingEvent, metadata)
	m[ObjectTypeKey] = tenantObjectType
	m[ValueKey] = value

	return createEvent(m)
}

func NewTenantUpdateEvent(metadata EventMetadata, objectID, propertyName string, t TenantUpdateActionType, oldValue, newValue any) (plog.Logs, error) {
	if propertyName == "" || !t.IsValid() {
		return plog.Logs{}, errEventCreation
	}

	m := newEventProperties(objectID, TenantUpdateEvent, metadata)
	m[ObjectTypeKey] = tenantObjectType
	m[PropertyNameKey] = propertyName
	m[OldValueKey] = oldValue
	m[NewValueKey] = newValue

	return createEvent(m)
}

func NewConfigurationCreateEvent(metadata EventMetadata, objectID string, value any) (plog.Logs, error) {
	m := newEventProperties(objectID, ConfigCreateEvent, metadata)
	m[ObjectTypeKey] = configObjectType
	m[PropertyNameKey] = configPropertyName
	m[ValueKey] = value

	return createEvent(m)
}

func NewConfigurationUpdateEvent(metadata EventMetadata, objectID string, oldValue, newValue any) (plog.Logs, error) {
	m := newEventProperties(objectID, ConfigUpdateEvent, metadata)
	m[ObjectTypeKey] = configObjectType
	m[PropertyNameKey] = configPropertyName
	m[OldValueKey] = oldValue
	m[NewValueKey] = newValue

	return createEvent(m)
}

func NewConfigurationDeleteEvent(metadata EventMetadata, objectID string, value any) (plog.Logs, error) {
	m := newEventProperties(objectID, ConfigDeleteEvent, metadata)
	m[ObjectTypeKey] = configObjectType
	m[PropertyNameKey] = configPropertyName
	m[ValueKey] = value

	return createEvent(m)
}

func NewConfigurationReadEvent(metadata EventMetadata, objectID, channelType, channelID string, value any) (plog.Logs, error) {
	if channelType == "" || channelID == "" {
		return plog.Logs{}, errEventCreation
	}

	m := newEventProperties(objectID, ConfigReadEvent, metadata)
	m[ObjectTypeKey] = configObjectType
	m[ChannelTypeKey] = channelType
	m[ChannelIDKey] = channelID
	m[PropertyNameKey] = configPropertyName
	m[ValueKey] = value

	return createEvent(m)
}

func NewCredentialCreateEvent(metadata EventMetadata, credentialID string, c CredentialType, value any) (plog.Logs, error) {
	if !c.IsValid() {
		return plog.Logs{}, errEventCreation
	}

	m := newEventProperties(credentialID, CredentialCreateEvent, metadata)
	m[CredentialTypeKey] = string(c)
	m[ValueKey] = value

	return createEvent(m)
}

func NewCredentialExpirationEvent(metadata EventMetadata, credentialID string, c CredentialType, value any) (plog.Logs, error) {
	if !c.IsValid() {
		return plog.Logs{}, errEventCreation
	}

	m := newEventProperties(credentialID, CredentialExpirationEvent, metadata)
	m[CredentialTypeKey] = string(c)
	m[ValueKey] = value

	return createEvent(m)
}

func NewCredentialDeleteEvent(metadata EventMetadata, credentialID string, c CredentialType, value any) (plog.Logs, error) {
	if !c.IsValid() {
		return plog.Logs{}, errEventCreation
	}

	m := newEventProperties(credentialID, CredentialDeleteEvent, metadata)
	m[CredentialTypeKey] = string(c)
	m[ValueKey] = value

	return createEvent(m)
}

func NewCredentialRevokationEvent(metadata EventMetadata, credentialID string, c CredentialType, value any) (plog.Logs, error) {
	if !c.IsValid() {
		return plog.Logs{}, errEventCreation
	}

	m := newEventProperties(credentialID, CredentialRevokationEvent, metadata)
	m[CredentialTypeKey] = string(c)
	m[ValueKey] = value

	return createEvent(m)
}

func createEvent(properties eventProperties) (plog.Logs, error) {
	if !properties.hasValues(ObjectIDKey, EventTypeKey, UserInitiatorIDKey, TenantIDKey) {
		return plog.NewLogs(), errEventCreation
	}
	if isOneOf(fmt.Sprint(properties[EventTypeKey]), ConfigCreateEvent, ConfigDeleteEvent, ConfigReadEvent) && !properties.hasValues(ValueKey) {
		return plog.NewLogs(), errEventCreation
	}
	if (fmt.Sprint(properties[EventTypeKey]) == ConfigUpdateEvent || fmt.Sprint(properties[EventTypeKey]) == TenantUpdateEvent) && !properties.hasValues(OldValueKey, NewValueKey) {
		return plog.NewLogs(), errEventCreation
	}
	logs := plog.NewLogs()
	lr := logs.ResourceLogs().AppendEmpty().ScopeLogs().AppendEmpty().LogRecords().AppendEmpty()

	lr.SetEventName(fmt.Sprint(properties[ObjectIDKey]))
	lr.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))

	lr.Attributes().PutStr(EventTypeKey, fmt.Sprint(properties[EventTypeKey]))
	lr.Attributes().PutStr(ObjectIDKey, fmt.Sprint(properties[ObjectIDKey]))
	lr.Attributes().PutStr(UserInitiatorIDKey, fmt.Sprint(properties[UserInitiatorIDKey]))
	lr.Attributes().PutStr(TenantIDKey, fmt.Sprint(properties[TenantIDKey]))

	if properties.hasValues(EventCorrelationIDKey) {
		lr.Attributes().PutStr(EventCorrelationIDKey, fmt.Sprint(properties[EventCorrelationIDKey]))
	}
	if properties.hasValues(ObjectTypeKey) {
		lr.Attributes().PutStr(ObjectTypeKey, fmt.Sprint(properties[ObjectTypeKey]))
	}
	if properties.hasValues(PropertyNameKey) {
		lr.Attributes().PutStr(PropertyNameKey, fmt.Sprint(properties[PropertyNameKey]))
	}
	if properties.hasValues(ChannelIDKey) {
		lr.Attributes().PutStr(ChannelIDKey, fmt.Sprint(properties[ChannelIDKey]))
	}
	if properties.hasValues(ChannelTypeKey) {
		lr.Attributes().PutStr(ChannelTypeKey, fmt.Sprint(properties[ChannelTypeKey]))
	}
	if properties.hasValues(ActionTypeKey) {
		lr.Attributes().PutStr(ActionTypeKey, fmt.Sprint(properties[ActionTypeKey]))
	}
	if fmt.Sprint(properties[EventTypeKey]) == UserLoginSuccessEvent {
		if properties.hasValues(LoginMethodKey) {
			lr.Attributes().PutStr(LoginMethodKey, fmt.Sprint(properties[LoginMethodKey]))
		} else {
			lr.Attributes().PutStr(LoginMethodKey, UNSPECIFIED)
		}
		if properties.hasValues(MfaTypeKey) {
			lr.Attributes().PutStr(MfaTypeKey, fmt.Sprint(properties[MfaTypeKey]))
		} else {
			lr.Attributes().PutStr(MfaTypeKey, UNSPECIFIED)
		}
		if properties.hasValues(UserTypeKey) {
			lr.Attributes().PutStr(UserTypeKey, fmt.Sprint(properties[UserTypeKey]))
		} else {
			lr.Attributes().PutStr(UserTypeKey, UNSPECIFIED)
		}
	}
	if fmt.Sprint(properties[EventTypeKey]) == UserLoginFailureEvent {
		if properties.hasValues(LoginMethodKey) {
			lr.Attributes().PutStr(LoginMethodKey, fmt.Sprint(properties[LoginMethodKey]))
		} else {
			lr.Attributes().PutStr(LoginMethodKey, UNSPECIFIED)
		}
		if properties.hasValues(FailureReasonKey) {
			lr.Attributes().PutStr(FailureReasonKey, fmt.Sprint(properties[FailureReasonKey]))
		} else {
			lr.Attributes().PutStr(FailureReasonKey, UNSPECIFIED)
		}
	}
	if isOneOf(properties[EventTypeKey], CredentialCreateEvent, CredentialExpirationEvent, CredentialRevokationEvent, CredentialDeleteEvent) {
		lr.Attributes().PutStr(CredentialTypeKey, fmt.Sprint(properties[CredentialTypeKey]))
	}
	if properties.hasValues(DppKey) {
		lr.Attributes().PutStr(DppKey, fmt.Sprint(properties[DppKey]))
	}
	if properties.hasValues(OldValueKey) {
		lr.Attributes().PutStr(OldValueKey, fmt.Sprint(properties[OldValueKey]))
	}
	if properties.hasValues(NewValueKey) {
		lr.Attributes().PutStr(NewValueKey, fmt.Sprint(properties[NewValueKey]))
	}
	if properties.hasValues(ValueKey) {
		lr.Attributes().PutStr(ValueKey, fmt.Sprint(properties[ValueKey]))
	}

	return logs, nil
}
