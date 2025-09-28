// Package loader provides a file-system–based key loader with live updates.
//
// It monitors a directory for resource files (such as signing keys), extracts
// a key identifier (KeyID) from file paths based on configurable rules,
// and stores the file contents in a key–value storage backend.
//
// Typical use cases include:
//   - Cryptographic key management
//   - Dynamic configuration loading
//   - Hot-reloading of resources from disk
package loader

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/fsnotify/fsnotify"

	"github.com/openkcm/common-sdk/pkg/commonfs/watcher"
	"github.com/openkcm/common-sdk/pkg/storage/keyvalue"
	"github.com/openkcm/common-sdk/pkg/utils"
)

// KeyIDType defines how the loader generates Key IDs from file paths.
//
// Different strategies are available depending on how keys should be identified.
type KeyIDType uint32

const (
	// FileNameWithoutExtension uses the base file name without its extension
	// as the Key ID.
	//
	// Example:
	//   Path:       /tmp/keys/signing.pem
	//   Extension:  .pem
	//   KeyID:      signing
	FileNameWithoutExtension KeyIDType = 1 << iota

	// FileNameWithExtension uses the base file name including its extension
	// as the Key ID.
	//
	// Example:
	//   Path:   /tmp/keys/signing.pem
	//   KeyID:  signing.pem
	FileNameWithExtension

	// FileFullPathRelativeToLocation uses the file path relative to the
	// configured root location as the Key ID.
	//
	// Example:
	//   Location:  /tmp/keys
	//   Path:      /tmp/keys/sub/key.pem
	//   KeyID:     /sub/key.pem
	FileFullPathRelativeToLocation

	// FileFullPath uses the absolute file path as the Key ID.
	//
	// Example:
	//   Path:   /tmp/keys/sub/key.pem
	//   KeyID:  /tmp/keys/sub/key.pem
	FileFullPath
)

var (
	// ErrStorageNotSpecified is returned when a nil storage is passed to WithStorage.
	ErrStorageNotSpecified = errors.New("storage not specified")
)

// Loader watches a directory for resource files and maintains a key–value store
// of file contents, indexed by Key IDs.
type Loader struct {
	paths          []string
	pathsToWatch   map[string]struct{}
	extension      string
	keyIDType      KeyIDType
	recursiveWatch bool
	operations     map[fsnotify.Op]struct{}

	startMu sync.Mutex
	watcher *watcher.Watcher
	storage keyvalue.StringToBytesStorage
}

// Option represents a configuration option for Loader.
type Option func(*Loader) error

// OnPath configures the notifier to observe a single path.
func OnPath(path string) Option {
	return func(w *Loader) error {
		return w.AddPath(path)
	}
}

// OnPaths configures the notifier to observe multiple paths at once.
func OnPaths(paths ...string) Option {
	return func(w *Loader) error {
		for _, path := range paths {
			err := w.AddPath(path)
			if err != nil {
				return err
			}
		}

		return nil
	}
}

// WithExtension configures the file extension that identifies relevant files.
//
// If the extension is missing a leading ".", it will be added automatically.
// The extension must not be empty, otherwise ErrExtensionIsEmpty is returned.
//
// This option is only used when KeyIDType is FileNameWithoutExtension.
func WithExtension(value string) Option {
	return func(w *Loader) error {
		if !strings.HasPrefix(value, ".") {
			w.extension = "." + value
		} else {
			w.extension = value
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
	return func(w *Loader) error {
		w.recursiveWatch = enabled
		return nil
	}
}

// WithKeyIDType configures the KeyID extraction strategy.
// The default KeyIDType is FileFullPath.
func WithKeyIDType(value KeyIDType) Option {
	return func(w *Loader) error {
		w.keyIDType = value
		return nil
	}
}

// WithStorage configures the storage backend used by Loader.
//
// If storage is nil, ErrStorageNotSpecified is returned.
// By default, an in-memory storage is used.
func WithStorage(storage keyvalue.StringToBytesStorage) Option {
	return func(w *Loader) error {
		if storage == nil {
			return ErrStorageNotSpecified
		}

		w.storage = storage

		return nil
	}
}

// ForOperations returns an Option that configures a Loader to
// only consider specific filesystem operations for triggering events.
//
// The provided operations (ops) are combined into a set. Only events
// matching one of these operations will be processed by the Loader.
// Supported operations are defined in fsnotify.Op:
//
//	fsnotify.Create, fsnotify.Write, fsnotify.Remove, fsnotify.Rename, fsnotify.Chmod
//
// This Option can be passed to a Loader during creation to filter
// events according to your requirements.
func ForOperations(ops ...fsnotify.Op) Option {
	return func(w *Loader) error {
		operations := make(map[fsnotify.Op]struct{})
		for _, op := range ops {
			operations[op] = struct{}{}
		}

		w.operations = operations

		return nil
	}
}

// Create initializes a Loader that watches the given location for files.
//
// Options may be provided to customize KeyID extraction, file extension
// handling, and storage backend.
//
// Example:
//
//	ldr, err := loader.Create(
//	    "/etc/keys",
//	    loader.WithExtension(".pem"),
//	    loader.WithKeyIDType(loader.FileNameWithoutExtension),
//	)
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	if err := ldr.StartWatching(); err != nil {
//	    log.Fatal(err)
//	}
//	defer ldr.StopWatching()
func Create(opts ...Option) (*Loader, error) {
	l := &Loader{
		paths:        make([]string, 0),
		pathsToWatch: make(map[string]struct{}),
		extension:    "",
		keyIDType:    FileFullPath,
		operations: map[fsnotify.Op]struct{}{
			fsnotify.Create: {},
			fsnotify.Write:  {},
			fsnotify.Rename: {},
			fsnotify.Remove: {},
		},

		startMu: sync.Mutex{},
		storage: keyvalue.NewMemoryStorage[string, []byte](),
	}

	// Apply options
	for _, opt := range opts {
		if opt == nil {
			continue
		}

		err := opt(l)
		if err != nil {
			return nil, err
		}
	}

	return l, nil
}

// onEvent is the fsnotify event handler.
func (l *Loader) onEvent(event fsnotify.Event) {
	_, exists := l.operations[event.Op]
	if !exists {
		return
	}

	l.loadResource(event)
}

// onError is the fsnotify error handler.
func (l *Loader) onError(err error) {
	slog.Warn("failed to load a resource", slog.String("error", err.Error()))
}

// StartWatching starts the file system watcher and performs an initial
// load of all resources.
//
// Returns an error if the watcher cannot be started or if resources
// cannot be loaded.
func (l *Loader) StartWatching() error {
	l.startMu.Lock()
	defer l.startMu.Unlock()

	if l.IsStarted() {
		return nil
	}

	w, err := watcher.Create(
		watcher.OnPaths(l.paths...),
		watcher.WatchSubfolders(l.recursiveWatch),
		watcher.WithEventHandler(l.onEvent),
		watcher.WithErrorEventHandler(l.onError),
	)
	if err != nil {
		return err
	}

	l.watcher = w

	for _, path := range l.paths {
		err := l.loadAllResources(path)
		if err != nil {
			return fmt.Errorf("failed to load resources: %w", err)
		}
	}

	return l.watcher.Start()
}

// StopWatching stops the watcher and releases resources.
// Safe to call multiple times.
func (l *Loader) StopWatching() error {
	l.startMu.Lock()
	defer l.startMu.Unlock()

	if !l.IsStarted() {
		return nil
	}

	defer func() {
		l.watcher = nil
	}()

	return l.watcher.Close()
}

// Storage returns a read-only view of the Loader’s storage.
// Consumers can use this to retrieve key data without modifying
// the internal storage.
func (l *Loader) Storage() keyvalue.ReadOnlyStringToBytesStorage {
	return l.storage
}

func (l *Loader) IsStarted() bool {
	return l.watcher != nil && l.watcher.IsStarted()
}

// AddPath adds a new path to the notifier. It must be called before Start.
// The path must exist on the filesystem, otherwise an error is returned.
func (l *Loader) AddPath(path string) error {
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

	l.paths = append(l.paths, absPath)
	l.pathsToWatch[absPath] = struct{}{}

	return nil
}

// loadAllResources recursively loads all files from the given path
// into the storage, applying the configured KeyID extraction rules.
func (l *Loader) loadAllResources(path string) error {
	_, err := os.Stat(path)
	if err != nil {
		return err
	}

	keys, err := os.ReadDir(path)
	if err != nil {
		return err
	}

	for _, keyFile := range keys {
		if l.recursiveWatch && keyFile.IsDir() {
			err = l.loadAllResources(filepath.Join(path, keyFile.Name()))
			if err != nil {
				return err
			}

			continue
		}

		l.loadResource(fsnotify.Event{
			Name: filepath.Join(path, keyFile.Name()),
			Op:   fsnotify.Write,
		})
	}

	return nil
}

// loadResource loads or removes a single file in response to a file system event.
//
// Supported operations:
//   - Create/Write: load or update file contents
//   - Rename/Remove: remove file from storage
//
// Files that are directories, unreadable, or empty are skipped.
func (l *Loader) loadResource(event fsnotify.Event) {
	filePath := event.Name

	keyID, ok := l.resolveKeyID(filePath)
	if !ok {
		return
	}

	if event.Op&(fsnotify.Rename|fsnotify.Remove) != 0 {
		l.storage.Remove(keyID)
		return
	}

	filePath, _ = strings.CutSuffix(filePath, "~")
	info, err := os.Stat(filePath)
	if err != nil || info.IsDir() {
		return
	}

	keyData, err := os.ReadFile(filePath)
	if err != nil || len(keyData) == 0 {
		// Skip unreadable files
		return
	}

	keyID, _ = strings.CutSuffix(keyID, "~")
	l.storage.Store(keyID, keyData)
}

// resolveKeyID determines the storage key based on Loader.keyIDType
// Returns (key, true) if the key is valid, or ("", false) if it should be skipped.
func (l *Loader) resolveKeyID(filePath string) (string, bool) {
	switch l.keyIDType {
	case FileNameWithExtension:
		_, name := filepath.Split(filePath)
		return name, true

	case FileNameWithoutExtension:
		_, name := filepath.Split(filePath)

		keyID, found := strings.CutSuffix(name, l.extension)
		if !found {
			return "", false
		}

		return keyID, true

	case FileFullPathRelativeToLocation:
		dir := filepath.Dir(filePath)
		for dir != "/" && len(dir) > 1 {
			_, ok := l.pathsToWatch[dir]
			if !ok {
				dir = filepath.Dir(dir)
				continue
			}

			keyID := strings.TrimPrefix(filePath, dir)
			if filepath.Dir(keyID) == "/" {
				return keyID[1:], true
			}

			if !strings.HasPrefix(keyID, string(os.PathSeparator)) {
				return string(os.PathSeparator) + keyID, true
			}

			return keyID, true
		}

		return "", false

	case FileFullPath:
		return filePath, true
	}

	return "", false
}
