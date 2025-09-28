// Package keyvalue defines generic interfaces for key-value storage systems.
// It separates read-only access from full read-write access, allowing flexible
// abstractions and implementations.
//
// The package provides two key interfaces:
//
//   - ReadStorage[K, V]:
//     Defines read-only operations on a storage backend, including fetching
//     values by key and checking if the storage is empty.
//
//   - Storage[K, V]:
//     Extends ReadStorage by adding mutation operations such as storing,
//     removing, and clearing entries.
//
// These interfaces are designed to be implemented by various backends such as
// in-memory maps, persistent databases, or distributed caches.
//
// Example:
//
//	type MemoryStorage[K comparable, V any] struct {
//	    data map[K]V
//	}
//
//	func (ms *MemoryStorage[K, V]) Get(key K) (V, bool)   { v, ok := ms.data[key]; return v, ok }
//	func (ms *MemoryStorage[K, V]) IsEmpty() bool         { return len(ms.data) == 0 }
//	func (ms *MemoryStorage[K, V]) Store(key K, val V)    { ms.data[key] = val }
//	func (ms *MemoryStorage[K, V]) Remove(key K) bool     { _, ok := ms.data[key]; delete(ms.data, key); return ok }
//	func (ms *MemoryStorage[K, V]) Clean() bool           { existed := len(ms.data) > 0; ms.data = map[K]V{}; return existed }
package keyvalue

// Storage defines a generic key-value storage interface that supports both
// read and write operations.
//
// It embeds ReadStorage for read-only access, and extends it with mutation
// methods for modifying the underlying storage.
//
// Type Parameters:
//   - K: the type of keys, must be comparable
//   - V: the type of values, can be any type
//
// Methods:
//   - Store(key K, value V):
//     Inserts or updates the value associated with the given key.
//   - Remove(key K) bool:
//     Removes the value associated with the key. Returns true if the key
//     existed and was removed, false otherwise.
//   - Clean() bool:
//     Removes all key-value pairs from the storage. Returns true if any
//     entries were present before cleanup, false otherwise.
type Storage[K comparable, V any] interface {
	ReadStorage[K, V]

	Store(key K, value V)
	Remove(key K) bool
	Clean() bool
}

// ReadStorage defines a generic read-only key-value storage interface.
//
// Type Parameters:
//   - K: the type of keys, must be comparable
//   - V: the type of values, can be any type
//
// Methods:
//   - Get(key K) (V, bool):
//     Retrieves the value associated with the given key. The second return
//     value indicates whether the key was present in the storage.
//   - IsEmpty() bool:
//     Returns true if the storage contains no entries, false otherwise.
type ReadStorage[K comparable, V any] interface {
	List() []K
	Get(key K) (V, bool)
	IsEmpty() bool
}
