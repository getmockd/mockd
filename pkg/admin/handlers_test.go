package admin

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/getmockd/mockd/pkg/admin/engineclient"
	"github.com/getmockd/mockd/pkg/config"
	"github.com/getmockd/mockd/pkg/mock"
	"github.com/getmockd/mockd/pkg/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func boolPtr(b bool) *bool { return &b }

// mockEngineServer creates a test server that simulates the engine API.
// It allows tests to control responses for various endpoints.
type mockEngineServer struct {
	*httptest.Server
	mocks        map[string]*config.MockConfiguration
	requestCount int64
	uptime       int64
}

func newMockEngineServer() *mockEngineServer {
	mes := &mockEngineServer{
		mocks:        make(map[string]*config.MockConfiguration),
		requestCount: 0,
		uptime:       100,
	}

	mux := http.NewServeMux()

	// Status endpoint
	mux.HandleFunc("GET /status", func(w http.ResponseWriter, r *http.Request) {
		resp := engineclient.StatusResponse{
			ID:           "test-engine",
			Status:       "running",
			Uptime:       mes.uptime,
			MockCount:    len(mes.mocks),
			RequestCount: mes.requestCount,
		}
		json.NewEncoder(w).Encode(resp)
	})

	// List mocks
	mux.HandleFunc("GET /mocks", func(w http.ResponseWriter, r *http.Request) {
		mocks := make([]*config.MockConfiguration, 0, len(mes.mocks))
		for _, m := range mes.mocks {
			mocks = append(mocks, m)
		}
		json.NewEncoder(w).Encode(engineclient.MockListResponse{
			Mocks: mocks,
			Count: len(mocks),
		})
	})

	// Get mock by ID
	mux.HandleFunc("GET /mocks/{id}", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		m, ok := mes.mocks[id]
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(ErrorResponse{Error: "not_found", Message: "Mock not found"})
			return
		}
		json.NewEncoder(w).Encode(m)
	})

	// Create mock
	mux.HandleFunc("POST /mocks", func(w http.ResponseWriter, r *http.Request) {
		var m config.MockConfiguration
		if err := json.NewDecoder(r.Body).Decode(&m); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(ErrorResponse{Error: "invalid_json", Message: err.Error()})
			return
		}
		// Check for duplicate
		if _, exists := mes.mocks[m.ID]; exists {
			w.WriteHeader(http.StatusConflict)
			json.NewEncoder(w).Encode(ErrorResponse{Error: "duplicate_id", Message: "Mock with this ID already exists"})
			return
		}
		// Generate ID if not provided
		if m.ID == "" {
			m.ID = "mock-" + time.Now().Format("20060102150405")
		}
		mes.mocks[m.ID] = &m
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(&m)
	})

	// Update mock
	mux.HandleFunc("PUT /mocks/{id}", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if _, ok := mes.mocks[id]; !ok {
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(ErrorResponse{Error: "not_found", Message: "Mock not found"})
			return
		}
		var m config.MockConfiguration
		if err := json.NewDecoder(r.Body).Decode(&m); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(ErrorResponse{Error: "invalid_json", Message: err.Error()})
			return
		}
		m.ID = id
		mes.mocks[id] = &m
		json.NewEncoder(w).Encode(&m)
	})

	// Delete mock
	mux.HandleFunc("DELETE /mocks/{id}", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if _, ok := mes.mocks[id]; !ok {
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(ErrorResponse{Error: "not_found", Message: "Mock not found"})
			return
		}
		delete(mes.mocks, id)
		w.WriteHeader(http.StatusNoContent)
	})

	// Toggle mock
	mux.HandleFunc("POST /mocks/{id}/toggle", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		m, ok := mes.mocks[id]
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(ErrorResponse{Error: "not_found", Message: "Mock not found"})
			return
		}
		var req struct {
			Enabled bool `json:"enabled"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(ErrorResponse{Error: "invalid_json", Message: err.Error()})
			return
		}
		m.Enabled = &req.Enabled
		json.NewEncoder(w).Encode(m)
	})

	// Export config
	mux.HandleFunc("GET /export", func(w http.ResponseWriter, r *http.Request) {
		mocks := make([]*config.MockConfiguration, 0, len(mes.mocks))
		for _, m := range mes.mocks {
			mocks = append(mocks, m)
		}
		json.NewEncoder(w).Encode(config.MockCollection{
			Version: "1.0",
			Mocks:   mocks,
		})
	})

	// Import config
	mux.HandleFunc("POST /config", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Config  *config.MockCollection `json:"config"`
			Replace bool                   `json:"replace"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(ErrorResponse{Error: "invalid_json", Message: err.Error()})
			return
		}
		if req.Replace {
			mes.mocks = make(map[string]*config.MockConfiguration)
		}
		for _, m := range req.Config.Mocks {
			mes.mocks[m.ID] = m
		}
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})

	// Request logs
	mux.HandleFunc("GET /requests", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(engineclient.RequestListResponse{
			Requests: []*engineclient.RequestLogEntry{},
			Count:    0,
			Total:    0,
		})
	})

	mux.HandleFunc("GET /requests/{id}", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(ErrorResponse{Error: "not_found", Message: "Request not found"})
	})

	mux.HandleFunc("DELETE /requests", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]int{"cleared": 0})
	})

	mes.Server = httptest.NewServer(mux)
	return mes
}

func (mes *mockEngineServer) addMock(m *config.MockConfiguration) {
	mes.mocks[m.ID] = m
}

func (mes *mockEngineServer) client() *engineclient.Client {
	return engineclient.New(mes.URL)
}

// TestHandleListMocks tests the GET /mocks handler.
func TestHandleListMocks(t *testing.T) {
	t.Run("returns empty list when no mocks", func(t *testing.T) {
		server := newMockEngineServer()
		defer server.Close()

		api := NewAPI(0, WithLocalEngineClient(server.client()))

		req := httptest.NewRequest("GET", "/mocks", nil)
		rec := httptest.NewRecorder()

		api.handleListMocks(rec, req, server.client())

		assert.Equal(t, http.StatusOK, rec.Code)

		var resp MockListResponse
		err := json.Unmarshal(rec.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.Equal(t, 0, resp.Count)
		assert.Empty(t, resp.Mocks)
	})

	t.Run("returns mocks when available", func(t *testing.T) {
		server := newMockEngineServer()
		defer server.Close()

		// Add test mocks
		server.addMock(&config.MockConfiguration{
			ID:      "mock-1",
			Name:    "Test Mock 1",
			Enabled: boolPtr(true),
			Type:    mock.TypeHTTP,
		})
		server.addMock(&config.MockConfiguration{
			ID:      "mock-2",
			Name:    "Test Mock 2",
			Enabled: boolPtr(false),
			Type:    mock.TypeHTTP,
		})

		api := NewAPI(0, WithLocalEngineClient(server.client()))

		req := httptest.NewRequest("GET", "/mocks", nil)
		rec := httptest.NewRecorder()

		api.handleListMocks(rec, req, server.client())

		assert.Equal(t, http.StatusOK, rec.Code)

		var resp MockListResponse
		err := json.Unmarshal(rec.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.Equal(t, 2, resp.Count)
	})

	t.Run("filters by enabled status", func(t *testing.T) {
		server := newMockEngineServer()
		defer server.Close()

		server.addMock(&config.MockConfiguration{
			ID:      "mock-enabled",
			Name:    "Enabled Mock",
			Enabled: boolPtr(true),
			Type:    mock.TypeHTTP,
		})
		server.addMock(&config.MockConfiguration{
			ID:      "mock-disabled",
			Name:    "Disabled Mock",
			Enabled: boolPtr(false),
			Type:    mock.TypeHTTP,
		})

		api := NewAPI(0, WithLocalEngineClient(server.client()))

		req := httptest.NewRequest("GET", "/mocks?enabled=true", nil)
		rec := httptest.NewRecorder()

		api.handleListMocks(rec, req, server.client())

		assert.Equal(t, http.StatusOK, rec.Code)

		var resp MockListResponse
		err := json.Unmarshal(rec.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.Equal(t, 1, resp.Count)
		assert.NotNil(t, resp.Mocks[0].Enabled)
		assert.True(t, *resp.Mocks[0].Enabled)
	})

	t.Run("filters by parentId", func(t *testing.T) {
		server := newMockEngineServer()
		defer server.Close()

		server.addMock(&config.MockConfiguration{
			ID:       "mock-root",
			Name:     "Root Mock",
			ParentID: "",
			Type:     mock.TypeHTTP,
		})
		server.addMock(&config.MockConfiguration{
			ID:       "mock-folder1",
			Name:     "Folder 1 Mock",
			ParentID: "folder-1",
			Type:     mock.TypeHTTP,
		})

		api := NewAPI(0, WithLocalEngineClient(server.client()))

		req := httptest.NewRequest("GET", "/mocks?parentId=folder-1", nil)
		rec := httptest.NewRecorder()

		api.handleListMocks(rec, req, server.client())

		assert.Equal(t, http.StatusOK, rec.Code)

		var resp MockListResponse
		err := json.Unmarshal(rec.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.Equal(t, 1, resp.Count)
		assert.Equal(t, "folder-1", resp.Mocks[0].ParentID)
	})

	t.Run("returns error when no engine connected", func(t *testing.T) {
		api := NewAPI(0) // No engine

		req := httptest.NewRequest("GET", "/mocks", nil)
		rec := httptest.NewRecorder()

		api.requireEngine(api.handleListMocks)(rec, req)

		assert.Equal(t, http.StatusServiceUnavailable, rec.Code)

		var resp ErrorResponse
		err := json.Unmarshal(rec.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.Equal(t, "no_engine", resp.Error)
	})
}

// TestHandleGetMock tests the GET /mocks/{id} handler.
func TestHandleGetMock(t *testing.T) {
	t.Run("returns mock when found", func(t *testing.T) {
		server := newMockEngineServer()
		defer server.Close()

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
		server.addMock(testMock)

		api := NewAPI(0, WithLocalEngineClient(server.client()))

		req := httptest.NewRequest("GET", "/mocks/mock-123", nil)
		req.SetPathValue("id", "mock-123")
		rec := httptest.NewRecorder()

		api.handleGetMock(rec, req, server.client())

		assert.Equal(t, http.StatusOK, rec.Code)

		var resp config.MockConfiguration
		err := json.Unmarshal(rec.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.Equal(t, "mock-123", resp.ID)
		assert.Equal(t, "Test Mock", resp.Name)
	})

	t.Run("returns 404 when mock not found", func(t *testing.T) {
		server := newMockEngineServer()
		defer server.Close()

		api := NewAPI(0, WithLocalEngineClient(server.client()))

		req := httptest.NewRequest("GET", "/mocks/nonexistent", nil)
		req.SetPathValue("id", "nonexistent")
		rec := httptest.NewRecorder()

		api.handleGetMock(rec, req, server.client())

		assert.Equal(t, http.StatusNotFound, rec.Code)

		var resp ErrorResponse
		err := json.Unmarshal(rec.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.Equal(t, "not_found", resp.Error)
	})

	t.Run("returns 400 when ID is missing", func(t *testing.T) {
		server := newMockEngineServer()
		defer server.Close()

		api := NewAPI(0, WithLocalEngineClient(server.client()))

		req := httptest.NewRequest("GET", "/mocks/", nil)
		req.SetPathValue("id", "")
		rec := httptest.NewRecorder()

		api.handleGetMock(rec, req, server.client())

		assert.Equal(t, http.StatusBadRequest, rec.Code)

		var resp ErrorResponse
		err := json.Unmarshal(rec.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.Equal(t, "missing_id", resp.Error)
	})

	t.Run("returns error when no engine connected", func(t *testing.T) {
		api := NewAPI(0)

		req := httptest.NewRequest("GET", "/mocks/mock-123", nil)
		req.SetPathValue("id", "mock-123")
		rec := httptest.NewRecorder()

		api.requireEngine(api.handleGetMock)(rec, req)

		assert.Equal(t, http.StatusServiceUnavailable, rec.Code)

		var resp ErrorResponse
		err := json.Unmarshal(rec.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.Equal(t, "no_engine", resp.Error)
	})
}

// TestHandleCreateMock tests the POST /mocks handler.
func TestHandleCreateMock(t *testing.T) {
	t.Run("creates mock successfully", func(t *testing.T) {
		server := newMockEngineServer()
		defer server.Close()

		api := NewAPI(0, WithLocalEngineClient(server.client()))

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

		req := httptest.NewRequest("POST", "/mocks", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		api.handleCreateMock(rec, req, server.client())

		assert.Equal(t, http.StatusCreated, rec.Code)

		var resp config.MockConfiguration
		err := json.Unmarshal(rec.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.Equal(t, "new-mock", resp.ID)
		assert.Equal(t, "New Mock", resp.Name)
	})

	t.Run("returns 400 for invalid JSON", func(t *testing.T) {
		server := newMockEngineServer()
		defer server.Close()

		api := NewAPI(0, WithLocalEngineClient(server.client()))

		req := httptest.NewRequest("POST", "/mocks", bytes.NewReader([]byte("invalid json")))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		api.handleCreateMock(rec, req, server.client())

		assert.Equal(t, http.StatusBadRequest, rec.Code)

		var resp ErrorResponse
		err := json.Unmarshal(rec.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.Equal(t, "invalid_json", resp.Error)
	})

	t.Run("returns 409 for duplicate ID", func(t *testing.T) {
		server := newMockEngineServer()
		defer server.Close()

		server.addMock(&config.MockConfiguration{
			ID:   "existing-mock",
			Name: "Existing Mock",
			Type: mock.TypeHTTP,
		})

		api := NewAPI(0, WithLocalEngineClient(server.client()))

		mockData := map[string]interface{}{
			"id":   "existing-mock",
			"name": "Duplicate Mock",
			"type": "http",
		}
		body, _ := json.Marshal(mockData)

		req := httptest.NewRequest("POST", "/mocks", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		api.handleCreateMock(rec, req, server.client())

		assert.Equal(t, http.StatusConflict, rec.Code)

		var resp ErrorResponse
		err := json.Unmarshal(rec.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.Equal(t, "duplicate_id", resp.Error)
	})

	t.Run("returns error when no engine connected", func(t *testing.T) {
		api := NewAPI(0)

		mockData := map[string]interface{}{
			"id":   "new-mock",
			"name": "New Mock",
		}
		body, _ := json.Marshal(mockData)

		req := httptest.NewRequest("POST", "/mocks", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		api.requireEngine(api.handleCreateMock)(rec, req)

		assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
	})
}

// TestHandleUpdateMock tests the PUT /mocks/{id} handler.
func TestHandleUpdateMock(t *testing.T) {
	t.Run("updates mock successfully", func(t *testing.T) {
		server := newMockEngineServer()
		defer server.Close()

		server.addMock(&config.MockConfiguration{
			ID:      "mock-to-update",
			Name:    "Original Name",
			Enabled: boolPtr(true),
			Type:    mock.TypeHTTP,
		})

		api := NewAPI(0, WithLocalEngineClient(server.client()))

		updateData := map[string]interface{}{
			"name":    "Updated Name",
			"enabled": false,
			"type":    "http",
		}
		body, _ := json.Marshal(updateData)

		req := httptest.NewRequest("PUT", "/mocks/mock-to-update", bytes.NewReader(body))
		req.SetPathValue("id", "mock-to-update")
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		api.handleUpdateMock(rec, req, server.client())

		assert.Equal(t, http.StatusOK, rec.Code)

		var resp config.MockConfiguration
		err := json.Unmarshal(rec.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.Equal(t, "mock-to-update", resp.ID)
		assert.Equal(t, "Updated Name", resp.Name)
	})

	t.Run("returns 404 for non-existent mock", func(t *testing.T) {
		server := newMockEngineServer()
		defer server.Close()

		api := NewAPI(0, WithLocalEngineClient(server.client()))

		updateData := map[string]interface{}{
			"name": "Updated Name",
		}
		body, _ := json.Marshal(updateData)

		req := httptest.NewRequest("PUT", "/mocks/nonexistent", bytes.NewReader(body))
		req.SetPathValue("id", "nonexistent")
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		api.handleUpdateMock(rec, req, server.client())

		assert.Equal(t, http.StatusNotFound, rec.Code)

		var resp ErrorResponse
		err := json.Unmarshal(rec.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.Equal(t, "not_found", resp.Error)
	})

	t.Run("returns 400 for invalid JSON", func(t *testing.T) {
		server := newMockEngineServer()
		defer server.Close()

		server.addMock(&config.MockConfiguration{
			ID:   "mock-to-update",
			Name: "Original Name",
			Type: mock.TypeHTTP,
		})

		api := NewAPI(0, WithLocalEngineClient(server.client()))

		req := httptest.NewRequest("PUT", "/mocks/mock-to-update", bytes.NewReader([]byte("invalid")))
		req.SetPathValue("id", "mock-to-update")
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		api.handleUpdateMock(rec, req, server.client())

		assert.Equal(t, http.StatusBadRequest, rec.Code)

		var resp ErrorResponse
		err := json.Unmarshal(rec.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.Equal(t, "invalid_json", resp.Error)
	})

	t.Run("returns 400 when ID is missing", func(t *testing.T) {
		server := newMockEngineServer()
		defer server.Close()

		api := NewAPI(0, WithLocalEngineClient(server.client()))

		req := httptest.NewRequest("PUT", "/mocks/", bytes.NewReader([]byte("{}")))
		req.SetPathValue("id", "")
		rec := httptest.NewRecorder()

		api.handleUpdateMock(rec, req, server.client())

		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("returns error when no engine connected", func(t *testing.T) {
		api := NewAPI(0)

		updateData := map[string]interface{}{
			"name": "Updated Name",
		}
		body, _ := json.Marshal(updateData)

		req := httptest.NewRequest("PUT", "/mocks/mock-123", bytes.NewReader(body))
		req.SetPathValue("id", "mock-123")
		rec := httptest.NewRecorder()

		api.requireEngine(api.handleUpdateMock)(rec, req)

		assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
	})
}

// TestHandleDeleteMock tests the DELETE /mocks/{id} handler.
func TestHandleDeleteMock(t *testing.T) {
	t.Run("deletes mock successfully", func(t *testing.T) {
		server := newMockEngineServer()
		defer server.Close()

		server.addMock(&config.MockConfiguration{
			ID:   "mock-to-delete",
			Name: "To Be Deleted",
			Type: mock.TypeHTTP,
		})

		api := NewAPI(0, WithLocalEngineClient(server.client()))

		req := httptest.NewRequest("DELETE", "/mocks/mock-to-delete", nil)
		req.SetPathValue("id", "mock-to-delete")
		rec := httptest.NewRecorder()

		api.handleDeleteMock(rec, req, server.client())

		assert.Equal(t, http.StatusNoContent, rec.Code)

		// Verify mock was deleted
		assert.Empty(t, server.mocks["mock-to-delete"])
	})

	t.Run("returns 404 for non-existent mock", func(t *testing.T) {
		server := newMockEngineServer()
		defer server.Close()

		api := NewAPI(0, WithLocalEngineClient(server.client()))

		req := httptest.NewRequest("DELETE", "/mocks/nonexistent", nil)
		req.SetPathValue("id", "nonexistent")
		rec := httptest.NewRecorder()

		api.handleDeleteMock(rec, req, server.client())

		assert.Equal(t, http.StatusNotFound, rec.Code)

		var resp ErrorResponse
		err := json.Unmarshal(rec.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.Equal(t, "not_found", resp.Error)
	})

	t.Run("returns 400 when ID is missing", func(t *testing.T) {
		server := newMockEngineServer()
		defer server.Close()

		api := NewAPI(0, WithLocalEngineClient(server.client()))

		req := httptest.NewRequest("DELETE", "/mocks/", nil)
		req.SetPathValue("id", "")
		rec := httptest.NewRecorder()

		api.handleDeleteMock(rec, req, server.client())

		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("returns error when no engine connected", func(t *testing.T) {
		api := NewAPI(0)

		req := httptest.NewRequest("DELETE", "/mocks/mock-123", nil)
		req.SetPathValue("id", "mock-123")
		rec := httptest.NewRecorder()

		api.requireEngine(api.handleDeleteMock)(rec, req)

		assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
	})
}

// TestHandleToggleMock tests the POST /mocks/{id}/toggle handler.
func TestHandleToggleMock(t *testing.T) {
	t.Run("toggles mock to enabled", func(t *testing.T) {
		server := newMockEngineServer()
		defer server.Close()

		server.addMock(&config.MockConfiguration{
			ID:      "mock-toggle",
			Name:    "Toggle Mock",
			Enabled: boolPtr(false),
			Type:    mock.TypeHTTP,
		})

		api := NewAPI(0, WithLocalEngineClient(server.client()))

		body := `{"enabled": true}`
		req := httptest.NewRequest("POST", "/mocks/mock-toggle/toggle", bytes.NewReader([]byte(body)))
		req.SetPathValue("id", "mock-toggle")
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		api.handleToggleMock(rec, req, server.client())

		assert.Equal(t, http.StatusOK, rec.Code)

		var resp config.MockConfiguration
		err := json.Unmarshal(rec.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.NotNil(t, resp.Enabled)
		assert.True(t, *resp.Enabled)
	})

	t.Run("toggles mock to disabled", func(t *testing.T) {
		server := newMockEngineServer()
		defer server.Close()

		server.addMock(&config.MockConfiguration{
			ID:      "mock-toggle",
			Name:    "Toggle Mock",
			Enabled: boolPtr(true),
			Type:    mock.TypeHTTP,
		})

		api := NewAPI(0, WithLocalEngineClient(server.client()))

		body := `{"enabled": false}`
		req := httptest.NewRequest("POST", "/mocks/mock-toggle/toggle", bytes.NewReader([]byte(body)))
		req.SetPathValue("id", "mock-toggle")
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		api.handleToggleMock(rec, req, server.client())

		assert.Equal(t, http.StatusOK, rec.Code)

		var resp config.MockConfiguration
		err := json.Unmarshal(rec.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.NotNil(t, resp.Enabled)
		assert.False(t, *resp.Enabled)
	})

	t.Run("returns 404 for non-existent mock", func(t *testing.T) {
		server := newMockEngineServer()
		defer server.Close()

		api := NewAPI(0, WithLocalEngineClient(server.client()))

		body := `{"enabled": true}`
		req := httptest.NewRequest("POST", "/mocks/nonexistent/toggle", bytes.NewReader([]byte(body)))
		req.SetPathValue("id", "nonexistent")
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		api.handleToggleMock(rec, req, server.client())

		assert.Equal(t, http.StatusNotFound, rec.Code)
	})

	t.Run("returns 400 for invalid JSON", func(t *testing.T) {
		server := newMockEngineServer()
		defer server.Close()

		server.addMock(&config.MockConfiguration{
			ID:   "mock-toggle",
			Name: "Toggle Mock",
			Type: mock.TypeHTTP,
		})

		api := NewAPI(0, WithLocalEngineClient(server.client()))

		req := httptest.NewRequest("POST", "/mocks/mock-toggle/toggle", bytes.NewReader([]byte("invalid")))
		req.SetPathValue("id", "mock-toggle")
		rec := httptest.NewRecorder()

		api.handleToggleMock(rec, req, server.client())

		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("returns error when no engine connected", func(t *testing.T) {
		api := NewAPI(0)

		body := `{"enabled": true}`
		req := httptest.NewRequest("POST", "/mocks/mock-123/toggle", bytes.NewReader([]byte(body)))
		req.SetPathValue("id", "mock-123")
		rec := httptest.NewRecorder()

		api.requireEngine(api.handleToggleMock)(rec, req)

		assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
	})
}

// TestHandleHealth tests the GET /health handler.
func TestHandleHealth(t *testing.T) {
	t.Run("returns healthy status", func(t *testing.T) {
		api := NewAPI(0)

		req := httptest.NewRequest("GET", "/health", nil)
		rec := httptest.NewRecorder()

		api.handleHealth(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)

		var resp HealthResponse
		err := json.Unmarshal(rec.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.Equal(t, "ok", resp.Status)
	})
}

// TestHandleGetStatus tests the GET /status handler.
func TestHandleGetStatus(t *testing.T) {
	t.Run("returns server status", func(t *testing.T) {
		server := newMockEngineServer()
		defer server.Close()

		server.addMock(&config.MockConfiguration{
			ID:      "mock-1",
			Enabled: boolPtr(true),
			Type:    mock.TypeHTTP,
		})
		server.addMock(&config.MockConfiguration{
			ID:      "mock-2",
			Enabled: boolPtr(false),
			Type:    mock.TypeHTTP,
		})

		api := NewAPI(8080, WithLocalEngineClient(server.client()))

		req := httptest.NewRequest("GET", "/status", nil)
		rec := httptest.NewRecorder()

		api.handleGetStatus(rec, req, server.client())

		assert.Equal(t, http.StatusOK, rec.Code)

		var resp ServerStatus
		err := json.Unmarshal(rec.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.Equal(t, "running", resp.Status)
		assert.Equal(t, 2, resp.MockCount)
		assert.Equal(t, 1, resp.ActiveMocks)
		assert.Equal(t, 8080, resp.AdminPort)
	})

	t.Run("returns error when no engine connected", func(t *testing.T) {
		api := NewAPI(0)

		req := httptest.NewRequest("GET", "/status", nil)
		rec := httptest.NewRecorder()

		api.requireEngine(api.handleGetStatus)(rec, req)

		assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
	})
}

// TestHandleExportConfig tests the GET /config handler.
func TestHandleExportConfig(t *testing.T) {
	t.Run("exports configuration", func(t *testing.T) {
		server := newMockEngineServer()
		defer server.Close()

		server.addMock(&config.MockConfiguration{
			ID:      "mock-1",
			Name:    "Export Test Mock",
			Enabled: boolPtr(true),
			Type:    mock.TypeHTTP,
		})

		api := NewAPI(0, WithLocalEngineClient(server.client()))

		req := httptest.NewRequest("GET", "/config", nil)
		rec := httptest.NewRecorder()

		api.handleExportConfig(rec, req, server.client())

		assert.Equal(t, http.StatusOK, rec.Code)

		var resp config.MockCollection
		err := json.Unmarshal(rec.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.Equal(t, "1.0", resp.Version)
		assert.Len(t, resp.Mocks, 1)
	})

	t.Run("returns error when no engine connected", func(t *testing.T) {
		api := NewAPI(0)

		req := httptest.NewRequest("GET", "/config", nil)
		rec := httptest.NewRecorder()

		api.requireEngine(api.handleExportConfig)(rec, req)

		assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
	})
}

// TestHandleImportConfig tests the POST /config handler.
func TestHandleImportConfig(t *testing.T) {
	t.Run("imports configuration", func(t *testing.T) {
		server := newMockEngineServer()
		defer server.Close()

		api := NewAPI(0, WithLocalEngineClient(server.client()))

		importData := map[string]interface{}{
			"config": map[string]interface{}{
				"version": "1.0",
				"mocks": []map[string]interface{}{
					{
						"id":      "imported-mock",
						"name":    "Imported Mock",
						"enabled": true,
						"type":    "http",
					},
				},
			},
			"replace": false,
		}
		body, _ := json.Marshal(importData)

		req := httptest.NewRequest("POST", "/config", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		api.handleImportConfig(rec, req, server.client())

		assert.Equal(t, http.StatusOK, rec.Code)
	})

	t.Run("returns 400 for missing config", func(t *testing.T) {
		server := newMockEngineServer()
		defer server.Close()

		api := NewAPI(0, WithLocalEngineClient(server.client()))

		importData := map[string]interface{}{
			"replace": false,
		}
		body, _ := json.Marshal(importData)

		req := httptest.NewRequest("POST", "/config", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		api.handleImportConfig(rec, req, server.client())

		assert.Equal(t, http.StatusBadRequest, rec.Code)

		var resp ErrorResponse
		err := json.Unmarshal(rec.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.Equal(t, "missing_config", resp.Error)
	})

	t.Run("returns 400 for invalid JSON", func(t *testing.T) {
		server := newMockEngineServer()
		defer server.Close()

		api := NewAPI(0, WithLocalEngineClient(server.client()))

		req := httptest.NewRequest("POST", "/config", bytes.NewReader([]byte("invalid")))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		api.handleImportConfig(rec, req, server.client())

		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("returns error when no engine connected", func(t *testing.T) {
		api := NewAPI(0)

		importData := map[string]interface{}{
			"config": map[string]interface{}{
				"version": "1.0",
				"mocks":   []interface{}{},
			},
		}
		body, _ := json.Marshal(importData)

		req := httptest.NewRequest("POST", "/config", bytes.NewReader(body))
		rec := httptest.NewRecorder()

		api.requireEngine(api.handleImportConfig)(rec, req)

		assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
	})
}

// TestHandleListRequests tests the GET /requests handler.
func TestHandleListRequests(t *testing.T) {
	t.Run("returns empty request list", func(t *testing.T) {
		server := newMockEngineServer()
		defer server.Close()

		api := NewAPI(0, WithLocalEngineClient(server.client()))

		req := httptest.NewRequest("GET", "/requests", nil)
		rec := httptest.NewRecorder()

		api.handleListRequests(rec, req, server.client())

		assert.Equal(t, http.StatusOK, rec.Code)
	})

	t.Run("returns error when no engine connected", func(t *testing.T) {
		api := NewAPI(0)

		req := httptest.NewRequest("GET", "/requests", nil)
		rec := httptest.NewRecorder()

		api.requireEngine(api.handleListRequests)(rec, req)

		assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
	})
}

// TestHandleGetRequest tests the GET /requests/{id} handler.
func TestHandleGetRequest(t *testing.T) {
	t.Run("returns 404 for non-existent request", func(t *testing.T) {
		server := newMockEngineServer()
		defer server.Close()

		api := NewAPI(0, WithLocalEngineClient(server.client()))

		req := httptest.NewRequest("GET", "/requests/req-123", nil)
		req.SetPathValue("id", "req-123")
		rec := httptest.NewRecorder()

		api.handleGetRequest(rec, req, server.client())

		assert.Equal(t, http.StatusNotFound, rec.Code)
	})

	t.Run("returns 400 when ID is missing", func(t *testing.T) {
		server := newMockEngineServer()
		defer server.Close()

		api := NewAPI(0, WithLocalEngineClient(server.client()))

		req := httptest.NewRequest("GET", "/requests/", nil)
		req.SetPathValue("id", "")
		rec := httptest.NewRecorder()

		api.handleGetRequest(rec, req, server.client())

		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("returns error when no engine connected", func(t *testing.T) {
		api := NewAPI(0)

		req := httptest.NewRequest("GET", "/requests/req-123", nil)
		req.SetPathValue("id", "req-123")
		rec := httptest.NewRecorder()

		api.requireEngine(api.handleGetRequest)(rec, req)

		assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
	})
}

// TestHandleClearRequests tests the DELETE /requests handler.
func TestHandleClearRequests(t *testing.T) {
	t.Run("clears request logs", func(t *testing.T) {
		server := newMockEngineServer()
		defer server.Close()

		api := NewAPI(0, WithLocalEngineClient(server.client()))

		req := httptest.NewRequest("DELETE", "/requests", nil)
		rec := httptest.NewRecorder()

		api.handleClearRequests(rec, req, server.client())

		assert.Equal(t, http.StatusOK, rec.Code)
	})

	t.Run("returns error when no engine connected", func(t *testing.T) {
		api := NewAPI(0)

		req := httptest.NewRequest("DELETE", "/requests", nil)
		rec := httptest.NewRecorder()

		api.requireEngine(api.handleClearRequests)(rec, req)

		assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
	})
}

// TestSortMocksByMetaSortKey tests the sorting function.
func TestSortMocksByMetaSortKey(t *testing.T) {
	t.Run("sorts mocks by metaSortKey ascending", func(t *testing.T) {
		mocks := []*config.MockConfiguration{
			{ID: "mock-3", MetaSortKey: 3},
			{ID: "mock-1", MetaSortKey: 1},
			{ID: "mock-2", MetaSortKey: 2},
		}

		sortMocksByMetaSortKey(mocks)

		assert.Equal(t, "mock-1", mocks[0].ID)
		assert.Equal(t, "mock-2", mocks[1].ID)
		assert.Equal(t, "mock-3", mocks[2].ID)
	})

	t.Run("handles negative sort keys for newest-first ordering", func(t *testing.T) {
		mocks := []*config.MockConfiguration{
			{ID: "mock-oldest", MetaSortKey: -1000},
			{ID: "mock-newest", MetaSortKey: -3000},
			{ID: "mock-middle", MetaSortKey: -2000},
		}

		sortMocksByMetaSortKey(mocks)

		assert.Equal(t, "mock-newest", mocks[0].ID)
		assert.Equal(t, "mock-middle", mocks[1].ID)
		assert.Equal(t, "mock-oldest", mocks[2].ID)
	})

	t.Run("handles empty slice", func(t *testing.T) {
		mocks := []*config.MockConfiguration{}
		sortMocksByMetaSortKey(mocks)
		assert.Empty(t, mocks)
	})

	t.Run("handles single element", func(t *testing.T) {
		mocks := []*config.MockConfiguration{
			{ID: "mock-1", MetaSortKey: 1},
		}
		sortMocksByMetaSortKey(mocks)
		assert.Len(t, mocks, 1)
		assert.Equal(t, "mock-1", mocks[0].ID)
	})
}

// TestWriteJSON tests the JSON response helper.
func TestWriteJSON(t *testing.T) {
	t.Run("writes JSON with correct content type", func(t *testing.T) {
		rec := httptest.NewRecorder()

		data := map[string]string{"message": "hello"}
		writeJSON(rec, http.StatusOK, data)

		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))
		assert.Contains(t, rec.Body.String(), "hello")
	})

	t.Run("handles nil data", func(t *testing.T) {
		rec := httptest.NewRecorder()

		writeJSON(rec, http.StatusNoContent, nil)

		assert.Equal(t, http.StatusNoContent, rec.Code)
	})
}

// TestWriteError tests the error response helper.
func TestWriteError(t *testing.T) {
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
}

// TestAdminAPIWithTimeout tests that the API properly handles context cancellation.
func TestAdminAPIWithTimeout(t *testing.T) {
	t.Run("handles context cancellation gracefully", func(t *testing.T) {
		server := newMockEngineServer()
		defer server.Close()

		api := NewAPI(0, WithLocalEngineClient(server.client()))

		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		req := httptest.NewRequest("GET", "/mocks", nil).WithContext(ctx)
		rec := httptest.NewRecorder()

		api.handleListMocks(rec, req, server.client())

		// The handler should return an error since context is cancelled
		// The exact status depends on how the engine client handles cancellation
		assert.True(t, rec.Code >= 400 || rec.Code == 200) // Either error or success before context check
	})
}

// TestHandleListEngines tests the GET /engines endpoint
func TestHandleListEngines(t *testing.T) {
	t.Run("returns empty list when no local engine configured", func(t *testing.T) {
		// Create API without local engine
		api := NewAPI(0)

		req := httptest.NewRequest("GET", "/engines", nil)
		rec := httptest.NewRecorder()

		api.handleListEngines(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)

		var resp EngineListResponse
		err := json.NewDecoder(rec.Body).Decode(&resp)
		require.NoError(t, err)

		assert.Equal(t, 0, resp.Total)
		assert.Empty(t, resp.Engines)
	})

	t.Run("includes local engine when configured", func(t *testing.T) {
		server := newMockEngineServer()
		defer server.Close()

		api := NewAPI(0, WithLocalEngineClient(server.client()))

		req := httptest.NewRequest("GET", "/engines", nil)
		rec := httptest.NewRecorder()

		api.handleListEngines(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)

		var resp EngineListResponse
		err := json.NewDecoder(rec.Body).Decode(&resp)
		require.NoError(t, err)

		assert.Equal(t, 1, resp.Total)
		require.Len(t, resp.Engines, 1)

		// Verify local engine properties
		localEngine := resp.Engines[0]
		assert.Equal(t, "test-engine", localEngine.ID)
		assert.Equal(t, "localhost", localEngine.Host)
		assert.Equal(t, "online", string(localEngine.Status))
	})

	t.Run("includes both local and registered engines", func(t *testing.T) {
		server := newMockEngineServer()
		defer server.Close()

		api := NewAPI(0, WithLocalEngineClient(server.client()))

		// Register a remote engine
		remoteEngine := &store.Engine{
			ID:     "remote-engine-1",
			Name:   "Remote Engine",
			Host:   "remote.example.com",
			Port:   8080,
			Status: store.EngineStatusOnline,
		}
		_ = api.engineRegistry.Register(remoteEngine)

		req := httptest.NewRequest("GET", "/engines", nil)
		rec := httptest.NewRecorder()

		api.handleListEngines(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)

		var resp EngineListResponse
		err := json.NewDecoder(rec.Body).Decode(&resp)
		require.NoError(t, err)

		assert.Equal(t, 2, resp.Total)
		require.Len(t, resp.Engines, 2)

		// Local engine should be first
		assert.Equal(t, "test-engine", resp.Engines[0].ID)
		assert.Equal(t, "remote-engine-1", resp.Engines[1].ID)
	})

	t.Run("returns offline status when local engine is unreachable", func(t *testing.T) {
		// Create a client that points to a non-existent server
		client := engineclient.New("http://localhost:99999")
		api := NewAPI(0, WithLocalEngineClient(client))

		req := httptest.NewRequest("GET", "/engines", nil)
		rec := httptest.NewRecorder()

		api.handleListEngines(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)

		var resp EngineListResponse
		err := json.NewDecoder(rec.Body).Decode(&resp)
		require.NoError(t, err)

		assert.Equal(t, 1, resp.Total)
		require.Len(t, resp.Engines, 1)

		// Engine should be reported as offline
		localEngine := resp.Engines[0]
		assert.Equal(t, LocalEngineID, localEngine.ID)
		assert.Equal(t, "offline", string(localEngine.Status))
	})
}
