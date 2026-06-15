package admin

import (
	"net/http"
	"strconv"
	"strings"
)

// CORSConfig holds the configuration for CORS middleware.
type CORSConfig struct {
	// AllowedOrigins is a list of origins that are allowed to make cross-origin requests.
	// If empty or contains "*", all origins are allowed. The reserved sentinel
	// value "loopback" matches any http(s)://localhost / 127.0.0.1 / [::1] origin
	// on any port (it is not a real Origin value, so it never collides with one);
	// it is the secure default for the admin API.
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

// DefaultCORSConfig returns the secure default CORS config for the admin API:
// loopback origins only (see the "loopback" sentinel below). The admin API is a
// localhost control plane, so it must not echo arbitrary web origins by default.
// Callers that genuinely need to allow other origins can opt in explicitly with
// WithCORS — wildcard ("*") is supported there but never the default.
func DefaultCORSConfig() CORSConfig {
	return CORSConfig{
		// Loopback-only by default: the admin API is a localhost control plane
		// that can read and mutate every mock, config, and captured request log
		// (which may contain secrets), so arbitrary web origins must not be able
		// to read its responses. The sentinel "loopback" entry matches any
		// http(s)://localhost / 127.0.0.1 / [::1] origin on any port (handled in
		// isOriginAllowed via isLoopbackOrigin), so the embedded dashboard works
		// on any local port without enumerating ports. We never emit "*" and
		// never echo "null". Wildcard remains opt-in via WithCORS for callers
		// that explicitly want it.
		AllowedOrigins:   []string{"loopback"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Content-Type", "Authorization", "X-API-Key"},
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
	// Never echo the opaque/null origin.
	if origin == "" || origin == "null" {
		return false
	}
	for _, allowed := range c.AllowedOrigins {
		if allowed == "*" {
			return true
		}
		if allowed == origin {
			return true
		}
		// "loopback" sentinel: allow any loopback origin (port-insensitive), so
		// the dashboard works on localhost/127.0.0.1/[::1] on any port without
		// enumerating ports. Rebinding/non-loopback origins still fail.
		if allowed == "loopback" && isLoopbackOrigin(origin) {
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
