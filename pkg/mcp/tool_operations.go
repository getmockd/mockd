package mcp

// =============================================================================
// Custom Operations Handler (multiplexed)
// =============================================================================

// handleManageCustomOperation manages custom operations on stateful resources.
// Uses a single tool with an `action` parameter to avoid consuming 5 tool slots.
func handleManageCustomOperation(args map[string]interface{}, session *MCPSession, server *Server) (*ToolResult, error) {
	client := session.GetAdminClient()
	if client == nil {
		return ToolResultError("admin client not available"), nil
	}

	action := getString(args, "action", "")
	if action == "" {
		return ToolResultError("action is required (list, get, register, delete, execute)"), nil
	}

	switch action {
	case "list":
		ops, err := client.ListCustomOperations()
		if err != nil {
			//nolint:nilerr // MCP spec: tool errors are returned in result content, not as JSON-RPC errors
			return ToolResultError("failed to list operations: " + adminError(err, session.GetAdminURL())), nil
		}
		return ToolResultJSON(map[string]interface{}{
			"operations": ops,
			"count":      len(ops),
		})

	case "get":
		name := getString(args, "name", "")
		if name == "" {
			return ToolResultError("name is required for action=get"), nil
		}
		op, err := client.GetCustomOperation(name)
		if err != nil {
			//nolint:nilerr // MCP spec: tool errors are returned in result content, not as JSON-RPC errors
			if isConnectionError(err) {
				return ToolResultError("failed to get operation: " + adminError(err, session.GetAdminURL())), nil
			}
			return ToolResultError("operation not found: " + name), nil
		}
		return ToolResultJSON(op)

	case "register":
		definition := getMap(args, "definition")
		if definition == nil {
			return ToolResultError("definition is required for action=register"), nil
		}
		if err := client.RegisterCustomOperation(definition); err != nil {
			//nolint:nilerr // MCP spec: tool errors are returned in result content, not as JSON-RPC errors
			return ToolResultError("failed to register operation: " + adminError(err, session.GetAdminURL())), nil
		}
		return ToolResultJSON(map[string]interface{}{
			"registered": true,
		})

	case "delete":
		name := getString(args, "name", "")
		if name == "" {
			return ToolResultError("name is required for action=delete"), nil
		}
		if err := client.DeleteCustomOperation(name); err != nil {
			//nolint:nilerr // MCP spec: tool errors are returned in result content, not as JSON-RPC errors
			if isConnectionError(err) {
				return ToolResultError("failed to delete operation: " + adminError(err, session.GetAdminURL())), nil
			}
			return ToolResultError("operation not found: " + name), nil
		}
		return ToolResultJSON(map[string]interface{}{
			"deleted": true,
			"name":    name,
		})

	case "execute":
		name := getString(args, "name", "")
		if name == "" {
			return ToolResultError("name is required for action=execute"), nil
		}
		input := getMap(args, "input")
		if input == nil {
			input = make(map[string]interface{})
		}
		result, err := client.ExecuteCustomOperation(name, input)
		if err != nil {
			//nolint:nilerr // MCP spec: tool errors are returned in result content, not as JSON-RPC errors
			if isConnectionError(err) {
				return ToolResultError("failed to execute operation: " + adminError(err, session.GetAdminURL())), nil
			}
			return ToolResultError("operation not found: " + name), nil
		}
		return ToolResultJSON(result)

	default:
		return ToolResultError("unknown action: " + action + ". Use list, get, register, delete, or execute"), nil
	}
}
