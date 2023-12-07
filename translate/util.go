//Utility functions (Optimization, shortening, and conversion functions)

package translate

import (
	"reflect"
	"unsafe"
)

// Unsafe: Convert a byte slice to a string
func b2s(b []byte) string {
	return *(*string)(unsafe.Pointer(&b))
}

// Unsafe: Convert a string to a byte slice
func s2b(s string) (b []byte) {
	str := (*reflect.StringHeader)(unsafe.Pointer(&s))
	sh := (*reflect.SliceHeader)(unsafe.Pointer(&b))
	sh.Data = str.Data
	sh.Len = str.Len
	sh.Cap = str.Len
	return b
}

// ---------------Return empty string when also returning an error---------------
const returnBlankStrOnErr = ""

func retErrWithStr(err error) (string, error) {
	return returnBlankStrOnErr, err
}

// -----------------------Convert any type to a byte array-----------------------
func any2b[T any](val *T) []byte {
	return any2bLen(val, 1)
}
func any2bLen[T any](val *T, length uint) []byte {
	//goland:noinspection GoRedundantConversion
	return unsafe.Slice((*byte)(unsafe.Pointer(val)), uint(unsafe.Sizeof(*val))*length)
}

// -------------------------Convert a pointer to *uint32-------------------------
func p2uint32p[T any](b *T) *uint32 {
	return (*uint32)(unsafe.Pointer(b))
}

//-----------------------Missing go language functionality----------------------

// Turn a return with 2 values into 1 value (ignore the second)
func twoToOne[V1 any, V2 any](v1 V1, _ V2) V1 {
	return v1
}

// ------------------------Pull length as uint and uint32------------------------
func ulen[S ~[]E, E any](v S) uint {
	return uint(len(v))
}
func ulen32[S ~[]E, E any](v S) uint32 {
	return uint32(len(v))
}
func ulens(v string) uint {
	return uint(len(v))
}
func ulen32s(v string) uint32 {
	return uint32(len(v))
}
func ulenm[K comparable, V any](v map[K]V) uint {
	return uint(len(v))
}
func ulen32m[K comparable, V any](v map[K]V) uint32 {
	return uint32(len(v))
}
