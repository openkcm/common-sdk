// Package featureflags provides an embedded OpenFeature provider backed by
// GO Feature Flag (GOFF). Flags are read from a local YAML file and evaluated
// entirely in-process — no relay proxy required.
package featureflags

import (
	"context"
	"errors"
	"fmt"
	"time"

	ffclient "github.com/thomaspoignant/go-feature-flag"
	"github.com/thomaspoignant/go-feature-flag/ffcontext"
	"github.com/thomaspoignant/go-feature-flag/modules/core/flag"
	"github.com/thomaspoignant/go-feature-flag/modules/core/model"
	"github.com/thomaspoignant/go-feature-flag/retriever/fileretriever"

	"github.com/open-feature/go-sdk/openfeature"

	"github.com/openkcm/common-sdk/pkg/commoncfg"
)

// ErrEmptyFilePath is returned by NewEmbeddedProvider when FilePath is empty.
var ErrEmptyFilePath = errors.New("featureflags: FilePath must not be empty")

// EmbeddedProvider implements openfeature.FeatureProvider by delegating to ffclient.
type EmbeddedProvider struct {
	flagFilePath    string
	pollingInterval time.Duration
	initialized     bool
}

// NewEmbeddedProvider creates an embedded provider from the given config.
// Returns ErrEmptyFilePath if cfg.FilePath is empty.
func NewEmbeddedProvider(cfg commoncfg.FeatureFlags) (openfeature.FeatureProvider, error) {
	if cfg.FilePath == "" {
		return nil, ErrEmptyFilePath
	}
	return &EmbeddedProvider{
		flagFilePath:    cfg.FilePath,
		pollingInterval: cfg.PollingInterval,
	}, nil
}

// Metadata implements openfeature.FeatureProvider.
func (p *EmbeddedProvider) Metadata() openfeature.Metadata {
	return openfeature.Metadata{Name: "GO Feature Flag Embedded Provider"}
}

// Hooks implements openfeature.FeatureProvider.
func (p *EmbeddedProvider) Hooks() []openfeature.Hook {
	return nil
}

// Init implements openfeature.StateHandler — called by SetProviderAndWait.
func (p *EmbeddedProvider) Init(_ openfeature.EvaluationContext) error {
	if err := ffclient.Init(ffclient.Config{
		PollingInterval: p.pollingInterval,
		Retriever:       &fileretriever.Retriever{Path: p.flagFilePath},
	}); err != nil {
		return err
	}
	p.initialized = true
	return nil
}

// Shutdown implements openfeature.StateHandler.
func (p *EmbeddedProvider) Shutdown() {
	if p.initialized {
		ffclient.Close()
		p.initialized = false
	}
}

// Status implements openfeature.StateHandler.
func (p *EmbeddedProvider) Status() openfeature.State {
	return openfeature.ReadyState
}

// BooleanEvaluation implements openfeature.FeatureProvider.
func (p *EmbeddedProvider) BooleanEvaluation(_ context.Context, flag string, defaultValue bool, flatCtx openfeature.FlattenedContext) openfeature.BoolResolutionDetail {
	user := toEvalContext(flatCtx)
	res, err := ffclient.BoolVariationDetails(flag, user, defaultValue)
	return openfeature.BoolResolutionDetail{
		Value:                    res.Value,
		ProviderResolutionDetail: toResolutionDetail(res, err),
	}
}

// StringEvaluation implements openfeature.FeatureProvider.
func (p *EmbeddedProvider) StringEvaluation(_ context.Context, flag string, defaultValue string, flatCtx openfeature.FlattenedContext) openfeature.StringResolutionDetail {
	user := toEvalContext(flatCtx)
	res, err := ffclient.StringVariationDetails(flag, user, defaultValue)
	return openfeature.StringResolutionDetail{
		Value:                    res.Value,
		ProviderResolutionDetail: toResolutionDetail(res, err),
	}
}

// IntEvaluation implements openfeature.FeatureProvider.
// GOFF's IntVariationDetails uses int (not int64), which is 64-bit on all
// supported platforms (linux/amd64, linux/arm64), so the cast is lossless.
func (p *EmbeddedProvider) IntEvaluation(_ context.Context, flag string, defaultValue int64, flatCtx openfeature.FlattenedContext) openfeature.IntResolutionDetail {
	user := toEvalContext(flatCtx)
	res, err := ffclient.IntVariationDetails(flag, user, int(defaultValue))
	return openfeature.IntResolutionDetail{
		Value:                    int64(res.Value),
		ProviderResolutionDetail: toResolutionDetail(res, err),
	}
}

// FloatEvaluation implements openfeature.FeatureProvider.
func (p *EmbeddedProvider) FloatEvaluation(_ context.Context, flag string, defaultValue float64, flatCtx openfeature.FlattenedContext) openfeature.FloatResolutionDetail {
	user := toEvalContext(flatCtx)
	res, err := ffclient.Float64VariationDetails(flag, user, defaultValue)
	return openfeature.FloatResolutionDetail{
		Value:                    res.Value,
		ProviderResolutionDetail: toResolutionDetail(res, err),
	}
}

// ObjectEvaluation implements openfeature.FeatureProvider.
func (p *EmbeddedProvider) ObjectEvaluation(_ context.Context, flag string, defaultValue interface{}, flatCtx openfeature.FlattenedContext) openfeature.InterfaceResolutionDetail {
	user := toEvalContext(flatCtx)
	defMap, ok := defaultValue.(map[string]any)
	if !ok {
		return openfeature.InterfaceResolutionDetail{
			Value: defaultValue,
			ProviderResolutionDetail: openfeature.ProviderResolutionDetail{
				ResolutionError: openfeature.NewTypeMismatchResolutionError("defaultValue must be map[string]any for object flags"),
				Reason:          openfeature.ErrorReason,
			},
		}
	}
	res, err := ffclient.JSONVariationDetails(flag, user, defMap)
	return openfeature.InterfaceResolutionDetail{
		Value:                    res.Value,
		ProviderResolutionDetail: toResolutionDetail(res, err),
	}
}

// toEvalContext converts an OpenFeature FlattenedContext into an ffcontext.Context.
func toEvalContext(flatCtx openfeature.FlattenedContext) ffcontext.Context {
	var key string
	if tk, ok := flatCtx["targetingKey"]; ok {
		if s, isStr := tk.(string); isStr {
			key = s
		}
	}
	b := ffcontext.NewEvaluationContextBuilder(key)
	for k, v := range flatCtx {
		if k == "targetingKey" {
			continue
		}
		b.AddCustom(k, v)
	}
	return b.Build()
}

// toResolutionDetail converts an ffclient VariationResult into an OpenFeature ProviderResolutionDetail.
func toResolutionDetail[T model.JSONType](res model.VariationResult[T], err error) openfeature.ProviderResolutionDetail {
	prd := openfeature.ProviderResolutionDetail{
		Variant: res.VariationType,
		Reason:  openfeature.Reason(res.Reason),
	}
	if res.Failed || err != nil {
		var resErr openfeature.ResolutionError
		if err != nil {
			resErr = openfeature.NewGeneralResolutionError(err.Error())
		} else {
			resErr = goffErrorToResolutionError(res.ErrorCode, res.ErrorDetails)
		}
		prd.ResolutionError = resErr
		prd.Reason = openfeature.ErrorReason
	}
	return prd
}

// goffErrorToResolutionError maps a GOFF error code to the matching OpenFeature resolution error.
func goffErrorToResolutionError(errorCode, errorDetails string) openfeature.ResolutionError {
	msg := errorDetails
	switch errorCode {
	case flag.ErrorCodeFlagNotFound:
		return openfeature.NewFlagNotFoundResolutionError(msg)
	case flag.ErrorCodeTypeMismatch:
		return openfeature.NewTypeMismatchResolutionError(msg)
	case flag.ErrorCodeProviderNotReady:
		return openfeature.NewProviderNotReadyResolutionError(msg)
	case flag.ErrorCodeParseError:
		return openfeature.NewParseErrorResolutionError(msg)
	case flag.ErrorCodeTargetingKeyMissing:
		return openfeature.NewTargetingKeyMissingResolutionError(msg)
	case flag.ErrorCodeInvalidContext:
		return openfeature.NewInvalidContextResolutionError(msg)
	default:
		return openfeature.NewGeneralResolutionError(fmt.Sprintf("%s: %s", errorCode, msg))
	}
}
