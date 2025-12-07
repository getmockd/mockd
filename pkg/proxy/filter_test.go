package proxy

import "testing"

// TestMatchGlob tests glob pattern matching.
func TestMatchGlob(t *testing.T) {
	tests := []struct {
		pattern string
		input   string
		want    bool
	}{
		// Exact match
		{"/api/users", "/api/users", true},
		{"/api/users", "/api/items", false},

		// Wildcard at end
		{"/api/*", "/api/users", true},
		{"/api/*", "/api/users/123", true},
		{"/api/*", "/other/path", false},

		// Wildcard at start
		{"*/users", "/api/users", true},
		{"*/users", "/v1/api/users", true},
		{"*/users", "/api/items", false},

		// Wildcard in middle
		{"/api/*/details", "/api/users/details", true},
		{"/api/*/details", "/api/items/details", true},
		{"/api/*/details", "/api/users/summary", false},

		// Multiple wildcards
		{"/*/users/*", "/api/users/123", true},
		{"/*/users/*", "/v1/users/abc", true},
		{"/*/items/*", "/api/users/123", false},

		// Single wildcard matches everything
		{"*", "/anything", true},
		{"*", "", true},

		// Edge cases
		{"", "", true},
		{"", "/path", false},
		{"/path", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.pattern+"_vs_"+tt.input, func(t *testing.T) {
			got := matchGlob(tt.pattern, tt.input)
			if got != tt.want {
				t.Errorf("matchGlob(%q, %q) = %v, want %v", tt.pattern, tt.input, got, tt.want)
			}
		})
	}
}

// TestShouldRecord tests filter logic with include/exclude precedence.
func TestShouldRecord(t *testing.T) {
	t.Run("empty filter records everything", func(t *testing.T) {
		f := NewFilterConfig()
		if !f.ShouldRecord("example.com", "/api/users") {
			t.Error("Empty filter should record all requests")
		}
	})

	t.Run("exclude paths take precedence", func(t *testing.T) {
		f := &FilterConfig{
			IncludePaths: []string{"/api/*"},
			ExcludePaths: []string{"/api/health"},
		}

		// Included by pattern
		if !f.ShouldRecord("example.com", "/api/users") {
			t.Error("Should record /api/users")
		}

		// Excluded even though it matches include
		if f.ShouldRecord("example.com", "/api/health") {
			t.Error("Should NOT record /api/health (excluded)")
		}
	})

	t.Run("include paths filter when set", func(t *testing.T) {
		f := &FilterConfig{
			IncludePaths: []string{"/api/*"},
		}

		if !f.ShouldRecord("example.com", "/api/users") {
			t.Error("Should record /api/users")
		}

		if f.ShouldRecord("example.com", "/other/path") {
			t.Error("Should NOT record /other/path (not included)")
		}
	})

	t.Run("host exclusion blocks all paths", func(t *testing.T) {
		f := &FilterConfig{
			ExcludeHosts: []string{"internal.example.com"},
		}

		if f.ShouldRecord("internal.example.com", "/api/users") {
			t.Error("Should NOT record from excluded host")
		}

		if !f.ShouldRecord("api.example.com", "/api/users") {
			t.Error("Should record from non-excluded host")
		}
	})

	t.Run("host inclusion filters when set", func(t *testing.T) {
		f := &FilterConfig{
			IncludeHosts: []string{"api.example.com", "*.prod.example.com"},
		}

		if !f.ShouldRecord("api.example.com", "/api/users") {
			t.Error("Should record from included host")
		}

		if !f.ShouldRecord("app.prod.example.com", "/api/users") {
			t.Error("Should record from included host pattern")
		}

		if f.ShouldRecord("other.example.com", "/api/users") {
			t.Error("Should NOT record from non-included host")
		}
	})

	t.Run("combined host and path filters", func(t *testing.T) {
		f := &FilterConfig{
			IncludeHosts: []string{"api.example.com"},
			IncludePaths: []string{"/api/*"},
			ExcludePaths: []string{"/api/internal/*"},
		}

		// Correct host and path
		if !f.ShouldRecord("api.example.com", "/api/users") {
			t.Error("Should record matching host and path")
		}

		// Wrong host
		if f.ShouldRecord("other.example.com", "/api/users") {
			t.Error("Should NOT record wrong host")
		}

		// Wrong path
		if f.ShouldRecord("api.example.com", "/other/path") {
			t.Error("Should NOT record wrong path")
		}

		// Excluded path
		if f.ShouldRecord("api.example.com", "/api/internal/admin") {
			t.Error("Should NOT record excluded path")
		}
	})

	t.Run("exclude host takes precedence over include", func(t *testing.T) {
		f := &FilterConfig{
			IncludeHosts: []string{"*.example.com"},
			ExcludeHosts: []string{"internal.example.com"},
		}

		if !f.ShouldRecord("api.example.com", "/path") {
			t.Error("Should record from included host")
		}

		if f.ShouldRecord("internal.example.com", "/path") {
			t.Error("Should NOT record from excluded host")
		}
	})
}

// TestFilterConfigDefaults tests default filter behavior.
func TestFilterConfigDefaults(t *testing.T) {
	f := NewFilterConfig()

	if len(f.IncludePaths) != 0 {
		t.Error("Default IncludePaths should be empty")
	}
	if len(f.ExcludePaths) != 0 {
		t.Error("Default ExcludePaths should be empty")
	}
	if len(f.IncludeHosts) != 0 {
		t.Error("Default IncludeHosts should be empty")
	}
	if len(f.ExcludeHosts) != 0 {
		t.Error("Default ExcludeHosts should be empty")
	}
}
