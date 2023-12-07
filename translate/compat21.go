//Compatibility functions for golang >=1.21
//go:build go1.21

package translate

import "slices"

// Find if a list contains a value
func arrayIn[T comparable](list []T, val T) bool {
	return slices.Contains(list, val)
}
