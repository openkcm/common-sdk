package otlpaudit

const (
	ConfigCreateEvent           = "configurationCreate"
	ConfigReadEvent             = "configurationRead"
	ConfigUpdateEvent           = "configurationUpdate"
	ConfigDeleteEvent           = "configurationDelete"
	GroupCreateEvent            = "groupCreate"
	GroupReadEvent              = "groupRead"
	GroupUpdateEvent            = "groupUpdate"
	GroupDeleteEvent            = "groupDelete"
	KeyCreateEvent              = "keyCreate"
	KeyDeleteEvent              = "keyDelete"
	KeyRestoreEvent             = "keyRestore"
	KeyPurgeEvent               = "keyPurge"
	KeyRotateEvent              = "keyRotate"
	KeyEnableEvent              = "keyEnable"
	KeyDisableEvent             = "keyDisable"
	WorkflowStartEvent          = "workflowStart"
	WorkflowUpdateEvent         = "workflowUpdate"
	WorkflowExecuteEvent        = "workflowExecute"
	WorkflowTerminateEvent      = "workflowTerminate"
	UserLoginSuccessEvent       = "userLoginSuccess"
	UserLoginFailureEvent       = "userLoginFailure"
	TenantOnboardingEvent       = "tenantOnboarding"
	TenantOffboardingEvent      = "tenantOffboarding"
	TenantUpdateEvent           = "tenantUpdate"
	CredentialExpirationEvent   = "credentialExpiration"
	CredentialCreateEvent       = "credentialCreate"
	CredentialRevokationEvent   = "credentialRevokation"
	CredentialDeleteEvent       = "credentialDelete"
	CmkOnboardingEvent          = "cmkOnboarding"
	CmkOffboardingEvent         = "cmkOffboarding"
	CmkSwitchEvent              = "cmkSwitch"
	CmkTenantModificationEvent  = "cmkTenantModification"
	CmkCreateEvent              = "cmkCreate"
	CmkDeleteEvent              = "cmkDelete"
	CmkRestoreEvent             = "cmkRestore"
	CmkEnableEvent              = "cmkEnable"
	CmkDisableEvent             = "cmkDisable"
	CmkRotateEvent              = "cmkRotate"
	UnauthorizedRequestEvent    = "unauthorizedRequest"
	UnauthenticatedRequestEvent = "unauthenticatedRequest"
)

const (
	EventTypeKey          = "eventType"
	ObjectIDKey           = "objectID"
	ObjectTypeKey         = "objectType"
	ActionTypeKey         = "actionType"
	ChannelTypeKey        = "channelType"
	ChannelIDKey          = "channelID"
	LoginMethodKey        = "loginMethod"
	MfaTypeKey            = "mfaType"
	UserTypeKey           = "userType"
	FailureReasonKey      = "failureReason"
	CredentialTypeKey     = "credentialType"
	ValueKey              = "value"
	PropertyNameKey       = "propertyName"
	OldValueKey           = "oldValue"
	NewValueKey           = "newValue"
	DppKey                = "dpp"
	UserInitiatorIDKey    = "userInitiatorID"
	TenantIDKey           = "tenantID"
	EventCorrelationIDKey = "eventCorrelationID"
	SystemIDKey           = "systemID"
	CmkIDKey              = "cmkID"
	CmkIDOldKey           = "cmkIDOld"
	CmkIDNewKey           = "cmkIDNew"
)

const (
	workflowObjectType = "WORKFLOW"
	groupObjectType    = "GROUP"
	tenantObjectType   = "TENANT"
	configObjectType   = "L1L2"
	configPropertyName = "SYSTEM_LINK"
)

func isOneOf[T comparable](val T, trgts ...T) bool {
	for _, trgt := range trgts {
		if trgt == val {
			return true
		}
	}

	return false
}
