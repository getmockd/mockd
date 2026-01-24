package admin

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"
)

func TestNewRateLimiter(t *testing.T) {
	t.Run("creates rate limiter with default values", func(t *testing.T) {
		rl := NewRateLimiter(0, 0)
		defer rl.Stop()

		if rl.rps != DefaultRateLimit {
			t.Errorf("expected rps %v, got %v", DefaultRateLimit, rl.rps)
		}
		if rl.burst != DefaultBurstSize {
			t.Errorf("expected burst %v, got %v", DefaultBurstSize, rl.burst)
		}
	})

	t.Run("creates rate limiter with custom values", func(t *testing.T) {
		rl := NewRateLimiter(50, 100)
		defer rl.Stop()

		if rl.rps != 50 {
			t.Errorf("expected rps 50, got %v", rl.rps)
		}
		if rl.burst != 100 {
			t.Errorf("expected burst 100, got %v", rl.burst)
		}
	})
}

func TestRateLimiter_Allow(t *testing.T) {
	t.Run("allows requests within limit", func(t *testing.T) {
		rl := NewRateLimiter(10, 5)
		defer rl.Stop()

		ip := "192.168.1.1"

		// First 5 requests should be allowed (burst size)
		for i := 0; i < 5; i++ {
			allowed, remaining, _ := rl.allow(ip)
			if !allowed {
				t.Errorf("request %d should be allowed", i+1)
			}
			expectedRemaining := 4 - i
			if expectedRemaining < 0 {
				expectedRemaining = 0
			}
			if remaining != expectedRemaining {
				t.Errorf("request %d: expected remaining %d, got %d", i+1, expectedRemaining, remaining)
			}
		}
	})

	t.Run("blocks requests when bucket is empty", func(t *testing.T) {
		rl := NewRateLimiter(10, 2) // 10 req/s, burst of 2
		defer rl.Stop()

		ip := "192.168.1.2"

		// Exhaust the bucket
		rl.allow(ip)
		rl.allow(ip)

		// Third request should be blocked
		allowed, remaining, retryAfter := rl.allow(ip)
		if allowed {
			t.Error("request should be blocked")
		}
		if remaining != 0 {
			t.Errorf("expected remaining 0, got %d", remaining)
		}
		if retryAfter < 1 {
			t.Errorf("expected retryAfter >= 1, got %d", retryAfter)
		}
	})

	t.Run("refills tokens over time", func(t *testing.T) {
		rl := NewRateLimiter(100, 2) // 100 req/s = 1 req per 10ms
		defer rl.Stop()

		ip := "192.168.1.3"

		// Exhaust the bucket
		rl.allow(ip)
		rl.allow(ip)

		// Wait for tokens to refill (at least 20ms for 2 tokens at 100 req/s)
		time.Sleep(30 * time.Millisecond)

		// Should be allowed again
		allowed, _, _ := rl.allow(ip)
		if !allowed {
			t.Error("request should be allowed after token refill")
		}
	})

	t.Run("tracks different IPs separately", func(t *testing.T) {
		rl := NewRateLimiter(10, 2)
		defer rl.Stop()

		ip1 := "192.168.1.1"
		ip2 := "192.168.1.2"

		// Exhaust bucket for IP1
		rl.allow(ip1)
		rl.allow(ip1)

		// IP1 should be blocked
		allowed, _, _ := rl.allow(ip1)
		if allowed {
			t.Error("IP1 should be blocked")
		}

		// IP2 should still be allowed
		allowed, _, _ = rl.allow(ip2)
		if !allowed {
			t.Error("IP2 should be allowed")
		}
	})
}

func TestRateLimiter_Middleware(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	t.Run("sets rate limit headers on allowed requests", func(t *testing.T) {
		rl := NewRateLimiter(10, 5)
		defer rl.Stop()

		middleware := rl.Middleware(handler)

		req := httptest.NewRequest("GET", "/test", nil)
		req.RemoteAddr = "192.168.1.1:12345"
		rr := httptest.NewRecorder()

		middleware.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, rr.Code)
		}

		limit := rr.Header().Get("X-RateLimit-Limit")
		if limit != "5" {
			t.Errorf("expected X-RateLimit-Limit 5, got %s", limit)
		}

		remaining := rr.Header().Get("X-RateLimit-Remaining")
		if remaining != "4" {
			t.Errorf("expected X-RateLimit-Remaining 4, got %s", remaining)
		}

		reset := rr.Header().Get("X-RateLimit-Reset")
		if reset == "" {
			t.Error("expected X-RateLimit-Reset header")
		}
	})

	t.Run("returns 429 when rate limited", func(t *testing.T) {
		rl := NewRateLimiter(10, 1) // Very small burst
		defer rl.Stop()

		middleware := rl.Middleware(handler)

		// First request - uses the one token
		req := httptest.NewRequest("GET", "/test", nil)
		req.RemoteAddr = "192.168.1.1:12345"
		rr := httptest.NewRecorder()
		middleware.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("first request: expected status %d, got %d", http.StatusOK, rr.Code)
		}

		// Second request - should be rate limited
		req = httptest.NewRequest("GET", "/test", nil)
		req.RemoteAddr = "192.168.1.1:12345"
		rr = httptest.NewRecorder()
		middleware.ServeHTTP(rr, req)

		if rr.Code != http.StatusTooManyRequests {
			t.Errorf("second request: expected status %d, got %d", http.StatusTooManyRequests, rr.Code)
		}

		retryAfter := rr.Header().Get("Retry-After")
		if retryAfter == "" {
			t.Error("expected Retry-After header")
		}
		retryAfterInt, _ := strconv.Atoi(retryAfter)
		if retryAfterInt < 1 {
			t.Errorf("expected Retry-After >= 1, got %s", retryAfter)
		}
	})
}

func TestRateLimiter_GetClientIP(t *testing.T) {
	t.Run("without trusted proxies", func(t *testing.T) {
		// By default, proxy headers are NOT trusted (secure default)
		rl := NewRateLimiter(10, 10)
		defer rl.Stop()

		tests := []struct {
			name       string
			remoteAddr string
			xff        string
			xri        string
			expected   string
		}{
			{
				name:       "uses RemoteAddr when no proxy headers",
				remoteAddr: "192.168.1.1:12345",
				expected:   "192.168.1.1",
			},
			{
				name:       "ignores X-Forwarded-For without trusted proxy",
				remoteAddr: "10.0.0.1:12345",
				xff:        "203.0.113.195",
				expected:   "10.0.0.1", // Should NOT trust XFF
			},
			{
				name:       "ignores X-Real-IP without trusted proxy",
				remoteAddr: "10.0.0.1:12345",
				xri:        "203.0.113.195",
				expected:   "10.0.0.1", // Should NOT trust XRI
			},
			{
				name:       "handles RemoteAddr without port",
				remoteAddr: "192.168.1.1",
				expected:   "192.168.1.1",
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				req := httptest.NewRequest("GET", "/", nil)
				req.RemoteAddr = tt.remoteAddr
				if tt.xff != "" {
					req.Header.Set("X-Forwarded-For", tt.xff)
				}
				if tt.xri != "" {
					req.Header.Set("X-Real-IP", tt.xri)
				}

				got := rl.getClientIP(req)
				if got != tt.expected {
					t.Errorf("getClientIP() = %v, want %v", got, tt.expected)
				}
			})
		}
	})

	t.Run("with trusted proxies configured", func(t *testing.T) {
		// Create rate limiter with trusted proxy CIDR
		rl := &RateLimiter{
			rps:       10,
			burst:     10,
			buckets:   make(map[string]*tokenBucket),
			stopCh:    make(chan struct{}),
			stoppedCh: make(chan struct{}),
		}
		WithTrustedProxies([]string{"10.0.0.0/8"})(rl)
		go rl.cleanup()
		defer rl.Stop()

		tests := []struct {
			name       string
			remoteAddr string
			xff        string
			xri        string
			expected   string
		}{
			{
				name:       "uses X-Forwarded-For from trusted proxy",
				remoteAddr: "10.0.0.1:12345", // In trusted range
				xff:        "203.0.113.195",
				expected:   "203.0.113.195",
			},
			{
				name:       "uses first IP from X-Forwarded-For chain",
				remoteAddr: "10.0.0.1:12345",
				xff:        "203.0.113.195, 70.41.3.18, 150.172.238.178",
				expected:   "203.0.113.195",
			},
			{
				name:       "uses X-Real-IP from trusted proxy",
				remoteAddr: "10.0.0.1:12345",
				xri:        "203.0.113.195",
				expected:   "203.0.113.195",
			},
			{
				name:       "prefers X-Forwarded-For over X-Real-IP",
				remoteAddr: "10.0.0.1:12345",
				xff:        "203.0.113.195",
				xri:        "70.41.3.18",
				expected:   "203.0.113.195",
			},
			{
				name:       "ignores headers from untrusted source",
				remoteAddr: "192.168.1.1:12345", // NOT in trusted range
				xff:        "203.0.113.195",
				expected:   "192.168.1.1",
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				req := httptest.NewRequest("GET", "/", nil)
				req.RemoteAddr = tt.remoteAddr
				if tt.xff != "" {
					req.Header.Set("X-Forwarded-For", tt.xff)
				}
				if tt.xri != "" {
					req.Header.Set("X-Real-IP", tt.xri)
				}

				got := rl.getClientIP(req)
				if got != tt.expected {
					t.Errorf("getClientIP() = %v, want %v", got, tt.expected)
				}
			})
		}
	})

	t.Run("with trust all proxies", func(t *testing.T) {
		rl := &RateLimiter{
			rps:       10,
			burst:     10,
			buckets:   make(map[string]*tokenBucket),
			stopCh:    make(chan struct{}),
			stoppedCh: make(chan struct{}),
		}
		WithTrustAllProxies()(rl)
		go rl.cleanup()
		defer rl.Stop()

		req := httptest.NewRequest("GET", "/", nil)
		req.RemoteAddr = "192.168.1.1:12345"
		req.Header.Set("X-Forwarded-For", "203.0.113.195")

		got := rl.getClientIP(req)
		if got != "203.0.113.195" {
			t.Errorf("getClientIP() = %v, want 203.0.113.195", got)
		}
	})
}

func TestRateLimiter_Cleanup(t *testing.T) {
	// This test verifies that stale entries are cleaned up
	// We can't easily test the cleanup goroutine timing, but we can test removeStaleEntries

	rl := NewRateLimiter(10, 10)
	defer rl.Stop()

	// Add an entry
	rl.allow("192.168.1.1")

	// Verify entry exists
	rl.mu.RLock()
	_, exists := rl.buckets["192.168.1.1"]
	rl.mu.RUnlock()
	if !exists {
		t.Error("expected bucket to exist")
	}

	// Manually set lastUpdate to be old
	rl.mu.Lock()
	bucket := rl.buckets["192.168.1.1"]
	bucket.mu.Lock()
	bucket.lastUpdate = time.Now().Add(-2 * DefaultEntryTTL)
	bucket.mu.Unlock()
	rl.mu.Unlock()

	// Run cleanup
	rl.removeStaleEntries()

	// Verify entry was removed
	rl.mu.RLock()
	_, exists = rl.buckets["192.168.1.1"]
	rl.mu.RUnlock()
	if exists {
		t.Error("expected bucket to be removed after cleanup")
	}
}
