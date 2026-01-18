package matching

import (
	"regexp"
	"strings"
)

// MatchBody checks if the request body matches the criteria.
// Returns a score > 0 if matched, 0 if not matched.
func MatchBody(contains, equals string, body []byte) int {
	bodyStr := string(body)

	// Exact body match
	if equals != "" {
		if bodyStr == equals {
			return ScoreBodyEquals
		}
		return 0
	}

	// Contains match
	if contains != "" {
		if strings.Contains(bodyStr, contains) {
			return ScoreBodyContains
		}
		return 0
	}

	// No body criteria specified - always matches
	return ScoreBodyNoCriteria
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

// MatchBodyPattern checks if the request body matches a regex pattern.
// Returns a score > 0 if matched, 0 if not matched.
// The score is ScoreBodyPattern (between contains and equals).
// Uses Go's regexp package with RE2 syntax.
func MatchBodyPattern(pattern string, body []byte) int {
	if pattern == "" {
		return 0
	}

	// Compile the regex pattern
	re, err := regexp.Compile(pattern)
	if err != nil {
		// Invalid regex pattern - gracefully return no match
		return 0
	}

	// Check if the body matches the pattern
	if re.Match(body) {
		return ScoreBodyPattern
	}

	return 0
}

// ValidateBodyPattern checks if a regex pattern is valid.
// Returns an error if the pattern cannot be compiled.
func ValidateBodyPattern(pattern string) error {
	if pattern == "" {
		return nil
	}
	_, err := regexp.Compile(pattern)
	return err
}
