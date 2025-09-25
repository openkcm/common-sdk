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

// --- Helpers ---

func newTestLoader(t *testing.T, dir string) (*loader.Loader, *keyvalue.MemoryStorage) {
	t.Helper()
	st := keyvalue.NewMemoryStorage()
	l, err := loader.Create(dir,
		loader.WithStorage(st),
		loader.WithExtension("pem"),
		loader.WithKeyIDType(loader.FileNameWithoutExtension),
	)
	require.NoError(t, err)
	return l, st
}

func startLoader(t *testing.T, l *loader.Loader) {
	t.Helper()
	require.NoError(t, l.StartWatching())
}

func stopLoader(t *testing.T, l *loader.Loader) {
	t.Helper()
	require.NoError(t, l.StopWatching())
}

func createTestPemFiles(t *testing.T, dir string, files map[string]string) {
	t.Helper()
	for name, content := range files {
		pemPath := filepath.Join(dir, name+".pem")
		require.NoError(t, os.WriteFile(pemPath, []byte(content), 0600))
	}
}

// --- Tests ---
func TestLoader_KeyIDTypes(t *testing.T) {
	dir := t.TempDir()
	subDir := filepath.Join(dir, "subdir")
	require.NoError(t, os.Mkdir(subDir, 0700))

	// Create files directly under dir for first two key types
	filesRoot := map[string]string{
		"key1.pem": "data1",
		"key2.pem": "data2",
	}
	for name, content := range filesRoot {
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(content), 0600))
	}

	// Create files in subdir for full path tests
	filesSub := map[string]string{
		"key1.pem": "data1",
		"key2.pem": "data2",
	}
	for name, content := range filesSub {
		require.NoError(t, os.WriteFile(filepath.Join(subDir, name), []byte(content), 0600))
	}

	tests := []struct {
		name      string
		keyIDType loader.KeyIDType
		expected  []string
		location  string
	}{
		{
			name:      "FileNameWithoutExtension",
			location:  subDir,
			keyIDType: loader.FileNameWithoutExtension,
			expected:  []string{"key1", "key2"},
		},
		{
			name:      "FileNameWithExtension",
			location:  subDir,
			keyIDType: loader.FileNameWithExtension,
			expected:  []string{"key1.pem", "key2.pem"},
		},
		{
			name:      "FileFullPathRelativeToLocation",
			location:  dir,
			keyIDType: loader.FileFullPathRelativeToLocation,
			expected:  []string{"/subdir/key1.pem", "/subdir/key2.pem"},
		},
		{
			name:      "FileFullPath",
			location:  subDir,
			keyIDType: loader.FileFullPath,
			expected: []string{
				filepath.Join(subDir, "key1.pem"),
				filepath.Join(subDir, "key2.pem"),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			st := keyvalue.NewMemoryStorage()
			l, err := loader.Create(tt.location,
				loader.WithStorage(st),
				loader.WithExtension("pem"),
				loader.WithKeyIDType(tt.keyIDType),
			)
			require.NoError(t, err)

			startLoader(t, l)
			defer stopLoader(t, l)

			time.Sleep(200 * time.Millisecond)

			for _, key := range tt.expected {
				val, ok := st.Get(key)
				require.True(t, ok, "expected key %s to exist", key)
				require.Contains(t, []string{"data1", "data2"}, string(val))
			}
		})
	}
}

func TestNewLoader_WithOptions(t *testing.T) {
	tmpDir := t.TempDir()

	t.Run("valid loader with storage + extension", func(t *testing.T) {
		st := keyvalue.NewMemoryStorage()
		l, err := loader.Create(tmpDir,
			loader.WithStorage(st),
			loader.WithExtension(".pem"),
			loader.WithKeyIDType(loader.FileNameWithExtension),
		)
		require.NoError(t, err)
		require.NotNil(t, l)
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
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "key.pem"), []byte("secret"), 0644))

	l, st := newTestLoader(t, dir)
	startLoader(t, l)
	defer stopLoader(t, l)

	val, ok := st.Get("key")
	require.True(t, ok)
	require.Equal(t, []byte("secret"), val)
}

func TestStartStopWatching(t *testing.T) {
	dir := t.TempDir()
	l, st := newTestLoader(t, dir)
	startLoader(t, l)
	defer stopLoader(t, l)

	require.NoError(t, os.WriteFile(filepath.Join(dir, "watched.pem"), []byte("supersecret"), 0644))

	require.Eventually(t, func() bool {
		_, ok := st.Get("watched")
		return ok
	}, time.Second, 50*time.Millisecond)
}

func TestFileRemovalUpdatesStorage(t *testing.T) {
	dir := t.TempDir()
	l, st := newTestLoader(t, dir)
	startLoader(t, l)
	defer stopLoader(t, l)

	file := filepath.Join(dir, "remove.pem")
	require.NoError(t, os.WriteFile(file, []byte("bye"), 0600))

	require.Eventually(t, func() bool {
		_, ok := st.Get("remove")
		return ok
	}, time.Second, 50*time.Millisecond)

	require.NoError(t, os.Remove(file))

	require.Eventually(t, func() bool {
		_, ok := st.Get("remove")
		return !ok
	}, time.Second, 50*time.Millisecond)
}

func TestLoadSigningKeys_LoadsPemFiles(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{"key1": "pemdata1", "key2": "pemdata2"}
	createTestPemFiles(t, dir, files)

	// Add a non-pem file and a directory
	require.NoError(t, os.WriteFile(filepath.Join(dir, "not_a_key.txt"), []byte("ignoreme"), 0600))
	require.NoError(t, os.Mkdir(filepath.Join(dir, "subdir"), 0755))

	l, st := newTestLoader(t, dir)
	startLoader(t, l)
	defer stopLoader(t, l)

	for k, v := range files {
		val, ok := st.Get(k)
		require.True(t, ok)
		require.Equal(t, []byte(v), val)
	}

	_, ok := st.Get("missing")
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
	require.NoError(t, os.WriteFile(pemPath, []byte("data"), 0000)) // no permissions

	l, st := newTestLoader(t, dir)
	startLoader(t, l)
	defer stopLoader(t, l)

	_, ok := st.Get("badkey")
	require.False(t, ok)
}

func TestStartSigningKeysWatcher_ReloadsOnChange(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{"key1": "pemdata1", "key2": "pemdata2"}
	createTestPemFiles(t, dir, files)

	l, st := newTestLoader(t, dir)
	startLoader(t, l)
	defer stopLoader(t, l)

	time.Sleep(300 * time.Millisecond) // watcher startup

	// Add new key
	require.NoError(t, os.WriteFile(filepath.Join(dir, "key3.pem"), []byte("pemdata3"), 0600))
	time.Sleep(300 * time.Millisecond)
	val, ok := st.Get("key3")
	require.True(t, ok)
	require.Equal(t, []byte("pemdata3"), val)

	// Modify key
	require.NoError(t, os.WriteFile(filepath.Join(dir, "key1.pem"), []byte("pemdata1_modified"), 0600))
	time.Sleep(300 * time.Millisecond)
	val, ok = st.Get("key1")
	require.True(t, ok)
	require.Equal(t, []byte("pemdata1_modified"), val)

	// Remove key
	require.NoError(t, os.Remove(filepath.Join(dir, "key2.pem")))
	time.Sleep(300 * time.Millisecond)
	_, ok = st.Get("key2")
	require.False(t, ok)
}

func TestStartSigningKeysWatcher_NoReloadOnNoChange(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{"key1": "pemdata1"}
	createTestPemFiles(t, dir, files)

	l, st := newTestLoader(t, dir)
	startLoader(t, l)
	defer stopLoader(t, l)

	time.Sleep(300 * time.Millisecond)

	val, ok := st.Get("key1")
	require.True(t, ok)
	require.Equal(t, []byte("pemdata1"), val)
}

func TestStartSigningKeysWatcher_ReloadOnTouch(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{"key1": "pemdata1"}
	createTestPemFiles(t, dir, files)

	l, st := newTestLoader(t, dir)
	startLoader(t, l)
	defer stopLoader(t, l)

	time.Sleep(300 * time.Millisecond)

	// Touch file
	require.NoError(t, os.Chtimes(filepath.Join(dir, "key1.pem"), time.Now(), time.Now()))
	time.Sleep(300 * time.Millisecond)

	val, ok := st.Get("key1")
	require.True(t, ok)
	require.Equal(t, []byte("pemdata1"), val)
}
