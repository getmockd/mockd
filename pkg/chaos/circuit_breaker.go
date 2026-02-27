package chaos

import (
	"fmt"
	"math/rand"
	"net/http"
	"strconv"
	"sync"
	"time"
)

// CircuitState represents the current state of a circuit breaker.
type CircuitState int

const (
	// CircuitClosed is the normal operating state — requests pass through.
	CircuitClosed CircuitState = iota
	// CircuitOpen is the tripped state — all requests are rejected.
	CircuitOpen
	// CircuitHalfOpen is the recovery-testing state — some requests pass through.
	CircuitHalfOpen
)

// String returns the human-readable state name.
func (s CircuitState) String() string {
	switch s {
	case CircuitClosed:
		return "closed"
	case CircuitOpen:
		return "open"
	case CircuitHalfOpen:
		return "half_open"
	default:
		return "unknown"
	}
}

// CircuitBreakerConfig holds parsed configuration for a circuit_breaker fault.
type CircuitBreakerConfig struct {
	// Closed state: synthetic error rate to simulate upstream failures
	ClosedErrorRate float64 // 0.0-1.0 — probability of injecting an error in closed state

	// Thresholds for state transitions
	FailureThreshold int           // Consecutive failures before tripping (default 5)
	SuccessThreshold int           // Consecutive successes in half-open before closing (default 3)
	OpenDuration     time.Duration // How long to stay open before half-open (default 30s)

	// Trip-after: deterministic trip after N requests (0 = disabled, use error-based)
	TripAfter int

	// Open state response
	OpenStatusCode int    // Status code when open (default 503)
	OpenBody       string // Body when open

	// Half-open state
	HalfOpenErrorRate float64 // Error rate in half-open (default 0.5)
}

// ParseCircuitBreakerConfig extracts circuit_breaker config from a FaultConfig map.
func ParseCircuitBreakerConfig(cfg map[string]interface{}) CircuitBreakerConfig {
	openDurStr := getStringOrDefault(cfg, "openDuration", "30s")
	openDur, err := time.ParseDuration(openDurStr)
	if err != nil || openDur <= 0 {
		openDur = 30 * time.Second
	}

	openStatus := getIntOrDefault(cfg, "openStatusCode", http.StatusServiceUnavailable)
	if openStatus < 400 || openStatus > 599 {
		openStatus = http.StatusServiceUnavailable
	}

	failThreshold := getIntOrDefault(cfg, "failureThreshold", 5)
	if failThreshold < 1 {
		failThreshold = 5
	}

	successThreshold := getIntOrDefault(cfg, "successThreshold", 3)
	if successThreshold < 1 {
		successThreshold = 3
	}

	closedErrorRate := getFloat64OrDefault(cfg, "closedErrorRate", 0.0)
	halfOpenErrorRate := getFloat64OrDefault(cfg, "halfOpenErrorRate", 0.5)

	return CircuitBreakerConfig{
		ClosedErrorRate:   closedErrorRate,
		FailureThreshold:  failThreshold,
		SuccessThreshold:  successThreshold,
		OpenDuration:      openDur,
		TripAfter:         getIntOrDefault(cfg, "tripAfter", 0),
		OpenStatusCode:    openStatus,
		OpenBody:          getStringOrDefault(cfg, "openBody", ""),
		HalfOpenErrorRate: halfOpenErrorRate,
	}
}

// CircuitBreaker implements a circuit breaker state machine for chaos simulation.
//
// States:
//   - CLOSED: Normal operation. Requests pass through. Synthetic errors at closedErrorRate
//     count toward failureThreshold. Or use tripAfter for deterministic trip.
//   - OPEN: All requests rejected with openStatusCode + Retry-After header.
//     After openDuration elapses, transitions to HALF_OPEN.
//   - HALF_OPEN: Testing recovery. Requests pass through at (1-halfOpenErrorRate).
//     Successes count toward successThreshold (→ CLOSED). Any failure → OPEN.
type CircuitBreaker struct {
	mu     sync.Mutex
	config CircuitBreakerConfig
	rng    *rand.Rand

	// State
	state                CircuitState
	consecutiveFailures  int
	consecutiveSuccesses int
	totalRequests        int64
	openedAt             time.Time

	// Stats
	totalTrips    int64
	totalRejected int64
	totalPassed   int64
	totalHalfOpen int64
	stateChanges  int64
}

// NewCircuitBreaker creates a new circuit breaker with the given config.
func NewCircuitBreaker(cfg CircuitBreakerConfig, rng *rand.Rand) *CircuitBreaker {
	return &CircuitBreaker{
		config: cfg,
		rng:    rng,
		state:  CircuitClosed,
	}
}

// Handle processes a request through the circuit breaker.
// Returns true if the request was rejected (response written), false if passed through.
func (cb *CircuitBreaker) Handle(w http.ResponseWriter) bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.totalRequests++

	// Check for time-based transition: OPEN → HALF_OPEN
	cb.maybeTransition()

	switch cb.state {
	case CircuitClosed:
		return cb.handleClosed(w)
	case CircuitOpen:
		return cb.handleOpen(w)
	case CircuitHalfOpen:
		return cb.handleHalfOpen(w)
	}

	return false
}

// handleClosed processes requests in closed (normal) state.
func (cb *CircuitBreaker) handleClosed(w http.ResponseWriter) bool {
	// Deterministic trip: after N total requests
	if cb.config.TripAfter > 0 && cb.totalRequests >= int64(cb.config.TripAfter) {
		cb.tripToOpen()
		cb.writeOpenResponse(w)
		return true
	}

	// Probabilistic error injection
	if cb.config.ClosedErrorRate > 0 && cb.rng.Float64() < cb.config.ClosedErrorRate {
		// Synthetic failure — count toward threshold
		cb.consecutiveFailures++
		if cb.consecutiveFailures >= cb.config.FailureThreshold {
			cb.tripToOpen()
			cb.writeOpenResponse(w)
			return true
		}
		// Inject error but don't trip yet
		cb.totalRejected++
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return true
	}

	// Success — reset failure counter
	cb.consecutiveFailures = 0
	cb.totalPassed++
	return false
}

// handleOpen processes requests in open (tripped) state.
func (cb *CircuitBreaker) handleOpen(w http.ResponseWriter) bool {
	cb.totalRejected++
	cb.writeOpenResponse(w)
	return true
}

// handleHalfOpen processes requests in half-open (recovery testing) state.
func (cb *CircuitBreaker) handleHalfOpen(w http.ResponseWriter) bool {
	cb.totalHalfOpen++

	if cb.rng.Float64() < cb.config.HalfOpenErrorRate {
		// Failed in half-open — trip back to open
		cb.tripToOpen()
		cb.writeOpenResponse(w)
		return true
	}

	// Success in half-open
	cb.consecutiveSuccesses++
	if cb.consecutiveSuccesses >= cb.config.SuccessThreshold {
		cb.transitionToClosed()
	}
	cb.totalPassed++
	return false
}

// maybeTransition checks if a time-based state transition should occur.
func (cb *CircuitBreaker) maybeTransition() {
	if cb.state == CircuitOpen && !cb.openedAt.IsZero() {
		if time.Since(cb.openedAt) >= cb.config.OpenDuration {
			cb.state = CircuitHalfOpen
			cb.consecutiveSuccesses = 0
			cb.stateChanges++
		}
	}
}

// tripToOpen transitions to the OPEN state.
func (cb *CircuitBreaker) tripToOpen() {
	cb.state = CircuitOpen
	cb.openedAt = time.Now()
	cb.consecutiveFailures = 0
	cb.consecutiveSuccesses = 0
	cb.totalTrips++
	cb.stateChanges++
}

// transitionToClosed transitions to the CLOSED state.
func (cb *CircuitBreaker) transitionToClosed() {
	cb.state = CircuitClosed
	cb.consecutiveFailures = 0
	cb.consecutiveSuccesses = 0
	cb.totalRequests = 0 // Reset for tripAfter counting
	cb.stateChanges++
}

// writeOpenResponse writes the circuit-open error response.
func (cb *CircuitBreaker) writeOpenResponse(w http.ResponseWriter) {
	retryAfterSecs := int(cb.config.OpenDuration.Seconds())
	if retryAfterSecs < 1 {
		retryAfterSecs = 1
	}

	w.Header().Set("Retry-After", strconv.Itoa(retryAfterSecs))
	w.Header().Set("X-Circuit-State", cb.state.String())
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(cb.config.OpenStatusCode)

	body := cb.config.OpenBody
	if body == "" {
		body = fmt.Sprintf(`{"error":"circuit breaker open","state":"%s","retry_after":%d}`,
			cb.state.String(), retryAfterSecs)
	}
	_, _ = w.Write([]byte(body))
}

// Trip forces the circuit breaker to the OPEN state (for admin API).
func (cb *CircuitBreaker) Trip() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.tripToOpen()
}

// Reset forces the circuit breaker to the CLOSED state (for admin API).
func (cb *CircuitBreaker) Reset() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.transitionToClosed()
}

// State returns the current circuit state.
func (cb *CircuitBreaker) State() CircuitState {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.maybeTransition()
	return cb.state
}

// CircuitBreakerStats contains statistics for a circuit breaker.
type CircuitBreakerStats struct {
	State                string `json:"state"`
	ConsecutiveFailures  int    `json:"consecutiveFailures"`
	ConsecutiveSuccesses int    `json:"consecutiveSuccesses"`
	TotalRequests        int64  `json:"totalRequests"`
	TotalTrips           int64  `json:"totalTrips"`
	TotalRejected        int64  `json:"totalRejected"`
	TotalPassed          int64  `json:"totalPassed"`
	TotalHalfOpen        int64  `json:"totalHalfOpen"`
	StateChanges         int64  `json:"stateChanges"`
	OpenedAt             string `json:"openedAt,omitempty"`
}

// Stats returns current statistics.
func (cb *CircuitBreaker) Stats() CircuitBreakerStats {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.maybeTransition()

	stats := CircuitBreakerStats{
		State:                cb.state.String(),
		ConsecutiveFailures:  cb.consecutiveFailures,
		ConsecutiveSuccesses: cb.consecutiveSuccesses,
		TotalRequests:        cb.totalRequests,
		TotalTrips:           cb.totalTrips,
		TotalRejected:        cb.totalRejected,
		TotalPassed:          cb.totalPassed,
		TotalHalfOpen:        cb.totalHalfOpen,
		StateChanges:         cb.stateChanges,
	}
	if !cb.openedAt.IsZero() {
		stats.OpenedAt = cb.openedAt.Format(time.RFC3339)
	}
	return stats
}
