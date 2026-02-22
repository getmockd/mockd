package admin

import "strconv"

// parsePositiveInt returns a parsed int only when the value is a valid positive integer.
func parsePositiveInt(v string) (int, bool) {
	if v == "" {
		return 0, false
	}
	n, err := strconv.Atoi(v)
	if err != nil || n <= 0 {
		return 0, false
	}
	return n, true
}

// parseNonNegativeInt returns a parsed int only when the value is a valid non-negative integer.
func parseNonNegativeInt(v string) (int, bool) {
	if v == "" {
		return 0, false
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < 0 {
		return 0, false
	}
	return n, true
}
