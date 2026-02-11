package ratelimit

import (
	"net/http"
	"strconv"
)

// MiddlewareOption configures the rate limiting middleware.
type MiddlewareOption func(*middlewareConfig)

type middlewareConfig struct {
	useTextResponse bool // if false, use JSON (default)
}

// WithTextResponse configures the middleware to emit a plain text 429 body.
func WithTextResponse() MiddlewareOption {
	return func(c *middlewareConfig) {
		c.useTextResponse = true
	}
}

// WithJSONResponse configures the middleware to emit a JSON 429 body.
// This is the default behaviour.
func WithJSONResponse() MiddlewareOption {
	return func(c *middlewareConfig) {
		c.useTextResponse = false
	}
}

// Middleware returns an HTTP middleware that enforces per-IP rate limiting.
// If limiter is nil, the middleware passes through without limiting.
func Middleware(limiter *PerIPLimiter, opts ...MiddlewareOption) func(http.Handler) http.Handler {
	cfg := &middlewareConfig{}
	for _, o := range opts {
		o(cfg)
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if limiter == nil {
				next.ServeHTTP(w, r)
				return
			}

			ip := limiter.ClientIP(r)
			allowed, remaining, resetOrRetry := limiter.Allow(ip)

			// Set rate limit headers
			w.Header().Set("X-RateLimit-Limit", strconv.Itoa(limiter.Burst()))
			w.Header().Set("X-RateLimit-Remaining", strconv.Itoa(remaining))

			if allowed {
				w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(resetOrRetry, 10))
				next.ServeHTTP(w, r)
				return
			}

			// Rate limit exceeded
			w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(resetOrRetry, 10))
			w.Header().Set("Retry-After", strconv.FormatInt(resetOrRetry, 10))

			if cfg.useTextResponse {
				http.Error(w, "Too Many Requests", http.StatusTooManyRequests)
			} else {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusTooManyRequests)
				_, _ = w.Write([]byte(`{"error":"rate_limit_exceeded","message":"Too many requests. Please slow down."}`))
			}
		})
	}
}
