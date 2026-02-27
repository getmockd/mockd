package chaos

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"
)

func TestRetryAfterTracker(t *testing.T) {
	t.Run("first request triggers rate limiting", func(t *testing.T) {
		tracker := NewRetryAfterTracker(RetryAfterConfig{
			StatusCode: http.StatusTooManyRequests,
			RetryAfter: 1 * time.Second,
		})

		w := httptest.NewRecorder()
		limited := tracker.Handle(w)

		if !limited {
			t.Error("first request should trigger rate limiting")
		}
		if w.Code != 429 {
			t.Errorf("expected 429, got %d", w.Code)
		}
		retryAfter := w.Header().Get("Retry-After")
		if retryAfter == "" {
			t.Error("expected Retry-After header")
		}
		secs, _ := strconv.Atoi(retryAfter)
		if secs < 1 {
			t.Errorf("expected Retry-After >= 1, got %d", secs)
		}
	})

	t.Run("subsequent requests during window are limited", func(t *testing.T) {
		tracker := NewRetryAfterTracker(RetryAfterConfig{
			StatusCode: http.StatusTooManyRequests,
			RetryAfter: 1 * time.Second,
		})

		// First request triggers limiting
		w := httptest.NewRecorder()
		tracker.Handle(w)

		// Subsequent requests should also be limited
		for i := 0; i < 3; i++ {
			w = httptest.NewRecorder()
			limited := tracker.Handle(w)
			if !limited {
				t.Errorf("request %d should be rate limited", i+1)
			}
			if w.Code != 429 {
				t.Errorf("expected 429, got %d", w.Code)
			}
		}
	})

	t.Run("recovers after retry-after duration", func(t *testing.T) {
		tracker := NewRetryAfterTracker(RetryAfterConfig{
			StatusCode: http.StatusTooManyRequests,
			RetryAfter: 10 * time.Millisecond,
		})

		// Trigger limiting
		w := httptest.NewRecorder()
		tracker.Handle(w)

		// Wait for recovery
		time.Sleep(15 * time.Millisecond)

		// Should pass through
		w = httptest.NewRecorder()
		limited := tracker.Handle(w)
		if limited {
			t.Error("should have recovered after retry-after duration")
		}
	})

	t.Run("re-triggers after recovery", func(t *testing.T) {
		tracker := NewRetryAfterTracker(RetryAfterConfig{
			StatusCode: http.StatusTooManyRequests,
			RetryAfter: 10 * time.Millisecond,
		})

		// First cycle: trigger → recover
		w := httptest.NewRecorder()
		tracker.Handle(w)
		time.Sleep(15 * time.Millisecond)

		// This passes (recovery)
		w = httptest.NewRecorder()
		tracker.Handle(w)

		// Next request should trigger again (new cycle)
		w = httptest.NewRecorder()
		limited := tracker.Handle(w)
		if !limited {
			t.Error("should re-trigger after recovery")
		}
		if w.Code != 429 {
			t.Errorf("expected 429, got %d", w.Code)
		}
	})

	t.Run("503 status code", func(t *testing.T) {
		tracker := NewRetryAfterTracker(RetryAfterConfig{
			StatusCode: http.StatusServiceUnavailable,
			RetryAfter: 30 * time.Second,
		})

		w := httptest.NewRecorder()
		tracker.Handle(w)

		if w.Code != 503 {
			t.Errorf("expected 503, got %d", w.Code)
		}
	})

	t.Run("custom body", func(t *testing.T) {
		tracker := NewRetryAfterTracker(RetryAfterConfig{
			StatusCode: http.StatusTooManyRequests,
			RetryAfter: 30 * time.Second,
			Body:       `{"error":"rate limit exceeded","plan":"free"}`,
		})

		w := httptest.NewRecorder()
		tracker.Handle(w)

		if w.Body.String() != `{"error":"rate limit exceeded","plan":"free"}` {
			t.Errorf("unexpected body: %s", w.Body.String())
		}
	})

	t.Run("default body when none specified", func(t *testing.T) {
		tracker := NewRetryAfterTracker(RetryAfterConfig{
			StatusCode: http.StatusTooManyRequests,
			RetryAfter: 30 * time.Second,
		})

		w := httptest.NewRecorder()
		tracker.Handle(w)

		body := w.Body.String()
		if body == "" {
			t.Error("expected non-empty default body")
		}
	})

	t.Run("reset clears state", func(t *testing.T) {
		tracker := NewRetryAfterTracker(RetryAfterConfig{
			StatusCode: http.StatusTooManyRequests,
			RetryAfter: 1 * time.Hour,
		})

		// Trigger limiting
		w := httptest.NewRecorder()
		tracker.Handle(w)

		// Reset
		tracker.Reset()

		stats := tracker.Stats()
		if stats.IsLimited {
			t.Error("should not be limited after reset")
		}
		if stats.TotalLimited != 0 {
			t.Errorf("expected 0 total limited, got %d", stats.TotalLimited)
		}
	})

	t.Run("stats tracking", func(t *testing.T) {
		tracker := NewRetryAfterTracker(RetryAfterConfig{
			StatusCode: http.StatusTooManyRequests,
			RetryAfter: 10 * time.Millisecond,
		})

		// Trigger and send 2 more during window
		for i := 0; i < 3; i++ {
			w := httptest.NewRecorder()
			tracker.Handle(w)
		}

		stats := tracker.Stats()
		if !stats.IsLimited {
			t.Error("should be limited")
		}
		if stats.TotalLimited != 1 {
			t.Errorf("expected 1 trigger (first request), got %d", stats.TotalLimited)
		}

		// Wait and recover
		time.Sleep(15 * time.Millisecond)
		w := httptest.NewRecorder()
		tracker.Handle(w)

		stats = tracker.Stats()
		if stats.TotalPassed != 1 {
			t.Errorf("expected 1 passed, got %d", stats.TotalPassed)
		}
	})
}

func TestParseRetryAfterConfig(t *testing.T) {
	t.Run("defaults", func(t *testing.T) {
		cfg := ParseRetryAfterConfig(nil)
		if cfg.StatusCode != 429 {
			t.Errorf("expected 429, got %d", cfg.StatusCode)
		}
		if cfg.RetryAfter != 30*time.Second {
			t.Errorf("expected 30s, got %s", cfg.RetryAfter)
		}
	})

	t.Run("custom values", func(t *testing.T) {
		cfg := ParseRetryAfterConfig(map[string]interface{}{
			"statusCode": float64(503),
			"retryAfter": "60s",
			"body":       `{"error":"too many requests"}`,
		})
		if cfg.StatusCode != 503 {
			t.Errorf("expected 503, got %d", cfg.StatusCode)
		}
		if cfg.RetryAfter != 60*time.Second {
			t.Errorf("expected 60s, got %s", cfg.RetryAfter)
		}
		if cfg.Body != `{"error":"too many requests"}` {
			t.Errorf("unexpected body: %s", cfg.Body)
		}
	})

	t.Run("invalid status code defaults to 429", func(t *testing.T) {
		cfg := ParseRetryAfterConfig(map[string]interface{}{
			"statusCode": float64(200),
		})
		if cfg.StatusCode != 429 {
			t.Errorf("expected 429 for invalid code, got %d", cfg.StatusCode)
		}
	})

	t.Run("invalid duration defaults to 30s", func(t *testing.T) {
		cfg := ParseRetryAfterConfig(map[string]interface{}{
			"retryAfter": "invalid",
		})
		if cfg.RetryAfter != 30*time.Second {
			t.Errorf("expected 30s, got %s", cfg.RetryAfter)
		}
	})
}
