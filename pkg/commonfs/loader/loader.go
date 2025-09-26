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

	"github.com/fsnotify/fsnotify"

	"github.com/openkcm/common-sdk/pkg/commonfs/watcher"
	"github.com/openkcm/common-sdk/pkg/storage/keyvalue"
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

// StringToBytesStorage is a convenience alias for a string→[]byte storage backend.
type StringToBytesStorage = keyvalue.Storage[string, []byte]

// ReadOnlyStringToBytesStorage is the read-only view of StringToBytesStorage.
type ReadOnlyStringToBytesStorage = keyvalue.ReadStorage[string, []byte]

var (
	// ErrExtensionIsEmpty is returned when WithExtension("") is called.
	ErrExtensionIsEmpty = errors.New("extension is empty")

	// ErrStorageNotSpecified is returned when a nil storage is passed to WithStorage.
	ErrStorageNotSpecified = errors.New("storage not specified")
)

// Loader watches a directory for resource files and maintains a key–value store
// of file contents, indexed by Key IDs.
type Loader struct {
	location  string
	extension string
	keyIDType KeyIDType

	watcher *watcher.NotifyWrapper
	storage StringToBytesStorage
}

// Option represents a configuration option for Loader.
type Option func(*Loader) error

// WithExtension configures the file extension that identifies relevant files.
//
// If the extension is missing a leading ".", it will be added automatically.
// The extension must not be empty, otherwise ErrExtensionIsEmpty is returned.
//
// This option is only used when KeyIDType is FileNameWithoutExtension.
func WithExtension(value string) Option {
	return func(w *Loader) error {
		if value == "" {
			return ErrExtensionIsEmpty
		}

		if !strings.HasPrefix(value, ".") {
			w.extension = "." + value
		} else {
			w.extension = value
		}

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
func WithStorage(storage StringToBytesStorage) Option {
	return func(w *Loader) error {
		if storage == nil {
			return ErrStorageNotSpecified
		}
		w.storage = storage
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
func Create(location string, opts ...Option) (*Loader, error) {
	dl := &Loader{
		location:  location,
		extension: "",
		keyIDType: FileFullPath,
		storage:   keyvalue.NewMemoryStorage[string, []byte](),
	}

	defaultWatcher, err := watcher.NewFSWatcher(
		watcher.OnPath(location),
		watcher.WithEventHandler(dl.onEvent),
		watcher.WithErrorEventHandler(dl.onError),
	)
	if err != nil {
		return nil, err
	}
	dl.watcher = defaultWatcher

	// Apply options
	for _, opt := range opts {
		if opt == nil {
			continue
		}
		err = opt(dl)
		if err != nil {
			return nil, err
		}
	}

	return dl, nil
}

// onEvent is the fsnotify event handler.
func (dl *Loader) onEvent(event fsnotify.Event) {
	if event.Op&(fsnotify.Create|fsnotify.Write|fsnotify.Remove|fsnotify.Rename) != 0 {
		dl.loadSigningKey(event)
	}
}

// onError is the fsnotify error handler.
func (dl *Loader) onError(err error) {
	slog.Error("failed to load signing keys", slog.String("error", err.Error()))
}

// StartWatching starts the file system watcher and performs an initial
// load of all resources.
//
// Returns an error if the watcher cannot be started or if resources
// cannot be loaded.
func (dl *Loader) StartWatching() error {
	errStart := dl.watcher.Start()

	err := dl.loadAllResources(dl.location)
	if err != nil {
		return fmt.Errorf("failed to load signing keys %w", err)
	}

	return errStart
}

// StopWatching stops the watcher and releases resources.
// Safe to call multiple times.
func (dl *Loader) StopWatching() error {
	return dl.watcher.Close()
}

// Storage returns a read-only view of the Loader’s storage.
// Consumers can use this to retrieve key data without modifying
// the internal storage.
func (dl *Loader) Storage() ReadOnlyStringToBytesStorage {
	return dl.storage
}

// loadAllResources recursively loads all files from the given path
// into the storage, applying the configured KeyID extraction rules.
func (dl *Loader) loadAllResources(path string) error {
	_, err := os.Stat(path)
	if err != nil {
		return err
	}

	keys, err := os.ReadDir(path)
	if err != nil {
		return err
	}

	for _, keyFile := range keys {
		if keyFile.IsDir() {
			err = dl.loadAllResources(filepath.Join(path, keyFile.Name()))
			if err != nil {
				return err
			}
		} else {
			dl.loadSigningKey(fsnotify.Event{
				Name: filepath.Join(path, keyFile.Name()),
				Op:   fsnotify.Write,
			})
		}
	}

	return nil
}

// loadSigningKey loads or removes a single file in response to a file system event.
//
// Supported operations:
//   - Create/Write: load or update file contents
//   - Rename/Remove: remove file from storage
//
// Files that are directories, unreadable, or empty are skipped.
func (dl *Loader) loadSigningKey(event fsnotify.Event) {
	filePath := event.Name

	var keyID string
	switch dl.keyIDType {
	case FileNameWithExtension:
		_, keyID = filepath.Split(filePath)

	case FileNameWithoutExtension:
		_, name := filepath.Split(filePath)
		var found bool
		keyID, found = strings.CutSuffix(name, dl.extension)
		if !found {
			return
		}

	case FileFullPathRelativeToLocation:
		keyID = strings.TrimPrefix(filePath, dl.location)
		if !strings.HasPrefix(keyID, string(os.PathSeparator)) {
			keyID = string(os.PathSeparator) + keyID
		}

	case FileFullPath:
		keyID = filePath
	}

	if event.Op&(fsnotify.Rename|fsnotify.Remove) != 0 {
		dl.storage.Remove(keyID)
		return
	}

	info, err := os.Stat(filePath)
	if err != nil {
		return
	}
	if info.IsDir() {
		return
	}

	keyData, err := os.ReadFile(filePath)
	if err != nil {
		// Skip unreadable files
		return
	}
	if len(keyData) == 0 {
		// Skip empty files
		return
	}

	dl.storage.Store(keyID, keyData)
}
