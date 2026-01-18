package sse

import (
	"context"
	"sync"
	"time"
)

// RateLimiter implements token bucket rate limiting for SSE streams.
type RateLimiter struct {
	config    *RateLimitConfig
	tokens    float64
	maxTokens float64
	lastTime  time.Time
	mu        sync.Mutex
}

// NewRateLimiter creates a new rate limiter.
func NewRateLimiter(config *RateLimitConfig) *RateLimiter {
	if config == nil {
		return nil
	}

	maxTokens := float64(config.BurstSize)
	if maxTokens <= 0 {
		maxTokens = config.EventsPerSecond
	}

	return &RateLimiter{
		config:    config,
		tokens:    maxTokens, // Start with full bucket
		maxTokens: maxTokens,
		lastTime:  time.Now(),
	}
}

// Wait blocks until a token is available or context is cancelled.
// Returns ErrRateLimited if strategy is "error" and rate is exceeded.
// Returns nil if strategy is "drop" (caller should skip the event).
func (r *RateLimiter) Wait(ctx context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Refill tokens based on elapsed time
	now := time.Now()
	elapsed := now.Sub(r.lastTime).Seconds()
	r.tokens += elapsed * r.config.EventsPerSecond
	if r.tokens > r.maxTokens {
		r.tokens = r.maxTokens
	}
	r.lastTime = now

	// Check if we have tokens
	if r.tokens >= 1 {
		r.tokens--
		return nil
	}

	// No tokens available - handle based on strategy
	switch r.config.Strategy {
	case RateLimitStrategyDrop:
		return ErrRateLimited // Caller should skip this event

	case RateLimitStrategyError:
		return ErrRateLimited

	case RateLimitStrategyWait:
		fallthrough
	default:
		// Calculate wait time
		waitTime := time.Duration((1 - r.tokens) / r.config.EventsPerSecond * float64(time.Second))
		r.mu.Unlock()

		select {
		case <-ctx.Done():
			r.mu.Lock()
			return ctx.Err()
		case <-time.After(waitTime):
			r.mu.Lock()
			r.tokens = 0 // Consume the token we waited for
			return nil
		}
	}
}

// TryAcquire attempts to acquire a token without blocking.
// Returns true if a token was acquired, false otherwise.
func (r *RateLimiter) TryAcquire() bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Refill tokens
	now := time.Now()
	elapsed := now.Sub(r.lastTime).Seconds()
	r.tokens += elapsed * r.config.EventsPerSecond
	if r.tokens > r.maxTokens {
		r.tokens = r.maxTokens
	}
	r.lastTime = now

	if r.tokens >= 1 {
		r.tokens--
		return true
	}
	return false
}

// Available returns the number of tokens currently available.
func (r *RateLimiter) Available() float64 {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Refill tokens
	now := time.Now()
	elapsed := now.Sub(r.lastTime).Seconds()
	tokens := r.tokens + elapsed*r.config.EventsPerSecond
	if tokens > r.maxTokens {
		tokens = r.maxTokens
	}

	return tokens
}

// Reset resets the rate limiter to full capacity.
func (r *RateLimiter) Reset() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.tokens = r.maxTokens
	r.lastTime = time.Now()
}

// Stats returns rate limiter statistics.
func (r *RateLimiter) Stats() RateLimiterStats {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Calculate current tokens
	now := time.Now()
	elapsed := now.Sub(r.lastTime).Seconds()
	tokens := r.tokens + elapsed*r.config.EventsPerSecond
	if tokens > r.maxTokens {
		tokens = r.maxTokens
	}

	return RateLimiterStats{
		TokensAvailable: tokens,
		MaxTokens:       r.maxTokens,
		Rate:            r.config.EventsPerSecond,
		Strategy:        r.config.Strategy,
	}
}

// RateLimiterStats contains rate limiter statistics.
type RateLimiterStats struct {
	TokensAvailable float64 `json:"tokensAvailable"`
	MaxTokens       float64 `json:"maxTokens"`
	Rate            float64 `json:"rate"`
	Strategy        string  `json:"strategy"`
}

// RateLimitHeaders returns rate limit headers for the response.
func (r *RateLimiter) RateLimitHeaders() map[string]string {
	if r == nil || !r.config.Headers {
		return nil
	}

	stats := r.Stats()

	return map[string]string{
		"X-RateLimit-Limit":     formatFloat64(r.config.EventsPerSecond),
		"X-RateLimit-Remaining": formatFloat64(stats.TokensAvailable),
		"X-RateLimit-Reset":     formatFloat64(float64(time.Now().Add(time.Second).Unix())),
	}
}

// formatFloat64 formats a float64 as a string.
func formatFloat64(f float64) string {
	// Simple formatting - just integer part for rate limits
	return formatInt64(int64(f))
}

// BackpressureHandler manages backpressure for SSE streams.
type BackpressureHandler struct {
	strategy   string
	buffer     chan *SSEEventDef
	bufferSize int
	mu         sync.Mutex
	dropped    int64
}

// NewBackpressureHandler creates a new backpressure handler.
func NewBackpressureHandler(strategy string, bufferSize int) *BackpressureHandler {
	if bufferSize <= 0 {
		bufferSize = 100
	}

	h := &BackpressureHandler{
		strategy:   strategy,
		bufferSize: bufferSize,
	}

	if strategy == BackpressureBuffer {
		h.buffer = make(chan *SSEEventDef, bufferSize)
	}

	return h
}

// Handle processes an event according to the backpressure strategy.
// Returns nil if the event should be sent, the event if buffered, or an error.
func (h *BackpressureHandler) Handle(ctx context.Context, event *SSEEventDef) (*SSEEventDef, error) {
	switch h.strategy {
	case BackpressureBuffer:
		select {
		case h.buffer <- event:
			return nil, nil // Buffered
		default:
			// Buffer full
			h.mu.Lock()
			h.dropped++
			h.mu.Unlock()
			return nil, ErrBufferFull
		}

	case BackpressureDrop:
		// Just drop the event
		h.mu.Lock()
		h.dropped++
		h.mu.Unlock()
		return nil, nil

	case BackpressureBlock:
		// Block until the client catches up (context handles timeout)
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
			return event, nil // Send immediately
		}

	default:
		return event, nil
	}
}

// Drain returns all buffered events.
func (h *BackpressureHandler) Drain() []*SSEEventDef {
	if h.buffer == nil {
		return nil
	}

	events := make([]*SSEEventDef, 0)
	for {
		select {
		case event := <-h.buffer:
			events = append(events, event)
		default:
			return events
		}
	}
}

// Stats returns backpressure statistics.
func (h *BackpressureHandler) Stats() BackpressureStats {
	h.mu.Lock()
	defer h.mu.Unlock()

	buffered := 0
	if h.buffer != nil {
		buffered = len(h.buffer)
	}

	return BackpressureStats{
		Strategy:   h.strategy,
		BufferSize: h.bufferSize,
		Buffered:   buffered,
		Dropped:    h.dropped,
	}
}

// BackpressureStats contains backpressure statistics.
type BackpressureStats struct {
	Strategy   string `json:"strategy"`
	BufferSize int    `json:"bufferSize"`
	Buffered   int    `json:"buffered"`
	Dropped    int64  `json:"dropped"`
}

// DefaultRateLimitConfig returns a default rate limit configuration.
func DefaultRateLimitConfig() *RateLimitConfig {
	return &RateLimitConfig{
		EventsPerSecond: 10,
		BurstSize:       20,
		Strategy:        RateLimitStrategyWait,
		Headers:         true,
	}
}
