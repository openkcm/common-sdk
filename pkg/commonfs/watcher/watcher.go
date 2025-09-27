// Package watcher provides a thin wrapper around fsnotify for watching
// file system paths. It offers a configurable interface to register event
// and error handlers, and ensures safe lifecycle management of the watcher.
//
// The Watcher struct is the main type that manages:
//   - Configured paths to watch
//   - Event handlers for file changes
//   - Error handlers for watcher errors
//
// Example usage:
//
//	events := make(chan fsnotify.Event, 1)
//	errors := make(chan error, 1)
//
//	w, err := watcher.Create(
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
	"log/slog"
	"os"
	"path/filepath"

	"github.com/fsnotify/fsnotify"

	"github.com/openkcm/common-sdk/pkg/utils"
)

var (
	// ErrFSWatcherStartedNotAllowingNewPath is returned when trying to add
	// new paths to a watcher after it has already been started.
	ErrFSWatcherStartedNotAllowingNewPath = errors.New("watcher already started, cannot add new path")

	// ErrFSWatcherHasNoPathsConfigured is returned when Start is called
	// without any paths configured to watch.
	ErrFSWatcherHasNoPathsConfigured = errors.New("watcher has no paths")
)

// Watcher wraps fsnotify.Watcher and provides higher-level configuration
// via functional options. It simplifies setting up file system watchers with
// custom event and error handlers.
//
// It tracks its lifecycle (`started`) and ensures consistent behavior
// when adding paths or starting multiple times.
type Watcher struct {
	started        bool
	paths          []string
	recursiveWatch bool

	watcher *fsnotify.Watcher

	handler      func(fsnotify.Event)
	errorHandler func(error)
}

// Option represents a configuration option that can be applied to a Watcher.
type Option func(*Watcher) error

// OnPath configures the watcher to observe a single path.
func OnPath(path string) Option {
	return func(w *Watcher) error {
		return w.AddPath(path)
	}
}

// OnPaths configures the watcher to observe multiple paths at once.
func OnPaths(paths ...string) Option {
	return func(w *Watcher) error {
		for _, path := range paths {
			err := w.AddPath(path)
			if err != nil {
				return err
			}
		}

		return nil
	}
}

// WatchSubfolders enables or disables recursive monitoring of subfolders.
//
// By default, fsnotify does not watch subfolders of a directory automatically.
// When enabled, this option ensures that all nested directories are included
// in the watch scope, so events such as file creation, modification, renaming,
// or deletion inside subdirectories will also be detected.
// Parameters:
//   - enabled: set to true to include subfolders in the watch, false to
//     restrict watching only to the top-level directory.
func WatchSubfolders(enabled bool) Option {
	return func(w *Watcher) error {
		w.recursiveWatch = enabled
		return nil
	}
}

// WithEventHandler sets the event handler that will be called
// whenever a file system event occurs on a watched path.
func WithEventHandler(handler func(fsnotify.Event)) Option {
	return func(w *Watcher) error {
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
	return func(w *Watcher) error {
		w.errorHandler = handler
		return nil
	}
}

// WithErrorChainAsHandler sends all watcher errors to the provided channel.
// This is useful for external error handling loops.
func WithErrorChainAsHandler(eventsCh chan<- error) Option {
	return WithErrorEventHandler(func(e error) { eventsCh <- e })
}

// Create creates a new Watcher with the given options.
//
// Example:
//
//	w, err := watcher.Create(
//	    watcher.OnPath("/tmp"),
//	    watcher.WithEventHandler(func(e fsnotify.Event) { fmt.Println(e) }),
//	)
//	if err != nil {
//	    log.Fatal(err)
//	}
func Create(opts ...Option) (*Watcher, error) {
	w := &Watcher{
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

	if !w.recursiveWatch {
		return w, nil
	}

	for _, path := range w.paths {
		err := w.addRecursive(path)
		if err != nil {
			return nil, err
		}
	}

	return w, nil
}

// AddPath adds a new path to the watcher. It must be called before Start.
// If the watcher has already been started, this returns ErrFSWatcherStartedNotAllowingNewPath.
//
// The path must exist on the filesystem, otherwise an error is returned.
func (w *Watcher) AddPath(path string) error {
	if w.started {
		return ErrFSWatcherStartedNotAllowingNewPath
	}

	absPath, err := filepath.Abs(path)
	if err != nil {
		return err
	}

	exist, err := utils.FileExist(absPath)
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
func (w *Watcher) Start() error {
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

func (w *Watcher) IsStarted() bool {
	return w.started
}

// addRecursive walks the given root directory and adds all of its
// subdirectories to the watcher, excluding the root directory itself.
//
// This is useful when recursive watching is enabled but the caller does
// not want to include the root folder directly.
//
// For example, if root = "/project", the watcher will register:
//   - /project/subdir1
//   - /project/subdir2
//   - /project/subdir2/nested
//
// But "/project" itself will not be added.
func (w *Watcher) addRecursive(root string) error {
	return filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Skip the root itself
		if path == root {
			return nil
		}

		if d.IsDir() {
			err := w.AddPath(path)
			if err != nil {
				return err
			}
		}

		return nil
	})
}

// eventProcessor is the internal loop that dispatches events and errors
// to the configured handlers.
func (w *Watcher) eventProcessor() {
	for {
		select {
		case event, ok := <-w.watcher.Events:
			if !ok {
				return
			}

			if w.processAsDirectory(event) {
				continue
			}

			if w.handler == nil {
				continue
			}

			w.handler(event)

		case err, ok := <-w.watcher.Errors:
			if !ok {
				return
			}

			if w.errorHandler == nil {
				continue
			}

			w.errorHandler(err)
		}
	}
}

// processAsDirectory checks whether a given fsnotify event refers to a newly
// created directory and, if recursive watching is enabled, ensures it is added
// to the watcher.
//
// Behavior:
//   - If recursive watching is disabled, the method always returns false.
//   - If the event is a Create operation and the target is a directory,
//     the method adds the directory and all of its subdirectories to the watcher
//     via addRecursive.
//   - Additionally, the newly created directory itself is explicitly registered
//     with the watcher.
//   - Any errors during Add are logged as warnings but do not stop execution.
//
// Returns true if a directory was successfully identified and processed, or
// false otherwise.
func (w *Watcher) processAsDirectory(event fsnotify.Event) bool {
	if !w.recursiveWatch || !event.Has(fsnotify.Create) {
		return false
	}

	fi, err := os.Stat(event.Name)
	if err != nil || !fi.IsDir() {
		return false
	}

	_ = w.addRecursive(event.Name)

	err = w.watcher.Add(event.Name)
	if err != nil {
		slog.Warn("Failed to include into watcher new created folder at realtime", "error", err)
	}

	return true
}

// Close stops the watcher and releases resources. It is safe to call
// multiple times; subsequent calls will simply reset the started flag.
func (w *Watcher) Close() error {
	defer func() {
		w.started = false
	}()

	return w.watcher.Close()
}
