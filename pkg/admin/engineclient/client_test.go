package engineclient

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/getmockd/mockd/pkg/config"
	"github.com/getmockd/mockd/pkg/requestlog"
)

// --- Helpers ---

// mockServer creates a test server that responds to a specific method+path with a handler.
func mockServer(t *testing.T, handler http.HandlerFunc) (*httptest.Server, *Client) {
	t.Helper()
	ts := httptest.NewServer(handler)
	t.Cleanup(ts.Close)
	c := New(ts.URL)
	return ts, c
}

func jsonHandler(t *testing.T, statusCode int, body interface{}) http.HandlerFunc {
	t.Helper()
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(statusCode)
		if body != nil {
			if err := json.NewEncoder(w).Encode(body); err != nil {
				t.Errorf("failed to encode response: %v", err)
			}
		}
	}
}

// --- New / Options Tests ---

func TestNew(t *testing.T) {
	c := New("http://localhost:4281")
	if c == nil {
		t.Fatal("New() returned nil")
	}
	if c.baseURL != "http://localhost:4281" {
		t.Errorf("baseURL = %q, want %q", c.baseURL, "http://localhost:4281")
	}
	if c.httpClient.Timeout != 30*time.Second {
		t.Errorf("default timeout = %v, want 30s", c.httpClient.Timeout)
	}
}

func TestNew_WithTimeout(t *testing.T) {
	c := New("http://localhost:4281", WithTimeout(5*time.Second))
	if c.httpClient.Timeout != 5*time.Second {
		t.Errorf("timeout = %v, want 5s", c.httpClient.Timeout)
	}
}

func TestNew_WithToken(t *testing.T) {
	c := New("http://localhost:4281", WithToken("secret-token"))
	if c.token != "secret-token" {
		t.Errorf("token = %q, want %q", c.token, "secret-token")
	}
}

// --- Health Tests ---

func TestHealth_Success(t *testing.T) {
	_, c := mockServer(t, jsonHandler(t, 200, map[string]string{"status": "ok"}))
	err := c.Health(context.Background())
	if err != nil {
		t.Errorf("Health() error = %v, want nil", err)
	}
}

func TestHealth_Unhealthy(t *testing.T) {
	_, c := mockServer(t, jsonHandler(t, 503, nil))
	err := c.Health(context.Background())
	if err == nil {
		t.Error("Health() error = nil, want error for 503")
	}
}

func TestHealth_ConnectionRefused(t *testing.T) {
	c := New("http://127.0.0.1:1") // port 1 should refuse
	err := c.Health(context.Background())
	if err == nil {
		t.Error("Health() error = nil, want connection error")
	}
}

// --- Status Tests ---

func TestStatus_Success(t *testing.T) {
	resp := StatusResponse{Status: "running", Uptime: 42, MockCount: 5}
	_, c := mockServer(t, jsonHandler(t, 200, resp))
	status, err := c.Status(context.Background())
	if err != nil {
		t.Fatalf("Status() error = %v", err)
	}
	if status.Status != "running" {
		t.Errorf("Status().Status = %q, want %q", status.Status, "running")
	}
	if status.Uptime != 42 {
		t.Errorf("Status().Uptime = %d, want 42", status.Uptime)
	}
	if status.MockCount != 5 {
		t.Errorf("Status().MockCount = %d, want 5", status.MockCount)
	}
}

func TestStatus_Error(t *testing.T) {
	_, c := mockServer(t, jsonHandler(t, 500, ErrorResponse{Error: "err", Message: "internal"}))
	_, err := c.Status(context.Background())
	if err == nil {
		t.Error("Status() error = nil, want error for 500")
	}
}

// --- Mock CRUD Tests ---

func TestCreateMock_Success(t *testing.T) {
	created := config.MockConfiguration{ID: "mock-1", Name: "Test"}
	_, c := mockServer(t, jsonHandler(t, 201, created))

	result, err := c.CreateMock(context.Background(), &config.MockConfiguration{Name: "Test"})
	if err != nil {
		t.Fatalf("CreateMock() error = %v", err)
	}
	if result.ID != "mock-1" {
		t.Errorf("CreateMock().ID = %q, want %q", result.ID, "mock-1")
	}
}

func TestCreateMock_Conflict(t *testing.T) {
	_, c := mockServer(t, jsonHandler(t, 409, ErrorResponse{Error: "conflict", Message: "duplicate"}))

	_, err := c.CreateMock(context.Background(), &config.MockConfiguration{ID: "dup"})
	if !errors.Is(err, ErrDuplicate) {
		t.Errorf("CreateMock() error = %v, want ErrDuplicate", err)
	}
}

func TestGetMock_Success(t *testing.T) {
	mock := config.MockConfiguration{ID: "mock-1", Name: "Test Mock"}
	_, c := mockServer(t, jsonHandler(t, 200, mock))

	result, err := c.GetMock(context.Background(), "mock-1")
	if err != nil {
		t.Fatalf("GetMock() error = %v", err)
	}
	if result.Name != "Test Mock" {
		t.Errorf("GetMock().Name = %q, want %q", result.Name, "Test Mock")
	}
}

func TestGetMock_NotFound(t *testing.T) {
	_, c := mockServer(t, jsonHandler(t, 404, nil))

	_, err := c.GetMock(context.Background(), "missing")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("GetMock() error = %v, want ErrNotFound", err)
	}
}

func TestListMocks_Success(t *testing.T) {
	resp := MockListResponse{
		Mocks: []*config.MockConfiguration{
			{ID: "mock-1"},
			{ID: "mock-2"},
		},
	}
	_, c := mockServer(t, jsonHandler(t, 200, resp))

	mocks, err := c.ListMocks(context.Background())
	if err != nil {
		t.Fatalf("ListMocks() error = %v", err)
	}
	if len(mocks) != 2 {
		t.Errorf("ListMocks() = %d mocks, want 2", len(mocks))
	}
}

func TestUpdateMock_Success(t *testing.T) {
	updated := config.MockConfiguration{ID: "mock-1", Name: "Updated"}
	_, c := mockServer(t, jsonHandler(t, 200, updated))

	result, err := c.UpdateMock(context.Background(), "mock-1", &config.MockConfiguration{Name: "Updated"})
	if err != nil {
		t.Fatalf("UpdateMock() error = %v", err)
	}
	if result.Name != "Updated" {
		t.Errorf("UpdateMock().Name = %q, want %q", result.Name, "Updated")
	}
}

func TestUpdateMock_NotFound(t *testing.T) {
	_, c := mockServer(t, jsonHandler(t, 404, nil))

	_, err := c.UpdateMock(context.Background(), "missing", &config.MockConfiguration{})
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("UpdateMock() error = %v, want ErrNotFound", err)
	}
}

func TestDeleteMock_Success(t *testing.T) {
	_, c := mockServer(t, jsonHandler(t, 204, nil))

	err := c.DeleteMock(context.Background(), "mock-1")
	if err != nil {
		t.Errorf("DeleteMock() error = %v, want nil", err)
	}
}

func TestDeleteMock_NotFound(t *testing.T) {
	_, c := mockServer(t, jsonHandler(t, 404, nil))

	err := c.DeleteMock(context.Background(), "missing")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("DeleteMock() error = %v, want ErrNotFound", err)
	}
}

func TestToggleMock_Success(t *testing.T) {
	toggled := config.MockConfiguration{ID: "mock-1"}
	_, c := mockServer(t, jsonHandler(t, 200, toggled))

	result, err := c.ToggleMock(context.Background(), "mock-1", true)
	if err != nil {
		t.Fatalf("ToggleMock() error = %v", err)
	}
	if result.ID != "mock-1" {
		t.Errorf("ToggleMock().ID = %q, want %q", result.ID, "mock-1")
	}
}

func TestToggleMock_NotFound(t *testing.T) {
	_, c := mockServer(t, jsonHandler(t, 404, nil))

	_, err := c.ToggleMock(context.Background(), "missing", true)
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("ToggleMock() error = %v, want ErrNotFound", err)
	}
}

// --- Request Logs Tests ---

func TestListRequests_NoFilter(t *testing.T) {
	resp := RequestListResponse{Count: 2}
	_, c := mockServer(t, jsonHandler(t, 200, resp))

	result, err := c.ListRequests(context.Background(), nil)
	if err != nil {
		t.Fatalf("ListRequests() error = %v", err)
	}
	if result.Count != 2 {
		t.Errorf("ListRequests().Count = %d, want 2", result.Count)
	}
}

func TestListRequests_WithFilter(t *testing.T) {
	var capturedPath string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedPath = r.URL.String()
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(RequestListResponse{Count: 0})
	}))
	defer ts.Close()
	c := New(ts.URL)

	_, err := c.ListRequests(context.Background(), &requestlog.Filter{
		Limit:     10,
		Offset:    5,
		Protocol:  "http",
		Method:    "GET",
		Path:      "/api/test",
		MatchedID: "mock-1",
	})
	if err != nil {
		t.Fatalf("ListRequests() error = %v", err)
	}

	// Verify query parameters were sent
	for _, param := range []string{"limit=10", "offset=5", "protocol=http", "method=GET", "matched=mock-1"} {
		if capturedPath == "" || !containsStr(capturedPath, param) {
			t.Errorf("request path %q missing param %q", capturedPath, param)
		}
	}
}

func TestClearRequests_Success(t *testing.T) {
	_, c := mockServer(t, jsonHandler(t, 200, map[string]int{"cleared": 42}))

	cleared, err := c.ClearRequests(context.Background())
	if err != nil {
		t.Fatalf("ClearRequests() error = %v", err)
	}
	if cleared != 42 {
		t.Errorf("ClearRequests() = %d, want 42", cleared)
	}
}

func TestClearRequestsByMockID_Success(t *testing.T) {
	_, c := mockServer(t, jsonHandler(t, 200, map[string]int{"cleared": 5}))

	cleared, err := c.ClearRequestsByMockID(context.Background(), "mock-1")
	if err != nil {
		t.Fatalf("ClearRequestsByMockID() error = %v", err)
	}
	if cleared != 5 {
		t.Errorf("ClearRequestsByMockID() = %d, want 5", cleared)
	}
}

func TestGetRequest_Success(t *testing.T) {
	entry := RequestLogEntry{ID: "req-1"}
	_, c := mockServer(t, jsonHandler(t, 200, entry))

	result, err := c.GetRequest(context.Background(), "req-1")
	if err != nil {
		t.Fatalf("GetRequest() error = %v", err)
	}
	if result.ID != "req-1" {
		t.Errorf("GetRequest().ID = %q, want %q", result.ID, "req-1")
	}
}

func TestGetRequest_NotFound(t *testing.T) {
	_, c := mockServer(t, jsonHandler(t, 404, nil))

	_, err := c.GetRequest(context.Background(), "missing")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("GetRequest() error = %v, want ErrNotFound", err)
	}
}

// --- Protocols Tests ---

func TestGetProtocols_Success(t *testing.T) {
	protocols := map[string]ProtocolStatus{
		"http": {Enabled: true, Port: 4280, Status: "running"},
	}
	_, c := mockServer(t, jsonHandler(t, 200, protocols))

	result, err := c.GetProtocols(context.Background())
	if err != nil {
		t.Fatalf("GetProtocols() error = %v", err)
	}
	if result["http"].Port != 4280 {
		t.Errorf("GetProtocols()[http].Port = %d, want 4280", result["http"].Port)
	}
}

// --- Deploy/Undeploy Tests ---

func TestDeploy_Success(t *testing.T) {
	resp := DeployResponse{Deployed: 3}
	_, c := mockServer(t, jsonHandler(t, 200, resp))

	result, err := c.Deploy(context.Background(), &DeployRequest{})
	if err != nil {
		t.Fatalf("Deploy() error = %v", err)
	}
	if result.Deployed != 3 {
		t.Errorf("Deploy().Deployed = %d, want 3", result.Deployed)
	}
}

func TestUndeploy_Success(t *testing.T) {
	_, c := mockServer(t, jsonHandler(t, 200, nil))
	err := c.Undeploy(context.Background())
	if err != nil {
		t.Errorf("Undeploy() error = %v, want nil", err)
	}
}

func TestUndeploy_Error(t *testing.T) {
	_, c := mockServer(t, jsonHandler(t, 500, ErrorResponse{Error: "err", Message: "fail"}))
	err := c.Undeploy(context.Background())
	if err == nil {
		t.Error("Undeploy() error = nil, want error for 500")
	}
}

// --- Chaos Tests ---

func TestGetChaos_Success(t *testing.T) {
	cfg := ChaosConfig{Enabled: true}
	_, c := mockServer(t, jsonHandler(t, 200, cfg))

	result, err := c.GetChaos(context.Background())
	if err != nil {
		t.Fatalf("GetChaos() error = %v", err)
	}
	if !result.Enabled {
		t.Error("GetChaos().Enabled = false, want true")
	}
}

func TestSetChaos_Success(t *testing.T) {
	_, c := mockServer(t, jsonHandler(t, 200, nil))
	err := c.SetChaos(context.Background(), &ChaosConfig{Enabled: true})
	if err != nil {
		t.Errorf("SetChaos() error = %v, want nil", err)
	}
}

// --- State Tests ---

func TestGetStateOverview_Success(t *testing.T) {
	overview := StateOverview{Total: 3, TotalItems: 15}
	_, c := mockServer(t, jsonHandler(t, 200, overview))

	result, err := c.GetStateOverview(context.Background())
	if err != nil {
		t.Fatalf("GetStateOverview() error = %v", err)
	}
	if result.Total != 3 {
		t.Errorf("GetStateOverview().Total = %d, want 3", result.Total)
	}
	if result.TotalItems != 15 {
		t.Errorf("GetStateOverview().TotalItems = %d, want 15", result.TotalItems)
	}
}

func TestResetState_Success(t *testing.T) {
	_, c := mockServer(t, jsonHandler(t, 200, nil))
	err := c.ResetState(context.Background(), "users")
	if err != nil {
		t.Errorf("ResetState() error = %v, want nil", err)
	}
}

func TestResetState_AllResources(t *testing.T) {
	var capturedBody map[string]string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&capturedBody)
		w.WriteHeader(200)
	}))
	defer ts.Close()
	c := New(ts.URL)

	_ = c.ResetState(context.Background(), "")
	if _, ok := capturedBody["resource"]; ok {
		t.Error("ResetState('') should not send resource field")
	}
}

func TestGetStateResource_NotFound(t *testing.T) {
	_, c := mockServer(t, jsonHandler(t, 404, nil))
	_, err := c.GetStateResource(context.Background(), "missing")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("GetStateResource() error = %v, want ErrNotFound", err)
	}
}

func TestClearStateResource_NotFound(t *testing.T) {
	_, c := mockServer(t, jsonHandler(t, 404, nil))
	err := c.ClearStateResource(context.Background(), "missing")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("ClearStateResource() error = %v, want ErrNotFound", err)
	}
}

// --- Handler Tests ---

func TestListHandlers_Success(t *testing.T) {
	resp := struct {
		Handlers []*ProtocolHandler `json:"handlers"`
		Count    int                `json:"count"`
	}{
		Handlers: []*ProtocolHandler{{ID: "h-1", Type: "graphql"}},
		Count:    1,
	}
	_, c := mockServer(t, jsonHandler(t, 200, resp))

	handlers, err := c.ListHandlers(context.Background())
	if err != nil {
		t.Fatalf("ListHandlers() error = %v", err)
	}
	if len(handlers) != 1 {
		t.Errorf("ListHandlers() = %d handlers, want 1", len(handlers))
	}
}

func TestGetHandler_NotFound(t *testing.T) {
	_, c := mockServer(t, jsonHandler(t, 404, nil))
	_, err := c.GetHandler(context.Background(), "missing")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("GetHandler() error = %v, want ErrNotFound", err)
	}
}

// --- SSE Tests ---

func TestListSSEConnections_Success(t *testing.T) {
	resp := struct {
		Connections []*SSEConnection `json:"connections"`
		Count       int              `json:"count"`
	}{
		Connections: []*SSEConnection{{ID: "sse-1"}},
		Count:       1,
	}
	_, c := mockServer(t, jsonHandler(t, 200, resp))

	conns, err := c.ListSSEConnections(context.Background())
	if err != nil {
		t.Fatalf("ListSSEConnections() error = %v", err)
	}
	if len(conns) != 1 {
		t.Errorf("ListSSEConnections() = %d, want 1", len(conns))
	}
}

func TestGetSSEConnection_NotFound(t *testing.T) {
	_, c := mockServer(t, jsonHandler(t, 404, nil))
	_, err := c.GetSSEConnection(context.Background(), "missing")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("GetSSEConnection() error = %v, want ErrNotFound", err)
	}
}

func TestCloseSSEConnection_NotFound(t *testing.T) {
	_, c := mockServer(t, jsonHandler(t, 404, nil))
	err := c.CloseSSEConnection(context.Background(), "missing")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("CloseSSEConnection() error = %v, want ErrNotFound", err)
	}
}

func TestGetSSEStats_Success(t *testing.T) {
	stats := SSEStats{ActiveConnections: 5}
	_, c := mockServer(t, jsonHandler(t, 200, stats))

	result, err := c.GetSSEStats(context.Background())
	if err != nil {
		t.Fatalf("GetSSEStats() error = %v", err)
	}
	if result.ActiveConnections != 5 {
		t.Errorf("GetSSEStats().ActiveConnections = %d, want 5", result.ActiveConnections)
	}
}

// --- Config Import/Export Tests ---

func TestExportConfig_Success(t *testing.T) {
	collection := config.MockCollection{Name: "test-export"}
	_, c := mockServer(t, jsonHandler(t, 200, collection))

	result, err := c.ExportConfig(context.Background(), "test-export")
	if err != nil {
		t.Fatalf("ExportConfig() error = %v", err)
	}
	if result.Name != "test-export" {
		t.Errorf("ExportConfig().Name = %q, want %q", result.Name, "test-export")
	}
}

func TestExportConfig_EmptyName(t *testing.T) {
	var capturedPath string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedPath = r.URL.String()
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(config.MockCollection{})
	}))
	defer ts.Close()
	c := New(ts.URL)

	_, _ = c.ExportConfig(context.Background(), "")
	if capturedPath != "/export" {
		t.Errorf("ExportConfig('') path = %q, want %q (no query param)", capturedPath, "/export")
	}
}

func TestImportConfig_Success(t *testing.T) {
	_, c := mockServer(t, jsonHandler(t, 200, map[string]any{"imported": 1, "total": 1}))

	result, err := c.ImportConfig(context.Background(), &config.MockCollection{Name: "test"}, false)
	if err != nil {
		t.Errorf("ImportConfig() error = %v, want nil", err)
	}
	if result == nil {
		t.Fatal("ImportConfig() result = nil, want non-nil")
	}
	if result.Imported != 1 {
		t.Errorf("ImportConfig() imported = %d, want 1", result.Imported)
	}
}

func TestImportConfig_Error(t *testing.T) {
	_, c := mockServer(t, jsonHandler(t, 400, ErrorResponse{Error: "bad_request", Message: "invalid config"}))

	_, err := c.ImportConfig(context.Background(), &config.MockCollection{}, false)
	if err == nil {
		t.Error("ImportConfig() error = nil, want error for 400")
	}
}

// --- Auth Token Tests ---

func TestAuthToken_SentInRequests(t *testing.T) {
	var capturedAuth string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedAuth = r.Header.Get("Authorization")
		w.WriteHeader(200)
	}))
	defer ts.Close()
	c := New(ts.URL, WithToken("my-token"))

	_ = c.Health(context.Background())
	if capturedAuth != "Bearer my-token" {
		t.Errorf("Authorization header = %q, want %q", capturedAuth, "Bearer my-token")
	}
}

func TestNoToken_NoAuthHeader(t *testing.T) {
	var capturedAuth string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedAuth = r.Header.Get("Authorization")
		w.WriteHeader(200)
	}))
	defer ts.Close()
	c := New(ts.URL)

	_ = c.Health(context.Background())
	if capturedAuth != "" {
		t.Errorf("Authorization header = %q, want empty", capturedAuth)
	}
}

// --- Error Parsing Tests ---

func TestParseError_StructuredError(t *testing.T) {
	_, c := mockServer(t, jsonHandler(t, 400, ErrorResponse{Error: "bad_request", Message: "invalid field"}))

	_, err := c.Status(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
	// Should contain the structured error message
	if !containsStr(err.Error(), "invalid field") {
		t.Errorf("error = %q, should contain 'invalid field'", err.Error())
	}
}

func TestParseError_PlainTextError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
		_, _ = w.Write([]byte("plain text error"))
	}))
	defer ts.Close()
	c := New(ts.URL)

	_, err := c.Status(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
	if !containsStr(err.Error(), "status 500") {
		t.Errorf("error = %q, should contain 'status 500'", err.Error())
	}
}

// --- Content-Type Tests ---

func TestPost_SetsContentType(t *testing.T) {
	var capturedCT string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedCT = r.Header.Get("Content-Type")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(201)
		_ = json.NewEncoder(w).Encode(config.MockConfiguration{ID: "test"})
	}))
	defer ts.Close()
	c := New(ts.URL)

	_, _ = c.CreateMock(context.Background(), &config.MockConfiguration{})
	if capturedCT != "application/json" {
		t.Errorf("Content-Type = %q, want %q", capturedCT, "application/json")
	}
}

// --- Context Cancellation Tests ---

func TestHealth_ContextCancelled(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(500 * time.Millisecond) // simulate slow server
		w.WriteHeader(200)
	}))
	defer ts.Close()
	c := New(ts.URL)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	err := c.Health(ctx)
	if err == nil {
		t.Error("Health() with cancelled context should error")
	}
}

// --- HTTP Method Verification Tests ---

func TestHTTPMethods(t *testing.T) {
	var capturedMethod string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedMethod = r.Method
		w.Header().Set("Content-Type", "application/json")
		switch r.Method {
		case "GET":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"status": "ok"})
		case "POST":
			w.WriteHeader(201)
			_ = json.NewEncoder(w).Encode(config.MockConfiguration{ID: "new"})
		case "PUT":
			_ = json.NewEncoder(w).Encode(config.MockConfiguration{ID: "updated"})
		case "DELETE":
			w.WriteHeader(204)
		}
	}))
	defer ts.Close()
	c := New(ts.URL)

	tests := []struct {
		name       string
		call       func() error
		wantMethod string
	}{
		{"Health/GET", func() error { return c.Health(context.Background()) }, "GET"},
		{"DeleteMock/DELETE", func() error { return c.DeleteMock(context.Background(), "1") }, "DELETE"},
		{"CreateMock/POST", func() error {
			_, err := c.CreateMock(context.Background(), &config.MockConfiguration{})
			return err
		}, "POST"},
		{"UpdateMock/PUT", func() error {
			_, err := c.UpdateMock(context.Background(), "1", &config.MockConfiguration{})
			return err
		}, "PUT"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_ = tt.call()
			if capturedMethod != tt.wantMethod {
				t.Errorf("HTTP method = %q, want %q", capturedMethod, tt.wantMethod)
			}
		})
	}
}

// containsStr checks if s contains substr.
func containsStr(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstring(s, substr))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
