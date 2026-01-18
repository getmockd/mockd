package sse

import (
	"math/rand"
	"sync"
	"time"
)

// TimingScheduler manages event timing and delays.
type TimingScheduler struct {
	config       *TimingConfig
	burstCounter int
	burstPaused  bool
	rng          *rand.Rand
	mu           sync.Mutex
}

// NewTimingScheduler creates a new timing scheduler.
func NewTimingScheduler(config *TimingConfig) *TimingScheduler {
	return &TimingScheduler{
		config: config,
		rng:    rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// NextDelay returns the delay before the next event.
// eventIndex is the 0-based index of the event.
// overrideDelay is an optional per-event delay override in milliseconds.
func (s *TimingScheduler) NextDelay(eventIndex int, overrideDelay *int) time.Duration {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Per-event override takes highest priority
	if overrideDelay != nil {
		return time.Duration(*overrideDelay) * time.Millisecond
	}

	// Check per-event delays array
	if s.config.PerEventDelays != nil && eventIndex < len(s.config.PerEventDelays) {
		return time.Duration(s.config.PerEventDelays[eventIndex]) * time.Millisecond
	}

	// Check burst mode
	if s.config.Burst != nil {
		return s.nextBurstDelay()
	}

	// Check random delay
	if s.config.RandomDelay != nil {
		return s.randomDelay()
	}

	// Fixed delay (default)
	if s.config.FixedDelay != nil {
		return time.Duration(*s.config.FixedDelay) * time.Millisecond
	}

	// Default: no delay
	return 0
}

// nextBurstDelay calculates the delay for burst mode.
func (s *TimingScheduler) nextBurstDelay() time.Duration {
	burst := s.config.Burst
	if burst == nil {
		return 0
	}

	s.burstCounter++

	// Check if we've reached the burst count
	if s.burstCounter >= burst.Count {
		s.burstCounter = 0
		s.burstPaused = true
		return time.Duration(burst.Pause) * time.Millisecond
	}

	// Within a burst
	s.burstPaused = false
	return time.Duration(burst.Interval) * time.Millisecond
}

// randomDelay calculates a random delay within the configured range.
func (s *TimingScheduler) randomDelay() time.Duration {
	rd := s.config.RandomDelay
	if rd == nil {
		return 0
	}

	if rd.Max <= rd.Min {
		return time.Duration(rd.Min) * time.Millisecond
	}

	delay := rd.Min + s.rng.Intn(rd.Max-rd.Min+1)
	return time.Duration(delay) * time.Millisecond
}

// Reset resets the scheduler state.
func (s *TimingScheduler) Reset() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.burstCounter = 0
	s.burstPaused = false
}

// IsBurstPaused returns whether the scheduler is in a burst pause.
func (s *TimingScheduler) IsBurstPaused() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.burstPaused
}

// BurstPosition returns the current position within a burst.
func (s *TimingScheduler) BurstPosition() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.burstCounter
}

// DefaultTimingConfig returns a default timing configuration.
func DefaultTimingConfig() *TimingConfig {
	defaultDelay := 100 // 100ms default
	return &TimingConfig{
		FixedDelay: &defaultDelay,
	}
}

// FastTimingConfig returns a fast timing configuration (10ms).
func FastTimingConfig() *TimingConfig {
	fastDelay := 10
	return &TimingConfig{
		FixedDelay: &fastDelay,
	}
}

// SlowTimingConfig returns a slow timing configuration (1s).
func SlowTimingConfig() *TimingConfig {
	slowDelay := 1000
	return &TimingConfig{
		FixedDelay: &slowDelay,
	}
}

// BurstTimingConfig returns a burst mode timing configuration.
func BurstTimingConfig(burstCount, burstInterval, pauseInterval int) *TimingConfig {
	return &TimingConfig{
		Burst: &BurstConfig{
			Count:    burstCount,
			Interval: burstInterval,
			Pause:    pauseInterval,
		},
	}
}

// RandomTimingConfig returns a random delay timing configuration.
func RandomTimingConfig(min, max int) *TimingConfig {
	return &TimingConfig{
		RandomDelay: &RandomDelayConfig{
			Min: min,
			Max: max,
		},
	}
}

// PerEventTimingConfig returns a per-event timing configuration.
func PerEventTimingConfig(delays []int) *TimingConfig {
	return &TimingConfig{
		PerEventDelays: delays,
	}
}

// WithInitialDelay adds an initial delay to a timing config.
func WithInitialDelay(config *TimingConfig, delayMs int) *TimingConfig {
	config.InitialDelay = delayMs
	return config
}
