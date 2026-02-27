package chaos

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"
)

// ProgressiveDegradation simulates a service that degrades over time.
// Latency increases with each request, optionally producing errors after a threshold.
// The counter resets after resetAfter requests (simulating a service restart).
type ProgressiveDegradation struct {
	mu sync.Mutex

	// Configuration
	initialDelay   time.Duration
	delayIncrement time.Duration
	maxDelay       time.Duration
	resetAfter     int // Reset counter after N requests (0 = never)
	errorAfter     int // Start returning errors after N requests (0 = never)
	errorCode      int // Error status code (default 500)

	// State
	requestCount int64
	totalErrors  int64
	totalResets  int64
}

// ProgressiveDegradationConfig holds parsed configuration.
type ProgressiveDegradationConfig struct {
	InitialDelay   time.Duration
	DelayIncrement time.Duration
	MaxDelay       time.Duration
	ResetAfter     int
	ErrorAfter     int
	ErrorCode      int
}

// ParseProgressiveDegradationConfig extracts config from a FaultConfig map.
func ParseProgressiveDegradationConfig(cfg map[string]interface{}) ProgressiveDegradationConfig {
	initialStr := getStringOrDefault(cfg, "initialDelay", "20ms")
	initial, err := time.ParseDuration(initialStr)
	if err != nil || initial < 0 {
		initial = 20 * time.Millisecond
	}

	incrStr := getStringOrDefault(cfg, "delayIncrement", "5ms")
	incr, err := time.ParseDuration(incrStr)
	if err != nil || incr < 0 {
		incr = 5 * time.Millisecond
	}

	maxStr := getStringOrDefault(cfg, "maxDelay", "5s")
	maxD, err := time.ParseDuration(maxStr)
	if err != nil || maxD <= 0 {
		maxD = 5 * time.Second
	}

	errorCode := getIntOrDefault(cfg, "errorCode", http.StatusInternalServerError)
	if errorCode < 400 || errorCode > 599 {
		errorCode = http.StatusInternalServerError
	}

	return ProgressiveDegradationConfig{
		InitialDelay:   initial,
		DelayIncrement: incr,
		MaxDelay:       maxD,
		ResetAfter:     getIntOrDefault(cfg, "resetAfter", 0),
		ErrorAfter:     getIntOrDefault(cfg, "errorAfter", 0),
		ErrorCode:      errorCode,
	}
}

// NewProgressiveDegradation creates a new progressive degradation tracker.
func NewProgressiveDegradation(cfg ProgressiveDegradationConfig) *ProgressiveDegradation {
	return &ProgressiveDegradation{
		initialDelay:   cfg.InitialDelay,
		delayIncrement: cfg.DelayIncrement,
		maxDelay:       cfg.MaxDelay,
		resetAfter:     cfg.ResetAfter,
		errorAfter:     cfg.ErrorAfter,
		errorCode:      cfg.ErrorCode,
	}
}

// Handle processes a request through the progressive degradation tracker.
// It injects computed latency and returns true if an error was returned
// (the request should not continue to the handler).
func (pd *ProgressiveDegradation) Handle(ctx context.Context, w http.ResponseWriter) bool {
	pd.mu.Lock()
	pd.requestCount++
	count := pd.requestCount

	// Check for reset
	if pd.resetAfter > 0 && count > int64(pd.resetAfter) {
		pd.requestCount = 1
		count = 1
		pd.totalResets++
	}

	// Compute delay: initialDelay + (count-1) * delayIncrement, capped at maxDelay
	delay := pd.initialDelay + time.Duration(count-1)*pd.delayIncrement
	if delay > pd.maxDelay {
		delay = pd.maxDelay
	}

	// Check if we should return errors
	shouldError := pd.errorAfter > 0 && count > int64(pd.errorAfter)
	if shouldError {
		pd.totalErrors++
	}
	errorCode := pd.errorCode
	pd.mu.Unlock()

	// Inject the computed delay (respects context cancellation)
	if delay > 0 {
		select {
		case <-ctx.Done():
			return true // Context cancelled during delay
		case <-time.After(delay):
		}
	}

	// Return error if past the error threshold
	if shouldError {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(errorCode)
		body := fmt.Sprintf(`{"error":"%s","degradation":{"request_count":%d,"delay_ms":%d}}`,
			http.StatusText(errorCode), count, delay.Milliseconds())
		_, _ = w.Write([]byte(body))
		return true
	}

	return false
}

// Reset resets the degradation counter.
func (pd *ProgressiveDegradation) Reset() {
	pd.mu.Lock()
	defer pd.mu.Unlock()
	pd.requestCount = 0
	pd.totalErrors = 0
	pd.totalResets = 0
}

// ProgressiveDegradationStats contains statistics.
type ProgressiveDegradationStats struct {
	RequestCount   int64 `json:"requestCount"`
	CurrentDelayMs int64 `json:"currentDelayMs"`
	MaxDelayMs     int64 `json:"maxDelayMs"`
	ErrorAfter     int   `json:"errorAfter"`
	ResetAfter     int   `json:"resetAfter"`
	TotalErrors    int64 `json:"totalErrors"`
	TotalResets    int64 `json:"totalResets"`
	IsErroring     bool  `json:"isErroring"`
}

// Stats returns current statistics.
func (pd *ProgressiveDegradation) Stats() ProgressiveDegradationStats {
	pd.mu.Lock()
	defer pd.mu.Unlock()

	count := pd.requestCount
	delay := pd.initialDelay + time.Duration(count)*pd.delayIncrement
	if delay > pd.maxDelay {
		delay = pd.maxDelay
	}

	return ProgressiveDegradationStats{
		RequestCount:   count,
		CurrentDelayMs: delay.Milliseconds(),
		MaxDelayMs:     pd.maxDelay.Milliseconds(),
		ErrorAfter:     pd.errorAfter,
		ResetAfter:     pd.resetAfter,
		TotalErrors:    pd.totalErrors,
		TotalResets:    pd.totalResets,
		IsErroring:     pd.errorAfter > 0 && count > int64(pd.errorAfter),
	}
}
