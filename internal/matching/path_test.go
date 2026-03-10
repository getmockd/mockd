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

// ── Param with literal suffix/prefix tests ───────────────────────────────────

func TestMatchPath_ParamWithSuffix(t *testing.T) {
	tests := []struct {
		name      string
		pattern   string
		path      string
		wantMatch bool
	}{
		{
			name:      "param with .json suffix",
			pattern:   "/api/users/{id}.json",
			path:      "/api/users/123.json",
			wantMatch: true,
		},
		{
			name:      "Twilio-style message fetch",
			pattern:   "/2010-04-01/Accounts/{AccountSid}/Messages/{Sid}.json",
			path:      "/2010-04-01/Accounts/AC_test/Messages/SM2a0b1c2d3e4f5a6b7c8d9e0f1a2b3c4d.json",
			wantMatch: true,
		},
		{
			name:      "Twilio-style message list",
			pattern:   "/2010-04-01/Accounts/{AccountSid}/Messages.json",
			path:      "/2010-04-01/Accounts/AC_test/Messages.json",
			wantMatch: true,
		},
		{
			name:      "param with .xml suffix",
			pattern:   "/api/users/{id}.xml",
			path:      "/api/users/123.xml",
			wantMatch: true,
		},
		{
			name:      "suffix mismatch",
			pattern:   "/api/users/{id}.json",
			path:      "/api/users/123.xml",
			wantMatch: false,
		},
		{
			name:      "prefix v and param",
			pattern:   "/api/v{version}/users",
			path:      "/api/v2/users",
			wantMatch: true,
		},
		{
			name:      "no suffix in actual path",
			pattern:   "/api/users/{id}.json",
			path:      "/api/users/123",
			wantMatch: false,
		},
		{
			name:      "multiple params with suffix",
			pattern:   "/api/{org}/{repo}.git",
			path:      "/api/acme/widget.git",
			wantMatch: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score := MatchPath(tt.pattern, tt.path)
			if tt.wantMatch {
				assert.Greater(t, score, 0, "expected match")
			} else {
				assert.Equal(t, 0, score, "expected no match")
			}
		})
	}
}

func TestMatchPathVariable_ParamWithSuffix(t *testing.T) {
	tests := []struct {
		name     string
		pattern  string
		path     string
		expected map[string]string
	}{
		{
			name:    "extract param before .json suffix",
			pattern: "/api/users/{id}.json",
			path:    "/api/users/123.json",
			expected: map[string]string{
				"id": "123",
			},
		},
		{
			name:    "Twilio message fetch — extract AccountSid and Sid",
			pattern: "/2010-04-01/Accounts/{AccountSid}/Messages/{Sid}.json",
			path:    "/2010-04-01/Accounts/AC_test/Messages/SM2a0b1c2d3e4f5a6b7c8d9e0f1a2b3c4d.json",
			expected: map[string]string{
				"AccountSid": "AC_test",
				"Sid":        "SM2a0b1c2d3e4f5a6b7c8d9e0f1a2b3c4d",
			},
		},
		{
			name:    "version prefix",
			pattern: "/api/v{version}/users",
			path:    "/api/v2/users",
			expected: map[string]string{
				"version": "2",
			},
		},
		{
			name:    "param with .git suffix",
			pattern: "/repos/{org}/{repo}.git",
			path:    "/repos/acme/widget.git",
			expected: map[string]string{
				"org":  "acme",
				"repo": "widget",
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
