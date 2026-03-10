package engine

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/getmockd/mockd/pkg/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockChecker is a simple MockChecker stub for testing.
type mockChecker struct{ hasMatch bool }

func (m *mockChecker) HasMatch(_ *http.Request) bool { return m.hasMatch }

// dummyHandler is a simple handler that writes 200 OK with a known body.
func dummyHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("next handler"))
	})
}

// ============================================================================
// CORSMiddleware Tests
// ============================================================================

func TestCORSMiddleware_PreflightOptions(t *testing.T) {
	t.Parallel()

	cfg := &config.CORSConfig{
		Enabled:      true,
		AllowOrigins: []string{"*"},
		AllowMethods: []string{"GET", "POST"},
		AllowHeaders: []string{"Content-Type"},
		MaxAge:       3600,
	}
	m := NewCORSMiddleware(dummyHandler(), cfg, nil)

	req := httptest.NewRequest(http.MethodOptions, "/api/test", nil)
	req.Header.Set("Origin", "http://example.com")
	rec := httptest.NewRecorder()

	m.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code, "preflight should return 200")
	assert.Equal(t, "*", rec.Header().Get("Access-Control-Allow-Origin"))
	assert.Equal(t, "GET, POST", rec.Header().Get("Access-Control-Allow-Methods"))
	assert.Equal(t, "Content-Type", rec.Header().Get("Access-Control-Allow-Headers"))
	assert.Equal(t, "3600", rec.Header().Get("Access-Control-Max-Age"))
	// Body should be empty — preflight is intercepted, not passed to next handler
	assert.Empty(t, rec.Body.String(), "preflight should not pass through to next handler")
}

func TestCORSMiddleware_RegularRequestWithOrigin(t *testing.T) {
	t.Parallel()

	cfg := &config.CORSConfig{
		Enabled:      true,
		AllowOrigins: []string{"http://example.com"},
		MaxAge:       86400,
	}
	m := NewCORSMiddleware(dummyHandler(), cfg, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/data", nil)
	req.Header.Set("Origin", "http://example.com")
	rec := httptest.NewRecorder()

	m.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "next handler", rec.Body.String(), "should pass through to next handler")
	assert.Equal(t, "http://example.com", rec.Header().Get("Access-Control-Allow-Origin"))
	// Default methods/headers should be set when config leaves them empty
	assert.Contains(t, rec.Header().Get("Access-Control-Allow-Methods"), "GET")
	assert.Contains(t, rec.Header().Get("Access-Control-Allow-Headers"), "Content-Type")
}

func TestCORSMiddleware_DisabledPassesThrough(t *testing.T) {
	t.Parallel()

	cfg := &config.CORSConfig{
		Enabled: false,
	}
	m := NewCORSMiddleware(dummyHandler(), cfg, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/data", nil)
	req.Header.Set("Origin", "http://example.com")
	rec := httptest.NewRecorder()

	m.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "next handler", rec.Body.String())
	assert.Empty(t, rec.Header().Get("Access-Control-Allow-Origin"), "disabled CORS should not set any CORS headers")
	assert.Empty(t, rec.Header().Get("Access-Control-Allow-Methods"))
	assert.Empty(t, rec.Header().Get("Access-Control-Max-Age"))
}

func TestCORSMiddleware_AllowCredentials(t *testing.T) {
	t.Parallel()

	cfg := &config.CORSConfig{
		Enabled:          true,
		AllowOrigins:     []string{"*"},
		AllowCredentials: true,
		MaxAge:           86400,
	}
	m := NewCORSMiddleware(dummyHandler(), cfg, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/data", nil)
	req.Header.Set("Origin", "http://example.com")
	rec := httptest.NewRecorder()

	m.ServeHTTP(rec, req)

	assert.Equal(t, "true", rec.Header().Get("Access-Control-Allow-Credentials"))
	// With credentials + wildcard, origin should be reflected, not "*"
	assert.Equal(t, "http://example.com", rec.Header().Get("Access-Control-Allow-Origin"))
}

func TestCORSMiddleware_ExposeHeaders(t *testing.T) {
	t.Parallel()

	cfg := &config.CORSConfig{
		Enabled:       true,
		AllowOrigins:  []string{"*"},
		ExposeHeaders: []string{"X-Custom-Header", "X-Request-Id"},
		MaxAge:        86400,
	}
	m := NewCORSMiddleware(dummyHandler(), cfg, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/data", nil)
	req.Header.Set("Origin", "http://example.com")
	rec := httptest.NewRecorder()

	m.ServeHTTP(rec, req)

	assert.Equal(t, "X-Custom-Header, X-Request-Id", rec.Header().Get("Access-Control-Expose-Headers"))
}

func TestCORSMiddleware_MaxAge(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		maxAge   int
		expected string
	}{
		{"custom value", 7200, "7200"},
		{"zero defaults to 86400", 0, "86400"},
		{"negative defaults to 86400", -1, "86400"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			cfg := &config.CORSConfig{
				Enabled:      true,
				AllowOrigins: []string{"*"},
				MaxAge:       tt.maxAge,
			}
			m := NewCORSMiddleware(dummyHandler(), cfg, nil)

			req := httptest.NewRequest(http.MethodGet, "/api/data", nil)
			req.Header.Set("Origin", "http://example.com")
			rec := httptest.NewRecorder()

			m.ServeHTTP(rec, req)

			assert.Equal(t, tt.expected, rec.Header().Get("Access-Control-Max-Age"))
		})
	}
}

func TestCORSMiddleware_MockCheckerTakesPrecedence(t *testing.T) {
	t.Parallel()

	cfg := &config.CORSConfig{
		Enabled:      true,
		AllowOrigins: []string{"*"},
		MaxAge:       86400,
	}
	checker := &mockChecker{hasMatch: true}
	m := NewCORSMiddleware(dummyHandler(), cfg, checker)

	req := httptest.NewRequest(http.MethodOptions, "/api/custom", nil)
	req.Header.Set("Origin", "http://example.com")
	rec := httptest.NewRecorder()

	m.ServeHTTP(rec, req)

	// When MockChecker returns true, the OPTIONS request should pass through to the next handler
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "next handler", rec.Body.String(), "should delegate to next handler when mock checker matches")
	// CORS headers should still be set
	assert.Equal(t, "*", rec.Header().Get("Access-Control-Allow-Origin"))
}

func TestCORSMiddleware_MockCheckerNoMatch(t *testing.T) {
	t.Parallel()

	cfg := &config.CORSConfig{
		Enabled:      true,
		AllowOrigins: []string{"*"},
		MaxAge:       86400,
	}
	checker := &mockChecker{hasMatch: false}
	m := NewCORSMiddleware(dummyHandler(), cfg, checker)

	req := httptest.NewRequest(http.MethodOptions, "/api/test", nil)
	req.Header.Set("Origin", "http://example.com")
	rec := httptest.NewRecorder()

	m.ServeHTTP(rec, req)

	// When MockChecker returns false, preflight should be handled by CORS middleware
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Empty(t, rec.Body.String(), "preflight should not pass through when checker has no match")
}

func TestCORSMiddleware_MissingOriginHeader(t *testing.T) {
	t.Parallel()

	cfg := &config.CORSConfig{
		Enabled:      true,
		AllowOrigins: []string{"http://example.com"},
		MaxAge:       86400,
	}
	m := NewCORSMiddleware(dummyHandler(), cfg, nil)

	t.Run("GET without Origin header", func(t *testing.T) {
		t.Parallel()
		req := httptest.NewRequest(http.MethodGet, "/api/data", nil)
		// No Origin header set
		rec := httptest.NewRecorder()

		m.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, "next handler", rec.Body.String(), "should still pass through to next handler")
		// No origin → GetAllowOriginValue returns "" → no CORS headers set
		assert.Empty(t, rec.Header().Get("Access-Control-Allow-Origin"), "no CORS headers without Origin")
	})

	t.Run("OPTIONS without Origin header", func(t *testing.T) {
		t.Parallel()
		req := httptest.NewRequest(http.MethodOptions, "/api/data", nil)
		// No Origin header
		rec := httptest.NewRecorder()

		m.ServeHTTP(rec, req)

		// No allowed origin → 403 Forbidden
		assert.Equal(t, http.StatusForbidden, rec.Code)
	})
}

func TestCORSMiddleware_DisallowedOriginPreflight(t *testing.T) {
	t.Parallel()

	cfg := &config.CORSConfig{
		Enabled:      true,
		AllowOrigins: []string{"http://allowed.com"},
		MaxAge:       86400,
	}
	m := NewCORSMiddleware(dummyHandler(), cfg, nil)

	req := httptest.NewRequest(http.MethodOptions, "/api/data", nil)
	req.Header.Set("Origin", "http://evil.com")
	rec := httptest.NewRecorder()

	m.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusForbidden, rec.Code, "disallowed origin preflight should return 403")
	assert.Empty(t, rec.Header().Get("Access-Control-Allow-Origin"))
}

func TestCORSMiddleware_NilConfig(t *testing.T) {
	t.Parallel()

	// When config is nil, NewCORSMiddleware should use DefaultCORSConfig
	m := NewCORSMiddleware(dummyHandler(), nil, nil)
	require.NotNil(t, m)
	require.NotNil(t, m.config)
	assert.True(t, m.config.Enabled)
}

// ============================================================================
// WildcardCORSConfig Tests
// ============================================================================

func TestWildcardCORSConfig(t *testing.T) {
	t.Parallel()

	cfg := WildcardCORSConfig()

	require.NotNil(t, cfg)
	assert.True(t, cfg.Enabled)
	assert.Equal(t, []string{"*"}, cfg.AllowOrigins)
	assert.True(t, cfg.IsWildcard())
	assert.Equal(t, 86400, cfg.MaxAge)
	assert.Contains(t, cfg.AllowMethods, "GET")
	assert.Contains(t, cfg.AllowMethods, "POST")
	assert.Contains(t, cfg.AllowMethods, "DELETE")
	assert.Contains(t, cfg.AllowHeaders, "Content-Type")
	assert.Contains(t, cfg.AllowHeaders, "Authorization")
	assert.False(t, cfg.AllowCredentials, "wildcard should not enable credentials by default")
}
