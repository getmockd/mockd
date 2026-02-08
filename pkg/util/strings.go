// Package util provides shared utility functions for mockd.
package util

import (
	"path/filepath"
	"strings"
)

// MaxLogBodySize is the default maximum body size for logging (10KB).
const MaxLogBodySize = 10 * 1024

// SafeFilePath validates that a file path is safe from path traversal attacks.
// It rejects paths containing ".." components and absolute paths.
// Use this for API-sourced paths (bodyFile, dataFile) where relative paths are expected.
// Returns the cleaned path and true if safe, or ("", false) if unsafe.
func SafeFilePath(path string) (string, bool) {
	if path == "" {
		return "", false
	}
	cleaned := filepath.Clean(path)
	// Reject path traversal
	if strings.Contains(cleaned, "..") {
		return "", false
	}
	// Reject absolute paths
	if filepath.IsAbs(cleaned) {
		return "", false
	}
	return cleaned, true
}

// SafeFilePathAllowAbsolute validates that a file path has no path traversal
// components ("..") but permits absolute paths. Use this for config-sourced
// file paths (protoFile, schemaFile, wsdlFile) where absolute paths are
// a legitimate use case.
// Returns the cleaned path and true if safe, or ("", false) if unsafe.
func SafeFilePathAllowAbsolute(path string) (string, bool) {
	if path == "" {
		return "", false
	}
	cleaned := filepath.Clean(path)
	// Reject path traversal
	if strings.Contains(cleaned, "..") {
		return "", false
	}
	return cleaned, true
}

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
