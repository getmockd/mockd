// CORS middleware for the mock engine.

package engine

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/getmockd/mockd/pkg/config"
)

// MockChecker can check if a request matches a user-defined mock.
type MockChecker interface {
	HasMatch(r *http.Request) bool
}

// CORSMiddleware wraps an http.Handler with CORS handling based on configuration.
type CORSMiddleware struct {
	handler http.Handler
	config  *config.CORSConfig
	checker MockChecker
}

// NewCORSMiddleware creates a new CORS middleware with the given configuration.
// If config is nil, default secure settings (localhost only) are used.
// The optional checker allows user-defined OPTIONS mocks to take precedence over CORS preflight handling.
func NewCORSMiddleware(handler http.Handler, cfg *config.CORSConfig, checker MockChecker) *CORSMiddleware {
	if cfg == nil {
		cfg = config.DefaultCORSConfig()
	}
	return &CORSMiddleware{
		handler: handler,
		config:  cfg,
		checker: checker,
	}
}

// ServeHTTP implements the http.Handler interface.
func (m *CORSMiddleware) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if !m.config.Enabled {
		m.handler.ServeHTTP(w, r)
		return
	}

	origin := r.Header.Get("Origin")
	allowOrigin := m.config.GetAllowOriginValue(origin)

	// Set CORS headers if origin is allowed
	if allowOrigin != "" {
		w.Header().Set("Access-Control-Allow-Origin", allowOrigin)

		// Set methods
		methods := m.config.AllowMethods
		if len(methods) == 0 {
			methods = []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS", "HEAD"}
		}
		w.Header().Set("Access-Control-Allow-Methods", strings.Join(methods, ", "))

		// Set headers
		headers := m.config.AllowHeaders
		if len(headers) == 0 {
			headers = []string{"Content-Type", "Authorization", "X-Requested-With", "Accept", "Origin"}
		}
		w.Header().Set("Access-Control-Allow-Headers", strings.Join(headers, ", "))

		// Set expose headers if configured
		if len(m.config.ExposeHeaders) > 0 {
			w.Header().Set("Access-Control-Expose-Headers", strings.Join(m.config.ExposeHeaders, ", "))
		}

		// Set credentials
		if m.config.AllowCredentials {
			w.Header().Set("Access-Control-Allow-Credentials", "true")
		}

		// Set max age
		maxAge := m.config.MaxAge
		if maxAge <= 0 {
			maxAge = 86400 // 24 hours default
		}
		w.Header().Set("Access-Control-Max-Age", strconv.Itoa(maxAge))
	}

	// Handle preflight requests — but let user-defined OPTIONS mocks take precedence
	if r.Method == http.MethodOptions {
		if m.checker != nil && m.checker.HasMatch(r) {
			// User has defined an OPTIONS mock for this path — pass through
			m.handler.ServeHTTP(w, r)
			return
		}
		if allowOrigin != "" {
			w.WriteHeader(http.StatusOK)
		} else {
			w.WriteHeader(http.StatusForbidden)
		}
		return
	}

	m.handler.ServeHTTP(w, r)
}

// WildcardCORSConfig returns a CORS config that allows all origins.
// WARNING: This should only be used in development or controlled environments.
func WildcardCORSConfig() *config.CORSConfig {
	return &config.CORSConfig{
		Enabled:      true,
		AllowOrigins: []string{"*"},
		AllowMethods: []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS", "HEAD"},
		AllowHeaders: []string{"Content-Type", "Authorization", "X-Requested-With", "Accept", "Origin"},
		MaxAge:       86400,
	}
}
