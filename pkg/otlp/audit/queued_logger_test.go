package otlpaudit

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"go.opentelemetry.io/collector/pdata/plog"

	"github.com/openkcm/common-sdk/pkg/commoncfg"
)

// newTestQueuedLogger builds a QueuedAuditLogger pointing at srv with the given
// options, reusing the real HTTP client path through NewLogger.
func newTestQueuedLogger(t *testing.T, srvURL string, opts QueuedOptions) *QueuedAuditLogger {
	t.Helper()

	cfg := &commoncfg.Audit{
		Endpoint: srvURL,
		HTTPClient: commoncfg.HTTPClient{
			Timeout: 5 * time.Second,
		},
	}

	logger, err := NewLogger(cfg)
	if err != nil {
		t.Fatalf("NewLogger() error = %v", err)
	}

	return NewQueuedLogger(logger, opts)
}

func TestQueuedLogger_T400_AsyncDelivery(t *testing.T) {
	var count atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		count.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	q := newTestQueuedLogger(t, srv.URL, QueuedOptions{})

	err := q.SendEvent(newTestEvent(t))
	if err != nil {
		t.Fatalf("SendEvent() error = %v", err)
	}

	err = q.Close(context.Background())
	if err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	if got := count.Load(); got != 1 {
		t.Fatalf("expected 1 delivered event, got %d", got)
	}
}

func TestQueuedLogger_T401_RetryThenSuccess(t *testing.T) {
	const failures = 2

	var count atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if count.Add(1) <= failures {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	var onErrCount atomic.Int32

	q := newTestQueuedLogger(t, srv.URL, QueuedOptions{
		MaxRetries: 5,
		BaseDelay:  time.Millisecond,
		MaxDelay:   5 * time.Millisecond,
		OnError:    func(error) { onErrCount.Add(1) },
	})

	err := q.SendEvent(newTestEvent(t))
	if err != nil {
		t.Fatalf("SendEvent() error = %v", err)
	}

	err = q.Close(context.Background())
	if err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	if got := count.Load(); got != failures+1 {
		t.Fatalf("expected %d attempts, got %d", failures+1, got)
	}

	if got := onErrCount.Load(); got != 0 {
		t.Fatalf("expected no OnError calls, got %d", got)
	}
}

func TestQueuedLogger_T402_ExhaustionReportsError(t *testing.T) {
	const maxRetries = 2

	var count atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		count.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	var (
		mu      sync.Mutex
		gotErrs []error
	)

	q := newTestQueuedLogger(t, srv.URL, QueuedOptions{
		MaxRetries: maxRetries,
		BaseDelay:  time.Millisecond,
		MaxDelay:   5 * time.Millisecond,
		OnError: func(err error) {
			mu.Lock()

			gotErrs = append(gotErrs, err)
			mu.Unlock()
		},
	})

	err := q.SendEvent(newTestEvent(t))
	if err != nil {
		t.Fatalf("SendEvent() error = %v", err)
	}

	err = q.Close(context.Background())
	if err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	if got := count.Load(); got != maxRetries+1 {
		t.Fatalf("expected %d attempts, got %d", maxRetries+1, got)
	}

	mu.Lock()
	defer mu.Unlock()

	if len(gotErrs) != 1 {
		t.Fatalf("expected 1 OnError call, got %d", len(gotErrs))
	}

	if gotErrs[0] == nil {
		t.Fatal("expected non-nil error reported to OnError")
	}
}

func TestQueuedLogger_T403_QueueFull(t *testing.T) {
	release := make(chan struct{})

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		<-release // block the single worker so the queue backs up
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()
	defer close(release)

	q := newTestQueuedLogger(t, srv.URL, QueuedOptions{
		QueueSize: 1,
		Workers:   1,
	})

	// First event is picked up by the worker (which blocks), the next fills the
	// size-1 queue, and at least one subsequent send must be rejected.
	var rejected bool

	for range 10 {
		if errors.Is(q.SendEvent(newTestEvent(t)), errQueueFull) {
			rejected = true
			break
		}
	}

	if !rejected {
		t.Fatal("expected at least one SendEvent to return errQueueFull")
	}
}

func TestQueuedLogger_T404_ClosedLogger(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	q := newTestQueuedLogger(t, srv.URL, QueuedOptions{})

	err := q.Close(context.Background())
	if err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	err = q.SendEvent(newTestEvent(t))
	if !errors.Is(err, errLoggerClosed) {
		t.Fatalf("expected errLoggerClosed, got %v", err)
	}

	err = q.Close(context.Background())
	if err != nil {
		t.Fatalf("second Close() should be idempotent, got %v", err)
	}
}

func TestQueuedLogger_T405_GracefulDrain(t *testing.T) {
	const events = 20

	var count atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		count.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	q := newTestQueuedLogger(t, srv.URL, QueuedOptions{
		QueueSize: events,
		Workers:   4,
	})

	for range events {
		err := q.SendEvent(newTestEvent(t))
		if err != nil {
			t.Fatalf("SendEvent() error = %v", err)
		}
	}

	err := q.Close(context.Background())
	if err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	if got := count.Load(); got != events {
		t.Fatalf("expected %d delivered events, got %d", events, got)
	}
}

func TestQueuedLogger_T406_Defaults(t *testing.T) {
	opts := QueuedOptions{}
	opts.setDefaults()

	cases := []struct {
		name string
		got  any
		want any
	}{
		{"QueueSize", opts.QueueSize, defaultQueueSize},
		{"Workers", opts.Workers, defaultWorkers},
		{"MaxRetries", opts.MaxRetries, defaultMaxRetries},
		{"BaseDelay", opts.BaseDelay, defaultBaseDelay},
		{"MaxDelay", opts.MaxDelay, defaultMaxDelay},
	}

	for _, c := range cases {
		if c.got != c.want {
			t.Errorf("%s = %v, want %v", c.name, c.got, c.want)
		}
	}
}

// newTestEvent builds a minimal valid audit event for delivery tests.
func newTestEvent(t *testing.T) plog.Logs {
	t.Helper()

	metadata, err := NewEventMetadata("userInitID", "tenantID", "correlationID")
	if err != nil {
		t.Fatalf("NewEventMetadata() error = %v", err)
	}

	event, err := NewCmkCreateEvent(metadata, "cmkID")
	if err != nil {
		t.Fatalf("NewCmkCreateEvent() error = %v", err)
	}

	return event
}
