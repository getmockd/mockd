package mcp

import "fmt"

// =============================================================================
// Chaos Engineering Handlers
// =============================================================================

// formatDurationMs converts a millisecond value to a Go duration string (e.g., "500ms", "2s").
func formatDurationMs(ms float64) string {
	if ms >= 1000 && float64(int(ms)) == ms && int(ms)%1000 == 0 {
		return fmt.Sprintf("%ds", int(ms)/1000)
	}
	return fmt.Sprintf("%dms", int(ms))
}

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

	// Check if a named profile was requested — use the dedicated admin endpoint.
	profile := getString(args, "profile", "")
	if profile != "" {
		if err := client.ApplyChaosProfile(profile); err != nil {
			//nolint:nilerr // MCP spec: tool errors are returned in result content, not as JSON-RPC errors
			return ToolResultError("failed to apply chaos profile: " + adminError(err, session.GetAdminURL())), nil
		}
		return ToolResultJSON(map[string]interface{}{
			"applied": true,
			"profile": profile,
		})
	}

	// Check if raw rules were provided — send them directly to the admin API.
	if rules, ok := args["rules"]; ok {
		chaosConfig := map[string]interface{}{
			"enabled": getBool(args, "enabled", true),
			"rules":   rules,
		}
		if err := client.SetChaosConfig(chaosConfig); err != nil {
			//nolint:nilerr // MCP spec: tool errors are returned in result content, not as JSON-RPC errors
			return ToolResultError("failed to set chaos rules: " + adminError(err, session.GetAdminURL())), nil
		}
		return ToolResultJSON(map[string]interface{}{
			"applied": true,
			"config":  chaosConfig,
		})
	}

	// Build config matching the admin API's types.ChaosConfig shape:
	//   { "enabled": bool, "latency": {...}, "errorRate": {...}, "bandwidth": {...} }
	chaosConfig := map[string]interface{}{
		"enabled": getBool(args, "enabled", true),
	}

	// Latency: admin expects { "min": "100ms", "max": "500ms", "probability": 1.0 }
	// MCP tool accepts latency_ms (fixed) or latency_min_ms/latency_max_ms (range) in milliseconds.
	latencyMs := getFloat(args, "latency_ms")
	latencyMinMs := getFloat(args, "latency_min_ms")
	latencyMaxMs := getFloat(args, "latency_max_ms")

	if latencyMs > 0 {
		// Fixed latency: use the same value for min and max.
		chaosConfig["latency"] = map[string]interface{}{
			"min":         formatDurationMs(latencyMs),
			"max":         formatDurationMs(latencyMs),
			"probability": 1.0,
		}
	} else if latencyMinMs > 0 || latencyMaxMs > 0 {
		// Range latency.
		if latencyMinMs <= 0 {
			latencyMinMs = latencyMaxMs
		}
		if latencyMaxMs <= 0 {
			latencyMaxMs = latencyMinMs
		}
		chaosConfig["latency"] = map[string]interface{}{
			"min":         formatDurationMs(latencyMinMs),
			"max":         formatDurationMs(latencyMaxMs),
			"probability": 1.0,
		}
	}

	// Error rate: admin expects { "probability": 0.2, "statusCodes": [500, 502] }
	errorRate := getFloat(args, "error_rate")
	if errorRate > 0 {
		errConfig := map[string]interface{}{
			"probability": errorRate,
		}
		if codes, ok := args["error_codes"]; ok {
			errConfig["statusCodes"] = codes
		}
		chaosConfig["errorRate"] = errConfig
	}

	// Bandwidth: admin expects { "bytesPerSecond": 50000, "probability": 1.0 }
	if bw, ok := args["bandwidth_bytes_per_sec"]; ok {
		chaosConfig["bandwidth"] = map[string]interface{}{
			"bytesPerSecond": bw,
			"probability":    1.0,
		}
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

// handleGetStatefulFaults retrieves status of all stateful fault instances.
func handleGetStatefulFaults(args map[string]interface{}, session *MCPSession, server *Server) (*ToolResult, error) {
	client := session.GetAdminClient()
	if client == nil {
		return ToolResultError("admin client not available"), nil
	}

	stats, err := client.GetStatefulFaultStats()
	if err != nil {
		//nolint:nilerr // MCP spec: tool errors are returned in result content, not as JSON-RPC errors
		return ToolResultError("failed to get stateful fault stats: " + adminError(err, session.GetAdminURL())), nil
	}

	return ToolResultJSON(stats)
}

// handleManageCircuitBreaker manually trips or resets a circuit breaker.
func handleManageCircuitBreaker(args map[string]interface{}, session *MCPSession, server *Server) (*ToolResult, error) {
	client := session.GetAdminClient()
	if client == nil {
		return ToolResultError("admin client not available"), nil
	}

	action := getString(args, "action", "")
	key := getString(args, "key", "")

	if action == "" {
		return ToolResultError("action is required (trip or reset)"), nil
	}
	if key == "" {
		return ToolResultError("key is required (e.g., \"0:0\")"), nil
	}

	switch action {
	case "trip":
		if err := client.TripCircuitBreaker(key); err != nil {
			//nolint:nilerr // MCP spec: tool errors are returned in result content, not as JSON-RPC errors
			return ToolResultError("failed to trip circuit breaker: " + adminError(err, session.GetAdminURL())), nil
		}
		return ToolResultJSON(map[string]interface{}{
			"action":  "tripped",
			"key":     key,
			"message": fmt.Sprintf("circuit breaker %q manually tripped to OPEN state", key),
		})

	case "reset":
		if err := client.ResetCircuitBreaker(key); err != nil {
			//nolint:nilerr // MCP spec: tool errors are returned in result content, not as JSON-RPC errors
			return ToolResultError("failed to reset circuit breaker: " + adminError(err, session.GetAdminURL())), nil
		}
		return ToolResultJSON(map[string]interface{}{
			"action":  "reset",
			"key":     key,
			"message": fmt.Sprintf("circuit breaker %q manually reset to CLOSED state", key),
		})

	default:
		return ToolResultError("invalid action: " + action + " (must be 'trip' or 'reset')"), nil
	}
}
