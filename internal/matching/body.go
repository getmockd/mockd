package matching

import (
	"strings"
)

// MatchBody checks if the request body matches the criteria.
// Returns a score > 0 if matched, 0 if not matched.
func MatchBody(contains, equals string, body []byte) int {
	bodyStr := string(body)

	// Exact body match
	if equals != "" {
		if bodyStr == equals {
			return 25 // Highest score for exact body match
		}
		return 0
	}

	// Contains match
	if contains != "" {
		if strings.Contains(bodyStr, contains) {
			return 20 // High score for body contains
		}
		return 0
	}

	// No body criteria specified - always matches
	return 1
}

// MatchBodyContains checks if the body contains the substring.
func MatchBodyContains(body []byte, contains string) bool {
	if contains == "" {
		return true
	}
	return strings.Contains(string(body), contains)
}

// MatchBodyEquals checks if the body exactly equals the expected value.
func MatchBodyEquals(body []byte, expected string) bool {
	if expected == "" {
		return true
	}
	return string(body) == expected
}
