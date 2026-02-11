package util

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSafeFilePath(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		wantPath string
		wantOK   bool
	}{
		// Valid relative paths
		{"simple relative", "data/test.json", "data/test.json", true},
		{"single file", "test.json", "test.json", true},
		{"nested relative", "a/b/c/file.txt", "a/b/c/file.txt", true},
		{"dot prefix", "./data/test.json", "data/test.json", true},
		{"current dir dot", ".", ".", true},

		// Path traversal attacks — must reject (.. remains after Clean)
		{"simple traversal", "../secret.json", "", false},
		{"double traversal", "../../etc/passwd", "", false},
		{"nested traversal", "data/../../etc/passwd", "", false},
		{"dot-dot only", "..", "", false},
		{"dot-slash-dot-dot", "./..", "", false},
		{"traversal with trailing slash", "../", "", false},

		// Paths with .. that resolve safely after filepath.Clean — allowed
		// "data/.." cleans to "." (current dir, no escape)
		{"traversal resolves to dot", "data/..", ".", true},
		// "a/b/c/../../../etc/passwd" cleans to "etc/passwd" (relative, no escape)
		{"deep traversal resolves safely", "a/b/c/../../../etc/passwd", "etc/passwd", true},

		// Absolute paths — must reject
		{"absolute unix", "/etc/passwd", "", false},
		{"absolute nested", "/var/data/file.json", "", false},
		{"absolute root", "/", "", false},

		// Empty — must reject
		{"empty string", "", "", false},

		// Edge cases
		{"double slash", "data//test.json", "data/test.json", true},
		{"trailing slash", "data/", "data", true},
		{"dot segments", "data/./test.json", "data/test.json", true},
		{"backslash traversal", `data\..\secret`, "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			gotPath, gotOK := SafeFilePath(tt.input)
			assert.Equal(t, tt.wantOK, gotOK, "SafeFilePath(%q) ok", tt.input)
			assert.Equal(t, tt.wantPath, gotPath, "SafeFilePath(%q) path", tt.input)
		})
	}
}

func TestSafeFilePathAllowAbsolute(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		wantPath string
		wantOK   bool
	}{
		// Valid relative paths
		{"simple relative", "data/test.json", "data/test.json", true},
		{"single file", "test.json", "test.json", true},
		{"dot prefix", "./data/test.json", "data/test.json", true},

		// Valid absolute paths — allowed by this variant
		{"absolute unix", "/etc/schemas/spec.json", "/etc/schemas/spec.json", true},
		{"absolute nested", "/var/data/file.proto", "/var/data/file.proto", true},
		{"absolute root", "/", "/", true},

		// Path traversal attacks — must still reject (.. remains after Clean)
		{"simple traversal", "../secret.json", "", false},
		{"double traversal", "../../etc/passwd", "", false},
		{"nested traversal", "data/../../etc/passwd", "", false},
		{"dot-dot only", "..", "", false},

		// Paths with .. that resolve safely after filepath.Clean — allowed
		// "/var/data/../../../etc/passwd" cleans to "/etc/passwd" (absolute, but allowed)
		{"absolute with resolved traversal", "/var/data/../../../etc/passwd", "/etc/passwd", true},
		// "data/.." cleans to "." (current dir, no escape)
		{"traversal resolves to dot", "data/..", ".", true},
		// "/.." cleans to "/" (root, no escape)
		{"absolute dot-dot resolves to root", "/..", "/", true},
		// "a/b/c/../../../etc/passwd" cleans to "etc/passwd" (relative, no escape)
		{"deep traversal resolves safely", "a/b/c/../../../etc/passwd", "etc/passwd", true},

		// Empty — must reject
		{"empty string", "", "", false},

		// Edge cases
		{"double slash", "data//test.json", "data/test.json", true},
		{"absolute double slash", "/var//data/file.json", "/var/data/file.json", true},
		{"backslash traversal", `data\..\secret`, "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			gotPath, gotOK := SafeFilePathAllowAbsolute(tt.input)
			assert.Equal(t, tt.wantOK, gotOK, "SafeFilePathAllowAbsolute(%q) ok", tt.input)
			assert.Equal(t, tt.wantPath, gotPath, "SafeFilePathAllowAbsolute(%q) path", tt.input)
		})
	}
}

func TestTruncateBody(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		data    string
		maxSize int
		want    string
	}{
		{"short string no truncation", "hello", 100, "hello"},
		{"exact length", "12345", 5, "12345"},
		{"one over", "123456", 5, "12345...(truncated)"},
		{"zero maxSize uses default", "hello", 0, "hello"},
		{"negative maxSize uses default", "hello", -1, "hello"},
		{"empty string", "", 10, ""},
		{"large truncation", "abcdefghij", 3, "abc...(truncated)"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := TruncateBody(tt.data, tt.maxSize)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestTruncateBody_DefaultMaxSize(t *testing.T) {
	t.Parallel()

	// MaxLogBodySize is 10KB (10240 bytes)
	data := make([]byte, MaxLogBodySize+100)
	for i := range data {
		data[i] = 'x'
	}

	result := TruncateBody(string(data), 0)
	assert.Equal(t, MaxLogBodySize+len("...(truncated)"), len(result))
	assert.Contains(t, result, "...(truncated)")

	// Under the limit — no truncation
	shortData := string(data[:MaxLogBodySize])
	result2 := TruncateBody(shortData, 0)
	assert.Equal(t, shortData, result2)
}
