// Package integration provides binary E2E tests for the mockd server.
// These tests exercise the compiled mockd binary in real-world scenarios.
package integration

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"math/big"
	"net"
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
	mqttclient "github.com/eclipse/paho.mqtt.golang"
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

// ============================================================================
// Test: MQTT Basic Pub/Sub
// ============================================================================

func TestBinaryE2E_MQTTBasicPubSub(t *testing.T) {
	binaryPath := buildBinary(t)
	proc := startServer(t, binaryPath)

	// Get a free port for the MQTT broker
	mqttPort := GetFreePortSafe()

	// Create an MQTT mock via admin API
	mockPayload := map[string]interface{}{
		"id":      "test-mqtt-pubsub",
		"name":    "Test MQTT Pub/Sub",
		"type":    "mqtt",
		"enabled": true,
		"mqtt": map[string]interface{}{
			"port": mqttPort,
		},
	}

	mockJSON, _ := json.Marshal(mockPayload)
	resp, err := http.Post(proc.adminURL()+"/mocks", "application/json", bytes.NewReader(mockJSON))
	if err != nil {
		t.Fatalf("Failed to create MQTT mock: %v", err)
	}
	respBody, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		t.Fatalf("Failed to create MQTT mock: status %d, body: %s", resp.StatusCode, respBody)
	}

	// Wait for MQTT broker to start
	if !waitForMQTTBroker(mqttPort, 10*time.Second) {
		t.Fatalf("MQTT broker did not become ready on port %d", mqttPort)
	}

	// Create MQTT client for subscriber
	subscriber := createMQTTClientForBinaryTest(t, mqttPort, "binary-subscriber")
	defer subscriber.Disconnect(250)

	// Channel to receive messages
	received := make(chan string, 1)

	// Subscribe to topic
	token := subscriber.Subscribe("test/binary/topic", 1, func(client mqttclient.Client, msg mqttclient.Message) {
		received <- string(msg.Payload())
	})
	if !token.WaitTimeout(5 * time.Second) {
		t.Fatal("Subscribe timeout")
	}
	if token.Error() != nil {
		t.Fatalf("Subscribe error: %v", token.Error())
	}

	// Create MQTT client for publisher
	publisher := createMQTTClientForBinaryTest(t, mqttPort, "binary-publisher")
	defer publisher.Disconnect(250)

	// Publish message
	testMessage := "Hello from binary E2E test!"
	pubToken := publisher.Publish("test/binary/topic", 1, false, testMessage)
	if !pubToken.WaitTimeout(5 * time.Second) {
		t.Fatal("Publish timeout")
	}
	if pubToken.Error() != nil {
		t.Fatalf("Publish error: %v", pubToken.Error())
	}

	// Verify message received
	select {
	case msg := <-received:
		if msg != testMessage {
			t.Errorf("Expected message %q, got %q", testMessage, msg)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Timeout waiting for message")
	}
}

// ============================================================================
// Test: MQTT QoS Levels
// ============================================================================

func TestBinaryE2E_MQTTQoSLevels(t *testing.T) {
	binaryPath := buildBinary(t)
	proc := startServer(t, binaryPath)

	// Get a free port for the MQTT broker
	mqttPort := GetFreePortSafe()

	// Create an MQTT mock via admin API
	mockPayload := map[string]interface{}{
		"id":      "test-mqtt-qos",
		"name":    "Test MQTT QoS",
		"type":    "mqtt",
		"enabled": true,
		"mqtt": map[string]interface{}{
			"port": mqttPort,
		},
	}

	mockJSON, _ := json.Marshal(mockPayload)
	resp, err := http.Post(proc.adminURL()+"/mocks", "application/json", bytes.NewReader(mockJSON))
	if err != nil {
		t.Fatalf("Failed to create MQTT mock: %v", err)
	}
	resp.Body.Close()

	// Wait for MQTT broker to start
	if !waitForMQTTBroker(mqttPort, 10*time.Second) {
		t.Fatalf("MQTT broker did not become ready on port %d", mqttPort)
	}

	// Test each QoS level
	testCases := []struct {
		name        string
		qos         byte
		description string
	}{
		{"QoS0_AtMostOnce", 0, "Fire and forget"},
		{"QoS1_AtLeastOnce", 1, "Acknowledged delivery"},
		{"QoS2_ExactlyOnce", 2, "Assured delivery"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			client := createMQTTClientForBinaryTest(t, mqttPort, fmt.Sprintf("qos%d-client", tc.qos))
			defer client.Disconnect(250)

			received := make(chan mqttclient.Message, 1)
			topic := fmt.Sprintf("qos/test/%d", tc.qos)

			// Subscribe with the test QoS level
			token := client.Subscribe(topic, tc.qos, func(c mqttclient.Client, msg mqttclient.Message) {
				received <- msg
			})
			if !token.WaitTimeout(5 * time.Second) {
				t.Fatal("Subscribe timeout")
			}
			if token.Error() != nil {
				t.Fatalf("Subscribe error: %v", token.Error())
			}

			// Publish with the test QoS level
			payload := fmt.Sprintf("QoS %d message: %s", tc.qos, tc.description)
			pubToken := client.Publish(topic, tc.qos, false, payload)
			if !pubToken.WaitTimeout(5 * time.Second) {
				t.Fatal("Publish timeout")
			}
			if pubToken.Error() != nil {
				t.Fatalf("Publish error: %v", pubToken.Error())
			}

			// Verify message received
			select {
			case msg := <-received:
				if string(msg.Payload()) != payload {
					t.Errorf("Expected payload %q, got %q", payload, string(msg.Payload()))
				}
				// Verify QoS level (effective QoS is min of publish and subscribe QoS)
				if msg.Qos() != tc.qos {
					t.Errorf("Expected QoS %d, got %d", tc.qos, msg.Qos())
				}
			case <-time.After(5 * time.Second):
				t.Fatalf("Timeout waiting for %s message", tc.name)
			}
		})
	}

	// Test QoS downgrade: publish at QoS 2, subscribe at QoS 0
	t.Run("QoSDowngrade", func(t *testing.T) {
		client := createMQTTClientForBinaryTest(t, mqttPort, "qos-downgrade-client")
		defer client.Disconnect(250)

		received := make(chan mqttclient.Message, 1)
		topic := "qos/downgrade/test"

		// Subscribe with QoS 0
		token := client.Subscribe(topic, 0, func(c mqttclient.Client, msg mqttclient.Message) {
			received <- msg
		})
		if !token.WaitTimeout(5 * time.Second) {
			t.Fatal("Subscribe timeout")
		}

		// Publish with QoS 2
		client.Publish(topic, 2, false, "downgrade test").Wait()

		select {
		case msg := <-received:
			// Message should be downgraded to QoS 0
			if msg.Qos() != 0 {
				t.Errorf("Expected QoS to be downgraded to 0, got %d", msg.Qos())
			}
		case <-time.After(3 * time.Second):
			t.Fatal("Timeout waiting for downgraded message")
		}
	})
}

// ============================================================================
// Test: MQTT Retained Messages
// ============================================================================

func TestBinaryE2E_MQTTRetainedMessages(t *testing.T) {
	binaryPath := buildBinary(t)
	proc := startServer(t, binaryPath)

	// Get a free port for the MQTT broker
	mqttPort := GetFreePortSafe()

	// Create an MQTT mock via admin API
	mockPayload := map[string]interface{}{
		"id":      "test-mqtt-retained",
		"name":    "Test MQTT Retained",
		"type":    "mqtt",
		"enabled": true,
		"mqtt": map[string]interface{}{
			"port": mqttPort,
		},
	}

	mockJSON, _ := json.Marshal(mockPayload)
	resp, err := http.Post(proc.adminURL()+"/mocks", "application/json", bytes.NewReader(mockJSON))
	if err != nil {
		t.Fatalf("Failed to create MQTT mock: %v", err)
	}
	resp.Body.Close()

	// Wait for MQTT broker to start
	if !waitForMQTTBroker(mqttPort, 10*time.Second) {
		t.Fatalf("MQTT broker did not become ready on port %d", mqttPort)
	}

	// Test basic retained message functionality
	t.Run("RetainedMessageDelivery", func(t *testing.T) {
		// First client publishes retained message
		publisher := createMQTTClientForBinaryTest(t, mqttPort, "retain-publisher")
		retainedValue := "retained sensor value: 42.5"
		publisher.Publish("retain/sensor/temp", 1, true, retainedValue).Wait()
		publisher.Disconnect(250)

		// Wait for message to be stored
		time.Sleep(200 * time.Millisecond)

		// New client subscribes and should immediately receive retained message
		received := make(chan string, 1)
		subscriber := createMQTTClientForBinaryTest(t, mqttPort, "retain-subscriber")
		defer subscriber.Disconnect(250)

		subscriber.Subscribe("retain/sensor/temp", 1, func(c mqttclient.Client, msg mqttclient.Message) {
			received <- string(msg.Payload())
		}).Wait()

		select {
		case msg := <-received:
			if msg != retainedValue {
				t.Errorf("Expected retained message %q, got %q", retainedValue, msg)
			}
		case <-time.After(3 * time.Second):
			t.Fatal("Timeout waiting for retained message")
		}
	})

	// Test retained message overwrite
	t.Run("RetainedMessageOverwrite", func(t *testing.T) {
		publisher := createMQTTClientForBinaryTest(t, mqttPort, "overwrite-publisher")

		// Publish first retained message
		publisher.Publish("retain/overwrite", 1, true, "first value").Wait()
		time.Sleep(100 * time.Millisecond)

		// Publish second retained message (should overwrite)
		publisher.Publish("retain/overwrite", 1, true, "second value").Wait()
		publisher.Disconnect(250)

		time.Sleep(200 * time.Millisecond)

		// New subscriber should receive only the latest retained message
		received := make(chan string, 5)
		subscriber := createMQTTClientForBinaryTest(t, mqttPort, "overwrite-subscriber")
		defer subscriber.Disconnect(250)

		subscriber.Subscribe("retain/overwrite", 1, func(c mqttclient.Client, msg mqttclient.Message) {
			received <- string(msg.Payload())
		}).Wait()

		select {
		case msg := <-received:
			if msg != "second value" {
				t.Errorf("Expected retained message 'second value', got %q", msg)
			}
		case <-time.After(3 * time.Second):
			t.Fatal("Timeout waiting for retained message")
		}

		// Verify no more messages (only one retained message)
		select {
		case <-received:
			t.Fatal("Should not receive multiple retained messages")
		case <-time.After(300 * time.Millisecond):
			// Expected - no more messages
		}
	})

	// Test retained message clear (publish empty payload with retain flag)
	t.Run("RetainedMessageClear", func(t *testing.T) {
		publisher := createMQTTClientForBinaryTest(t, mqttPort, "clear-publisher")

		// Publish retained message
		publisher.Publish("retain/clear", 1, true, "value to clear").Wait()
		time.Sleep(100 * time.Millisecond)

		// Clear retained message by publishing empty payload with retain flag
		publisher.Publish("retain/clear", 1, true, "").Wait()
		publisher.Disconnect(250)

		time.Sleep(200 * time.Millisecond)

		// New subscriber should not receive any retained message (or empty)
		received := make(chan string, 1)
		subscriber := createMQTTClientForBinaryTest(t, mqttPort, "clear-subscriber")
		defer subscriber.Disconnect(250)

		subscriber.Subscribe("retain/clear", 1, func(c mqttclient.Client, msg mqttclient.Message) {
			received <- string(msg.Payload())
		}).Wait()

		select {
		case msg := <-received:
			// Some brokers may deliver the empty retained message
			if msg != "" {
				t.Errorf("Expected empty or no retained message after clear, got %q", msg)
			}
		case <-time.After(500 * time.Millisecond):
			// Expected - no retained message after clear
		}
	})

	// Test retained messages with wildcard subscription
	t.Run("RetainedMessagesWithWildcard", func(t *testing.T) {
		publisher := createMQTTClientForBinaryTest(t, mqttPort, "wildcard-publisher")

		// Publish retained messages to multiple topics
		publisher.Publish("retain/wildcard/topic1", 1, true, "value1").Wait()
		publisher.Publish("retain/wildcard/topic2", 1, true, "value2").Wait()
		publisher.Publish("retain/wildcard/topic3", 1, true, "value3").Wait()
		publisher.Disconnect(250)

		time.Sleep(300 * time.Millisecond)

		// New subscriber using wildcard should receive all retained messages
		received := make(chan string, 10)
		subscriber := createMQTTClientForBinaryTest(t, mqttPort, "wildcard-subscriber")
		defer subscriber.Disconnect(250)

		subscriber.Subscribe("retain/wildcard/#", 1, func(c mqttclient.Client, msg mqttclient.Message) {
			received <- string(msg.Payload())
		}).Wait()

		// Wait for retained messages
		time.Sleep(500 * time.Millisecond)

		// Disconnect before draining to prevent race
		subscriber.Disconnect(100)
		time.Sleep(50 * time.Millisecond)

		// Collect all received messages
		messages := []string{}
	drainLoop:
		for {
			select {
			case msg := <-received:
				messages = append(messages, msg)
			default:
				break drainLoop
			}
		}

		if len(messages) != 3 {
			t.Errorf("Expected 3 retained messages, got %d: %v", len(messages), messages)
		}

		// Verify all values are present
		hasValue1, hasValue2, hasValue3 := false, false, false
		for _, msg := range messages {
			switch msg {
			case "value1":
				hasValue1 = true
			case "value2":
				hasValue2 = true
			case "value3":
				hasValue3 = true
			}
		}
		if !hasValue1 || !hasValue2 || !hasValue3 {
			t.Errorf("Missing retained messages: value1=%v, value2=%v, value3=%v", hasValue1, hasValue2, hasValue3)
		}
	})
}

// ============================================================================
// MQTT Test Helpers
// ============================================================================

// waitForMQTTBroker waits for an MQTT broker to become available on the given port.
func waitForMQTTBroker(port int, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", fmt.Sprintf("localhost:%d", port), 500*time.Millisecond)
		if err == nil {
			conn.Close()
			// Give the broker a moment to fully initialize after accepting connections
			time.Sleep(100 * time.Millisecond)
			return true
		}
		time.Sleep(100 * time.Millisecond)
	}
	return false
}

// createMQTTClientForBinaryTest creates an MQTT client for binary E2E testing.
func createMQTTClientForBinaryTest(t *testing.T, port int, clientID string) mqttclient.Client {
	t.Helper()

	opts := mqttclient.NewClientOptions()
	opts.AddBroker(fmt.Sprintf("tcp://localhost:%d", port))
	opts.SetClientID(clientID)
	opts.SetAutoReconnect(false)
	opts.SetConnectTimeout(5 * time.Second)

	client := mqttclient.NewClient(opts)
	token := client.Connect()
	if !token.WaitTimeout(5 * time.Second) {
		t.Fatalf("MQTT connect timeout for client %s", clientID)
	}
	if token.Error() != nil {
		t.Fatalf("MQTT connect error for client %s: %v", clientID, token.Error())
	}

	return client
}

// ============================================================================
// Test: HTTPS with Auto-Generated Certificate
// ============================================================================

func TestBinaryE2E_HTTPSAutoGeneratedCert(t *testing.T) {
	binaryPath := buildBinary(t)

	httpPort := GetFreePortSafe()
	httpsPort := GetFreePortSafe()
	adminPort := GetFreePortSafe()
	dataDir := t.TempDir()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, binaryPath, "start",
		"--port", fmt.Sprintf("%d", httpPort),
		"--https-port", fmt.Sprintf("%d", httpsPort),
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
	defer func() {
		cmd.Process.Kill()
		cmd.Wait()
	}()

	// Wait for server to be ready (check HTTP admin first)
	adminURL := fmt.Sprintf("http://localhost:%d", adminPort)
	if !waitForServer(adminURL+"/health", 10*time.Second) {
		t.Fatalf("Server did not become ready\nstdout: %s\nstderr: %s", stdout.String(), stderr.String())
	}

	// Create an HTTP mock via admin API
	mockPayload := map[string]interface{}{
		"id":      "https-auto-cert-mock",
		"name":    "HTTPS Auto Cert Mock",
		"type":    "http",
		"enabled": true,
		"http": map[string]interface{}{
			"matcher": map[string]interface{}{
				"method": "GET",
				"path":   "/api/secure",
			},
			"response": map[string]interface{}{
				"statusCode": 200,
				"body":       `{"secure": true, "method": "auto-cert"}`,
				"headers": map[string]string{
					"Content-Type": "application/json",
				},
			},
		},
	}

	mockJSON, _ := json.Marshal(mockPayload)
	resp, err := http.Post(adminURL+"/mocks", "application/json", bytes.NewReader(mockJSON))
	if err != nil {
		t.Fatalf("Failed to create mock: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		t.Fatalf("Failed to create mock: status %d", resp.StatusCode)
	}

	// Wait for HTTPS server to be ready
	httpsURL := fmt.Sprintf("https://localhost:%d", httpsPort)
	httpClient := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		},
		Timeout: 5 * time.Second,
	}

	// Wait for HTTPS endpoint to become available
	deadline := time.Now().Add(10 * time.Second)
	var lastErr error
	for time.Now().Before(deadline) {
		resp, err := httpClient.Get(httpsURL + "/__mockd/health")
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				break
			}
		}
		lastErr = err
		time.Sleep(200 * time.Millisecond)
	}

	// Test HTTPS endpoint with InsecureSkipVerify
	t.Run("HTTPSRequest", func(t *testing.T) {
		resp, err := httpClient.Get(httpsURL + "/api/secure")
		if err != nil {
			t.Fatalf("HTTPS request failed: %v (last wait error: %v)", err, lastErr)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected status 200, got %d", resp.StatusCode)
		}

		body, _ := io.ReadAll(resp.Body)
		if !strings.Contains(string(body), "auto-cert") {
			t.Errorf("Unexpected body: %s", body)
		}

		// Verify we're actually using TLS
		if resp.TLS == nil {
			t.Error("Expected TLS connection, got nil TLS state")
		}
	})

	// Test that HTTP still works alongside HTTPS
	t.Run("HTTPStillWorks", func(t *testing.T) {
		httpURL := fmt.Sprintf("http://localhost:%d", httpPort)
		resp, err := http.Get(httpURL + "/api/secure")
		if err != nil {
			t.Fatalf("HTTP request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected status 200, got %d", resp.StatusCode)
		}
	})
}

// ============================================================================
// Test: HTTPS with Custom Certificate
// ============================================================================

func TestBinaryE2E_HTTPSCustomCert(t *testing.T) {
	binaryPath := buildBinary(t)

	// Generate test certificate and key
	certDir := t.TempDir()
	certFile := filepath.Join(certDir, "server.crt")
	keyFile := filepath.Join(certDir, "server.key")

	if err := generateTestCertificate(certFile, keyFile); err != nil {
		t.Fatalf("Failed to generate test certificate: %v", err)
	}

	httpPort := GetFreePortSafe()
	httpsPort := GetFreePortSafe()
	adminPort := GetFreePortSafe()
	dataDir := t.TempDir()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, binaryPath, "start",
		"--port", fmt.Sprintf("%d", httpPort),
		"--https-port", fmt.Sprintf("%d", httpsPort),
		"--admin-port", fmt.Sprintf("%d", adminPort),
		"--tls-cert", certFile,
		"--tls-key", keyFile,
		"--no-auth",
		"--data-dir", dataDir,
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

	// Create an HTTP mock via admin API
	mockPayload := map[string]interface{}{
		"id":      "https-custom-cert-mock",
		"name":    "HTTPS Custom Cert Mock",
		"type":    "http",
		"enabled": true,
		"http": map[string]interface{}{
			"matcher": map[string]interface{}{
				"method": "GET",
				"path":   "/api/custom-secure",
			},
			"response": map[string]interface{}{
				"statusCode": 200,
				"body":       `{"secure": true, "method": "custom-cert"}`,
				"headers": map[string]string{
					"Content-Type": "application/json",
				},
			},
		},
	}

	mockJSON, _ := json.Marshal(mockPayload)
	resp, err := http.Post(adminURL+"/mocks", "application/json", bytes.NewReader(mockJSON))
	if err != nil {
		t.Fatalf("Failed to create mock: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		t.Fatalf("Failed to create mock: status %d", resp.StatusCode)
	}

	// Create HTTPS client with InsecureSkipVerify (since we're using self-signed cert)
	httpsURL := fmt.Sprintf("https://localhost:%d", httpsPort)
	httpClient := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		},
		Timeout: 5 * time.Second,
	}

	// Wait for HTTPS endpoint to become available
	deadline := time.Now().Add(10 * time.Second)
	var lastErr error
	for time.Now().Before(deadline) {
		resp, err := httpClient.Get(httpsURL + "/__mockd/health")
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				break
			}
		}
		lastErr = err
		time.Sleep(200 * time.Millisecond)
	}

	// Test HTTPS endpoint with custom certificate
	t.Run("HTTPSWithCustomCert", func(t *testing.T) {
		resp, err := httpClient.Get(httpsURL + "/api/custom-secure")
		if err != nil {
			t.Fatalf("HTTPS request failed: %v (last wait error: %v)", err, lastErr)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected status 200, got %d", resp.StatusCode)
		}

		body, _ := io.ReadAll(resp.Body)
		if !strings.Contains(string(body), "custom-cert") {
			t.Errorf("Unexpected body: %s", body)
		}

		// Verify we're using TLS
		if resp.TLS == nil {
			t.Error("Expected TLS connection, got nil TLS state")
		}
	})

	// Test that the custom certificate is being used (verify CN or other cert properties)
	t.Run("VerifyCustomCertificate", func(t *testing.T) {
		// Create a custom dialer to inspect the certificate
		conn, err := tls.Dial("tcp", fmt.Sprintf("localhost:%d", httpsPort), &tls.Config{
			InsecureSkipVerify: true,
		})
		if err != nil {
			t.Fatalf("Failed to establish TLS connection: %v", err)
		}
		defer conn.Close()

		// Get the peer certificate
		state := conn.ConnectionState()
		if len(state.PeerCertificates) == 0 {
			t.Fatal("No peer certificates received")
		}

		cert := state.PeerCertificates[0]

		// Verify the certificate has the expected Common Name
		if cert.Subject.CommonName != "localhost" {
			t.Errorf("Expected CN 'localhost', got '%s'", cert.Subject.CommonName)
		}

		// Verify it's a valid certificate (not expired)
		now := time.Now()
		if now.Before(cert.NotBefore) || now.After(cert.NotAfter) {
			t.Errorf("Certificate validity period issue: NotBefore=%v, NotAfter=%v, Now=%v",
				cert.NotBefore, cert.NotAfter, now)
		}
	})
}

// generateTestCertificate creates a self-signed certificate and key for testing
func generateTestCertificate(certFile, keyFile string) error {
	// Generate RSA private key
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return fmt.Errorf("failed to generate private key: %w", err)
	}

	// Create certificate template
	serialNumber, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return fmt.Errorf("failed to generate serial number: %w", err)
	}

	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{"Test Organization"},
			CommonName:   "localhost",
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IPAddresses:           []net.IP{net.ParseIP("127.0.0.1"), net.ParseIP("::1")},
		DNSNames:              []string{"localhost"},
	}

	// Create self-signed certificate
	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &privateKey.PublicKey, privateKey)
	if err != nil {
		return fmt.Errorf("failed to create certificate: %w", err)
	}

	// Write certificate to file
	certOut, err := os.Create(certFile)
	if err != nil {
		return fmt.Errorf("failed to create cert file: %w", err)
	}
	defer certOut.Close()

	if err := pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: certDER}); err != nil {
		return fmt.Errorf("failed to write cert: %w", err)
	}

	// Write private key to file
	keyOut, err := os.Create(keyFile)
	if err != nil {
		return fmt.Errorf("failed to create key file: %w", err)
	}
	defer keyOut.Close()

	privateKeyBytes, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		return fmt.Errorf("failed to marshal private key: %w", err)
	}

	if err := pem.Encode(keyOut, &pem.Block{Type: "PRIVATE KEY", Bytes: privateKeyBytes}); err != nil {
		return fmt.Errorf("failed to write key: %w", err)
	}

	return nil
}

// ============================================================================
// Test: Import OpenAPI via CLI Binary
// ============================================================================

func TestBinaryE2E_ImportOpenAPI(t *testing.T) {
	binaryPath := buildBinary(t)
	proc := startServer(t, binaryPath)

	// Create OpenAPI spec file
	openAPISpec := `openapi: "3.0.3"
info:
  title: Test API
  version: "1.0.0"
paths:
  /api/users:
    get:
      operationId: listUsers
      summary: List all users
      responses:
        "200":
          description: Success
          content:
            application/json:
              schema:
                type: array
                items:
                  type: object
                  properties:
                    id:
                      type: string
                    name:
                      type: string
              example:
                - id: "user-1"
                  name: "Alice"
                - id: "user-2"
                  name: "Bob"
  /api/users/{userId}:
    get:
      operationId: getUser
      summary: Get user by ID
      parameters:
        - name: userId
          in: path
          required: true
          schema:
            type: string
      responses:
        "200":
          description: Success
          content:
            application/json:
              schema:
                type: object
                properties:
                  id:
                    type: string
                  name:
                    type: string
              example:
                id: "user-123"
                name: "Test User"
`
	specDir := t.TempDir()
	specPath := filepath.Join(specDir, "openapi.yaml")
	if err := os.WriteFile(specPath, []byte(openAPISpec), 0644); err != nil {
		t.Fatalf("Failed to write OpenAPI spec: %v", err)
	}

	// Run mockd import command
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	importCmd := exec.CommandContext(ctx, binaryPath, "import", specPath,
		"--format", "openapi",
		"--admin-url", proc.adminURL(),
	)

	var importStdout, importStderr bytes.Buffer
	importCmd.Stdout = &importStdout
	importCmd.Stderr = &importStderr

	if err := importCmd.Run(); err != nil {
		t.Fatalf("Import command failed: %v\nstdout: %s\nstderr: %s", err, importStdout.String(), importStderr.String())
	}

	// Verify mocks were created by listing mocks via admin API
	resp, err := http.Get(proc.adminURL() + "/mocks")
	if err != nil {
		t.Fatalf("Failed to list mocks: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var mocksResp struct {
		Mocks []map[string]interface{} `json:"mocks"`
	}
	if err := json.Unmarshal(body, &mocksResp); err != nil {
		// Try parsing as array directly
		var mocks []map[string]interface{}
		if err := json.Unmarshal(body, &mocks); err != nil {
			t.Fatalf("Failed to parse mocks response: %v\nBody: %s", err, body)
		}
		mocksResp.Mocks = mocks
	}

	// Should have at least 2 mocks (one for each path)
	if len(mocksResp.Mocks) < 2 {
		t.Errorf("Expected at least 2 mocks from OpenAPI import, got %d", len(mocksResp.Mocks))
	}

	// Verify we can call the imported mock endpoints
	t.Run("CallImportedListEndpoint", func(t *testing.T) {
		resp, err := http.Get(proc.mockURL() + "/api/users")
		if err != nil {
			t.Fatalf("Failed to call imported mock: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			t.Errorf("Expected status 200, got %d: %s", resp.StatusCode, body)
		}
	})

	// Verify the parameterized endpoint mock was created by checking the mocks list
	// The path pattern might be /api/users/{userId} which requires path parameter matching
	t.Run("VerifyParameterizedEndpointMockCreated", func(t *testing.T) {
		// Check that we have a mock for /api/users/{userId} in the mock list
		found := false
		for _, mock := range mocksResp.Mocks {
			if httpConfig, ok := mock["http"].(map[string]interface{}); ok {
				if matcher, ok := httpConfig["matcher"].(map[string]interface{}); ok {
					if path, ok := matcher["path"].(string); ok {
						if strings.Contains(path, "/api/users/") || strings.Contains(path, "{userId}") {
							found = true
							break
						}
					}
				}
			}
		}
		if !found {
			t.Errorf("Expected to find a mock for /api/users/{userId} in imported mocks")
		}
	})
}

// ============================================================================
// Test: Export Mocks via CLI Binary
// ============================================================================

func TestBinaryE2E_ExportMocks(t *testing.T) {
	binaryPath := buildBinary(t)
	proc := startServer(t, binaryPath)

	// Create mocks via admin API first
	mocks := []map[string]interface{}{
		{
			"id":      "export-mock-1",
			"name":    "Export Test Mock 1",
			"type":    "http",
			"enabled": true,
			"http": map[string]interface{}{
				"matcher": map[string]interface{}{
					"method": "GET",
					"path":   "/api/export-test-1",
				},
				"response": map[string]interface{}{
					"statusCode": 200,
					"body":       `{"id": 1, "name": "Export Test 1"}`,
					"headers": map[string]string{
						"Content-Type": "application/json",
					},
				},
			},
		},
		{
			"id":      "export-mock-2",
			"name":    "Export Test Mock 2",
			"type":    "http",
			"enabled": true,
			"http": map[string]interface{}{
				"matcher": map[string]interface{}{
					"method": "POST",
					"path":   "/api/export-test-2",
				},
				"response": map[string]interface{}{
					"statusCode": 201,
					"body":       `{"created": true}`,
					"headers": map[string]string{
						"Content-Type": "application/json",
					},
				},
			},
		},
	}

	for _, mock := range mocks {
		mockJSON, _ := json.Marshal(mock)
		resp, err := http.Post(proc.adminURL()+"/mocks", "application/json", bytes.NewReader(mockJSON))
		if err != nil {
			t.Fatalf("Failed to create mock: %v", err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
			t.Fatalf("Failed to create mock: status %d", resp.StatusCode)
		}
	}

	// Run mockd export command
	exportDir := t.TempDir()
	exportPath := filepath.Join(exportDir, "exported-mocks.yaml")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	exportCmd := exec.CommandContext(ctx, binaryPath, "export",
		"-o", exportPath,
		"--admin-url", proc.adminURL(),
	)

	var exportStdout, exportStderr bytes.Buffer
	exportCmd.Stdout = &exportStdout
	exportCmd.Stderr = &exportStderr

	if err := exportCmd.Run(); err != nil {
		t.Fatalf("Export command failed: %v\nstdout: %s\nstderr: %s", err, exportStdout.String(), exportStderr.String())
	}

	// Verify export file was created
	if _, err := os.Stat(exportPath); os.IsNotExist(err) {
		t.Fatalf("Export file was not created at %s", exportPath)
	}

	// Read and verify export file contents
	exportedContent, err := os.ReadFile(exportPath)
	if err != nil {
		t.Fatalf("Failed to read export file: %v", err)
	}

	exportedStr := string(exportedContent)

	// Verify export contains our mocks
	if !strings.Contains(exportedStr, "export-mock-1") {
		t.Errorf("Export file should contain 'export-mock-1'")
	}
	if !strings.Contains(exportedStr, "export-mock-2") {
		t.Errorf("Export file should contain 'export-mock-2'")
	}
	if !strings.Contains(exportedStr, "/api/export-test-1") {
		t.Errorf("Export file should contain '/api/export-test-1'")
	}
	if !strings.Contains(exportedStr, "/api/export-test-2") {
		t.Errorf("Export file should contain '/api/export-test-2'")
	}

	// Test export to JSON format
	t.Run("ExportJSON", func(t *testing.T) {
		exportJSONPath := filepath.Join(exportDir, "exported-mocks.json")

		exportJSONCmd := exec.CommandContext(ctx, binaryPath, "export",
			"-o", exportJSONPath,
			"--admin-url", proc.adminURL(),
		)

		var stdout, stderr bytes.Buffer
		exportJSONCmd.Stdout = &stdout
		exportJSONCmd.Stderr = &stderr

		if err := exportJSONCmd.Run(); err != nil {
			t.Fatalf("Export JSON command failed: %v\nstdout: %s\nstderr: %s", err, stdout.String(), stderr.String())
		}

		jsonContent, err := os.ReadFile(exportJSONPath)
		if err != nil {
			t.Fatalf("Failed to read JSON export file: %v", err)
		}

		// Verify it's valid JSON
		var exported map[string]interface{}
		if err := json.Unmarshal(jsonContent, &exported); err != nil {
			t.Errorf("Export file is not valid JSON: %v", err)
		}
	})
}

// ============================================================================
// Test: Import Postman Collection via CLI Binary
// ============================================================================

func TestBinaryE2E_ImportPostman(t *testing.T) {
	binaryPath := buildBinary(t)
	proc := startServer(t, binaryPath)

	// Create Postman Collection v2.1 format file
	postmanCollection := `{
	"info": {
		"_postman_id": "test-collection-id",
		"name": "Test Postman Collection",
		"schema": "https://schema.getpostman.com/json/collection/v2.1.0/collection.json"
	},
	"item": [
		{
			"name": "Get Products",
			"request": {
				"method": "GET",
				"header": [],
				"url": {
					"raw": "{{baseUrl}}/api/products",
					"host": ["{{baseUrl}}"],
					"path": ["api", "products"]
				}
			},
			"response": [
				{
					"name": "Success",
					"originalRequest": {
						"method": "GET",
						"header": [],
						"url": {
							"raw": "{{baseUrl}}/api/products",
							"host": ["{{baseUrl}}"],
							"path": ["api", "products"]
						}
					},
					"status": "OK",
					"code": 200,
					"header": [
						{
							"key": "Content-Type",
							"value": "application/json"
						}
					],
					"body": "[{\"id\": \"prod-1\", \"name\": \"Widget\", \"price\": 9.99}, {\"id\": \"prod-2\", \"name\": \"Gadget\", \"price\": 19.99}]"
				}
			]
		},
		{
			"name": "Create Order",
			"request": {
				"method": "POST",
				"header": [
					{
						"key": "Content-Type",
						"value": "application/json"
					}
				],
				"body": {
					"mode": "raw",
					"raw": "{\"productId\": \"prod-1\", \"quantity\": 2}"
				},
				"url": {
					"raw": "{{baseUrl}}/api/orders",
					"host": ["{{baseUrl}}"],
					"path": ["api", "orders"]
				}
			},
			"response": [
				{
					"name": "Created",
					"originalRequest": {
						"method": "POST",
						"header": [
							{
								"key": "Content-Type",
								"value": "application/json"
							}
						],
						"body": {
							"mode": "raw",
							"raw": "{\"productId\": \"prod-1\", \"quantity\": 2}"
						},
						"url": {
							"raw": "{{baseUrl}}/api/orders",
							"host": ["{{baseUrl}}"],
							"path": ["api", "orders"]
						}
					},
					"status": "Created",
					"code": 201,
					"header": [
						{
							"key": "Content-Type",
							"value": "application/json"
						}
					],
					"body": "{\"id\": \"order-123\", \"productId\": \"prod-1\", \"quantity\": 2, \"status\": \"pending\"}"
				}
			]
		}
	]
}`
	collectionDir := t.TempDir()
	collectionPath := filepath.Join(collectionDir, "postman_collection.json")
	if err := os.WriteFile(collectionPath, []byte(postmanCollection), 0644); err != nil {
		t.Fatalf("Failed to write Postman collection: %v", err)
	}

	// Run mockd import command
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	importCmd := exec.CommandContext(ctx, binaryPath, "import", collectionPath,
		"--format", "postman",
		"--admin-url", proc.adminURL(),
	)

	var importStdout, importStderr bytes.Buffer
	importCmd.Stdout = &importStdout
	importCmd.Stderr = &importStderr

	if err := importCmd.Run(); err != nil {
		t.Fatalf("Import command failed: %v\nstdout: %s\nstderr: %s", err, importStdout.String(), importStderr.String())
	}

	// Verify mocks were created by listing mocks via admin API
	resp, err := http.Get(proc.adminURL() + "/mocks")
	if err != nil {
		t.Fatalf("Failed to list mocks: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var mocksResp struct {
		Mocks []map[string]interface{} `json:"mocks"`
	}
	if err := json.Unmarshal(body, &mocksResp); err != nil {
		// Try parsing as array directly
		var mocks []map[string]interface{}
		if err := json.Unmarshal(body, &mocks); err != nil {
			t.Fatalf("Failed to parse mocks response: %v\nBody: %s", err, body)
		}
		mocksResp.Mocks = mocks
	}

	// Should have at least 2 mocks (one for each request in the collection)
	if len(mocksResp.Mocks) < 2 {
		t.Errorf("Expected at least 2 mocks from Postman import, got %d", len(mocksResp.Mocks))
	}

	// Verify we can call the imported mock endpoints
	t.Run("CallImportedGetProducts", func(t *testing.T) {
		resp, err := http.Get(proc.mockURL() + "/api/products")
		if err != nil {
			t.Fatalf("Failed to call imported mock: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			t.Errorf("Expected status 200, got %d: %s", resp.StatusCode, body)
		}

		body, _ := io.ReadAll(resp.Body)
		if !strings.Contains(string(body), "Widget") {
			t.Errorf("Response should contain 'Widget' from Postman example, got: %s", body)
		}
	})

	t.Run("CallImportedCreateOrder", func(t *testing.T) {
		orderPayload := `{"productId": "prod-1", "quantity": 2}`
		resp, err := http.Post(proc.mockURL()+"/api/orders", "application/json", strings.NewReader(orderPayload))
		if err != nil {
			t.Fatalf("Failed to call imported mock: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusCreated {
			body, _ := io.ReadAll(resp.Body)
			t.Errorf("Expected status 201, got %d: %s", resp.StatusCode, body)
		}

		body, _ := io.ReadAll(resp.Body)
		if !strings.Contains(string(body), "order-123") {
			t.Errorf("Response should contain 'order-123' from Postman example, got: %s", body)
		}
	})
}

// ============================================================================
// Test: HTTP Header Matching
// ============================================================================

func TestBinaryE2E_HTTPHeaderMatching(t *testing.T) {
	binaryPath := buildBinary(t)
	proc := startServer(t, binaryPath)

	// Create mock with exact header match
	t.Run("ExactHeaderMatch", func(t *testing.T) {
		mockPayload := map[string]interface{}{
			"id":      "header-exact-mock",
			"name":    "Header Exact Match Mock",
			"type":    "http",
			"enabled": true,
			"http": map[string]interface{}{
				"matcher": map[string]interface{}{
					"method": "GET",
					"path":   "/api/header-exact",
					"headers": map[string]string{
						"X-Custom-Header": "exact-value",
					},
				},
				"response": map[string]interface{}{
					"statusCode": 200,
					"body":       `{"matched": "exact-header"}`,
				},
			},
		}

		mockJSON, _ := json.Marshal(mockPayload)
		resp, err := http.Post(proc.adminURL()+"/mocks", "application/json", bytes.NewReader(mockJSON))
		if err != nil {
			t.Fatalf("Failed to create mock: %v", err)
		}
		resp.Body.Close()

		// Test with matching header
		req, _ := http.NewRequest("GET", proc.mockURL()+"/api/header-exact", nil)
		req.Header.Set("X-Custom-Header", "exact-value")
		resp, err = http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected status 200 with matching header, got %d", resp.StatusCode)
		}

		body, _ := io.ReadAll(resp.Body)
		if !strings.Contains(string(body), "exact-header") {
			t.Errorf("Expected body to contain 'exact-header', got: %s", body)
		}

		// Test without header - should return 404
		req2, _ := http.NewRequest("GET", proc.mockURL()+"/api/header-exact", nil)
		resp2, err := http.DefaultClient.Do(req2)
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}
		resp2.Body.Close()

		if resp2.StatusCode != http.StatusNotFound {
			t.Errorf("Expected status 404 without header, got %d", resp2.StatusCode)
		}

		// Test with wrong header value - should return 404
		req3, _ := http.NewRequest("GET", proc.mockURL()+"/api/header-exact", nil)
		req3.Header.Set("X-Custom-Header", "wrong-value")
		resp3, err := http.DefaultClient.Do(req3)
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}
		resp3.Body.Close()

		if resp3.StatusCode != http.StatusNotFound {
			t.Errorf("Expected status 404 with wrong header value, got %d", resp3.StatusCode)
		}
	})

	// Create mock with wildcard header match (prefix pattern)
	t.Run("PrefixHeaderMatch", func(t *testing.T) {
		mockPayload := map[string]interface{}{
			"id":      "header-prefix-mock",
			"name":    "Header Prefix Match Mock",
			"type":    "http",
			"enabled": true,
			"http": map[string]interface{}{
				"matcher": map[string]interface{}{
					"method": "GET",
					"path":   "/api/header-prefix",
					"headers": map[string]string{
						"Authorization": "Bearer *",
					},
				},
				"response": map[string]interface{}{
					"statusCode": 200,
					"body":       `{"matched": "prefix-header"}`,
				},
			},
		}

		mockJSON, _ := json.Marshal(mockPayload)
		resp, err := http.Post(proc.adminURL()+"/mocks", "application/json", bytes.NewReader(mockJSON))
		if err != nil {
			t.Fatalf("Failed to create mock: %v", err)
		}
		resp.Body.Close()

		// Test with matching prefix
		req, _ := http.NewRequest("GET", proc.mockURL()+"/api/header-prefix", nil)
		req.Header.Set("Authorization", "Bearer my-token-12345")
		resp, err = http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected status 200 with Bearer prefix, got %d", resp.StatusCode)
		}

		// Test with different prefix - should return 404
		req2, _ := http.NewRequest("GET", proc.mockURL()+"/api/header-prefix", nil)
		req2.Header.Set("Authorization", "Basic credentials")
		resp2, err := http.DefaultClient.Do(req2)
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}
		resp2.Body.Close()

		if resp2.StatusCode != http.StatusNotFound {
			t.Errorf("Expected status 404 with non-Bearer prefix, got %d", resp2.StatusCode)
		}
	})

	// Create mock with contains header match
	t.Run("ContainsHeaderMatch", func(t *testing.T) {
		mockPayload := map[string]interface{}{
			"id":      "header-contains-mock",
			"name":    "Header Contains Match Mock",
			"type":    "http",
			"enabled": true,
			"http": map[string]interface{}{
				"matcher": map[string]interface{}{
					"method": "GET",
					"path":   "/api/header-contains",
					"headers": map[string]string{
						"Content-Type": "*json*",
					},
				},
				"response": map[string]interface{}{
					"statusCode": 200,
					"body":       `{"matched": "contains-header"}`,
				},
			},
		}

		mockJSON, _ := json.Marshal(mockPayload)
		resp, err := http.Post(proc.adminURL()+"/mocks", "application/json", bytes.NewReader(mockJSON))
		if err != nil {
			t.Fatalf("Failed to create mock: %v", err)
		}
		resp.Body.Close()

		// Test with content-type containing json
		req, _ := http.NewRequest("GET", proc.mockURL()+"/api/header-contains", nil)
		req.Header.Set("Content-Type", "application/json")
		resp, err = http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected status 200 with json in Content-Type, got %d", resp.StatusCode)
		}

		// Test with another json content-type
		req2, _ := http.NewRequest("GET", proc.mockURL()+"/api/header-contains", nil)
		req2.Header.Set("Content-Type", "text/json; charset=utf-8")
		resp2, err := http.DefaultClient.Do(req2)
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}
		resp2.Body.Close()

		if resp2.StatusCode != http.StatusOK {
			t.Errorf("Expected status 200 with text/json Content-Type, got %d", resp2.StatusCode)
		}

		// Test with non-json content-type - should return 404
		req3, _ := http.NewRequest("GET", proc.mockURL()+"/api/header-contains", nil)
		req3.Header.Set("Content-Type", "text/html")
		resp3, err := http.DefaultClient.Do(req3)
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}
		resp3.Body.Close()

		if resp3.StatusCode != http.StatusNotFound {
			t.Errorf("Expected status 404 with text/html Content-Type, got %d", resp3.StatusCode)
		}
	})

	// Test multiple headers matching
	t.Run("MultipleHeadersMatch", func(t *testing.T) {
		mockPayload := map[string]interface{}{
			"id":      "header-multiple-mock",
			"name":    "Multiple Headers Match Mock",
			"type":    "http",
			"enabled": true,
			"http": map[string]interface{}{
				"matcher": map[string]interface{}{
					"method": "POST",
					"path":   "/api/header-multiple",
					"headers": map[string]string{
						"X-API-Key":   "secret-key-123",
						"X-Client-ID": "client-abc",
						"Accept":      "application/json",
					},
				},
				"response": map[string]interface{}{
					"statusCode": 200,
					"body":       `{"matched": "multiple-headers"}`,
				},
			},
		}

		mockJSON, _ := json.Marshal(mockPayload)
		resp, err := http.Post(proc.adminURL()+"/mocks", "application/json", bytes.NewReader(mockJSON))
		if err != nil {
			t.Fatalf("Failed to create mock: %v", err)
		}
		resp.Body.Close()

		// Test with all matching headers
		req, _ := http.NewRequest("POST", proc.mockURL()+"/api/header-multiple", nil)
		req.Header.Set("X-API-Key", "secret-key-123")
		req.Header.Set("X-Client-ID", "client-abc")
		req.Header.Set("Accept", "application/json")
		resp, err = http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected status 200 with all headers, got %d", resp.StatusCode)
		}

		// Test with one missing header - should return 404
		req2, _ := http.NewRequest("POST", proc.mockURL()+"/api/header-multiple", nil)
		req2.Header.Set("X-API-Key", "secret-key-123")
		req2.Header.Set("Accept", "application/json")
		// Missing X-Client-ID
		resp2, err := http.DefaultClient.Do(req2)
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}
		resp2.Body.Close()

		if resp2.StatusCode != http.StatusNotFound {
			t.Errorf("Expected status 404 with missing header, got %d", resp2.StatusCode)
		}
	})
}

// ============================================================================
// Test: HTTP Body Matching
// ============================================================================

func TestBinaryE2E_HTTPBodyMatching(t *testing.T) {
	binaryPath := buildBinary(t)
	proc := startServer(t, binaryPath)

	// Test body contains matching
	t.Run("BodyContainsMatch", func(t *testing.T) {
		mockPayload := map[string]interface{}{
			"id":      "body-contains-mock",
			"name":    "Body Contains Match Mock",
			"type":    "http",
			"enabled": true,
			"http": map[string]interface{}{
				"matcher": map[string]interface{}{
					"method":       "POST",
					"path":         "/api/body-contains",
					"bodyContains": "action\":\"create",
				},
				"response": map[string]interface{}{
					"statusCode": 200,
					"body":       `{"matched": "body-contains"}`,
				},
			},
		}

		mockJSON, _ := json.Marshal(mockPayload)
		resp, err := http.Post(proc.adminURL()+"/mocks", "application/json", bytes.NewReader(mockJSON))
		if err != nil {
			t.Fatalf("Failed to create mock: %v", err)
		}
		resp.Body.Close()

		// Test with matching body
		reqBody := `{"action":"create", "name": "test"}`
		req, _ := http.NewRequest("POST", proc.mockURL()+"/api/body-contains", strings.NewReader(reqBody))
		req.Header.Set("Content-Type", "application/json")
		resp, err = http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected status 200 with matching body, got %d", resp.StatusCode)
		}

		// Test with non-matching body - should return 404
		reqBody2 := `{"action":"delete", "name": "test"}`
		req2, _ := http.NewRequest("POST", proc.mockURL()+"/api/body-contains", strings.NewReader(reqBody2))
		req2.Header.Set("Content-Type", "application/json")
		resp2, err := http.DefaultClient.Do(req2)
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}
		resp2.Body.Close()

		if resp2.StatusCode != http.StatusNotFound {
			t.Errorf("Expected status 404 with non-matching body, got %d", resp2.StatusCode)
		}
	})

	// Test body equals matching
	t.Run("BodyEqualsMatch", func(t *testing.T) {
		exactBody := `{"type":"ping"}`
		mockPayload := map[string]interface{}{
			"id":      "body-equals-mock",
			"name":    "Body Equals Match Mock",
			"type":    "http",
			"enabled": true,
			"http": map[string]interface{}{
				"matcher": map[string]interface{}{
					"method":     "POST",
					"path":       "/api/body-equals",
					"bodyEquals": exactBody,
				},
				"response": map[string]interface{}{
					"statusCode": 200,
					"body":       `{"matched": "body-equals"}`,
				},
			},
		}

		mockJSON, _ := json.Marshal(mockPayload)
		resp, err := http.Post(proc.adminURL()+"/mocks", "application/json", bytes.NewReader(mockJSON))
		if err != nil {
			t.Fatalf("Failed to create mock: %v", err)
		}
		resp.Body.Close()

		// Test with exactly matching body
		req, _ := http.NewRequest("POST", proc.mockURL()+"/api/body-equals", strings.NewReader(exactBody))
		req.Header.Set("Content-Type", "application/json")
		resp, err = http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected status 200 with exact body match, got %d", resp.StatusCode)
		}

		// Test with similar but not exact body - should return 404
		req2, _ := http.NewRequest("POST", proc.mockURL()+"/api/body-equals", strings.NewReader(`{"type":"ping"} `))
		req2.Header.Set("Content-Type", "application/json")
		resp2, err := http.DefaultClient.Do(req2)
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}
		resp2.Body.Close()

		if resp2.StatusCode != http.StatusNotFound {
			t.Errorf("Expected status 404 with non-exact body, got %d", resp2.StatusCode)
		}
	})

	// Test JSONPath body matching
	t.Run("JSONPathMatch", func(t *testing.T) {
		mockPayload := map[string]interface{}{
			"id":      "body-jsonpath-mock",
			"name":    "Body JSONPath Match Mock",
			"type":    "http",
			"enabled": true,
			"http": map[string]interface{}{
				"matcher": map[string]interface{}{
					"method": "POST",
					"path":   "/api/body-jsonpath",
					"bodyJsonPath": map[string]interface{}{
						"$.user.role":   "admin",
						"$.user.active": true,
					},
				},
				"response": map[string]interface{}{
					"statusCode": 200,
					"body":       `{"matched": "jsonpath"}`,
				},
			},
		}

		mockJSON, _ := json.Marshal(mockPayload)
		resp, err := http.Post(proc.adminURL()+"/mocks", "application/json", bytes.NewReader(mockJSON))
		if err != nil {
			t.Fatalf("Failed to create mock: %v", err)
		}
		resp.Body.Close()

		// Test with matching JSONPath values
		reqBody := `{"user": {"name": "John", "role": "admin", "active": true}}`
		req, _ := http.NewRequest("POST", proc.mockURL()+"/api/body-jsonpath", strings.NewReader(reqBody))
		req.Header.Set("Content-Type", "application/json")
		resp, err = http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			t.Errorf("Expected status 200 with matching JSONPath, got %d, body: %s", resp.StatusCode, body)
		}

		// Test with non-matching role - should return 404
		reqBody2 := `{"user": {"name": "John", "role": "user", "active": true}}`
		req2, _ := http.NewRequest("POST", proc.mockURL()+"/api/body-jsonpath", strings.NewReader(reqBody2))
		req2.Header.Set("Content-Type", "application/json")
		resp2, err := http.DefaultClient.Do(req2)
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}
		resp2.Body.Close()

		if resp2.StatusCode != http.StatusNotFound {
			t.Errorf("Expected status 404 with non-matching JSONPath value, got %d", resp2.StatusCode)
		}

		// Test with non-matching active - should return 404
		reqBody3 := `{"user": {"name": "John", "role": "admin", "active": false}}`
		req3, _ := http.NewRequest("POST", proc.mockURL()+"/api/body-jsonpath", strings.NewReader(reqBody3))
		req3.Header.Set("Content-Type", "application/json")
		resp3, err := http.DefaultClient.Do(req3)
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}
		resp3.Body.Close()

		if resp3.StatusCode != http.StatusNotFound {
			t.Errorf("Expected status 404 with inactive user, got %d", resp3.StatusCode)
		}
	})

	// Test body pattern (regex) matching
	t.Run("BodyPatternMatch", func(t *testing.T) {
		mockPayload := map[string]interface{}{
			"id":      "body-pattern-mock",
			"name":    "Body Pattern Match Mock",
			"type":    "http",
			"enabled": true,
			"http": map[string]interface{}{
				"matcher": map[string]interface{}{
					"method":      "POST",
					"path":        "/api/body-pattern",
					"bodyPattern": `"email"\s*:\s*"[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}"`,
				},
				"response": map[string]interface{}{
					"statusCode": 200,
					"body":       `{"matched": "body-pattern"}`,
				},
			},
		}

		mockJSON, _ := json.Marshal(mockPayload)
		resp, err := http.Post(proc.adminURL()+"/mocks", "application/json", bytes.NewReader(mockJSON))
		if err != nil {
			t.Fatalf("Failed to create mock: %v", err)
		}
		resp.Body.Close()

		// Test with valid email in body
		reqBody := `{"name": "John", "email": "john.doe@example.com"}`
		req, _ := http.NewRequest("POST", proc.mockURL()+"/api/body-pattern", strings.NewReader(reqBody))
		req.Header.Set("Content-Type", "application/json")
		resp, err = http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected status 200 with valid email pattern, got %d", resp.StatusCode)
		}

		// Test with invalid email - should return 404
		reqBody2 := `{"name": "John", "email": "invalid-email"}`
		req2, _ := http.NewRequest("POST", proc.mockURL()+"/api/body-pattern", strings.NewReader(reqBody2))
		req2.Header.Set("Content-Type", "application/json")
		resp2, err := http.DefaultClient.Do(req2)
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}
		resp2.Body.Close()

		if resp2.StatusCode != http.StatusNotFound {
			t.Errorf("Expected status 404 with invalid email pattern, got %d", resp2.StatusCode)
		}
	})
}

// ============================================================================
// Test: HTTP Regex Path Matching
// ============================================================================

func TestBinaryE2E_HTTPRegexPathMatching(t *testing.T) {
	binaryPath := buildBinary(t)
	proc := startServer(t, binaryPath)

	// Test basic regex path pattern
	t.Run("BasicRegexPath", func(t *testing.T) {
		mockPayload := map[string]interface{}{
			"id":      "regex-path-basic-mock",
			"name":    "Basic Regex Path Mock",
			"type":    "http",
			"enabled": true,
			"http": map[string]interface{}{
				"matcher": map[string]interface{}{
					"method":      "GET",
					"pathPattern": "^/api/v[0-9]+/users$",
				},
				"response": map[string]interface{}{
					"statusCode": 200,
					"body":       `{"matched": "regex-path-basic"}`,
				},
			},
		}

		mockJSON, _ := json.Marshal(mockPayload)
		resp, err := http.Post(proc.adminURL()+"/mocks", "application/json", bytes.NewReader(mockJSON))
		if err != nil {
			t.Fatalf("Failed to create mock: %v", err)
		}
		resp.Body.Close()

		// Test matching paths
		for _, path := range []string{"/api/v1/users", "/api/v2/users", "/api/v99/users"} {
			resp, err := http.Get(proc.mockURL() + path)
			if err != nil {
				t.Fatalf("Request failed for %s: %v", path, err)
			}
			resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				t.Errorf("Expected status 200 for path %s, got %d", path, resp.StatusCode)
			}
		}

		// Test non-matching paths
		for _, path := range []string{"/api/users", "/api/vX/users", "/api/v1/users/123"} {
			resp, err := http.Get(proc.mockURL() + path)
			if err != nil {
				t.Fatalf("Request failed for %s: %v", path, err)
			}
			resp.Body.Close()

			if resp.StatusCode != http.StatusNotFound {
				t.Errorf("Expected status 404 for path %s, got %d", path, resp.StatusCode)
			}
		}
	})

	// Test named capture groups in path pattern
	t.Run("NamedCaptureGroups", func(t *testing.T) {
		mockPayload := map[string]interface{}{
			"id":      "regex-path-capture-mock",
			"name":    "Regex Path Capture Mock",
			"type":    "http",
			"enabled": true,
			"http": map[string]interface{}{
				"matcher": map[string]interface{}{
					"method":      "GET",
					"pathPattern": `^/api/orders/(?P<orderId>[A-Z0-9-]+)/items/(?P<itemId>\d+)$`,
				},
				"response": map[string]interface{}{
					"statusCode": 200,
					"body":       `{"orderId": "{{request.pathPattern.orderId}}", "itemId": "{{request.pathPattern.itemId}}"}`,
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

		// Test with valid path
		resp, err = http.Get(proc.mockURL() + "/api/orders/ORD-12345/items/42")
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected status 200, got %d", resp.StatusCode)
		}

		body, _ := io.ReadAll(resp.Body)
		if !strings.Contains(string(body), "ORD-12345") {
			t.Errorf("Expected body to contain captured orderId 'ORD-12345', got: %s", body)
		}
		if !strings.Contains(string(body), `"itemId": "42"`) {
			t.Errorf("Expected body to contain captured itemId '42', got: %s", body)
		}

		// Test with non-matching path format
		resp2, err := http.Get(proc.mockURL() + "/api/orders/lowercase-id/items/notanumber")
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}
		resp2.Body.Close()

		if resp2.StatusCode != http.StatusNotFound {
			t.Errorf("Expected status 404 for non-matching path, got %d", resp2.StatusCode)
		}
	})

	// Test UUID path pattern
	t.Run("UUIDPathPattern", func(t *testing.T) {
		mockPayload := map[string]interface{}{
			"id":      "regex-path-uuid-mock",
			"name":    "Regex Path UUID Mock",
			"type":    "http",
			"enabled": true,
			"http": map[string]interface{}{
				"matcher": map[string]interface{}{
					"method":      "GET",
					"pathPattern": `^/api/resources/(?P<uuid>[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12})$`,
				},
				"response": map[string]interface{}{
					"statusCode": 200,
					"body":       `{"resourceId": "{{request.pathPattern.uuid}}"}`,
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

		// Test with valid UUID
		uuid := "550e8400-e29b-41d4-a716-446655440000"
		resp, err = http.Get(proc.mockURL() + "/api/resources/" + uuid)
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected status 200 for UUID path, got %d", resp.StatusCode)
		}

		body, _ := io.ReadAll(resp.Body)
		if !strings.Contains(string(body), uuid) {
			t.Errorf("Expected body to contain UUID '%s', got: %s", uuid, body)
		}

		// Test with invalid UUID format
		resp2, err := http.Get(proc.mockURL() + "/api/resources/not-a-valid-uuid")
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}
		resp2.Body.Close()

		if resp2.StatusCode != http.StatusNotFound {
			t.Errorf("Expected status 404 for invalid UUID, got %d", resp2.StatusCode)
		}
	})
}

// ============================================================================
// Test: HTTP Priority Matching
// ============================================================================

func TestBinaryE2E_HTTPPriorityMatching(t *testing.T) {
	binaryPath := buildBinary(t)
	proc := startServer(t, binaryPath)

	// Create multiple mocks with same path but different priorities
	t.Run("PriorityDeterminesWinner", func(t *testing.T) {
		// Create low priority mock first
		lowPriorityMock := map[string]interface{}{
			"id":      "priority-low-mock",
			"name":    "Low Priority Mock",
			"type":    "http",
			"enabled": true,
			"http": map[string]interface{}{
				"priority": 10,
				"matcher": map[string]interface{}{
					"method": "GET",
					"path":   "/api/priority-test",
				},
				"response": map[string]interface{}{
					"statusCode": 200,
					"body":       `{"priority": "low"}`,
				},
			},
		}

		mockJSON, _ := json.Marshal(lowPriorityMock)
		resp, err := http.Post(proc.adminURL()+"/mocks", "application/json", bytes.NewReader(mockJSON))
		if err != nil {
			t.Fatalf("Failed to create low priority mock: %v", err)
		}
		resp.Body.Close()

		// Create high priority mock
		highPriorityMock := map[string]interface{}{
			"id":      "priority-high-mock",
			"name":    "High Priority Mock",
			"type":    "http",
			"enabled": true,
			"http": map[string]interface{}{
				"priority": 100,
				"matcher": map[string]interface{}{
					"method": "GET",
					"path":   "/api/priority-test",
				},
				"response": map[string]interface{}{
					"statusCode": 200,
					"body":       `{"priority": "high"}`,
				},
			},
		}

		mockJSON, _ = json.Marshal(highPriorityMock)
		resp, err = http.Post(proc.adminURL()+"/mocks", "application/json", bytes.NewReader(mockJSON))
		if err != nil {
			t.Fatalf("Failed to create high priority mock: %v", err)
		}
		resp.Body.Close()

		// Request should match high priority mock
		resp, err = http.Get(proc.mockURL() + "/api/priority-test")
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected status 200, got %d", resp.StatusCode)
		}

		body, _ := io.ReadAll(resp.Body)
		if !strings.Contains(string(body), `"priority": "high"`) {
			t.Errorf("Expected high priority response, got: %s", body)
		}
	})

	// Test that score takes precedence over priority for different specificities
	t.Run("ScoreTakesPrecedenceOverPriority", func(t *testing.T) {
		// Create generic path mock with HIGH priority
		genericMock := map[string]interface{}{
			"id":      "priority-generic-mock",
			"name":    "Generic Path High Priority Mock",
			"type":    "http",
			"enabled": true,
			"http": map[string]interface{}{
				"priority": 1000, // Very high priority
				"matcher": map[string]interface{}{
					"method": "GET",
					"path":   "/api/score-vs-priority/*",
				},
				"response": map[string]interface{}{
					"statusCode": 200,
					"body":       `{"source": "generic-wildcard"}`,
				},
			},
		}

		mockJSON, _ := json.Marshal(genericMock)
		resp, err := http.Post(proc.adminURL()+"/mocks", "application/json", bytes.NewReader(mockJSON))
		if err != nil {
			t.Fatalf("Failed to create generic mock: %v", err)
		}
		resp.Body.Close()

		// Create specific path mock with LOW priority
		specificMock := map[string]interface{}{
			"id":      "priority-specific-mock",
			"name":    "Specific Path Low Priority Mock",
			"type":    "http",
			"enabled": true,
			"http": map[string]interface{}{
				"priority": 1, // Very low priority
				"matcher": map[string]interface{}{
					"method": "GET",
					"path":   "/api/score-vs-priority/specific",
				},
				"response": map[string]interface{}{
					"statusCode": 200,
					"body":       `{"source": "specific-exact"}`,
				},
			},
		}

		mockJSON, _ = json.Marshal(specificMock)
		resp, err = http.Post(proc.adminURL()+"/mocks", "application/json", bytes.NewReader(mockJSON))
		if err != nil {
			t.Fatalf("Failed to create specific mock: %v", err)
		}
		resp.Body.Close()

		// Request to specific path should match the specific mock due to higher score
		resp, err = http.Get(proc.mockURL() + "/api/score-vs-priority/specific")
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}
		defer resp.Body.Close()

		body, _ := io.ReadAll(resp.Body)
		if !strings.Contains(string(body), `"source": "specific-exact"`) {
			t.Errorf("Expected specific mock response (higher score wins), got: %s", body)
		}

		// Request to other path should match generic mock
		resp2, err := http.Get(proc.mockURL() + "/api/score-vs-priority/other")
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}
		defer resp2.Body.Close()

		body2, _ := io.ReadAll(resp2.Body)
		if !strings.Contains(string(body2), `"source": "generic-wildcard"`) {
			t.Errorf("Expected generic mock response, got: %s", body2)
		}
	})

	// Test priority with same score (same specificity)
	t.Run("PriorityBreaksTiesWithSameScore", func(t *testing.T) {
		// Create two mocks with same path but different headers and priorities
		mock1 := map[string]interface{}{
			"id":      "priority-tie-1",
			"name":    "Tie Breaker Mock 1",
			"type":    "http",
			"enabled": true,
			"http": map[string]interface{}{
				"priority": 50,
				"matcher": map[string]interface{}{
					"method": "POST",
					"path":   "/api/priority-tie",
					"headers": map[string]string{
						"X-Version": "v1",
					},
				},
				"response": map[string]interface{}{
					"statusCode": 200,
					"body":       `{"winner": "mock1-priority50"}`,
				},
			},
		}

		mockJSON, _ := json.Marshal(mock1)
		resp, err := http.Post(proc.adminURL()+"/mocks", "application/json", bytes.NewReader(mockJSON))
		if err != nil {
			t.Fatalf("Failed to create mock1: %v", err)
		}
		resp.Body.Close()

		mock2 := map[string]interface{}{
			"id":      "priority-tie-2",
			"name":    "Tie Breaker Mock 2",
			"type":    "http",
			"enabled": true,
			"http": map[string]interface{}{
				"priority": 75,
				"matcher": map[string]interface{}{
					"method": "POST",
					"path":   "/api/priority-tie",
					"headers": map[string]string{
						"X-Version": "v1",
					},
				},
				"response": map[string]interface{}{
					"statusCode": 200,
					"body":       `{"winner": "mock2-priority75"}`,
				},
			},
		}

		mockJSON, _ = json.Marshal(mock2)
		resp, err = http.Post(proc.adminURL()+"/mocks", "application/json", bytes.NewReader(mockJSON))
		if err != nil {
			t.Fatalf("Failed to create mock2: %v", err)
		}
		resp.Body.Close()

		// Request should match mock2 due to higher priority
		req, _ := http.NewRequest("POST", proc.mockURL()+"/api/priority-tie", nil)
		req.Header.Set("X-Version", "v1")
		resp, err = http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}
		defer resp.Body.Close()

		body, _ := io.ReadAll(resp.Body)
		if !strings.Contains(string(body), `"winner": "mock2-priority75"`) {
			t.Errorf("Expected mock2 (priority 75) to win, got: %s", body)
		}
	})

	// Test zero/default priority behavior
	t.Run("ZeroPriorityBehavior", func(t *testing.T) {
		// Create mock with no priority (defaults to 0)
		defaultPriorityMock := map[string]interface{}{
			"id":      "priority-default-mock",
			"name":    "Default Priority Mock",
			"type":    "http",
			"enabled": true,
			"http": map[string]interface{}{
				// No priority field - defaults to 0
				"matcher": map[string]interface{}{
					"method": "GET",
					"path":   "/api/default-priority",
				},
				"response": map[string]interface{}{
					"statusCode": 200,
					"body":       `{"priority": "default"}`,
				},
			},
		}

		mockJSON, _ := json.Marshal(defaultPriorityMock)
		resp, err := http.Post(proc.adminURL()+"/mocks", "application/json", bytes.NewReader(mockJSON))
		if err != nil {
			t.Fatalf("Failed to create default priority mock: %v", err)
		}
		resp.Body.Close()

		// Create mock with explicit low priority
		explicitPriorityMock := map[string]interface{}{
			"id":      "priority-explicit-mock",
			"name":    "Explicit Priority Mock",
			"type":    "http",
			"enabled": true,
			"http": map[string]interface{}{
				"priority": 5,
				"matcher": map[string]interface{}{
					"method": "GET",
					"path":   "/api/default-priority",
				},
				"response": map[string]interface{}{
					"statusCode": 200,
					"body":       `{"priority": "explicit-5"}`,
				},
			},
		}

		mockJSON, _ = json.Marshal(explicitPriorityMock)
		resp, err = http.Post(proc.adminURL()+"/mocks", "application/json", bytes.NewReader(mockJSON))
		if err != nil {
			t.Fatalf("Failed to create explicit priority mock: %v", err)
		}
		resp.Body.Close()

		// Explicit priority (5) should win over default (0)
		resp, err = http.Get(proc.mockURL() + "/api/default-priority")
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}
		defer resp.Body.Close()

		body, _ := io.ReadAll(resp.Body)
		if !strings.Contains(string(body), `"priority": "explicit-5"`) {
			t.Errorf("Expected explicit priority mock to win, got: %s", body)
		}
	})
}

// Note: SOAP protocol tests are in soap_test.go using direct engine setup
// The admin API mock creation for SOAP has a different config structure

// ============================================================================
// Test: OAuth/OIDC Protocol
// ============================================================================

func TestBinaryE2E_OAuthTokenEndpoint(t *testing.T) {
	binaryPath := buildBinary(t)
	proc := startServer(t, binaryPath)

	// Create OAuth provider mock
	oauthMock := map[string]interface{}{
		"id":      "oauth-test",
		"name":    "OAuth Test Provider",
		"type":    "oauth",
		"enabled": true,
		"oauth": map[string]interface{}{
			"path":   "/oauth",
			"issuer": fmt.Sprintf("http://localhost:%d/oauth", proc.httpPort),
			"clients": []map[string]interface{}{
				{
					"clientId":     "test-client",
					"clientSecret": "test-secret",
					"grantTypes":   []string{"client_credentials", "password"},
				},
			},
			"users": []map[string]interface{}{
				{
					"username": "testuser",
					"password": "testpass",
					"claims": map[string]interface{}{
						"sub":   "user-123",
						"name":  "Test User",
						"email": "test@example.com",
					},
				},
			},
			"accessTokenLifetime":  3600,
			"refreshTokenLifetime": 86400,
		},
	}

	mockJSON, _ := json.Marshal(oauthMock)
	resp, err := http.Post(proc.adminURL()+"/mocks", "application/json", bytes.NewReader(mockJSON))
	if err != nil {
		t.Fatalf("Failed to create OAuth mock: %v", err)
	}
	resp.Body.Close()

	// Test client credentials grant
	t.Run("ClientCredentialsGrant", func(t *testing.T) {
		data := "grant_type=client_credentials"
		req, _ := http.NewRequest("POST", proc.mockURL()+"/oauth/token", strings.NewReader(data))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.SetBasicAuth("test-client", "test-secret")

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("Token request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			t.Fatalf("Expected 200, got %d: %s", resp.StatusCode, body)
		}

		var tokenResp map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&tokenResp)

		if _, ok := tokenResp["access_token"]; !ok {
			t.Error("Response should contain access_token")
		}
		if tokenResp["token_type"] != "Bearer" {
			t.Errorf("Token type should be Bearer, got %v", tokenResp["token_type"])
		}
	})

	// Test password grant
	t.Run("PasswordGrant", func(t *testing.T) {
		data := "grant_type=password&username=testuser&password=testpass"
		req, _ := http.NewRequest("POST", proc.mockURL()+"/oauth/token", strings.NewReader(data))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.SetBasicAuth("test-client", "test-secret")

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("Token request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			t.Fatalf("Expected 200, got %d: %s", resp.StatusCode, body)
		}

		var tokenResp map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&tokenResp)

		if _, ok := tokenResp["access_token"]; !ok {
			t.Error("Response should contain access_token")
		}
	})

	// Test invalid credentials
	t.Run("InvalidCredentials", func(t *testing.T) {
		data := "grant_type=client_credentials"
		req, _ := http.NewRequest("POST", proc.mockURL()+"/oauth/token", strings.NewReader(data))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.SetBasicAuth("wrong-client", "wrong-secret")

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("Token request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("Expected 401, got %d", resp.StatusCode)
		}
	})
}

func TestBinaryE2E_OAuthOIDCDiscovery(t *testing.T) {
	binaryPath := buildBinary(t)
	proc := startServer(t, binaryPath)

	// Create OAuth provider mock
	oauthMock := map[string]interface{}{
		"id":      "oauth-oidc-test",
		"name":    "OIDC Test Provider",
		"type":    "oauth",
		"enabled": true,
		"oauth": map[string]interface{}{
			"path":   "/oidc",
			"issuer": fmt.Sprintf("http://localhost:%d/oidc", proc.httpPort),
			"clients": []map[string]interface{}{
				{"clientId": "oidc-client", "clientSecret": "oidc-secret"},
			},
		},
	}

	mockJSON, _ := json.Marshal(oauthMock)
	resp, err := http.Post(proc.adminURL()+"/mocks", "application/json", bytes.NewReader(mockJSON))
	if err != nil {
		t.Fatalf("Failed to create OAuth mock: %v", err)
	}
	resp.Body.Close()

	// Test OIDC discovery endpoint
	resp, err = http.Get(proc.mockURL() + "/oidc/.well-known/openid-configuration")
	if err != nil {
		t.Fatalf("Discovery request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected 200, got %d", resp.StatusCode)
	}

	var discovery map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&discovery)

	// Verify discovery document has required fields
	if _, ok := discovery["issuer"]; !ok {
		t.Error("Discovery should contain issuer")
	}
	if _, ok := discovery["token_endpoint"]; !ok {
		t.Error("Discovery should contain token_endpoint")
	}
	if _, ok := discovery["jwks_uri"]; !ok {
		t.Error("Discovery should contain jwks_uri")
	}
}

// ============================================================================
// Test: Chaos Engineering
// ============================================================================

func TestBinaryE2E_ChaosLatencyInjection(t *testing.T) {
	binaryPath := buildBinary(t)
	proc := startServer(t, binaryPath)

	// Create a simple mock
	httpMock := map[string]interface{}{
		"id":      "chaos-latency-test",
		"name":    "Chaos Latency Test",
		"type":    "http",
		"enabled": true,
		"http": map[string]interface{}{
			"matcher":  map[string]interface{}{"method": "GET", "path": "/api/chaos-latency"},
			"response": map[string]interface{}{"statusCode": 200, "body": `{"ok": true}`},
		},
	}

	mockJSON, _ := json.Marshal(httpMock)
	resp, err := http.Post(proc.adminURL()+"/mocks", "application/json", bytes.NewReader(mockJSON))
	if err != nil {
		t.Fatalf("Failed to create mock: %v", err)
	}
	resp.Body.Close()

	// Measure baseline latency
	start := time.Now()
	resp, _ = http.Get(proc.mockURL() + "/api/chaos-latency")
	resp.Body.Close()
	baseline := time.Since(start)

	// Enable chaos with 200ms latency
	chaosConfig := map[string]interface{}{
		"enabled": true,
		"latency": map[string]interface{}{
			"min":         "200ms",
			"max":         "200ms",
			"probability": 1.0,
		},
	}

	chaosJSON, _ := json.Marshal(chaosConfig)
	req, _ := http.NewRequest("PUT", proc.adminURL()+"/chaos", bytes.NewReader(chaosJSON))
	req.Header.Set("Content-Type", "application/json")
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Failed to enable chaos: %v", err)
	}
	resp.Body.Close()

	// Measure latency with chaos enabled
	start = time.Now()
	resp, _ = http.Get(proc.mockURL() + "/api/chaos-latency")
	resp.Body.Close()
	withChaos := time.Since(start)

	// Chaos latency should be significantly higher
	if withChaos < 200*time.Millisecond {
		t.Errorf("Expected latency >= 200ms with chaos, got %v (baseline: %v)", withChaos, baseline)
	}
}

func TestBinaryE2E_ChaosErrorRateInjection(t *testing.T) {
	binaryPath := buildBinary(t)
	proc := startServer(t, binaryPath)

	// Create a simple mock
	httpMock := map[string]interface{}{
		"id":      "chaos-error-test",
		"name":    "Chaos Error Test",
		"type":    "http",
		"enabled": true,
		"http": map[string]interface{}{
			"matcher":  map[string]interface{}{"method": "GET", "path": "/api/chaos-error"},
			"response": map[string]interface{}{"statusCode": 200, "body": `{"ok": true}`},
		},
	}

	mockJSON, _ := json.Marshal(httpMock)
	resp, err := http.Post(proc.adminURL()+"/mocks", "application/json", bytes.NewReader(mockJSON))
	if err != nil {
		t.Fatalf("Failed to create mock: %v", err)
	}
	resp.Body.Close()

	// Enable chaos with 50% error rate
	chaosConfig := map[string]interface{}{
		"enabled": true,
		"errorRate": map[string]interface{}{
			"probability": 0.5,
			"defaultCode": 500,
		},
	}

	chaosJSON, _ := json.Marshal(chaosConfig)
	req, _ := http.NewRequest("PUT", proc.adminURL()+"/chaos", bytes.NewReader(chaosJSON))
	req.Header.Set("Content-Type", "application/json")
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Failed to enable chaos: %v", err)
	}
	resp.Body.Close()

	// Make 50 requests and count errors
	errors := 0
	for i := 0; i < 50; i++ {
		resp, _ := http.Get(proc.mockURL() + "/api/chaos-error")
		if resp.StatusCode == 500 {
			errors++
		}
		resp.Body.Close()
	}

	// With 50% error rate, expect roughly 15-35 errors (allowing variance)
	if errors < 10 || errors > 40 {
		t.Errorf("Expected ~25 errors with 50%% rate, got %d", errors)
	}
}

// Note: Stateful mocking tests are in stateful_test.go using direct engine setup
// The admin API doesn't support stateful resource configuration via mock creation
