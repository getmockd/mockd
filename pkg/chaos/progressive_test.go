package chaos

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestProgressiveDegradation(t *testing.T) {
	t.Run("initial request has minimal delay", func(t *testing.T) {
		pd := NewProgressiveDegradation(ProgressiveDegradationConfig{
			InitialDelay:   1 * time.Millisecond,
			DelayIncrement: 1 * time.Millisecond,
			MaxDelay:       1 * time.Second,
		})

		start := time.Now()
		w := httptest.NewRecorder()
		errored := pd.Handle(context.Background(), w)
		elapsed := time.Since(start)

		if errored {
			t.Error("should not error on first request")
		}
		if elapsed > 50*time.Millisecond {
			t.Errorf("first request took too long: %s", elapsed)
		}
	})

	t.Run("delay increases with each request", func(t *testing.T) {
		pd := NewProgressiveDegradation(ProgressiveDegradationConfig{
			InitialDelay:   1 * time.Millisecond,
			DelayIncrement: 5 * time.Millisecond,
			MaxDelay:       1 * time.Second,
		})

		// Send 5 requests and measure total time
		start := time.Now()
		for i := 0; i < 5; i++ {
			w := httptest.NewRecorder()
			pd.Handle(context.Background(), w)
		}
		elapsed := time.Since(start)

		// Expected: 1 + 6 + 11 + 16 + 21 = 55ms (approximately)
		if elapsed < 40*time.Millisecond {
			t.Errorf("expected progressive delay, total only took %s", elapsed)
		}
	})

	t.Run("delay capped at maxDelay", func(t *testing.T) {
		pd := NewProgressiveDegradation(ProgressiveDegradationConfig{
			InitialDelay:   1 * time.Millisecond,
			DelayIncrement: 100 * time.Millisecond,
			MaxDelay:       10 * time.Millisecond, // Very low cap
		})

		// After a few requests, delay should be capped
		for i := 0; i < 3; i++ {
			w := httptest.NewRecorder()
			pd.Handle(context.Background(), w)
		}

		stats := pd.Stats()
		if stats.CurrentDelayMs > stats.MaxDelayMs {
			t.Errorf("delay %dms exceeds max %dms", stats.CurrentDelayMs, stats.MaxDelayMs)
		}
	})

	t.Run("errors after errorAfter threshold", func(t *testing.T) {
		pd := NewProgressiveDegradation(ProgressiveDegradationConfig{
			InitialDelay:   0,
			DelayIncrement: 0,
			MaxDelay:       1 * time.Second,
			ErrorAfter:     3,
			ErrorCode:      http.StatusInternalServerError,
		})

		// First 3 requests pass through
		for i := 0; i < 3; i++ {
			w := httptest.NewRecorder()
			errored := pd.Handle(context.Background(), w)
			if errored {
				t.Errorf("request %d should pass through (errorAfter=3)", i+1)
			}
		}

		// Request 4 should error
		w := httptest.NewRecorder()
		errored := pd.Handle(context.Background(), w)
		if !errored {
			t.Error("request 4 should return error (past errorAfter=3)")
		}
		if w.Code != 500 {
			t.Errorf("expected 500, got %d", w.Code)
		}
	})

	t.Run("resets after resetAfter requests", func(t *testing.T) {
		pd := NewProgressiveDegradation(ProgressiveDegradationConfig{
			InitialDelay:   1 * time.Millisecond,
			DelayIncrement: 1 * time.Millisecond,
			MaxDelay:       1 * time.Second,
			ResetAfter:     5,
		})

		// Send 5 requests
		for i := 0; i < 5; i++ {
			w := httptest.NewRecorder()
			pd.Handle(context.Background(), w)
		}

		stats := pd.Stats()
		if stats.RequestCount != 5 {
			t.Errorf("expected 5 requests, got %d", stats.RequestCount)
		}

		// Request 6 should reset counter (6 > 5, wraps to 1)
		w := httptest.NewRecorder()
		pd.Handle(context.Background(), w)

		stats = pd.Stats()
		if stats.RequestCount != 1 {
			t.Errorf("expected counter reset to 1, got %d", stats.RequestCount)
		}
		if stats.TotalResets != 1 {
			t.Errorf("expected 1 reset, got %d", stats.TotalResets)
		}
	})

	t.Run("context cancellation during delay", func(t *testing.T) {
		pd := NewProgressiveDegradation(ProgressiveDegradationConfig{
			InitialDelay:   5 * time.Second, // Very long
			DelayIncrement: 0,
			MaxDelay:       5 * time.Second,
		})

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
		defer cancel()

		start := time.Now()
		w := httptest.NewRecorder()
		errored := pd.Handle(ctx, w)
		elapsed := time.Since(start)

		if !errored {
			t.Error("should return true when context cancelled")
		}
		if elapsed > 100*time.Millisecond {
			t.Errorf("should have cancelled quickly, took %s", elapsed)
		}
	})

	t.Run("error + reset interaction", func(t *testing.T) {
		pd := NewProgressiveDegradation(ProgressiveDegradationConfig{
			InitialDelay:   0,
			DelayIncrement: 0,
			MaxDelay:       1 * time.Second,
			ErrorAfter:     3,
			ResetAfter:     5,
			ErrorCode:      http.StatusServiceUnavailable,
		})

		// Requests 1-3: pass
		for i := 0; i < 3; i++ {
			w := httptest.NewRecorder()
			errored := pd.Handle(context.Background(), w)
			if errored {
				t.Errorf("request %d should pass", i+1)
			}
		}

		// Requests 4-5: error
		for i := 4; i <= 5; i++ {
			w := httptest.NewRecorder()
			errored := pd.Handle(context.Background(), w)
			if !errored {
				t.Errorf("request %d should error", i)
			}
			if w.Code != 503 {
				t.Errorf("expected 503, got %d", w.Code)
			}
		}

		// Request 6: reset → back to request 1 → should pass
		w := httptest.NewRecorder()
		errored := pd.Handle(context.Background(), w)
		if errored {
			t.Error("request after reset should pass")
		}
	})

	t.Run("reset method clears state", func(t *testing.T) {
		pd := NewProgressiveDegradation(ProgressiveDegradationConfig{
			InitialDelay:   0,
			DelayIncrement: 0,
			MaxDelay:       1 * time.Second,
		})

		// Send some requests
		for i := 0; i < 10; i++ {
			w := httptest.NewRecorder()
			pd.Handle(context.Background(), w)
		}

		pd.Reset()
		stats := pd.Stats()
		if stats.RequestCount != 0 {
			t.Errorf("expected 0 after reset, got %d", stats.RequestCount)
		}
	})

	t.Run("stats accuracy", func(t *testing.T) {
		pd := NewProgressiveDegradation(ProgressiveDegradationConfig{
			InitialDelay:   0,
			DelayIncrement: 0,
			MaxDelay:       100 * time.Millisecond,
			ErrorAfter:     2,
			ErrorCode:      500,
		})

		// 2 pass + 1 error
		for i := 0; i < 3; i++ {
			w := httptest.NewRecorder()
			pd.Handle(context.Background(), w)
		}

		stats := pd.Stats()
		if stats.RequestCount != 3 {
			t.Errorf("expected 3 requests, got %d", stats.RequestCount)
		}
		if stats.TotalErrors != 1 {
			t.Errorf("expected 1 error, got %d", stats.TotalErrors)
		}
		if !stats.IsErroring {
			t.Error("expected isErroring=true")
		}
		if stats.ErrorAfter != 2 {
			t.Errorf("expected errorAfter=2, got %d", stats.ErrorAfter)
		}
	})
}

func TestParseProgressiveDegradationConfig(t *testing.T) {
	t.Run("defaults", func(t *testing.T) {
		cfg := ParseProgressiveDegradationConfig(nil)
		if cfg.InitialDelay != 20*time.Millisecond {
			t.Errorf("expected 20ms, got %s", cfg.InitialDelay)
		}
		if cfg.DelayIncrement != 5*time.Millisecond {
			t.Errorf("expected 5ms, got %s", cfg.DelayIncrement)
		}
		if cfg.MaxDelay != 5*time.Second {
			t.Errorf("expected 5s, got %s", cfg.MaxDelay)
		}
		if cfg.ErrorCode != 500 {
			t.Errorf("expected 500, got %d", cfg.ErrorCode)
		}
	})

	t.Run("custom values", func(t *testing.T) {
		cfg := ParseProgressiveDegradationConfig(map[string]interface{}{
			"initialDelay":   "100ms",
			"delayIncrement": "10ms",
			"maxDelay":       "10s",
			"resetAfter":     float64(500),
			"errorAfter":     float64(400),
			"errorCode":      float64(503),
		})
		if cfg.InitialDelay != 100*time.Millisecond {
			t.Errorf("expected 100ms, got %s", cfg.InitialDelay)
		}
		if cfg.DelayIncrement != 10*time.Millisecond {
			t.Errorf("expected 10ms, got %s", cfg.DelayIncrement)
		}
		if cfg.MaxDelay != 10*time.Second {
			t.Errorf("expected 10s, got %s", cfg.MaxDelay)
		}
		if cfg.ResetAfter != 500 {
			t.Errorf("expected 500, got %d", cfg.ResetAfter)
		}
		if cfg.ErrorAfter != 400 {
			t.Errorf("expected 400, got %d", cfg.ErrorAfter)
		}
		if cfg.ErrorCode != 503 {
			t.Errorf("expected 503, got %d", cfg.ErrorCode)
		}
	})
}
