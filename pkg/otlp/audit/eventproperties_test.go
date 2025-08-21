package otlpaudit

import (
	"testing"
)

func TestNewEventMetadata(t *testing.T) {
	type args struct {
		userInitiatorID    string
		tenantID           string
		eventCorrelationID string
	}

	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "T2500_NewEventMetadata_AllFields_Success",
			args: args{
				userInitiatorID:    "user-initiator-id",
				tenantID:           "tenant-id",
				eventCorrelationID: "event-correlation-id",
			},
			wantErr: false,
		},
		{
			name: "T2501_NewEventMetadata_EventCorrelationIDMissing_Success",
			args: args{
				userInitiatorID:    "user-initiator-id",
				tenantID:           "tenant-id",
				eventCorrelationID: "",
			},
			wantErr: false,
		},
		{
			name: "T2502_NewEventMetadata_TenantIDMissing_Fail",
			args: args{
				userInitiatorID:    "user-initiator-id",
				tenantID:           "",
				eventCorrelationID: "event-correlation-id",
			},
			wantErr: true,
		},
		{
			name: "T2503_NewEventMetadata_UserInitiatorIDMissing_Fail",
			args: args{
				userInitiatorID:    "",
				tenantID:           "tenant-id",
				eventCorrelationID: "event-correlation-id",
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewEventMetadata(tt.args.userInitiatorID, tt.args.tenantID, tt.args.eventCorrelationID)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewEventMetadata() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
		})
	}
}
