package mcp

// =============================================================================
// Verification Handlers
// =============================================================================

// handleVerifyMock checks whether a mock was called the expected number of times.
func handleVerifyMock(args map[string]interface{}, session *MCPSession, server *Server) (*ToolResult, error) {
	client := session.GetAdminClient()
	if client == nil {
		return ToolResultError("admin client not available"), nil
	}

	id := getString(args, "id", "")
	if id == "" {
		return ToolResultError("id is required"), nil
	}

	verification, err := client.GetMockVerification(id)
	if err != nil {
		//nolint:nilerr // MCP spec: tool errors are returned in result content, not as JSON-RPC errors
		if isConnectionError(err) {
			return ToolResultError("failed to verify mock: " + adminError(err, session.GetAdminURL())), nil
		}
		return ToolResultError("mock not found: " + id), nil
	}

	// Extract actual call count from verification response
	actualCount := 0
	if v, ok := verification["callCount"]; ok {
		switch n := v.(type) {
		case float64:
			actualCount = int(n)
		case int:
			actualCount = n
		}
	}

	// Apply local count assertions if provided
	verified := true
	expected := make(map[string]interface{})

	if v, ok := args["expected_count"]; ok {
		expectedCount := 0
		switch n := v.(type) {
		case float64:
			expectedCount = int(n)
		case int:
			expectedCount = n
		}
		expected["exactCount"] = expectedCount
		if actualCount != expectedCount {
			verified = false
		}
	}

	if v, ok := args["at_least"]; ok {
		atLeast := 0
		switch n := v.(type) {
		case float64:
			atLeast = int(n)
		case int:
			atLeast = n
		}
		expected["atLeast"] = atLeast
		if actualCount < atLeast {
			verified = false
		}
	}

	if v, ok := args["at_most"]; ok {
		atMost := 0
		switch n := v.(type) {
		case float64:
			atMost = int(n)
		case int:
			atMost = n
		}
		expected["atMost"] = atMost
		if actualCount > atMost {
			verified = false
		}
	}

	result := map[string]interface{}{
		"mockId":      id,
		"verified":    verified,
		"actualCount": actualCount,
	}
	if len(expected) > 0 {
		result["expected"] = expected
	}

	// Include invocations if present in the verification response
	if invocations, ok := verification["invocations"]; ok {
		result["invocations"] = invocations
	}

	return ToolResultJSON(result)
}

// handleGetMockInvocations lists all recorded invocations for a specific mock.
func handleGetMockInvocations(args map[string]interface{}, session *MCPSession, server *Server) (*ToolResult, error) {
	client := session.GetAdminClient()
	if client == nil {
		return ToolResultError("admin client not available"), nil
	}

	id := getString(args, "id", "")
	if id == "" {
		return ToolResultError("id is required"), nil
	}

	limit := getInt(args, "limit", 50)

	result, err := client.ListMockInvocations(id)
	if err != nil {
		//nolint:nilerr // MCP spec: tool errors are returned in result content, not as JSON-RPC errors
		if isConnectionError(err) {
			return ToolResultError("failed to get invocations: " + adminError(err, session.GetAdminURL())), nil
		}
		return ToolResultError("mock not found: " + id), nil
	}

	// Truncate invocations list if it exceeds the limit
	if invocations, ok := result["invocations"].([]interface{}); ok && len(invocations) > limit {
		result["invocations"] = invocations[:limit]
		result["truncated"] = true
		result["totalCount"] = len(invocations)
	}

	return ToolResultJSON(result)
}

// handleResetVerification clears verification data for a specific mock or all mocks.
func handleResetVerification(args map[string]interface{}, session *MCPSession, server *Server) (*ToolResult, error) {
	client := session.GetAdminClient()
	if client == nil {
		return ToolResultError("admin client not available"), nil
	}

	id := getString(args, "id", "")

	if id != "" {
		// Reset specific mock
		if err := client.ResetMockVerification(id); err != nil {
			//nolint:nilerr // MCP spec: tool errors are returned in result content, not as JSON-RPC errors
			if isConnectionError(err) {
				return ToolResultError("failed to reset verification: " + adminError(err, session.GetAdminURL())), nil
			}
			return ToolResultError("mock not found: " + id), nil
		}
		return ToolResultJSON(map[string]interface{}{
			"reset":  true,
			"mockId": id,
		})
	}

	// Reset all mocks
	if err := client.ResetAllVerification(); err != nil {
		//nolint:nilerr // MCP spec: tool errors are returned in result content, not as JSON-RPC errors
		return ToolResultError("failed to reset verification: " + adminError(err, session.GetAdminURL())), nil
	}

	return ToolResultJSON(map[string]interface{}{
		"reset":   true,
		"scope":   "all",
		"message": "verification data cleared for all mocks",
	})
}
