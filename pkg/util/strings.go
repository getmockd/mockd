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
