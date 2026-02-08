// Package ratelimit provides unified token-bucket rate limiting primitives.
//
// It offers two levels of abstraction:
//   - Bucket: a single token bucket (no per-IP tracking). Suitable for
//     stream-level rate limiting (e.g., SSE event delivery).
//   - PerIPLimiter: a per-IP rate limiter with automatic cleanup, suitable
//     for HTTP middleware protecting admin and engine endpoints.
package ratelimit

import (
	"context"
	"sync"
	"time"
)

// Bucket is a single token bucket rate limiter.
// It is safe for concurrent use.
type Bucket struct {
	tokens     float64
	maxTokens  float64
	rate       float64 // tokens per second
	lastUpdate time.Time
	mu         sync.Mutex
}

// BucketStats contains token bucket statistics.
type BucketStats struct {
	Available float64 `json:"available"`
	Max       float64 `json:"max"`
	Rate      float64 `json:"rate"`
}

// NewBucket creates a new token bucket with the given rate (tokens/second)
// and burst (maximum tokens). The bucket starts full.
func NewBucket(rate float64, burst int) *Bucket {
	maxTokens := float64(burst)
	if maxTokens <= 0 {
		maxTokens = rate
	}
	return &Bucket{
		tokens:     maxTokens,
		maxTokens:  maxTokens,
		rate:       rate,
		lastUpdate: time.Now(),
	}
}

// refill adds tokens based on elapsed time. Caller must hold b.mu.
func (b *Bucket) refill(now time.Time) {
	elapsed := now.Sub(b.lastUpdate).Seconds()
	b.tokens += elapsed * b.rate
	if b.tokens > b.maxTokens {
		b.tokens = b.maxTokens
	}
	b.lastUpdate = now
}

// Allow tries to consume one token. Returns true if a token was available.
func (b *Bucket) Allow() bool {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.refill(time.Now())

	if b.tokens >= 1 {
		b.tokens--
		return true
	}
	return false
}

// TryAcquire is an alias for Allow.
func (b *Bucket) TryAcquire() bool {
	return b.Allow()
}

// Wait blocks until a token is available or ctx is cancelled.
func (b *Bucket) Wait(ctx context.Context) error {
	b.mu.Lock()
	b.refill(time.Now())

	if b.tokens >= 1 {
		b.tokens--
		b.mu.Unlock()
		return nil
	}

	// Calculate how long until one token is available
	waitTime := time.Duration((1 - b.tokens) / b.rate * float64(time.Second))
	b.mu.Unlock()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(waitTime):
		b.mu.Lock()
		b.tokens = 0 // consume the token we waited for
		b.mu.Unlock()
		return nil
	}
}

// Available returns the current number of tokens (including time-based refill).
func (b *Bucket) Available() float64 {
	b.mu.Lock()
	defer b.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(b.lastUpdate).Seconds()
	tokens := b.tokens + elapsed*b.rate
	if tokens > b.maxTokens {
		tokens = b.maxTokens
	}
	return tokens
}

// Reset refills the bucket to its maximum capacity.
func (b *Bucket) Reset() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.tokens = b.maxTokens
	b.lastUpdate = time.Now()
}

// Stats returns the current bucket statistics.
func (b *Bucket) Stats() BucketStats {
	return BucketStats{
		Available: b.Available(),
		Max:       b.maxTokens,
		Rate:      b.rate,
	}
}
