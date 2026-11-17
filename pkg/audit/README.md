# Audit Library

Go library for creating and sending audit log events to an audit log collector using proto-based events via HTTP or gRPC transports.

## Features

- **Proto-based events**: Uses typed proto messages from `api-sdk` with auto-generated builders
- **Async by default**: Non-blocking event queuing with background delivery
- **Dual transport**: Supports both HTTP and gRPC (OTLP) protocols
- **Automatic retries**: Exponential backoff for transient failures (both application-level and gRPC-level)
- **gRPC batching**: Efficient batching of events at the auditor level for gRPC transport
- **gRPC compression**: gzip compression enabled by default
- **Connection monitoring**: Optional callback for gRPC connection state changes
- **Simple API**: `auditor.Send(event)` enqueues events immediately
- **Configuration-driven**: Properties from config are auto-copied to every event

## Dependencies

- **Go 1.27** or higher
- Working audit log collector instance with an HTTP or gRPC receiver
- `api-sdk` proto definitions

## Quick Start

### 1. Configure

Add an `audit` section to your `config.yaml`:

**HTTP Transport (default):**

```yaml
audit:
  endpoint: "https://audit.example.com/v1/logs"
  httpClient:
    # Optional: mTLS, basic auth, OAuth2, etc.
  additionalProperties: |
    application_id: my-service
    environment: production
    region: us-east-1
```

**gRPC Transport:**

```yaml
audit:
  endpoint: "audit.example.com:4317"
  protocol: grpc
  grpcClient:
    secretRef:
      type: mtls  # or "insecure" for development
      mtls:
        certFile: /path/to/cert.pem
        keyFile: /path/to/key.pem
        caFile: /path/to/ca.pem
    attributes:
      keepaliveTime: 80s
      keepaliveTimeout: 40s
      retry:
        maxAttempts: 3
        initialBackoff: 100ms
        maxBackoff: 1s
        backoffMultiplier: 2
        retryableStatusCodes:
          - UNAVAILABLE
          - RESOURCE_EXHAUSTED
  additionalProperties: |
    application_id: my-service
    environment: production
```

The `additionalProperties` are automatically added to every event's `properties` map.

### 2. Create Auditor

```go
import (
    "github.com/openkcm/common-sdk/pkg/audit"
    "github.com/openkcm/common-sdk/pkg/commoncfg"
    auditlogeventsv1 "github.com/openkcm/api-sdk/proto/kms/api/cmk/auditlog/events/v1"
)

func main() {
    cfg := loadConfig() // Returns *commoncfg.Config

    auditor, err := audit.NewAuditor(&cfg.Audit,
        audit.WithQueueSize(2048),
        audit.WithMaxConcurrent(20),
        audit.WithOnError(func(err error) {
            log.Error("audit delivery failed", "error", err)
        }),
    )
    if err != nil {
        log.Fatal("failed to create auditor", "error", err)
    }
    defer auditor.Close(context.Background())

    // ... use auditor
}
```

### 3. Send Events

```go
// Inline composition: additionalProperties auto-copied, add per-event properties
err := auditor.Send(auditlogeventsv1.NewCMKCreatedEvent(
    auditor.NewCommon(
        audit.WithProperty("request_id", "req-123"),
        audit.WithProperty("user_id", "user-456"),
        audit.WithTraceContext(traceID, spanID),
    ),
    "tenant-456",
    "cmk-789",
))

if err != nil {
    // Only fails if queue is full or auditor is closed
    log.Error("failed to enqueue audit event", "error", err)
}
```

## API Reference

### Auditor Creation

```go
func NewAuditor(cfg *commoncfg.Audit, opts ...Option) (*Auditor, error)
```

Creates an auditor from config. Starts background workers immediately.

**General Options:**
- `WithQueueSize(size int)` - Sets queue capacity (default: 1024)
- `WithMaxConcurrent(n int)` - Sets max concurrent requests (default: 10)
- `WithMaxRetries(n int)` - Sets retry attempts (default: 5)
- `WithBaseDelay(d time.Duration)` - Sets initial backoff delay (default: 100ms)
- `WithMaxDelay(d time.Duration)` - Sets max backoff delay (default: 30s)
- `WithOnError(fn func(error))` - Error callback for failed deliveries

**gRPC-Specific Options:**
- `WithGRPCBatchSize(size int)` - Sets batch size before sending (default: 100)
- `WithGRPCBatchTimeout(d time.Duration)` - Sets max wait for partial batch (default: 200ms)
- `WithGRPCRequestTimeout(d time.Duration)` - Sets request timeout (default: 30s)
- `WithGRPCCompression(enabled bool)` - Enables/disables gzip compression (default: true)
- `WithGRPCConnectionStateCallback(cb ConnectionStateCallback)` - Sets connection state change callback

### Sending Events

```go
func (a *Auditor) Send(event auditlogeventsv1.AuditEvent) error
```

Enqueues an event for async delivery. Non-blocking.

**Returns:**
- `nil` - Event enqueued successfully
- `ErrQueueFull` - Queue is at capacity, event dropped
- `ErrClosed` - Auditor has been closed

### Creating Common Fields

```go
func (a *Auditor) NewCommon(opts ...CommonOption) *auditlogeventsv1.Common
```

Creates a `Common` struct with:
- `timestamp` set to current time
- `properties` copied from `additionalProperties` config
- Optional per-event additions via `CommonOption`

**CommonOptions:**
- `WithProperty(key, value string)` - Adds or overrides a property
- `WithTraceContext(traceID, spanID string)` - Sets OpenTelemetry trace context

### Graceful Shutdown

```go
func (a *Auditor) Close(ctx context.Context) error
```

Stops accepting new events, drains the queue (with retries), flushes any batched events, and waits for workers to finish. Idempotent.

If `ctx` is cancelled before draining completes, in-flight sends are aborted and `ctx.Err()` is returned.

## Available Event Types

See the [api-sdk proto definitions](https://github.com/openkcm/api-sdk/tree/main/proto/kms/api/cmk/auditlog/events/v1) for all available event types and their generated builders.

## Transport Comparison

| Feature | HTTP | gRPC |
|---------|------|------|
| Dispatch Model | Goroutine-per-event | Batch dispatcher |
| Batching | None (1 event per request) | Configurable (default: 100 events) |
| Retry | Per-event | Per-batch + gRPC service config |
| Compression | None | gzip (enabled by default) |
| Connection | HTTP/1.1 or HTTP/2 | HTTP/2 with persistent connection |
| Best For | Low-volume, simple setup | High-volume, efficiency-critical |

## gRPC Transport

### Batching

When using gRPC transport, events are batched for efficiency:
- Events accumulate until batch size is reached (default: 100)
- Partial batches are flushed on timeout (default: 200ms)
- Batches are flushed on `Close()`

This significantly reduces network overhead compared to sending one event per request.

**Note:** The batch is retried as a unit. If a batch fails after all retries, all events in that batch are reported to `OnError` and dropped.

### Retry Policy

gRPC transport uses a two-layer retry strategy:
1. **gRPC service config retry**: Handles transport-level retries for `UNAVAILABLE` and `RESOURCE_EXHAUSTED`
2. **Application-level retry**: Handles higher-level failures with exponential backoff on the entire batch

Configure via `grpcClient.attributes.retry` in config.

### Compression

gzip compression is enabled by default for gRPC transport. Disable with:

```go
audit.NewAuditor(&cfg.Audit, audit.WithGRPCCompression(false))
```

### Connection State Monitoring

Monitor gRPC connection state changes:

```go
auditor, _ := audit.NewAuditor(&cfg.Audit,
    audit.WithGRPCConnectionStateCallback(func(state audit.ConnectionState) {
        log.Info("connection state changed", "state", state.String())
        if state == audit.ConnectionStateTransientFailure {
            metrics.Inc("audit.connection.failure")
        }
    }),
)
```

Available states:
- `ConnectionStateIdle`
- `ConnectionStateConnecting`
- `ConnectionStateReady`
- `ConnectionStateTransientFailure`
- `ConnectionStateShutdown`

### Supported Authentication

For gRPC transport:
- **mTLS**: Mutual TLS with client certificates
- **Basic Auth**: Username/password via metadata headers
- **Insecure**: For development only (no TLS)

## Error Handling

### Queue Full

When the queue is at capacity, `Send()` returns `ErrQueueFull`:

```go
if err := auditor.Send(event); err != nil {
    if errors.Is(err, audit.ErrQueueFull) {
        log.Warn("audit queue full, event dropped")
        metrics.Inc("audit.dropped")
    }
}
```

Consider:
- Increasing queue size via `WithQueueSize()`
- Increasing concurrent requests via `WithMaxConcurrent()`
- Investigating why the collector is slow

### Delivery Failures

Background delivery failures (after retries) are reported via `WithOnError()`:

```go
auditor, _ := audit.NewAuditor(&cfg.Audit,
    audit.WithOnError(func(err error) {
        log.Error("audit delivery failed", "error", err)
        metrics.Inc("audit.delivery_failed")
    }),
)
```

Events that fail all retries are dropped.

## Migration from `pkg/otlp/audit`

| Old API | New API |
|---------|---------|
| `otlpaudit.NewLogger(config)` | `audit.NewAuditor(&config.Audit, ...)` |
| `logger.SendEvent(ctx, logs)` | `auditor.Send(event)` (no ctx needed) |
| `otlpaudit.NewEventMetadata(...)` | `auditor.NewCommon(...)` |
| `otlpaudit.NewCmkCreateEvent(...)` | `auditlogeventsv1.NewCMKCreatedEvent(...)` |
| `QueuedAuditLogger` | Built-in (async by default) |

### Key Differences

1. **Proto-based**: Events are typed proto messages, not `plog.Logs`
2. **Simpler API**: `Send()` is async by default, no separate queued wrapper
3. **Config-driven properties**: `additionalProperties` from config, not per-instance
4. **No context on Send**: Background delivery owns the context
5. **gRPC support**: Native OTLP/gRPC transport with batching

## Testing

Run tests with race detection:

```bash
go test -race ./pkg/audit/...
```

Check coverage:

```bash
go test -cover ./pkg/audit/...
```

## Performance Considerations

### HTTP Transport

- **Queue size**: Larger queues handle bursts better but use more memory
- **MaxConcurrent**: Limits concurrent HTTP requests via semaphore
- **Retries**: More retries = better reliability, but slower to drop bad events
- **Concurrency model**: Goroutine-per-event for maximum parallelism

### gRPC Transport

- **Batch size**: Larger batches = fewer RPCs but higher latency for individual events
- **Batch timeout**: Lower timeout = lower latency but more RPCs
- **Compression**: Enabled by default, reduces bandwidth but adds CPU overhead
- **Concurrency model**: Single batch dispatcher with configurable concurrent batch sends

### Example: High-Volume HTTP

```go
auditor, _ := audit.NewAuditor(&cfg.Audit,
    audit.WithQueueSize(4096),      // Handle large bursts
    audit.WithMaxConcurrent(50),    // Allow 50 concurrent HTTP requests
    audit.WithMaxRetries(3),        // Fail faster
)
```

### Example: High-Volume gRPC

```go
auditor, _ := audit.NewAuditor(&cfg.Audit,
    audit.WithQueueSize(4096),
    audit.WithMaxConcurrent(10),                    // 10 concurrent batch sends
    audit.WithGRPCBatchSize(500),                   // Larger batches for throughput
    audit.WithGRPCBatchTimeout(100*time.Millisecond), // Lower latency
    audit.WithGRPCRequestTimeout(10*time.Second),
)
```
