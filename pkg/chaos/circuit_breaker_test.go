package chaos

import (
	"math/rand"
	"net/http/httptest"
	"testing"
	"time"
)

func TestCircuitBreakerStates(t *testing.T) {
	t.Run("initial state is closed", func(t *testing.T) {
		cb := NewCircuitBreaker(CircuitBreakerConfig{
			FailureThreshold: 5,
			SuccessThreshold: 3,
			OpenDuration:     30 * time.Second,
			OpenStatusCode:   503,
		}, rand.New(rand.NewSource(42)))

		if cb.State() != CircuitClosed {
			t.Errorf("expected closed, got %s", cb.State())
		}
	})

	t.Run("closed state passes requests through", func(t *testing.T) {
		cb := NewCircuitBreaker(CircuitBreakerConfig{
			FailureThreshold: 5,
			SuccessThreshold: 3,
			OpenDuration:     30 * time.Second,
			OpenStatusCode:   503,
			ClosedErrorRate:  0, // No errors in closed state
		}, rand.New(rand.NewSource(42)))

		w := httptest.NewRecorder()
		handled := cb.Handle(w)

		if handled {
			t.Error("expected request to pass through in closed state")
		}
	})

	t.Run("deterministic trip after N requests", func(t *testing.T) {
		cb := NewCircuitBreaker(CircuitBreakerConfig{
			FailureThreshold: 5,
			SuccessThreshold: 3,
			OpenDuration:     30 * time.Second,
			OpenStatusCode:   503,
			TripAfter:        3,
		}, rand.New(rand.NewSource(42)))

		// Requests 1 and 2 pass
		for i := 0; i < 2; i++ {
			w := httptest.NewRecorder()
			if cb.Handle(w) {
				t.Errorf("request %d should pass through", i+1)
			}
		}

		// Request 3 trips the circuit
		w := httptest.NewRecorder()
		if !cb.Handle(w) {
			t.Error("request 3 should trip the circuit")
		}
		if w.Code != 503 {
			t.Errorf("expected 503, got %d", w.Code)
		}
		if cb.State() != CircuitOpen {
			t.Errorf("expected open, got %s", cb.State())
		}
	})

	t.Run("error-based trip with closedErrorRate", func(t *testing.T) {
		// Use 100% error rate in closed state to guarantee consecutive failures
		cb := NewCircuitBreaker(CircuitBreakerConfig{
			FailureThreshold: 3,
			SuccessThreshold: 3,
			OpenDuration:     30 * time.Second,
			OpenStatusCode:   503,
			ClosedErrorRate:  1.0, // Always fail
		}, rand.New(rand.NewSource(42)))

		// First 2 errors don't trip (threshold is 3)
		for i := 0; i < 2; i++ {
			w := httptest.NewRecorder()
			handled := cb.Handle(w)
			if !handled {
				t.Errorf("request %d should inject error", i+1)
			}
			if w.Code != 500 {
				t.Errorf("request %d: expected 500, got %d", i+1, w.Code)
			}
		}

		if cb.State() != CircuitClosed {
			t.Errorf("should still be closed after 2 failures, got %s", cb.State())
		}

		// 3rd error trips the circuit (returns 503, not 500)
		w := httptest.NewRecorder()
		handled := cb.Handle(w)
		if !handled {
			t.Error("request 3 should trip the circuit")
		}
		if w.Code != 503 {
			t.Errorf("expected 503 (circuit open), got %d", w.Code)
		}
		if cb.State() != CircuitOpen {
			t.Errorf("expected open, got %s", cb.State())
		}
	})

	t.Run("open state rejects all requests", func(t *testing.T) {
		cb := NewCircuitBreaker(CircuitBreakerConfig{
			FailureThreshold: 1,
			SuccessThreshold: 3,
			OpenDuration:     1 * time.Hour, // Long enough to stay open
			OpenStatusCode:   503,
			ClosedErrorRate:  1.0,
		}, rand.New(rand.NewSource(42)))

		// Trip the circuit
		w := httptest.NewRecorder()
		cb.Handle(w)

		// All subsequent requests should be rejected
		for i := 0; i < 5; i++ {
			w := httptest.NewRecorder()
			handled := cb.Handle(w)
			if !handled {
				t.Errorf("request %d should be rejected in open state", i)
			}
			if w.Code != 503 {
				t.Errorf("expected 503, got %d", w.Code)
			}
			if w.Header().Get("Retry-After") == "" {
				t.Error("expected Retry-After header")
			}
			if w.Header().Get("X-Circuit-State") != "open" {
				t.Errorf("expected X-Circuit-State: open, got %s", w.Header().Get("X-Circuit-State"))
			}
		}
	})

	t.Run("open transitions to half-open after duration", func(t *testing.T) {
		cb := NewCircuitBreaker(CircuitBreakerConfig{
			FailureThreshold:  1,
			SuccessThreshold:  3,
			OpenDuration:      10 * time.Millisecond, // Very short
			OpenStatusCode:    503,
			ClosedErrorRate:   1.0,
			HalfOpenErrorRate: 0, // No errors in half-open
		}, rand.New(rand.NewSource(42)))

		// Trip the circuit
		w := httptest.NewRecorder()
		cb.Handle(w)

		if cb.State() != CircuitOpen {
			t.Errorf("expected open, got %s", cb.State())
		}

		// Wait for open duration
		time.Sleep(15 * time.Millisecond)

		// Should be half-open now
		if cb.State() != CircuitHalfOpen {
			t.Errorf("expected half_open, got %s", cb.State())
		}
	})

	t.Run("half-open closes after success threshold", func(t *testing.T) {
		cb := NewCircuitBreaker(CircuitBreakerConfig{
			FailureThreshold:  1,
			SuccessThreshold:  2,
			OpenDuration:      1 * time.Millisecond,
			OpenStatusCode:    503,
			ClosedErrorRate:   1.0,
			HalfOpenErrorRate: 0, // All requests succeed in half-open
		}, rand.New(rand.NewSource(42)))

		// Trip and wait for half-open
		w := httptest.NewRecorder()
		cb.Handle(w)
		time.Sleep(5 * time.Millisecond)

		// 2 successes should close the circuit
		for i := 0; i < 2; i++ {
			w := httptest.NewRecorder()
			handled := cb.Handle(w)
			if handled {
				t.Errorf("request %d should pass through in half-open", i+1)
			}
		}

		if cb.State() != CircuitClosed {
			t.Errorf("expected closed after success threshold, got %s", cb.State())
		}
	})

	t.Run("half-open trips to open on failure", func(t *testing.T) {
		cb := NewCircuitBreaker(CircuitBreakerConfig{
			FailureThreshold:  1,
			SuccessThreshold:  3,
			OpenDuration:      1 * time.Millisecond,
			OpenStatusCode:    503,
			ClosedErrorRate:   1.0,
			HalfOpenErrorRate: 1.0, // Always fail in half-open
		}, rand.New(rand.NewSource(42)))

		// Trip and wait for half-open
		w := httptest.NewRecorder()
		cb.Handle(w)
		time.Sleep(5 * time.Millisecond)

		// Failure in half-open should trip back to open
		w = httptest.NewRecorder()
		handled := cb.Handle(w)
		if !handled {
			t.Error("expected failure in half-open")
		}
		if w.Code != 503 {
			t.Errorf("expected 503, got %d", w.Code)
		}
		if cb.State() != CircuitOpen {
			t.Errorf("expected open after half-open failure, got %s", cb.State())
		}
	})
}

func TestCircuitBreakerAdminControl(t *testing.T) {
	t.Run("manual trip via Trip()", func(t *testing.T) {
		cb := NewCircuitBreaker(CircuitBreakerConfig{
			FailureThreshold: 100,
			SuccessThreshold: 3,
			OpenDuration:     30 * time.Second,
			OpenStatusCode:   503,
		}, rand.New(rand.NewSource(42)))

		cb.Trip()
		if cb.State() != CircuitOpen {
			t.Errorf("expected open after Trip(), got %s", cb.State())
		}

		w := httptest.NewRecorder()
		handled := cb.Handle(w)
		if !handled {
			t.Error("should reject after Trip()")
		}
	})

	t.Run("manual reset via Reset()", func(t *testing.T) {
		cb := NewCircuitBreaker(CircuitBreakerConfig{
			FailureThreshold: 1,
			SuccessThreshold: 3,
			OpenDuration:     1 * time.Hour,
			OpenStatusCode:   503,
			ClosedErrorRate:  1.0,
		}, rand.New(rand.NewSource(42)))

		// Trip the circuit
		w := httptest.NewRecorder()
		cb.Handle(w)

		if cb.State() != CircuitOpen {
			t.Errorf("expected open, got %s", cb.State())
		}

		// Reset manually
		cb.Reset()
		if cb.State() != CircuitClosed {
			t.Errorf("expected closed after Reset(), got %s", cb.State())
		}
	})
}

func TestCircuitBreakerStats(t *testing.T) {
	cb := NewCircuitBreaker(CircuitBreakerConfig{
		FailureThreshold:  2,
		SuccessThreshold:  1,
		OpenDuration:      1 * time.Millisecond,
		OpenStatusCode:    503,
		ClosedErrorRate:   1.0,
		HalfOpenErrorRate: 0,
	}, rand.New(rand.NewSource(42)))

	// Trip circuit (2 errors)
	for i := 0; i < 2; i++ {
		w := httptest.NewRecorder()
		cb.Handle(w)
	}

	stats := cb.Stats()
	if stats.State != "open" {
		t.Errorf("expected state=open, got %s", stats.State)
	}
	if stats.TotalTrips != 1 {
		t.Errorf("expected 1 trip, got %d", stats.TotalTrips)
	}
	if stats.TotalRequests != 2 {
		t.Errorf("expected 2 total requests, got %d", stats.TotalRequests)
	}
	if stats.OpenedAt == "" {
		t.Error("expected openedAt to be set")
	}
}

func TestCircuitBreakerOpenBody(t *testing.T) {
	cb := NewCircuitBreaker(CircuitBreakerConfig{
		FailureThreshold: 1,
		OpenDuration:     1 * time.Hour,
		OpenStatusCode:   429,
		OpenBody:         `{"error":"rate limit exceeded"}`,
		ClosedErrorRate:  1.0,
	}, rand.New(rand.NewSource(42)))

	w := httptest.NewRecorder()
	cb.Handle(w)

	if w.Code != 429 {
		t.Errorf("expected 429, got %d", w.Code)
	}
	if w.Body.String() != `{"error":"rate limit exceeded"}` {
		t.Errorf("unexpected body: %s", w.Body.String())
	}
}

func TestParseCircuitBreakerConfig(t *testing.T) {
	t.Run("defaults", func(t *testing.T) {
		cfg := ParseCircuitBreakerConfig(nil)
		if cfg.FailureThreshold != 5 {
			t.Errorf("expected failureThreshold=5, got %d", cfg.FailureThreshold)
		}
		if cfg.SuccessThreshold != 3 {
			t.Errorf("expected successThreshold=3, got %d", cfg.SuccessThreshold)
		}
		if cfg.OpenDuration != 30*time.Second {
			t.Errorf("expected openDuration=30s, got %s", cfg.OpenDuration)
		}
		if cfg.OpenStatusCode != 503 {
			t.Errorf("expected openStatusCode=503, got %d", cfg.OpenStatusCode)
		}
		if cfg.HalfOpenErrorRate != 0.5 {
			t.Errorf("expected halfOpenErrorRate=0.5, got %f", cfg.HalfOpenErrorRate)
		}
	})

	t.Run("custom values", func(t *testing.T) {
		cfg := ParseCircuitBreakerConfig(map[string]interface{}{
			"failureThreshold":  float64(10),
			"successThreshold":  float64(5),
			"openDuration":      "60s",
			"openStatusCode":    float64(429),
			"closedErrorRate":   0.2,
			"halfOpenErrorRate": 0.3,
			"tripAfter":         float64(100),
			"openBody":          `{"error":"circuit open"}`,
		})
		if cfg.FailureThreshold != 10 {
			t.Errorf("expected 10, got %d", cfg.FailureThreshold)
		}
		if cfg.SuccessThreshold != 5 {
			t.Errorf("expected 5, got %d", cfg.SuccessThreshold)
		}
		if cfg.OpenDuration != 60*time.Second {
			t.Errorf("expected 60s, got %s", cfg.OpenDuration)
		}
		if cfg.OpenStatusCode != 429 {
			t.Errorf("expected 429, got %d", cfg.OpenStatusCode)
		}
		if cfg.ClosedErrorRate != 0.2 {
			t.Errorf("expected 0.2, got %f", cfg.ClosedErrorRate)
		}
		if cfg.HalfOpenErrorRate != 0.3 {
			t.Errorf("expected 0.3, got %f", cfg.HalfOpenErrorRate)
		}
		if cfg.TripAfter != 100 {
			t.Errorf("expected 100, got %d", cfg.TripAfter)
		}
	})
}

func TestCircuitBreakerStateString(t *testing.T) {
	tests := []struct {
		state CircuitState
		want  string
	}{
		{CircuitClosed, "closed"},
		{CircuitOpen, "open"},
		{CircuitHalfOpen, "half_open"},
		{CircuitState(99), "unknown"},
	}
	for _, tt := range tests {
		if got := tt.state.String(); got != tt.want {
			t.Errorf("CircuitState(%d).String() = %q, want %q", tt.state, got, tt.want)
		}
	}
}
