// Copyright 2025 Mockd LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package chaos

import "sort"

// Profile is a pre-built chaos configuration that users can apply by name
// instead of manually setting individual fault parameters.
type Profile struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Config      ChaosConfig `json:"config"`
}

// builtinProfiles contains the 10 pre-built chaos profiles.
var builtinProfiles = map[string]Profile{
	"slow-api": {
		Name:        "slow-api",
		Description: "Simulates slow upstream API",
		Config: ChaosConfig{
			Enabled: true,
			GlobalRules: &GlobalChaosRules{
				Latency: &LatencyFault{
					Min:         "500ms",
					Max:         "2000ms",
					Probability: 1.0,
				},
			},
		},
	},
	"degraded": {
		Name:        "degraded",
		Description: "Partially degraded service",
		Config: ChaosConfig{
			Enabled: true,
			GlobalRules: &GlobalChaosRules{
				Latency: &LatencyFault{
					Min:         "200ms",
					Max:         "800ms",
					Probability: 1.0,
				},
				ErrorRate: &ErrorRateFault{
					Probability: 0.05,
					StatusCodes: []int{503},
				},
			},
		},
	},
	"flaky": {
		Name:        "flaky",
		Description: "Unreliable service with random errors",
		Config: ChaosConfig{
			Enabled: true,
			GlobalRules: &GlobalChaosRules{
				Latency: &LatencyFault{
					Min:         "0ms",
					Max:         "100ms",
					Probability: 1.0,
				},
				ErrorRate: &ErrorRateFault{
					Probability: 0.20,
					StatusCodes: []int{500, 502, 503},
				},
			},
		},
	},
	"offline": {
		Name:        "offline",
		Description: "Service completely down",
		Config: ChaosConfig{
			Enabled: true,
			GlobalRules: &GlobalChaosRules{
				ErrorRate: &ErrorRateFault{
					Probability: 1.0,
					StatusCodes: []int{503},
				},
			},
		},
	},
	"timeout": {
		Name:        "timeout",
		Description: "Connection timeout simulation",
		Config: ChaosConfig{
			Enabled: true,
			GlobalRules: &GlobalChaosRules{
				Latency: &LatencyFault{
					Min:         "30000ms",
					Max:         "30000ms",
					Probability: 1.0,
				},
			},
		},
	},
	"rate-limited": {
		Name:        "rate-limited",
		Description: "Rate-limited API",
		Config: ChaosConfig{
			Enabled: true,
			GlobalRules: &GlobalChaosRules{
				Latency: &LatencyFault{
					Min:         "50ms",
					Max:         "200ms",
					Probability: 1.0,
				},
				ErrorRate: &ErrorRateFault{
					Probability: 0.30,
					StatusCodes: []int{429},
				},
			},
		},
	},
	"mobile-3g": {
		Name:        "mobile-3g",
		Description: "Mobile 3G network conditions",
		Config: ChaosConfig{
			Enabled: true,
			GlobalRules: &GlobalChaosRules{
				Latency: &LatencyFault{
					Min:         "300ms",
					Max:         "800ms",
					Probability: 1.0,
				},
				ErrorRate: &ErrorRateFault{
					Probability: 0.02,
					StatusCodes: []int{503},
				},
				Bandwidth: &BandwidthFault{
					BytesPerSecond: 51200, // 50 KB/s
					Probability:    1.0,
				},
			},
		},
	},
	"satellite": {
		Name:        "satellite",
		Description: "Satellite internet simulation",
		Config: ChaosConfig{
			Enabled: true,
			GlobalRules: &GlobalChaosRules{
				Latency: &LatencyFault{
					Min:         "600ms",
					Max:         "2000ms",
					Probability: 1.0,
				},
				ErrorRate: &ErrorRateFault{
					Probability: 0.05,
					StatusCodes: []int{503},
				},
				Bandwidth: &BandwidthFault{
					BytesPerSecond: 20480, // 20 KB/s
					Probability:    1.0,
				},
			},
		},
	},
	"dns-flaky": {
		Name:        "dns-flaky",
		Description: "Intermittent DNS resolution failures",
		Config: ChaosConfig{
			Enabled: true,
			GlobalRules: &GlobalChaosRules{
				ErrorRate: &ErrorRateFault{
					Probability: 0.10,
					StatusCodes: []int{503},
				},
			},
		},
	},
	"overloaded": {
		Name:        "overloaded",
		Description: "Overloaded server under heavy load",
		Config: ChaosConfig{
			Enabled: true,
			GlobalRules: &GlobalChaosRules{
				Latency: &LatencyFault{
					Min:         "1000ms",
					Max:         "5000ms",
					Probability: 1.0,
				},
				ErrorRate: &ErrorRateFault{
					Probability: 0.15,
					StatusCodes: []int{500, 502, 503, 504},
				},
				Bandwidth: &BandwidthFault{
					BytesPerSecond: 102400, // 100 KB/s
					Probability:    1.0,
				},
			},
		},
	},
}

// ListProfiles returns all built-in chaos profiles sorted alphabetically by name.
func ListProfiles() []Profile {
	profiles := make([]Profile, 0, len(builtinProfiles))
	for _, p := range builtinProfiles {
		profiles = append(profiles, p)
	}
	sort.Slice(profiles, func(i, j int) bool {
		return profiles[i].Name < profiles[j].Name
	})
	return profiles
}

// GetProfile returns a built-in chaos profile by name.
// Returns the profile and true if found, or a zero Profile and false if not.
func GetProfile(name string) (Profile, bool) {
	p, ok := builtinProfiles[name]
	return p, ok
}

// ApplyProfile returns a copy of the ChaosConfig for the named profile,
// ready to be passed to an Injector. Returns nil if the profile is not found.
func ApplyProfile(name string) *ChaosConfig {
	p, ok := builtinProfiles[name]
	if !ok {
		return nil
	}
	// Return a deep copy so callers can't mutate the built-in registry.
	cfg := deepCopyChaosConfig(&p.Config)
	return cfg
}

// ProfileNames returns the names of all built-in profiles sorted alphabetically.
func ProfileNames() []string {
	names := make([]string, 0, len(builtinProfiles))
	for name := range builtinProfiles {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// deepCopyChaosConfig creates a deep copy of a ChaosConfig.
func deepCopyChaosConfig(src *ChaosConfig) *ChaosConfig {
	if src == nil {
		return nil
	}
	dst := &ChaosConfig{
		Enabled: src.Enabled,
	}

	// Deep copy rules
	if len(src.Rules) > 0 {
		dst.Rules = make([]ChaosRule, len(src.Rules))
		for i, rule := range src.Rules {
			dst.Rules[i] = ChaosRule{
				PathPattern: rule.PathPattern,
				Probability: rule.Probability,
			}
			if len(rule.Methods) > 0 {
				dst.Rules[i].Methods = make([]string, len(rule.Methods))
				copy(dst.Rules[i].Methods, rule.Methods)
			}
			if len(rule.Faults) > 0 {
				dst.Rules[i].Faults = make([]FaultConfig, len(rule.Faults))
				copy(dst.Rules[i].Faults, rule.Faults)
			}
		}
	}

	// Deep copy global rules
	if src.GlobalRules != nil {
		dst.GlobalRules = &GlobalChaosRules{}
		if src.GlobalRules.Latency != nil {
			dst.GlobalRules.Latency = &LatencyFault{
				Min:         src.GlobalRules.Latency.Min,
				Max:         src.GlobalRules.Latency.Max,
				Probability: src.GlobalRules.Latency.Probability,
			}
		}
		if src.GlobalRules.ErrorRate != nil {
			dst.GlobalRules.ErrorRate = &ErrorRateFault{
				Probability: src.GlobalRules.ErrorRate.Probability,
				DefaultCode: src.GlobalRules.ErrorRate.DefaultCode,
			}
			if len(src.GlobalRules.ErrorRate.StatusCodes) > 0 {
				dst.GlobalRules.ErrorRate.StatusCodes = make([]int, len(src.GlobalRules.ErrorRate.StatusCodes))
				copy(dst.GlobalRules.ErrorRate.StatusCodes, src.GlobalRules.ErrorRate.StatusCodes)
			}
		}
		if src.GlobalRules.Bandwidth != nil {
			dst.GlobalRules.Bandwidth = &BandwidthFault{
				BytesPerSecond: src.GlobalRules.Bandwidth.BytesPerSecond,
				Probability:    src.GlobalRules.Bandwidth.Probability,
			}
		}
	}

	return dst
}
