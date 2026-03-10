package matching

import (
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
)

// regexCache caches compiled regex patterns for performance.
// Patterns are validated at config load time, so cache misses
// are rare in steady-state operation. The cache is bounded to
// maxRegexCacheSize entries to prevent unbounded memory growth.
const maxRegexCacheSize = 1024

var (
	regexCache   = make(map[string]*regexp.Regexp)
	regexCacheMu sync.RWMutex
)

// getCompiledRegex returns a cached compiled regex or compiles and caches it.
// Returns nil if the pattern is invalid.
func getCompiledRegex(pattern string) *regexp.Regexp {
	// Fast path: check cache with read lock
	regexCacheMu.RLock()
	re, ok := regexCache[pattern]
	regexCacheMu.RUnlock()
	if ok {
		return re
	}

	// Slow path: compile and cache
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil
	}

	regexCacheMu.Lock()
	// Double-check after acquiring write lock
	if existing, ok := regexCache[pattern]; ok {
		regexCacheMu.Unlock()
		return existing
	}
	// Evict all entries if cache is full (simple reset strategy — patterns are
	// re-compiled on demand, so this is a performance cost, not a correctness issue)
	if len(regexCache) >= maxRegexCacheSize {
		regexCache = make(map[string]*regexp.Regexp)
	}
	regexCache[pattern] = re
	regexCacheMu.Unlock()

	return re
}

// MatchPath checks if the request path matches the pattern.
// Returns a score > 0 if matched, 0 if not matched.
// Exact matches score higher than wildcard matches.
// Supports:
//   - Exact match: "/api/users" matches "/api/users"
//   - Wildcard: "/api/users/*" matches "/api/users/123"
//   - Named params: "/api/users/{id}" matches "/api/users/123"
func MatchPath(pattern, path string) int {
	// Normalize: URL-decode the pattern so that stored paths like "/api/hello%20world"
	// match incoming requests where Go's net/http has already decoded the path to
	// "/api/hello world". If decoding fails, use the original pattern.
	if decoded, err := url.PathUnescape(pattern); err == nil {
		pattern = decoded
	}

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
// Supports full-segment params ("/users/{id}") and params with literal
// prefix/suffix ("/users/{id}.json", "v{version}-api").
func matchNamedParams(pattern, path string) bool {
	patternParts := strings.Split(strings.Trim(pattern, "/"), "/")
	pathParts := strings.Split(strings.Trim(path, "/"), "/")

	// Must have same number of segments
	if len(patternParts) != len(pathParts) {
		return false
	}

	for i, patternPart := range patternParts {
		// Full-segment named parameter matches any value
		if strings.HasPrefix(patternPart, "{") && strings.HasSuffix(patternPart, "}") {
			continue
		}
		// Segment containing a {param} with literal prefix/suffix
		// e.g., "{Sid}.json" matches "SM123.json", "v{ver}-api" matches "v2-api"
		if strings.Contains(patternPart, "{") && strings.Contains(patternPart, "}") {
			if matchSegmentWithParam(patternPart, pathParts[i]) {
				continue
			}
			return false
		}
		// Literal parts must match exactly
		if patternPart != pathParts[i] {
			return false
		}
	}

	return true
}

// matchSegmentWithParam checks if a path segment matches a pattern segment
// containing one or more {param} placeholders with optional literal text.
// Examples: "{Sid}.json" matches "SM123.json", "v{ver}" matches "v2".
func matchSegmentWithParam(pattern, segment string) bool {
	// Build a regex from the pattern: replace {param} with (.+) for greedy match
	regexStr := "^"
	rest := pattern
	for {
		openIdx := strings.Index(rest, "{")
		if openIdx == -1 {
			regexStr += regexp.QuoteMeta(rest)
			break
		}
		closeIdx := strings.Index(rest[openIdx:], "}")
		if closeIdx == -1 {
			regexStr += regexp.QuoteMeta(rest)
			break
		}
		closeIdx += openIdx
		regexStr += regexp.QuoteMeta(rest[:openIdx]) + "(.+)"
		rest = rest[closeIdx+1:]
	}
	regexStr += "$"

	re := getCompiledRegex(regexStr)
	if re == nil {
		return false
	}
	return re.MatchString(segment)
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
// Compiled patterns are cached for performance.
func MatchPathPattern(pattern, path string) (score int, captures map[string]string) {
	if pattern == "" {
		return 0, nil
	}

	// Get cached compiled regex pattern
	re := getCompiledRegex(pattern)
	if re == nil {
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

		// Full-segment named parameter: {name}
		if strings.HasPrefix(patternPart, "{") && strings.HasSuffix(patternPart, "}") {
			paramName := patternPart[1 : len(patternPart)-1]
			result[paramName] = pathParts[i]
			continue
		}

		// Segment containing {param} with literal prefix/suffix: {Sid}.json
		if strings.Contains(patternPart, "{") && strings.Contains(patternPart, "}") {
			extractSegmentParams(patternPart, pathParts[i], result)
			continue
		}

		// Wildcard: * - capture remaining segment(s)
		if patternPart == "*" {
			// For trailing wildcard, capture the rest of the path
			if i == len(patternParts)-1 {
				// Join remaining path parts
				remaining := strings.Join(pathParts[i:], "/")
				result[strconv.Itoa(wildcardIndex)] = remaining
			} else {
				result[strconv.Itoa(wildcardIndex)] = pathParts[i]
			}
			wildcardIndex++
			continue
		}
	}

	return result
}

// extractSegmentParams extracts named parameter values from a segment with
// literal prefix/suffix. Pattern "{Sid}.json" with segment "SM123.json"
// yields {"Sid": "SM123"}.
func extractSegmentParams(pattern, segment string, result map[string]string) {
	// Build a regex with named capture groups
	regexStr := "^"
	rest := pattern
	for {
		openIdx := strings.Index(rest, "{")
		if openIdx == -1 {
			regexStr += regexp.QuoteMeta(rest)
			break
		}
		closeIdx := strings.Index(rest[openIdx:], "}")
		if closeIdx == -1 {
			regexStr += regexp.QuoteMeta(rest)
			break
		}
		closeIdx += openIdx
		paramName := rest[openIdx+1 : closeIdx]
		regexStr += regexp.QuoteMeta(rest[:openIdx]) + "(?P<" + paramName + ">.+)"
		rest = rest[closeIdx+1:]
	}
	regexStr += "$"

	re := getCompiledRegex(regexStr)
	if re == nil {
		return
	}
	match := re.FindStringSubmatch(segment)
	if match == nil {
		return
	}
	for i, name := range re.SubexpNames() {
		if i > 0 && name != "" && i < len(match) {
			result[name] = match[i]
		}
	}
}
