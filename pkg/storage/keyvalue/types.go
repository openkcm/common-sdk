package keyvalue

// StringToBytesStorage is a convenience alias for a string→[]byte storage backend.
type StringToBytesStorage = Storage[string, []byte]

// ReadOnlyStringToBytesStorage is the read-only view of StringToBytesStorage.
type ReadOnlyStringToBytesStorage = ReadStorage[string, []byte]
