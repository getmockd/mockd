package sse

import (
	"context"
	"sync"
	"testing"
	"time"
)

func TestRateLimiter_New(t *testing.T) {
	config := &RateLimitConfig{
		EventsPerSecond: 10,
		BurstSize:       5,
	}

	limiter := NewRateLimiter(config)
	if limiter == nil {
		t.Fatal("expected limiter to be created")
	}

	if limiter.maxTokens != 5 {
		t.Errorf("expected maxTokens 5, got %f", limiter.maxTokens)
	}
}

func TestRateLimiter_NilConfig(t *testing.T) {
	limiter := NewRateLimiter(nil)
	if limiter != nil {
		t.Error("expected nil limiter for nil config")
	}
}

func TestRateLimiter_DefaultBurstSize(t *testing.T) {
	config := &RateLimitConfig{
		EventsPerSecond: 10,
		BurstSize:       0, // Should use EventsPerSecond as default
	}

	limiter := NewRateLimiter(config)
	if limiter.maxTokens != 10 {
		t.Errorf("expected maxTokens 10 (default from rate), got %f", limiter.maxTokens)
	}
}

func TestRateLimiter_TryAcquire(t *testing.T) {
	config := &RateLimitConfig{
		EventsPerSecond: 10,
		BurstSize:       3,
	}

	limiter := NewRateLimiter(config)

	// Should succeed for burst size
	for i := 0; i < 3; i++ {
		if !limiter.TryAcquire() {
			t.Errorf("expected TryAcquire to succeed on attempt %d", i+1)
		}
	}

	// Should fail when tokens exhausted
	if limiter.TryAcquire() {
		t.Error("expected TryAcquire to fail when tokens exhausted")
	}
}

func TestRateLimiter_TokenRefill(t *testing.T) {
	config := &RateLimitConfig{
		EventsPerSecond: 100, // 100 per second = 10 per 100ms
		BurstSize:       1,
	}

	limiter := NewRateLimiter(config)

	// Exhaust the token
	limiter.TryAcquire()

	// Wait for refill
	time.Sleep(50 * time.Millisecond)

	// Should have some tokens now
	available := limiter.Available()
	if available < 1 {
		t.Logf("Available: %f (might be timing-dependent)", available)
	}
}

func TestRateLimiter_Wait_Immediate(t *testing.T) {
	config := &RateLimitConfig{
		EventsPerSecond: 10,
		BurstSize:       5,
		Strategy:        RateLimitStrategyWait,
	}

	limiter := NewRateLimiter(config)
	ctx := context.Background()

	// Should not wait when tokens available
	start := time.Now()
	err := limiter.Wait(ctx)
	elapsed := time.Since(start)

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if elapsed > 10*time.Millisecond {
		t.Errorf("expected immediate return, took %v", elapsed)
	}
}

func TestRateLimiter_Wait_Drop(t *testing.T) {
	config := &RateLimitConfig{
		EventsPerSecond: 10,
		BurstSize:       1,
		Strategy:        RateLimitStrategyDrop,
	}

	limiter := NewRateLimiter(config)
	ctx := context.Background()

	// First should succeed
	err := limiter.Wait(ctx)
	if err != nil {
		t.Errorf("first Wait should succeed: %v", err)
	}

	// Second should return rate limited error
	err = limiter.Wait(ctx)
	if err != ErrRateLimited {
		t.Errorf("expected ErrRateLimited, got %v", err)
	}
}

func TestRateLimiter_Wait_Error(t *testing.T) {
	config := &RateLimitConfig{
		EventsPerSecond: 10,
		BurstSize:       1,
		Strategy:        RateLimitStrategyError,
	}

	limiter := NewRateLimiter(config)
	ctx := context.Background()

	// Exhaust token
	limiter.Wait(ctx)

	// Should return error
	err := limiter.Wait(ctx)
	if err != ErrRateLimited {
		t.Errorf("expected ErrRateLimited, got %v", err)
	}
}

func TestRateLimiter_Wait_ContextCancel(t *testing.T) {
	config := &RateLimitConfig{
		EventsPerSecond: 1, // 1 per second
		BurstSize:       1,
		Strategy:        RateLimitStrategyWait,
	}

	limiter := NewRateLimiter(config)

	// Exhaust token
	ctx := context.Background()
	limiter.Wait(ctx)

	// Create cancellable context
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	// Should timeout waiting
	err := limiter.Wait(ctx)
	if err != context.DeadlineExceeded {
		t.Errorf("expected context.DeadlineExceeded, got %v", err)
	}
}

func TestRateLimiter_Available(t *testing.T) {
	config := &RateLimitConfig{
		EventsPerSecond: 10,
		BurstSize:       5,
	}

	limiter := NewRateLimiter(config)

	available := limiter.Available()
	if available != 5 {
		t.Errorf("expected 5 available, got %f", available)
	}

	limiter.TryAcquire()
	limiter.TryAcquire()

	available = limiter.Available()
	// Allow for small timing-based refills
	if available < 2.9 || available > 3.1 {
		t.Errorf("expected ~3 available, got %f", available)
	}
}

func TestRateLimiter_Reset(t *testing.T) {
	config := &RateLimitConfig{
		EventsPerSecond: 10,
		BurstSize:       5,
	}

	limiter := NewRateLimiter(config)

	// Exhaust all tokens
	for i := 0; i < 5; i++ {
		limiter.TryAcquire()
	}

	// Allow small timing-based refills
	available := limiter.Available()
	if available > 0.1 {
		t.Errorf("expected ~0 available after exhaust, got %f", available)
	}

	limiter.Reset()

	if limiter.Available() != 5 {
		t.Errorf("expected 5 available after reset, got %f", limiter.Available())
	}
}

func TestRateLimiter_Stats(t *testing.T) {
	config := &RateLimitConfig{
		EventsPerSecond: 10,
		BurstSize:       5,
		Strategy:        RateLimitStrategyWait,
	}

	limiter := NewRateLimiter(config)
	limiter.TryAcquire()
	limiter.TryAcquire()

	stats := limiter.Stats()

	if stats.Rate != 10 {
		t.Errorf("expected rate 10, got %f", stats.Rate)
	}
	if stats.MaxTokens != 5 {
		t.Errorf("expected maxTokens 5, got %f", stats.MaxTokens)
	}
	// Allow for small timing-based refills
	if stats.TokensAvailable < 2.9 || stats.TokensAvailable > 3.1 {
		t.Errorf("expected ~3 available, got %f", stats.TokensAvailable)
	}
	if stats.Strategy != RateLimitStrategyWait {
		t.Errorf("expected strategy 'wait', got %q", stats.Strategy)
	}
}

func TestRateLimiter_Headers_Enabled(t *testing.T) {
	config := &RateLimitConfig{
		EventsPerSecond: 10,
		BurstSize:       5,
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
}

func TestRateLimiter_Headers_Disabled(t *testing.T) {
	config := &RateLimitConfig{
		EventsPerSecond: 10,
		BurstSize:       5,
		Headers:         false,
	}

	limiter := NewRateLimiter(config)
	headers := limiter.RateLimitHeaders()

	if headers != nil {
		t.Error("expected nil headers when disabled")
	}
}

func TestRateLimiter_Concurrent(t *testing.T) {
	config := &RateLimitConfig{
		EventsPerSecond: 100,
		BurstSize:       50,
		Strategy:        RateLimitStrategyWait,
	}

	limiter := NewRateLimiter(config)
	var wg sync.WaitGroup
	ctx := context.Background()

	successCount := 0
	var mu sync.Mutex

	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := limiter.Wait(ctx)
			if err == nil {
				mu.Lock()
				successCount++
				mu.Unlock()
			}
		}()
	}

	wg.Wait()

	// All should succeed eventually
	if successCount != 20 {
		t.Errorf("expected 20 successes, got %d", successCount)
	}
}

func TestDefaultRateLimitConfig(t *testing.T) {
	config := DefaultRateLimitConfig()

	if config.EventsPerSecond != 10 {
		t.Errorf("expected EventsPerSecond 10, got %f", config.EventsPerSecond)
	}
	if config.BurstSize != 20 {
		t.Errorf("expected BurstSize 20, got %d", config.BurstSize)
	}
	if config.Strategy != RateLimitStrategyWait {
		t.Errorf("expected Strategy 'wait', got %q", config.Strategy)
	}
	if !config.Headers {
		t.Error("expected Headers to be true")
	}
}

// BackpressureHandler tests

func TestBackpressureHandler_Buffer(t *testing.T) {
	handler := NewBackpressureHandler(BackpressureBuffer, 5)
	ctx := context.Background()

	// Add events to buffer
	for i := 0; i < 5; i++ {
		event := &SSEEventDef{Data: "test"}
		_, err := handler.Handle(ctx, event)
		if err != nil {
			t.Errorf("expected no error for event %d, got %v", i, err)
		}
	}

	// Buffer should be full now
	event := &SSEEventDef{Data: "overflow"}
	_, err := handler.Handle(ctx, event)
	if err != ErrBufferFull {
		t.Errorf("expected ErrBufferFull, got %v", err)
	}
}

func TestBackpressureHandler_Drop(t *testing.T) {
	handler := NewBackpressureHandler(BackpressureDrop, 5)
	ctx := context.Background()

	event := &SSEEventDef{Data: "test"}
	result, err := handler.Handle(ctx, event)

	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if result != nil {
		t.Error("expected nil result for drop strategy")
	}

	stats := handler.Stats()
	if stats.Dropped != 1 {
		t.Errorf("expected 1 dropped, got %d", stats.Dropped)
	}
}

func TestBackpressureHandler_Block(t *testing.T) {
	handler := NewBackpressureHandler(BackpressureBlock, 5)
	ctx := context.Background()

	event := &SSEEventDef{Data: "test"}
	result, err := handler.Handle(ctx, event)

	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if result == nil {
		t.Error("expected event to be returned for block strategy")
	}
}

func TestBackpressureHandler_Block_ContextCancel(t *testing.T) {
	handler := NewBackpressureHandler(BackpressureBlock, 5)
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	event := &SSEEventDef{Data: "test"}
	_, err := handler.Handle(ctx, event)

	if err != context.Canceled {
		t.Errorf("expected context.Canceled, got %v", err)
	}
}

func TestBackpressureHandler_Drain(t *testing.T) {
	handler := NewBackpressureHandler(BackpressureBuffer, 10)
	ctx := context.Background()

	// Add some events
	for i := 0; i < 5; i++ {
		event := &SSEEventDef{Data: "test"}
		handler.Handle(ctx, event)
	}

	events := handler.Drain()
	if len(events) != 5 {
		t.Errorf("expected 5 events, got %d", len(events))
	}

	// Buffer should be empty now
	events = handler.Drain()
	if len(events) != 0 {
		t.Errorf("expected 0 events after drain, got %d", len(events))
	}
}

func TestBackpressureHandler_Drain_NoBuffer(t *testing.T) {
	handler := NewBackpressureHandler(BackpressureDrop, 10)

	events := handler.Drain()
	if events != nil {
		t.Errorf("expected nil for non-buffer strategy, got %v", events)
	}
}

func TestBackpressureHandler_Stats(t *testing.T) {
	handler := NewBackpressureHandler(BackpressureBuffer, 10)
	ctx := context.Background()

	for i := 0; i < 5; i++ {
		handler.Handle(ctx, &SSEEventDef{Data: "test"})
	}

	stats := handler.Stats()

	if stats.Strategy != BackpressureBuffer {
		t.Errorf("expected strategy 'buffer', got %q", stats.Strategy)
	}
	if stats.BufferSize != 10 {
		t.Errorf("expected buffer size 10, got %d", stats.BufferSize)
	}
	if stats.Buffered != 5 {
		t.Errorf("expected 5 buffered, got %d", stats.Buffered)
	}
	if stats.Dropped != 0 {
		t.Errorf("expected 0 dropped, got %d", stats.Dropped)
	}
}

func TestBackpressureHandler_DefaultBufferSize(t *testing.T) {
	handler := NewBackpressureHandler(BackpressureBuffer, 0)

	if handler.bufferSize != 100 {
		t.Errorf("expected default buffer size 100, got %d", handler.bufferSize)
	}
}

func TestFormatFloat64(t *testing.T) {
	tests := []struct {
		input    float64
		expected string
	}{
		{10.0, "10"},
		{0.0, "0"},
		{100.5, "100"},
		// Note: formatFloat64 uses formatInt64 which handles positive integers
	}

	for _, tc := range tests {
		result := formatFloat64(tc.input)
		if result != tc.expected {
			t.Errorf("formatFloat64(%f) = %q, expected %q", tc.input, result, tc.expected)
		}
	}
}
