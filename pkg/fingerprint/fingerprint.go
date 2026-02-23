// Package fingerprint provides utilities to generate request fingerprints
// used by the OpenKCM Session Manager and ExtAuthZ components.
package fingerprint

import (
	"context"
	"errors"
)

type ctxKey struct{}

// WithFingerprint injects a fingerprint value into the context.
func WithFingerprint(ctx context.Context, fp string) context.Context {
	return context.WithValue(ctx, ctxKey{}, fp)
}

// ExtractFingerprint extrancts a fingerprint value from the context
func ExtractFingerprint(ctx context.Context) (string, error) {
	fp, ok := ctx.Value(ctxKey{}).(string)
	if !ok {
		return "", errors.New("no fingerprint in ctx")
	}

	return fp, nil
}
