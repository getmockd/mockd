package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ============================================================================
// DefaultCORSConfig Tests
// ============================================================================

func TestDefaultCORSConfig(t *testing.T) {
	t.Parallel()

	cfg := DefaultCORSConfig()

	require.NotNil(t, cfg)
	assert.True(t, cfg.Enabled, "CORS should be enabled by default")
	assert.Equal(t, 86400, cfg.MaxAge, "default MaxAge should be 24 hours")
	assert.False(t, cfg.AllowCredentials, "credentials should not be enabled by default")

	// Should include localhost origins for development
	assert.Contains(t, cfg.AllowOrigins, "http://localhost:3000")
	assert.Contains(t, cfg.AllowOrigins, "http://localhost:4290")
	assert.Contains(t, cfg.AllowOrigins, "http://localhost:5173")
	assert.Contains(t, cfg.AllowOrigins, "http://127.0.0.1:3000")
	assert.Contains(t, cfg.AllowOrigins, "http://127.0.0.1:4290")
	assert.Contains(t, cfg.AllowOrigins, "http://127.0.0.1:5173")

	// Should have default methods
	assert.Equal(t, []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS", "HEAD"}, cfg.AllowMethods)

	// Should have default headers
	assert.Equal(t, []string{"Content-Type", "Authorization", "X-Requested-With", "Accept", "Origin"}, cfg.AllowHeaders)

	// Should not be wildcard
	assert.False(t, cfg.IsWildcard())
}

// ============================================================================
// IsWildcard Tests
// ============================================================================

func TestIsWildcard(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		cfg          *CORSConfig
		expectResult bool
	}{
		{
			name:         "wildcard origin",
			cfg:          &CORSConfig{AllowOrigins: []string{"*"}},
			expectResult: true,
		},
		{
			name:         "wildcard among specific origins",
			cfg:          &CORSConfig{AllowOrigins: []string{"http://example.com", "*"}},
			expectResult: true,
		},
		{
			name:         "specific origins only",
			cfg:          &CORSConfig{AllowOrigins: []string{"http://example.com", "http://other.com"}},
			expectResult: false,
		},
		{
			name:         "empty origins",
			cfg:          &CORSConfig{AllowOrigins: []string{}},
			expectResult: false,
		},
		{
			name:         "nil config",
			cfg:          nil,
			expectResult: false,
		},
		{
			name:         "nil origins slice",
			cfg:          &CORSConfig{},
			expectResult: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := tt.cfg.IsWildcard()
			assert.Equal(t, tt.expectResult, result)
		})
	}
}

// ============================================================================
// GetAllowOriginValue Tests
// ============================================================================

func TestGetAllowOriginValue(t *testing.T) {
	t.Parallel()

	t.Run("wildcard config returns asterisk", func(t *testing.T) {
		t.Parallel()
		cfg := &CORSConfig{
			Enabled:      true,
			AllowOrigins: []string{"*"},
		}
		result := cfg.GetAllowOriginValue("http://any-origin.com")
		assert.Equal(t, "*", result)
	})

	t.Run("specific origin in allow list returns that origin", func(t *testing.T) {
		t.Parallel()
		cfg := &CORSConfig{
			Enabled:      true,
			AllowOrigins: []string{"http://example.com", "http://other.com"},
		}
		result := cfg.GetAllowOriginValue("http://example.com")
		assert.Equal(t, "http://example.com", result)
	})

	t.Run("origin not in allow list returns empty", func(t *testing.T) {
		t.Parallel()
		cfg := &CORSConfig{
			Enabled:      true,
			AllowOrigins: []string{"http://example.com"},
		}
		result := cfg.GetAllowOriginValue("http://evil.com")
		assert.Empty(t, result)
	})

	t.Run("credentials with wildcard returns actual origin", func(t *testing.T) {
		t.Parallel()
		cfg := &CORSConfig{
			Enabled:          true,
			AllowOrigins:     []string{"*"},
			AllowCredentials: true,
		}
		result := cfg.GetAllowOriginValue("http://example.com")
		assert.Equal(t, "http://example.com", result, "should return actual origin, not * when credentials enabled")
	})

	t.Run("credentials with wildcard and empty origin returns empty", func(t *testing.T) {
		t.Parallel()
		cfg := &CORSConfig{
			Enabled:          true,
			AllowOrigins:     []string{"*"},
			AllowCredentials: true,
		}
		result := cfg.GetAllowOriginValue("")
		assert.Empty(t, result, "should return empty when credentials + wildcard but no origin provided")
	})

	t.Run("automatic localhost allowance", func(t *testing.T) {
		t.Parallel()
		cfg := &CORSConfig{
			Enabled:      true,
			AllowOrigins: []string{"http://example.com"}, // localhost NOT in explicit list
		}

		// localhost origins should always be allowed (mockd is a dev tool)
		assert.Equal(t, "http://localhost:8080", cfg.GetAllowOriginValue("http://localhost:8080"))
		assert.Equal(t, "http://127.0.0.1:9090", cfg.GetAllowOriginValue("http://127.0.0.1:9090"))
	})

	t.Run("non-localhost unlisted origin is rejected", func(t *testing.T) {
		t.Parallel()
		cfg := &CORSConfig{
			Enabled:      true,
			AllowOrigins: []string{"http://example.com"},
		}
		result := cfg.GetAllowOriginValue("http://attacker.com")
		assert.Empty(t, result)
	})

	t.Run("disabled config returns empty", func(t *testing.T) {
		t.Parallel()
		cfg := &CORSConfig{
			Enabled:      false,
			AllowOrigins: []string{"*"},
		}
		result := cfg.GetAllowOriginValue("http://example.com")
		assert.Empty(t, result, "disabled CORS should return empty for any origin")
	})

	t.Run("nil config returns empty", func(t *testing.T) {
		t.Parallel()
		var cfg *CORSConfig
		result := cfg.GetAllowOriginValue("http://example.com")
		assert.Empty(t, result)
	})
}
