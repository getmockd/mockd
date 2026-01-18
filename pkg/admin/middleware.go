package admin

import (
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// CORSConfig holds the configuration for CORS middleware.
type CORSConfig struct {
	// AllowedOrigins is a list of origins that are allowed to make cross-origin requests.
	// If empty or contains "*", all origins are allowed.
	AllowedOrigins []string

	// AllowedMethods is a list of HTTP methods allowed for cross-origin requests.
	// Default: GET, POST, PUT, PATCH, DELETE, OPTIONS
	AllowedMethods []string

	// AllowedHeaders is a list of headers that are allowed in cross-origin requests.
	// Default: Content-Type, Authorization
	AllowedHeaders []string

	// AllowCredentials indicates whether the request can include credentials like
	// cookies, authorization headers, or TLS client certificates.
	// When true, AllowedOrigins cannot contain "*" - specific origins must be listed.
	AllowCredentials bool

	// MaxAge indicates how long (in seconds) the results of a preflight request
	// can be cached. Default: 86400 (24 hours)
	MaxAge int
}

// DefaultCORSConfig returns a CORSConfig with default values that maintains
// backward compatibility (allows all origins).
//
// SECURITY WARNING: The default configuration uses AllowedOrigins: ["*"] which permits
// cross-origin requests from ANY domain. For production deployments, you should:
// - Specify explicit allowed origins instead of using "*"
// - Use NewCORSMiddlewareWithConfig() with a custom CORSConfig
// - Consider the security implications if AllowCredentials is enabled with wildcards
func DefaultCORSConfig() CORSConfig {
	// Log warning about wildcard CORS configuration
	log.Println("[SECURITY WARNING] CORS configured with wildcard origin (*). " +
		"This allows cross-origin requests from any domain. " +
		"For production, specify explicit allowed origins using NewCORSMiddlewareWithConfig().")

	return CORSConfig{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Content-Type", "Authorization"},
		AllowCredentials: false,
		MaxAge:           86400,
	}
}

// isOriginAllowed checks if the given origin is allowed based on the config.
func (c *CORSConfig) isOriginAllowed(origin string) bool {
	// If no origins specified or wildcard, allow all
	if len(c.AllowedOrigins) == 0 {
		return true
	}
	for _, allowed := range c.AllowedOrigins {
		if allowed == "*" {
			return true
		}
		if allowed == origin {
			return true
		}
	}
	return false
}

// getAllowOriginValue returns the appropriate Access-Control-Allow-Origin header value.
func (c *CORSConfig) getAllowOriginValue(origin string) string {
	// If credentials are allowed, we must echo the specific origin (not *)
	if c.AllowCredentials {
		if c.isOriginAllowed(origin) && origin != "" {
			return origin
		}
		return ""
	}

	// If no origins specified or contains wildcard, return *
	if len(c.AllowedOrigins) == 0 {
		return "*"
	}
	for _, allowed := range c.AllowedOrigins {
		if allowed == "*" {
			return "*"
		}
	}

	// Otherwise, echo the origin if it's allowed
	if c.isOriginAllowed(origin) {
		return origin
	}
	return ""
}

// getMethods returns the allowed methods as a comma-separated string.
func (c *CORSConfig) getMethods() string {
	if len(c.AllowedMethods) == 0 {
		return "GET, POST, PUT, PATCH, DELETE, OPTIONS"
	}
	return strings.Join(c.AllowedMethods, ", ")
}

// getHeaders returns the allowed headers as a comma-separated string.
func (c *CORSConfig) getHeaders() string {
	if len(c.AllowedHeaders) == 0 {
		return "Content-Type, Authorization"
	}
	return strings.Join(c.AllowedHeaders, ", ")
}

// getMaxAge returns the max age as a string.
func (c *CORSConfig) getMaxAge() string {
	if c.MaxAge <= 0 {
		return "86400"
	}
	return strconv.Itoa(c.MaxAge)
}

// LoggingMiddleware logs HTTP requests.
type LoggingMiddleware struct {
	handler http.Handler
}

// NewLoggingMiddleware creates a new logging middleware.
func NewLoggingMiddleware(handler http.Handler) *LoggingMiddleware {
	return &LoggingMiddleware{handler: handler}
}

// ServeHTTP implements the http.Handler interface.
func (m *LoggingMiddleware) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	// Wrap response writer to capture status code
	lrw := &loggingResponseWriter{ResponseWriter: w, statusCode: http.StatusOK}

	m.handler.ServeHTTP(lrw, r)

	log.Printf("[ADMIN] %s %s %d %v",
		r.Method,
		r.URL.Path,
		lrw.statusCode,
		time.Since(start),
	)
}

// loggingResponseWriter wraps http.ResponseWriter to capture status code.
type loggingResponseWriter struct {
	http.ResponseWriter
	statusCode int
}

// WriteHeader captures the status code.
func (lrw *loggingResponseWriter) WriteHeader(code int) {
	lrw.statusCode = code
	lrw.ResponseWriter.WriteHeader(code)
}

// CORSMiddleware adds CORS headers to responses.
type CORSMiddleware struct {
	handler http.Handler
	config  CORSConfig
}

// NewCORSMiddleware creates a new CORS middleware with default configuration.
func NewCORSMiddleware(handler http.Handler) *CORSMiddleware {
	return &CORSMiddleware{
		handler: handler,
		config:  DefaultCORSConfig(),
	}
}

// NewCORSMiddlewareWithConfig creates a new CORS middleware with custom configuration.
func NewCORSMiddlewareWithConfig(handler http.Handler, config CORSConfig) *CORSMiddleware {
	return &CORSMiddleware{
		handler: handler,
		config:  config,
	}
}

// ServeHTTP implements the http.Handler interface.
func (m *CORSMiddleware) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	origin := r.Header.Get("Origin")

	// Always set Vary header to indicate origin-dependent response
	w.Header().Add("Vary", "Origin")

	// Get the appropriate Allow-Origin value
	allowOrigin := m.config.getAllowOriginValue(origin)
	if allowOrigin == "" {
		// Origin not allowed, but still process the request (browser will block response)
		m.handler.ServeHTTP(w, r)
		return
	}

	// Set CORS headers
	w.Header().Set("Access-Control-Allow-Origin", allowOrigin)
	w.Header().Set("Access-Control-Allow-Methods", m.config.getMethods())
	w.Header().Set("Access-Control-Allow-Headers", m.config.getHeaders())
	w.Header().Set("Access-Control-Max-Age", m.config.getMaxAge())

	// Set credentials header if enabled
	if m.config.AllowCredentials {
		w.Header().Set("Access-Control-Allow-Credentials", "true")
	}

	// Handle preflight requests
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	m.handler.ServeHTTP(w, r)
}

// SecurityHeadersMiddleware adds security headers to all responses.
// These headers help protect against common web vulnerabilities like
// clickjacking, XSS attacks, MIME type sniffing, and information leakage.
func SecurityHeadersMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Prevent MIME type sniffing
		w.Header().Set("X-Content-Type-Options", "nosniff")

		// Prevent clickjacking by denying framing
		w.Header().Set("X-Frame-Options", "DENY")

		// Enable XSS filtering in browsers
		w.Header().Set("X-XSS-Protection", "1; mode=block")

		// Control referrer information sent with requests
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")

		// Restrict resource loading to same origin
		w.Header().Set("Content-Security-Policy", "default-src 'self'")

		// Prevent caching of sensitive responses
		w.Header().Set("Cache-Control", "no-store")

		next.ServeHTTP(w, r)
	})
}
