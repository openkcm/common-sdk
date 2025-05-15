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

func TestNewKeyCreateEvent(t *testing.T) {
	type args struct {
		metadata   EventMetadata
		objectID   string
		keyLevel   KeyLevel
		actionType KeyCreateActionType
		value      any
		dpp        bool
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "T1100_KeyCreate_EmptyObjectID_Fail",
			args: args{
				metadata: EventMetadata{
					UserInitiatorIDKey:    "userInitiatorID",
					TenantIDKey:           "tenantID",
					EventCorrelationIDKey: "eventCorrelationID",
				},
				objectID:   "",
				keyLevel:   L1,
				actionType: KEYCREATE_RESTORE,
				value:      "someValue",
				dpp:        true,
			},
			wantErr: true,
		},
		{
			name: "T1101_KeyCreate_InvalidActionType_Fail",
			args: args{
				metadata: EventMetadata{
					UserInitiatorIDKey:    "userInitiatorID",
					TenantIDKey:           "tenantID",
					EventCorrelationIDKey: "eventCorrelationID",
				},
				objectID:   "validObjectID",
				keyLevel:   L1,
				actionType: KeyCreateActionType("invalidAction"),
				value:      "someValue",
				dpp:        true,
			},
			wantErr: true,
		},
		{
			name: "T1102_KeyCreate_InvalidKeyLevel_Fail",
			args: args{
				metadata: EventMetadata{
					UserInitiatorIDKey:    "userInitiatorID",
					TenantIDKey:           "tenantID",
					EventCorrelationIDKey: "eventCorrelationID",
				},
				objectID:   "validObjectID",
				keyLevel:   KeyLevel("invalidLevel"),
				actionType: KEYCREATE_RESTORE,
				value:      "someValue",
				dpp:        false,
			},
			wantErr: true,
		},
		{
			name: "T1103_KeyCreate_ValidObjectIDAndKeyLevelAndActionType_Success",
			args: args{
				metadata: EventMetadata{
					UserInitiatorIDKey:    "userInitiatorID",
					TenantIDKey:           "tenantID",
					EventCorrelationIDKey: "eventCorrelationID",
				},
				objectID:   "validObjectID",
				keyLevel:   L1,
				actionType: KEYCREATE_RESTORE,
				value:      "testValue",
				dpp:        true,
			},
			wantErr: false,
		},
		{
			name: "T1104_KeyCreate_NoValue_Success",
			args: args{
				metadata: EventMetadata{
					UserInitiatorIDKey:    "userInitiatorID",
					TenantIDKey:           "tenantID",
					EventCorrelationIDKey: "eventCorrelationID",
				},
				objectID:   "validObjectID",
				keyLevel:   L1,
				actionType: KEYCREATE_IMPORT,
				value:      nil,
				dpp:        false,
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewKeyCreateEvent(tt.args.metadata, tt.args.objectID, tt.args.keyLevel, tt.args.actionType, tt.args.value, tt.args.dpp)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewKeyCreateEvent() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
		})
	}
}

func TestNewKeyDeleteEvent(t *testing.T) {
	type args struct {
		metadata EventMetadata
		objectID string
		keyLevel KeyLevel
		value    any
		dpp      bool
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "T1200_KeyDelete_EmptyObjectID_Fail",
			args: args{
				metadata: EventMetadata{
					UserInitiatorIDKey:    "userInitiatorID",
					TenantIDKey:           "tenantID",
					EventCorrelationIDKey: "eventCorrelationID",
				},
				objectID: "",
				keyLevel: L3,
				value:    "someValue",
				dpp:      true,
			},
			wantErr: true,
		},
		{
			name: "T1201_KeyDelete_InvalidKeyLevel_Fail",
			args: args{
				metadata: EventMetadata{
					UserInitiatorIDKey:    "userInitiatorID",
					TenantIDKey:           "tenantID",
					EventCorrelationIDKey: "eventCorrelationID",
				},
				objectID: "validObjectID",
				keyLevel: KeyLevel("invalidLevel"),
				value:    "someValue",
				dpp:      false,
			},
			wantErr: true,
		},
		{
			name: "T1202_KeyDelete_ValidObjectIDAndKeyLevel_Success",
			args: args{
				metadata: EventMetadata{
					UserInitiatorIDKey:    "userInitiatorID",
					TenantIDKey:           "tenantID",
					EventCorrelationIDKey: "eventCorrelationID",
				},
				objectID: "validObjectID",
				keyLevel: L1,
				value:    "testValue",
				dpp:      true,
			},
			wantErr: false,
		},
		{
			name: "T1203_KeyDelete_NoValue_Success",
			args: args{
				metadata: EventMetadata{
					UserInitiatorIDKey:    "userInitiatorID",
					TenantIDKey:           "tenantID",
					EventCorrelationIDKey: "eventCorrelationID",
				},
				objectID: "validObjectID",
				keyLevel: L2,
				value:    nil,
				dpp:      false,
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewKeyDeleteEvent(tt.args.metadata, tt.args.objectID, tt.args.keyLevel, tt.args.value, tt.args.dpp)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewKeyDeleteEvent() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
		})
	}
}

func TestNewKeyReadEvent(t *testing.T) {
	type args struct {
		metadata    EventMetadata
		objectID    string
		channelType string
		channelID   string
		keyLevel    KeyLevel
		actionType  KeyReadActionType
		value       any
		dpp         bool
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "T1300_KeyRead_EmptyObjectID_Fail",
			args: args{
				metadata: EventMetadata{
					UserInitiatorIDKey:    "userInitiatorID",
					TenantIDKey:           "tenantID",
					EventCorrelationIDKey: "eventCorrelationID",
				},
				objectID:    "",
				channelType: "web",
				channelID:   "123-23",
				keyLevel:    L1,
				actionType:  KEYREAD_CRYPTOACCESS,
				value:       "someValue",
				dpp:         true,
			},
			wantErr: true,
		},
		{
			name: "T1301_KeyRead_InvalidKeyLevel_Fail",
			args: args{
				metadata: EventMetadata{
					UserInitiatorIDKey:    "userInitiatorID",
					TenantIDKey:           "tenantID",
					EventCorrelationIDKey: "eventCorrelationID",
				},
				objectID:    "validObjectID",
				channelType: "web",
				channelID:   "123-23",
				keyLevel:    KeyLevel("invalidLevel"),
				actionType:  KEYREAD_CRYPTOACCESS,
				value:       "someValue",
				dpp:         false,
			},
			wantErr: true,
		},
		{
			name: "T1302_KeyRead_InvalidActionType_Fail",
			args: args{
				metadata: EventMetadata{
					UserInitiatorIDKey:    "userInitiatorID",
					TenantIDKey:           "tenantID",
					EventCorrelationIDKey: "eventCorrelationID",
				},
				objectID:    "validObjectID",
				channelType: "web",
				channelID:   "123-23",
				keyLevel:    L2,
				actionType:  KeyReadActionType("invalidAction"),
				value:       "someValue",
				dpp:         false,
			},
			wantErr: true,
		},
		{
			name: "T1303_KeyRead_ValidObjectIDAndKeyLevelAndActionType_Success",
			args: args{
				metadata: EventMetadata{
					UserInitiatorIDKey:    "userInitiatorID",
					TenantIDKey:           "tenantID",
					EventCorrelationIDKey: "eventCorrelationID",
				},
				objectID:    "validObjectID",
				channelType: "web",
				channelID:   "123-23",
				keyLevel:    L3,
				actionType:  KEYREAD_READMETADATA,
				value:       "testValue",
				dpp:         true,
			},
			wantErr: false,
		},
		{
			name: "T1304_KeyRead_NoValue_Success",
			args: args{
				metadata: EventMetadata{
					UserInitiatorIDKey:    "userInitiatorID",
					TenantIDKey:           "tenantID",
					EventCorrelationIDKey: "eventCorrelationID",
				},
				objectID:    "validObjectID",
				channelType: "web",
				channelID:   "123-23",
				keyLevel:    L2,
				actionType:  KEYREAD_READMETADATA,
				value:       nil,
				dpp:         false,
			},
			wantErr: false,
		},
		{
			name: "T1305_KeyRead_NoChannelID_Fail",
			args: args{
				metadata: EventMetadata{
					UserInitiatorIDKey:    "userInitiatorID",
					TenantIDKey:           "tenantID",
					EventCorrelationIDKey: "eventCorrelationID",
				},
				objectID:    "validObjectID",
				channelType: "web",
				keyLevel:    L2,
				actionType:  KEYREAD_READMETADATA,
				value:       nil,
				dpp:         false,
			},
			wantErr: true,
		},
		{
			name: "T1306_KeyRead_NoChannelType_Fail",
			args: args{
				metadata: EventMetadata{
					UserInitiatorIDKey:    "userInitiatorID",
					TenantIDKey:           "tenantID",
					EventCorrelationIDKey: "eventCorrelationID",
				},
				objectID:   "validObjectID",
				channelID:  "123-23",
				keyLevel:   L2,
				actionType: KEYREAD_READMETADATA,
				value:      nil,
				dpp:        false,
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewKeyReadEvent(tt.args.metadata, tt.args.objectID, tt.args.channelType, tt.args.channelID, tt.args.keyLevel, tt.args.actionType, tt.args.value, tt.args.dpp)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewKeyReadEvent() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
		})
	}
}

func TestNewKeyUpdateEvent(t *testing.T) {
	type args struct {
		metadata     EventMetadata
		objectID     string
		propertyName string
		keyLevel     KeyLevel
		actionType   KeyUpdateActionType
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
			name: "T1400_KeyUpdate_EmptyObjectID_Fail",
			args: args{
				metadata: EventMetadata{
					UserInitiatorIDKey:    "userInitiatorID",
					TenantIDKey:           "tenantID",
					EventCorrelationIDKey: "eventCorrelationID",
				},
				objectID:     "",
				propertyName: "validProperty",
				keyLevel:     L1,
				actionType:   KEYUPDATE_ENABLE,
				oldValue:     "oldValue",
				newValue:     "newValue",
				dpp:          true,
			},
			wantErr: true,
		},
		{
			name: "T1401_KeyUpdate_InvalidActionType_Fail",
			args: args{
				metadata: EventMetadata{
					UserInitiatorIDKey:    "userInitiatorID",
					TenantIDKey:           "tenantID",
					EventCorrelationIDKey: "eventCorrelationID",
				},
				objectID:     "validObjectID",
				propertyName: "validProperty",
				keyLevel:     L1,
				actionType:   KeyUpdateActionType("invalidAction"),
				oldValue:     "oldValue",
				newValue:     "newValue",
				dpp:          false,
			},
			wantErr: true,
		},
		{
			name: "T1402_KeyUpdate_InvalidKeyLevel_Fail",
			args: args{
				metadata: EventMetadata{
					UserInitiatorIDKey:    "userInitiatorID",
					TenantIDKey:           "tenantID",
					EventCorrelationIDKey: "eventCorrelationID",
				},
				objectID:     "validObjectID",
				propertyName: "validProperty",
				keyLevel:     KeyLevel("invalidLevel"),
				actionType:   KEYUPDATE_DISABLE,
				oldValue:     "oldValue",
				newValue:     "newValue",
				dpp:          true,
			},
			wantErr: true,
		},
		{
			name: "T1403_KeyUpdate_ValidObjectIDAndPropertyNameAndKeyLevelAndActionType_Success",
			args: args{
				metadata: EventMetadata{
					UserInitiatorIDKey:    "userInitiatorID",
					TenantIDKey:           "tenantID",
					EventCorrelationIDKey: "eventCorrelationID",
				},
				objectID:     "validObjectID",
				propertyName: "validProperty",
				keyLevel:     L2,
				actionType:   KEYUPDATE_ROTATE,
				oldValue:     "oldValue",
				newValue:     "newValue",
				dpp:          false,
			},
			wantErr: false,
		},
		{
			name: "T1404_KeyUpdate_NoOldValue_Success",
			args: args{
				metadata: EventMetadata{
					UserInitiatorIDKey:    "userInitiatorID",
					TenantIDKey:           "tenantID",
					EventCorrelationIDKey: "eventCorrelationID",
				},
				objectID:     "validObjectID",
				propertyName: "validProperty",
				keyLevel:     L3,
				actionType:   KEYUPDATE_ENABLE,
				oldValue:     nil,
				newValue:     "newValue",
				dpp:          true,
			},
			wantErr: false,
		},
		{
			name: "T1405_KeyUpdate_NoNewValue_Success",
			args: args{
				metadata: EventMetadata{
					UserInitiatorIDKey:    "userInitiatorID",
					TenantIDKey:           "tenantID",
					EventCorrelationIDKey: "eventCorrelationID",
				},
				objectID:     "validObjectID",
				propertyName: "validProperty",
				keyLevel:     L1,
				actionType:   KEYUPDATE_ROTATE,
				oldValue:     "oldValue",
				newValue:     nil,
				dpp:          false,
			},
			wantErr: false,
		},
		{
			name: "T1406_KeyUpdate_EmptyPropertyName_Fail",
			args: args{
				metadata: EventMetadata{
					UserInitiatorIDKey:    "userInitiatorID",
					TenantIDKey:           "tenantID",
					EventCorrelationIDKey: "eventCorrelationID",
				},
				objectID:     "validObjectID",
				propertyName: "",
				keyLevel:     L1,
				actionType:   KEYUPDATE_ROTATE,
				oldValue:     "oldValue",
				newValue:     nil,
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewKeyUpdateEvent(tt.args.metadata, tt.args.objectID, tt.args.propertyName, tt.args.keyLevel, tt.args.actionType, tt.args.oldValue, tt.args.newValue, tt.args.dpp)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewKeyUpdateEvent() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
		})
	}
}

func TestNewTenantOffboardingEvent(t *testing.T) {
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
			name: "T1500_TenantOffboarding_EmptyObjectID_Fail",
			args: args{
				metadata: EventMetadata{
					UserInitiatorIDKey:    "userInitiatorID",
					TenantIDKey:           "tenantID",
					EventCorrelationIDKey: "eventCorrelationID",
				},
				objectID: "",
				value:    "someValue",
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
				value:    "testValue",
			},
			wantErr: false,
		},
		{
			name: "T1502_TenantOffboarding_NoValue_Success",
			args: args{
				metadata: EventMetadata{
					UserInitiatorIDKey:    "userInitiatorID",
					TenantIDKey:           "tenantID",
					EventCorrelationIDKey: "eventCorrelationID",
				},
				objectID: "validObjectID",
				value:    nil,
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewTenantOffboardingEvent(tt.args.metadata, tt.args.objectID, tt.args.value)
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
		value    any
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
				value:    "someValue",
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
				value:    "testValue",
			},
			wantErr: false,
		},
		{
			name: "T1602_TenantOnboarding_NoValue_Success",
			args: args{
				metadata: EventMetadata{
					UserInitiatorIDKey:    "userInitiatorID",
					TenantIDKey:           "tenantID",
					EventCorrelationIDKey: "eventCorrelationID",
				},
				objectID: "validObjectID",
				value:    nil,
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewTenantOnboardingEvent(tt.args.metadata, tt.args.objectID, tt.args.value)
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
		actionType   TenantUpdateActionType
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
				actionType:   TENANTUPDATE_WORKFLOWENABLE,
				oldValue:     "oldValue",
				newValue:     "newValue",
			},
			wantErr: true,
		},
		{
			name: "T1701_TenantUpdate_InvalidActionType_Fail",
			args: args{
				metadata: EventMetadata{
					UserInitiatorIDKey:    "userInitiatorID",
					TenantIDKey:           "tenantID",
					EventCorrelationIDKey: "eventCorrelationID",
				},
				objectID:     "validObjectID",
				propertyName: "validProperty",
				actionType:   TenantUpdateActionType("invalidAction"),
				oldValue:     "oldValue",
				newValue:     "newValue",
			},
			wantErr: true,
		},
		{
			name: "T1702_TenantUpdate_ValidObjectIDAndPropertyNameAndActionType_Success",
			args: args{
				metadata: EventMetadata{
					UserInitiatorIDKey:    "userInitiatorID",
					TenantIDKey:           "tenantID",
					EventCorrelationIDKey: "eventCorrelationID",
				},
				objectID:     "validObjectID",
				propertyName: "validProperty",
				actionType:   TENANTUPDATE_WORKFLOWDISABLE,
				oldValue:     "oldValue",
				newValue:     "newValue",
			},
			wantErr: false,
		},
		{
			name: "T1703_TenantUpdate_NoOldValue_Fail",
			args: args{
				metadata: EventMetadata{
					UserInitiatorIDKey:    "userInitiatorID",
					TenantIDKey:           "tenantID",
					EventCorrelationIDKey: "eventCorrelationID",
				},
				objectID:     "validObjectID",
				propertyName: "validProperty",
				actionType:   TENANTUPDATE_TESTMODE,
				oldValue:     nil,
				newValue:     "newValue",
			},
			wantErr: true,
		},
		{
			name: "T1704_TenantUpdate_NoNewValue_Fail",
			args: args{
				metadata: EventMetadata{
					UserInitiatorIDKey:    "userInitiatorID",
					TenantIDKey:           "tenantID",
					EventCorrelationIDKey: "eventCorrelationID",
				},
				objectID:     "validObjectID",
				propertyName: "validProperty",
				actionType:   TENANTUPDATE_TESTMODE,
				oldValue:     "oldValue",
				newValue:     nil,
			},
			wantErr: true,
		},
		{
			name: "T1705_TenantUpdate_EmptyPropertyName_Fail",
			args: args{
				metadata: EventMetadata{
					UserInitiatorIDKey:    "userInitiatorID",
					TenantIDKey:           "tenantID",
					EventCorrelationIDKey: "eventCorrelationID",
				},
				objectID:     "validObjectID",
				propertyName: "",
				actionType:   TENANTUPDATE_TESTMODE,
				oldValue:     "oldValue",
				newValue:     nil,
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewTenantUpdateEvent(tt.args.metadata, tt.args.objectID, tt.args.propertyName, tt.args.actionType, tt.args.oldValue, tt.args.newValue)
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
		value    any
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
				value:    "someValue",
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
				value:    "someValue",
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
				value:    "someValue",
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
				value:    nil,
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewCredentialCreateEvent(tt.args.metadata, tt.args.objectID, tt.args.c, tt.args.value)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewCredentialCreateEvent() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			_, err = NewCredentialExpirationEvent(tt.args.metadata, tt.args.objectID, tt.args.c, tt.args.value)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewCredentialExpirationEvent() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			_, err = NewCredentialDeleteEvent(tt.args.metadata, tt.args.objectID, tt.args.c, tt.args.value)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewCredentialDeleteEvent() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			_, err = NewCredentialRevokationEvent(tt.args.metadata, tt.args.objectID, tt.args.c, tt.args.value)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewCredentialRevokationEvent() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
		})
	}
}
