package mcp

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/getmockd/mockd/pkg/cli"
	"github.com/getmockd/mockd/pkg/config"
	"github.com/getmockd/mockd/pkg/mock"
)

// =============================================================================
// Mock AdminClient — function-field pattern for per-test customization
// =============================================================================

type mockAdminClient struct {
	// Mock CRUD
	listMocksFn       func() ([]*config.MockConfiguration, error)
	listMocksByTypeFn func(string) ([]*config.MockConfiguration, error)
	getMockFn         func(id string) (*config.MockConfiguration, error)
	createMockFn      func(m *config.MockConfiguration) (*cli.CreateMockResult, error)
	updateMockFn      func(id string, m *config.MockConfiguration) (*config.MockConfiguration, error)
	toggleMockFn      func(id string) (*config.MockConfiguration, error)
	patchMockFn       func(id string, patch map[string]interface{}) (*config.MockConfiguration, error)
	deleteMockFn      func(id string) error

	// Stateful
	getStateOverviewFn       func() (*cli.StateOverviewResult, error)
	listStatefulItemsFn      func(name string, limit, offset int, sort, order string) (*cli.StatefulItemsResult, error)
	getStatefulItemFn        func(name, id string) (map[string]interface{}, error)
	createStatefulItemFn     func(name string, data map[string]interface{}) (map[string]interface{}, error)
	resetStatefulResourceFn  func(name string) error
	createStatefulResourceFn func(name, idField string) error
}

// Ensure mockAdminClient implements cli.AdminClient at compile time.
var _ cli.AdminClient = (*mockAdminClient)(nil)

// --- Mock CRUD ---

func (m *mockAdminClient) ListMocks() ([]*config.MockConfiguration, error) {
	if m.listMocksFn != nil {
		return m.listMocksFn()
	}
	return nil, nil
}

func (m *mockAdminClient) ListMocksByType(t string) ([]*config.MockConfiguration, error) {
	if m.listMocksByTypeFn != nil {
		return m.listMocksByTypeFn(t)
	}
	return nil, nil
}

func (m *mockAdminClient) GetMock(id string) (*config.MockConfiguration, error) {
	if m.getMockFn != nil {
		return m.getMockFn(id)
	}
	return nil, nil
}

func (m *mockAdminClient) CreateMock(mc *config.MockConfiguration) (*cli.CreateMockResult, error) {
	if m.createMockFn != nil {
		return m.createMockFn(mc)
	}
	return nil, nil
}

func (m *mockAdminClient) UpdateMock(id string, mc *config.MockConfiguration) (*config.MockConfiguration, error) {
	if m.updateMockFn != nil {
		return m.updateMockFn(id, mc)
	}
	return nil, nil
}

func (m *mockAdminClient) ToggleMock(id string) (*config.MockConfiguration, error) {
	if m.toggleMockFn != nil {
		return m.toggleMockFn(id)
	}
	return nil, nil
}

func (m *mockAdminClient) PatchMock(id string, patch map[string]interface{}) (*config.MockConfiguration, error) {
	if m.patchMockFn != nil {
		return m.patchMockFn(id, patch)
	}
	return nil, nil
}

func (m *mockAdminClient) DeleteMock(id string) error {
	if m.deleteMockFn != nil {
		return m.deleteMockFn(id)
	}
	return nil
}

// --- Import/Export ---

func (m *mockAdminClient) ImportConfig(_ *config.MockCollection, _ bool) (*cli.ImportResult, error) {
	return nil, nil
}

func (m *mockAdminClient) ExportConfig(_ string) (*config.MockCollection, error) {
	return nil, nil
}

// --- Logs ---

func (m *mockAdminClient) GetLogs(_ *cli.LogFilter) (*cli.LogResult, error) {
	return nil, nil
}

func (m *mockAdminClient) ClearLogs() (int, error) {
	return 0, nil
}

// --- Health/Status ---

func (m *mockAdminClient) Health() error { return nil }

func (m *mockAdminClient) GetStats() (*cli.StatsResult, error) {
	return nil, nil
}

func (m *mockAdminClient) GetPorts() ([]cli.PortInfo, error) {
	return nil, nil
}

func (m *mockAdminClient) GetPortsVerbose(_ bool) ([]cli.PortInfo, error) {
	return nil, nil
}

// --- Chaos ---

func (m *mockAdminClient) GetChaosConfig() (map[string]interface{}, error) {
	return nil, nil
}

func (m *mockAdminClient) SetChaosConfig(_ map[string]interface{}) error { return nil }

func (m *mockAdminClient) ListChaosProfiles() ([]cli.ChaosProfileInfo, error) {
	return nil, nil
}

func (m *mockAdminClient) GetChaosProfile(_ string) (*cli.ChaosProfileInfo, error) {
	return nil, nil
}

func (m *mockAdminClient) ApplyChaosProfile(_ string) error { return nil }

func (m *mockAdminClient) GetChaosStats() (map[string]interface{}, error) {
	return nil, nil
}

func (m *mockAdminClient) ResetChaosStats() error { return nil }

func (m *mockAdminClient) GetStatefulFaultStats() (map[string]interface{}, error) {
	return nil, nil
}

func (m *mockAdminClient) TripCircuitBreaker(_ string) error { return nil }

func (m *mockAdminClient) ResetCircuitBreaker(_ string) error { return nil }

// --- MQTT ---

func (m *mockAdminClient) GetMQTTStatus() (map[string]interface{}, error) {
	return nil, nil
}

// --- Stateful ---

func (m *mockAdminClient) CreateStatefulResource(name, idField string) error {
	if m.createStatefulResourceFn != nil {
		return m.createStatefulResourceFn(name, idField)
	}
	return nil
}

func (m *mockAdminClient) GetStateOverview() (*cli.StateOverviewResult, error) {
	if m.getStateOverviewFn != nil {
		return m.getStateOverviewFn()
	}
	return nil, nil
}

func (m *mockAdminClient) ListStatefulItems(name string, limit, offset int, sort, order string) (*cli.StatefulItemsResult, error) {
	if m.listStatefulItemsFn != nil {
		return m.listStatefulItemsFn(name, limit, offset, sort, order)
	}
	return nil, nil
}

func (m *mockAdminClient) GetStatefulItem(name, id string) (map[string]interface{}, error) {
	if m.getStatefulItemFn != nil {
		return m.getStatefulItemFn(name, id)
	}
	return nil, nil
}

func (m *mockAdminClient) CreateStatefulItem(name string, data map[string]interface{}) (map[string]interface{}, error) {
	if m.createStatefulItemFn != nil {
		return m.createStatefulItemFn(name, data)
	}
	return nil, nil
}

func (m *mockAdminClient) ResetStatefulResource(name string) error {
	if m.resetStatefulResourceFn != nil {
		return m.resetStatefulResourceFn(name)
	}
	return nil
}

// --- Custom Operations ---

func (m *mockAdminClient) ListCustomOperations() ([]cli.CustomOperationInfo, error) {
	return nil, nil
}

func (m *mockAdminClient) GetCustomOperation(_ string) (*cli.CustomOperationDetail, error) {
	return nil, nil
}

func (m *mockAdminClient) RegisterCustomOperation(_ map[string]interface{}) error { return nil }

func (m *mockAdminClient) DeleteCustomOperation(_ string) error { return nil }

func (m *mockAdminClient) ExecuteCustomOperation(_ string, _ map[string]interface{}) (map[string]interface{}, error) {
	return nil, nil
}

// --- Verification ---

func (m *mockAdminClient) GetMockVerification(_ string) (map[string]interface{}, error) {
	return nil, nil
}

func (m *mockAdminClient) VerifyMock(_ string, _ map[string]interface{}) (map[string]interface{}, error) {
	return nil, nil
}

func (m *mockAdminClient) ListMockInvocations(_ string) (map[string]interface{}, error) {
	return nil, nil
}

func (m *mockAdminClient) ResetMockVerification(_ string) error { return nil }

func (m *mockAdminClient) ResetAllVerification() error { return nil }

// --- Workspaces ---

func (m *mockAdminClient) ListWorkspaces() ([]*cli.WorkspaceDTO, error) {
	return nil, nil
}

func (m *mockAdminClient) CreateWorkspace(_ string) (*cli.WorkspaceResult, error) {
	return nil, nil
}

func (m *mockAdminClient) RegisterEngine(_ string, _ string, _ int) (*cli.RegisterEngineResult, error) {
	return nil, nil
}

func (m *mockAdminClient) HeartbeatEngine(_, _ string) error { return nil }

func (m *mockAdminClient) AddEngineWorkspace(_, _, _ string) error { return nil }

func (m *mockAdminClient) BulkCreateMocks(_ []*mock.Mock, _ string) (*cli.BulkCreateResult, error) {
	return nil, nil
}

// =============================================================================
// Test Helpers
// =============================================================================

// newTestSession creates a ready session with the given admin client attached.
func newTestSession(client cli.AdminClient) *MCPSession {
	session := NewSession()
	session.SetContext("test-ctx", "http://localhost:4290", "", client)
	session.SetState(SessionStateReady)
	return session
}

// newTestServer creates a minimal MCP server for testing tool handlers.
func newTestServer(client cli.AdminClient) *Server {
	cfg := DefaultConfig()
	return NewServer(cfg, client, nil)
}

// resultText extracts the text from the first content block of a ToolResult.
func resultText(t *testing.T, result *ToolResult) string {
	t.Helper()
	if len(result.Content) == 0 {
		t.Fatal("ToolResult has no content blocks")
	}
	return result.Content[0].Text
}

// resultJSON unmarshals the first content block text into the target.
func resultJSON(t *testing.T, result *ToolResult, target interface{}) {
	t.Helper()
	text := resultText(t, result)
	if err := json.Unmarshal([]byte(text), target); err != nil {
		t.Fatalf("failed to unmarshal result JSON: %v\nraw: %s", err, text)
	}
}

// boolPtr returns a pointer to a bool value.
func boolPtr(b bool) *bool {
	return &b
}

// =============================================================================
// handleManageMock Tests
// =============================================================================

func TestHandleManageMock_ListAction(t *testing.T) {
	t.Parallel()

	enabled := true
	client := &mockAdminClient{
		listMocksFn: func() ([]*config.MockConfiguration, error) {
			return []*config.MockConfiguration{
				{
					ID:      "http_abc123",
					Type:    mock.TypeHTTP,
					Name:    "Get Users",
					Enabled: &enabled,
					HTTP: &mock.HTTPSpec{
						Matcher: &mock.HTTPMatcher{
							Method: "GET",
							Path:   "/api/users",
						},
					},
				},
				{
					ID:      "http_def456",
					Type:    mock.TypeHTTP,
					Name:    "Create User",
					Enabled: &enabled,
					HTTP: &mock.HTTPSpec{
						Matcher: &mock.HTTPMatcher{
							Method: "POST",
							Path:   "/api/users",
						},
					},
				},
			}, nil
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

	if len(summaries) != 2 {
		t.Fatalf("expected 2 mocks, got %d", len(summaries))
	}
	if summaries[0].ID != "http_abc123" {
		t.Errorf("summaries[0].ID = %s, want http_abc123", summaries[0].ID)
	}
	if summaries[0].Summary != "GET /api/users" {
		t.Errorf("summaries[0].Summary = %s, want GET /api/users", summaries[0].Summary)
	}
	if summaries[1].ID != "http_def456" {
		t.Errorf("summaries[1].ID = %s, want http_def456", summaries[1].ID)
	}
}

func TestHandleManageMock_GetAction(t *testing.T) {
	t.Parallel()

	enabled := true
	client := &mockAdminClient{
		getMockFn: func(id string) (*config.MockConfiguration, error) {
			if id == "http_abc123" {
				return &config.MockConfiguration{
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
						},
					},
				}, nil
			}
			return nil, &cli.APIError{StatusCode: 404, ErrorCode: "not_found", Message: "mock not found: " + id}
		},
	}

	session := newTestSession(client)
	server := newTestServer(client)

	args := map[string]interface{}{"action": "get", "id": "http_abc123"}
	result, err := handleManageMock(args, session, server)
	if err != nil {
		t.Fatalf("handleManageMock() error = %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success, got error: %s", resultText(t, result))
	}

	var mockCfg config.MockConfiguration
	resultJSON(t, result, &mockCfg)

	if mockCfg.ID != "http_abc123" {
		t.Errorf("mock ID = %s, want http_abc123", mockCfg.ID)
	}
	if mockCfg.Type != mock.TypeHTTP {
		t.Errorf("mock Type = %s, want http", mockCfg.Type)
	}
}

func TestHandleManageMock_GetNotFound(t *testing.T) {
	t.Parallel()

	client := &mockAdminClient{
		getMockFn: func(id string) (*config.MockConfiguration, error) {
			return nil, &cli.APIError{StatusCode: 404, ErrorCode: "not_found", Message: "mock not found: " + id}
		},
	}

	session := newTestSession(client)
	server := newTestServer(client)

	args := map[string]interface{}{"action": "get", "id": "nonexistent"}
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

func TestHandleManageMock_CreateAction(t *testing.T) {
	t.Parallel()

	client := &mockAdminClient{
		createMockFn: func(m *config.MockConfiguration) (*cli.CreateMockResult, error) {
			return &cli.CreateMockResult{
				Mock:   &config.MockConfiguration{ID: "http_new789", Type: mock.TypeHTTP},
				Action: "created",
			}, nil
		},
	}

	session := newTestSession(client)
	server := newTestServer(client)

	args := map[string]interface{}{
		"action": "create",
		"type":   "http",
		"name":   "My Mock",
		"http": map[string]interface{}{
			"matcher": map[string]interface{}{
				"method": "GET",
				"path":   "/api/hello",
			},
			"response": map[string]interface{}{
				"statusCode": float64(200),
				"body":       `{"msg":"hello"}`,
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

	if parsed["id"] != "http_new789" {
		t.Errorf("id = %v, want http_new789", parsed["id"])
	}
	if parsed["action"] != "created" {
		t.Errorf("action = %v, want created", parsed["action"])
	}
}

func TestHandleManageMock_DeleteAction(t *testing.T) {
	t.Parallel()

	deletedID := ""
	client := &mockAdminClient{
		deleteMockFn: func(id string) error {
			deletedID = id
			return nil
		},
	}

	session := newTestSession(client)
	server := newTestServer(client)

	args := map[string]interface{}{"action": "delete", "id": "http_abc123"}
	result, err := handleManageMock(args, session, server)
	if err != nil {
		t.Fatalf("handleManageMock() error = %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success, got error: %s", resultText(t, result))
	}

	if deletedID != "http_abc123" {
		t.Errorf("deleted ID = %s, want http_abc123", deletedID)
	}

	var parsed map[string]interface{}
	resultJSON(t, result, &parsed)

	if parsed["deleted"] != true {
		t.Errorf("deleted = %v, want true", parsed["deleted"])
	}
	if parsed["id"] != "http_abc123" {
		t.Errorf("id = %v, want http_abc123", parsed["id"])
	}
}

func TestHandleManageMock_DeleteNotFound(t *testing.T) {
	t.Parallel()

	client := &mockAdminClient{
		deleteMockFn: func(id string) error {
			return &cli.APIError{StatusCode: 404, ErrorCode: "not_found", Message: "mock not found: " + id}
		},
	}

	session := newTestSession(client)
	server := newTestServer(client)

	args := map[string]interface{}{"action": "delete", "id": "nonexistent"}
	result, err := handleManageMock(args, session, server)
	if err != nil {
		t.Fatalf("handleManageMock() error = %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error result for deleting non-existent mock")
	}

	text := resultText(t, result)
	if text == "" {
		t.Error("expected non-empty error text")
	}
}

func TestHandleManageMock_MissingAction(t *testing.T) {
	t.Parallel()

	client := &mockAdminClient{}
	session := newTestSession(client)
	server := newTestServer(client)

	// No "action" key at all
	args := map[string]interface{}{}
	result, err := handleManageMock(args, session, server)
	if err != nil {
		t.Fatalf("handleManageMock() error = %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error result for missing action")
	}

	text := resultText(t, result)
	if text == "" {
		t.Error("expected non-empty error message")
	}
}

func TestHandleManageMock_UnknownAction(t *testing.T) {
	t.Parallel()

	client := &mockAdminClient{}
	session := newTestSession(client)
	server := newTestServer(client)

	args := map[string]interface{}{"action": "explode"}
	result, err := handleManageMock(args, session, server)
	if err != nil {
		t.Fatalf("handleManageMock() error = %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error result for unknown action")
	}

	text := resultText(t, result)
	if text != "invalid action: explode. Use: list, get, create, update, delete, toggle" {
		t.Errorf("error text = %q, want explicit invalid action message", text)
	}
}

func TestHandleManageMock_ToggleAction(t *testing.T) {
	t.Parallel()

	patchedID := ""
	patchedEnabled := false
	client := &mockAdminClient{
		patchMockFn: func(id string, patch map[string]interface{}) (*config.MockConfiguration, error) {
			patchedID = id
			if v, ok := patch["enabled"].(bool); ok {
				patchedEnabled = v
			}
			return &config.MockConfiguration{ID: id, Type: mock.TypeHTTP, Enabled: boolPtr(false)}, nil
		},
	}

	session := newTestSession(client)
	server := newTestServer(client)

	args := map[string]interface{}{"action": "toggle", "id": "http_abc123", "enabled": false}
	result, err := handleManageMock(args, session, server)
	if err != nil {
		t.Fatalf("handleManageMock() error = %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success, got error: %s", resultText(t, result))
	}

	if patchedID != "http_abc123" {
		t.Errorf("patched ID = %s, want http_abc123", patchedID)
	}
	if patchedEnabled != false {
		t.Errorf("patched enabled = %v, want false", patchedEnabled)
	}

	var parsed map[string]interface{}
	resultJSON(t, result, &parsed)

	if parsed["toggled"] != true {
		t.Errorf("toggled = %v, want true", parsed["toggled"])
	}
}

func TestHandleManageMock_NoAdminClient(t *testing.T) {
	t.Parallel()

	// Session without an admin client — simulates pre-initialization state
	session := NewSession()
	session.SetState(SessionStateReady)
	server := newTestServer(nil)

	args := map[string]interface{}{"action": "list"}
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
// handleManageState Tests
// =============================================================================

func TestHandleManageState_OverviewAction(t *testing.T) {
	t.Parallel()

	client := &mockAdminClient{
		getStateOverviewFn: func() (*cli.StateOverviewResult, error) {
			return &cli.StateOverviewResult{
				Resources: []cli.StatefulResourceInfo{
					{Name: "users", ItemCount: 5, IDField: "id"},
					{Name: "products", ItemCount: 12, IDField: "id"},
				},
				Total:      2,
				TotalItems: 17,
			}, nil
		},
	}

	session := newTestSession(client)
	server := newTestServer(client)

	args := map[string]interface{}{"action": "overview"}
	result, err := handleManageState(args, session, server)
	if err != nil {
		t.Fatalf("handleManageState() error = %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success, got error: %s", resultText(t, result))
	}

	var overview cli.StateOverviewResult
	resultJSON(t, result, &overview)

	if overview.Total != 2 {
		t.Errorf("Total = %d, want 2", overview.Total)
	}
	if overview.TotalItems != 17 {
		t.Errorf("TotalItems = %d, want 17", overview.TotalItems)
	}
	if len(overview.Resources) != 2 {
		t.Fatalf("expected 2 resources, got %d", len(overview.Resources))
	}
	if overview.Resources[0].Name != "users" {
		t.Errorf("Resources[0].Name = %s, want users", overview.Resources[0].Name)
	}
}

func TestHandleManageState_ListItemsAction(t *testing.T) {
	t.Parallel()

	client := &mockAdminClient{
		listStatefulItemsFn: func(name string, limit, offset int, sort, order string) (*cli.StatefulItemsResult, error) {
			if name != "users" {
				return nil, fmt.Errorf("unexpected resource: %s", name)
			}
			return &cli.StatefulItemsResult{
				Data: []map[string]interface{}{
					{"id": "u1", "name": "Alice"},
					{"id": "u2", "name": "Bob"},
				},
				Meta: cli.StatefulPaginationMeta{
					Total:  2,
					Limit:  limit,
					Offset: offset,
					Count:  2,
				},
			}, nil
		},
	}

	session := newTestSession(client)
	server := newTestServer(client)

	args := map[string]interface{}{
		"action":   "list_items",
		"resource": "users",
		"limit":    float64(50),
		"offset":   float64(0),
	}
	result, err := handleManageState(args, session, server)
	if err != nil {
		t.Fatalf("handleManageState() error = %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success, got error: %s", resultText(t, result))
	}

	var listResult StatefulListResult
	resultJSON(t, result, &listResult)

	if len(listResult.Data) != 2 {
		t.Fatalf("expected 2 items, got %d", len(listResult.Data))
	}
	if listResult.Data[0]["name"] != "Alice" {
		t.Errorf("Data[0].name = %v, want Alice", listResult.Data[0]["name"])
	}
	if listResult.Meta.Total != 2 {
		t.Errorf("Meta.Total = %d, want 2", listResult.Meta.Total)
	}
}

func TestHandleManageState_GetItemAction(t *testing.T) {
	t.Parallel()

	client := &mockAdminClient{
		getStatefulItemFn: func(name, id string) (map[string]interface{}, error) {
			if name == "users" && id == "u1" {
				return map[string]interface{}{
					"id":    "u1",
					"name":  "Alice",
					"email": "alice@example.com",
				}, nil
			}
			return nil, &cli.APIError{StatusCode: 404, ErrorCode: "not_found", Message: "item not found"}
		},
	}

	session := newTestSession(client)
	server := newTestServer(client)

	args := map[string]interface{}{
		"action":   "get_item",
		"resource": "users",
		"item_id":  "u1",
	}
	result, err := handleManageState(args, session, server)
	if err != nil {
		t.Fatalf("handleManageState() error = %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success, got error: %s", resultText(t, result))
	}

	var item map[string]interface{}
	resultJSON(t, result, &item)

	if item["id"] != "u1" {
		t.Errorf("item.id = %v, want u1", item["id"])
	}
	if item["email"] != "alice@example.com" {
		t.Errorf("item.email = %v, want alice@example.com", item["email"])
	}
}

func TestHandleManageState_GetItemNotFound(t *testing.T) {
	t.Parallel()

	client := &mockAdminClient{
		getStatefulItemFn: func(name, id string) (map[string]interface{}, error) {
			return nil, &cli.APIError{StatusCode: 404, ErrorCode: "not_found", Message: "item not found"}
		},
	}

	session := newTestSession(client)
	server := newTestServer(client)

	args := map[string]interface{}{
		"action":   "get_item",
		"resource": "users",
		"item_id":  "nonexistent",
	}
	result, err := handleManageState(args, session, server)
	if err != nil {
		t.Fatalf("handleManageState() error = %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error result for not-found item")
	}

	text := resultText(t, result)
	if text != "item not found: nonexistent in resource users" {
		t.Errorf("error text = %q, want %q", text, "item not found: nonexistent in resource users")
	}
}

func TestHandleManageState_CreateItemAction(t *testing.T) {
	t.Parallel()

	client := &mockAdminClient{
		createStatefulItemFn: func(name string, data map[string]interface{}) (map[string]interface{}, error) {
			if name != "users" {
				return nil, fmt.Errorf("unexpected resource: %s", name)
			}
			// Simulate server assigning an ID
			result := make(map[string]interface{})
			for k, v := range data {
				result[k] = v
			}
			result["id"] = "u_generated"
			return result, nil
		},
	}

	session := newTestSession(client)
	server := newTestServer(client)

	args := map[string]interface{}{
		"action":   "create_item",
		"resource": "users",
		"data": map[string]interface{}{
			"name":  "Charlie",
			"email": "charlie@example.com",
		},
	}
	result, err := handleManageState(args, session, server)
	if err != nil {
		t.Fatalf("handleManageState() error = %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success, got error: %s", resultText(t, result))
	}

	var item map[string]interface{}
	resultJSON(t, result, &item)

	if item["id"] != "u_generated" {
		t.Errorf("item.id = %v, want u_generated", item["id"])
	}
	if item["name"] != "Charlie" {
		t.Errorf("item.name = %v, want Charlie", item["name"])
	}
}

func TestHandleManageState_AddResourceAction(t *testing.T) {
	t.Parallel()

	createdName := ""
	createdIDField := ""
	client := &mockAdminClient{
		createStatefulResourceFn: func(name, idField string) error {
			createdName = name
			createdIDField = idField
			return nil
		},
	}

	session := newTestSession(client)
	server := newTestServer(client)

	args := map[string]interface{}{
		"action":   "add_resource",
		"resource": "orders",
		"id_field": "order_id",
	}
	result, err := handleManageState(args, session, server)
	if err != nil {
		t.Fatalf("handleManageState() error = %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success, got error: %s", resultText(t, result))
	}

	if createdName != "orders" {
		t.Errorf("created resource name = %s, want orders", createdName)
	}
	if createdIDField != "order_id" {
		t.Errorf("created idField = %s, want order_id", createdIDField)
	}

	var parsed map[string]interface{}
	resultJSON(t, result, &parsed)

	if parsed["created"] != true {
		t.Errorf("created = %v, want true", parsed["created"])
	}
	if parsed["resource"] != "orders" {
		t.Errorf("resource = %v, want orders", parsed["resource"])
	}
}

func TestHandleManageState_ResetAction(t *testing.T) {
	t.Parallel()

	resetName := ""
	client := &mockAdminClient{
		resetStatefulResourceFn: func(name string) error {
			resetName = name
			return nil
		},
	}

	session := newTestSession(client)
	server := newTestServer(client)

	args := map[string]interface{}{
		"action":   "reset",
		"resource": "users",
	}
	result, err := handleManageState(args, session, server)
	if err != nil {
		t.Fatalf("handleManageState() error = %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success, got error: %s", resultText(t, result))
	}

	if resetName != "users" {
		t.Errorf("reset resource = %s, want users", resetName)
	}

	var parsed map[string]interface{}
	resultJSON(t, result, &parsed)

	if parsed["reset"] != true {
		t.Errorf("reset = %v, want true", parsed["reset"])
	}
	if parsed["resource"] != "users" {
		t.Errorf("resource = %v, want users", parsed["resource"])
	}
}

func TestHandleManageState_MissingAction(t *testing.T) {
	t.Parallel()

	client := &mockAdminClient{}
	session := newTestSession(client)
	server := newTestServer(client)

	args := map[string]interface{}{}
	result, err := handleManageState(args, session, server)
	if err != nil {
		t.Fatalf("handleManageState() error = %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error result for missing action")
	}

	text := resultText(t, result)
	if text == "" {
		t.Error("expected non-empty error message")
	}
}

func TestHandleManageState_UnknownAction(t *testing.T) {
	t.Parallel()

	client := &mockAdminClient{}
	session := newTestSession(client)
	server := newTestServer(client)

	args := map[string]interface{}{"action": "nuke"}
	result, err := handleManageState(args, session, server)
	if err != nil {
		t.Fatalf("handleManageState() error = %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error result for unknown action")
	}

	text := resultText(t, result)
	if text != "invalid action: nuke. Use: overview, add_resource, list_items, get_item, create_item, reset" {
		t.Errorf("error text = %q, want explicit invalid action message", text)
	}
}

func TestHandleManageState_NoAdminClient(t *testing.T) {
	t.Parallel()

	session := NewSession()
	session.SetState(SessionStateReady)
	server := newTestServer(nil)

	args := map[string]interface{}{"action": "overview"}
	result, err := handleManageState(args, session, server)
	if err != nil {
		t.Fatalf("handleManageState() error = %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error result when admin client is nil")
	}

	text := resultText(t, result)
	if text != "admin client not available" {
		t.Errorf("error text = %q, want %q", text, "admin client not available")
	}
}

func TestHandleManageState_ResetMissingResource(t *testing.T) {
	t.Parallel()

	client := &mockAdminClient{}
	session := newTestSession(client)
	server := newTestServer(client)

	// reset without specifying resource
	args := map[string]interface{}{"action": "reset"}
	result, err := handleManageState(args, session, server)
	if err != nil {
		t.Fatalf("handleManageState() error = %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error result for reset without resource")
	}

	text := resultText(t, result)
	if text == "" {
		t.Error("expected non-empty error message for missing resource")
	}
}
