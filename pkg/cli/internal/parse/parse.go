// Package parse provides string parsing utilities for CLI commands.
package parse

import "strings"

// Consolidates header parsing from:
// - websocket.go:splitHeader (629-636)
// - graphql.go:splitGraphQLHeaders (264-277)
// - soap.go:splitSOAPHeaders (387-400)
// - add.go:parseKeyValue (731-737)

// KeyValue parses a "key:value" or "key=value" string.
// If delimiters are provided, uses the first one found; otherwise defaults to ':'.
// Returns the key, value, and a boolean indicating success.
func KeyValue(s string, delimiters ...rune) (key, value string, ok bool) {
	if len(delimiters) == 0 {
		delimiters = []rune{':'}
	}

	for i, c := range s {
		for _, d := range delimiters {
			if c == d {
				return s[:i], s[i+1:], true
			}
		}
	}
	return "", "", false
}

// Headers parses a slice of "key:value" strings into a map.
// Values are trimmed of leading/trailing whitespace.
func Headers(headers []string) map[string]string {
	result := make(map[string]string)
	for _, h := range headers {
		if key, value, ok := KeyValue(h, ':'); ok {
			result[key] = strings.TrimSpace(value)
		}
	}
	return result
}

// HeaderParts splits a header string into [key, value] for use with http.Header.
// Returns a slice with one element (just key) if no delimiter found.
func HeaderParts(s string) []string {
	for i := 0; i < len(s); i++ {
		if s[i] == ':' {
			return []string{s[:i], strings.TrimSpace(s[i+1:])}
		}
	}
	return []string{s}
}

// SplitTrim splits a string by separator and trims each part.
func SplitTrim(s, sep string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, sep)
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		trimmed := strings.TrimSpace(p)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

// SplitHeaders splits a comma-separated header string into individual headers.
// Handles empty segments gracefully.
func SplitHeaders(s string) []string {
	var headers []string
	start := 0
	for i := 0; i <= len(s); i++ {
		if i == len(s) || s[i] == ',' {
			header := s[start:i]
			if header != "" {
				headers = append(headers, header)
			}
			start = i + 1
		}
	}
	return headers
}
