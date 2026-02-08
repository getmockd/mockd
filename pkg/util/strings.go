// Package util provides shared utility functions for mockd.
package util

// MaxLogBodySize is the default maximum body size for logging (10KB).
const MaxLogBodySize = 10 * 1024

// TruncateBody truncates a string to maxSize bytes, appending "...(truncated)" if truncated.
// If maxSize <= 0, uses MaxLogBodySize.
func TruncateBody(data string, maxSize int) string {
	if maxSize <= 0 {
		maxSize = MaxLogBodySize
	}
	if len(data) > maxSize {
		return data[:maxSize] + "...(truncated)"
	}
	return data
}

// FormatInt formats an int as a decimal string without using fmt.
func FormatInt(n int) string {
	if n == 0 {
		return "0"
	}

	negative := n < 0
	if negative {
		n = -n
	}

	var digits [20]byte
	i := len(digits)

	for n > 0 {
		i--
		digits[i] = byte('0' + n%10)
		n /= 10
	}

	if negative {
		i--
		digits[i] = '-'
	}

	return string(digits[i:])
}

// FormatInt64 formats an int64 as a decimal string without using fmt.
func FormatInt64(n int64) string {
	if n == 0 {
		return "0"
	}

	negative := n < 0
	if negative {
		n = -n
	}

	var digits [20]byte
	i := len(digits)

	for n > 0 {
		i--
		digits[i] = byte('0' + n%10)
		n /= 10
	}

	if negative {
		i--
		digits[i] = '-'
	}

	return string(digits[i:])
}
