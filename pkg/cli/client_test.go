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

// =============================================================================
// GetChaosStats — Chaos statistics tests
// =============================================================================

// TestGetChaosStats_CallsCorrectEndpoint verifies GetChaosStats hits GET /chaos/stats.
func TestGetChaosStats_CallsCorrectEndpoint(t *testing.T) {
	t.Parallel()

	var calledMethod, calledPath string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calledMethod = r.Method
		calledPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"totalRequests":   100,
			"faultedRequests": 15,
			"latencyInjected": 42,
		})
	}))
	defer ts.Close()

	client := NewAdminClient(ts.URL)
	stats, err := client.GetChaosStats()
	if err != nil {
		t.Fatalf("GetChaosStats() error = %v", err)
	}

	if calledMethod != "GET" {
		t.Errorf("GetChaosStats() used method %q, want GET", calledMethod)
	}
	if calledPath != "/chaos/stats" {
		t.Errorf("GetChaosStats() called %q, want /chaos/stats", calledPath)
	}
	if stats == nil {
		t.Fatal("GetChaosStats() returned nil")
	}
	if stats["totalRequests"] != float64(100) {
		t.Errorf("totalRequests = %v, want 100", stats["totalRequests"])
	}
}

// TestGetChaosStats_ServerError returns error for non-200 status.
func TestGetChaosStats_ServerError(t *testing.T) {
	t.Parallel()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error":   "internal_error",
			"message": "chaos not available",
		})
	}))
	defer ts.Close()

	client := NewAdminClient(ts.URL)
	_, err := client.GetChaosStats()
	if err == nil {
		t.Fatal("GetChaosStats() should return error for 500 response")
	}
}

// =============================================================================
// ResetChaosStats — Chaos stats reset tests
// =============================================================================

// TestResetChaosStats_CallsCorrectEndpoint verifies ResetChaosStats hits POST /chaos/stats/reset.
func TestResetChaosStats_CallsCorrectEndpoint(t *testing.T) {
	t.Parallel()

	var calledMethod, calledPath string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calledMethod = r.Method
		calledPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"reset": true})
	}))
	defer ts.Close()

	client := NewAdminClient(ts.URL)
	err := client.ResetChaosStats()
	if err != nil {
		t.Fatalf("ResetChaosStats() error = %v", err)
	}

	if calledMethod != "POST" {
		t.Errorf("ResetChaosStats() used method %q, want POST", calledMethod)
	}
	if calledPath != "/chaos/stats/reset" {
		t.Errorf("ResetChaosStats() called %q, want /chaos/stats/reset", calledPath)
	}
}

// =============================================================================
// GetMockVerification — Verification status tests
// =============================================================================

// TestGetMockVerification_CallsCorrectEndpoint verifies GetMockVerification hits GET /mocks/{id}/verify.
func TestGetMockVerification_CallsCorrectEndpoint(t *testing.T) {
	t.Parallel()

	var calledMethod, calledPath string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calledMethod = r.Method
		calledPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"mockId":    "mock-1",
			"callCount": 5,
			"verified":  true,
		})
	}))
	defer ts.Close()

	client := NewAdminClient(ts.URL)
	result, err := client.GetMockVerification("mock-1")
	if err != nil {
		t.Fatalf("GetMockVerification() error = %v", err)
	}

	if calledMethod != "GET" {
		t.Errorf("GetMockVerification() used method %q, want GET", calledMethod)
	}
	if calledPath != "/mocks/mock-1/verify" {
		t.Errorf("GetMockVerification() called %q, want /mocks/mock-1/verify", calledPath)
	}
	if result == nil {
		t.Fatal("GetMockVerification() returned nil")
	}
	if result["callCount"] != float64(5) {
		t.Errorf("callCount = %v, want 5", result["callCount"])
	}
}

// TestGetMockVerification_NotFound returns error for 404.
func TestGetMockVerification_NotFound(t *testing.T) {
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
	_, err := client.GetMockVerification("nonexistent")
	if err == nil {
		t.Fatal("GetMockVerification() should return error for 404")
	}

	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("expected *APIError, got %T", err)
	}
	if apiErr.ErrorCode != "not_found" {
		t.Errorf("ErrorCode = %q, want not_found", apiErr.ErrorCode)
	}
}

// TestGetMockVerification_EscapesID verifies URL-unsafe mock IDs are properly escaped.
func TestGetMockVerification_EscapesID(t *testing.T) {
	t.Parallel()

	var calledRawPath string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calledRawPath = r.RequestURI
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"callCount": 0})
	}))
	defer ts.Close()

	client := NewAdminClient(ts.URL)
	_, _ = client.GetMockVerification("mock/with/slashes")

	expected := "/mocks/mock%2Fwith%2Fslashes/verify"
	if calledRawPath != expected {
		t.Errorf("GetMockVerification() sent request URI %q, want %q", calledRawPath, expected)
	}
}

// =============================================================================
// VerifyMock — POST verification tests
// =============================================================================

// TestVerifyMock_CallsCorrectEndpoint verifies VerifyMock hits POST /mocks/{id}/verify.
func TestVerifyMock_CallsCorrectEndpoint(t *testing.T) {
	t.Parallel()

	var calledMethod, calledPath string
	var receivedBody map[string]interface{}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calledMethod = r.Method
		calledPath = r.URL.Path
		json.NewDecoder(r.Body).Decode(&receivedBody)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"verified":    true,
			"actualCount": 3,
		})
	}))
	defer ts.Close()

	client := NewAdminClient(ts.URL)
	result, err := client.VerifyMock("mock-1", map[string]interface{}{
		"expectedCount": 3,
	})
	if err != nil {
		t.Fatalf("VerifyMock() error = %v", err)
	}

	if calledMethod != "POST" {
		t.Errorf("VerifyMock() used method %q, want POST", calledMethod)
	}
	if calledPath != "/mocks/mock-1/verify" {
		t.Errorf("VerifyMock() called %q, want /mocks/mock-1/verify", calledPath)
	}
	if receivedBody["expectedCount"] != float64(3) {
		t.Errorf("VerifyMock() sent expectedCount = %v, want 3", receivedBody["expectedCount"])
	}
	if result == nil {
		t.Fatal("VerifyMock() returned nil")
	}
}

// =============================================================================
// ListMockInvocations — Invocation list tests
// =============================================================================

// TestListMockInvocations_CallsCorrectEndpoint verifies ListMockInvocations hits GET /mocks/{id}/invocations.
func TestListMockInvocations_CallsCorrectEndpoint(t *testing.T) {
	t.Parallel()

	var calledMethod, calledPath string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calledMethod = r.Method
		calledPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"invocations": []interface{}{
				map[string]interface{}{"method": "GET", "path": "/api/users"},
			},
			"count": 1,
		})
	}))
	defer ts.Close()

	client := NewAdminClient(ts.URL)
	result, err := client.ListMockInvocations("mock-1")
	if err != nil {
		t.Fatalf("ListMockInvocations() error = %v", err)
	}

	if calledMethod != "GET" {
		t.Errorf("ListMockInvocations() used method %q, want GET", calledMethod)
	}
	if calledPath != "/mocks/mock-1/invocations" {
		t.Errorf("ListMockInvocations() called %q, want /mocks/mock-1/invocations", calledPath)
	}
	if result == nil {
		t.Fatal("ListMockInvocations() returned nil")
	}
	if result["count"] != float64(1) {
		t.Errorf("count = %v, want 1", result["count"])
	}
}

// TestListMockInvocations_NotFound returns error for 404.
func TestListMockInvocations_NotFound(t *testing.T) {
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
	_, err := client.ListMockInvocations("nonexistent")
	if err == nil {
		t.Fatal("ListMockInvocations() should return error for 404")
	}

	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("expected *APIError, got %T", err)
	}
	if apiErr.ErrorCode != "not_found" {
		t.Errorf("ErrorCode = %q, want not_found", apiErr.ErrorCode)
	}
}

// =============================================================================
// ResetMockVerification — Per-mock reset tests
// =============================================================================

// TestResetMockVerification_CallsCorrectEndpoint verifies ResetMockVerification hits DELETE /mocks/{id}/invocations.
func TestResetMockVerification_CallsCorrectEndpoint(t *testing.T) {
	t.Parallel()

	var calledMethod, calledPath string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calledMethod = r.Method
		calledPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	client := NewAdminClient(ts.URL)
	err := client.ResetMockVerification("mock-1")
	if err != nil {
		t.Fatalf("ResetMockVerification() error = %v", err)
	}

	if calledMethod != "DELETE" {
		t.Errorf("ResetMockVerification() used method %q, want DELETE", calledMethod)
	}
	if calledPath != "/mocks/mock-1/invocations" {
		t.Errorf("ResetMockVerification() called %q, want /mocks/mock-1/invocations", calledPath)
	}
}

// TestResetMockVerification_NotFound returns error for 404.
func TestResetMockVerification_NotFound(t *testing.T) {
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
	err := client.ResetMockVerification("nonexistent")
	if err == nil {
		t.Fatal("ResetMockVerification() should return error for 404")
	}

	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("expected *APIError, got %T", err)
	}
	if apiErr.ErrorCode != "not_found" {
		t.Errorf("ErrorCode = %q, want not_found", apiErr.ErrorCode)
	}
}

// TestResetMockVerification_EscapesID verifies URL-unsafe mock IDs are properly escaped.
func TestResetMockVerification_EscapesID(t *testing.T) {
	t.Parallel()

	var calledRawPath string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calledRawPath = r.RequestURI
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	client := NewAdminClient(ts.URL)
	_ = client.ResetMockVerification("mock/with/slashes")

	expected := "/mocks/mock%2Fwith%2Fslashes/invocations"
	if calledRawPath != expected {
		t.Errorf("ResetMockVerification() sent request URI %q, want %q", calledRawPath, expected)
	}
}

// =============================================================================
// ResetAllVerification — Global reset tests
// =============================================================================

// TestResetAllVerification_CallsCorrectEndpoint verifies ResetAllVerification hits DELETE /verify.
func TestResetAllVerification_CallsCorrectEndpoint(t *testing.T) {
	t.Parallel()

	var calledMethod, calledPath string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calledMethod = r.Method
		calledPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	client := NewAdminClient(ts.URL)
	err := client.ResetAllVerification()
	if err != nil {
		t.Fatalf("ResetAllVerification() error = %v", err)
	}

	if calledMethod != "DELETE" {
		t.Errorf("ResetAllVerification() used method %q, want DELETE", calledMethod)
	}
	if calledPath != "/verify" {
		t.Errorf("ResetAllVerification() called %q, want /verify", calledPath)
	}
}

// TestResetAllVerification_ServerError returns error for non-200 status.
func TestResetAllVerification_ServerError(t *testing.T) {
	t.Parallel()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error":   "internal_error",
			"message": "reset failed",
		})
	}))
	defer ts.Close()

	client := NewAdminClient(ts.URL)
	err := client.ResetAllVerification()
	if err == nil {
		t.Fatal("ResetAllVerification() should return error for 500 response")
	}
}
