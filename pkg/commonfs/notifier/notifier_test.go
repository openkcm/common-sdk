package notifier_test

import (
	"os"
	"path/filepath"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/stretchr/testify/require"

	"github.com/openkcm/common-sdk/pkg/commonfs/notifier"
)

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
