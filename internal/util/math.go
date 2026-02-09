package util

// IntAbs returns the absolute value of an integer.
func IntAbs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
