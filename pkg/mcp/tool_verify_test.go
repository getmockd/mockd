package mcp

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/getmockd/mockd/pkg/cli"
)

// =============================================================================
// handleVerifyMock Tests
// =============================================================================

func TestHandleVerifyMock_ExactMatch_Pass(t *testing.T) {
	t.Parallel()

	client := &mockAdminClient{
		getMockVerificationFn: func(id string) (map[string]interface{}, error) {
			return map[string]interface{}{"callCount": float64(3)}, nil
		},
	}

	session := newTestSession(client)
	server := newTestServer(client)

	args := map[string]interface{}{
		"id":             "http_abc123",
		"expected_count": float64(3),
	}
	result, err := handleVerifyMock(args, session, server)
	if err != nil {
		t.Fatalf("handleVerifyMock() error = %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success, got error: %s", resultText(t, result))
	}

	var parsed map[string]interface{}
	resultJSON(t, result, &parsed)

	if parsed["verified"] != true {
		t.Errorf("verified = %v, want true", parsed["verified"])
	}
	if parsed["actualCount"] != float64(3) {
		t.Errorf("actualCount = %v, want 3", parsed["actualCount"])
	}
	if parsed["mockId"] != "http_abc123" {
		t.Errorf("mockId = %v, want http_abc123", parsed["mockId"])
	}

	expected, ok := parsed["expected"].(map[string]interface{})
	if !ok {
		t.Fatal("expected field missing or not a map")
	}
	if expected["exactCount"] != float64(3) {
		t.Errorf("expected.exactCount = %v, want 3", expected["exactCount"])
	}
}

func TestHandleVerifyMock_ExactMatch_Fail(t *testing.T) {
	t.Parallel()

	client := &mockAdminClient{
		getMockVerificationFn: func(id string) (map[string]interface{}, error) {
			return map[string]interface{}{"callCount": float64(2)}, nil
		},
	}

	session := newTestSession(client)
	server := newTestServer(client)

	args := map[string]interface{}{
		"id":             "http_abc123",
		"expected_count": float64(3),
	}
	result, err := handleVerifyMock(args, session, server)
	if err != nil {
		t.Fatalf("handleVerifyMock() error = %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success result (verified=false, not tool error), got error: %s", resultText(t, result))
	}

	var parsed map[string]interface{}
	resultJSON(t, result, &parsed)

	if parsed["verified"] != false {
		t.Errorf("verified = %v, want false", parsed["verified"])
	}
	if parsed["actualCount"] != float64(2) {
		t.Errorf("actualCount = %v, want 2", parsed["actualCount"])
	}
}

func TestHandleVerifyMock_AtLeast_Pass(t *testing.T) {
	t.Parallel()

	client := &mockAdminClient{
		getMockVerificationFn: func(id string) (map[string]interface{}, error) {
			return map[string]interface{}{"callCount": float64(5)}, nil
		},
	}

	session := newTestSession(client)
	server := newTestServer(client)

	args := map[string]interface{}{
		"id":       "http_abc123",
		"at_least": float64(3),
	}
	result, err := handleVerifyMock(args, session, server)
	if err != nil {
		t.Fatalf("handleVerifyMock() error = %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success, got error: %s", resultText(t, result))
	}

	var parsed map[string]interface{}
	resultJSON(t, result, &parsed)

	if parsed["verified"] != true {
		t.Errorf("verified = %v, want true", parsed["verified"])
	}
	if parsed["actualCount"] != float64(5) {
		t.Errorf("actualCount = %v, want 5", parsed["actualCount"])
	}

	expected, ok := parsed["expected"].(map[string]interface{})
	if !ok {
		t.Fatal("expected field missing or not a map")
	}
	if expected["atLeast"] != float64(3) {
		t.Errorf("expected.atLeast = %v, want 3", expected["atLeast"])
	}
}

func TestHandleVerifyMock_AtLeast_Fail(t *testing.T) {
	t.Parallel()

	client := &mockAdminClient{
		getMockVerificationFn: func(id string) (map[string]interface{}, error) {
			return map[string]interface{}{"callCount": float64(1)}, nil
		},
	}

	session := newTestSession(client)
	server := newTestServer(client)

	args := map[string]interface{}{
		"id":       "http_abc123",
		"at_least": float64(3),
	}
	result, err := handleVerifyMock(args, session, server)
	if err != nil {
		t.Fatalf("handleVerifyMock() error = %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success result (verified=false), got error: %s", resultText(t, result))
	}

	var parsed map[string]interface{}
	resultJSON(t, result, &parsed)

	if parsed["verified"] != false {
		t.Errorf("verified = %v, want false", parsed["verified"])
	}
	if parsed["actualCount"] != float64(1) {
		t.Errorf("actualCount = %v, want 1", parsed["actualCount"])
	}
}

func TestHandleVerifyMock_AtMost_Pass(t *testing.T) {
	t.Parallel()

	client := &mockAdminClient{
		getMockVerificationFn: func(id string) (map[string]interface{}, error) {
			return map[string]interface{}{"callCount": float64(2)}, nil
		},
	}

	session := newTestSession(client)
	server := newTestServer(client)

	args := map[string]interface{}{
		"id":      "http_abc123",
		"at_most": float64(5),
	}
	result, err := handleVerifyMock(args, session, server)
	if err != nil {
		t.Fatalf("handleVerifyMock() error = %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success, got error: %s", resultText(t, result))
	}

	var parsed map[string]interface{}
	resultJSON(t, result, &parsed)

	if parsed["verified"] != true {
		t.Errorf("verified = %v, want true", parsed["verified"])
	}

	expected, ok := parsed["expected"].(map[string]interface{})
	if !ok {
		t.Fatal("expected field missing or not a map")
	}
	if expected["atMost"] != float64(5) {
		t.Errorf("expected.atMost = %v, want 5", expected["atMost"])
	}
}

func TestHandleVerifyMock_AtMost_Fail(t *testing.T) {
	t.Parallel()

	client := &mockAdminClient{
		getMockVerificationFn: func(id string) (map[string]interface{}, error) {
			return map[string]interface{}{"callCount": float64(10)}, nil
		},
	}

	session := newTestSession(client)
	server := newTestServer(client)

	args := map[string]interface{}{
		"id":      "http_abc123",
		"at_most": float64(5),
	}
	result, err := handleVerifyMock(args, session, server)
	if err != nil {
		t.Fatalf("handleVerifyMock() error = %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success result (verified=false), got error: %s", resultText(t, result))
	}

	var parsed map[string]interface{}
	resultJSON(t, result, &parsed)

	if parsed["verified"] != false {
		t.Errorf("verified = %v, want false", parsed["verified"])
	}
	if parsed["actualCount"] != float64(10) {
		t.Errorf("actualCount = %v, want 10", parsed["actualCount"])
	}
}

func TestHandleVerifyMock_Combined_Pass(t *testing.T) {
	t.Parallel()

	client := &mockAdminClient{
		getMockVerificationFn: func(id string) (map[string]interface{}, error) {
			return map[string]interface{}{"callCount": float64(5)}, nil
		},
	}

	session := newTestSession(client)
	server := newTestServer(client)

	args := map[string]interface{}{
		"id":       "http_abc123",
		"at_least": float64(1),
		"at_most":  float64(10),
	}
	result, err := handleVerifyMock(args, session, server)
	if err != nil {
		t.Fatalf("handleVerifyMock() error = %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success, got error: %s", resultText(t, result))
	}

	var parsed map[string]interface{}
	resultJSON(t, result, &parsed)

	if parsed["verified"] != true {
		t.Errorf("verified = %v, want true", parsed["verified"])
	}
	if parsed["actualCount"] != float64(5) {
		t.Errorf("actualCount = %v, want 5", parsed["actualCount"])
	}

	expected, ok := parsed["expected"].(map[string]interface{})
	if !ok {
		t.Fatal("expected field missing or not a map")
	}
	if expected["atLeast"] != float64(1) {
		t.Errorf("expected.atLeast = %v, want 1", expected["atLeast"])
	}
	if expected["atMost"] != float64(10) {
		t.Errorf("expected.atMost = %v, want 10", expected["atMost"])
	}
}

func TestHandleVerifyMock_Combined_Fail(t *testing.T) {
	t.Parallel()

	client := &mockAdminClient{
		getMockVerificationFn: func(id string) (map[string]interface{}, error) {
			return map[string]interface{}{"callCount": float64(3)}, nil
		},
	}

	session := newTestSession(client)
	server := newTestServer(client)

	args := map[string]interface{}{
		"id":       "http_abc123",
		"at_least": float64(5),
		"at_most":  float64(10),
	}
	result, err := handleVerifyMock(args, session, server)
	if err != nil {
		t.Fatalf("handleVerifyMock() error = %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success result (verified=false), got error: %s", resultText(t, result))
	}

	var parsed map[string]interface{}
	resultJSON(t, result, &parsed)

	if parsed["verified"] != false {
		t.Errorf("verified = %v, want false (at_least=5 should fail with actualCount=3)", parsed["verified"])
	}
}

func TestHandleVerifyMock_NoAssertions(t *testing.T) {
	t.Parallel()

	client := &mockAdminClient{
		getMockVerificationFn: func(id string) (map[string]interface{}, error) {
			return map[string]interface{}{"callCount": float64(7)}, nil
		},
	}

	session := newTestSession(client)
	server := newTestServer(client)

	args := map[string]interface{}{
		"id": "http_abc123",
	}
	result, err := handleVerifyMock(args, session, server)
	if err != nil {
		t.Fatalf("handleVerifyMock() error = %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success, got error: %s", resultText(t, result))
	}

	var parsed map[string]interface{}
	resultJSON(t, result, &parsed)

	if parsed["verified"] != true {
		t.Errorf("verified = %v, want true (no assertions = always verified)", parsed["verified"])
	}
	if parsed["actualCount"] != float64(7) {
		t.Errorf("actualCount = %v, want 7", parsed["actualCount"])
	}

	// No assertions provided, so "expected" field should be absent
	if _, ok := parsed["expected"]; ok {
		t.Error("expected field should not be present when no assertions are provided")
	}
}

func TestHandleVerifyMock_WithInvocations(t *testing.T) {
	t.Parallel()

	invocations := []interface{}{
		map[string]interface{}{"method": "GET", "path": "/api/users", "timestamp": "2026-03-10T12:00:00Z"},
		map[string]interface{}{"method": "GET", "path": "/api/users", "timestamp": "2026-03-10T12:01:00Z"},
	}

	client := &mockAdminClient{
		getMockVerificationFn: func(id string) (map[string]interface{}, error) {
			return map[string]interface{}{
				"callCount":   float64(2),
				"invocations": invocations,
			}, nil
		},
	}

	session := newTestSession(client)
	server := newTestServer(client)

	args := map[string]interface{}{
		"id": "http_abc123",
	}
	result, err := handleVerifyMock(args, session, server)
	if err != nil {
		t.Fatalf("handleVerifyMock() error = %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success, got error: %s", resultText(t, result))
	}

	var parsed map[string]interface{}
	resultJSON(t, result, &parsed)

	inv, ok := parsed["invocations"].([]interface{})
	if !ok {
		t.Fatal("invocations field missing or not a list")
	}
	if len(inv) != 2 {
		t.Errorf("len(invocations) = %d, want 2", len(inv))
	}

	first, ok := inv[0].(map[string]interface{})
	if !ok {
		t.Fatal("invocations[0] is not a map")
	}
	if first["method"] != "GET" {
		t.Errorf("invocations[0].method = %v, want GET", first["method"])
	}
}

func TestHandleVerifyMock_MissingID(t *testing.T) {
	t.Parallel()

	client := &mockAdminClient{}
	session := newTestSession(client)
	server := newTestServer(client)

	args := map[string]interface{}{}
	result, err := handleVerifyMock(args, session, server)
	if err != nil {
		t.Fatalf("handleVerifyMock() error = %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error result for missing id")
	}

	text := resultText(t, result)
	if text != "id is required" {
		t.Errorf("error text = %q, want %q", text, "id is required")
	}
}

func TestHandleVerifyMock_NotFound(t *testing.T) {
	t.Parallel()

	client := &mockAdminClient{
		getMockVerificationFn: func(id string) (map[string]interface{}, error) {
			return nil, &cli.APIError{StatusCode: 404, Message: "not found"}
		},
	}

	session := newTestSession(client)
	server := newTestServer(client)

	args := map[string]interface{}{"id": "nonexistent"}
	result, err := handleVerifyMock(args, session, server)
	if err != nil {
		t.Fatalf("handleVerifyMock() error = %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error result for not-found mock")
	}

	text := resultText(t, result)
	if text != "mock not found: nonexistent" {
		t.Errorf("error text = %q, want %q", text, "mock not found: nonexistent")
	}
}

func TestHandleVerifyMock_ConnectionError(t *testing.T) {
	t.Parallel()

	client := &mockAdminClient{
		getMockVerificationFn: func(id string) (map[string]interface{}, error) {
			return nil, fmt.Errorf("dial tcp: connection refused")
		},
	}

	session := newTestSession(client)
	server := newTestServer(client)

	args := map[string]interface{}{"id": "http_abc123"}
	result, err := handleVerifyMock(args, session, server)
	if err != nil {
		t.Fatalf("handleVerifyMock() error = %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error result for connection error")
	}

	text := resultText(t, result)
	// Should start with "failed to verify mock:" (connection error path)
	if len(text) < 22 || text[:22] != "failed to verify mock:" {
		t.Errorf("error text = %q, want prefix %q", text, "failed to verify mock:")
	}
}

func TestHandleVerifyMock_NoClient(t *testing.T) {
	t.Parallel()

	session := NewSession()
	session.SetState(SessionStateReady)
	server := newTestServer(nil)

	args := map[string]interface{}{"id": "http_abc123"}
	result, err := handleVerifyMock(args, session, server)
	if err != nil {
		t.Fatalf("handleVerifyMock() error = %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error result when admin client is nil")
	}

	text := resultText(t, result)
	if text != "admin client not available" {
		t.Errorf("error text = %q, want %q", text, "admin client not available")
	}
}

// =============================================================================
// handleGetMockInvocations Tests
// =============================================================================

func TestHandleGetMockInvocations_Success(t *testing.T) {
	t.Parallel()

	client := &mockAdminClient{
		listMockInvocationsFn: func(id string) (map[string]interface{}, error) {
			return map[string]interface{}{
				"mockId": id,
				"invocations": []interface{}{
					map[string]interface{}{"method": "GET", "path": "/api/users"},
					map[string]interface{}{"method": "POST", "path": "/api/users"},
				},
			}, nil
		},
	}

	session := newTestSession(client)
	server := newTestServer(client)

	args := map[string]interface{}{"id": "http_abc123"}
	result, err := handleGetMockInvocations(args, session, server)
	if err != nil {
		t.Fatalf("handleGetMockInvocations() error = %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success, got error: %s", resultText(t, result))
	}

	var parsed map[string]interface{}
	resultJSON(t, result, &parsed)

	inv, ok := parsed["invocations"].([]interface{})
	if !ok {
		t.Fatal("invocations field missing or not a list")
	}
	if len(inv) != 2 {
		t.Errorf("len(invocations) = %d, want 2", len(inv))
	}
	if parsed["mockId"] != "http_abc123" {
		t.Errorf("mockId = %v, want http_abc123", parsed["mockId"])
	}
}

func TestHandleGetMockInvocations_WithLimit(t *testing.T) {
	t.Parallel()

	// Build 5 invocations; request limit=2
	invocations := make([]interface{}, 5)
	for i := 0; i < 5; i++ {
		invocations[i] = map[string]interface{}{
			"method": "GET",
			"path":   fmt.Sprintf("/api/item/%d", i),
		}
	}

	client := &mockAdminClient{
		listMockInvocationsFn: func(id string) (map[string]interface{}, error) {
			return map[string]interface{}{
				"mockId":      id,
				"invocations": invocations,
			}, nil
		},
	}

	session := newTestSession(client)
	server := newTestServer(client)

	args := map[string]interface{}{
		"id":    "http_abc123",
		"limit": float64(2),
	}
	result, err := handleGetMockInvocations(args, session, server)
	if err != nil {
		t.Fatalf("handleGetMockInvocations() error = %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success, got error: %s", resultText(t, result))
	}

	var parsed map[string]interface{}
	resultJSON(t, result, &parsed)

	inv, ok := parsed["invocations"].([]interface{})
	if !ok {
		t.Fatal("invocations field missing or not a list")
	}
	if len(inv) != 2 {
		t.Errorf("len(invocations) = %d, want 2 (truncated)", len(inv))
	}
	if parsed["truncated"] != true {
		t.Errorf("truncated = %v, want true", parsed["truncated"])
	}
	if parsed["totalCount"] != float64(5) {
		t.Errorf("totalCount = %v, want 5", parsed["totalCount"])
	}
}

func TestHandleGetMockInvocations_DefaultLimit(t *testing.T) {
	t.Parallel()

	// Build 60 invocations; default limit is 50
	invocations := make([]interface{}, 60)
	for i := 0; i < 60; i++ {
		invocations[i] = map[string]interface{}{
			"method": "GET",
			"path":   fmt.Sprintf("/api/item/%d", i),
		}
	}

	client := &mockAdminClient{
		listMockInvocationsFn: func(id string) (map[string]interface{}, error) {
			return map[string]interface{}{
				"mockId":      id,
				"invocations": invocations,
			}, nil
		},
	}

	session := newTestSession(client)
	server := newTestServer(client)

	// No "limit" arg — should default to 50
	args := map[string]interface{}{"id": "http_abc123"}
	result, err := handleGetMockInvocations(args, session, server)
	if err != nil {
		t.Fatalf("handleGetMockInvocations() error = %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success, got error: %s", resultText(t, result))
	}

	var parsed map[string]interface{}
	resultJSON(t, result, &parsed)

	inv, ok := parsed["invocations"].([]interface{})
	if !ok {
		t.Fatal("invocations field missing or not a list")
	}
	if len(inv) != 50 {
		t.Errorf("len(invocations) = %d, want 50 (default limit)", len(inv))
	}
	if parsed["truncated"] != true {
		t.Errorf("truncated = %v, want true", parsed["truncated"])
	}
	if parsed["totalCount"] != float64(60) {
		t.Errorf("totalCount = %v, want 60", parsed["totalCount"])
	}
}

func TestHandleGetMockInvocations_MissingID(t *testing.T) {
	t.Parallel()

	client := &mockAdminClient{}
	session := newTestSession(client)
	server := newTestServer(client)

	args := map[string]interface{}{}
	result, err := handleGetMockInvocations(args, session, server)
	if err != nil {
		t.Fatalf("handleGetMockInvocations() error = %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error result for missing id")
	}

	text := resultText(t, result)
	if text != "id is required" {
		t.Errorf("error text = %q, want %q", text, "id is required")
	}
}

func TestHandleGetMockInvocations_NotFound(t *testing.T) {
	t.Parallel()

	client := &mockAdminClient{
		listMockInvocationsFn: func(id string) (map[string]interface{}, error) {
			return nil, &cli.APIError{StatusCode: 404, Message: "not found"}
		},
	}

	session := newTestSession(client)
	server := newTestServer(client)

	args := map[string]interface{}{"id": "nonexistent"}
	result, err := handleGetMockInvocations(args, session, server)
	if err != nil {
		t.Fatalf("handleGetMockInvocations() error = %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error result for not-found mock")
	}

	text := resultText(t, result)
	if text != "mock not found: nonexistent" {
		t.Errorf("error text = %q, want %q", text, "mock not found: nonexistent")
	}
}

func TestHandleGetMockInvocations_NoClient(t *testing.T) {
	t.Parallel()

	session := NewSession()
	session.SetState(SessionStateReady)
	server := newTestServer(nil)

	args := map[string]interface{}{"id": "http_abc123"}
	result, err := handleGetMockInvocations(args, session, server)
	if err != nil {
		t.Fatalf("handleGetMockInvocations() error = %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error result when admin client is nil")
	}

	text := resultText(t, result)
	if text != "admin client not available" {
		t.Errorf("error text = %q, want %q", text, "admin client not available")
	}
}

// =============================================================================
// handleResetVerification Tests
// =============================================================================

func TestHandleResetVerification_SingleMock(t *testing.T) {
	t.Parallel()

	resetID := ""
	client := &mockAdminClient{
		resetMockVerificationFn: func(id string) error {
			resetID = id
			return nil
		},
	}

	session := newTestSession(client)
	server := newTestServer(client)

	args := map[string]interface{}{"id": "http_abc123"}
	result, err := handleResetVerification(args, session, server)
	if err != nil {
		t.Fatalf("handleResetVerification() error = %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success, got error: %s", resultText(t, result))
	}

	if resetID != "http_abc123" {
		t.Errorf("reset ID = %s, want http_abc123", resetID)
	}

	var parsed map[string]interface{}
	resultJSON(t, result, &parsed)

	if parsed["reset"] != true {
		t.Errorf("reset = %v, want true", parsed["reset"])
	}
	if parsed["mockId"] != "http_abc123" {
		t.Errorf("mockId = %v, want http_abc123", parsed["mockId"])
	}
}

func TestHandleResetVerification_AllMocks(t *testing.T) {
	t.Parallel()

	allResetCalled := false
	client := &mockAdminClient{
		resetAllVerificationFn: func() error {
			allResetCalled = true
			return nil
		},
	}

	session := newTestSession(client)
	server := newTestServer(client)

	// No "id" arg → reset all
	args := map[string]interface{}{}
	result, err := handleResetVerification(args, session, server)
	if err != nil {
		t.Fatalf("handleResetVerification() error = %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success, got error: %s", resultText(t, result))
	}

	if !allResetCalled {
		t.Error("ResetAllVerification was not called")
	}

	var parsed map[string]interface{}
	resultJSON(t, result, &parsed)

	if parsed["reset"] != true {
		t.Errorf("reset = %v, want true", parsed["reset"])
	}
	if parsed["scope"] != "all" {
		t.Errorf("scope = %v, want all", parsed["scope"])
	}
	if parsed["message"] != "verification data cleared for all mocks" {
		t.Errorf("message = %v, want %q", parsed["message"], "verification data cleared for all mocks")
	}
}

func TestHandleResetVerification_SingleMockError(t *testing.T) {
	t.Parallel()

	client := &mockAdminClient{
		resetMockVerificationFn: func(id string) error {
			return &cli.APIError{StatusCode: 404, Message: "not found"}
		},
	}

	session := newTestSession(client)
	server := newTestServer(client)

	args := map[string]interface{}{"id": "nonexistent"}
	result, err := handleResetVerification(args, session, server)
	if err != nil {
		t.Fatalf("handleResetVerification() error = %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error result for not-found mock")
	}

	text := resultText(t, result)
	if text != "mock not found: nonexistent" {
		t.Errorf("error text = %q, want %q", text, "mock not found: nonexistent")
	}
}

func TestHandleResetVerification_AllMocksError(t *testing.T) {
	t.Parallel()

	client := &mockAdminClient{
		resetAllVerificationFn: func() error {
			return fmt.Errorf("dial tcp: connection refused")
		},
	}

	session := newTestSession(client)
	server := newTestServer(client)

	args := map[string]interface{}{}
	result, err := handleResetVerification(args, session, server)
	if err != nil {
		t.Fatalf("handleResetVerification() error = %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error result for connection error")
	}

	text := resultText(t, result)
	if len(text) < 29 || text[:29] != "failed to reset verification:" {
		t.Errorf("error text = %q, want prefix %q", text, "failed to reset verification:")
	}
}

func TestHandleResetVerification_NoClient(t *testing.T) {
	t.Parallel()

	session := NewSession()
	session.SetState(SessionStateReady)
	server := newTestServer(nil)

	args := map[string]interface{}{"id": "http_abc123"}
	result, err := handleResetVerification(args, session, server)
	if err != nil {
		t.Fatalf("handleResetVerification() error = %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error result when admin client is nil")
	}

	text := resultText(t, result)
	if text != "admin client not available" {
		t.Errorf("error text = %q, want %q", text, "admin client not available")
	}
}

// Ensure unused imports don't cause compilation errors.
var _ = json.Marshal
