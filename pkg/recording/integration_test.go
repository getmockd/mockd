package recording_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/getmockd/mockd/pkg/recording"
)

// TestWebSocketRecordingFlow tests the complete WebSocket recording lifecycle.
func TestWebSocketRecordingFlow(t *testing.T) {
	// Create temp directory for storage
	tmpDir, err := os.MkdirTemp("", "ws-recording-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create store
	store, err := recording.NewFileStore(recording.StorageConfig{
		DataDir:     tmpDir,
		MaxBytes:    100 * 1024 * 1024, // 100MB
		WarnPercent: 80,
	})
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	// Create metadata
	metadata := recording.RecordingMetadata{
		Path:     "/api/v1/ws/chat",
		Method:   "GET",
		Host:     "localhost:8080",
		ClientIP: "127.0.0.1",
		Headers: map[string]string{
			"Sec-WebSocket-Protocol": "chat",
			"User-Agent":             "test-client/1.0",
		},
	}

	// Create hook
	hook, err := recording.NewFileStoreWebSocketHook(store, metadata)
	if err != nil {
		t.Fatalf("failed to create hook: %v", err)
	}

	recordingID := hook.ID()
	t.Logf("Created recording with ID: %s", recordingID)

	// Simulate connection
	hook.OnConnect("chat")

	// Simulate message exchange
	startTime := time.Now()
	messages := []struct {
		dir     recording.Direction
		msgType recording.MessageType
		data    string
	}{
		{recording.DirectionClientToServer, recording.MessageTypeText, `{"type":"subscribe","channel":"room-1"}`},
		{recording.DirectionServerToClient, recording.MessageTypeText, `{"type":"subscribed","channel":"room-1"}`},
		{recording.DirectionServerToClient, recording.MessageTypeText, `{"type":"message","channel":"room-1","data":"Hello"}`},
		{recording.DirectionClientToServer, recording.MessageTypeText, `{"type":"message","channel":"room-1","data":"Hi there"}`},
		{recording.DirectionServerToClient, recording.MessageTypeText, `{"type":"message","channel":"room-1","data":"How are you?"}`},
	}

	for i, msg := range messages {
		frame := recording.NewWebSocketFrame(
			int64(i+1),
			startTime,
			msg.dir,
			msg.msgType,
			[]byte(msg.data),
		)
		if err := hook.OnFrame(frame); err != nil {
			t.Fatalf("failed to record frame %d: %v", i, err)
		}
		time.Sleep(10 * time.Millisecond) // Small delay between frames
	}

	// Simulate connection close
	hook.OnClose(1000, "normal closure")

	// Complete the recording
	if err := hook.OnComplete(); err != nil {
		t.Fatalf("failed to complete recording: %v", err)
	}

	// Verify the recording
	rec, err := store.Get(recordingID)
	if err != nil {
		t.Fatalf("failed to get recording: %v", err)
	}

	// Verify metadata
	if rec.Protocol != recording.ProtocolWebSocket {
		t.Errorf("expected protocol WebSocket, got %s", rec.Protocol)
	}
	if rec.Metadata.Path != "/api/v1/ws/chat" {
		t.Errorf("expected path /api/v1/ws/chat, got %s", rec.Metadata.Path)
	}
	if rec.Status != recording.RecordingStatusComplete {
		t.Errorf("expected status complete, got %s", rec.Status)
	}

	// Verify frame count (5 message frames + 1 close frame = 6)
	if rec.Stats.FrameCount != 6 {
		t.Errorf("expected 6 frames (5 messages + 1 close), got %d", rec.Stats.FrameCount)
	}

	// Verify WebSocket-specific data
	if rec.WebSocket == nil {
		t.Fatal("expected WebSocket data to be present")
	}
	if rec.WebSocket.CloseCode == nil || *rec.WebSocket.CloseCode != 1000 {
		t.Errorf("expected close code 1000")
	}

	t.Logf("Recording completed successfully with %d frames", rec.Stats.FrameCount)
}

// TestSSERecordingFlow tests the complete SSE recording lifecycle.
func TestSSERecordingFlow(t *testing.T) {
	// Create temp directory for storage
	tmpDir, err := os.MkdirTemp("", "sse-recording-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create store
	store, err := recording.NewFileStore(recording.StorageConfig{
		DataDir:     tmpDir,
		MaxBytes:    100 * 1024 * 1024,
		WarnPercent: 80,
	})
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	// Create metadata
	metadata := recording.RecordingMetadata{
		Path:     "/api/v1/events",
		Method:   "GET",
		Host:     "localhost:8080",
		ClientIP: "127.0.0.1",
	}

	// Create hook
	hook, err := recording.NewFileStoreSSEHook(store, metadata)
	if err != nil {
		t.Fatalf("failed to create hook: %v", err)
	}

	recordingID := hook.ID()
	t.Logf("Created SSE recording with ID: %s", recordingID)

	// Simulate stream start
	hook.OnStreamStart()

	// Simulate event sequence
	startTime := time.Now()
	events := []struct {
		eventType string
		data      string
		id        string
	}{
		{"message", `{"content":"First message"}`, "1"},
		{"message", `{"content":"Second message"}`, "2"},
		{"heartbeat", "", "3"},
		{"message", `{"content":"Third message"}`, "4"},
		{"done", `{"status":"complete"}`, "5"},
	}

	for i, evt := range events {
		var retry *int
		event := recording.NewSSEEvent(
			int64(i+1),
			startTime,
			evt.eventType,
			evt.data,
			evt.id,
			retry,
		)
		if err := hook.OnFrame(event); err != nil {
			t.Fatalf("failed to record event %d: %v", i, err)
		}
		time.Sleep(10 * time.Millisecond)
	}

	// Simulate stream end
	hook.OnStreamEnd()

	// Complete the recording
	if err := hook.OnComplete(); err != nil {
		t.Fatalf("failed to complete recording: %v", err)
	}

	// Verify the recording
	rec, err := store.Get(recordingID)
	if err != nil {
		t.Fatalf("failed to get recording: %v", err)
	}

	// Verify metadata
	if rec.Protocol != recording.ProtocolSSE {
		t.Errorf("expected protocol SSE, got %s", rec.Protocol)
	}
	if rec.Status != recording.RecordingStatusComplete {
		t.Errorf("expected status complete, got %s", rec.Status)
	}

	// Verify event count
	if rec.Stats.EventCount != 5 {
		t.Errorf("expected 5 events, got %d", rec.Stats.EventCount)
	}

	// Verify SSE-specific data
	if rec.SSE == nil {
		t.Fatal("expected SSE data to be present")
	}

	t.Logf("SSE Recording completed successfully with %d events", rec.Stats.EventCount)
}

// TestRecordingConversion tests converting a recording to mock config.
func TestRecordingConversion(t *testing.T) {
	// Create temp directory for storage
	tmpDir, err := os.MkdirTemp("", "conversion-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create store
	store, err := recording.NewFileStore(recording.StorageConfig{
		DataDir:     tmpDir,
		MaxBytes:    100 * 1024 * 1024,
		WarnPercent: 80,
	})
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	// Create a WebSocket recording
	metadata := recording.RecordingMetadata{
		Path: "/api/ws/test",
	}

	hook, err := recording.NewFileStoreWebSocketHook(store, metadata)
	if err != nil {
		t.Fatalf("failed to create hook: %v", err)
	}

	recordingID := hook.ID()
	startTime := time.Now()

	// Add some frames
	frames := []struct {
		dir  recording.Direction
		data string
	}{
		{recording.DirectionServerToClient, `{"event":"welcome"}`},
		{recording.DirectionClientToServer, `{"action":"ping"}`},
		{recording.DirectionServerToClient, `{"event":"pong"}`},
	}

	for i, f := range frames {
		frame := recording.NewWebSocketFrame(
			int64(i+1),
			startTime,
			f.dir,
			recording.MessageTypeText,
			[]byte(f.data),
		)
		if err := hook.OnFrame(frame); err != nil {
			t.Fatalf("failed to record frame: %v", err)
		}
		time.Sleep(50 * time.Millisecond)
	}

	hook.OnClose(1000, "")
	if err := hook.OnComplete(); err != nil {
		t.Fatalf("failed to complete recording: %v", err)
	}

	// Get the recording
	rec, err := store.Get(recordingID)
	if err != nil {
		t.Fatalf("failed to get recording: %v", err)
	}

	// Convert to mock config
	opts := recording.StreamConvertOptions{
		SimplifyTiming:        true,
		MinDelay:              10,
		MaxDelay:              5000,
		IncludeClientMessages: true,
		DeduplicateMessages:   false,
		Format:                "json",
	}

	result, err := recording.ConvertStreamRecording(rec, opts)
	if err != nil {
		t.Fatalf("failed to convert recording: %v", err)
	}

	// Verify result
	if result.Protocol != recording.ProtocolWebSocket {
		t.Errorf("expected protocol WebSocket, got %s", result.Protocol)
	}
	if len(result.ConfigJSON) == 0 {
		t.Error("expected non-empty config JSON")
	}

	t.Logf("Converted recording to mock config (%d bytes)", len(result.ConfigJSON))
}

// TestRecordingStorageLifecycle tests store operations.
func TestRecordingStorageLifecycle(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "storage-lifecycle-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create store
	store, err := recording.NewFileStore(recording.StorageConfig{
		DataDir:     tmpDir,
		MaxBytes:    100 * 1024 * 1024,
		WarnPercent: 80,
	})
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	// Create multiple recordings
	var recordingIDs []string
	for i := 0; i < 3; i++ {
		hook, err := recording.NewFileStoreWebSocketHook(store, recording.RecordingMetadata{
			Path: "/test/path",
		})
		if err != nil {
			t.Fatalf("failed to create hook %d: %v", i, err)
		}
		recordingIDs = append(recordingIDs, hook.ID())

		// Add a frame
		frame := recording.NewWebSocketFrame(1, time.Now(), recording.DirectionServerToClient,
			recording.MessageTypeText, []byte(`{"test":true}`))
		hook.OnFrame(frame)
		hook.OnComplete()
	}

	// List recordings
	filter := recording.StreamRecordingFilter{
		Limit:  10,
		Offset: 0,
	}
	summaries, total, err := store.List(filter)
	if err != nil {
		t.Fatalf("failed to list recordings: %v", err)
	}

	if total != 3 {
		t.Errorf("expected 3 recordings, got %d", total)
	}
	if len(summaries) != 3 {
		t.Errorf("expected 3 summaries, got %d", len(summaries))
	}

	// Soft delete one
	if err := store.Delete(recordingIDs[0]); err != nil {
		t.Fatalf("failed to delete recording: %v", err)
	}

	// List should now show 2 (excluding deleted)
	summaries, total, err = store.List(filter)
	if err != nil {
		t.Fatalf("failed to list after delete: %v", err)
	}
	if total != 2 {
		t.Errorf("expected 2 recordings after delete, got %d", total)
	}

	// List with include deleted should show 3
	filter.IncludeDeleted = true
	summaries, total, err = store.List(filter)
	if err != nil {
		t.Fatalf("failed to list with deleted: %v", err)
	}
	if total != 3 {
		t.Errorf("expected 3 recordings with deleted, got %d", total)
	}

	// Vacuum should remove the soft-deleted one
	removed, freed, err := store.Vacuum()
	if err != nil {
		t.Fatalf("vacuum failed: %v", err)
	}
	if removed != 1 {
		t.Errorf("expected 1 removed, got %d", removed)
	}
	t.Logf("Vacuum freed %d bytes", freed)

	// Final count should be 2
	filter.IncludeDeleted = true
	summaries, total, err = store.List(filter)
	if err != nil {
		t.Fatalf("failed to list after vacuum: %v", err)
	}
	if total != 2 {
		t.Errorf("expected 2 recordings after vacuum, got %d", total)
	}

	// Get stats
	stats, err := store.GetStats()
	if err != nil {
		t.Fatalf("failed to get stats: %v", err)
	}
	t.Logf("Storage stats: %d recordings, %d bytes used", stats.RecordingCount, stats.UsedBytes)
}

// TestRecordingExportImport tests export and import functionality.
func TestRecordingExportImport(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "export-import-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create store
	store, err := recording.NewFileStore(recording.StorageConfig{
		DataDir:     tmpDir,
		MaxBytes:    100 * 1024 * 1024,
		WarnPercent: 80,
	})
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	// Create a recording
	hook, err := recording.NewFileStoreWebSocketHook(store, recording.RecordingMetadata{
		Path:   "/test/export",
		Method: "GET",
	})
	if err != nil {
		t.Fatalf("failed to create hook: %v", err)
	}

	recordingID := hook.ID()

	// Add frames
	startTime := time.Now()
	for i := 0; i < 5; i++ {
		frame := recording.NewWebSocketFrame(
			int64(i+1),
			startTime,
			recording.DirectionServerToClient,
			recording.MessageTypeText,
			[]byte(`{"index":`+string(rune('0'+i))+`}`),
		)
		hook.OnFrame(frame)
	}
	hook.OnComplete()

	// Export
	exported, err := store.Export(recordingID, recording.ExportFormatJSON)
	if err != nil {
		t.Fatalf("failed to export: %v", err)
	}

	if len(exported) == 0 {
		t.Error("exported data is empty")
	}

	// Save to file (verify file can be written)
	exportPath := filepath.Join(tmpDir, "exported.json")
	if err := os.WriteFile(exportPath, exported, 0644); err != nil {
		t.Fatalf("failed to write export: %v", err)
	}

	// Verify file was written
	stat, err := os.Stat(exportPath)
	if err != nil {
		t.Fatalf("failed to stat export file: %v", err)
	}
	if stat.Size() == 0 {
		t.Error("export file is empty")
	}

	t.Logf("Successfully exported recording to %s (%d bytes)", exportPath, stat.Size())
}

// TestConcurrentRecordings tests concurrent recording sessions.
func TestConcurrentRecordings(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "concurrent-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create store
	store, err := recording.NewFileStore(recording.StorageConfig{
		DataDir:     tmpDir,
		MaxBytes:    100 * 1024 * 1024,
		WarnPercent: 80,
	})
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	// Run concurrent recordings
	const numRecordings = 10
	const framesPerRecording = 100

	done := make(chan string, numRecordings)
	errors := make(chan error, numRecordings)

	for i := 0; i < numRecordings; i++ {
		go func(idx int) {
			hook, err := recording.NewFileStoreWebSocketHook(store, recording.RecordingMetadata{
				Path: "/concurrent/test",
			})
			if err != nil {
				errors <- err
				return
			}

			startTime := time.Now()
			for j := 0; j < framesPerRecording; j++ {
				frame := recording.NewWebSocketFrame(
					int64(j+1),
					startTime,
					recording.DirectionServerToClient,
					recording.MessageTypeText,
					[]byte(`{"idx":`+string(rune('0'+idx))+`,"frame":`+string(rune('0'+j%10))+`}`),
				)
				if err := hook.OnFrame(frame); err != nil {
					errors <- err
					return
				}
			}

			if err := hook.OnComplete(); err != nil {
				errors <- err
				return
			}

			done <- hook.ID()
		}(i)
	}

	// Wait for all to complete
	recordingIDs := make([]string, 0, numRecordings)
	for i := 0; i < numRecordings; i++ {
		select {
		case id := <-done:
			recordingIDs = append(recordingIDs, id)
		case err := <-errors:
			t.Fatalf("concurrent recording failed: %v", err)
		case <-time.After(30 * time.Second):
			t.Fatal("timeout waiting for recordings")
		}
	}

	// Verify all recordings
	for _, id := range recordingIDs {
		rec, err := store.Get(id)
		if err != nil {
			t.Errorf("failed to get recording %s: %v", id, err)
			continue
		}
		if rec.Stats.FrameCount != framesPerRecording {
			t.Errorf("recording %s: expected %d frames, got %d", id, framesPerRecording, rec.Stats.FrameCount)
		}
	}

	// Verify active sessions are empty
	sessions := store.GetActiveSessions()
	if len(sessions) != 0 {
		t.Errorf("expected 0 active sessions, got %d", len(sessions))
	}

	t.Logf("Successfully completed %d concurrent recordings", len(recordingIDs))
}

// TestHookErrorHandling tests hook behavior on errors.
func TestHookErrorHandling(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "error-handling-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create store
	store, err := recording.NewFileStore(recording.StorageConfig{
		DataDir:     tmpDir,
		MaxBytes:    100 * 1024 * 1024,
		WarnPercent: 80,
	})
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	// Create hook
	hook, err := recording.NewFileStoreWebSocketHook(store, recording.RecordingMetadata{
		Path: "/error/test",
	})
	if err != nil {
		t.Fatalf("failed to create hook: %v", err)
	}

	recordingID := hook.ID()

	// Record some frames
	startTime := time.Now()
	frame := recording.NewWebSocketFrame(1, startTime, recording.DirectionServerToClient,
		recording.MessageTypeText, []byte(`{"test":true}`))
	hook.OnFrame(frame)

	// Signal an error
	hook.OnError(err)

	// Try to record more frames (should fail)
	err = hook.OnFrame(frame)
	if err == nil {
		t.Error("expected error after OnError")
	}

	// Recording should be marked incomplete
	rec, err := store.Get(recordingID)
	if err != nil {
		t.Fatalf("failed to get recording: %v", err)
	}

	if rec.Status != recording.RecordingStatusIncomplete {
		t.Errorf("expected status incomplete, got %s", rec.Status)
	}

	t.Logf("Error handling works correctly, recording marked as %s", rec.Status)
}
