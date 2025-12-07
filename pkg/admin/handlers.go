package admin

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/getmockd/mockd/pkg/config"
	"github.com/getmockd/mockd/pkg/engine"
)

// ErrorResponse represents an error response.
type ErrorResponse struct {
	Error   string      `json:"error"`
	Message string      `json:"message"`
	Details interface{} `json:"details,omitempty"`
}

// HealthResponse represents a health check response.
type HealthResponse struct {
	Status string `json:"status"`
	Uptime int    `json:"uptime"`
}

// MockListResponse represents a list of mocks response.
type MockListResponse struct {
	Mocks []*config.MockConfiguration `json:"mocks"`
	Count int                         `json:"count"`
}

// writeJSON writes a JSON response.
func writeJSON(w http.ResponseWriter, status int, data interface{}) {
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

// handleListMocks handles GET /mocks.
func (a *AdminAPI) handleListMocks(w http.ResponseWriter, r *http.Request) {
	mocks := a.server.ListMocks()

	// Filter by enabled status if specified
	enabledParam := r.URL.Query().Get("enabled")
	if enabledParam != "" {
		enabled := enabledParam == "true"
		filtered := make([]*config.MockConfiguration, 0)
		for _, mock := range mocks {
			if mock.Enabled == enabled {
				filtered = append(filtered, mock)
			}
		}
		mocks = filtered
	}

	writeJSON(w, http.StatusOK, MockListResponse{
		Mocks: mocks,
		Count: len(mocks),
	})
}

// handleCreateMock handles POST /mocks.
func (a *AdminAPI) handleCreateMock(w http.ResponseWriter, r *http.Request) {
	var mock config.MockConfiguration
	if err := json.NewDecoder(r.Body).Decode(&mock); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "Failed to parse request body: "+err.Error())
		return
	}

	// Set defaults
	if !mock.Enabled {
		mock.Enabled = true
	}
	now := time.Now()
	mock.CreatedAt = now
	mock.UpdatedAt = now

	if err := a.server.AddMock(&mock); err != nil {
		// Check if it's a duplicate ID error
		if a.server.GetMock(mock.ID) != nil {
			writeError(w, http.StatusConflict, "duplicate_id", "Mock with this ID already exists")
			return
		}
		writeError(w, http.StatusBadRequest, "validation_error", err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, mock)
}

// handleGetMock handles GET /mocks/{id}.
func (a *AdminAPI) handleGetMock(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "missing_id", "Mock ID is required")
		return
	}

	mock := a.server.GetMock(id)
	if mock == nil {
		writeError(w, http.StatusNotFound, "not_found", "Mock not found")
		return
	}

	writeJSON(w, http.StatusOK, mock)
}

// handleUpdateMock handles PUT /mocks/{id}.
func (a *AdminAPI) handleUpdateMock(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "missing_id", "Mock ID is required")
		return
	}

	var mock config.MockConfiguration
	if err := json.NewDecoder(r.Body).Decode(&mock); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "Failed to parse request body: "+err.Error())
		return
	}

	if err := a.server.UpdateMock(id, &mock); err != nil {
		if a.server.GetMock(id) == nil {
			writeError(w, http.StatusNotFound, "not_found", "Mock not found")
			return
		}
		writeError(w, http.StatusBadRequest, "validation_error", err.Error())
		return
	}

	// Return the updated mock
	updated := a.server.GetMock(id)
	writeJSON(w, http.StatusOK, updated)
}

// handleDeleteMock handles DELETE /mocks/{id}.
func (a *AdminAPI) handleDeleteMock(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "missing_id", "Mock ID is required")
		return
	}

	if err := a.server.DeleteMock(id); err != nil {
		writeError(w, http.StatusNotFound, "not_found", "Mock not found")
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
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "missing_id", "Mock ID is required")
		return
	}

	var req ToggleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "Failed to parse request body: "+err.Error())
		return
	}

	mock := a.server.GetMock(id)
	if mock == nil {
		writeError(w, http.StatusNotFound, "not_found", "Mock not found")
		return
	}

	// Update enabled status
	mock.Enabled = req.Enabled
	mock.UpdatedAt = time.Now()

	if err := a.server.Store().Set(mock); err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
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
	Requests []*config.RequestLogEntry `json:"requests"`
	Count    int                       `json:"count"`
	Total    int                       `json:"total"`
}

// handleExportConfig handles GET /config.
func (a *AdminAPI) handleExportConfig(w http.ResponseWriter, r *http.Request) {
	name := r.URL.Query().Get("name")
	if name == "" {
		name = "mockd-export"
	}

	collection := a.server.ExportConfig(name)
	writeJSON(w, http.StatusOK, collection)
}

// handleImportConfig handles POST /config.
func (a *AdminAPI) handleImportConfig(w http.ResponseWriter, r *http.Request) {
	var req ConfigImportRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "Failed to parse request body: "+err.Error())
		return
	}

	if req.Config == nil {
		writeError(w, http.StatusBadRequest, "missing_config", "config field is required")
		return
	}

	if err := a.server.ImportConfig(req.Config, req.Replace); err != nil {
		writeError(w, http.StatusBadRequest, "import_error", err.Error())
		return
	}

	// Return the current state after import
	collection := a.server.ExportConfig("imported")
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"message":  "Configuration imported successfully",
		"imported": len(req.Config.Mocks),
		"total":    len(collection.Mocks),
	})
}

// handleListRequests handles GET /requests.
func (a *AdminAPI) handleListRequests(w http.ResponseWriter, r *http.Request) {
	filter := &engine.RequestLogFilter{}

	// Parse query parameters
	if method := r.URL.Query().Get("method"); method != "" {
		filter.Method = method
	}
	if path := r.URL.Query().Get("path"); path != "" {
		filter.Path = path
	}
	if matched := r.URL.Query().Get("matched"); matched != "" {
		filter.MatchedID = matched
	}
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		var limit int
		if _, err := fmt.Sscanf(limitStr, "%d", &limit); err == nil {
			filter.Limit = limit
		}
	}
	if offsetStr := r.URL.Query().Get("offset"); offsetStr != "" {
		var offset int
		if _, err := fmt.Sscanf(offsetStr, "%d", &offset); err == nil {
			filter.Offset = offset
		}
	}

	requests := a.server.GetRequestLogs(filter)
	total := a.server.RequestLogCount()

	writeJSON(w, http.StatusOK, RequestLogListResponse{
		Requests: requests,
		Count:    len(requests),
		Total:    total,
	})
}

// handleGetRequest handles GET /requests/{id}.
func (a *AdminAPI) handleGetRequest(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "missing_id", "Request ID is required")
		return
	}

	entry := a.server.GetRequestLog(id)
	if entry == nil {
		writeError(w, http.StatusNotFound, "not_found", "Request log not found")
		return
	}

	writeJSON(w, http.StatusOK, entry)
}

// handleClearRequests handles DELETE /requests.
func (a *AdminAPI) handleClearRequests(w http.ResponseWriter, r *http.Request) {
	count := a.server.RequestLogCount()
	a.server.ClearRequestLogs()

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"message": "Request logs cleared",
		"cleared": count,
	})
}
