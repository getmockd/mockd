package chaos

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"net/http"
	"regexp"
	"sync"
	"time"
)

// Injector injects chaos into HTTP responses
type Injector struct {
	config *ChaosConfig
	rules  []*compiledRule
	rng    *rand.Rand
	mu     sync.Mutex
	stats  *ChaosStats

	// Stateful fault managers (keyed by "ruleIdx:faultIdx")
	circuitBreakers map[string]*CircuitBreaker
	retryTrackers   map[string]*RetryAfterTracker
	progressives    map[string]*ProgressiveDegradation
}

type compiledRule struct {
	pattern *regexp.Regexp
	methods map[string]bool
	faults  []FaultConfig
	prob    float64
}

// NewInjector creates a chaos injector from configuration
func NewInjector(config *ChaosConfig) (*Injector, error) {
	if config == nil {
		return nil, errors.New("chaos config is required")
	}

	// Clamp probability/rate values to [0.0, 1.0]
	config.Clamp()

	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	i := &Injector{
		config:          config,
		rng:             rng,
		stats:           NewChaosStats(),
		circuitBreakers: make(map[string]*CircuitBreaker),
		retryTrackers:   make(map[string]*RetryAfterTracker),
		progressives:    make(map[string]*ProgressiveDegradation),
	}

	// Compile rules and initialize stateful fault managers
	for ruleIdx, rule := range config.Rules {
		compiled, err := compileRule(rule)
		if err != nil {
			return nil, fmt.Errorf("failed to compile rule for pattern %q: %w", rule.PathPattern, err)
		}
		i.rules = append(i.rules, compiled)

		// Create stateful fault managers for this rule
		for faultIdx, fault := range rule.Faults {
			key := statefulFaultKey(ruleIdx, faultIdx)
			switch fault.Type { //nolint:exhaustive // only stateful types need initialization
			case FaultCircuitBreaker:
				cfg := ParseCircuitBreakerConfig(fault.Config)
				i.circuitBreakers[key] = NewCircuitBreaker(cfg, rng)
			case FaultRetryAfter:
				cfg := ParseRetryAfterConfig(fault.Config)
				i.retryTrackers[key] = NewRetryAfterTracker(cfg)
			case FaultProgressiveDegradation:
				cfg := ParseProgressiveDegradationConfig(fault.Config)
				i.progressives[key] = NewProgressiveDegradation(cfg)
			}
		}
	}

	return i, nil
}

func compileRule(rule ChaosRule) (*compiledRule, error) {
	pattern, err := regexp.Compile(rule.PathPattern)
	if err != nil {
		return nil, err
	}

	methods := make(map[string]bool)
	for _, m := range rule.Methods {
		methods[m] = true
	}

	prob := rule.Probability
	if prob < 0 {
		prob = 1.0 // Default to always apply if probability not set (negative = unset)
	}
	// prob == 0 means "disabled" — rule matches but never fires

	return &compiledRule{
		pattern: pattern,
		methods: methods,
		faults:  rule.Faults,
		prob:    prob,
	}, nil
}

// IsEnabled returns whether chaos injection is enabled
func (i *Injector) IsEnabled() bool {
	return i.config != nil && i.config.Enabled
}

// ShouldInject determines if chaos should be injected for a request
// Returns the list of faults to apply
func (i *Injector) ShouldInject(r *http.Request) []FaultConfig {
	if !i.IsEnabled() {
		return nil
	}

	i.mu.Lock()
	defer i.mu.Unlock()

	i.stats.TotalRequests++

	var faults []FaultConfig
	pathRuleMatched := false

	// Check path-specific rules first
	for ruleIdx, rule := range i.rules {
		if !rule.pattern.MatchString(r.URL.Path) {
			continue
		}

		// Check method filter
		if len(rule.methods) > 0 && !rule.methods[r.Method] {
			continue
		}

		// A per-path rule matched this request. Even if the probability
		// roll below fails, we must NOT fall back to global rules —
		// the per-path rule preempts global config for matching requests.
		pathRuleMatched = true

		// Check rule probability
		if i.rng.Float64() > rule.prob {
			continue
		}

		// Check each fault's probability
		for faultIdx, fault := range rule.faults {
			// Stateful faults always pass through — the state machine decides
			if isStatefulFault(fault.Type) {
				key := statefulFaultKey(ruleIdx, faultIdx)
				faultCopy := fault
				if faultCopy.Config == nil {
					faultCopy.Config = make(map[string]interface{})
				}
				faultCopy.Config["_stateKey"] = key
				faults = append(faults, faultCopy)
				i.stats.InjectedFaults++
				i.stats.FaultsByType[fault.Type]++
				continue
			}

			if i.rng.Float64() <= fault.Probability {
				faults = append(faults, fault)
				i.stats.InjectedFaults++
				i.stats.FaultsByType[fault.Type]++
			}
		}
	}

	// Apply global rules ONLY when no path-specific rule matched.
	// If a per-path rule matched but its probability roll failed,
	// we intentionally skip global rules — the per-path rule takes precedence.
	if !pathRuleMatched && i.config.GlobalRules != nil {
		faults = i.applyGlobalRules()
	}

	return faults
}

func (i *Injector) applyGlobalRules() []FaultConfig {
	var faults []FaultConfig
	global := i.config.GlobalRules

	if global.Latency != nil && i.rng.Float64() <= global.Latency.Probability {
		faults = append(faults, FaultConfig{
			Type:        FaultLatency,
			Probability: global.Latency.Probability,
			Config: map[string]interface{}{
				"min": global.Latency.Min,
				"max": global.Latency.Max,
			},
		})
		i.stats.InjectedFaults++
		i.stats.FaultsByType[FaultLatency]++
		i.stats.LatencyInjected++
	}

	if global.ErrorRate != nil && i.rng.Float64() <= global.ErrorRate.Probability {
		faults = append(faults, FaultConfig{
			Type:        FaultError,
			Probability: global.ErrorRate.Probability,
			Config: map[string]interface{}{
				"statusCodes": global.ErrorRate.StatusCodes,
				"defaultCode": global.ErrorRate.DefaultCode,
			},
		})
		i.stats.InjectedFaults++
		i.stats.FaultsByType[FaultError]++
		i.stats.ErrorsInjected++
	}

	if global.Bandwidth != nil && i.rng.Float64() <= global.Bandwidth.Probability {
		faults = append(faults, FaultConfig{
			Type:        FaultSlowBody,
			Probability: global.Bandwidth.Probability,
			Config: map[string]interface{}{
				"bytesPerSecond": global.Bandwidth.BytesPerSecond,
			},
		})
		i.stats.InjectedFaults++
		i.stats.FaultsByType[FaultSlowBody]++
	}

	return faults
}

// InjectLatency injects latency fault by sleeping for a random duration
func (i *Injector) InjectLatency(ctx context.Context, fault *LatencyFault) error {
	if fault == nil {
		return nil
	}

	minDur, err := time.ParseDuration(fault.Min)
	if err != nil {
		return fmt.Errorf("invalid min duration %q: %w", fault.Min, err)
	}

	maxDur, err := time.ParseDuration(fault.Max)
	if err != nil {
		return fmt.Errorf("invalid max duration %q: %w", fault.Max, err)
	}

	if minDur > maxDur {
		minDur, maxDur = maxDur, minDur
	}

	// Calculate random delay between min and max
	delay := minDur
	if maxDur > minDur {
		i.mu.Lock()
		delay = minDur + time.Duration(i.rng.Int63n(int64(maxDur-minDur)))
		i.mu.Unlock()
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(delay):
		return nil
	}
}

// getStringOrDefault extracts a string value from a map, returning defaultVal if not found or not a string.
func getStringOrDefault(m map[string]interface{}, key, defaultVal string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return defaultVal
}

// getIntOrDefault extracts an int value from a map, handling both int and float64 types.
// Returns defaultVal if not found or not a numeric type.
func getIntOrDefault(m map[string]interface{}, key string, defaultVal int) int {
	if v, ok := m[key].(int); ok {
		return v
	}
	if v, ok := m[key].(float64); ok {
		return int(v)
	}
	return defaultVal
}

// getFloat64OrDefault extracts a float64 value from a map.
// Returns defaultVal if not found or not a numeric type.
func getFloat64OrDefault(m map[string]interface{}, key string, defaultVal float64) float64 {
	if v, ok := m[key].(float64); ok {
		return v
	}
	if v, ok := m[key].(int); ok {
		return float64(v)
	}
	return defaultVal
}

// getIntSlice extracts an int slice from a map, handling []interface{} and []int types.
func getIntSlice(m map[string]interface{}, key string) []int {
	if v, ok := m[key].([]int); ok {
		return v
	}
	if v, ok := m[key].([]interface{}); ok {
		var result []int
		for _, item := range v {
			if c, ok := item.(int); ok {
				result = append(result, c)
			} else if c, ok := item.(float64); ok {
				result = append(result, int(c))
			}
		}
		return result
	}
	return nil
}

// statefulFaultKey generates a unique key for a stateful fault within a rule.
func statefulFaultKey(ruleIdx, faultIdx int) string {
	return fmt.Sprintf("%d:%d", ruleIdx, faultIdx)
}

// InjectLatencyFromConfig injects latency from a FaultConfig
func (i *Injector) InjectLatencyFromConfig(ctx context.Context, config map[string]interface{}) error {
	fault := &LatencyFault{
		Min: getStringOrDefault(config, "min", "0ms"),
		Max: getStringOrDefault(config, "max", "100ms"),
	}
	return i.InjectLatency(ctx, fault)
}

// InjectError writes an error response
func (i *Injector) InjectError(w http.ResponseWriter, fault *ErrorRateFault) {
	if fault == nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	statusCode := fault.DefaultCode
	if statusCode == 0 {
		statusCode = http.StatusInternalServerError
	}

	// Pick random status code if available
	if len(fault.StatusCodes) > 0 {
		i.mu.Lock()
		statusCode = fault.StatusCodes[i.rng.Intn(len(fault.StatusCodes))]
		i.mu.Unlock()
	}

	http.Error(w, http.StatusText(statusCode), statusCode)
}

// InjectErrorFromConfig injects an error from a FaultConfig
func (i *Injector) InjectErrorFromConfig(w http.ResponseWriter, config map[string]interface{}) {
	fault := &ErrorRateFault{
		DefaultCode: getIntOrDefault(config, "defaultCode", http.StatusInternalServerError),
		StatusCodes: getIntSlice(config, "statusCodes"),
	}
	i.InjectError(w, fault)
}

// InjectTimeout simulates a timeout by waiting for a long duration
func (i *Injector) InjectTimeout(ctx context.Context, duration time.Duration) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(duration):
		return nil
	}
}

// WrapResponseWriter wraps writer for bandwidth limiting
func (i *Injector) WrapResponseWriter(w http.ResponseWriter, fault *BandwidthFault) http.ResponseWriter {
	if fault == nil || fault.BytesPerSecond <= 0 {
		return w
	}

	return &SlowWriter{
		w:              w,
		bytesPerSecond: fault.BytesPerSecond,
	}
}

// WrapForCorruption wraps writer for body corruption
func (i *Injector) WrapForCorruption(w http.ResponseWriter, corruptRate float64) http.ResponseWriter {
	// Create a new RNG for this writer to avoid race conditions
	// when multiple CorruptingWriters run concurrently
	return &CorruptingWriter{
		w:           w,
		corruptRate: corruptRate,
		rng:         rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// WrapForTruncation wraps writer for response truncation
func (i *Injector) WrapForTruncation(w http.ResponseWriter, maxBytes int) http.ResponseWriter {
	return &TruncatingWriter{
		w:        w,
		maxBytes: maxBytes,
	}
}

// GetStats returns the current chaos injection statistics
func (i *Injector) GetStats() ChaosStats {
	i.mu.Lock()
	defer i.mu.Unlock()

	// Return a copy
	statsCopy := ChaosStats{
		TotalRequests:    i.stats.TotalRequests,
		InjectedFaults:   i.stats.InjectedFaults,
		LatencyInjected:  i.stats.LatencyInjected,
		ErrorsInjected:   i.stats.ErrorsInjected,
		TimeoutsInjected: i.stats.TimeoutsInjected,
		FaultsByType:     make(map[FaultType]int64),
	}
	for k, v := range i.stats.FaultsByType {
		statsCopy.FaultsByType[k] = v
	}
	return statsCopy
}

// ResetStats resets the chaos statistics
func (i *Injector) ResetStats() {
	i.mu.Lock()
	defer i.mu.Unlock()
	i.stats = NewChaosStats()
}

// GetConfig returns the current chaos configuration
func (i *Injector) GetConfig() *ChaosConfig {
	i.mu.Lock()
	defer i.mu.Unlock()
	return i.config
}

// isStatefulFault returns true for fault types that require internal state machines.
func isStatefulFault(ft FaultType) bool {
	switch ft {
	case FaultCircuitBreaker, FaultRetryAfter, FaultProgressiveDegradation:
		return true
	default:
		return false
	}
}

// HandleCircuitBreaker processes a request through a circuit breaker.
// Returns true if the request was rejected (response written).
func (i *Injector) HandleCircuitBreaker(key string, w http.ResponseWriter) bool {
	cb, ok := i.circuitBreakers[key]
	if !ok {
		return false
	}
	return cb.Handle(w)
}

// HandleRetryAfter processes a request through a retry-after tracker.
// Returns true if the request was rate-limited (response written).
func (i *Injector) HandleRetryAfter(key string, w http.ResponseWriter) bool {
	tracker, ok := i.retryTrackers[key]
	if !ok {
		return false
	}
	return tracker.Handle(w)
}

// HandleProgressiveDegradation processes a request through a progressive degradation tracker.
// Returns true if an error was returned (the request should not continue).
func (i *Injector) HandleProgressiveDegradation(key string, ctx context.Context, w http.ResponseWriter) bool {
	pd, ok := i.progressives[key]
	if !ok {
		return false
	}
	return pd.Handle(ctx, w)
}

// GetCircuitBreakers returns all circuit breakers (for admin API introspection).
func (i *Injector) GetCircuitBreakers() map[string]*CircuitBreaker {
	return i.circuitBreakers
}

// GetRetryTrackers returns all retry-after trackers (for admin API introspection).
func (i *Injector) GetRetryTrackers() map[string]*RetryAfterTracker {
	return i.retryTrackers
}

// GetProgressives returns all progressive degradation trackers (for admin API introspection).
func (i *Injector) GetProgressives() map[string]*ProgressiveDegradation {
	return i.progressives
}

// UpdateConfig updates the chaos configuration
func (i *Injector) UpdateConfig(config *ChaosConfig) error {
	if config == nil {
		return errors.New("chaos config is required")
	}

	// Clamp probability/rate values to [0.0, 1.0]
	config.Clamp()

	// Compile new rules
	var newRules []*compiledRule
	for _, rule := range config.Rules {
		compiled, err := compileRule(rule)
		if err != nil {
			return fmt.Errorf("failed to compile rule for pattern %q: %w", rule.PathPattern, err)
		}
		newRules = append(newRules, compiled)
	}

	// Build new stateful fault managers
	newCBs := make(map[string]*CircuitBreaker)
	newRTs := make(map[string]*RetryAfterTracker)
	newPDs := make(map[string]*ProgressiveDegradation)

	for ruleIdx, rule := range config.Rules {
		for faultIdx, fault := range rule.Faults {
			key := statefulFaultKey(ruleIdx, faultIdx)
			switch fault.Type { //nolint:exhaustive // only stateful types need initialization
			case FaultCircuitBreaker:
				cfg := ParseCircuitBreakerConfig(fault.Config)
				newCBs[key] = NewCircuitBreaker(cfg, i.rng)
			case FaultRetryAfter:
				cfg := ParseRetryAfterConfig(fault.Config)
				newRTs[key] = NewRetryAfterTracker(cfg)
			case FaultProgressiveDegradation:
				cfg := ParseProgressiveDegradationConfig(fault.Config)
				newPDs[key] = NewProgressiveDegradation(cfg)
			}
		}
	}

	i.mu.Lock()
	defer i.mu.Unlock()

	i.config = config
	i.rules = newRules
	i.circuitBreakers = newCBs
	i.retryTrackers = newRTs
	i.progressives = newPDs

	return nil
}
