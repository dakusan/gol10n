//Compatibility functions for golang <1.21
//go:build !go1.21

package translate

func max[T int | int8 | int16 | int32 | int64 | uint | uint8 | uint16 | uint32 | uint64](args ...T) T {
	if len(args) == 0 {
		return 0
	} else if len(args) == 1 {
		return args[0]
	}

	maxV := args[0]
	for _, v := range args[1:] {
		if v > maxV {
			maxV = v
		}
	}

	return maxV
}

// Find if a list contains a value
func arrayIn[T comparable](list []T, val T) bool {
	for _, v := range list {
		if v == val {
			return true
		}
	}
	return false
}
