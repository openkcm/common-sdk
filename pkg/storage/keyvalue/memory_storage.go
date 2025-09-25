package keyvalue

type MemoryStorage struct {
	data map[string][]byte
}

func NewMemoryStorage() *MemoryStorage {
	return &MemoryStorage{
		data: make(map[string][]byte),
	}
}

func (ms *MemoryStorage) AsReadStorage() ReadStorage[string, []byte] {
	return ms
}

func (ms *MemoryStorage) Store(key string, value []byte) {
	ms.data[key] = value
}

func (ms *MemoryStorage) Clean() bool {
	hasdata := len(ms.data) > 0
	for k := range ms.data {
		delete(ms.data, k)
	}

	return hasdata
}

func (ms *MemoryStorage) IsEmpty() bool {
	return len(ms.data) == 0
}

func (ms *MemoryStorage) Get(key string) ([]byte, bool) {
	value, exist := ms.data[key]
	return value, exist
}

func (ms *MemoryStorage) Remove(key string) bool {
	_, exist := ms.data[key]
	if exist {
		delete(ms.data, key)
	}

	return exist
}
