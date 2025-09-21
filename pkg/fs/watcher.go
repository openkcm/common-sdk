package fs

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/fsnotify/fsnotify"
)

var (
	ErrFSWatcherStartedNotAllowingNewPath = errors.New("watcher already started, cannot add new path")
	ErrFSWatcherHasNoPathsConfigured      = errors.New("watcher has no paths")
)

type NotifyWrapper struct {
	started bool
	paths   []string
	watcher *fsnotify.Watcher

	handler      func(fsnotify.Event)
	errorHandler func(error)
}

// Option is used to configure a NotifyWrapper.
type Option func(*NotifyWrapper) error

func OnPath(path string) Option {
	return func(w *NotifyWrapper) error {
		return w.AddPath(path)
	}
}

func OnPaths(paths ...string) Option {
	return func(w *NotifyWrapper) error {
		for _, path := range paths {
			err := w.AddPath(path)
			if err != nil {
				return err
			}
		}

		return nil
	}
}

func WithEventHandler(handler func(fsnotify.Event)) Option {
	return func(w *NotifyWrapper) error {
		w.handler = handler
		return nil
	}
}

func WithEventChainAsHandler(eventsCh chan<- fsnotify.Event) Option {
	return WithEventHandler(func(e fsnotify.Event) { eventsCh <- e })
}

func WithErrorEventHandler(handler func(error)) Option {
	return func(w *NotifyWrapper) error {
		w.errorHandler = handler
		return nil
	}
}

func WithErrorChainAsHandler(eventsCh chan<- error) Option {
	return WithErrorEventHandler(func(e error) { eventsCh <- e })
}

func NewFSWatcher(opts ...Option) (*NotifyWrapper, error) {
	w := &NotifyWrapper{
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

	// Create new watcher.
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	w.watcher = watcher

	return w, nil
}

func (w *NotifyWrapper) AddPath(path string) error {
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

func (w *NotifyWrapper) Start() error {
	if len(w.paths) == 0 {
		return ErrFSWatcherHasNoPathsConfigured
	}

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

func (w *NotifyWrapper) eventProcessor() {
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

func (w *NotifyWrapper) Close() error {
	defer func() {
		w.started = false
	}()

	return w.watcher.Close()
}

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
