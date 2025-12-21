package recording

import (
	"encoding/json"
	"testing"
	"time"
)

func TestToWebSocketScenario(t *testing.T) {
	t.Run("converts basic WebSocket recording", func(t *testing.T) {
		recording := createTestWSRecording()

		result, err := ToWebSocketScenario(recording, DefaultStreamConvertOptions())
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
	})

	t.Run("excludes client messages when disabled", func(t *testing.T) {
		recording := createTestWSRecording()

		opts := DefaultStreamConvertOptions()
		opts.IncludeClientMessages = false

		result, err := ToWebSocketScenario(recording, opts)
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

		result, err := ToWebSocketScenario(recording, opts)
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

		result, err := ToWebSocketScenario(recording, opts)
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

		result, err := ToWebSocketScenario(recording, DefaultStreamConvertOptions())
		if err != nil {
			t.Fatalf("ToWebSocketScenario failed: %v", err)
		}

		if result.Metadata == nil {
			t.Fatal("expected metadata to be set")
		}

		if result.Metadata.SourceRecordingID != recording.ID {
			t.Errorf("expected sourceRecordingId to be %s, got %s", recording.ID, result.Metadata.SourceRecordingID)
		}

		if result.Metadata.TotalFrames != 4 {
			t.Errorf("expected totalFrames to be 4, got %d", result.Metadata.TotalFrames)
		}
	})

	t.Run("returns error for nil recording", func(t *testing.T) {
		_, err := ToWebSocketScenario(nil, DefaultStreamConvertOptions())
		if err != ErrRecordingNotFound {
			t.Errorf("expected ErrRecordingNotFound, got %v", err)
		}
	})

	t.Run("returns error for non-WebSocket recording", func(t *testing.T) {
		recording := createTestSSERecording()

		_, err := ToWebSocketScenario(recording, DefaultStreamConvertOptions())
		if err != ErrRecordingNotFound {
			t.Errorf("expected ErrRecordingNotFound, got %v", err)
		}
	})
}

func TestToSSEConfig(t *testing.T) {
	t.Run("converts basic SSE recording", func(t *testing.T) {
		recording := createTestSSERecording()

		result, err := ToSSEConfig(recording, DefaultStreamConvertOptions())
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
	})

	t.Run("preserves per-event delays", func(t *testing.T) {
		recording := createTestSSERecording()

		opts := DefaultStreamConvertOptions()
		opts.SimplifyTiming = false

		result, err := ToSSEConfig(recording, opts)
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

		result, err := ToSSEConfig(recording, opts)
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

		result, err := ToSSEConfig(recording, DefaultStreamConvertOptions())
		if err != nil {
			t.Fatalf("ToSSEConfig failed: %v", err)
		}

		if result.Lifecycle.MaxEvents != 3 {
			t.Errorf("expected maxEvents to be 3, got %d", result.Lifecycle.MaxEvents)
		}
	})

	t.Run("enables resume by default", func(t *testing.T) {
		recording := createTestSSERecording()

		result, err := ToSSEConfig(recording, DefaultStreamConvertOptions())
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

		result, err := ToSSEConfig(recording, DefaultStreamConvertOptions())
		if err != nil {
			t.Fatalf("ToSSEConfig failed: %v", err)
		}

		if result.Metadata == nil {
			t.Fatal("expected metadata to be set")
		}

		if result.Metadata.SourceRecordingID != recording.ID {
			t.Errorf("expected sourceRecordingId to be %s, got %s", recording.ID, result.Metadata.SourceRecordingID)
		}

		if result.Metadata.TotalEvents != 3 {
			t.Errorf("expected totalEvents to be 3, got %d", result.Metadata.TotalEvents)
		}
	})

	t.Run("parses JSON data", func(t *testing.T) {
		recording := createTestSSERecordingWithJSON()

		result, err := ToSSEConfig(recording, DefaultStreamConvertOptions())
		if err != nil {
			t.Fatalf("ToSSEConfig failed: %v", err)
		}

		// First event should have parsed JSON
		if _, ok := result.Events[0].Data.(map[string]interface{}); !ok {
			t.Errorf("expected first event data to be parsed JSON, got %T", result.Events[0].Data)
		}
	})

	t.Run("returns error for nil recording", func(t *testing.T) {
		_, err := ToSSEConfig(nil, DefaultStreamConvertOptions())
		if err != ErrRecordingNotFound {
			t.Errorf("expected ErrRecordingNotFound, got %v", err)
		}
	})

	t.Run("returns error for non-SSE recording", func(t *testing.T) {
		recording := createTestWSRecording()

		_, err := ToSSEConfig(recording, DefaultStreamConvertOptions())
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

		if _, ok := result.Config.(*WebSocketScenarioConfig); !ok {
			t.Errorf("expected config to be *WebSocketScenarioConfig, got %T", result.Config)
		}

		if len(result.ConfigJSON) == 0 {
			t.Error("expected configJson to be set")
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

		if _, ok := result.Config.(*SSEMockConfig); !ok {
			t.Errorf("expected config to be *SSEMockConfig, got %T", result.Config)
		}

		if len(result.ConfigJSON) == 0 {
			t.Error("expected configJson to be set")
		}
	})

	t.Run("generates valid JSON", func(t *testing.T) {
		recording := createTestWSRecording()

		result, err := ConvertStreamRecording(recording, DefaultStreamConvertOptions())
		if err != nil {
			t.Fatalf("ConvertStreamRecording failed: %v", err)
		}

		// Verify JSON is valid
		var parsed WebSocketScenarioConfig
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
		ID:        NewULID(),
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
		ID:        NewULID(),
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
		ID:        NewULID(),
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
		ID:        NewULID(),
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
