package chaos

import (
	"fmt"
)

// ChaosConfig configures chaos injection for fault simulation
type ChaosConfig struct {
	Enabled     bool              `json:"enabled" yaml:"enabled"`
	Rules       []ChaosRule       `json:"rules,omitempty" yaml:"rules,omitempty"`
	GlobalRules *GlobalChaosRules `json:"global,omitempty" yaml:"global,omitempty"`
}

// ChaosRule defines a chaos rule for specific paths
type ChaosRule struct {
	PathPattern string        `json:"pathPattern" yaml:"pathPattern"` // Regex or glob
	Methods     []string      `json:"methods,omitempty" yaml:"methods,omitempty"`
	Faults      []FaultConfig `json:"faults" yaml:"faults"`
	Probability float64       `json:"probability,omitempty" yaml:"probability,omitempty"` // 0.0-1.0
}

// GlobalChaosRules apply to all requests
type GlobalChaosRules struct {
	Latency   *LatencyFault   `json:"latency,omitempty" yaml:"latency,omitempty"`
	ErrorRate *ErrorRateFault `json:"errorRate,omitempty" yaml:"errorRate,omitempty"`
	Bandwidth *BandwidthFault `json:"bandwidth,omitempty" yaml:"bandwidth,omitempty"`
}

// FaultConfig configures a specific fault type
type FaultConfig struct {
	Type        FaultType              `json:"type" yaml:"type"`
	Probability float64                `json:"probability" yaml:"probability"` // 0.0-1.0
	Config      map[string]interface{} `json:"config,omitempty" yaml:"config,omitempty"`
}

// FaultType represents the type of fault to inject
type FaultType string

const (
	// FaultLatency adds random latency to responses
	FaultLatency FaultType = "latency"
	// FaultError returns error status codes
	FaultError FaultType = "error"
	// FaultTimeout simulates connection timeout
	FaultTimeout FaultType = "timeout"
	// FaultCorruptBody corrupts response body data
	FaultCorruptBody FaultType = "corrupt_body"
	// FaultEmptyResponse returns empty body
	FaultEmptyResponse FaultType = "empty_response"
	// FaultSlowBody returns response data slowly (drip feed)
	FaultSlowBody FaultType = "slow_body"
	// FaultConnectionReset simulates connection reset
	FaultConnectionReset FaultType = "connection_reset"
	// FaultPartialResponse truncates response at random point
	FaultPartialResponse FaultType = "partial_response"
)

// LatencyFault adds random latency to responses
type LatencyFault struct {
	Min         string  `json:"min" yaml:"min"`                 // e.g., "100ms"
	Max         string  `json:"max" yaml:"max"`                 // e.g., "5s"
	Probability float64 `json:"probability" yaml:"probability"` // 0.0-1.0
}

// ErrorRateFault returns errors randomly
type ErrorRateFault struct {
	Probability float64 `json:"probability" yaml:"probability"`
	StatusCodes []int   `json:"statusCodes,omitempty" yaml:"statusCodes,omitempty"` // Random from these
	DefaultCode int     `json:"defaultCode,omitempty" yaml:"defaultCode,omitempty"` // Default: 500
}

// BandwidthFault limits response bandwidth
type BandwidthFault struct {
	BytesPerSecond int     `json:"bytesPerSecond" yaml:"bytesPerSecond"`
	Probability    float64 `json:"probability" yaml:"probability"`
}

// TimeoutFault configures timeout simulation
type TimeoutFault struct {
	Duration    string  `json:"duration" yaml:"duration"`       // How long to wait before timeout
	Probability float64 `json:"probability" yaml:"probability"` // 0.0-1.0
}

// CorruptBodyFault configures body corruption
type CorruptBodyFault struct {
	CorruptRate float64 `json:"corruptRate" yaml:"corruptRate"` // Percentage of bytes to corrupt (0.0-1.0)
	Probability float64 `json:"probability" yaml:"probability"` // 0.0-1.0
}

// SlowBodyFault configures slow response delivery
type SlowBodyFault struct {
	BytesPerSecond int     `json:"bytesPerSecond" yaml:"bytesPerSecond"`
	Probability    float64 `json:"probability" yaml:"probability"` // 0.0-1.0
}

// PartialResponseFault configures response truncation
type PartialResponseFault struct {
	MaxPercent  float64 `json:"maxPercent" yaml:"maxPercent"`   // Max percentage of response to return (0.0-1.0)
	MinPercent  float64 `json:"minPercent" yaml:"minPercent"`   // Min percentage of response to return (0.0-1.0)
	Probability float64 `json:"probability" yaml:"probability"` // 0.0-1.0
}

// ChaosStats tracks chaos injection statistics
type ChaosStats struct {
	TotalRequests    int64               `json:"totalRequests"`
	InjectedFaults   int64               `json:"injectedFaults"`
	FaultsByType     map[FaultType]int64 `json:"faultsByType"`
	LatencyInjected  int64               `json:"latencyInjected"`
	ErrorsInjected   int64               `json:"errorsInjected"`
	TimeoutsInjected int64               `json:"timeoutsInjected"`
}

// NewChaosStats creates a new stats tracker
func NewChaosStats() *ChaosStats {
	return &ChaosStats{
		FaultsByType: make(map[FaultType]int64),
	}
}

// validateProbability checks that a probability value is in the valid range [0.0, 1.0].
func validateProbability(value float64, fieldName string) error {
	if value < 0.0 || value > 1.0 {
		return fmt.Errorf("%s must be between 0.0 and 1.0, got %v", fieldName, value)
	}
	return nil
}

// Validate checks if the ChaosConfig is valid.
func (c *ChaosConfig) Validate() error {
	if !c.Enabled {
		return nil
	}

	for i, rule := range c.Rules {
		if err := rule.Validate(); err != nil {
			return fmt.Errorf("rules[%d]: %w", i, err)
		}
	}

	if c.GlobalRules != nil {
		if err := c.GlobalRules.Validate(); err != nil {
			return fmt.Errorf("global: %w", err)
		}
	}

	return nil
}

// Validate checks if the ChaosRule is valid.
func (r *ChaosRule) Validate() error {
	if r.Probability != 0 {
		if err := validateProbability(r.Probability, "probability"); err != nil {
			return err
		}
	}

	for i, fault := range r.Faults {
		if err := fault.Validate(); err != nil {
			return fmt.Errorf("faults[%d]: %w", i, err)
		}
	}

	return nil
}

// Validate checks if the FaultConfig is valid.
func (f *FaultConfig) Validate() error {
	return validateProbability(f.Probability, "probability")
}

// Validate checks if the GlobalChaosRules are valid.
func (g *GlobalChaosRules) Validate() error {
	if g.Latency != nil {
		if err := g.Latency.Validate(); err != nil {
			return fmt.Errorf("latency: %w", err)
		}
	}
	if g.ErrorRate != nil {
		if err := g.ErrorRate.Validate(); err != nil {
			return fmt.Errorf("errorRate: %w", err)
		}
	}
	if g.Bandwidth != nil {
		if err := g.Bandwidth.Validate(); err != nil {
			return fmt.Errorf("bandwidth: %w", err)
		}
	}
	return nil
}

// Validate checks if the LatencyFault is valid.
func (l *LatencyFault) Validate() error {
	return validateProbability(l.Probability, "probability")
}

// Validate checks if the ErrorRateFault is valid.
func (e *ErrorRateFault) Validate() error {
	return validateProbability(e.Probability, "probability")
}

// Validate checks if the BandwidthFault is valid.
func (b *BandwidthFault) Validate() error {
	if err := validateProbability(b.Probability, "probability"); err != nil {
		return err
	}
	if b.BytesPerSecond <= 0 {
		return fmt.Errorf("bytesPerSecond must be > 0, got %d", b.BytesPerSecond)
	}
	return nil
}

// Validate checks if the TimeoutFault is valid.
func (t *TimeoutFault) Validate() error {
	return validateProbability(t.Probability, "probability")
}

// Validate checks if the CorruptBodyFault is valid.
func (c *CorruptBodyFault) Validate() error {
	if err := validateProbability(c.Probability, "probability"); err != nil {
		return err
	}
	if err := validateProbability(c.CorruptRate, "corruptRate"); err != nil {
		return err
	}
	return nil
}

// Validate checks if the SlowBodyFault is valid.
func (s *SlowBodyFault) Validate() error {
	if err := validateProbability(s.Probability, "probability"); err != nil {
		return err
	}
	if s.BytesPerSecond <= 0 {
		return fmt.Errorf("bytesPerSecond must be > 0, got %d", s.BytesPerSecond)
	}
	return nil
}

// Validate checks if the PartialResponseFault is valid.
func (p *PartialResponseFault) Validate() error {
	if err := validateProbability(p.Probability, "probability"); err != nil {
		return err
	}
	if err := validateProbability(p.MinPercent, "minPercent"); err != nil {
		return err
	}
	if err := validateProbability(p.MaxPercent, "maxPercent"); err != nil {
		return err
	}
	if p.MinPercent > p.MaxPercent {
		return fmt.Errorf("minPercent (%v) must be <= maxPercent (%v)", p.MinPercent, p.MaxPercent)
	}
	return nil
}
