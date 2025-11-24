package health_test

import (
	"errors"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	healthpb "google.golang.org/grpc/health/grpc_health_v1"

	"github.com/openkcm/common-sdk/v2/pkg/health"
)

func TestCheck(t *testing.T) {
	// Arrange
	s := &health.GRPCServer{}

	// Act
	resp, err := s.Check(t.Context(), nil)

	// Assert
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if resp.GetStatus() != healthpb.HealthCheckResponse_SERVING {
		t.Errorf("unexpected status: %v", resp.GetStatus())
	}
}

func TestWatch(t *testing.T) {
	// Arrange
	s := &health.GRPCServer{}

	// Act
	err := s.Watch(nil, nil)

	// Assert
	if !errors.Is(err, status.Error(codes.Unimplemented, "Watch is not yet implemented")) {
		t.Errorf("expected unimplemented error, got: %v", err)
	}
}
