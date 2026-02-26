package cli

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// =============================================================================
// GetStats — Bug fix regression tests (T0-6)
// =============================================================================

// TestGetStats_CallsStatusEndpoint verifies GetStats hits /status (not /stats).
// This is a regression test for the MCP drift bug where the client called the
// non-existent /stats endpoint, causing stats to silently return nil.
func TestGetStats_CallsStatusEndpoint(t *testing.T) {
	t.Parallel()

	var calledPath string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calledPath = r.URL.Path
		resp := map[string]interface{}{
			"status":       "running",
			"uptime":       int64(3600),
			"mockCount":    5,
			"requestCount": int64(1234),
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	client := NewAdminClient(ts.URL)
	stats, err := client.GetStats()
	if err != nil {
		t.Fatalf("GetStats() error = %v", err)
	}

	// Verify it called /status, NOT /stats
	if calledPath != "/status" {
		t.Errorf("GetStats() called %q, want /status", calledPath)
	}

	if stats == nil {
		t.Fatal("GetStats() returned nil stats")
	}
	if stats.Uptime != 3600 {
		t.Errorf("Uptime = %d, want 3600", stats.Uptime)
	}
	if stats.MockCount != 5 {
		t.Errorf("MockCount = %d, want 5", stats.MockCount)
	}
	if stats.RequestCount != 1234 {
		t.Errorf("RequestCount = %d, want 1234", stats.RequestCount)
	}
}

// TestGetStats_DecodesRequestCount verifies the JSON field name mapping.
// The /status endpoint returns "requestCount" (not "totalRequests").
func TestGetStats_DecodesRequestCount(t *testing.T) {
	t.Parallel()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Return the exact JSON shape that GET /status produces
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{
			"status": "running",
			"id": "engine-1",
			"httpPort": 4280,
			"adminPort": 4290,
			"uptime": 7200,
			"mockCount": 10,
			"activeMocks": 8,
			"requestCount": 5678,
			"version": "1.0.0"
		}`))
	}))
	defer ts.Close()

	client := NewAdminClient(ts.URL)
	stats, err := client.GetStats()
	if err != nil {
		t.Fatalf("GetStats() error = %v", err)
	}

	if stats.RequestCount != 5678 {
		t.Errorf("RequestCount = %d, want 5678 (should decode from 'requestCount' JSON field)", stats.RequestCount)
	}
	if stats.Uptime != 7200 {
		t.Errorf("Uptime = %d, want 7200", stats.Uptime)
	}
	if stats.MockCount != 10 {
		t.Errorf("MockCount = %d, want 10", stats.MockCount)
	}
}

// TestGetStats_ServerError returns error for non-200 status.
func TestGetStats_ServerError(t *testing.T) {
	t.Parallel()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error":   "engine_unavailable",
			"message": "No engine connected",
		})
	}))
	defer ts.Close()

	client := NewAdminClient(ts.URL)
	_, err := client.GetStats()
	if err == nil {
		t.Fatal("GetStats() should return error for 503 response")
	}
}

// =============================================================================
// ToggleMock — New method tests
// =============================================================================

// TestToggleMock_CallsToggleEndpoint verifies ToggleMock hits POST /mocks/{id}/toggle.
func TestToggleMock_CallsToggleEndpoint(t *testing.T) {
	t.Parallel()

	var calledMethod, calledPath string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calledMethod = r.Method
		calledPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id":      "mock-1",
			"action":  "toggled",
			"message": "Mock enabled",
			"mock": map[string]interface{}{
				"id":      "mock-1",
				"type":    "http",
				"enabled": true,
			},
		})
	}))
	defer ts.Close()

	client := NewAdminClient(ts.URL)
	mock, err := client.ToggleMock("mock-1")
	if err != nil {
		t.Fatalf("ToggleMock() error = %v", err)
	}

	if calledMethod != "POST" {
		t.Errorf("ToggleMock() used method %q, want POST", calledMethod)
	}
	if calledPath != "/mocks/mock-1/toggle" {
		t.Errorf("ToggleMock() called %q, want /mocks/mock-1/toggle", calledPath)
	}
	if mock == nil {
		t.Fatal("ToggleMock() returned nil mock")
	}
	if mock.ID != "mock-1" {
		t.Errorf("mock.ID = %q, want mock-1", mock.ID)
	}
}

// TestToggleMock_NotFound returns error for 404.
func TestToggleMock_NotFound(t *testing.T) {
	t.Parallel()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error":   "not_found",
			"message": "mock not found",
		})
	}))
	defer ts.Close()

	client := NewAdminClient(ts.URL)
	_, err := client.ToggleMock("nonexistent")
	if err == nil {
		t.Fatal("ToggleMock() should return error for 404")
	}

	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("expected *APIError, got %T", err)
	}
	if apiErr.ErrorCode != "not_found" {
		t.Errorf("ErrorCode = %q, want not_found", apiErr.ErrorCode)
	}
}

// TestToggleMock_EscapesID verifies URL-unsafe mock IDs are properly escaped.
func TestToggleMock_EscapesID(t *testing.T) {
	t.Parallel()

	var calledRawPath string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Use RequestURI to see the raw (still-escaped) path
		calledRawPath = r.RequestURI
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"mock": map[string]interface{}{"id": "mock/with/slashes"},
		})
	}))
	defer ts.Close()

	client := NewAdminClient(ts.URL)
	_, _ = client.ToggleMock("mock/with/slashes")

	// The path should have the slashes escaped in the raw request
	expected := "/mocks/mock%2Fwith%2Fslashes/toggle"
	if calledRawPath != expected {
		t.Errorf("ToggleMock() sent request URI %q, want %q", calledRawPath, expected)
	}
}

// =============================================================================
// PatchMock — New method tests
// =============================================================================

// TestPatchMock_CallsPatchEndpoint verifies PatchMock hits PATCH /mocks/{id}.
func TestPatchMock_CallsPatchEndpoint(t *testing.T) {
	t.Parallel()

	var calledMethod, calledPath string
	var receivedBody map[string]interface{}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calledMethod = r.Method
		calledPath = r.URL.Path
		json.NewDecoder(r.Body).Decode(&receivedBody)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id":     "mock-1",
			"action": "updated",
			"mock": map[string]interface{}{
				"id":      "mock-1",
				"type":    "http",
				"enabled": false,
			},
		})
	}))
	defer ts.Close()

	client := NewAdminClient(ts.URL)
	mock, err := client.PatchMock("mock-1", map[string]interface{}{
		"enabled": false,
	})
	if err != nil {
		t.Fatalf("PatchMock() error = %v", err)
	}

	if calledMethod != "PATCH" {
		t.Errorf("PatchMock() used method %q, want PATCH", calledMethod)
	}
	if calledPath != "/mocks/mock-1" {
		t.Errorf("PatchMock() called %q, want /mocks/mock-1", calledPath)
	}
	if receivedBody["enabled"] != false {
		t.Errorf("PatchMock() sent enabled = %v, want false", receivedBody["enabled"])
	}
	if mock == nil {
		t.Fatal("PatchMock() returned nil mock")
	}
}

// TestPatchMock_NotFound returns error for 404.
func TestPatchMock_NotFound(t *testing.T) {
	t.Parallel()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error":   "not_found",
			"message": "mock not found",
		})
	}))
	defer ts.Close()

	client := NewAdminClient(ts.URL)
	_, err := client.PatchMock("nonexistent", map[string]interface{}{"enabled": true})
	if err == nil {
		t.Fatal("PatchMock() should return error for 404")
	}

	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("expected *APIError, got %T", err)
	}
	if apiErr.ErrorCode != "not_found" {
		t.Errorf("ErrorCode = %q, want not_found", apiErr.ErrorCode)
	}
}

// TestPatchMock_PartialUpdate sends only specific fields.
func TestPatchMock_PartialUpdate(t *testing.T) {
	t.Parallel()

	var receivedBody map[string]interface{}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&receivedBody)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"mock": map[string]interface{}{"id": "mock-1", "name": "Updated Name"},
		})
	}))
	defer ts.Close()

	client := NewAdminClient(ts.URL)
	_, err := client.PatchMock("mock-1", map[string]interface{}{
		"name": "Updated Name",
	})
	if err != nil {
		t.Fatalf("PatchMock() error = %v", err)
	}

	// Should only contain the patched field
	if receivedBody["name"] != "Updated Name" {
		t.Errorf("PatchMock() sent name = %v, want 'Updated Name'", receivedBody["name"])
	}
	// Should NOT contain unrelated fields
	if _, ok := receivedBody["type"]; ok {
		t.Error("PatchMock() should not send 'type' field in partial update")
	}
}
