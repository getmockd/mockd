// Package flags provides reusable flag types for CLI commands.
package flags

import "strings"

// StringSlice implements flag.Value for repeatable string flags.
// Consolidates: add.go:stringSliceFlag, websocket.go:headerFlags
type StringSlice []string

// String returns the string representation of the flag value.
func (s *StringSlice) String() string {
	return strings.Join(*s, ",")
}

// Set appends a value to the slice.
func (s *StringSlice) Set(value string) error {
	*s = append(*s, value)
	return nil
}

// Type specifies the type label for Cobra flags.
func (s *StringSlice) Type() string {
	return "stringSlice"
}
