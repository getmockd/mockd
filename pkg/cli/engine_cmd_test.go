package cli

import (
	"testing"

	"github.com/getmockd/mockd/pkg/config"
)

func TestComputeConfigHash(t *testing.T) {
	collection := &config.MockCollection{
		Version: "1.0",
		Name:    "test",
	}

	hash := computeConfigHash(collection)
	if len(hash) < 10 {
		t.Errorf("expected meaningful hash, got %q", hash)
	}
	if hash[:7] != "sha256:" {
		t.Errorf("expected sha256: prefix, got %q", hash)
	}

	// Same input should produce same hash
	hash2 := computeConfigHash(collection)
	if hash != hash2 {
		t.Error("same input should produce same hash")
	}
}

func TestComputeConfigHash_DifferentInputsDifferentHash(t *testing.T) {
	c1 := &config.MockCollection{Version: "1.0", Name: "a"}
	c2 := &config.MockCollection{Version: "1.0", Name: "b"}

	h1 := computeConfigHash(c1)
	h2 := computeConfigHash(c2)

	if h1 == h2 {
		t.Error("different configs should produce different hashes")
	}
}
