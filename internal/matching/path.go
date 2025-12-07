package matching

import (
	"strings"
)

// MatchPath checks if the request path matches the pattern.
// Returns a score > 0 if matched, 0 if not matched.
// Exact matches score higher than wildcard matches.
func MatchPath(pattern, path string) int {
	// Exact match
	if pattern == path {
		return 15 // Highest score for exact match
	}

	// Trailing wildcard (e.g., /api/users/*)
	if strings.HasSuffix(pattern, "/*") {
		prefix := strings.TrimSuffix(pattern, "/*")
		if strings.HasPrefix(path, prefix+"/") || path == prefix {
			return 10 // Lower score for wildcard
		}
	}

	// General wildcard matching
	if strings.Contains(pattern, "*") {
		if matchWildcard(pattern, path) {
			return 10
		}
	}

	return 0
}

// matchWildcard performs simple wildcard pattern matching.
// * matches any sequence of characters.
func matchWildcard(pattern, path string) bool {
	// Split pattern by wildcards
	parts := strings.Split(pattern, "*")
	if len(parts) == 1 {
		return pattern == path
	}

	// Track position in path
	pos := 0

	for i, part := range parts {
		if part == "" {
			continue
		}

		// For first part, must be prefix
		if i == 0 {
			if !strings.HasPrefix(path, part) {
				return false
			}
			pos = len(part)
			continue
		}

		// For middle/last parts, find the substring
		idx := strings.Index(path[pos:], part)
		if idx == -1 {
			return false
		}
		pos += idx + len(part)
	}

	return true
}

// MatchPathVariable extracts path variables from a path pattern.
// Example: pattern "/users/{id}" with path "/users/123" returns {"id": "123"}
// This is a future feature - currently returns empty map.
func MatchPathVariable(pattern, path string) map[string]string {
	// TODO: Implement path variable extraction in future version
	return make(map[string]string)
}
