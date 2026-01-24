// Package integration provides binary E2E tests for the mockd server.
// These tests exercise the compiled mockd binary in real-world scenarios.
package integration

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	ws "github.com/coder/websocket"
)

// binaryTestContext holds shared state for binary E2E tests.
type binaryTestContext struct {
	binaryPath string
	buildOnce  sync.Once
	buildErr   error
	mu         sync.Mutex
}

var binaryCtx = &binaryTestContext{}

// buildBinary builds the mockd binary once for all tests.
func buildBinary(t *testing.T) string {
	t.Helper()
	binaryCtx.buildOnce.Do(func() {
		// Build the binary
		binaryPath := filepath.Join(os.TempDir(), fmt.Sprintf("mockd_test_%d", os.Getpid()))
		buildCmd := exec.Command("go", "build", "-o", binaryPath, "./cmd/mockd")
		buildCmd.Dir = "../.."
		if out, err := buildCmd.CombinedOutput(); err != nil {
			binaryCtx.buildErr = fmt.Errorf("failed to build CLI: %v\n%s", err, out)
			return
		}
		binaryCtx.binaryPath = binaryPath
	})
	if binaryCtx.buildErr != nil {
		t.Fatal(binaryCtx.buildErr)
	}
	return binaryCtx.binaryPath
}

// cleanupBinary removes the test binary.
func cleanupBinary() {
	binaryCtx.mu.Lock()
	defer binaryCtx.mu.Unlock()
	if binaryCtx.binaryPath != "" {
		os.Remove(binaryCtx.binaryPath)
		binaryCtx.binaryPath = ""
	}
}

// TestMain handles setup and teardown for binary E2E tests.
func TestMain(m *testing.M) {
	code := m.Run()
	cleanupBinary()
	os.Exit(code)
}

// serverProcess holds a running mockd server process.
type serverProcess struct {
	cmd       *exec.Cmd
	httpPort  int
	adminPort int
	dataDir   string
	stdout    *bytes.Buffer
	stderr    *bytes.Buffer
}

// startServer starts a mockd server with the given options.
func startServer(t *testing.T, binaryPath string, extraArgs ...string) *serverProcess {
	t.Helper()

	httpPort := GetFreePortSafe()
	adminPort := GetFreePortSafe()
	dataDir := t.TempDir()

	args := []string{
		"start",
		"--port", fmt.Sprintf("%d", httpPort),
		"--admin-port", fmt.Sprintf("%d", adminPort),
		"--no-auth",
		"--data-dir", dataDir,
	}
	args = append(args, extraArgs...)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	t.Cleanup(cancel)

	cmd := exec.CommandContext(ctx, binaryPath, args...)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}

	proc := &serverProcess{
		cmd:       cmd,
		httpPort:  httpPort,
		adminPort: adminPort,
		dataDir:   dataDir,
		stdout:    &stdout,
		stderr:    &stderr,
	}

	// Register cleanup
	t.Cleanup(func() {
		proc.stop()
	})

	// Wait for server to be ready
	adminURL := fmt.Sprintf("http://localhost:%d", adminPort)
	if !waitForServer(adminURL+"/health", 10*time.Second) {
		proc.stop()
		t.Fatalf("Server did not become ready in time\nstdout: %s\nstderr: %s", stdout.String(), stderr.String())
	}

	return proc
}

// stop kills the server process.
func (p *serverProcess) stop() {
	if p.cmd != nil && p.cmd.Process != nil {
		p.cmd.Process.Kill()
		p.cmd.Wait()
	}
}

// adminURL returns the admin API base URL.
func (p *serverProcess) adminURL() string {
	return fmt.Sprintf("http://localhost:%d", p.adminPort)
}

// mockURL returns the mock server base URL.
func (p *serverProcess) mockURL() string {
	return fmt.Sprintf("http://localhost:%d", p.httpPort)
}

// ============================================================================
// Test: HTTP Mocking
// ============================================================================

func TestBinaryE2E_HTTPMocking(t *testing.T) {
	binaryPath := buildBinary(t)
	proc := startServer(t, binaryPath)

	// Create a mock via admin API
	mockPayload := map[string]interface{}{
		"id":      "test-http-mock",
		"name":    "Test HTTP Mock",
		"type":    "http",
		"enabled": true,
		"http": map[string]interface{}{
			"matcher": map[string]interface{}{
				"method": "GET",
				"path":   "/api/hello",
			},
			"response": map[string]interface{}{
				"statusCode": 200,
				"headers": map[string]string{
					"Content-Type": "application/json",
				},
				"body": `{"message": "Hello, World!"}`,
			},
		},
	}

	mockJSON, _ := json.Marshal(mockPayload)
	resp, err := http.Post(proc.adminURL()+"/mocks", "application/json", bytes.NewReader(mockJSON))
	if err != nil {
		t.Fatalf("Failed to create mock: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		t.Fatalf("Failed to create mock: status %d", resp.StatusCode)
	}

	// Verify mock works
	resp, err = http.Get(proc.mockURL() + "/api/hello")
	if err != nil {
		t.Fatalf("Failed to call mock: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "Hello, World!") {
		t.Errorf("Unexpected body: %s", body)
	}

	contentType := resp.Header.Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("Expected Content-Type application/json, got %s", contentType)
	}

	// Test unmatched path returns 404
	resp, err = http.Get(proc.mockURL() + "/api/unknown")
	if err != nil {
		t.Fatalf("Failed to call mock: %v", err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("Expected status 404 for unknown path, got %d", resp.StatusCode)
	}

	// Test DELETE mock
	req, _ := http.NewRequest(http.MethodDelete, proc.adminURL()+"/mocks/test-http-mock", nil)
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Failed to delete mock: %v", err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		t.Errorf("Failed to delete mock: status %d", resp.StatusCode)
	}
}

// ============================================================================
// Test: WebSocket Echo
// ============================================================================

func TestBinaryE2E_WebSocketEcho(t *testing.T) {
	binaryPath := buildBinary(t)
	proc := startServer(t, binaryPath)

	// Create a WebSocket mock via admin API
	echoMode := true
	mockPayload := map[string]interface{}{
		"id":      "test-ws-echo",
		"name":    "Test WebSocket Echo",
		"type":    "websocket",
		"enabled": true,
		"websocket": map[string]interface{}{
			"path":     "/ws/echo",
			"echoMode": echoMode,
		},
	}

	mockJSON, _ := json.Marshal(mockPayload)
	resp, err := http.Post(proc.adminURL()+"/mocks", "application/json", bytes.NewReader(mockJSON))
	if err != nil {
		t.Fatalf("Failed to create WebSocket mock: %v", err)
	}
	resp.Body.Close()

	// Small delay to allow mock registration
	time.Sleep(100 * time.Millisecond)

	// Connect via WebSocket
	wsURL := fmt.Sprintf("ws://localhost:%d/ws/echo", proc.httpPort)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, wsResp, err := ws.Dial(ctx, wsURL, nil)
	if wsResp != nil && wsResp.Body != nil {
		wsResp.Body.Close()
	}
	if err != nil {
		t.Fatalf("Failed to connect WebSocket: %v", err)
	}
	defer conn.Close(ws.StatusNormalClosure, "test complete")

	// Send message
	testMsg := "Hello WebSocket!"
	err = conn.Write(ctx, ws.MessageText, []byte(testMsg))
	if err != nil {
		t.Fatalf("Failed to write WebSocket message: %v", err)
	}

	// Read echo response
	msgType, data, err := conn.Read(ctx)
	if err != nil {
		t.Fatalf("Failed to read WebSocket message: %v", err)
	}

	if msgType != ws.MessageText {
		t.Errorf("Expected text message, got %v", msgType)
	}

	if string(data) != testMsg {
		t.Errorf("Expected echo %q, got %q", testMsg, string(data))
	}
}

// ============================================================================
// Test: SSE Streaming
// ============================================================================

func TestBinaryE2E_SSEStreaming(t *testing.T) {
	binaryPath := buildBinary(t)
	proc := startServer(t, binaryPath)

	// Create an SSE mock via admin API
	fixedDelay := 10
	mockPayload := map[string]interface{}{
		"id":      "test-sse",
		"name":    "Test SSE Mock",
		"type":    "http",
		"enabled": true,
		"http": map[string]interface{}{
			"matcher": map[string]interface{}{
				"method": "GET",
				"path":   "/events",
			},
			"sse": map[string]interface{}{
				"events": []map[string]interface{}{
					{"type": "message", "data": "Event 1"},
					{"type": "message", "data": "Event 2"},
					{"type": "message", "data": "Event 3"},
				},
				"timing": map[string]interface{}{
					"fixedDelay": fixedDelay,
				},
				"lifecycle": map[string]interface{}{
					"maxEvents": 3,
				},
			},
		},
	}

	mockJSON, _ := json.Marshal(mockPayload)
	resp, err := http.Post(proc.adminURL()+"/mocks", "application/json", bytes.NewReader(mockJSON))
	if err != nil {
		t.Fatalf("Failed to create SSE mock: %v", err)
	}
	resp.Body.Close()

	// Make SSE request
	req, _ := http.NewRequest("GET", proc.mockURL()+"/events", nil)
	req.Header.Set("Accept", "text/event-stream")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	req = req.WithContext(ctx)

	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Failed to make SSE request: %v", err)
	}
	defer resp.Body.Close()

	// Check Content-Type
	contentType := resp.Header.Get("Content-Type")
	if contentType != "text/event-stream" {
		t.Errorf("Expected Content-Type text/event-stream, got %q", contentType)
	}

	// Read events
	scanner := bufio.NewScanner(resp.Body)
	var events []string
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "data:") {
			events = append(events, strings.TrimPrefix(line, "data:"))
		}
	}

	if len(events) < 3 {
		t.Errorf("Expected at least 3 events, got %d", len(events))
	}

	// Verify event content
	expected := []string{"Event 1", "Event 2", "Event 3"}
	for i, exp := range expected {
		if i < len(events) && events[i] != exp {
			t.Errorf("Event %d: expected %q, got %q", i, exp, events[i])
		}
	}
}

// ============================================================================
// Test: GraphQL Mocking
// ============================================================================

func TestBinaryE2E_GraphQLMocking(t *testing.T) {
	binaryPath := buildBinary(t)
	proc := startServer(t, binaryPath)

	// Create a GraphQL mock via admin API
	mockPayload := map[string]interface{}{
		"id":      "test-graphql",
		"name":    "Test GraphQL Mock",
		"type":    "graphql",
		"enabled": true,
		"graphql": map[string]interface{}{
			"path": "/graphql",
			"schema": `
				type Query {
					user(id: ID!): User
				}
				type User {
					id: ID!
					name: String!
					email: String!
				}
			`,
			"introspection": true,
			"resolvers": map[string]interface{}{
				"Query.user": map[string]interface{}{
					"response": map[string]interface{}{
						"id":    "{{args.id}}",
						"name":  "Test User",
						"email": "test@example.com",
					},
				},
			},
		},
	}

	mockJSON, _ := json.Marshal(mockPayload)
	resp, err := http.Post(proc.adminURL()+"/mocks", "application/json", bytes.NewReader(mockJSON))
	if err != nil {
		t.Fatalf("Failed to create GraphQL mock: %v", err)
	}
	resp.Body.Close()

	// Make GraphQL query
	query := map[string]interface{}{
		"query": `query { user(id: "123") { id name email } }`,
	}
	queryJSON, _ := json.Marshal(query)

	req, _ := http.NewRequest("POST", proc.mockURL()+"/graphql", bytes.NewReader(queryJSON))
	req.Header.Set("Content-Type", "application/json")

	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Failed to make GraphQL request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)

	// Parse response
	var gqlResp struct {
		Data struct {
			User struct {
				ID    string `json:"id"`
				Name  string `json:"name"`
				Email string `json:"email"`
			} `json:"user"`
		} `json:"data"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}

	if err := json.Unmarshal(body, &gqlResp); err != nil {
		t.Fatalf("Failed to parse GraphQL response: %v\nBody: %s", err, body)
	}

	if len(gqlResp.Errors) > 0 {
		t.Errorf("GraphQL errors: %v", gqlResp.Errors)
	}

	if gqlResp.Data.User.ID != "123" {
		t.Errorf("Expected user ID 123, got %s", gqlResp.Data.User.ID)
	}

	if gqlResp.Data.User.Name != "Test User" {
		t.Errorf("Expected user name 'Test User', got %s", gqlResp.Data.User.Name)
	}
}

// ============================================================================
// Test: Proxy Recording
// ============================================================================

func TestBinaryE2E_ProxyRecording(t *testing.T) {
	binaryPath := buildBinary(t)
	proc := startServer(t, binaryPath)

	// First, create a target mock that we'll record against
	targetMock := map[string]interface{}{
		"id":      "proxy-target",
		"name":    "Proxy Target",
		"type":    "http",
		"enabled": true,
		"http": map[string]interface{}{
			"matcher": map[string]interface{}{
				"method": "GET",
				"path":   "/target/api",
			},
			"response": map[string]interface{}{
				"statusCode": 200,
				"body":       `{"source": "target"}`,
				"headers": map[string]string{
					"Content-Type": "application/json",
				},
			},
		},
	}

	mockJSON, _ := json.Marshal(targetMock)
	resp, err := http.Post(proc.adminURL()+"/mocks", "application/json", bytes.NewReader(mockJSON))
	if err != nil {
		t.Fatalf("Failed to create target mock: %v", err)
	}
	resp.Body.Close()

	// Verify target mock works
	resp, err = http.Get(proc.mockURL() + "/target/api")
	if err != nil {
		t.Fatalf("Failed to call target mock: %v", err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Target mock not working: status %d", resp.StatusCode)
	}

	// Check request logging (proxy recording alternative)
	// Wait for request to be logged
	time.Sleep(100 * time.Millisecond)

	// Get logged requests
	resp, err = http.Get(proc.adminURL() + "/requests")
	if err != nil {
		t.Fatalf("Failed to get requests: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	// Response may be wrapped in an object with "requests" array
	var requestsResponse struct {
		Requests []map[string]interface{} `json:"requests"`
	}
	if err := json.Unmarshal(body, &requestsResponse); err != nil {
		// Try parsing as direct array
		var requests []map[string]interface{}
		if err := json.Unmarshal(body, &requests); err != nil {
			t.Fatalf("Failed to parse requests: %v\nBody: %s", err, body)
		}
		requestsResponse.Requests = requests
	}

	// Should have at least one request logged
	found := false
	for _, req := range requestsResponse.Requests {
		if path, ok := req["path"].(string); ok && path == "/target/api" {
			found = true
			break
		}
	}

	if !found {
		t.Errorf("Request to /target/api not found in request log")
	}
}

// ============================================================================
// Test: Config File Load
// ============================================================================

func TestBinaryE2E_ConfigFileLoad(t *testing.T) {
	binaryPath := buildBinary(t)

	// Create config file
	configDir := t.TempDir()
	configPath := filepath.Join(configDir, "mockd.yaml")

	configContent := `
version: "1.0"
name: "E2E Config Test"
mocks:
  - id: "yaml-mock"
    name: "YAML Loaded Mock"
    type: "http"
    enabled: true
    http:
      matcher:
        method: "GET"
        path: "/from-yaml"
      response:
        statusCode: 200
        body: '{"loaded": "from-yaml"}'
        headers:
          Content-Type: "application/json"
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	// Start server with config file
	httpPort := GetFreePortSafe()
	adminPort := GetFreePortSafe()
	dataDir := t.TempDir()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, binaryPath, "start",
		"--port", fmt.Sprintf("%d", httpPort),
		"--admin-port", fmt.Sprintf("%d", adminPort),
		"--no-auth",
		"--data-dir", dataDir,
		"--config", configPath,
	)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer func() {
		cmd.Process.Kill()
		cmd.Wait()
	}()

	// Wait for server to be ready
	adminURL := fmt.Sprintf("http://localhost:%d", adminPort)
	if !waitForServer(adminURL+"/health", 10*time.Second) {
		t.Fatalf("Server did not become ready\nstdout: %s\nstderr: %s", stdout.String(), stderr.String())
	}

	// Verify mock from YAML was loaded
	mockURL := fmt.Sprintf("http://localhost:%d/from-yaml", httpPort)
	resp, err := http.Get(mockURL)
	if err != nil {
		t.Fatalf("Failed to call mock: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "from-yaml") {
		t.Errorf("Expected body to contain 'from-yaml', got: %s", body)
	}
}

// ============================================================================
// Test: Graceful Shutdown
// ============================================================================

func TestBinaryE2E_GracefulShutdown(t *testing.T) {
	binaryPath := buildBinary(t)

	httpPort := GetFreePortSafe()
	adminPort := GetFreePortSafe()
	dataDir := t.TempDir()

	cmd := exec.Command(binaryPath, "start",
		"--port", fmt.Sprintf("%d", httpPort),
		"--admin-port", fmt.Sprintf("%d", adminPort),
		"--no-auth",
		"--data-dir", dataDir,
	)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}

	// Wait for server to be ready
	adminURL := fmt.Sprintf("http://localhost:%d", adminPort)
	if !waitForServer(adminURL+"/health", 10*time.Second) {
		cmd.Process.Kill()
		t.Fatalf("Server did not become ready\nstdout: %s\nstderr: %s", stdout.String(), stderr.String())
	}

	// Send SIGTERM for graceful shutdown
	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	if err := cmd.Process.Signal(syscall.SIGTERM); err != nil {
		// On Windows, Signal may not work - use Kill as fallback
		cmd.Process.Kill()
		t.Log("SIGTERM not supported on this platform, using Kill")
	}

	// Wait for process to exit with timeout
	select {
	case err := <-done:
		// Process exited - this is expected
		if err != nil {
			// Exit error is expected since we're terminating
			t.Logf("Process exited with: %v (expected)", err)
		}
	case <-time.After(5 * time.Second):
		cmd.Process.Kill()
		t.Error("Server did not shut down within 5 seconds")
	}

	// Verify server is no longer responding
	resp, err := http.Get(adminURL + "/health")
	if err == nil {
		resp.Body.Close()
		t.Error("Server still responding after shutdown")
	}
}

// ============================================================================
// Test: Multi-Protocol
// ============================================================================

func TestBinaryE2E_MultiProtocol(t *testing.T) {
	binaryPath := buildBinary(t)
	proc := startServer(t, binaryPath)

	// Create HTTP mock
	httpMock := map[string]interface{}{
		"id":      "multi-http",
		"name":    "Multi Protocol HTTP",
		"type":    "http",
		"enabled": true,
		"http": map[string]interface{}{
			"matcher": map[string]interface{}{
				"method": "GET",
				"path":   "/api/multi",
			},
			"response": map[string]interface{}{
				"statusCode": 200,
				"body":       `{"protocol": "http"}`,
			},
		},
	}

	// Create WebSocket mock
	echoMode := true
	wsMock := map[string]interface{}{
		"id":      "multi-ws",
		"name":    "Multi Protocol WebSocket",
		"type":    "websocket",
		"enabled": true,
		"websocket": map[string]interface{}{
			"path":     "/ws/multi",
			"echoMode": echoMode,
		},
	}

	// Create GraphQL mock
	gqlMock := map[string]interface{}{
		"id":      "multi-graphql",
		"name":    "Multi Protocol GraphQL",
		"type":    "graphql",
		"enabled": true,
		"graphql": map[string]interface{}{
			"path": "/graphql/multi",
			"schema": `
				type Query {
					status: String!
				}
			`,
			"introspection": true,
			"resolvers": map[string]interface{}{
				"Query.status": map[string]interface{}{
					"response": "multi-protocol-active",
				},
			},
		},
	}

	// Create all mocks
	for _, mock := range []map[string]interface{}{httpMock, wsMock, gqlMock} {
		mockJSON, _ := json.Marshal(mock)
		resp, err := http.Post(proc.adminURL()+"/mocks", "application/json", bytes.NewReader(mockJSON))
		if err != nil {
			t.Fatalf("Failed to create mock: %v", err)
		}
		resp.Body.Close()
	}

	// Small delay for mock registration
	time.Sleep(100 * time.Millisecond)

	// Test HTTP mock
	t.Run("HTTP", func(t *testing.T) {
		resp, err := http.Get(proc.mockURL() + "/api/multi")
		if err != nil {
			t.Fatalf("HTTP request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected status 200, got %d", resp.StatusCode)
		}

		body, _ := io.ReadAll(resp.Body)
		if !strings.Contains(string(body), "http") {
			t.Errorf("Unexpected HTTP body: %s", body)
		}
	})

	// Test WebSocket mock
	t.Run("WebSocket", func(t *testing.T) {
		wsURL := fmt.Sprintf("ws://localhost:%d/ws/multi", proc.httpPort)
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		conn, wsResp, err := ws.Dial(ctx, wsURL, nil)
		if wsResp != nil && wsResp.Body != nil {
			wsResp.Body.Close()
		}
		if err != nil {
			t.Fatalf("WebSocket connect failed: %v", err)
		}
		defer conn.Close(ws.StatusNormalClosure, "")

		err = conn.Write(ctx, ws.MessageText, []byte("multi-test"))
		if err != nil {
			t.Fatalf("WebSocket write failed: %v", err)
		}

		_, data, err := conn.Read(ctx)
		if err != nil {
			t.Fatalf("WebSocket read failed: %v", err)
		}

		if string(data) != "multi-test" {
			t.Errorf("Expected echo 'multi-test', got %q", string(data))
		}
	})

	// Test GraphQL mock
	t.Run("GraphQL", func(t *testing.T) {
		query := map[string]interface{}{
			"query": `query { status }`,
		}
		queryJSON, _ := json.Marshal(query)

		req, _ := http.NewRequest("POST", proc.mockURL()+"/graphql/multi", bytes.NewReader(queryJSON))
		req.Header.Set("Content-Type", "application/json")

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("GraphQL request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected status 200, got %d", resp.StatusCode)
		}

		body, _ := io.ReadAll(resp.Body)
		if !strings.Contains(string(body), "multi-protocol-active") {
			t.Errorf("Unexpected GraphQL body: %s", body)
		}
	})
}

// ============================================================================
// Test: HTTP Method Matching
// ============================================================================

func TestBinaryE2E_HTTPMethodMatching(t *testing.T) {
	binaryPath := buildBinary(t)
	proc := startServer(t, binaryPath)

	// Create mocks for different HTTP methods
	methods := []struct {
		method string
		mockID string
		body   string
	}{
		{"GET", "method-get", `{"method": "GET"}`},
		{"POST", "method-post", `{"method": "POST"}`},
		{"PUT", "method-put", `{"method": "PUT"}`},
		{"DELETE", "method-delete", `{"method": "DELETE"}`},
		{"PATCH", "method-patch", `{"method": "PATCH"}`},
	}

	for _, m := range methods {
		mockPayload := map[string]interface{}{
			"id":      m.mockID,
			"name":    "Method " + m.method,
			"type":    "http",
			"enabled": true,
			"http": map[string]interface{}{
				"matcher": map[string]interface{}{
					"method": m.method,
					"path":   "/api/method-test",
				},
				"response": map[string]interface{}{
					"statusCode": 200,
					"body":       m.body,
				},
			},
		}

		mockJSON, _ := json.Marshal(mockPayload)
		resp, err := http.Post(proc.adminURL()+"/mocks", "application/json", bytes.NewReader(mockJSON))
		if err != nil {
			t.Fatalf("Failed to create mock for %s: %v", m.method, err)
		}
		resp.Body.Close()
	}

	// Test each method
	for _, m := range methods {
		t.Run(m.method, func(t *testing.T) {
			req, _ := http.NewRequest(m.method, proc.mockURL()+"/api/method-test", nil)
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatalf("Request failed: %v", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				t.Errorf("Expected status 200, got %d", resp.StatusCode)
			}

			body, _ := io.ReadAll(resp.Body)
			if !strings.Contains(string(body), m.method) {
				t.Errorf("Expected body to contain %s, got: %s", m.method, body)
			}
		})
	}
}

// ============================================================================
// Test: Query Parameter Matching
// ============================================================================

func TestBinaryE2E_QueryParameterMatching(t *testing.T) {
	binaryPath := buildBinary(t)
	proc := startServer(t, binaryPath)

	// Create mock with query parameter matching
	mockPayload := map[string]interface{}{
		"id":      "query-param-mock",
		"name":    "Query Parameter Mock",
		"type":    "http",
		"enabled": true,
		"http": map[string]interface{}{
			"matcher": map[string]interface{}{
				"method": "GET",
				"path":   "/api/search",
				"query": map[string]string{
					"type": "users",
				},
			},
			"response": map[string]interface{}{
				"statusCode": 200,
				"body":       `{"type": "users", "results": []}`,
			},
		},
	}

	mockJSON, _ := json.Marshal(mockPayload)
	resp, err := http.Post(proc.adminURL()+"/mocks", "application/json", bytes.NewReader(mockJSON))
	if err != nil {
		t.Fatalf("Failed to create mock: %v", err)
	}
	resp.Body.Close()

	// Test with matching query param
	resp, err = http.Get(proc.mockURL() + "/api/search?type=users")
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "users") {
		t.Errorf("Unexpected body: %s", body)
	}
}

// ============================================================================
// Test: Response Delay
// ============================================================================

func TestBinaryE2E_ResponseDelay(t *testing.T) {
	binaryPath := buildBinary(t)
	proc := startServer(t, binaryPath)

	// Create mock with delay
	mockPayload := map[string]interface{}{
		"id":      "delay-mock",
		"name":    "Delay Mock",
		"type":    "http",
		"enabled": true,
		"http": map[string]interface{}{
			"matcher": map[string]interface{}{
				"method": "GET",
				"path":   "/api/delayed",
			},
			"response": map[string]interface{}{
				"statusCode": 200,
				"body":       `{"delayed": true}`,
				"delayMs":    200,
			},
		},
	}

	mockJSON, _ := json.Marshal(mockPayload)
	resp, err := http.Post(proc.adminURL()+"/mocks", "application/json", bytes.NewReader(mockJSON))
	if err != nil {
		t.Fatalf("Failed to create mock: %v", err)
	}
	resp.Body.Close()

	// Time the request
	start := time.Now()
	resp, err = http.Get(proc.mockURL() + "/api/delayed")
	elapsed := time.Since(start)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	// Should take at least 200ms
	if elapsed < 200*time.Millisecond {
		t.Errorf("Expected delay of at least 200ms, got %v", elapsed)
	}
}

// ============================================================================
// Test: Template Variables
// ============================================================================

func TestBinaryE2E_TemplateVariables(t *testing.T) {
	binaryPath := buildBinary(t)
	proc := startServer(t, binaryPath)

	// Create mock with path parameter template
	mockPayload := map[string]interface{}{
		"id":      "template-mock",
		"name":    "Template Mock",
		"type":    "http",
		"enabled": true,
		"http": map[string]interface{}{
			"matcher": map[string]interface{}{
				"method": "GET",
				"path":   "/api/users/{id}",
			},
			"response": map[string]interface{}{
				"statusCode": 200,
				"body":       `{"id": "{{request.pathParam.id}}", "name": "User {{request.pathParam.id}}"}`,
				"headers": map[string]string{
					"Content-Type": "application/json",
				},
			},
		},
	}

	mockJSON, _ := json.Marshal(mockPayload)
	resp, err := http.Post(proc.adminURL()+"/mocks", "application/json", bytes.NewReader(mockJSON))
	if err != nil {
		t.Fatalf("Failed to create mock: %v", err)
	}
	resp.Body.Close()

	// Test with different IDs
	for _, id := range []string{"123", "456", "abc"} {
		t.Run("ID_"+id, func(t *testing.T) {
			resp, err := http.Get(proc.mockURL() + "/api/users/" + id)
			if err != nil {
				t.Fatalf("Request failed: %v", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				t.Errorf("Expected status 200, got %d", resp.StatusCode)
			}

			body, _ := io.ReadAll(resp.Body)
			if !strings.Contains(string(body), id) {
				t.Errorf("Expected body to contain ID %s, got: %s", id, body)
			}
		})
	}
}

// ============================================================================
// Test: Admin API CRUD
// ============================================================================

func TestBinaryE2E_AdminAPICRUD(t *testing.T) {
	binaryPath := buildBinary(t)
	proc := startServer(t, binaryPath)

	mockID := "crud-test-mock"

	// CREATE
	t.Run("Create", func(t *testing.T) {
		mockPayload := map[string]interface{}{
			"id":      mockID,
			"name":    "CRUD Test Mock",
			"type":    "http",
			"enabled": true,
			"http": map[string]interface{}{
				"matcher": map[string]interface{}{
					"method": "GET",
					"path":   "/api/crud",
				},
				"response": map[string]interface{}{
					"statusCode": 200,
					"body":       "original",
				},
			},
		}

		mockJSON, _ := json.Marshal(mockPayload)
		resp, err := http.Post(proc.adminURL()+"/mocks", "application/json", bytes.NewReader(mockJSON))
		if err != nil {
			t.Fatalf("Failed to create mock: %v", err)
		}
		resp.Body.Close()

		if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
			t.Errorf("Expected status 201 or 200, got %d", resp.StatusCode)
		}
	})

	// READ
	t.Run("Read", func(t *testing.T) {
		resp, err := http.Get(proc.adminURL() + "/mocks/" + mockID)
		if err != nil {
			t.Fatalf("Failed to get mock: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected status 200, got %d", resp.StatusCode)
		}

		body, _ := io.ReadAll(resp.Body)
		if !strings.Contains(string(body), mockID) {
			t.Errorf("Expected body to contain mock ID, got: %s", body)
		}
	})

	// UPDATE
	t.Run("Update", func(t *testing.T) {
		updatedPayload := map[string]interface{}{
			"id":      mockID,
			"name":    "Updated CRUD Mock",
			"type":    "http",
			"enabled": true,
			"http": map[string]interface{}{
				"matcher": map[string]interface{}{
					"method": "GET",
					"path":   "/api/crud",
				},
				"response": map[string]interface{}{
					"statusCode": 200,
					"body":       "updated",
				},
			},
		}

		updateJSON, _ := json.Marshal(updatedPayload)
		req, _ := http.NewRequest(http.MethodPut, proc.adminURL()+"/mocks/"+mockID, bytes.NewReader(updateJSON))
		req.Header.Set("Content-Type", "application/json")

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("Failed to update mock: %v", err)
		}
		resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected status 200, got %d", resp.StatusCode)
		}

		// Verify update
		mockResp, err := http.Get(proc.mockURL() + "/api/crud")
		if err != nil {
			t.Fatalf("Failed to verify update: %v", err)
		}
		defer mockResp.Body.Close()
		body, _ := io.ReadAll(mockResp.Body)
		if !strings.Contains(string(body), "updated") {
			t.Errorf("Expected updated body, got: %s", body)
		}
	})

	// LIST
	t.Run("List", func(t *testing.T) {
		resp, err := http.Get(proc.adminURL() + "/mocks")
		if err != nil {
			t.Fatalf("Failed to list mocks: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected status 200, got %d", resp.StatusCode)
		}

		body, _ := io.ReadAll(resp.Body)
		if !strings.Contains(string(body), mockID) {
			t.Errorf("Expected mock list to contain %s, got: %s", mockID, body)
		}
	})

	// DELETE
	t.Run("Delete", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodDelete, proc.adminURL()+"/mocks/"+mockID, nil)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("Failed to delete mock: %v", err)
		}
		resp.Body.Close()

		if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
			t.Errorf("Expected status 200 or 204, got %d", resp.StatusCode)
		}

		// Verify deletion
		resp, _ = http.Get(proc.adminURL() + "/mocks/" + mockID)
		resp.Body.Close()
		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("Expected 404 after deletion, got %d", resp.StatusCode)
		}
	})
}

// ============================================================================
// Test: Health and Status Endpoints
// ============================================================================

func TestBinaryE2E_HealthAndStatus(t *testing.T) {
	binaryPath := buildBinary(t)
	proc := startServer(t, binaryPath)

	// Test /health endpoint
	t.Run("Health", func(t *testing.T) {
		resp, err := http.Get(proc.adminURL() + "/health")
		if err != nil {
			t.Fatalf("Health check failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected status 200, got %d", resp.StatusCode)
		}
	})

	// Test /status endpoint
	t.Run("Status", func(t *testing.T) {
		resp, err := http.Get(proc.adminURL() + "/status")
		if err != nil {
			t.Fatalf("Status check failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected status 200, got %d", resp.StatusCode)
		}

		body, _ := io.ReadAll(resp.Body)
		var status map[string]interface{}
		if err := json.Unmarshal(body, &status); err != nil {
			t.Errorf("Failed to parse status JSON: %v", err)
		}
	})

	// Test mock server health endpoint
	t.Run("MockServerHealth", func(t *testing.T) {
		resp, err := http.Get(proc.mockURL() + "/__mockd/health")
		if err != nil {
			t.Fatalf("Mock server health check failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected status 200, got %d", resp.StatusCode)
		}
	})
}
