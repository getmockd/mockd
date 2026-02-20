package mcp

import (
	"fmt"
)

// =============================================================================
// Stateful Resource Handlers
// =============================================================================
// All stateful tools go through the admin API (HTTP) so they work in both
// stdio mode (MCP → admin → engine) and embedded mode.

// handleListStatefulItems lists items in a stateful resource with pagination.
func handleListStatefulItems(args map[string]interface{}, session *MCPSession, server *Server) (*ToolResult, error) {
	client := session.GetAdminClient()
	if client == nil {
		return ToolResultError("admin client not available"), nil
	}

	resourceName := getString(args, "resource", "")
	if resourceName == "" {
		return ToolResultError("resource is required"), nil
	}

	limit := getInt(args, "limit", 100)
	offset := getInt(args, "offset", 0)
	sort := getString(args, "sort", "createdAt")
	order := getString(args, "order", "desc")

	result, err := client.ListStatefulItems(resourceName, limit, offset, sort, order)
	if err != nil {
		//nolint:nilerr // MCP spec: tool errors are returned in result content, not as JSON-RPC errors
		return ToolResultError("failed to list stateful items: " + adminError(err, session.GetAdminURL())), nil
	}

	listResult := StatefulListResult{
		Data: result.Data,
		Meta: PaginationMeta{
			Total:  result.Meta.Total,
			Limit:  result.Meta.Limit,
			Offset: result.Meta.Offset,
			Count:  result.Meta.Count,
		},
	}

	return ToolResultJSON(listResult)
}

// handleGetStatefulItem retrieves a specific item from a stateful resource.
func handleGetStatefulItem(args map[string]interface{}, session *MCPSession, server *Server) (*ToolResult, error) {
	client := session.GetAdminClient()
	if client == nil {
		return ToolResultError("admin client not available"), nil
	}

	resourceName := getString(args, "resource", "")
	if resourceName == "" {
		return ToolResultError("resource is required"), nil
	}

	id := getString(args, "id", "")
	if id == "" {
		return ToolResultError("id is required"), nil
	}

	item, err := client.GetStatefulItem(resourceName, id)
	if err != nil {
		//nolint:nilerr // MCP spec: tool errors are returned in result content, not as JSON-RPC errors
		return ToolResultError(fmt.Sprintf("item not found: %s in resource %s", id, resourceName)), nil
	}

	return ToolResultJSON(item)
}

// handleCreateStatefulItem creates a new item in a stateful resource.
func handleCreateStatefulItem(args map[string]interface{}, session *MCPSession, server *Server) (*ToolResult, error) {
	client := session.GetAdminClient()
	if client == nil {
		return ToolResultError("admin client not available"), nil
	}

	resourceName := getString(args, "resource", "")
	if resourceName == "" {
		return ToolResultError("resource is required"), nil
	}

	data := getMap(args, "data")
	if data == nil {
		return ToolResultError("data is required"), nil
	}

	item, err := client.CreateStatefulItem(resourceName, data)
	if err != nil {
		//nolint:nilerr // MCP spec: tool errors are returned in result content, not as JSON-RPC errors
		return ToolResultError("failed to create item: " + adminError(err, session.GetAdminURL())), nil
	}

	return ToolResultJSON(item)
}

// handleResetStatefulData resets a stateful resource to its seed data.
// Resource is required — no accidental full resets.
func handleResetStatefulData(args map[string]interface{}, session *MCPSession, server *Server) (*ToolResult, error) {
	client := session.GetAdminClient()
	if client == nil {
		return ToolResultError("admin client not available"), nil
	}

	resourceName := getString(args, "resource", "")
	if resourceName == "" {
		return ToolResultError("resource is required — specify which resource to reset"), nil
	}

	err := client.ResetStatefulResource(resourceName)
	if err != nil {
		//nolint:nilerr // MCP spec: tool errors are returned in result content, not as JSON-RPC errors
		return ToolResultError("failed to reset: " + adminError(err, session.GetAdminURL())), nil
	}

	return ToolResultJSON(map[string]interface{}{
		"reset":    true,
		"resource": resourceName,
		"message":  "resource reset to seed data",
	})
}
