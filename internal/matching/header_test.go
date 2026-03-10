package matching

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMatchHeader(t *testing.T) {
	tests := []struct {
		name     string
		header   string
		expected string
		headers  http.Header
		want     bool
	}{
		{
			name:     "exact match",
			header:   "Content-Type",
			expected: "application/json",
			headers:  http.Header{"Content-Type": []string{"application/json"}},
			want:     true,
		},
		{
			name:     "case-insensitive key lookup",
			header:   "content-type",
			expected: "application/json",
			headers:  http.Header{"Content-Type": []string{"application/json"}},
			want:     true,
		},
		{
			name:     "case-insensitive key uppercase",
			header:   "CONTENT-TYPE",
			expected: "application/json",
			headers:  http.Header{"Content-Type": []string{"application/json"}},
			want:     true,
		},
		{
			name:     "value mismatch",
			header:   "Content-Type",
			expected: "text/html",
			headers:  http.Header{"Content-Type": []string{"application/json"}},
			want:     false,
		},
		{
			name:     "missing header",
			header:   "X-Custom-Header",
			expected: "some-value",
			headers:  http.Header{},
			want:     false,
		},
		{
			name:     "empty expected matches empty actual",
			header:   "X-Missing",
			expected: "",
			headers:  http.Header{},
			want:     true, // headers.Get returns "" for missing, which equals ""
		},
		{
			name:     "empty expected does not match present header",
			header:   "Content-Type",
			expected: "",
			headers:  http.Header{"Content-Type": []string{"application/json"}},
			want:     false,
		},
		{
			name:     "multi-value header returns first value via Get",
			header:   "Accept",
			expected: "text/html",
			headers:  http.Header{"Accept": []string{"text/html", "application/json"}},
			want:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MatchHeader(tt.header, tt.expected, tt.headers)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestMatchHeaders(t *testing.T) {
	tests := []struct {
		name     string
		expected map[string]string
		headers  http.Header
		want     bool
	}{
		{
			name:     "nil expected matches anything",
			expected: nil,
			headers:  http.Header{"Content-Type": []string{"application/json"}},
			want:     true,
		},
		{
			name:     "empty expected matches anything",
			expected: map[string]string{},
			headers:  http.Header{"Content-Type": []string{"application/json"}},
			want:     true,
		},
		{
			name: "single header present and matching",
			expected: map[string]string{
				"Content-Type": "application/json",
			},
			headers: http.Header{"Content-Type": []string{"application/json"}},
			want:    true,
		},
		{
			name: "single header present but wrong value",
			expected: map[string]string{
				"Content-Type": "text/xml",
			},
			headers: http.Header{"Content-Type": []string{"application/json"}},
			want:    false,
		},
		{
			name: "multiple headers all match",
			expected: map[string]string{
				"Content-Type":  "application/json",
				"Authorization": "Bearer abc123",
			},
			headers: http.Header{
				"Content-Type":  []string{"application/json"},
				"Authorization": []string{"Bearer abc123"},
			},
			want: true,
		},
		{
			name: "multiple headers one missing",
			expected: map[string]string{
				"Content-Type": "application/json",
				"X-Api-Key":    "secret",
			},
			headers: http.Header{
				"Content-Type": []string{"application/json"},
			},
			want: false,
		},
		{
			name: "multiple headers one wrong value",
			expected: map[string]string{
				"Content-Type": "application/json",
				"X-Api-Key":    "secret",
			},
			headers: http.Header{
				"Content-Type": []string{"application/json"},
				"X-Api-Key":    []string{"wrong"},
			},
			want: false,
		},
		{
			name: "case-insensitive key matching across multiple headers",
			expected: map[string]string{
				"content-type": "application/json",
				"x-api-key":    "secret",
			},
			headers: http.Header{
				"Content-Type": []string{"application/json"},
				"X-Api-Key":    []string{"secret"},
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MatchHeaders(tt.expected, tt.headers)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestMatchHeaderPattern(t *testing.T) {
	tests := []struct {
		name    string
		header  string
		pattern string
		headers http.Header
		want    bool
	}{
		// Exact match (no wildcards)
		{
			name:    "exact match",
			header:  "Content-Type",
			pattern: "application/json",
			headers: http.Header{"Content-Type": []string{"application/json"}},
			want:    true,
		},
		{
			name:    "exact match failure",
			header:  "Content-Type",
			pattern: "text/html",
			headers: http.Header{"Content-Type": []string{"application/json"}},
			want:    false,
		},

		// Prefix match (pattern*)
		{
			name:    "prefix glob: Bearer*",
			header:  "Authorization",
			pattern: "Bearer*",
			headers: http.Header{"Authorization": []string{"Bearer abc123"}},
			want:    true,
		},
		{
			name:    "prefix glob: Bearer* no match",
			header:  "Authorization",
			pattern: "Bearer*",
			headers: http.Header{"Authorization": []string{"Basic dXNlcjpwYXNz"}},
			want:    false,
		},
		{
			name:    "prefix glob: application/*",
			header:  "Content-Type",
			pattern: "application/*",
			headers: http.Header{"Content-Type": []string{"application/json"}},
			want:    true,
		},

		// Suffix match (*pattern)
		{
			name:    "suffix glob: *json",
			header:  "Content-Type",
			pattern: "*json",
			headers: http.Header{"Content-Type": []string{"application/json"}},
			want:    true,
		},
		{
			name:    "suffix glob: *xml",
			header:  "Content-Type",
			pattern: "*xml",
			headers: http.Header{"Content-Type": []string{"application/json"}},
			want:    false,
		},
		{
			name:    "suffix glob: *xml matches text/xml",
			header:  "Accept",
			pattern: "*xml",
			headers: http.Header{"Accept": []string{"text/xml"}},
			want:    true,
		},

		// Contains match (*pattern*)
		{
			name:    "contains glob: *text* matches text/plain",
			header:  "Accept",
			pattern: "*text*",
			headers: http.Header{"Accept": []string{"text/plain"}},
			want:    true,
		},
		{
			name:    "contains glob: *text* matches application/text+xml",
			header:  "Content-Type",
			pattern: "*text*",
			headers: http.Header{"Content-Type": []string{"application/text+xml"}},
			want:    true,
		},
		{
			name:    "contains glob: *json* matches application/json",
			header:  "Content-Type",
			pattern: "*json*",
			headers: http.Header{"Content-Type": []string{"application/json"}},
			want:    true,
		},
		{
			name:    "contains glob: *xml* no match",
			header:  "Content-Type",
			pattern: "*xml*",
			headers: http.Header{"Content-Type": []string{"application/json"}},
			want:    false,
		},

		// Missing header always returns false
		{
			name:    "missing header returns false even with wildcard",
			header:  "X-Missing",
			pattern: "*",
			headers: http.Header{},
			want:    false,
		},
		{
			name:    "missing header returns false with exact pattern",
			header:  "X-Missing",
			pattern: "value",
			headers: http.Header{},
			want:    false,
		},

		// Case-insensitive key
		{
			name:    "case-insensitive key with glob",
			header:  "authorization",
			pattern: "Bearer*",
			headers: http.Header{"Authorization": []string{"Bearer tok"}},
			want:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MatchHeaderPattern(tt.header, tt.pattern, tt.headers)
			assert.Equal(t, tt.want, got)
		})
	}
}
