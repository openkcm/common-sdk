package featureflags_test

import (
	"context"
	"errors"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/open-feature/go-sdk/openfeature"
	"github.com/thomaspoignant/go-feature-flag/modules/core/flag"
	"github.com/thomaspoignant/go-feature-flag/modules/core/model"

	"github.com/openkcm/common-sdk/pkg/commoncfg"
	. "github.com/openkcm/common-sdk/pkg/featureflags"
)

// flagsFile returns the absolute path to the test fixture flag file.
func flagsFile(t *testing.T) string {
	t.Helper()

	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}

	return filepath.Join(filepath.Dir(file), "testdata", "flags.yaml")
}

// newProvider creates a valid provider pointing at the test fixture.
func newProvider(t *testing.T) openfeature.FeatureProvider {
	t.Helper()

	p, err := NewEmbeddedProvider(commoncfg.FeatureFlags{
		FilePath:        flagsFile(t),
		PollingInterval: 0,
	})
	if err != nil {
		t.Fatalf("NewEmbeddedProvider: %v", err)
	}

	return p
}

// initProvider initialises the provider and registers a cleanup that shuts it down.
func initProvider(t *testing.T, p openfeature.FeatureProvider) {
	t.Helper()

	sh, ok := p.(openfeature.StateHandler)
	if !ok {
		t.Fatal("provider does not implement StateHandler")
	}

	err := sh.Init(openfeature.EvaluationContext{})
	if err != nil {
		t.Fatalf("Init: %v", err)
	}

	t.Cleanup(sh.Shutdown)
}

// ---- constructor ----------------------------------------------------------------

func TestNewEmbeddedProvider_EmptyFilePath(t *testing.T) {
	cfg := commoncfg.FeatureFlags{Enabled: true, FilePath: "", PollingInterval: 60 * time.Second}

	_, err := NewEmbeddedProvider(cfg)
	if err == nil {
		t.Fatal("expected error for empty FilePath, got nil")
	}

	if !errors.Is(err, ErrEmptyFilePath) {
		t.Fatalf("expected ErrEmptyFilePath, got %v", err)
	}
}

func TestNewEmbeddedProvider_Valid(t *testing.T) {
	cfg := commoncfg.FeatureFlags{Enabled: true, FilePath: "/etc/featureflags/flags.yaml", PollingInterval: 60 * time.Second}

	p, err := NewEmbeddedProvider(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if p == nil {
		t.Fatal("expected non-nil provider")
	}
}

// ---- Metadata / Hooks -----------------------------------------------------------

func TestEmbeddedProvider_Metadata(t *testing.T) {
	p := newProvider(t)

	name := p.Metadata().Name
	if name == "" {
		t.Fatal("expected non-empty provider name")
	}
}

func TestEmbeddedProvider_Hooks(t *testing.T) {
	p := newProvider(t)

	if hooks := p.Hooks(); hooks != nil {
		t.Fatalf("expected nil hooks, got %v", hooks)
	}
}

// ---- StateHandler ---------------------------------------------------------------

func TestEmbeddedProvider_Status(t *testing.T) {
	p := newProvider(t)

	sh, ok := p.(interface{ Status() openfeature.State })
	if !ok {
		t.Fatal("provider does not implement Status()")
	}

	if got := sh.Status(); got != openfeature.NotReadyState {
		t.Fatalf("expected NotReadyState before Init, got %v", got)
	}

	initProvider(t, p)

	if got := sh.Status(); got != openfeature.ReadyState {
		t.Fatalf("expected ReadyState after Init, got %v", got)
	}
}

func TestEmbeddedProvider_Init_ValidFile(t *testing.T) {
	p := newProvider(t)
	initProvider(t, p)
}

func TestEmbeddedProvider_Init_MissingFile(t *testing.T) {
	p, err := NewEmbeddedProvider(commoncfg.FeatureFlags{FilePath: "/nonexistent/flags.yaml"})
	if err != nil {
		t.Fatalf("unexpected construction error: %v", err)
	}

	sh, ok := p.(openfeature.StateHandler)
	if !ok {
		t.Fatal("provider does not implement StateHandler")
	}

	err = sh.Init(openfeature.EvaluationContext{})
	if err == nil {
		t.Fatal("expected Init to fail for missing file, got nil")
	}
}

func TestEmbeddedProvider_Shutdown_WithoutInit(t *testing.T) {
	// Shutdown on an uninitialised provider must not panic.
	p := newProvider(t)

	sh, ok := p.(openfeature.StateHandler)
	if !ok {
		t.Fatal("provider does not implement StateHandler")
	}

	sh.Shutdown()
}

func TestEmbeddedProvider_Shutdown_AfterInit(t *testing.T) {
	p := newProvider(t)

	sh, ok := p.(openfeature.StateHandler)
	if !ok {
		t.Fatal("provider does not implement StateHandler")
	}

	err := sh.Init(openfeature.EvaluationContext{})
	if err != nil {
		t.Fatalf("Init: %v", err)
	}

	sh.Shutdown() // must not panic
	sh.Shutdown() // second call must also be safe
}

// ---- Evaluation methods ---------------------------------------------------------

func TestEmbeddedProvider_BooleanEvaluation(t *testing.T) {
	p := newProvider(t)
	initProvider(t, p)

	ctx := context.Background()
	flat := openfeature.FlattenedContext{"targetingKey": "tenant-abc"}

	res := p.BooleanEvaluation(ctx, "bool-flag", false, flat)
	if !res.Value {
		t.Fatalf("expected true for targeting key tenant-abc, got false")
	}

	// Default rule returns false.
	flatOther := openfeature.FlattenedContext{"targetingKey": "other"}

	res2 := p.BooleanEvaluation(ctx, "bool-flag", true, flatOther)
	if res2.Value {
		t.Fatalf("expected false from default rule, got true")
	}
}

func TestEmbeddedProvider_BooleanEvaluation_UnknownFlag(t *testing.T) {
	p := newProvider(t)
	initProvider(t, p)

	res := p.BooleanEvaluation(context.Background(), "no-such-flag", true, openfeature.FlattenedContext{})
	if res.Value != true {
		t.Fatalf("expected default value true on unknown flag, got %v", res.Value)
	}

	if res.Reason != openfeature.ErrorReason {
		t.Fatalf("expected ErrorReason, got %v", res.Reason)
	}
}

func TestEmbeddedProvider_StringEvaluation(t *testing.T) {
	p := newProvider(t)
	initProvider(t, p)

	res := p.StringEvaluation(context.Background(), "string-flag", "default", openfeature.FlattenedContext{})
	if res.Value != "hello" {
		t.Fatalf("expected %q, got %q", "hello", res.Value)
	}
}

func TestEmbeddedProvider_IntEvaluation(t *testing.T) {
	p := newProvider(t)
	initProvider(t, p)

	res := p.IntEvaluation(context.Background(), "int-flag", 0, openfeature.FlattenedContext{})
	if res.Value != 42 {
		t.Fatalf("expected 42, got %d", res.Value)
	}
}

func TestEmbeddedProvider_FloatEvaluation(t *testing.T) {
	p := newProvider(t)
	initProvider(t, p)

	res := p.FloatEvaluation(context.Background(), "float-flag", 0, openfeature.FlattenedContext{})
	if res.Value != 3.14 {
		t.Fatalf("expected 3.14, got %v", res.Value)
	}
}

func TestEmbeddedProvider_ObjectEvaluation(t *testing.T) {
	p := newProvider(t)
	initProvider(t, p)

	defaultVal := map[string]any{"key": "fallback"}

	res := p.ObjectEvaluation(context.Background(), "object-flag", defaultVal, openfeature.FlattenedContext{})

	m, ok := res.Value.(map[string]any)
	if !ok {
		t.Fatalf("expected map[string]any, got %T", res.Value)
	}

	if m["key"] != "value" {
		t.Fatalf("expected key=value, got %v", m["key"])
	}
}

func TestEmbeddedProvider_ObjectEvaluation_NonMapDefault(t *testing.T) {
	p := newProvider(t)
	initProvider(t, p)

	res := p.ObjectEvaluation(context.Background(), "object-flag", "not-a-map", openfeature.FlattenedContext{})
	if res.Reason != openfeature.ErrorReason {
		t.Fatalf("expected ErrorReason for non-map default, got %v", res.Reason)
	}

	if res.Value != "not-a-map" {
		t.Fatalf("expected default value to be returned unchanged")
	}
}

// ---- toEvalContext --------------------------------------------------------------

func TestToEvalContext_TargetingKey(t *testing.T) {
	flatCtx := openfeature.FlattenedContext{
		"targetingKey": "tenant-abc",
	}

	ctx := ToEvalContext(flatCtx)
	if ctx.GetKey() != "tenant-abc" {
		t.Fatalf("expected key %q, got %q", "tenant-abc", ctx.GetKey())
	}
}

func TestToEvalContext_CustomAttributes(t *testing.T) {
	flatCtx := openfeature.FlattenedContext{
		"targetingKey": "tenant-abc",
		"region":       "eu-west-1",
	}

	ctx := ToEvalContext(flatCtx)

	custom := ctx.GetCustom()

	val, ok := custom["region"]
	if !ok {
		t.Fatal("expected custom attribute 'region' to be present")
	}

	if val != "eu-west-1" {
		t.Fatalf("expected region %q, got %v", "eu-west-1", val)
	}
}

func TestToEvalContext_MissingTargetingKey(t *testing.T) {
	flatCtx := openfeature.FlattenedContext{
		"region": "eu-west-1",
	}

	ctx := ToEvalContext(flatCtx)
	if ctx.GetKey() != "" {
		t.Fatalf("expected empty key, got %q", ctx.GetKey())
	}
}

func TestToEvalContext_NonStringTargetingKey(t *testing.T) {
	flatCtx := openfeature.FlattenedContext{
		"targetingKey": 42, // non-string: should fall back to empty key
	}

	ctx := ToEvalContext(flatCtx)
	if ctx.GetKey() != "" {
		t.Fatalf("expected empty key for non-string targeting key, got %q", ctx.GetKey())
	}
}

// ---- toResolutionDetail ---------------------------------------------------------

func TestToResolutionDetail_Success(t *testing.T) {
	res := model.VariationResult[bool]{
		Value:         true,
		VariationType: "on",
		Reason:        "STATIC",
	}

	prd := ToResolutionDetailBool(res, nil)

	if prd.Variant != "on" {
		t.Fatalf("expected variant %q, got %q", "on", prd.Variant)
	}

	if prd.Reason != openfeature.Reason("STATIC") {
		t.Fatalf("expected reason STATIC, got %v", prd.Reason)
	}

	if (prd.ResolutionError != openfeature.ResolutionError{}) {
		t.Fatalf("expected no resolution error, got %v", prd.ResolutionError)
	}
}

func TestToResolutionDetail_ExternalError(t *testing.T) {
	res := model.VariationResult[bool]{}

	prd := ToResolutionDetailBool(res, errors.New("network failure"))
	if prd.Reason != openfeature.ErrorReason {
		t.Fatalf("expected ErrorReason, got %v", prd.Reason)
	}

	if prd.ResolutionError.Error() == "" {
		t.Fatal("expected non-empty resolution error message")
	}
}

func TestToResolutionDetail_FailedResult(t *testing.T) {
	res := model.VariationResult[bool]{
		Failed:       true,
		ErrorCode:    flag.ErrorCodeFlagNotFound,
		ErrorDetails: "flag missing",
	}

	prd := ToResolutionDetailBool(res, nil)
	if prd.Reason != openfeature.ErrorReason {
		t.Fatalf("expected ErrorReason, got %v", prd.Reason)
	}
}

// ---- goffErrorToResolutionError -------------------------------------------------

func TestGoffErrorToResolutionError(t *testing.T) {
	cases := []struct {
		code        string
		wantContain string
	}{
		{flag.ErrorCodeFlagNotFound, string(openfeature.FlagNotFoundCode)},
		{flag.ErrorCodeTypeMismatch, string(openfeature.TypeMismatchCode)},
		{flag.ErrorCodeProviderNotReady, string(openfeature.ProviderNotReadyCode)},
		{flag.ErrorCodeParseError, string(openfeature.ParseErrorCode)},
		{flag.ErrorCodeTargetingKeyMissing, string(openfeature.TargetingKeyMissingCode)},
		{flag.ErrorCodeInvalidContext, string(openfeature.InvalidContextCode)},
		{"UNKNOWN_CODE", string(openfeature.GeneralCode)},
	}

	for _, tc := range cases {
		t.Run(tc.code, func(t *testing.T) {
			resErr := GoffErrorToResolutionError(tc.code, "details")

			errStr := resErr.Error()
			if errStr == "" {
				t.Fatal("expected non-empty error string")
			}

			if !containsSubstring(errStr, tc.wantContain) {
				t.Fatalf("expected error string to contain %q, got %q", tc.wantContain, errStr)
			}
		})
	}
}

func containsSubstring(s, substr string) bool {
	if len(substr) == 0 {
		return true
	}

	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}

	return false
}
