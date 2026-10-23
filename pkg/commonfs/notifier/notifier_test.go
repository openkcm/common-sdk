package notifier_test

import (
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/openkcm/common-sdk/pkg/commonfs/notifier"
)

const (
	waitFor = 5 * time.Second
	tick    = 10 * time.Millisecond
	// quiet is how long a test waits to conclude that no further call comes.
	quiet = 300 * time.Millisecond
	// throttle is short so that batches are delivered quickly in tests.
	throttle = 20 * time.Millisecond
)

// --- Helpers ---

// batch is one call of the event handler.
type batch struct {
	path   string
	events []fsnotify.Event
}

// recorder collects the calls of a Notifier's event handler.
type recorder struct {
	mu      sync.Mutex
	batches []batch
}

func (r *recorder) handle(path string, events []fsnotify.Event) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.batches = append(r.batches, batch{path: path, events: slices.Clone(events)})
}

func (r *recorder) calls() int {
	r.mu.Lock()
	defer r.mu.Unlock()

	return len(r.batches)
}

// has reports whether a batch for path holds an event for name with at
// least one of ops.
func (r *recorder) has(path, name string, ops ...fsnotify.Op) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, b := range r.batches {
		if b.path != path {
			continue
		}

		for _, e := range b.events {
			if e.Name == name && slices.ContainsFunc(ops, e.Has) {
				return true
			}
		}
	}

	return false
}

// startNotifier creates and starts a Notifier that records its batches.
func startNotifier(t *testing.T, opts ...notifier.Option) (*notifier.Notifier, *recorder) {
	t.Helper()

	rec := &recorder{}

	opts = append([]notifier.Option{notifier.WithThrottleInterval(throttle)}, opts...)

	n, err := notifier.Create(append(opts, notifier.WithEventHandler(rec.handle))...)
	require.NoError(t, err)
	require.NoError(t, n.Start())
	t.Cleanup(func() { _ = n.Close() })

	return n, rec
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()

	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
}

func requireEventEventually(t *testing.T, rec *recorder, path, name string, ops ...fsnotify.Op) {
	t.Helper()

	require.Eventually(t, func() bool { return rec.has(path, name, ops...) }, waitFor, tick,
		"no %v event for %s in a batch for %s", ops, name, path)
}

// --- Configuration ---

func TestCreateWithInvalidPaths(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	file := filepath.Join(dir, "file")
	writeFile(t, file, "content")

	tests := []struct {
		name string
		opt  notifier.Option
	}{
		{name: "missing path", opt: notifier.OnPath(filepath.Join(dir, "missing"))},
		{name: "missing path among others", opt: notifier.OnPaths(dir, filepath.Join(dir, "missing"))},
		{name: "path below a regular file", opt: notifier.OnPath(filepath.Join(file, "below"))},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			n, err := notifier.Create(tt.opt)
			require.Error(t, err)
			assert.Nil(t, n)
		})
	}
}

func TestCreateIgnoresNilOptions(t *testing.T) {
	t.Parallel()

	n, err := notifier.Create(nil, notifier.OnPath(t.TempDir()), nil)
	require.NoError(t, err)
	assert.False(t, n.IsStarted())
}

// --- Lifecycle ---

func TestStartAndClose(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	n, err := notifier.Create(notifier.OnPath(dir), notifier.WithThrottleInterval(throttle))
	require.NoError(t, err)

	require.NoError(t, n.Close(), "Close before Start must be safe")

	require.NoError(t, n.Start())
	require.NoError(t, n.Start(), "Start on a started notifier must do nothing")
	assert.True(t, n.IsStarted())

	require.NoError(t, n.Close())
	require.NoError(t, n.Close(), "Close must be safe to call twice")
	assert.False(t, n.IsStarted())
}

func TestRestartDeliversEventsAgain(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	n, rec := startNotifier(t, notifier.OnPath(dir))

	require.NoError(t, n.Close())
	require.NoError(t, n.Start())

	file := filepath.Join(dir, "after-restart.txt")
	writeFile(t, file, "hello")
	requireEventEventually(t, rec, dir, file, fsnotify.Create, fsnotify.Write)
}

func TestStartFailsWhenPathIsGone(t *testing.T) {
	t.Parallel()

	dir := filepath.Join(t.TempDir(), "gone")
	require.NoError(t, os.Mkdir(dir, 0o700))

	n, err := notifier.Create(notifier.OnPath(dir))
	require.NoError(t, err)

	require.NoError(t, os.Remove(dir))
	require.Error(t, n.Start())
	assert.False(t, n.IsStarted())
}

// --- Events ---

func TestEventsAreGroupedByWatchedPath(t *testing.T) {
	t.Parallel()

	first, second := t.TempDir(), t.TempDir()
	_, rec := startNotifier(t, notifier.OnPaths(first, second))

	firstFile := filepath.Join(first, "a.txt")
	secondFile := filepath.Join(second, "b.txt")

	writeFile(t, firstFile, "a")
	writeFile(t, secondFile, "b")

	requireEventEventually(t, rec, first, firstFile, fsnotify.Create, fsnotify.Write)
	requireEventEventually(t, rec, second, secondFile, fsnotify.Create, fsnotify.Write)
	assert.False(t, rec.has(first, secondFile, fsnotify.Create, fsnotify.Write))
}

func TestWatchingASingleFile(t *testing.T) {
	t.Parallel()

	file := filepath.Join(t.TempDir(), "single.txt")
	writeFile(t, file, "v1")

	_, rec := startNotifier(t, notifier.OnPath(file))

	writeFile(t, file, "v2")
	requireEventEventually(t, rec, file, file, fsnotify.Write)
}

func TestWatchSubfolders(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	sub := filepath.Join(dir, "a", "b")
	require.NoError(t, os.MkdirAll(sub, 0o700))

	_, rec := startNotifier(t, notifier.OnPath(dir), notifier.WatchSubfolders(true))

	// Events in subdirectories are reported in the batch of the watched
	// directory they are under.
	file := filepath.Join(sub, "nested.txt")
	writeFile(t, file, "hello")
	requireEventEventually(t, rec, dir, file, fsnotify.Create, fsnotify.Write)
}

func TestForOperations(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	file := filepath.Join(dir, "file.txt")
	writeFile(t, file, "hello")

	_, rec := startNotifier(t, notifier.OnPath(dir), notifier.ForOperations(fsnotify.Remove))

	writeFile(t, file, "ignored write")
	time.Sleep(quiet)
	assert.Zero(t, rec.calls(), "a write must not be reported")

	require.NoError(t, os.Remove(file))
	requireEventEventually(t, rec, dir, file, fsnotify.Remove)
}

// TestThrottleGroupsEventsIntoOneCall writes several times in quick
// succession: with no burst allowed, the handler is called once, after the
// throttle interval has passed without a further event.
func TestThrottleGroupsEventsIntoOneCall(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	_, rec := startNotifier(t,
		notifier.OnPath(dir),
		notifier.WithThrottleInterval(quiet),
		notifier.WithBurstNumber(0),
	)

	for i := range 5 {
		writeFile(t, filepath.Join(dir, "f"+strconv.Itoa(i)), "x")
	}

	require.Eventually(t, func() bool { return rec.calls() > 0 }, waitFor, tick)
	time.Sleep(2 * quiet)

	assert.Equal(t, 1, rec.calls())
}

func TestWithEventChainAsHandler(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	events := make(chan notifier.Event, 16)

	n, err := notifier.Create(
		notifier.OnPath(dir),
		notifier.WithThrottleInterval(throttle),
		notifier.WithEventChainAsHandler(events),
	)
	require.NoError(t, err)
	require.NoError(t, n.Start())
	t.Cleanup(func() { _ = n.Close() })

	file := filepath.Join(dir, "file.txt")
	writeFile(t, file, "hello")

	select {
	case e := <-events:
		assert.Equal(t, dir, e.Path)
		assert.True(t, slices.ContainsFunc(e.Events, func(ev fsnotify.Event) bool { return ev.Name == file }))
	case <-time.After(waitFor):
		t.Fatal("no event on the channel")
	}
}

func TestWithEventChainAsHandlerDropsWhenFull(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	events := make(chan notifier.Event) // never read

	n, err := notifier.Create(
		notifier.OnPath(dir),
		notifier.WithThrottleInterval(throttle),
		notifier.WithEventChainAsHandler(events),
	)
	require.NoError(t, err)
	require.NoError(t, n.Start())

	for i := range 5 {
		writeFile(t, filepath.Join(dir, "f"+strconv.Itoa(i)), "x")
		time.Sleep(2 * throttle)
	}

	closed := make(chan error, 1)

	go func() { closed <- n.Close() }()

	select {
	case err := <-closed:
		require.NoError(t, err)
	case <-time.After(waitFor):
		t.Fatal("Close blocked: the handler is stuck on the full channel")
	}
}

func TestWithSimpleHandler(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		burst uint
	}{
		{name: "without burst"},
		{name: "with a burst that delivers at once", burst: 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			dir := t.TempDir()

			var calls atomic.Int64

			n, err := notifier.Create(
				notifier.OnPath(dir),
				notifier.WithThrottleInterval(throttle),
				notifier.WithBurstNumber(tt.burst),
				notifier.WithSimpleHandler(func() { calls.Add(1) }),
			)
			require.NoError(t, err)
			require.NoError(t, n.Start())
			t.Cleanup(func() { _ = n.Close() })

			writeFile(t, filepath.Join(dir, "file.txt"), "hello")

			require.Eventually(t, func() bool { return calls.Load() > 0 }, waitFor, tick)
			time.Sleep(quiet)

			assert.Equal(t, int64(1), calls.Load(), "one change must cause one call")
		})
	}
}

// TestNotifierDeliversEventsWhileFlushTimerRuns writes to a watched directory
// while the throttle timer keeps flushing batches. The timer flush and the
// event intake share the event cache, so run this with -race: it guards
// against the flush touching the cache without holding its lock.
func TestNotifierDeliversEventsWhileFlushTimerRuns(t *testing.T) {
	dir := t.TempDir()

	var calls atomic.Int64

	n, err := notifier.Create(
		notifier.OnPath(dir),
		notifier.WithEventHandler(func(_ string, _ []fsnotify.Event) {
			calls.Add(1)
		}),
		notifier.WithThrottleInterval(time.Millisecond),
		notifier.WithBurstNumber(0),
	)
	require.NoError(t, err)
	require.NoError(t, n.Start())

	t.Cleanup(func() { _ = n.Close() })

	file := filepath.Join(dir, "data")
	deadline := time.Now().Add(300 * time.Millisecond)

	for i := 0; time.Now().Before(deadline); i++ {
		require.NoError(t, os.WriteFile(file, []byte(strconv.Itoa(i)), 0o600))
		time.Sleep(500 * time.Microsecond)
	}

	require.Eventually(t, func() bool { return calls.Load() > 0 }, 2*time.Second, 10*time.Millisecond)
}
