package admin

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
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
			Name:         "Test Engine",
			Status:       "running",
			Uptime:       mes.uptime,
			MockCount:    len(mes.mocks),
			RequestCount: mes.requestCount,
			Protocols: map[string]engineclient.ProtocolStatus{
				"http": {
					Enabled: true,
					Port:    4280,
				},
			},
			StartedAt: time.Unix(1700000000, 0).UTC(),
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
		imported := 0
		for _, m := range req.Config.Mocks {
			if m != nil {
				mes.mocks[m.ID] = m
				imported++
			}
		}
		json.NewEncoder(w).Encode(engineclient.ImportResult{
			Imported: imported,
			Total:    imported,
			Message:  "ok",
		})
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

func (mes *mockEngineServer) client() *engineclient.Client {
	return engineclient.New(mes.URL)
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
	t.Run("returns server status with active mocks from admin store", func(t *testing.T) {
		server := newMockEngineServer()
		defer server.Close()

		api := NewAPI(8080,
			WithDataDir(t.TempDir()),
			WithLocalEngineClient(server.client()),
		)

		// Populate the admin store (single source of truth for mock count).
		ctx := t.Context()
		mockStore := api.dataStore.Mocks()
		_ = mockStore.Create(ctx, &config.MockConfiguration{
			ID:      "mock-1",
			Enabled: boolPtr(true),
			Type:    mock.TypeHTTP,
		})
		_ = mockStore.Create(ctx, &config.MockConfiguration{
			ID:      "mock-2",
			Enabled: boolPtr(false),
			Type:    mock.TypeHTTP,
		})

		req := httptest.NewRequest("GET", "/status", nil)
		rec := httptest.NewRecorder()

		api.handleGetStatus(rec, req, server.client())

		assert.Equal(t, http.StatusOK, rec.Code)

		var resp ServerStatus
		err := json.Unmarshal(rec.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.Equal(t, "test-engine", resp.ID)
		assert.Equal(t, "Test Engine", resp.Name)
		assert.Equal(t, "running", resp.Status)
		assert.Equal(t, int64(100), resp.Uptime)
		assert.Equal(t, 4280, resp.HTTPPort)
		assert.False(t, resp.StartedAt.IsZero())
		// MockCount comes from engine status (total registered).
		// ActiveMocks comes from admin store (enabled mocks).
		assert.Equal(t, 1, resp.ActiveMocks, "only 1 mock is enabled in admin store")
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
	t.Run("exports configuration from admin store", func(t *testing.T) {
		api := NewAPI(0, WithDataDir(t.TempDir()))

		// Populate the admin store (single source of truth for export).
		ctx := t.Context()
		mockStore := api.dataStore.Mocks()
		_ = mockStore.Create(ctx, &config.MockConfiguration{
			ID:      "mock-1",
			Name:    "Export Test Mock",
			Enabled: boolPtr(true),
			Type:    mock.TypeHTTP,
		})

		req := httptest.NewRequest("GET", "/config", nil)
		rec := httptest.NewRecorder()

		api.handleExportConfig(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)

		var resp config.MockCollection
		err := json.Unmarshal(rec.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.Equal(t, "1.0", resp.Version)
		assert.Equal(t, "MockCollection", resp.Kind)
		assert.Len(t, resp.Mocks, 1)
		assert.Equal(t, "mock-1", resp.Mocks[0].ID)
	})

	t.Run("exports with custom name via query param", func(t *testing.T) {
		api := NewAPI(0, WithDataDir(t.TempDir()))

		req := httptest.NewRequest("GET", "/config?name=my-export", nil)
		rec := httptest.NewRecorder()

		api.handleExportConfig(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)

		var resp config.MockCollection
		err := json.Unmarshal(rec.Body.Bytes(), &resp)
		require.NoError(t, err)
		require.NotNil(t, resp.Metadata)
		assert.Equal(t, "my-export", resp.Metadata.Name)
	})

	t.Run("export works even without engine (store-only)", func(t *testing.T) {
		// No engine configured — export should still work from the store.
		api := NewAPI(0, WithDataDir(t.TempDir()))

		ctx := t.Context()
		_ = api.dataStore.Mocks().Create(ctx, &config.MockConfiguration{
			ID:   "store-only-mock",
			Name: "Store Only",
			Type: mock.TypeHTTP,
		})

		req := httptest.NewRequest("GET", "/config", nil)
		rec := httptest.NewRecorder()

		api.handleExportConfig(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)

		var resp config.MockCollection
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
		assert.Len(t, resp.Mocks, 1)
		assert.Equal(t, "store-only-mock", resp.Mocks[0].ID)
	})

	t.Run("includes stateful resources in export", func(t *testing.T) {
		api := NewAPI(0, WithDataDir(t.TempDir()))

		ctx := t.Context()
		_ = api.dataStore.StatefulResources().Create(ctx, &config.StatefulResourceConfig{
			Name:     "users",
			BasePath: "/api/users",
		})

		req := httptest.NewRequest("GET", "/config", nil)
		rec := httptest.NewRecorder()

		api.handleExportConfig(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)

		var resp config.MockCollection
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
		require.Len(t, resp.StatefulResources, 1)
		assert.Equal(t, "users", resp.StatefulResources[0].Name)
	})
}

// TestHandleImportConfig tests the POST /config handler.
func TestHandleImportConfig(t *testing.T) {
	t.Run("imports configuration and reports total from admin store", func(t *testing.T) {
		server := newMockEngineServer()
		defer server.Close()

		api := NewAPI(0,
			WithDataDir(t.TempDir()),
			WithLocalEngineClient(server.client()),
		)

		// Pre-populate store with an existing mock so total > imported.
		ctx := t.Context()
		_ = api.dataStore.Mocks().Create(ctx, &config.MockConfiguration{
			ID:   "existing-mock",
			Name: "Existing",
			Type: mock.TypeHTTP,
		})

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

		require.Equal(t, http.StatusOK, rec.Code)

		// Verify the response total comes from admin store (existing + imported).
		var resp map[string]interface{}
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
		// total should be 2: 1 existing + 1 imported (both in admin store).
		assert.Equal(t, float64(2), resp["total"], "total should come from admin store, not engine")
		assert.Equal(t, float64(1), resp["imported"], "imported should come from engine result")
	})

	t.Run("imports unwrapped config (export format)", func(t *testing.T) {
		server := newMockEngineServer()
		defer server.Close()

		api := NewAPI(0, WithLocalEngineClient(server.client()))

		// This is the format returned by GET /config (export) — no "config" wrapper.
		importData := map[string]interface{}{
			"version": "1.0",
			"name":    "mockd-export",
			"mocks": []map[string]interface{}{
				{
					"id":      "imported-mock",
					"name":    "Imported Mock",
					"enabled": true,
					"type":    "http",
				},
			},
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

	t.Run("supports dryRun query bool variants", func(t *testing.T) {
		api := NewAPI(0)

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
		}
		body, _ := json.Marshal(importData)

		req := httptest.NewRequest("POST", "/config?dryRun=1", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		// dryRun should short-circuit before engine use
		api.handleImportConfig(rec, req, nil)

		assert.Equal(t, http.StatusOK, rec.Code)
		var resp map[string]any
		err := json.Unmarshal(rec.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.Equal(t, true, resp["dryRun"])
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

func TestBuildRequestFilter_ParsesStatusAndHasError(t *testing.T) {
	q := url.Values{}
	q.Set("status", "500")
	q.Set("hasError", "1")
	q.Set("method", "GET")

	filter := buildRequestFilter(q)
	assert.Equal(t, 500, filter.StatusCode)
	if assert.NotNil(t, filter.HasError) {
		assert.True(t, *filter.HasError)
	}
	assert.Equal(t, "GET", filter.Method)
}

func TestBuildRequestFilter_IgnoresInvalidHasError(t *testing.T) {
	q := url.Values{}
	q.Set("hasError", "sometimes")

	filter := buildRequestFilter(q)
	assert.Nil(t, filter.HasError)
}

func TestBuildRequestFilter_IgnoresInvalidLimitOffset(t *testing.T) {
	q := url.Values{}
	q.Set("limit", "10x")
	q.Set("offset", "5x")

	filter := buildRequestFilter(q)
	assert.Equal(t, 0, filter.Limit)
	assert.Equal(t, 0, filter.Offset)
}

func TestBuildRequestFilter_IgnoresNegativeLimitOffset(t *testing.T) {
	q := url.Values{}
	q.Set("limit", "-1")
	q.Set("offset", "-2")

	filter := buildRequestFilter(q)
	assert.Equal(t, 0, filter.Limit)
	assert.Equal(t, 0, filter.Offset)
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
