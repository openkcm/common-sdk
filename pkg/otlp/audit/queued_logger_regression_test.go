package otlpaudit

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"go.opentelemetry.io/collector/pdata/plog"
)

func TestQueuedLogger_SnapshotsEvents(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	unblock := sync.OnceFunc(func() { close(release) })
	payloads := make(chan []byte, 1)

	var requests atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		payload, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("read audit request: %v", err)
			w.WriteHeader(http.StatusBadRequest)

			return
		}

		if requests.Add(1) == 1 {
			close(started)
			<-release
		} else {
			payloads <- payload
		}

		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	q := newTestQueuedLogger(t, srv.URL, QueuedOptions{QueueSize: 1, Workers: 1})
	cleanupQueuedRegression(t, q, unblock)

	err := q.SendEvent(newTestEvent(t))
	if err != nil {
		t.Fatalf("SendEvent(blocker) error = %v", err)
	}

	awaitQueuedRegression(t, started, "first request to start")

	// The worker cannot touch this event until the caller has finished modifying it.
	event := newTestEvent(t)
	attrs := event.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Attributes()
	attrs.PutStr("snapshot", "original")

	err = q.SendEvent(event)
	if err != nil {
		t.Fatalf("SendEvent(snapshot) error = %v", err)
	}

	attrs.PutStr("snapshot", "changed")
	unblock()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = q.Close(ctx)
	if err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	payload := awaitQueuedRegression(t, payloads, "snapshot delivery")
	unmarshaller := plog.JSONUnmarshaler{}

	delivered, err := unmarshaller.UnmarshalLogs(payload)
	if err != nil {
		t.Fatalf("decode delivered audit event: %v", err)
	}

	value, ok := delivered.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Attributes().Get("snapshot")
	if !ok {
		t.Fatal("delivered event is missing the snapshot attribute")
	}

	if value.Str() != "original" {
		t.Fatalf("delivered snapshot = %q, want original", value.Str())
	}
}

func TestQueuedLogger_ConcurrentCloseSharesCancellation(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	unblock := sync.OnceFunc(func() { close(release) })

	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)

		close(started)

		select {
		case <-release:
		case <-r.Context().Done():
		}
	}))
	t.Cleanup(srv.Close)

	q := newTestQueuedLogger(t, srv.URL, QueuedOptions{OnError: func(error) {}})
	cleanupQueuedRegression(t, q, unblock)

	err := q.SendEvent(newTestEvent(t))
	if err != nil {
		t.Fatalf("SendEvent() error = %v", err)
	}

	awaitQueuedRegression(t, started, "request to start")

	firstCtx := t.Context()

	firstClose := make(chan error, 1)

	go func() { firstClose <- q.Close(firstCtx) }()

	waitForQueuedShutdown(t, q)

	// Only the second caller cancels. The first caller must observe that failure too.
	secondCtx, cancelSecond := context.WithCancel(context.Background())
	cancelSecond()

	secondClose := make(chan error, 1)

	go func() { secondClose <- q.Close(secondCtx) }()

	err = awaitQueuedRegression(t, secondClose, "second Close to return")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("second Close() error = %v, want context.Canceled", err)
	}

	err = awaitQueuedRegression(t, firstClose, "first Close to observe cancellation")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("first Close() error = %v, want context.Canceled", err)
	}

	err = q.Close(context.Background())
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("repeated Close() error = %v, want original cancellation", err)
	}
}

func TestQueuedLogger_CancellationWithBlockedCallback(t *testing.T) {
	entered := make(chan struct{})
	release := make(chan struct{})
	unblock := sync.OnceFunc(func() { close(release) })

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	}))
	t.Cleanup(srv.Close)

	q := newTestQueuedLogger(t, srv.URL, QueuedOptions{
		OnError: func(error) {
			close(entered)
			<-release
		},
	})
	cleanupQueuedRegression(t, q, unblock)

	err := q.SendEvent(newTestEvent(t))
	if err != nil {
		t.Fatalf("SendEvent() error = %v", err)
	}

	awaitQueuedRegression(t, entered, "OnError to block")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	closed := make(chan error, 1)

	go func() { closed <- q.Close(ctx) }()

	waitForQueuedShutdown(t, q)
	cancel()

	// The callback stays blocked until cleanup, so worker completion cannot release Close.
	err = awaitQueuedRegression(t, closed, "Close to return while OnError is blocked")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Close() error = %v, want context.Canceled", err)
	}
}

func awaitQueuedRegression[T any](t *testing.T, ch <-chan T, description string) T {
	t.Helper()

	select {
	case value := <-ch:
		return value
	case <-time.After(5 * time.Second):
		t.Fatalf("timed out waiting for %s", description)

		var zero T

		return zero
	}
}

// waitForQueuedShutdown only synchronizes the test with Close; assertions use its public result.
func waitForQueuedShutdown(t *testing.T, q *QueuedAuditLogger) {
	t.Helper()

	timeout := time.NewTimer(5 * time.Second)
	defer timeout.Stop()

	ticker := time.NewTicker(time.Millisecond)
	defer ticker.Stop()

	for {
		q.mu.Lock()
		closed := q.closed
		q.mu.Unlock()

		if closed {
			return
		}

		select {
		case <-ticker.C:
		case <-timeout.C:
			t.Fatal("timed out waiting for Close to stop accepting events")
		}
	}
}

func cleanupQueuedRegression(t *testing.T, q *QueuedAuditLogger, unblock func()) {
	t.Helper()
	t.Cleanup(func() {
		unblock()

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		// Cancellation is an expected outcome in the shutdown regression tests.
		_ = q.Close(ctx)

		done := make(chan struct{})

		go func() {
			q.wg.Wait()
			close(done)
		}()

		awaitQueuedRegression(t, done, "delivery workers to exit")
	})
}
