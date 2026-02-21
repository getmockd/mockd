package integration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/getmockd/mockd/pkg/admin"
	"github.com/getmockd/mockd/pkg/config"
	"github.com/getmockd/mockd/pkg/engine"
	"github.com/getmockd/mockd/pkg/recording"
)

// setupStreamRecordingAPITest creates a server with stream recording support for testing.
func setupStreamRecordingAPITest(t *testing.T) (*admin.API, *recording.FileStore, int, func()) {
	adminPort := getFreePort()

	// Create temp directory for recordings
	tmpDir, err := os.MkdirTemp("", "stream-api-test-*")
	require.NoError(t, err)

	// Create recording store
	store, err := recording.NewFileStore(recording.StorageConfig{
		DataDir:     tmpDir,
		MaxBytes:    100 * 1024 * 1024,
		WarnPercent: 80,
	})
	require.NoError(t, err)

	managementPort := getFreePort()
	cfg := &config.ServerConfiguration{
		HTTPPort:       getFreePort(),
		AdminPort:      adminPort,
		ManagementPort: managementPort,
		ReadTimeout:    30,
		WriteTimeout:   30,
	}

	srv := engine.NewServer(cfg)
	adminDataDir := t.TempDir() // Use temp dir for admin API test isolation
	adminAPI := admin.NewAPI(adminPort,
		admin.WithLocalEngine(fmt.Sprintf("http://localhost:%d", srv.ManagementPort())),
		admin.WithAPIKeyDisabled(),
		admin.WithDataDir(adminDataDir),
	)

	// Set the recording store on the manager
	adminAPI.StreamRecordingManager().SetStore(store)

	err = srv.Start()
	require.NoError(t, err)

	err = adminAPI.Start()
	require.NoError(t, err)

	time.Sleep(50 * time.Millisecond)

	cleanup := func() {
		adminAPI.Stop()
		srv.Stop()
		time.Sleep(10 * time.Millisecond) // Allow file handles to release
		os.RemoveAll(tmpDir)
	}

	return adminAPI, store, adminPort, cleanup
}

// createStreamTestRecording creates a test recording in the store.
func createStreamTestRecording(t *testing.T, store *recording.FileStore, protocol recording.Protocol) string {
	metadata := recording.RecordingMetadata{
		Path:   "/test/api-recording",
		Method: "GET",
		Host:   "localhost",
	}

	switch protocol {
	case recording.ProtocolWebSocket:
		hook, err := recording.NewFileStoreWebSocketHook(store, metadata)
		require.NoError(t, err)

		startTime := time.Now()
		for i := 0; i < 3; i++ {
			frame := recording.NewWebSocketFrame(
				int64(i+1),
				startTime,
				recording.DirectionServerToClient,
				recording.MessageTypeText,
				[]byte(fmt.Sprintf(`{"i":%d}`, i)),
			)
			err := hook.OnFrame(frame)
			require.NoError(t, err)
		}
		hook.OnComplete()
		return hook.ID()

	case recording.ProtocolSSE:
		hook, err := recording.NewFileStoreSSEHook(store, metadata)
		require.NoError(t, err)

		startTime := time.Now()
		for i := 0; i < 3; i++ {
			event := recording.NewSSEEvent(
				int64(i+1),
				startTime,
				"message",
				fmt.Sprintf(`{"i":%d}`, i),
				fmt.Sprintf("%d", i+1),
				nil,
			)
			err := hook.OnFrame(event)
			require.NoError(t, err)
		}
		hook.OnComplete()
		return hook.ID()
	}

	return ""
}

// TestStreamRecordingAPI_List tests GET /stream-recordings
func TestStreamRecordingAPI_List(t *testing.T) {
	_, store, adminPort, cleanup := setupStreamRecordingAPITest(t)
	defer cleanup()

	// Create test recordings
	wsID := createStreamTestRecording(t, store, recording.ProtocolWebSocket)
	sseID := createStreamTestRecording(t, store, recording.ProtocolSSE)

	// List recordings
	resp, err := http.Get(fmt.Sprintf("http://localhost:%d/stream-recordings", adminPort))
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	body, _ := io.ReadAll(resp.Body)
	var result admin.StreamRecordingListResponse
	err = json.Unmarshal(body, &result)
	require.NoError(t, err)

	assert.GreaterOrEqual(t, result.Total, 2)

	// Verify our recordings are in the list
	foundIDs := make(map[string]bool)
	for _, r := range result.Recordings {
		foundIDs[r.ID] = true
	}
	assert.True(t, foundIDs[wsID], "WebSocket recording should be in list")
	assert.True(t, foundIDs[sseID], "SSE recording should be in list")
}

// TestStreamRecordingAPI_ListWithFilter tests GET /stream-recordings with protocol filter
func TestStreamRecordingAPI_ListWithFilter(t *testing.T) {
	_, store, adminPort, cleanup := setupStreamRecordingAPITest(t)
	defer cleanup()

	// Create test recordings
	createStreamTestRecording(t, store, recording.ProtocolWebSocket)
	createStreamTestRecording(t, store, recording.ProtocolSSE)

	// Filter by WebSocket protocol
	resp, err := http.Get(fmt.Sprintf("http://localhost:%d/stream-recordings?protocol=websocket", adminPort))
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	body, _ := io.ReadAll(resp.Body)
	var result admin.StreamRecordingListResponse
	err = json.Unmarshal(body, &result)
	require.NoError(t, err)

	// Should only have WebSocket recordings
	for _, r := range result.Recordings {
		assert.Equal(t, recording.ProtocolWebSocket, r.Protocol)
	}
}

// TestStreamRecordingAPI_Get tests GET /stream-recordings/{id}
func TestStreamRecordingAPI_Get(t *testing.T) {
	_, store, adminPort, cleanup := setupStreamRecordingAPITest(t)
	defer cleanup()

	recordingID := createStreamTestRecording(t, store, recording.ProtocolWebSocket)

	resp, err := http.Get(fmt.Sprintf("http://localhost:%d/stream-recordings/%s", adminPort, recordingID))
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	body, _ := io.ReadAll(resp.Body)
	var rec recording.StreamRecording
	err = json.Unmarshal(body, &rec)
	require.NoError(t, err)

	assert.Equal(t, recordingID, rec.ID)
	assert.Equal(t, recording.ProtocolWebSocket, rec.Protocol)
	assert.Equal(t, recording.RecordingStatusComplete, rec.Status)
}

// TestStreamRecordingAPI_GetNotFound tests GET /stream-recordings/{id} with invalid ID
func TestStreamRecordingAPI_GetNotFound(t *testing.T) {
	_, _, adminPort, cleanup := setupStreamRecordingAPITest(t)
	defer cleanup()

	resp, err := http.Get(fmt.Sprintf("http://localhost:%d/stream-recordings/nonexistent-id", adminPort))
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

// TestStreamRecordingAPI_Delete tests DELETE /stream-recordings/{id}
func TestStreamRecordingAPI_Delete(t *testing.T) {
	_, store, adminPort, cleanup := setupStreamRecordingAPITest(t)
	defer cleanup()

	recordingID := createStreamTestRecording(t, store, recording.ProtocolWebSocket)

	// Delete the recording
	req, _ := http.NewRequest(http.MethodDelete, fmt.Sprintf("http://localhost:%d/stream-recordings/%s", adminPort, recordingID), nil)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusNoContent, resp.StatusCode)

	// Verify it's soft-deleted (not in normal list)
	listResp, err := http.Get(fmt.Sprintf("http://localhost:%d/stream-recordings", adminPort))
	require.NoError(t, err)
	defer listResp.Body.Close()

	body, _ := io.ReadAll(listResp.Body)
	var result admin.StreamRecordingListResponse
	json.Unmarshal(body, &result)

	for _, r := range result.Recordings {
		assert.NotEqual(t, recordingID, r.ID, "Deleted recording should not appear in list")
	}
}

// TestStreamRecordingAPI_Export tests POST /stream-recordings/{id}/export
func TestStreamRecordingAPI_Export(t *testing.T) {
	_, store, adminPort, cleanup := setupStreamRecordingAPITest(t)
	defer cleanup()

	recordingID := createStreamTestRecording(t, store, recording.ProtocolWebSocket)

	resp, err := http.Post(
		fmt.Sprintf("http://localhost:%d/stream-recordings/%s/export", adminPort, recordingID),
		"application/json",
		nil,
	)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	// Should return JSON
	body, _ := io.ReadAll(resp.Body)
	var exported recording.StreamRecording
	err = json.Unmarshal(body, &exported)
	require.NoError(t, err)

	assert.Equal(t, recordingID, exported.ID)
}

// TestStreamRecordingAPI_Convert tests POST /stream-recordings/{id}/convert
func TestStreamRecordingAPI_Convert(t *testing.T) {
	_, store, adminPort, cleanup := setupStreamRecordingAPITest(t)
	defer cleanup()

	recordingID := createStreamTestRecording(t, store, recording.ProtocolWebSocket)

	// Convert with options
	convertReq := map[string]interface{}{
		"simplifyTiming": true,
		"format":         "json",
	}
	reqBody, _ := json.Marshal(convertReq)

	resp, err := http.Post(
		fmt.Sprintf("http://localhost:%d/stream-recordings/%s/convert", adminPort, recordingID),
		"application/json",
		bytes.NewReader(reqBody),
	)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	body, _ := io.ReadAll(resp.Body)
	var result struct {
		Protocol string          `json:"protocol"`
		Config   json.RawMessage `json:"config"`
	}
	err = json.Unmarshal(body, &result)
	require.NoError(t, err)

	assert.Equal(t, "websocket", result.Protocol)
	assert.NotEmpty(t, result.Config)
}

// TestStreamRecordingAPI_Stats tests GET /stream-recordings/stats
func TestStreamRecordingAPI_Stats(t *testing.T) {
	_, store, adminPort, cleanup := setupStreamRecordingAPITest(t)
	defer cleanup()

	// Create some recordings
	createStreamTestRecording(t, store, recording.ProtocolWebSocket)
	createStreamTestRecording(t, store, recording.ProtocolSSE)

	resp, err := http.Get(fmt.Sprintf("http://localhost:%d/stream-recordings/stats", adminPort))
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	body, _ := io.ReadAll(resp.Body)
	var stats recording.StorageStats
	err = json.Unmarshal(body, &stats)
	require.NoError(t, err)

	assert.GreaterOrEqual(t, stats.RecordingCount, 2)
	assert.GreaterOrEqual(t, stats.WebSocketCount, 1)
	assert.GreaterOrEqual(t, stats.SSECount, 1)
}

// TestStreamRecordingAPI_Vacuum tests POST /stream-recordings/vacuum
func TestStreamRecordingAPI_Vacuum(t *testing.T) {
	_, store, adminPort, cleanup := setupStreamRecordingAPITest(t)
	defer cleanup()

	// Create and delete a recording
	recordingID := createStreamTestRecording(t, store, recording.ProtocolWebSocket)
	store.Delete(recordingID)

	// Vacuum
	resp, err := http.Post(
		fmt.Sprintf("http://localhost:%d/stream-recordings/vacuum", adminPort),
		"application/json",
		nil,
	)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	body, _ := io.ReadAll(resp.Body)
	var result struct {
		Removed    int   `json:"removed"`
		FreedBytes int64 `json:"freedBytes"`
	}
	err = json.Unmarshal(body, &result)
	require.NoError(t, err)

	assert.Equal(t, 1, result.Removed)
}

// TestStreamRecordingAPI_Sessions tests GET /stream-recordings/sessions
func TestStreamRecordingAPI_Sessions(t *testing.T) {
	_, _, adminPort, cleanup := setupStreamRecordingAPITest(t)
	defer cleanup()

	resp, err := http.Get(fmt.Sprintf("http://localhost:%d/stream-recordings/sessions", adminPort))
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	body, _ := io.ReadAll(resp.Body)
	var sessions []interface{}
	err = json.Unmarshal(body, &sessions)
	require.NoError(t, err)

	// No active sessions after completion
	assert.Empty(t, sessions)
}

// TestReplayAPI_StartAndList tests POST /stream-recordings/{id}/replay and GET /replay
func TestReplayAPI_StartAndList(t *testing.T) {
	_, store, adminPort, cleanup := setupStreamRecordingAPITest(t)
	defer cleanup()

	recordingID := createStreamTestRecording(t, store, recording.ProtocolWebSocket)

	// Start replay
	replayReq := map[string]interface{}{
		"mode":        "triggered",
		"timingScale": 1.0,
	}
	reqBody, _ := json.Marshal(replayReq)

	resp, err := http.Post(
		fmt.Sprintf("http://localhost:%d/stream-recordings/%s/replay", adminPort, recordingID),
		"application/json",
		bytes.NewReader(reqBody),
	)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusCreated, resp.StatusCode)

	body, _ := io.ReadAll(resp.Body)
	var startResult admin.StartReplayResponse
	err = json.Unmarshal(body, &startResult)
	require.NoError(t, err)

	assert.NotEmpty(t, startResult.SessionID)

	// List replays
	listResp, err := http.Get(fmt.Sprintf("http://localhost:%d/replay", adminPort))
	require.NoError(t, err)
	defer listResp.Body.Close()

	assert.Equal(t, http.StatusOK, listResp.StatusCode)

	listBody, _ := io.ReadAll(listResp.Body)
	// Response is a plain array, not wrapped in object
	var listResult []interface{}
	err = json.Unmarshal(listBody, &listResult)
	require.NoError(t, err)

	assert.GreaterOrEqual(t, len(listResult), 1)
}

// TestReplayAPI_Stop tests DELETE /replay/{id}
func TestReplayAPI_Stop(t *testing.T) {
	_, store, adminPort, cleanup := setupStreamRecordingAPITest(t)
	defer cleanup()

	recordingID := createStreamTestRecording(t, store, recording.ProtocolWebSocket)

	// Start a replay
	replayReq := map[string]interface{}{"mode": "triggered"}
	reqBody, _ := json.Marshal(replayReq)
	resp, err := http.Post(
		fmt.Sprintf("http://localhost:%d/stream-recordings/%s/replay", adminPort, recordingID),
		"application/json",
		bytes.NewReader(reqBody),
	)
	require.NoError(t, err)
	respBody, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	var startResult admin.StartReplayResponse
	json.Unmarshal(respBody, &startResult)

	// Stop the replay
	req, _ := http.NewRequest(http.MethodDelete, fmt.Sprintf("http://localhost:%d/replay/%s", adminPort, startResult.SessionID), nil)
	stopResp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer stopResp.Body.Close()

	assert.Equal(t, http.StatusNoContent, stopResp.StatusCode)
}
