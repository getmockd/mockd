package integration

import (
	"net/http"
	"testing"
	"time"

	"github.com/getmockd/mockd/pkg/recording"
)

// TestRecordingsConvertToMocks tests that recordings can be converted to mock definitions.
func TestRecordingsConvertToMocks(t *testing.T) {
	// Create a recording with request/response data
	rec := recording.NewRecording("")

	// Manually set request data
	rec.Request = recording.RecordedRequest{
		Method: "POST",
		Host:   "api.example.com",
		Path:   "/users",
		Headers: http.Header{
			"Content-Type": []string{"application/json"},
		},
		Body: []byte(`{"name": "John"}`),
	}

	// Manually set response data
	rec.Response = recording.RecordedResponse{
		StatusCode: 201,
		Headers: http.Header{
			"Content-Type": []string{"application/json"},
		},
		Body: []byte(`{"id": 1, "name": "John"}`),
	}
	rec.Duration = 50 * time.Millisecond

	// Convert to mock
	opts := recording.DefaultConvertOptions()
	mock := recording.ToMock(rec, opts)

	// Verify mock structure
	if mock == nil {
		t.Fatal("Expected mock to be non-nil")
		return
	}

	if mock.HTTP == nil || mock.HTTP.Matcher == nil {
		t.Fatal("Expected mock.HTTP.Matcher to be non-nil")
	}

	if mock.HTTP.Matcher.Method != "POST" {
		t.Errorf("Expected method POST, got %s", mock.HTTP.Matcher.Method)
	}

	if mock.HTTP.Matcher.Path != "/users" {
		t.Errorf("Expected path /users, got %s", mock.HTTP.Matcher.Path)
	}

	if mock.HTTP.Response == nil {
		t.Fatal("Expected mock.HTTP.Response to be non-nil")
	}

	if mock.HTTP.Response.StatusCode != 201 {
		t.Errorf("Expected status 201, got %d", mock.HTTP.Response.StatusCode)
	}

	if mock.HTTP.Response.Body != `{"id": 1, "name": "John"}` {
		t.Errorf("Unexpected body: %s", mock.HTTP.Response.Body)
	}

	if mock.Enabled == nil || !*mock.Enabled {
		t.Error("Expected mock to be enabled")
	}

	// Test with headers included
	opts.IncludeHeaders = true
	mockWithHeaders := recording.ToMock(rec, opts)

	if mockWithHeaders.HTTP == nil || mockWithHeaders.HTTP.Matcher == nil {
		t.Fatal("Expected mockWithHeaders.HTTP.Matcher to be non-nil")
	}

	if mockWithHeaders.HTTP.Matcher.Headers == nil {
		t.Error("Expected headers to be included")
	}
	if mockWithHeaders.HTTP.Matcher.Headers["Content-Type"] != "application/json" {
		t.Errorf("Expected Content-Type header, got %v", mockWithHeaders.HTTP.Matcher.Headers)
	}
}

// TestRecordingSessionManagement tests session creation and management.
func TestRecordingSessionManagement(t *testing.T) {
	store := recording.NewStore()

	// Initially no active session
	if store.ActiveSession() != nil {
		t.Error("Expected no active session initially")
	}

	// Create a session
	session1 := store.CreateSession("session-1", nil)
	if session1 == nil {
		t.Fatal("Expected session to be created")
		return
	}

	if session1.Name != "session-1" {
		t.Errorf("Expected name 'session-1', got '%s'", session1.Name)
	}

	if !session1.IsActive() {
		t.Error("Expected session to be active")
	}

	// Verify it's the active session
	if store.ActiveSession() != session1 {
		t.Error("Expected session1 to be active session")
	}

	// Add recordings to session
	rec1 := recording.NewRecording("")
	rec1.Request.Method = "GET"
	rec1.Request.Path = "/api/v1"
	session1.AddRecording(rec1)

	rec2 := recording.NewRecording("")
	rec2.Request.Method = "POST"
	rec2.Request.Path = "/api/v2"
	session1.AddRecording(rec2)

	if session1.RecordingCount() != 2 {
		t.Errorf("Expected 2 recordings, got %d", session1.RecordingCount())
	}

	// Create another session (should end previous)
	session2 := store.CreateSession("session-2", nil)
	if session2 == nil {
		t.Fatal("Expected session2 to be created")
	}

	// Session1 should now be ended
	if session1.IsActive() {
		t.Error("Expected session1 to be ended")
	}

	// Session2 should be active
	if !session2.IsActive() {
		t.Error("Expected session2 to be active")
	}

	if store.ActiveSession() != session2 {
		t.Error("Expected session2 to be active session")
	}

	// List sessions
	sessions := store.ListSessions()
	if len(sessions) != 2 {
		t.Errorf("Expected 2 sessions, got %d", len(sessions))
	}

	// Get session by ID
	retrieved := store.GetSession(session1.ID)
	if retrieved == nil {
		t.Fatalf("Failed to get session: not found")
		return
	}
	if retrieved.Name != "session-1" {
		t.Errorf("Expected name 'session-1', got '%s'", retrieved.Name)
	}

	// Delete session
	err := store.DeleteSession(session1.ID)
	if err != nil {
		t.Fatalf("Failed to delete session: %v", err)
	}

	sessions = store.ListSessions()
	if len(sessions) != 1 {
		t.Errorf("Expected 1 session after delete, got %d", len(sessions))
	}
}

// TestRecordingStoreOperations tests store add/get/list/clear operations.
func TestRecordingStoreOperations(t *testing.T) {
	store := recording.NewStore()

	// Create session first (required for adding recordings)
	store.CreateSession("test-session", nil)

	// Add recordings
	rec1 := recording.NewRecording("")
	rec1.Request.Method = "GET"
	rec1.Request.Path = "/api/users"
	err := store.AddRecording(rec1)
	if err != nil {
		t.Fatalf("Failed to add recording: %v", err)
	}

	rec2 := recording.NewRecording("")
	rec2.Request.Method = "POST"
	rec2.Request.Path = "/api/users"
	err = store.AddRecording(rec2)
	if err != nil {
		t.Fatalf("Failed to add recording: %v", err)
	}

	rec3 := recording.NewRecording("")
	rec3.Request.Method = "GET"
	rec3.Request.Path = "/api/items"
	err = store.AddRecording(rec3)
	if err != nil {
		t.Fatalf("Failed to add recording: %v", err)
	}

	// List all recordings
	recordings, total := store.ListRecordings(recording.RecordingFilter{})
	if total != 3 {
		t.Errorf("Expected 3 recordings, got %d", total)
	}
	if len(recordings) != 3 {
		t.Errorf("Expected 3 recordings returned, got %d", len(recordings))
	}

	// Filter by method
	_, total = store.ListRecordings(recording.RecordingFilter{Method: "GET"})
	if total != 2 {
		t.Errorf("Expected 2 GET recordings, got %d", total)
	}

	// Filter by path
	_, total = store.ListRecordings(recording.RecordingFilter{Path: "/api/users"})
	if total != 2 {
		t.Errorf("Expected 2 /api/users recordings, got %d", total)
	}

	// Get recording by ID
	retrievedRec := store.GetRecording(rec1.ID)
	if retrievedRec == nil {
		t.Fatalf("Failed to get recording: not found")
		return
	}
	if retrievedRec.Request.Path != "/api/users" {
		t.Errorf("Expected path /api/users, got %s", retrievedRec.Request.Path)
	}

	// Delete recording
	err = store.DeleteRecording(rec1.ID)
	if err != nil {
		t.Fatalf("Failed to delete recording: %v", err)
	}

	_, total = store.ListRecordings(recording.RecordingFilter{})
	if total != 2 {
		t.Errorf("Expected 2 recordings after delete, got %d", total)
	}

	// Clear all
	cleared := store.Clear()
	if cleared != 2 {
		t.Errorf("Expected 2 cleared, got %d", cleared)
	}

	_, total = store.ListRecordings(recording.RecordingFilter{})
	if total != 0 {
		t.Errorf("Expected 0 recordings after clear, got %d", total)
	}
}

// TestRecordingExportImport tests JSON export/import of recordings.
func TestRecordingExportImport(t *testing.T) {
	store := recording.NewStore()

	// Create session and add recordings
	session := store.CreateSession("export-test", nil)

	rec1 := recording.NewRecording("")
	rec1.Request.Method = "GET"
	rec1.Request.Path = "/api/data"
	rec1.Request.Host = "example.com"
	rec1.Response.StatusCode = 200
	rec1.Response.Body = []byte(`{"data": "value"}`)
	session.AddRecording(rec1)

	rec2 := recording.NewRecording("")
	rec2.Request.Method = "POST"
	rec2.Request.Path = "/api/data"
	rec2.Request.Host = "example.com"
	rec2.Response.StatusCode = 201
	rec2.Response.Body = []byte(`{"id": 1}`)
	session.AddRecording(rec2)

	// Export session
	sessionJSON, err := store.ExportSession(session.ID)
	if err != nil {
		t.Fatalf("Failed to export session: %v", err)
	}

	if len(sessionJSON) == 0 {
		t.Error("Expected non-empty JSON export")
	}

	// Verify JSON contains expected fields
	jsonStr := string(sessionJSON)
	if !containsSubstring(jsonStr, "export-test") {
		t.Error("Expected session name in JSON")
	}
	if !containsSubstring(jsonStr, "/api/data") {
		t.Error("Expected path in JSON")
	}

	// Export recordings
	recordingsJSON, err := store.ExportRecordings(recording.RecordingFilter{})
	if err != nil {
		t.Fatalf("Failed to export recordings: %v", err)
	}

	if len(recordingsJSON) == 0 {
		t.Error("Expected non-empty recordings JSON export")
	}

	// Verify recordings JSON
	recordingsStr := string(recordingsJSON)
	if !containsSubstring(recordingsStr, "GET") {
		t.Error("Expected GET method in recordings JSON")
	}
	if !containsSubstring(recordingsStr, "POST") {
		t.Error("Expected POST method in recordings JSON")
	}
}

// TestRecordingDeduplication tests deduplication during mock conversion.
func TestRecordingDeduplication(t *testing.T) {
	// Create multiple recordings with same method/path
	recordings := []*recording.Recording{
		createTestRecording("GET", "/api/users", 200),
		createTestRecording("GET", "/api/users", 200), // Duplicate
		createTestRecording("POST", "/api/users", 201),
		createTestRecording("GET", "/api/users", 200), // Duplicate
		createTestRecording("GET", "/api/items", 200),
	}

	// Convert without deduplication
	opts := recording.ConvertOptions{Deduplicate: false}
	mocks := recording.ToMocks(recordings, opts)
	if len(mocks) != 5 {
		t.Errorf("Expected 5 mocks without dedup, got %d", len(mocks))
	}

	// Convert with deduplication
	opts.Deduplicate = true
	mocks = recording.ToMocks(recordings, opts)
	if len(mocks) != 3 {
		t.Errorf("Expected 3 mocks with dedup (GET /api/users, POST /api/users, GET /api/items), got %d", len(mocks))
	}
}

// TestConvertSession tests converting an entire session to mocks.
func TestConvertSession(t *testing.T) {
	session := recording.NewSession("test-session", nil)

	session.AddRecording(createTestRecording("GET", "/api/v1", 200))
	session.AddRecording(createTestRecording("POST", "/api/v1", 201))
	session.AddRecording(createTestRecording("DELETE", "/api/v1/1", 204))

	mocks := recording.ConvertSession(session, recording.DefaultConvertOptions())

	if len(mocks) != 3 {
		t.Errorf("Expected 3 mocks, got %d", len(mocks))
	}

	// Verify each mock is valid
	for _, m := range mocks {
		if m.ID == "" {
			t.Error("Expected mock to have an ID")
		}
		if m.HTTP == nil || m.HTTP.Matcher == nil {
			t.Error("Expected mock to have HTTP.Matcher")
		}
		if m.HTTP == nil || m.HTTP.Response == nil {
			t.Error("Expected mock to have HTTP.Response")
		}
	}
}

// Helper functions

func createTestRecording(method, path string, status int) *recording.Recording {
	rec := recording.NewRecording("")
	rec.Request.Method = method
	rec.Request.Path = path
	rec.Response.StatusCode = status
	rec.Response.Body = []byte(`{}`)
	return rec
}

func containsSubstring(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
