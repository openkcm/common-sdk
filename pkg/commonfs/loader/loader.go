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

type KeyIDType uint32

const (
	// FileNameWithoutExtension - Key ID Type for which the key id is formated based on the file name, excluding the extension.
	//
	//	FilePath: /tmp/xxx/.../ss/key.pem
	//  KeyID -> key
	//
	FileNameWithoutExtension KeyIDType = 1 << iota

	// FileNameWithExtension - Key ID Type for which the key id is formated based on the file name, including the extension.
	//
	//	FilePath: /tmp/xxx/.../ss/key.pem
	//  KeyID -> key.pem
	//
	FileNameWithExtension

	// FileFullPathRelativeToLocation - Key ID Type for which the key id will be the subpath by excluding the given location.
	//
	//	FilePath: /tmp/xxx/ss/key.pem
	//  Given Location: /tmp/xxx
	//  KeyID -> /ss/key.pem
	//
	FileFullPathRelativeToLocation

	// FileFullPath - Key ID Type for which the key id will be the full path.
	//
	//	FilePath: /tmp/xxx/ss/key.pem
	//  KeyID -> /tmp/xxx/ss/key.pem
	//
	FileFullPath
)

var (
	ErrExtensionIsEmpty    = errors.New("extension is empty")
	ErrStorageNotSpecified = errors.New("storage not specified")
)

type Loader struct {
	location  string
	extension string
	keyIDType KeyIDType

	watcher *watcher.NotifyWrapper
	storage keyvalue.Storage[string, []byte]
}

type Option func(*Loader) error

func WithExtension(value string) Option {
	return func(w *Loader) error {
		if value == "" {
			return ErrExtensionIsEmpty
		}

		if !strings.HasPrefix(value, ".") {
			w.extension = "." + value
		}

		return nil
	}
}
func WithKeyIDType(value KeyIDType) Option {
	return func(w *Loader) error {
		w.keyIDType = value
		return nil
	}
}

func WithStorage(storage keyvalue.Storage[string, []byte]) Option {
	return func(w *Loader) error {
		if storage == nil {
			return ErrStorageNotSpecified
		}

		w.storage = storage

		return nil
	}
}

func Create(location string, opts ...Option) (*Loader, error) {
	dl := &Loader{
		location:  location,
		extension: "",
		keyIDType: FileFullPath,
		storage:   keyvalue.NewMemoryStorage(),
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

	// Apply the options
	for _, opt := range opts {
		if opt != nil {
			err := opt(dl)
			if err != nil {
				return nil, err
			}
		}
	}

	return dl, nil
}

func (dl *Loader) onEvent(event fsnotify.Event) {
	if event.Op&(fsnotify.Create|fsnotify.Write|fsnotify.Remove|fsnotify.Rename) != 0 {
		dl.loadSigningKey(event)
	}
}

func (dl *Loader) onError(err error) {
	slog.Error("failed to load signing keys", slog.String("error", err.Error()))
}

// StartWatching starts a watcher on the given directory.
func (dl *Loader) StartWatching() error {
	errStart := dl.watcher.Start()

	err := dl.loadAllResources(dl.location)
	if err != nil {
		return fmt.Errorf("failed to load signing keys %w", err)
	}

	return errStart
}

// StopWatching stop a watcher on the given directory.
func (dl *Loader) StopWatching() error {
	return dl.watcher.Close()
}

// loadAllResources loads signing keys from the in config specified directory.
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
		// skip files that cannot be read
		// this allows partial loading of keys
		return
	}

	if len(keyData) == 0 {
		// skip on no data
		return
	}

	dl.storage.Store(keyID, keyData)
}
