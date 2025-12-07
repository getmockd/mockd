// Package proxy provides traffic filtering for selective recording.
package proxy

import (
	"strings"
)

// FilterConfig defines include/exclude patterns for traffic filtering.
type FilterConfig struct {
	IncludePaths []string // Record only if path matches (empty = all)
	ExcludePaths []string // Never record if path matches
	IncludeHosts []string // Record only from these hosts (empty = all)
	ExcludeHosts []string // Never record from these hosts
}

// NewFilterConfig creates an empty filter config (records everything).
func NewFilterConfig() *FilterConfig {
	return &FilterConfig{}
}

// ShouldRecord determines if a request should be recorded based on filters.
// Precedence:
// 1. If matches ANY exclude pattern → NOT recorded
// 2. If include patterns exist AND matches NONE → NOT recorded
// 3. Otherwise → recorded
func (f *FilterConfig) ShouldRecord(host, path string) bool {
	// Check host exclusions first
	for _, pattern := range f.ExcludeHosts {
		if matchGlob(pattern, host) {
			return false
		}
	}

	// Check path exclusions
	for _, pattern := range f.ExcludePaths {
		if matchGlob(pattern, path) {
			return false
		}
	}

	// Check host inclusions (if any defined)
	if len(f.IncludeHosts) > 0 {
		hostMatched := false
		for _, pattern := range f.IncludeHosts {
			if matchGlob(pattern, host) {
				hostMatched = true
				break
			}
		}
		if !hostMatched {
			return false
		}
	}

	// Check path inclusions (if any defined)
	if len(f.IncludePaths) > 0 {
		for _, pattern := range f.IncludePaths {
			if matchGlob(pattern, path) {
				return true
			}
		}
		return false
	}

	return true
}

// matchGlob matches a glob pattern against a string.
// Supports * wildcard for matching any sequence of characters.
// Patterns are case-sensitive for paths, case-insensitive for hosts.
func matchGlob(pattern, s string) bool {
	// Simple glob matching with * wildcard
	if pattern == "*" {
		return true
	}

	// No wildcards - exact match
	if !strings.Contains(pattern, "*") {
		return pattern == s
	}

	// Split pattern by * and match each segment
	parts := strings.Split(pattern, "*")

	// Handle leading *
	if strings.HasPrefix(pattern, "*") {
		// Must end with the first non-empty part
		parts = parts[1:]
	} else {
		// Must start with the first part
		if !strings.HasPrefix(s, parts[0]) {
			return false
		}
		s = s[len(parts[0]):]
		parts = parts[1:]
	}

	// Handle trailing *
	if strings.HasSuffix(pattern, "*") {
		parts = parts[:len(parts)-1]
	} else if len(parts) > 0 {
		// Must end with the last part
		lastPart := parts[len(parts)-1]
		if !strings.HasSuffix(s, lastPart) {
			return false
		}
		s = s[:len(s)-len(lastPart)]
		parts = parts[:len(parts)-1]
	}

	// Check remaining parts exist in order
	for _, part := range parts {
		if part == "" {
			continue
		}
		idx := strings.Index(s, part)
		if idx < 0 {
			return false
		}
		s = s[idx+len(part):]
	}

	return true
}
