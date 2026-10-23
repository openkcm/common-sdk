package notifier

import (
	"path/filepath"
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
