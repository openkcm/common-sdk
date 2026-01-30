// Package keyvalue provides simple in-memory key–value storage utilities
// with generic type parameters for keys and values.
//
// The MemoryStorage type is a minimal, type-safe, map-backed storage implementation
// that satisfies both read and write semantics. It is intended for use cases such as:
//   - Caching
//   - Testing
//   - Lightweight in-memory data stores
//
// Example usage:
//
//	storage := keyvalue.NewMemoryStorage[string, []byte]()
//
//	storage.Store("foo", []byte("bar"))
//	value, ok := storage.Get("foo")
//	if ok {
//	    fmt.Println(string(value)) // "bar"
//	}
//
//	storage.Remove("foo")
//	fmt.Println(storage.IsEmpty()) // true
package keyvalue

import "sync"

// MemoryStorage is a simple in-memory implementation of a generic key–value store.
//
// It is backed by a Go map and supports storing, retrieving, removing, and
// cleaning up entries. The key type must be comparable (so it can be used as
// a map key), and the value type may be any Go type.
//
// MemoryStorage is safe for use by a single goroutine at a time. For concurrent
// use, wrap it with synchronization (e.g., sync.Mutex).
type MemoryStorage[K comparable, V any] struct {
	mu   sync.RWMutex
	data map[K]V
}

// NewMemoryStorage creates and returns a new empty MemoryStorage instance.
//
// Example:
//
//	storage := keyvalue.NewMemoryStorage[string, int]()
//	storage.Store("x", 42)
func NewMemoryStorage[K comparable, V any]() *MemoryStorage[K, V] {
	return &MemoryStorage[K, V]{
		mu:   sync.RWMutex{},
		data: make(map[K]V),
	}
}

// AsReadStorage exposes this MemoryStorage as a read-only storage interface.
//
// This is useful when consumers should only read values but not modify them.
func (ms *MemoryStorage[K, V]) AsReadStorage() ReadStorage[K, V] {
	return ms
}

// Store inserts or updates a key–value pair in the storage.
//
// If the key already exists, its value is overwritten.
func (ms *MemoryStorage[K, V]) Store(key K, value V) {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	ms.data[key] = value
}

// Clean removes all entries from the storage and returns whether the storage
// had any data before cleanup.
//
// Returns:
//   - true if the storage contained one or more items
//   - false if it was already empty
func (ms *MemoryStorage[K, V]) Clean() bool {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	exists := len(ms.data) > 0
	for k := range ms.data {
		delete(ms.data, k)
	}

	return exists
}

// IsEmpty reports whether the storage contains no entries.
//
// Example:
//
//	storage := keyvalue.NewMemoryStorage[string, int]()
//	fmt.Println(storage.IsEmpty()) // true
func (ms *MemoryStorage[K, V]) IsEmpty() bool {
	ms.mu.RLock()
	defer ms.mu.RUnlock()

	return len(ms.data) == 0
}

// Get retrieves the value associated with the given key.
//
// Returns:
//   - value: the value if found (zero value of V if not found)
//   - bool: true if the key exists, false otherwise
func (ms *MemoryStorage[K, V]) Get(key K) (V, bool) {
	ms.mu.RLock()
	defer ms.mu.RUnlock()

	value, exist := ms.data[key]

	return value, exist
}

func (ms *MemoryStorage[K, V]) List() []K {
	ms.mu.RLock()
	defer ms.mu.RUnlock()

	keys := make([]K, 0, len(ms.data))
	for k := range ms.data {
		keys = append(keys, k)
	}

	return keys
}

// Remove deletes the entry for the given key if it exists.
//
// Returns:
//   - true if the key was found and removed
//   - false if the key did not exist
func (ms *MemoryStorage[K, V]) Remove(key K) bool {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	_, exist := ms.data[key]
	if exist {
		delete(ms.data, key)
	}

	return exist
}
