// Package recording provides ULID generation for recording identifiers.
package recording

import (
	"time"

	"github.com/getmockd/mockd/internal/id"
)

// NewULID generates a new ULID (Universally Unique Lexicographically Sortable Identifier).
// ULIDs are 26 characters long, time-sortable, and collision-free.
// Delegates to internal/id.ULID().
func NewULID() string {
	return id.ULID()
}

// IsValidULID checks if a string is a valid ULID.
// Delegates to internal/id.IsValidULID().
func IsValidULID(s string) bool {
	return id.IsValidULID(s)
}

// ULIDTime extracts the timestamp from a ULID.
// Delegates to internal/id.ULIDTime().
func ULIDTime(ulid string) (time.Time, error) {
	return id.ULIDTime(ulid)
}
