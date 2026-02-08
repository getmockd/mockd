// Package recording provides tests for the file-based recording store.
package recording

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/getmockd/mockd/internal/id"
)

// TestNewFileStore tests creating a new FileStore with various configurations.
func TestNewFileStore(t *testing.T) {
	t.Run("creates store with defaults", func(t *testing.T) {
		tmpDir := t.TempDir()
		config := StorageConfig{
			DataDir: tmpDir,
		}

		store, err := NewFileStore(config)
		if err != nil {
			t.Fatalf("NewFileStore failed: %v", err)
		}

		if store.config.MaxBytes != DefaultMaxStorageBytes {
			t.Errorf("expected MaxBytes=%d, got %d", DefaultMaxStorageBytes, store.config.MaxBytes)
		}
		if store.config.WarnPercent != DefaultWarnPercent {
			t.Errorf("expected WarnPercent=%d, got %d", DefaultWarnPercent, store.config.WarnPercent)
		}
		if store.config.RedactValue != "[REDACTED]" {
			t.Errorf("expected RedactValue=[REDACTED], got %s", store.config.RedactValue)
		}
		if len(store.config.FilterHeaders) != len(DefaultFilterHeaders) {
			t.Errorf("expected %d FilterHeaders, got %d", len(DefaultFilterHeaders), len(store.config.FilterHeaders))
		}
	})

	t.Run("creates store with custom config", func(t *testing.T) {
		tmpDir := t.TempDir()
		config := StorageConfig{
			DataDir:       tmpDir,
			MaxBytes:      1024 * 1024,
			WarnPercent:   90,
			FilterHeaders: []string{"X-Custom-Key"},
			RedactValue:   "[HIDDEN]",
		}

		store, err := NewFileStore(config)
		if err != nil {
			t.Fatalf("NewFileStore failed: %v", err)
		}

		if store.config.MaxBytes != 1024*1024 {
			t.Errorf("expected MaxBytes=%d, got %d", 1024*1024, store.config.MaxBytes)
		}
		if store.config.WarnPercent != 90 {
			t.Errorf("expected WarnPercent=90, got %d", store.config.WarnPercent)
		}
		if store.config.RedactValue != "[HIDDEN]" {
			t.Errorf("expected RedactValue=[HIDDEN], got %s", store.config.RedactValue)
		}
	})

	t.Run("creates data directory if missing", func(t *testing.T) {
		tmpDir := t.TempDir()
		dataDir := filepath.Join(tmpDir, "nested", "recordings")

		config := StorageConfig{
			DataDir: dataDir,
		}

		_, err := NewFileStore(config)
		if err != nil {
			t.Fatalf("NewFileStore failed: %v", err)
		}

		if _, err := os.Stat(dataDir); os.IsNotExist(err) {
			t.Error("expected data directory to be created")
		}
	})
}

// TestStartRecording tests starting new recording sessions.
func TestStartRecording(t *testing.T) {
	t.Run("starts WebSocket recording session", func(t *testing.T) {
		store := newTestStore(t)

		metadata := RecordingMetadata{
			Path:   "/ws/chat",
			Host:   "example.com",
			Source: RecordingSourceProxy,
		}

		session, err := store.StartRecording(ProtocolWebSocket, metadata)
		if err != nil {
			t.Fatalf("StartRecording failed: %v", err)
		}

		if session == nil {
			t.Fatal("expected session, got nil")
			return
		}
		if session.recording.Protocol != ProtocolWebSocket {
			t.Errorf("expected protocol=%s, got %s", ProtocolWebSocket, session.recording.Protocol)
		}
		if session.recording.Metadata.Path != "/ws/chat" {
			t.Errorf("expected path=/ws/chat, got %s", session.recording.Metadata.Path)
		}
		if session.recording.Status != RecordingStatusRecording {
			t.Errorf("expected status=%s, got %s", RecordingStatusRecording, session.recording.Status)
		}
		if session.closed {
			t.Error("expected session not closed")
		}
	})

	t.Run("starts SSE recording session", func(t *testing.T) {
		store := newTestStore(t)

		metadata := RecordingMetadata{
			Path:             "/api/events",
			Host:             "api.example.com",
			Source:           RecordingSourceProxy,
			DetectedTemplate: "openai-chat",
		}

		session, err := store.StartRecording(ProtocolSSE, metadata)
		if err != nil {
			t.Fatalf("StartRecording failed: %v", err)
		}

		if session.recording.Protocol != ProtocolSSE {
			t.Errorf("expected protocol=%s, got %s", ProtocolSSE, session.recording.Protocol)
		}
		if session.recording.Metadata.DetectedTemplate != "openai-chat" {
			t.Errorf("expected template=openai-chat, got %s", session.recording.Metadata.DetectedTemplate)
		}
	})

	t.Run("filters sensitive headers", func(t *testing.T) {
		store := newTestStore(t)

		metadata := RecordingMetadata{
			Path: "/ws",
			Headers: map[string]string{
				"Authorization": "Bearer secret-token",
				"Content-Type":  "application/json",
				"X-API-Key":     "api-key-123",
			},
		}

		session, err := store.StartRecording(ProtocolWebSocket, metadata)
		if err != nil {
			t.Fatalf("StartRecording failed: %v", err)
		}

		if session.recording.Metadata.Headers["Authorization"] != "[REDACTED]" {
			t.Errorf("expected Authorization to be redacted, got %s", session.recording.Metadata.Headers["Authorization"])
		}
		if session.recording.Metadata.Headers["X-API-Key"] != "[REDACTED]" {
			t.Errorf("expected X-API-Key to be redacted, got %s", session.recording.Metadata.Headers["X-API-Key"])
		}
		if session.recording.Metadata.Headers["Content-Type"] != "application/json" {
			t.Errorf("expected Content-Type preserved, got %s", session.recording.Metadata.Headers["Content-Type"])
		}
	})

	t.Run("fails when storage is full", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Create store with very small limit
		config := StorageConfig{
			DataDir:  tmpDir,
			MaxBytes: 100, // Very small limit
		}
		store, err := NewFileStore(config)
		if err != nil {
			t.Fatalf("NewFileStore failed: %v", err)
		}

		// Create a file that exceeds the limit
		dummyFile := filepath.Join(tmpDir, "rec_dummy.json")
		if err := os.WriteFile(dummyFile, make([]byte, 200), 0600); err != nil {
			t.Fatalf("failed to create dummy file: %v", err)
		}

		metadata := RecordingMetadata{Path: "/ws"}
		_, err = store.StartRecording(ProtocolWebSocket, metadata)
		if err != ErrStorageFull {
			t.Errorf("expected ErrStorageFull, got %v", err)
		}
	})
}

// TestAppendWebSocketFrame tests appending frames to WebSocket recordings.
func TestAppendWebSocketFrame(t *testing.T) {
	t.Run("appends text frame", func(t *testing.T) {
		store := newTestStore(t)
		session, _ := store.StartRecording(ProtocolWebSocket, RecordingMetadata{Path: "/ws"})

		err := store.AppendWebSocketFrame(session.recording.ID, DirectionClientToServer, MessageTypeText, []byte("hello"))
		if err != nil {
			t.Fatalf("AppendWebSocketFrame failed: %v", err)
		}

		session.mu.Lock()
		frameCount := len(session.recording.WebSocket.Frames)
		session.mu.Unlock()

		if frameCount != 1 {
			t.Errorf("expected 1 frame, got %d", frameCount)
		}
	})

	t.Run("appends binary frame with base64 encoding", func(t *testing.T) {
		store := newTestStore(t)
		session, _ := store.StartRecording(ProtocolWebSocket, RecordingMetadata{Path: "/ws"})

		binaryData := []byte{0x00, 0x01, 0x02, 0xFF}
		err := store.AppendWebSocketFrame(session.recording.ID, DirectionServerToClient, MessageTypeBinary, binaryData)
		if err != nil {
			t.Fatalf("AppendWebSocketFrame failed: %v", err)
		}

		session.mu.Lock()
		frame := session.recording.WebSocket.Frames[0]
		session.mu.Unlock()

		if frame.DataEncoding != DataEncodingBase64 {
			t.Errorf("expected encoding=%s, got %s", DataEncodingBase64, frame.DataEncoding)
		}
		if frame.MessageType != MessageTypeBinary {
			t.Errorf("expected type=%s, got %s", MessageTypeBinary, frame.MessageType)
		}
	})

	t.Run("increments sequence numbers", func(t *testing.T) {
		store := newTestStore(t)
		session, _ := store.StartRecording(ProtocolWebSocket, RecordingMetadata{Path: "/ws"})

		for i := 0; i < 5; i++ {
			if err := store.AppendWebSocketFrame(session.recording.ID, DirectionClientToServer, MessageTypeText, []byte("msg")); err != nil {
				t.Fatalf("AppendWebSocketFrame failed: %v", err)
			}
		}

		session.mu.Lock()
		frames := session.recording.WebSocket.Frames
		session.mu.Unlock()

		for i, frame := range frames {
			expectedSeq := int64(i + 1)
			if frame.Sequence != expectedSeq {
				t.Errorf("frame %d: expected seq=%d, got %d", i, expectedSeq, frame.Sequence)
			}
		}
	})

	t.Run("fails for non-existent session", func(t *testing.T) {
		store := newTestStore(t)

		err := store.AppendWebSocketFrame("nonexistent-id", DirectionClientToServer, MessageTypeText, []byte("hello"))
		if err != ErrNotFound {
			t.Errorf("expected ErrNotFound, got %v", err)
		}
	})

	t.Run("fails for closed session", func(t *testing.T) {
		store := newTestStore(t)
		session, _ := store.StartRecording(ProtocolWebSocket, RecordingMetadata{Path: "/ws"})
		sessionID := session.recording.ID

		// Complete the session
		_, err := store.CompleteRecording(sessionID)
		if err != nil {
			t.Fatalf("CompleteRecording failed: %v", err)
		}

		// Try to append to closed session
		err = store.AppendWebSocketFrame(sessionID, DirectionClientToServer, MessageTypeText, []byte("hello"))
		if err != ErrNotFound {
			t.Errorf("expected ErrNotFound for completed session, got %v", err)
		}
	})
}

// TestAppendSSEEvent tests appending events to SSE recordings.
func TestAppendSSEEvent(t *testing.T) {
	t.Run("appends SSE event", func(t *testing.T) {
		store := newTestStore(t)
		session, _ := store.StartRecording(ProtocolSSE, RecordingMetadata{Path: "/events"})

		err := store.AppendSSEEvent(session.recording.ID, "message", `{"text":"hello"}`, "evt-1", nil)
		if err != nil {
			t.Fatalf("AppendSSEEvent failed: %v", err)
		}

		session.mu.Lock()
		eventCount := len(session.recording.SSE.Events)
		session.mu.Unlock()

		if eventCount != 1 {
			t.Errorf("expected 1 event, got %d", eventCount)
		}
	})

	t.Run("appends event with retry", func(t *testing.T) {
		store := newTestStore(t)
		session, _ := store.StartRecording(ProtocolSSE, RecordingMetadata{Path: "/events"})

		retry := 3000
		err := store.AppendSSEEvent(session.recording.ID, "ping", "", "", &retry)
		if err != nil {
			t.Fatalf("AppendSSEEvent failed: %v", err)
		}

		session.mu.Lock()
		event := session.recording.SSE.Events[0]
		session.mu.Unlock()

		if event.Retry == nil || *event.Retry != 3000 {
			t.Errorf("expected retry=3000, got %v", event.Retry)
		}
	})

	t.Run("calculates relative timestamps", func(t *testing.T) {
		store := newTestStore(t)
		session, _ := store.StartRecording(ProtocolSSE, RecordingMetadata{Path: "/events"})

		// Add first event
		if err := store.AppendSSEEvent(session.recording.ID, "event", "first", "1", nil); err != nil {
			t.Fatalf("AppendSSEEvent failed: %v", err)
		}

		time.Sleep(10 * time.Millisecond)

		// Add second event
		if err := store.AppendSSEEvent(session.recording.ID, "event", "second", "2", nil); err != nil {
			t.Fatalf("AppendSSEEvent failed: %v", err)
		}

		session.mu.Lock()
		events := session.recording.SSE.Events
		session.mu.Unlock()

		// First event should have RelativeMs = 0
		if events[0].RelativeMs != 0 {
			t.Errorf("first event expected RelativeMs=0, got %d", events[0].RelativeMs)
		}
		// Second event should have RelativeMs > 0
		if events[1].RelativeMs <= 0 {
			t.Errorf("second event expected RelativeMs>0, got %d", events[1].RelativeMs)
		}
	})

	t.Run("fails for non-existent session", func(t *testing.T) {
		store := newTestStore(t)

		err := store.AppendSSEEvent("nonexistent-id", "message", "data", "1", nil)
		if err != ErrNotFound {
			t.Errorf("expected ErrNotFound, got %v", err)
		}
	})
}

// TestCompleteRecording tests completing and persisting recordings.
func TestCompleteRecording(t *testing.T) {
	t.Run("completes and persists recording", func(t *testing.T) {
		store := newTestStore(t)
		session, _ := store.StartRecording(ProtocolWebSocket, RecordingMetadata{Path: "/ws"})
		sessionID := session.recording.ID

		// Add some frames
		store.AppendWebSocketFrame(sessionID, DirectionClientToServer, MessageTypeText, []byte("hello"))
		store.AppendWebSocketFrame(sessionID, DirectionServerToClient, MessageTypeText, []byte("world"))

		recording, err := store.CompleteRecording(sessionID)
		if err != nil {
			t.Fatalf("CompleteRecording failed: %v", err)
		}

		if recording.Status != RecordingStatusComplete {
			t.Errorf("expected status=%s, got %s", RecordingStatusComplete, recording.Status)
		}
		if recording.EndTime == nil {
			t.Error("expected EndTime to be set")
		}
		// Duration may be 0 if test runs fast enough (sub-millisecond)
		if recording.Duration < 0 {
			t.Error("expected Duration >= 0")
		}

		// Verify file was written
		filename := store.recordingFilename(sessionID)
		if _, err := os.Stat(filename); os.IsNotExist(err) {
			t.Error("expected recording file to exist")
		}
	})

	t.Run("can retrieve completed recording", func(t *testing.T) {
		store := newTestStore(t)
		session, _ := store.StartRecording(ProtocolWebSocket, RecordingMetadata{Path: "/ws/test"})
		sessionID := session.recording.ID

		store.AppendWebSocketFrame(sessionID, DirectionClientToServer, MessageTypeText, []byte("test message"))
		store.CompleteRecording(sessionID)

		// Retrieve from disk
		recording, err := store.Get(sessionID)
		if err != nil {
			t.Fatalf("Get failed: %v", err)
		}

		if recording.Metadata.Path != "/ws/test" {
			t.Errorf("expected path=/ws/test, got %s", recording.Metadata.Path)
		}
		if len(recording.WebSocket.Frames) != 1 {
			t.Errorf("expected 1 frame, got %d", len(recording.WebSocket.Frames))
		}
	})

	t.Run("fails for non-existent session", func(t *testing.T) {
		store := newTestStore(t)

		_, err := store.CompleteRecording("nonexistent-id")
		if err != ErrNotFound {
			t.Errorf("expected ErrNotFound, got %v", err)
		}
	})

	t.Run("fails for already completed session", func(t *testing.T) {
		store := newTestStore(t)
		session, _ := store.StartRecording(ProtocolWebSocket, RecordingMetadata{Path: "/ws"})
		sessionID := session.recording.ID

		// Complete once
		_, err := store.CompleteRecording(sessionID)
		if err != nil {
			t.Fatalf("first CompleteRecording failed: %v", err)
		}

		// Try to complete again
		_, err = store.CompleteRecording(sessionID)
		if err != ErrNotFound {
			t.Errorf("expected ErrNotFound for already completed session, got %v", err)
		}
	})
}

// TestCancelRecording tests cancelling recordings without saving.
func TestCancelRecording(t *testing.T) {
	t.Run("cancels without saving", func(t *testing.T) {
		store := newTestStore(t)
		session, _ := store.StartRecording(ProtocolWebSocket, RecordingMetadata{Path: "/ws"})
		sessionID := session.recording.ID

		store.AppendWebSocketFrame(sessionID, DirectionClientToServer, MessageTypeText, []byte("hello"))

		err := store.CancelRecording(sessionID)
		if err != nil {
			t.Fatalf("CancelRecording failed: %v", err)
		}

		// Verify no file was created
		filename := store.recordingFilename(sessionID)
		if _, err := os.Stat(filename); !os.IsNotExist(err) {
			t.Error("expected no file after cancel")
		}
	})

	t.Run("marks session as closed", func(t *testing.T) {
		store := newTestStore(t)
		session, _ := store.StartRecording(ProtocolWebSocket, RecordingMetadata{Path: "/ws"})
		sessionID := session.recording.ID

		store.CancelRecording(sessionID)

		// Try to append after cancel - should fail since session is removed
		err := store.AppendWebSocketFrame(sessionID, DirectionClientToServer, MessageTypeText, []byte("hello"))
		if err != ErrNotFound {
			t.Errorf("expected ErrNotFound after cancel, got %v", err)
		}
	})

	t.Run("fails for non-existent session", func(t *testing.T) {
		store := newTestStore(t)

		err := store.CancelRecording("nonexistent-id")
		if err != ErrNotFound {
			t.Errorf("expected ErrNotFound, got %v", err)
		}
	})
}

// TestGet tests retrieving recordings by ID.
func TestGet(t *testing.T) {
	t.Run("retrieves recording by ID", func(t *testing.T) {
		store := newTestStore(t)

		// Create and complete a recording
		session, _ := store.StartRecording(ProtocolSSE, RecordingMetadata{
			Path: "/api/stream",
			Host: "api.example.com",
		})
		store.AppendSSEEvent(session.recording.ID, "message", "data", "1", nil)
		store.CompleteRecording(session.recording.ID)

		// Retrieve
		recording, err := store.Get(session.recording.ID)
		if err != nil {
			t.Fatalf("Get failed: %v", err)
		}

		if recording.Protocol != ProtocolSSE {
			t.Errorf("expected protocol=%s, got %s", ProtocolSSE, recording.Protocol)
		}
		if recording.Metadata.Host != "api.example.com" {
			t.Errorf("expected host=api.example.com, got %s", recording.Metadata.Host)
		}
	})

	t.Run("returns ErrNotFound for missing recording", func(t *testing.T) {
		store := newTestStore(t)

		_, err := store.Get("nonexistent-id")
		if err != ErrNotFound {
			t.Errorf("expected ErrNotFound, got %v", err)
		}
	})

	t.Run("returns ErrCorrupted for invalid recording", func(t *testing.T) {
		store := newTestStore(t)

		// Create a corrupted recording file
		corruptedID := id.ULID()
		filename := store.recordingFilename(corruptedID)
		corruptedData := `{"id":"invalid-ulid","protocol":"websocket","status":"complete"}`
		if err := os.WriteFile(filename, []byte(corruptedData), 0600); err != nil {
			t.Fatalf("failed to write corrupted file: %v", err)
		}

		recording, err := store.Get(corruptedID)
		if err != ErrCorrupted {
			t.Errorf("expected ErrCorrupted, got %v", err)
		}
		if recording == nil {
			t.Error("expected corrupted recording to be returned")
		}
		if recording != nil && recording.Status != RecordingStatusCorrupted {
			t.Errorf("expected status=%s, got %s", RecordingStatusCorrupted, recording.Status)
		}
	})
}

// TestList tests listing and filtering recordings.
func TestList(t *testing.T) {
	t.Run("lists all recordings", func(t *testing.T) {
		store := newTestStore(t)

		// Create multiple recordings
		createAndCompleteRecording(t, store, ProtocolWebSocket, "/ws/1")
		createAndCompleteRecording(t, store, ProtocolSSE, "/sse/1")
		createAndCompleteRecording(t, store, ProtocolWebSocket, "/ws/2")

		summaries, total, err := store.List(StreamRecordingFilter{})
		if err != nil {
			t.Fatalf("List failed: %v", err)
		}

		if total != 3 {
			t.Errorf("expected total=3, got %d", total)
		}
		if len(summaries) != 3 {
			t.Errorf("expected 3 summaries, got %d", len(summaries))
		}
	})

	t.Run("filters by protocol", func(t *testing.T) {
		store := newTestStore(t)

		createAndCompleteRecording(t, store, ProtocolWebSocket, "/ws/1")
		createAndCompleteRecording(t, store, ProtocolSSE, "/sse/1")
		createAndCompleteRecording(t, store, ProtocolWebSocket, "/ws/2")

		summaries, total, err := store.List(StreamRecordingFilter{
			Protocol: ProtocolWebSocket,
		})
		if err != nil {
			t.Fatalf("List failed: %v", err)
		}

		if total != 2 {
			t.Errorf("expected total=2 for WebSocket, got %d", total)
		}
		for _, s := range summaries {
			if s.Protocol != ProtocolWebSocket {
				t.Errorf("expected protocol=websocket, got %s", s.Protocol)
			}
		}
	})

	t.Run("filters by path prefix", func(t *testing.T) {
		store := newTestStore(t)

		createAndCompleteRecording(t, store, ProtocolWebSocket, "/api/ws/chat")
		createAndCompleteRecording(t, store, ProtocolWebSocket, "/api/ws/notify")
		createAndCompleteRecording(t, store, ProtocolWebSocket, "/other/ws")

		summaries, total, err := store.List(StreamRecordingFilter{
			Path: "/api/ws",
		})
		if err != nil {
			t.Fatalf("List failed: %v", err)
		}

		if total != 2 {
			t.Errorf("expected total=2, got %d", total)
		}
		if len(summaries) != 2 {
			t.Errorf("expected 2 summaries, got %d", len(summaries))
		}
	})

	t.Run("paginates results", func(t *testing.T) {
		store := newTestStore(t)

		for i := 0; i < 10; i++ {
			createAndCompleteRecording(t, store, ProtocolWebSocket, "/ws")
		}

		// First page
		summaries, total, err := store.List(StreamRecordingFilter{
			Limit:  3,
			Offset: 0,
		})
		if err != nil {
			t.Fatalf("List failed: %v", err)
		}

		if total != 10 {
			t.Errorf("expected total=10, got %d", total)
		}
		if len(summaries) != 3 {
			t.Errorf("expected 3 summaries, got %d", len(summaries))
		}

		// Second page
		summaries, _, _ = store.List(StreamRecordingFilter{
			Limit:  3,
			Offset: 3,
		})
		if len(summaries) != 3 {
			t.Errorf("expected 3 summaries on second page, got %d", len(summaries))
		}

		// Last page
		summaries, _, _ = store.List(StreamRecordingFilter{
			Limit:  3,
			Offset: 9,
		})
		if len(summaries) != 1 {
			t.Errorf("expected 1 summary on last page, got %d", len(summaries))
		}
	})

	t.Run("sorts by startTime descending by default", func(t *testing.T) {
		store := newTestStore(t)

		createAndCompleteRecording(t, store, ProtocolWebSocket, "/ws/first")
		time.Sleep(10 * time.Millisecond)
		createAndCompleteRecording(t, store, ProtocolWebSocket, "/ws/second")
		time.Sleep(10 * time.Millisecond)
		createAndCompleteRecording(t, store, ProtocolWebSocket, "/ws/third")

		summaries, _, _ := store.List(StreamRecordingFilter{})

		// Default sort is startTime desc, so newest first
		if len(summaries) < 2 {
			t.Skip("not enough recordings to test sorting")
		}
		if summaries[0].StartTime.Before(summaries[1].StartTime) {
			t.Error("expected newest first (descending order)")
		}
	})

	t.Run("sorts by startTime ascending", func(t *testing.T) {
		store := newTestStore(t)

		createAndCompleteRecording(t, store, ProtocolWebSocket, "/ws/first")
		time.Sleep(10 * time.Millisecond)
		createAndCompleteRecording(t, store, ProtocolWebSocket, "/ws/second")

		summaries, _, _ := store.List(StreamRecordingFilter{
			SortBy:    "startTime",
			SortOrder: "asc",
		})

		if len(summaries) < 2 {
			t.Skip("not enough recordings")
		}
		if summaries[0].StartTime.After(summaries[1].StartTime) {
			t.Error("expected oldest first (ascending order)")
		}
	})

	t.Run("returns empty for offset beyond total", func(t *testing.T) {
		store := newTestStore(t)

		createAndCompleteRecording(t, store, ProtocolWebSocket, "/ws")

		summaries, total, err := store.List(StreamRecordingFilter{
			Offset: 100,
		})
		if err != nil {
			t.Fatalf("List failed: %v", err)
		}

		if total != 1 {
			t.Errorf("expected total=1, got %d", total)
		}
		if len(summaries) != 0 {
			t.Errorf("expected 0 summaries for large offset, got %d", len(summaries))
		}
	})
}

// TestDelete tests soft-deleting recordings.
func TestDelete(t *testing.T) {
	t.Run("soft deletes recording", func(t *testing.T) {
		store := newTestStore(t)
		id := createAndCompleteRecording(t, store, ProtocolWebSocket, "/ws")

		err := store.Delete(id)
		if err != nil {
			t.Fatalf("Delete failed: %v", err)
		}

		// Recording should still exist but be marked as deleted
		recording, err := store.Get(id)
		if err != nil {
			t.Fatalf("Get after delete failed: %v", err)
		}

		if !recording.Deleted {
			t.Error("expected Deleted=true")
		}
		if recording.DeletedAt == "" {
			t.Error("expected DeletedAt to be set")
		}
	})

	t.Run("file still exists after soft delete", func(t *testing.T) {
		store := newTestStore(t)
		id := createAndCompleteRecording(t, store, ProtocolWebSocket, "/ws")

		store.Delete(id)

		filename := store.recordingFilename(id)
		if _, err := os.Stat(filename); os.IsNotExist(err) {
			t.Error("expected file to still exist after soft delete")
		}
	})

	t.Run("fails for non-existent recording", func(t *testing.T) {
		store := newTestStore(t)

		err := store.Delete("nonexistent-id")
		if err != ErrNotFound {
			t.Errorf("expected ErrNotFound, got %v", err)
		}
	})
}

// TestPurge tests permanently removing recordings.
func TestPurge(t *testing.T) {
	t.Run("permanently removes recording", func(t *testing.T) {
		store := newTestStore(t)
		id := createAndCompleteRecording(t, store, ProtocolWebSocket, "/ws")

		err := store.Purge(id)
		if err != nil {
			t.Fatalf("Purge failed: %v", err)
		}

		// File should no longer exist
		filename := store.recordingFilename(id)
		if _, err := os.Stat(filename); !os.IsNotExist(err) {
			t.Error("expected file to be removed after purge")
		}

		// Get should fail
		_, err = store.Get(id)
		if err != ErrNotFound {
			t.Errorf("expected ErrNotFound after purge, got %v", err)
		}
	})

	t.Run("fails for non-existent recording", func(t *testing.T) {
		store := newTestStore(t)

		err := store.Purge("nonexistent-id")
		if err != ErrNotFound {
			t.Errorf("expected ErrNotFound, got %v", err)
		}
	})
}

// TestGetStats tests storage statistics.
func TestGetStats(t *testing.T) {
	t.Run("returns empty stats for new store", func(t *testing.T) {
		store := newTestStore(t)

		stats, err := store.GetStats()
		if err != nil {
			t.Fatalf("GetStats failed: %v", err)
		}

		if stats.RecordingCount != 0 {
			t.Errorf("expected RecordingCount=0, got %d", stats.RecordingCount)
		}
		if stats.UsedBytes != 0 {
			t.Errorf("expected UsedBytes=0, got %d", stats.UsedBytes)
		}
	})

	t.Run("counts recordings by protocol", func(t *testing.T) {
		store := newTestStore(t)

		createAndCompleteRecording(t, store, ProtocolWebSocket, "/ws/1")
		createAndCompleteRecording(t, store, ProtocolWebSocket, "/ws/2")
		createAndCompleteRecording(t, store, ProtocolSSE, "/sse/1")

		stats, err := store.GetStats()
		if err != nil {
			t.Fatalf("GetStats failed: %v", err)
		}

		if stats.RecordingCount != 3 {
			t.Errorf("expected RecordingCount=3, got %d", stats.RecordingCount)
		}
		if stats.WebSocketCount != 2 {
			t.Errorf("expected WebSocketCount=2, got %d", stats.WebSocketCount)
		}
		if stats.SSECount != 1 {
			t.Errorf("expected SSECount=1, got %d", stats.SSECount)
		}
	})

	t.Run("tracks used bytes", func(t *testing.T) {
		store := newTestStore(t)

		createAndCompleteRecording(t, store, ProtocolWebSocket, "/ws")

		stats, err := store.GetStats()
		if err != nil {
			t.Fatalf("GetStats failed: %v", err)
		}

		if stats.UsedBytes <= 0 {
			t.Error("expected UsedBytes > 0")
		}
	})

	t.Run("calculates used percent", func(t *testing.T) {
		tmpDir := t.TempDir()
		config := StorageConfig{
			DataDir:  tmpDir,
			MaxBytes: 10000,
		}
		store, _ := NewFileStore(config)

		createAndCompleteRecording(t, store, ProtocolWebSocket, "/ws")

		stats, _ := store.GetStats()

		if stats.UsedPercent <= 0 {
			t.Error("expected UsedPercent > 0")
		}
		if stats.UsedPercent > 100 {
			t.Errorf("expected UsedPercent <= 100, got %f", stats.UsedPercent)
		}
	})

	t.Run("tracks oldest and newest recordings", func(t *testing.T) {
		store := newTestStore(t)

		id1 := createAndCompleteRecording(t, store, ProtocolWebSocket, "/ws/first")
		time.Sleep(10 * time.Millisecond)
		id2 := createAndCompleteRecording(t, store, ProtocolWebSocket, "/ws/second")

		stats, _ := store.GetStats()

		if stats.OldestRecording != id1 {
			t.Errorf("expected oldest=%s, got %s", id1, stats.OldestRecording)
		}
		if stats.NewestRecording != id2 {
			t.Errorf("expected newest=%s, got %s", id2, stats.NewestRecording)
		}
	})
}

// TestVacuum tests cleaning up deleted recordings.
func TestVacuum(t *testing.T) {
	t.Run("removes soft-deleted recordings", func(t *testing.T) {
		store := newTestStore(t)

		id1 := createAndCompleteRecording(t, store, ProtocolWebSocket, "/ws/1")
		id2 := createAndCompleteRecording(t, store, ProtocolWebSocket, "/ws/2")
		createAndCompleteRecording(t, store, ProtocolWebSocket, "/ws/3")

		// Soft delete two recordings
		store.Delete(id1)
		store.Delete(id2)

		removed, freedBytes, err := store.Vacuum()
		if err != nil {
			t.Fatalf("Vacuum failed: %v", err)
		}

		if removed != 2 {
			t.Errorf("expected removed=2, got %d", removed)
		}
		if freedBytes <= 0 {
			t.Error("expected freedBytes > 0")
		}

		// Deleted recordings should be gone
		_, err = store.Get(id1)
		if err != ErrNotFound {
			t.Errorf("expected ErrNotFound for vacuumed recording, got %v", err)
		}
	})

	t.Run("does nothing when no deleted recordings", func(t *testing.T) {
		store := newTestStore(t)

		createAndCompleteRecording(t, store, ProtocolWebSocket, "/ws")

		removed, freedBytes, err := store.Vacuum()
		if err != nil {
			t.Fatalf("Vacuum failed: %v", err)
		}

		if removed != 0 {
			t.Errorf("expected removed=0, got %d", removed)
		}
		if freedBytes != 0 {
			t.Errorf("expected freedBytes=0, got %d", freedBytes)
		}
	})
}

// TestStorageLimitEnforcement tests that storage limits are enforced.
func TestStorageLimitEnforcement(t *testing.T) {
	t.Run("prevents new recordings when storage full", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Create store with very small limit
		config := StorageConfig{
			DataDir:  tmpDir,
			MaxBytes: 500,
		}
		store, _ := NewFileStore(config)

		// First recording should succeed
		session, err := store.StartRecording(ProtocolWebSocket, RecordingMetadata{Path: "/ws"})
		if err != nil {
			t.Fatalf("first recording failed: %v", err)
		}

		// Add enough data to exceed the limit when saved
		for i := 0; i < 20; i++ {
			store.AppendWebSocketFrame(session.recording.ID, DirectionClientToServer, MessageTypeText, []byte("large message data here"))
		}
		store.CompleteRecording(session.recording.ID)

		// Now the store should be over the limit
		_, err = store.StartRecording(ProtocolWebSocket, RecordingMetadata{Path: "/ws2"})
		if err != ErrStorageFull {
			t.Errorf("expected ErrStorageFull, got %v", err)
		}
	})

	t.Run("CanRecord returns false when storage full", func(t *testing.T) {
		tmpDir := t.TempDir()

		config := StorageConfig{
			DataDir:  tmpDir,
			MaxBytes: 100,
		}
		store, _ := NewFileStore(config)

		// Create a file that exceeds the limit
		dummyFile := filepath.Join(tmpDir, "rec_dummy.json")
		os.WriteFile(dummyFile, make([]byte, 200), 0600)

		canRecord, msg := store.CanRecord()
		if canRecord {
			t.Error("expected CanRecord=false when storage full")
		}
		if msg != "storage limit exceeded" {
			t.Errorf("expected 'storage limit exceeded' message, got %s", msg)
		}
	})

	t.Run("CanRecord returns warning when near limit", func(t *testing.T) {
		tmpDir := t.TempDir()

		config := StorageConfig{
			DataDir:     tmpDir,
			MaxBytes:    1000,
			WarnPercent: 80,
		}
		store, _ := NewFileStore(config)

		// Create a file that's 85% of the limit
		dummyFile := filepath.Join(tmpDir, "rec_dummy.json")
		os.WriteFile(dummyFile, make([]byte, 850), 0600)

		canRecord, msg := store.CanRecord()
		if !canRecord {
			t.Error("expected CanRecord=true when just at warning level")
		}
		if msg == "" {
			t.Error("expected warning message when near limit")
		}
	})
}

// TestConcurrentAccess tests thread safety of the FileStore.
func TestConcurrentAccess(t *testing.T) {
	t.Run("handles concurrent frame appends", func(t *testing.T) {
		store := newTestStore(t)
		session, _ := store.StartRecording(ProtocolWebSocket, RecordingMetadata{Path: "/ws"})
		sessionID := session.recording.ID

		var wg sync.WaitGroup
		numGoroutines := 10
		framesPerGoroutine := 10

		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				for j := 0; j < framesPerGoroutine; j++ {
					err := store.AppendWebSocketFrame(sessionID, DirectionClientToServer, MessageTypeText, []byte("msg"))
					if err != nil {
						t.Errorf("goroutine %d: AppendWebSocketFrame failed: %v", id, err)
					}
				}
			}(i)
		}

		wg.Wait()

		session.mu.Lock()
		frameCount := len(session.recording.WebSocket.Frames)
		session.mu.Unlock()

		expectedFrames := numGoroutines * framesPerGoroutine
		if frameCount != expectedFrames {
			t.Errorf("expected %d frames, got %d", expectedFrames, frameCount)
		}
	})

	t.Run("handles concurrent session creation", func(t *testing.T) {
		store := newTestStore(t)

		var wg sync.WaitGroup
		numSessions := 20
		sessions := make(chan *StreamRecordingSession, numSessions)

		for i := 0; i < numSessions; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				session, err := store.StartRecording(ProtocolWebSocket, RecordingMetadata{Path: "/ws"})
				if err != nil {
					t.Errorf("session %d: StartRecording failed: %v", id, err)
					return
				}
				sessions <- session
			}(i)
		}

		wg.Wait()
		close(sessions)

		// Collect all session IDs
		ids := make(map[string]bool)
		for session := range sessions {
			if ids[session.recording.ID] {
				t.Error("duplicate session ID detected")
			}
			ids[session.recording.ID] = true
		}

		if len(ids) != numSessions {
			t.Errorf("expected %d unique sessions, got %d", numSessions, len(ids))
		}
	})
}

// TestAppendWebSocketCloseFrame tests appending close frames.
func TestAppendWebSocketCloseFrame(t *testing.T) {
	t.Run("appends close frame", func(t *testing.T) {
		store := newTestStore(t)
		session, _ := store.StartRecording(ProtocolWebSocket, RecordingMetadata{Path: "/ws"})

		err := store.AppendWebSocketCloseFrame(session.recording.ID, DirectionServerToClient, 1000, "normal closure")
		if err != nil {
			t.Fatalf("AppendWebSocketCloseFrame failed: %v", err)
		}

		session.mu.Lock()
		ws := session.recording.WebSocket
		session.mu.Unlock()

		if len(ws.Frames) != 1 {
			t.Fatalf("expected 1 frame, got %d", len(ws.Frames))
		}

		frame := ws.Frames[0]
		if frame.MessageType != MessageTypeClose {
			t.Errorf("expected type=%s, got %s", MessageTypeClose, frame.MessageType)
		}
		if frame.CloseCode == nil || *frame.CloseCode != 1000 {
			t.Error("expected CloseCode=1000")
		}
		if frame.CloseReason == nil || *frame.CloseReason != "normal closure" {
			t.Error("expected CloseReason='normal closure'")
		}

		// Check that close info is also set on the WebSocket data
		if ws.CloseCode == nil || *ws.CloseCode != 1000 {
			t.Error("expected WebSocket.CloseCode=1000")
		}
	})
}

// TestMarkIncomplete tests marking recordings as incomplete.
func TestMarkIncomplete(t *testing.T) {
	t.Run("marks recording as incomplete and saves", func(t *testing.T) {
		store := newTestStore(t)
		session, _ := store.StartRecording(ProtocolWebSocket, RecordingMetadata{Path: "/ws"})
		sessionID := session.recording.ID

		store.AppendWebSocketFrame(sessionID, DirectionClientToServer, MessageTypeText, []byte("partial"))

		recording, err := store.MarkIncomplete(sessionID)
		if err != nil {
			t.Fatalf("MarkIncomplete failed: %v", err)
		}

		if recording.Status != RecordingStatusIncomplete {
			t.Errorf("expected status=%s, got %s", RecordingStatusIncomplete, recording.Status)
		}

		// Verify it was saved
		saved, _ := store.Get(sessionID)
		if saved.Status != RecordingStatusIncomplete {
			t.Error("expected saved recording to be incomplete")
		}
	})
}

// TestUpdate tests updating recording metadata.
func TestUpdate(t *testing.T) {
	t.Run("updates name and description", func(t *testing.T) {
		store := newTestStore(t)
		id := createAndCompleteRecording(t, store, ProtocolWebSocket, "/ws")

		name := "My Recording"
		desc := "A test recording"
		err := store.Update(id, &name, &desc, nil)
		if err != nil {
			t.Fatalf("Update failed: %v", err)
		}

		recording, _ := store.Get(id)
		if recording.Name != "My Recording" {
			t.Errorf("expected name='My Recording', got %s", recording.Name)
		}
		if recording.Description != "A test recording" {
			t.Errorf("expected description='A test recording', got %s", recording.Description)
		}
	})

	t.Run("updates tags", func(t *testing.T) {
		store := newTestStore(t)
		id := createAndCompleteRecording(t, store, ProtocolWebSocket, "/ws")

		tags := []string{"chat", "production"}
		err := store.Update(id, nil, nil, tags)
		if err != nil {
			t.Fatalf("Update failed: %v", err)
		}

		recording, _ := store.Get(id)
		if len(recording.Tags) != 2 {
			t.Errorf("expected 2 tags, got %d", len(recording.Tags))
		}
	})
}

// TestExport tests exporting recordings.
func TestExport(t *testing.T) {
	t.Run("exports as JSON", func(t *testing.T) {
		store := newTestStore(t)
		id := createAndCompleteRecording(t, store, ProtocolWebSocket, "/ws/export")

		data, err := store.Export(id, ExportFormatJSON)
		if err != nil {
			t.Fatalf("Export failed: %v", err)
		}

		var recording StreamRecording
		if err := json.Unmarshal(data, &recording); err != nil {
			t.Fatalf("failed to parse exported JSON: %v", err)
		}

		if recording.Metadata.Path != "/ws/export" {
			t.Errorf("expected path=/ws/export, got %s", recording.Metadata.Path)
		}
	})
}

// TestGetActiveSessions tests retrieving active session info.
func TestGetActiveSessions(t *testing.T) {
	t.Run("returns active sessions", func(t *testing.T) {
		store := newTestStore(t)

		store.StartRecording(ProtocolWebSocket, RecordingMetadata{Path: "/ws/1"})
		store.StartRecording(ProtocolSSE, RecordingMetadata{Path: "/sse/1"})

		sessions := store.GetActiveSessions()

		if len(sessions) != 2 {
			t.Errorf("expected 2 active sessions, got %d", len(sessions))
		}
	})

	t.Run("returns empty when no active sessions", func(t *testing.T) {
		store := newTestStore(t)

		sessions := store.GetActiveSessions()

		if len(sessions) != 0 {
			t.Errorf("expected 0 active sessions, got %d", len(sessions))
		}
	})

	t.Run("excludes completed sessions", func(t *testing.T) {
		store := newTestStore(t)

		session, _ := store.StartRecording(ProtocolWebSocket, RecordingMetadata{Path: "/ws"})
		store.CompleteRecording(session.recording.ID)

		sessions := store.GetActiveSessions()

		if len(sessions) != 0 {
			t.Errorf("expected 0 active sessions after complete, got %d", len(sessions))
		}
	})
}

// TestConfig tests the Config method.
func TestConfig(t *testing.T) {
	t.Run("returns store configuration", func(t *testing.T) {
		tmpDir := t.TempDir()
		config := StorageConfig{
			DataDir:     tmpDir,
			MaxBytes:    12345,
			WarnPercent: 75,
			RedactValue: "[HIDDEN]",
		}
		store, _ := NewFileStore(config)

		got := store.Config()

		if got.DataDir != tmpDir {
			t.Errorf("expected DataDir=%s, got %s", tmpDir, got.DataDir)
		}
		if got.MaxBytes != 12345 {
			t.Errorf("expected MaxBytes=12345, got %d", got.MaxBytes)
		}
		if got.WarnPercent != 75 {
			t.Errorf("expected WarnPercent=75, got %d", got.WarnPercent)
		}
	})
}

// Helper functions

// newTestStore creates a new FileStore with a temporary directory.
func newTestStore(t *testing.T) *FileStore {
	t.Helper()
	config := StorageConfig{
		DataDir: t.TempDir(),
	}
	store, err := NewFileStore(config)
	if err != nil {
		t.Fatalf("failed to create test store: %v", err)
	}
	return store
}

// createAndCompleteRecording creates a recording, adds a frame, and completes it.
// Returns the recording ID.
func createAndCompleteRecording(t *testing.T, store *FileStore, protocol Protocol, path string) string {
	t.Helper()

	session, err := store.StartRecording(protocol, RecordingMetadata{Path: path})
	if err != nil {
		t.Fatalf("StartRecording failed: %v", err)
	}

	sessionID := session.recording.ID

	switch protocol {
	case ProtocolWebSocket:
		if err := store.AppendWebSocketFrame(sessionID, DirectionClientToServer, MessageTypeText, []byte("test")); err != nil {
			t.Fatalf("AppendWebSocketFrame failed: %v", err)
		}
	case ProtocolSSE:
		if err := store.AppendSSEEvent(sessionID, "message", "test", "1", nil); err != nil {
			t.Fatalf("AppendSSEEvent failed: %v", err)
		}
	}

	if _, err := store.CompleteRecording(sessionID); err != nil {
		t.Fatalf("CompleteRecording failed: %v", err)
	}

	return sessionID
}
