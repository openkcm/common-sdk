package keyvalue

type MemoryStorage[K comparable, V any] struct {
	data map[K]V
}

func NewMemoryStorage[K comparable, V any]() *MemoryStorage[K, V] {
	return &MemoryStorage[K, V]{
		data: make(map[K]V),
	}
}

func (ms *MemoryStorage[K, V]) AsReadStorage() ReadStorage[K, V] {
	return ms
}

func (ms *MemoryStorage[K, V]) Store(key K, value V) {
	ms.data[key] = value
}

func (ms *MemoryStorage[K, V]) Clean() bool {
	exists := len(ms.data) > 0
	for k := range ms.data {
		delete(ms.data, k)
	}

	return exists
}

func (ms *MemoryStorage[K, V]) IsEmpty() bool {
	return len(ms.data) == 0
}

func (ms *MemoryStorage[K, V]) Get(key K) (V, bool) {
	value, exist := ms.data[key]
	return value, exist
}

func (ms *MemoryStorage[K, V]) Remove(key K) bool {
	_, exist := ms.data[key]
	if exist {
		delete(ms.data, key)
	}

	return exist
}
