/*
Package notifier provides a grouped event notification mechanism that
accumulates filesystem events and errors and triggers user-defined callbacks
at controlled intervals.

The Notifier is designed to avoid flooding handlers when many filesystem
events happen in quick succession. It supports:

  - Event batching with a configurable delay.
  - Rate limiting using golang.org/x/time/rate.
  - Custom callbacks for events and errors.
  - Safe concurrent access and automatic recovery from panics in handlers.

Example usage:

	paths := []string{"/tmp/watchdir"}

	// Create a new notifier with a delay of 200ms and up to 5 events per delay
	notifier, err := notifier.Create(paths,
		notifier.WithEventHandler(func(events []fsnotify.Event) {
			fmt.Println("Received events:", events)
		}),
		notifier.WithLimitDelay(200*time.Millisecond),
		notifier.WithBurstNumber(5),
	)
	if err != nil {
		log.Fatal(err)
	}

	// Start watching
	err = notifier.StartWatching()
	if err != nil {
		log.Fatal(err)
	}
	defer notifier.StopWatching()
*/
package notifier

import (
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"golang.org/x/time/rate"

	"github.com/openkcm/common-sdk/pkg/commonfs/watcher"
	"github.com/openkcm/common-sdk/pkg/utils"
)

var (
	ErrPathsNotSpecified = errors.New("paths not specified")
)

// Notifier accumulates filesystem events and errors and triggers
// user-defined callbacks in a rate-limited and batched manner.
//
// It is safe for concurrent use.
type Notifier struct {
	paths          []string
	recursiveWatch bool
	operations     map[fsnotify.Op]struct{}
	eventHandler   func(string, []fsnotify.Event) // Callback for batch of fsnotify events
	simpleHandler  func()                         // Simple callback if no event details are needed
	errorHandler   func([]error)                  // Callback for batch of errors

	interval time.Duration // Minimum time between notifications triggers
	burst    uint          // Maximum events allowed per delay window
	limiter  *rate.Limiter

	cacheMu          sync.Mutex
	cacheEvents      map[string][]fsnotify.Event // Accumulated events
	jobSendingEvents *time.Timer                 // Timer for sending events
	cacheErrors      []error                     // Accumulated errors
	jobSendingErrors *time.Timer                 // Timer for sending errors

	watcher *watcher.Watcher // Underlying filesystem watcher
}

// Option is a function type for configuring Notifier.
type Option func(*Notifier) error

// WithEventHandler sets the event handler callback that receives batched fsnotify events.
func WithEventHandler(handler func(string, []fsnotify.Event)) Option {
	return func(w *Notifier) error {
		w.eventHandler = handler
		return nil
	}
}

// WithSimpleHandler sets a simple callback invoked when events are accumulated, without event details.
func WithSimpleHandler(handler func()) Option {
	return func(w *Notifier) error {
		w.simpleHandler = handler
		return nil
	}
}

// WithThrottleInterval sets the throttling interval used to configure
// the rate limiter. It represents the minimum time between allowed
// callback invocations.
func WithThrottleInterval(interval time.Duration) Option {
	return func(w *Notifier) error {
		w.interval = interval
		return nil
	}
}

// WithBurstNumber sets the maximum number of events allowed per delay window.
func WithBurstNumber(burst uint) Option {
	return func(w *Notifier) error {
		w.burst = burst
		return nil
	}
}

// OnPath configures the notifier to observe a single path.
func OnPath(path string) Option {
	return func(w *Notifier) error {
		return w.AddPath(path)
	}
}

// OnPaths configures the notifier to observe multiple paths at once.
func OnPaths(paths ...string) Option {
	return func(w *Notifier) error {
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
//
// Returns:
//   - Option: a configuration function applied when creating the watcher.
func WatchSubfolders(enabled bool) Option {
	return func(w *Notifier) error {
		w.recursiveWatch = enabled
		return nil
	}
}

// ForOperations returns an Option that configures a Notifier to
// only consider specific filesystem operations for triggering events.
//
// The provided operations (ops) are combined into a set. Only events
// matching one of these operations will be processed by the Notifier.
//
// Example:
//
//	notifier, err := Create(
//	    "/tmp/watchdir",
//	    ForOperations(fsnotify.Create, fsnotify.Write, fsnotify.Remove),
//	)
//	if err != nil {
//	    log.Fatal(err)
//	}
//
// Supported operations are defined in fsnotify.Op:
//
//	fsnotify.Create, fsnotify.Write, fsnotify.Remove, fsnotify.Rename, fsnotify.Chmod
//
// This Option can be passed to a Notifier during creation to filter
// events according to your requirements.
func ForOperations(ops ...fsnotify.Op) Option {
	return func(w *Notifier) error {
		operations := make(map[fsnotify.Op]struct{})
		for _, op := range ops {
			operations[op] = struct{}{}
		}

		w.operations = operations

		return nil
	}
}

// Create creates a new Notifier for the specified filesystem locations.
//
// The notifier will accumulate events and errors from the given paths and trigger
// configured callbacks according to the delay and event-per-delay settings.
func Create(opts ...Option) (*Notifier, error) {
	n := &Notifier{
		paths: make([]string, 0),

		interval: time.Nanosecond,
		burst:    0,
		operations: map[fsnotify.Op]struct{}{
			fsnotify.Create: {},
			fsnotify.Write:  {},
			fsnotify.Rename: {},
			fsnotify.Remove: {},
		},
		cacheMu:     sync.Mutex{},
		cacheErrors: make([]error, 0),
	}

	for _, opt := range opts {
		if opt != nil {
			err := opt(n)
			if err != nil {
				return nil, err
			}
		}
	}

	if len(n.paths) == 0 {
		return nil, ErrPathsNotSpecified
	}

	n.limiter = rate.NewLimiter(rate.Every(n.interval), int(n.burst))

	cacheEvents := make(map[string][]fsnotify.Event)
	for _, path := range n.paths {
		cacheEvents[path] = make([]fsnotify.Event, 0)
	}

	n.cacheEvents = cacheEvents

	w, err := watcher.Create(
		watcher.OnPaths(n.paths...),
		watcher.WatchSubfolders(n.recursiveWatch),
		watcher.WithEventHandler(n.onEvent),
		watcher.WithErrorEventHandler(n.onError),
	)
	if err != nil {
		return nil, err
	}

	n.watcher = w

	return n, nil
}

// AddPath adds a new path to the notifier. It must be called before Start.
// The path must exist on the filesystem, otherwise an error is returned.
func (n *Notifier) AddPath(path string) error {
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

	n.paths = append(n.paths, absPath)

	return nil
}

// StartWatching starts the underlying filesystem watcher.
func (n *Notifier) StartWatching() error {
	if n.IsStarted() {
		return nil
	}

	return n.watcher.Start()
}

// StopWatching stops the watcher and releases all associated resources.
// It is safe to call multiple times.
func (n *Notifier) StopWatching() error {
	if !n.IsStarted() {
		return nil
	}

	return n.watcher.Close()
}

func (n *Notifier) IsStarted() bool {
	return n.watcher.IsStarted()
}

// onEvent is the internal fsnotify event handler that accumulates events
// and triggers the configured callbacks based on rate limiting.
func (n *Notifier) onEvent(event fsnotify.Event) {
	_, exists := n.operations[event.Op]
	if !exists {
		return
	}

	n.cacheMu.Lock()
	defer n.cacheMu.Unlock()

	dir := filepath.Dir(event.Name)
	for {
		if _, ok := n.cacheEvents[dir]; ok {
			n.cacheEvents[dir] = append(n.cacheEvents[dir], event)
			break
		} else {
			dir = filepath.Dir(dir)
		}

		if dir == "/" {
			break
		}
	}

	if n.limiter.Allow() {
		n.sendCachedEvents()
	}

	if n.jobSendingEvents != nil {
		n.jobSendingEvents.Reset(n.interval)
	} else {
		n.jobSendingEvents = time.AfterFunc(n.interval, n.sendCachedEvents)
	}
}

// sendCachedEvents sends accumulated events to the configured callback
// and resets the internal cache. Recovers from panics in user callbacks.
func (n *Notifier) sendCachedEvents() {
	defer func() {
		if err := recover(); err != nil {
			slog.Error("Notifier onEvent recover", "error", err, "events", n.cacheEvents)
		}

		n.jobSendingEvents = nil
		for _, path := range n.paths {
			n.cacheEvents[path] = make([]fsnotify.Event, 0)
		}
	}()

	defer func() {
		if n.jobSendingEvents != nil {
			n.jobSendingEvents.Stop()
			n.jobSendingEvents = nil
		}
	}()

	if n.eventHandler != nil {
		for path, events := range n.cacheEvents {
			if len(events) > 0 {
				n.eventHandler(path, events)
			}
		}

		return
	}

	if n.simpleHandler != nil && len(n.cacheEvents) > 0 {
		n.simpleHandler()
		return
	}
}

// onError is the internal error handler that accumulates errors
// and triggers the configured error callback based on rate limiting.
func (n *Notifier) onError(err error) {
	n.cacheMu.Lock()
	defer n.cacheMu.Unlock()

	if n.limiter.Allow() {
		n.sendCachedErrors()
	}

	n.cacheErrors = append(n.cacheErrors, err)

	if n.jobSendingErrors != nil {
		n.jobSendingErrors.Reset(n.interval)
	} else {
		n.jobSendingErrors = time.AfterFunc(n.interval, n.sendCachedErrors)
	}
}

// sendCachedErrors sends accumulated errors to the configured error callback
// and resets the internal cache. Recovers from panics in user callbacks.
func (n *Notifier) sendCachedErrors() {
	defer func() {
		if errRec := recover(); errRec != nil {
			slog.Error("Notifier onError recover err", "error", errRec)
		}

		n.jobSendingErrors = nil
		n.cacheErrors = make([]error, 0)
	}()

	defer func() {
		if n.jobSendingErrors != nil {
			n.jobSendingErrors.Stop()
			n.jobSendingErrors = nil
		}
	}()

	if n.errorHandler != nil && len(n.cacheErrors) > 0 {
		n.errorHandler(n.cacheErrors)
		return
	}
}
