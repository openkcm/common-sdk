// Package watcher provides a thin wrapper around fsnotify for watching
// file system paths. It offers a configurable interface to register event
// and error handlers, and ensures safe lifecycle management of the watcher.
//
// The NotifyWatcher struct is the main type that manages:
//   - Configured paths to watch
//   - Event handlers for file changes
//   - Error handlers for watcher errors
//
// Example usage:
//
//	events := make(chan fsnotify.Event, 1)
//	errors := make(chan error, 1)
//
//	w, err := watcher.NewFSWatcher(
//	    watcher.OnPath("/tmp"),
//	    watcher.WithEventChainAsHandler(events),
//	    watcher.WithErrorChainAsHandler(errors),
//	)
//	if err != nil {
//	    log.Fatalf("failed to create watcher: %v", err)
//	}
//
//	if err := w.Start(); err != nil {
//	    log.Fatalf("failed to start watcher: %v", err)
//	}
//
//	go func() {
//	    for {
//	        select {
//	        case e := <-events:
//	            fmt.Println("file event:", e)
//	        case err := <-errors:
//	            fmt.Println("watcher error:", err)
//	        }
//	    }
//	}
//
//	defer w.Close()
package watcher

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/fsnotify/fsnotify"
)

var (
	// ErrFSWatcherStartedNotAllowingNewPath is returned when trying to add
	// new paths to a watcher after it has already been started.
	ErrFSWatcherStartedNotAllowingNewPath = errors.New("watcher already started, cannot add new path")

	// ErrFSWatcherHasNoPathsConfigured is returned when Start is called
	// without any paths configured to watch.
	ErrFSWatcherHasNoPathsConfigured = errors.New("watcher has no paths")
)

// NotifyWatcher wraps fsnotify.Watcher and provides higher-level configuration
// via functional options. It simplifies setting up file system watchers with
// custom event and error handlers.
//
// It tracks its lifecycle (`started`) and ensures consistent behavior
// when adding paths or starting multiple times.
type NotifyWatcher struct {
	started bool
	paths   []string
	watcher *fsnotify.Watcher

	handler      func(fsnotify.Event)
	errorHandler func(error)
}

// Option represents a configuration option that can be applied to a NotifyWatcher.
type Option func(*NotifyWatcher) error

// OnPath configures the watcher to observe a single path.
func OnPath(path string) Option {
	return func(w *NotifyWatcher) error {
		return w.AddPath(path)
	}
}

// OnPaths configures the watcher to observe multiple paths at once.
func OnPaths(paths ...string) Option {
	return func(w *NotifyWatcher) error {
		for _, path := range paths {
			err := w.AddPath(path)
			if err != nil {
				return err
			}
		}

		return nil
	}
}

// WithEventHandler sets the event handler that will be called
// whenever a file system event occurs on a watched path.
func WithEventHandler(handler func(fsnotify.Event)) Option {
	return func(w *NotifyWatcher) error {
		w.handler = handler
		return nil
	}
}

// WithEventChainAsHandler sends all file system events to the provided channel.
// This is useful for external event processing loops.
func WithEventChainAsHandler(eventsCh chan<- fsnotify.Event) Option {
	return WithEventHandler(func(e fsnotify.Event) { eventsCh <- e })
}

// WithErrorEventHandler sets the error handler that will be called
// whenever the watcher encounters an error.
func WithErrorEventHandler(handler func(error)) Option {
	return func(w *NotifyWatcher) error {
		w.errorHandler = handler
		return nil
	}
}

// WithErrorChainAsHandler sends all watcher errors to the provided channel.
// This is useful for external error handling loops.
func WithErrorChainAsHandler(eventsCh chan<- error) Option {
	return WithErrorEventHandler(func(e error) { eventsCh <- e })
}

// NewFSWatcher creates a new NotifyWatcher with the given options.
//
// Example:
//
//	w, err := watcher.NewFSWatcher(
//	    watcher.OnPath("/tmp"),
//	    watcher.WithEventHandler(func(e fsnotify.Event) { fmt.Println(e) }),
//	)
//	if err != nil {
//	    log.Fatal(err)
//	}
func NewFSWatcher(opts ...Option) (*NotifyWatcher, error) {
	w := &NotifyWatcher{
		paths: make([]string, 0),
	}

	// Apply the options
	for _, opt := range opts {
		if opt != nil {
			err := opt(w)
			if err != nil {
				return nil, err
			}
		}
	}

	return w, nil
}

// AddPath adds a new path to the watcher. It must be called before Start.
// If the watcher has already been started, this returns ErrFSWatcherStartedNotAllowingNewPath.
//
// The path must exist on the filesystem, otherwise an error is returned.
func (w *NotifyWatcher) AddPath(path string) error {
	if w.started {
		return ErrFSWatcherStartedNotAllowingNewPath
	}

	absPath, err := filepath.Abs(path)
	if err != nil {
		return err
	}

	exist, err := exists(absPath)
	if err != nil {
		return err
	}

	if !exist {
		return fmt.Errorf("path does not exist: %s", absPath)
	}

	w.paths = append(w.paths, absPath)

	return nil
}

// Start initializes the underlying fsnotify.Watcher and begins watching all
// configured paths. It also launches the event processing goroutine.
//
// Returns ErrFSWatcherHasNoPathsConfigured if no paths were added.
//
// After Start, the watcher cannot accept new paths (AddPath will fail).
func (w *NotifyWatcher) Start() error {
	if len(w.paths) == 0 {
		return ErrFSWatcherHasNoPathsConfigured
	}

	if w.watcher != nil {
		err := w.watcher.Close()
		if err != nil {
			return err
		}
	}

	// Create new watcher.
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}

	w.watcher = watcher

	for _, path := range w.paths {
		err := w.watcher.Add(path)
		if err != nil {
			return err
		}
	}

	defer func() {
		w.started = true
	}()

	go w.eventProcessor()

	return nil
}

// eventProcessor is the internal loop that dispatches events and errors
// to the configured handlers.
func (w *NotifyWatcher) eventProcessor() {
	for {
		select {
		case event, ok := <-w.watcher.Events:
			if !ok {
				return
			}

			if w.handler != nil {
				w.handler(event)
			}
		case err, ok := <-w.watcher.Errors:
			if !ok {
				return
			}

			if w.handler != nil {
				w.errorHandler(err)
			}
		}
	}
}

// Close stops the watcher and releases resources. It is safe to call
// multiple times; subsequent calls will simply reset the started flag.
func (w *NotifyWatcher) Close() error {
	defer func() {
		w.started = false
	}()

	return w.watcher.Close()
}

// exists checks if a filesystem path exists. It distinguishes between
// "does not exist" and other errors (e.g., permission denied).
func exists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}

	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}

	return false, err
}
