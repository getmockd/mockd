package matching

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMatchBodyPattern(t *testing.T) {
	tests := []struct {
		name      string
		pattern   string
		body      []byte
		wantScore int
	}{
		{
			name:      "simple match",
			pattern:   `"email":\s*"[^"]+"`,
			body:      []byte(`{"email": "test@example.com"}`),
			wantScore: 22,
		},
		{
			name:      "no match",
			pattern:   `"email":\s*"[^"]+"`,
			body:      []byte(`{"name": "John"}`),
			wantScore: 0,
		},
		{
			name:      "empty pattern returns 0",
			pattern:   "",
			body:      []byte(`any content`),
			wantScore: 0,
		},
		{
			name:      "invalid regex pattern",
			pattern:   `[invalid`,
			body:      []byte(`any content`),
			wantScore: 0,
		},
		{
			name:      "match JSON structure",
			pattern:   `\{"id":\s*\d+,\s*"name":\s*"[^"]+"\}`,
			body:      []byte(`{"id": 123, "name": "Test"}`),
			wantScore: 22,
		},
		{
			name:      "match XML content",
			pattern:   `<user>.*</user>`,
			body:      []byte(`<user><name>John</name></user>`),
			wantScore: 22,
		},
		{
			name:      "case insensitive match with flag",
			pattern:   `(?i)error`,
			body:      []byte(`An ERROR occurred`),
			wantScore: 22,
		},
		{
			name:      "multiline match",
			pattern:   `(?s)start.*end`,
			body:      []byte("start\nmiddle\nend"),
			wantScore: 22,
		},
		{
			name:      "empty body no match",
			pattern:   `\w+`,
			body:      []byte(``),
			wantScore: 0,
		},
		{
			name:      "empty body with anchored pattern",
			pattern:   `^$`,
			body:      []byte(``),
			wantScore: 22,
		},
		{
			name:      "complex JSON validation",
			pattern:   `"status":\s*"(pending|approved|rejected)"`,
			body:      []byte(`{"id": 1, "status": "approved"}`),
			wantScore: 22,
		},
		{
			name:      "UUID pattern in body",
			pattern:   `[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`,
			body:      []byte(`{"id": "550e8400-e29b-41d4-a716-446655440000"}`),
			wantScore: 22,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score := MatchBodyPattern(tt.pattern, tt.body)
			assert.Equal(t, tt.wantScore, score)
		})
	}
}

func TestValidateBodyPattern(t *testing.T) {
	tests := []struct {
		name    string
		pattern string
		wantErr bool
	}{
		{
			name:    "valid pattern",
			pattern: `"email":\s*"[^"]+"`,
			wantErr: false,
		},
		{
			name:    "empty pattern is valid",
			pattern: "",
			wantErr: false,
		},
		{
			name:    "invalid unclosed bracket",
			pattern: `[invalid`,
			wantErr: true,
		},
		{
			name:    "invalid unclosed group",
			pattern: `(unclosed`,
			wantErr: true,
		},
		{
			name:    "valid complex pattern",
			pattern: `(?i)(?:error|warning|info):\s*.+`,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateBodyPattern(tt.pattern)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestMatchBody(t *testing.T) {
	tests := []struct {
		name      string
		contains  string
		equals    string
		body      []byte
		wantScore int
	}{
		{
			name:      "exact match",
			contains:  "",
			equals:    "exact body",
			body:      []byte("exact body"),
			wantScore: 25,
		},
		{
			name:      "contains match",
			contains:  "partial",
			equals:    "",
			body:      []byte("this is a partial match"),
			wantScore: 20,
		},
		{
			name:      "no criteria returns 1",
			contains:  "",
			equals:    "",
			body:      []byte("any content"),
			wantScore: 1,
		},
		{
			name:      "equals no match",
			contains:  "",
			equals:    "expected",
			body:      []byte("actual"),
			wantScore: 0,
		},
		{
			name:      "contains no match",
			contains:  "needle",
			equals:    "",
			body:      []byte("haystack"),
			wantScore: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score := MatchBody(tt.contains, tt.equals, tt.body)
			assert.Equal(t, tt.wantScore, score)
		})
	}
}

func TestMatchBodyContains(t *testing.T) {
	tests := []struct {
		name     string
		body     []byte
		contains string
		want     bool
	}{
		{"contains substring", []byte("hello world"), "world", true},
		{"empty contains matches", []byte("anything"), "", true},
		{"not found", []byte("hello"), "world", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := MatchBodyContains(tt.body, tt.contains)
			assert.Equal(t, tt.want, result)
		})
	}
}

func TestMatchBodyEquals(t *testing.T) {
	tests := []struct {
		name     string
		body     []byte
		expected string
		want     bool
	}{
		{"exact match", []byte("exact"), "exact", true},
		{"empty expected matches", []byte("anything"), "", true},
		{"not equal", []byte("actual"), "expected", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := MatchBodyEquals(tt.body, tt.expected)
			assert.Equal(t, tt.want, result)
		})
	}
}
