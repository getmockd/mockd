package mcp

import (
	"fmt"

	"github.com/getmockd/mockd/pkg/cli"
)

// =============================================================================
// Observability Handlers
// =============================================================================

// handleGetServerStatus returns server health, ports, and statistics.
func handleGetServerStatus(args map[string]interface{}, session *MCPSession, server *Server) (*ToolResult, error) {
	client := session.GetAdminClient()
	if client == nil {
		return ToolResultError("admin client not available"), nil
	}

	adminURL := session.GetAdminURL()

	result := map[string]interface{}{
		"version":  ServerVersion,
		"adminUrl": adminURL,
	}

	// Health check — returns error only
	if err := client.Health(); err != nil {
		if isConnectionError(err) {
			// Server is unreachable — return structured diagnostic instead of failing
			result["reachable"] = false
			result["healthy"] = false
			result["error"] = err.Error()
			result["hint"] = "Run 'mockd serve' to start the mockd server"
			return ToolResultJSON(result)
		}
		result["reachable"] = true
		result["healthy"] = false
		result["healthError"] = err.Error()
	} else {
		result["reachable"] = true
		result["healthy"] = true
	}

	// Stats
	stats, err := client.GetStats()
	if err == nil && stats != nil {
		result["uptime"] = stats.Uptime
		result["totalRequests"] = stats.RequestCount
		result["mockCount"] = stats.MockCount
	}

	// Ports
	ports, err := client.GetPorts()
	if err == nil && ports != nil {
		result["ports"] = ports
	}

	return ToolResultJSON(result)
}

// handleGetRequestLogs retrieves captured request/response logs.
func handleGetRequestLogs(args map[string]interface{}, session *MCPSession, server *Server) (*ToolResult, error) {
	client := session.GetAdminClient()
	if client == nil {
		return ToolResultError("admin client not available"), nil
	}

	limit := getInt(args, "limit", 100)
	offset := getInt(args, "offset", 0)
	method := getString(args, "method", "")
	pathPrefix := getString(args, "pathPrefix", "")
	mockID := getString(args, "mockId", "")
	protocol := getString(args, "protocol", "")

	filter := &cli.LogFilter{
		Protocol:  protocol,
		Limit:     limit,
		Offset:    offset,
		Method:    method,
		Path:      pathPrefix,
		MatchedID: mockID,
	}

	logsResult, err := client.GetLogs(filter)
	if err != nil {
		//nolint:nilerr // MCP spec: tool errors are returned in result content, not as JSON-RPC errors
		return ToolResultError("failed to get logs: " + adminError(err, session.GetAdminURL())), nil
	}

	entries := make([]RequestLogEntry, 0, len(logsResult.Requests))
	for _, log := range logsResult.Requests {
		entries = append(entries, RequestLogEntry{
			ID:        log.ID,
			Method:    log.Method,
			Path:      log.Path,
			Status:    log.ResponseStatus,
			Duration:  fmt.Sprintf("%dms", log.DurationMs),
			Timestamp: log.Timestamp,
			MockID:    log.MatchedMockID,
		})
	}

	return ToolResultJSON(entries)
}

// handleClearRequestLogs clears all captured request logs.
func handleClearRequestLogs(args map[string]interface{}, session *MCPSession, server *Server) (*ToolResult, error) {
	client := session.GetAdminClient()
	if client == nil {
		return ToolResultError("admin client not available"), nil
	}

	cleared, err := client.ClearLogs()
	if err != nil {
		//nolint:nilerr // MCP spec: tool errors are returned in result content, not as JSON-RPC errors
		return ToolResultError("failed to clear logs: " + adminError(err, session.GetAdminURL())), nil
	}

	result := map[string]interface{}{
		"cleared": cleared,
	}
	return ToolResultJSON(result)
}
