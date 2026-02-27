package mcp

import (
	"fmt"
)

// =============================================================================
// Stateful Resource Handlers
// =============================================================================
// All stateful tools go through the admin API (HTTP) so they work in both
// stdio mode (MCP → admin → engine) and embedded mode.

// handleManageState dispatches stateful resource operations based on the action parameter.
// This is the single entry point for all stateful resource management — overview,
// list_items, get_item, create_item, and reset are all routed through this handler.
func handleManageState(args map[string]interface{}, session *MCPSession, server *Server) (*ToolResult, error) {
	action := getString(args, "action", "")
	switch action {
	case "overview":
		return handleGetStateOverview(args, session, server)
	case "add_resource":
		return handleAddStatefulResource(args, session, server)
	case "list_items":
		return handleListStatefulItems(args, session, server)
	case "get_item":
		return handleGetStatefulItem(args, session, server)
	case "create_item":
		return handleCreateStatefulItem(args, session, server)
	case "reset":
		return handleResetStatefulData(args, session, server)
	default:
		return ToolResultError("invalid action: " + action + ". Use: overview, add_resource, list_items, get_item, create_item, reset"), nil
	}
}

// handleAddStatefulResource creates a new stateful resource definition.
func handleAddStatefulResource(args map[string]interface{}, session *MCPSession, server *Server) (*ToolResult, error) {
	client := session.GetAdminClient()
	if client == nil {
		return ToolResultError("admin client not available"), nil
	}

	name := getString(args, "resource", "")
	if name == "" {
		return ToolResultError("resource name is required"), nil
	}

	basePath := getString(args, "path", "")
	idField := getString(args, "id_field", "")

	err := client.CreateStatefulResource(name, basePath, idField)
	if err != nil {
		//nolint:nilerr // MCP spec: tool errors are returned in result content, not as JSON-RPC errors
		return ToolResultError("failed to create stateful resource: " + adminError(err, session.GetAdminURL())), nil
	}

	result := map[string]interface{}{
		"created":  true,
		"resource": name,
		"idField":  idField,
	}
	if idField == "" {
		result["idField"] = "id"
	}
	if basePath != "" {
		result["basePath"] = basePath
		result["mode"] = "http+bridge"
	} else {
		result["mode"] = "bridge-only"
	}

	return ToolResultJSON(result)
}

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

	// Accept both "item_id" (multiplexed manage_state) and "id" (legacy) for
	// the item identifier.
	id := getString(args, "item_id", "")
	if id == "" {
		id = getString(args, "id", "")
	}
	if id == "" {
		return ToolResultError("item_id is required"), nil
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

// handleGetStateOverview returns an overview of all stateful mock resources.
func handleGetStateOverview(args map[string]interface{}, session *MCPSession, server *Server) (*ToolResult, error) {
	client := session.GetAdminClient()
	if client == nil {
		return ToolResultError("admin client not available"), nil
	}

	overview, err := client.GetStateOverview()
	if err != nil {
		//nolint:nilerr // MCP spec: tool errors are returned in result content, not as JSON-RPC errors
		return ToolResultError("failed to get state overview: " + adminError(err, session.GetAdminURL())), nil
	}

	return ToolResultJSON(overview)
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
