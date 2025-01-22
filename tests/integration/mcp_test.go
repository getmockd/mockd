package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/getmockd/mockd/pkg/admin"
	"github.com/getmockd/mockd/pkg/admin/engineclient"
	"github.com/getmockd/mockd/pkg/cli"
	"github.com/getmockd/mockd/pkg/config"
	"github.com/getmockd/mockd/pkg/engine"
	"github.com/getmockd/mockd/pkg/mcp"
	"github.com/getmockd/mockd/pkg/mock"
)

// testServer creates a test MCP server with admin API client.
// Returns MCP server, engine, and cleanup function.
func testServer(t *testing.T) (*mcp.Server, *engine.Server, func()) {
	t.Helper()

	// Create mock engine with control API on random port
	engineCfg := config.DefaultServerConfiguration()
	engineCfg.ManagementPort = getFreePort()
	eng := engine.NewServer(engineCfg)

	// Start the engine (starts control API)
	if err := eng.Start(); err != nil {
		t.Fatalf("failed to start engine: %v", err)
	}

	// Wait for engine management API to be ready
	engineURL := fmt.Sprintf("http://localhost:%d", engineCfg.ManagementPort)
	engClient := engineclient.New(engineURL)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	for {
		if err := engClient.Health(ctx); err == nil {
			break
		}
		if ctx.Err() != nil {
			eng.Stop()
			t.Fatalf("engine management API not ready: %v", ctx.Err())
		}
		time.Sleep(50 * time.Millisecond)
	}

	// Create admin API with engine client
	adminPort := getFreePort()
	tempDir := t.TempDir() // Use temp dir for test isolation
	adminAPI := admin.NewAdminAPI(adminPort,
		admin.WithLocalEngine(engineURL),
		admin.WithAPIKeyDisabled(),
		admin.WithDataDir(tempDir),
	)

	// Start admin API
	if err := adminAPI.Start(); err != nil {
		eng.Stop()
		t.Fatalf("failed to start admin API: %v", err)
	}

	// Create admin client pointing to admin API
	adminURL := fmt.Sprintf("http://localhost:%d", adminPort)
	adminClient := cli.NewAdminClient(adminURL)

	// Create MCP server with admin client
	mcpCfg := mcp.DefaultConfig()
	mcpCfg.Enabled = true
	mcpCfg.AllowRemote = true // Allow for testing since httptest uses test IP
	mcpCfg.AdminURL = adminURL

	mcpServer := mcp.NewServer(mcpCfg, adminClient, eng.StatefulStore())

	cleanup := func() {
		adminAPI.Stop()
		eng.Stop()
		// Small delay to ensure file handles are released before TempDir cleanup
		time.Sleep(10 * time.Millisecond)
	}

	return mcpServer, eng, cleanup
}

// sendJSONRPC sends a JSON-RPC request and returns the response.
func sendJSONRPC(t *testing.T, handler http.Handler, method string, params interface{}, sessionID string) *mcp.JSONRPCResponse {
	t.Helper()

	req := mcp.JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  method,
	}

	if params != nil {
		paramsJSON, err := json.Marshal(params)
		if err != nil {
			t.Fatalf("failed to marshal params: %v", err)
		}
		req.Params = paramsJSON
	}

	body, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("failed to marshal request: %v", err)
	}

	httpReq := httptest.NewRequest("POST", "/mcp", bytes.NewReader(body))
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json, text/event-stream")
	if sessionID != "" {
		httpReq.Header.Set("Mcp-Session-Id", sessionID)
	}

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, httpReq)

	var resp mcp.JSONRPCResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v (body: %s)", err, rr.Body.String())
	}

	return &resp
}

// initializeSession performs the initialize handshake and returns the session ID.
func initializeSession(t *testing.T, handler http.Handler) string {
	t.Helper()

	params := mcp.InitializeParams{
		ProtocolVersion: mcp.ProtocolVersion,
		Capabilities:    mcp.ClientCapabilities{},
		ClientInfo: mcp.ClientInfo{
			Name:    "test-client",
			Version: "1.0.0",
		},
	}

	paramsJSON, _ := json.Marshal(params)
	req := mcp.JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "initialize",
		Params:  paramsJSON,
	}

	body, _ := json.Marshal(req)
	httpReq := httptest.NewRequest("POST", "/mcp", bytes.NewReader(body))
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json, text/event-stream")

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, httpReq)

	sessionID := rr.Header().Get("Mcp-Session-Id")
	if sessionID == "" {
		t.Fatal("no session ID returned from initialize")
	}

	// Send initialized notification
	initNotif := mcp.JSONRPCRequest{
		JSONRPC: "2.0",
		Method:  "initialized",
	}
	notifBody, _ := json.Marshal(initNotif)
	notifReq := httptest.NewRequest("POST", "/mcp", bytes.NewReader(notifBody))
	notifReq.Header.Set("Content-Type", "application/json")
	notifReq.Header.Set("Mcp-Session-Id", sessionID)

	rr2 := httptest.NewRecorder()
	handler.ServeHTTP(rr2, notifReq)

	return sessionID
}

// Phase 3: User Story 1 - MCP Server Initialization and Discovery

func TestMCP_Initialize(t *testing.T) {
	mcpServer, _, cleanup := testServer(t)
	defer cleanup()
	handler := mcpServer.Handler()

	params := mcp.InitializeParams{
		ProtocolVersion: mcp.ProtocolVersion,
		Capabilities:    mcp.ClientCapabilities{},
		ClientInfo: mcp.ClientInfo{
			Name:    "test-client",
			Version: "1.0.0",
		},
	}

	resp := sendJSONRPC(t, handler, "initialize", params, "")

	if resp.Error != nil {
		t.Fatalf("unexpected error: %v", resp.Error)
	}

	// Verify result structure
	resultJSON, _ := json.Marshal(resp.Result)
	var result mcp.InitializeResult
	if err := json.Unmarshal(resultJSON, &result); err != nil {
		t.Fatalf("failed to unmarshal result: %v", err)
	}

	if result.ProtocolVersion != mcp.ProtocolVersion {
		t.Errorf("expected protocol version %s, got %s", mcp.ProtocolVersion, result.ProtocolVersion)
	}

	if result.ServerInfo.Name != "mockd" {
		t.Errorf("expected server name 'mockd', got %s", result.ServerInfo.Name)
	}

	if result.Capabilities.Tools == nil {
		t.Error("expected tools capability to be present")
	}

	if result.Capabilities.Resources == nil {
		t.Error("expected resources capability to be present")
	}
}

func TestMCP_Initialize_ReturnsCapabilities(t *testing.T) {
	mcpServer, _, cleanup := testServer(t)
	defer cleanup()
	handler := mcpServer.Handler()

	params := mcp.InitializeParams{
		ProtocolVersion: mcp.ProtocolVersion,
		Capabilities:    mcp.ClientCapabilities{},
		ClientInfo: mcp.ClientInfo{
			Name:    "test-client",
			Version: "1.0.0",
		},
	}

	resp := sendJSONRPC(t, handler, "initialize", params, "")

	if resp.Error != nil {
		t.Fatalf("unexpected error: %v", resp.Error)
	}

	resultJSON, _ := json.Marshal(resp.Result)
	var result mcp.InitializeResult
	json.Unmarshal(resultJSON, &result)

	if result.Capabilities.Tools == nil {
		t.Fatal("tools capability should be present")
	}

	if result.Capabilities.Resources == nil {
		t.Fatal("resources capability should be present")
	}

	if !result.Capabilities.Resources.ListChanged {
		t.Error("resources.listChanged should be true")
	}
}

func TestMCP_Initialized_Notification(t *testing.T) {
	mcpServer, _, cleanup := testServer(t)
	defer cleanup()
	handler := mcpServer.Handler()

	// First initialize
	initParams := mcp.InitializeParams{
		ProtocolVersion: mcp.ProtocolVersion,
		Capabilities:    mcp.ClientCapabilities{},
		ClientInfo: mcp.ClientInfo{
			Name:    "test-client",
			Version: "1.0.0",
		},
	}

	paramsJSON, _ := json.Marshal(initParams)
	req := mcp.JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "initialize",
		Params:  paramsJSON,
	}

	body, _ := json.Marshal(req)
	httpReq := httptest.NewRequest("POST", "/mcp", bytes.NewReader(body))
	httpReq.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, httpReq)

	sessionID := rr.Header().Get("Mcp-Session-Id")
	if sessionID == "" {
		t.Fatal("no session ID returned")
	}

	// Send initialized notification (no ID = notification)
	notif := mcp.JSONRPCRequest{
		JSONRPC: "2.0",
		Method:  "initialized",
	}

	notifBody, _ := json.Marshal(notif)
	notifReq := httptest.NewRequest("POST", "/mcp", bytes.NewReader(notifBody))
	notifReq.Header.Set("Content-Type", "application/json")
	notifReq.Header.Set("Mcp-Session-Id", sessionID)

	rr2 := httptest.NewRecorder()
	handler.ServeHTTP(rr2, notifReq)

	// Notifications should return 202 Accepted
	if rr2.Code != http.StatusAccepted {
		t.Errorf("expected status 202, got %d", rr2.Code)
	}
}

func TestMCP_Initialize_InvalidProtocolVersion(t *testing.T) {
	mcpServer, _, cleanup := testServer(t)
	defer cleanup()
	handler := mcpServer.Handler()

	params := mcp.InitializeParams{
		ProtocolVersion: "invalid-version",
		Capabilities:    mcp.ClientCapabilities{},
		ClientInfo: mcp.ClientInfo{
			Name:    "test-client",
			Version: "1.0.0",
		},
	}

	resp := sendJSONRPC(t, handler, "initialize", params, "")

	if resp.Error == nil {
		t.Fatal("expected error for invalid protocol version")
	}
}

func TestMCP_MultipleSessions(t *testing.T) {
	mcpServer, _, cleanup := testServer(t)
	defer cleanup()
	handler := mcpServer.Handler()

	// Create multiple sessions
	sessions := make([]string, 3)
	for i := 0; i < 3; i++ {
		sessions[i] = initializeSession(t, handler)
	}

	// Verify all sessions are different
	seen := make(map[string]bool)
	for _, s := range sessions {
		if seen[s] {
			t.Error("duplicate session ID")
		}
		seen[s] = true
	}

	// Verify each session can make requests independently
	for _, sessionID := range sessions {
		resp := sendJSONRPC(t, handler, "tools/list", nil, sessionID)
		if resp.Error != nil {
			t.Errorf("session %s failed: %v", sessionID, resp.Error)
		}
	}
}

// Phase 4: User Story 2 - Mock Data Retrieval via MCP Tools

func TestMCP_GetMockData(t *testing.T) {
	mcpServer, _, cleanup := testServer(t)
	defer cleanup()
	handler := mcpServer.Handler()

	// Add a mock via admin client
	testMock := &config.MockConfiguration{
		Enabled: true,
		Type:    mock.MockTypeHTTP,
		HTTP: &mock.HTTPSpec{
			Matcher: &mock.HTTPMatcher{
				Method: "GET",
				Path:   "/api/users",
			},
			Response: &mock.HTTPResponse{
				StatusCode: 200,
				Headers:    map[string]string{"Content-Type": "application/json"},
				Body:       `{"users":[{"id":1,"name":"Alice"}]}`,
			},
		},
	}
	if _, err := mcpServer.AdminClient().CreateMock(testMock); err != nil {
		t.Fatalf("failed to add mock: %v", err)
	}

	sessionID := initializeSession(t, handler)

	resp := sendJSONRPC(t, handler, "tools/call", mcp.ToolCallParams{
		Name:      "get_mock_data",
		Arguments: map[string]interface{}{"path": "/api/users", "method": "GET"},
	}, sessionID)

	if resp.Error != nil {
		t.Fatalf("unexpected error: %v", resp.Error)
	}

	resultJSON, _ := json.Marshal(resp.Result)
	var result mcp.ToolResult
	json.Unmarshal(resultJSON, &result)

	if result.IsError {
		t.Error("expected success, got error")
	}

	if len(result.Content) == 0 {
		t.Fatal("expected content in result")
	}

	if result.Content[0].Type != "text" {
		t.Errorf("expected text content, got %s", result.Content[0].Type)
	}
}

func TestMCP_GetMockData_MethodOverride(t *testing.T) {
	mcpServer, _, cleanup := testServer(t)
	defer cleanup()
	handler := mcpServer.Handler()

	// Add GET and POST mocks via admin client
	getMock := &config.MockConfiguration{
		Enabled: true,
		Type:    mock.MockTypeHTTP,
		HTTP: &mock.HTTPSpec{
			Matcher: &mock.HTTPMatcher{
				Method: "GET",
				Path:   "/api/users",
			},
			Response: &mock.HTTPResponse{
				StatusCode: 200,
				Body:       `{"action":"list"}`,
			},
		},
	}
	postMock := &config.MockConfiguration{
		Enabled: true,
		Type:    mock.MockTypeHTTP,
		HTTP: &mock.HTTPSpec{
			Matcher: &mock.HTTPMatcher{
				Method: "POST",
				Path:   "/api/users",
			},
			Response: &mock.HTTPResponse{
				StatusCode: 201,
				Body:       `{"action":"create"}`,
			},
		},
	}
	mcpServer.AdminClient().CreateMock(getMock)
	mcpServer.AdminClient().CreateMock(postMock)

	sessionID := initializeSession(t, handler)

	// Test POST method override
	resp := sendJSONRPC(t, handler, "tools/call", mcp.ToolCallParams{
		Name:      "get_mock_data",
		Arguments: map[string]interface{}{"path": "/api/users", "method": "POST"},
	}, sessionID)

	if resp.Error != nil {
		t.Fatalf("unexpected error: %v", resp.Error)
	}

	resultJSON, _ := json.Marshal(resp.Result)
	var result mcp.ToolResult
	json.Unmarshal(resultJSON, &result)

	if len(result.Content) == 0 || result.Content[0].Text == "" {
		t.Fatal("expected content")
	}

	// Verify we got the POST mock response (returns full mock info including status, body, headers, mockId)
	var mockResult map[string]interface{}
	if err := json.Unmarshal([]byte(result.Content[0].Text), &mockResult); err != nil {
		t.Fatalf("failed to parse mock result: %v", err)
	}
	if mockResult["body"] != `{"action":"create"}` {
		t.Errorf("expected POST body, got: %v", mockResult["body"])
	}
	if mockResult["status"] != float64(201) {
		t.Errorf("expected status 201, got: %v", mockResult["status"])
	}
}

func TestMCP_GetMockData_NoMatch(t *testing.T) {
	mcpServer, _, cleanup := testServer(t)
	defer cleanup()
	handler := mcpServer.Handler()

	sessionID := initializeSession(t, handler)

	resp := sendJSONRPC(t, handler, "tools/call", mcp.ToolCallParams{
		Name:      "get_mock_data",
		Arguments: map[string]interface{}{"path": "/api/nonexistent"},
	}, sessionID)

	if resp.Error != nil {
		t.Fatalf("unexpected JSON-RPC error: %v", resp.Error)
	}

	resultJSON, _ := json.Marshal(resp.Result)
	var result mcp.ToolResult
	json.Unmarshal(resultJSON, &result)

	if !result.IsError {
		t.Error("expected isError=true for no match")
	}
}

func TestMCP_ToolsList(t *testing.T) {
	mcpServer, _, cleanup := testServer(t)
	defer cleanup()
	handler := mcpServer.Handler()

	sessionID := initializeSession(t, handler)

	resp := sendJSONRPC(t, handler, "tools/list", nil, sessionID)

	if resp.Error != nil {
		t.Fatalf("unexpected error: %v", resp.Error)
	}

	resultJSON, _ := json.Marshal(resp.Result)
	var result mcp.ToolsListResult
	json.Unmarshal(resultJSON, &result)

	if len(result.Tools) == 0 {
		t.Fatal("expected tools to be listed")
	}

	// Verify get_mock_data tool is present
	found := false
	for _, tool := range result.Tools {
		if tool.Name == "get_mock_data" {
			found = true
			if tool.Description == "" {
				t.Error("tool should have description")
			}
			if tool.InputSchema == nil {
				t.Error("tool should have inputSchema")
			}
			break
		}
	}
	if !found {
		t.Error("get_mock_data tool not found")
	}
}

// Phase 5: User Story 3 - Mock Endpoint Listing and Discovery

func TestMCP_ResourcesList(t *testing.T) {
	mcpServer, _, cleanup := testServer(t)
	defer cleanup()
	handler := mcpServer.Handler()

	// Add mocks via admin client
	for i := 0; i < 3; i++ {
		testMock := &config.MockConfiguration{
			Enabled: true,
			Type:    mock.MockTypeHTTP,
			HTTP: &mock.HTTPSpec{
				Matcher: &mock.HTTPMatcher{
					Method: "GET",
					Path:   "/api/test" + string(rune('0'+i)),
				},
				Response: &mock.HTTPResponse{
					StatusCode: 200,
					Body:       "{}",
				},
			},
		}
		mcpServer.AdminClient().CreateMock(testMock)
	}

	sessionID := initializeSession(t, handler)

	resp := sendJSONRPC(t, handler, "resources/list", nil, sessionID)

	if resp.Error != nil {
		t.Fatalf("unexpected error: %v", resp.Error)
	}

	resultJSON, _ := json.Marshal(resp.Result)
	var result mcp.ResourcesListResult
	json.Unmarshal(resultJSON, &result)

	// Should have at least the 3 mock endpoints + system resources
	if len(result.Resources) < 3 {
		t.Errorf("expected at least 3 resources, got %d", len(result.Resources))
	}
}

func TestMCP_ResourcesList_MockURIs(t *testing.T) {
	mcpServer, _, cleanup := testServer(t)
	defer cleanup()
	handler := mcpServer.Handler()

	testMock := &config.MockConfiguration{
		Enabled: true,
		Type:    mock.MockTypeHTTP,
		HTTP: &mock.HTTPSpec{
			Matcher: &mock.HTTPMatcher{
				Method: "GET",
				Path:   "/api/users",
			},
			Response: &mock.HTTPResponse{
				StatusCode: 200,
				Body:       "{}",
			},
		},
	}
	mcpServer.AdminClient().CreateMock(testMock)

	sessionID := initializeSession(t, handler)

	resp := sendJSONRPC(t, handler, "resources/list", nil, sessionID)

	resultJSON, _ := json.Marshal(resp.Result)
	var result mcp.ResourcesListResult
	json.Unmarshal(resultJSON, &result)

	// Check for mock:// URI scheme
	found := false
	for _, res := range result.Resources {
		if res.URI == "mock:///api/users#GET" || res.URI == "mock:///api/users" {
			found = true
			if res.MimeType != "application/json" {
				t.Errorf("expected mimeType application/json, got %s", res.MimeType)
			}
			break
		}
	}
	if !found {
		t.Error("mock:///api/users resource not found")
	}
}

func TestMCP_ResourcesRead(t *testing.T) {
	mcpServer, _, cleanup := testServer(t)
	defer cleanup()
	handler := mcpServer.Handler()

	testMock := &config.MockConfiguration{
		Name:    "Test Users",
		Enabled: true,
		Type:    mock.MockTypeHTTP,
		HTTP: &mock.HTTPSpec{
			Matcher: &mock.HTTPMatcher{
				Method: "GET",
				Path:   "/api/users",
			},
			Response: &mock.HTTPResponse{
				StatusCode: 200,
				Headers:    map[string]string{"Content-Type": "application/json"},
				Body:       `{"users":[]}`,
			},
		},
	}
	mcpServer.AdminClient().CreateMock(testMock)

	sessionID := initializeSession(t, handler)

	resp := sendJSONRPC(t, handler, "resources/read", mcp.ResourceReadParams{
		URI: "mock:///api/users#GET",
	}, sessionID)

	if resp.Error != nil {
		t.Fatalf("unexpected error: %v", resp.Error)
	}

	resultJSON, _ := json.Marshal(resp.Result)
	var result mcp.ResourceReadResult
	json.Unmarshal(resultJSON, &result)

	if len(result.Contents) == 0 {
		t.Fatal("expected contents")
	}

	if result.Contents[0].URI == "" {
		t.Error("expected URI in content")
	}

	if result.Contents[0].Text == "" {
		t.Error("expected text content")
	}
}

func TestMCP_ResourcesRead_DynamicUpdates(t *testing.T) {
	mcpServer, _, cleanup := testServer(t)
	defer cleanup()
	handler := mcpServer.Handler()

	sessionID := initializeSession(t, handler)

	// Initially no mocks
	resp1 := sendJSONRPC(t, handler, "resources/list", nil, sessionID)
	resultJSON1, _ := json.Marshal(resp1.Result)
	var result1 mcp.ResourcesListResult
	json.Unmarshal(resultJSON1, &result1)
	initialCount := len(result1.Resources)

	// Add a mock via admin client
	testMock := &config.MockConfiguration{
		Enabled: true,
		Type:    mock.MockTypeHTTP,
		HTTP: &mock.HTTPSpec{
			Matcher: &mock.HTTPMatcher{
				Method: "GET",
				Path:   "/api/dynamic",
			},
			Response: &mock.HTTPResponse{
				StatusCode: 200,
				Body:       "{}",
			},
		},
	}
	mcpServer.AdminClient().CreateMock(testMock)

	// List again - should include new mock
	resp2 := sendJSONRPC(t, handler, "resources/list", nil, sessionID)
	resultJSON2, _ := json.Marshal(resp2.Result)
	var result2 mcp.ResourcesListResult
	json.Unmarshal(resultJSON2, &result2)

	if len(result2.Resources) != initialCount+1 {
		t.Errorf("expected %d resources after adding mock, got %d", initialCount+1, len(result2.Resources))
	}
}

// Additional integration tests for later phases can be added here...

// Helper for unused imports
var _ = io.EOF
var _ = time.Now
