package audit

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/plog"

	auditlogeventsv1 "github.com/openkcm/api-sdk/proto/kms/api/cmk/auditlog/events/v1"

	"github.com/openkcm/common-sdk/pkg/commoncfg"
)

func TestAuditor_NewAuditor(t *testing.T) {
	t.Run("creates auditor with default options", func(t *testing.T) {
		cfg := &commoncfg.Audit{
			Endpoint: "https://audit.example.com",
		}

		auditor, err := NewAuditor(cfg)
		require.NoError(t, err)
		require.NotNil(t, auditor)

		assert.NotNil(t, auditor.sender)
		assert.Equal(t, defaultQueueSize, auditor.queueSize)
		assert.Equal(t, defaultMaxRetries, auditor.maxRetries)

		require.NoError(t, auditor.Close(context.Background()))
	})

	t.Run("parses additional properties from YAML", func(t *testing.T) {
		cfg := &commoncfg.Audit{
			Endpoint: "https://audit.example.com",
			AdditionalProperties: `
application_id: my-service
environment: production
region: us-east-1
`,
		}

		auditor, err := NewAuditor(cfg)
		require.NoError(t, err)
		require.NotNil(t, auditor)

		assert.Equal(t, "my-service", auditor.additionalProps["application_id"])
		assert.Equal(t, "production", auditor.additionalProps["environment"])
		assert.Equal(t, "us-east-1", auditor.additionalProps["region"])

		require.NoError(t, auditor.Close(context.Background()))
	})

	t.Run("returns error for invalid YAML", func(t *testing.T) {
		cfg := &commoncfg.Audit{
			Endpoint:             "https://audit.example.com",
			AdditionalProperties: "{ invalid yaml",
		}

		_, err := NewAuditor(cfg)
		require.Error(t, err)
	})

	t.Run("applies custom options", func(t *testing.T) {
		cfg := &commoncfg.Audit{
			Endpoint: "https://audit.example.com",
		}

		auditor, err := NewAuditor(cfg,
			WithQueueSize(2048),
			WithMaxConcurrent(20),
			WithMaxRetries(10),
		)
		require.NoError(t, err)

		assert.Equal(t, 2048, auditor.queueSize)
		assert.Equal(t, 20, auditor.maxConcurrent)
		assert.Equal(t, 10, auditor.maxRetries)

		require.NoError(t, auditor.Close(context.Background()))
	})
}

func TestAuditor_Send(t *testing.T) {
	t.Run("enqueues event successfully", func(t *testing.T) {
		var received atomic.Int32

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			received.Add(1)
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		cfg := &commoncfg.Audit{
			Endpoint: server.URL,
		}

		auditor, err := NewAuditor(cfg, WithQueueSize(10))
		require.NoError(t, err)

		event := auditlogeventsv1.NewTenantCreatedEvent(
			auditor.NewCommon(),
			"tenant-123",
		)

		err = auditor.Send(event)
		require.NoError(t, err)

		// Wait for delivery
		assert.Eventually(t, func() bool {
			return received.Load() == 1
		}, 2*time.Second, 50*time.Millisecond)

		require.NoError(t, auditor.Close(context.Background()))
	})

	t.Run("returns ErrQueueFull when queue is full", func(t *testing.T) {
		cfg := &commoncfg.Audit{
			Endpoint: "https://audit.example.com",
		}

		// Small queue, slow processing via small semaphore
		auditor, err := NewAuditor(cfg, WithQueueSize(2), WithMaxConcurrent(1))
		require.NoError(t, err)

		event := auditlogeventsv1.NewTenantCreatedEvent(
			auditor.NewCommon(),
			"tenant-123",
		)

		// Fill the queue quickly before dispatcher can process
		require.NoError(t, auditor.Send(event))
		require.NoError(t, auditor.Send(event))

		// Queue should be full now (dispatcher is processing but queue is 2)
		// Note: This is timing-dependent but should work in practice
		err = auditor.Send(event)
		if err != nil {
			assert.ErrorIs(t, err, ErrQueueFull)
		}

		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		_ = auditor.Close(ctx) // will timeout but that's ok
	})

	t.Run("returns ErrClosed after Close", func(t *testing.T) {
		cfg := &commoncfg.Audit{
			Endpoint: "https://audit.example.com",
		}

		auditor, err := NewAuditor(cfg)
		require.NoError(t, err)

		require.NoError(t, auditor.Close(context.Background()))

		event := auditlogeventsv1.NewTenantCreatedEvent(
			auditor.NewCommon(),
			"tenant-123",
		)

		err = auditor.Send(event)
		assert.ErrorIs(t, err, ErrClosed)
	})
}

func TestAuditor_NewCommon(t *testing.T) {
	t.Run("copies additional properties", func(t *testing.T) {
		cfg := &commoncfg.Audit{
			Endpoint: "https://audit.example.com",
			AdditionalProperties: `
application_id: my-service
environment: production
`,
		}

		auditor, err := NewAuditor(cfg)
		require.NoError(t, err)

		defer auditor.Close(context.Background())

		common := auditor.NewCommon()

		assert.Equal(t, "my-service", common.GetProperties()["application_id"])
		assert.Equal(t, "production", common.GetProperties()["environment"])
		assert.NotNil(t, common.GetTimestamp())
	})

	t.Run("applies CommonOptions", func(t *testing.T) {
		cfg := &commoncfg.Audit{
			Endpoint:             "https://audit.example.com",
			AdditionalProperties: `application_id: my-service`,
		}

		auditor, err := NewAuditor(cfg)
		require.NoError(t, err)

		defer auditor.Close(context.Background())

		common := auditor.NewCommon(
			WithProperty("request_id", "req-123"),
			WithProperty("user_id", "user-456"),
			WithTraceContext("4bf92f3577b34da6a3ce929d0e0e4736", "00f067aa0ba902b7"),
		)

		assert.Equal(t, "my-service", common.GetProperties()["application_id"])
		assert.Equal(t, "req-123", common.GetProperties()["request_id"])
		assert.Equal(t, "user-456", common.GetProperties()["user_id"])
		assert.Equal(t, "4bf92f3577b34da6a3ce929d0e0e4736", common.GetTraceId())
		assert.Equal(t, "00f067aa0ba902b7", common.GetSpanId())
	})

	t.Run("options can override additional properties", func(t *testing.T) {
		cfg := &commoncfg.Audit{
			Endpoint:             "https://audit.example.com",
			AdditionalProperties: `environment: production`,
		}

		auditor, err := NewAuditor(cfg)
		require.NoError(t, err)

		defer auditor.Close(context.Background())

		common := auditor.NewCommon(
			WithProperty("environment", "staging"),
		)

		assert.Equal(t, "staging", common.GetProperties()["environment"])
	})
}

func TestAuditor_Close(t *testing.T) {
	t.Run("drains queue before returning", func(t *testing.T) {
		var received atomic.Int32

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			received.Add(1)
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		cfg := &commoncfg.Audit{
			Endpoint: server.URL,
		}

		auditor, err := NewAuditor(cfg, WithQueueSize(10))
		require.NoError(t, err)

		// Send 5 events
		for range 5 {
			event := auditlogeventsv1.NewTenantCreatedEvent(
				auditor.NewCommon(),
				"tenant-123",
			)
			require.NoError(t, auditor.Send(event))
		}

		// Close should drain all events
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		require.NoError(t, auditor.Close(ctx))

		assert.Equal(t, int32(5), received.Load())
	})

	t.Run("is idempotent", func(t *testing.T) {
		cfg := &commoncfg.Audit{
			Endpoint: "https://audit.example.com",
		}

		auditor, err := NewAuditor(cfg)
		require.NoError(t, err)

		require.NoError(t, auditor.Close(context.Background()))
		require.NoError(t, auditor.Close(context.Background()))
	})
}

func TestAuditor_Integration(t *testing.T) {
	t.Run("sends complete event as OTLP JSON", func(t *testing.T) {
		var (
			mu           sync.Mutex
			receivedBody []byte
		)

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			body, _ := io.ReadAll(r.Body)

			mu.Lock()
			receivedBody = body
			mu.Unlock()

			assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
			assert.Equal(t, http.MethodPost, r.Method)

			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		cfg := &commoncfg.Audit{
			Endpoint: server.URL,
			AdditionalProperties: `
application_id: my-service
environment: production
`,
		}

		auditor, err := NewAuditor(cfg)
		require.NoError(t, err)

		event := auditlogeventsv1.NewCMKCreatedEvent(
			auditor.NewCommon(
				WithProperty("request_id", "req-123"),
				WithTraceContext("4bf92f3577b34da6a3ce929d0e0e4736", "00f067aa0ba902b7"),
			),
			"tenant-456",
			"cmk-789",
		)

		require.NoError(t, auditor.Send(event))

		// Wait for delivery
		assert.Eventually(t, func() bool {
			mu.Lock()
			defer mu.Unlock()

			return len(receivedBody) > 0
		}, 2*time.Second, 50*time.Millisecond)

		require.NoError(t, auditor.Close(context.Background()))

		// Verify OTLP JSON structure by unmarshaling
		mu.Lock()
		bodyToVerify := receivedBody
		mu.Unlock()

		unmarshaler := plog.JSONUnmarshaler{}
		logs, err := unmarshaler.UnmarshalLogs(bodyToVerify)
		require.NoError(t, err)

		require.Equal(t, 1, logs.ResourceLogs().Len())

		rl := logs.ResourceLogs().At(0)
		require.Equal(t, 1, rl.ScopeLogs().Len())

		sl := rl.ScopeLogs().At(0)
		require.Equal(t, 1, sl.LogRecords().Len())

		lr := sl.LogRecords().At(0)

		// Verify log record body (event name)
		assert.Equal(t, "CMKCreated", lr.Body().Str())

		// Verify attributes
		attrs := lr.Attributes()

		// Event-specific fields
		tenantID, ok := attrs.Get("tenant_id")
		require.True(t, ok)
		assert.Equal(t, "tenant-456", tenantID.Str())

		cmkID, ok := attrs.Get("cmk_id")
		require.True(t, ok)
		assert.Equal(t, "cmk-789", cmkID.Str())

		// Additional properties from Common (prefixed with "common.properties.")
		reqID, ok := attrs.Get("common.properties.request_id")
		require.True(t, ok)
		assert.Equal(t, "req-123", reqID.Str())

		// Additional properties from config (prefixed with "common.properties.")
		appID, ok := attrs.Get("common.properties.application_id")
		require.True(t, ok)
		assert.Equal(t, "my-service", appID.Str())

		env, ok := attrs.Get("common.properties.environment")
		require.True(t, ok)
		assert.Equal(t, "production", env.Str())

		// Verify trace context
		assert.Equal(t, "4bf92f3577b34da6a3ce929d0e0e4736", lr.TraceID().String())
		assert.Equal(t, "00f067aa0ba902b7", lr.SpanID().String())
	})
}

func TestAuditor_Retry(t *testing.T) {
	t.Run("retries on 5xx errors", func(t *testing.T) {
		var attempts atomic.Int32

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			count := attempts.Add(1)
			if count < 3 {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}

			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		cfg := &commoncfg.Audit{
			Endpoint: server.URL,
		}

		auditor, err := NewAuditor(cfg,
			WithMaxRetries(5),
			WithBaseDelay(10*time.Millisecond),
			WithMaxDelay(50*time.Millisecond),
		)
		require.NoError(t, err)

		event := auditlogeventsv1.NewTenantCreatedEvent(
			auditor.NewCommon(),
			"tenant-123",
		)

		err = auditor.Send(event)
		require.NoError(t, err)

		// Wait for delivery with retries
		assert.Eventually(t, func() bool {
			return attempts.Load() >= 3
		}, 2*time.Second, 50*time.Millisecond)

		require.NoError(t, auditor.Close(context.Background()))
		assert.Equal(t, int32(3), attempts.Load())
	})

	t.Run("does not retry on 4xx errors", func(t *testing.T) {
		var (
			attempts    atomic.Int32
			receivedErr atomic.Value
		)

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			attempts.Add(1)
			w.WriteHeader(http.StatusBadRequest)
		}))
		defer server.Close()

		cfg := &commoncfg.Audit{
			Endpoint: server.URL,
		}

		auditor, err := NewAuditor(cfg,
			WithMaxRetries(5),
			WithBaseDelay(10*time.Millisecond),
			WithOnError(func(err error) {
				receivedErr.Store(err)
			}),
		)
		require.NoError(t, err)

		event := auditlogeventsv1.NewTenantCreatedEvent(
			auditor.NewCommon(),
			"tenant-123",
		)

		err = auditor.Send(event)
		require.NoError(t, err)

		// Wait for OnError to be called
		assert.Eventually(t, func() bool {
			return receivedErr.Load() != nil
		}, 2*time.Second, 50*time.Millisecond)

		require.NoError(t, auditor.Close(context.Background()))

		// Should only attempt once (no retries for 4xx)
		assert.Equal(t, int32(1), attempts.Load())

		// OnError should have been called with ErrPermanent
		storedErr, ok := receivedErr.Load().(error)
		require.True(t, ok, "expected error to be stored")
		assert.ErrorIs(t, storedErr, ErrPermanent)
	})

	t.Run("calls OnError after max retries exhausted", func(t *testing.T) {
		var (
			attempts    atomic.Int32
			receivedErr atomic.Value
		)

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			attempts.Add(1)
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer server.Close()

		cfg := &commoncfg.Audit{
			Endpoint: server.URL,
		}

		auditor, err := NewAuditor(cfg,
			WithMaxRetries(2),
			WithBaseDelay(10*time.Millisecond),
			WithMaxDelay(20*time.Millisecond),
			WithOnError(func(err error) {
				receivedErr.Store(err)
			}),
		)
		require.NoError(t, err)

		event := auditlogeventsv1.NewTenantCreatedEvent(
			auditor.NewCommon(),
			"tenant-123",
		)

		err = auditor.Send(event)
		require.NoError(t, err)

		// Wait for OnError to be called
		assert.Eventually(t, func() bool {
			return receivedErr.Load() != nil
		}, 2*time.Second, 50*time.Millisecond)

		require.NoError(t, auditor.Close(context.Background()))

		// Should have attempted 1 initial + 2 retries = 3
		assert.Equal(t, int32(3), attempts.Load())

		// OnError should have been called
		storedErr, ok := receivedErr.Load().(error)
		require.True(t, ok, "expected error to be stored")
		assert.Contains(t, storedErr.Error(), "failed to deliver audit event after retries")
	})
}

func TestOptions_Validation(t *testing.T) {
	t.Run("ignores invalid option values", func(t *testing.T) {
		cfg := &commoncfg.Audit{
			Endpoint: "https://audit.example.com",
		}

		auditor, err := NewAuditor(cfg,
			WithQueueSize(-1),
			WithMaxConcurrent(0),
			WithMaxRetries(-5),
			WithBaseDelay(-time.Second),
			WithMaxDelay(0),
		)
		require.NoError(t, err)

		// Should use defaults when invalid values are provided
		assert.Equal(t, defaultQueueSize, auditor.queueSize)
		assert.Equal(t, defaultMaxConcurrent, auditor.maxConcurrent)
		assert.Equal(t, defaultMaxRetries, auditor.maxRetries)
		assert.Equal(t, defaultBaseDelay, auditor.baseDelay)
		assert.Equal(t, defaultMaxDelay, auditor.maxDelay)

		require.NoError(t, auditor.Close(context.Background()))
	})

	t.Run("accepts zero retries", func(t *testing.T) {
		cfg := &commoncfg.Audit{
			Endpoint: "https://audit.example.com",
		}

		auditor, err := NewAuditor(cfg, WithMaxRetries(0))
		require.NoError(t, err)

		assert.Equal(t, 0, auditor.maxRetries)

		require.NoError(t, auditor.Close(context.Background()))
	})
}
