package logger_test

import (
	"bytes"
	"context"
	"log/slog"
	"regexp"
	"strings"
	"testing"

	"github.com/openkcm/common-sdk/internal/testutils"
	"github.com/openkcm/common-sdk/pkg/commoncfg"
	"github.com/openkcm/common-sdk/pkg/logger"
)

func TestInitAsDefaultWithWriter(t *testing.T) {
	loggerConfigMutator := testutils.NewMutator(func() commoncfg.Logger {
		return commoncfg.Logger{
			Source: true,
			Format: commoncfg.TextLoggerFormat,
			Level:  "info",
			Formatter: commoncfg.LoggerFormatter{
				Time: commoncfg.LoggerTime{
					Type:    commoncfg.PatternTimeLogger,
					Pattern: "2006-01-02 15:04:05",
				},
				Fields: commoncfg.LoggerFields{
					Time:    "timestamp",
					Level:   "severity",
					Message: "message",
					Error:   "error",
					OTel: commoncfg.LoggerOTel{
						TraceID: "trace_id",
						SpanID:  "span_id",
					},
					Masking: commoncfg.LoggerFieldsMasking{
						PII:   []string{"password", "secret"},
						Other: map[string]string{"token": "masked_token"},
					},
				},
			},
		}
	})

	appConfigMutator := testutils.NewMutator(func() commoncfg.Application {
		return commoncfg.Application{
			Name:        "ExampleApp",
			Environment: "development",
			Labels:      map[string]string{"env": "dev", "region": "us-west"},
		}
	})

	// create the test cases
	tests := []struct {
		name         string
		appConfig    commoncfg.Application
		loggerConfig commoncfg.Logger
		wantError    bool
		test         func()
		check        func(string) bool
	}{
		{
			name:         "defaults",
			appConfig:    appConfigMutator(),
			loggerConfig: loggerConfigMutator(),
		}, {
			name:      "invalid loglevel, default to info",
			appConfig: appConfigMutator(),
			loggerConfig: loggerConfigMutator(func(cfg *commoncfg.Logger) {
				cfg.Level = "adbsfgbd"
			}),
			test: func() {
				slog.Log(context.Background(), logger.LevelTrace, "=trace=")
				slog.Debug("=debug=")
				slog.Info("=info=")
				slog.Warn("=warn=")
				slog.Error("=error=")
			},
			check: func(got string) bool {
				return !strings.Contains(got, "=trace=") &&
					!strings.Contains(got, "=debug=") &&
					strings.Contains(got, "=info=") &&
					strings.Contains(got, "=warn=") &&
					strings.Contains(got, "=error=")
			},
		}, {
			name:      "loglevel info",
			appConfig: appConfigMutator(),
			loggerConfig: loggerConfigMutator(func(cfg *commoncfg.Logger) {
				cfg.Level = "info"
			}),
			test: func() {
				slog.Log(context.Background(), logger.LevelTrace, "=trace=")
				slog.Debug("=debug=")
				slog.Info("=info=")
				slog.Warn("=warn=")
				slog.Error("=error=")
			},
			check: func(got string) bool {
				return !strings.Contains(got, "=trace=") &&
					!strings.Contains(got, "=debug=") &&
					strings.Contains(got, "=info=") &&
					strings.Contains(got, "=warn=") &&
					strings.Contains(got, "=error=")
			},
		}, {
			name:      "loglevel warn",
			appConfig: appConfigMutator(),
			loggerConfig: loggerConfigMutator(func(cfg *commoncfg.Logger) {
				cfg.Level = "warn"
			}),
			test: func() {
				slog.Log(context.Background(), logger.LevelTrace, "=trace=")
				slog.Debug("=debug=")
				slog.Info("=info=")
				slog.Warn("=warn=")
				slog.Error("=error=")
			},
			check: func(got string) bool {
				return !strings.Contains(got, "=trace=") &&
					!strings.Contains(got, "=debug=") &&
					!strings.Contains(got, "=info=") &&
					strings.Contains(got, "=warn=") &&
					strings.Contains(got, "=error=")
			},
		}, {
			name:      "loglevel error",
			appConfig: appConfigMutator(),
			loggerConfig: loggerConfigMutator(func(cfg *commoncfg.Logger) {
				cfg.Level = "error"
			}),
			test: func() {
				slog.Log(context.Background(), logger.LevelTrace, "=trace=")
				slog.Debug("=debug=")
				slog.Info("=info=")
				slog.Warn("=warn=")
				slog.Error("=error=")
			},
			check: func(got string) bool {
				return !strings.Contains(got, "=trace=") &&
					!strings.Contains(got, "=debug=") &&
					!strings.Contains(got, "=info=") &&
					!strings.Contains(got, "=warn=") &&
					strings.Contains(got, "=error=")
			},
		}, {
			name:      "loglevel debug",
			appConfig: appConfigMutator(),
			loggerConfig: loggerConfigMutator(func(cfg *commoncfg.Logger) {
				cfg.Level = "debug"
			}),
			test: func() {
				slog.Log(context.Background(), logger.LevelTrace, "=trace=")
				slog.Debug("=debug=")
				slog.Info("=info=")
				slog.Warn("=warn=")
				slog.Error("=error=")
			},
			check: func(got string) bool {
				return !strings.Contains(got, "=trace=") &&
					strings.Contains(got, "=debug=") &&
					strings.Contains(got, "=info=") &&
					strings.Contains(got, "=warn=") &&
					strings.Contains(got, "=error=")
			},
		}, {
			name:      "loglevel trace",
			appConfig: appConfigMutator(),
			loggerConfig: loggerConfigMutator(func(cfg *commoncfg.Logger) {
				cfg.Level = "trace"
			}),
			test: func() {
				slog.Log(context.Background(), logger.LevelTrace, "=trace=")
				slog.Debug("=debug=")
				slog.Info("=info=")
				slog.Warn("=warn=")
				slog.Error("=error=")
			},
			check: func(got string) bool {
				return strings.Contains(got, "=trace=") &&
					strings.Contains(got, "=debug=") &&
					strings.Contains(got, "=info=") &&
					strings.Contains(got, "=warn=") &&
					strings.Contains(got, "=error=")
			},
		}, {
			name:      "with Unix time logger and 0 precision",
			appConfig: appConfigMutator(),
			loggerConfig: loggerConfigMutator(func(cfg *commoncfg.Logger) {
				cfg.Formatter.Time.Type = commoncfg.UnixTimeLogger
				cfg.Formatter.Time.Precision = "0"
			}),
			test: func() {
				slog.Info("=info=")
			},
			check: func(got string) bool {
				matched, _ := regexp.MatchString(`timestamp=[0-9]+ `, got)
				return matched
			},
		}, {
			name:      "with Unix time logger and 1ms precision",
			appConfig: appConfigMutator(),
			loggerConfig: loggerConfigMutator(func(cfg *commoncfg.Logger) {
				cfg.Formatter.Time.Type = commoncfg.UnixTimeLogger
				cfg.Formatter.Time.Precision = "1ms"
			}),
			test: func() {
				slog.Info("=info=")
			},
			check: func(got string) bool {
				matched, _ := regexp.MatchString(`timestamp=[0-9]+ `, got)
				return matched
			},
		}, {
			name:      "with Unix time logger but invalid precision",
			appConfig: appConfigMutator(),
			loggerConfig: loggerConfigMutator(func(cfg *commoncfg.Logger) {
				cfg.Formatter.Time.Type = commoncfg.UnixTimeLogger
				cfg.Formatter.Time.Precision = "adfb"
			}),
			wantError: true,
		}, {
			name:      "with JSON formatter",
			appConfig: appConfigMutator(),
			loggerConfig: loggerConfigMutator(func(cfg *commoncfg.Logger) {
				cfg.Format = commoncfg.JSONLoggerFormat
			}),
			test: func() {
				slog.Info("=info=")
			},
			check: func(got string) bool {
				return strings.Contains(got, `"message":"=info="`)
			},
		}, {
			name:      "empty time field",
			appConfig: appConfigMutator(),
			loggerConfig: loggerConfigMutator(func(cfg *commoncfg.Logger) {
				cfg.Formatter.Fields.Time = ""
			}),
			test: func() {
				slog.Info("=info=")
			},
			check: func(got string) bool {
				return strings.Contains(got, `time=`)
			},
		}, {
			name:      "empty level field",
			appConfig: appConfigMutator(),
			loggerConfig: loggerConfigMutator(func(cfg *commoncfg.Logger) {
				cfg.Formatter.Fields.Level = ""
			}),
			test: func() {
				slog.Info("=info=")
			},
			check: func(got string) bool {
				return strings.Contains(got, `level=`)
			},
		}, {
			name:      "empty message field",
			appConfig: appConfigMutator(),
			loggerConfig: loggerConfigMutator(func(cfg *commoncfg.Logger) {
				cfg.Formatter.Fields.Message = ""
			}),
			test: func() {
				slog.Info("=info=")
			},
			check: func(got string) bool {
				return strings.Contains(got, `msg=`)
			},
		},
	}

	// run the tests
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Arrange
			var buf bytes.Buffer

			// Act
			err := logger.InitAsDefaultWithWriter(&buf, tc.loggerConfig, tc.appConfig)

			// Assert
			if !wantErrorMatches(tc.wantError, err) {
				t.Errorf("error expectation mismatch. wantError: %v, got: %v", tc.wantError, err)
			}

			if tc.test != nil && tc.check != nil {
				tc.test()

				got := buf.String()
				if !tc.check(got) {
					t.Errorf("output does not match expectation: %s", got)
				}
			}
		})
	}
}

func wantErrorMatches(wantError bool, err error) bool {
	if wantError && err == nil {
		return false
	}

	if !wantError && err != nil {
		return false
	}

	return true
}
