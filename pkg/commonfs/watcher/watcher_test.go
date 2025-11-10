package watcher_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/fsnotify/fsnotify"

	"github.com/openkcm/common-sdk/v2/pkg/commonfs/watcher"
)

// --- Helpers ---

func newWatcher(t *testing.T, opts ...watcher.Option) *watcher.Watcher {
	t.Helper()

	w, err := watcher.Create(opts...)
	if err != nil {
		t.Fatalf("failed to create watcher: %v", err)
	}

	return w
}

func startWatcher(t *testing.T, w *watcher.Watcher) {
	t.Helper()

	err := w.Start()
	if err != nil {
		t.Fatalf("failed to start watcher: %v", err)
	}
}

func closeWatcher(t *testing.T, w *watcher.Watcher) {
	t.Helper()

	err := w.Close()
	if err != nil {
		t.Fatalf("failed to close watcher: %v", err)
	}
}

// --- Tests ---
func TestStartNoPaths(t *testing.T) {
	w := newWatcher(t)

	err := w.Start()
	if !errors.Is(err, watcher.ErrNoPathsConfigured) {
		t.Errorf("expected ErrNoPathsConfigured, got: %v", err)
	}
}

func TestEventHandlerReceivesEvents(t *testing.T) {
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test.txt")

	eventsCh := make(chan fsnotify.Event, 1)
	errorsCh := make(chan error, 1)

	w := newWatcher(t,
		watcher.OnPath(tmpDir),
		watcher.WithEventHandler(func(e fsnotify.Event) { eventsCh <- e }),
		watcher.WithErrorEventHandler(func(e error) { errorsCh <- e }),
	)
	defer closeWatcher(t, w)

	startWatcher(t, w)

	// Trigger an event: write a file
	err := os.WriteFile(tmpFile, []byte("hello"), 0644)
	if err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	select {
	case ev := <-eventsCh:
		if ev.Name != tmpFile {
			t.Errorf("expected event for %s, got %s", tmpFile, ev.Name)
		}
	case <-time.After(2 * time.Second):
		t.Error("timeout waiting for file event")
	}
}

func TestAddPathNonexistent(t *testing.T) {
	w := newWatcher(t)
	nonexistent := filepath.Join(t.TempDir(), "does-not-exist")

	err := w.AddPath(nonexistent)
	if err == nil {
		t.Errorf("expected error for nonexistent path, got nil")
	}
}

func TestCloseIsSafeToCallMultipleTimes(t *testing.T) {
	tmpDir := t.TempDir()
	w := newWatcher(t, watcher.OnPath(tmpDir))
	startWatcher(t, w)

	done := make(chan struct{})

	go func() {
		defer close(done)

		for range 5 {
			_ = w.Close() // should not panic or deadlock
		}
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Error("timeout: Close() may have deadlocked")
	}
}
