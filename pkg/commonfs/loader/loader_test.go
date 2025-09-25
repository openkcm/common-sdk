package loader_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/openkcm/common-sdk/pkg/commonfs/loader"
	"github.com/openkcm/common-sdk/pkg/storage/keyvalue"
)

func TestNewLoader_WithOptions(t *testing.T) {
	tmpDir := t.TempDir()

	t.Run("valid loader with storage + extension", func(t *testing.T) {
		st := keyvalue.NewMemoryStorage()
		loader, err := loader.Create(tmpDir,
			loader.WithStorage(st),
			loader.WithExtension(".pem"),
			loader.WithKeyIDType(loader.FileNameWithExtension),
		)

		require.NoError(t, err)
		require.NotNil(t, loader)
	})

	t.Run("invalid extension", func(t *testing.T) {
		_, err := loader.Create(tmpDir, loader.WithExtension(""))
		require.ErrorIs(t, err, loader.ErrExtensionIsEmpty)
	})

	t.Run("nil storage", func(t *testing.T) {
		_, err := loader.Create(tmpDir, loader.WithStorage(nil))
		require.ErrorIs(t, err, loader.ErrStorageNotSpecified)
	})
}

func TestLoadAllloaders_Positive(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "key.pem")
	require.NoError(t, os.WriteFile(filePath, []byte("secret"), 0644))

	st := keyvalue.NewMemoryStorage()
	loader, err := loader.Create(tmpDir,
		loader.WithStorage(st),
		loader.WithExtension(".pem"),
		loader.WithKeyIDType(loader.FileNameWithExtension),
	)
	require.NoError(t, err)

	err = loader.StartWatching()
	require.NoError(t, err)

	val, ok := st.Get("key.pem")
	require.True(t, ok)
	require.Equal(t, []byte("secret"), val)
}

func TestStartStopWatching(t *testing.T) {
	tmpDir := t.TempDir()
	st := keyvalue.NewMemoryStorage()

	loader, err := loader.Create(tmpDir,
		loader.WithStorage(st),
		loader.WithExtension(".pem"),
		loader.WithKeyIDType(loader.FileNameWithExtension),
	)
	require.NoError(t, err)

	// Start watching
	err = loader.StartWatching()
	require.NoError(t, err)

	// Add a new file after watcher is running
	filePath := filepath.Join(tmpDir, "watched.pem")
	require.NoError(t, os.WriteFile(filePath, []byte("supersecret"), 0644))

	// Wait until it shows up in storage
	require.Eventually(t, func() bool {
		_, ok := st.Get("watched.pem")
		return ok
	}, time.Second, 50*time.Millisecond)

	// Stop watching
	err = loader.StopWatching()
	require.NoError(t, err)
}

func TestFileRemovalUpdatesStorage(t *testing.T) {
	tmpDir := t.TempDir()
	st := keyvalue.NewMemoryStorage()

	loader, err := loader.Create(tmpDir,
		loader.WithStorage(st),
		loader.WithExtension(".pem"),
		loader.WithKeyIDType(loader.FileNameWithExtension),
	)
	require.NoError(t, err)

	err = loader.StartWatching()
	require.NoError(t, err)

	defer func() {
		err = loader.StopWatching()
		require.NoError(t, err)
	}()

	// Create file
	filePath := filepath.Join(tmpDir, "remove.pem")
	require.NoError(t, os.WriteFile(filePath, []byte("bye"), 0644))

	require.Eventually(t, func() bool {
		_, ok := st.Get("remove.pem")
		return ok
	}, time.Second, 50*time.Millisecond)

	// Remove file
	require.NoError(t, os.Remove(filePath))

	require.Eventually(t, func() bool {
		_, ok := st.Get("remove.pem")
		return !ok
	}, time.Second, 50*time.Millisecond)
}

func createTestPemFiles(t *testing.T, dir string, files map[string]string) {
	t.Helper()

	for name, content := range files {
		pemPath := filepath.Join(dir, name+".pem")
		err := os.WriteFile(pemPath, []byte(content), 0600)
		require.NoError(t, err)
	}
}

func TestLoadSigningKeys_LoadsPemFiles(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		"key1": "pemdata1",
		"key2": "pemdata2",
	}
	createTestPemFiles(t, dir, files)
	// Add a non-pem file and a directory
	nonPem := filepath.Join(dir, "not_a_key.txt")
	err := os.WriteFile(nonPem, []byte("ignoreme"), 0600)
	require.NoError(t, err)
	err = os.Mkdir(filepath.Join(dir, "subdir"), 0755)
	require.NoError(t, err)

	memoryStorage := keyvalue.NewMemoryStorage()
	dl, err := loader.Create(dir,
		loader.WithExtension("pem"),
		loader.WithKeyIDType(loader.FileNameWithoutExtension),
		loader.WithStorage(memoryStorage),
	)
	require.NoError(t, err)

	err = dl.StartWatching()
	require.NoError(t, err)

	defer func() {
		err := dl.StopWatching()
		require.NoError(t, err)
	}()

	for k, v := range files {
		key, ok := memoryStorage.Get(k)
		require.True(t, ok)
		require.Equal(t, []byte(v), key)
	}
	// Non-existent key
	_, ok := memoryStorage.Get("missing")
	require.False(t, ok)
}

func TestLoadSigningKeys_ErrorOnReadDir(t *testing.T) {
	_, err := loader.Create("/non/existent/path",
		loader.WithExtension("pem"),
		loader.WithKeyIDType(loader.FileNameWithoutExtension),
	)
	require.Error(t, err)
}

func TestLoadSigningKeys_ErrorOnReadFile(t *testing.T) {
	dir := t.TempDir()
	pemPath := filepath.Join(dir, "badkey.pem")
	// Create file and remove permissions
	err := os.WriteFile(pemPath, []byte("data"), 0000)
	require.NoError(t, err)

	memoryStorage := keyvalue.NewMemoryStorage()
	dl, err := loader.Create(dir,
		loader.WithExtension("pem"),
		loader.WithKeyIDType(loader.FileNameWithoutExtension),
		loader.WithStorage(memoryStorage),
	)
	require.NoError(t, err)
	err = dl.StartWatching()
	require.NoError(t, err)

	defer func() {
		err := dl.StopWatching()
		require.NoError(t, err)
	}()

	_, ok := memoryStorage.Get("badkey")
	require.False(t, ok) // Key should not be loaded
}

func TestStartSigningKeysWatcher_ReloadsOnChange(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		"key1": "pemdata1",
		"key2": "pemdata2",
	}
	createTestPemFiles(t, dir, files)

	memoryStorage := keyvalue.NewMemoryStorage()
	dl, err := loader.Create(dir,
		loader.WithExtension("pem"),
		loader.WithKeyIDType(loader.FileNameWithoutExtension),
		loader.WithStorage(memoryStorage),
	)
	require.NoError(t, err)

	err = dl.StartWatching()
	require.NoError(t, err)

	defer func(dl *loader.Loader) {
		err = dl.StopWatching()
		require.NoError(t, err)
	}(dl)

	// Wait for watcher to start
	time.Sleep(300 * time.Millisecond)

	// Add a new key
	newKeyName := "key3"
	newKeyContent := "pemdata3"
	err = os.WriteFile(filepath.Join(dir, newKeyName+".pem"), []byte(newKeyContent), 0600)
	require.NoError(t, err)

	// Wait for watcher to reload
	time.Sleep(300 * time.Millisecond)

	key, ok := memoryStorage.Get(newKeyName)
	require.True(t, ok)
	require.Equal(t, []byte(newKeyContent), key)

	// Modify an existing key
	modKeyName := "key1"
	modKeyContent := "pemdata1_modified"
	err = os.WriteFile(filepath.Join(dir, modKeyName+".pem"), []byte(modKeyContent), 0600)
	require.NoError(t, err)

	// Wait for watcher to reload
	time.Sleep(300 * time.Millisecond)

	key, ok = memoryStorage.Get(modKeyName)
	require.True(t, ok)
	require.Equal(t, []byte(modKeyContent), key)

	// Remove a key
	err = os.Remove(filepath.Join(dir, "key2.pem"))
	require.NoError(t, err)

	// Wait for watcher to reload
	time.Sleep(300 * time.Millisecond)

	_, ok = memoryStorage.Get("key2")
	require.False(t, ok)

	// Wait to ensure goroutine exits
	time.Sleep(100 * time.Millisecond)
}

func TestStartSigningKeysWatcher_NoReloadOnNoChange(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		"key1": "pemdata1",
	}
	createTestPemFiles(t, dir, files)

	memoryStorage := keyvalue.NewMemoryStorage()
	dl, err := loader.Create(dir,
		loader.WithExtension("pem"),
		loader.WithKeyIDType(loader.FileNameWithoutExtension),
		loader.WithStorage(memoryStorage),
	)
	require.NoError(t, err)

	err = dl.StartWatching()
	require.NoError(t, err)

	defer func() {
		err := dl.StopWatching()
		require.NoError(t, err)
	}()

	// Wait for watcher to start
	time.Sleep(300 * time.Millisecond)

	// No change, so key should remain the same
	key, ok := memoryStorage.Get("key1")
	require.True(t, ok)
	require.Equal(t, []byte("pemdata1"), key)

	// Wait to ensure goroutine exits
	time.Sleep(100 * time.Millisecond)
}

func TestStartSigningKeysWatcher_ReloadOnTouch(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		"key1": "pemdata1",
	}
	createTestPemFiles(t, dir, files)

	memoryStorage := keyvalue.NewMemoryStorage()
	dl, err := loader.Create(dir,
		loader.WithExtension("pem"),
		loader.WithKeyIDType(loader.FileNameWithoutExtension),
		loader.WithStorage(memoryStorage),
	)
	require.NoError(t, err)

	err = dl.StartWatching()
	require.NoError(t, err)

	defer func(dl *loader.Loader) {
		err = dl.StopWatching()
		require.NoError(t, err)
	}(dl)

	// Wait for watcher to start
	time.Sleep(300 * time.Millisecond)

	// Touch the file (update mod time)
	pemPath := filepath.Join(dir, "key1.pem")
	err = os.Chtimes(pemPath, time.Now(), time.Now())
	require.NoError(t, err)

	// Wait for watcher to reload
	time.Sleep(300 * time.Millisecond)

	key, ok := memoryStorage.Get("key1")
	require.True(t, ok)
	require.Equal(t, []byte("pemdata1"), key)

	// Wait to ensure goroutine exits
	time.Sleep(100 * time.Millisecond)
}
