package sse

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/getmockd/mockd/pkg/config"
	"github.com/getmockd/mockd/pkg/mock"
)

func TestSSEHandler_SetSSEHeaders(t *testing.T) {
	handler := NewSSEHandler(100)
	w := httptest.NewRecorder()

	handler.setSSEHeaders(w)

	// Check Content-Type
	contentType := w.Header().Get("Content-Type")
	if contentType != ContentTypeEventStream {
		t.Errorf("expected Content-Type %q, got %q", ContentTypeEventStream, contentType)
	}

	// Check Cache-Control
	cacheControl := w.Header().Get("Cache-Control")
	if cacheControl != "no-cache" {
		t.Errorf("expected Cache-Control 'no-cache', got %q", cacheControl)
	}

	// Check Connection
	connection := w.Header().Get("Connection")
	if connection != "keep-alive" {
		t.Errorf("expected Connection 'keep-alive', got %q", connection)
	}

	// Check X-Accel-Buffering (nginx)
	accelBuffering := w.Header().Get("X-Accel-Buffering")
	if accelBuffering != "no" {
		t.Errorf("expected X-Accel-Buffering 'no', got %q", accelBuffering)
	}
}

func TestSSEHandler_FlusherDetection(t *testing.T) {
	handler := NewSSEHandler(100)

	// Create a mock configuration
	mockCfg := &config.MockConfiguration{
		ID:   "test-sse",
		Name: "Test SSE",
		Type: mock.MockTypeHTTP,
		HTTP: &mock.HTTPSpec{
			SSE: &mock.SSEConfig{
				Events: []mock.SSEEventDef{
					{Data: "test"},
				},
			},
		},
	}

	// Test with non-flushing response writer
	t.Run("non-flushing writer", func(t *testing.T) {
		w := &nonFlushingResponseWriter{header: make(http.Header)}
		r := httptest.NewRequest(http.MethodGet, "/events", nil)

		handler.ServeHTTP(w, r, mockCfg)

		// Should return error about streaming not supported
		if w.code != http.StatusInternalServerError {
			t.Errorf("expected status %d, got %d", http.StatusInternalServerError, w.code)
		}
	})
}

// nonFlushingResponseWriter is a ResponseWriter that doesn't implement Flusher
type nonFlushingResponseWriter struct {
	header http.Header
	body   []byte
	code   int
}

func (w *nonFlushingResponseWriter) Header() http.Header {
	return w.header
}

func (w *nonFlushingResponseWriter) Write(b []byte) (int, error) {
	w.body = append(w.body, b...)
	return len(b), nil
}

func (w *nonFlushingResponseWriter) WriteHeader(code int) {
	w.code = code
}

func TestSSEHandler_GenerateStreamID(t *testing.T) {
	handler := NewSSEHandler(100)

	id1 := handler.generateStreamID()
	id2 := handler.generateStreamID()

	if id1 == id2 {
		t.Error("expected unique stream IDs")
	}

	if !strings.HasPrefix(id1, "sse-") {
		t.Errorf("expected ID to start with 'sse-', got %q", id1)
	}
}

func TestSSEHandler_ConfigFromMock(t *testing.T) {
	handler := NewSSEHandler(100)

	delay := 100
	mockCfg := &mock.SSEConfig{
		Events: []mock.SSEEventDef{
			{Type: "message", Data: "Hello", ID: "1"},
		},
		Timing: mock.SSETimingConfig{
			FixedDelay:   &delay,
			InitialDelay: 50,
		},
		Lifecycle: mock.SSELifecycleConfig{
			KeepaliveInterval: 15,
			MaxEvents:         100,
		},
		Resume: mock.SSEResumeConfig{
			Enabled:    true,
			BufferSize: 50,
		},
	}

	sseConfig := handler.configFromMock(mockCfg)

	// Verify events
	if len(sseConfig.Events) != 1 {
		t.Errorf("expected 1 event, got %d", len(sseConfig.Events))
	}
	if sseConfig.Events[0].Type != "message" {
		t.Errorf("expected event type 'message', got %q", sseConfig.Events[0].Type)
	}

	// Verify timing
	if sseConfig.Timing.InitialDelay != 50 {
		t.Errorf("expected initial delay 50, got %d", sseConfig.Timing.InitialDelay)
	}
	if sseConfig.Timing.FixedDelay == nil || *sseConfig.Timing.FixedDelay != 100 {
		t.Error("expected fixed delay 100")
	}

	// Verify lifecycle
	if sseConfig.Lifecycle.KeepaliveInterval != 15 {
		t.Errorf("expected keepalive 15, got %d", sseConfig.Lifecycle.KeepaliveInterval)
	}
	if sseConfig.Lifecycle.MaxEvents != 100 {
		t.Errorf("expected max events 100, got %d", sseConfig.Lifecycle.MaxEvents)
	}

	// Verify resume
	if !sseConfig.Resume.Enabled {
		t.Error("expected resume to be enabled")
	}
	if sseConfig.Resume.BufferSize != 50 {
		t.Errorf("expected buffer size 50, got %d", sseConfig.Resume.BufferSize)
	}
}

func TestSSEHandler_GetManager(t *testing.T) {
	handler := NewSSEHandler(100)
	manager := handler.GetManager()

	if manager == nil {
		t.Error("expected manager to not be nil")
	}
}

func TestSSEHandler_GetTemplates(t *testing.T) {
	handler := NewSSEHandler(100)
	templates := handler.GetTemplates()

	if templates == nil {
		t.Error("expected templates to not be nil")
	}

	// Check built-in templates
	if _, ok := templates.Get(TemplateOpenAIChat); !ok {
		t.Error("expected openai-chat template to be registered")
	}
}

func TestSSEHandler_Buffer(t *testing.T) {
	handler := NewSSEHandler(100)

	// Initially no buffer
	buffer := handler.GetBuffer("test-mock")
	if buffer != nil {
		t.Error("expected no buffer initially")
	}

	// Buffer an event
	handler.bufferEvent("test-mock", &SSEEventDef{Data: "test", ID: "1"}, 0)

	// Now buffer should exist
	buffer = handler.GetBuffer("test-mock")
	if buffer == nil {
		t.Error("expected buffer to exist")
	}
	if buffer.Size() != 1 {
		t.Errorf("expected buffer size 1, got %d", buffer.Size())
	}

	// Clear buffer
	handler.ClearBuffer("test-mock")
	buffer = handler.GetBuffer("test-mock")
	if buffer != nil {
		t.Error("expected buffer to be cleared")
	}
}

func TestFormatStreamID(t *testing.T) {
	tests := []struct {
		id       int64
		expected string
	}{
		{1, "sse-1"},
		{42, "sse-42"},
		{100, "sse-100"},
		{0, "sse-0"},
	}

	for _, tc := range tests {
		result := formatStreamID(tc.id)
		if result != tc.expected {
			t.Errorf("formatStreamID(%d) = %q, expected %q", tc.id, result, tc.expected)
		}
	}
}

func TestFormatInt64(t *testing.T) {
	tests := []struct {
		n        int64
		expected string
	}{
		{0, "0"},
		{1, "1"},
		{42, "42"},
		{123456789, "123456789"},
	}

	for _, tc := range tests {
		result := formatInt64(tc.n)
		if result != tc.expected {
			t.Errorf("formatInt64(%d) = %q, expected %q", tc.n, result, tc.expected)
		}
	}
}

func TestSSEStream_Properties(t *testing.T) {
	now := time.Now()
	stream := &SSEStream{
		ID:         "test-1",
		MockID:     "mock-1",
		ClientIP:   "127.0.0.1:1234",
		UserAgent:  "test-agent",
		StartTime:  now,
		EventsSent: 10,
		BytesSent:  1024,
		Status:     StreamStatusActive,
	}

	if stream.ID != "test-1" {
		t.Errorf("expected ID 'test-1', got %q", stream.ID)
	}
	if stream.MockID != "mock-1" {
		t.Errorf("expected MockID 'mock-1', got %q", stream.MockID)
	}
	if stream.Status != StreamStatusActive {
		t.Errorf("expected status Active, got %v", stream.Status)
	}
}

// ============================================================================
// Rate Limiting Integration Tests
// ============================================================================

func TestSSEHandler_RateLimitConfigFromMock(t *testing.T) {
	handler := NewSSEHandler(100)

	t.Run("copies rate limit config when present", func(t *testing.T) {
		mockCfg := &mock.SSEConfig{
			Events: []mock.SSEEventDef{
				{Data: "test"},
			},
			RateLimit: &mock.SSERateLimitConfig{
				EventsPerSecond: 5.0,
				BurstSize:       10,
				Strategy:        "wait",
				Headers:         true,
			},
		}

		sseConfig := handler.configFromMock(mockCfg)

		if sseConfig.RateLimit == nil {
			t.Fatal("expected RateLimit config to be copied")
		}
		if sseConfig.RateLimit.EventsPerSecond != 5.0 {
			t.Errorf("expected EventsPerSecond=5.0, got %v", sseConfig.RateLimit.EventsPerSecond)
		}
		if sseConfig.RateLimit.BurstSize != 10 {
			t.Errorf("expected BurstSize=10, got %v", sseConfig.RateLimit.BurstSize)
		}
		if sseConfig.RateLimit.Strategy != "wait" {
			t.Errorf("expected Strategy=wait, got %v", sseConfig.RateLimit.Strategy)
		}
		if !sseConfig.RateLimit.Headers {
			t.Error("expected Headers=true")
		}
	})

	t.Run("rate limit config is nil when not configured", func(t *testing.T) {
		mockCfg := &mock.SSEConfig{
			Events: []mock.SSEEventDef{
				{Data: "test"},
			},
		}

		sseConfig := handler.configFromMock(mockCfg)

		if sseConfig.RateLimit != nil {
			t.Error("expected RateLimit config to be nil")
		}
	})

	t.Run("copies all strategy types", func(t *testing.T) {
		strategies := []string{"wait", "drop", "error"}

		for _, strategy := range strategies {
			mockCfg := &mock.SSEConfig{
				Events: []mock.SSEEventDef{{Data: "test"}},
				RateLimit: &mock.SSERateLimitConfig{
					EventsPerSecond: 10,
					Strategy:        strategy,
				},
			}

			sseConfig := handler.configFromMock(mockCfg)

			if sseConfig.RateLimit.Strategy != strategy {
				t.Errorf("expected Strategy=%s, got %v", strategy, sseConfig.RateLimit.Strategy)
			}
		}
	})
}

func TestSSEHandler_RateLimiterCreation(t *testing.T) {
	t.Run("rate limiter created when config has positive rate", func(t *testing.T) {
		config := &RateLimitConfig{
			EventsPerSecond: 10.0,
			BurstSize:       20,
			Strategy:        RateLimitStrategyWait,
		}

		limiter := NewRateLimiter(config)

		if limiter == nil {
			t.Fatal("expected rate limiter to be created")
		}

		stats := limiter.Stats()
		if stats.Rate != 10.0 {
			t.Errorf("expected rate=10.0, got %v", stats.Rate)
		}
		if stats.MaxTokens != 20.0 {
			t.Errorf("expected maxTokens=20.0, got %v", stats.MaxTokens)
		}
	})

	t.Run("rate limiter nil when config is nil", func(t *testing.T) {
		limiter := NewRateLimiter(nil)

		if limiter != nil {
			t.Error("expected rate limiter to be nil")
		}
	})

	t.Run("burst size defaults to events per second when not set", func(t *testing.T) {
		config := &RateLimitConfig{
			EventsPerSecond: 15.0,
			BurstSize:       0, // Not set
			Strategy:        RateLimitStrategyWait,
		}

		limiter := NewRateLimiter(config)
		stats := limiter.Stats()

		if stats.MaxTokens != 15.0 {
			t.Errorf("expected maxTokens to default to rate (15.0), got %v", stats.MaxTokens)
		}
	})
}

func TestSSEHandler_RateLimitStrategies(t *testing.T) {
	t.Run("wait strategy blocks until token available", func(t *testing.T) {
		config := &RateLimitConfig{
			EventsPerSecond: 100, // 100/sec = 10ms per token
			BurstSize:       1,   // Only 1 token
			Strategy:        RateLimitStrategyWait,
		}

		limiter := NewRateLimiter(config)
		ctx := context.Background()

		// First call should succeed immediately (uses burst token)
		start := time.Now()
		err := limiter.Wait(ctx)
		if err != nil {
			t.Fatalf("first wait failed: %v", err)
		}
		firstDuration := time.Since(start)

		// Second call should block waiting for token refill
		start = time.Now()
		err = limiter.Wait(ctx)
		if err != nil {
			t.Fatalf("second wait failed: %v", err)
		}
		secondDuration := time.Since(start)

		// Second call should have taken longer (waiting for refill)
		if secondDuration < 5*time.Millisecond {
			t.Errorf("expected second call to wait for token refill, took only %v", secondDuration)
		}

		// First call should be fast
		if firstDuration > 5*time.Millisecond {
			t.Errorf("expected first call to be immediate, took %v", firstDuration)
		}
	})

	t.Run("drop strategy returns error immediately", func(t *testing.T) {
		config := &RateLimitConfig{
			EventsPerSecond: 100,
			BurstSize:       1,
			Strategy:        RateLimitStrategyDrop,
		}

		limiter := NewRateLimiter(config)
		ctx := context.Background()

		// First call consumes the token
		err := limiter.Wait(ctx)
		if err != nil {
			t.Fatalf("first wait failed: %v", err)
		}

		// Second call should return ErrRateLimited immediately
		start := time.Now()
		err = limiter.Wait(ctx)
		duration := time.Since(start)

		if err != ErrRateLimited {
			t.Errorf("expected ErrRateLimited, got %v", err)
		}
		if duration > 5*time.Millisecond {
			t.Errorf("expected drop strategy to return immediately, took %v", duration)
		}
	})

	t.Run("error strategy returns error immediately", func(t *testing.T) {
		config := &RateLimitConfig{
			EventsPerSecond: 100,
			BurstSize:       1,
			Strategy:        RateLimitStrategyError,
		}

		limiter := NewRateLimiter(config)
		ctx := context.Background()

		// First call consumes the token
		_ = limiter.Wait(ctx)

		// Second call should return ErrRateLimited
		err := limiter.Wait(ctx)
		if err != ErrRateLimited {
			t.Errorf("expected ErrRateLimited, got %v", err)
		}
	})

	t.Run("wait strategy respects context cancellation", func(t *testing.T) {
		config := &RateLimitConfig{
			EventsPerSecond: 1, // Very slow - 1 per second
			BurstSize:       1,
			Strategy:        RateLimitStrategyWait,
		}

		limiter := NewRateLimiter(config)

		// Consume the token
		_ = limiter.Wait(context.Background())

		// Create context that cancels after 50ms
		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()

		// Wait should be cancelled
		err := limiter.Wait(ctx)
		if err != context.DeadlineExceeded {
			t.Errorf("expected context.DeadlineExceeded, got %v", err)
		}
	})
}

func TestSSEHandler_RateLimitHeaders(t *testing.T) {
	t.Run("returns headers when enabled", func(t *testing.T) {
		config := &RateLimitConfig{
			EventsPerSecond: 10.0,
			BurstSize:       20,
			Strategy:        RateLimitStrategyWait,
			Headers:         true,
		}

		limiter := NewRateLimiter(config)
		headers := limiter.RateLimitHeaders()

		if headers == nil {
			t.Fatal("expected headers to be returned")
		}
		if _, ok := headers["X-RateLimit-Limit"]; !ok {
			t.Error("expected X-RateLimit-Limit header")
		}
		if _, ok := headers["X-RateLimit-Remaining"]; !ok {
			t.Error("expected X-RateLimit-Remaining header")
		}
		if _, ok := headers["X-RateLimit-Reset"]; !ok {
			t.Error("expected X-RateLimit-Reset header")
		}
	})

	t.Run("returns nil when headers disabled", func(t *testing.T) {
		config := &RateLimitConfig{
			EventsPerSecond: 10.0,
			Headers:         false,
		}

		limiter := NewRateLimiter(config)
		headers := limiter.RateLimitHeaders()

		if headers != nil {
			t.Error("expected nil headers when disabled")
		}
	})

	t.Run("limit header matches configured rate", func(t *testing.T) {
		config := &RateLimitConfig{
			EventsPerSecond: 42.0,
			Headers:         true,
		}

		limiter := NewRateLimiter(config)
		headers := limiter.RateLimitHeaders()

		if headers["X-RateLimit-Limit"] != "42" {
			t.Errorf("expected X-RateLimit-Limit=42, got %v", headers["X-RateLimit-Limit"])
		}
	})

	t.Run("remaining header decreases after consumption", func(t *testing.T) {
		config := &RateLimitConfig{
			EventsPerSecond: 100,
			BurstSize:       10,
			Headers:         true,
		}

		limiter := NewRateLimiter(config)

		// Get initial remaining
		headers1 := limiter.RateLimitHeaders()
		remaining1 := headers1["X-RateLimit-Remaining"]

		// Consume a token
		_ = limiter.Wait(context.Background())

		// Check remaining decreased
		headers2 := limiter.RateLimitHeaders()
		remaining2 := headers2["X-RateLimit-Remaining"]

		// remaining2 should be less than remaining1
		// (they're strings so we compare as numbers would be more robust,
		// but for this test we just check they're different)
		if remaining1 == remaining2 {
			t.Error("expected remaining to decrease after consumption")
		}
	})
}

func TestSSEHandler_RateLimiterReset(t *testing.T) {
	config := &RateLimitConfig{
		EventsPerSecond: 100,
		BurstSize:       10,
		Strategy:        RateLimitStrategyDrop,
	}

	limiter := NewRateLimiter(config)
	ctx := context.Background()

	// Consume all tokens
	for i := 0; i < 10; i++ {
		_ = limiter.Wait(ctx)
	}

	// Should be rate limited now
	err := limiter.Wait(ctx)
	if err != ErrRateLimited {
		t.Fatal("expected to be rate limited")
	}

	// Reset the limiter
	limiter.Reset()

	// Should have tokens again
	err = limiter.Wait(ctx)
	if err != nil {
		t.Errorf("expected success after reset, got %v", err)
	}
}

func TestSSEHandler_RateLimiterTryAcquire(t *testing.T) {
	config := &RateLimitConfig{
		EventsPerSecond: 100,
		BurstSize:       2,
	}

	limiter := NewRateLimiter(config)

	// First two should succeed
	if !limiter.TryAcquire() {
		t.Error("expected first TryAcquire to succeed")
	}
	if !limiter.TryAcquire() {
		t.Error("expected second TryAcquire to succeed")
	}

	// Third should fail (no tokens left)
	if limiter.TryAcquire() {
		t.Error("expected third TryAcquire to fail")
	}

	// Wait for token refill
	time.Sleep(15 * time.Millisecond)

	// Should succeed again
	if !limiter.TryAcquire() {
		t.Error("expected TryAcquire to succeed after refill")
	}
}

func TestSSEHandler_RateLimiterStats(t *testing.T) {
	config := &RateLimitConfig{
		EventsPerSecond: 25.0,
		BurstSize:       50,
		Strategy:        RateLimitStrategyWait,
	}

	limiter := NewRateLimiter(config)

	stats := limiter.Stats()

	if stats.Rate != 25.0 {
		t.Errorf("expected Rate=25.0, got %v", stats.Rate)
	}
	if stats.MaxTokens != 50.0 {
		t.Errorf("expected MaxTokens=50.0, got %v", stats.MaxTokens)
	}
	if stats.Strategy != RateLimitStrategyWait {
		t.Errorf("expected Strategy=wait, got %v", stats.Strategy)
	}
	if stats.TokensAvailable != 50.0 {
		t.Errorf("expected TokensAvailable=50.0 initially, got %v", stats.TokensAvailable)
	}
}
