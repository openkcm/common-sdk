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

const (
	Key1        = "key1"
	Key2        = "key2"
	Key3        = "key3"
	PemKey1     = Key1 + ".pem"
	PemKey2     = Key2 + ".pem"
	PemKey3     = Key3 + ".pem"
	PemKey1Data = "pemdata1"
	PemKey2Data = "pemdata2"
	PemKey3Data = "pemdata3"
)

// --- Helpers ---

func newTestLoader(t *testing.T, dir string) (*loader.Loader, *keyvalue.MemoryStorage[string, []byte]) {
	t.Helper()

	st := keyvalue.NewMemoryStorage[string, []byte]()
	l, err := loader.Create(
		loader.OnPath(dir),
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
func TestLoaderKeyIDTypes(t *testing.T) {
	dir := t.TempDir()
	subDir := filepath.Join(dir, "subdir")
	require.NoError(t, os.Mkdir(subDir, 0700))

	// Create files directly under dir for first two key types
	filesRoot := map[string]string{
		PemKey1: "data1",
		PemKey2: "data2",
	}
	for name, content := range filesRoot {
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(content), 0600))
	}

	// Create files in subdir for full path tests
	filesSub := map[string]string{
		PemKey1: "data1",
		PemKey2: "data2",
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
			expected:  []string{Key1, Key2},
		},
		{
			name:      "FileNameWithExtension",
			location:  subDir,
			keyIDType: loader.FileNameWithExtension,
			expected:  []string{PemKey1, PemKey2},
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
				filepath.Join(subDir, PemKey1),
				filepath.Join(subDir, PemKey2),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			st := keyvalue.NewMemoryStorage[string, []byte]()
			l, err := loader.Create(
				loader.OnPath(tt.location),
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

func TestNewLoaderWithOptions(t *testing.T) {
	tmpDir := t.TempDir()

	t.Run("valid loader with storage + extension", func(t *testing.T) {
		st := keyvalue.NewMemoryStorage[string, []byte]()
		l, err := loader.Create(
			loader.OnPath(tmpDir),
			loader.WithStorage(st),
			loader.WithExtension(".pem"),
			loader.WithKeyIDType(loader.FileNameWithExtension),
		)
		require.NoError(t, err)
		require.NotNil(t, l)
	})

	t.Run("nil storage", func(t *testing.T) {
		_, err := loader.Create(
			loader.OnPath(tmpDir),
			loader.WithStorage(nil),
		)
		require.ErrorIs(t, err, loader.ErrStorageNotSpecified)
	})
}

func TestLoadAllloadersPositive(t *testing.T) {
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

func TestLoadSigningKeysLoadsPemFiles(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{Key1: PemKey1Data, Key2: PemKey2Data}
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

func TestLoadSigningKeysErrorOnReadDir(t *testing.T) {
	_, err := loader.Create(
		loader.OnPath("/non/existent/path"),
		loader.WithExtension("pem"),
		loader.WithKeyIDType(loader.FileNameWithoutExtension),
	)
	require.Error(t, err)
}

func TestLoadSigningKeysErrorOnReadFile(t *testing.T) {
	dir := t.TempDir()
	pemPath := filepath.Join(dir, "badkey.pem")
	require.NoError(t, os.WriteFile(pemPath, []byte("data"), 0000)) // no permissions

	l, st := newTestLoader(t, dir)

	startLoader(t, l)
	defer stopLoader(t, l)

	_, ok := st.Get("badkey")
	require.False(t, ok)
}

func TestStartSigningKeysWatcherReloadsOnChange(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{Key1: PemKey1Data, Key2: PemKey2Data}
	createTestPemFiles(t, dir, files)

	l, st := newTestLoader(t, dir)

	startLoader(t, l)
	defer stopLoader(t, l)

	time.Sleep(300 * time.Millisecond) // watcher startup

	// Add new key
	require.NoError(t, os.WriteFile(filepath.Join(dir, PemKey3), []byte(PemKey3Data), 0600))
	time.Sleep(300 * time.Millisecond)

	val, ok := st.Get(Key3)
	require.True(t, ok)
	require.Equal(t, []byte(PemKey3Data), val)

	// Modify key
	require.NoError(t, os.WriteFile(filepath.Join(dir, PemKey1), []byte("pemdata1_modified"), 0600))
	time.Sleep(300 * time.Millisecond)

	val, ok = st.Get(Key1)
	require.True(t, ok)
	require.Equal(t, []byte("pemdata1_modified"), val)

	// Remove key
	require.NoError(t, os.Remove(filepath.Join(dir, PemKey2)))
	time.Sleep(300 * time.Millisecond)

	_, ok = st.Get(Key2)
	require.False(t, ok)
}

func TestStartSigningKeysWatcherNoReloadOnNoChange(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{Key1: PemKey1Data}
	createTestPemFiles(t, dir, files)

	l, st := newTestLoader(t, dir)

	startLoader(t, l)
	defer stopLoader(t, l)

	time.Sleep(300 * time.Millisecond)

	val, ok := st.Get(Key1)
	require.True(t, ok)
	require.Equal(t, []byte(PemKey1Data), val)
}

func TestStartSigningKeysWatcherReloadOnTouch(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{Key1: PemKey1Data}
	createTestPemFiles(t, dir, files)

	l, st := newTestLoader(t, dir)

	startLoader(t, l)
	defer stopLoader(t, l)

	time.Sleep(300 * time.Millisecond)

	// Touch file
	require.NoError(t, os.Chtimes(filepath.Join(dir, PemKey1), time.Now(), time.Now()))
	time.Sleep(300 * time.Millisecond)

	val, ok := st.Get(Key1)
	require.True(t, ok)
	require.Equal(t, []byte(PemKey1Data), val)
}
