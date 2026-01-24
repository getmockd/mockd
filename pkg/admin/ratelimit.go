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
	rps            float64 // tokens added per second
	burst          int     // maximum bucket capacity
	buckets        map[string]*tokenBucket
	mu             sync.RWMutex
	stopCh         chan struct{}
	stoppedCh      chan struct{}
	trustedProxies []*net.IPNet // CIDR ranges of trusted proxies
	trustProxy     bool         // whether to trust X-Forwarded-For/X-Real-IP
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
// It checks X-Forwarded-For and X-Real-IP headers only if the request comes from a
// trusted proxy, then falls back to RemoteAddr.
//
// By default, proxy headers are NOT trusted to prevent IP spoofing attacks.
// Use WithTrustedProxies or WithTrustAllProxies to enable proxy header processing.
func (rl *RateLimiter) getClientIP(r *http.Request) string {
	// First, extract the direct connection IP (RemoteAddr)
	remoteIP := rl.extractRemoteIP(r.RemoteAddr)

	// Only check proxy headers if we trust the source
	if rl.isTrustedProxy(remoteIP) {
		// Check X-Forwarded-For header (may contain multiple IPs)
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			// Take the first IP (original client)
			if idx := indexByte(xff, ','); idx != -1 {
				xff = xff[:idx]
			}
			ip := trimSpaces(xff)
			if ip != "" && isValidIP(ip) {
				return ip
			}
		}

		// Check X-Real-IP header
		if xri := r.Header.Get("X-Real-IP"); xri != "" {
			ip := trimSpaces(xri)
			if ip != "" && isValidIP(ip) {
				return ip
			}
		}
	}

	// Fall back to RemoteAddr (most secure - set by the TCP connection)
	return remoteIP
}

// extractRemoteIP extracts the IP address from RemoteAddr (strips port if present).
func (rl *RateLimiter) extractRemoteIP(remoteAddr string) string {
	ip, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		// RemoteAddr might not have a port
		return remoteAddr
	}
	return ip
}

// isTrustedProxy checks if the given IP is from a trusted proxy.
func (rl *RateLimiter) isTrustedProxy(ip string) bool {
	// If trust is not enabled, never trust proxy headers
	if !rl.trustProxy {
		return false
	}

	// If trustedProxies is nil but trustProxy is true, trust all (WithTrustAllProxies)
	if rl.trustedProxies == nil {
		return true
	}

	// Check if IP is in any trusted CIDR range
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

// isValidIP checks if the string is a valid IP address.
func isValidIP(s string) bool {
	return net.ParseIP(s) != nil
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

// WithTrustedProxies sets the list of trusted proxy CIDR ranges.
// When set, X-Forwarded-For and X-Real-IP headers are only trusted
// if the request comes from an IP within one of these ranges.
// Common values include:
//   - "127.0.0.1/32" for localhost
//   - "10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16" for private networks
//   - Cloud provider load balancer IP ranges
func WithTrustedProxies(cidrs []string) RateLimiterOption {
	return func(rl *RateLimiter) {
		for _, cidr := range cidrs {
			_, network, err := net.ParseCIDR(cidr)
			if err != nil {
				// Try parsing as a single IP
				ip := net.ParseIP(cidr)
				if ip != nil {
					// Convert to /32 or /128 CIDR
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
}

// WithTrustAllProxies configures the rate limiter to trust proxy headers
// from any source. This is INSECURE and should only be used in controlled
// environments (e.g., development) where you're certain all traffic comes
// through a trusted proxy.
func WithTrustAllProxies() RateLimiterOption {
	return func(rl *RateLimiter) {
		rl.trustProxy = true
		rl.trustedProxies = nil // nil means trust all
	}
}
