package ratelimit

import (
	"net/http"
	"testing"
	"time"
)

func TestNewPerIPLimiter_Defaults(t *testing.T) {
	t.Parallel()
	limiter := NewPerIPLimiter(PerIPConfig{})
	defer limiter.Stop()

	// Default rate is 100, default burst is rate*2 = 200.
	if limiter.rps != 100 {
		t.Errorf("expected default rps 100, got %v", limiter.rps)
	}
	if limiter.burst != 200 {
		t.Errorf("expected default burst 200, got %v", limiter.burst)
	}
	if limiter.Burst() != 200 {
		t.Errorf("expected Burst() 200, got %v", limiter.Burst())
	}
	if limiter.cleanupInterval != DefaultCleanupInterval {
		t.Errorf("expected default cleanup interval %v, got %v", DefaultCleanupInterval, limiter.cleanupInterval)
	}
	if limiter.entryTTL != DefaultEntryTTL {
		t.Errorf("expected default entry TTL %v, got %v", DefaultEntryTTL, limiter.entryTTL)
	}
}

func TestAllow_NewIP_ReturnsTrue(t *testing.T) {
	t.Parallel()
	limiter := NewPerIPLimiter(PerIPConfig{Rate: 100, Burst: 10})
	defer limiter.Stop()

	allowed, remaining, resetSec := limiter.Allow("10.0.0.1")
	if !allowed {
		t.Error("expected Allow to return true for new IP")
	}
	if remaining != 9 {
		t.Errorf("expected remaining 9, got %d", remaining)
	}
	// resetSec is time until bucket is full again; should be >= 0.
	if resetSec < 0 {
		t.Errorf("expected non-negative reset, got %d", resetSec)
	}
}

func TestAllow_Exhausted_ReturnsFalse(t *testing.T) {
	t.Parallel()
	limiter := NewPerIPLimiter(PerIPConfig{Rate: 1, Burst: 3})
	defer limiter.Stop()

	ip := "10.0.0.2"
	// Drain all 3 tokens.
	for i := 0; i < 3; i++ {
		allowed, _, _ := limiter.Allow(ip)
		if !allowed {
			t.Fatalf("Allow #%d should have succeeded", i+1)
		}
	}

	allowed, remaining, retryAfter := limiter.Allow(ip)
	if allowed {
		t.Error("expected Allow to return false when exhausted")
	}
	if remaining != 0 {
		t.Errorf("expected remaining 0 when exhausted, got %d", remaining)
	}
	if retryAfter < 1 {
		t.Errorf("expected retryAfter >= 1, got %d", retryAfter)
	}
}

func TestAllow_DifferentIPsAreIndependent(t *testing.T) {
	t.Parallel()
	limiter := NewPerIPLimiter(PerIPConfig{Rate: 1, Burst: 1})
	defer limiter.Stop()

	// Drain IP A.
	allowed, _, _ := limiter.Allow("10.0.0.1")
	if !allowed {
		t.Fatal("first Allow for IP A should succeed")
	}
	allowed, _, _ = limiter.Allow("10.0.0.1")
	if allowed {
		t.Fatal("second Allow for IP A should fail (drained)")
	}

	// IP B should still have its own full bucket.
	allowed, remaining, _ := limiter.Allow("10.0.0.2")
	if !allowed {
		t.Error("Allow for IP B should succeed (independent bucket)")
	}
	if remaining != 0 {
		t.Errorf("expected remaining 0 for burst=1 after one Allow, got %d", remaining)
	}
}

func TestPerIP_TokenRefillOverTime(t *testing.T) {
	t.Parallel()
	// Rate 100/s, burst 1. Drain, sleep 50ms → ~5 tokens refilled, capped at 1.
	limiter := NewPerIPLimiter(PerIPConfig{Rate: 100, Burst: 1})
	defer limiter.Stop()

	ip := "10.0.0.3"
	limiter.Allow(ip) // drain

	allowed, _, _ := limiter.Allow(ip)
	if allowed {
		t.Fatal("should be empty immediately after drain")
	}

	time.Sleep(50 * time.Millisecond)

	allowed, _, _ = limiter.Allow(ip)
	if !allowed {
		t.Error("expected Allow to succeed after refill period")
	}
}

func TestStop_StopsCleanupGoroutine(t *testing.T) {
	t.Parallel()
	limiter := NewPerIPLimiter(PerIPConfig{
		Rate:            100,
		Burst:           10,
		CleanupInterval: 10 * time.Millisecond,
	})

	// Stop should return promptly, confirming the goroutine exited.
	done := make(chan struct{})
	go func() {
		limiter.Stop()
		close(done)
	}()

	select {
	case <-done:
		// Success: Stop returned.
	case <-time.After(2 * time.Second):
		t.Fatal("Stop did not return in time — possible goroutine leak")
	}
}

func TestClientIP_ExtractsFromRemoteAddrWithPort(t *testing.T) {
	t.Parallel()
	limiter := NewPerIPLimiter(PerIPConfig{Rate: 100, Burst: 10})
	defer limiter.Stop()

	r := &http.Request{RemoteAddr: "192.168.1.50:12345"}
	ip := limiter.ClientIP(r)
	if ip != "192.168.1.50" {
		t.Errorf("expected 192.168.1.50, got %s", ip)
	}
}

func TestClientIP_ExtractsFromRemoteAddrWithoutPort(t *testing.T) {
	t.Parallel()
	limiter := NewPerIPLimiter(PerIPConfig{Rate: 100, Burst: 10})
	defer limiter.Stop()

	r := &http.Request{RemoteAddr: "192.168.1.50"}
	ip := limiter.ClientIP(r)
	if ip != "192.168.1.50" {
		t.Errorf("expected 192.168.1.50, got %s", ip)
	}
}

func TestClientIP_RespectsXForwardedForWhenTrusted(t *testing.T) {
	t.Parallel()
	limiter := NewPerIPLimiter(PerIPConfig{
		Rate:            100,
		Burst:           10,
		TrustAllProxies: true,
	})
	defer limiter.Stop()

	r := &http.Request{
		RemoteAddr: "10.0.0.1:1234",
		Header:     http.Header{"X-Forwarded-For": []string{"203.0.113.50"}},
	}
	ip := limiter.ClientIP(r)
	if ip != "203.0.113.50" {
		t.Errorf("expected 203.0.113.50, got %s", ip)
	}
}

func TestClientIP_IgnoresXForwardedForWhenNotTrusted(t *testing.T) {
	t.Parallel()
	limiter := NewPerIPLimiter(PerIPConfig{
		Rate:  100,
		Burst: 10,
		// No TrustAllProxies or TrustedProxies — proxy headers should be ignored.
	})
	defer limiter.Stop()

	r := &http.Request{
		RemoteAddr: "10.0.0.1:1234",
		Header:     http.Header{"X-Forwarded-For": []string{"203.0.113.50"}},
	}
	ip := limiter.ClientIP(r)
	if ip != "10.0.0.1" {
		t.Errorf("expected 10.0.0.1 (ignoring XFF), got %s", ip)
	}
}

func TestClientIP_RespectsXRealIPWhenTrusted(t *testing.T) {
	t.Parallel()
	limiter := NewPerIPLimiter(PerIPConfig{
		Rate:            100,
		Burst:           10,
		TrustAllProxies: true,
	})
	defer limiter.Stop()

	r := &http.Request{
		RemoteAddr: "10.0.0.1:1234",
		Header:     http.Header{"X-Real-Ip": []string{"198.51.100.42"}},
	}
	ip := limiter.ClientIP(r)
	if ip != "198.51.100.42" {
		t.Errorf("expected 198.51.100.42, got %s", ip)
	}
}

func TestClientIP_UsesFirstIPFromMultiValueXFF(t *testing.T) {
	t.Parallel()
	limiter := NewPerIPLimiter(PerIPConfig{
		Rate:            100,
		Burst:           10,
		TrustAllProxies: true,
	})
	defer limiter.Stop()

	r := &http.Request{
		RemoteAddr: "10.0.0.1:1234",
		Header:     http.Header{"X-Forwarded-For": []string{"203.0.113.1, 203.0.113.2, 203.0.113.3"}},
	}
	ip := limiter.ClientIP(r)
	if ip != "203.0.113.1" {
		t.Errorf("expected first IP 203.0.113.1, got %s", ip)
	}
}

func TestClientIP_XFFTakesPrecedenceOverXRealIP(t *testing.T) {
	t.Parallel()
	limiter := NewPerIPLimiter(PerIPConfig{
		Rate:            100,
		Burst:           10,
		TrustAllProxies: true,
	})
	defer limiter.Stop()

	r := &http.Request{
		RemoteAddr: "10.0.0.1:1234",
		Header: http.Header{
			"X-Forwarded-For": []string{"203.0.113.50"},
			"X-Real-Ip":       []string{"198.51.100.42"},
		},
	}
	ip := limiter.ClientIP(r)
	if ip != "203.0.113.50" {
		t.Errorf("expected XFF IP 203.0.113.50 to take precedence, got %s", ip)
	}
}

func TestTrustAllProxies_TrustsAnySource(t *testing.T) {
	t.Parallel()
	limiter := NewPerIPLimiter(PerIPConfig{
		Rate:            100,
		Burst:           10,
		TrustAllProxies: true,
	})
	defer limiter.Stop()

	// Any remote address should be trusted as a proxy.
	for _, remote := range []string{"1.2.3.4:80", "192.168.0.1:443", "::1:9999"} {
		r := &http.Request{
			RemoteAddr: remote,
			Header:     http.Header{"X-Forwarded-For": []string{"99.99.99.99"}},
		}
		ip := limiter.ClientIP(r)
		if ip != "99.99.99.99" {
			t.Errorf("remote=%s: expected 99.99.99.99, got %s", remote, ip)
		}
	}
}

func TestTrustedProxyCIDR(t *testing.T) {
	t.Parallel()
	limiter := NewPerIPLimiter(PerIPConfig{
		Rate:           100,
		Burst:          10,
		TrustedProxies: []string{"10.0.0.0/8"},
	})
	defer limiter.Stop()

	t.Run("trusted proxy in CIDR", func(t *testing.T) {
		r := &http.Request{
			RemoteAddr: "10.1.2.3:1234",
			Header:     http.Header{"X-Forwarded-For": []string{"203.0.113.50"}},
		}
		ip := limiter.ClientIP(r)
		if ip != "203.0.113.50" {
			t.Errorf("expected 203.0.113.50 via trusted proxy, got %s", ip)
		}
	})

	t.Run("untrusted proxy outside CIDR", func(t *testing.T) {
		r := &http.Request{
			RemoteAddr: "192.168.1.1:1234",
			Header:     http.Header{"X-Forwarded-For": []string{"203.0.113.50"}},
		}
		ip := limiter.ClientIP(r)
		if ip != "192.168.1.1" {
			t.Errorf("expected 192.168.1.1 (untrusted, ignore XFF), got %s", ip)
		}
	})
}

func TestTrustedProxies_SingleIP(t *testing.T) {
	t.Parallel()
	limiter := NewPerIPLimiter(PerIPConfig{
		Rate:           100,
		Burst:          10,
		TrustedProxies: []string{"10.0.0.1"},
	})
	defer limiter.Stop()

	t.Run("exact match trusted", func(t *testing.T) {
		r := &http.Request{
			RemoteAddr: "10.0.0.1:1234",
			Header:     http.Header{"X-Forwarded-For": []string{"203.0.113.50"}},
		}
		ip := limiter.ClientIP(r)
		if ip != "203.0.113.50" {
			t.Errorf("expected 203.0.113.50, got %s", ip)
		}
	})

	t.Run("different IP not trusted", func(t *testing.T) {
		r := &http.Request{
			RemoteAddr: "10.0.0.2:1234",
			Header:     http.Header{"X-Forwarded-For": []string{"203.0.113.50"}},
		}
		ip := limiter.ClientIP(r)
		if ip != "10.0.0.2" {
			t.Errorf("expected 10.0.0.2, got %s", ip)
		}
	})
}

func TestExtractRemoteIP(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"ip:port format", "192.168.1.1:8080", "192.168.1.1"},
		{"ipv6 with port", "[::1]:8080", "::1"},
		{"bare IPv4", "192.168.1.1", "192.168.1.1"},
		{"bare IPv6", "::1", "::1"},
		{"empty string", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := extractRemoteIP(tt.input)
			if result != tt.expected {
				t.Errorf("extractRemoteIP(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestIsValidIP(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{"valid IPv4", "192.168.1.1", true},
		{"valid IPv6", "::1", true},
		{"valid IPv6 full", "2001:db8::1", true},
		{"localhost IPv4", "127.0.0.1", true},
		{"garbage", "notanip", false},
		{"empty string", "", false},
		{"ip with port", "192.168.1.1:80", false},
		{"hostname", "example.com", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := isValidIP(tt.input)
			if result != tt.expected {
				t.Errorf("isValidIP(%q) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestIsTrustedProxy(t *testing.T) {
	t.Parallel()

	t.Run("no trust configured", func(t *testing.T) {
		t.Parallel()
		limiter := NewPerIPLimiter(PerIPConfig{Rate: 100, Burst: 10})
		defer limiter.Stop()

		if limiter.isTrustedProxy("10.0.0.1") {
			t.Error("expected no trust when nothing configured")
		}
	})

	t.Run("trust all proxies", func(t *testing.T) {
		t.Parallel()
		limiter := NewPerIPLimiter(PerIPConfig{Rate: 100, Burst: 10, TrustAllProxies: true})
		defer limiter.Stop()

		if !limiter.isTrustedProxy("1.2.3.4") {
			t.Error("expected any IP to be trusted with TrustAllProxies")
		}
		if !limiter.isTrustedProxy("192.168.0.1") {
			t.Error("expected any IP to be trusted with TrustAllProxies")
		}
	})

	t.Run("CIDR trust", func(t *testing.T) {
		t.Parallel()
		limiter := NewPerIPLimiter(PerIPConfig{
			Rate:           100,
			Burst:          10,
			TrustedProxies: []string{"10.0.0.0/24"},
		})
		defer limiter.Stop()

		if !limiter.isTrustedProxy("10.0.0.1") {
			t.Error("10.0.0.1 should be in 10.0.0.0/24")
		}
		if !limiter.isTrustedProxy("10.0.0.254") {
			t.Error("10.0.0.254 should be in 10.0.0.0/24")
		}
		if limiter.isTrustedProxy("10.0.1.1") {
			t.Error("10.0.1.1 should NOT be in 10.0.0.0/24")
		}
	})

	t.Run("invalid IP returns false", func(t *testing.T) {
		t.Parallel()
		limiter := NewPerIPLimiter(PerIPConfig{
			Rate:           100,
			Burst:          10,
			TrustedProxies: []string{"10.0.0.0/8"},
		})
		defer limiter.Stop()

		if limiter.isTrustedProxy("not-an-ip") {
			t.Error("invalid IP should not be trusted")
		}
	})
}
