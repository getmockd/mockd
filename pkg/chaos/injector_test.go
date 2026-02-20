package chaos

import (
	"context"
	"io"
	"math"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestNewInjector(t *testing.T) {
	tests := []struct {
		name    string
		config  *ChaosConfig
		wantErr bool
	}{
		{
			name:    "nil config returns error",
			config:  nil,
			wantErr: true,
		},
		{
			name: "valid config with no rules",
			config: &ChaosConfig{
				Enabled: true,
			},
			wantErr: false,
		},
		{
			name: "valid config with rules",
			config: &ChaosConfig{
				Enabled: true,
				Rules: []ChaosRule{
					{
						PathPattern: "/api/.*",
						Faults: []FaultConfig{
							{Type: FaultLatency, Probability: 0.5},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "invalid regex pattern",
			config: &ChaosConfig{
				Enabled: true,
				Rules: []ChaosRule{
					{
						PathPattern: "[invalid",
						Faults:      []FaultConfig{},
					},
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewInjector(tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewInjector() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestInjector_IsEnabled(t *testing.T) {
	tests := []struct {
		name    string
		config  *ChaosConfig
		enabled bool
	}{
		{
			name: "enabled config",
			config: &ChaosConfig{
				Enabled: true,
			},
			enabled: true,
		},
		{
			name: "disabled config",
			config: &ChaosConfig{
				Enabled: false,
			},
			enabled: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			inj, err := NewInjector(tt.config)
			if err != nil {
				t.Fatalf("NewInjector() error = %v", err)
			}
			if got := inj.IsEnabled(); got != tt.enabled {
				t.Errorf("IsEnabled() = %v, want %v", got, tt.enabled)
			}
		})
	}
}

func TestInjector_ShouldInject(t *testing.T) {
	config := &ChaosConfig{
		Enabled: true,
		Rules: []ChaosRule{
			{
				PathPattern: "/api/users.*",
				Methods:     []string{"GET", "POST"},
				Faults: []FaultConfig{
					{Type: FaultLatency, Probability: 1.0}, // Always inject for testing
				},
				Probability: 1.0,
			},
			{
				PathPattern: "/api/admin.*",
				Methods:     []string{"DELETE"},
				Faults: []FaultConfig{
					{Type: FaultError, Probability: 1.0},
				},
				Probability: 1.0,
			},
		},
	}

	inj, err := NewInjector(config)
	if err != nil {
		t.Fatalf("NewInjector() error = %v", err)
	}

	tests := []struct {
		name       string
		method     string
		path       string
		wantFaults int
	}{
		{
			name:       "matching path and method",
			method:     "GET",
			path:       "/api/users/123",
			wantFaults: 1,
		},
		{
			name:       "matching path wrong method",
			method:     "DELETE",
			path:       "/api/users/123",
			wantFaults: 0,
		},
		{
			name:       "non-matching path",
			method:     "GET",
			path:       "/health",
			wantFaults: 0,
		},
		{
			name:       "admin DELETE",
			method:     "DELETE",
			path:       "/api/admin/user/1",
			wantFaults: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, nil)
			faults := inj.ShouldInject(req)
			if len(faults) != tt.wantFaults {
				t.Errorf("ShouldInject() returned %d faults, want %d", len(faults), tt.wantFaults)
			}
		})
	}
}

func TestInjector_ShouldInject_Disabled(t *testing.T) {
	config := &ChaosConfig{
		Enabled: false,
		Rules: []ChaosRule{
			{
				PathPattern: ".*",
				Faults: []FaultConfig{
					{Type: FaultLatency, Probability: 1.0},
				},
				Probability: 1.0,
			},
		},
	}

	inj, err := NewInjector(config)
	if err != nil {
		t.Fatalf("NewInjector() error = %v", err)
	}

	req := httptest.NewRequest("GET", "/any/path", nil)
	faults := inj.ShouldInject(req)
	if len(faults) != 0 {
		t.Errorf("ShouldInject() with disabled config returned %d faults, want 0", len(faults))
	}
}

func TestInjector_InjectLatency(t *testing.T) {
	config := &ChaosConfig{Enabled: true}
	inj, err := NewInjector(config)
	if err != nil {
		t.Fatalf("NewInjector() error = %v", err)
	}

	tests := []struct {
		name     string
		fault    *LatencyFault
		minDelay time.Duration
		maxDelay time.Duration
		wantErr  bool
	}{
		{
			name:     "nil fault",
			fault:    nil,
			minDelay: 0,
			maxDelay: time.Millisecond,
			wantErr:  false,
		},
		{
			name: "valid latency range",
			fault: &LatencyFault{
				Min: "10ms",
				Max: "50ms",
			},
			minDelay: 10 * time.Millisecond,
			maxDelay: 60 * time.Millisecond, // Allow some buffer
			wantErr:  false,
		},
		{
			name: "invalid min duration",
			fault: &LatencyFault{
				Min: "invalid",
				Max: "50ms",
			},
			wantErr: true,
		},
		{
			name: "invalid max duration",
			fault: &LatencyFault{
				Min: "10ms",
				Max: "invalid",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			start := time.Now()
			err := inj.InjectLatency(ctx, tt.fault)
			elapsed := time.Since(start)

			if (err != nil) != tt.wantErr {
				t.Errorf("InjectLatency() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && tt.fault != nil {
				if elapsed < tt.minDelay {
					t.Errorf("InjectLatency() elapsed %v < min %v", elapsed, tt.minDelay)
				}
				if elapsed > tt.maxDelay {
					t.Errorf("InjectLatency() elapsed %v > max %v", elapsed, tt.maxDelay)
				}
			}
		})
	}
}

func TestInjector_InjectLatency_ContextCancellation(t *testing.T) {
	config := &ChaosConfig{Enabled: true}
	inj, err := NewInjector(config)
	if err != nil {
		t.Fatalf("NewInjector() error = %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	fault := &LatencyFault{
		Min: "1s",
		Max: "2s",
	}

	start := time.Now()
	err = inj.InjectLatency(ctx, fault)
	elapsed := time.Since(start)

	if err != context.Canceled {
		t.Errorf("InjectLatency() error = %v, want context.Canceled", err)
	}

	if elapsed > 100*time.Millisecond {
		t.Errorf("InjectLatency() with cancelled context took %v, should be near instant", elapsed)
	}
}

func TestInjector_InjectError(t *testing.T) {
	config := &ChaosConfig{Enabled: true}
	inj, err := NewInjector(config)
	if err != nil {
		t.Fatalf("NewInjector() error = %v", err)
	}

	tests := []struct {
		name         string
		fault        *ErrorRateFault
		expectStatus int
	}{
		{
			name:         "nil fault defaults to 500",
			fault:        nil,
			expectStatus: http.StatusInternalServerError,
		},
		{
			name: "custom default code",
			fault: &ErrorRateFault{
				DefaultCode: http.StatusServiceUnavailable,
			},
			expectStatus: http.StatusServiceUnavailable,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			inj.InjectError(w, tt.fault)

			if w.Code != tt.expectStatus {
				t.Errorf("InjectError() status = %d, want %d", w.Code, tt.expectStatus)
			}
		})
	}
}

func TestInjector_InjectError_RandomStatusCode(t *testing.T) {
	config := &ChaosConfig{Enabled: true}
	inj, err := NewInjector(config)
	if err != nil {
		t.Fatalf("NewInjector() error = %v", err)
	}

	fault := &ErrorRateFault{
		StatusCodes: []int{500, 502, 503},
	}

	statusCodes := make(map[int]bool)
	for i := 0; i < 100; i++ {
		w := httptest.NewRecorder()
		inj.InjectError(w, fault)
		statusCodes[w.Code] = true
	}

	// Should have gotten at least 2 different status codes in 100 tries
	if len(statusCodes) < 2 {
		t.Errorf("InjectError() only returned %d unique status codes, expected random selection", len(statusCodes))
	}

	// All codes should be from the configured list
	for code := range statusCodes {
		found := false
		for _, expected := range fault.StatusCodes {
			if code == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("InjectError() returned unexpected status code %d", code)
		}
	}
}

func TestInjector_GlobalRules(t *testing.T) {
	config := &ChaosConfig{
		Enabled: true,
		GlobalRules: &GlobalChaosRules{
			Latency: &LatencyFault{
				Min:         "1ms",
				Max:         "5ms",
				Probability: 1.0, // Always apply for testing
			},
		},
	}

	inj, err := NewInjector(config)
	if err != nil {
		t.Fatalf("NewInjector() error = %v", err)
	}

	req := httptest.NewRequest("GET", "/any/path", nil)
	faults := inj.ShouldInject(req)

	if len(faults) == 0 {
		t.Error("ShouldInject() with global rules returned no faults")
	}

	found := false
	for _, f := range faults {
		if f.Type == FaultLatency {
			found = true
			break
		}
	}
	if !found {
		t.Error("ShouldInject() did not include latency fault from global rules")
	}
}

func TestInjector_Stats(t *testing.T) {
	config := &ChaosConfig{
		Enabled: true,
		Rules: []ChaosRule{
			{
				PathPattern: ".*",
				Faults: []FaultConfig{
					{Type: FaultLatency, Probability: 1.0},
				},
				Probability: 1.0,
			},
		},
	}

	inj, err := NewInjector(config)
	if err != nil {
		t.Fatalf("NewInjector() error = %v", err)
	}

	// Make some requests
	for i := 0; i < 10; i++ {
		req := httptest.NewRequest("GET", "/test", nil)
		inj.ShouldInject(req)
	}

	stats := inj.GetStats()
	if stats.TotalRequests != 10 {
		t.Errorf("Stats.TotalRequests = %d, want 10", stats.TotalRequests)
	}
	if stats.InjectedFaults != 10 {
		t.Errorf("Stats.InjectedFaults = %d, want 10", stats.InjectedFaults)
	}
	if stats.FaultsByType[FaultLatency] != 10 {
		t.Errorf("Stats.FaultsByType[FaultLatency] = %d, want 10", stats.FaultsByType[FaultLatency])
	}

	// Test reset
	inj.ResetStats()
	stats = inj.GetStats()
	if stats.TotalRequests != 0 {
		t.Errorf("After reset, Stats.TotalRequests = %d, want 0", stats.TotalRequests)
	}
}

func TestInjector_UpdateConfig(t *testing.T) {
	config := &ChaosConfig{
		Enabled: true,
		Rules: []ChaosRule{
			{
				PathPattern: "/old.*",
				Faults: []FaultConfig{
					{Type: FaultLatency, Probability: 1.0},
				},
				Probability: 1.0,
			},
		},
	}

	inj, err := NewInjector(config)
	if err != nil {
		t.Fatalf("NewInjector() error = %v", err)
	}

	// Check old rule works
	req := httptest.NewRequest("GET", "/old/path", nil)
	faults := inj.ShouldInject(req)
	if len(faults) == 0 {
		t.Error("Old rule should match")
	}

	// Update config
	newConfig := &ChaosConfig{
		Enabled: true,
		Rules: []ChaosRule{
			{
				PathPattern: "/new.*",
				Faults: []FaultConfig{
					{Type: FaultError, Probability: 1.0},
				},
				Probability: 1.0,
			},
		},
	}

	err = inj.UpdateConfig(newConfig)
	if err != nil {
		t.Fatalf("UpdateConfig() error = %v", err)
	}

	// Old rule should not match
	req = httptest.NewRequest("GET", "/old/path", nil)
	faults = inj.ShouldInject(req)
	if len(faults) != 0 {
		t.Error("Old rule should not match after config update")
	}

	// New rule should match
	req = httptest.NewRequest("GET", "/new/path", nil)
	faults = inj.ShouldInject(req)
	if len(faults) == 0 {
		t.Error("New rule should match after config update")
	}
}

func TestMiddleware_ServeHTTP(t *testing.T) {
	// Create a simple handler
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	})

	t.Run("disabled chaos passes through", func(t *testing.T) {
		config := &ChaosConfig{Enabled: false}
		inj, _ := NewInjector(config)
		middleware := NewMiddleware(handler, inj)

		req := httptest.NewRequest("GET", "/test", nil)
		w := httptest.NewRecorder()
		middleware.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("ServeHTTP() status = %d, want %d", w.Code, http.StatusOK)
		}
	})

	t.Run("error fault returns error", func(t *testing.T) {
		config := &ChaosConfig{
			Enabled: true,
			Rules: []ChaosRule{
				{
					PathPattern: ".*",
					Faults: []FaultConfig{
						{
							Type:        FaultError,
							Probability: 1.0,
							Config: map[string]interface{}{
								"defaultCode": 503,
							},
						},
					},
					Probability: 1.0,
				},
			},
		}
		inj, _ := NewInjector(config)
		middleware := NewMiddleware(handler, inj)

		req := httptest.NewRequest("GET", "/test", nil)
		w := httptest.NewRecorder()
		middleware.ServeHTTP(w, req)

		if w.Code != http.StatusServiceUnavailable {
			t.Errorf("ServeHTTP() status = %d, want %d", w.Code, http.StatusServiceUnavailable)
		}
	})

	t.Run("empty response fault returns empty body", func(t *testing.T) {
		config := &ChaosConfig{
			Enabled: true,
			Rules: []ChaosRule{
				{
					PathPattern: ".*",
					Faults: []FaultConfig{
						{Type: FaultEmptyResponse, Probability: 1.0},
					},
					Probability: 1.0,
				},
			},
		}
		inj, _ := NewInjector(config)
		middleware := NewMiddleware(handler, inj)

		req := httptest.NewRequest("GET", "/test", nil)
		w := httptest.NewRecorder()
		middleware.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("ServeHTTP() status = %d, want %d", w.Code, http.StatusOK)
		}
		if w.Body.Len() != 0 {
			t.Errorf("ServeHTTP() body length = %d, want 0", w.Body.Len())
		}
	})
}

func TestSlowWriter(t *testing.T) {
	w := httptest.NewRecorder()
	sw := &SlowWriter{
		w:              w,
		bytesPerSecond: 100, // Very slow for testing
	}

	data := []byte("Hello, World!")
	start := time.Now()
	n, err := sw.Write(data)
	elapsed := time.Since(start)

	if err != nil {
		t.Errorf("Write() error = %v", err)
	}
	if n != len(data) {
		t.Errorf("Write() wrote %d bytes, want %d", n, len(data))
	}

	// Should have taken some time due to bandwidth limiting
	// At 100 bytes/sec, 13 bytes should take ~130ms
	if elapsed < 100*time.Millisecond {
		t.Logf("Write() took %v (expected ~130ms at 100 bytes/sec)", elapsed)
	}
}

func TestCorruptingWriter(t *testing.T) {
	w := httptest.NewRecorder()
	config := &ChaosConfig{Enabled: true}
	inj, _ := NewInjector(config)

	cw := inj.WrapForCorruption(w, 1.0) // 100% corruption rate

	data := []byte("Hello, World!")
	n, err := cw.Write(data)

	if err != nil {
		t.Errorf("Write() error = %v", err)
	}
	if n != len(data) {
		t.Errorf("Write() wrote %d bytes, want %d", n, len(data))
	}

	// Data should be corrupted (different from original)
	if w.Body.String() == string(data) {
		t.Error("Write() with 100% corruption rate should corrupt data")
	}
}

func TestTruncatingWriter(t *testing.T) {
	w := httptest.NewRecorder()
	tw := &TruncatingWriter{
		w:        w,
		maxBytes: 5,
	}

	data := []byte("Hello, World!")
	n, err := tw.Write(data)

	if err != nil {
		t.Errorf("Write() error = %v", err)
	}
	// TruncatingWriter returns actual bytes written (truncated to maxBytes)
	if n != 5 {
		t.Errorf("Write() reported %d bytes, want %d (truncated)", n, 5)
	}

	// Only first 5 bytes should be written
	if w.Body.String() != "Hello" {
		t.Errorf("Write() output = %q, want %q", w.Body.String(), "Hello")
	}

	if tw.BytesWritten() != 5 {
		t.Errorf("BytesWritten() = %d, want 5", tw.BytesWritten())
	}
}

func TestDelayedWriter(t *testing.T) {
	w := httptest.NewRecorder()
	dw := NewDelayedWriter(w, 50*time.Millisecond)

	start := time.Now()
	_, _ = dw.Write([]byte("first"))
	firstWriteTime := time.Since(start)

	start = time.Now()
	_, _ = dw.Write([]byte("second"))
	secondWriteTime := time.Since(start)

	// First write should be delayed
	if firstWriteTime < 40*time.Millisecond {
		t.Errorf("First write took %v, expected >= 50ms", firstWriteTime)
	}

	// Second write should not be delayed
	if secondWriteTime > 10*time.Millisecond {
		t.Errorf("Second write took %v, expected to be fast", secondWriteTime)
	}
}

func TestChaosContext(t *testing.T) {
	ctx := context.Background()

	// Initially no context
	if cc := GetChaosContext(ctx); cc != nil {
		t.Error("GetChaosContext() on empty context should return nil")
	}

	// Add context
	chaosCtx := &ChaosContext{
		Faults:   []FaultConfig{{Type: FaultLatency}},
		Injected: true,
	}
	ctx = WithChaosContext(ctx, chaosCtx)

	// Should retrieve it
	if cc := GetChaosContext(ctx); cc == nil {
		t.Error("GetChaosContext() should return the chaos context")
	} else if !cc.Injected {
		t.Error("GetChaosContext() returned wrong context")
	}
}

func TestMiddleware_TimeoutFault(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	config := &ChaosConfig{
		Enabled: true,
		Rules: []ChaosRule{
			{
				PathPattern: ".*",
				Faults: []FaultConfig{
					{
						Type:        FaultTimeout,
						Probability: 1.0,
						Config: map[string]interface{}{
							"duration": "50ms",
						},
					},
				},
				Probability: 1.0,
			},
		},
	}
	inj, _ := NewInjector(config)
	middleware := NewMiddleware(handler, inj)

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	start := time.Now()
	middleware.ServeHTTP(w, req)
	elapsed := time.Since(start)

	if w.Code != http.StatusGatewayTimeout {
		t.Errorf("Timeout fault status = %d, want %d", w.Code, http.StatusGatewayTimeout)
	}

	// Should have waited for the timeout duration
	if elapsed < 40*time.Millisecond {
		t.Errorf("Timeout fault elapsed %v, expected >= 50ms", elapsed)
	}
}

func TestMiddleware_SlowBodyFault(t *testing.T) {
	responseData := strings.Repeat("X", 100) // 100 bytes
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(responseData))
	})

	config := &ChaosConfig{
		Enabled: true,
		Rules: []ChaosRule{
			{
				PathPattern: ".*",
				Faults: []FaultConfig{
					{
						Type:        FaultSlowBody,
						Probability: 1.0,
						Config: map[string]interface{}{
							"bytesPerSecond": 1000, // 1KB/s
						},
					},
				},
				Probability: 1.0,
			},
		},
	}
	inj, _ := NewInjector(config)
	middleware := NewMiddleware(handler, inj)

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	start := time.Now()
	middleware.ServeHTTP(w, req)
	elapsed := time.Since(start)

	if w.Code != http.StatusOK {
		t.Errorf("SlowBody fault status = %d, want %d", w.Code, http.StatusOK)
	}

	body, _ := io.ReadAll(w.Body)
	if len(body) != 100 {
		t.Errorf("SlowBody fault body length = %d, want 100", len(body))
	}

	// At 1000 bytes/sec, 100 bytes should take ~100ms
	t.Logf("SlowBody fault elapsed: %v", elapsed)
}

// =============================================================================
// Session 12: Probability distribution tests
// =============================================================================

func TestInjector_ShouldInject_Probability50(t *testing.T) {
	config := &ChaosConfig{
		Enabled: true,
		Rules: []ChaosRule{
			{
				PathPattern: "/api/.*",
				Probability: 1.0, // Rule always matches
				Faults: []FaultConfig{
					{
						Type:        FaultError,
						Probability: 0.5, // Fault fires ~50% of the time
						Config: map[string]interface{}{
							"statusCodes": []int{500},
						},
					},
				},
			},
		},
	}

	injector, err := NewInjector(config)
	if err != nil {
		t.Fatalf("NewInjector failed: %v", err)
	}

	const n = 1000
	injected := 0

	for i := 0; i < n; i++ {
		req := httptest.NewRequest("GET", "/api/data", nil)
		faults := injector.ShouldInject(req)
		if len(faults) > 0 {
			injected++
		}
	}

	rate := float64(injected) / float64(n)
	t.Logf("Probability 0.5: %d/%d injected (%.2f%%)", injected, n, rate*100)

	// With 1000 samples and probability 0.5, expect rate within [0.40, 0.60]
	// (roughly ±3 standard deviations for binomial distribution)
	if math.Abs(rate-0.5) > 0.10 {
		t.Errorf("Expected ~50%% injection rate, got %.1f%% (%d/%d)", rate*100, injected, n)
	}
}

func TestInjector_ShouldInject_ProbabilityZero(t *testing.T) {
	config := &ChaosConfig{
		Enabled: true,
		Rules: []ChaosRule{
			{
				PathPattern: "/api/.*",
				Probability: 1.0,
				Faults: []FaultConfig{
					{
						Type:        FaultError,
						Probability: 0.0, // Never inject
						Config: map[string]interface{}{
							"statusCodes": []int{500},
						},
					},
				},
			},
		},
	}

	injector, err := NewInjector(config)
	if err != nil {
		t.Fatalf("NewInjector failed: %v", err)
	}

	for i := 0; i < 100; i++ {
		req := httptest.NewRequest("GET", "/api/data", nil)
		faults := injector.ShouldInject(req)
		if len(faults) > 0 {
			t.Fatal("Probability 0.0 should never inject faults")
		}
	}
}

func TestInjector_ShouldInject_RuleProbability(t *testing.T) {
	// Test that the RULE-level probability is also checked
	config := &ChaosConfig{
		Enabled: true,
		Rules: []ChaosRule{
			{
				PathPattern: "/api/.*",
				Probability: 0.5, // Rule matches ~50% of requests
				Faults: []FaultConfig{
					{
						Type:        FaultError,
						Probability: 1.0, // Fault always fires when rule matches
						Config: map[string]interface{}{
							"statusCodes": []int{500},
						},
					},
				},
			},
		},
	}

	injector, err := NewInjector(config)
	if err != nil {
		t.Fatalf("NewInjector failed: %v", err)
	}

	const n = 1000
	injected := 0

	for i := 0; i < n; i++ {
		req := httptest.NewRequest("GET", "/api/data", nil)
		faults := injector.ShouldInject(req)
		if len(faults) > 0 {
			injected++
		}
	}

	rate := float64(injected) / float64(n)
	t.Logf("Rule probability 0.5: %d/%d injected (%.2f%%)", injected, n, rate*100)

	if math.Abs(rate-0.5) > 0.10 {
		t.Errorf("Expected ~50%% injection rate, got %.1f%% (%d/%d)", rate*100, injected, n)
	}
}

// =============================================================================
// Session 12: Connection reset fault test
// =============================================================================

func TestMiddleware_ConnectionReset(t *testing.T) {
	config := &ChaosConfig{
		Enabled: true,
		Rules: []ChaosRule{
			{
				PathPattern: "/.*",
				Probability: 1.0,
				Faults: []FaultConfig{
					{
						Type:        FaultConnectionReset,
						Probability: 1.0,
					},
				},
			},
		},
	}

	injector, err := NewInjector(config)
	if err != nil {
		t.Fatalf("NewInjector failed: %v", err)
	}

	backend := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("should not reach client"))
	})

	middleware := NewMiddleware(backend, injector)
	server := httptest.NewServer(middleware)
	defer server.Close()

	// Make a request - should get a connection error
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(server.URL + "/test")
	if resp != nil {
		resp.Body.Close()
	}
	if err == nil {
		t.Fatal("Expected connection error for connection reset fault")
	}

	// The error should be a connection-level error (EOF, reset, etc.)
	errStr := err.Error()
	isConnectionError := strings.Contains(errStr, "EOF") ||
		strings.Contains(errStr, "connection reset") ||
		strings.Contains(errStr, "broken pipe") ||
		strings.Contains(errStr, "forcibly closed") ||
		strings.Contains(errStr, "unexpected EOF")

	if !isConnectionError {
		// On some platforms, the error may be wrapped differently
		if _, ok := err.(*net.OpError); ok {
			isConnectionError = true
		}
	}

	if !isConnectionError {
		t.Errorf("Expected connection-level error, got: %v", err)
	}
}

func TestMiddleware_ConnectionReset_NonHijackable(t *testing.T) {
	// When the ResponseWriter doesn't support Hijack (e.g., test recorder),
	// the connection reset fault should be a no-op (request passes through).
	config := &ChaosConfig{
		Enabled: true,
		Rules: []ChaosRule{
			{
				PathPattern: "/.*",
				Probability: 1.0,
				Faults: []FaultConfig{
					{
						Type:        FaultConnectionReset,
						Probability: 1.0,
					},
				},
			},
		},
	}

	injector, err := NewInjector(config)
	if err != nil {
		t.Fatalf("NewInjector failed: %v", err)
	}

	backend := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("passthrough"))
	})

	middleware := NewMiddleware(backend, injector)

	// Using httptest.NewRecorder which does NOT implement http.Hijacker
	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/test", nil)

	middleware.ServeHTTP(w, req)

	// With a non-hijackable writer, the Hijack() call fails, so the
	// middleware returns early without calling the backend handler.
	// The response body should be empty (no-op connection reset).
	body := w.Body.String()
	if body != "" {
		t.Errorf("Expected empty body (connection reset no-op), got %q", body)
	}
}

// =============================================================================
// Probability clamping tests
// =============================================================================

func TestChaosConfig_Clamp(t *testing.T) {
	cfg := &ChaosConfig{
		Enabled: true,
		Rules: []ChaosRule{
			{
				PathPattern: "/api/.*",
				Probability: 1.5, // Over 1.0
				Faults: []FaultConfig{
					{Type: FaultError, Probability: 2.0},    // Over 1.0
					{Type: FaultLatency, Probability: -0.5}, // Below 0.0
				},
			},
			{
				PathPattern: "/health",
				Probability: -1.0, // Below 0.0
				Faults: []FaultConfig{
					{Type: FaultTimeout, Probability: 0.5}, // Valid, should be unchanged
				},
			},
		},
		GlobalRules: &GlobalChaosRules{
			Latency: &LatencyFault{
				Min:         "10ms",
				Max:         "100ms",
				Probability: 3.0, // Over 1.0
			},
			ErrorRate: &ErrorRateFault{
				Probability: -0.1, // Below 0.0
				StatusCodes: []int{500},
			},
			Bandwidth: &BandwidthFault{
				BytesPerSecond: 1024,
				Probability:    1.1, // Over 1.0
			},
		},
	}

	cfg.Clamp()

	// Rule 0
	if cfg.Rules[0].Probability != 1.0 {
		t.Errorf("Rules[0].Probability = %v, want 1.0", cfg.Rules[0].Probability)
	}
	if cfg.Rules[0].Faults[0].Probability != 1.0 {
		t.Errorf("Rules[0].Faults[0].Probability = %v, want 1.0", cfg.Rules[0].Faults[0].Probability)
	}
	if cfg.Rules[0].Faults[1].Probability != 0.0 {
		t.Errorf("Rules[0].Faults[1].Probability = %v, want 0.0", cfg.Rules[0].Faults[1].Probability)
	}

	// Rule 1
	if cfg.Rules[1].Probability != 0.0 {
		t.Errorf("Rules[1].Probability = %v, want 0.0", cfg.Rules[1].Probability)
	}
	if cfg.Rules[1].Faults[0].Probability != 0.5 {
		t.Errorf("Rules[1].Faults[0].Probability = %v, want 0.5", cfg.Rules[1].Faults[0].Probability)
	}

	// Global rules
	if cfg.GlobalRules.Latency.Probability != 1.0 {
		t.Errorf("GlobalRules.Latency.Probability = %v, want 1.0", cfg.GlobalRules.Latency.Probability)
	}
	if cfg.GlobalRules.ErrorRate.Probability != 0.0 {
		t.Errorf("GlobalRules.ErrorRate.Probability = %v, want 0.0", cfg.GlobalRules.ErrorRate.Probability)
	}
	if cfg.GlobalRules.Bandwidth.Probability != 1.0 {
		t.Errorf("GlobalRules.Bandwidth.Probability = %v, want 1.0", cfg.GlobalRules.Bandwidth.Probability)
	}
}

func TestChaosConfig_Clamp_NilGlobal(t *testing.T) {
	cfg := &ChaosConfig{
		Enabled:     true,
		GlobalRules: nil, // Should not panic
	}
	cfg.Clamp() // Should not panic
}

func TestNewInjector_ClampsProbabilities(t *testing.T) {
	// Probability > 1.0 should be clamped, not rejected
	config := &ChaosConfig{
		Enabled: true,
		Rules: []ChaosRule{
			{
				PathPattern: "/api/.*",
				Probability: 1.5,
				Faults: []FaultConfig{
					{Type: FaultError, Probability: 2.0},
				},
			},
		},
	}

	inj, err := NewInjector(config)
	if err != nil {
		t.Fatalf("NewInjector() should not error after clamping, got: %v", err)
	}

	// The clamped config should always inject (probability clamped to 1.0)
	req := httptest.NewRequest("GET", "/api/test", nil)
	faults := inj.ShouldInject(req)
	if len(faults) != 1 {
		t.Errorf("Expected 1 fault (clamped probability 1.0), got %d", len(faults))
	}
}

func TestUpdateConfig_ClampsProbabilities(t *testing.T) {
	config := &ChaosConfig{Enabled: true}
	inj, err := NewInjector(config)
	if err != nil {
		t.Fatalf("NewInjector() error = %v", err)
	}

	// Probability > 1.0 on faults should be clamped to 1.0
	newConfig := &ChaosConfig{
		Enabled: true,
		Rules: []ChaosRule{
			{
				PathPattern: ".*",
				Probability: 1.5, // Over 1.0, should be clamped to 1.0
				Faults: []FaultConfig{
					{Type: FaultError, Probability: 5.0}, // Over 1.0, should be clamped to 1.0
				},
			},
		},
	}

	err = inj.UpdateConfig(newConfig)
	if err != nil {
		t.Fatalf("UpdateConfig() should not error after clamping, got: %v", err)
	}

	// Probability was clamped to 1.0, so rule should always fire
	req := httptest.NewRequest("GET", "/test", nil)
	faults := inj.ShouldInject(req)
	if len(faults) != 1 {
		t.Fatalf("Expected 1 fault (clamped probability 1.0), got %d", len(faults))
	}

	// Negative fault probability should be clamped to 0 (never fires)
	newConfig2 := &ChaosConfig{
		Enabled: true,
		Rules: []ChaosRule{
			{
				PathPattern: ".*",
				Probability: 1.0,
				Faults: []FaultConfig{
					{Type: FaultError, Probability: -0.5}, // Below 0, clamped to 0
				},
			},
		},
	}

	err = inj.UpdateConfig(newConfig2)
	if err != nil {
		t.Fatalf("UpdateConfig() should not error after clamping, got: %v", err)
	}

	for i := 0; i < 100; i++ {
		req := httptest.NewRequest("GET", "/test", nil)
		faults := inj.ShouldInject(req)
		if len(faults) > 0 {
			t.Fatal("Fault with clamped probability 0 should never inject")
		}
	}
}

// =============================================================================
// Per-path rule preemption tests
// =============================================================================

func TestInjector_PerPathRulesPreemptGlobal(t *testing.T) {
	// When a per-path rule matches a request (path + method), global rules
	// should NOT apply — even if the per-path fault probability is 0.
	// The per-path rule "claims" the request, preempting global config.
	config := &ChaosConfig{
		Enabled: true,
		Rules: []ChaosRule{
			{
				PathPattern: "/api/.*",
				Probability: 1.0, // Rule always fires
				Faults: []FaultConfig{
					{Type: FaultError, Probability: 0.0}, // Fault never triggers
				},
			},
		},
		GlobalRules: &GlobalChaosRules{
			Latency: &LatencyFault{
				Min:         "1ms",
				Max:         "2ms",
				Probability: 1.0, // Global always fires
			},
		},
	}

	inj, err := NewInjector(config)
	if err != nil {
		t.Fatalf("NewInjector() error = %v", err)
	}

	// Request matching per-path rule: global should be preempted.
	// Even though the per-path fault has prob=0 (never triggers),
	// the path match itself should block global rules.
	for i := 0; i < 50; i++ {
		req := httptest.NewRequest("GET", "/api/data", nil)
		faults := inj.ShouldInject(req)
		for _, f := range faults {
			if f.Type == FaultLatency {
				t.Fatal("Global latency fault should not apply when per-path rule matches")
			}
		}
		if len(faults) > 0 {
			t.Fatalf("Per-path rule with fault prob=0 should produce 0 faults, got %d", len(faults))
		}
	}

	// Request NOT matching per-path rule: global should apply
	req := httptest.NewRequest("GET", "/health", nil)
	faults := inj.ShouldInject(req)
	if len(faults) == 0 {
		t.Error("No per-path rule matched /health; global rules should apply but returned 0 faults")
	}
}

func TestInjector_PerPathRuleMethodMismatch_FallsBackToGlobal(t *testing.T) {
	// If the path matches but method does NOT match, the rule does NOT
	// count as matched, so global should apply.
	config := &ChaosConfig{
		Enabled: true,
		Rules: []ChaosRule{
			{
				PathPattern: "/api/.*",
				Methods:     []string{"POST"}, // Only POST
				Probability: 1.0,
				Faults: []FaultConfig{
					{Type: FaultError, Probability: 1.0},
				},
			},
		},
		GlobalRules: &GlobalChaosRules{
			Latency: &LatencyFault{
				Min:         "1ms",
				Max:         "2ms",
				Probability: 1.0,
			},
		},
	}

	inj, err := NewInjector(config)
	if err != nil {
		t.Fatalf("NewInjector() error = %v", err)
	}

	// GET /api/data — path matches but method doesn't, so global should apply
	req := httptest.NewRequest("GET", "/api/data", nil)
	faults := inj.ShouldInject(req)
	if len(faults) == 0 {
		t.Error("Method mismatch should NOT count as a path-rule match; global rules should apply")
	}

	foundLatency := false
	for _, f := range faults {
		if f.Type == FaultLatency {
			foundLatency = true
		}
	}
	if !foundLatency {
		t.Error("Expected global latency fault to apply when method doesn't match per-path rule")
	}
}

func TestInjector_PerPathRulePartialProbability_NoGlobalFallback(t *testing.T) {
	// Per-path rule has prob=0.5. When it matches but the probability roll
	// fails, global rules should still NOT apply.
	config := &ChaosConfig{
		Enabled: true,
		Rules: []ChaosRule{
			{
				PathPattern: "/api/.*",
				Probability: 0.5, // Fires ~50% of the time
				Faults: []FaultConfig{
					{Type: FaultError, Probability: 1.0},
				},
			},
		},
		GlobalRules: &GlobalChaosRules{
			Latency: &LatencyFault{
				Min:         "1ms",
				Max:         "2ms",
				Probability: 1.0,
			},
		},
	}

	inj, err := NewInjector(config)
	if err != nil {
		t.Fatalf("NewInjector() error = %v", err)
	}

	// Run many times. We should NEVER see a latency fault (from global rules)
	// for requests that match the per-path pattern.
	for i := 0; i < 500; i++ {
		req := httptest.NewRequest("GET", "/api/data", nil)
		faults := inj.ShouldInject(req)
		for _, f := range faults {
			if f.Type == FaultLatency {
				t.Fatal("Global latency fault should never apply when per-path rule matches the request")
			}
		}
	}
}
