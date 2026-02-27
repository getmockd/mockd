package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/getmockd/mockd/pkg/config"
	"github.com/getmockd/mockd/pkg/mock"
	"github.com/getmockd/mockd/pkg/requestlog"
	"github.com/getmockd/mockd/pkg/stateful"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockEngine is a test double for EngineController.
type mockEngine struct {
	mocks          map[string]*config.MockConfiguration
	requestLogs    map[string]*requestlog.Entry
	running        bool
	uptime         int
	chaosConfig    *ChaosConfig
	chaosStats     *ChaosStats
	stateOverview  *StateOverview
	handlers       []*ProtocolHandler
	sseConnections []*SSEConnection
	wsConnections  []*WebSocketConnection
	sseStats       *SSEStats
	wsStats        *WebSocketStats
	configResp     *ConfigResponse
	protocols      map[string]ProtocolStatusInfo

	// Custom operations support
	customOps map[string]*CustomOperationDetail

	// Error injection for testing error paths
	addMockErr            error
	updateMockErr         error
	deleteMockErr         error
	createStatefulItemErr error
	getStateResourceErr   error
	clearStateResourceErr error
	resetStateErr         error
	listStatefulItemsErr  error
	getStatefulItemErr    error
}

func newMockEngine() *mockEngine {
	return &mockEngine{
		mocks:       make(map[string]*config.MockConfiguration),
		requestLogs: make(map[string]*requestlog.Entry),
		customOps:   make(map[string]*CustomOperationDetail),
		running:     true,
		uptime:      100,
		protocols: map[string]ProtocolStatusInfo{
			"http": {Enabled: true, Port: 4280, Status: "running"},
		},
	}
}

func (m *mockEngine) IsRunning() bool {
	return m.running
}

func (m *mockEngine) Uptime() int {
	return m.uptime
}

func (m *mockEngine) AddMock(cfg *config.MockConfiguration) error {
	if m.addMockErr != nil {
		return m.addMockErr
	}
	if cfg.ID == "" {
		cfg.ID = "mock-" + time.Now().Format("20060102150405")
	}
	m.mocks[cfg.ID] = cfg
	return nil
}

func (m *mockEngine) UpdateMock(id string, cfg *config.MockConfiguration) error {
	if m.updateMockErr != nil {
		return m.updateMockErr
	}
	if _, ok := m.mocks[id]; !ok {
		return errors.New("mock not found")
	}
	m.mocks[id] = cfg
	return nil
}

func (m *mockEngine) DeleteMock(id string) error {
	if m.deleteMockErr != nil {
		return m.deleteMockErr
	}
	if _, ok := m.mocks[id]; !ok {
		return errors.New("mock not found")
	}
	delete(m.mocks, id)
	return nil
}

func (m *mockEngine) GetMock(id string) *config.MockConfiguration {
	return m.mocks[id]
}

func (m *mockEngine) ListMocks() []*config.MockConfiguration {
	result := make([]*config.MockConfiguration, 0, len(m.mocks))
	for _, mock := range m.mocks {
		result = append(result, mock)
	}
	return result
}

func (m *mockEngine) ClearMocks() {
	m.mocks = make(map[string]*config.MockConfiguration)
}

func (m *mockEngine) GetRequestLogs(filter *requestlog.Filter) []*requestlog.Entry {
	result := make([]*requestlog.Entry, 0, len(m.requestLogs))
	for _, entry := range m.requestLogs {
		if filter != nil {
			// Filter by mock ID (matched filter)
			if filter.MatchedID != "" && entry.MatchedMockID != filter.MatchedID {
				continue
			}
			// Filter by method
			if filter.Method != "" && entry.Method != filter.Method {
				continue
			}
			// Filter by path (substring match)
			if filter.Path != "" && !strings.Contains(entry.Path, filter.Path) {
				continue
			}
			// Filter by hasError
			if filter.HasError != nil {
				hasErr := entry.Error != ""
				if hasErr != *filter.HasError {
					continue
				}
			}
		}
		result = append(result, entry)
	}
	// Apply limit if set
	if filter != nil && filter.Limit > 0 && len(result) > filter.Limit {
		result = result[:filter.Limit]
	}
	return result
}

func (m *mockEngine) GetRequestLog(id string) *requestlog.Entry {
	return m.requestLogs[id]
}

func (m *mockEngine) RequestLogCount() int {
	return len(m.requestLogs)
}

func (m *mockEngine) ClearRequestLogs() {
	m.requestLogs = make(map[string]*requestlog.Entry)
}

func (m *mockEngine) ClearRequestLogsByMockID(mockID string) int {
	return 0
}

func (m *mockEngine) ProtocolStatus() map[string]ProtocolStatusInfo {
	return m.protocols
}

func (m *mockEngine) GetChaosConfig() *ChaosConfig {
	return m.chaosConfig
}

func (m *mockEngine) SetChaosConfig(cfg *ChaosConfig) error {
	m.chaosConfig = cfg
	return nil
}

func (m *mockEngine) GetChaosStats() *ChaosStats {
	return m.chaosStats
}

func (m *mockEngine) ResetChaosStats() {
	m.chaosStats = &ChaosStats{FaultsByType: make(map[string]int64)}
}

func (m *mockEngine) GetStatefulFaultStats() *StatefulFaultStats {
	return &StatefulFaultStats{}
}

func (m *mockEngine) TripCircuitBreaker(_ string) error {
	return nil
}

func (m *mockEngine) ResetCircuitBreaker(_ string) error {
	return nil
}

func (m *mockEngine) GetStateOverview() *StateOverview {
	return m.stateOverview
}

func (m *mockEngine) GetStateResource(name string) (*StatefulResource, error) {
	if m.getStateResourceErr != nil {
		return nil, m.getStateResourceErr
	}
	return nil, errors.New("resource not found")
}

func (m *mockEngine) ClearStateResource(name string) (int, error) {
	if m.clearStateResourceErr != nil {
		return 0, m.clearStateResourceErr
	}
	return 0, errors.New("resource not found")
}

func (m *mockEngine) ResetState(resourceName string) (*ResetStateResponse, error) {
	if m.resetStateErr != nil {
		return nil, m.resetStateErr
	}
	return &ResetStateResponse{Reset: true, Resources: []string{}, Message: "state reset"}, nil
}

func (m *mockEngine) RegisterStatefulResource(cfg *config.StatefulResourceConfig) error {
	return nil // No-op for mock
}

func (m *mockEngine) ListStatefulItems(name string, limit, offset int, sort, order string) (*StatefulItemsResponse, error) {
	if m.listStatefulItemsErr != nil {
		return nil, m.listStatefulItemsErr
	}
	return &StatefulItemsResponse{
		Data: []map[string]interface{}{},
	}, nil
}

func (m *mockEngine) GetStatefulItem(resourceName, itemID string) (map[string]interface{}, error) {
	if m.getStatefulItemErr != nil {
		return nil, m.getStatefulItemErr
	}
	return nil, errors.New("item not found")
}

func (m *mockEngine) CreateStatefulItem(resourceName string, data map[string]interface{}) (map[string]interface{}, error) {
	if m.createStatefulItemErr != nil {
		return nil, m.createStatefulItemErr
	}
	return data, nil
}

func (m *mockEngine) ListProtocolHandlers() []*ProtocolHandler {
	return m.handlers
}

func (m *mockEngine) GetProtocolHandler(id string) *ProtocolHandler {
	for _, h := range m.handlers {
		if h.ID == id {
			return h
		}
	}
	return nil
}

func (m *mockEngine) ListSSEConnections() []*SSEConnection {
	return m.sseConnections
}

func (m *mockEngine) GetSSEConnection(id string) *SSEConnection {
	for _, c := range m.sseConnections {
		if c.ID == id {
			return c
		}
	}
	return nil
}

func (m *mockEngine) CloseSSEConnection(id string) error {
	for i, c := range m.sseConnections {
		if c.ID == id {
			m.sseConnections = append(m.sseConnections[:i], m.sseConnections[i+1:]...)
			return nil
		}
	}
	return errors.New("connection not found")
}

func (m *mockEngine) GetSSEStats() *SSEStats {
	return m.sseStats
}

func (m *mockEngine) ListWebSocketConnections() []*WebSocketConnection {
	return m.wsConnections
}

func (m *mockEngine) GetWebSocketConnection(id string) *WebSocketConnection {
	for _, c := range m.wsConnections {
		if c.ID == id {
			return c
		}
	}
	return nil
}

func (m *mockEngine) CloseWebSocketConnection(id string) error {
	for i, c := range m.wsConnections {
		if c.ID == id {
			m.wsConnections = append(m.wsConnections[:i], m.wsConnections[i+1:]...)
			return nil
		}
	}
	return errors.New("connection not found")
}

func (m *mockEngine) GetWebSocketStats() *WebSocketStats {
	return m.wsStats
}

func (m *mockEngine) GetConfig() *ConfigResponse {
	return m.configResp
}

func (m *mockEngine) ListCustomOperations() []CustomOperationInfo {
	var ops []CustomOperationInfo
	for _, op := range m.customOps {
		ops = append(ops, CustomOperationInfo{Name: op.Name, StepCount: len(op.Steps)})
	}
	return ops
}

func (m *mockEngine) GetCustomOperation(name string) (*CustomOperationDetail, error) {
	op, ok := m.customOps[name]
	if !ok {
		return nil, errors.New("operation not found: " + name)
	}
	return op, nil
}

func (m *mockEngine) RegisterCustomOperation(cfg *config.CustomOperationConfig) error {
	steps := make([]CustomOperationStep, len(cfg.Steps))
	for i, s := range cfg.Steps {
		steps[i] = CustomOperationStep{
			Type:     s.Type,
			Resource: s.Resource,
			ID:       s.ID,
			As:       s.As,
			Set:      s.Set,
			Var:      s.Var,
			Value:    s.Value,
		}
	}
	m.customOps[cfg.Name] = &CustomOperationDetail{
		Name:     cfg.Name,
		Steps:    steps,
		Response: cfg.Response,
	}
	return nil
}

func (m *mockEngine) DeleteCustomOperation(name string) error {
	if _, ok := m.customOps[name]; !ok {
		return errors.New("operation not found: " + name)
	}
	delete(m.customOps, name)
	return nil
}

func (m *mockEngine) ExecuteCustomOperation(name string, input map[string]interface{}) (map[string]interface{}, error) {
	_, ok := m.customOps[name]
	if !ok {
		return nil, errors.New("operation not found: " + name)
	}
	// Simulate successful execution
	result := map[string]interface{}{
		"status": "completed",
	}
	for k, v := range input {
		result["input_"+k] = v
	}
	return result, nil
}

// boolPtr returns a pointer to a bool value.
func boolPtr(v bool) *bool { return &v }

// Helper to create a server with a mock engine
func newTestServer(engine *mockEngine) *Server {
	return NewServer(engine, 0)
}

// TestHandleHealth tests the GET /health handler.
func TestHandleHealth(t *testing.T) {
	t.Run("returns healthy status", func(t *testing.T) {
		engine := newMockEngine()
		server := newTestServer(engine)

		req := httptest.NewRequest(http.MethodGet, "/health", nil)
		rec := httptest.NewRecorder()

		server.handleHealth(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)

		var resp HealthResponse
		err := json.Unmarshal(rec.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.Equal(t, "healthy", resp.Status)
		assert.False(t, resp.Timestamp.IsZero())
	})
}

// TestHandleStatus tests the GET /status handler.
func TestHandleStatus(t *testing.T) {
	t.Run("returns running status with mocks", func(t *testing.T) {
		engine := newMockEngine()
		engine.mocks["mock-1"] = &config.MockConfiguration{ID: "mock-1", Name: "Test Mock", Enabled: boolPtr(true)}
		engine.mocks["mock-2"] = &config.MockConfiguration{ID: "mock-2", Name: "Test Mock 2", Enabled: boolPtr(true)}
		engine.requestLogs["req-1"] = &requestlog.Entry{ID: "req-1"}
		engine.requestLogs["req-2"] = &requestlog.Entry{ID: "req-2"}
		server := newTestServer(engine)

		req := httptest.NewRequest(http.MethodGet, "/status", nil)
		rec := httptest.NewRecorder()

		server.handleStatus(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)

		var resp StatusResponse
		err := json.Unmarshal(rec.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.Equal(t, "running", resp.Status)
		assert.Equal(t, 2, resp.MockCount)
		assert.Equal(t, int64(2), resp.RequestCount)
		assert.Equal(t, int64(100), resp.Uptime)
	})

	t.Run("returns stopped status when engine not running", func(t *testing.T) {
		engine := newMockEngine()
		engine.running = false
		server := newTestServer(engine)

		req := httptest.NewRequest(http.MethodGet, "/status", nil)
		rec := httptest.NewRecorder()

		server.handleStatus(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)

		var resp StatusResponse
		err := json.Unmarshal(rec.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.Equal(t, "stopped", resp.Status)
	})

	t.Run("includes protocol status", func(t *testing.T) {
		engine := newMockEngine()
		engine.protocols = map[string]ProtocolStatusInfo{
			"http": {Enabled: true, Port: 4280, Status: "running", Connections: 5},
			"grpc": {Enabled: true, Port: 9090, Status: "running"},
		}
		server := newTestServer(engine)

		req := httptest.NewRequest(http.MethodGet, "/status", nil)
		rec := httptest.NewRecorder()

		server.handleStatus(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)

		var resp StatusResponse
		err := json.Unmarshal(rec.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.Len(t, resp.Protocols, 2)
		assert.Equal(t, 4280, resp.Protocols["http"].Port)
		assert.Equal(t, 5, resp.Protocols["http"].Connections)
	})
}

// TestHandleCreateMock tests the POST /mocks handler.
func TestHandleCreateMock(t *testing.T) {
	t.Run("creates mock successfully", func(t *testing.T) {
		engine := newMockEngine()
		server := newTestServer(engine)

		mockData := map[string]interface{}{
			"id":      "new-mock",
			"name":    "New Mock",
			"enabled": true,
			"type":    "http",
			"http": map[string]interface{}{
				"matcher": map[string]interface{}{
					"method": "POST",
					"path":   "/api/users",
				},
				"response": map[string]interface{}{
					"statusCode": 201,
					"body":       `{"id": 1}`,
				},
			},
		}
		body, _ := json.Marshal(mockData)

		req := httptest.NewRequest(http.MethodPost, "/mocks", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		server.handleCreateMock(rec, req)

		assert.Equal(t, http.StatusCreated, rec.Code)

		var resp config.MockConfiguration
		err := json.Unmarshal(rec.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.Equal(t, "new-mock", resp.ID)
		assert.Equal(t, "New Mock", resp.Name)
	})

	t.Run("auto-generates ID when not provided", func(t *testing.T) {
		engine := newMockEngine()
		server := newTestServer(engine)

		mockData := map[string]interface{}{
			"name":    "Auto ID Mock",
			"enabled": true,
			"type":    "http",
		}
		body, _ := json.Marshal(mockData)

		req := httptest.NewRequest(http.MethodPost, "/mocks", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		server.handleCreateMock(rec, req)

		assert.Equal(t, http.StatusCreated, rec.Code)

		var resp config.MockConfiguration
		err := json.Unmarshal(rec.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.NotEmpty(t, resp.ID)
	})

	t.Run("returns 400 for invalid JSON", func(t *testing.T) {
		engine := newMockEngine()
		server := newTestServer(engine)

		req := httptest.NewRequest(http.MethodPost, "/mocks", bytes.NewReader([]byte("invalid json")))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		server.handleCreateMock(rec, req)

		assert.Equal(t, http.StatusBadRequest, rec.Code)

		var resp ErrorResponse
		err := json.Unmarshal(rec.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.Equal(t, "invalid_json", resp.Error)
	})

	t.Run("returns 409 for duplicate ID", func(t *testing.T) {
		engine := newMockEngine()
		engine.mocks["existing-mock"] = &config.MockConfiguration{
			ID:   "existing-mock",
			Name: "Existing Mock",
			Type: mock.TypeHTTP,
		}
		server := newTestServer(engine)

		mockData := map[string]interface{}{
			"id":   "existing-mock",
			"name": "Duplicate Mock",
			"type": "http",
		}
		body, _ := json.Marshal(mockData)

		req := httptest.NewRequest(http.MethodPost, "/mocks", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		server.handleCreateMock(rec, req)

		assert.Equal(t, http.StatusConflict, rec.Code)

		var resp ErrorResponse
		err := json.Unmarshal(rec.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.Equal(t, "duplicate_id", resp.Error)
	})

	t.Run("returns 400 for validation error", func(t *testing.T) {
		engine := newMockEngine()
		engine.addMockErr = errors.New("validation failed: missing required field")
		server := newTestServer(engine)

		mockData := map[string]interface{}{
			"id":      "invalid-mock",
			"name":    "Invalid Mock",
			"enabled": true,
		}
		body, _ := json.Marshal(mockData)

		req := httptest.NewRequest(http.MethodPost, "/mocks", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		server.handleCreateMock(rec, req)

		assert.Equal(t, http.StatusBadRequest, rec.Code)

		var resp ErrorResponse
		err := json.Unmarshal(rec.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.Equal(t, "validation_error", resp.Error)
	})

	t.Run("returns 413 for oversized body", func(t *testing.T) {
		engine := newMockEngine()
		server := newTestServer(engine)

		oversizedPayload := map[string]interface{}{
			"name": "oversized",
			"type": "http",
			"http": map[string]interface{}{
				"matcher": map[string]interface{}{"method": "GET", "path": "/x"},
				"response": map[string]interface{}{
					"statusCode": 200,
					"body":       strings.Repeat("a", maxRequestBodySize),
				},
			},
		}
		oversized, err := json.Marshal(oversizedPayload)
		require.NoError(t, err)
		req := httptest.NewRequest(http.MethodPost, "/mocks", bytes.NewReader(oversized))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		server.handleCreateMock(rec, req)

		assert.Equal(t, http.StatusRequestEntityTooLarge, rec.Code)
		var resp ErrorResponse
		err = json.Unmarshal(rec.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.Equal(t, "body_too_large", resp.Error)
	})
}

func TestHandleCreateStatefulItem_ErrorMapping(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		engineErr  error
		wantStatus int
		wantCode   string
	}{
		{
			name:       "maps not found",
			engineErr:  &stateful.NotFoundError{Resource: "users"},
			wantStatus: http.StatusNotFound,
			wantCode:   "not_found",
		},
		{
			name:       "maps conflict",
			engineErr:  &stateful.ConflictError{Resource: "users", ID: "u1"},
			wantStatus: http.StatusConflict,
			wantCode:   "conflict",
		},
		{
			name:       "maps capacity",
			engineErr:  &stateful.CapacityError{Resource: "users", MaxItems: 1},
			wantStatus: http.StatusInsufficientStorage,
			wantCode:   "capacity_exceeded",
		},
		{
			name:       "maps validation",
			engineErr:  &stateful.ValidationError{Message: "invalid email", Field: "email"},
			wantStatus: http.StatusBadRequest,
			wantCode:   "validation_error",
		},
		{
			name:       "maps legacy not found message",
			engineErr:  errors.New("stateful resource not found: users"),
			wantStatus: http.StatusNotFound,
			wantCode:   "not_found",
		},
		{
			name:       "fallback create failed",
			engineErr:  errors.New("boom"),
			wantStatus: http.StatusBadRequest,
			wantCode:   "create_failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			engine := newMockEngine()
			engine.createStatefulItemErr = tt.engineErr
			server := newTestServer(engine)

			req := httptest.NewRequest(http.MethodPost, "/state/resources/users/items", strings.NewReader(`{"id":"u1"}`))
			req.SetPathValue("name", "users")
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()

			server.handleCreateStatefulItem(rec, req)

			assert.Equal(t, tt.wantStatus, rec.Code)
			var resp ErrorResponse
			err := json.Unmarshal(rec.Body.Bytes(), &resp)
			require.NoError(t, err)
			assert.Equal(t, tt.wantCode, resp.Error)
		})
	}
}

func TestStatefulLookupHandlers_Return500ForInternalErrors(t *testing.T) {
	t.Parallel()

	t.Run("get state resource", func(t *testing.T) {
		engine := newMockEngine()
		engine.getStateResourceErr = errors.New("stateful store not initialized")
		server := newTestServer(engine)

		req := httptest.NewRequest(http.MethodGet, "/state/resources/users", nil)
		req.SetPathValue("name", "users")
		rec := httptest.NewRecorder()

		server.handleGetStateResource(rec, req)

		assert.Equal(t, http.StatusInternalServerError, rec.Code)
		var resp ErrorResponse
		err := json.Unmarshal(rec.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.Equal(t, "state_error", resp.Error)
	})

	t.Run("list stateful items", func(t *testing.T) {
		engine := newMockEngine()
		engine.listStatefulItemsErr = errors.New("stateful store not initialized")
		server := newTestServer(engine)

		req := httptest.NewRequest(http.MethodGet, "/state/resources/users/items", nil)
		req.SetPathValue("name", "users")
		rec := httptest.NewRecorder()

		server.handleListStatefulItems(rec, req)

		assert.Equal(t, http.StatusInternalServerError, rec.Code)
		var resp ErrorResponse
		err := json.Unmarshal(rec.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.Equal(t, "state_error", resp.Error)
	})
}

// TestHandleListMocks tests the GET /mocks handler.
func TestHandleListMocks(t *testing.T) {
	t.Run("returns empty list when no mocks", func(t *testing.T) {
		engine := newMockEngine()
		server := newTestServer(engine)

		req := httptest.NewRequest(http.MethodGet, "/mocks", nil)
		rec := httptest.NewRecorder()

		server.handleListMocks(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)

		var resp MockListResponse
		err := json.Unmarshal(rec.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.Equal(t, 0, resp.Count)
		assert.Empty(t, resp.Mocks)
	})

	t.Run("returns all mocks", func(t *testing.T) {
		engine := newMockEngine()
		engine.mocks["mock-1"] = &config.MockConfiguration{
			ID:      "mock-1",
			Name:    "Test Mock 1",
			Enabled: boolPtr(true),
			Type:    mock.TypeHTTP,
		}
		engine.mocks["mock-2"] = &config.MockConfiguration{
			ID:      "mock-2",
			Name:    "Test Mock 2",
			Enabled: boolPtr(false),
			Type:    mock.TypeHTTP,
		}
		server := newTestServer(engine)

		req := httptest.NewRequest(http.MethodGet, "/mocks", nil)
		rec := httptest.NewRecorder()

		server.handleListMocks(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)

		var resp MockListResponse
		err := json.Unmarshal(rec.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.Equal(t, 2, resp.Count)
		assert.Len(t, resp.Mocks, 2)
	})
}

// TestHandleGetMock tests the GET /mocks/{id} handler.
func TestHandleGetMock(t *testing.T) {
	t.Run("returns mock when found", func(t *testing.T) {
		engine := newMockEngine()
		testMock := &config.MockConfiguration{
			ID:      "mock-123",
			Name:    "Test Mock",
			Enabled: boolPtr(true),
			Type:    mock.TypeHTTP,
			HTTP: &mock.HTTPSpec{
				Matcher: &mock.HTTPMatcher{
					Method: "GET",
					Path:   "/api/test",
				},
				Response: &mock.HTTPResponse{
					StatusCode: 200,
					Body:       `{"status": "ok"}`,
				},
			},
		}
		engine.mocks["mock-123"] = testMock
		server := newTestServer(engine)

		req := httptest.NewRequest(http.MethodGet, "/mocks/mock-123", nil)
		req.SetPathValue("id", "mock-123")
		rec := httptest.NewRecorder()

		server.handleGetMock(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)

		var resp config.MockConfiguration
		err := json.Unmarshal(rec.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.Equal(t, "mock-123", resp.ID)
		assert.Equal(t, "Test Mock", resp.Name)
		assert.True(t, *resp.Enabled)
	})

	t.Run("returns 404 when mock not found", func(t *testing.T) {
		engine := newMockEngine()
		server := newTestServer(engine)

		req := httptest.NewRequest(http.MethodGet, "/mocks/nonexistent", nil)
		req.SetPathValue("id", "nonexistent")
		rec := httptest.NewRecorder()

		server.handleGetMock(rec, req)

		assert.Equal(t, http.StatusNotFound, rec.Code)

		var resp ErrorResponse
		err := json.Unmarshal(rec.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.Equal(t, "not_found", resp.Error)
		assert.Contains(t, resp.Message, "mock not found")
	})
}

// TestHandleUpdateMock tests the PUT /mocks/{id} handler.
func TestHandleUpdateMock(t *testing.T) {
	t.Run("updates mock successfully", func(t *testing.T) {
		engine := newMockEngine()
		engine.mocks["mock-to-update"] = &config.MockConfiguration{
			ID:      "mock-to-update",
			Name:    "Original Name",
			Enabled: boolPtr(true),
			Type:    mock.TypeHTTP,
		}
		server := newTestServer(engine)

		updateData := map[string]interface{}{
			"name":    "Updated Name",
			"enabled": false,
			"type":    "http",
		}
		body, _ := json.Marshal(updateData)

		req := httptest.NewRequest(http.MethodPut, "/mocks/mock-to-update", bytes.NewReader(body))
		req.SetPathValue("id", "mock-to-update")
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		server.handleUpdateMock(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)

		var resp config.MockConfiguration
		err := json.Unmarshal(rec.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.Equal(t, "mock-to-update", resp.ID)
		assert.Equal(t, "Updated Name", resp.Name)
		assert.False(t, *resp.Enabled)
	})

	t.Run("returns 404 for non-existent mock", func(t *testing.T) {
		engine := newMockEngine()
		server := newTestServer(engine)

		updateData := map[string]interface{}{
			"name": "Updated Name",
		}
		body, _ := json.Marshal(updateData)

		req := httptest.NewRequest(http.MethodPut, "/mocks/nonexistent", bytes.NewReader(body))
		req.SetPathValue("id", "nonexistent")
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		server.handleUpdateMock(rec, req)

		assert.Equal(t, http.StatusNotFound, rec.Code)

		var resp ErrorResponse
		err := json.Unmarshal(rec.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.Equal(t, "not_found", resp.Error)
	})

	t.Run("returns 400 for invalid JSON", func(t *testing.T) {
		engine := newMockEngine()
		engine.mocks["mock-to-update"] = &config.MockConfiguration{
			ID:   "mock-to-update",
			Name: "Original Name",
			Type: mock.TypeHTTP,
		}
		server := newTestServer(engine)

		req := httptest.NewRequest(http.MethodPut, "/mocks/mock-to-update", bytes.NewReader([]byte("invalid")))
		req.SetPathValue("id", "mock-to-update")
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		server.handleUpdateMock(rec, req)

		assert.Equal(t, http.StatusBadRequest, rec.Code)

		var resp ErrorResponse
		err := json.Unmarshal(rec.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.Equal(t, "invalid_json", resp.Error)
	})

	t.Run("returns 400 for validation error", func(t *testing.T) {
		engine := newMockEngine()
		engine.mocks["mock-to-update"] = &config.MockConfiguration{
			ID:   "mock-to-update",
			Name: "Original Name",
			Type: mock.TypeHTTP,
		}
		engine.updateMockErr = errors.New("validation failed")
		server := newTestServer(engine)

		updateData := map[string]interface{}{
			"name": "Updated Name",
		}
		body, _ := json.Marshal(updateData)

		req := httptest.NewRequest(http.MethodPut, "/mocks/mock-to-update", bytes.NewReader(body))
		req.SetPathValue("id", "mock-to-update")
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		server.handleUpdateMock(rec, req)

		assert.Equal(t, http.StatusBadRequest, rec.Code)

		var resp ErrorResponse
		err := json.Unmarshal(rec.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.Equal(t, "validation_error", resp.Error)
	})

	t.Run("ensures ID matches path parameter", func(t *testing.T) {
		engine := newMockEngine()
		engine.mocks["mock-123"] = &config.MockConfiguration{
			ID:   "mock-123",
			Name: "Original",
			Type: mock.TypeHTTP,
		}
		server := newTestServer(engine)

		// Request body has different ID than path
		updateData := map[string]interface{}{
			"id":   "different-id",
			"name": "Updated",
		}
		body, _ := json.Marshal(updateData)

		req := httptest.NewRequest(http.MethodPut, "/mocks/mock-123", bytes.NewReader(body))
		req.SetPathValue("id", "mock-123")
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		server.handleUpdateMock(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)

		// Verify the mock was updated with path ID, not body ID
		var resp config.MockConfiguration
		err := json.Unmarshal(rec.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.Equal(t, "mock-123", resp.ID)
	})
}

// TestHandleDeleteMock tests the DELETE /mocks/{id} handler.
func TestHandleDeleteMock(t *testing.T) {
	t.Run("deletes mock successfully", func(t *testing.T) {
		engine := newMockEngine()
		engine.mocks["mock-to-delete"] = &config.MockConfiguration{
			ID:   "mock-to-delete",
			Name: "To Be Deleted",
			Type: mock.TypeHTTP,
		}
		server := newTestServer(engine)

		req := httptest.NewRequest(http.MethodDelete, "/mocks/mock-to-delete", nil)
		req.SetPathValue("id", "mock-to-delete")
		rec := httptest.NewRecorder()

		server.handleDeleteMock(rec, req)

		assert.Equal(t, http.StatusNoContent, rec.Code)
		assert.Empty(t, rec.Body.String())

		// Verify mock was deleted
		assert.Nil(t, engine.mocks["mock-to-delete"])
	})

	t.Run("returns 404 for non-existent mock", func(t *testing.T) {
		engine := newMockEngine()
		server := newTestServer(engine)

		req := httptest.NewRequest(http.MethodDelete, "/mocks/nonexistent", nil)
		req.SetPathValue("id", "nonexistent")
		rec := httptest.NewRecorder()

		server.handleDeleteMock(rec, req)

		assert.Equal(t, http.StatusNotFound, rec.Code)

		var resp ErrorResponse
		err := json.Unmarshal(rec.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.Equal(t, "not_found", resp.Error)
	})
}

// TestHandleToggleMock tests the POST /mocks/{id}/toggle handler.
func TestHandleToggleMock(t *testing.T) {
	t.Run("toggles mock to enabled", func(t *testing.T) {
		engine := newMockEngine()
		engine.mocks["mock-toggle"] = &config.MockConfiguration{
			ID:      "mock-toggle",
			Name:    "Toggle Mock",
			Enabled: boolPtr(false),
			Type:    mock.TypeHTTP,
		}
		server := newTestServer(engine)

		body := `{"enabled": true}`
		req := httptest.NewRequest(http.MethodPost, "/mocks/mock-toggle/toggle", bytes.NewReader([]byte(body)))
		req.SetPathValue("id", "mock-toggle")
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		server.handleToggleMock(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)

		var resp config.MockConfiguration
		err := json.Unmarshal(rec.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.True(t, *resp.Enabled)
	})

	t.Run("toggles mock to disabled", func(t *testing.T) {
		engine := newMockEngine()
		engine.mocks["mock-toggle"] = &config.MockConfiguration{
			ID:      "mock-toggle",
			Name:    "Toggle Mock",
			Enabled: boolPtr(true),
			Type:    mock.TypeHTTP,
		}
		server := newTestServer(engine)

		body := `{"enabled": false}`
		req := httptest.NewRequest(http.MethodPost, "/mocks/mock-toggle/toggle", bytes.NewReader([]byte(body)))
		req.SetPathValue("id", "mock-toggle")
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		server.handleToggleMock(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)

		var resp config.MockConfiguration
		err := json.Unmarshal(rec.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.False(t, *resp.Enabled)
	})

	t.Run("returns 404 for non-existent mock", func(t *testing.T) {
		engine := newMockEngine()
		server := newTestServer(engine)

		body := `{"enabled": true}`
		req := httptest.NewRequest(http.MethodPost, "/mocks/nonexistent/toggle", bytes.NewReader([]byte(body)))
		req.SetPathValue("id", "nonexistent")
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		server.handleToggleMock(rec, req)

		assert.Equal(t, http.StatusNotFound, rec.Code)
	})

	t.Run("returns 400 for invalid JSON", func(t *testing.T) {
		engine := newMockEngine()
		engine.mocks["mock-toggle"] = &config.MockConfiguration{
			ID:   "mock-toggle",
			Name: "Toggle Mock",
			Type: mock.TypeHTTP,
		}
		server := newTestServer(engine)

		req := httptest.NewRequest(http.MethodPost, "/mocks/mock-toggle/toggle", bytes.NewReader([]byte("invalid")))
		req.SetPathValue("id", "mock-toggle")
		rec := httptest.NewRecorder()

		server.handleToggleMock(rec, req)

		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})
}

// TestHandleDeploy tests the POST /deploy handler.
func TestHandleDeploy(t *testing.T) {
	t.Run("deploys mocks successfully", func(t *testing.T) {
		engine := newMockEngine()
		server := newTestServer(engine)

		deployData := DeployRequest{
			Mocks: []*config.MockConfiguration{
				{ID: "mock-1", Name: "Mock 1", Type: mock.TypeHTTP},
				{ID: "mock-2", Name: "Mock 2", Type: mock.TypeHTTP},
			},
			Replace: false,
		}
		body, _ := json.Marshal(deployData)

		req := httptest.NewRequest(http.MethodPost, "/deploy", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		server.handleDeploy(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)

		var resp DeployResponse
		err := json.Unmarshal(rec.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.Equal(t, 2, resp.Deployed)
		assert.Contains(t, resp.Message, "2 mocks")
	})

	t.Run("replaces existing mocks when replace=true", func(t *testing.T) {
		engine := newMockEngine()
		engine.mocks["existing"] = &config.MockConfiguration{ID: "existing", Name: "Existing"}
		server := newTestServer(engine)

		deployData := DeployRequest{
			Mocks: []*config.MockConfiguration{
				{ID: "new-mock", Name: "New Mock", Type: mock.TypeHTTP},
			},
			Replace: true,
		}
		body, _ := json.Marshal(deployData)

		req := httptest.NewRequest(http.MethodPost, "/deploy", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		server.handleDeploy(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)

		// Verify existing mock was cleared
		assert.Nil(t, engine.mocks["existing"])
		assert.NotNil(t, engine.mocks["new-mock"])
	})

	t.Run("returns 400 for invalid JSON", func(t *testing.T) {
		engine := newMockEngine()
		server := newTestServer(engine)

		req := httptest.NewRequest(http.MethodPost, "/deploy", bytes.NewReader([]byte("invalid")))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		server.handleDeploy(rec, req)

		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})
}

// TestHandleUndeploy tests the DELETE /deploy handler.
func TestHandleUndeploy(t *testing.T) {
	t.Run("removes all mocks", func(t *testing.T) {
		engine := newMockEngine()
		engine.mocks["mock-1"] = &config.MockConfiguration{ID: "mock-1"}
		engine.mocks["mock-2"] = &config.MockConfiguration{ID: "mock-2"}
		server := newTestServer(engine)

		req := httptest.NewRequest(http.MethodDelete, "/deploy", nil)
		rec := httptest.NewRecorder()

		server.handleUndeploy(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Empty(t, engine.mocks)
	})
}

// TestHandleListRequests tests the GET /requests handler.
func TestHandleListRequests(t *testing.T) {
	t.Run("returns empty list when no requests", func(t *testing.T) {
		engine := newMockEngine()
		server := newTestServer(engine)

		req := httptest.NewRequest(http.MethodGet, "/requests", nil)
		rec := httptest.NewRecorder()

		server.handleListRequests(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)

		var resp RequestListResponse
		err := json.Unmarshal(rec.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.Equal(t, 0, resp.Count)
		assert.Equal(t, 0, resp.Total)
	})

	t.Run("returns request logs", func(t *testing.T) {
		engine := newMockEngine()
		engine.requestLogs["req-1"] = &requestlog.Entry{
			ID:             "req-1",
			Timestamp:      time.Now(),
			Protocol:       "http",
			Method:         "GET",
			Path:           "/api/test",
			MatchedMockID:  "mock-1",
			ResponseStatus: 200,
			DurationMs:     15,
		}
		server := newTestServer(engine)

		req := httptest.NewRequest(http.MethodGet, "/requests", nil)
		rec := httptest.NewRecorder()

		server.handleListRequests(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)

		var resp RequestListResponse
		err := json.Unmarshal(rec.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.Equal(t, 1, resp.Count)
		assert.Equal(t, 1, resp.Total)
	})

	t.Run("respects limit query param", func(t *testing.T) {
		engine := newMockEngine()
		for i := 0; i < 10; i++ {
			engine.requestLogs[string(rune('0'+i))] = &requestlog.Entry{
				ID: string(rune('0' + i)),
			}
		}
		server := newTestServer(engine)

		req := httptest.NewRequest(http.MethodGet, "/requests?limit=5", nil)
		rec := httptest.NewRecorder()

		server.handleListRequests(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)

		var resp RequestListResponse
		err := json.Unmarshal(rec.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.Equal(t, 5, resp.Count)
		assert.Equal(t, 10, resp.Total)
	})

	t.Run("filters hasError when valid boolean", func(t *testing.T) {
		engine := newMockEngine()
		engine.requestLogs["ok"] = &requestlog.Entry{ID: "ok"}
		engine.requestLogs["err"] = &requestlog.Entry{ID: "err", Error: "boom"}
		server := newTestServer(engine)

		req := httptest.NewRequest(http.MethodGet, "/requests?hasError=true", nil)
		rec := httptest.NewRecorder()

		server.handleListRequests(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)

		var resp RequestListResponse
		err := json.Unmarshal(rec.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.Equal(t, 1, resp.Count)
		assert.Equal(t, "err", resp.Requests[0].ID)
	})

	t.Run("ignores invalid hasError value", func(t *testing.T) {
		engine := newMockEngine()
		engine.requestLogs["ok"] = &requestlog.Entry{ID: "ok"}
		engine.requestLogs["err"] = &requestlog.Entry{ID: "err", Error: "boom"}
		server := newTestServer(engine)

		req := httptest.NewRequest(http.MethodGet, "/requests?hasError=maybe", nil)
		rec := httptest.NewRecorder()

		server.handleListRequests(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)

		var resp RequestListResponse
		err := json.Unmarshal(rec.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.Equal(t, 2, resp.Count)
	})
}

// TestHandleListRequests_MatchedFilter tests filtering requests by matched mock ID.
func TestHandleListRequests_MatchedFilter(t *testing.T) {
	t.Run("filters by matched mock ID", func(t *testing.T) {
		engine := newMockEngine()
		engine.requestLogs["req-1"] = &requestlog.Entry{
			ID:             "req-1",
			Timestamp:      time.Now(),
			Protocol:       "http",
			Method:         "GET",
			Path:           "/api/users",
			MatchedMockID:  "mock-1",
			ResponseStatus: 200,
			DurationMs:     10,
		}
		engine.requestLogs["req-2"] = &requestlog.Entry{
			ID:             "req-2",
			Timestamp:      time.Now(),
			Protocol:       "http",
			Method:         "POST",
			Path:           "/api/orders",
			MatchedMockID:  "mock-2",
			ResponseStatus: 201,
			DurationMs:     20,
		}
		engine.requestLogs["req-3"] = &requestlog.Entry{
			ID:             "req-3",
			Timestamp:      time.Now(),
			Protocol:       "http",
			Method:         "GET",
			Path:           "/api/users/1",
			MatchedMockID:  "mock-1",
			ResponseStatus: 200,
			DurationMs:     5,
		}
		server := newTestServer(engine)

		req := httptest.NewRequest(http.MethodGet, "/requests?matched=mock-1", nil)
		rec := httptest.NewRecorder()

		server.handleListRequests(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)

		var resp RequestListResponse
		err := json.Unmarshal(rec.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.Equal(t, 2, resp.Count)
		assert.Equal(t, 3, resp.Total)
		for _, r := range resp.Requests {
			assert.Equal(t, "mock-1", r.MatchedMockID)
		}
	})

	t.Run("returns empty list when no requests match mock ID", func(t *testing.T) {
		engine := newMockEngine()
		engine.requestLogs["req-1"] = &requestlog.Entry{
			ID:            "req-1",
			MatchedMockID: "mock-1",
		}
		server := newTestServer(engine)

		req := httptest.NewRequest(http.MethodGet, "/requests?matched=nonexistent", nil)
		rec := httptest.NewRecorder()

		server.handleListRequests(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)

		var resp RequestListResponse
		err := json.Unmarshal(rec.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.Equal(t, 0, resp.Count)
		assert.Equal(t, 1, resp.Total)
		assert.Empty(t, resp.Requests)
	})

	t.Run("filters by method", func(t *testing.T) {
		engine := newMockEngine()
		engine.requestLogs["req-1"] = &requestlog.Entry{
			ID:     "req-1",
			Method: "GET",
			Path:   "/api/users",
		}
		engine.requestLogs["req-2"] = &requestlog.Entry{
			ID:     "req-2",
			Method: "POST",
			Path:   "/api/users",
		}
		server := newTestServer(engine)

		req := httptest.NewRequest(http.MethodGet, "/requests?method=GET", nil)
		rec := httptest.NewRecorder()

		server.handleListRequests(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)

		var resp RequestListResponse
		err := json.Unmarshal(rec.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.Equal(t, 1, resp.Count)
		for _, r := range resp.Requests {
			assert.Equal(t, "GET", r.Method)
		}
	})

	t.Run("filters by path substring", func(t *testing.T) {
		engine := newMockEngine()
		engine.requestLogs["req-1"] = &requestlog.Entry{
			ID:   "req-1",
			Path: "/api/users",
		}
		engine.requestLogs["req-2"] = &requestlog.Entry{
			ID:   "req-2",
			Path: "/api/orders",
		}
		engine.requestLogs["req-3"] = &requestlog.Entry{
			ID:   "req-3",
			Path: "/api/users/1",
		}
		server := newTestServer(engine)

		req := httptest.NewRequest(http.MethodGet, "/requests?path=users", nil)
		rec := httptest.NewRecorder()

		server.handleListRequests(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)

		var resp RequestListResponse
		err := json.Unmarshal(rec.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.Equal(t, 2, resp.Count)
		for _, r := range resp.Requests {
			assert.Contains(t, r.Path, "users")
		}
	})

	t.Run("combines matched and method filters", func(t *testing.T) {
		engine := newMockEngine()
		engine.requestLogs["req-1"] = &requestlog.Entry{
			ID:            "req-1",
			Method:        "GET",
			MatchedMockID: "mock-1",
		}
		engine.requestLogs["req-2"] = &requestlog.Entry{
			ID:            "req-2",
			Method:        "POST",
			MatchedMockID: "mock-1",
		}
		engine.requestLogs["req-3"] = &requestlog.Entry{
			ID:            "req-3",
			Method:        "GET",
			MatchedMockID: "mock-2",
		}
		server := newTestServer(engine)

		req := httptest.NewRequest(http.MethodGet, "/requests?matched=mock-1&method=GET", nil)
		rec := httptest.NewRecorder()

		server.handleListRequests(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)

		var resp RequestListResponse
		err := json.Unmarshal(rec.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.Equal(t, 1, resp.Count)
		assert.Equal(t, "mock-1", resp.Requests[0].MatchedMockID)
		assert.Equal(t, "GET", resp.Requests[0].Method)
	})
}

// TestHandleGetRequest tests the GET /requests/{id} handler.
func TestHandleGetRequest(t *testing.T) {
	t.Run("returns request when found", func(t *testing.T) {
		engine := newMockEngine()
		engine.requestLogs["req-123"] = &requestlog.Entry{
			ID:             "req-123",
			Timestamp:      time.Now(),
			Protocol:       "http",
			Method:         "POST",
			Path:           "/api/users",
			Headers:        map[string][]string{"Content-Type": {"application/json"}},
			Body:           `{"name": "test"}`,
			MatchedMockID:  "mock-1",
			ResponseStatus: 201,
			DurationMs:     25,
		}
		server := newTestServer(engine)

		req := httptest.NewRequest(http.MethodGet, "/requests/req-123", nil)
		req.SetPathValue("id", "req-123")
		rec := httptest.NewRecorder()

		server.handleGetRequest(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)

		var resp RequestLogEntry
		err := json.Unmarshal(rec.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.Equal(t, "req-123", resp.ID)
		assert.Equal(t, "POST", resp.Method)
		assert.Equal(t, "/api/users", resp.Path)
		assert.Equal(t, 201, resp.StatusCode)
	})

	t.Run("returns 404 when request not found", func(t *testing.T) {
		engine := newMockEngine()
		server := newTestServer(engine)

		req := httptest.NewRequest(http.MethodGet, "/requests/nonexistent", nil)
		req.SetPathValue("id", "nonexistent")
		rec := httptest.NewRecorder()

		server.handleGetRequest(rec, req)

		assert.Equal(t, http.StatusNotFound, rec.Code)

		var resp ErrorResponse
		err := json.Unmarshal(rec.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.Equal(t, "not_found", resp.Error)
	})
}

// TestHandleClearRequests tests the DELETE /requests handler.
func TestHandleClearRequests(t *testing.T) {
	t.Run("clears all request logs", func(t *testing.T) {
		engine := newMockEngine()
		engine.requestLogs["req-1"] = &requestlog.Entry{ID: "req-1"}
		engine.requestLogs["req-2"] = &requestlog.Entry{ID: "req-2"}
		server := newTestServer(engine)

		req := httptest.NewRequest(http.MethodDelete, "/requests", nil)
		rec := httptest.NewRecorder()

		server.handleClearRequests(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Empty(t, engine.requestLogs)

		var resp map[string]interface{}
		err := json.Unmarshal(rec.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.Equal(t, float64(2), resp["cleared"])
	})
}

// TestHandleExportMocks tests the GET /export handler.
func TestHandleExportMocks(t *testing.T) {
	t.Run("exports all mocks", func(t *testing.T) {
		engine := newMockEngine()
		engine.mocks["mock-1"] = &config.MockConfiguration{
			ID:      "mock-1",
			Name:    "Export Test Mock",
			Enabled: boolPtr(true),
			Type:    mock.TypeHTTP,
		}
		server := newTestServer(engine)

		req := httptest.NewRequest(http.MethodGet, "/export", nil)
		rec := httptest.NewRecorder()

		server.handleExportMocks(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)

		var resp config.MockCollection
		err := json.Unmarshal(rec.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.Equal(t, "1.0", resp.Version)
		assert.Len(t, resp.Mocks, 1)
		assert.Equal(t, "mock-1", resp.Mocks[0].ID)
	})

	t.Run("uses custom name from query param", func(t *testing.T) {
		engine := newMockEngine()
		server := newTestServer(engine)

		req := httptest.NewRequest(http.MethodGet, "/export?name=my-collection", nil)
		rec := httptest.NewRecorder()

		server.handleExportMocks(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)

		var resp config.MockCollection
		err := json.Unmarshal(rec.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.Equal(t, "my-collection", resp.Name)
	})
}

// TestHandleImportConfig tests the POST /config handler.
func TestHandleImportConfig(t *testing.T) {
	t.Run("imports mocks successfully", func(t *testing.T) {
		engine := newMockEngine()
		server := newTestServer(engine)

		importData := ImportConfigRequest{
			Config: &config.MockCollection{
				Version: "1.0",
				Mocks: []*config.MockConfiguration{
					{ID: "imported-1", Name: "Imported Mock 1", Type: mock.TypeHTTP},
					{ID: "imported-2", Name: "Imported Mock 2", Type: mock.TypeHTTP},
				},
			},
			Replace: false,
		}
		body, _ := json.Marshal(importData)

		req := httptest.NewRequest(http.MethodPost, "/config", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		server.handleImportConfig(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)

		var resp map[string]interface{}
		err := json.Unmarshal(rec.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.Equal(t, float64(2), resp["imported"])
	})

	t.Run("replaces existing mocks when replace=true", func(t *testing.T) {
		engine := newMockEngine()
		engine.mocks["existing"] = &config.MockConfiguration{ID: "existing"}
		server := newTestServer(engine)

		importData := ImportConfigRequest{
			Config: &config.MockCollection{
				Version: "1.0",
				Mocks: []*config.MockConfiguration{
					{ID: "new", Name: "New Mock", Type: mock.TypeHTTP},
				},
			},
			Replace: true,
		}
		body, _ := json.Marshal(importData)

		req := httptest.NewRequest(http.MethodPost, "/config", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		server.handleImportConfig(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Nil(t, engine.mocks["existing"])
		assert.NotNil(t, engine.mocks["new"])
	})

	t.Run("returns 400 for missing config", func(t *testing.T) {
		engine := newMockEngine()
		server := newTestServer(engine)

		importData := ImportConfigRequest{
			Config:  nil,
			Replace: false,
		}
		body, _ := json.Marshal(importData)

		req := httptest.NewRequest(http.MethodPost, "/config", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		server.handleImportConfig(rec, req)

		assert.Equal(t, http.StatusBadRequest, rec.Code)

		var resp ErrorResponse
		err := json.Unmarshal(rec.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.Equal(t, "invalid_request", resp.Error)
	})

	t.Run("returns 400 for invalid JSON", func(t *testing.T) {
		engine := newMockEngine()
		server := newTestServer(engine)

		req := httptest.NewRequest(http.MethodPost, "/config", bytes.NewReader([]byte("invalid")))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		server.handleImportConfig(rec, req)

		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})
}

// TestWriteJSONHelper tests the writeJSON helper function.
func TestWriteJSONHelper(t *testing.T) {
	t.Run("writes JSON response", func(t *testing.T) {
		rec := httptest.NewRecorder()

		data := map[string]string{"message": "hello"}
		writeJSON(rec, http.StatusOK, data)

		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Contains(t, rec.Body.String(), "hello")
	})

	t.Run("sets correct status code", func(t *testing.T) {
		rec := httptest.NewRecorder()

		writeJSON(rec, http.StatusCreated, map[string]string{"status": "created"})

		assert.Equal(t, http.StatusCreated, rec.Code)
	})
}

// TestWriteErrorHelper tests the writeError helper function.
func TestWriteErrorHelper(t *testing.T) {
	t.Run("writes error response", func(t *testing.T) {
		rec := httptest.NewRecorder()

		writeError(rec, http.StatusBadRequest, "test_error", "Test error message")

		assert.Equal(t, http.StatusBadRequest, rec.Code)

		var resp ErrorResponse
		err := json.Unmarshal(rec.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.Equal(t, "test_error", resp.Error)
		assert.Equal(t, "Test error message", resp.Message)
	})

	t.Run("handles different status codes", func(t *testing.T) {
		testCases := []struct {
			name       string
			statusCode int
			errorCode  string
		}{
			{"not found", http.StatusNotFound, "not_found"},
			{"conflict", http.StatusConflict, "duplicate"},
			{"internal error", http.StatusInternalServerError, "internal_error"},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				rec := httptest.NewRecorder()
				writeError(rec, tc.statusCode, tc.errorCode, "error message")
				assert.Equal(t, tc.statusCode, rec.Code)
			})
		}
	})
}

// TestServerIntegration tests full request/response cycle through the server.
func TestServerIntegration(t *testing.T) {
	t.Run("full CRUD cycle", func(t *testing.T) {
		engine := newMockEngine()
		server := newTestServer(engine)

		// Create
		createData := map[string]interface{}{
			"id":      "crud-test",
			"name":    "CRUD Test Mock",
			"enabled": true,
			"type":    "http",
		}
		createBody, _ := json.Marshal(createData)
		createReq := httptest.NewRequest(http.MethodPost, "/mocks", bytes.NewReader(createBody))
		createReq.Header.Set("Content-Type", "application/json")
		createRec := httptest.NewRecorder()
		server.handleCreateMock(createRec, createReq)
		assert.Equal(t, http.StatusCreated, createRec.Code)

		// Read
		readReq := httptest.NewRequest(http.MethodGet, "/mocks/crud-test", nil)
		readReq.SetPathValue("id", "crud-test")
		readRec := httptest.NewRecorder()
		server.handleGetMock(readRec, readReq)
		assert.Equal(t, http.StatusOK, readRec.Code)

		// Update
		updateData := map[string]interface{}{
			"name":    "Updated CRUD Test Mock",
			"enabled": false,
		}
		updateBody, _ := json.Marshal(updateData)
		updateReq := httptest.NewRequest(http.MethodPut, "/mocks/crud-test", bytes.NewReader(updateBody))
		updateReq.SetPathValue("id", "crud-test")
		updateReq.Header.Set("Content-Type", "application/json")
		updateRec := httptest.NewRecorder()
		server.handleUpdateMock(updateRec, updateReq)
		assert.Equal(t, http.StatusOK, updateRec.Code)

		// Verify update
		var updated config.MockConfiguration
		json.Unmarshal(updateRec.Body.Bytes(), &updated)
		assert.Equal(t, "Updated CRUD Test Mock", updated.Name)
		assert.False(t, *updated.Enabled)

		// Delete
		deleteReq := httptest.NewRequest(http.MethodDelete, "/mocks/crud-test", nil)
		deleteReq.SetPathValue("id", "crud-test")
		deleteRec := httptest.NewRecorder()
		server.handleDeleteMock(deleteRec, deleteReq)
		assert.Equal(t, http.StatusNoContent, deleteRec.Code)

		// Verify deletion
		verifyReq := httptest.NewRequest(http.MethodGet, "/mocks/crud-test", nil)
		verifyReq.SetPathValue("id", "crud-test")
		verifyRec := httptest.NewRecorder()
		server.handleGetMock(verifyRec, verifyReq)
		assert.Equal(t, http.StatusNotFound, verifyRec.Code)
	})
}

// --- Custom Operation Handler Tests ---

func TestHandleListCustomOperations(t *testing.T) {
	t.Run("returns empty list when no operations", func(t *testing.T) {
		engine := newMockEngine()
		server := newTestServer(engine)

		req := httptest.NewRequest(http.MethodGet, "/state/operations", nil)
		rec := httptest.NewRecorder()

		server.handleListCustomOperations(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		var result map[string]interface{}
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &result))
		assert.Equal(t, float64(0), result["count"])
		ops, ok := result["operations"].([]interface{})
		require.True(t, ok)
		assert.Empty(t, ops)
	})

	t.Run("returns registered operations", func(t *testing.T) {
		engine := newMockEngine()
		server := newTestServer(engine)

		// Register an operation
		engine.customOps["TransferFunds"] = &CustomOperationDetail{
			Name: "TransferFunds",
			Steps: []CustomOperationStep{
				{Type: "read", Resource: "accounts", ID: "input.sourceId", As: "source"},
				{Type: "read", Resource: "accounts", ID: "input.destId", As: "dest"},
			},
			Response: map[string]string{"status": `"completed"`},
		}

		req := httptest.NewRequest(http.MethodGet, "/state/operations", nil)
		rec := httptest.NewRecorder()

		server.handleListCustomOperations(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		var result map[string]interface{}
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &result))
		assert.Equal(t, float64(1), result["count"])
	})
}

func TestHandleGetCustomOperation(t *testing.T) {
	t.Run("returns operation details", func(t *testing.T) {
		engine := newMockEngine()
		server := newTestServer(engine)

		engine.customOps["TransferFunds"] = &CustomOperationDetail{
			Name: "TransferFunds",
			Steps: []CustomOperationStep{
				{Type: "read", Resource: "accounts", ID: "input.sourceId", As: "source"},
			},
			Response: map[string]string{"status": `"done"`},
		}

		req := httptest.NewRequest(http.MethodGet, "/state/operations/TransferFunds", nil)
		req.SetPathValue("name", "TransferFunds")
		rec := httptest.NewRecorder()

		server.handleGetCustomOperation(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		var result CustomOperationDetail
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &result))
		assert.Equal(t, "TransferFunds", result.Name)
		assert.Len(t, result.Steps, 1)
		assert.Equal(t, "read", result.Steps[0].Type)
	})

	t.Run("returns 404 for missing operation", func(t *testing.T) {
		engine := newMockEngine()
		server := newTestServer(engine)

		req := httptest.NewRequest(http.MethodGet, "/state/operations/NonExistent", nil)
		req.SetPathValue("name", "NonExistent")
		rec := httptest.NewRecorder()

		server.handleGetCustomOperation(rec, req)

		assert.Equal(t, http.StatusNotFound, rec.Code)
	})
}

func TestHandleRegisterCustomOperation(t *testing.T) {
	t.Run("registers a valid operation", func(t *testing.T) {
		engine := newMockEngine()
		server := newTestServer(engine)

		body := `{
			"name": "TransferFunds",
			"steps": [
				{"type": "read", "resource": "accounts", "id": "input.sourceId", "as": "source"},
				{"type": "update", "resource": "accounts", "id": "input.sourceId", "set": {"balance": "source.balance - input.amount"}}
			],
			"response": {"status": "\"completed\""}
		}`

		req := httptest.NewRequest(http.MethodPost, "/state/operations", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		server.handleRegisterCustomOperation(rec, req)

		assert.Equal(t, http.StatusCreated, rec.Code)
		var result map[string]interface{}
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &result))
		assert.Equal(t, "TransferFunds", result["name"])
		assert.Equal(t, "custom operation registered", result["message"])

		// Verify it was actually stored
		_, ok := engine.customOps["TransferFunds"]
		assert.True(t, ok)
	})

	t.Run("rejects empty name", func(t *testing.T) {
		engine := newMockEngine()
		server := newTestServer(engine)

		body := `{"name": "", "steps": [{"type": "read", "resource": "accounts", "id": "1", "as": "x"}]}`
		req := httptest.NewRequest(http.MethodPost, "/state/operations", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		server.handleRegisterCustomOperation(rec, req)

		assert.Equal(t, http.StatusBadRequest, rec.Code)
		assert.Contains(t, rec.Body.String(), "operation name is required")
	})

	t.Run("rejects no steps", func(t *testing.T) {
		engine := newMockEngine()
		server := newTestServer(engine)

		body := `{"name": "EmptyOp", "steps": []}`
		req := httptest.NewRequest(http.MethodPost, "/state/operations", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		server.handleRegisterCustomOperation(rec, req)

		assert.Equal(t, http.StatusBadRequest, rec.Code)
		assert.Contains(t, rec.Body.String(), "at least one step")
	})

	t.Run("rejects invalid JSON", func(t *testing.T) {
		engine := newMockEngine()
		server := newTestServer(engine)

		req := httptest.NewRequest(http.MethodPost, "/state/operations", strings.NewReader(`{invalid`))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		server.handleRegisterCustomOperation(rec, req)

		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})
}

func TestHandleDeleteCustomOperation(t *testing.T) {
	t.Run("deletes existing operation", func(t *testing.T) {
		engine := newMockEngine()
		server := newTestServer(engine)

		engine.customOps["ToDelete"] = &CustomOperationDetail{
			Name:  "ToDelete",
			Steps: []CustomOperationStep{{Type: "set", As: "x", Value: "1"}},
		}

		req := httptest.NewRequest(http.MethodDelete, "/state/operations/ToDelete", nil)
		req.SetPathValue("name", "ToDelete")
		rec := httptest.NewRecorder()

		server.handleDeleteCustomOperation(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		var result map[string]interface{}
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &result))
		assert.Equal(t, "ToDelete", result["name"])
		assert.Equal(t, "custom operation deleted", result["message"])

		// Verify it was removed
		_, ok := engine.customOps["ToDelete"]
		assert.False(t, ok)
	})

	t.Run("returns 404 for missing operation", func(t *testing.T) {
		engine := newMockEngine()
		server := newTestServer(engine)

		req := httptest.NewRequest(http.MethodDelete, "/state/operations/NonExistent", nil)
		req.SetPathValue("name", "NonExistent")
		rec := httptest.NewRecorder()

		server.handleDeleteCustomOperation(rec, req)

		assert.Equal(t, http.StatusNotFound, rec.Code)
	})
}

func TestHandleExecuteCustomOperation(t *testing.T) {
	t.Run("executes operation with input", func(t *testing.T) {
		engine := newMockEngine()
		server := newTestServer(engine)

		engine.customOps["TransferFunds"] = &CustomOperationDetail{
			Name:  "TransferFunds",
			Steps: []CustomOperationStep{{Type: "read", Resource: "accounts", ID: "input.sourceId", As: "source"}},
		}

		body := `{"sourceId": "acct-1", "amount": 100}`
		req := httptest.NewRequest(http.MethodPost, "/state/operations/TransferFunds/execute", strings.NewReader(body))
		req.SetPathValue("name", "TransferFunds")
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		server.handleExecuteCustomOperation(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		var result map[string]interface{}
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &result))
		assert.Equal(t, "completed", result["status"])
		assert.Equal(t, "acct-1", result["input_sourceId"])
	})

	t.Run("executes operation with empty body", func(t *testing.T) {
		engine := newMockEngine()
		server := newTestServer(engine)

		engine.customOps["SimpleOp"] = &CustomOperationDetail{
			Name:  "SimpleOp",
			Steps: []CustomOperationStep{{Type: "set", As: "x", Value: "1"}},
		}

		req := httptest.NewRequest(http.MethodPost, "/state/operations/SimpleOp/execute", nil)
		req.SetPathValue("name", "SimpleOp")
		rec := httptest.NewRecorder()

		server.handleExecuteCustomOperation(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
	})

	t.Run("returns error for non-existent operation", func(t *testing.T) {
		engine := newMockEngine()
		server := newTestServer(engine)

		body := `{"x": 1}`
		req := httptest.NewRequest(http.MethodPost, "/state/operations/Missing/execute", strings.NewReader(body))
		req.SetPathValue("name", "Missing")
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		server.handleExecuteCustomOperation(rec, req)

		// Should get an error status (not 200)
		assert.NotEqual(t, http.StatusOK, rec.Code)
	})
}

// TestCustomOperationFullCRUD tests the complete lifecycle of a custom operation.
func TestCustomOperationFullCRUD(t *testing.T) {
	engine := newMockEngine()
	server := newTestServer(engine)

	// 1. List — should be empty
	listReq := httptest.NewRequest(http.MethodGet, "/state/operations", nil)
	listRec := httptest.NewRecorder()
	server.handleListCustomOperations(listRec, listReq)
	assert.Equal(t, http.StatusOK, listRec.Code)
	var listResult map[string]interface{}
	json.Unmarshal(listRec.Body.Bytes(), &listResult)
	assert.Equal(t, float64(0), listResult["count"])

	// 2. Register
	registerBody := `{"name":"TestOp","steps":[{"type":"set","as":"x","value":"42"}],"response":{"result":"string(x)"}}`
	regReq := httptest.NewRequest(http.MethodPost, "/state/operations", strings.NewReader(registerBody))
	regReq.Header.Set("Content-Type", "application/json")
	regRec := httptest.NewRecorder()
	server.handleRegisterCustomOperation(regRec, regReq)
	assert.Equal(t, http.StatusCreated, regRec.Code)

	// 3. Get — should exist
	getReq := httptest.NewRequest(http.MethodGet, "/state/operations/TestOp", nil)
	getReq.SetPathValue("name", "TestOp")
	getRec := httptest.NewRecorder()
	server.handleGetCustomOperation(getRec, getReq)
	assert.Equal(t, http.StatusOK, getRec.Code)
	var detail CustomOperationDetail
	json.Unmarshal(getRec.Body.Bytes(), &detail)
	assert.Equal(t, "TestOp", detail.Name)
	assert.Len(t, detail.Steps, 1)

	// 4. Execute
	execReq := httptest.NewRequest(http.MethodPost, "/state/operations/TestOp/execute", strings.NewReader(`{}`))
	execReq.SetPathValue("name", "TestOp")
	execReq.Header.Set("Content-Type", "application/json")
	execRec := httptest.NewRecorder()
	server.handleExecuteCustomOperation(execRec, execReq)
	assert.Equal(t, http.StatusOK, execRec.Code)

	// 5. List — should have 1
	listReq2 := httptest.NewRequest(http.MethodGet, "/state/operations", nil)
	listRec2 := httptest.NewRecorder()
	server.handleListCustomOperations(listRec2, listReq2)
	var listResult2 map[string]interface{}
	json.Unmarshal(listRec2.Body.Bytes(), &listResult2)
	assert.Equal(t, float64(1), listResult2["count"])

	// 6. Delete
	delReq := httptest.NewRequest(http.MethodDelete, "/state/operations/TestOp", nil)
	delReq.SetPathValue("name", "TestOp")
	delRec := httptest.NewRecorder()
	server.handleDeleteCustomOperation(delRec, delReq)
	assert.Equal(t, http.StatusOK, delRec.Code)

	// 7. Get — should be gone
	getReq2 := httptest.NewRequest(http.MethodGet, "/state/operations/TestOp", nil)
	getReq2.SetPathValue("name", "TestOp")
	getRec2 := httptest.NewRecorder()
	server.handleGetCustomOperation(getRec2, getReq2)
	assert.Equal(t, http.StatusNotFound, getRec2.Code)
}
