# Logging SDK Guide

This document explains how to use the common-sdk logging. The SDK is built on top of Go’s `log/slog` and extends it with formatting, attribute customization, masking (PII & custom), OpenTelemetry support, and GDPR compliance.

---

## Getting Started

### 1. Initialize the Logger

Use `InitAsDefault` to initialize and set the logger globally for your service:

```go
import "github.com/openkcm/common-sdk/v2/pkg/logger"

config := &commoncfg.BaseConfig{}
err := commoncfg.LoadConfig(config, defaults, "./your/config/path")
if err != nil {
    log.Fatalf("failed to load config: %v", err)
}

err := logger.InitAsDefault(config.Logger, config.Application)
if err != nil {
	log.Fatalf("failed to initialize logger: %v", err)
}
```

You can also use a custom writer (e.g., for testing or file output):

```go
err := logger.InitAsDefaultWithWriter(myWriter, config.Logger, config.Application)
```

---

##  Configuration Overview

The logger is configured via the `commoncfg.Logger` struct. Here are the key configuration fields:

### Log Format
```yaml
format: json        # or "text"
```

### Log Level
```yaml
level: info         # debug, info, warn, error
```

### Time Format
```yaml
formatter:
  time:
    type: unix      # or "pattern"
    precision: 1ms
    pattern: "2006-01-02T15:04:05Z07:00" # if using "pattern"
```

### Attribute Mapping
Customize log field names:
```yaml
formatter:
  fields:
    time: timestamp
    level: severity
    message: msg
    error: err
```

### PII & Field Masking
```yaml
formatter:
  fields:
    masking:
      pii: ["email", "phone"]
      other:
        secretKey: "[REDACTED]"
```

---

## GDPR Middleware

To comply with GDPR and protect sensitive data, our SDK includes masking capabilities for personally identifiable information (PII) and other fields defined in your configuration.

The GDPR middleware is automatically applied only when OpenTelemetry logging is enabled.

For manual integration:

```go
handler := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{})
gdpr := logger.NewGDPRMiddleware(&config.Logger)
wrapped := gdpr(handler)
```

This middleware ensures all specified PII and custom fields are masked in both structured and text logs.

---

## Telemetry Integration (OTLP)

The SDK integrates with OpenTelemetry (OTEL) to forward logs to a collector (GRPC/HTTP).

To enable OTLP logging:
```yaml
telemetry:
  logs:
    enabled: true
    protocol: grpc
    host:
      source: env
      env: OTEL_EXPORTER_HOST
    secretRef:
      type: api-token
      apiToken:
        source: env
        env: OTEL_API_TOKEN
```

The logs are automatically enriched with:
- `trace_id` and `span_id` from OpenTelemetry context
- `service`, `name`, `environment`, and `labels` from application config

---

##  Adding Custom Attributes

Use `slogctx` to include contextual attributes:

```go
ctx := slogctx.With(ctx,
    slog.String("request_id", reqID),
    slog.String("user_id", userID),
)
slogctx.Info(ctx, "user authenticated")
```

You can also inject attributes globally using the `Application.Labels` map.

---

## Example Output

### JSON format:
```json
{
  "timestamp": "2025-04-10T09:55:52.384+02:00",
  "severity": "INFO",
  "msg": "user authenticated",
  "service": {
    "name": "user-api",
    "environment": "production"
  },
  "labels": {
    "version": "1.2.3"
  },
  "request_id": "abc-123",
  "user_id": "john*******"
}
```

### Text format:
```
time=2025-04-10T09:55:52.384+02:00 level=INFO msg="user authenticated" request_id=abc-123 user_id=john*******
```

---

## Helper: `CreateAttributes`

For dynamically adding labels:

```go
attrs := logger.CreateAttributes(map[string]string{
    "region": "eu",
    "tenant": "abc",
})
slog.Info("tenant initialized", attrs...)
```

---

## Best Practices

- Always mask PII using config.
- Use context-aware logging via `slogctx`.
- Keep logs structured and consistent using the formatter config.
- Attach OpenTelemetry context for distributed tracing.

---

## Related Packages

- [`slog`](https://pkg.go.dev/log/slog): Structured logging.
- [`slog-multi`](https://github.com/samber/slog-multi): Attaching multiple slog.Handlers.
- [`slog-formatter`](https://github.com/samber/slog-formatter): Log attribute formatting.
- [`slog-context`](https://github.com/veqryn/slog-context): Logging with context.
- [`OpenTelemetry`](https://opentelemetry.io): Telemetry framework for metrics, traces, and logs.
