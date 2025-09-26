package notifier_test

import (
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/openkcm/common-sdk/pkg/commonfs/notifier"
)

// helper to create temporary files
func createTempFiles(t *testing.T, dir string, files map[string]string) {
	t.Helper()

	for name, content := range files {
		path := filepath.Join(dir, name)
		err := os.WriteFile(path, []byte(content), 0644)
		require.NoError(t, err)
	}
}

func TestGroupNotifierSimpleHandlerInvocation(t *testing.T) {
	tmpDir := t.TempDir()
	files := map[string]string{"a.txt": "1"}

	var called int32

	g, err := notifier.NewGroupNotifyWrapper(
		notifier.WithPath(tmpDir),
		notifier.WithSimpleHandler(func() { atomic.AddInt32(&called, 1) }),
	)
	require.NoError(t, err)

	err = g.StartWatching()
	require.NoError(t, err)

	defer func(g *notifier.GroupNotifier) {
		err := g.StopWatching()
		require.NoError(t, err)
	}(g)

	createTempFiles(t, tmpDir, files)
	time.Sleep(200 * time.Millisecond)

	require.Equal(t, int32(1), atomic.LoadInt32(&called))
}

func TestGroupNotifierStartStopWatching(t *testing.T) {
	tmpDir := t.TempDir()
	g, err := notifier.NewGroupNotifyWrapper(notifier.WithPath(tmpDir))
	require.NoError(t, err)

	err = g.StartWatching()
	require.NoError(t, err)
	require.NoError(t, g.StopWatching())
	require.NoError(t, g.StopWatching())
}
