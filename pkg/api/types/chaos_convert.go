package types

import (
	"github.com/getmockd/mockd/pkg/chaos"
)

// ChaosConfigFromInternal converts an internal chaos.ChaosConfig to the
// API-level ChaosConfig used for admin-engine HTTP communication.
//
// This is the single canonical conversion point. All code that needs to
// convert internal → API chaos config should call this function.
func ChaosConfigFromInternal(src *chaos.ChaosConfig) ChaosConfig {
	if src == nil {
		return ChaosConfig{}
	}

	cfg := ChaosConfig{
		Enabled: src.Enabled,
	}

	// Convert global rules (flat latency/error/bandwidth fields)
	if src.GlobalRules != nil {
		if src.GlobalRules.Latency != nil {
			cfg.Latency = &LatencyConfig{
				Min:         src.GlobalRules.Latency.Min,
				Max:         src.GlobalRules.Latency.Max,
				Probability: src.GlobalRules.Latency.Probability,
			}
		}
		if src.GlobalRules.ErrorRate != nil {
			cfg.ErrorRate = &ErrorRateConfig{
				Probability: src.GlobalRules.ErrorRate.Probability,
				DefaultCode: src.GlobalRules.ErrorRate.DefaultCode,
			}
			if len(src.GlobalRules.ErrorRate.StatusCodes) > 0 {
				cfg.ErrorRate.StatusCodes = make([]int, len(src.GlobalRules.ErrorRate.StatusCodes))
				copy(cfg.ErrorRate.StatusCodes, src.GlobalRules.ErrorRate.StatusCodes)
			}
		}
		if src.GlobalRules.Bandwidth != nil {
			cfg.Bandwidth = &BandwidthConfig{
				BytesPerSecond: src.GlobalRules.Bandwidth.BytesPerSecond,
				Probability:    src.GlobalRules.Bandwidth.Probability,
			}
		}
	}

	// Convert per-path rules
	for _, rule := range src.Rules {
		apiRule := ChaosRuleConfig{
			PathPattern: rule.PathPattern,
			Probability: rule.Probability,
		}
		if len(rule.Methods) > 0 {
			apiRule.Methods = make([]string, len(rule.Methods))
			copy(apiRule.Methods, rule.Methods)
		}
		for _, f := range rule.Faults {
			apiRule.Faults = append(apiRule.Faults, ChaosFaultConfig{
				Type:        string(f.Type),
				Probability: f.Probability,
				Config:      f.Config,
			})
		}
		cfg.Rules = append(cfg.Rules, apiRule)
	}

	return cfg
}

// ChaosConfigToInternal converts an API-level ChaosConfig back to an
// internal chaos.ChaosConfig.
//
// This is the single canonical reverse conversion point.
func ChaosConfigToInternal(src *ChaosConfig) *chaos.ChaosConfig {
	if src == nil {
		return nil
	}

	cfg := &chaos.ChaosConfig{
		Enabled: src.Enabled,
	}

	// Convert global rules
	if src.Latency != nil || src.ErrorRate != nil || src.Bandwidth != nil {
		cfg.GlobalRules = &chaos.GlobalChaosRules{}
		if src.Latency != nil {
			cfg.GlobalRules.Latency = &chaos.LatencyFault{
				Min:         src.Latency.Min,
				Max:         src.Latency.Max,
				Probability: src.Latency.Probability,
			}
		}
		if src.ErrorRate != nil {
			cfg.GlobalRules.ErrorRate = &chaos.ErrorRateFault{
				Probability: src.ErrorRate.Probability,
				StatusCodes: src.ErrorRate.StatusCodes,
				DefaultCode: src.ErrorRate.DefaultCode,
			}
		}
		if src.Bandwidth != nil {
			cfg.GlobalRules.Bandwidth = &chaos.BandwidthFault{
				BytesPerSecond: src.Bandwidth.BytesPerSecond,
				Probability:    src.Bandwidth.Probability,
			}
		}
	}

	// Convert per-path rules
	for _, rule := range src.Rules {
		cr := chaos.ChaosRule{
			PathPattern: rule.PathPattern,
			Methods:     rule.Methods,
			Probability: rule.Probability,
		}
		for _, f := range rule.Faults {
			cr.Faults = append(cr.Faults, chaos.FaultConfig{
				Type:        chaos.FaultType(f.Type),
				Probability: f.Probability,
				Config:      f.Config,
			})
		}
		cfg.Rules = append(cfg.Rules, cr)
	}

	return cfg
}
