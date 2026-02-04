package mcp

import (
	"fmt"

	"github.com/getmockd/mockd/pkg/stateful"
)

// =============================================================================
// Stateful Resource Handlers
// =============================================================================

// requireStatefulStore returns an error result if stateful store is not available.
func requireStatefulStore(server *Server) *ToolResult {
	if server.statefulStore == nil {
		return ToolResultError("stateful store not available")
	}
	return nil
}

// handleListStatefulItems lists items in a stateful resource with pagination.
func handleListStatefulItems(args map[string]interface{}, session *MCPSession, server *Server) (*ToolResult, error) {
	if err := requireStatefulStore(server); err != nil {
		return err, nil
	}

	resourceName := getString(args, "resource", "")
	if resourceName == "" {
		return ToolResultError("resource is required"), nil
	}

	limit := getInt(args, "limit", 100)
	offset := getInt(args, "offset", 0)

	resource := server.statefulStore.Get(resourceName)
	if resource == nil {
		return ToolResultError("stateful resource not found: " + resourceName), nil
	}

	filter := &stateful.QueryFilter{
		Limit:  limit,
		Offset: offset,
		Sort:   getString(args, "sort", "createdAt"),
		Order:  getString(args, "order", "desc"),
	}

	response := resource.List(filter)

	result := StatefulListResult{
		Data: response.Data,
		Meta: PaginationMeta{
			Total:  response.Meta.Total,
			Limit:  response.Meta.Limit,
			Offset: response.Meta.Offset,
			Count:  response.Meta.Count,
		},
	}

	return ToolResultJSON(result)
}

// handleGetStatefulItem retrieves a specific item from a stateful resource.
func handleGetStatefulItem(args map[string]interface{}, session *MCPSession, server *Server) (*ToolResult, error) {
	if err := requireStatefulStore(server); err != nil {
		return err, nil
	}

	resourceName := getString(args, "resource", "")
	if resourceName == "" {
		return ToolResultError("resource is required"), nil
	}

	id := getString(args, "id", "")
	if id == "" {
		return ToolResultError("id is required"), nil
	}

	resource := server.statefulStore.Get(resourceName)
	if resource == nil {
		return ToolResultError("stateful resource not found: " + resourceName), nil
	}

	item := resource.Get(id)
	if item == nil {
		return ToolResultError(fmt.Sprintf("item not found: %s in resource %s", id, resourceName)), nil
	}

	return ToolResultJSON(item)
}

// handleCreateStatefulItem creates a new item in a stateful resource.
func handleCreateStatefulItem(args map[string]interface{}, session *MCPSession, server *Server) (*ToolResult, error) {
	if err := requireStatefulStore(server); err != nil {
		return err, nil
	}

	resourceName := getString(args, "resource", "")
	if resourceName == "" {
		return ToolResultError("resource is required"), nil
	}

	data := getMap(args, "data")
	if data == nil {
		return ToolResultError("data is required"), nil
	}

	resource := server.statefulStore.Get(resourceName)
	if resource == nil {
		return ToolResultError("stateful resource not found: " + resourceName), nil
	}

	item, err := resource.Create(data, nil) // no path params for MCP
	if err != nil {
		//nolint:nilerr // MCP spec: tool errors are returned in result content, not as JSON-RPC errors
		return ToolResultError("failed to create item: " + err.Error()), nil
	}

	return ToolResultJSON(item)
}

// handleResetStatefulData resets a stateful resource to its seed data.
// Resource is required — no accidental full resets.
func handleResetStatefulData(args map[string]interface{}, session *MCPSession, server *Server) (*ToolResult, error) {
	if err := requireStatefulStore(server); err != nil {
		return err, nil
	}

	resourceName := getString(args, "resource", "")
	if resourceName == "" {
		return ToolResultError("resource is required — specify which resource to reset"), nil
	}

	response, err := server.statefulStore.Reset(resourceName)
	if err != nil {
		//nolint:nilerr // MCP spec: tool errors are returned in result content, not as JSON-RPC errors
		return ToolResultError("failed to reset: " + err.Error()), nil
	}

	return ToolResultJSON(response)
}
