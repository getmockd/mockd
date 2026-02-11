package recording

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/getmockd/mockd/internal/id"
	"github.com/getmockd/mockd/pkg/config"
	"github.com/getmockd/mockd/pkg/mock"
)

func TestToWebSocketScenario(t *testing.T) {
	t.Run("converts basic WebSocket recording", func(t *testing.T) {
		recording := createTestWSRecording()

		result, metadata, err := ToWebSocketScenario(recording, DefaultStreamConvertOptions())
		if err != nil {
			t.Fatalf("ToWebSocketScenario failed: %v", err)
		}

		if result.Name != "Recorded: "+recording.ID {
			t.Errorf("expected name to contain recording ID, got %s", result.Name)
		}

		// Should have 4 steps: 2 server messages, 2 client messages
		if len(result.Steps) != 4 {
			t.Errorf("expected 4 steps, got %d", len(result.Steps))
		}

		// First step should be server message (send)
		if result.Steps[0].Type != "send" {
			t.Errorf("expected first step to be 'send', got '%s'", result.Steps[0].Type)
		}

		// Second step should be client message (expect)
		if result.Steps[1].Type != "expect" {
			t.Errorf("expected second step to be 'expect', got '%s'", result.Steps[1].Type)
		}

		// Check metadata was returned
		if metadata == nil {
			t.Fatal("expected metadata to be returned")
		}
	})

	t.Run("excludes client messages when disabled", func(t *testing.T) {
		recording := createTestWSRecording()

		opts := DefaultStreamConvertOptions()
		opts.IncludeClientMessages = false

		result, _, err := ToWebSocketScenario(recording, opts)
		if err != nil {
			t.Fatalf("ToWebSocketScenario failed: %v", err)
		}

		// Should only have 2 server messages
		if len(result.Steps) != 2 {
			t.Errorf("expected 2 steps, got %d", len(result.Steps))
		}

		for _, step := range result.Steps {
			if step.Type == "expect" {
				t.Error("found 'expect' step when client messages should be excluded")
			}
		}
	})

	t.Run("deduplicates consecutive messages", func(t *testing.T) {
		recording := createTestWSRecordingWithDuplicates()

		opts := DefaultStreamConvertOptions()
		opts.DeduplicateMessages = true

		result, _, err := ToWebSocketScenario(recording, opts)
		if err != nil {
			t.Fatalf("ToWebSocketScenario failed: %v", err)
		}

		// Original has 3 frames: 2 consecutive duplicate server messages + 1 client message
		// With deduplication, the duplicate "Hello from server" should be removed
		// Resulting in 2 steps (1 server + 1 client)
		if len(result.Steps) != 2 {
			t.Errorf("expected 2 steps after deduplication (removed 1 consecutive duplicate), got %d", len(result.Steps))
		}
	})

	t.Run("simplifies timing", func(t *testing.T) {
		recording := createTestWSRecording()

		opts := DefaultStreamConvertOptions()
		opts.SimplifyTiming = true
		opts.MinDelay = 50
		opts.MaxDelay = 1000

		result, _, err := ToWebSocketScenario(recording, opts)
		if err != nil {
			t.Fatalf("ToWebSocketScenario failed: %v", err)
		}

		// Verify delays are within bounds
		for _, step := range result.Steps {
			if step.Message != nil && step.Message.Delay != "" {
				d, err := time.ParseDuration(step.Message.Delay)
				if err != nil {
					t.Errorf("invalid delay format: %s", step.Message.Delay)
					continue
				}
				if d.Milliseconds() > int64(opts.MaxDelay) {
					t.Errorf("delay %s exceeds max delay %dms", step.Message.Delay, opts.MaxDelay)
				}
			}
		}
	})

	t.Run("includes metadata", func(t *testing.T) {
		recording := createTestWSRecording()

		_, metadata, err := ToWebSocketScenario(recording, DefaultStreamConvertOptions())
		if err != nil {
			t.Fatalf("ToWebSocketScenario failed: %v", err)
		}

		if metadata == nil {
			t.Fatal("expected metadata to be set")
			return
		}

		if metadata.SourceRecordingID != recording.ID {
			t.Errorf("expected sourceRecordingId to be %s, got %s", recording.ID, metadata.SourceRecordingID)
		}

		if metadata.ItemCount != 4 {
			t.Errorf("expected itemCount to be 4, got %d", metadata.ItemCount)
		}

		if metadata.Protocol != ProtocolWebSocket {
			t.Errorf("expected protocol to be websocket, got %s", metadata.Protocol)
		}
	})

	t.Run("returns error for nil recording", func(t *testing.T) {
		_, _, err := ToWebSocketScenario(nil, DefaultStreamConvertOptions())
		if err != ErrRecordingNotFound {
			t.Errorf("expected ErrRecordingNotFound, got %v", err)
		}
	})

	t.Run("returns error for non-WebSocket recording", func(t *testing.T) {
		recording := createTestSSERecording()

		_, _, err := ToWebSocketScenario(recording, DefaultStreamConvertOptions())
		if err != ErrRecordingNotFound {
			t.Errorf("expected ErrRecordingNotFound, got %v", err)
		}
	})
}

func TestToSSEConfig(t *testing.T) {
	t.Run("converts basic SSE recording", func(t *testing.T) {
		recording := createTestSSERecording()

		result, metadata, err := ToSSEConfig(recording, DefaultStreamConvertOptions())
		if err != nil {
			t.Fatalf("ToSSEConfig failed: %v", err)
		}

		if len(result.Events) != 3 {
			t.Errorf("expected 3 events, got %d", len(result.Events))
		}

		// Check first event
		if result.Events[0].Type != "message" {
			t.Errorf("expected first event type to be 'message', got '%s'", result.Events[0].Type)
		}

		// Check metadata was returned
		if metadata == nil {
			t.Fatal("expected metadata to be returned")
		}
	})

	t.Run("preserves per-event delays", func(t *testing.T) {
		recording := createTestSSERecording()

		opts := DefaultStreamConvertOptions()
		opts.SimplifyTiming = false

		result, _, err := ToSSEConfig(recording, opts)
		if err != nil {
			t.Fatalf("ToSSEConfig failed: %v", err)
		}

		if len(result.Timing.PerEventDelays) != 3 {
			t.Errorf("expected 3 per-event delays, got %d", len(result.Timing.PerEventDelays))
		}
	})

	t.Run("uses fixed delay when simplifying", func(t *testing.T) {
		recording := createTestSSERecording()

		opts := DefaultStreamConvertOptions()
		opts.SimplifyTiming = true

		result, _, err := ToSSEConfig(recording, opts)
		if err != nil {
			t.Fatalf("ToSSEConfig failed: %v", err)
		}

		if result.Timing.FixedDelay == nil {
			t.Error("expected fixed delay to be set when simplifying timing")
		}

		if result.Timing.PerEventDelays != nil {
			t.Error("expected per-event delays to be nil when simplifying timing")
		}
	})

	t.Run("sets lifecycle max events", func(t *testing.T) {
		recording := createTestSSERecording()

		result, _, err := ToSSEConfig(recording, DefaultStreamConvertOptions())
		if err != nil {
			t.Fatalf("ToSSEConfig failed: %v", err)
		}

		if result.Lifecycle.MaxEvents != 3 {
			t.Errorf("expected maxEvents to be 3, got %d", result.Lifecycle.MaxEvents)
		}
	})

	t.Run("enables resume by default", func(t *testing.T) {
		recording := createTestSSERecording()

		result, _, err := ToSSEConfig(recording, DefaultStreamConvertOptions())
		if err != nil {
			t.Fatalf("ToSSEConfig failed: %v", err)
		}

		if !result.Resume.Enabled {
			t.Error("expected resume to be enabled")
		}

		if result.Resume.BufferSize != 3 {
			t.Errorf("expected buffer size to be 3, got %d", result.Resume.BufferSize)
		}
	})

	t.Run("includes metadata", func(t *testing.T) {
		recording := createTestSSERecording()

		_, metadata, err := ToSSEConfig(recording, DefaultStreamConvertOptions())
		if err != nil {
			t.Fatalf("ToSSEConfig failed: %v", err)
		}

		if metadata == nil {
			t.Fatal("expected metadata to be set")
			return
		}

		if metadata.SourceRecordingID != recording.ID {
			t.Errorf("expected sourceRecordingId to be %s, got %s", recording.ID, metadata.SourceRecordingID)
		}

		if metadata.ItemCount != 3 {
			t.Errorf("expected itemCount to be 3, got %d", metadata.ItemCount)
		}

		if metadata.Protocol != ProtocolSSE {
			t.Errorf("expected protocol to be sse, got %s", metadata.Protocol)
		}
	})

	t.Run("parses JSON data", func(t *testing.T) {
		recording := createTestSSERecordingWithJSON()

		result, _, err := ToSSEConfig(recording, DefaultStreamConvertOptions())
		if err != nil {
			t.Fatalf("ToSSEConfig failed: %v", err)
		}

		// First event should have parsed JSON
		if _, ok := result.Events[0].Data.(map[string]interface{}); !ok {
			t.Errorf("expected first event data to be parsed JSON, got %T", result.Events[0].Data)
		}
	})

	t.Run("returns error for nil recording", func(t *testing.T) {
		_, _, err := ToSSEConfig(nil, DefaultStreamConvertOptions())
		if err != ErrRecordingNotFound {
			t.Errorf("expected ErrRecordingNotFound, got %v", err)
		}
	})

	t.Run("returns error for non-SSE recording", func(t *testing.T) {
		recording := createTestWSRecording()

		_, _, err := ToSSEConfig(recording, DefaultStreamConvertOptions())
		if err != ErrRecordingNotFound {
			t.Errorf("expected ErrRecordingNotFound, got %v", err)
		}
	})
}

func TestConvertStreamRecording(t *testing.T) {
	t.Run("converts WebSocket recording", func(t *testing.T) {
		recording := createTestWSRecording()

		result, err := ConvertStreamRecording(recording, DefaultStreamConvertOptions())
		if err != nil {
			t.Fatalf("ConvertStreamRecording failed: %v", err)
		}

		if result.Protocol != ProtocolWebSocket {
			t.Errorf("expected protocol to be websocket, got %s", result.Protocol)
		}

		if _, ok := result.Config.(*mock.WSScenarioConfig); !ok {
			t.Errorf("expected config to be *mock.WSScenarioConfig, got %T", result.Config)
		}

		if len(result.ConfigJSON) == 0 {
			t.Error("expected configJson to be set")
		}

		if result.Metadata == nil {
			t.Error("expected metadata to be set")
		}
	})

	t.Run("converts SSE recording", func(t *testing.T) {
		recording := createTestSSERecording()

		result, err := ConvertStreamRecording(recording, DefaultStreamConvertOptions())
		if err != nil {
			t.Fatalf("ConvertStreamRecording failed: %v", err)
		}

		if result.Protocol != ProtocolSSE {
			t.Errorf("expected protocol to be sse, got %s", result.Protocol)
		}

		if _, ok := result.Config.(*mock.SSEConfig); !ok {
			t.Errorf("expected config to be *mock.SSEConfig, got %T", result.Config)
		}

		if len(result.ConfigJSON) == 0 {
			t.Error("expected configJson to be set")
		}

		if result.Metadata == nil {
			t.Error("expected metadata to be set")
		}
	})

	t.Run("generates valid JSON", func(t *testing.T) {
		recording := createTestWSRecording()

		result, err := ConvertStreamRecording(recording, DefaultStreamConvertOptions())
		if err != nil {
			t.Fatalf("ConvertStreamRecording failed: %v", err)
		}

		// Verify JSON is valid
		var parsed mock.WSScenarioConfig
		if err := json.Unmarshal(result.ConfigJSON, &parsed); err != nil {
			t.Errorf("failed to parse generated JSON: %v", err)
		}

		if parsed.Name == "" {
			t.Error("parsed config has empty name")
		}
	})
}

// Helper functions to create test recordings

func createTestWSRecording() *StreamRecording {
	startTime := time.Now().Add(-5 * time.Second)
	recording := &StreamRecording{
		ID:        id.ULID(),
		Version:   FormatVersion,
		Protocol:  ProtocolWebSocket,
		Name:      "Test WebSocket Recording",
		Status:    RecordingStatusComplete,
		StartTime: startTime,
		Metadata: RecordingMetadata{
			Path: "/ws/test",
		},
		WebSocket: &WebSocketRecordingData{
			ConnectedAt: startTime,
			Frames: []WebSocketFrame{
				{
					Sequence:     1,
					Timestamp:    startTime,
					RelativeMs:   0,
					Direction:    DirectionServerToClient,
					MessageType:  MessageTypeText,
					Data:         "Hello from server",
					DataEncoding: DataEncodingUTF8,
					DataSize:     17,
				},
				{
					Sequence:     2,
					Timestamp:    startTime.Add(100 * time.Millisecond),
					RelativeMs:   100,
					Direction:    DirectionClientToServer,
					MessageType:  MessageTypeText,
					Data:         "Hello from client",
					DataEncoding: DataEncodingUTF8,
					DataSize:     17,
				},
				{
					Sequence:     3,
					Timestamp:    startTime.Add(200 * time.Millisecond),
					RelativeMs:   200,
					Direction:    DirectionServerToClient,
					MessageType:  MessageTypeText,
					Data:         "Response from server",
					DataEncoding: DataEncodingUTF8,
					DataSize:     20,
				},
				{
					Sequence:     4,
					Timestamp:    startTime.Add(300 * time.Millisecond),
					RelativeMs:   300,
					Direction:    DirectionClientToServer,
					MessageType:  MessageTypeText,
					Data:         "Bye from client",
					DataEncoding: DataEncodingUTF8,
					DataSize:     15,
				},
			},
		},
	}
	return recording
}

func createTestWSRecordingWithDuplicates() *StreamRecording {
	startTime := time.Now().Add(-5 * time.Second)
	recording := &StreamRecording{
		ID:        id.ULID(),
		Version:   FormatVersion,
		Protocol:  ProtocolWebSocket,
		Name:      "Test WebSocket Recording with Duplicates",
		Status:    RecordingStatusComplete,
		StartTime: startTime,
		Metadata: RecordingMetadata{
			Path: "/ws/test",
		},
		WebSocket: &WebSocketRecordingData{
			ConnectedAt: startTime,
			Frames: []WebSocketFrame{
				{
					Sequence:     1,
					Timestamp:    startTime,
					RelativeMs:   0,
					Direction:    DirectionServerToClient,
					MessageType:  MessageTypeText,
					Data:         "Hello from server",
					DataEncoding: DataEncodingUTF8,
					DataSize:     17,
				},
				{
					Sequence:     2,
					Timestamp:    startTime.Add(50 * time.Millisecond),
					RelativeMs:   50,
					Direction:    DirectionServerToClient,
					MessageType:  MessageTypeText,
					Data:         "Hello from server", // Consecutive duplicate
					DataEncoding: DataEncodingUTF8,
					DataSize:     17,
				},
				{
					Sequence:     3,
					Timestamp:    startTime.Add(100 * time.Millisecond),
					RelativeMs:   100,
					Direction:    DirectionClientToServer,
					MessageType:  MessageTypeText,
					Data:         "Hello from client",
					DataEncoding: DataEncodingUTF8,
					DataSize:     17,
				},
			},
		},
	}
	return recording
}

func createTestSSERecording() *StreamRecording {
	startTime := time.Now().Add(-5 * time.Second)
	recording := &StreamRecording{
		ID:        id.ULID(),
		Version:   FormatVersion,
		Protocol:  ProtocolSSE,
		Name:      "Test SSE Recording",
		Status:    RecordingStatusComplete,
		StartTime: startTime,
		Metadata: RecordingMetadata{
			Path: "/api/events",
		},
		SSE: &SSERecordingData{
			StreamStartedAt: startTime,
			Events: []SSEEvent{
				{
					Sequence:   1,
					Timestamp:  startTime,
					RelativeMs: 0,
					EventType:  "message",
					Data:       "First event",
					ID:         "1",
					DataSize:   11,
				},
				{
					Sequence:   2,
					Timestamp:  startTime.Add(100 * time.Millisecond),
					RelativeMs: 100,
					EventType:  "message",
					Data:       "Second event",
					ID:         "2",
					DataSize:   12,
				},
				{
					Sequence:   3,
					Timestamp:  startTime.Add(250 * time.Millisecond),
					RelativeMs: 250,
					EventType:  "done",
					Data:       "[DONE]",
					ID:         "3",
					DataSize:   6,
				},
			},
		},
	}
	return recording
}

func createTestSSERecordingWithJSON() *StreamRecording {
	startTime := time.Now().Add(-5 * time.Second)
	recording := &StreamRecording{
		ID:        id.ULID(),
		Version:   FormatVersion,
		Protocol:  ProtocolSSE,
		Name:      "Test SSE Recording with JSON",
		Status:    RecordingStatusComplete,
		StartTime: startTime,
		Metadata: RecordingMetadata{
			Path:             "/api/chat",
			DetectedTemplate: "openai",
		},
		SSE: &SSERecordingData{
			StreamStartedAt: startTime,
			Events: []SSEEvent{
				{
					Sequence:   1,
					Timestamp:  startTime,
					RelativeMs: 0,
					EventType:  "message",
					Data:       `{"choices":[{"delta":{"content":"Hello"}}]}`,
					ID:         "1",
					DataSize:   45,
				},
				{
					Sequence:   2,
					Timestamp:  startTime.Add(50 * time.Millisecond),
					RelativeMs: 50,
					EventType:  "message",
					Data:       `{"choices":[{"delta":{"content":" world"}}]}`,
					ID:         "2",
					DataSize:   46,
				},
			},
		},
	}
	return recording
}

// Tests for HTTP recording conversion

func TestToMock(t *testing.T) {
	t.Run("converts basic recording to mock", func(t *testing.T) {
		rec := createTestHTTPRecording("GET", "/api/users", 200)

		mockCfg := ToMock(rec, DefaultConvertOptions())

		if mockCfg.HTTP == nil || mockCfg.HTTP.Matcher == nil {
			t.Fatal("expected HTTP.Matcher to be set")
		}
		if mockCfg.HTTP.Matcher.Method != "GET" {
			t.Errorf("expected method GET, got %s", mockCfg.HTTP.Matcher.Method)
		}
		if mockCfg.HTTP.Matcher.Path != "/api/users" {
			t.Errorf("expected path /api/users, got %s", mockCfg.HTTP.Matcher.Path)
		}
		if mockCfg.HTTP.Response == nil {
			t.Fatal("expected HTTP.Response to be set")
		}
		if mockCfg.HTTP.Response.StatusCode != 200 {
			t.Errorf("expected status 200, got %d", mockCfg.HTTP.Response.StatusCode)
		}
		if mockCfg.Enabled != nil && !*mockCfg.Enabled {
			t.Error("expected mock to be enabled")
		}
	})

	t.Run("includes headers when option is set", func(t *testing.T) {
		rec := createTestHTTPRecording("POST", "/api/users", 201)
		rec.Request.Headers = make(map[string][]string)
		rec.Request.Headers["Content-Type"] = []string{"application/json"}

		opts := ConvertOptions{IncludeHeaders: true}
		mockCfg := ToMock(rec, opts)

		if mockCfg.HTTP == nil || mockCfg.HTTP.Matcher == nil {
			t.Fatal("expected HTTP.Matcher to be set")
		}
		if mockCfg.HTTP.Matcher.Headers == nil {
			t.Fatal("expected headers to be set")
		}
		if mockCfg.HTTP.Matcher.Headers["Content-Type"] != "application/json" {
			t.Errorf("expected Content-Type header, got %v", mockCfg.HTTP.Matcher.Headers)
		}
	})

	t.Run("excludes dynamic response headers", func(t *testing.T) {
		rec := createTestHTTPRecording("GET", "/api/users", 200)
		rec.Response.Headers = make(map[string][]string)
		rec.Response.Headers["Content-Type"] = []string{"application/json"}
		rec.Response.Headers["Date"] = []string{"Mon, 01 Jan 2024 00:00:00 GMT"}
		rec.Response.Headers["Content-Length"] = []string{"100"}

		mockCfg := ToMock(rec, DefaultConvertOptions())

		if mockCfg.HTTP == nil || mockCfg.HTTP.Response == nil {
			t.Fatal("expected HTTP.Response to be set")
		}
		if mockCfg.HTTP.Response.Headers["Content-Type"] != "application/json" {
			t.Error("expected Content-Type to be preserved")
		}
		if _, ok := mockCfg.HTTP.Response.Headers["Date"]; ok {
			t.Error("Date header should be excluded")
		}
		if _, ok := mockCfg.HTTP.Response.Headers["Content-Length"]; ok {
			t.Error("Content-Length header should be excluded")
		}
	})
}

func TestFilterRecordings(t *testing.T) {
	recordings := []*Recording{
		createTestHTTPRecording("GET", "/api/users", 200),
		createTestHTTPRecording("POST", "/api/users", 201),
		createTestHTTPRecording("GET", "/api/products", 200),
		createTestHTTPRecording("DELETE", "/api/users/123", 204),
		createTestHTTPRecording("GET", "/api/users/456", 404),
		createTestHTTPRecording("POST", "/api/orders", 500),
	}

	t.Run("filters by path pattern", func(t *testing.T) {
		opts := FilterOptions{PathPattern: "/api/users*"}
		filtered := FilterRecordings(recordings, opts)

		// Should match: /api/users (2x), /api/users/123, /api/users/456
		if len(filtered) != 4 {
			t.Errorf("expected 4 recordings matching /api/users*, got %d", len(filtered))
			for _, r := range filtered {
				t.Logf("matched: %s", r.Request.Path)
			}
		}
	})

	t.Run("filters by HTTP method", func(t *testing.T) {
		opts := FilterOptions{Methods: []string{"GET"}}
		filtered := FilterRecordings(recordings, opts)

		if len(filtered) != 3 {
			t.Errorf("expected 3 GET recordings, got %d", len(filtered))
		}

		for _, r := range filtered {
			if r.Request.Method != "GET" {
				t.Errorf("expected GET method, got %s", r.Request.Method)
			}
		}
	})

	t.Run("filters by multiple methods", func(t *testing.T) {
		opts := FilterOptions{Methods: []string{"GET", "POST"}}
		filtered := FilterRecordings(recordings, opts)

		if len(filtered) != 5 {
			t.Errorf("expected 5 GET/POST recordings, got %d", len(filtered))
		}
	})

	t.Run("filters by specific status codes", func(t *testing.T) {
		opts := FilterOptions{StatusCodes: []int{200, 201}}
		filtered := FilterRecordings(recordings, opts)

		if len(filtered) != 3 {
			t.Errorf("expected 3 recordings with status 200/201, got %d", len(filtered))
		}
	})

	t.Run("filters by status range 2xx", func(t *testing.T) {
		opts := FilterOptions{StatusRange: "2xx"}
		filtered := FilterRecordings(recordings, opts)

		if len(filtered) != 4 {
			t.Errorf("expected 4 recordings with 2xx status, got %d", len(filtered))
		}
	})

	t.Run("filters by status range 4xx", func(t *testing.T) {
		opts := FilterOptions{StatusRange: "4xx"}
		filtered := FilterRecordings(recordings, opts)

		if len(filtered) != 1 {
			t.Errorf("expected 1 recording with 4xx status, got %d", len(filtered))
		}
	})

	t.Run("filters by status range 5xx", func(t *testing.T) {
		opts := FilterOptions{StatusRange: "5xx"}
		filtered := FilterRecordings(recordings, opts)

		if len(filtered) != 1 {
			t.Errorf("expected 1 recording with 5xx status, got %d", len(filtered))
		}
	})

	t.Run("combines multiple filters", func(t *testing.T) {
		opts := FilterOptions{
			PathPattern: "/api/users*",
			Methods:     []string{"GET"},
			StatusRange: "2xx",
		}
		filtered := FilterRecordings(recordings, opts)

		if len(filtered) != 1 {
			t.Errorf("expected 1 recording matching all filters, got %d", len(filtered))
		}
	})

	t.Run("returns all when no filters set", func(t *testing.T) {
		opts := FilterOptions{}
		filtered := FilterRecordings(recordings, opts)

		if len(filtered) != len(recordings) {
			t.Errorf("expected all %d recordings, got %d", len(recordings), len(filtered))
		}
	})
}

func TestSmartPathMatcher(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"/users/123", "/users/{id}"},
		{"/users/456789", "/users/{id}"},
		{"/orders/abc-def-1234-5678-ghij-klmn", "/orders/{id}"},              // UUID-like
		{"/products/12345678-1234-1234-1234-123456789012", "/products/{id}"}, // UUID
		{"/api/v1/users", "/api/v1/users"},                                   // No ID
		{"/api/v1/users/42/posts/99", "/api/v1/users/{id}/posts/{id}"},       // Multiple IDs
		{"/", "/"},
		{"", ""},
		{"/users", "/users"},
		{"/users/", "/users/"},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			result := SmartPathMatcher(tc.input)
			if result != tc.expected {
				t.Errorf("SmartPathMatcher(%q) = %q, want %q", tc.input, result, tc.expected)
			}
		})
	}
}

func TestIsUUID(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"12345678-1234-1234-1234-123456789012", true},
		{"abcdef12-abcd-abcd-abcd-abcdef123456", true},
		{"ABCDEF12-ABCD-ABCD-ABCD-ABCDEF123456", true},
		{"12345678123412341234123456789012", false},      // No dashes
		{"12345678-1234-1234-1234-12345678901", false},   // Too short
		{"12345678-1234-1234-1234-1234567890123", false}, // Too long
		{"not-a-uuid", false},
		{"123", false},
		{"", false},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			result := isUUID(tc.input)
			if result != tc.expected {
				t.Errorf("isUUID(%q) = %v, want %v", tc.input, result, tc.expected)
			}
		})
	}
}

func TestIsNumericID(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"123", true},
		{"0", true},
		{"999999999999", true},
		{"-1", true},
		{"abc", false},
		{"12a3", false},
		{"", false},
		{"1.5", false},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			result := isNumericID(tc.input)
			if result != tc.expected {
				t.Errorf("isNumericID(%q) = %v, want %v", tc.input, result, tc.expected)
			}
		})
	}
}

func TestDeduplicatePaths(t *testing.T) {
	t.Run("deduplicates using first strategy", func(t *testing.T) {
		mocks := createTestMocksForDedup()

		result := DeduplicatePaths(mocks, "first")

		if len(result) != 2 {
			t.Errorf("expected 2 unique mocks, got %d", len(result))
		}

		// First mock should have response body "first"
		if result[0].HTTP == nil || result[0].HTTP.Response == nil {
			t.Fatal("expected HTTP.Response to be set")
		}
		if result[0].HTTP.Response.Body != "first" {
			t.Errorf("expected first mock body to be 'first', got %s", result[0].HTTP.Response.Body)
		}
	})

	t.Run("deduplicates using last strategy", func(t *testing.T) {
		mocks := createTestMocksForDedup()

		result := DeduplicatePaths(mocks, "last")

		if len(result) != 2 {
			t.Errorf("expected 2 unique mocks, got %d", len(result))
		}

		// First mock should have response body "third" (last of GET /users/{id})
		if result[0].HTTP == nil || result[0].HTTP.Response == nil {
			t.Fatal("expected HTTP.Response to be set")
		}
		if result[0].HTTP.Response.Body != "third" {
			t.Errorf("expected first mock body to be 'third', got %s", result[0].HTTP.Response.Body)
		}
	})

	t.Run("keeps all with all strategy", func(t *testing.T) {
		mocks := createTestMocksForDedup()

		result := DeduplicatePaths(mocks, "all")

		if len(result) != 4 {
			t.Errorf("expected all 4 mocks, got %d", len(result))
		}
	})
}

func TestCheckSensitiveData(t *testing.T) {
	t.Run("detects Authorization header", func(t *testing.T) {
		rec := createTestHTTPRecording("GET", "/api/users", 200)
		rec.Request.Headers = make(map[string][]string)
		rec.Request.Headers["Authorization"] = []string{"Bearer secret-token"}

		warnings := CheckSensitiveData(rec)

		if len(warnings) == 0 {
			t.Error("expected warnings for Authorization header")
		}

		found := false
		for _, w := range warnings {
			if w.Type == "header" && w.Field == "Authorization" {
				found = true
				break
			}
		}
		if !found {
			t.Error("expected warning for Authorization header")
		}
	})

	t.Run("detects API key header", func(t *testing.T) {
		rec := createTestHTTPRecording("GET", "/api/users", 200)
		rec.Request.Headers = make(map[string][]string)
		rec.Request.Headers["X-API-Key"] = []string{"my-api-key"}

		warnings := CheckSensitiveData(rec)

		if len(warnings) == 0 {
			t.Error("expected warnings for X-API-Key header")
		}
	})

	t.Run("detects sensitive cookies", func(t *testing.T) {
		rec := createTestHTTPRecording("GET", "/api/users", 200)
		rec.Request.Headers = make(map[string][]string)
		rec.Request.Headers["Cookie"] = []string{"session_id=abc123; user_token=xyz"}

		warnings := CheckSensitiveData(rec)

		// Should detect at least one sensitive cookie pattern
		if len(warnings) < 1 {
			t.Errorf("expected at least 1 warning for sensitive cookies, got %d", len(warnings))
		}

		// Verify a cookie warning was found
		cookieWarningFound := false
		for _, w := range warnings {
			if w.Type == "cookie" {
				cookieWarningFound = true
				break
			}
		}
		if !cookieWarningFound {
			t.Error("expected at least one cookie warning")
		}
	})

	t.Run("detects sensitive query parameters", func(t *testing.T) {
		rec := createTestHTTPRecording("GET", "/api/users", 200)
		rec.Request.URL = "/api/users?api_key=secret123&user=john"

		warnings := CheckSensitiveData(rec)

		found := false
		for _, w := range warnings {
			if w.Type == "query" && w.Field == "api_key" {
				found = true
				break
			}
		}
		if !found {
			t.Error("expected warning for api_key query parameter")
		}
	})

	t.Run("returns empty for clean recording", func(t *testing.T) {
		rec := createTestHTTPRecording("GET", "/api/users", 200)
		rec.Request.Headers = make(map[string][]string)
		rec.Request.Headers["Content-Type"] = []string{"application/json"}

		warnings := CheckSensitiveData(rec)

		if len(warnings) != 0 {
			t.Errorf("expected no warnings, got %d", len(warnings))
		}
	})
}

func TestParseMethodFilter(t *testing.T) {
	tests := []struct {
		input    string
		expected []string
	}{
		{"GET", []string{"GET"}},
		{"GET,POST", []string{"GET", "POST"}},
		{"get,post,delete", []string{"GET", "POST", "DELETE"}},
		{" GET , POST ", []string{"GET", "POST"}},
		{"", nil},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			result := ParseMethodFilter(tc.input)
			if len(result) != len(tc.expected) {
				t.Errorf("ParseMethodFilter(%q) length = %d, want %d", tc.input, len(result), len(tc.expected))
				return
			}
			for i, v := range result {
				if v != tc.expected[i] {
					t.Errorf("ParseMethodFilter(%q)[%d] = %q, want %q", tc.input, i, v, tc.expected[i])
				}
			}
		})
	}
}

func TestParseStatusFilter(t *testing.T) {
	t.Run("parses range pattern", func(t *testing.T) {
		codes, rangeStr := ParseStatusFilter("2xx")
		if len(codes) != 0 {
			t.Error("expected no codes for range pattern")
		}
		if rangeStr != "2xx" {
			t.Errorf("expected range '2xx', got '%s'", rangeStr)
		}
	})

	t.Run("parses specific codes", func(t *testing.T) {
		codes, rangeStr := ParseStatusFilter("200,201,404")
		if rangeStr != "" {
			t.Error("expected no range for specific codes")
		}
		if len(codes) != 3 {
			t.Errorf("expected 3 codes, got %d", len(codes))
		}
	})

	t.Run("handles empty string", func(t *testing.T) {
		codes, rangeStr := ParseStatusFilter("")
		if len(codes) != 0 || rangeStr != "" {
			t.Error("expected empty result for empty input")
		}
	})
}

func TestConvertRecordingsWithOptions(t *testing.T) {
	recordings := []*Recording{
		createTestHTTPRecording("GET", "/api/users/123", 200),
		createTestHTTPRecording("GET", "/api/users/456", 200),
		createTestHTTPRecording("POST", "/api/users", 201),
		createTestHTTPRecording("GET", "/api/products", 200),
	}

	// Add Authorization header to first recording
	recordings[0].Request.Headers = make(map[string][]string)
	recordings[0].Request.Headers["Authorization"] = []string{"Bearer token"}

	t.Run("converts with smart matching", func(t *testing.T) {
		opts := SessionConvertOptions{
			ConvertOptions: ConvertOptions{SmartMatch: true},
			Duplicates:     "first",
		}

		result := ConvertRecordingsWithOptions(recordings, opts)

		// Should have deduplicated /users/{id} paths
		if len(result.Mocks) != 3 {
			t.Errorf("expected 3 mocks after smart matching, got %d", len(result.Mocks))
		}
	})

	t.Run("returns warnings for sensitive data", func(t *testing.T) {
		opts := DefaultSessionConvertOptions()

		result := ConvertRecordingsWithOptions(recordings, opts)

		if len(result.Warnings) == 0 {
			t.Error("expected warnings for Authorization header")
		}
	})

	t.Run("tracks filtered count", func(t *testing.T) {
		opts := SessionConvertOptions{
			Filter: FilterOptions{Methods: []string{"GET"}},
		}

		result := ConvertRecordingsWithOptions(recordings, opts)

		if result.Filtered != 1 {
			t.Errorf("expected 1 filtered recording (POST), got %d", result.Filtered)
		}
		if result.Total != 4 {
			t.Errorf("expected total 4, got %d", result.Total)
		}
	})
}

func TestToMocksWithStrategy(t *testing.T) {
	recordings := []*Recording{
		createTestHTTPRecordingWithBody("GET", "/api/users", 200, "first"),
		createTestHTTPRecordingWithBody("GET", "/api/users", 200, "second"),
		createTestHTTPRecordingWithBody("POST", "/api/users", 201, "post"),
	}

	t.Run("first strategy keeps first occurrence", func(t *testing.T) {
		mocks := ToMocksWithStrategy(recordings, DefaultConvertOptions(), "first")

		if len(mocks) != 2 {
			t.Errorf("expected 2 mocks, got %d", len(mocks))
		}

		// Find the GET mock
		for _, m := range mocks {
			if m.HTTP != nil && m.HTTP.Matcher != nil && m.HTTP.Matcher.Method == "GET" {
				if m.HTTP.Response.Body != "first" {
					t.Errorf("expected first GET response, got %s", m.HTTP.Response.Body)
				}
			}
		}
	})

	t.Run("last strategy keeps last occurrence", func(t *testing.T) {
		mocks := ToMocksWithStrategy(recordings, DefaultConvertOptions(), "last")

		if len(mocks) != 2 {
			t.Errorf("expected 2 mocks, got %d", len(mocks))
		}

		// Find the GET mock
		for _, m := range mocks {
			if m.HTTP != nil && m.HTTP.Matcher != nil && m.HTTP.Matcher.Method == "GET" {
				if m.HTTP.Response.Body != "second" {
					t.Errorf("expected second GET response, got %s", m.HTTP.Response.Body)
				}
			}
		}
	})

	t.Run("all strategy keeps all", func(t *testing.T) {
		mocks := ToMocksWithStrategy(recordings, DefaultConvertOptions(), "all")

		if len(mocks) != 3 {
			t.Errorf("expected 3 mocks, got %d", len(mocks))
		}
	})
}

// Helper functions for HTTP recording tests

func createTestHTTPRecording(method, path string, status int) *Recording {
	return &Recording{
		ID:        id.Short(),
		SessionID: "test-session",
		Timestamp: time.Now(),
		Request: RecordedRequest{
			Method:  method,
			URL:     path,
			Path:    path,
			Host:    "localhost",
			Scheme:  "http",
			Headers: make(map[string][]string),
		},
		Response: RecordedResponse{
			StatusCode: status,
			Status:     http.StatusText(status),
			Headers:    make(map[string][]string),
			Body:       []byte(`{"message":"test"}`),
		},
		Duration: 100 * time.Millisecond,
	}
}

func createTestHTTPRecordingWithBody(method, path string, status int, body string) *Recording {
	rec := createTestHTTPRecording(method, path, status)
	rec.Response.Body = []byte(body)
	return rec
}

func createTestMocksForDedup() []*config.MockConfiguration {
	now := time.Now()
	enabled := true
	return []*config.MockConfiguration{
		{
			ID:        "1",
			Enabled:   &enabled,
			Type:      mock.TypeHTTP,
			CreatedAt: now,
			UpdatedAt: now,
			HTTP: &mock.HTTPSpec{
				Matcher:  &mock.HTTPMatcher{Method: "GET", Path: "/users/{id}"},
				Response: &mock.HTTPResponse{StatusCode: 200, Body: "first"},
			},
		},
		{
			ID:        "2",
			Enabled:   &enabled,
			Type:      mock.TypeHTTP,
			CreatedAt: now,
			UpdatedAt: now,
			HTTP: &mock.HTTPSpec{
				Matcher:  &mock.HTTPMatcher{Method: "GET", Path: "/users/{id}"},
				Response: &mock.HTTPResponse{StatusCode: 200, Body: "second"},
			},
		},
		{
			ID:        "3",
			Enabled:   &enabled,
			Type:      mock.TypeHTTP,
			CreatedAt: now,
			UpdatedAt: now,
			HTTP: &mock.HTTPSpec{
				Matcher:  &mock.HTTPMatcher{Method: "GET", Path: "/users/{id}"},
				Response: &mock.HTTPResponse{StatusCode: 200, Body: "third"},
			},
		},
		{
			ID:        "4",
			Enabled:   &enabled,
			Type:      mock.TypeHTTP,
			CreatedAt: now,
			UpdatedAt: now,
			HTTP: &mock.HTTPSpec{
				Matcher:  &mock.HTTPMatcher{Method: "POST", Path: "/users"},
				Response: &mock.HTTPResponse{StatusCode: 201, Body: "created"},
			},
		},
	}
}
