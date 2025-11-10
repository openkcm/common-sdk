package logger_test

import (
	"bytes"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/openkcm/common-sdk/v2/pkg/commoncfg"
	"github.com/openkcm/common-sdk/v2/pkg/logger"
)

// TestGDORMiddleware tests gdprMiddleware
func TestGDPRMiddleware(t *testing.T) {
	loggerCfg := &commoncfg.Logger{
		Formatter: commoncfg.LoggerFormatter{
			Time: commoncfg.LoggerTime{
				Pattern:   "2006-01-02T15:04:05Z07:00",
				Precision: "ms",
			},
			Fields: commoncfg.LoggerFields{
				Time:    "time",
				Level:   "level",
				Message: "msg",
			},
		},
	}

	t.Run("masking works for ", func(t *testing.T) {
		t.Run("group fields that are masked", func(t *testing.T) {
			var buf bytes.Buffer

			loggerCfg.Formatter.Fields.Masking = commoncfg.LoggerFieldsMasking{
				PII:   []string{"email"},
				Other: map[string]string{"credit_card": "****"},
			}

			mw := logger.NewGDPRMiddleware(loggerCfg)
			handler := mw(slog.NewJSONHandler(&buf, nil))
			slogLogger := slog.New(handler)

			slogLogger.WithGroup("user").Info("login",
				"email", "john@example.com",
				"credit_card", "1111-2222-3333-4444",
				"unmasked", "ok",
			)

			output := buf.String()
			t.Logf("Log output: %s", output)

			assert.Contains(t, output, `"user"`)
			assert.Contains(t, output, `"email":"john*******"`)
			assert.Contains(t, output, `"credit_card":"****"`)
			assert.Contains(t, output, `"unmasked":"ok"`)
		})

		t.Run("PII mask field", func(t *testing.T) {
			var buf bytes.Buffer

			loggerCfg.Formatter.Fields.Masking = commoncfg.LoggerFieldsMasking{
				PII:   []string{"masked"},
				Other: nil,
			}
			mw := logger.NewGDPRMiddleware(loggerCfg)
			handler := mw(slog.NewJSONHandler(&buf, nil))
			slogLogger := slog.New(handler)

			slogLogger.Info("test", "masked", "example_text")

			output := buf.String()
			t.Logf("Log output: %s", output)

			assert.Contains(t, output, `"masked":"exam*******"`)
			assert.NotContains(t, output, "example_text")
		})

		t.Run("other mask field", func(t *testing.T) {
			var buf bytes.Buffer

			loggerCfg.Formatter.Fields.Masking = commoncfg.LoggerFieldsMasking{
				PII:   nil,
				Other: map[string]string{"masked": "*custom_mask*"},
			}
			mw := logger.NewGDPRMiddleware(loggerCfg)
			handler := mw(slog.NewJSONHandler(&buf, nil))
			slogLogger := slog.New(handler)

			slogLogger.Info("test", "masked", "example_text")

			output := buf.String()
			t.Logf("Log output: %s", output)

			assert.Contains(t, output, `"masked":"*custom_mask*"`)
			assert.NotContains(t, output, "example_text")
		})

		t.Run("other not masked fields", func(t *testing.T) {
			var buf bytes.Buffer

			loggerCfg.Formatter.Fields.Masking = commoncfg.LoggerFieldsMasking{
				PII:   []string{"pii_masked"},
				Other: map[string]string{"other_masked": "*custom_mask*"},
			}
			mw := logger.NewGDPRMiddleware(loggerCfg)
			handler := mw(slog.NewJSONHandler(&buf, nil))
			slogLogger := slog.New(handler)

			slogLogger.Info("test", "not_masked", "example_text")

			output := buf.String()
			t.Logf("Log output: %s", output)

			assert.NotContains(t, output, "*******")
			assert.NotContains(t, output, "*custom_mask*")
			assert.Contains(t, output, `"not_masked":"example_text"`)
		})
	})
}
