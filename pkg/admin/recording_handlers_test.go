package admin

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/getmockd/mockd/pkg/recording"
	"gopkg.in/yaml.v3"
)

// createTestStoreWithRecordings creates a recording store with test recordings.
// Returns the store and a map of logical session names to actual session IDs.
func createTestStoreWithRecordings(recordings ...*recording.Recording) (*recording.Store, map[string]string) {
	store := recording.NewStore()
	sessionMap := make(map[string]string)

	for _, rec := range recordings {
		logicalSessionID := rec.SessionID
		actualSessionID, exists := sessionMap[logicalSessionID]

		if !exists {
			session := store.CreateSession(logicalSessionID, nil)
			actualSessionID = session.ID
			sessionMap[logicalSessionID] = actualSessionID
		}

		// Update the recording to use the actual session ID
		rec.SessionID = actualSessionID

		// Get session and add recording
		session := store.GetSession(actualSessionID)
		session.AddRecording(rec)
	}

	return store, sessionMap
}

// createTestRecording creates a test recording with the given parameters.
func createTestRecording(id, sessionID, method, path string) *recording.Recording {
	return &recording.Recording{
		ID:        id,
		SessionID: sessionID,
		Timestamp: time.Now(),
		Request: recording.RecordedRequest{
			Method: method,
			Path:   path,
			URL:    "http://example.com" + path,
			Host:   "example.com",
			Scheme: "http",
			Headers: http.Header{
				"Content-Type": []string{"application/json"},
			},
			Body: []byte(`{"test": "data"}`),
		},
		Response: recording.RecordedResponse{
			StatusCode: 200,
			Status:     "OK",
			Headers: http.Header{
				"Content-Type": []string{"application/json"},
			},
			Body: []byte(`{"result": "success"}`),
		},
		Duration: 50 * time.Millisecond,
	}
}

// ============================================================================
// Export Recordings Tests
// ============================================================================

func TestHandleExportRecordings_JSONFormat(t *testing.T) {
	store, _ := createTestStoreWithRecordings(
		createTestRecording("rec-1", "session-1", "GET", "/api/users"),
		createTestRecording("rec-2", "session-1", "POST", "/api/users"),
	)

	pm := &ProxyManager{
		store: store,
	}

	t.Run("exports all recordings as JSON by default", func(t *testing.T) {
		// Empty JSON object as body - handler requires valid JSON
		body := bytes.NewBufferString(`{}`)
		req := httptest.NewRequest(http.MethodPost, "/recordings/export", body)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		pm.handleExportRecordings(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
		}

		contentType := rec.Header().Get("Content-Type")
		if contentType != "application/json" {
			t.Errorf("expected Content-Type=application/json, got %v", contentType)
		}

		var recordings []*recording.Recording
		if err := json.Unmarshal(rec.Body.Bytes(), &recordings); err != nil {
			t.Fatalf("failed to parse JSON response: %v", err)
		}

		if len(recordings) != 2 {
			t.Errorf("expected 2 recordings, got %d", len(recordings))
		}
	})

	t.Run("exports recordings as JSON with explicit format", func(t *testing.T) {
		body := bytes.NewBufferString(`{"format": "json"}`)
		req := httptest.NewRequest(http.MethodPost, "/recordings/export", body)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		pm.handleExportRecordings(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", rec.Code)
		}

		contentType := rec.Header().Get("Content-Type")
		if contentType != "application/json" {
			t.Errorf("expected Content-Type=application/json, got %v", contentType)
		}
	})
}

func TestHandleExportRecordings_YAMLFormat(t *testing.T) {
	store, _ := createTestStoreWithRecordings(
		createTestRecording("rec-1", "session-1", "GET", "/api/users"),
		createTestRecording("rec-2", "session-1", "POST", "/api/users"),
	)

	pm := &ProxyManager{
		store: store,
	}

	t.Run("exports recordings as YAML when format=yaml", func(t *testing.T) {
		body := bytes.NewBufferString(`{"format": "yaml"}`)
		req := httptest.NewRequest(http.MethodPost, "/recordings/export", body)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		pm.handleExportRecordings(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
		}

		contentType := rec.Header().Get("Content-Type")
		if contentType != "application/x-yaml" {
			t.Errorf("expected Content-Type=application/x-yaml, got %v", contentType)
		}

		// Verify it's valid YAML
		var recordings []map[string]interface{}
		if err := yaml.Unmarshal(rec.Body.Bytes(), &recordings); err != nil {
			t.Fatalf("failed to parse YAML response: %v", err)
		}

		if len(recordings) != 2 {
			t.Errorf("expected 2 recordings, got %d", len(recordings))
		}

		// Verify YAML structure
		if recordings[0]["id"] == nil {
			t.Error("expected 'id' field in YAML output")
		}
		if recordings[0]["request"] == nil {
			t.Error("expected 'request' field in YAML output")
		}
		if recordings[0]["response"] == nil {
			t.Error("expected 'response' field in YAML output")
		}
	})

	t.Run("exports YAML with case-insensitive format parameter", func(t *testing.T) {
		testCases := []string{"YAML", "Yaml", "YaML", "yaml"}

		for _, format := range testCases {
			t.Run(format, func(t *testing.T) {
				body := bytes.NewBufferString(`{"format": "` + format + `"}`)
				req := httptest.NewRequest(http.MethodPost, "/recordings/export", body)
				req.Header.Set("Content-Type", "application/json")
				rec := httptest.NewRecorder()

				pm.handleExportRecordings(rec, req)

				if rec.Code != http.StatusOK {
					t.Fatalf("expected status 200 for format=%s, got %d", format, rec.Code)
				}

				contentType := rec.Header().Get("Content-Type")
				if contentType != "application/x-yaml" {
					t.Errorf("expected Content-Type=application/x-yaml for format=%s, got %v", format, contentType)
				}
			})
		}
	})

	t.Run("YAML output is human-readable", func(t *testing.T) {
		body := bytes.NewBufferString(`{"format": "yaml"}`)
		req := httptest.NewRequest(http.MethodPost, "/recordings/export", body)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		pm.handleExportRecordings(rec, req)

		yamlStr := rec.Body.String()

		// YAML should contain readable field names
		if !strings.Contains(yamlStr, "id:") {
			t.Error("YAML output should contain 'id:' field")
		}
		if !strings.Contains(yamlStr, "method:") {
			t.Error("YAML output should contain 'method:' field")
		}
		if !strings.Contains(yamlStr, "statusCode:") || !strings.Contains(yamlStr, "statuscode:") {
			// YAML field names might be lowercase
			if !strings.Contains(strings.ToLower(yamlStr), "statuscode") {
				t.Error("YAML output should contain status code field")
			}
		}
	})
}

func TestHandleExportRecordings_Filtering(t *testing.T) {
	store, sessionMap := createTestStoreWithRecordings(
		createTestRecording("rec-1", "session-1", "GET", "/api/users"),
		createTestRecording("rec-2", "session-1", "POST", "/api/users"),
		createTestRecording("rec-3", "session-2", "GET", "/api/posts"),
	)

	pm := &ProxyManager{
		store: store,
	}

	t.Run("filters by session ID", func(t *testing.T) {
		actualSessionID := sessionMap["session-1"]
		body := bytes.NewBufferString(`{"sessionId": "` + actualSessionID + `", "format": "json"}`)
		req := httptest.NewRequest(http.MethodPost, "/recordings/export", body)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		pm.handleExportRecordings(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", rec.Code)
		}

		var recordings []*recording.Recording
		json.Unmarshal(rec.Body.Bytes(), &recordings)

		if len(recordings) != 2 {
			t.Errorf("expected 2 recordings for session-1, got %d", len(recordings))
		}

		for _, r := range recordings {
			if r.SessionID != actualSessionID {
				t.Errorf("expected sessionId=%s, got %v", actualSessionID, r.SessionID)
			}
		}
	})

	t.Run("filters by recording IDs", func(t *testing.T) {
		body := bytes.NewBufferString(`{"recordingIds": ["rec-1", "rec-3"], "format": "json"}`)
		req := httptest.NewRequest(http.MethodPost, "/recordings/export", body)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		pm.handleExportRecordings(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", rec.Code)
		}

		var recordings []*recording.Recording
		json.Unmarshal(rec.Body.Bytes(), &recordings)

		if len(recordings) != 2 {
			t.Errorf("expected 2 recordings, got %d", len(recordings))
		}
	})

	t.Run("filters work with YAML format", func(t *testing.T) {
		actualSessionID := sessionMap["session-2"]
		body := bytes.NewBufferString(`{"sessionId": "` + actualSessionID + `", "format": "yaml"}`)
		req := httptest.NewRequest(http.MethodPost, "/recordings/export", body)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		pm.handleExportRecordings(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", rec.Code)
		}

		contentType := rec.Header().Get("Content-Type")
		if contentType != "application/x-yaml" {
			t.Errorf("expected Content-Type=application/x-yaml, got %v", contentType)
		}

		var recordings []map[string]interface{}
		yaml.Unmarshal(rec.Body.Bytes(), &recordings)

		if len(recordings) != 1 {
			t.Errorf("expected 1 recording for session-2, got %d", len(recordings))
		}
	})
}

func TestHandleExportRecordings_EmptyStore(t *testing.T) {
	store := recording.NewStore()
	pm := &ProxyManager{
		store: store,
	}

	t.Run("returns empty JSON array when no recordings", func(t *testing.T) {
		body := bytes.NewBufferString(`{}`)
		req := httptest.NewRequest(http.MethodPost, "/recordings/export", body)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		pm.handleExportRecordings(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
		}

		var recordings []*recording.Recording
		json.Unmarshal(rec.Body.Bytes(), &recordings)

		if len(recordings) != 0 {
			t.Errorf("expected 0 recordings, got %d", len(recordings))
		}
	})

	t.Run("returns empty YAML array when no recordings", func(t *testing.T) {
		body := bytes.NewBufferString(`{"format": "yaml"}`)
		req := httptest.NewRequest(http.MethodPost, "/recordings/export", body)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		pm.handleExportRecordings(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", rec.Code)
		}

		var recordings []map[string]interface{}
		yaml.Unmarshal(rec.Body.Bytes(), &recordings)

		if len(recordings) != 0 {
			t.Errorf("expected 0 recordings, got %d", len(recordings))
		}
	})
}

func TestHandleExportRecordings_NoStore(t *testing.T) {
	pm := &ProxyManager{
		store: nil,
	}

	t.Run("returns error when no store available", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/recordings/export", nil)
		rec := httptest.NewRecorder()

		pm.handleExportRecordings(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected status 400, got %d", rec.Code)
		}

		var errResp ErrorResponse
		json.Unmarshal(rec.Body.Bytes(), &errResp)

		if errResp.Error != "no_store" {
			t.Errorf("expected error=no_store, got %v", errResp.Error)
		}
	})
}

func TestHandleExportRecordings_InvalidJSON(t *testing.T) {
	store := recording.NewStore()
	pm := &ProxyManager{
		store: store,
	}

	t.Run("returns error for invalid JSON body", func(t *testing.T) {
		body := bytes.NewBufferString(`{invalid json}`)
		req := httptest.NewRequest(http.MethodPost, "/recordings/export", body)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		pm.handleExportRecordings(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected status 400, got %d", rec.Code)
		}

		var errResp ErrorResponse
		json.Unmarshal(rec.Body.Bytes(), &errResp)

		if errResp.Error != "invalid_json" {
			t.Errorf("expected error=invalid_json, got %v", errResp.Error)
		}
	})
}

func TestShouldAddToServer(t *testing.T) {
	tests := []struct {
		name     string
		bodyFlag bool
		query    string
		want     bool
	}{
		{name: "body true wins", bodyFlag: true, query: "false", want: true},
		{name: "query true", bodyFlag: false, query: "true", want: true},
		{name: "query one", bodyFlag: false, query: "1", want: true},
		{name: "query false", bodyFlag: false, query: "false", want: false},
		{name: "invalid query ignored", bodyFlag: false, query: "maybe", want: false},
		{name: "empty", bodyFlag: false, query: "", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := shouldAddToServer(tt.bodyFlag, tt.query)
			if got != tt.want {
				t.Fatalf("shouldAddToServer(%v, %q)=%v want %v", tt.bodyFlag, tt.query, got, tt.want)
			}
		})
	}
}

func TestHandleListRecordings_LimitParsing(t *testing.T) {
	store, _ := createTestStoreWithRecordings(
		createTestRecording("rec-1", "session-1", "GET", "/a"),
		createTestRecording("rec-2", "session-1", "GET", "/b"),
	)
	pm := &ProxyManager{store: store}

	t.Run("invalid limit is ignored", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/recordings?limit=1x", nil)
		rec := httptest.NewRecorder()

		pm.handleListRecordings(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rec.Code)
		}
		var resp RecordingListResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode response: %v", err)
		}
		if resp.Limit != 0 {
			t.Fatalf("expected limit=0 for invalid input, got %d", resp.Limit)
		}
		if len(resp.Recordings) != 2 {
			t.Fatalf("expected all recordings, got %d", len(resp.Recordings))
		}
	})

	t.Run("valid limit is applied", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/recordings?limit=1", nil)
		rec := httptest.NewRecorder()

		pm.handleListRecordings(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rec.Code)
		}
		var resp RecordingListResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode response: %v", err)
		}
		if resp.Limit != 1 {
			t.Fatalf("expected limit=1, got %d", resp.Limit)
		}
		if len(resp.Recordings) != 1 {
			t.Fatalf("expected 1 recording, got %d", len(resp.Recordings))
		}
	})

	t.Run("offset is applied", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/recordings?offset=1", nil)
		rec := httptest.NewRecorder()

		pm.handleListRecordings(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rec.Code)
		}
		var resp RecordingListResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode response: %v", err)
		}
		if resp.Offset != 1 {
			t.Fatalf("expected offset=1, got %d", resp.Offset)
		}
		if len(resp.Recordings) != 1 {
			t.Fatalf("expected 1 recording after offset, got %d", len(resp.Recordings))
		}
	})

	t.Run("offset beyond total returns empty array", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/recordings?offset=99", nil)
		rec := httptest.NewRecorder()

		pm.handleListRecordings(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rec.Code)
		}
		var resp RecordingListResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode response: %v", err)
		}
		if resp.Offset != 99 {
			t.Fatalf("expected offset=99, got %d", resp.Offset)
		}
		if resp.Recordings == nil {
			t.Fatalf("expected empty array, got nil")
		}
		if len(resp.Recordings) != 0 {
			t.Fatalf("expected 0 recordings after large offset, got %d", len(resp.Recordings))
		}
	})

	t.Run("negative limit and offset are ignored", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/recordings?limit=-1&offset=-2", nil)
		rec := httptest.NewRecorder()

		pm.handleListRecordings(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", rec.Code)
		}

		var resp RecordingListResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if resp.Limit != 0 {
			t.Fatalf("expected limit=0 for invalid negative limit, got %d", resp.Limit)
		}
		if resp.Offset != 0 {
			t.Fatalf("expected offset=0 for invalid negative offset, got %d", resp.Offset)
		}
		if len(resp.Recordings) != 2 {
			t.Fatalf("expected all recordings when negative pagination ignored, got %d", len(resp.Recordings))
		}
	})
}

func TestHandleConvertRecording_AddRequestedWithoutEngine(t *testing.T) {
	recStore, _ := createTestStoreWithRecordings(
		createTestRecording("rec-1", "session-1", "GET", "/api/users"),
	)
	pm := &ProxyManager{store: recStore}

	req := httptest.NewRequest(http.MethodPost, "/recordings/rec-1/convert?add=1", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	req.SetPathValue("id", "rec-1")
	rec := httptest.NewRecorder()

	// The MockCreatorFunc from an API with no engine writes to the store only.
	// This is valid — the engine will sync on reconnect. So we expect success.
	api := NewAPI(0, WithDataDir(t.TempDir()))
	pm.handleConvertSingleRecording(rec, req, api.mockCreator())

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200 (store-only write), got %d body=%s", rec.Code, rec.Body.String())
	}

	var resp SingleConvertResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Mock == nil {
		t.Fatal("expected mock in response")
	}

	// Verify mock was persisted in the admin store.
	ctx := context.Background()
	_, err := api.dataStore.Mocks().Get(ctx, resp.Mock.ID)
	if err != nil {
		t.Fatalf("mock should be in admin store after store-only write: %v", err)
	}
}

func TestHandleConvertRecording_ChunkedBodyIsParsed(t *testing.T) {
	recStore, _ := createTestStoreWithRecordings(
		createTestRecording("rec-1", "session-1", "GET", "/api/users"),
	)
	pm := &ProxyManager{store: recStore}

	req := httptest.NewRequest(http.MethodPost, "/recordings/rec-1/convert", strings.NewReader(`{"addToServer":true}`))
	req.Header.Set("Content-Type", "application/json")
	req.ContentLength = -1 // simulate chunked transfer (unknown length)
	req.SetPathValue("id", "rec-1")
	rec := httptest.NewRecorder()

	// With the dual-write design, addToServer without an engine does a
	// store-only write (succeeds). Chunked body parsing is the real test.
	api := NewAPI(0, WithDataDir(t.TempDir()))
	pm.handleConvertSingleRecording(rec, req, api.mockCreator())

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200 (chunked body should be parsed), got %d body=%s", rec.Code, rec.Body.String())
	}
}
