package matching

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMatchPathPattern(t *testing.T) {
	tests := []struct {
		name            string
		pattern         string
		path            string
		wantScore       int
		wantCaptures    map[string]string
		wantCaptureKeys []string // For verifying specific keys exist
	}{
		{
			name:         "simple regex match",
			pattern:      `^/api/users/\d+$`,
			path:         "/api/users/123",
			wantScore:    14,
			wantCaptures: map[string]string{},
		},
		{
			name:         "no match",
			pattern:      `^/api/users/\d+$`,
			path:         "/api/users/abc",
			wantScore:    0,
			wantCaptures: nil,
		},
		{
			name:      "named capture group",
			pattern:   `^/api/users/(?P<id>\d+)$`,
			path:      "/api/users/456",
			wantScore: 14,
			wantCaptures: map[string]string{
				"id": "456",
			},
		},
		{
			name:      "multiple named capture groups",
			pattern:   `^/api/(?P<resource>\w+)/(?P<id>\d+)/(?P<action>\w+)$`,
			path:      "/api/users/789/edit",
			wantScore: 14,
			wantCaptures: map[string]string{
				"resource": "users",
				"id":       "789",
				"action":   "edit",
			},
		},
		{
			name:         "partial match without anchors",
			pattern:      `/users/\d+`,
			path:         "/api/users/123/profile",
			wantScore:    14,
			wantCaptures: map[string]string{},
		},
		{
			name:         "invalid regex pattern",
			pattern:      `[invalid`,
			path:         "/api/users/123",
			wantScore:    0,
			wantCaptures: nil,
		},
		{
			name:         "empty pattern",
			pattern:      "",
			path:         "/api/users/123",
			wantScore:    0,
			wantCaptures: nil,
		},
		{
			name:         "complex regex with optional groups",
			pattern:      `^/api/v(\d+)/users(?:/(\d+))?$`,
			path:         "/api/v2/users/123",
			wantScore:    14,
			wantCaptures: map[string]string{},
		},
		{
			name:         "regex with alternation",
			pattern:      `^/api/(users|products)/\d+$`,
			path:         "/api/products/999",
			wantScore:    14,
			wantCaptures: map[string]string{},
		},
		{
			name:      "named capture with special characters",
			pattern:   `^/api/items/(?P<slug>[\w-]+)$`,
			path:      "/api/items/my-cool-item",
			wantScore: 14,
			wantCaptures: map[string]string{
				"slug": "my-cool-item",
			},
		},
		{
			name:      "UUID pattern capture",
			pattern:   `^/api/orders/(?P<uuid>[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12})$`,
			path:      "/api/orders/550e8400-e29b-41d4-a716-446655440000",
			wantScore: 14,
			wantCaptures: map[string]string{
				"uuid": "550e8400-e29b-41d4-a716-446655440000",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score, captures := MatchPathPattern(tt.pattern, tt.path)

			assert.Equal(t, tt.wantScore, score, "score mismatch")

			if tt.wantCaptures == nil {
				assert.Nil(t, captures)
			} else {
				require.NotNil(t, captures)
				assert.Equal(t, tt.wantCaptures, captures, "captures mismatch")
			}
		})
	}
}

func TestValidatePathPattern(t *testing.T) {
	tests := []struct {
		name    string
		pattern string
		wantErr bool
	}{
		{
			name:    "valid simple pattern",
			pattern: `^/api/users/\d+$`,
			wantErr: false,
		},
		{
			name:    "valid with named groups",
			pattern: `^/api/(?P<resource>\w+)/(?P<id>\d+)$`,
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
			name:    "invalid repetition",
			pattern: `a{2,1}`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidatePathPattern(tt.pattern)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestMatchPath(t *testing.T) {
	tests := []struct {
		name      string
		pattern   string
		path      string
		wantScore int
	}{
		{
			name:      "exact match",
			pattern:   "/api/users",
			path:      "/api/users",
			wantScore: 15,
		},
		{
			name:      "named param match",
			pattern:   "/api/users/{id}",
			path:      "/api/users/123",
			wantScore: 12,
		},
		{
			name:      "wildcard match",
			pattern:   "/api/users/*",
			path:      "/api/users/123",
			wantScore: 10,
		},
		{
			name:      "no match",
			pattern:   "/api/users",
			path:      "/api/products",
			wantScore: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score := MatchPath(tt.pattern, tt.path)
			assert.Equal(t, tt.wantScore, score)
		})
	}
}

func TestMatchPathVariable(t *testing.T) {
	tests := []struct {
		name     string
		pattern  string
		path     string
		expected map[string]string
	}{
		{
			name:    "single named param",
			pattern: "/users/{id}",
			path:    "/users/123",
			expected: map[string]string{
				"id": "123",
			},
		},
		{
			name:    "multiple named params",
			pattern: "/users/{userId}/posts/{postId}",
			path:    "/users/42/posts/99",
			expected: map[string]string{
				"userId": "42",
				"postId": "99",
			},
		},
		{
			name:    "single wildcard",
			pattern: "/api/*",
			path:    "/api/anything",
			expected: map[string]string{
				"0": "anything",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := MatchPathVariable(tt.pattern, tt.path)
			assert.Equal(t, tt.expected, result)
		})
	}
}
