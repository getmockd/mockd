// Rate limiting middleware for the mock engine.

package engine

import (
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/getmockd/mockd/pkg/config"
)

// DefaultEngineRateLimit is the default requests per second limit for the mock engine.
const DefaultEngineRateLimit float64 = 1000

// DefaultEngineBurstSize is the default burst size for the mock engine.
const DefaultEngineBurstSize int = 2000

// DefaultEngineCleanupInterval is how often stale rate limit entries are cleaned up.
const DefaultEngineCleanupInterval = 1 * time.Minute

// DefaultEngineEntryTTL is how long an entry lives without activity before cleanup.
const DefaultEngineEntryTTL = 1 * time.Minute

// engineTokenBucket represents a token bucket for a single client.
type engineTokenBucket struct {
	tokens     float64
	lastUpdate time.Time
	mu         sync.Mutex
}

// EngineRateLimiter implements per-IP rate limiting for the mock engine.
type EngineRateLimiter struct {
	rps            float64 // tokens added per second
	burst          int     // maximum bucket capacity
	buckets        map[string]*engineTokenBucket
	mu             sync.RWMutex
	stopCh         chan struct{}
	stoppedCh      chan struct{}
	trustedProxies []*net.IPNet // CIDR ranges of trusted proxies
	trustProxy     bool         // whether to trust X-Forwarded-For/X-Real-IP
}

// NewEngineRateLimiter creates a new rate limiter for the mock engine.
func NewEngineRateLimiter(cfg *config.RateLimitConfig) *EngineRateLimiter {
	if cfg == nil {
		return nil
	}

	rps := cfg.RequestsPerSecond
	if rps <= 0 {
		rps = DefaultEngineRateLimit
	}

	burst := cfg.BurstSize
	if burst <= 0 {
		burst = DefaultEngineBurstSize
	}

	rl := &EngineRateLimiter{
		rps:       rps,
		burst:     burst,
		buckets:   make(map[string]*engineTokenBucket),
		stopCh:    make(chan struct{}),
		stoppedCh: make(chan struct{}),
	}

	// Configure trusted proxies
	if len(cfg.TrustedProxies) > 0 {
		for _, cidr := range cfg.TrustedProxies {
			_, network, err := net.ParseCIDR(cidr)
			if err != nil {
				// Try parsing as a single IP
				ip := net.ParseIP(cidr)
				if ip != nil {
					if ip.To4() != nil {
						_, network, _ = net.ParseCIDR(cidr + "/32")
					} else {
						_, network, _ = net.ParseCIDR(cidr + "/128")
					}
				}
			}
			if network != nil {
				rl.trustedProxies = append(rl.trustedProxies, network)
				rl.trustProxy = true
			}
		}
	}

	go rl.cleanup()

	return rl
}

// allow checks if a request from the given IP is allowed.
// Returns (allowed, remaining tokens, reset time in seconds).
func (rl *EngineRateLimiter) allow(ip string) (bool, int, int64) {
	now := time.Now()

	rl.mu.RLock()
	bucket, exists := rl.buckets[ip]
	rl.mu.RUnlock()

	if !exists {
		rl.mu.Lock()
		// Double-check after acquiring write lock
		bucket, exists = rl.buckets[ip]
		if !exists {
			bucket = &engineTokenBucket{
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
func (rl *EngineRateLimiter) cleanup() {
	ticker := time.NewTicker(DefaultEngineCleanupInterval)
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
func (rl *EngineRateLimiter) removeStaleEntries() {
	now := time.Now()
	cutoff := now.Add(-DefaultEngineEntryTTL)

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
func (rl *EngineRateLimiter) Stop() {
	close(rl.stopCh)
	<-rl.stoppedCh
}

// getClientIP extracts the client IP from the request.
func (rl *EngineRateLimiter) getClientIP(r *http.Request) string {
	remoteIP := rl.extractRemoteIP(r.RemoteAddr)

	if rl.isTrustedProxy(remoteIP) {
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			if idx := strings.IndexByte(xff, ','); idx != -1 {
				xff = xff[:idx]
			}
			ip := strings.TrimSpace(xff)
			if ip != "" && isValidEngineIP(ip) {
				return ip
			}
		}

		if xri := r.Header.Get("X-Real-IP"); xri != "" {
			ip := strings.TrimSpace(xri)
			if ip != "" && isValidEngineIP(ip) {
				return ip
			}
		}
	}

	return remoteIP
}

// extractRemoteIP extracts the IP address from RemoteAddr.
func (rl *EngineRateLimiter) extractRemoteIP(remoteAddr string) string {
	ip, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		return remoteAddr
	}
	return ip
}

// isTrustedProxy checks if the given IP is from a trusted proxy.
func (rl *EngineRateLimiter) isTrustedProxy(ip string) bool {
	if !rl.trustProxy {
		return false
	}

	if rl.trustedProxies == nil {
		return true
	}

	parsedIP := net.ParseIP(ip)
	if parsedIP == nil {
		return false
	}

	for _, network := range rl.trustedProxies {
		if network.Contains(parsedIP) {
			return true
		}
	}

	return false
}

// isValidEngineIP checks if the string is a valid IP address.
func isValidEngineIP(s string) bool {
	return net.ParseIP(s) != nil
}

// RateLimitMiddleware wraps an http.Handler with rate limiting.
type RateLimitMiddleware struct {
	handler http.Handler
	limiter *EngineRateLimiter
}

// NewRateLimitMiddleware creates a new rate limiting middleware.
// If limiter is nil, the middleware passes through without limiting.
func NewRateLimitMiddleware(handler http.Handler, limiter *EngineRateLimiter) *RateLimitMiddleware {
	return &RateLimitMiddleware{
		handler: handler,
		limiter: limiter,
	}
}

// ServeHTTP implements the http.Handler interface.
func (m *RateLimitMiddleware) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if m.limiter == nil {
		m.handler.ServeHTTP(w, r)
		return
	}

	ip := m.limiter.getClientIP(r)
	allowed, remaining, resetOrRetry := m.limiter.allow(ip)

	// Set rate limit headers
	w.Header().Set("X-RateLimit-Limit", strconv.Itoa(m.limiter.burst))
	w.Header().Set("X-RateLimit-Remaining", strconv.Itoa(remaining))

	if allowed {
		w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(resetOrRetry, 10))
		m.handler.ServeHTTP(w, r)
		return
	}

	// Rate limit exceeded
	w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(time.Now().Unix()+resetOrRetry, 10))
	w.Header().Set("Retry-After", strconv.FormatInt(resetOrRetry, 10))
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusTooManyRequests)
	_, _ = w.Write([]byte(`{"error":"rate_limit_exceeded","message":"Too many requests. Please slow down."}`))
}
