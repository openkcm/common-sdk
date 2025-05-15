package logger

import (
	"context"
	"log/slog"

	"github.com/openkcm/common-sdk/pkg/commoncfg"
)

type masking struct {
	pii  bool
	mask string
}

type Middleware func(slog.Handler) slog.Handler

// NewGDPRMiddleware creates a new gdprMiddleware with masking and renaming rules.
func NewGDPRMiddleware(logger *commoncfg.Logger) Middleware {
	replaceAttr := map[string]string{
		TimeAttribute:    logger.Formatter.Fields.Time,
		MessageAttribute: logger.Formatter.Fields.Message,
		LevelAttribute:   logger.Formatter.Fields.Level,
	}

	maskingFields := map[string]masking{}
	for _, pii := range logger.Formatter.Fields.Masking.PII {
		maskingFields[pii] = masking{
			pii:  true,
			mask: "*******",
		}
	}

	for key, value := range logger.Formatter.Fields.Masking.Other {
		maskingFields[key] = masking{
			pii:  false,
			mask: value,
		}
	}

	return func(next slog.Handler) slog.Handler {
		return &gdprMiddleware{
			next:          next,
			maskingFields: maskingFields,
			replaceAttr:   replaceAttr,
		}
	}
}

// gdprMiddleware is a slog.Handler that masks and renames sensitive log attributes.
type gdprMiddleware struct {
	next          slog.Handler
	maskingFields map[string]masking
	replaceAttr   map[string]string
}

// Enabled delegates the check to the wrapped handler.
func (h *gdprMiddleware) Enabled(ctx context.Context, level slog.Level) bool {
	return h.next.Enabled(ctx, level)
}

// Handle anonymizes attributes and passes the modified record to the wrapped handler.
func (h *gdprMiddleware) Handle(ctx context.Context, record slog.Record) error {
	var attrs []slog.Attr

	record.Attrs(func(attr slog.Attr) bool {
		attrs = append(attrs, h.anonymizeAndRename(attr))

		return true
	})

	// new record with anonymized data
	record = slog.NewRecord(record.Time, record.Level, record.Message, record.PC)
	record.AddAttrs(attrs...)

	return h.next.Handle(ctx, record)
}

// WithAttrs applies attributes after anonymizing and renaming them.
func (h *gdprMiddleware) WithAttrs(attrs []slog.Attr) slog.Handler {
	for i := range attrs {
		attrs[i] = h.anonymizeAndRename(attrs[i])
	}

	return &gdprMiddleware{
		next:          h.next.WithAttrs(attrs),
		maskingFields: h.maskingFields,
		replaceAttr:   h.replaceAttr,
	}
}

// WithGroup applies a group name to the wrapped handler.
func (h *gdprMiddleware) WithGroup(name string) slog.Handler {
	return &gdprMiddleware{
		next:          h.next.WithGroup(name),
		maskingFields: h.maskingFields,
		replaceAttr:   h.replaceAttr,
	}
}

// anonymizeAndRename masks and optionally renames sensitive attributes.
func (h *gdprMiddleware) anonymizeAndRename(attr slog.Attr) slog.Attr {
	k := attr.Key
	v := attr.Value
	kind := attr.Value.Kind()

	switch kind {
	case slog.KindGroup:
		attrs := v.Group()
		for i := range attrs {
			attrs[i] = h.anonymizeAndRename(attrs[i])
		}
		return slog.Group(k, toAnySlice(attrs)...)
	default:
		mask, ok := h.maskingFields[k]
		if ok {
			newKey, found := h.replaceAttr[k]
			if !found {
				newKey = k
			}

			if mask.pii {
				return slog.String(newKey, v.String()[0:4]+mask.mask)
			}
			return slog.String(newKey, mask.mask)
		}
		return attr
	}
}

func toAnySlice[T any](collection []T) []any {
	result := make([]any, len(collection))
	for i := range collection {
		result[i] = collection[i]
	}
	return result
}
