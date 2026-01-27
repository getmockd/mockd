package sse_test

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/getmockd/mockd/pkg/admin/engineclient"
	"github.com/getmockd/mockd/pkg/config"
	"github.com/getmockd/mockd/pkg/engine"
	"github.com/getmockd/mockd/pkg/mock"
)

// getFreePort gets a free port for testing
func getFreePort() int {
	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		panic(err)
	}
	defer listener.Close()
	return listener.Addr().(*net.TCPAddr).Port
}

// waitForEngineHealth waits for the engine management API to become healthy.
func waitForEngineHealth(ctx context.Context, client *engineclient.Client, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if err := client.Health(ctx); err == nil {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(50 * time.Millisecond):
			// retry
		}
	}
	return fmt.Errorf("timeout waiting for engine health after %v", timeout)
}

func TestSSEBasicStream(t *testing.T) {
	// Create server with SSE mock
	cfg := config.DefaultServerConfiguration()
	cfg.HTTPPort = 0 // Disable HTTP server, we'll use httptest
	cfg.HTTPSPort = 0
	cfg.ManagementPort = getFreePort()
	server := engine.NewServer(cfg)

	// Start server to get control API
	if err := server.Start(); err != nil {
		t.Fatalf("failed to start server: %v", err)
	}
	defer server.Stop()

	// Create engine client and wait for it to be ready
	client := engineclient.New(fmt.Sprintf("http://localhost:%d", server.ManagementPort()))
	ctx := context.Background()
	if err := waitForEngineHealth(ctx, client, 5*time.Second); err != nil {
		t.Fatalf("engine not ready: %v", err)
	}

	delay := 10
	sseEnabled := true
	sseMock := &config.MockConfiguration{
		ID:      "test-sse",
		Name:    "Test SSE Mock",
		Type:    mock.MockTypeHTTP,
		Enabled: &sseEnabled,
		HTTP: &mock.HTTPSpec{
			Matcher: &mock.HTTPMatcher{
				Method: "GET",
				Path:   "/events",
			},
			SSE: &mock.SSEConfig{
				Events: []mock.SSEEventDef{
					{Data: "Hello"},
					{Data: "World"},
					{Data: "!"},
				},
				Timing: mock.SSETimingConfig{
					FixedDelay: &delay,
				},
				Lifecycle: mock.SSELifecycleConfig{
					MaxEvents: 3,
				},
			},
		},
	}

	if _, err := client.CreateMock(context.Background(), sseMock); err != nil {
		t.Fatalf("failed to add mock: %v", err)
	}

	// Create test server using the handler
	ts := httptest.NewServer(server.Handler())
	defer ts.Close()

	// Make SSE request
	req, _ := http.NewRequest("GET", ts.URL+"/events", nil)
	req.Header.Set("Accept", "text/event-stream")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	req = req.WithContext(ctx)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("failed to make request: %v", err)
	}
	defer resp.Body.Close()

	// Check Content-Type
	contentType := resp.Header.Get("Content-Type")
	if contentType != "text/event-stream" {
		t.Errorf("expected Content-Type text/event-stream, got %q", contentType)
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

	if len(events) != 3 {
		t.Errorf("expected 3 events, got %d", len(events))
	}

	expected := []string{"Hello", "World", "!"}
	for i, e := range expected {
		if i < len(events) && events[i] != e {
			t.Errorf("event %d: expected %q, got %q", i, e, events[i])
		}
	}
}

func TestSSEOpenAITemplate(t *testing.T) {
	// Create server with OpenAI template mock
	cfg := config.DefaultServerConfiguration()
	cfg.HTTPPort = 0
	cfg.HTTPSPort = 0
	cfg.ManagementPort = getFreePort()
	server := engine.NewServer(cfg)

	// Start server
	if err := server.Start(); err != nil {
		t.Fatalf("failed to start server: %v", err)
	}
	defer server.Stop()

	// Create engine client and wait for it to be ready
	client := engineclient.New(fmt.Sprintf("http://localhost:%d", server.ManagementPort()))
	ctx := context.Background()
	if err := waitForEngineHealth(ctx, client, 5*time.Second); err != nil {
		t.Fatalf("engine not ready: %v", err)
	}

	openaiEnabled := true
	openaiMock := &config.MockConfiguration{
		ID:      "openai-mock",
		Name:    "OpenAI Streaming Mock",
		Type:    mock.MockTypeHTTP,
		Enabled: &openaiEnabled,
		HTTP: &mock.HTTPSpec{
			Matcher: &mock.HTTPMatcher{
				Method: "POST",
				Path:   "/v1/chat/completions",
			},
			SSE: &mock.SSEConfig{
				Template: "openai-chat",
				TemplateParams: map[string]interface{}{
					"tokens":        []string{"Hello", "!", " World"},
					"model":         "gpt-4-test",
					"finishReason":  "stop",
					"includeDone":   true,
					"delayPerToken": 10,
				},
			},
		},
	}

	if _, err := client.CreateMock(context.Background(), openaiMock); err != nil {
		t.Fatalf("failed to add mock: %v", err)
	}

	// Create test server
	ts := httptest.NewServer(server.Handler())
	defer ts.Close()

	// Make SSE request
	req, _ := http.NewRequest("POST", ts.URL+"/v1/chat/completions", strings.NewReader(`{"stream": true}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	req = req.WithContext(ctx)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("failed to make request: %v", err)
	}
	defer resp.Body.Close()

	// Check Content-Type
	contentType := resp.Header.Get("Content-Type")
	if contentType != "text/event-stream" {
		t.Errorf("expected Content-Type text/event-stream, got %q", contentType)
	}

	// Read events
	scanner := bufio.NewScanner(resp.Body)
	var dataLines []string
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "data:") {
			dataLines = append(dataLines, strings.TrimPrefix(line, "data:"))
		}
	}

	// Should have 3 token events + [DONE]
	if len(dataLines) < 4 {
		t.Errorf("expected at least 4 data lines, got %d", len(dataLines))
	}

	// Last should be [DONE]
	if len(dataLines) > 0 && dataLines[len(dataLines)-1] != "[DONE]" {
		t.Errorf("expected last event to be [DONE], got %q", dataLines[len(dataLines)-1])
	}

	// Verify JSON format of token events
	for i, data := range dataLines[:len(dataLines)-1] {
		if !strings.Contains(data, "chat.completion.chunk") {
			t.Errorf("event %d: expected chat.completion.chunk object, got %q", i, data)
		}
		if !strings.Contains(data, "gpt-4-test") {
			t.Errorf("event %d: expected model gpt-4-test, got %q", i, data)
		}
	}
}

func TestSSEWithEventTypes(t *testing.T) {
	// Create server with typed events
	cfg := config.DefaultServerConfiguration()
	cfg.HTTPPort = 0
	cfg.HTTPSPort = 0
	cfg.ManagementPort = getFreePort()
	server := engine.NewServer(cfg)

	// Start server
	if err := server.Start(); err != nil {
		t.Fatalf("failed to start server: %v", err)
	}
	defer server.Stop()

	// Create engine client and wait for it to be ready
	client := engineclient.New(fmt.Sprintf("http://localhost:%d", server.ManagementPort()))
	ctx := context.Background()
	if err := waitForEngineHealth(ctx, client, 5*time.Second); err != nil {
		t.Fatalf("engine not ready: %v", err)
	}

	delay := 10
	typedEnabled := true
	typedMock := &config.MockConfiguration{
		ID:      "typed-events",
		Name:    "Typed Events Mock",
		Type:    mock.MockTypeHTTP,
		Enabled: &typedEnabled,
		HTTP: &mock.HTTPSpec{
			Matcher: &mock.HTTPMatcher{
				Method: "GET",
				Path:   "/notifications",
			},
			SSE: &mock.SSEConfig{
				Events: []mock.SSEEventDef{
					{Type: "message", Data: "Hello"},
					{Type: "update", Data: "Status changed"},
					{Type: "heartbeat", Data: "ping"},
				},
				Timing: mock.SSETimingConfig{
					FixedDelay: &delay,
				},
				Lifecycle: mock.SSELifecycleConfig{
					MaxEvents: 3,
				},
			},
		},
	}

	if _, err := client.CreateMock(context.Background(), typedMock); err != nil {
		t.Fatalf("failed to add mock: %v", err)
	}

	// Create test server
	ts := httptest.NewServer(server.Handler())
	defer ts.Close()

	// Make SSE request
	req, _ := http.NewRequest("GET", ts.URL+"/notifications", nil)
	req.Header.Set("Accept", "text/event-stream")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	req = req.WithContext(ctx)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("failed to make request: %v", err)
	}
	defer resp.Body.Close()

	// Read and parse events
	scanner := bufio.NewScanner(resp.Body)
	var eventTypes []string
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "event:") {
			eventTypes = append(eventTypes, strings.TrimPrefix(line, "event:"))
		}
	}

	expected := []string{"message", "update", "heartbeat"}
	if len(eventTypes) != len(expected) {
		t.Errorf("expected %d event types, got %d", len(expected), len(eventTypes))
	}

	for i, e := range expected {
		if i < len(eventTypes) && eventTypes[i] != e {
			t.Errorf("event %d: expected type %q, got %q", i, e, eventTypes[i])
		}
	}
}

func TestSSEChunkedResponse(t *testing.T) {
	// Create server with chunked response mock
	cfg := config.DefaultServerConfiguration()
	cfg.HTTPPort = 0
	cfg.HTTPSPort = 0
	cfg.ManagementPort = getFreePort()
	server := engine.NewServer(cfg)

	// Start server
	if err := server.Start(); err != nil {
		t.Fatalf("failed to start server: %v", err)
	}
	defer server.Stop()

	// Create engine client and wait for it to be ready
	client := engineclient.New(fmt.Sprintf("http://localhost:%d", server.ManagementPort()))
	ctx := context.Background()
	if err := waitForEngineHealth(ctx, client, 5*time.Second); err != nil {
		t.Fatalf("engine not ready: %v", err)
	}

	chunkedEnabled := true
	chunkedMock := &config.MockConfiguration{
		ID:      "chunked-mock",
		Name:    "Chunked Response Mock",
		Type:    mock.MockTypeHTTP,
		Enabled: &chunkedEnabled,
		HTTP: &mock.HTTPSpec{
			Matcher: &mock.HTTPMatcher{
				Method: "GET",
				Path:   "/download",
			},
			Chunked: &mock.ChunkedConfig{
				Data:       "Hello World! This is chunked data.",
				ChunkSize:  10,
				ChunkDelay: 10,
			},
		},
	}

	if _, err := client.CreateMock(context.Background(), chunkedMock); err != nil {
		t.Fatalf("failed to add mock: %v", err)
	}

	// Create test server
	ts := httptest.NewServer(server.Handler())
	defer ts.Close()

	// Make request
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req, _ := http.NewRequestWithContext(ctx, "GET", ts.URL+"/download", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("failed to make request: %v", err)
	}
	defer resp.Body.Close()

	// Read response body
	var body strings.Builder
	buf := make([]byte, 1024)
	for {
		n, err := resp.Body.Read(buf)
		if n > 0 {
			body.Write(buf[:n])
		}
		if err != nil {
			break
		}
	}

	expected := "Hello World! This is chunked data."
	if body.String() != expected {
		t.Errorf("expected body %q, got %q", expected, body.String())
	}
}

func TestSSENDJSONResponse(t *testing.T) {
	// Create server with NDJSON response mock
	cfg := config.DefaultServerConfiguration()
	cfg.HTTPPort = 0
	cfg.HTTPSPort = 0
	cfg.ManagementPort = getFreePort()
	server := engine.NewServer(cfg)

	// Start server
	if err := server.Start(); err != nil {
		t.Fatalf("failed to start server: %v", err)
	}
	defer server.Stop()

	// Create engine client and wait for it to be ready
	client := engineclient.New(fmt.Sprintf("http://localhost:%d", server.ManagementPort()))
	ctx := context.Background()
	if err := waitForEngineHealth(ctx, client, 5*time.Second); err != nil {
		t.Fatalf("engine not ready: %v", err)
	}

	ndjsonEnabled := true
	ndjsonMock := &config.MockConfiguration{
		ID:      "ndjson-mock",
		Name:    "NDJSON Response Mock",
		Type:    mock.MockTypeHTTP,
		Enabled: &ndjsonEnabled,
		HTTP: &mock.HTTPSpec{
			Matcher: &mock.HTTPMatcher{
				Method: "GET",
				Path:   "/stream",
			},
			Chunked: &mock.ChunkedConfig{
				Format: "ndjson",
				NDJSONItems: []interface{}{
					map[string]interface{}{"id": 1, "name": "Alice"},
					map[string]interface{}{"id": 2, "name": "Bob"},
					map[string]interface{}{"id": 3, "name": "Charlie"},
				},
				ChunkDelay: 10,
			},
		},
	}

	if _, err := client.CreateMock(context.Background(), ndjsonMock); err != nil {
		t.Fatalf("failed to add mock: %v", err)
	}

	// Create test server
	ts := httptest.NewServer(server.Handler())
	defer ts.Close()

	// Make request
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req, _ := http.NewRequestWithContext(ctx, "GET", ts.URL+"/stream", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("failed to make request: %v", err)
	}
	defer resp.Body.Close()

	// Check Content-Type
	contentType := resp.Header.Get("Content-Type")
	if contentType != "application/x-ndjson" {
		t.Errorf("expected Content-Type application/x-ndjson, got %q", contentType)
	}

	// Read lines
	scanner := bufio.NewScanner(resp.Body)
	var lines []string
	for scanner.Scan() {
		line := scanner.Text()
		if line != "" {
			lines = append(lines, line)
		}
	}

	if len(lines) != 3 {
		t.Errorf("expected 3 lines, got %d", len(lines))
	}

	// Verify JSON format
	for i, line := range lines {
		if !strings.Contains(line, "\"id\"") || !strings.Contains(line, "\"name\"") {
			t.Errorf("line %d: expected JSON object with id and name, got %q", i, line)
		}
	}
}

// ============================================================================
// Full Server Integration Test (with middleware chain)
// ============================================================================
// This test uses the full server startup (srv.Start()) instead of
// httptest.NewServer(srv.Handler()) to ensure the middleware chain
// is properly tested, including metrics middleware that needs http.Flusher.

func TestSSE_FullServer_WithMiddleware(t *testing.T) {
	// This test ensures SSE works through the full middleware chain.
	// SSE requires http.Flusher which metricsResponseWriter must support.

	port := getFreePort()
	mgmtPort := getFreePort()
	cfg := config.DefaultServerConfiguration()
	cfg.HTTPPort = port
	cfg.HTTPSPort = 0
	cfg.ManagementPort = mgmtPort

	server := engine.NewServer(cfg)

	// Start the FULL server (with middleware chain)
	if err := server.Start(); err != nil {
		t.Fatalf("failed to start server: %v", err)
	}
	defer server.Stop()

	// Wait for server to be ready
	time.Sleep(100 * time.Millisecond)

	// Create engine client to add mock via API
	client := engineclient.New(fmt.Sprintf("http://localhost:%d", mgmtPort))
	ctx := context.Background()

	if err := waitForEngineHealth(ctx, client, 5*time.Second); err != nil {
		t.Fatalf("engine not healthy: %v", err)
	}

	// Add SSE mock via management API
	fixedDelay := 100
	sseMwEnabled := true
	sseMock := &config.MockConfiguration{
		ID:      "sse_middleware_test",
		Type:    mock.MockTypeHTTP,
		Name:    "SSE Middleware Test",
		Enabled: &sseMwEnabled,
		HTTP: &mock.HTTPSpec{
			Matcher: &mock.HTTPMatcher{
				Method: "GET",
				Path:   "/events/test",
			},
			SSE: &mock.SSEConfig{
				Events: []mock.SSEEventDef{
					{Type: "connected", Data: map[string]interface{}{"status": "ok"}, ID: "0"},
					{Type: "message", Data: map[string]interface{}{"text": "hello"}, ID: "1"},
					{Type: "message", Data: map[string]interface{}{"text": "world"}, ID: "2"},
				},
				Timing: mock.SSETimingConfig{
					FixedDelay: &fixedDelay,
				},
				Lifecycle: mock.SSELifecycleConfig{
					MaxEvents: 3,
				},
			},
		},
	}

	if _, err := client.CreateMock(ctx, sseMock); err != nil {
		t.Fatalf("failed to add SSE mock: %v", err)
	}

	// Make SSE request - this goes through the FULL middleware chain
	reqCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	url := fmt.Sprintf("http://localhost:%d/events/test", port)
	req, _ := http.NewRequestWithContext(reqCtx, "GET", url, nil)
	req.Header.Set("Accept", "text/event-stream")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("SSE request failed: %v", err)
	}
	defer resp.Body.Close()

	// Verify response headers
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}

	contentType := resp.Header.Get("Content-Type")
	if !strings.HasPrefix(contentType, "text/event-stream") {
		t.Errorf("expected Content-Type text/event-stream, got %q", contentType)
	}

	// Read and verify events
	scanner := bufio.NewScanner(resp.Body)
	var events []string
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "event:") {
			events = append(events, strings.TrimPrefix(line, "event:"))
		}
	}

	if len(events) < 2 {
		t.Errorf("expected at least 2 events, got %d: %v", len(events), events)
	}

	// Verify we got the expected event types
	hasConnected := false
	hasMessage := false
	for _, e := range events {
		if e == "connected" {
			hasConnected = true
		}
		if e == "message" {
			hasMessage = true
		}
	}

	if !hasConnected {
		t.Error("missing 'connected' event")
	}
	if !hasMessage {
		t.Error("missing 'message' event")
	}
}
