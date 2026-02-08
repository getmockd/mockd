package ratelimit

import (
	"context"
	"sync"
	"testing"
	"time"
)

func TestNewBucket_CorrectRateAndBurst(t *testing.T) {
	t.Parallel()
	b := NewBucket(50, 10)

	stats := b.Stats()
	if stats.Rate != 50 {
		t.Errorf("expected rate 50, got %v", stats.Rate)
	}
	if stats.Max != 10 {
		t.Errorf("expected max 10, got %v", stats.Max)
	}
	// Bucket should start full.
	if stats.Available < 9.9 {
		t.Errorf("expected bucket to start full (~10), got %v", stats.Available)
	}
}

func TestNewBucket_ZeroBurstDefaultsToRate(t *testing.T) {
	t.Parallel()
	b := NewBucket(25, 0)

	stats := b.Stats()
	if stats.Max != 25 {
		t.Errorf("expected max to default to rate (25), got %v", stats.Max)
	}
	if stats.Available < 24.9 {
		t.Errorf("expected bucket to start full (~25), got %v", stats.Available)
	}
}

func TestAllow_SucceedsWhenTokensAvailable(t *testing.T) {
	t.Parallel()
	b := NewBucket(100, 5)

	if !b.Allow() {
		t.Error("expected Allow to return true when tokens are available")
	}
}

func TestAllow_FailsWhenEmpty(t *testing.T) {
	t.Parallel()
	b := NewBucket(1, 1)

	if !b.Allow() {
		t.Fatal("first Allow should succeed")
	}
	if b.Allow() {
		t.Error("expected Allow to return false when bucket is empty")
	}
}

func TestAllow_DrainsTokensCorrectly(t *testing.T) {
	t.Parallel()
	b := NewBucket(1, 3) // burst=3, very low rate so no meaningful refill

	for i := 0; i < 3; i++ {
		if !b.Allow() {
			t.Errorf("Allow #%d should have succeeded", i+1)
		}
	}
	if b.Allow() {
		t.Error("fourth Allow should have failed after draining 3 tokens")
	}
}

func TestTryAcquire_IsAliasForAllow(t *testing.T) {
	t.Parallel()
	b := NewBucket(1, 2)

	if !b.TryAcquire() {
		t.Error("first TryAcquire should succeed")
	}
	if !b.TryAcquire() {
		t.Error("second TryAcquire should succeed")
	}
	if b.TryAcquire() {
		t.Error("third TryAcquire should fail after draining 2 tokens")
	}
}

func TestAllow_TokenRefillOverTime(t *testing.T) {
	t.Parallel()
	// Rate of 100 tokens/sec, burst of 1. After draining, sleeping 50ms
	// should refill ~5 tokens, but burst caps at 1, so at least 1 should be available.
	b := NewBucket(100, 1)

	if !b.Allow() {
		t.Fatal("first Allow should succeed")
	}
	if b.Allow() {
		t.Fatal("second Allow should fail immediately")
	}

	time.Sleep(50 * time.Millisecond)

	if !b.Allow() {
		t.Error("Allow should succeed after refill period")
	}
}

func TestWait_ReturnsImmediatelyWhenTokensAvailable(t *testing.T) {
	t.Parallel()
	b := NewBucket(100, 5)

	ctx := context.Background()
	start := time.Now()
	err := b.Wait(ctx)
	elapsed := time.Since(start)

	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if elapsed > 50*time.Millisecond {
		t.Errorf("Wait took too long (%v), expected near-instant", elapsed)
	}
}

func TestWait_BlocksAndSucceedsAfterRefill(t *testing.T) {
	t.Parallel()
	// Rate of 100/s, burst=1. Drain, then Wait should block ~10ms.
	b := NewBucket(100, 1)
	b.Allow() // drain the one token

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	start := time.Now()
	err := b.Wait(ctx)
	elapsed := time.Since(start)

	if err != nil {
		t.Errorf("expected Wait to succeed, got %v", err)
	}
	if elapsed < 5*time.Millisecond {
		t.Errorf("Wait returned too quickly (%v), expected some blocking", elapsed)
	}
}

func TestWait_ReturnsContextErrorOnCancel(t *testing.T) {
	t.Parallel()
	b := NewBucket(0.5, 1) // very slow refill: 1 token every 2 seconds
	b.Allow()              // drain

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()

	err := b.Wait(ctx)
	if err == nil {
		t.Error("expected context error, got nil")
	}
	if err != context.DeadlineExceeded {
		t.Errorf("expected DeadlineExceeded, got %v", err)
	}
}

func TestWait_HandlesContention(t *testing.T) {
	t.Parallel()
	// Rate 200/s, burst 5. 10 goroutines each Wait for one token.
	// With 5 burst + refill at 200/s, 10 tokens are available within ~25ms.
	b := NewBucket(200, 5)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	const n = 10
	var wg sync.WaitGroup
	errs := make([]error, n)

	wg.Add(n)
	for i := 0; i < n; i++ {
		go func(idx int) {
			defer wg.Done()
			errs[idx] = b.Wait(ctx)
		}(i)
	}
	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Errorf("goroutine %d got error: %v", i, err)
		}
	}
}

func TestAvailable_ReturnsCorrectCount(t *testing.T) {
	t.Parallel()
	b := NewBucket(100, 10)

	// Starts full at 10.
	avail := b.Available()
	if avail < 9.9 || avail > 10.1 {
		t.Errorf("expected ~10 available, got %v", avail)
	}

	// Drain 3 tokens.
	b.Allow()
	b.Allow()
	b.Allow()

	avail = b.Available()
	if avail < 6.9 || avail > 7.5 {
		t.Errorf("expected ~7 available after draining 3, got %v", avail)
	}
}

func TestAvailable_IncludesTimeBasedRefill(t *testing.T) {
	t.Parallel()
	b := NewBucket(100, 10)

	// Drain all 10 tokens.
	for i := 0; i < 10; i++ {
		b.Allow()
	}

	// Sleep for 50ms at 100 tokens/s → expect ~5 tokens refilled.
	time.Sleep(50 * time.Millisecond)

	avail := b.Available()
	if avail < 3 || avail > 7 {
		t.Errorf("expected ~5 available after 50ms refill at 100/s, got %v", avail)
	}
}

func TestReset_RefillsToMax(t *testing.T) {
	t.Parallel()
	b := NewBucket(10, 5)

	// Drain all tokens.
	for i := 0; i < 5; i++ {
		b.Allow()
	}

	avail := b.Available()
	if avail > 1 {
		t.Errorf("expected near-zero after drain, got %v", avail)
	}

	b.Reset()

	avail = b.Available()
	if avail < 4.9 {
		t.Errorf("expected ~5 after Reset, got %v", avail)
	}
}

func TestStats_ReturnsCorrectValues(t *testing.T) {
	t.Parallel()
	b := NewBucket(42, 7)

	stats := b.Stats()
	if stats.Rate != 42 {
		t.Errorf("expected Rate 42, got %v", stats.Rate)
	}
	if stats.Max != 7 {
		t.Errorf("expected Max 7, got %v", stats.Max)
	}
	if stats.Available < 6.9 || stats.Available > 7.1 {
		t.Errorf("expected Available ~7, got %v", stats.Available)
	}

	// Consume 2 tokens.
	b.Allow()
	b.Allow()

	stats = b.Stats()
	if stats.Available < 4.9 || stats.Available > 5.5 {
		t.Errorf("expected Available ~5 after consuming 2, got %v", stats.Available)
	}
	// Rate and Max shouldn't change.
	if stats.Rate != 42 {
		t.Errorf("expected Rate 42 unchanged, got %v", stats.Rate)
	}
	if stats.Max != 7 {
		t.Errorf("expected Max 7 unchanged, got %v", stats.Max)
	}
}
