// Package watcher provides a thin wrapper around fsnotify for watching
// file system paths, with event and error handlers set through options and a
// safe start and close lifecycle.
//
// A Watcher watches the paths it is given: directories, whose entries are
// reported, or single files. With WatchSubfolders it also watches every
// directory below the given directories, including directories created while
// it runs.
//
// Example usage:
//
//	events := make(chan fsnotify.Event, 16)
//	errs := make(chan error, 16)
//
//	w, err := watcher.Create(
//	    watcher.OnPath("/etc/keys"),
//	    watcher.WithEventChainAsHandler(events),
//	    watcher.WithErrorChainAsHandler(errs),
//	)
//	if err != nil {
//	    return err
//	}
//
//	err = w.Start()
//	if err != nil {
//	    return err
//	}
//	defer w.Close()
//
//	for {
//	    select {
//	    case e := <-events:
//	        fmt.Println("file event:", e)
//	    case err := <-errs:
//	        fmt.Println("watcher error:", err)
//	    }
//	}
package watcher

import (
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"

	"github.com/fsnotify/fsnotify"

	"github.com/openkcm/common-sdk/pkg/utils"
)

var (
	// ErrNoPathsConfigured is returned when Start is called
	// without any paths configured to watch.
	ErrNoPathsConfigured = errors.New("no paths has been configured")
)

// Watcher wraps fsnotify.Watcher and is configured through options. It calls
// the event handler for every file system event on the watched paths, and
// the error handler for every error reported by fsnotify.
//
// Both handlers are called from a single goroutine of the Watcher, one call
// at a time, so a slow handler delays the events after it.
//
// A Watcher can be started again after Close. Its methods are safe for
// concurrent use.
type Watcher struct {
	startMu        sync.Mutex
	started        atomic.Bool
	paths          map[string]struct{}
	recursiveWatch bool

	done    chan struct{}
	watcher *fsnotify.Watcher

	handler      func(fsnotify.Event)
	errorHandler func(error)
}

// Option represents a configuration option that can be applied to a Watcher.
type Option func(*Watcher) error

// OnPath configures the watcher to observe a single path. See
// Watcher.AddPath.
func OnPath(path string) Option {
	return func(w *Watcher) error {
		return w.AddPath(path)
	}
}

// OnPaths configures the watcher to observe multiple paths at once. See
// Watcher.AddPath.
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

// WatchSubfolders enables or disables watching the subdirectories of the
// watched directories.
//
// fsnotify only reports changes to the entries of a watched directory, not to
// the entries of its subdirectories. When enabled, Start watches every
// directory below the watched directories as well, and a directory created
// while the watcher runs is watched, together with the directories it
// contains, before its Create event is passed to the event handler.
//
// Files that were created in such a directory before it was watched produce
// no events of their own; a handler that needs them should list the directory
// when it receives the directory's Create event. Symbolic links to
// directories are not followed.
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

// WithEventChainAsHandler sends all file system events to the provided
// channel. An event that cannot be sent at once, because the channel is full,
// is logged and dropped rather than blocking the watcher.
func WithEventChainAsHandler(eventsCh chan<- fsnotify.Event) Option {
	return WithEventHandler(func(e fsnotify.Event) {
		select {
		case eventsCh <- e:
		default:
			slog.Error("watcher event couldn't be delivered", "event", e)
		}
	})
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
// An error that cannot be sent at once, because the channel is full, is
// logged and dropped rather than blocking the watcher.
func WithErrorChainAsHandler(eventsCh chan<- error) Option {
	return WithErrorEventHandler(func(e error) {
		select {
		case eventsCh <- e:
		default:
			slog.Error("watcher error as event couldn't be delivered",
				"error", e)
		}
	})
}

// Create creates a new Watcher with the given options. It does not start
// watching; call Start for that.
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
		startMu: sync.Mutex{},
		paths:   make(map[string]struct{}),
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

// AddPath adds a path to watch: a directory, whose entries are reported, or a
// single file. The path is made absolute and must exist, otherwise an error
// is returned.
//
// A path added while the watcher is started is watched from the next Start,
// after Close.
func (w *Watcher) AddPath(path string) error {
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

	w.startMu.Lock()
	defer w.startMu.Unlock()

	w.paths[absPath] = struct{}{}

	return nil
}

// Start creates the underlying fsnotify.Watcher, watches all configured paths
// (and, with WatchSubfolders, the directories below them) and starts passing
// events to the handlers.
//
// It returns ErrNoPathsConfigured if no paths were added, and an error if a
// path cannot be watched, for example because it no longer exists; nothing is
// left running in that case. Calling Start on a started Watcher does nothing.
func (w *Watcher) Start() error {
	w.startMu.Lock()
	defer w.startMu.Unlock()

	if w.started.Load() {
		return nil
	}

	if len(w.paths) == 0 {
		return ErrNoPathsConfigured
	}

	fw, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}

	for path := range w.paths {
		err := w.watch(fw, path)
		if err != nil {
			_ = fw.Close()
			return err
		}
	}

	w.watcher = fw
	w.done = make(chan struct{})
	w.started.Store(true)

	go w.eventProcessor(fw, w.done)

	return nil
}

// Close stops watching and releases the underlying fsnotify.Watcher. No
// handler is called after Close returns, except for a call already in
// progress. Calling Close more than once, or on a Watcher that is not
// started, is safe.
func (w *Watcher) Close() error {
	w.startMu.Lock()
	defer w.startMu.Unlock()

	if !w.started.Load() {
		return nil
	}

	w.started.Store(false)
	close(w.done)

	return w.watcher.Close()
}

// IsStarted reports whether the Watcher is started.
func (w *Watcher) IsStarted() bool {
	return w.started.Load()
}

// watch adds root to fw and, with WatchSubfolders, every directory below it.
func (w *Watcher) watch(fw *fsnotify.Watcher, root string) error {
	err := fw.Add(root)
	if err != nil || !w.recursiveWatch {
		return err
	}

	return filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if path == root || !d.IsDir() {
			return nil
		}

		return fw.Add(path)
	})
}

// eventProcessor passes the events and errors of fw to the handlers until fw
// is closed or done is closed. fw and done are those of one Start, so a
// processor left over from before a restart never reads the new ones.
func (w *Watcher) eventProcessor(fw *fsnotify.Watcher, done <-chan struct{}) {
	for {
		select {
		case event, ok := <-fw.Events:
			if !ok {
				return
			}

			w.watchNewDirectory(fw, event)

			if w.handler != nil {
				w.handler(event)
			}

		case err, ok := <-fw.Errors:
			if !ok {
				return
			}

			if w.errorHandler != nil {
				w.errorHandler(err)
			}

		case <-done:
			return
		}
	}
}

// watchNewDirectory watches a directory created while the watcher runs, and
// the directories it contains, when WatchSubfolders is enabled. Failures are
// logged; the event is passed to the handler either way.
func (w *Watcher) watchNewDirectory(fw *fsnotify.Watcher, event fsnotify.Event) {
	if !w.recursiveWatch || !event.Has(fsnotify.Create) {
		return
	}

	fi, err := os.Lstat(event.Name)
	if err != nil || !fi.IsDir() {
		return
	}

	err = filepath.WalkDir(event.Name, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if !d.IsDir() {
			return nil
		}

		return fw.Add(path)
	})
	if err != nil {
		slog.Warn("failed to watch a new directory", "path", event.Name, "error", err)
	}
}
