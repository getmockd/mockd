package matching

import (
	"fmt"
	"regexp"
	"strings"
)

// MatchPath checks if the request path matches the pattern.
// Returns a score > 0 if matched, 0 if not matched.
// Exact matches score higher than wildcard matches.
// Supports:
//   - Exact match: "/api/users" matches "/api/users"
//   - Wildcard: "/api/users/*" matches "/api/users/123"
//   - Named params: "/api/users/{id}" matches "/api/users/123"
func MatchPath(pattern, path string) int {
	// Exact match
	if pattern == path {
		return ScorePathExact
	}

	// Check for named parameter pattern (e.g., /api/users/{id})
	if strings.Contains(pattern, "{") && strings.Contains(pattern, "}") {
		if matchNamedParams(pattern, path) {
			return ScorePathNamedParams
		}
	}

	// Trailing wildcard (e.g., /api/users/*)
	if strings.HasSuffix(pattern, "/*") {
		prefix := strings.TrimSuffix(pattern, "/*")
		if strings.HasPrefix(path, prefix+"/") || path == prefix {
			return ScorePathWildcard
		}
	}

	// General wildcard matching
	if strings.Contains(pattern, "*") {
		if matchWildcard(pattern, path) {
			return ScorePathWildcard
		}
	}

	return 0
}

// matchNamedParams checks if path matches a pattern with named parameters.
// Example: "/users/{id}" matches "/users/123"
func matchNamedParams(pattern, path string) bool {
	patternParts := strings.Split(strings.Trim(pattern, "/"), "/")
	pathParts := strings.Split(strings.Trim(path, "/"), "/")

	// Must have same number of segments
	if len(patternParts) != len(pathParts) {
		return false
	}

	for i, patternPart := range patternParts {
		// Named parameter matches any value
		if strings.HasPrefix(patternPart, "{") && strings.HasSuffix(patternPart, "}") {
			continue
		}
		// Literal parts must match exactly
		if patternPart != pathParts[i] {
			return false
		}
	}

	return true
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

// MatchPathPattern checks if the request path matches a regex pattern.
// Returns a score > 0 if matched, 0 if not matched.
// The score is ScorePathPattern (between exact match and named params).
// Also returns a map of named capture groups for template variable access.
// Uses Go's regexp package with RE2 syntax.
func MatchPathPattern(pattern, path string) (score int, captures map[string]string) {
	if pattern == "" {
		return 0, nil
	}

	// Compile the regex pattern
	re, err := regexp.Compile(pattern)
	if err != nil {
		// Invalid regex pattern - gracefully return no match
		return 0, nil
	}

	// Check if the path matches the pattern
	match := re.FindStringSubmatch(path)
	if match == nil {
		return 0, nil
	}

	// Extract named capture groups
	captures = make(map[string]string)
	subexpNames := re.SubexpNames()
	for i, name := range subexpNames {
		if i > 0 && name != "" && i < len(match) {
			captures[name] = match[i]
		}
	}

	// Return score between exact and named params
	return ScorePathPattern, captures
}

// ValidatePathPattern checks if a regex pattern is valid.
// Returns an error if the pattern cannot be compiled.
func ValidatePathPattern(pattern string) error {
	if pattern == "" {
		return nil
	}
	_, err := regexp.Compile(pattern)
	return err
}

// MatchPathVariable extracts path variables from a path pattern.
// Supports both {name} style params and * wildcards.
// Examples:
//   - pattern "/users/{id}" with path "/users/123" returns {"id": "123"}
//   - pattern "/api/users/*" with path "/api/users/456" returns {"0": "456"}
//   - pattern "/api/*/items/*" with path "/api/users/items/789" returns {"0": "users", "1": "789"}
func MatchPathVariable(pattern, path string) map[string]string {
	result := make(map[string]string)

	patternParts := strings.Split(strings.Trim(pattern, "/"), "/")
	pathParts := strings.Split(strings.Trim(path, "/"), "/")

	wildcardIndex := 0

	for i, patternPart := range patternParts {
		if i >= len(pathParts) {
			break
		}

		// Named parameter: {name}
		if strings.HasPrefix(patternPart, "{") && strings.HasSuffix(patternPart, "}") {
			paramName := patternPart[1 : len(patternPart)-1]
			result[paramName] = pathParts[i]
			continue
		}

		// Wildcard: * - capture remaining segment(s)
		if patternPart == "*" {
			// For trailing wildcard, capture the rest of the path
			if i == len(patternParts)-1 {
				// Join remaining path parts
				remaining := strings.Join(pathParts[i:], "/")
				result[fmt.Sprintf("%d", wildcardIndex)] = remaining
			} else {
				result[fmt.Sprintf("%d", wildcardIndex)] = pathParts[i]
			}
			wildcardIndex++
			continue
		}
	}

	return result
}
