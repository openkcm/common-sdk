// Package loader keeps an in-memory key–value storage in sync with the files
// in one or more directories.
//
// The loader is meant for resources stored as files that must be reflected in
// memory as soon as they change on disk, for example signing keys or
// certificates in a mounted Kubernetes secret. It loads every matching file
// when it starts, then watches the directories and loads, reloads or removes
// entries as files are created, written, renamed or deleted.
//
// Key concepts
//   - Location: a directory the loader has been told to watch (OnPath,
//     OnPaths). With WatchSubfolders, its subdirectories are included.
//   - Key ID: the key under which the contents of a file are stored, derived
//     from the file path as configured with WithKeyIDType.
//   - Storage: where the contents are stored, a keyvalue.StringToBytesStorage;
//     by default an in-memory one. Read it through Loader.Storage.
//
// How the storage follows the files
//   - Start loads every file in the locations that resolves to a Key ID.
//   - When a file changes, the loader looks at the file as it is now: if it
//     exists it is (re)loaded, if it is gone its key is removed. Files that
//     are empty or cannot be read are skipped and keep their previous
//     contents, so a file caught halfway through being written is picked up
//     by the event of its next write.
//   - When anything else in a location changes, such as a directory or a
//     symbolic link to one, the loader scans the locations again, loads
//     every file and removes the keys of files that are gone. This is how it
//     follows a Kubernetes secret or config map: the kubelet updates those by
//     swapping the ..data symbolic link, without touching the visible files.
//   - Only keys the loader stored itself are ever removed, so a storage shared
//     with other writers keeps their entries.
//   - Files whose name ends with "~" (editor backups) are ignored, and so are
//     entries whose name starts with ".." (the internals of a Kubernetes
//     volume, whose files are reached through the visible symbolic links).
//
// Typical usage
//
//	ldr, err := loader.Create(
//	    loader.OnPath("/etc/keys"),
//	    loader.WithExtension(".pem"),
//	    loader.WithKeyIDType(loader.FileNameWithoutExtension),
//	)
//	if err != nil {
//	    return err
//	}
//
//	err = ldr.Start()
//	if err != nil {
//	    return err
//	}
//	defer ldr.Close()
//
//	key, ok := ldr.Storage().Get("signing")
package loader

import (
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
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
	// FileNameWithoutExtension uses the base file name without the extension
	// set with WithExtension as the Key ID. Files without that extension are
	// ignored.
	//
	// Example:
	//   Path:       /tmp/keys/signing.pem
	//   Extension:  .pem
	//   KeyID:      signing
	FileNameWithoutExtension KeyIDType = 1 << iota

	// FileNameWithExtension uses the base file name including its extension
	// as the Key ID. All files are loaded, whatever their extension.
	//
	// Example:
	//   Path:   /tmp/keys/signing.pem
	//   KeyID:  signing.pem
	FileNameWithExtension

	// FileFullPathRelativeToLocation uses the file path relative to the
	// location as the Key ID. Files directly in the location get their name;
	// files in subdirectories get their relative path with a leading path
	// separator.
	//
	// Example:
	//   Location:  /tmp/keys
	//   Path:      /tmp/keys/key.pem      KeyID: key.pem
	//   Path:      /tmp/keys/sub/key.pem  KeyID: /sub/key.pem
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

// Loader watches directories for resource files and keeps a key–value
// storage of their contents, indexed by Key IDs. See the package
// documentation for how the storage follows the files.
//
// Its methods are safe for concurrent use. The storage is written from the
// loader's own goroutine while it runs, so a storage given with WithStorage
// must be safe for concurrent use as well; the default one is.
type Loader struct {
	paths          []string
	extension      string
	keyIDType      KeyIDType
	recursiveWatch bool
	operations     map[fsnotify.Op]struct{}

	startMu sync.Mutex
	watcher *watcher.Watcher
	storage keyvalue.StringToBytesStorage

	// syncMu serializes the updates of the storage, from events and from
	// scans, and guards the fields below.
	syncMu sync.Mutex
	// locations are the paths of the last Start.
	locations   []string
	locationSet map[string]struct{}
	// owned maps the Key IDs this loader stored to the file each came from.
	owned map[string]string
}

// Option represents a configuration option for Loader.
type Option func(*Loader) error

// OnPath configures the loader to load and watch a single location. See
// Loader.AddPath.
func OnPath(path string) Option {
	return func(l *Loader) error {
		return l.AddPath(path)
	}
}

// OnPaths configures the loader to load and watch several locations at once.
// See Loader.AddPath.
func OnPaths(paths ...string) Option {
	return func(l *Loader) error {
		for _, path := range paths {
			err := l.AddPath(path)
			if err != nil {
				return err
			}
		}

		return nil
	}
}

// WithExtension sets the file extension used by FileNameWithoutExtension:
// only files with this extension are loaded, and the extension is cut from
// their name to form the Key ID. A leading "." is added if missing. An empty
// value means no extension, which is also the default. Other Key ID types
// ignore the extension.
func WithExtension(value string) Option {
	return func(l *Loader) error {
		switch {
		case value == "":
			l.extension = ""
		case !strings.HasPrefix(value, "."):
			l.extension = "." + value
		default:
			l.extension = value
		}

		return nil
	}
}

// WatchSubfolders enables or disables loading and watching the
// subdirectories of the locations, including subdirectories created while
// the loader runs. Symbolic links to directories are not followed.
func WatchSubfolders(enabled bool) Option {
	return func(l *Loader) error {
		l.recursiveWatch = enabled
		return nil
	}
}

// WithKeyIDType configures the KeyID extraction strategy.
// The default KeyIDType is FileFullPath.
func WithKeyIDType(value KeyIDType) Option {
	return func(l *Loader) error {
		l.keyIDType = value
		return nil
	}
}

// WithStorage configures the storage the loader writes to. It must be safe
// for concurrent use. By default, an in-memory storage is used.
//
// If storage is nil, ErrStorageNotSpecified is returned.
func WithStorage(storage keyvalue.StringToBytesStorage) Option {
	return func(l *Loader) error {
		if storage == nil {
			return ErrStorageNotSpecified
		}

		l.storage = storage

		return nil
	}
}

// ForOperations restricts the file system operations the loader reacts to.
// An event is handled if it includes at least one of ops; events can carry
// several operations at once, such as Write|Chmod. The default is Create,
// Write, Rename and Remove.
//
// Without Write, a file must appear with its contents, for example by being
// renamed into the location: a file written in place can be read before its
// contents are written, at its Create event, and its later Write events are
// not followed.
//
// Supported operations are defined in fsnotify.Op:
//
//	fsnotify.Create, fsnotify.Write, fsnotify.Remove, fsnotify.Rename, fsnotify.Chmod
func ForOperations(ops ...fsnotify.Op) Option {
	return func(l *Loader) error {
		operations := make(map[fsnotify.Op]struct{})
		for _, op := range ops {
			operations[op] = struct{}{}
		}

		l.operations = operations

		return nil
	}
}

// Create initializes a Loader with the given options. It does not load or
// watch anything yet; call Start for that.
//
// Example:
//
//	ldr, err := loader.Create(
//	    loader.OnPath("/etc/keys"),
//	    loader.WithExtension(".pem"),
//	    loader.WithKeyIDType(loader.FileNameWithoutExtension),
//	)
//	if err != nil {
//	    log.Fatal(err)
//	}
func Create(opts ...Option) (*Loader, error) {
	l := &Loader{
		paths: make([]string, 0),
		operations: map[fsnotify.Op]struct{}{
			fsnotify.Create: {},
			fsnotify.Write:  {},
			fsnotify.Rename: {},
			fsnotify.Remove: {},
		},
		extension: "",
		keyIDType: FileFullPath,

		startMu: sync.Mutex{},
		storage: keyvalue.NewMemoryStorage[string, []byte](),

		owned: make(map[string]string),
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

// Start watches the locations and loads every file in them into the storage.
// Watching starts before loading, so a change made while loading is not
// missed.
//
// It returns an error if a location cannot be watched or read, for example
// because it no longer exists; nothing is left running in that case. Calling
// Start on a started Loader does nothing. A Loader can be started again after
// Close; the storage is then brought in line with the files again.
func (l *Loader) Start() error {
	l.startMu.Lock()
	defer l.startMu.Unlock()

	if l.isStarted() {
		return nil
	}

	l.setLocations()

	w, err := watcher.Create(
		watcher.OnPaths(l.locations...),
		watcher.WatchSubfolders(l.recursiveWatch),
		watcher.WithEventHandler(l.onEvent),
		watcher.WithErrorEventHandler(l.onError),
	)
	if err != nil {
		return err
	}

	err = w.Start()
	if err != nil {
		return err
	}

	err = l.resync()
	if err != nil {
		_ = w.Close()
		return fmt.Errorf("failed to load resources: %w", err)
	}

	l.watcher = w

	return nil
}

// Close stops watching the locations. The storage keeps its contents.
// Calling Close more than once, or on a Loader that is not started, is safe.
func (l *Loader) Close() error {
	l.startMu.Lock()
	defer l.startMu.Unlock()

	if !l.isStarted() {
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

// IsStarted reports whether the Loader is started.
func (l *Loader) IsStarted() bool {
	l.startMu.Lock()
	defer l.startMu.Unlock()

	return l.isStarted()
}

func (l *Loader) isStarted() bool {
	return l.watcher != nil && l.watcher.IsStarted()
}

// AddPath adds a location: a directory to load and watch. The path is made
// absolute and must exist, otherwise an error is returned.
//
// A location added while the loader is started is loaded and watched from
// the next Start, after Close.
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

	l.startMu.Lock()
	defer l.startMu.Unlock()

	l.paths = append(l.paths, absPath)

	return nil
}

// setLocations makes the configured paths the locations to load and watch.
// The caller must hold startMu.
func (l *Loader) setLocations() {
	l.syncMu.Lock()
	defer l.syncMu.Unlock()

	l.locations = slices.Clone(l.paths)
	l.locationSet = make(map[string]struct{}, len(l.paths))

	for _, path := range l.paths {
		l.locationSet[path] = struct{}{}
	}
}

// onEvent is the watcher's event handler.
func (l *Loader) onEvent(event fsnotify.Event) {
	if !l.handlesAnyOf(event.Op) || isEditorBackup(filepath.Base(event.Name)) {
		return
	}

	l.syncMu.Lock()
	handled := l.refreshFile(event.Name)
	l.syncMu.Unlock()

	if handled {
		return
	}

	err := l.resync()
	if err != nil {
		slog.Warn("failed to reload resources, keeping the previous ones", "path", event.Name, "error", err)
	}
}

// onError is the watcher's error handler.
func (l *Loader) onError(err error) {
	slog.Warn("file system watcher error", slog.String("error", err.Error()))
}

// handlesAnyOf reports whether op includes at least one of the configured
// operations.
func (l *Loader) handlesAnyOf(op fsnotify.Op) bool {
	for handled := range l.operations {
		if op.Has(handled) {
			return true
		}
	}

	return false
}

// refreshFile brings the storage in line with the file at path, as it is now.
// It returns false if path is not a single file it can account for, such as
// a directory, a symbolic link to one or an unknown entry that is gone; the
// locations must then be scanned again. The caller must hold syncMu.
func (l *Loader) refreshFile(path string) bool {
	if isVolumeInternal(filepath.Base(path)) {
		return false
	}

	info, err := os.Stat(path)

	switch {
	case err == nil && info.Mode().IsRegular():
		keyID, ok := l.resolveKeyID(path)
		if ok {
			l.load(keyID, path)
		}

		return true

	case err == nil:
		return false

	case errors.Is(err, fs.ErrNotExist):
		keyID, ok := l.resolveKeyID(path)
		if ok && l.owned[keyID] == path {
			l.remove(keyID)
			return true
		}

		return false

	default:
		slog.Warn("failed to read a resource", "path", path, "error", err)
		return true
	}
}

// resync scans all locations, loads every file that resolves to a Key ID and
// removes the keys this loader stored for files that are gone. If a location
// cannot be read, it returns the error without changing the storage.
func (l *Loader) resync() error {
	l.syncMu.Lock()
	defer l.syncMu.Unlock()

	files := make(map[string]string)

	for _, location := range l.locations {
		err := l.collect(location, files)
		if err != nil {
			return err
		}
	}

	for keyID, path := range files {
		l.load(keyID, path)
	}

	for keyID := range l.owned {
		_, ok := files[keyID]
		if !ok {
			l.remove(keyID)
		}
	}

	return nil
}

// collect adds the files of dir, and with WatchSubfolders of its
// subdirectories, to files, by Key ID.
func (l *Loader) collect(dir string, files map[string]string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		name := entry.Name()
		if isEditorBackup(name) || isVolumeInternal(name) {
			continue
		}

		path := filepath.Join(dir, name)

		if entry.IsDir() {
			if !l.recursiveWatch {
				continue
			}

			err := l.collect(path, files)
			if err != nil {
				return err
			}

			continue
		}

		keyID, ok := l.resolveKeyID(path)
		if ok {
			files[keyID] = path
		}
	}

	return nil
}

// load stores the contents of the file at path under keyID. A file that is
// gone removes the key if it came from that file; a file that is not a
// regular file, is empty or cannot be read is skipped. The caller must hold
// syncMu.
func (l *Loader) load(keyID, path string) {
	info, err := os.Stat(path)
	if errors.Is(err, fs.ErrNotExist) {
		if l.owned[keyID] == path {
			l.remove(keyID)
		}

		return
	}

	if err != nil || !info.Mode().IsRegular() {
		return
	}

	data, err := os.ReadFile(path)
	if err != nil {
		slog.Warn("failed to read a resource", "path", path, "error", err)
		return
	}

	if len(data) == 0 {
		return
	}

	l.storage.Store(keyID, data)
	l.owned[keyID] = path
}

// remove removes keyID from the storage. The caller must hold syncMu.
func (l *Loader) remove(keyID string) {
	l.storage.Remove(keyID)
	delete(l.owned, keyID)
}

// resolveKeyID determines the storage key based on Loader.keyIDType.
// Returns (key, true) if the key is valid, or ("", false) if the file should
// be skipped. The caller must hold syncMu.
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
			_, ok := l.locationSet[dir]
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

// isEditorBackup reports whether name is a backup file of an editor.
func isEditorBackup(name string) bool {
	return strings.HasSuffix(name, "~")
}

// isVolumeInternal reports whether name is one of the internal entries of a
// Kubernetes volume, such as ..data or a timestamped ..2026_... directory.
func isVolumeInternal(name string) bool {
	return strings.HasPrefix(name, "..")
}
