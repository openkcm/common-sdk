package logger

import (
	"io"
	"log/slog"
	"os"
	"strings"
	"time"

	slogformatter "github.com/samber/slog-formatter"
	slogmulti "github.com/samber/slog-multi"
	slogctx "github.com/veqryn/slog-context"
	slogotel "github.com/veqryn/slog-context/otel"

	"github.com/openkcm/common-sdk/pkg/commoncfg"
)

var (
	logLevel = new(slog.LevelVar)
)

const (
	TimeAttribute    = "time"
	LevelAttribute   = "level"
	MessageAttribute = "msg"
	// LevelTrace is a custom log level for trace logging with value -8
	LevelTrace = slog.Level(-8)
)

// setLogLevel converts the level string used in the config to a slog.LevelVar
// and sets the levelVar to the corresponding level.
func setLogLevel(levelVar *slog.LevelVar, level string) {
	switch strings.ToLower(level) {
	case "trace":
		levelVar.Set(LevelTrace)
	case "debug":
		levelVar.Set(slog.LevelDebug)
	case "info":
		levelVar.Set(slog.LevelInfo)
	case "warn":
		levelVar.Set(slog.LevelWarn)
	case "error":
		levelVar.Set(slog.LevelError)
	default:
		levelVar.Set(slog.LevelInfo)
	}
}

// InitAsDefault sets default logger according to configuration
func InitAsDefault(cfgLogger commoncfg.Logger, app commoncfg.Application) error {
	return InitAsDefaultWithWriter(os.Stdout, cfgLogger, app)
}

// InitAsDefaultWithWriter initializes the default logger using the provided writer, logger config, and application info.
func InitAsDefaultWithWriter(w io.Writer, cfgLogger commoncfg.Logger, app commoncfg.Application) error {
	handler, err := InitHandlerWithWriter(w, cfgLogger, app)
	if err != nil {
		return err
	}

	logger := slog.New(handler)
	slog.SetDefault(logger)

	return nil
}

// InitHandler initializes the default handler
func InitHandler(cfgLogger commoncfg.Logger, app commoncfg.Application) (slog.Handler, error) {
	return InitHandlerWithWriter(os.Stdout, cfgLogger, app)
}

// InitHandlerWithWriter initializes the default Handler with provided writer, logger config and application info.
func InitHandlerWithWriter(w io.Writer, cfgLogger commoncfg.Logger, app commoncfg.Application) (slog.Handler, error) {
	setLogLevel(logLevel, cfgLogger.Level)

	formatters := []slogformatter.Formatter{}

	switch cfgLogger.Formatter.Time.Type {
	case commoncfg.UnixTimeLogger:
		precision, err := time.ParseDuration(cfgLogger.Formatter.Time.Precision)
		if err != nil {
			return nil, err
		}

		if precision == 0 {
			precision = time.Microsecond
		}

		formatters = append(formatters, slogformatter.UnixTimestampFormatter(precision))
	case commoncfg.PatternTimeLogger:
		formatters = append(formatters, slogformatter.TimeFormatter(cfgLogger.Formatter.Time.Pattern, time.UTC))
	}

	replaceAttr := func(groups []string, a slog.Attr) slog.Attr {
		switch a.Key {
		case TimeAttribute:
			v, _ := formatters[0](groups, a)

			timeKey := strings.TrimSpace(cfgLogger.Formatter.Fields.Time)
			if timeKey == "" {
				timeKey = a.Key
			}

			return slog.Attr{Key: timeKey, Value: v}
		case LevelAttribute:
			levelKey := strings.TrimSpace(cfgLogger.Formatter.Fields.Level)
			if levelKey == "" {
				levelKey = a.Key
			}

			// Handle custom LevelTrace to display as "TRACE" instead of "DEBUG-4"
			if level, ok := a.Value.Any().(slog.Level); ok && level == LevelTrace {
				return slog.Attr{Key: levelKey, Value: slog.StringValue("TRACE")}
			}

			return slog.Attr{Key: levelKey, Value: a.Value}
		case MessageAttribute:
			msgKey := strings.TrimSpace(cfgLogger.Formatter.Fields.Message)
			if msgKey == "" {
				msgKey = a.Key
			}

			return slog.Attr{Key: msgKey, Value: a.Value}
		default:
			return a
		}
	}

	attrs := []slog.Attr{
		slog.String(commoncfg.AttrServiceName, app.Name),
		slog.String(commoncfg.AttrEnvironment, app.Environment),
	}

	labels := CreateAttributes(app.Labels)
	if len(labels) > 0 {
		attrs = append(attrs, slog.Group(commoncfg.AttrLabels, labels...))
	}

	var handler slog.Handler

	switch cfgLogger.Format {
	case commoncfg.TextLoggerFormat:
		handler = slog.NewTextHandler(w, &slog.HandlerOptions{
			AddSource:   cfgLogger.Source,
			Level:       logLevel,
			ReplaceAttr: replaceAttr,
		},
		).WithAttrs(attrs)
	case commoncfg.JSONLoggerFormat:
		handler = slog.NewJSONHandler(w, &slog.HandlerOptions{
			Level:       logLevel,
			AddSource:   cfgLogger.Source,
			ReplaceAttr: replaceAttr,
		}).WithAttrs(attrs)
	}

	for _, pii := range cfgLogger.Formatter.Fields.Masking.PII {
		formatters = append(formatters, slogformatter.PIIFormatter(pii))
	}

	for key, value := range cfgLogger.Formatter.Fields.Masking.Other {
		formatters = append(formatters, slogformatter.FormatByKey(key, func(_ slog.Value) slog.Value {
			return slog.StringValue(value)
		}))
	}

	errorField := "error"
	if strings.TrimSpace(cfgLogger.Formatter.Fields.Error) != "" {
		errorField = strings.TrimSpace(cfgLogger.Formatter.Fields.Error)
	}

	formatters = append(formatters, slogformatter.ErrorFormatter(errorField))

	if strings.TrimSpace(cfgLogger.Formatter.Fields.OTel.TraceID) != "" {
		slogotel.DefaultKeyTraceID = strings.TrimSpace(cfgLogger.Formatter.Fields.OTel.TraceID)
	}

	if strings.TrimSpace(cfgLogger.Formatter.Fields.OTel.SpanID) != "" {
		slogotel.DefaultKeySpanID = strings.TrimSpace(cfgLogger.Formatter.Fields.OTel.SpanID)
	}

	return slogctx.NewHandler(
		slogmulti.
			Pipe(slogformatter.NewFormatterHandler(formatters...)).Handler(handler),
		&slogctx.HandlerOptions{
			// Prependers will first add the Open Telemetry Traces ID,
			// then anything else Prepended to the ctx
			Prependers: []slogctx.AttrExtractor{
				slogotel.ExtractTraceSpanID,
				slogctx.ExtractPrepended,
			},
			// Appenders stays as default (leaving as nil would accomplish the same)
			Appenders: []slogctx.AttrExtractor{
				slogctx.ExtractAppended,
			},
		},
	), nil
}

// CreateAttributes converts a map and additional slog attributes into a unified slice of attributes.
func CreateAttributes(m map[string]string, attrs ...slog.Attr) []any {
	attributes := make([]any, 0)
	for k, v := range m {
		attributes = append(attributes, slog.String(k, v))
	}

	for _, attr := range attrs {
		attributes = append(attributes, attr)
	}

	return attributes
}
