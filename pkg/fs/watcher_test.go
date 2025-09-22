package fs_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/fsnotify/fsnotify"

	"github.com/openkcm/common-sdk/pkg/fs"
)

func TestAddPathAndStart(t *testing.T) {
	tmpDir := t.TempDir()

	w, err := fs.NewFSWatcher()
	if err != nil {
		t.Fatalf("failed to create watcher: %v", err)
	}

	// Should add path successfully
	err = w.AddPath(tmpDir)
	if err != nil {
		t.Fatalf("expected AddPath to succeed, got error: %v", err)
	}

	// Should start successfully
	err = w.Start()
	if err != nil {
		t.Fatalf("expected Start to succeed, got error: %v", err)
	}

	// Adding path after start should fail
	err = w.AddPath(tmpDir)
	if !errors.Is(err, fs.ErrFSWatcherStartedNotAllowingNewPath) {
		t.Errorf("expected ErrFSWatcherStartedNotAllowingNewPath, got: %v", err)
	}

	_ = w.Close()
}

func TestStartNoPaths(t *testing.T) {
	w, err := fs.NewFSWatcher()
	if err != nil {
		t.Fatalf("failed to create watcher: %v", err)
	}

	err = w.Start()
	if !errors.Is(err, fs.ErrFSWatcherHasNoPathsConfigured) {
		t.Errorf("expected ErrFSWatcherHasNoPathsConfigured, got: %v", err)
	}
}

func TestEventHandlerReceivesEvents(t *testing.T) {
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test.txt")

	eventsCh := make(chan fsnotify.Event, 1)
	errorsCh := make(chan error, 1)

	w, err := fs.NewFSWatcher(
		fs.OnPath(tmpDir),
		fs.WithEventHandler(func(e fsnotify.Event) { eventsCh <- e }),
		fs.WithErrorEventHandler(func(e error) { errorsCh <- e }),
	)
	if err != nil {
		t.Fatalf("failed to create watcher: %v", err)
	}
	defer func(w *fs.NotifyWrapper) {
		err := w.Close()
		if err != nil {
			t.Fatalf("failed to close watcher: %v", err)
		}
	}(w)

	err = w.Start()
	if err != nil {
		t.Fatalf("failed to start watcher: %v", err)
	}

	// Trigger an event: write a file
	err = os.WriteFile(tmpFile, []byte("hello"), 0644)
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
	w, err := fs.NewFSWatcher()
	if err != nil {
		t.Fatalf("failed to create watcher: %v", err)
	}

	nonexistent := filepath.Join(t.TempDir(), "does-not-exist")

	err = w.AddPath(nonexistent)
	if err == nil {
		t.Errorf("expected error for nonexistent path, got nil")
	}
}

func TestCloseIsSafeToCallMultipleTimes(t *testing.T) {
	tmpDir := t.TempDir()

	w, err := fs.NewFSWatcher(fs.OnPath(tmpDir))
	if err != nil {
		t.Fatalf("failed to create watcher: %v", err)
	}

	err = w.Start()
	if err != nil {
		t.Fatalf("failed to start watcher: %v", err)
	}

	// Call Close() multiple times in different goroutines
	done := make(chan struct{})

	go func() {
		defer close(done)

		for range 5 {
			_ = w.Close() // should not panic
		}
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Error("timeout: Close() may have deadlocked")
	}
}
