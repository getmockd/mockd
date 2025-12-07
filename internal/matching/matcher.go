// Package matching provides request matching algorithms.
package matching

import (
	"net/http"
	"strings"

	"github.com/getmockd/mockd/pkg/config"
)

// MatchResult contains the result of matching a request against a mock.
type MatchResult struct {
	Mock    *config.MockConfiguration
	Score   int
	Matched bool
}

// MatchScore calculates the match score for a request against a matcher.
// Returns 0 if there's no match, higher scores indicate better matches.
func MatchScore(matcher *config.RequestMatcher, r *http.Request, body []byte) int {
	if matcher == nil {
		return 0
	}

	score := 0

	// Method matching (required if specified)
	if matcher.Method != "" {
		if !MatchMethod(matcher.Method, r.Method) {
			return 0 // Method mismatch = no match
		}
		score += 10
	}

	// Path matching (required if specified)
	if matcher.Path != "" {
		pathScore := MatchPath(matcher.Path, r.URL.Path)
		if pathScore == 0 {
			return 0 // Path mismatch = no match
		}
		score += pathScore
	}

	// Header matching
	for name, value := range matcher.Headers {
		if !MatchHeader(name, value, r.Header) {
			return 0 // All headers must match
		}
		score += 10
	}

	// Query param matching
	for name, value := range matcher.QueryParams {
		if !MatchQueryParam(name, value, r.URL.Query()) {
			return 0 // All query params must match
		}
		score += 5
	}

	// Body matching
	if matcher.BodyContains != "" || matcher.BodyEquals != "" {
		bodyScore := MatchBody(matcher.BodyContains, matcher.BodyEquals, body)
		if bodyScore == 0 {
			return 0 // Body must match if specified
		}
		score += bodyScore
	}

	return score
}

// MatchMethod checks if the request method matches.
func MatchMethod(expected, actual string) bool {
	return strings.EqualFold(expected, actual)
}
