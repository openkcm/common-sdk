package pointers

// To returns a pointer to the given value.
func To[T any](v T) *T {
	return &v
}

// Value returns the value of the pointer, or the zero value if nil.
func Value[T any](p *T) T {
	if p == nil {
		var zero T
		return zero
	}
	return *p
}

// Bool returns a pointer to the given bool.
func Bool(v bool) *bool { return &v }

// String returns a pointer to the given string.
func String(v string) *string { return &v }

// Int Integers
func Int(v int) *int             { return &v }
func Int8(v int8) *int8          { return &v }
func Int16(v int16) *int16       { return &v }
func Int32(v int32) *int32       { return &v }
func Int64(v int64) *int64       { return &v }
func Uint(v uint) *uint          { return &v }
func Uint8(v uint8) *uint8       { return &v }
func Uint16(v uint16) *uint16    { return &v }
func Uint32(v uint32) *uint32    { return &v }
func Uint64(v uint64) *uint64    { return &v }
func Uintptr(v uintptr) *uintptr { return &v }

// Float32 Floats
func Float32(v float32) *float32 { return &v }
func Float64(v float64) *float64 { return &v }

// Complex64 Complex numbers
func Complex64(v complex64) *complex64    { return &v }
func Complex128(v complex128) *complex128 { return &v }

// Byte Bytes (slice) and Runes
func Byte(v byte) *byte { return &v }
func Rune(v rune) *rune { return &v }
