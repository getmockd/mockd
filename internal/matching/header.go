package matching

import (
	"net/http"
	"strings"
)

// MatchHeader checks if a specific header matches.
// Header names are case-insensitive (per HTTP spec).
func MatchHeader(name, expectedValue string, headers http.Header) bool {
	// Get header value (case-insensitive key)
	actualValue := headers.Get(name)

	// Exact match
	return actualValue == expectedValue
}

// MatchHeaders checks if all specified headers match.
// Returns true only if ALL headers match.
func MatchHeaders(expected map[string]string, headers http.Header) bool {
	for name, value := range expected {
		if !MatchHeader(name, value, headers) {
			return false
		}
	}
	return true
}

// MatchHeaderPattern checks if a header matches a pattern.
// Supports simple prefix (*suffix), suffix (prefix*), and contains (*middle*) patterns.
func MatchHeaderPattern(name, pattern string, headers http.Header) bool {
	actualValue := headers.Get(name)
	if actualValue == "" {
		return false
	}

	// Exact match
	if !strings.Contains(pattern, "*") {
		return actualValue == pattern
	}

	// Prefix match (pattern*)
	if strings.HasSuffix(pattern, "*") && !strings.HasPrefix(pattern, "*") {
		prefix := strings.TrimSuffix(pattern, "*")
		return strings.HasPrefix(actualValue, prefix)
	}

	// Suffix match (*pattern)
	if strings.HasPrefix(pattern, "*") && !strings.HasSuffix(pattern, "*") {
		suffix := strings.TrimPrefix(pattern, "*")
		return strings.HasSuffix(actualValue, suffix)
	}

	// Contains match (*pattern*)
	if strings.HasPrefix(pattern, "*") && strings.HasSuffix(pattern, "*") {
		middle := strings.Trim(pattern, "*")
		return strings.Contains(actualValue, middle)
	}

	return false
}
