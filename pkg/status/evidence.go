package status

import (
	"context"
	"crypto/fips140"
	"errors"
	"runtime/debug"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"

	slogctx "github.com/veqryn/slog-context"
)

// Build and runtime evidence metric descriptors. These expose standard module
// build identity, an allowlisted subset of build settings, and FIPS 140-3
// runtime evidence so that services can prove how their binary was built and
// whether the Go Cryptographic Module is active in the running process.
var (
	// go_build_settings_info exposes a safe, allowlisted subset of the build
	// settings reported by runtime/debug.ReadBuildInfo. Its value is always 1;
	// the evidence lives in the labels.
	buildSettingsDesc = prometheus.NewDesc(
		"go_build_settings_info",
		"Selected, safe build settings",
		[]string{
			"go_version",
			"goos",
			"goarch",
			"cgo_enabled",
			"gofips140",
			"default_godebug",
			"vcs_revision",
			"vcs_modified",
		},
		nil,
	)

	// go_fips140_enabled is 1 when FIPS mode is active in the running process.
	fips140EnabledDesc = prometheus.NewDesc(
		"go_fips140_enabled",
		"1 when FIPS mode is active in the running process",
		nil,
		nil,
	)

	// go_fips140_info exposes the Go Cryptographic Module version as a label.
	// Its value is always 1.
	fips140InfoDesc = prometheus.NewDesc(
		"go_fips140_info",
		"Go Cryptographic Module version",
		[]string{"version"},
		nil,
	)
)

// buildSettings holds the allowlisted build settings extracted from
// runtime/debug.ReadBuildInfo.
type buildSettings struct {
	goVersion      string
	goos           string
	goarch         string
	cgoEnabled     string
	gofips140      string
	defaultGODEBUG string
	vcsRevision    string
	vcsModified    string
}

// readBuildSettings reads the allowlisted build settings from the running
// binary. Unknown or absent settings are left empty.
func readBuildSettings() buildSettings {
	s := buildSettings{}

	info, ok := debug.ReadBuildInfo()
	if !ok {
		return s
	}

	s.goVersion = info.GoVersion

	for _, setting := range info.Settings {
		switch setting.Key {
		case "GOOS":
			s.goos = setting.Value
		case "GOARCH":
			s.goarch = setting.Value
		case "CGO_ENABLED":
			s.cgoEnabled = setting.Value
		case "GOFIPS140":
			s.gofips140 = setting.Value
		case "DefaultGODEBUG":
			s.defaultGODEBUG = setting.Value
		case "vcs.revision":
			s.vcsRevision = setting.Value
		case "vcs.modified":
			s.vcsModified = setting.Value
		}
	}

	return s
}

// runtimeEvidenceCollector is a prometheus.Collector that exposes the
// allowlisted build settings and FIPS 140-3 runtime evidence of the running
// process.
type runtimeEvidenceCollector struct{}

// Describe implements prometheus.Collector.
func (runtimeEvidenceCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- buildSettingsDesc

	ch <- fips140EnabledDesc

	ch <- fips140InfoDesc
}

// Collect implements prometheus.Collector.
func (runtimeEvidenceCollector) Collect(ch chan<- prometheus.Metric) {
	s := readBuildSettings()
	ch <- prometheus.MustNewConstMetric(
		buildSettingsDesc,
		prometheus.GaugeValue,
		1,
		s.goVersion,
		s.goos,
		s.goarch,
		s.cgoEnabled,
		s.gofips140,
		s.defaultGODEBUG,
		s.vcsRevision,
		s.vcsModified,
	)

	fipsEnabled := 0.0
	if fips140.Enabled() {
		fipsEnabled = 1.0
	}

	ch <- prometheus.MustNewConstMetric(fips140EnabledDesc, prometheus.GaugeValue, fipsEnabled)

	ch <- prometheus.MustNewConstMetric(fips140InfoDesc, prometheus.GaugeValue, 1, fips140.Version())
}

// registerRuntimeEvidence registers the standard build-info collector together
// with the build settings and FIPS runtime evidence collector on the given
// registerer. Collectors that are already registered (e.g. when Start is called
// more than once in the same process) are skipped so registration is
// idempotent.
func registerRuntimeEvidence(ctx context.Context, reg prometheus.Registerer) {
	toRegister := []prometheus.Collector{
		// go_build_info{path,version,checksum}: standard module build identity.
		collectors.NewBuildInfoCollector(),
		// go_build_settings_info, go_fips140_enabled and go_fips140_info.
		runtimeEvidenceCollector{},
	}

	for _, c := range toRegister {
		err := reg.Register(c)
		if err == nil {
			continue
		}

		_, alreadyRegistered := errors.AsType[prometheus.AlreadyRegisteredError](err)
		if alreadyRegistered {
			continue
		}

		slogctx.Error(ctx, "Failed registering build evidence metrics", "error", err)
	}
}
