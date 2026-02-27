package chaos

import (
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"time"
)

// RetryAfterTracker manages the state for a retry_after fault.
// It returns 429/503 with a Retry-After header, then recovers after the
// specified duration elapses. The cycle repeats for each subsequent trigger.
type RetryAfterTracker struct {
	mu sync.Mutex

	// Configuration
	statusCode int           // 429 or 503 (default 429)
	retryAfter time.Duration // How long until recovery
	body       string        // Response body when rate-limited

	// State
	limitedAt    time.Time // When the current rate-limit window started
	isLimited    bool      // Whether currently in rate-limited state
	totalLimited int64     // Total times rate-limiting was triggered
	totalPassed  int64     // Total requests that passed through
}

// RetryAfterConfig holds parsed configuration for a retry_after fault.
type RetryAfterConfig struct {
	StatusCode int           // 429 (default) or 503
	RetryAfter time.Duration // Recovery duration
	Body       string        // Response body when limited
}

// ParseRetryAfterConfig extracts retry_after config from a FaultConfig map.
func ParseRetryAfterConfig(cfg map[string]interface{}) RetryAfterConfig {
	statusCode := getIntOrDefault(cfg, "statusCode", http.StatusTooManyRequests)
	if statusCode != http.StatusTooManyRequests && statusCode != http.StatusServiceUnavailable {
		statusCode = http.StatusTooManyRequests
	}

	retryAfterStr := getStringOrDefault(cfg, "retryAfter", "30s")
	retryAfter, err := time.ParseDuration(retryAfterStr)
	if err != nil || retryAfter <= 0 {
		retryAfter = 30 * time.Second
	}

	body := getStringOrDefault(cfg, "body", "")

	return RetryAfterConfig{
		StatusCode: statusCode,
		RetryAfter: retryAfter,
		Body:       body,
	}
}

// NewRetryAfterTracker creates a new retry-after tracker.
func NewRetryAfterTracker(cfg RetryAfterConfig) *RetryAfterTracker {
	return &RetryAfterTracker{
		statusCode: cfg.StatusCode,
		retryAfter: cfg.RetryAfter,
		body:       cfg.Body,
	}
}

// Handle processes a request through the retry-after tracker.
// Returns true if the request was rate-limited (response written), false if passed through.
func (t *RetryAfterTracker) Handle(w http.ResponseWriter) bool {
	t.mu.Lock()
	defer t.mu.Unlock()

	now := time.Now()

	// If currently limited, check if recovery period has elapsed
	if t.isLimited {
		if now.After(t.limitedAt.Add(t.retryAfter)) {
			// Recovery period elapsed — let this request through
			t.isLimited = false
			t.totalPassed++
			return false
		}
		// Still rate-limited — return error with Retry-After
		t.writeRateLimitResponse(w, now)
		return true
	}

	// Not yet limited — trigger rate limiting now
	t.isLimited = true
	t.limitedAt = now
	t.totalLimited++
	t.writeRateLimitResponse(w, now)
	return true
}

// writeRateLimitResponse writes the 429/503 response with Retry-After header.
func (t *RetryAfterTracker) writeRateLimitResponse(w http.ResponseWriter, now time.Time) {
	remaining := t.retryAfter - now.Sub(t.limitedAt)
	if remaining < 0 {
		remaining = 0
	}
	retryAfterSecs := int(remaining.Seconds())
	if retryAfterSecs < 1 {
		retryAfterSecs = 1
	}

	w.Header().Set("Retry-After", strconv.Itoa(retryAfterSecs))
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(t.statusCode)

	body := t.body
	if body == "" {
		body = fmt.Sprintf(`{"error":"%s","retry_after":%d}`,
			http.StatusText(t.statusCode), retryAfterSecs)
	}
	_, _ = w.Write([]byte(body))
}

// Reset resets the tracker to its initial state.
func (t *RetryAfterTracker) Reset() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.isLimited = false
	t.limitedAt = time.Time{}
	t.totalLimited = 0
	t.totalPassed = 0
}

// RetryAfterStats contains statistics for a retry_after fault.
type RetryAfterStats struct {
	IsLimited    bool   `json:"isLimited"`
	StatusCode   int    `json:"statusCode"`
	RetryAfterMs int64  `json:"retryAfterMs"`
	TotalLimited int64  `json:"totalLimited"`
	TotalPassed  int64  `json:"totalPassed"`
	LimitedAt    string `json:"limitedAt,omitempty"`
}

// Stats returns current statistics.
func (t *RetryAfterTracker) Stats() RetryAfterStats {
	t.mu.Lock()
	defer t.mu.Unlock()

	stats := RetryAfterStats{
		IsLimited:    t.isLimited,
		StatusCode:   t.statusCode,
		RetryAfterMs: t.retryAfter.Milliseconds(),
		TotalLimited: t.totalLimited,
		TotalPassed:  t.totalPassed,
	}
	if t.isLimited {
		stats.LimitedAt = t.limitedAt.Format(time.RFC3339)
	}
	return stats
}
