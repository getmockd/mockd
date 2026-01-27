package admin

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"time"

	"github.com/getmockd/mockd/pkg/admin/engineclient"
	"github.com/getmockd/mockd/pkg/config"
	"github.com/getmockd/mockd/pkg/requestlog"
)

// ErrorResponse represents an error response.
type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
	Details any    `json:"details,omitempty"`
}

// HealthResponse represents a health check response.
type HealthResponse struct {
	Status string `json:"status"`
	Uptime int    `json:"uptime"`
}

// ServerStatus represents a detailed server status response.
type ServerStatus struct {
	Status       string `json:"status"`
	HTTPPort     int    `json:"httpPort"`
	HTTPSPort    int    `json:"httpsPort,omitempty"`
	AdminPort    int    `json:"adminPort"`
	Uptime       int64  `json:"uptime"`
	MockCount    int    `json:"mockCount"`
	ActiveMocks  int    `json:"activeMocks"`
	RequestCount int64  `json:"requestCount"`
	TLSEnabled   bool   `json:"tlsEnabled"`
	Version      string `json:"version"`
}

// MockListResponse represents a list of mocks response.
type MockListResponse struct {
	Mocks []*config.MockConfiguration `json:"mocks"`
	Count int                         `json:"count"`
}

// writeJSON writes a JSON response.
func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if data != nil {
		_ = json.NewEncoder(w).Encode(data)
	}
}

// writeError writes an error response.
func writeError(w http.ResponseWriter, status int, errCode, message string) {
	writeJSON(w, status, ErrorResponse{
		Error:   errCode,
		Message: message,
	})
}

// handleHealth handles GET /health.
func (a *AdminAPI) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, HealthResponse{
		Status: "ok",
		Uptime: a.Uptime(),
	})
}

// handleGetStatus handles GET /status and returns detailed server status.
func (a *AdminAPI) handleGetStatus(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if a.localEngine == nil {
		writeError(w, http.StatusServiceUnavailable, "no_engine", "No engine connected")
		return
	}

	engineStatus, err := a.localEngine.Status(ctx)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "engine_unavailable", sanitizeEngineError(err, a.log, "get engine status"))
		return
	}

	// Count active mocks from engine status
	mocks, err := a.localEngine.ListMocks(ctx)
	if err != nil {
		// Log the error but continue with zero active mocks count
		// This is non-critical for the status endpoint
		mocks = nil
	}
	activeMocks := 0
	for _, mock := range mocks {
		if mock.Enabled == nil || *mock.Enabled {
			activeMocks++
		}
	}

	version := a.version
	if version == "" {
		version = "dev"
	}

	writeJSON(w, http.StatusOK, ServerStatus{
		Status:       engineStatus.Status,
		HTTPPort:     0, // TODO: Get from engine config when available via HTTP
		HTTPSPort:    0,
		AdminPort:    a.port,
		Uptime:       engineStatus.Uptime,
		MockCount:    engineStatus.MockCount,
		ActiveMocks:  activeMocks,
		RequestCount: engineStatus.RequestCount,
		TLSEnabled:   false,
		Version:      version,
	})
}

// handleListMocks handles GET /mocks.
func (a *AdminAPI) handleListMocks(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if a.localEngine == nil {
		writeError(w, http.StatusServiceUnavailable, "no_engine", "No engine connected")
		return
	}

	mocks, err := a.localEngine.ListMocks(ctx)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "engine_unavailable", sanitizeEngineError(err, a.log, "list mocks"))
		return
	}

	// Filter by enabled status if specified
	enabledParam := r.URL.Query().Get("enabled")
	if enabledParam != "" {
		enabled := enabledParam == "true"
		filtered := make([]*config.MockConfiguration, 0)
		for _, mock := range mocks {
			mockEnabled := mock.Enabled == nil || *mock.Enabled
			if mockEnabled == enabled {
				filtered = append(filtered, mock)
			}
		}
		mocks = filtered
	}

	// Filter by parentId if specified
	parentIDParam := r.URL.Query().Get("parentId")
	if parentIDParam != "" {
		filtered := make([]*config.MockConfiguration, 0)
		for _, mock := range mocks {
			if mock.ParentID == parentIDParam {
				filtered = append(filtered, mock)
			}
		}
		mocks = filtered
	}

	// Sort by metaSortKey (ascending order - lower values first, including negative for newest-first)
	sortParam := r.URL.Query().Get("sort")
	if sortParam == "" || sortParam == "metaSortKey" {
		sortMocksByMetaSortKey(mocks)
	}

	writeJSON(w, http.StatusOK, MockListResponse{
		Mocks: mocks,
		Count: len(mocks),
	})
}

// sortMocksByMetaSortKey sorts mocks by their MetaSortKey in ascending order.
func sortMocksByMetaSortKey(mocks []*config.MockConfiguration) {
	sort.Slice(mocks, func(i, j int) bool {
		return mocks[i].MetaSortKey < mocks[j].MetaSortKey
	})
}

// handleCreateMock handles POST /mocks.
func (a *AdminAPI) handleCreateMock(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if a.localEngine == nil {
		writeError(w, http.StatusServiceUnavailable, "no_engine", "No engine connected")
		return
	}

	var mock config.MockConfiguration
	if err := json.NewDecoder(r.Body).Decode(&mock); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", sanitizeJSONError(err, a.log))
		return
	}

	// Set defaults
	if mock.Enabled == nil {
		enabled := true
		mock.Enabled = &enabled
	}
	now := time.Now()
	mock.CreatedAt = now
	mock.UpdatedAt = now

	// Set default metaSortKey to negative timestamp if not provided (newest first)
	if mock.MetaSortKey == 0 {
		mock.MetaSortKey = -float64(now.UnixMilli())
	}

	created, err := a.localEngine.CreateMock(ctx, &mock)
	if err != nil {
		if errors.Is(err, engineclient.ErrDuplicate) {
			writeError(w, http.StatusConflict, "duplicate_id", ErrMsgConflict)
			return
		}
		writeError(w, http.StatusBadRequest, "validation_error", sanitizeValidationError(err, a.log))
		return
	}
	writeJSON(w, http.StatusCreated, created)
}

// handleGetMock handles GET /mocks/{id}.
func (a *AdminAPI) handleGetMock(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "missing_id", "Mock ID is required")
		return
	}

	if a.localEngine == nil {
		writeError(w, http.StatusServiceUnavailable, "no_engine", "No engine connected")
		return
	}

	mock, err := a.localEngine.GetMock(ctx, id)
	if err != nil {
		if errors.Is(err, engineclient.ErrNotFound) {
			writeError(w, http.StatusNotFound, "not_found", ErrMsgNotFound)
			return
		}
		writeError(w, http.StatusServiceUnavailable, "engine_unavailable", sanitizeEngineError(err, a.log, "get mock"))
		return
	}
	writeJSON(w, http.StatusOK, mock)
}

// handleUpdateMock handles PUT /mocks/{id}.
func (a *AdminAPI) handleUpdateMock(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "missing_id", "Mock ID is required")
		return
	}

	if a.localEngine == nil {
		writeError(w, http.StatusServiceUnavailable, "no_engine", "No engine connected")
		return
	}

	var mock config.MockConfiguration
	if err := json.NewDecoder(r.Body).Decode(&mock); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", sanitizeJSONError(err, a.log))
		return
	}

	updated, err := a.localEngine.UpdateMock(ctx, id, &mock)
	if err != nil {
		if errors.Is(err, engineclient.ErrNotFound) {
			writeError(w, http.StatusNotFound, "not_found", ErrMsgNotFound)
			return
		}
		writeError(w, http.StatusBadRequest, "validation_error", sanitizeValidationError(err, a.log))
		return
	}
	writeJSON(w, http.StatusOK, updated)
}

// handleDeleteMock handles DELETE /mocks/{id}.
func (a *AdminAPI) handleDeleteMock(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "missing_id", "Mock ID is required")
		return
	}

	if a.localEngine == nil {
		writeError(w, http.StatusServiceUnavailable, "no_engine", "No engine connected")
		return
	}

	if err := a.localEngine.DeleteMock(ctx, id); err != nil {
		if errors.Is(err, engineclient.ErrNotFound) {
			writeError(w, http.StatusNotFound, "not_found", ErrMsgNotFound)
			return
		}
		writeError(w, http.StatusServiceUnavailable, "engine_unavailable", sanitizeEngineError(err, a.log, "delete mock"))
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ToggleRequest represents a toggle mock request.
type ToggleRequest struct {
	Enabled bool `json:"enabled"`
}

// handleToggleMock handles POST /mocks/{id}/toggle.
func (a *AdminAPI) handleToggleMock(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "missing_id", "Mock ID is required")
		return
	}

	if a.localEngine == nil {
		writeError(w, http.StatusServiceUnavailable, "no_engine", "No engine connected")
		return
	}

	var req ToggleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", sanitizeJSONError(err, a.log))
		return
	}

	mock, err := a.localEngine.ToggleMock(ctx, id, req.Enabled)
	if err != nil {
		if errors.Is(err, engineclient.ErrNotFound) {
			writeError(w, http.StatusNotFound, "not_found", ErrMsgNotFound)
			return
		}
		writeError(w, http.StatusInternalServerError, "internal_error", sanitizeError(err, a.log, "toggle mock", "id", id))
		return
	}
	writeJSON(w, http.StatusOK, mock)
}

// ConfigImportRequest represents a config import request.
type ConfigImportRequest struct {
	Replace bool                   `json:"replace"`
	Config  *config.MockCollection `json:"config"`
}

// RequestLogListResponse represents a list of request logs response.
type RequestLogListResponse struct {
	Requests []*requestlog.Entry `json:"requests"`
	Count    int                 `json:"count"`
	Total    int                 `json:"total"`
}

// handleExportConfig handles GET /config.
func (a *AdminAPI) handleExportConfig(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	name := r.URL.Query().Get("name")
	if name == "" {
		name = "mockd-export"
	}

	if a.localEngine == nil {
		writeError(w, http.StatusServiceUnavailable, "no_engine", "No engine connected")
		return
	}

	collection, err := a.localEngine.ExportConfig(ctx, name)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "engine_unavailable", sanitizeEngineError(err, a.log, "export config"))
		return
	}
	writeJSON(w, http.StatusOK, collection)
}

// handleImportConfig handles POST /config.
func (a *AdminAPI) handleImportConfig(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var req ConfigImportRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", sanitizeJSONError(err, a.log))
		return
	}

	if req.Config == nil {
		writeError(w, http.StatusBadRequest, "missing_config", "config field is required")
		return
	}

	if a.localEngine == nil {
		writeError(w, http.StatusServiceUnavailable, "no_engine", "No engine connected")
		return
	}

	if err := a.localEngine.ImportConfig(ctx, req.Config, req.Replace); err != nil {
		writeError(w, http.StatusBadRequest, "import_error", sanitizeError(err, a.log, "import config"))
		return
	}
	// Get the updated state
	collection, _ := a.localEngine.ExportConfig(ctx, "imported")
	total := 0
	if collection != nil {
		total = len(collection.Mocks)
	}
	response := map[string]any{
		"message":  "Configuration imported successfully",
		"imported": len(req.Config.Mocks),
		"total":    total,
	}
	if len(req.Config.StatefulResources) > 0 {
		response["statefulResources"] = len(req.Config.StatefulResources)
	}
	writeJSON(w, http.StatusOK, response)
}

// handleListRequests handles GET /requests.
// Supports filtering by protocol, method, path, and protocol-specific fields.
//
// Query Parameters:
//   - protocol: Filter by protocol (http, grpc, websocket, sse, mqtt, soap, graphql)
//   - method: Filter by method (HTTP method, gRPC method, MQTT PUBLISH/SUBSCRIBE, etc.)
//   - path: Filter by path prefix (or topic pattern for MQTT)
//   - matched: Filter by matched mock ID
//   - status: Filter by response status code
//   - hasError: Filter by error presence (true/false)
//   - limit: Maximum number of entries to return
//   - offset: Pagination offset
//
// Protocol-specific filters:
//   - grpcService: Filter gRPC by service name
//   - mqttTopic: Filter MQTT by topic (supports wildcards + and #)
//   - mqttClientId: Filter MQTT by client ID
//   - soapOperation: Filter SOAP by operation name
//   - graphqlOpType: Filter GraphQL by operation type (query, mutation, subscription)
//   - wsConnectionId: Filter WebSocket by connection ID
//   - sseConnectionId: Filter SSE by connection ID
func (a *AdminAPI) handleListRequests(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if a.localEngine == nil {
		writeError(w, http.StatusServiceUnavailable, "no_engine", "No engine connected")
		return
	}

	// Build filter from query parameters
	clientFilter := &engineclient.RequestFilter{}
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		var limit int
		if _, err := fmt.Sscanf(limitStr, "%d", &limit); err == nil {
			clientFilter.Limit = limit
		}
	}
	if offsetStr := r.URL.Query().Get("offset"); offsetStr != "" {
		var offset int
		if _, err := fmt.Sscanf(offsetStr, "%d", &offset); err == nil {
			clientFilter.Offset = offset
		}
	}
	if protocol := r.URL.Query().Get("protocol"); protocol != "" {
		clientFilter.Protocol = protocol
	}
	if method := r.URL.Query().Get("method"); method != "" {
		clientFilter.Method = method
	}
	if path := r.URL.Query().Get("path"); path != "" {
		clientFilter.Path = path
	}
	if matched := r.URL.Query().Get("matched"); matched != "" {
		clientFilter.MockID = matched
	}

	result, err := a.localEngine.ListRequests(ctx, clientFilter)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "engine_unavailable", sanitizeEngineError(err, a.log, "list requests"))
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// handleGetRequest handles GET /requests/{id}.
func (a *AdminAPI) handleGetRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "missing_id", "Request ID is required")
		return
	}

	if a.localEngine == nil {
		writeError(w, http.StatusServiceUnavailable, "no_engine", "No engine connected")
		return
	}

	entry, err := a.localEngine.GetRequest(ctx, id)
	if err != nil {
		if errors.Is(err, engineclient.ErrNotFound) {
			writeError(w, http.StatusNotFound, "not_found", ErrMsgNotFound)
			return
		}
		writeError(w, http.StatusServiceUnavailable, "engine_unavailable", sanitizeEngineError(err, a.log, "get request"))
		return
	}
	writeJSON(w, http.StatusOK, entry)
}

// handleClearRequests handles DELETE /requests.
func (a *AdminAPI) handleClearRequests(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if a.localEngine == nil {
		writeError(w, http.StatusServiceUnavailable, "no_engine", "No engine connected")
		return
	}

	count, err := a.localEngine.ClearRequests(ctx)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "engine_unavailable", sanitizeEngineError(err, a.log, "clear requests"))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"message": "Request logs cleared",
		"cleared": count,
	})
}

// handleStreamRequests handles GET /requests/stream - SSE endpoint for streaming new requests.
func (a *AdminAPI) handleStreamRequests(w http.ResponseWriter, r *http.Request) {
	if a.localEngine == nil {
		writeError(w, http.StatusServiceUnavailable, "no_engine", "No engine connected")
		return
	}

	// Set SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	// Use configured CORS settings instead of hardcoded "*"
	origin := r.Header.Get("Origin")
	if allowOrigin := a.corsConfig.getAllowOriginValue(origin); allowOrigin != "" {
		w.Header().Set("Access-Control-Allow-Origin", allowOrigin)
	}

	// Get the flusher
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "sse_error", "Streaming not supported")
		return
	}

	// Send initial connection message
	_, _ = fmt.Fprintf(w, "event: connected\ndata: {\"message\": \"Connected to request stream\"}\n\n")
	flusher.Flush()

	// Poll for request log updates
	ctx := r.Context()
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	lastID := ""
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// Get latest requests from engine
			filter := &engineclient.RequestFilter{Limit: 10}
			result, err := a.localEngine.ListRequests(ctx, filter)
			if err != nil {
				continue
			}

			// Send new entries (result.Requests is newest first)
			for i := len(result.Requests) - 1; i >= 0; i-- {
				entry := result.Requests[i]
				if entry.ID == lastID {
					break
				}
				if lastID == "" && i < len(result.Requests)-1 {
					// First iteration, only send most recent
					continue
				}

				data, _ := json.Marshal(entry)
				_, _ = fmt.Fprintf(w, "event: request\ndata: %s\n\n", data)
				flusher.Flush()

				if i == 0 {
					lastID = entry.ID
				}
			}
			if len(result.Requests) > 0 && lastID == "" {
				lastID = result.Requests[0].ID
			}
		}
	}
}
