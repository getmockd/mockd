// Package runtime provides the control plane client for runtime mode.
package runtime

import (
	"errors"
	"fmt"
	"strings"
)

// MockdURIScheme is the URI scheme for mockd resources.
const MockdURIScheme = "mockd://"

// MockdURI represents a parsed mockd:// URI.
type MockdURI struct {
	Workspace  string
	Collection string
	Version    string // Optional: v1.0, main, latest
}

// ParseMockdURI parses a mockd:// URI.
// Format: mockd://workspace/collection[@version]
// Examples:
//   - mockd://acme/payment-api
//   - mockd://acme/payment-api@v1.0
//   - mockd://acme/payment-api@main
func ParseMockdURI(uri string) (*MockdURI, error) {
	if !strings.HasPrefix(uri, MockdURIScheme) {
		return nil, fmt.Errorf("invalid URI scheme: expected %s prefix", MockdURIScheme)
	}

	// Remove scheme
	path := strings.TrimPrefix(uri, MockdURIScheme)
	if path == "" {
		return nil, errors.New("empty URI path")
	}

	// Split by @
	var version string
	if idx := strings.LastIndex(path, "@"); idx != -1 {
		version = path[idx+1:]
		path = path[:idx]
	}

	// Split by /
	parts := strings.SplitN(path, "/", 2)
	if len(parts) < 2 {
		return nil, errors.New("invalid URI format: expected mockd://workspace/collection[@version]")
	}

	workspace := parts[0]
	collection := parts[1]

	if workspace == "" {
		return nil, errors.New("workspace cannot be empty")
	}
	if collection == "" {
		return nil, errors.New("collection cannot be empty")
	}

	// Validate workspace (slug format)
	if !isValidSlug(workspace) {
		return nil, fmt.Errorf("invalid workspace slug: %s", workspace)
	}

	// Collection can have nested paths
	collectionParts := strings.Split(collection, "/")
	for _, part := range collectionParts {
		if part == "" {
			return nil, errors.New("invalid collection path: empty segment")
		}
	}

	return &MockdURI{
		Workspace:  workspace,
		Collection: collection,
		Version:    version,
	}, nil
}

// String returns the string representation of the URI.
func (u *MockdURI) String() string {
	s := MockdURIScheme + u.Workspace + "/" + u.Collection
	if u.Version != "" {
		s += "@" + u.Version
	}
	return s
}

// IsVersioned returns true if the URI specifies a version.
func (u *MockdURI) IsVersioned() bool {
	return u.Version != ""
}

// IsBranch returns true if the version looks like a branch name (not a semantic version).
func (u *MockdURI) IsBranch() bool {
	if u.Version == "" {
		return false
	}
	// Branches don't start with v followed by a digit
	if len(u.Version) >= 2 && u.Version[0] == 'v' && isDigit(u.Version[1]) {
		return false
	}
	return true
}

// IsSemanticVersion returns true if the version is a semantic version (vX.Y.Z).
func (u *MockdURI) IsSemanticVersion() bool {
	if u.Version == "" {
		return false
	}
	// Simple check: starts with v followed by a digit
	return len(u.Version) >= 2 && u.Version[0] == 'v' && isDigit(u.Version[1])
}

// isValidSlug checks if a string is a valid slug.
func isValidSlug(s string) bool {
	if len(s) < 3 || len(s) > 63 {
		return false
	}
	// Must start and end with alphanumeric
	if !isAlphanumeric(s[0]) || !isAlphanumeric(s[len(s)-1]) {
		return false
	}
	// Can only contain lowercase letters, numbers, and hyphens
	for _, c := range s {
		if !isLowerAlphanumeric(byte(c)) && c != '-' {
			return false
		}
	}
	return true
}

func isAlphanumeric(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9')
}

func isLowerAlphanumeric(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9')
}

func isDigit(c byte) bool {
	return c >= '0' && c <= '9'
}
