package mcp

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/getmockd/mockd/pkg/config"
	"github.com/getmockd/mockd/pkg/mock"
)

// =============================================================================
// Mock CRUD Handlers
// =============================================================================

// handleManageMock dispatches mock CRUD operations based on the action parameter.
// This is the single entry point for all mock management — list, get, create,
// update, delete, and toggle are all routed through this multiplexed handler.
func handleManageMock(args map[string]interface{}, session *MCPSession, server *Server) (*ToolResult, error) {
	action := getString(args, "action", "")
	switch action {
	case "list":
		return handleListMocks(args, session, server)
	case "get":
		return handleGetMock(args, session, server)
	case "create":
		return handleCreateMock(args, session, server)
	case "update":
		return handleUpdateMock(args, session, server)
	case "delete":
		return handleDeleteMock(args, session, server)
	case "toggle":
		return handleToggleMock(args, session, server)
	default:
		return ToolResultError("invalid action: " + action + ". Use: list, get, create, update, delete, toggle"), nil
	}
}

// handleListMocks lists all configured mocks across all protocols.
func handleListMocks(args map[string]interface{}, session *MCPSession, server *Server) (*ToolResult, error) {
	client := session.GetAdminClient()
	if client == nil {
		return ToolResultError("admin client not available"), nil
	}

	typeFilter := getString(args, "type", "")
	enabledFilter := getBoolPtr(args, "enabled")

	var mocks []*config.MockConfiguration
	var err error

	if typeFilter != "" {
		mocks, err = client.ListMocksByType(typeFilter)
	} else {
		mocks, err = client.ListMocks()
	}
	if err != nil {
		//nolint:nilerr // MCP spec: tool errors are returned in result content, not as JSON-RPC errors
		return ToolResultError("failed to list mocks: " + adminError(err, session.GetAdminURL())), nil
	}

	summaries := make([]MockSummary, 0, len(mocks))
	for _, m := range mocks {
		enabled := m.Enabled == nil || *m.Enabled

		// Apply enabled filter
		if enabledFilter != nil && enabled != *enabledFilter {
			continue
		}

		summaries = append(summaries, MockSummary{
			ID:      m.ID,
			Type:    string(m.Type),
			Name:    m.Name,
			Enabled: enabled,
			Summary: mockSummaryLine(m),
		})
	}

	return ToolResultJSON(summaries)
}

// mockSummaryLine returns a human-readable one-line summary for a mock.
func mockSummaryLine(m *config.MockConfiguration) string {
	switch m.Type {
	case mock.TypeHTTP:
		if m.HTTP != nil && m.HTTP.Matcher != nil {
			method := m.HTTP.Matcher.Method
			if method == "" {
				method = "ANY"
			}
			return method + " " + m.HTTP.Matcher.Path
		}
	case mock.TypeWebSocket:
		if m.WebSocket != nil {
			return "ws://" + m.WebSocket.Path
		}
	case mock.TypeGraphQL:
		if m.GraphQL != nil {
			return "graphql " + m.GraphQL.Path
		}
	case mock.TypeGRPC:
		if m.GRPC != nil {
			services := make([]string, 0)
			for svc := range m.GRPC.Services {
				services = append(services, svc)
			}
			return fmt.Sprintf("grpc :%d (%s)", m.GRPC.Port, strings.Join(services, ", "))
		}
	case mock.TypeSOAP:
		if m.SOAP != nil {
			return "soap " + m.SOAP.Path
		}
	case mock.TypeMQTT:
		if m.MQTT != nil {
			topics := make([]string, 0)
			for _, t := range m.MQTT.Topics {
				topics = append(topics, t.Topic)
			}
			return fmt.Sprintf("mqtt :%d (%s)", m.MQTT.Port, strings.Join(topics, ", "))
		}
	case mock.TypeOAuth:
		if m.OAuth != nil {
			return "oauth " + m.OAuth.Issuer
		}
	}

	if m.Name != "" {
		return m.Name
	}
	return string(m.Type)
}

// handleGetMock retrieves full configuration for a specific mock.
func handleGetMock(args map[string]interface{}, session *MCPSession, server *Server) (*ToolResult, error) {
	client := session.GetAdminClient()
	if client == nil {
		return ToolResultError("admin client not available"), nil
	}

	id := getString(args, "id", "")
	if id == "" {
		return ToolResultError("id is required"), nil
	}

	mockCfg, err := client.GetMock(id)
	if err != nil {
		//nolint:nilerr // MCP spec: tool errors are returned in result content, not as JSON-RPC errors
		if isConnectionError(err) {
			return ToolResultError("failed to get mock: " + adminError(err, session.GetAdminURL())), nil
		}
		return ToolResultError("mock not found: " + id), nil
	}

	return ToolResultJSON(mockCfg)
}

// handleCreateMock creates a new mock for any supported protocol.
func handleCreateMock(args map[string]interface{}, session *MCPSession, server *Server) (*ToolResult, error) {
	client := session.GetAdminClient()
	if client == nil {
		return ToolResultError("admin client not available"), nil
	}

	mockType := getString(args, "type", "")
	if mockType == "" {
		return ToolResultError("type is required"), nil
	}

	name := getString(args, "name", "")

	// Marshal the full args to JSON, then unmarshal into MockConfiguration.
	// This gives us the pass-through behavior — protocol-specific fields
	// are forwarded as-is to the admin API.
	argsJSON, err := json.Marshal(args)
	if err != nil {
		//nolint:nilerr // MCP spec: tool errors are returned in result content, not as JSON-RPC errors
		return ToolResultError("failed to serialize mock config: " + err.Error()), nil
	}

	var mockCfg config.MockConfiguration
	if err := json.Unmarshal(argsJSON, &mockCfg); err != nil {
		//nolint:nilerr // MCP spec: tool errors are returned in result content, not as JSON-RPC errors
		return ToolResultError("invalid mock configuration: " + err.Error()), nil
	}

	// Ensure required fields are set.
	mockCfg.Type = mock.Type(mockType)
	if name != "" {
		mockCfg.Name = name
	}
	enabled := true
	mockCfg.Enabled = &enabled

	createResult, err := client.CreateMock(&mockCfg)
	if err != nil {
		//nolint:nilerr // MCP spec: tool errors are returned in result content, not as JSON-RPC errors
		return ToolResultError("failed to create mock: " + adminError(err, session.GetAdminURL())), nil
	}

	// Notify resource change
	server.NotifyResourceListChanged()

	result := map[string]interface{}{
		"id":     createResult.Mock.ID,
		"action": createResult.Action,
	}
	if createResult.IsMerge() {
		result["message"] = createResult.Message
	}
	return ToolResultJSON(result)
}

// handleUpdateMock updates an existing mock's configuration.
func handleUpdateMock(args map[string]interface{}, session *MCPSession, server *Server) (*ToolResult, error) {
	client := session.GetAdminClient()
	if client == nil {
		return ToolResultError("admin client not available"), nil
	}

	id := getString(args, "id", "")
	if id == "" {
		return ToolResultError("id is required"), nil
	}

	// Fetch existing mock
	existingMock, err := client.GetMock(id)
	if err != nil {
		//nolint:nilerr // MCP spec: tool errors are returned in result content, not as JSON-RPC errors
		if isConnectionError(err) {
			return ToolResultError("failed to update mock: " + adminError(err, session.GetAdminURL())), nil
		}
		return ToolResultError("mock not found: " + id), nil
	}

	// Apply simple field updates
	if name, ok := args["name"].(string); ok {
		existingMock.Name = name
	}
	if enabled, ok := args["enabled"].(bool); ok {
		existingMock.Enabled = &enabled
	}

	// For protocol-specific updates, extract ONLY the protocol fields and overlay
	// them onto the existing mock. We must not re-serialize all args (which includes
	// "id", "name", etc.) to avoid overwriting fields that were already handled above.
	protocolFields := []string{"http", "websocket", "graphql", "grpc", "soap", "mqtt", "oauth"}
	protocolUpdate := make(map[string]interface{})
	for _, field := range protocolFields {
		if val, ok := args[field]; ok {
			protocolUpdate[field] = val
		}
	}
	if len(protocolUpdate) > 0 {
		updateJSON, _ := json.Marshal(protocolUpdate)
		if err := json.Unmarshal(updateJSON, existingMock); err != nil {
			//nolint:nilerr // MCP spec: tool errors are returned in result content, not as JSON-RPC errors
			return ToolResultError("failed to merge update: " + err.Error()), nil
		}
	}

	if _, err := client.UpdateMock(id, existingMock); err != nil {
		//nolint:nilerr // MCP spec: tool errors are returned in result content, not as JSON-RPC errors
		return ToolResultError("failed to update mock: " + adminError(err, session.GetAdminURL())), nil
	}

	server.NotifyResourceListChanged()

	result := map[string]interface{}{
		"updated": true,
		"id":      id,
	}
	return ToolResultJSON(result)
}

// handleDeleteMock deletes a mock by ID.
func handleDeleteMock(args map[string]interface{}, session *MCPSession, server *Server) (*ToolResult, error) {
	client := session.GetAdminClient()
	if client == nil {
		return ToolResultError("admin client not available"), nil
	}

	id := getString(args, "id", "")
	if id == "" {
		return ToolResultError("id is required"), nil
	}

	if err := client.DeleteMock(id); err != nil {
		//nolint:nilerr // MCP spec: tool errors are returned in result content, not as JSON-RPC errors
		return ToolResultError("failed to delete mock: " + adminError(err, session.GetAdminURL())), nil
	}

	server.NotifyResourceListChanged()

	result := map[string]interface{}{
		"deleted": true,
		"id":      id,
	}
	return ToolResultJSON(result)
}

// handleToggleMock enables or disables a mock using PATCH for atomic state change.
func handleToggleMock(args map[string]interface{}, session *MCPSession, server *Server) (*ToolResult, error) {
	client := session.GetAdminClient()
	if client == nil {
		return ToolResultError("admin client not available"), nil
	}

	id := getString(args, "id", "")
	if id == "" {
		return ToolResultError("id is required"), nil
	}

	enabled := getBool(args, "enabled", true)

	// Use PATCH to atomically set the enabled state without a GET+PUT race.
	_, err := client.PatchMock(id, map[string]interface{}{
		"enabled": enabled,
	})
	if err != nil {
		//nolint:nilerr // MCP spec: tool errors are returned in result content, not as JSON-RPC errors
		if isConnectionError(err) {
			return ToolResultError("failed to toggle mock: " + adminError(err, session.GetAdminURL())), nil
		}
		return ToolResultError("failed to toggle mock: " + adminError(err, session.GetAdminURL())), nil
	}

	server.NotifyResourceListChanged()

	result := map[string]interface{}{
		"toggled": true,
		"id":      id,
		"enabled": enabled,
	}
	return ToolResultJSON(result)
}
