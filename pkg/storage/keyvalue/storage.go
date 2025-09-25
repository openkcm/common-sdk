package keyvalue

type Storage[K comparable, V any] interface {
	ReadStorage[K, V]

	Store(key K, value V)
	Remove(key K) bool
	Clean() bool
}

type ReadStorage[K comparable, V any] interface {
	Get(key K) (V, bool)
	IsEmpty() bool
}
