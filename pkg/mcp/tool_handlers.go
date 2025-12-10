package mcp

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/getmockd/mockd/pkg/config"
	"github.com/getmockd/mockd/pkg/engine"
	"github.com/getmockd/mockd/pkg/stateful"
)

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

	if server.engine == nil {
		return ToolResultError("mock engine not available"), nil
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

	// Try to find a matching mock
	mocks := server.engine.ListMocks()
	for _, mock := range mocks {
		if !mock.Enabled {
			continue
		}
		if mock.Matcher == nil {
			continue
		}

		// Check method match
		if mock.Matcher.Method != "" && mock.Matcher.Method != method {
			continue
		}

		// Check path match
		if mock.Matcher.Path != "" && !matchPath(mock.Matcher.Path, path) {
			continue
		}

		// Found a match - return the response body
		if mock.Response != nil {
			result := map[string]interface{}{
				"status":  mock.Response.StatusCode,
				"body":    mock.Response.Body,
				"headers": mock.Response.Headers,
				"mockId":  mock.ID,
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
	if server.engine == nil {
		return ToolResultError("mock engine not available"), nil
	}

	methodFilter := getString(args, "method", "")
	pathPrefix := getString(args, "pathPrefix", "")
	enabledFilter := getBoolPtr(args, "enabled")

	mocks := server.engine.ListMocks()
	endpoints := make([]EndpointInfo, 0, len(mocks))

	for _, mock := range mocks {
		// Apply filters
		if methodFilter != "" && mock.Matcher != nil && mock.Matcher.Method != methodFilter {
			continue
		}
		if pathPrefix != "" && mock.Matcher != nil && !strings.HasPrefix(mock.Matcher.Path, pathPrefix) {
			continue
		}
		if enabledFilter != nil && mock.Enabled != *enabledFilter {
			continue
		}

		path := ""
		method := ""
		if mock.Matcher != nil {
			path = mock.Matcher.Path
			method = mock.Matcher.Method
		}

		endpoints = append(endpoints, EndpointInfo{
			ID:          mock.ID,
			Method:      method,
			Path:        path,
			Enabled:     mock.Enabled,
			Description: mock.Name,
			Priority:    mock.Priority,
			CreatedAt:   mock.CreatedAt,
			UpdatedAt:   mock.UpdatedAt,
		})
	}

	return ToolResultJSON(endpoints)
}

// handleCreateEndpoint creates a new mock endpoint.
func handleCreateEndpoint(args map[string]interface{}, session *MCPSession, server *Server) (*ToolResult, error) {
	if server.engine == nil {
		return ToolResultError("mock engine not available"), nil
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
	mock := &config.MockConfiguration{
		Name:     description,
		Priority: priority,
		Enabled:  true,
		Matcher: &config.RequestMatcher{
			Method: method,
			Path:   path,
		},
		Response: &config.ResponseDefinition{
			StatusCode: status,
			Headers:    headers,
			Body:       body,
			DelayMs:    delayMs,
		},
	}

	// Add to engine
	if err := server.engine.AddMock(mock); err != nil {
		return ToolResultError("failed to create mock: " + err.Error()), nil
	}

	// Notify resource change
	server.NotifyResourceListChanged()

	result := map[string]interface{}{
		"created": true,
		"id":      mock.ID,
		"path":    path,
		"method":  method,
	}
	return ToolResultJSON(result)
}

// handleUpdateEndpoint updates an existing mock endpoint.
func handleUpdateEndpoint(args map[string]interface{}, session *MCPSession, server *Server) (*ToolResult, error) {
	if server.engine == nil {
		return ToolResultError("mock engine not available"), nil
	}

	id := getString(args, "id", "")
	if id == "" {
		return ToolResultError("id is required"), nil
	}

	mock := server.engine.GetMock(id)
	if mock == nil {
		return ToolResultError("mock not found: " + id), nil
	}

	// Apply updates
	if desc, ok := args["description"].(string); ok {
		mock.Name = desc
	}
	if priority, ok := args["priority"].(float64); ok {
		mock.Priority = int(priority)
	}

	if responseArg := getMap(args, "response"); responseArg != nil {
		if mock.Response == nil {
			mock.Response = &config.ResponseDefinition{}
		}
		if status, ok := responseArg["status"].(float64); ok {
			mock.Response.StatusCode = int(status)
		}
		if headers := getStringMap(responseArg, "headers"); headers != nil {
			mock.Response.Headers = headers
		}
		if body, ok := responseArg["body"]; ok {
			switch v := body.(type) {
			case string:
				mock.Response.Body = v
			default:
				bodyJSON, _ := json.Marshal(v)
				mock.Response.Body = string(bodyJSON)
			}
		}
		if delay, ok := responseArg["delay"].(string); ok {
			d, _ := time.ParseDuration(delay)
			mock.Response.DelayMs = int(d.Milliseconds())
		}
	}

	// Update in engine
	if err := server.engine.UpdateMock(id, mock); err != nil {
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
	if server.engine == nil {
		return ToolResultError("mock engine not available"), nil
	}

	id := getString(args, "id", "")
	if id == "" {
		return ToolResultError("id is required"), nil
	}

	if err := server.engine.DeleteMock(id); err != nil {
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
	if server.engine == nil {
		return ToolResultError("mock engine not available"), nil
	}

	id := getString(args, "id", "")
	if id == "" {
		return ToolResultError("id is required"), nil
	}

	enabled := getBool(args, "enabled", true)

	mock := server.engine.GetMock(id)
	if mock == nil {
		return ToolResultError("mock not found: " + id), nil
	}

	mock.Enabled = enabled

	if err := server.engine.UpdateMock(id, mock); err != nil {
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
	if server.statefulStore == nil {
		return ToolResultError("stateful store not available"), nil
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
	if server.statefulStore == nil {
		return ToolResultError("stateful store not available"), nil
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
	if server.statefulStore == nil {
		return ToolResultError("stateful store not available"), nil
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
	if server.statefulStore == nil {
		return ToolResultError("stateful store not available"), nil
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
	if server.engine == nil {
		return ToolResultError("mock engine not available"), nil
	}

	limit := getInt(args, "limit", 100)
	offset := getInt(args, "offset", 0)
	method := getString(args, "method", "")
	pathPrefix := getString(args, "pathPrefix", "")
	mockID := getString(args, "mockId", "")
	statusCode := getInt(args, "statusCode", 0)

	// Build filter
	filter := &engine.RequestLogFilter{
		Limit:  limit,
		Offset: offset,
	}
	if method != "" {
		filter.Method = method
	}
	if pathPrefix != "" {
		filter.Path = pathPrefix
	}
	if mockID != "" {
		filter.MatchedID = mockID
	}
	if statusCode > 0 {
		filter.StatusCode = statusCode
	}

	logs := server.engine.GetRequestLogs(filter)

	// Convert to MCP format
	entries := make([]RequestLogEntry, 0, len(logs))
	for _, log := range logs {
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
	if server.engine == nil {
		return ToolResultError("mock engine not available"), nil
	}

	server.engine.ClearRequestLogs()

	result := map[string]interface{}{
		"cleared": true,
	}
	return ToolResultJSON(result)
}
