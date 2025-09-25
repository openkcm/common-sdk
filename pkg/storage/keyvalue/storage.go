package keyvalue

type Storage[K any, V any] interface {
	ReadStorage[K, V]

	Store(key K, value V)
	Remove(key K) bool
	Clean() bool
}

type ReadStorage[K any, V any] interface {
	Get(key K) (V, bool)
	IsEmpty() bool
}
