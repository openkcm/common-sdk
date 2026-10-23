package loader_test

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/openkcm/common-sdk/pkg/commonfs/loader"
	"github.com/openkcm/common-sdk/pkg/storage/keyvalue"
)

const (
	waitFor = 5 * time.Second
	tick    = 10 * time.Millisecond
	// quiet is how long a test waits to conclude that nothing changes.
	quiet = 300 * time.Millisecond
)

// --- Helpers ---

type memoryStorage = keyvalue.MemoryStorage[string, []byte]

// startLoader creates and starts a loader on dir with the given options,
// writing to a new in-memory storage.
func startLoader(t *testing.T, dir string, opts ...loader.Option) (*loader.Loader, *memoryStorage) {
	t.Helper()

	st := keyvalue.NewMemoryStorage[string, []byte]()

	l, err := loader.Create(append([]loader.Option{loader.OnPath(dir), loader.WithStorage(st)}, opts...)...)
	require.NoError(t, err)
	require.NoError(t, l.Start())
	t.Cleanup(func() { _ = l.Close() })

	return l, st
}

// pemLoaderOptions load the .pem files of a directory under their name
// without the extension, the way cmk loads its signing keys.
func pemLoaderOptions() []loader.Option {
	return []loader.Option{
		loader.WithExtension("pem"),
		loader.WithKeyIDType(loader.FileNameWithoutExtension),
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()

	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
}

func requireValueEventually(t *testing.T, st *memoryStorage, key, want string) {
	t.Helper()

	require.EventuallyWithT(t, func(c *assert.CollectT) {
		got, ok := st.Get(key)
		assert.True(c, ok, "key %q not loaded", key)
		assert.Equal(c, want, string(got))
	}, waitFor, tick)
}

func requireAbsentEventually(t *testing.T, st *memoryStorage, key string) {
	t.Helper()

	require.Eventually(t, func() bool {
		_, ok := st.Get(key)
		return !ok
	}, waitFor, tick, "key %q not removed", key)
}

func requireValue(t *testing.T, st *memoryStorage, key, want string) {
	t.Helper()

	got, ok := st.Get(key)
	require.True(t, ok, "key %q not loaded", key)
	require.Equal(t, want, string(got))
}

// --- Construction ---

func TestCreateOptions(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	t.Run("valid options", func(t *testing.T) {
		t.Parallel()

		l, err := loader.Create(
			loader.OnPath(dir),
			loader.WithStorage(keyvalue.NewMemoryStorage[string, []byte]()),
			loader.WithExtension(".pem"),
			loader.WithKeyIDType(loader.FileNameWithExtension),
			loader.WatchSubfolders(true),
			loader.ForOperations(fsnotify.Create),
			nil,
		)
		require.NoError(t, err)
		assert.NotNil(t, l)
		assert.False(t, l.IsStarted())
	})

	t.Run("nil storage", func(t *testing.T) {
		t.Parallel()

		_, err := loader.Create(loader.OnPath(dir), loader.WithStorage(nil))
		require.ErrorIs(t, err, loader.ErrStorageNotSpecified)
	})

	t.Run("nonexistent location", func(t *testing.T) {
		t.Parallel()

		_, err := loader.Create(loader.OnPaths(dir, filepath.Join(dir, "does-not-exist")))
		require.ErrorContains(t, err, "path does not exist")
	})
}

// --- Key IDs ---

func TestKeyIDTypes(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	sub := filepath.Join(dir, "sub")
	require.NoError(t, os.Mkdir(sub, 0o700))
	writeFile(t, filepath.Join(dir, "top.pem"), "top")
	writeFile(t, filepath.Join(dir, "notes.txt"), "notes")
	writeFile(t, filepath.Join(sub, "nested.pem"), "nested")

	tests := []struct {
		name string
		opts []loader.Option
		want map[string]string
	}{
		{
			name: "FileNameWithoutExtension loads only files with the extension",
			opts: []loader.Option{loader.WithKeyIDType(loader.FileNameWithoutExtension), loader.WithExtension("pem")},
			want: map[string]string{"top": "top"},
		},
		{
			name: "FileNameWithoutExtension without an extension loads everything",
			opts: []loader.Option{loader.WithKeyIDType(loader.FileNameWithoutExtension), loader.WithExtension("")},
			want: map[string]string{"top.pem": "top", "notes.txt": "notes"},
		},
		{
			name: "FileNameWithExtension ignores the extension option",
			opts: []loader.Option{loader.WithKeyIDType(loader.FileNameWithExtension), loader.WithExtension("pem")},
			want: map[string]string{"top.pem": "top", "notes.txt": "notes"},
		},
		{
			name: "FileFullPathRelativeToLocation with subfolders",
			opts: []loader.Option{loader.WithKeyIDType(loader.FileFullPathRelativeToLocation), loader.WatchSubfolders(true)},
			want: map[string]string{"top.pem": "top", "notes.txt": "notes", "/sub/nested.pem": "nested"},
		},
		{
			name: "FileFullPath is the default",
			want: map[string]string{
				filepath.Join(dir, "top.pem"):   "top",
				filepath.Join(dir, "notes.txt"): "notes",
			},
		},
		{
			name: "FileFullPath with subfolders",
			opts: []loader.Option{loader.WithKeyIDType(loader.FileFullPath), loader.WatchSubfolders(true)},
			want: map[string]string{
				filepath.Join(dir, "top.pem"):    "top",
				filepath.Join(dir, "notes.txt"):  "notes",
				filepath.Join(sub, "nested.pem"): "nested",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, st := startLoader(t, dir, tt.opts...)

			assert.ElementsMatch(t, keys(tt.want), st.List())

			for key, want := range tt.want {
				requireValue(t, st, key, want)
			}
		})
	}
}

func keys(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}

	return out
}

// --- Initial load ---

func TestStartLoadsFiles(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "key1.pem"), "pemdata1")
	writeFile(t, filepath.Join(dir, "key2.pem"), "pemdata2")
	writeFile(t, filepath.Join(dir, "not_a_key.txt"), "ignored")
	writeFile(t, filepath.Join(dir, "empty.pem"), "")
	require.NoError(t, os.Mkdir(filepath.Join(dir, "subdir.pem"), 0o700))

	_, st := startLoader(t, dir, pemLoaderOptions()...)

	assert.ElementsMatch(t, []string{"key1", "key2"}, st.List())
	requireValue(t, st, "key1", "pemdata1")
	requireValue(t, st, "key2", "pemdata2")
}

func TestStartSkipsUnreadableFiles(t *testing.T) {
	t.Parallel()

	if os.Geteuid() == 0 {
		t.Skip("root can read files without permissions")
	}

	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "badkey.pem"), []byte("data"), 0o000))
	writeFile(t, filepath.Join(dir, "goodkey.pem"), "data")

	_, st := startLoader(t, dir, pemLoaderOptions()...)

	assert.Equal(t, []string{"goodkey"}, st.List())
}

func TestStartFailsWhenLocationIsGone(t *testing.T) {
	t.Parallel()

	dir := filepath.Join(t.TempDir(), "keys")
	require.NoError(t, os.Mkdir(dir, 0o700))

	l, err := loader.Create(loader.OnPath(dir))
	require.NoError(t, err)

	require.NoError(t, os.Remove(dir))
	require.Error(t, l.Start())
	assert.False(t, l.IsStarted())
}

// --- Following changes ---

func TestFollowsFileChanges(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "key1.pem"), "pemdata1")
	writeFile(t, filepath.Join(dir, "key2.pem"), "pemdata2")

	_, st := startLoader(t, dir, pemLoaderOptions()...)

	writeFile(t, filepath.Join(dir, "key3.pem"), "pemdata3")
	requireValueEventually(t, st, "key3", "pemdata3")

	writeFile(t, filepath.Join(dir, "key1.pem"), "pemdata1-modified")
	requireValueEventually(t, st, "key1", "pemdata1-modified")

	require.NoError(t, os.Remove(filepath.Join(dir, "key2.pem")))
	requireAbsentEventually(t, st, "key2")

	require.NoError(t, os.Rename(filepath.Join(dir, "key3.pem"), filepath.Join(dir, "key4.pem")))
	requireAbsentEventually(t, st, "key3")
	requireValueEventually(t, st, "key4", "pemdata3")

	requireValue(t, st, "key1", "pemdata1-modified")
}

func TestTouchKeepsContents(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	file := filepath.Join(dir, "key1.pem")
	writeFile(t, file, "pemdata1")

	_, st := startLoader(t, dir, pemLoaderOptions()...)

	require.NoError(t, os.Chtimes(file, time.Now(), time.Now()))
	time.Sleep(quiet)

	requireValue(t, st, "key1", "pemdata1")
}

func TestEmptyFileKeepsPreviousContents(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	file := filepath.Join(dir, "key1.pem")
	writeFile(t, file, "pemdata1")

	_, st := startLoader(t, dir, pemLoaderOptions()...)

	// Truncating is what a writer does first; the contents come with the
	// next write.
	require.NoError(t, os.Truncate(file, 0))
	time.Sleep(quiet)
	requireValue(t, st, "key1", "pemdata1")

	writeFile(t, file, "pemdata2")
	requireValueEventually(t, st, "key1", "pemdata2")
}

// TestEditorBackupFilesAreIgnored covers the backup an editor such as vim
// writes next to a file it saves, and deletes afterwards. Neither may change
// the key of the file itself.
func TestEditorBackupFilesAreIgnored(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	file := filepath.Join(dir, "keyId")
	writeFile(t, file, "key-1")

	_, st := startLoader(t, dir, loader.WithKeyIDType(loader.FileNameWithExtension))

	backup := file + "~"
	writeFile(t, backup, "backup")
	writeFile(t, file, "key-2")
	requireValueEventually(t, st, "keyId", "key-2")

	require.NoError(t, os.Remove(backup))
	time.Sleep(quiet)

	requireValue(t, st, "keyId", "key-2")
	assert.Equal(t, []string{"keyId"}, st.List())
}

func TestForOperations(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	_, st := startLoader(t, dir, append(pemLoaderOptions(), loader.ForOperations(fsnotify.Create))...)

	file := filepath.Join(dir, "key1.pem")
	writeFile(t, file, "pemdata1")
	requireValueEventually(t, st, "key1", "pemdata1")

	// Writes and removals are not followed.
	writeFile(t, file, "pemdata2")
	require.NoError(t, os.Remove(file))
	time.Sleep(quiet)

	requireValue(t, st, "key1", "pemdata1")
}

// --- Subfolders ---

func TestSubfoldersAreNotLoadedByDefault(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	sub := filepath.Join(dir, "sub")
	require.NoError(t, os.Mkdir(sub, 0o700))
	writeFile(t, filepath.Join(sub, "nested.pem"), "nested")

	_, st := startLoader(t, dir, loader.WithKeyIDType(loader.FileFullPathRelativeToLocation))

	writeFile(t, filepath.Join(sub, "late.pem"), "late")
	writeFile(t, filepath.Join(dir, "top.pem"), "top")
	requireValueEventually(t, st, "top.pem", "top")

	time.Sleep(quiet)
	assert.Equal(t, []string{"top.pem"}, st.List())
}

func TestSubfoldersCreatedWhileRunning(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	_, st := startLoader(t, dir,
		loader.WithKeyIDType(loader.FileFullPathRelativeToLocation),
		loader.WatchSubfolders(true),
	)

	deep := filepath.Join(dir, "a", "b")
	require.NoError(t, os.MkdirAll(deep, 0o700))
	writeFile(t, filepath.Join(deep, "deep.pem"), "deep")

	requireValueEventually(t, st, "/a/b/deep.pem", "deep")
}

// TestSubfolderMovedInAndOut moves a directory that already holds files into
// the location and out again. Its files produce no events of their own, so
// the loader must find them through the directory's events.
func TestSubfolderMovedInAndOut(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	_, st := startLoader(t, dir,
		loader.WithKeyIDType(loader.FileFullPathRelativeToLocation),
		loader.WatchSubfolders(true),
	)

	outside := filepath.Join(t.TempDir(), "batch")
	require.NoError(t, os.MkdirAll(filepath.Join(outside, "inner"), 0o700))
	writeFile(t, filepath.Join(outside, "a.pem"), "a")
	writeFile(t, filepath.Join(outside, "inner", "b.pem"), "b")

	inside := filepath.Join(dir, "batch")
	require.NoError(t, os.Rename(outside, inside))

	requireValueEventually(t, st, "/batch/a.pem", "a")
	requireValueEventually(t, st, "/batch/inner/b.pem", "b")

	require.NoError(t, os.Rename(inside, filepath.Join(t.TempDir(), "gone")))

	requireAbsentEventually(t, st, "/batch/a.pem")
	requireAbsentEventually(t, st, "/batch/inner/b.pem")
}

// --- Kubernetes volumes ---

// kubernetesVolume lays out a directory the way the kubelet does for a
// secret or config map: the files are symbolic links through ..data, which
// points to a timestamped directory and is swapped atomically on update.
type kubernetesVolume struct {
	t       *testing.T
	dir     string
	version int
	current string
	visible map[string]bool
}

func newKubernetesVolume(t *testing.T, files map[string]string) *kubernetesVolume {
	t.Helper()

	v := &kubernetesVolume{t: t, dir: t.TempDir(), visible: map[string]bool{}}
	v.update(files)

	return v
}

// update writes a new version of the files in the kubelet's order: a new
// timestamped directory, the swap of ..data, the visible links of added and
// removed files, and the removal of the previous directory.
func (v *kubernetesVolume) update(files map[string]string) {
	v.t.Helper()

	v.version++
	versionDir := filepath.Join(v.dir, "..2026_v"+string(rune('0'+v.version)))
	require.NoError(v.t, os.Mkdir(versionDir, 0o700))

	for name, content := range files {
		writeFile(v.t, filepath.Join(versionDir, name), content)
	}

	tmpLink := filepath.Join(v.dir, "..data_tmp")
	require.NoError(v.t, os.Symlink(filepath.Base(versionDir), tmpLink))
	require.NoError(v.t, os.Rename(tmpLink, filepath.Join(v.dir, "..data")))

	for name := range files {
		if !v.visible[name] {
			require.NoError(v.t, os.Symlink(filepath.Join("..data", name), filepath.Join(v.dir, name)))
			v.visible[name] = true
		}
	}

	for name := range v.visible {
		if _, ok := files[name]; !ok {
			require.NoError(v.t, os.Remove(filepath.Join(v.dir, name)))
			delete(v.visible, name)
		}
	}

	if v.current != "" {
		require.NoError(v.t, os.RemoveAll(v.current))
	}

	v.current = versionDir
}

// TestKubernetesVolumeUpdates follows a mounted secret through updates that
// change a file in place, add a file and remove one, the way extauthz and
// cmk read their signing keys. A key written to the shared storage by
// someone else must survive.
func TestKubernetesVolumeUpdates(t *testing.T) {
	t.Parallel()

	vol := newKubernetesVolume(t, map[string]string{"keyId": "key-1", "key-1.pem": "pem-1"})

	st := keyvalue.NewMemoryStorage[string, []byte]()
	st.Store("foreign", []byte("not from the loader"))

	l, err := loader.Create(
		loader.OnPath(vol.dir),
		loader.WithKeyIDType(loader.FileNameWithExtension),
		loader.WithStorage(st),
	)
	require.NoError(t, err)
	require.NoError(t, l.Start())
	t.Cleanup(func() { _ = l.Close() })

	assert.ElementsMatch(t, []string{"keyId", "key-1.pem", "foreign"}, st.List())
	requireValue(t, st, "keyId", "key-1")

	vol.update(map[string]string{"keyId": "key-2", "key-1.pem": "pem-1", "key-2.pem": "pem-2"})
	requireValueEventually(t, st, "keyId", "key-2")
	requireValueEventually(t, st, "key-2.pem", "pem-2")

	vol.update(map[string]string{"keyId": "key-3", "key-2.pem": "pem-2"})
	requireValueEventually(t, st, "keyId", "key-3")
	requireAbsentEventually(t, st, "key-1.pem")

	assert.ElementsMatch(t, []string{"keyId", "key-2.pem", "foreign"}, st.List())
}

// --- Lifecycle ---

func TestCloseAndRestart(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "key1.pem"), "pemdata1")
	writeFile(t, filepath.Join(dir, "key2.pem"), "pemdata2")

	l, st := startLoader(t, dir, pemLoaderOptions()...)

	require.NoError(t, l.Close())
	require.NoError(t, l.Close(), "Close must be safe to call twice")
	assert.False(t, l.IsStarted())

	// While closed, the storage keeps its contents and does not follow.
	writeFile(t, filepath.Join(dir, "key1.pem"), "pemdata1-modified")
	require.NoError(t, os.Remove(filepath.Join(dir, "key2.pem")))
	writeFile(t, filepath.Join(dir, "key3.pem"), "pemdata3")
	time.Sleep(quiet)

	assert.ElementsMatch(t, []string{"key1", "key2"}, st.List())
	requireValue(t, st, "key1", "pemdata1")

	// Starting again brings the storage in line, and follows changes again.
	require.NoError(t, l.Start())
	require.NoError(t, l.Start(), "Start on a started loader must do nothing")
	assert.True(t, l.IsStarted())

	assert.ElementsMatch(t, []string{"key1", "key3"}, st.List())
	requireValue(t, st, "key1", "pemdata1-modified")

	writeFile(t, filepath.Join(dir, "key4.pem"), "pemdata4")
	requireValueEventually(t, st, "key4", "pemdata4")
}

// TestConcurrentLifecycle starts, closes and inspects one loader from several
// goroutines while its files change. Run with -race.
func TestConcurrentLifecycle(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "key1.pem"), "pemdata1")

	st := keyvalue.NewMemoryStorage[string, []byte]()

	l, err := loader.Create(append(pemLoaderOptions(), loader.OnPath(dir), loader.WithStorage(st))...)
	require.NoError(t, err)
	t.Cleanup(func() { _ = l.Close() })

	var wg sync.WaitGroup

	for i := range 8 {
		wg.Go(func() {
			for j := range 20 {
				switch i % 4 {
				case 0:
					assert.NoError(t, l.Start())
				case 1:
					assert.NoError(t, l.Close())
				case 2:
					writeFile(t, filepath.Join(dir, "key1.pem"), string(rune('a'+j)))
				default:
					_ = l.IsStarted()
					_, _ = l.Storage().Get("key1")
				}
			}
		})
	}

	wg.Wait()

	require.NoError(t, l.Start())
	writeFile(t, filepath.Join(dir, "key1.pem"), "final")
	requireValueEventually(t, st, "key1", "final")
}
