package chaos

import (
	"testing"
)

func TestListProfiles(t *testing.T) {
	profiles := ListProfiles()

	if len(profiles) != 10 {
		t.Fatalf("expected 10 profiles, got %d", len(profiles))
	}

	// Verify sorted alphabetically
	for i := 1; i < len(profiles); i++ {
		if profiles[i].Name < profiles[i-1].Name {
			t.Errorf("profiles not sorted: %q appears after %q", profiles[i].Name, profiles[i-1].Name)
		}
	}

	// Verify all profiles have required fields
	for _, p := range profiles {
		if p.Name == "" {
			t.Error("profile has empty name")
		}
		if p.Description == "" {
			t.Errorf("profile %q has empty description", p.Name)
		}
		if !p.Config.Enabled {
			t.Errorf("profile %q config is not enabled", p.Name)
		}
	}
}

func TestListProfilesContainsAllExpected(t *testing.T) {
	expected := []string{
		"degraded",
		"dns-flaky",
		"flaky",
		"mobile-3g",
		"offline",
		"overloaded",
		"rate-limited",
		"satellite",
		"slow-api",
		"timeout",
	}

	profiles := ListProfiles()
	names := make(map[string]bool)
	for _, p := range profiles {
		names[p.Name] = true
	}

	for _, name := range expected {
		if !names[name] {
			t.Errorf("expected profile %q not found", name)
		}
	}
}

func TestGetProfile(t *testing.T) {
	t.Run("existing profile", func(t *testing.T) {
		p, ok := GetProfile("slow-api")
		if !ok {
			t.Fatal("expected to find profile slow-api")
		}
		if p.Name != "slow-api" {
			t.Errorf("expected name slow-api, got %q", p.Name)
		}
		if p.Description != "Simulates slow upstream API" {
			t.Errorf("unexpected description: %q", p.Description)
		}
		if !p.Config.Enabled {
			t.Error("expected config to be enabled")
		}
		if p.Config.GlobalRules == nil {
			t.Fatal("expected global rules to be set")
		}
		if p.Config.GlobalRules.Latency == nil {
			t.Fatal("expected latency to be set")
		}
		if p.Config.GlobalRules.Latency.Min != "500ms" {
			t.Errorf("expected latency min 500ms, got %q", p.Config.GlobalRules.Latency.Min)
		}
		if p.Config.GlobalRules.Latency.Max != "2000ms" {
			t.Errorf("expected latency max 2000ms, got %q", p.Config.GlobalRules.Latency.Max)
		}
	})

	t.Run("nonexistent profile", func(t *testing.T) {
		_, ok := GetProfile("nonexistent")
		if ok {
			t.Error("expected not to find nonexistent profile")
		}
	})

	t.Run("empty name", func(t *testing.T) {
		_, ok := GetProfile("")
		if ok {
			t.Error("expected not to find profile with empty name")
		}
	})
}

func TestApplyProfile(t *testing.T) {
	t.Run("returns nil for unknown profile", func(t *testing.T) {
		cfg := ApplyProfile("nonexistent")
		if cfg != nil {
			t.Error("expected nil config for unknown profile")
		}
	})

	t.Run("returns valid config for slow-api", func(t *testing.T) {
		cfg := ApplyProfile("slow-api")
		if cfg == nil {
			t.Fatal("expected non-nil config")
		}
		if !cfg.Enabled {
			t.Error("expected config to be enabled")
		}
		if cfg.GlobalRules == nil || cfg.GlobalRules.Latency == nil {
			t.Fatal("expected global latency rules")
		}
		if cfg.GlobalRules.Latency.Min != "500ms" {
			t.Errorf("expected latency min 500ms, got %q", cfg.GlobalRules.Latency.Min)
		}
		if cfg.GlobalRules.Latency.Max != "2000ms" {
			t.Errorf("expected latency max 2000ms, got %q", cfg.GlobalRules.Latency.Max)
		}
		if cfg.GlobalRules.Latency.Probability != 1.0 {
			t.Errorf("expected latency probability 1.0, got %v", cfg.GlobalRules.Latency.Probability)
		}
	})

	t.Run("returns deep copy (mutation safe)", func(t *testing.T) {
		cfg1 := ApplyProfile("flaky")
		cfg2 := ApplyProfile("flaky")
		if cfg1 == nil || cfg2 == nil {
			t.Fatal("expected non-nil configs")
		}

		// Mutate cfg1
		cfg1.GlobalRules.Latency.Min = "999ms"
		cfg1.GlobalRules.ErrorRate.StatusCodes[0] = 999

		// cfg2 should be unaffected
		if cfg2.GlobalRules.Latency.Min != "0ms" {
			t.Errorf("deep copy failed: latency min changed to %q", cfg2.GlobalRules.Latency.Min)
		}
		if cfg2.GlobalRules.ErrorRate.StatusCodes[0] != 500 {
			t.Errorf("deep copy failed: status code changed to %d", cfg2.GlobalRules.ErrorRate.StatusCodes[0])
		}

		// Original registry should also be unaffected
		p, _ := GetProfile("flaky")
		if p.Config.GlobalRules.Latency.Min != "0ms" {
			t.Errorf("registry mutated: latency min is %q", p.Config.GlobalRules.Latency.Min)
		}
	})

	t.Run("can create injector from profile config", func(t *testing.T) {
		cfg := ApplyProfile("degraded")
		if cfg == nil {
			t.Fatal("expected non-nil config")
		}

		injector, err := NewInjector(cfg)
		if err != nil {
			t.Fatalf("failed to create injector from profile: %v", err)
		}
		if !injector.IsEnabled() {
			t.Error("expected injector to be enabled")
		}
	})
}

func TestProfileNames(t *testing.T) {
	names := ProfileNames()

	if len(names) != 10 {
		t.Fatalf("expected 10 names, got %d", len(names))
	}

	// Verify sorted
	for i := 1; i < len(names); i++ {
		if names[i] < names[i-1] {
			t.Errorf("names not sorted: %q appears after %q", names[i], names[i-1])
		}
	}

	// Spot check specific names
	expected := map[string]bool{
		"slow-api": true, "degraded": true, "flaky": true,
		"offline": true, "timeout": true, "rate-limited": true,
		"mobile-3g": true, "satellite": true, "dns-flaky": true,
		"overloaded": true,
	}
	for _, name := range names {
		if !expected[name] {
			t.Errorf("unexpected profile name: %q", name)
		}
	}
}

func TestProfileConfigsAreValid(t *testing.T) {
	profiles := ListProfiles()

	for _, p := range profiles {
		t.Run(p.Name, func(t *testing.T) {
			// All profiles should pass validation
			if err := p.Config.Validate(); err != nil {
				t.Errorf("profile %q has invalid config: %v", p.Name, err)
			}

			// All profiles should be enabled
			if !p.Config.Enabled {
				t.Errorf("profile %q should be enabled", p.Name)
			}

			// All profiles should have global rules (none use per-path rules)
			if p.Config.GlobalRules == nil {
				t.Errorf("profile %q should have global rules", p.Name)
			}
		})
	}
}

func TestProfileSpecificConfigurations(t *testing.T) {
	tests := []struct {
		name         string
		hasLatency   bool
		hasErrorRate bool
		hasBandwidth bool
		latencyMin   string
		latencyMax   string
		errorRate    float64
		errorCodes   []int
		bandwidthBPS int
	}{
		{
			name: "slow-api", hasLatency: true,
			latencyMin: "500ms", latencyMax: "2000ms",
		},
		{
			name: "degraded", hasLatency: true, hasErrorRate: true,
			latencyMin: "200ms", latencyMax: "800ms",
			errorRate: 0.05, errorCodes: []int{503},
		},
		{
			name: "flaky", hasLatency: true, hasErrorRate: true,
			latencyMin: "0ms", latencyMax: "100ms",
			errorRate: 0.20, errorCodes: []int{500, 502, 503},
		},
		{
			name: "offline", hasErrorRate: true,
			errorRate: 1.0, errorCodes: []int{503},
		},
		{
			name: "timeout", hasLatency: true,
			latencyMin: "30000ms", latencyMax: "30000ms",
		},
		{
			name: "rate-limited", hasLatency: true, hasErrorRate: true,
			latencyMin: "50ms", latencyMax: "200ms",
			errorRate: 0.30, errorCodes: []int{429},
		},
		{
			name: "mobile-3g", hasLatency: true, hasErrorRate: true, hasBandwidth: true,
			latencyMin: "300ms", latencyMax: "800ms",
			errorRate: 0.02, errorCodes: []int{503},
			bandwidthBPS: 51200,
		},
		{
			name: "satellite", hasLatency: true, hasErrorRate: true, hasBandwidth: true,
			latencyMin: "600ms", latencyMax: "2000ms",
			errorRate: 0.05, errorCodes: []int{503},
			bandwidthBPS: 20480,
		},
		{
			name: "dns-flaky", hasErrorRate: true,
			errorRate: 0.10, errorCodes: []int{503},
		},
		{
			name: "overloaded", hasLatency: true, hasErrorRate: true, hasBandwidth: true,
			latencyMin: "1000ms", latencyMax: "5000ms",
			errorRate: 0.15, errorCodes: []int{500, 502, 503, 504},
			bandwidthBPS: 102400,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, ok := GetProfile(tt.name)
			if !ok {
				t.Fatalf("profile %q not found", tt.name)
			}

			g := p.Config.GlobalRules
			if g == nil {
				t.Fatal("expected global rules")
			}

			// Check latency
			if tt.hasLatency {
				if g.Latency == nil {
					t.Fatal("expected latency fault")
				}
				if g.Latency.Min != tt.latencyMin {
					t.Errorf("latency min: want %q, got %q", tt.latencyMin, g.Latency.Min)
				}
				if g.Latency.Max != tt.latencyMax {
					t.Errorf("latency max: want %q, got %q", tt.latencyMax, g.Latency.Max)
				}
				if g.Latency.Probability != 1.0 {
					t.Errorf("latency probability: want 1.0, got %v", g.Latency.Probability)
				}
			} else {
				if g.Latency != nil {
					t.Error("expected no latency fault")
				}
			}

			// Check error rate
			if tt.hasErrorRate {
				if g.ErrorRate == nil {
					t.Fatal("expected error rate fault")
				}
				if g.ErrorRate.Probability != tt.errorRate {
					t.Errorf("error rate: want %v, got %v", tt.errorRate, g.ErrorRate.Probability)
				}
				if len(g.ErrorRate.StatusCodes) != len(tt.errorCodes) {
					t.Fatalf("error codes length: want %d, got %d", len(tt.errorCodes), len(g.ErrorRate.StatusCodes))
				}
				for i, code := range tt.errorCodes {
					if g.ErrorRate.StatusCodes[i] != code {
						t.Errorf("error code[%d]: want %d, got %d", i, code, g.ErrorRate.StatusCodes[i])
					}
				}
			} else {
				if g.ErrorRate != nil {
					t.Error("expected no error rate fault")
				}
			}

			// Check bandwidth
			if tt.hasBandwidth {
				if g.Bandwidth == nil {
					t.Fatal("expected bandwidth fault")
				}
				if g.Bandwidth.BytesPerSecond != tt.bandwidthBPS {
					t.Errorf("bandwidth: want %d B/s, got %d B/s", tt.bandwidthBPS, g.Bandwidth.BytesPerSecond)
				}
				if g.Bandwidth.Probability != 1.0 {
					t.Errorf("bandwidth probability: want 1.0, got %v", g.Bandwidth.Probability)
				}
			} else {
				if g.Bandwidth != nil {
					t.Error("expected no bandwidth fault")
				}
			}
		})
	}
}

func TestDeepCopyChaosConfig(t *testing.T) {
	t.Run("nil input", func(t *testing.T) {
		result := deepCopyChaosConfig(nil)
		if result != nil {
			t.Error("expected nil output for nil input")
		}
	})

	t.Run("empty config", func(t *testing.T) {
		src := &ChaosConfig{Enabled: true}
		dst := deepCopyChaosConfig(src)
		if dst == nil {
			t.Fatal("expected non-nil output")
		}
		if !dst.Enabled {
			t.Error("expected enabled to be true")
		}
		if dst.GlobalRules != nil {
			t.Error("expected nil global rules")
		}
	})

	t.Run("full config with all fields", func(t *testing.T) {
		src := &ChaosConfig{
			Enabled: true,
			Rules: []ChaosRule{
				{
					PathPattern: "/api/.*",
					Methods:     []string{"GET", "POST"},
					Probability: 0.5,
					Faults: []FaultConfig{
						{Type: FaultLatency, Probability: 0.8},
					},
				},
			},
			GlobalRules: &GlobalChaosRules{
				Latency: &LatencyFault{
					Min:         "100ms",
					Max:         "500ms",
					Probability: 1.0,
				},
				ErrorRate: &ErrorRateFault{
					Probability: 0.1,
					StatusCodes: []int{500, 503},
					DefaultCode: 500,
				},
				Bandwidth: &BandwidthFault{
					BytesPerSecond: 1024,
					Probability:    0.5,
				},
			},
		}

		dst := deepCopyChaosConfig(src)

		// Mutate source and verify destination is independent
		src.Enabled = false
		src.Rules[0].PathPattern = "changed"
		src.Rules[0].Methods[0] = "PUT"
		src.Rules[0].Faults[0].Probability = 0.1
		src.GlobalRules.Latency.Min = "999ms"
		src.GlobalRules.ErrorRate.StatusCodes[0] = 999
		src.GlobalRules.Bandwidth.BytesPerSecond = 9999

		if !dst.Enabled {
			t.Error("dst.Enabled should still be true")
		}
		if dst.Rules[0].PathPattern != "/api/.*" {
			t.Errorf("dst rule pattern mutated: %q", dst.Rules[0].PathPattern)
		}
		if dst.Rules[0].Methods[0] != "GET" {
			t.Errorf("dst rule method mutated: %q", dst.Rules[0].Methods[0])
		}
		if dst.Rules[0].Faults[0].Probability != 0.8 {
			t.Errorf("dst fault probability mutated: %v", dst.Rules[0].Faults[0].Probability)
		}
		if dst.GlobalRules.Latency.Min != "100ms" {
			t.Errorf("dst latency min mutated: %q", dst.GlobalRules.Latency.Min)
		}
		if dst.GlobalRules.ErrorRate.StatusCodes[0] != 500 {
			t.Errorf("dst error code mutated: %d", dst.GlobalRules.ErrorRate.StatusCodes[0])
		}
		if dst.GlobalRules.Bandwidth.BytesPerSecond != 1024 {
			t.Errorf("dst bandwidth mutated: %d", dst.GlobalRules.Bandwidth.BytesPerSecond)
		}
	})
}
