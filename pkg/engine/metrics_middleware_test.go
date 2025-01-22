package engine

import "testing"

func TestNormalizePathForMetrics(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		// Numeric IDs
		{
			name:     "numeric ID in path",
			input:    "/users/123",
			expected: "/users/{id}",
		},
		{
			name:     "multiple numeric IDs",
			input:    "/users/123/posts/456",
			expected: "/users/{id}/posts/{id}",
		},
		{
			name:     "long numeric ID",
			input:    "/orders/9876543210",
			expected: "/orders/{id}",
		},

		// UUIDs
		{
			name:     "UUID in path",
			input:    "/items/a1b2c3d4-e5f6-7890-abcd-ef1234567890",
			expected: "/items/{uuid}",
		},
		{
			name:     "uppercase UUID",
			input:    "/items/A1B2C3D4-E5F6-7890-ABCD-EF1234567890",
			expected: "/items/{uuid}",
		},
		{
			name:     "UUID with prefix path",
			input:    "/api/v1/users/a1b2c3d4-e5f6-7890-abcd-ef1234567890/profile",
			expected: "/api/v1/users/{uuid}/profile",
		},

		// MongoDB ObjectIDs (24 hex chars)
		{
			name:     "MongoDB ObjectID",
			input:    "/documents/507f1f77bcf86cd799439011",
			expected: "/documents/{id}",
		},

		// Short hex strings are NOT normalized (too risky - could match legitimate paths)
		// Only UUIDs, ObjectIDs, and numeric IDs are normalized
		{
			name:     "short hex NOT normalized (could be legitimate)",
			input:    "/sessions/abc123def",
			expected: "/sessions/abc123def",
		},
		{
			name:     "medium hex NOT normalized (could be legitimate)",
			input:    "/tokens/abc123def456789a",
			expected: "/tokens/abc123def456789a",
		},

		// No normalization needed
		{
			name:     "static path",
			input:    "/api/v1/users",
			expected: "/api/v1/users",
		},
		{
			name:     "root path",
			input:    "/",
			expected: "/",
		},
		{
			name:     "named resource",
			input:    "/users/admin",
			expected: "/users/admin",
		},
		{
			name:     "query params not affected",
			input:    "/search?q=test",
			expected: "/search?q=test",
		},

		// Mixed scenarios
		{
			name:     "mixed IDs",
			input:    "/orgs/123/projects/a1b2c3d4-e5f6-7890-abcd-ef1234567890/tasks/456",
			expected: "/orgs/{id}/projects/{uuid}/tasks/{id}",
		},
		{
			name:     "API versioning preserved",
			input:    "/api/v2/users/123",
			expected: "/api/v2/users/{id}",
		},

		// Edge cases
		{
			name:     "empty path",
			input:    "",
			expected: "",
		},
		{
			name:     "path with trailing slash",
			input:    "/users/123/",
			expected: "/users/{id}/",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := normalizePathForMetrics(tt.input)
			if result != tt.expected {
				t.Errorf("normalizePathForMetrics(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func BenchmarkNormalizePathForMetrics(b *testing.B) {
	paths := []string{
		"/api/v1/users",
		"/users/123/posts/456",
		"/items/a1b2c3d4-e5f6-7890-abcd-ef1234567890",
		"/documents/507f1f77bcf86cd799439011/comments/789",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, p := range paths {
			normalizePathForMetrics(p)
		}
	}
}
