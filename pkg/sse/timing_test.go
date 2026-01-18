package sse

import (
	"testing"
	"time"
)

func TestTimingScheduler_FixedDelay(t *testing.T) {
	delay := 100
	config := &TimingConfig{
		FixedDelay: &delay,
	}
	scheduler := NewTimingScheduler(config)

	// Multiple calls should return the same fixed delay
	for i := 0; i < 5; i++ {
		d := scheduler.NextDelay(i, nil)
		if d != 100*time.Millisecond {
			t.Errorf("event %d: expected 100ms, got %v", i, d)
		}
	}
}

func TestTimingScheduler_RandomDelay(t *testing.T) {
	config := &TimingConfig{
		RandomDelay: &RandomDelayConfig{
			Min: 50,
			Max: 150,
		},
	}
	scheduler := NewTimingScheduler(config)

	// Test that delays are within range
	for i := 0; i < 100; i++ {
		d := scheduler.NextDelay(i, nil)
		if d < 50*time.Millisecond || d > 150*time.Millisecond {
			t.Errorf("delay %v outside range [50ms, 150ms]", d)
		}
	}

	// Test that we get some variation (not all the same)
	seen := make(map[time.Duration]bool)
	for i := 0; i < 50; i++ {
		d := scheduler.NextDelay(i, nil)
		seen[d] = true
	}
	if len(seen) < 2 {
		t.Error("expected variation in random delays")
	}
}

func TestTimingScheduler_RandomDelay_MinEqualsMax(t *testing.T) {
	config := &TimingConfig{
		RandomDelay: &RandomDelayConfig{
			Min: 100,
			Max: 100,
		},
	}
	scheduler := NewTimingScheduler(config)

	d := scheduler.NextDelay(0, nil)
	if d != 100*time.Millisecond {
		t.Errorf("expected 100ms when min==max, got %v", d)
	}
}

func TestTimingScheduler_PerEventDelays(t *testing.T) {
	config := &TimingConfig{
		PerEventDelays: []int{10, 20, 30, 40, 50},
	}
	scheduler := NewTimingScheduler(config)

	expected := []time.Duration{
		10 * time.Millisecond,
		20 * time.Millisecond,
		30 * time.Millisecond,
		40 * time.Millisecond,
		50 * time.Millisecond,
	}

	for i, exp := range expected {
		d := scheduler.NextDelay(i, nil)
		if d != exp {
			t.Errorf("event %d: expected %v, got %v", i, exp, d)
		}
	}

	// Events beyond the array should return 0 (no delay)
	d := scheduler.NextDelay(10, nil)
	if d != 0 {
		t.Errorf("expected 0 for index beyond array, got %v", d)
	}
}

func TestTimingScheduler_BurstMode(t *testing.T) {
	config := &TimingConfig{
		Burst: &BurstConfig{
			Count:    3,
			Interval: 10,
			Pause:    100,
		},
	}
	scheduler := NewTimingScheduler(config)

	// First 3 events should have burst interval
	for i := 0; i < 3; i++ {
		d := scheduler.NextDelay(i, nil)
		if i < 2 {
			// First two should be burst interval
			if d != 10*time.Millisecond {
				t.Errorf("event %d: expected 10ms (burst interval), got %v", i, d)
			}
		} else {
			// Third triggers pause
			if d != 100*time.Millisecond {
				t.Errorf("event %d: expected 100ms (pause), got %v", i, d)
			}
		}
	}

	// After pause, should reset to burst interval
	d := scheduler.NextDelay(3, nil)
	if d != 10*time.Millisecond {
		t.Errorf("after pause: expected 10ms, got %v", d)
	}
}

func TestTimingScheduler_OverrideDelay(t *testing.T) {
	delay := 100
	config := &TimingConfig{
		FixedDelay: &delay,
	}
	scheduler := NewTimingScheduler(config)

	// Override should take precedence
	override := 500
	d := scheduler.NextDelay(0, &override)
	if d != 500*time.Millisecond {
		t.Errorf("expected 500ms override, got %v", d)
	}

	// Without override, should use fixed delay
	d = scheduler.NextDelay(1, nil)
	if d != 100*time.Millisecond {
		t.Errorf("expected 100ms fixed, got %v", d)
	}
}

func TestTimingScheduler_NoDelay(t *testing.T) {
	config := &TimingConfig{}
	scheduler := NewTimingScheduler(config)

	d := scheduler.NextDelay(0, nil)
	if d != 0 {
		t.Errorf("expected 0 delay, got %v", d)
	}
}

func TestTimingScheduler_Reset(t *testing.T) {
	config := &TimingConfig{
		Burst: &BurstConfig{
			Count:    2,
			Interval: 10,
			Pause:    100,
		},
	}
	scheduler := NewTimingScheduler(config)

	// Trigger burst pause
	scheduler.NextDelay(0, nil)
	scheduler.NextDelay(1, nil) // This should trigger pause

	if !scheduler.IsBurstPaused() {
		t.Error("expected burst to be paused")
	}

	// Reset
	scheduler.Reset()

	if scheduler.IsBurstPaused() {
		t.Error("expected burst pause to be cleared after reset")
	}
	if scheduler.BurstPosition() != 0 {
		t.Errorf("expected burst position 0 after reset, got %d", scheduler.BurstPosition())
	}
}

func TestTimingScheduler_IsBurstPaused(t *testing.T) {
	config := &TimingConfig{
		Burst: &BurstConfig{
			Count:    2,
			Interval: 10,
			Pause:    100,
		},
	}
	scheduler := NewTimingScheduler(config)

	if scheduler.IsBurstPaused() {
		t.Error("should not be paused initially")
	}

	scheduler.NextDelay(0, nil)
	if scheduler.IsBurstPaused() {
		t.Error("should not be paused after first event")
	}

	scheduler.NextDelay(1, nil) // Triggers pause
	if !scheduler.IsBurstPaused() {
		t.Error("should be paused after burst count reached")
	}
}

func TestTimingScheduler_BurstPosition(t *testing.T) {
	config := &TimingConfig{
		Burst: &BurstConfig{
			Count:    3,
			Interval: 10,
			Pause:    100,
		},
	}
	scheduler := NewTimingScheduler(config)

	if pos := scheduler.BurstPosition(); pos != 0 {
		t.Errorf("expected initial position 0, got %d", pos)
	}

	scheduler.NextDelay(0, nil)
	if pos := scheduler.BurstPosition(); pos != 1 {
		t.Errorf("expected position 1, got %d", pos)
	}

	scheduler.NextDelay(1, nil)
	if pos := scheduler.BurstPosition(); pos != 2 {
		t.Errorf("expected position 2, got %d", pos)
	}

	scheduler.NextDelay(2, nil) // Triggers pause, resets counter
	if pos := scheduler.BurstPosition(); pos != 0 {
		t.Errorf("expected position 0 after pause, got %d", pos)
	}
}

func TestDefaultTimingConfig(t *testing.T) {
	config := DefaultTimingConfig()
	if config.FixedDelay == nil || *config.FixedDelay != 100 {
		t.Error("expected default fixed delay of 100ms")
	}
}

func TestFastTimingConfig(t *testing.T) {
	config := FastTimingConfig()
	if config.FixedDelay == nil || *config.FixedDelay != 10 {
		t.Error("expected fast fixed delay of 10ms")
	}
}

func TestSlowTimingConfig(t *testing.T) {
	config := SlowTimingConfig()
	if config.FixedDelay == nil || *config.FixedDelay != 1000 {
		t.Error("expected slow fixed delay of 1000ms")
	}
}

func TestBurstTimingConfig(t *testing.T) {
	config := BurstTimingConfig(5, 20, 500)
	if config.Burst == nil {
		t.Fatal("expected burst config")
	}
	if config.Burst.Count != 5 {
		t.Errorf("expected count 5, got %d", config.Burst.Count)
	}
	if config.Burst.Interval != 20 {
		t.Errorf("expected interval 20, got %d", config.Burst.Interval)
	}
	if config.Burst.Pause != 500 {
		t.Errorf("expected pause 500, got %d", config.Burst.Pause)
	}
}

func TestRandomTimingConfig(t *testing.T) {
	config := RandomTimingConfig(100, 500)
	if config.RandomDelay == nil {
		t.Fatal("expected random delay config")
	}
	if config.RandomDelay.Min != 100 {
		t.Errorf("expected min 100, got %d", config.RandomDelay.Min)
	}
	if config.RandomDelay.Max != 500 {
		t.Errorf("expected max 500, got %d", config.RandomDelay.Max)
	}
}

func TestPerEventTimingConfig(t *testing.T) {
	delays := []int{10, 20, 30}
	config := PerEventTimingConfig(delays)
	if len(config.PerEventDelays) != 3 {
		t.Errorf("expected 3 delays, got %d", len(config.PerEventDelays))
	}
	for i, exp := range delays {
		if config.PerEventDelays[i] != exp {
			t.Errorf("delay %d: expected %d, got %d", i, exp, config.PerEventDelays[i])
		}
	}
}

func TestWithInitialDelay(t *testing.T) {
	config := DefaultTimingConfig()
	config = WithInitialDelay(config, 500)
	if config.InitialDelay != 500 {
		t.Errorf("expected initial delay 500, got %d", config.InitialDelay)
	}
}

func TestTimingScheduler_Priority(t *testing.T) {
	// Test that override > perEvent > burst > random > fixed
	delay := 1000
	config := &TimingConfig{
		FixedDelay:     &delay,
		PerEventDelays: []int{100, 200},
	}
	scheduler := NewTimingScheduler(config)

	// Per-event should take precedence over fixed
	d := scheduler.NextDelay(0, nil)
	if d != 100*time.Millisecond {
		t.Errorf("expected per-event 100ms, got %v", d)
	}

	// Override should take precedence over per-event
	override := 50
	d = scheduler.NextDelay(0, &override)
	if d != 50*time.Millisecond {
		t.Errorf("expected override 50ms, got %v", d)
	}

	// Beyond per-event array, fall back to fixed
	d = scheduler.NextDelay(5, nil)
	if d != 1000*time.Millisecond {
		t.Errorf("expected fixed 1000ms, got %v", d)
	}
}
