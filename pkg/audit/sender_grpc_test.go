package audit

import (
	"context"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/plog/plogotlp"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	auditlogeventsv1 "github.com/openkcm/api-sdk/proto/kms/api/cmk/auditlog/events/v1"

	"github.com/openkcm/common-sdk/pkg/commoncfg"
)

// mockLogsServer implements the OTLP LogsService for testing.
type mockLogsServer struct {
	plogotlp.UnimplementedGRPCServer

	received     atomic.Int32
	totalRecords atomic.Int32
	responseCode codes.Code
	responseMsg  string
	mu           sync.Mutex
	lastRequest  plogotlp.ExportRequest
}

func (m *mockLogsServer) Export(_ context.Context, req plogotlp.ExportRequest) (plogotlp.ExportResponse, error) {
	m.received.Add(1)

	// Count total log records in the request
	logs := req.Logs()

	var recordCount int32

	for i := range logs.ResourceLogs().Len() {
		rl := logs.ResourceLogs().At(i)
		for j := range rl.ScopeLogs().Len() {
			sl := rl.ScopeLogs().At(j)
			recordCount += int32(sl.LogRecords().Len())
		}
	}

	m.totalRecords.Add(recordCount)

	m.mu.Lock()
	m.lastRequest = req
	m.mu.Unlock()

	if m.responseCode != codes.OK {
		return plogotlp.ExportResponse{}, status.Error(m.responseCode, m.responseMsg)
	}

	return plogotlp.NewExportResponse(), nil
}

func startMockGRPCServer(t *testing.T, mock *mockLogsServer) (string, func()) {
	t.Helper()

	lis, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	server := grpc.NewServer()
	plogotlp.RegisterGRPCServer(server, mock)

	go func() {
		_ = server.Serve(lis)
	}()

	return lis.Addr().String(), func() {
		server.GracefulStop()
	}
}

func TestGRPCSender_Send(t *testing.T) {
	t.Run("sends logs successfully", func(t *testing.T) {
		mock := &mockLogsServer{responseCode: codes.OK}

		addr, cleanup := startMockGRPCServer(t, mock)
		defer cleanup()

		cfg := &commoncfg.Audit{
			Endpoint: addr,
			Protocol: commoncfg.GRPCProtocol,
			GRPCClient: &commoncfg.GRPCClient{
				Enabled: true,
				Address: addr,
				SecretRef: &commoncfg.SecretRef{
					Type: commoncfg.InsecureSecretType,
				},
			},
		}

		sender, err := newGRPCSender(cfg, WithCompression(false))
		require.NoError(t, err)

		defer sender.Close()

		logs := plog.NewLogs()
		rl := logs.ResourceLogs().AppendEmpty()
		sl := rl.ScopeLogs().AppendEmpty()
		lr := sl.LogRecords().AppendEmpty()
		lr.Body().SetStr("test event")

		err = sender.Send(context.Background(), logs)
		require.NoError(t, err)
		assert.Equal(t, int32(1), mock.received.Load())
		assert.Equal(t, int32(1), mock.totalRecords.Load())
	})

	t.Run("returns permanent error for INVALID_ARGUMENT", func(t *testing.T) {
		mock := &mockLogsServer{
			responseCode: codes.InvalidArgument,
			responseMsg:  "bad request",
		}

		addr, cleanup := startMockGRPCServer(t, mock)
		defer cleanup()

		cfg := &commoncfg.Audit{
			Endpoint: addr,
			Protocol: commoncfg.GRPCProtocol,
			GRPCClient: &commoncfg.GRPCClient{
				SecretRef: &commoncfg.SecretRef{
					Type: commoncfg.InsecureSecretType,
				},
			},
		}

		sender, err := newGRPCSender(cfg, WithCompression(false))
		require.NoError(t, err)

		defer sender.Close()

		logs := plog.NewLogs()
		rl := logs.ResourceLogs().AppendEmpty()
		sl := rl.ScopeLogs().AppendEmpty()
		sl.LogRecords().AppendEmpty()

		err = sender.Send(context.Background(), logs)
		require.Error(t, err)
		assert.ErrorIs(t, err, ErrPermanent)
	})

	t.Run("returns transient error for UNAVAILABLE", func(t *testing.T) {
		mock := &mockLogsServer{
			responseCode: codes.Unavailable,
			responseMsg:  "service unavailable",
		}

		addr, cleanup := startMockGRPCServer(t, mock)
		defer cleanup()

		cfg := &commoncfg.Audit{
			Endpoint: addr,
			Protocol: commoncfg.GRPCProtocol,
			GRPCClient: &commoncfg.GRPCClient{
				SecretRef: &commoncfg.SecretRef{
					Type: commoncfg.InsecureSecretType,
				},
			},
		}

		sender, err := newGRPCSender(cfg, WithCompression(false))
		require.NoError(t, err)

		defer sender.Close()

		logs := plog.NewLogs()
		rl := logs.ResourceLogs().AppendEmpty()
		sl := rl.ScopeLogs().AppendEmpty()
		sl.LogRecords().AppendEmpty()

		err = sender.Send(context.Background(), logs)
		require.Error(t, err)
		assert.NotErrorIs(t, err, ErrPermanent)
		assert.Contains(t, err.Error(), "transient")
	})
}

func TestGRPCSender_ErrorClassification(t *testing.T) {
	tests := []struct {
		name        string
		code        codes.Code
		isPermanent bool
	}{
		// Permanent errors
		{"InvalidArgument", codes.InvalidArgument, true},
		{"NotFound", codes.NotFound, true},
		{"AlreadyExists", codes.AlreadyExists, true},
		{"PermissionDenied", codes.PermissionDenied, true},
		{"Unauthenticated", codes.Unauthenticated, true},
		{"FailedPrecondition", codes.FailedPrecondition, true},
		{"Unimplemented", codes.Unimplemented, true},

		// Transient errors
		{"Unavailable", codes.Unavailable, false},
		{"DeadlineExceeded", codes.DeadlineExceeded, false},
		{"ResourceExhausted", codes.ResourceExhausted, false},
		{"Aborted", codes.Aborted, false},
		{"Internal", codes.Internal, false},
		{"Unknown", codes.Unknown, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockLogsServer{
				responseCode: tt.code,
				responseMsg:  "test error",
			}

			addr, cleanup := startMockGRPCServer(t, mock)
			defer cleanup()

			cfg := &commoncfg.Audit{
				Endpoint: addr,
				Protocol: commoncfg.GRPCProtocol,
				GRPCClient: &commoncfg.GRPCClient{
					SecretRef: &commoncfg.SecretRef{
						Type: commoncfg.InsecureSecretType,
					},
				},
			}

			sender, err := newGRPCSender(cfg, WithCompression(false))
			require.NoError(t, err)

			defer sender.Close()

			logs := plog.NewLogs()
			rl := logs.ResourceLogs().AppendEmpty()
			sl := rl.ScopeLogs().AppendEmpty()
			sl.LogRecords().AppendEmpty()

			err = sender.Send(context.Background(), logs)
			require.Error(t, err)

			if tt.isPermanent {
				assert.ErrorIs(t, err, ErrPermanent)
			} else {
				assert.NotErrorIs(t, err, ErrPermanent)
			}
		})
	}
}

func TestGRPCSender_ConnectionStateMonitoring(t *testing.T) {
	t.Run("invokes callback on state change", func(t *testing.T) {
		mock := &mockLogsServer{responseCode: codes.OK}

		addr, cleanup := startMockGRPCServer(t, mock)
		defer cleanup()

		cfg := &commoncfg.Audit{
			Endpoint: addr,
			Protocol: commoncfg.GRPCProtocol,
			GRPCClient: &commoncfg.GRPCClient{
				SecretRef: &commoncfg.SecretRef{
					Type: commoncfg.InsecureSecretType,
				},
			},
		}

		var (
			states   []ConnectionState
			statesMu sync.Mutex
		)

		sender, err := newGRPCSender(cfg,
			WithCompression(false),
			WithConnectionStateCallback(func(state ConnectionState) {
				statesMu.Lock()

				states = append(states, state)
				statesMu.Unlock()
			}),
		)
		require.NoError(t, err)

		// Send a log to trigger connection
		logs := plog.NewLogs()
		rl := logs.ResourceLogs().AppendEmpty()
		sl := rl.ScopeLogs().AppendEmpty()
		sl.LogRecords().AppendEmpty()

		_ = sender.Send(context.Background(), logs)

		// Wait for state changes
		time.Sleep(200 * time.Millisecond)

		sender.Close()

		statesMu.Lock()
		defer statesMu.Unlock()

		// Should have received at least one state notification
		assert.NotEmpty(t, states, "should have received state notifications")
	})

	t.Run("ConnectionState returns current state", func(t *testing.T) {
		mock := &mockLogsServer{responseCode: codes.OK}

		addr, cleanup := startMockGRPCServer(t, mock)
		defer cleanup()

		cfg := &commoncfg.Audit{
			Endpoint: addr,
			Protocol: commoncfg.GRPCProtocol,
			GRPCClient: &commoncfg.GRPCClient{
				SecretRef: &commoncfg.SecretRef{
					Type: commoncfg.InsecureSecretType,
				},
			},
		}

		sender, err := newGRPCSender(cfg, WithCompression(false))
		require.NoError(t, err)

		defer sender.Close()

		// Initially should be idle or connecting
		state := sender.ConnectionState()
		assert.True(t, state == ConnectionStateIdle || state == ConnectionStateConnecting || state == ConnectionStateReady,
			"unexpected state: %s", state.String())
	})
}

func TestGRPCSender_Compression(t *testing.T) {
	t.Run("compression enabled by default", func(t *testing.T) {
		mock := &mockLogsServer{responseCode: codes.OK}

		addr, cleanup := startMockGRPCServer(t, mock)
		defer cleanup()

		cfg := &commoncfg.Audit{
			Endpoint: addr,
			Protocol: commoncfg.GRPCProtocol,
			GRPCClient: &commoncfg.GRPCClient{
				SecretRef: &commoncfg.SecretRef{
					Type: commoncfg.InsecureSecretType,
				},
			},
		}

		// Default should have compression enabled
		sender, err := newGRPCSender(cfg)
		require.NoError(t, err)

		defer sender.Close()

		logs := plog.NewLogs()
		rl := logs.ResourceLogs().AppendEmpty()
		sl := rl.ScopeLogs().AppendEmpty()
		sl.LogRecords().AppendEmpty()

		err = sender.Send(context.Background(), logs)
		require.NoError(t, err)
		assert.Equal(t, int32(1), mock.received.Load())
	})
}

func TestGRPCSender_RequestTimeout(t *testing.T) {
	t.Run("respects request timeout", func(t *testing.T) {
		mock := &mockLogsServer{responseCode: codes.OK}

		addr, cleanup := startMockGRPCServer(t, mock)
		defer cleanup()

		cfg := &commoncfg.Audit{
			Endpoint: addr,
			Protocol: commoncfg.GRPCProtocol,
			GRPCClient: &commoncfg.GRPCClient{
				SecretRef: &commoncfg.SecretRef{
					Type: commoncfg.InsecureSecretType,
				},
			},
		}

		sender, err := newGRPCSender(cfg,
			WithRequestTimeout(5*time.Second),
			WithCompression(false),
		)
		require.NoError(t, err)

		defer sender.Close()

		logs := plog.NewLogs()
		rl := logs.ResourceLogs().AppendEmpty()
		sl := rl.ScopeLogs().AppendEmpty()
		sl.LogRecords().AppendEmpty()

		err = sender.Send(context.Background(), logs)
		require.NoError(t, err)
	})
}

func TestAuditor_GRPC(t *testing.T) {
	t.Run("sends events via gRPC", func(t *testing.T) {
		mock := &mockLogsServer{responseCode: codes.OK}

		addr, cleanup := startMockGRPCServer(t, mock)
		defer cleanup()

		cfg := &commoncfg.Audit{
			Endpoint: addr,
			Protocol: commoncfg.GRPCProtocol,
			GRPCClient: &commoncfg.GRPCClient{
				SecretRef: &commoncfg.SecretRef{
					Type: commoncfg.InsecureSecretType,
				},
			},
		}

		auditor, err := NewAuditor(cfg,
			WithGRPCBatchSize(1), // Immediate send for testing
			WithGRPCBatchTimeout(50*time.Millisecond),
			WithGRPCCompression(false),
		)
		require.NoError(t, err)

		event := auditlogeventsv1.NewTenantCreatedEvent(
			auditor.NewCommon(),
			"tenant-123",
		)

		err = auditor.Send(event)
		require.NoError(t, err)

		// Wait for delivery
		assert.Eventually(t, func() bool {
			return mock.totalRecords.Load() >= 1
		}, 2*time.Second, 50*time.Millisecond)

		require.NoError(t, auditor.Close(context.Background()))
	})

	t.Run("batches multiple events", func(t *testing.T) {
		mock := &mockLogsServer{responseCode: codes.OK}

		addr, cleanup := startMockGRPCServer(t, mock)
		defer cleanup()

		cfg := &commoncfg.Audit{
			Endpoint: addr,
			Protocol: commoncfg.GRPCProtocol,
			GRPCClient: &commoncfg.GRPCClient{
				SecretRef: &commoncfg.SecretRef{
					Type: commoncfg.InsecureSecretType,
				},
			},
		}

		auditor, err := NewAuditor(cfg,
			WithGRPCBatchSize(5),
			WithGRPCBatchTimeout(1*time.Second),
			WithGRPCCompression(false),
		)
		require.NoError(t, err)

		// Send 5 events to trigger batch
		for i := range 5 {
			event := auditlogeventsv1.NewTenantCreatedEvent(
				auditor.NewCommon(),
				"tenant-"+string(rune('0'+i)),
			)
			err = auditor.Send(event)
			require.NoError(t, err)
		}

		// Wait for delivery
		assert.Eventually(t, func() bool {
			return mock.totalRecords.Load() >= 5
		}, 2*time.Second, 50*time.Millisecond)

		// Should have sent all 5 records in 1 batch (1 gRPC call)
		assert.Equal(t, int32(1), mock.received.Load(), "expected 1 batched gRPC call")
		assert.Equal(t, int32(5), mock.totalRecords.Load(), "expected 5 total records")

		require.NoError(t, auditor.Close(context.Background()))
	})

	t.Run("flushes partial batch on timeout", func(t *testing.T) {
		mock := &mockLogsServer{responseCode: codes.OK}

		addr, cleanup := startMockGRPCServer(t, mock)
		defer cleanup()

		cfg := &commoncfg.Audit{
			Endpoint: addr,
			Protocol: commoncfg.GRPCProtocol,
			GRPCClient: &commoncfg.GRPCClient{
				SecretRef: &commoncfg.SecretRef{
					Type: commoncfg.InsecureSecretType,
				},
			},
		}

		auditor, err := NewAuditor(cfg,
			WithGRPCBatchSize(100),                     // Large batch size
			WithGRPCBatchTimeout(100*time.Millisecond), // Short timeout
			WithGRPCCompression(false),
		)
		require.NoError(t, err)

		// Send only 2 events (less than batch size)
		for i := range 2 {
			event := auditlogeventsv1.NewTenantCreatedEvent(
				auditor.NewCommon(),
				"tenant-"+string(rune('0'+i)),
			)
			err = auditor.Send(event)
			require.NoError(t, err)
		}

		// Wait for timeout flush
		assert.Eventually(t, func() bool {
			return mock.totalRecords.Load() >= 2
		}, 500*time.Millisecond, 50*time.Millisecond)

		require.NoError(t, auditor.Close(context.Background()))
	})

	t.Run("retries transient gRPC errors", func(t *testing.T) {
		mock := &mockLogsServer{
			responseCode: codes.Unavailable,
			responseMsg:  "service unavailable",
		}

		addr, cleanup := startMockGRPCServer(t, mock)
		defer cleanup()

		cfg := &commoncfg.Audit{
			Endpoint: addr,
			Protocol: commoncfg.GRPCProtocol,
			GRPCClient: &commoncfg.GRPCClient{
				SecretRef: &commoncfg.SecretRef{
					Type: commoncfg.InsecureSecretType,
				},
			},
		}

		var receivedErr atomic.Value

		auditor, err := NewAuditor(cfg,
			WithMaxRetries(2),
			WithBaseDelay(10*time.Millisecond),
			WithMaxDelay(20*time.Millisecond),
			WithGRPCBatchSize(1),
			WithGRPCCompression(false),
			WithOnError(func(err error) {
				receivedErr.Store(err)
			}),
		)
		require.NoError(t, err)

		event := auditlogeventsv1.NewTenantCreatedEvent(
			auditor.NewCommon(),
			"tenant-123",
		)

		err = auditor.Send(event)
		require.NoError(t, err)

		// Wait for OnError to be called
		assert.Eventually(t, func() bool {
			return receivedErr.Load() != nil
		}, 2*time.Second, 50*time.Millisecond)

		require.NoError(t, auditor.Close(context.Background()))

		// With gRPC service config retry (default 3 attempts per call) and auditor retry (1 initial + 2 retries = 3 calls),
		// we get 3 * 3 = 9 total attempts for UNAVAILABLE errors since both layers retry.
		assert.GreaterOrEqual(t, mock.received.Load(), int32(3), "should have multiple retry attempts")
	})

	t.Run("does not retry permanent gRPC errors", func(t *testing.T) {
		mock := &mockLogsServer{
			responseCode: codes.PermissionDenied,
			responseMsg:  "permission denied",
		}

		addr, cleanup := startMockGRPCServer(t, mock)
		defer cleanup()

		cfg := &commoncfg.Audit{
			Endpoint: addr,
			Protocol: commoncfg.GRPCProtocol,
			GRPCClient: &commoncfg.GRPCClient{
				SecretRef: &commoncfg.SecretRef{
					Type: commoncfg.InsecureSecretType,
				},
			},
		}

		var receivedErr atomic.Value

		auditor, err := NewAuditor(cfg,
			WithMaxRetries(5),
			WithBaseDelay(10*time.Millisecond),
			WithGRPCBatchSize(1),
			WithGRPCCompression(false),
			WithOnError(func(err error) {
				receivedErr.Store(err)
			}),
		)
		require.NoError(t, err)

		event := auditlogeventsv1.NewTenantCreatedEvent(
			auditor.NewCommon(),
			"tenant-123",
		)

		err = auditor.Send(event)
		require.NoError(t, err)

		// Wait for OnError to be called
		assert.Eventually(t, func() bool {
			return receivedErr.Load() != nil
		}, 2*time.Second, 50*time.Millisecond)

		require.NoError(t, auditor.Close(context.Background()))

		// Should only attempt once (no retries for permanent errors)
		assert.Equal(t, int32(1), mock.received.Load())

		// OnError should have been called with ErrPermanent
		storedErr, ok := receivedErr.Load().(error)
		require.True(t, ok, "expected error to be stored")
		assert.ErrorIs(t, storedErr, ErrPermanent)
	})

	t.Run("connection state callback via auditor options", func(t *testing.T) {
		mock := &mockLogsServer{responseCode: codes.OK}

		addr, cleanup := startMockGRPCServer(t, mock)
		defer cleanup()

		cfg := &commoncfg.Audit{
			Endpoint: addr,
			Protocol: commoncfg.GRPCProtocol,
			GRPCClient: &commoncfg.GRPCClient{
				SecretRef: &commoncfg.SecretRef{
					Type: commoncfg.InsecureSecretType,
				},
			},
		}

		var stateReceived atomic.Bool

		auditor, err := NewAuditor(cfg,
			WithGRPCBatchSize(1),
			WithGRPCCompression(false),
			WithGRPCConnectionStateCallback(func(state ConnectionState) {
				stateReceived.Store(true)
			}),
		)
		require.NoError(t, err)

		// Send an event to trigger connection
		event := auditlogeventsv1.NewTenantCreatedEvent(
			auditor.NewCommon(),
			"tenant-123",
		)
		_ = auditor.Send(event)

		// Wait for state callback
		assert.Eventually(t, func() bool {
			return stateReceived.Load()
		}, 2*time.Second, 50*time.Millisecond)

		require.NoError(t, auditor.Close(context.Background()))
	})
}

func TestNewGRPCSender_ConfigValidation(t *testing.T) {
	t.Run("returns error when GRPCClient is nil", func(t *testing.T) {
		cfg := &commoncfg.Audit{
			Endpoint: "localhost:4317",
			Protocol: commoncfg.GRPCProtocol,
			// GRPCClient is nil
		}

		_, err := newGRPCSender(cfg)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "grpcClient config is required")
	})

	t.Run("returns error for unsupported secret type", func(t *testing.T) {
		cfg := &commoncfg.Audit{
			Endpoint: "localhost:4317",
			Protocol: commoncfg.GRPCProtocol,
			GRPCClient: &commoncfg.GRPCClient{
				SecretRef: &commoncfg.SecretRef{
					Type: "unsupported",
				},
			},
		}

		_, err := newGRPCSender(cfg)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unsupported secret type")
	})
}

func TestConnectionState_String(t *testing.T) {
	tests := []struct {
		state    ConnectionState
		expected string
	}{
		{ConnectionStateIdle, "IDLE"},
		{ConnectionStateConnecting, "CONNECTING"},
		{ConnectionStateReady, "READY"},
		{ConnectionStateTransientFailure, "TRANSIENT_FAILURE"},
		{ConnectionStateShutdown, "SHUTDOWN"},
		{ConnectionState(99), "UNKNOWN"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.state.String())
		})
	}
}
