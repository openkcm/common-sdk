package status

import (
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
)

func TestRegisterRuntimeEvidence(t *testing.T) {
	// Arrange
	reg := prometheus.NewRegistry()

	// Act: registering twice must be idempotent (no panic, no error).
	registerRuntimeEvidence(t.Context(), reg)
	registerRuntimeEvidence(t.Context(), reg)

	families, err := reg.Gather()
	if err != nil {
		t.Fatalf("failed to gather metrics: %v", err)
	}

	gathered := make(map[string]int, len(families))
	for _, fam := range families {
		gathered[fam.GetName()] = len(fam.GetMetric())
	}

	// Assert: all evidence metrics are exposed exactly once.
	for _, name := range []string{
		"go_build_info",
		"go_build_settings_info",
		"go_fips140_enabled",
		"go_fips140_info",
	} {
		if got := gathered[name]; got != 1 {
			t.Errorf("expected exactly 1 sample for %q, got %d", name, got)
		}
	}
}

func TestRuntimeEvidenceCollectorLabels(t *testing.T) {
	// Arrange
	reg := prometheus.NewRegistry()

	err := reg.Register(runtimeEvidenceCollector{})
	if err != nil {
		t.Fatalf("failed to register collector: %v", err)
	}

	// Act
	families, err := reg.Gather()
	if err != nil {
		t.Fatalf("failed to gather metrics: %v", err)
	}

	// Assert: go_build_settings_info carries the allowlisted label set.
	wantLabels := []string{
		"go_version",
		"goos",
		"goarch",
		"cgo_enabled",
		"gofips140",
		"default_godebug",
		"vcs_revision",
		"vcs_modified",
	}

	var found bool

	for _, fam := range families {
		if fam.GetName() != "go_build_settings_info" {
			continue
		}

		found = true

		metrics := fam.GetMetric()
		if len(metrics) != 1 {
			t.Fatalf("expected 1 go_build_settings_info metric, got %d", len(metrics))
		}

		got := make(map[string]struct{})
		for _, lp := range metrics[0].GetLabel() {
			got[lp.GetName()] = struct{}{}
		}

		for _, want := range wantLabels {
			if _, ok := got[want]; !ok {
				t.Errorf("go_build_settings_info missing label %q", want)
			}
		}
	}

	if !found {
		t.Error("go_build_settings_info metric family not found")
	}
}

func TestReadBuildSettings(t *testing.T) {
	// Act
	s := readBuildSettings()

	// Assert: the Go version is always available from ReadBuildInfo.
	if !strings.HasPrefix(s.goVersion, "go") {
		t.Errorf("expected goVersion to start with %q, got %q", "go", s.goVersion)
	}
}
