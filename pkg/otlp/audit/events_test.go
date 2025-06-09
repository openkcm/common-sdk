package otlpaudit

import (
	"testing"
)

func TestNewConfigurationCreateEvent(t *testing.T) {
	type args struct {
		metadata EventMetadata
		objectID string
		value    any
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "T300_ConfigCreateEvent_EmptyObjectIDValue_Fail",
			args: args{
				metadata: EventMetadata{
					UserInitiatorIDKey:    "userInitiatorID",
					TenantIDKey:           "tenantID",
					EventCorrelationIDKey: "eventCorrelationID",
				},
				objectID: "",
				value:    nil,
			},
			wantErr: true,
		},
		{
			name: "T301_ConfigCreateEvent_Success",
			args: args{
				metadata: EventMetadata{
					UserInitiatorIDKey:    "userInitiatorID",
					TenantIDKey:           "tenantID",
					EventCorrelationIDKey: "eventCorrelationID",
				},
				objectID: "validObjectID",
				value:    "testValue",
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewConfigurationCreateEvent(tt.args.metadata, tt.args.objectID, tt.args.value)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewConfigurationCreateEvent() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
		})
	}
}

func TestNewConfigurationDeleteEvent(t *testing.T) {
	type args struct {
		metadata EventMetadata
		objectID string
		value    any
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "T400_ConfigDeleteEvent_EmptyObjectIDValue_Fail",
			args: args{
				metadata: EventMetadata{
					UserInitiatorIDKey:    "userInitiatorID",
					TenantIDKey:           "tenantID",
					EventCorrelationIDKey: "eventCorrelationID",
				},
				objectID: "",
				value:    nil,
			},
			wantErr: true,
		},
		{
			name: "T401_ConfigDeleteEvent_Success",
			args: args{
				metadata: EventMetadata{
					UserInitiatorIDKey:    "userInitiatorID",
					TenantIDKey:           "tenantID",
					EventCorrelationIDKey: "eventCorrelationID",
				},
				objectID: "validObjectID",
				value:    "testValue",
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewConfigurationDeleteEvent(tt.args.metadata, tt.args.objectID, tt.args.value)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewConfigurationDeleteEvent() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
		})
	}
}

func TestNewConfigurationReadEvent(t *testing.T) {
	type args struct {
		metadata    EventMetadata
		objectID    string
		channelType string
		channelID   string
		value       any
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "T500_ConfigRead_EmptyObjectIDValue_Fail",
			args: args{
				metadata: EventMetadata{
					UserInitiatorIDKey:    "userInitiatorID",
					TenantIDKey:           "tenantID",
					EventCorrelationIDKey: "eventCorrelationID",
				},
				objectID:    "",
				channelType: "web",
				channelID:   "123-23",
				value:       "someValue",
			},
			wantErr: true,
		},
		{
			name: "T501_ConfigRead_Success",
			args: args{
				metadata: EventMetadata{
					UserInitiatorIDKey:    "userInitiatorID",
					TenantIDKey:           "tenantID",
					EventCorrelationIDKey: "eventCorrelationID",
				},
				objectID:    "validObjectID",
				channelType: "web",
				channelID:   "123-23",
				value:       "testValue",
			},
			wantErr: false,
		},
		{
			name: "T502_ConfigRead_NoValue_Fail",
			args: args{
				metadata: EventMetadata{
					UserInitiatorIDKey:    "userInitiatorID",
					TenantIDKey:           "tenantID",
					EventCorrelationIDKey: "eventCorrelationID",
				},
				objectID:    "validObjectID",
				channelType: "web",
				channelID:   "123-23",
				value:       nil,
			},
			wantErr: true,
		},
		{
			name: "T503_ConfigRead_EmptyChannelType_Fail",
			args: args{
				metadata: EventMetadata{
					UserInitiatorIDKey:    "userInitiatorID",
					TenantIDKey:           "tenantID",
					EventCorrelationIDKey: "eventCorrelationID",
				},
				objectID:  "validObjectID",
				channelID: "123-23",
				value:     nil,
			},
			wantErr: true,
		},
		{
			name: "T503_ConfigRead_EmptyChannelID_Fail",
			args: args{
				metadata: EventMetadata{
					UserInitiatorIDKey:    "userInitiatorID",
					TenantIDKey:           "tenantID",
					EventCorrelationIDKey: "eventCorrelationID",
				},
				objectID:    "validObjectID",
				channelType: "web",
				value:       nil,
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewConfigurationReadEvent(tt.args.metadata, tt.args.objectID, tt.args.channelType, tt.args.channelID, tt.args.value)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewConfigurationReadEvent() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
		})
	}
}

func TestNewConfigurationUpdateEvent(t *testing.T) {
	type args struct {
		metadata EventMetadata
		objectID string
		oldValue any
		newValue any
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "T600_ConfigUpdate_EmptyObjectID_Fail",
			args: args{
				metadata: EventMetadata{
					UserInitiatorIDKey:    "userInitiatorID",
					TenantIDKey:           "tenantID",
					EventCorrelationIDKey: "eventCorrelationID",
				},
				objectID: "",
				oldValue: "oldValue",
				newValue: "newValue",
			},
			wantErr: true,
		},
		{
			name: "T601_ConfigUpdate_Success",
			args: args{
				metadata: EventMetadata{
					UserInitiatorIDKey:    "userInitiatorID",
					TenantIDKey:           "tenantID",
					EventCorrelationIDKey: "eventCorrelationID",
				},
				objectID: "validObjectID",
				oldValue: "oldValue",
				newValue: "newValue",
			},
			wantErr: false,
		},
		{
			name: "T602_ConfigUpdate_NoOldValue_Fail",
			args: args{
				metadata: EventMetadata{
					UserInitiatorIDKey:    "userInitiatorID",
					TenantIDKey:           "tenantID",
					EventCorrelationIDKey: "eventCorrelationID",
				},
				objectID: "validObjectID",
				oldValue: nil,
				newValue: "newValue",
			},
			wantErr: true,
		},
		{
			name: "T603_ConfigUpdate_NoNewValue_Fail",
			args: args{
				metadata: EventMetadata{
					UserInitiatorIDKey:    "userInitiatorID",
					TenantIDKey:           "tenantID",
					EventCorrelationIDKey: "eventCorrelationID",
				},
				objectID: "validObjectID",
				oldValue: "oldValue",
				newValue: nil,
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewConfigurationUpdateEvent(tt.args.metadata, tt.args.objectID, tt.args.oldValue, tt.args.newValue)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewConfigurationUpdateEvent() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
		})
	}
}

func TestNewGroupCreateEvent(t *testing.T) {
	type args struct {
		metadata EventMetadata
		objectID string
		value    any
		dpp      bool
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "T700_GroupCreate_EmptyObjectID_Fail",
			args: args{
				metadata: EventMetadata{
					UserInitiatorIDKey:    "userInitiatorID",
					TenantIDKey:           "tenantID",
					EventCorrelationIDKey: "eventCorrelationID",
				},
				objectID: "",
				value:    "someValue",
				dpp:      true,
			},
			wantErr: true,
		},
		{
			name: "T701_GroupCreate_ValidObjectID_Success",
			args: args{
				metadata: EventMetadata{
					UserInitiatorIDKey:    "userInitiatorID",
					TenantIDKey:           "tenantID",
					EventCorrelationIDKey: "eventCorrelationID",
				},
				objectID: "validObjectID",
				value:    "testValue",
				dpp:      false,
			},
			wantErr: false,
		},
		{
			name: "T702_GroupCreate_NoValue_Success",
			args: args{
				metadata: EventMetadata{
					UserInitiatorIDKey:    "userInitiatorID",
					TenantIDKey:           "tenantID",
					EventCorrelationIDKey: "eventCorrelationID",
				},
				objectID: "validObjectID",
				value:    nil,
				dpp:      true,
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewGroupCreateEvent(tt.args.metadata, tt.args.objectID, tt.args.value, tt.args.dpp)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewGroupCreateEvent() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
		})
	}
}

func TestNewGroupDeleteEvent(t *testing.T) {
	type args struct {
		metadata EventMetadata
		objectID string
		value    any
		dpp      bool
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "T800_GroupDelete_EmptyObjectID_Fail",
			args: args{
				metadata: EventMetadata{
					UserInitiatorIDKey:    "userInitiatorID",
					TenantIDKey:           "tenantID",
					EventCorrelationIDKey: "eventCorrelationID",
				},
				objectID: "",
				value:    "someValue",
				dpp:      true,
			},
			wantErr: true,
		},
		{
			name: "T801_GroupDelete_ValidObjectID_Success",
			args: args{
				metadata: EventMetadata{
					UserInitiatorIDKey:    "userInitiatorID",
					TenantIDKey:           "tenantID",
					EventCorrelationIDKey: "eventCorrelationID",
				},
				objectID: "validObjectID",
				value:    "testValue",
				dpp:      false,
			},
			wantErr: false,
		},
		{
			name: "T802_GroupDelete_NoValue_Success",
			args: args{
				metadata: EventMetadata{
					UserInitiatorIDKey:    "userInitiatorID",
					TenantIDKey:           "tenantID",
					EventCorrelationIDKey: "eventCorrelationID",
				},
				objectID: "validObjectID",
				value:    nil,
				dpp:      true,
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewGroupDeleteEvent(tt.args.metadata, tt.args.objectID, tt.args.value, tt.args.dpp)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewGroupDeleteEvent() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
		})
	}
}

func TestNewGroupReadEvent(t *testing.T) {
	type args struct {
		metadata    EventMetadata
		objectID    string
		channelID   string
		channelType string
		value       any
		dpp         bool
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "T900_GroupRead_EmptyObjectID_Fail",
			args: args{
				metadata: EventMetadata{
					UserInitiatorIDKey:    "userInitiatorID",
					TenantIDKey:           "tenantID",
					EventCorrelationIDKey: "eventCorrelationID",
				},
				objectID:    "",
				channelID:   "123",
				channelType: "web",
				value:       "someValue",
				dpp:         true,
			},
			wantErr: true,
		},
		{
			name: "T901_GroupRead_ValidObjectID_Success",
			args: args{
				metadata: EventMetadata{
					UserInitiatorIDKey:    "userInitiatorID",
					TenantIDKey:           "tenantID",
					EventCorrelationIDKey: "eventCorrelationID",
				},
				objectID:    "validObjectID",
				value:       "testValue",
				channelID:   "12312",
				channelType: "web",
				dpp:         false,
			},
			wantErr: false,
		},
		{
			name: "T902_GroupRead_NoValue_Success",
			args: args{
				metadata: EventMetadata{
					UserInitiatorIDKey:    "userInitiatorID",
					TenantIDKey:           "tenantID",
					EventCorrelationIDKey: "eventCorrelationID",
				},
				objectID:    "validObjectID",
				channelID:   "123",
				channelType: "web",
				value:       nil,
				dpp:         true,
			},
			wantErr: false,
		},
		{
			name: "T903_GroupRead_NoChannelID_Fail",
			args: args{
				metadata: EventMetadata{
					UserInitiatorIDKey:    "userInitiatorID",
					TenantIDKey:           "tenantID",
					EventCorrelationIDKey: "eventCorrelationID",
				},
				objectID:    "validObjectID",
				channelID:   "",
				channelType: "web",
				value:       nil,
				dpp:         true,
			},
			wantErr: true,
		},
		{
			name: "T904_GroupRead_NoChannelType_Fail",
			args: args{
				metadata: EventMetadata{
					UserInitiatorIDKey:    "userInitiatorID",
					TenantIDKey:           "tenantID",
					EventCorrelationIDKey: "eventCorrelationID",
				},
				objectID:    "validObjectID",
				channelID:   "123",
				channelType: "",
				value:       nil,
				dpp:         true,
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewGroupReadEvent(tt.args.metadata, tt.args.objectID, tt.args.channelID, tt.args.channelType, tt.args.value, tt.args.dpp)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewGroupReadEvent() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
		})
	}
}

func TestNewGroupUpdateEvent(t *testing.T) {
	type args struct {
		metadata     EventMetadata
		objectID     string
		propertyName string
		oldValue     any
		newValue     any
		dpp          bool
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "T1001_GroupUpdate_EmptyObjectID_Fail",
			args: args{
				metadata: EventMetadata{
					UserInitiatorIDKey:    "userInitiatorID",
					TenantIDKey:           "tenantID",
					EventCorrelationIDKey: "eventCorrelationID",
				},
				objectID:     "",
				propertyName: "validProperty",
				oldValue:     "oldValue",
				newValue:     "newValue",
				dpp:          true,
			},
			wantErr: true,
		},
		{
			name: "T1002_GroupUpdate_EmptyPropertyName_Fail",
			args: args{
				metadata: EventMetadata{
					UserInitiatorIDKey:    "userInitiatorID",
					TenantIDKey:           "tenantID",
					EventCorrelationIDKey: "eventCorrelationID",
				},
				objectID:     "validObjectID",
				propertyName: "",
				oldValue:     "oldValue",
				newValue:     "newValue",
				dpp:          true,
			},
			wantErr: true,
		},
		{
			name: "T1003_GroupUpdate_ValidObjectIDAndPropertyName_Success",
			args: args{
				metadata: EventMetadata{
					UserInitiatorIDKey:    "userInitiatorID",
					TenantIDKey:           "tenantID",
					EventCorrelationIDKey: "eventCorrelationID",
				},
				objectID:     "validObjectID",
				propertyName: "validProperty",
				oldValue:     "oldValue",
				newValue:     "newValue",
				dpp:          false,
			},
			wantErr: false,
		},
		{
			name: "T1004_GroupUpdate_NoOldValue_Success",
			args: args{
				metadata: EventMetadata{
					UserInitiatorIDKey:    "userInitiatorID",
					TenantIDKey:           "tenantID",
					EventCorrelationIDKey: "eventCorrelationID",
				},
				objectID:     "validObjectID",
				propertyName: "validProperty",
				oldValue:     nil,
				newValue:     "newValue",
				dpp:          true,
			},
			wantErr: false,
		},
		{
			name: "T1005_GroupUpdate_NoNewValue_Success",
			args: args{
				metadata: EventMetadata{
					UserInitiatorIDKey:    "userInitiatorID",
					TenantIDKey:           "tenantID",
					EventCorrelationIDKey: "eventCorrelationID",
				},
				objectID:     "validObjectID",
				propertyName: "validProperty",
				oldValue:     "oldValue",
				newValue:     nil,
				dpp:          true,
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewGroupUpdateEvent(tt.args.metadata, tt.args.objectID, tt.args.propertyName, tt.args.oldValue, tt.args.newValue, tt.args.dpp)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewGroupUpdateEvent() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
		})
	}
}

func TestKeyEvents(t *testing.T) {
	type args struct {
		metadata EventMetadata
		objectID string
		keyType  KeyType
		systemID string
		cmkID    string
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "T1100_KeyEvent_EmptyObjectID_Fail",
			args: args{
				metadata: EventMetadata{
					UserInitiatorIDKey:    "userInitiatorID",
					TenantIDKey:           "tenantID",
					EventCorrelationIDKey: "eventCorrelationID",
				},
				objectID: "",
				keyType:  KEYTYPE_SYSTEM,
				systemID: "1111-11111",
				cmkID:    "1111-11111",
			},
			wantErr: true,
		},
		{
			name: "T1101_KeyEvent_EmptySystemID_Fail",
			args: args{
				metadata: EventMetadata{
					UserInitiatorIDKey:    "userInitiatorID",
					TenantIDKey:           "tenantID",
					EventCorrelationIDKey: "eventCorrelationID",
				},
				objectID: "validObjectID",
				keyType:  KEYTYPE_KEK,
				systemID: "",
				cmkID:    "1111-11111",
			},
			wantErr: true,
		},
		{
			name: "T1102_KeyEvent_InvalidKeyType_Fail",
			args: args{
				metadata: EventMetadata{
					UserInitiatorIDKey:    "userInitiatorID",
					TenantIDKey:           "tenantID",
					EventCorrelationIDKey: "eventCorrelationID",
				},
				objectID: "validObjectID",
				keyType:  KeyType("invalidType"),
				systemID: "1111-11111",
				cmkID:    "1111-11111",
			},
			wantErr: true,
		},
		{
			name: "T1103_KeyEvent_CorrectData_Success",
			args: args{
				metadata: EventMetadata{
					UserInitiatorIDKey:    "userInitiatorID",
					TenantIDKey:           "tenantID",
					EventCorrelationIDKey: "eventCorrelationID",
				},
				objectID: "validObjectID",
				keyType:  KEYTYPE_SYSTEM,
				systemID: "1111-11111",
				cmkID:    "1111-11111",
			},
			wantErr: false,
		},
		{
			name: "T1104_KeyEvent_NoCmkID_Fail",
			args: args{
				metadata: EventMetadata{
					UserInitiatorIDKey:    "userInitiatorID",
					TenantIDKey:           "tenantID",
					EventCorrelationIDKey: "eventCorrelationID",
				},
				objectID: "validObjectID",
				keyType:  KEYTYPE_DATA,
				systemID: "1111-11111",
				cmkID:    "",
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewKeyCreateEvent(tt.args.metadata, tt.args.objectID, tt.args.systemID, tt.args.cmkID, tt.args.keyType)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewKeyCreateEvent() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			_, err = NewKeyDeleteEvent(tt.args.metadata, tt.args.objectID, tt.args.systemID, tt.args.cmkID, tt.args.keyType)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewKeyDeleteEvent() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			_, err = NewKeyRotateEvent(tt.args.metadata, tt.args.objectID, tt.args.systemID, tt.args.cmkID, tt.args.keyType)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewKeyRotateEvent() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			_, err = NewKeyPurgeEvent(tt.args.metadata, tt.args.objectID, tt.args.systemID, tt.args.cmkID, tt.args.keyType)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewKeyPurgeEvent() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			_, err = NewKeyEnableEvent(tt.args.metadata, tt.args.objectID, tt.args.systemID, tt.args.cmkID, tt.args.keyType)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewKeyEnableEvent() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			_, err = NewKeyDisableEvent(tt.args.metadata, tt.args.objectID, tt.args.systemID, tt.args.cmkID, tt.args.keyType)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewKeyDisableEvent() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			_, err = NewKeyRestoreEvent(tt.args.metadata, tt.args.objectID, tt.args.systemID, tt.args.cmkID, tt.args.keyType)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewKeyRestoreEvent() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
		})
	}
}

func TestNewTenantOffboardingEvent(t *testing.T) {
	type args struct {
		metadata EventMetadata
		objectID string
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "T1500_TenantOffboarding_EmptyObjectID_Fail",
			args: args{
				metadata: EventMetadata{
					UserInitiatorIDKey:    "userInitiatorID",
					TenantIDKey:           "tenantID",
					EventCorrelationIDKey: "eventCorrelationID",
				},
				objectID: "",
			},
			wantErr: true,
		},
		{
			name: "T1501_TenantOffboarding_ValidObjectID_Success",
			args: args{
				metadata: EventMetadata{
					UserInitiatorIDKey:    "userInitiatorID",
					TenantIDKey:           "tenantID",
					EventCorrelationIDKey: "eventCorrelationID",
				},
				objectID: "validObjectID",
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewTenantOffboardingEvent(tt.args.metadata, tt.args.objectID)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewTenantOffboardingEvent() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
		})
	}
}

func TestNewTenantOnboardingEvent(t *testing.T) {
	type args struct {
		metadata EventMetadata
		objectID string
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "T1600_TenantOnboarding_EmptyObjectID_Fail",
			args: args{
				metadata: EventMetadata{
					UserInitiatorIDKey:    "userInitiatorID",
					TenantIDKey:           "tenantID",
					EventCorrelationIDKey: "eventCorrelationID",
				},
				objectID: "",
			},
			wantErr: true,
		},
		{
			name: "T1601_TenantOnboarding_ValidObjectID_Success",
			args: args{
				metadata: EventMetadata{
					UserInitiatorIDKey:    "userInitiatorID",
					TenantIDKey:           "tenantID",
					EventCorrelationIDKey: "eventCorrelationID",
				},
				objectID: "validObjectID",
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewTenantOnboardingEvent(tt.args.metadata, tt.args.objectID)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewTenantOnboardingEvent() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
		})
	}
}

func TestNewTenantUpdateEvent(t *testing.T) {
	type args struct {
		metadata     EventMetadata
		objectID     string
		propertyName string
		oldValue     any
		newValue     any
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "T1700_TenantUpdate_EmptyObjectID_Fail",
			args: args{
				metadata: EventMetadata{
					UserInitiatorIDKey:    "userInitiatorID",
					TenantIDKey:           "tenantID",
					EventCorrelationIDKey: "eventCorrelationID",
				},
				objectID:     "",
				propertyName: "validProperty",
				oldValue:     "oldValue",
				newValue:     "newValue",
			},
			wantErr: true,
		},
		{
			name: "T1701_TenantUpdate_ValidObjectIDAndPropertyName_Success",
			args: args{
				metadata: EventMetadata{
					UserInitiatorIDKey:    "userInitiatorID",
					TenantIDKey:           "tenantID",
					EventCorrelationIDKey: "eventCorrelationID",
				},
				objectID:     "validObjectID",
				propertyName: "validProperty",
				oldValue:     "oldValue",
				newValue:     "newValue",
			},
			wantErr: false,
		},
		{
			name: "T1702_TenantUpdate_NoOldValue_Fail",
			args: args{
				metadata: EventMetadata{
					UserInitiatorIDKey:    "userInitiatorID",
					TenantIDKey:           "tenantID",
					EventCorrelationIDKey: "eventCorrelationID",
				},
				objectID:     "validObjectID",
				propertyName: "validProperty",
				oldValue:     nil,
				newValue:     "newValue",
			},
			wantErr: true,
		},
		{
			name: "T1703_TenantUpdate_NoNewValue_Fail",
			args: args{
				metadata: EventMetadata{
					UserInitiatorIDKey:    "userInitiatorID",
					TenantIDKey:           "tenantID",
					EventCorrelationIDKey: "eventCorrelationID",
				},
				objectID:     "validObjectID",
				propertyName: "validProperty",
				oldValue:     "oldValue",
				newValue:     nil,
			},
			wantErr: true,
		},
		{
			name: "T1704_TenantUpdate_EmptyPropertyName_Fail",
			args: args{
				metadata: EventMetadata{
					UserInitiatorIDKey:    "userInitiatorID",
					TenantIDKey:           "tenantID",
					EventCorrelationIDKey: "eventCorrelationID",
				},
				objectID:     "validObjectID",
				propertyName: "",
				oldValue:     "oldValue",
				newValue:     nil,
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewTenantUpdateEvent(tt.args.metadata, tt.args.objectID, tt.args.propertyName, tt.args.oldValue, tt.args.newValue)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewTenantUpdateEvent() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
		})
	}
}

func TestNewUserLoginFailureEvent(t *testing.T) {
	type args struct {
		metadata    EventMetadata
		objectID    string
		loginMethod LoginMethod
		failReason  FailReason
		value       any
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "T1800_UserLoginFailure_EmptyObjectID_Fail",
			args: args{
				metadata: EventMetadata{
					UserInitiatorIDKey:    "userInitiatorID",
					TenantIDKey:           "tenantID",
					EventCorrelationIDKey: "eventCorrelationID",
				},
				objectID:    "",
				loginMethod: LOGINMETHOD_OPENIDCONNECT,
				failReason:  FAILREASON_USERLOCKED,
				value:       "someValue",
			},
			wantErr: true,
		},
		{
			name: "T1801_UserLoginFailure_WrongLoginMethod_Fail",
			args: args{
				metadata: EventMetadata{
					UserInitiatorIDKey:    "userInitiatorID",
					TenantIDKey:           "tenantID",
					EventCorrelationIDKey: "eventCorrelationID",
				},
				objectID:    "validObjectID",
				loginMethod: LoginMethod("invalid method"),
				failReason:  FAILREASON_USERLOCKED,
				value:       "someValue",
			},
			wantErr: true,
		},
		{
			name: "T1802_UserLoginFailure_WrongFailReason_Fail",
			args: args{
				metadata: EventMetadata{
					UserInitiatorIDKey:    "userInitiatorID",
					TenantIDKey:           "tenantID",
					EventCorrelationIDKey: "eventCorrelationID",
				},
				objectID:    "validObjectID",
				loginMethod: LOGINMETHOD_X509CERT,
				failReason:  FailReason("invalid reason"),
				value:       "someValue",
			},
			wantErr: true,
		},
		{
			name: "T1803_UserLoginFailure_ValidObjectIDAndActionType_Success",
			args: args{
				metadata: EventMetadata{
					UserInitiatorIDKey:    "userInitiatorID",
					TenantIDKey:           "tenantID",
					EventCorrelationIDKey: "eventCorrelationID",
				},
				objectID:    "validObjectID",
				loginMethod: LOGINMETHOD_OPENIDCONNECT,
				failReason:  FAILREASON_USERLOCKED,
				value:       "testValue",
			},
			wantErr: false,
		},
		{
			name: "T1804_UserLoginFailure_NoValue_Success",
			args: args{
				metadata: EventMetadata{
					UserInitiatorIDKey:    "userInitiatorID",
					TenantIDKey:           "tenantID",
					EventCorrelationIDKey: "eventCorrelationID",
				},
				objectID:    "validObjectID",
				loginMethod: LOGINMETHOD_OPENIDCONNECT,
				failReason:  FAILREASON_TOKENREVOKED,
				value:       nil,
			},
			wantErr: false,
		},
		{
			name: "T1805_UserLoginFailure_EmptyMethod_Success",
			args: args{
				metadata: EventMetadata{
					UserInitiatorIDKey:    "userInitiatorID",
					TenantIDKey:           "tenantID",
					EventCorrelationIDKey: "eventCorrelationID",
				},
				objectID:    "validObjectID",
				loginMethod: "",
				failReason:  FAILREASON_TOKENREVOKED,
				value:       nil,
			},
			wantErr: false,
		},
		{
			name: "T1806_UserLoginFailure_EmptyReason_Success",
			args: args{
				metadata: EventMetadata{
					UserInitiatorIDKey:    "userInitiatorID",
					TenantIDKey:           "tenantID",
					EventCorrelationIDKey: "eventCorrelationID",
				},
				objectID:    "validObjectID",
				loginMethod: LOGINMETHOD_OPENIDCONNECT,
				failReason:  "",
				value:       nil,
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewUserLoginFailureEvent(tt.args.metadata, tt.args.objectID, tt.args.loginMethod, tt.args.failReason, tt.args.value)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewUserLoginFailureEvent() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
		})
	}
}

func TestNewUserLoginSuccessEvent(t *testing.T) {
	type args struct {
		metadata    EventMetadata
		objectID    string
		loginMethod LoginMethod
		mfaType     MfaType
		userType    UserType
		value       any
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "T1900_UserLoginSuccess_EmptyObjectID_Fail",
			args: args{
				metadata: EventMetadata{
					UserInitiatorIDKey:    "userInitiatorID",
					TenantIDKey:           "tenantID",
					EventCorrelationIDKey: "eventCorrelationID",
				},
				objectID:    "",
				loginMethod: LOGINMETHOD_OPENIDCONNECT,
				mfaType:     MFATYPE_WEBAUTHN,
				userType:    USERTYPE_BUSINESS,
				value:       "someValue",
			},
			wantErr: true,
		},
		{
			name: "T1901_UserLoginSuccess_InvalidLoginMethod_Fail",
			args: args{
				metadata: EventMetadata{
					UserInitiatorIDKey:    "userInitiatorID",
					TenantIDKey:           "tenantID",
					EventCorrelationIDKey: "eventCorrelationID",
				},
				objectID:    "validObjectID",
				loginMethod: LoginMethod("invalid method"),
				mfaType:     MFATYPE_WEBAUTHN,
				userType:    USERTYPE_BUSINESS,
				value:       "testValue",
			},
			wantErr: true,
		},
		{
			name: "T1902_UserLoginSuccess_InvalidMFAType_Fail",
			args: args{
				metadata: EventMetadata{
					UserInitiatorIDKey:    "userInitiatorID",
					TenantIDKey:           "tenantID",
					EventCorrelationIDKey: "eventCorrelationID",
				},
				objectID:    "validObjectID",
				loginMethod: LOGINMETHOD_OPENIDCONNECT,
				mfaType:     MfaType("invalid MFA"),
				userType:    USERTYPE_BUSINESS,
				value:       "testValue",
			},
			wantErr: true,
		},
		{
			name: "T1902_UserLoginSuccess_InvalidUserType_Fail",
			args: args{
				metadata: EventMetadata{
					UserInitiatorIDKey:    "userInitiatorID",
					TenantIDKey:           "tenantID",
					EventCorrelationIDKey: "eventCorrelationID",
				},
				objectID:    "validObjectID",
				loginMethod: LOGINMETHOD_OPENIDCONNECT,
				mfaType:     MFATYPE_WEBAUTHN,
				userType:    UserType("invalid user type"),
				value:       "testValue",
			},
			wantErr: true,
		},
		{
			name: "T1903_UserLoginSuccess_NoValue_Success",
			args: args{
				metadata: EventMetadata{
					UserInitiatorIDKey:    "userInitiatorID",
					TenantIDKey:           "tenantID",
					EventCorrelationIDKey: "eventCorrelationID",
				},
				objectID:    "validObjectID",
				loginMethod: LOGINMETHOD_OPENIDCONNECT,
				mfaType:     MFATYPE_WEBAUTHN,
				userType:    USERTYPE_BUSINESS,
				value:       nil,
			},
			wantErr: false,
		},
		{
			name: "T1904_UserLoginSuccess_EmptyMethod_Success",
			args: args{
				metadata: EventMetadata{
					UserInitiatorIDKey:    "userInitiatorID",
					TenantIDKey:           "tenantID",
					EventCorrelationIDKey: "eventCorrelationID",
				},
				objectID:    "validObjectID",
				loginMethod: "",
				mfaType:     MFATYPE_WEBAUTHN,
				userType:    USERTYPE_BUSINESS,
				value:       nil,
			},
			wantErr: false,
		},
		{
			name: "T1905_UserLoginSuccess_EmptyMfaType_Success",
			args: args{
				metadata: EventMetadata{
					UserInitiatorIDKey:    "userInitiatorID",
					TenantIDKey:           "tenantID",
					EventCorrelationIDKey: "eventCorrelationID",
				},
				objectID:    "validObjectID",
				loginMethod: LOGINMETHOD_OPENIDCONNECT,
				mfaType:     "",
				userType:    USERTYPE_BUSINESS,
				value:       nil,
			},
			wantErr: false,
		},
		{
			name: "T1906_UserLoginSuccess_EmptyUserType_Success",
			args: args{
				metadata: EventMetadata{
					UserInitiatorIDKey:    "userInitiatorID",
					TenantIDKey:           "tenantID",
					EventCorrelationIDKey: "eventCorrelationID",
				},
				objectID:    "validObjectID",
				loginMethod: LOGINMETHOD_OPENIDCONNECT,
				mfaType:     MFATYPE_WEBAUTHN,
				userType:    "",
				value:       nil,
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewUserLoginSuccessEvent(tt.args.metadata, tt.args.objectID, tt.args.loginMethod, tt.args.mfaType, tt.args.userType, tt.args.value)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewUserLoginSuccessEvent() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
		})
	}
}

func TestNewWorkflowExecuteEvent(t *testing.T) {
	type args struct {
		metadata    EventMetadata
		objectID    string
		channelID   string
		channelType string
		value       any
		dpp         bool
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "T2000_WorkflowExecute_EmptyObjectID_Fail",
			args: args{
				metadata: EventMetadata{
					UserInitiatorIDKey:    "userInitiatorID",
					TenantIDKey:           "tenantID",
					EventCorrelationIDKey: "eventCorrelationID",
				},
				objectID:    "",
				channelType: "web",
				channelID:   "123-23",
				value:       "someValue",
				dpp:         false,
			},
			wantErr: true,
		},
		{
			name: "T2001_WorkflowExecute_ValidObjectID_Success",
			args: args{
				metadata: EventMetadata{
					UserInitiatorIDKey:    "userInitiatorID",
					TenantIDKey:           "tenantID",
					EventCorrelationIDKey: "eventCorrelationID",
				},
				objectID:    "validObjectID",
				channelType: "web",
				channelID:   "123-23",
				value:       "testValue",
				dpp:         false,
			},
			wantErr: false,
		},
		{
			name: "T2002_WorkflowExecute_NoValue_Success",
			args: args{
				metadata: EventMetadata{
					UserInitiatorIDKey:    "userInitiatorID",
					TenantIDKey:           "tenantID",
					EventCorrelationIDKey: "eventCorrelationID",
				},
				objectID:    "validObjectID",
				channelType: "web",
				channelID:   "123-23",
				value:       nil,
				dpp:         false,
			},
			wantErr: false,
		},
		{
			name: "T2003_WorkflowExecute_NoChannelID_Fail",
			args: args{
				metadata: EventMetadata{
					UserInitiatorIDKey:    "userInitiatorID",
					TenantIDKey:           "tenantID",
					EventCorrelationIDKey: "eventCorrelationID",
				},
				objectID:    "validObjectID",
				channelType: "web",
				channelID:   "",
				value:       nil,
				dpp:         false,
			},
			wantErr: true,
		},
		{
			name: "T2004_WorkflowExecute_NoChannelType_Fail",
			args: args{
				metadata: EventMetadata{
					UserInitiatorIDKey:    "userInitiatorID",
					TenantIDKey:           "tenantID",
					EventCorrelationIDKey: "eventCorrelationID",
				},
				objectID:    "validObjectID",
				channelType: "",
				channelID:   "123-23",
				value:       nil,
				dpp:         false,
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewWorkflowExecuteEvent(tt.args.metadata, tt.args.objectID, tt.args.channelID, tt.args.channelType, tt.args.value, tt.args.dpp)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewWorkflowExecuteEvent() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
		})
	}
}

func TestNewWorkflowStartEvent(t *testing.T) {
	type args struct {
		metadata    EventMetadata
		objectID    string
		channelID   string
		channelType string
		value       any
		dpp         bool
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "T2100_WorkflowStart_EmptyObjectID_Fail",
			args: args{
				metadata: EventMetadata{
					UserInitiatorIDKey:    "userInitiatorID",
					TenantIDKey:           "tenantID",
					EventCorrelationIDKey: "eventCorrelationID",
				},
				objectID:    "",
				channelID:   "123-23",
				channelType: "web",
				value:       "someValue",
				dpp:         false,
			},
			wantErr: true,
		},
		{
			name: "T2101_WorkflowStart_ValidObjectID_Success",
			args: args{
				metadata: EventMetadata{
					UserInitiatorIDKey:    "userInitiatorID",
					TenantIDKey:           "tenantID",
					EventCorrelationIDKey: "eventCorrelationID",
				},
				objectID:    "validObjectID",
				channelID:   "123-23",
				channelType: "web",
				value:       "testValue",
				dpp:         false,
			},
			wantErr: false,
		},
		{
			name: "T2102_WorkflowStart_NoValue_Success",
			args: args{
				metadata: EventMetadata{
					UserInitiatorIDKey:    "userInitiatorID",
					TenantIDKey:           "tenantID",
					EventCorrelationIDKey: "eventCorrelationID",
				},
				objectID:    "validObjectID",
				channelID:   "123-23",
				channelType: "web",
				value:       nil,
				dpp:         false,
			},
			wantErr: false,
		},
		{
			name: "T2103_WorkflowStart_NoChannelID_Fail",
			args: args{
				metadata: EventMetadata{
					UserInitiatorIDKey:    "userInitiatorID",
					TenantIDKey:           "tenantID",
					EventCorrelationIDKey: "eventCorrelationID",
				},
				objectID:    "validObjectID",
				channelID:   "",
				channelType: "web",
				value:       nil,
				dpp:         false,
			},
			wantErr: true,
		},
		{
			name: "T2104_WorkflowStart_NoChannelType_Fail",
			args: args{
				metadata: EventMetadata{
					UserInitiatorIDKey:    "userInitiatorID",
					TenantIDKey:           "tenantID",
					EventCorrelationIDKey: "eventCorrelationID",
				},
				objectID:    "validObjectID",
				channelID:   "123-23",
				channelType: "",
				value:       nil,
				dpp:         false,
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewWorkflowStartEvent(tt.args.metadata, tt.args.objectID, tt.args.channelID, tt.args.channelType, tt.args.value, tt.args.dpp)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewWorkflowStartEvent() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
		})
	}
}

func TestNewWorkflowTerminateEvent(t *testing.T) {
	type args struct {
		metadata    EventMetadata
		objectID    string
		channelID   string
		channelType string
		value       any
		dpp         bool
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "T2200_WorkflowTerminate_EmptyObjectID_Fail",
			args: args{
				metadata: EventMetadata{
					UserInitiatorIDKey:    "userInitiatorID",
					TenantIDKey:           "tenantID",
					EventCorrelationIDKey: "eventCorrelationID",
				},
				objectID:    "",
				channelID:   "123-23",
				channelType: "web",
				value:       "someValue",
				dpp:         true,
			},
			wantErr: true,
		},
		{
			name: "T2201_WorkflowTerminate_ValidObjectID_Success",
			args: args{
				metadata: EventMetadata{
					UserInitiatorIDKey:    "userInitiatorID",
					TenantIDKey:           "tenantID",
					EventCorrelationIDKey: "eventCorrelationID",
				},
				objectID:    "validObjectID",
				channelID:   "123-23",
				channelType: "web",
				value:       "testValue",
				dpp:         false,
			},
			wantErr: false,
		},
		{
			name: "T2202_WorkflowTerminate_NoValue_Success",
			args: args{
				metadata: EventMetadata{
					UserInitiatorIDKey:    "userInitiatorID",
					TenantIDKey:           "tenantID",
					EventCorrelationIDKey: "eventCorrelationID",
				},
				objectID:    "validObjectID",
				channelID:   "123-23",
				channelType: "web",
				value:       nil,
				dpp:         false,
			},
			wantErr: false,
		},
		{
			name: "T2203_WorkflowTerminate_NoChannelID_Fail",
			args: args{
				metadata: EventMetadata{
					UserInitiatorIDKey:    "userInitiatorID",
					TenantIDKey:           "tenantID",
					EventCorrelationIDKey: "eventCorrelationID",
				},
				objectID:    "validObjectID",
				channelID:   "",
				channelType: "web",
				value:       nil,
				dpp:         false,
			},
			wantErr: true,
		},
		{
			name: "T2204_WorkflowTerminate_NoChannelType_Fail",
			args: args{
				metadata: EventMetadata{
					UserInitiatorIDKey:    "userInitiatorID",
					TenantIDKey:           "tenantID",
					EventCorrelationIDKey: "eventCorrelationID",
				},
				objectID:    "validObjectID",
				channelID:   "123-23",
				channelType: "",
				value:       nil,
				dpp:         false,
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewWorkflowTerminateEvent(tt.args.metadata, tt.args.objectID, tt.args.channelID, tt.args.channelType, tt.args.value, tt.args.dpp)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewWorkflowTerminateEvent() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
		})
	}
}

func TestNewWorkflowUpdateEvent(t *testing.T) {
	type args struct {
		metadata EventMetadata
		objectID string
		oldValue any
		newValue any
		dpp      bool
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "T2300_WorkflowUpdate_EmptyObjectID_Fail",
			args: args{
				metadata: EventMetadata{
					UserInitiatorIDKey:    "userInitiatorID",
					TenantIDKey:           "tenantID",
					EventCorrelationIDKey: "eventCorrelationID",
				},
				objectID: "",
				oldValue: "oldValue",
				newValue: "newValue",
				dpp:      false,
			},
			wantErr: true,
		},
		{
			name: "T2301_WorkflowUpdate_ValidObjectID_Success",
			args: args{
				metadata: EventMetadata{
					UserInitiatorIDKey:    "userInitiatorID",
					TenantIDKey:           "tenantID",
					EventCorrelationIDKey: "eventCorrelationID",
				},
				objectID: "validObjectID",
				oldValue: "oldValue",
				newValue: "newValue",
				dpp:      true,
			},
			wantErr: false,
		},
		{
			name: "T2302_WorkflowUpdate_NoOldValue_Success",
			args: args{
				metadata: EventMetadata{
					UserInitiatorIDKey:    "userInitiatorID",
					TenantIDKey:           "tenantID",
					EventCorrelationIDKey: "eventCorrelationID",
				},
				objectID: "validObjectID",
				oldValue: nil,
				newValue: "newValue",
				dpp:      false,
			},
			wantErr: false,
		},
		{
			name: "T2303_WorkflowUpdate_NoNewValue_Success",
			args: args{
				metadata: EventMetadata{
					UserInitiatorIDKey:    "userInitiatorID",
					TenantIDKey:           "tenantID",
					EventCorrelationIDKey: "eventCorrelationID",
				},
				objectID: "validObjectID",
				oldValue: "oldValue",
				newValue: nil,
				dpp:      true,
			},
			wantErr: false,
		},
		{
			name: "T2304_WorkflowUpdate_NoValues_Success",
			args: args{
				metadata: EventMetadata{
					UserInitiatorIDKey:    "userInitiatorID",
					TenantIDKey:           "tenantID",
					EventCorrelationIDKey: "eventCorrelationID",
				},
				objectID: "validObjectID",
				oldValue: nil,
				newValue: nil,
				dpp:      false,
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewWorkflowUpdateEvent(tt.args.metadata, tt.args.objectID, tt.args.oldValue, tt.args.newValue, tt.args.dpp)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewWorkflowUpdateEvent() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
		})
	}
}

func TestCredentialEvents(t *testing.T) {
	type args struct {
		metadata EventMetadata
		objectID string
		c        CredentialType
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "T2400_CredentialEvents_EmptyObjectID_Fail",
			args: args{
				metadata: EventMetadata{
					UserInitiatorIDKey:    "userInitiatorID",
					TenantIDKey:           "tenantID",
					EventCorrelationIDKey: "eventCorrelationID",
				},
				objectID: "",
				c:        CREDTYPE_SECRET,
			},
			wantErr: true,
		},
		{
			name: "T2401_CredentialEvents_ValidData_Success",
			args: args{
				metadata: EventMetadata{
					UserInitiatorIDKey:    "userInitiatorID",
					TenantIDKey:           "tenantID",
					EventCorrelationIDKey: "eventCorrelationID",
				},
				objectID: "validObjectID",
				c:        CREDTYPE_KEY,
			},
			wantErr: false,
		},
		{
			name: "T2402_CredentialEvents_InvalidCredType_Fail",
			args: args{
				metadata: EventMetadata{
					UserInitiatorIDKey:    "userInitiatorID",
					TenantIDKey:           "tenantID",
					EventCorrelationIDKey: "eventCorrelationID",
				},
				objectID: "validObjectID",
				c:        CredentialType("invalid type"),
			},
			wantErr: true,
		},
		{
			name: "T2403_CredentialEvents_NoValue_Success",
			args: args{
				metadata: EventMetadata{
					UserInitiatorIDKey:    "userInitiatorID",
					TenantIDKey:           "tenantID",
					EventCorrelationIDKey: "eventCorrelationID",
				},
				objectID: "validObjectID",
				c:        CREDTYPE_X509CERT,
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewCredentialCreateEvent(tt.args.metadata, tt.args.objectID, tt.args.c)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewCredentialCreateEvent() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			_, err = NewCredentialExpirationEvent(tt.args.metadata, tt.args.objectID, tt.args.c)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewCredentialExpirationEvent() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			_, err = NewCredentialDeleteEvent(tt.args.metadata, tt.args.objectID, tt.args.c)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewCredentialDeleteEvent() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			_, err = NewCredentialRevokationEvent(tt.args.metadata, tt.args.objectID, tt.args.c)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewCredentialRevokationEvent() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
		})
	}
}

func TestCmkEvents(t *testing.T) {
	type args struct {
		metadata EventMetadata
		cmkID    string
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "T2500_CmkEvents_EmptyObjectID_Fail",
			args: args{
				metadata: EventMetadata{
					UserInitiatorIDKey:    "userInitiatorID",
					TenantIDKey:           "tenantID",
					EventCorrelationIDKey: "eventCorrelationID",
				},
				cmkID: "",
			},
			wantErr: true,
		},
		{
			name: "T2501_CmkEvents_ValidData_Success",
			args: args{
				metadata: EventMetadata{
					UserInitiatorIDKey:    "userInitiatorID",
					TenantIDKey:           "tenantID",
					EventCorrelationIDKey: "eventCorrelationID",
				},
				cmkID: "validObjectID",
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewCmkCreateEvent(tt.args.metadata, tt.args.cmkID)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewCmkCreateEvent() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			_, err = NewCmkDeleteEvent(tt.args.metadata, tt.args.cmkID)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewCmkDeleteEvent() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			_, err = NewCmkDisableEvent(tt.args.metadata, tt.args.cmkID)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewCmkDisableEvent() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			_, err = NewCmkEnableEvent(tt.args.metadata, tt.args.cmkID)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewCmkEnableEvent() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			_, err = NewCmkRestoreEvent(tt.args.metadata, tt.args.cmkID)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewCmkRestoreEvent() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			_, err = NewCmkRotateEvent(tt.args.metadata, tt.args.cmkID)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewCmkRotateEvent() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
		})
	}
}

func TestCmkBoardingEvents(t *testing.T) {
	type args struct {
		metadata EventMetadata
		cmkID    string
		systemID string
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "T2600_CmkBoardingEvents_EmptyObjectID_Fail",
			args: args{
				metadata: EventMetadata{
					UserInitiatorIDKey:    "userInitiatorID",
					TenantIDKey:           "tenantID",
					EventCorrelationIDKey: "eventCorrelationID",
				},
				cmkID:    "",
				systemID: "123123",
			},
			wantErr: true,
		},
		{
			name: "T2601_CmkBoardingEvents_EmptySystemID_Fail",
			args: args{
				metadata: EventMetadata{
					UserInitiatorIDKey:    "userInitiatorID",
					TenantIDKey:           "tenantID",
					EventCorrelationIDKey: "eventCorrelationID",
				},
				cmkID:    "123123",
				systemID: "",
			},
			wantErr: true,
		},
		{
			name: "T2602_CmkBoardingEvents_ValidData_Success",
			args: args{
				metadata: EventMetadata{
					UserInitiatorIDKey:    "userInitiatorID",
					TenantIDKey:           "tenantID",
					EventCorrelationIDKey: "eventCorrelationID",
				},
				cmkID:    "validObjectID",
				systemID: "123123",
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewCmkOnboardingEvent(tt.args.metadata, tt.args.cmkID, tt.args.systemID)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewCmkOnboardingEvent() error = %v, wantErr %v", err, tt.wantErr)
			}
			_, err = NewCmkOffboardingEvent(tt.args.metadata, tt.args.cmkID, tt.args.systemID)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewCmkOffboardingEvent() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestNewCmkSwitchEvent(t *testing.T) {
	type args struct {
		metadata EventMetadata
		cmkID    string
		cmkIDOld string
		cmkIDNew string
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "T2700_NewCmkSwitchEvent_EmptyObjectID_Fail",
			args: args{
				metadata: EventMetadata{
					UserInitiatorIDKey:    "userInitiatorID",
					TenantIDKey:           "tenantID",
					EventCorrelationIDKey: "eventCorrelationID",
				},
				cmkID:    "",
				cmkIDOld: "123123",
				cmkIDNew: "123123",
			},
			wantErr: true,
		},
		{
			name: "T2701_NewCmkSwitchEvent_EmptyCMKIDOld_Fail",
			args: args{
				metadata: EventMetadata{
					UserInitiatorIDKey:    "userInitiatorID",
					TenantIDKey:           "tenantID",
					EventCorrelationIDKey: "eventCorrelationID",
				},
				cmkID:    "123123",
				cmkIDOld: "",
				cmkIDNew: "123123",
			},
			wantErr: true,
		},
		{
			name: "T2702_NewCmkSwitchEvent_EmptyCMKIDNew_Fail",
			args: args{
				metadata: EventMetadata{
					UserInitiatorIDKey:    "userInitiatorID",
					TenantIDKey:           "tenantID",
					EventCorrelationIDKey: "eventCorrelationID",
				},
				cmkID:    "12333",
				cmkIDOld: "123123",
				cmkIDNew: "",
			},
			wantErr: true,
		},
		{
			name: "T2703_NewCmkSwitchEvent_CorrectData_Success",
			args: args{
				metadata: EventMetadata{
					UserInitiatorIDKey:    "userInitiatorID",
					TenantIDKey:           "tenantID",
					EventCorrelationIDKey: "eventCorrelationID",
				},
				cmkID:    "12333",
				cmkIDOld: "123123",
				cmkIDNew: "123123",
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewCmkSwitchEvent(tt.args.metadata, tt.args.cmkID, tt.args.cmkIDOld, tt.args.cmkIDNew)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewCmkSwitchEvent() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestNewCmkTenantModificationEvent(t *testing.T) {
	type args struct {
		metadata EventMetadata
		cmkID    string
		systemID string
		c        CmkAction
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "T2800_NewCmkTenantModificationEvent_EmptyObjectID_Fail",
			args: args{
				metadata: EventMetadata{
					UserInitiatorIDKey:    "userInitiatorID",
					TenantIDKey:           "tenantID",
					EventCorrelationIDKey: "eventCorrelationID",
				},
				cmkID:    "",
				systemID: "123123",
				c:        CMKACTION_ONBOARD,
			},
			wantErr: true,
		},
		{
			name: "T2801_NewCmkTenantModificationEvent_EmptySystemID_Fail",
			args: args{
				metadata: EventMetadata{
					UserInitiatorIDKey:    "userInitiatorID",
					TenantIDKey:           "tenantID",
					EventCorrelationIDKey: "eventCorrelationID",
				},
				cmkID:    "12312",
				systemID: "",
				c:        CMKACTION_ONBOARD,
			},
			wantErr: true,
		},
		{
			name: "T2800_NewCmkTenantModificationEvent_WrongCMKAction_Fail",
			args: args{
				metadata: EventMetadata{
					UserInitiatorIDKey:    "userInitiatorID",
					TenantIDKey:           "tenantID",
					EventCorrelationIDKey: "eventCorrelationID",
				},
				cmkID:    "123123",
				systemID: "123123",
				c:        CmkAction("invalid"),
			},
			wantErr: true,
		},
		{
			name: "T2800_NewCmkTenantModificationEvent_CorrectData_Success",
			args: args{
				metadata: EventMetadata{
					UserInitiatorIDKey:    "userInitiatorID",
					TenantIDKey:           "tenantID",
					EventCorrelationIDKey: "eventCorrelationID",
				},
				cmkID:    "2222",
				systemID: "123123",
				c:        CMKACTION_ONBOARD,
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewCmkTenantModificationEvent(tt.args.metadata, tt.args.cmkID, tt.args.systemID, tt.args.c)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewCmkTenantModificationEvent() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
