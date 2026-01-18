package matching

import (
	"net/url"
)

// MatchQueryParam checks if a specific query parameter matches.
func MatchQueryParam(name, expectedValue string, params url.Values) bool {
	actualValue := params.Get(name)
	return actualValue == expectedValue
}

// MatchQueryParams checks if all specified query parameters match.
// Returns true only if ALL parameters match.
func MatchQueryParams(expected map[string]string, params url.Values) bool {
	for name, value := range expected {
		if !MatchQueryParam(name, value, params) {
			return false
		}
	}
	return true
}

// HasQueryParam checks if a query parameter exists (regardless of value).
func HasQueryParam(name string, params url.Values) bool {
	_, exists := params[name]
	return exists
}
