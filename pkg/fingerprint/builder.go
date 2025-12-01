package fingerprint

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"log/slog"
	"net/http"

	envoyauth "github.com/envoyproxy/go-control-plane/envoy/service/auth/v3"
)

var defaultHeaderKeys = []string{"user-agent"}

type BuilderOption func(*Builder)

type Builder struct {
	headerKeys []string
}

func WithHeaderKeys(keys []string) BuilderOption {
	return func(b *Builder) {
		b.headerKeys = keys
	}
}

func NewBuilder(opts ...BuilderOption) *Builder {
	b := &Builder{
		headerKeys: defaultHeaderKeys,
	}

	for _, opt := range opts {
		if opt != nil {
			opt(b)
		}
	}

	return b
}

func (b *Builder) FromHTTPRequest(r *http.Request) (string, error) {
	if r == nil {
		return "", errors.New("http request is nil")
	}

	headerMap := make(map[string]string, len(b.headerKeys))
	for _, key := range b.headerKeys {
		val := r.Header.Get(key)
		headerMap[key] = val
	}

	return b.fromHeaders(headerMap)
}

func (b *Builder) FromEnvoyHTTPRequest(r *envoyauth.AttributeContext_HttpRequest) (string, error) {
	if r == nil {
		return "", errors.New("envoy http request is nil")
	}

	headerMap := make(map[string]string, len(b.headerKeys))
	for _, key := range b.headerKeys {
		var val string
		if v, ok := r.GetHeaders()[key]; ok {
			val = v
		}

		headerMap[key] = val
	}

	return b.fromHeaders(headerMap)
}

func (b *Builder) fromHeaders(headerMap map[string]string) (string, error) {
	h := sha256.New()
	logVals := make([]slog.Attr, 0, len(headerMap)+1)

	for key, val := range headerMap {
		h.Write([]byte(val))
		logVals = append(logVals, slog.Attr{
			Key:   key,
			Value: slog.StringValue(val),
		})
	}

	fp := hex.EncodeToString(h.Sum(nil))
	logVals = append(logVals, slog.Attr{
		Key:   "fingerprint",
		Value: slog.StringValue(fp),
	})
	slog.LogAttrs(context.Background(), slog.LevelDebug, "Created fingerprint", logVals...)

	return fp, nil
}
