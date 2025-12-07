package stateful

import (
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// T037: ID generation produces unique UUIDs across 10,000 calls
func TestGenerateID_UniqueAcross10000Calls(t *testing.T) {
	seen := make(map[string]bool)
	count := 10000

	for i := 0; i < count; i++ {
		id := GenerateID()

		// Check for collision
		if seen[id] {
			t.Fatalf("collision detected at iteration %d: %s", i, id)
		}
		seen[id] = true
	}

	assert.Equal(t, count, len(seen), "should have generated %d unique IDs", count)
}

func TestGenerateID_ValidUUIDFormat(t *testing.T) {
	// UUID v4 format: xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx
	// where y is one of 8, 9, a, or b
	uuidRegex := regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

	for i := 0; i < 100; i++ {
		id := GenerateID()
		require.Regexp(t, uuidRegex, id, "ID should be valid UUID v4 format: %s", id)
	}
}

func TestGenerateID_Deterministic(t *testing.T) {
	// Verify that each call produces a different ID (not deterministic)
	id1 := GenerateID()
	id2 := GenerateID()
	id3 := GenerateID()

	assert.NotEqual(t, id1, id2, "consecutive IDs should be different")
	assert.NotEqual(t, id2, id3, "consecutive IDs should be different")
	assert.NotEqual(t, id1, id3, "consecutive IDs should be different")
}

func TestGenerateID_NotEmpty(t *testing.T) {
	id := GenerateID()
	assert.NotEmpty(t, id, "generated ID should not be empty")
	assert.Len(t, id, 36, "UUID should be 36 characters (with hyphens)")
}
