package admin

import (
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"
)

// DefaultRateLimit is the default requests per second limit.
const DefaultRateLimit float64 = 100

// DefaultBurstSize is the default burst size.
const DefaultBurstSize int = 200

// DefaultCleanupInterval is how often stale entries are cleaned up.
const DefaultCleanupInterval = 1 * time.Minute

// DefaultEntryTTL is how long an entry lives without activity before cleanup.
const DefaultEntryTTL = 1 * time.Minute

// tokenBucket represents a token bucket for a single client.
type tokenBucket struct {
	tokens     float64
	lastUpdate time.Time
	mu         sync.Mutex
}

// RateLimiter implements per-IP rate limiting using the token bucket algorithm.
type RateLimiter struct {
	rps       float64 // tokens added per second
	burst     int     // maximum bucket capacity
	buckets   map[string]*tokenBucket
	mu        sync.RWMutex
	stopCh    chan struct{}
	stoppedCh chan struct{}
}

// NewRateLimiter creates a new rate limiter with the specified requests per second
// and burst size. It starts a background goroutine to clean up stale entries.
func NewRateLimiter(rps float64, burst int) *RateLimiter {
	if rps <= 0 {
		rps = DefaultRateLimit
	}
	if burst <= 0 {
		burst = DefaultBurstSize
	}

	rl := &RateLimiter{
		rps:       rps,
		burst:     burst,
		buckets:   make(map[string]*tokenBucket),
		stopCh:    make(chan struct{}),
		stoppedCh: make(chan struct{}),
	}

	go rl.cleanup()

	return rl
}

// allow checks if a request from the given IP is allowed.
// Returns (allowed, remaining tokens, reset time in seconds).
func (rl *RateLimiter) allow(ip string) (bool, int, int64) {
	now := time.Now()

	rl.mu.RLock()
	bucket, exists := rl.buckets[ip]
	rl.mu.RUnlock()

	if !exists {
		rl.mu.Lock()
		// Double-check after acquiring write lock
		bucket, exists = rl.buckets[ip]
		if !exists {
			bucket = &tokenBucket{
				tokens:     float64(rl.burst),
				lastUpdate: now,
			}
			rl.buckets[ip] = bucket
		}
		rl.mu.Unlock()
	}

	bucket.mu.Lock()
	defer bucket.mu.Unlock()

	// Add tokens based on time elapsed
	elapsed := now.Sub(bucket.lastUpdate).Seconds()
	bucket.tokens += elapsed * rl.rps
	if bucket.tokens > float64(rl.burst) {
		bucket.tokens = float64(rl.burst)
	}
	bucket.lastUpdate = now

	// Calculate time until bucket is full (for reset header)
	tokensNeeded := float64(rl.burst) - bucket.tokens
	resetSeconds := int64(tokensNeeded / rl.rps)
	if resetSeconds < 1 && tokensNeeded > 0 {
		resetSeconds = 1
	}

	// Check if we have enough tokens
	if bucket.tokens >= 1 {
		bucket.tokens--
		remaining := int(bucket.tokens)
		if remaining < 0 {
			remaining = 0
		}
		return true, remaining, resetSeconds
	}

	// Calculate time until one token is available
	retryAfter := int64((1 - bucket.tokens) / rl.rps)
	if retryAfter < 1 {
		retryAfter = 1
	}

	return false, 0, retryAfter
}

// cleanup periodically removes stale entries from the buckets map.
func (rl *RateLimiter) cleanup() {
	ticker := time.NewTicker(DefaultCleanupInterval)
	defer ticker.Stop()
	defer close(rl.stoppedCh)

	for {
		select {
		case <-ticker.C:
			rl.removeStaleEntries()
		case <-rl.stopCh:
			return
		}
	}
}

// removeStaleEntries removes entries that haven't been accessed recently.
func (rl *RateLimiter) removeStaleEntries() {
	now := time.Now()
	cutoff := now.Add(-DefaultEntryTTL)

	rl.mu.Lock()
	defer rl.mu.Unlock()

	for ip, bucket := range rl.buckets {
		bucket.mu.Lock()
		if bucket.lastUpdate.Before(cutoff) {
			delete(rl.buckets, ip)
		}
		bucket.mu.Unlock()
	}
}

// Stop stops the cleanup goroutine.
func (rl *RateLimiter) Stop() {
	close(rl.stopCh)
	<-rl.stoppedCh
}

// Middleware returns an HTTP middleware that enforces rate limiting.
func (rl *RateLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := rl.getClientIP(r)

		allowed, remaining, resetOrRetry := rl.allow(ip)

		// Set rate limit headers
		w.Header().Set("X-RateLimit-Limit", strconv.Itoa(rl.burst))
		w.Header().Set("X-RateLimit-Remaining", strconv.Itoa(remaining))

		if allowed {
			w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(resetOrRetry, 10))
			next.ServeHTTP(w, r)
			return
		}

		// Rate limit exceeded
		w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(time.Now().Unix()+resetOrRetry, 10))
		w.Header().Set("Retry-After", strconv.FormatInt(resetOrRetry, 10))
		http.Error(w, "Too Many Requests", http.StatusTooManyRequests)
	})
}

// getClientIP extracts the client IP from the request.
// It checks X-Forwarded-For and X-Real-IP headers first, then falls back to RemoteAddr.
//
// SECURITY WARNING: X-Forwarded-For and X-Real-IP headers are client-controlled and can be
// spoofed by malicious actors. These headers should only be trusted when the application
// is deployed behind a trusted reverse proxy (e.g., nginx, AWS ALB, Cloudflare) that
// properly sets these headers. Without a trusted proxy, attackers can bypass rate limiting
// by setting arbitrary IP addresses in these headers.
//
// For production deployments:
// - Ensure the application is behind a trusted reverse proxy
// - Configure the proxy to overwrite (not append to) these headers
// - Consider using a configurable trusted proxy list
func (rl *RateLimiter) getClientIP(r *http.Request) string {
	// Check X-Forwarded-For header (may contain multiple IPs)
	// WARNING: This header can be spoofed if not behind a trusted proxy
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// Take the first IP (original client)
		if idx := indexByte(xff, ','); idx != -1 {
			xff = xff[:idx]
		}
		ip := trimSpaces(xff)
		if ip != "" {
			return ip
		}
	}

	// Check X-Real-IP header
	// WARNING: This header can be spoofed if not behind a trusted proxy
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		ip := trimSpaces(xri)
		if ip != "" {
			return ip
		}
	}

	// Fall back to RemoteAddr (most secure - set by the TCP connection)
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		// RemoteAddr might not have a port
		return r.RemoteAddr
	}
	return ip
}

// indexByte returns the index of the first occurrence of c in s, or -1.
func indexByte(s string, c byte) int {
	for i := 0; i < len(s); i++ {
		if s[i] == c {
			return i
		}
	}
	return -1
}

// trimSpaces removes leading and trailing spaces from s.
func trimSpaces(s string) string {
	start := 0
	end := len(s)
	for start < end && s[start] == ' ' {
		start++
	}
	for end > start && s[end-1] == ' ' {
		end--
	}
	return s[start:end]
}

// RateLimiterOption configures a RateLimiter.
type RateLimiterOption func(*RateLimiter)

// WithRPS sets the requests per second rate.
func WithRPS(rps float64) RateLimiterOption {
	return func(rl *RateLimiter) {
		if rps > 0 {
			rl.rps = rps
		}
	}
}

// WithBurst sets the burst size.
func WithBurst(burst int) RateLimiterOption {
	return func(rl *RateLimiter) {
		if burst > 0 {
			rl.burst = burst
		}
	}
}
