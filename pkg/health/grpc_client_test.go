package health_test

import (
	"net"
	"testing"
	"time"

	"google.golang.org/grpc"

	healthpb "google.golang.org/grpc/health/grpc_health_v1"

	"github.com/openkcm/common-sdk/pkg/commoncfg"
	"github.com/openkcm/common-sdk/pkg/health"
)

func TestCheckGRPCServerHealth(t *testing.T) {
	// Arrange
	grpcServer := grpc.NewServer()
	healthpb.RegisterHealthServer(grpcServer, &health.GRPCServer{})
	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatalf("failed to listen: %v", err)
	}
	go func() {
		if err := grpcServer.Serve(listener); err != nil {
			t.Errorf("failed to serve: %v", err)
		}
	}()
	defer grpcServer.GracefulStop()

	// create the test cases
	tests := []struct {
		name      string
		grpcCfg   *commoncfg.GRPCClient
		wantError bool
	}{
		{
			name:    "gRPC client config is nil",
			grpcCfg: nil,
		}, {
			name:      "no gRPC server listening",
			grpcCfg:   &commoncfg.GRPCClient{Address: "localhost:9999"},
			wantError: true,
		}, {
			name: "gRPC server listening",
			grpcCfg: func() *commoncfg.GRPCClient {
				cfg := &commoncfg.GRPCClient{Address: listener.Addr().String()}
				cfg.Pool = commoncfg.GRPCPool{
					InitialCapacity: 1,
					MaxCapacity:     3,
					IdleTimeout:     3 * time.Second,
					MaxLifeDuration: 60 * time.Second,
				}
				return cfg
			}(),
		},
	}

	// run the tests
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Arrange

			// Act
			err = health.CheckGRPCServerHealth(t.Context(), tc.grpcCfg)

			// Assert
			if tc.wantError {
				if err == nil {
					t.Error("expected error, but got nil")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %s", err)
				}
			}
		})
	}
}
