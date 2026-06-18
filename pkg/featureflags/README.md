# featureflags

Embedded [OpenFeature](https://openfeature.dev/) provider backed by [GO Feature Flag](https://gofeatureflag.org/) (GOFF). Flags are read from a local YAML file and evaluated entirely in-process — no relay proxy required.

## Usage

### 1. Configure

Add `featureFlags` to your service's `config.yaml`:

```yaml
featureFlags:
  enabled: true
  filePath: /etc/featureflags/flags.yaml
  pollingInterval: 60s
```

A `pollingInterval` of `0s` disables polling and reads the file only once at startup. The file is read on every poll, so updates to the file are reflected in the service without a restart.

### 2. Wire up in `main()`

```go
import (
    "github.com/open-feature/go-sdk/openfeature"
    "github.com/openkcm/common-sdk/pkg/featureflags"
)

if cfg.FeatureFlags.Enabled {
    provider, err := featureflags.NewEmbeddedProvider(cfg.FeatureFlags)
    if err != nil {
        log.Error("failed to create feature flag provider", "error", err)
        os.Exit(1)
    }
    if err := openfeature.SetProviderAndWait(provider); err != nil {
        log.Error("failed to initialise feature flag provider", "error", err)
        os.Exit(1)
    }
}
```

When `Enabled` is `false`, no provider is registered. The OpenFeature SDK's built-in NoOp provider is used and all evaluations return the supplied default value silently.

### 3. Evaluate flags

```go
client := openfeature.NewClient("my-service")
evalCtx := openfeature.NewEvaluationContext(tenantID, map[string]interface{}{
    "tenantID": tenantID,
})
enabled, err := client.BooleanValue(ctx, "cmk_my_flag", false, evalCtx)
```

See the [OpenFeature Go SDK](https://github.com/open-feature/go-sdk) for `StringValue`, `IntValue`, and `FloatValue`.

## Flag file format

```yaml
cmk_enable_new_flow:
  variations:
    enabled: true
    disabled: false
  defaultRule:
    variation: disabled
  targeting:
    - query: targetingKey eq "tenant-abc"
      variation: enabled
```

Flag names can be prefixed by owning service (`cmk_`, `registry_`, etc.). Cross-service flags can use a `shared_` prefix or no prefix. 

## Testing

**Do not use `EmbeddedProvider` in unit tests.** Its `Init` method calls `ffclient.Init`, which is a global singleton — a second call reinitialises it and will stomp state set by a concurrent test. Two patterns avoid this:

### Option T1 — Serialize flag-touching tests (simpler)

Register an `InMemoryProvider` directly and reset it in `t.Cleanup`. Skip `t.Parallel()` on any test that touches flag state.

```go
import "github.com/open-feature/go-sdk/openfeature/memprovider"

func setFlags(t *testing.T, flags map[string]memprovider.InMemoryFlag) {
    t.Helper()
    if err := openfeature.SetProvider(memprovider.NewInMemoryProvider(flags)); err != nil {
        t.Fatalf("failed to set in-memory provider: %v", err)
    }
    t.Cleanup(func() {
        _ = openfeature.SetProvider(openfeature.NoopProvider{})
    })
}

func TestHandle_FlagEnabled(t *testing.T) { // no t.Parallel()
    setFlags(t, map[string]memprovider.InMemoryFlag{
        "cmk_my_flag": {State: memprovider.Enabled, DefaultVariant: "on",
            Variants: map[string]interface{}{"on": true, "off": false}},
    })
    // assert behaviour when flag is on
}
```

`InMemoryProvider` never touches `ffclient`, so this is safe as long as flag-touching tests are not parallelised.

### Option T2 — Interface injection (parallel-safe)

Define a minimal interface over the flag calls your component needs and inject it. Tests provide a plain stub with no global state.

```go
type FlagClient interface {
    BooleanValue(ctx context.Context, flag string, defaultValue bool, evalCtx openfeature.EvaluationContext) (bool, error)
}

// production wiring
handler := NewMyHandler(openfeature.NewClient("cmk"))

// test stub
type stubFlags struct{ values map[string]bool }
func (s *stubFlags) BooleanValue(_ context.Context, flag string, def bool, _ openfeature.EvaluationContext) (bool, error) {
    if v, ok := s.values[flag]; ok { return v, nil }
    return def, nil
}

func TestHandle_FlagEnabled(t *testing.T) {
    t.Parallel()
    h := NewMyHandler(&stubFlags{values: map[string]bool{"cmk_my_flag": true}})
    // assert behaviour when flag is on
}
```

This is the only parallel-safe approach — the Go SDK has no isolated-context constructor, so T1 cannot be made parallel-safe.

Always test both the flag-on and flag-off paths. The default value passed to `BooleanValue` is a provider-error fallback, not a substitute for an explicit test case.
