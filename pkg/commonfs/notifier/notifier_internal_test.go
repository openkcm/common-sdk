package notifier

import (
	"errors"
	"path/filepath"
	"slices"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestOnEventAcceptsCombinedOperations covers events carrying several
// operations at once. The kqueue backend (macOS, BSD) merges the flags of
// one kernel event, so a truncate followed by a write can arrive as a single
// Write|Chmod event; it must not be dropped for not being exactly Write.
func TestOnEventAcceptsCombinedOperations(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		op   fsnotify.Op
		want bool
	}{
		{name: "single watched operation", op: fsnotify.Write, want: true},
		{name: "watched operation combined with another", op: fsnotify.Write | fsnotify.Chmod, want: true},
		{name: "only operations that are not watched", op: fsnotify.Chmod},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			dir := t.TempDir()

			var calls atomic.Int64

			n, err := Create(
				OnPath(dir),
				WithEventHandler(func(_ string, _ []fsnotify.Event) { calls.Add(1) }),
				WithThrottleInterval(time.Millisecond),
			)
			require.NoError(t, err)

			n.onEvent(fsnotify.Event{Name: filepath.Join(dir, "file"), Op: tt.op})

			if tt.want {
				require.Eventually(t, func() bool { return calls.Load() == 1 }, time.Second, time.Millisecond)
				return
			}

			time.Sleep(50 * time.Millisecond)
			assert.Zero(t, calls.Load())
		})
	}
}

// No option sets the error handler, so these tests set it directly.

func TestErrorsAreBatchedAndDelivered(t *testing.T) {
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

			n, err := Create(OnPath(t.TempDir()), WithThrottleInterval(10*time.Millisecond), WithBurstNumber(tt.burst))
			require.NoError(t, err)

			var (
				mu  sync.Mutex
				got []error
			)

			n.errorHandler = func(errs []error) {
				mu.Lock()
				defer mu.Unlock()

				got = append(got, errs...)
			}

			errA, errB := errors.New("a"), errors.New("b")
			n.onError(errA)
			n.onError(errB)

			require.Eventually(t, func() bool {
				mu.Lock()
				defer mu.Unlock()

				return slices.Contains(got, errA) && slices.Contains(got, errB)
			}, time.Second, time.Millisecond)

			n.cacheMu.Lock()
			defer n.cacheMu.Unlock()

			assert.Empty(t, n.cacheErrors, "delivered errors must leave the cache")
		})
	}
}

// TestHandlerPanicsAreRecovered checks that a panicking handler neither
// crashes the program nor stops later deliveries.
func TestHandlerPanicsAreRecovered(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	var eventCalls, errorCalls atomic.Int64

	n, err := Create(
		OnPath(dir),
		WithThrottleInterval(10*time.Millisecond),
		WithEventHandler(func(string, []fsnotify.Event) {
			eventCalls.Add(1)
			panic("event handler failure")
		}),
	)
	require.NoError(t, err)

	n.errorHandler = func([]error) {
		errorCalls.Add(1)
		panic("error handler failure")
	}

	file := filepath.Join(dir, "file")

	for want := int64(1); want <= 2; want++ {
		n.onEvent(fsnotify.Event{Name: file, Op: fsnotify.Write})
		require.Eventually(t, func() bool { return eventCalls.Load() == want }, time.Second, time.Millisecond)

		n.onError(errors.New("failure"))
		require.Eventually(t, func() bool { return errorCalls.Load() == want }, time.Second, time.Millisecond)
	}
}
