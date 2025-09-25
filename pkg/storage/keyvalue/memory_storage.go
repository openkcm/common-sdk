package keyvalue

type MemoryKeyStringStorage[V any] struct {
	data map[string]V
}

func NewMemoryKeyStringStorage[V any]() *MemoryKeyStringStorage[V] {
	return &MemoryKeyStringStorage[V]{
		data: make(map[string]V),
	}
}

func (ms *MemoryKeyStringStorage[V]) AsReadStorage() ReadStorage[string, V] {
	return ms
}

func (ms *MemoryKeyStringStorage[V]) Store(key string, value V) {
	ms.data[key] = value
}

func (ms *MemoryKeyStringStorage[V]) Clean() bool {
	exists := len(ms.data) > 0
	for k := range ms.data {
		delete(ms.data, k)
	}

	return exists
}

func (ms *MemoryKeyStringStorage[V]) IsEmpty() bool {
	return len(ms.data) == 0
}

func (ms *MemoryKeyStringStorage[V]) Get(key string) (V, bool) {
	value, exist := ms.data[key]
	return value, exist
}

func (ms *MemoryKeyStringStorage[V]) Remove(key string) bool {
	_, exist := ms.data[key]
	if exist {
		delete(ms.data, key)
	}

	return exist
}
