package status_test

import (
	"net/http"
	"testing"
	"time"

	"github.com/openkcm/common-sdk/pkg/commoncfg"
	"github.com/openkcm/common-sdk/pkg/status"
)

func TestStartDisabled(t *testing.T) {
	// Arrange
	cfg := &commoncfg.BaseConfig{
		Status: commoncfg.Status{
			Enabled: false,
		},
	}

	// Act
	err := status.Start(t.Context(), cfg)

	// Assert
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestStartWithInvalidAddress(t *testing.T) {
	// Arrange
	cfg := &commoncfg.BaseConfig{
		Status: commoncfg.Status{
			Enabled: true,
			Address: "invalid-address",
		},
	}

	// Act
	err := status.Start(t.Context(), cfg)

	// Assert
	if err == nil {
		t.Error("expected error, but got nil")
	}
}

func TestStart(t *testing.T) {
	// Arrange
	cfg := &commoncfg.BaseConfig{
		Status: commoncfg.Status{
			Enabled:   true,
			Address:   ":8080",
			Profiling: true,
		},
		Telemetry: commoncfg.Telemetry{
			Metrics: commoncfg.Metric{
				Prometheus: commoncfg.Prometheus{
					Enabled: true,
				},
			},
		},
	}

	fn := func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		//nolint:errcheck
		w.Write([]byte("OK"))
	}

	// define the options
	opts := []status.ProbeOption{
		nil,
		status.WithCustom("nil", nil),
		status.WithCustom("foobar", fn),
		status.WithStartup(fn),
		status.WithLiveness(fn),
		status.WithHealthZ(fn),
		status.WithReadiness(fn),
	}

	// Act
	go func() {
		if err := status.Start(t.Context(), cfg, opts...); err != nil {
			t.Errorf("failed to start status server: %v", err)
		}
	}()

	// wait for the status server to start
	start := time.Now()
	for {
		time.Sleep(100 * time.Millisecond)
		if time.Since(start) > 5*time.Second {
			t.Fatalf("status server did not start within the timeout")
		}
		resp, err := http.Get("http://localhost:8080/probe/startup")
		if err != nil {
			continue
		}
		//nolint:errcheck
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			continue
		}
		break
	}

	// test all endpoints
	for _, endpoint := range []string{
		"/probe/foobar",
		"/probe/startup",
		"/probe/liveness",
		"/probe/healthz",
		"/probe/readiness",
		"/version",
	} {
		resp, err := http.Get("http://localhost:8080" + endpoint)
		if err != nil {
			t.Errorf("failed to get %s: %v", endpoint, err)
			continue
		}
		//nolint:errcheck
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Errorf("expected status OK for %s, got %v", endpoint, resp.Status)
		}
	}
}
