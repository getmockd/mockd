package sse_test

import (
	"bufio"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/getmockd/mockd/pkg/config"
	"github.com/getmockd/mockd/pkg/engine"
)

func TestSSEBasicStream(t *testing.T) {
	// Create server with SSE mock
	cfg := config.DefaultServerConfiguration()
	cfg.HTTPPort = 0 // Disable HTTP server, we'll use httptest
	cfg.HTTPSPort = 0
	server := engine.NewServer(cfg)

	delay := 10
	mock := &config.MockConfiguration{
		ID:   "test-sse",
		Name: "Test SSE Mock",
		Matcher: &config.RequestMatcher{
			Method: "GET",
			Path:   "/events",
		},
		SSE: &config.SSEConfig{
			Events: []config.SSEEventDef{
				{Data: "Hello"},
				{Data: "World"},
				{Data: "!"},
			},
			Timing: config.SSETimingConfig{
				FixedDelay: &delay,
			},
			Lifecycle: config.SSELifecycleConfig{
				MaxEvents: 3,
			},
		},
		Enabled: true,
	}

	if err := server.AddMock(mock); err != nil {
		t.Fatalf("failed to add mock: %v", err)
	}

	// Create test server
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
	server := engine.NewServer(cfg)

	mock := &config.MockConfiguration{
		ID:   "openai-mock",
		Name: "OpenAI Streaming Mock",
		Matcher: &config.RequestMatcher{
			Method: "POST",
			Path:   "/v1/chat/completions",
		},
		SSE: &config.SSEConfig{
			Template: "openai-chat",
			TemplateParams: map[string]interface{}{
				"tokens":        []string{"Hello", "!", " World"},
				"model":         "gpt-4-test",
				"finishReason":  "stop",
				"includeDone":   true,
				"delayPerToken": 10,
			},
		},
		Enabled: true,
	}

	if err := server.AddMock(mock); err != nil {
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
	server := engine.NewServer(cfg)

	delay := 10
	mock := &config.MockConfiguration{
		ID:   "typed-events",
		Name: "Typed Events Mock",
		Matcher: &config.RequestMatcher{
			Method: "GET",
			Path:   "/notifications",
		},
		SSE: &config.SSEConfig{
			Events: []config.SSEEventDef{
				{Type: "message", Data: "Hello"},
				{Type: "update", Data: "Status changed"},
				{Type: "heartbeat", Data: "ping"},
			},
			Timing: config.SSETimingConfig{
				FixedDelay: &delay,
			},
			Lifecycle: config.SSELifecycleConfig{
				MaxEvents: 3,
			},
		},
		Enabled: true,
	}

	if err := server.AddMock(mock); err != nil {
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
	server := engine.NewServer(cfg)

	mock := &config.MockConfiguration{
		ID:   "chunked-mock",
		Name: "Chunked Response Mock",
		Matcher: &config.RequestMatcher{
			Method: "GET",
			Path:   "/download",
		},
		Chunked: &config.ChunkedConfig{
			Data:       "Hello World! This is chunked data.",
			ChunkSize:  10,
			ChunkDelay: 10,
		},
		Enabled: true,
	}

	if err := server.AddMock(mock); err != nil {
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
	server := engine.NewServer(cfg)

	mock := &config.MockConfiguration{
		ID:   "ndjson-mock",
		Name: "NDJSON Response Mock",
		Matcher: &config.RequestMatcher{
			Method: "GET",
			Path:   "/stream",
		},
		Chunked: &config.ChunkedConfig{
			Format: "ndjson",
			NDJSONItems: []interface{}{
				map[string]interface{}{"id": 1, "name": "Alice"},
				map[string]interface{}{"id": 2, "name": "Bob"},
				map[string]interface{}{"id": 3, "name": "Charlie"},
			},
			ChunkDelay: 10,
		},
		Enabled: true,
	}

	if err := server.AddMock(mock); err != nil {
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
