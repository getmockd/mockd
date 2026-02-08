package ratelimit

import (
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

// Default per-IP limiter values.
const (
	DefaultCleanupInterval = 1 * time.Minute
	DefaultEntryTTL        = 1 * time.Minute
)

// ipBucket is an internal token bucket for a single IP.
type ipBucket struct {
	tokens     float64
	lastUpdate time.Time
	mu         sync.Mutex
}

// PerIPConfig configures a PerIPLimiter.
type PerIPConfig struct {
	Rate            float64       // tokens per second
	Burst           int           // maximum bucket capacity
	TrustedProxies  []string      // CIDR ranges of trusted proxies
	TrustAllProxies bool          // trust proxy headers from any source (insecure)
	CleanupInterval time.Duration // how often stale entries are cleaned up
	EntryTTL        time.Duration // how long an entry lives without activity
}

// PerIPLimiter implements per-IP rate limiting using the token bucket algorithm.
type PerIPLimiter struct {
	rps             float64
	burst           int
	buckets         map[string]*ipBucket
	mu              sync.RWMutex
	stopCh          chan struct{}
	stoppedCh       chan struct{}
	trustedProxies  []*net.IPNet
	trustProxy      bool
	cleanupInterval time.Duration
	entryTTL        time.Duration
}

// NewPerIPLimiter creates a new per-IP rate limiter with the given configuration.
// It starts a background goroutine for cleaning up stale entries.
func NewPerIPLimiter(cfg PerIPConfig) *PerIPLimiter {
	rps := cfg.Rate
	if rps <= 0 {
		rps = 100
	}
	burst := cfg.Burst
	if burst <= 0 {
		burst = int(rps * 2)
	}
	cleanupInterval := cfg.CleanupInterval
	if cleanupInterval <= 0 {
		cleanupInterval = DefaultCleanupInterval
	}
	entryTTL := cfg.EntryTTL
	if entryTTL <= 0 {
		entryTTL = DefaultEntryTTL
	}

	rl := &PerIPLimiter{
		rps:             rps,
		burst:           burst,
		buckets:         make(map[string]*ipBucket),
		stopCh:          make(chan struct{}),
		stoppedCh:       make(chan struct{}),
		cleanupInterval: cleanupInterval,
		entryTTL:        entryTTL,
	}

	// Configure trusted proxies
	if cfg.TrustAllProxies {
		rl.trustProxy = true
		rl.trustedProxies = nil // nil means trust all
	} else if len(cfg.TrustedProxies) > 0 {
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

// Burst returns the burst size (maximum bucket capacity).
func (rl *PerIPLimiter) Burst() int {
	return rl.burst
}

// Allow checks if a request from the given IP is allowed.
// Returns (allowed, remaining tokens, reset/retry-after time in seconds).
func (rl *PerIPLimiter) Allow(ip string) (allowed bool, remaining int, retryAfterSec int64) {
	now := time.Now()

	rl.mu.RLock()
	bucket, exists := rl.buckets[ip]
	rl.mu.RUnlock()

	if !exists {
		rl.mu.Lock()
		// Double-check after acquiring write lock
		bucket, exists = rl.buckets[ip]
		if !exists {
			bucket = &ipBucket{
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
		rem := int(bucket.tokens)
		if rem < 0 {
			rem = 0
		}
		return true, rem, resetSeconds
	}

	// Calculate time until one token is available
	retry := int64((1 - bucket.tokens) / rl.rps)
	if retry < 1 {
		retry = 1
	}

	return false, 0, retry
}

// ClientIP extracts the client IP from the request, respecting trusted proxy settings.
func (rl *PerIPLimiter) ClientIP(r *http.Request) string {
	remoteIP := extractRemoteIP(r.RemoteAddr)

	if rl.isTrustedProxy(remoteIP) {
		// Check X-Forwarded-For header (may contain multiple IPs)
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			if idx := strings.IndexByte(xff, ','); idx != -1 {
				xff = xff[:idx]
			}
			ip := strings.TrimSpace(xff)
			if ip != "" && isValidIP(ip) {
				return ip
			}
		}

		// Check X-Real-IP header
		if xri := r.Header.Get("X-Real-IP"); xri != "" {
			ip := strings.TrimSpace(xri)
			if ip != "" && isValidIP(ip) {
				return ip
			}
		}
	}

	return remoteIP
}

// Stop stops the cleanup goroutine. Must be called when the limiter is no longer needed.
func (rl *PerIPLimiter) Stop() {
	close(rl.stopCh)
	<-rl.stoppedCh
}

// cleanup periodically removes stale entries.
func (rl *PerIPLimiter) cleanup() {
	ticker := time.NewTicker(rl.cleanupInterval)
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
func (rl *PerIPLimiter) removeStaleEntries() {
	now := time.Now()
	cutoff := now.Add(-rl.entryTTL)

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

// isTrustedProxy checks if the given IP is from a trusted proxy.
func (rl *PerIPLimiter) isTrustedProxy(ip string) bool {
	if !rl.trustProxy {
		return false
	}
	// nil trustedProxies with trustProxy=true means trust all
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

// extractRemoteIP extracts the IP address from RemoteAddr (strips port if present).
func extractRemoteIP(remoteAddr string) string {
	ip, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		return remoteAddr
	}
	return ip
}

// isValidIP checks if the string is a valid IP address.
func isValidIP(s string) bool {
	return net.ParseIP(s) != nil
}
