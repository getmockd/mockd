package sse

import (
	"context"
	"strconv"
	"sync"
	"time"

	"github.com/getmockd/mockd/pkg/ratelimit"
)

// RateLimiter implements token bucket rate limiting for SSE streams.
// It wraps a ratelimit.Bucket for the core token math and adds
// SSE-specific strategy handling (wait/drop/error).
type RateLimiter struct {
	config *RateLimitConfig
	bucket *ratelimit.Bucket
}

// NewRateLimiter creates a new rate limiter.
func NewRateLimiter(config *RateLimitConfig) *RateLimiter {
	if config == nil {
		return nil
	}

	return &RateLimiter{
		config: config,
		bucket: ratelimit.NewBucket(config.EventsPerSecond, config.BurstSize),
	}
}

// Wait blocks until a token is available or context is cancelled.
// Returns ErrRateLimited if strategy is "error" and rate is exceeded.
// Returns ErrRateLimited if strategy is "drop" (caller should skip the event).
func (r *RateLimiter) Wait(ctx context.Context) error {
	// Fast path: try to acquire without blocking
	if r.bucket.TryAcquire() {
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
		return r.bucket.Wait(ctx)
	}
}

// TryAcquire attempts to acquire a token without blocking.
// Returns true if a token was acquired, false otherwise.
func (r *RateLimiter) TryAcquire() bool {
	return r.bucket.TryAcquire()
}

// Available returns the number of tokens currently available.
func (r *RateLimiter) Available() float64 {
	return r.bucket.Available()
}

// Reset resets the rate limiter to full capacity.
func (r *RateLimiter) Reset() {
	r.bucket.Reset()
}

// Stats returns rate limiter statistics.
func (r *RateLimiter) Stats() RateLimiterStats {
	bs := r.bucket.Stats()
	return RateLimiterStats{
		TokensAvailable: bs.Available,
		MaxTokens:       bs.Max,
		Rate:            bs.Rate,
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
	return strconv.FormatInt(int64(f), 10)
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
