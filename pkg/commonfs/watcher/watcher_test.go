package watcher_test

import (
	"os"
	"path/filepath"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/openkcm/common-sdk/pkg/commonfs/watcher"
)

const (
	waitFor = 5 * time.Second
	tick    = 10 * time.Millisecond
	// quiet is how long a test waits to conclude that no event comes.
	quiet = 300 * time.Millisecond
)

// --- Helpers ---

// recorder collects the events passed to a Watcher's event handler.
type recorder struct {
	mu     sync.Mutex
	events []fsnotify.Event
}

func (r *recorder) handle(e fsnotify.Event) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.events = append(r.events, e)
}

// has reports whether an event for name, with at least one of ops, was
// recorded.
func (r *recorder) has(name string, ops ...fsnotify.Op) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, e := range r.events {
		if e.Name == name && slices.ContainsFunc(ops, e.Has) {
			return true
		}
	}

	return false
}

func (r *recorder) count() int {
	r.mu.Lock()
	defer r.mu.Unlock()

	return len(r.events)
}

// startWatcher creates and starts a Watcher that records its events.
func startWatcher(t *testing.T, opts ...watcher.Option) (*watcher.Watcher, *recorder) {
	t.Helper()

	rec := &recorder{}

	w, err := watcher.Create(append(opts, watcher.WithEventHandler(rec.handle))...)
	require.NoError(t, err)
	require.NoError(t, w.Start())
	t.Cleanup(func() { _ = w.Close() })

	return w, rec
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()

	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
}

func requireEventEventually(t *testing.T, rec *recorder, name string, ops ...fsnotify.Op) {
	t.Helper()

	require.Eventually(t, func() bool { return rec.has(name, ops...) }, waitFor, tick, "no %v event for %s", ops, name)
}

// requireFileWatched writes to path until its Write event arrives. On kqueue
// (macOS, BSD) a file in a watched directory gets its own watch shortly after
// its Create event, and only then are its renames and removals reported; a
// Write event shows that watch exists. On inotify it returns at once.
func requireFileWatched(t *testing.T, rec *recorder, path, content string) {
	t.Helper()

	require.Eventually(t, func() bool {
		writeFile(t, path, content)
		return rec.has(path, fsnotify.Write)
	}, waitFor, 50*time.Millisecond, "no Write event for %s", path)
}

// --- Configuration ---

func TestStartWithoutPaths(t *testing.T) {
	t.Parallel()

	w, err := watcher.Create()
	require.NoError(t, err)

	require.ErrorIs(t, w.Start(), watcher.ErrNoPathsConfigured)
	assert.False(t, w.IsStarted())
}

func TestAddPathNonexistent(t *testing.T) {
	t.Parallel()

	w, err := watcher.Create()
	require.NoError(t, err)

	require.ErrorContains(t, w.AddPath(filepath.Join(t.TempDir(), "does-not-exist")), "path does not exist")

	_, err = watcher.Create(watcher.OnPaths(t.TempDir(), filepath.Join(t.TempDir(), "does-not-exist")))
	require.Error(t, err)
}

// --- Events ---

func TestEventsOfDirectoryEntries(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	_, rec := startWatcher(t, watcher.OnPath(dir))

	file := filepath.Join(dir, "test.txt")
	writeFile(t, file, "hello")
	requireEventEventually(t, rec, file, fsnotify.Create, fsnotify.Write)
	requireFileWatched(t, rec, file, "hello again")

	renamed := filepath.Join(dir, "renamed.txt")
	require.NoError(t, os.Rename(file, renamed))
	requireEventEventually(t, rec, file, fsnotify.Rename)
	requireEventEventually(t, rec, renamed, fsnotify.Create)
	requireFileWatched(t, rec, renamed, "renamed")

	require.NoError(t, os.Remove(renamed))
	requireEventEventually(t, rec, renamed, fsnotify.Remove)
}

func TestWatchesSingleFile(t *testing.T) {
	t.Parallel()

	file := filepath.Join(t.TempDir(), "single.txt")
	writeFile(t, file, "v1")

	_, rec := startWatcher(t, watcher.OnPath(file))

	writeFile(t, file, "v2")
	requireEventEventually(t, rec, file, fsnotify.Write)
}

func TestSubfoldersAreNotWatchedByDefault(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	sub := filepath.Join(dir, "sub")
	require.NoError(t, os.Mkdir(sub, 0o700))

	_, rec := startWatcher(t, watcher.OnPath(dir))

	nested := filepath.Join(sub, "nested.txt")
	writeFile(t, nested, "hello")

	// An entry of the watched directory still reports, to show the watcher
	// runs; the file in the subdirectory does not.
	top := filepath.Join(dir, "top.txt")
	writeFile(t, top, "hello")
	requireEventEventually(t, rec, top, fsnotify.Create, fsnotify.Write)

	time.Sleep(quiet)
	assert.False(t, rec.has(nested, fsnotify.Create, fsnotify.Write))
}

func TestWatchSubfoldersWatchesExistingSubdirectories(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	deep := filepath.Join(dir, "a", "b", "c")
	require.NoError(t, os.MkdirAll(deep, 0o700))

	_, rec := startWatcher(t, watcher.OnPath(dir), watcher.WatchSubfolders(true))

	file := filepath.Join(deep, "deep.txt")
	writeFile(t, file, "hello")
	requireEventEventually(t, rec, file, fsnotify.Create, fsnotify.Write)
}

// TestWatchSubfoldersWatchesNewNestedDirectories creates several levels of
// directories at once while the watcher runs. Every level must end up
// watched, and the Create event of the new directory must reach the handler.
func TestWatchSubfoldersWatchesNewNestedDirectories(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	_, rec := startWatcher(t, watcher.OnPath(dir), watcher.WatchSubfolders(true))

	top := filepath.Join(dir, "x")
	deep := filepath.Join(top, "y", "z")
	require.NoError(t, os.MkdirAll(deep, 0o700))

	requireEventEventually(t, rec, top, fsnotify.Create)

	// The watch on the deepest directory is added asynchronously; write until
	// its file reports.
	file := filepath.Join(deep, "deep.txt")

	require.Eventually(t, func() bool {
		writeFile(t, file, "hello")
		return rec.has(file, fsnotify.Create, fsnotify.Write)
	}, waitFor, 50*time.Millisecond, "the new nested directory is not watched")
}

func TestEventChannelFullDoesNotBlock(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	events := make(chan fsnotify.Event) // never read

	w, err := watcher.Create(watcher.OnPath(dir), watcher.WithEventChainAsHandler(events))
	require.NoError(t, err)
	require.NoError(t, w.Start())

	for i := range 20 {
		writeFile(t, filepath.Join(dir, "f"), string(rune('a'+i)))
	}

	closed := make(chan error, 1)

	go func() { closed <- w.Close() }()

	select {
	case err := <-closed:
		require.NoError(t, err)
	case <-time.After(waitFor):
		t.Fatal("Close blocked: the handler is stuck on the full channel")
	}
}

// --- Lifecycle ---

func TestCloseStopsEventsAndStartResumes(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	w, rec := startWatcher(t, watcher.OnPath(dir))

	require.NoError(t, w.Close())
	require.NoError(t, w.Close(), "Close must be safe to call twice")
	assert.False(t, w.IsStarted())

	whileClosed := filepath.Join(dir, "closed.txt")
	writeFile(t, whileClosed, "hello")
	time.Sleep(quiet)
	assert.Zero(t, rec.count(), "no event may be passed on after Close")

	require.NoError(t, w.Start())
	require.NoError(t, w.Start(), "Start on a started watcher must do nothing")
	assert.True(t, w.IsStarted())

	afterRestart := filepath.Join(dir, "restarted.txt")
	writeFile(t, afterRestart, "hello")
	requireEventEventually(t, rec, afterRestart, fsnotify.Create, fsnotify.Write)
	assert.False(t, rec.has(whileClosed, fsnotify.Create, fsnotify.Write))
}

func TestCloseBeforeStart(t *testing.T) {
	t.Parallel()

	w, err := watcher.Create(watcher.OnPath(t.TempDir()))
	require.NoError(t, err)

	require.NoError(t, w.Close())
	assert.False(t, w.IsStarted())
}

func TestStartFailureLeavesWatcherStopped(t *testing.T) {
	t.Parallel()

	dir := filepath.Join(t.TempDir(), "gone")
	require.NoError(t, os.Mkdir(dir, 0o700))

	w, err := watcher.Create(watcher.OnPath(dir))
	require.NoError(t, err)

	require.NoError(t, os.Remove(dir))
	require.Error(t, w.Start())
	assert.False(t, w.IsStarted())

	require.NoError(t, os.Mkdir(dir, 0o700))
	require.NoError(t, w.Start())
	assert.True(t, w.IsStarted())
	require.NoError(t, w.Close())
}

// TestConcurrentLifecycle starts, closes and inspects one watcher from several
// goroutines at once. Run with -race.
func TestConcurrentLifecycle(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	w, err := watcher.Create(watcher.OnPath(dir), watcher.WithEventHandler(func(fsnotify.Event) {}))
	require.NoError(t, err)
	t.Cleanup(func() { _ = w.Close() })

	var wg sync.WaitGroup

	for i := range 8 {
		wg.Go(func() {
			for range 20 {
				switch i % 3 {
				case 0:
					assert.NoError(t, w.Start())
				case 1:
					assert.NoError(t, w.Close())
				default:
					_ = w.IsStarted()
					_ = w.AddPath(dir)
				}
			}
		})
	}

	wg.Wait()

	require.NoError(t, w.Close())
	assert.False(t, w.IsStarted())
}
