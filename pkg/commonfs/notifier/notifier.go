/*
Package notifier provides a grouped event notification mechanism that
accumulates filesystem events and errors and triggers user-defined callbacks
at controlled intervals.

The GroupNotifier is designed to avoid flooding handlers when many filesystem
events happen in quick succession. It supports:

  - Event batching with a configurable delay.
  - Rate limiting using golang.org/x/time/rate.
  - Custom callbacks for events and errors.
  - Safe concurrent access and automatic recovery from panics in handlers.

Example usage:

	paths := []string{"/tmp/watchdir"}

	// Create a new notifier with a delay of 200ms and up to 5 events per delay
	notifier, err := notifier.NewGroupNotifyWrapper(paths,
		notifier.WithEventHandler(func(events []fsnotify.Event) {
			fmt.Println("Received events:", events)
		}),
		notifier.WithLimitDelay(200*time.Millisecond),
		notifier.WithEventPerDelay(5),
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

// GroupNotifier accumulates filesystem events and errors and triggers
// user-defined callbacks in a rate-limited and batched manner.
//
// It is safe for concurrent use.
type GroupNotifier struct {
	paths         []string
	eventHandler  func([]fsnotify.Event) // Callback for batch of fsnotify events
	simpleHandler func()                 // Simple callback if no event details are needed
	errorHandler  func([]error)          // Callback for batch of errors

	delay         time.Duration // Minimum time between callback triggers
	eventPerDelay uint          // Maximum events allowed per delay window
	limiter       *rate.Limiter

	cacheMu          sync.Mutex
	cacheEvents      []fsnotify.Event // Accumulated events
	jobSendingEvents *time.Timer      // Timer for sending events
	cacheErrors      []error          // Accumulated errors
	jobSendingErrors *time.Timer      // Timer for sending errors

	watcher *watcher.NotifyWatcher // Underlying filesystem watcher
}

// Option is a function type for configuring GroupNotifier.
type Option func(*GroupNotifier) error

// WithEventHandler sets the event handler callback that receives batched fsnotify events.
func WithEventHandler(handler func([]fsnotify.Event)) Option {
	return func(w *GroupNotifier) error {
		w.eventHandler = handler
		return nil
	}
}

// WithSimpleHandler sets a simple callback invoked when events are accumulated, without event details.
func WithSimpleHandler(handler func()) Option {
	return func(w *GroupNotifier) error {
		w.simpleHandler = handler
		return nil
	}
}

// WithLimitDelay sets the minimum delay between callback invocations.
func WithLimitDelay(delay time.Duration) Option {
	return func(w *GroupNotifier) error {
		w.delay = delay
		return nil
	}
}

// WithEventPerDelay sets the maximum number of events allowed per delay window.
func WithEventPerDelay(eventPerDelay uint) Option {
	return func(w *GroupNotifier) error {
		w.eventPerDelay = eventPerDelay
		return nil
	}
}

// WithPaths sets the paths to be watched.
func WithPaths(paths ...string) Option {
	return func(w *GroupNotifier) error {
		w.paths = paths
		return nil
	}
}

// WithPath sets the path to be watched.
func WithPath(path string) Option {
	return WithPaths(path)
}

// NewGroupNotifyWrapper creates a new GroupNotifier for the specified filesystem locations.
//
// The notifier will accumulate events and errors from the given paths and trigger
// configured callbacks according to the delay and event-per-delay settings.
func NewGroupNotifyWrapper(opts ...Option) (*GroupNotifier, error) {
	c := &GroupNotifier{
		delay:         100 * time.Millisecond,
		eventPerDelay: 1,
		cacheMu:       sync.Mutex{},
		cacheEvents:   make([]fsnotify.Event, 0),
		cacheErrors:   make([]error, 0),
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

	c.limiter = rate.NewLimiter(rate.Limit(c.delay), int(c.eventPerDelay))

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
func (c *GroupNotifier) StartWatching() error {
	return c.watcher.Start()
}

// StopWatching stops the watcher and releases all associated resources.
// It is safe to call multiple times.
func (c *GroupNotifier) StopWatching() error {
	return c.watcher.Close()
}

// onEvent is the internal fsnotify event handler that accumulates events
// and triggers the configured callbacks based on rate limiting.
func (c *GroupNotifier) onEvent(event fsnotify.Event) {
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
func (c *GroupNotifier) sendCachedEvents() {
	defer func() {
		if err := recover(); err != nil {
			slog.Error("Notifier onEvent recover", "error", err, "events", c.cacheEvents)
		}

		if c.jobSendingEvents != nil {
			c.jobSendingEvents.Stop()
			c.jobSendingEvents = nil
		}

		c.cacheEvents = make([]fsnotify.Event, 0)
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
func (c *GroupNotifier) onError(err error) {
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
func (c *GroupNotifier) sendCachedErrors() {
	defer func() {
		if errRec := recover(); errRec != nil {
			slog.Error("Notifier onError recover err", "error", errRec)
		}

		if c.jobSendingErrors != nil {
			c.jobSendingErrors.Stop()
			c.jobSendingErrors = nil
		}

		c.cacheErrors = make([]error, 0)
	}()

	if c.errorHandler != nil && len(c.cacheErrors) > 0 {
		c.errorHandler(c.cacheErrors)
		return
	}
}
