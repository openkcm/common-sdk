package otlpaudit

import (
	"fmt"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
)

func NewKeyCreateEvent(metadata EventMetadata, objectID, systemID, cmkID string, t KeyType) (plog.Logs, error) {
	m, err := newKeyEvent(KeyCreateEvent, metadata, objectID, systemID, cmkID, t)
	if err != nil {
		return plog.Logs{}, err
	}

	return createEvent(m)
}

func NewKeyDeleteEvent(metadata EventMetadata, objectID, systemID, cmkID string, t KeyType) (plog.Logs, error) {
	m, err := newKeyEvent(KeyDeleteEvent, metadata, objectID, systemID, cmkID, t)
	if err != nil {
		return plog.Logs{}, err
	}

	return createEvent(m)
}

func NewKeyRestoreEvent(metadata EventMetadata, objectID, systemID, cmkID string, t KeyType) (plog.Logs, error) {
	m, err := newKeyEvent(KeyRestoreEvent, metadata, objectID, systemID, cmkID, t)
	if err != nil {
		return plog.Logs{}, err
	}

	return createEvent(m)
}

func NewKeyPurgeEvent(metadata EventMetadata, objectID, systemID, cmkID string, t KeyType) (plog.Logs, error) {
	m, err := newKeyEvent(KeyPurgeEvent, metadata, objectID, systemID, cmkID, t)
	if err != nil {
		return plog.Logs{}, err
	}

	return createEvent(m)
}

func NewKeyRotateEvent(metadata EventMetadata, objectID, systemID, cmkID string, t KeyType) (plog.Logs, error) {
	m, err := newKeyEvent(KeyRotateEvent, metadata, objectID, systemID, cmkID, t)
	if err != nil {
		return plog.Logs{}, err
	}

	return createEvent(m)
}

func NewKeyEnableEvent(metadata EventMetadata, objectID, systemID, cmkID string, t KeyType) (plog.Logs, error) {
	m, err := newKeyEvent(KeyEnableEvent, metadata, objectID, systemID, cmkID, t)
	if err != nil {
		return plog.Logs{}, err
	}

	return createEvent(m)
}

func NewKeyDisableEvent(metadata EventMetadata, objectID, systemID, cmkID string, t KeyType) (plog.Logs, error) {
	m, err := newKeyEvent(KeyDisableEvent, metadata, objectID, systemID, cmkID, t)
	if err != nil {
		return plog.Logs{}, err
	}

	return createEvent(m)
}

func NewWorkflowStartEvent(metadata EventMetadata, objectID, channelID, channelType string, value any, dpp bool) (plog.Logs, error) {
	if !hasValues(channelID, channelType) {
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
	if !hasValues(channelID, channelType) {
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
	if !hasValues(channelID, channelType) {
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
	if !hasValues(channelID, channelType) {
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
	if !hasValues(propertyName) {
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
	m[LoginMethodKey] = unspecifiedIfEmpty(string(l))
	m[MfaTypeKey] = unspecifiedIfEmpty(string(t))
	m[UserTypeKey] = unspecifiedIfEmpty(string(u))
	m[ValueKey] = value

	return createEvent(m)
}

func NewUserLoginFailureEvent(metadata EventMetadata, objectID string, l LoginMethod, f FailReason, value any) (plog.Logs, error) {
	if !l.IsValid() || !f.IsValid() {
		return plog.Logs{}, errEventCreation
	}

	m := newEventProperties(objectID, UserLoginFailureEvent, metadata)
	m[LoginMethodKey] = unspecifiedIfEmpty(string(l))
	m[FailureReasonKey] = unspecifiedIfEmpty(string(f))
	m[ValueKey] = value

	return createEvent(m)
}
func NewTenantOnboardingEvent(metadata EventMetadata, tenantID string) (plog.Logs, error) {
	m := newEventProperties(tenantID, TenantOnboardingEvent, metadata)

	return createEvent(m)
}

func NewTenantOffboardingEvent(metadata EventMetadata, tenantID string) (plog.Logs, error) {
	m := newEventProperties(tenantID, TenantOffboardingEvent, metadata)

	return createEvent(m)
}

func NewTenantUpdateEvent(metadata EventMetadata, objectID, propertyName string, oldValue, newValue any) (plog.Logs, error) {
	if !hasValues(propertyName, oldValue, newValue) {
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
	if !hasValues(value) {
		return plog.Logs{}, errEventCreation
	}

	m := newEventProperties(objectID, ConfigCreateEvent, metadata)
	m[ObjectTypeKey] = configObjectType
	m[PropertyNameKey] = configPropertyName
	m[ValueKey] = value

	return createEvent(m)
}

func NewConfigurationUpdateEvent(metadata EventMetadata, objectID string, oldValue, newValue any) (plog.Logs, error) {
	if !hasValues(oldValue, newValue) {
		return plog.Logs{}, errEventCreation
	}

	m := newEventProperties(objectID, ConfigUpdateEvent, metadata)
	m[ObjectTypeKey] = configObjectType
	m[PropertyNameKey] = configPropertyName
	m[OldValueKey] = oldValue
	m[NewValueKey] = newValue

	return createEvent(m)
}

func NewConfigurationDeleteEvent(metadata EventMetadata, objectID string, value any) (plog.Logs, error) {
	if !hasValues(value) {
		return plog.Logs{}, errEventCreation
	}

	m := newEventProperties(objectID, ConfigDeleteEvent, metadata)
	m[ObjectTypeKey] = configObjectType
	m[PropertyNameKey] = configPropertyName
	m[ValueKey] = value

	return createEvent(m)
}

func NewConfigurationReadEvent(metadata EventMetadata, objectID, channelType, channelID string, value any) (plog.Logs, error) {
	if !hasValues(channelID, channelType, value) {
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

func NewCredentialCreateEvent(metadata EventMetadata, credentialID string, c CredentialType) (plog.Logs, error) {
	if !c.IsValid() {
		return plog.Logs{}, errEventCreation
	}

	m := newEventProperties(credentialID, CredentialCreateEvent, metadata)
	m[CredentialTypeKey] = string(c)

	return createEvent(m)
}

func NewCredentialExpirationEvent(metadata EventMetadata, credentialID string, c CredentialType) (plog.Logs, error) {
	if !c.IsValid() {
		return plog.Logs{}, errEventCreation
	}

	m := newEventProperties(credentialID, CredentialExpirationEvent, metadata)
	m[CredentialTypeKey] = string(c)

	return createEvent(m)
}

func NewCredentialDeleteEvent(metadata EventMetadata, credentialID string, c CredentialType) (plog.Logs, error) {
	if !c.IsValid() {
		return plog.Logs{}, errEventCreation
	}

	m := newEventProperties(credentialID, CredentialDeleteEvent, metadata)
	m[CredentialTypeKey] = string(c)

	return createEvent(m)
}

func NewCredentialRevokationEvent(metadata EventMetadata, credentialID string, c CredentialType) (plog.Logs, error) {
	if !c.IsValid() {
		return plog.Logs{}, errEventCreation
	}

	m := newEventProperties(credentialID, CredentialRevokationEvent, metadata)
	m[CredentialTypeKey] = string(c)

	return createEvent(m)
}

func NewCmkOnboardingEvent(metadata EventMetadata, cmkID, systemID string) (plog.Logs, error) {
	if !hasValues(systemID) {
		return plog.Logs{}, errEventCreation
	}

	m := newEventProperties(cmkID, CmkOnboardingEvent, metadata)
	m[SystemIDKey] = systemID

	return createEvent(m)
}

func NewCmkOffboardingEvent(metadata EventMetadata, cmkID, systemID string) (plog.Logs, error) {
	if !hasValues(systemID) {
		return plog.Logs{}, errEventCreation
	}

	m := newEventProperties(cmkID, CmkOffboardingEvent, metadata)
	m[SystemIDKey] = systemID

	return createEvent(m)
}

func NewCmkSwitchEvent(metadata EventMetadata, systemID, cmkIDOld, cmkIDNew string) (plog.Logs, error) {
	if !hasValues(cmkIDOld, cmkIDNew) {
		return plog.Logs{}, errEventCreation
	}

	m := newEventProperties(systemID, CmkSwitchEvent, metadata)
	m[CmkIDOldKey] = cmkIDOld
	m[CmkIDNewKey] = cmkIDNew

	return createEvent(m)
}

func NewCmkTenantModificationEvent(metadata EventMetadata, cmkID, systemID string, c CmkAction) (plog.Logs, error) {
	if !hasValues(systemID) || !c.IsValid() {
		return plog.Logs{}, errEventCreation
	}

	m := newEventProperties(cmkID, CmkTenantModificationEvent, metadata)
	m[SystemIDKey] = systemID
	m[ObjectTypeKey] = string(c)

	return createEvent(m)
}

func NewCmkTenantDeleteEvent(metadata EventMetadata, cmkID string) (plog.Logs, error) {
	m := newEventProperties(cmkID, CmkTenantDeleteEvent, metadata)
	return createEvent(m)
}

func NewCmkCreateEvent(metadata EventMetadata, cmkID string) (plog.Logs, error) {
	m := newEventProperties(cmkID, CmkCreateEvent, metadata)
	return createEvent(m)
}

func NewCmkDeleteEvent(metadata EventMetadata, cmkID string) (plog.Logs, error) {
	m := newEventProperties(cmkID, CmkDeleteEvent, metadata)
	return createEvent(m)
}

func NewCmkDetachEvent(metadata EventMetadata, cmkID string) (plog.Logs, error) {
	m := newEventProperties(cmkID, CmkDetachEvent, metadata)
	return createEvent(m)
}

func NewCmkRestoreEvent(metadata EventMetadata, cmkID string) (plog.Logs, error) {
	m := newEventProperties(cmkID, CmkRestoreEvent, metadata)
	return createEvent(m)
}

func NewCmkEnableEvent(metadata EventMetadata, cmkID string) (plog.Logs, error) {
	m := newEventProperties(cmkID, CmkEnableEvent, metadata)
	return createEvent(m)
}

func NewCmkDisableEvent(metadata EventMetadata, cmkID string) (plog.Logs, error) {
	m := newEventProperties(cmkID, CmkDisableEvent, metadata)
	return createEvent(m)
}

func NewCmkRotateEvent(metadata EventMetadata, cmkID string) (plog.Logs, error) {
	m := newEventProperties(cmkID, CmkRotateEvent, metadata)
	return createEvent(m)
}

func NewCmkAvailableEvent(metadata EventMetadata, cmkID string) (plog.Logs, error) {
	m := newEventProperties(cmkID, CmkAvailableEvent, metadata)
	return createEvent(m)
}

func NewCmkUnavailableEvent(metadata EventMetadata, cmkID string) (plog.Logs, error) {
	m := newEventProperties(cmkID, CmkUnavailableEvent, metadata)
	return createEvent(m)
}

func NewUnauthorizedRequestEvent(metadata EventMetadata, resource, action string) (plog.Logs, error) {
	uid, ok := metadata[UserInitiatorIDKey]
	if !ok {
		return plog.Logs{}, errEventCreation
	}

	m := newEventProperties(uid, UnauthorizedRequestEvent, metadata)
	m[ResourceKey] = resource
	m[ActionKey] = action

	return createEvent(m)
}

func NewUnauthenticatedRequestEvent(metadata EventMetadata) (plog.Logs, error) {
	uid, ok := metadata[UserInitiatorIDKey]
	if !ok {
		return plog.Logs{}, errEventCreation
	}

	m := newEventProperties(uid, UnauthenticatedRequestEvent, metadata)

	return createEvent(m)
}

func newKeyEvent(keyEventType string, metadata EventMetadata, objectID string, systemID string, cmkID string, t KeyType) (eventProperties, error) {
	if !hasValues(systemID, cmkID) || !t.IsValid() {
		return nil, errEventCreation
	}

	m := newEventProperties(objectID, keyEventType, metadata)
	m[ObjectTypeKey] = unspecifiedIfEmpty(string(t))
	m[SystemIDKey] = systemID
	m[CmkIDKey] = cmkID

	return m, nil
}

func unspecifiedIfEmpty(input string) string {
	if input == "" {
		return UNSPECIFIED
	}

	return input
}

func createEvent(properties eventProperties) (plog.Logs, error) {
	if !properties.hasValues(ObjectIDKey, EventTypeKey, UserInitiatorIDKey, TenantIDKey) {
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

	if properties.hasValues(SystemIDKey) {
		lr.Attributes().PutStr(SystemIDKey, fmt.Sprint(properties[SystemIDKey]))
	}

	if properties.hasValues(CmkIDKey) {
		lr.Attributes().PutStr(CmkIDKey, fmt.Sprint(properties[CmkIDKey]))
	}

	if properties.hasValues(CmkIDOldKey) {
		lr.Attributes().PutStr(CmkIDOldKey, fmt.Sprint(properties[CmkIDOldKey]))
	}

	if properties.hasValues(CmkIDNewKey) {
		lr.Attributes().PutStr(CmkIDNewKey, fmt.Sprint(properties[CmkIDNewKey]))
	}

	if properties.hasValues(ActionTypeKey) {
		lr.Attributes().PutStr(ActionTypeKey, fmt.Sprint(properties[ActionTypeKey]))
	}

	if properties.hasValues(CredentialTypeKey) {
		lr.Attributes().PutStr(CredentialTypeKey, fmt.Sprint(properties[CredentialTypeKey]))
	}

	if properties.hasValues(LoginMethodKey) {
		lr.Attributes().PutStr(LoginMethodKey, fmt.Sprint(properties[LoginMethodKey]))
	}

	if properties.hasValues(MfaTypeKey) {
		lr.Attributes().PutStr(MfaTypeKey, fmt.Sprint(properties[MfaTypeKey]))
	}

	if properties.hasValues(UserTypeKey) {
		lr.Attributes().PutStr(UserTypeKey, fmt.Sprint(properties[UserTypeKey]))
	}

	if properties.hasValues(FailureReasonKey) {
		lr.Attributes().PutStr(FailureReasonKey, fmt.Sprint(properties[FailureReasonKey]))
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
