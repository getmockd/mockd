package mcp

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/getmockd/mockd/pkg/cli"
	"github.com/getmockd/mockd/pkg/config"
	"github.com/getmockd/mockd/pkg/mock"
	"github.com/getmockd/mockd/pkg/stateful"
)

// =============================================================================
// Precondition Helpers - DRY pattern for common checks
// =============================================================================

// requireAdminClient returns an error result if admin client is not available.
func requireAdminClient(server *Server) *ToolResult {
	if server.adminClient == nil {
		return ToolResultError("admin client not available")
	}
	return nil
}

// requireStatefulStore returns an error result if stateful store is not available.
func requireStatefulStore(server *Server) *ToolResult {
	if server.statefulStore == nil {
		return ToolResultError("stateful store not available")
	}
	return nil
}

// =============================================================================
// Tool Handlers
// =============================================================================

// handleGetMockData retrieves mock response data for a path/method.
func handleGetMockData(args map[string]interface{}, session *MCPSession, server *Server) (*ToolResult, error) {
	path := getString(args, "path", "")
	if path == "" {
		return ToolResultError("path is required"), nil
	}

	method := getString(args, "method", "GET")
	headers := getStringMap(args, "headers")
	queryParams := getStringMap(args, "queryParams")
	_ = getString(args, "body", "") // body matching not yet implemented

	if err := requireAdminClient(server); err != nil {
		return err, nil
	}

	// Build a mock HTTP request to match against
	reqURL, _ := url.Parse(path)
	if queryParams != nil {
		q := reqURL.Query()
		for k, v := range queryParams {
			q.Set(k, v)
		}
		reqURL.RawQuery = q.Encode()
	}

	// Create a minimal request for matching
	req := &http.Request{
		Method: method,
		URL:    reqURL,
		Header: make(http.Header),
	}

	if headers != nil {
		for k, v := range headers {
			req.Header.Set(k, v)
		}
	}

	// Try to find a matching mock via HTTP
	mocks, err := server.adminClient.ListMocks()
	if err != nil {
		return ToolResultError("failed to list mocks: " + err.Error()), nil
	}
	for _, m := range mocks {
		if !m.Enabled {
			continue
		}
		if m.HTTP == nil || m.HTTP.Matcher == nil {
			continue
		}

		// Check method match
		if m.HTTP.Matcher.Method != "" && m.HTTP.Matcher.Method != method {
			continue
		}

		// Check path match
		if m.HTTP.Matcher.Path != "" && !matchPath(m.HTTP.Matcher.Path, path) {
			continue
		}

		// Found a match - return the response body
		if m.HTTP.Response != nil {
			result := map[string]interface{}{
				"status":  m.HTTP.Response.StatusCode,
				"body":    m.HTTP.Response.Body,
				"headers": m.HTTP.Response.Headers,
				"mockId":  m.ID,
			}
			return ToolResultJSON(result)
		}
	}

	return ToolResultError(fmt.Sprintf("no mock found for %s %s", method, path)), nil
}

// matchPath checks if a request path matches a mock path pattern.
func matchPath(pattern, path string) bool {
	// Exact match
	if pattern == path {
		return true
	}

	// Wildcard match for path parameters like /api/users/{id}
	patternParts := strings.Split(pattern, "/")
	pathParts := strings.Split(path, "/")

	if len(patternParts) != len(pathParts) {
		return false
	}

	for i, part := range patternParts {
		if strings.HasPrefix(part, "{") && strings.HasSuffix(part, "}") {
			// Path parameter - matches anything
			continue
		}
		if part != pathParts[i] {
			return false
		}
	}

	return true
}

// handleListEndpoints lists all configured mock endpoints.
func handleListEndpoints(args map[string]interface{}, session *MCPSession, server *Server) (*ToolResult, error) {
	if err := requireAdminClient(server); err != nil {
		return err, nil
	}

	methodFilter := getString(args, "method", "")
	pathPrefix := getString(args, "pathPrefix", "")
	enabledFilter := getBoolPtr(args, "enabled")

	mocks, err := server.adminClient.ListMocks()
	if err != nil {
		return ToolResultError("failed to list mocks: " + err.Error()), nil
	}
	endpoints := make([]EndpointInfo, 0, len(mocks))

	for _, m := range mocks {
		// Apply filters
		if methodFilter != "" && m.HTTP != nil && m.HTTP.Matcher != nil && m.HTTP.Matcher.Method != methodFilter {
			continue
		}
		if pathPrefix != "" && m.HTTP != nil && m.HTTP.Matcher != nil && !strings.HasPrefix(m.HTTP.Matcher.Path, pathPrefix) {
			continue
		}
		if enabledFilter != nil && m.Enabled != *enabledFilter {
			continue
		}

		path := ""
		method := ""
		priority := 0
		if m.HTTP != nil && m.HTTP.Matcher != nil {
			path = m.HTTP.Matcher.Path
			method = m.HTTP.Matcher.Method
			priority = m.HTTP.Priority
		}

		endpoints = append(endpoints, EndpointInfo{
			ID:          m.ID,
			Method:      method,
			Path:        path,
			Enabled:     m.Enabled,
			Description: m.Name,
			Priority:    priority,
			CreatedAt:   m.CreatedAt,
			UpdatedAt:   m.UpdatedAt,
		})
	}

	return ToolResultJSON(endpoints)
}

// handleCreateEndpoint creates a new mock endpoint.
func handleCreateEndpoint(args map[string]interface{}, session *MCPSession, server *Server) (*ToolResult, error) {
	if err := requireAdminClient(server); err != nil {
		return err, nil
	}

	path := getString(args, "path", "")
	method := getString(args, "method", "")
	description := getString(args, "description", "")
	priority := getInt(args, "priority", 0)

	if path == "" {
		return ToolResultError("path is required"), nil
	}
	if method == "" {
		return ToolResultError("method is required"), nil
	}

	responseArg := getMap(args, "response")
	if responseArg == nil {
		return ToolResultError("response is required"), nil
	}

	// Build response definition
	status := getInt(responseArg, "status", 200)
	headers := getStringMap(responseArg, "headers")
	bodyArg := responseArg["body"]
	delayStr := getString(responseArg, "delay", "0s")

	// Convert body to string
	var body string
	switch v := bodyArg.(type) {
	case string:
		body = v
	default:
		bodyJSON, _ := json.Marshal(v)
		body = string(bodyJSON)
	}

	// Parse delay
	delay, _ := time.ParseDuration(delayStr)
	delayMs := int(delay.Milliseconds())

	// Create mock configuration
	m := &config.MockConfiguration{
		Name:    description,
		Type:    mock.MockTypeHTTP,
		Enabled: true,
		HTTP: &mock.HTTPSpec{
			Priority: priority,
			Matcher: &mock.HTTPMatcher{
				Method: method,
				Path:   path,
			},
			Response: &mock.HTTPResponse{
				StatusCode: status,
				Headers:    headers,
				Body:       body,
				DelayMs:    delayMs,
			},
		},
	}

	// Add via HTTP client
	createResult, err := server.adminClient.CreateMock(m)
	if err != nil {
		return ToolResultError("failed to create mock: " + err.Error()), nil
	}

	// Notify resource change
	server.NotifyResourceListChanged()

	result := map[string]interface{}{
		"created": createResult.Action == "created",
		"merged":  createResult.Action == "merged",
		"id":      createResult.Mock.ID,
		"path":    path,
		"method":  method,
	}
	if createResult.IsMerge() {
		result["message"] = createResult.Message
	}
	return ToolResultJSON(result)
}

// handleUpdateEndpoint updates an existing mock endpoint.
func handleUpdateEndpoint(args map[string]interface{}, session *MCPSession, server *Server) (*ToolResult, error) {
	if err := requireAdminClient(server); err != nil {
		return err, nil
	}

	id := getString(args, "id", "")
	if id == "" {
		return ToolResultError("id is required"), nil
	}

	existingMock, err := server.adminClient.GetMock(id)
	if err != nil {
		return ToolResultError("mock not found: " + id), nil
	}

	// Ensure HTTP spec exists
	if existingMock.HTTP == nil {
		existingMock.HTTP = &mock.HTTPSpec{}
	}

	// Apply updates
	if desc, ok := args["description"].(string); ok {
		existingMock.Name = desc
	}
	if priority, ok := args["priority"].(float64); ok {
		existingMock.HTTP.Priority = int(priority)
	}

	if responseArg := getMap(args, "response"); responseArg != nil {
		if existingMock.HTTP.Response == nil {
			existingMock.HTTP.Response = &mock.HTTPResponse{}
		}
		if status, ok := responseArg["status"].(float64); ok {
			existingMock.HTTP.Response.StatusCode = int(status)
		}
		if headers := getStringMap(responseArg, "headers"); headers != nil {
			existingMock.HTTP.Response.Headers = headers
		}
		if body, ok := responseArg["body"]; ok {
			switch v := body.(type) {
			case string:
				existingMock.HTTP.Response.Body = v
			default:
				bodyJSON, _ := json.Marshal(v)
				existingMock.HTTP.Response.Body = string(bodyJSON)
			}
		}
		if delay, ok := responseArg["delay"].(string); ok {
			d, _ := time.ParseDuration(delay)
			existingMock.HTTP.Response.DelayMs = int(d.Milliseconds())
		}
	}

	// Update via HTTP client
	if _, err := server.adminClient.UpdateMock(id, existingMock); err != nil {
		return ToolResultError("failed to update mock: " + err.Error()), nil
	}

	// Notify resource change
	server.NotifyResourceListChanged()

	result := map[string]interface{}{
		"updated": true,
		"id":      id,
	}
	return ToolResultJSON(result)
}

// handleDeleteEndpoint deletes a mock endpoint.
func handleDeleteEndpoint(args map[string]interface{}, session *MCPSession, server *Server) (*ToolResult, error) {
	if err := requireAdminClient(server); err != nil {
		return err, nil
	}

	id := getString(args, "id", "")
	if id == "" {
		return ToolResultError("id is required"), nil
	}

	if err := server.adminClient.DeleteMock(id); err != nil {
		return ToolResultError("failed to delete mock: " + err.Error()), nil
	}

	// Notify resource change
	server.NotifyResourceListChanged()

	result := map[string]interface{}{
		"deleted": true,
		"id":      id,
	}
	return ToolResultJSON(result)
}

// handleToggleEndpoint enables or disables a mock endpoint.
func handleToggleEndpoint(args map[string]interface{}, session *MCPSession, server *Server) (*ToolResult, error) {
	if err := requireAdminClient(server); err != nil {
		return err, nil
	}

	id := getString(args, "id", "")
	if id == "" {
		return ToolResultError("id is required"), nil
	}

	enabled := getBool(args, "enabled", true)

	mockCfg, err := server.adminClient.GetMock(id)
	if err != nil {
		return ToolResultError("mock not found: " + id), nil
	}

	mockCfg.Enabled = enabled

	if _, err := server.adminClient.UpdateMock(id, mockCfg); err != nil {
		return ToolResultError("failed to toggle mock: " + err.Error()), nil
	}

	// Notify resource change
	server.NotifyResourceListChanged()

	result := map[string]interface{}{
		"toggled": true,
		"id":      id,
		"enabled": enabled,
	}
	return ToolResultJSON(result)
}

// handleStatefulList lists items in a stateful resource.
func handleStatefulList(args map[string]interface{}, session *MCPSession, server *Server) (*ToolResult, error) {
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

	// Build query filter
	filter := &stateful.QueryFilter{
		Limit:  limit,
		Offset: offset,
		Sort:   getString(args, "sort", "createdAt"),
		Order:  getString(args, "order", "desc"),
	}

	// Get paginated items
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

// handleStatefulGet retrieves a specific item from a stateful resource.
func handleStatefulGet(args map[string]interface{}, session *MCPSession, server *Server) (*ToolResult, error) {
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

// handleStatefulCreate creates a new item in a stateful resource.
func handleStatefulCreate(args map[string]interface{}, session *MCPSession, server *Server) (*ToolResult, error) {
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
		return ToolResultError("failed to create item: " + err.Error()), nil
	}

	return ToolResultJSON(item)
}

// handleStatefulReset resets a stateful resource to seed data.
func handleStatefulReset(args map[string]interface{}, session *MCPSession, server *Server) (*ToolResult, error) {
	if err := requireStatefulStore(server); err != nil {
		return err, nil
	}

	resourceName := getString(args, "resource", "")

	response, err := server.statefulStore.Reset(resourceName)
	if err != nil {
		return ToolResultError("failed to reset: " + err.Error()), nil
	}

	return ToolResultJSON(response)
}

// handleGetRequestLogs retrieves captured request logs.
func handleGetRequestLogs(args map[string]interface{}, session *MCPSession, server *Server) (*ToolResult, error) {
	if err := requireAdminClient(server); err != nil {
		return err, nil
	}

	limit := getInt(args, "limit", 100)
	offset := getInt(args, "offset", 0)
	method := getString(args, "method", "")
	pathPrefix := getString(args, "pathPrefix", "")
	mockID := getString(args, "mockId", "")
	// Note: statusCode filtering not available via admin API

	// Build filter using cli.LogFilter
	filter := &cli.LogFilter{
		Limit:     limit,
		Offset:    offset,
		Method:    method,
		Path:      pathPrefix,
		MatchedID: mockID,
	}

	logsResult, err := server.adminClient.GetLogs(filter)
	if err != nil {
		return ToolResultError("failed to get logs: " + err.Error()), nil
	}

	// Convert to MCP format
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

// handleClearLogs clears all request logs.
func handleClearLogs(args map[string]interface{}, session *MCPSession, server *Server) (*ToolResult, error) {
	if err := requireAdminClient(server); err != nil {
		return err, nil
	}

	cleared, err := server.adminClient.ClearLogs()
	if err != nil {
		return ToolResultError("failed to clear logs: " + err.Error()), nil
	}

	result := map[string]interface{}{
		"cleared": cleared,
	}
	return ToolResultJSON(result)
}
