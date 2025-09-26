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
	notifier, err := notifier.NewNotifier(paths,
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
	"log/slog"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"golang.org/x/time/rate"

	"github.com/openkcm/common-sdk/pkg/commonfs/watcher"
)

var (
	ErrPathsNotSpecified = errors.New("paths not specified")
)

// Notifier accumulates filesystem events and errors and triggers
// user-defined callbacks in a rate-limited and batched manner.
//
// It is safe for concurrent use.
type Notifier struct {
	paths         []string
	operations    map[fsnotify.Op]struct{}
	eventHandler  func([]fsnotify.Event) // Callback for batch of fsnotify events
	simpleHandler func()                 // Simple callback if no event details are needed
	errorHandler  func([]error)          // Callback for batch of errors

	delay   time.Duration // Minimum time between notifications triggers
	burst   uint          // Maximum events allowed per delay window
	limiter *rate.Limiter

	cacheMu          sync.Mutex
	cacheEvents      []fsnotify.Event // Accumulated events
	jobSendingEvents *time.Timer      // Timer for sending events
	cacheErrors      []error          // Accumulated errors
	jobSendingErrors *time.Timer      // Timer for sending errors

	watcher *watcher.NotifyWatcher // Underlying filesystem watcher
}

// Option is a function type for configuring Notifier.
type Option func(*Notifier) error

// WithEventHandler sets the event handler callback that receives batched fsnotify events.
func WithEventHandler(handler func([]fsnotify.Event)) Option {
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

// WithLimitDelay sets the minimum delay between callback invocations.
func WithLimitDelay(delay time.Duration) Option {
	return func(w *Notifier) error {
		w.delay = delay
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

// WithPaths sets the paths to be watched.
func WithPaths(paths ...string) Option {
	return func(w *Notifier) error {
		w.paths = paths
		return nil
	}
}

// WithPath sets the path to be watched.
func WithPath(path string) Option {
	return WithPaths(path)
}

// ForOperations returns an Option that configures a Notifier to
// only consider specific filesystem operations for triggering events.
//
// The provided operations (ops) are combined into a set. Only events
// matching one of these operations will be processed by the Notifier.
//
// Example:
//
//	notifier, err := NewNotifier(
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

// NewNotifier creates a new Notifier for the specified filesystem locations.
//
// The notifier will accumulate events and errors from the given paths and trigger
// configured callbacks according to the delay and event-per-delay settings.
func NewNotifier(opts ...Option) (*Notifier, error) {
	c := &Notifier{
		delay: time.Nanosecond,
		burst: 0,
		operations: map[fsnotify.Op]struct{}{
			fsnotify.Create: {},
			fsnotify.Write:  {},
			fsnotify.Rename: {},
			fsnotify.Remove: {},
		},
		cacheMu:     sync.Mutex{},
		cacheEvents: make([]fsnotify.Event, 0),
		cacheErrors: make([]error, 0),
	}

	for _, opt := range opts {
		if opt != nil {
			err := opt(c)
			if err != nil {
				return nil, err
			}
		}
	}

	if len(c.paths) == 0 {
		return nil, ErrPathsNotSpecified
	}

	c.limiter = rate.NewLimiter(rate.Every(c.delay), int(c.burst))

	defaultWatcher, err := watcher.NewFSWatcher(
		watcher.OnPaths(c.paths...),
		watcher.WithEventHandler(c.onEvent),
		watcher.WithErrorEventHandler(c.onError),
	)
	if err != nil {
		return nil, err
	}

	c.watcher = defaultWatcher

	return c, nil
}

// StartWatching starts the underlying filesystem watcher.
func (c *Notifier) StartWatching() error {
	return c.watcher.Start()
}

// StopWatching stops the watcher and releases all associated resources.
// It is safe to call multiple times.
func (c *Notifier) StopWatching() error {
	return c.watcher.Close()
}

// onEvent is the internal fsnotify event handler that accumulates events
// and triggers the configured callbacks based on rate limiting.
func (c *Notifier) onEvent(event fsnotify.Event) {
	_, exists := c.operations[event.Op]
	if !exists {
		return
	}

	c.cacheMu.Lock()
	defer c.cacheMu.Unlock()

	c.cacheEvents = append(c.cacheEvents, event)

	if c.limiter.Allow() {
		c.sendCachedEvents()
	}

	if c.jobSendingEvents != nil {
		c.jobSendingEvents.Reset(c.delay)
	} else {
		c.jobSendingEvents = time.AfterFunc(c.delay, c.sendCachedEvents)
	}
}

// sendCachedEvents sends accumulated events to the configured callback
// and resets the internal cache. Recovers from panics in user callbacks.
func (c *Notifier) sendCachedEvents() {
	defer func() {
		if err := recover(); err != nil {
			slog.Error("Notifier onEvent recover", "error", err, "events", c.cacheEvents)
		}

		c.jobSendingEvents = nil
		c.cacheEvents = make([]fsnotify.Event, 0)
	}()

	defer func() {
		if c.jobSendingEvents != nil {
			c.jobSendingEvents.Stop()
			c.jobSendingEvents = nil
		}
	}()

	if c.eventHandler != nil && len(c.cacheEvents) > 0 {
		c.eventHandler(c.cacheEvents)
		return
	}

	if c.simpleHandler != nil && len(c.cacheEvents) > 0 {
		c.simpleHandler()
		return
	}
}

// onError is the internal error handler that accumulates errors
// and triggers the configured error callback based on rate limiting.
func (c *Notifier) onError(err error) {
	c.cacheMu.Lock()
	defer c.cacheMu.Unlock()

	if c.limiter.Allow() {
		c.sendCachedErrors()
	}

	c.cacheErrors = append(c.cacheErrors, err)

	if c.jobSendingErrors != nil {
		c.jobSendingErrors.Reset(c.delay)
	} else {
		c.jobSendingErrors = time.AfterFunc(c.delay, c.sendCachedErrors)
	}

	slog.Warn("Notifier onError err", "error", err)
}

// sendCachedErrors sends accumulated errors to the configured error callback
// and resets the internal cache. Recovers from panics in user callbacks.
func (c *Notifier) sendCachedErrors() {
	defer func() {
		if errRec := recover(); errRec != nil {
			slog.Error("Notifier onError recover err", "error", errRec)
		}

		c.jobSendingErrors = nil
		c.cacheErrors = make([]error, 0)
	}()

	defer func() {
		if c.jobSendingErrors != nil {
			c.jobSendingErrors.Stop()
			c.jobSendingErrors = nil
		}
	}()

	if c.errorHandler != nil && len(c.cacheErrors) > 0 {
		c.errorHandler(c.cacheErrors)
		return
	}
}
