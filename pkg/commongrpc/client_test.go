package commongrpc_test

import (
	"context"
	"net"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"

	healthpb "google.golang.org/grpc/health/grpc_health_v1"

	"github.com/openkcm/common-sdk/pkg/commoncfg"
	"github.com/openkcm/common-sdk/pkg/commongrpc"
)

// failingHealthServer returns UNAVAILABLE for the first `failFor` calls, then SERVING.
type failingHealthServer struct {
	healthpb.UnimplementedHealthServer

	calls   atomic.Int32
	failFor int32
}

func (s *failingHealthServer) Check(_ context.Context, _ *healthpb.HealthCheckRequest) (*healthpb.HealthCheckResponse, error) {
	n := s.calls.Add(1)
	if n <= s.failFor {
		return nil, status.Error(codes.Unavailable, "not ready yet")
	}

	return &healthpb.HealthCheckResponse{Status: healthpb.HealthCheckResponse_SERVING}, nil
}

// startBufconnServer starts an in-process gRPC server and returns a dial option
// that routes connections through it.
func startBufconnServer(t *testing.T, srv healthpb.HealthServer) grpc.DialOption {
	t.Helper()

	lis := bufconn.Listen(1024 * 1024)

	t.Cleanup(func() { lis.Close() })

	s := grpc.NewServer()
	healthpb.RegisterHealthServer(s, srv)
	t.Cleanup(s.Stop)

	go s.Serve(lis) //nolint:errcheck

	return grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
		return lis.DialContext(ctx)
	})
}

func TestNewClientRetry(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		failFor       int32 // number of calls that return UNAVAILABLE before succeeding
		retry         commoncfg.GRPCRetry
		wantErr       bool
		wantCallCount int32
	}{
		{
			name:          "no retry — fails on first UNAVAILABLE",
			failFor:       1,
			retry:         commoncfg.GRPCRetry{},
			wantErr:       true,
			wantCallCount: 1,
		},
		{
			name:    "retry succeeds after 2 failures",
			failFor: 2,
			retry: commoncfg.GRPCRetry{
				MaxAttemps:           3,
				InitialBackoff:       10 * time.Millisecond,
				MaxBackoff:           100 * time.Millisecond,
				BackoffMultiplier:    2,
				RetryableStatusCodes: []string{"UNAVAILABLE"},
			},
			wantErr:       false,
			wantCallCount: 3,
		},
		{
			name:    "retry exhausted — all attempts fail",
			failFor: 5,
			retry: commoncfg.GRPCRetry{
				MaxAttemps:           3,
				InitialBackoff:       10 * time.Millisecond,
				MaxBackoff:           100 * time.Millisecond,
				BackoffMultiplier:    2,
				RetryableStatusCodes: []string{"UNAVAILABLE"},
			},
			wantErr:       true,
			wantCallCount: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cfg := &commoncfg.GRPCClient{
				Address: "passthrough:///bufconn",
				Attributes: commoncfg.GRPCClientAttributes{
					Retry: tt.retry,
				},
			}

			srv := &failingHealthServer{failFor: tt.failFor}
			conn, err := commongrpc.NewClient(cfg, startBufconnServer(t, srv))
			require.NoError(t, err)

			defer conn.Close()

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			_, callErr := healthpb.NewHealthClient(conn).Check(ctx, &healthpb.HealthCheckRequest{})
			if tt.wantErr {
				require.Error(t, callErr)
			} else {
				require.NoError(t, callErr)
			}

			assert.Equal(t, tt.wantCallCount, srv.calls.Load())
		})
	}
}
