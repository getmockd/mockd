package admin

import (
	"net/http"
	"strconv"
	"strings"
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
