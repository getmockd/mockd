package mcp

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/getmockd/mockd/pkg/cli"
	"github.com/getmockd/mockd/pkg/config"
	"github.com/getmockd/mockd/pkg/mock"
)

// =============================================================================
// handleManageMock — Update Action Tests
// =============================================================================

func TestHandleManageMock_UpdateName(t *testing.T) {
	t.Parallel()

	enabled := true
	existingMock := &config.MockConfiguration{
		ID:      "http_abc123",
		Type:    mock.TypeHTTP,
		Name:    "Old Name",
		Enabled: &enabled,
		HTTP: &mock.HTTPSpec{
			Matcher: &mock.HTTPMatcher{
				Method: "GET",
				Path:   "/api/users",
			},
			Response: &mock.HTTPResponse{
				StatusCode: 200,
				Body:       `{"users":[]}`,
			},
		},
	}

	var updatedName string
	client := &mockAdminClient{
		getMockFn: func(id string) (*config.MockConfiguration, error) {
			if id == "http_abc123" {
				// Return a copy to avoid mutation issues
				cp := *existingMock
				return &cp, nil
			}
			return nil, &cli.APIError{StatusCode: 404, Message: "mock not found: " + id}
		},
		updateMockFn: func(id string, m *config.MockConfiguration) (*config.MockConfiguration, error) {
			updatedName = m.Name
			return m, nil
		},
	}

	session := newTestSession(client)
	server := newTestServer(client)

	args := map[string]interface{}{
		"action": "update",
		"id":     "http_abc123",
		"name":   "New Name",
	}
	result, err := handleManageMock(args, session, server)
	if err != nil {
		t.Fatalf("handleManageMock() error = %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success, got error: %s", resultText(t, result))
	}

	var parsed map[string]interface{}
	resultJSON(t, result, &parsed)

	if parsed["updated"] != true {
		t.Errorf("updated = %v, want true", parsed["updated"])
	}
	if parsed["id"] != "http_abc123" {
		t.Errorf("id = %v, want http_abc123", parsed["id"])
	}
	if updatedName != "New Name" {
		t.Errorf("updated name = %s, want New Name", updatedName)
	}
}

func TestHandleManageMock_UpdateEnabled(t *testing.T) {
	t.Parallel()

	enabled := true
	existingMock := &config.MockConfiguration{
		ID:      "http_abc123",
		Type:    mock.TypeHTTP,
		Name:    "Test Mock",
		Enabled: &enabled,
		HTTP: &mock.HTTPSpec{
			Matcher: &mock.HTTPMatcher{
				Method: "GET",
				Path:   "/api/users",
			},
		},
	}

	var updatedEnabled *bool
	client := &mockAdminClient{
		getMockFn: func(id string) (*config.MockConfiguration, error) {
			if id == "http_abc123" {
				cp := *existingMock
				cp.Enabled = boolPtr(true)
				return &cp, nil
			}
			return nil, &cli.APIError{StatusCode: 404, Message: "mock not found: " + id}
		},
		updateMockFn: func(id string, m *config.MockConfiguration) (*config.MockConfiguration, error) {
			updatedEnabled = m.Enabled
			return m, nil
		},
	}

	session := newTestSession(client)
	server := newTestServer(client)

	args := map[string]interface{}{
		"action":  "update",
		"id":      "http_abc123",
		"enabled": false,
	}
	result, err := handleManageMock(args, session, server)
	if err != nil {
		t.Fatalf("handleManageMock() error = %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success, got error: %s", resultText(t, result))
	}

	if updatedEnabled == nil {
		t.Fatal("expected enabled to be set")
	}
	if *updatedEnabled != false {
		t.Errorf("enabled = %v, want false", *updatedEnabled)
	}
}

func TestHandleManageMock_UpdateHTTPResponse(t *testing.T) {
	t.Parallel()

	enabled := true
	existingMock := &config.MockConfiguration{
		ID:      "http_abc123",
		Type:    mock.TypeHTTP,
		Name:    "Get Users",
		Enabled: &enabled,
		HTTP: &mock.HTTPSpec{
			Matcher: &mock.HTTPMatcher{
				Method: "GET",
				Path:   "/api/users",
			},
			Response: &mock.HTTPResponse{
				StatusCode: 200,
				Body:       `{"users":[]}`,
			},
		},
	}

	var capturedMock *config.MockConfiguration
	client := &mockAdminClient{
		getMockFn: func(id string) (*config.MockConfiguration, error) {
			if id == "http_abc123" {
				// Deep copy via JSON round-trip to avoid mutation
				data, _ := json.Marshal(existingMock)
				var cp config.MockConfiguration
				_ = json.Unmarshal(data, &cp)
				return &cp, nil
			}
			return nil, &cli.APIError{StatusCode: 404, Message: "mock not found: " + id}
		},
		updateMockFn: func(id string, m *config.MockConfiguration) (*config.MockConfiguration, error) {
			capturedMock = m
			return m, nil
		},
	}

	session := newTestSession(client)
	server := newTestServer(client)

	args := map[string]interface{}{
		"action": "update",
		"id":     "http_abc123",
		"http": map[string]interface{}{
			"response": map[string]interface{}{
				"statusCode": float64(201),
				"body":       `{"created":true}`,
			},
		},
	}
	result, err := handleManageMock(args, session, server)
	if err != nil {
		t.Fatalf("handleManageMock() error = %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success, got error: %s", resultText(t, result))
	}

	var parsed map[string]interface{}
	resultJSON(t, result, &parsed)

	if parsed["updated"] != true {
		t.Errorf("updated = %v, want true", parsed["updated"])
	}

	if capturedMock == nil {
		t.Fatal("expected mock to be passed to UpdateMock")
	}
	if capturedMock.HTTP == nil {
		t.Fatal("expected HTTP spec to be present")
	}
	if capturedMock.HTTP.Response == nil {
		t.Fatal("expected HTTP response to be present")
	}
	if capturedMock.HTTP.Response.StatusCode != 201 {
		t.Errorf("HTTP response statusCode = %d, want 201", capturedMock.HTTP.Response.StatusCode)
	}
}

func TestHandleManageMock_UpdateMissingID(t *testing.T) {
	t.Parallel()

	client := &mockAdminClient{}
	session := newTestSession(client)
	server := newTestServer(client)

	args := map[string]interface{}{"action": "update"}
	result, err := handleManageMock(args, session, server)
	if err != nil {
		t.Fatalf("handleManageMock() error = %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error result for missing id")
	}

	text := resultText(t, result)
	if text != "id is required" {
		t.Errorf("error text = %q, want %q", text, "id is required")
	}
}

func TestHandleManageMock_UpdateNotFound(t *testing.T) {
	t.Parallel()

	client := &mockAdminClient{
		getMockFn: func(id string) (*config.MockConfiguration, error) {
			return nil, &cli.APIError{StatusCode: 404, Message: "mock not found: " + id}
		},
	}

	session := newTestSession(client)
	server := newTestServer(client)

	args := map[string]interface{}{"action": "update", "id": "nonexistent"}
	result, err := handleManageMock(args, session, server)
	if err != nil {
		t.Fatalf("handleManageMock() error = %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error result for not-found mock")
	}

	text := resultText(t, result)
	if text != "mock not found: nonexistent" {
		t.Errorf("error text = %q, want %q", text, "mock not found: nonexistent")
	}
}

func TestHandleManageMock_UpdateFailure(t *testing.T) {
	t.Parallel()

	enabled := true
	client := &mockAdminClient{
		getMockFn: func(id string) (*config.MockConfiguration, error) {
			return &config.MockConfiguration{
				ID:      id,
				Type:    mock.TypeHTTP,
				Enabled: &enabled,
				HTTP: &mock.HTTPSpec{
					Matcher: &mock.HTTPMatcher{Method: "GET", Path: "/api/test"},
				},
			}, nil
		},
		updateMockFn: func(id string, m *config.MockConfiguration) (*config.MockConfiguration, error) {
			return nil, fmt.Errorf("dial tcp: connection refused")
		},
	}

	session := newTestSession(client)
	server := newTestServer(client)

	args := map[string]interface{}{
		"action": "update",
		"id":     "http_abc123",
		"name":   "Updated Name",
	}
	result, err := handleManageMock(args, session, server)
	if err != nil {
		t.Fatalf("handleManageMock() error = %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error result for update failure")
	}

	text := resultText(t, result)
	if !strings.Contains(text, "failed to update mock") {
		t.Errorf("error text = %q, want containing 'failed to update mock'", text)
	}
}

func TestHandleManageMock_UpdateNoClient(t *testing.T) {
	t.Parallel()

	session := NewSession()
	session.SetState(SessionStateReady)
	server := newTestServer(nil)

	args := map[string]interface{}{"action": "update", "id": "http_abc123"}
	result, err := handleManageMock(args, session, server)
	if err != nil {
		t.Fatalf("handleManageMock() error = %v", err)
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
// handleManageMock — List Filter Tests
// =============================================================================

func TestHandleManageMock_ListByType(t *testing.T) {
	t.Parallel()

	enabled := true
	client := &mockAdminClient{
		listMocksByTypeFn: func(mockType string) ([]*config.MockConfiguration, error) {
			if mockType != "http" {
				return nil, fmt.Errorf("unexpected type: %s", mockType)
			}
			return []*config.MockConfiguration{
				{
					ID:      "http_abc123",
					Type:    mock.TypeHTTP,
					Name:    "Get Users",
					Enabled: &enabled,
					HTTP: &mock.HTTPSpec{
						Matcher: &mock.HTTPMatcher{Method: "GET", Path: "/api/users"},
					},
				},
			}, nil
		},
	}

	session := newTestSession(client)
	server := newTestServer(client)

	args := map[string]interface{}{"action": "list", "type": "http"}
	result, err := handleManageMock(args, session, server)
	if err != nil {
		t.Fatalf("handleManageMock() error = %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success, got error: %s", resultText(t, result))
	}

	var summaries []MockSummary
	resultJSON(t, result, &summaries)

	if len(summaries) != 1 {
		t.Fatalf("expected 1 mock, got %d", len(summaries))
	}
	if summaries[0].Type != "http" {
		t.Errorf("summaries[0].Type = %s, want http", summaries[0].Type)
	}
}

func TestHandleManageMock_ListByEnabled(t *testing.T) {
	t.Parallel()

	enabledTrue := true
	enabledFalse := false
	client := &mockAdminClient{
		listMocksFn: func() ([]*config.MockConfiguration, error) {
			return []*config.MockConfiguration{
				{
					ID:      "http_1",
					Type:    mock.TypeHTTP,
					Name:    "Enabled Mock",
					Enabled: &enabledTrue,
					HTTP: &mock.HTTPSpec{
						Matcher: &mock.HTTPMatcher{Method: "GET", Path: "/api/a"},
					},
				},
				{
					ID:      "http_2",
					Type:    mock.TypeHTTP,
					Name:    "Disabled Mock",
					Enabled: &enabledFalse,
					HTTP: &mock.HTTPSpec{
						Matcher: &mock.HTTPMatcher{Method: "GET", Path: "/api/b"},
					},
				},
			}, nil
		},
	}

	session := newTestSession(client)
	server := newTestServer(client)

	args := map[string]interface{}{"action": "list", "enabled": true}
	result, err := handleManageMock(args, session, server)
	if err != nil {
		t.Fatalf("handleManageMock() error = %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success, got error: %s", resultText(t, result))
	}

	var summaries []MockSummary
	resultJSON(t, result, &summaries)

	if len(summaries) != 1 {
		t.Fatalf("expected 1 enabled mock, got %d", len(summaries))
	}
	if summaries[0].ID != "http_1" {
		t.Errorf("summaries[0].ID = %s, want http_1", summaries[0].ID)
	}
	if !summaries[0].Enabled {
		t.Error("expected summaries[0].Enabled = true")
	}
}

func TestHandleManageMock_ListByDisabled(t *testing.T) {
	t.Parallel()

	enabledTrue := true
	enabledFalse := false
	client := &mockAdminClient{
		listMocksFn: func() ([]*config.MockConfiguration, error) {
			return []*config.MockConfiguration{
				{
					ID:      "http_1",
					Type:    mock.TypeHTTP,
					Name:    "Enabled Mock",
					Enabled: &enabledTrue,
					HTTP: &mock.HTTPSpec{
						Matcher: &mock.HTTPMatcher{Method: "GET", Path: "/api/a"},
					},
				},
				{
					ID:      "http_2",
					Type:    mock.TypeHTTP,
					Name:    "Disabled Mock",
					Enabled: &enabledFalse,
					HTTP: &mock.HTTPSpec{
						Matcher: &mock.HTTPMatcher{Method: "GET", Path: "/api/b"},
					},
				},
			}, nil
		},
	}

	session := newTestSession(client)
	server := newTestServer(client)

	args := map[string]interface{}{"action": "list", "enabled": false}
	result, err := handleManageMock(args, session, server)
	if err != nil {
		t.Fatalf("handleManageMock() error = %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success, got error: %s", resultText(t, result))
	}

	var summaries []MockSummary
	resultJSON(t, result, &summaries)

	if len(summaries) != 1 {
		t.Fatalf("expected 1 disabled mock, got %d", len(summaries))
	}
	if summaries[0].ID != "http_2" {
		t.Errorf("summaries[0].ID = %s, want http_2", summaries[0].ID)
	}
	if summaries[0].Enabled {
		t.Error("expected summaries[0].Enabled = false")
	}
}

func TestHandleManageMock_ListByTypeAndEnabled(t *testing.T) {
	t.Parallel()

	enabledTrue := true
	enabledFalse := false
	client := &mockAdminClient{
		listMocksByTypeFn: func(mockType string) ([]*config.MockConfiguration, error) {
			return []*config.MockConfiguration{
				{
					ID:      "http_1",
					Type:    mock.TypeHTTP,
					Name:    "Enabled HTTP",
					Enabled: &enabledTrue,
					HTTP: &mock.HTTPSpec{
						Matcher: &mock.HTTPMatcher{Method: "GET", Path: "/api/a"},
					},
				},
				{
					ID:      "http_2",
					Type:    mock.TypeHTTP,
					Name:    "Disabled HTTP",
					Enabled: &enabledFalse,
					HTTP: &mock.HTTPSpec{
						Matcher: &mock.HTTPMatcher{Method: "POST", Path: "/api/b"},
					},
				},
			}, nil
		},
	}

	session := newTestSession(client)
	server := newTestServer(client)

	args := map[string]interface{}{"action": "list", "type": "http", "enabled": true}
	result, err := handleManageMock(args, session, server)
	if err != nil {
		t.Fatalf("handleManageMock() error = %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success, got error: %s", resultText(t, result))
	}

	var summaries []MockSummary
	resultJSON(t, result, &summaries)

	if len(summaries) != 1 {
		t.Fatalf("expected 1 enabled HTTP mock, got %d", len(summaries))
	}
	if summaries[0].ID != "http_1" {
		t.Errorf("summaries[0].ID = %s, want http_1", summaries[0].ID)
	}
}

func TestHandleManageMock_ListEmpty(t *testing.T) {
	t.Parallel()

	client := &mockAdminClient{
		listMocksFn: func() ([]*config.MockConfiguration, error) {
			return []*config.MockConfiguration{}, nil
		},
	}

	session := newTestSession(client)
	server := newTestServer(client)

	args := map[string]interface{}{"action": "list"}
	result, err := handleManageMock(args, session, server)
	if err != nil {
		t.Fatalf("handleManageMock() error = %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success, got error: %s", resultText(t, result))
	}

	var summaries []MockSummary
	resultJSON(t, result, &summaries)

	if len(summaries) != 0 {
		t.Errorf("expected 0 mocks, got %d", len(summaries))
	}
}

func TestHandleManageMock_ListError(t *testing.T) {
	t.Parallel()

	client := &mockAdminClient{
		listMocksFn: func() ([]*config.MockConfiguration, error) {
			return nil, fmt.Errorf("dial tcp: connection refused")
		},
	}

	session := newTestSession(client)
	server := newTestServer(client)

	args := map[string]interface{}{"action": "list"}
	result, err := handleManageMock(args, session, server)
	if err != nil {
		t.Fatalf("handleManageMock() error = %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error result for list failure")
	}

	text := resultText(t, result)
	if !strings.Contains(text, "failed to list mocks") {
		t.Errorf("error text = %q, want containing 'failed to list mocks'", text)
	}
}

func TestHandleManageMock_ListByTypeError(t *testing.T) {
	t.Parallel()

	client := &mockAdminClient{
		listMocksByTypeFn: func(mockType string) ([]*config.MockConfiguration, error) {
			return nil, fmt.Errorf("dial tcp: connection refused")
		},
	}

	session := newTestSession(client)
	server := newTestServer(client)

	args := map[string]interface{}{"action": "list", "type": "http"}
	result, err := handleManageMock(args, session, server)
	if err != nil {
		t.Fatalf("handleManageMock() error = %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error result for list by type failure")
	}

	text := resultText(t, result)
	if !strings.Contains(text, "failed to list mocks") {
		t.Errorf("error text = %q, want containing 'failed to list mocks'", text)
	}
}

// Ensure imports are used.
var _ = json.Unmarshal
var _ = fmt.Sprintf
var _ = strings.Contains
var _ config.MockConfiguration
var _ mock.Type
