package keyvalue_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/openkcm/common-sdk/pkg/storage/keyvalue"
)

func TestNewMemoryStorage(t *testing.T) {
	st := keyvalue.NewMemoryStorage[string, []byte]()
	require.NotNil(t, st)
	require.True(t, st.IsEmpty())
}

func TestStoreAndGet(t *testing.T) {
	st := keyvalue.NewMemoryStorage[string, []byte]()

	// Store a key
	st.Store("foo", []byte("bar"))

	// Get existing
	val, ok := st.Get("foo")
	require.True(t, ok)
	require.Equal(t, []byte("bar"), val)

	// Get non-existing
	val, ok = st.Get("nope")
	require.False(t, ok)
	require.Nil(t, val)
}

func TestRemove(t *testing.T) {
	st := keyvalue.NewMemoryStorage[string, []byte]()

	// Remove non-existing
	ok := st.Remove("ghost")
	require.False(t, ok)

	// Store then remove
	st.Store("foo", []byte("bar"))
	ok = st.Remove("foo")
	require.True(t, ok)

	// Ensure it’s gone
	_, exist := st.Get("foo")
	require.False(t, exist)
}

func TestClean(t *testing.T) {
	st := keyvalue.NewMemoryStorage[string, []byte]()

	// Clean on empty storage
	ok := st.Clean()
	require.False(t, ok)

	// Store and clean
	st.Store("foo", []byte("bar"))
	st.Store("baz", []byte("qux"))

	ok = st.Clean()
	require.True(t, ok)

	// Verify it is empty
	_, exist := st.Get("foo")
	require.False(t, exist)
	_, exist = st.Get("baz")
	require.False(t, exist)
}

func TestAsReadStorage(t *testing.T) {
	st := keyvalue.NewMemoryStorage[string, []byte]()

	// AsReadStorage should not panic and return same storage
	readOnly := st.AsReadStorage()
	require.NotNil(t, readOnly)

	// Ensure it still behaves like ReadStorage
	st.Store("foo", []byte("bar"))

	val, ok := readOnly.Get("foo")
	require.True(t, ok)
	require.Equal(t, []byte("bar"), val)
}
