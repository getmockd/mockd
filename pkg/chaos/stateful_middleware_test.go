package chaos

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// testHandler returns a simple 200 OK handler.
func testHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})
}

func TestMiddlewareCircuitBreaker(t *testing.T) {
	t.Run("circuit breaker trips and recovers through middleware", func(t *testing.T) {
		config := &ChaosConfig{
			Enabled: true,
			Rules: []ChaosRule{
				{
					PathPattern: "/api/.*",
					Probability: 1.0,
					Faults: []FaultConfig{
						{
							Type:        FaultCircuitBreaker,
							Probability: 1.0,
							Config: map[string]interface{}{
								"tripAfter":         float64(3),
								"openDuration":      "20ms",
								"openStatusCode":    float64(503),
								"successThreshold":  float64(1),
								"halfOpenErrorRate": 0.0, // All succeed in half-open
							},
						},
					},
				},
			},
		}

		injector, err := NewInjector(config)
		if err != nil {
			t.Fatalf("NewInjector: %v", err)
		}

		mw := NewMiddleware(testHandler(), injector)

		// Requests 1-2: pass through (circuit closed)
		for i := 0; i < 2; i++ {
			req := httptest.NewRequest("GET", "/api/users", nil)
			w := httptest.NewRecorder()
			mw.ServeHTTP(w, req)
			if w.Code != 200 {
				t.Errorf("request %d: expected 200, got %d", i+1, w.Code)
			}
		}

		// Request 3: trips the circuit
		req := httptest.NewRequest("GET", "/api/users", nil)
		w := httptest.NewRecorder()
		mw.ServeHTTP(w, req)
		if w.Code != 503 {
			t.Errorf("request 3: expected 503 (trip), got %d", w.Code)
		}

		// Request 4: still open
		req = httptest.NewRequest("GET", "/api/users", nil)
		w = httptest.NewRecorder()
		mw.ServeHTTP(w, req)
		if w.Code != 503 {
			t.Errorf("request 4: expected 503 (open), got %d", w.Code)
		}

		// Wait for recovery
		time.Sleep(25 * time.Millisecond)

		// Request 5: half-open → success → closes
		req = httptest.NewRequest("GET", "/api/users", nil)
		w = httptest.NewRecorder()
		mw.ServeHTTP(w, req)
		if w.Code != 200 {
			t.Errorf("request 5: expected 200 (half-open pass), got %d", w.Code)
		}
	})

	t.Run("non-matching path bypasses circuit breaker", func(t *testing.T) {
		config := &ChaosConfig{
			Enabled: true,
			Rules: []ChaosRule{
				{
					PathPattern: "/api/payments/.*",
					Probability: 1.0,
					Faults: []FaultConfig{
						{
							Type:        FaultCircuitBreaker,
							Probability: 1.0,
							Config: map[string]interface{}{
								"tripAfter":      float64(1),
								"openDuration":   "1h",
								"openStatusCode": float64(503),
							},
						},
					},
				},
			},
		}

		injector, err := NewInjector(config)
		if err != nil {
			t.Fatalf("NewInjector: %v", err)
		}

		mw := NewMiddleware(testHandler(), injector)

		// Trip the circuit on /api/payments
		req := httptest.NewRequest("GET", "/api/payments/123", nil)
		w := httptest.NewRecorder()
		mw.ServeHTTP(w, req)

		// /api/users should still work
		req = httptest.NewRequest("GET", "/api/users", nil)
		w = httptest.NewRecorder()
		mw.ServeHTTP(w, req)
		if w.Code != 200 {
			t.Errorf("non-matching path should pass: got %d", w.Code)
		}
	})
}

func TestMiddlewareRetryAfter(t *testing.T) {
	t.Run("retry-after rate limits through middleware", func(t *testing.T) {
		config := &ChaosConfig{
			Enabled: true,
			Rules: []ChaosRule{
				{
					PathPattern: "/api/.*",
					Probability: 1.0,
					Faults: []FaultConfig{
						{
							Type:        FaultRetryAfter,
							Probability: 1.0,
							Config: map[string]interface{}{
								"statusCode": float64(429),
								"retryAfter": "20ms",
							},
						},
					},
				},
			},
		}

		injector, err := NewInjector(config)
		if err != nil {
			t.Fatalf("NewInjector: %v", err)
		}

		mw := NewMiddleware(testHandler(), injector)

		// First request: rate limited
		req := httptest.NewRequest("GET", "/api/users", nil)
		w := httptest.NewRecorder()
		mw.ServeHTTP(w, req)
		if w.Code != 429 {
			t.Errorf("expected 429, got %d", w.Code)
		}
		if w.Header().Get("Retry-After") == "" {
			t.Error("expected Retry-After header")
		}

		// Wait for recovery
		time.Sleep(25 * time.Millisecond)

		// Should pass through
		req = httptest.NewRequest("GET", "/api/users", nil)
		w = httptest.NewRecorder()
		mw.ServeHTTP(w, req)
		if w.Code != 200 {
			t.Errorf("expected 200 after recovery, got %d", w.Code)
		}
	})
}

func TestMiddlewareProgressiveDegradation(t *testing.T) {
	t.Run("progressive degradation with errors through middleware", func(t *testing.T) {
		config := &ChaosConfig{
			Enabled: true,
			Rules: []ChaosRule{
				{
					PathPattern: "/api/.*",
					Probability: 1.0,
					Faults: []FaultConfig{
						{
							Type:        FaultProgressiveDegradation,
							Probability: 1.0,
							Config: map[string]interface{}{
								"initialDelay":   "0ms",
								"delayIncrement": "0ms",
								"maxDelay":       "1s",
								"errorAfter":     float64(3),
								"errorCode":      float64(500),
							},
						},
					},
				},
			},
		}

		injector, err := NewInjector(config)
		if err != nil {
			t.Fatalf("NewInjector: %v", err)
		}

		mw := NewMiddleware(testHandler(), injector)

		// Requests 1-3: pass through
		for i := 0; i < 3; i++ {
			req := httptest.NewRequest("GET", "/api/search", nil)
			w := httptest.NewRecorder()
			mw.ServeHTTP(w, req)
			if w.Code != 200 {
				t.Errorf("request %d: expected 200, got %d", i+1, w.Code)
			}
		}

		// Request 4: error
		req := httptest.NewRequest("GET", "/api/search", nil)
		w := httptest.NewRecorder()
		mw.ServeHTTP(w, req)
		if w.Code != 500 {
			t.Errorf("request 4: expected 500, got %d", w.Code)
		}
	})
}

func TestMiddlewareChunkedDribble(t *testing.T) {
	t.Run("chunked dribble wraps response", func(t *testing.T) {
		config := &ChaosConfig{
			Enabled: true,
			Rules: []ChaosRule{
				{
					PathPattern: "/api/.*",
					Probability: 1.0,
					Faults: []FaultConfig{
						{
							Type:        FaultChunkedDribble,
							Probability: 1.0,
							Config: map[string]interface{}{
								"chunkSize":    float64(5),
								"chunkDelay":   "1ms",
								"initialDelay": "1ms",
							},
						},
					},
				},
			},
		}

		injector, err := NewInjector(config)
		if err != nil {
			t.Fatalf("NewInjector: %v", err)
		}

		mw := NewMiddleware(testHandler(), injector)

		req := httptest.NewRequest("GET", "/api/export", nil)
		w := httptest.NewRecorder()
		mw.ServeHTTP(w, req)

		if w.Code != 200 {
			t.Errorf("expected 200, got %d", w.Code)
		}
		if w.Body.String() != `{"status":"ok"}` {
			t.Errorf("unexpected body: %s", w.Body.String())
		}
	})
}

func TestMiddlewareMixedFaults(t *testing.T) {
	t.Run("stateful + regular faults in same rule", func(t *testing.T) {
		config := &ChaosConfig{
			Enabled: true,
			Rules: []ChaosRule{
				{
					PathPattern: "/api/.*",
					Probability: 1.0,
					Faults: []FaultConfig{
						{
							Type:        FaultProgressiveDegradation,
							Probability: 1.0,
							Config: map[string]interface{}{
								"initialDelay":   "0ms",
								"delayIncrement": "0ms",
								"maxDelay":       "1s",
								"errorAfter":     float64(100), // Never error in this test
							},
						},
						{
							Type:        FaultChunkedDribble,
							Probability: 1.0,
							Config: map[string]interface{}{
								"chunkSize":  float64(5),
								"chunkDelay": "0ms",
							},
						},
					},
				},
			},
		}

		injector, err := NewInjector(config)
		if err != nil {
			t.Fatalf("NewInjector: %v", err)
		}

		mw := NewMiddleware(testHandler(), injector)

		req := httptest.NewRequest("GET", "/api/data", nil)
		w := httptest.NewRecorder()
		mw.ServeHTTP(w, req)

		if w.Code != 200 {
			t.Errorf("expected 200, got %d", w.Code)
		}
		if w.Body.String() != `{"status":"ok"}` {
			t.Errorf("unexpected body: %s", w.Body.String())
		}
	})
}

func TestStatefulFaultInjectorIntegration(t *testing.T) {
	t.Run("UpdateConfig rebuilds state machines", func(t *testing.T) {
		config := &ChaosConfig{
			Enabled: true,
			Rules: []ChaosRule{
				{
					PathPattern: "/api/.*",
					Probability: 1.0,
					Faults: []FaultConfig{
						{
							Type:        FaultCircuitBreaker,
							Probability: 1.0,
							Config: map[string]interface{}{
								"tripAfter":    float64(2),
								"openDuration": "1h",
							},
						},
					},
				},
			},
		}

		injector, err := NewInjector(config)
		if err != nil {
			t.Fatalf("NewInjector: %v", err)
		}

		if len(injector.GetCircuitBreakers()) != 1 {
			t.Errorf("expected 1 circuit breaker, got %d", len(injector.GetCircuitBreakers()))
		}

		// Update config with different rules
		newConfig := &ChaosConfig{
			Enabled: true,
			Rules: []ChaosRule{
				{
					PathPattern: "/api/a/.*",
					Probability: 1.0,
					Faults: []FaultConfig{
						{Type: FaultCircuitBreaker, Probability: 1.0, Config: map[string]interface{}{}},
					},
				},
				{
					PathPattern: "/api/b/.*",
					Probability: 1.0,
					Faults: []FaultConfig{
						{Type: FaultRetryAfter, Probability: 1.0, Config: map[string]interface{}{}},
					},
				},
			},
		}

		if err := injector.UpdateConfig(newConfig); err != nil {
			t.Fatalf("UpdateConfig: %v", err)
		}

		if len(injector.GetCircuitBreakers()) != 1 {
			t.Errorf("expected 1 circuit breaker, got %d", len(injector.GetCircuitBreakers()))
		}
		if len(injector.GetRetryTrackers()) != 1 {
			t.Errorf("expected 1 retry tracker, got %d", len(injector.GetRetryTrackers()))
		}
	})

	t.Run("handler methods with unknown keys return false", func(t *testing.T) {
		config := &ChaosConfig{Enabled: true}
		injector, err := NewInjector(config)
		if err != nil {
			t.Fatalf("NewInjector: %v", err)
		}

		w := httptest.NewRecorder()
		if injector.HandleCircuitBreaker("nonexistent", w) {
			t.Error("should return false for unknown key")
		}
		if injector.HandleRetryAfter("nonexistent", w) {
			t.Error("should return false for unknown key")
		}
	})
}
