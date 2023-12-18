//Utility functions
//go:build !gol10n_read_compiled_only

package execute

// Keys returns the keys of the map m. The keys will be in an indeterminate order.
// I had this as a compat for go1.21, but maps.Keys was removed from go1.21 in the release version
func getMapKeys[M ~map[K]V, K comparable, V any](m M) []K {
	ret := make([]K, len(m))
	index := 0
	for k := range m {
		ret[index] = k
		index++
	}

	return ret
}

// Conditional
func cond[T any](isTrue bool, ifTrue, ifFalse T) T {
	if isTrue {
		return ifTrue
	}
	return ifFalse
}
