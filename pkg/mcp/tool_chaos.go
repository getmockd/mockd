package mcp

// =============================================================================
// Chaos Engineering Handlers
// =============================================================================

// handleGetChaosConfig retrieves the current chaos configuration and stats.
func handleGetChaosConfig(args map[string]interface{}, session *MCPSession, server *Server) (*ToolResult, error) {
	client := session.GetAdminClient()
	if client == nil {
		return ToolResultError("admin client not available"), nil
	}

	chaosConfig, err := client.GetChaosConfig()
	if err != nil {
		//nolint:nilerr // MCP spec: tool errors are returned in result content, not as JSON-RPC errors
		return ToolResultError("failed to get chaos config: " + adminError(err, session.GetAdminURL())), nil
	}

	// Also fetch stats for a complete picture
	chaosStats, err := client.GetChaosStats()
	if err == nil && chaosStats != nil {
		chaosConfig["stats"] = chaosStats
	}

	return ToolResultJSON(chaosConfig)
}

// handleSetChaosConfig configures chaos fault injection rules.
func handleSetChaosConfig(args map[string]interface{}, session *MCPSession, server *Server) (*ToolResult, error) {
	client := session.GetAdminClient()
	if client == nil {
		return ToolResultError("admin client not available"), nil
	}

	// Check if a named profile was requested
	profile := getString(args, "profile", "")
	if profile != "" {
		// TODO: Support chaos profiles from pkg/chaos/profiles.go when available.
		// For now, map well-known profile names to inline configs.
		profileConfig := resolveProfile(profile)
		if profileConfig == nil {
			return ToolResultError("unknown chaos profile: " + profile + ". Supported: slow-api, degraded, flaky, offline, timeout, rate-limited, mobile-3g, satellite, dns-flaky, overloaded"), nil
		}
		if err := client.SetChaosConfig(profileConfig); err != nil {
			//nolint:nilerr // MCP spec: tool errors are returned in result content, not as JSON-RPC errors
			return ToolResultError("failed to set chaos config: " + adminError(err, session.GetAdminURL())), nil
		}
		return ToolResultJSON(map[string]interface{}{
			"applied": true,
			"profile": profile,
			"config":  profileConfig,
		})
	}

	// Build config from individual params
	chaosConfig := make(map[string]interface{})

	chaosConfig["enabled"] = getBool(args, "enabled", true)

	if v, ok := args["latency_ms"]; ok {
		chaosConfig["latencyMs"] = v
	}
	if v, ok := args["latency_min_ms"]; ok {
		chaosConfig["latencyMinMs"] = v
	}
	if v, ok := args["latency_max_ms"]; ok {
		chaosConfig["latencyMaxMs"] = v
	}
	if v, ok := args["error_rate"]; ok {
		chaosConfig["errorRate"] = v
	}
	if v, ok := args["error_codes"]; ok {
		chaosConfig["errorCodes"] = v
	}
	if v, ok := args["bandwidth_bytes_per_sec"]; ok {
		chaosConfig["bandwidthBytesPerSec"] = v
	}

	if err := client.SetChaosConfig(chaosConfig); err != nil {
		//nolint:nilerr // MCP spec: tool errors are returned in result content, not as JSON-RPC errors
		return ToolResultError("failed to set chaos config: " + adminError(err, session.GetAdminURL())), nil
	}

	return ToolResultJSON(map[string]interface{}{
		"applied": true,
		"config":  chaosConfig,
	})
}

// handleResetChaosStats resets chaos injection statistics counters.
func handleResetChaosStats(args map[string]interface{}, session *MCPSession, server *Server) (*ToolResult, error) {
	client := session.GetAdminClient()
	if client == nil {
		return ToolResultError("admin client not available"), nil
	}

	if err := client.ResetChaosStats(); err != nil {
		//nolint:nilerr // MCP spec: tool errors are returned in result content, not as JSON-RPC errors
		return ToolResultError("failed to reset chaos stats: " + adminError(err, session.GetAdminURL())), nil
	}

	return ToolResultJSON(map[string]interface{}{
		"reset":   true,
		"message": "chaos statistics counters reset to zero",
	})
}

// resolveProfile maps a named chaos profile to its configuration.
// Returns nil if the profile name is not recognized.
func resolveProfile(name string) map[string]interface{} {
	profiles := map[string]map[string]interface{}{
		"slow-api": {
			"enabled":      true,
			"latencyMinMs": 500,
			"latencyMaxMs": 2000,
		},
		"degraded": {
			"enabled":      true,
			"latencyMinMs": 100,
			"latencyMaxMs": 500,
			"errorRate":    0.05,
			"errorCodes":   []int{500, 503},
		},
		"flaky": {
			"enabled":    true,
			"errorRate":  0.2,
			"errorCodes": []int{500, 502, 503},
		},
		"offline": {
			"enabled":    true,
			"errorRate":  1.0,
			"errorCodes": []int{503},
		},
		"timeout": {
			"enabled":      true,
			"latencyMinMs": 30000,
			"latencyMaxMs": 60000,
		},
		"rate-limited": {
			"enabled":    true,
			"errorRate":  0.5,
			"errorCodes": []int{429},
		},
		"mobile-3g": {
			"enabled":              true,
			"latencyMinMs":         100,
			"latencyMaxMs":         500,
			"bandwidthBytesPerSec": 50000,
		},
		"satellite": {
			"enabled":              true,
			"latencyMinMs":         500,
			"latencyMaxMs":         1500,
			"bandwidthBytesPerSec": 10000,
		},
		"dns-flaky": {
			"enabled":    true,
			"errorRate":  0.1,
			"errorCodes": []int{502, 504},
		},
		"overloaded": {
			"enabled":      true,
			"latencyMinMs": 1000,
			"latencyMaxMs": 5000,
			"errorRate":    0.3,
			"errorCodes":   []int{500, 502, 503, 504},
		},
	}

	return profiles[name]
}
