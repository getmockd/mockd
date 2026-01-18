package chaos

import (
	"context"
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
		return nil, fmt.Errorf("chaos config is required")
	}

	i := &Injector{
		config: config,
		rng:    rand.New(rand.NewSource(time.Now().UnixNano())),
		stats:  NewChaosStats(),
	}

	// Compile rules
	for _, rule := range config.Rules {
		compiled, err := compileRule(rule)
		if err != nil {
			return nil, fmt.Errorf("failed to compile rule for pattern %q: %w", rule.PathPattern, err)
		}
		i.rules = append(i.rules, compiled)
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
	if prob <= 0 {
		prob = 1.0 // Default to always apply if probability not set
	}

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

	// Check path-specific rules first
	for _, rule := range i.rules {
		if !rule.pattern.MatchString(r.URL.Path) {
			continue
		}

		// Check method filter
		if len(rule.methods) > 0 && !rule.methods[r.Method] {
			continue
		}

		// Check rule probability
		if i.rng.Float64() > rule.prob {
			continue
		}

		// Check each fault's probability
		for _, fault := range rule.faults {
			if i.rng.Float64() <= fault.Probability {
				faults = append(faults, fault)
				i.stats.InjectedFaults++
				i.stats.FaultsByType[fault.Type]++
			}
		}
	}

	// Apply global rules if no path-specific faults triggered
	if len(faults) == 0 && i.config.GlobalRules != nil {
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

// InjectLatencyFromConfig injects latency from a FaultConfig
func (i *Injector) InjectLatencyFromConfig(ctx context.Context, config map[string]interface{}) error {
	fault := &LatencyFault{}

	if v, ok := config["min"].(string); ok {
		fault.Min = v
	} else {
		fault.Min = "0ms"
	}

	if v, ok := config["max"].(string); ok {
		fault.Max = v
	} else {
		fault.Max = "100ms"
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
		DefaultCode: http.StatusInternalServerError,
	}

	if v, ok := config["defaultCode"].(int); ok {
		fault.DefaultCode = v
	} else if v, ok := config["defaultCode"].(float64); ok {
		fault.DefaultCode = int(v)
	}

	if v, ok := config["statusCodes"].([]interface{}); ok {
		for _, code := range v {
			if c, ok := code.(int); ok {
				fault.StatusCodes = append(fault.StatusCodes, c)
			} else if c, ok := code.(float64); ok {
				fault.StatusCodes = append(fault.StatusCodes, int(c))
			}
		}
	} else if v, ok := config["statusCodes"].([]int); ok {
		fault.StatusCodes = v
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
	return &CorruptingWriter{
		w:           w,
		corruptRate: corruptRate,
		rng:         i.rng,
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
	return i.config
}

// UpdateConfig updates the chaos configuration
func (i *Injector) UpdateConfig(config *ChaosConfig) error {
	if config == nil {
		return fmt.Errorf("chaos config is required")
	}

	// Compile new rules
	var newRules []*compiledRule
	for _, rule := range config.Rules {
		compiled, err := compileRule(rule)
		if err != nil {
			return fmt.Errorf("failed to compile rule for pattern %q: %w", rule.PathPattern, err)
		}
		newRules = append(newRules, compiled)
	}

	i.mu.Lock()
	defer i.mu.Unlock()

	i.config = config
	i.rules = newRules

	return nil
}
