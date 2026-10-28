package loader

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/fsnotify/fsnotify"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestOnEventHandlesCombinedOperations covers events carrying several
// operations at once. The kqueue backend (macOS, BSD) merges the flags of one
// kernel event, so a truncate and a write can arrive as a single Write|Chmod
// event; it must not be dropped for not being exactly Write.
func TestOnEventHandlesCombinedOperations(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		op   fsnotify.Op
		want string
	}{
		{name: "single handled operation", op: fsnotify.Write, want: "new"},
		{name: "handled operation combined with another", op: fsnotify.Write | fsnotify.Chmod, want: "new"},
		{name: "only operations that are not handled", op: fsnotify.Chmod, want: "old"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			dir := t.TempDir()
			file := filepath.Join(dir, "key.pem")
			require.NoError(t, os.WriteFile(file, []byte("old"), 0o600))

			l, err := Create(OnPath(dir), WithKeyIDType(FileNameWithExtension))
			require.NoError(t, err)

			// Load without starting the watcher, so that the only event is
			// the one sent below.
			l.setLocations()
			require.NoError(t, l.resync())

			require.NoError(t, os.WriteFile(file, []byte("new"), 0o600))
			l.onEvent(fsnotify.Event{Name: file, Op: tt.op})

			got, ok := l.Storage().Get("key.pem")
			require.True(t, ok)
			assert.Equal(t, tt.want, string(got))
		})
	}
}
