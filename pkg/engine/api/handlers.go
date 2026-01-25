package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/getmockd/mockd/pkg/config"
)

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, HealthResponse{
		Status:    "healthy",
		Timestamp: time.Now().UTC(),
	})
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	uptime := s.engine.Uptime()

	// Convert engine protocol status to API protocol status
	engineProtocols := s.engine.ProtocolStatus()
	protocols := make(map[string]ProtocolStatus, len(engineProtocols))
	for k, v := range engineProtocols {
		protocols[k] = ProtocolStatus(v)
	}

	resp := StatusResponse{
		Status:       "running",
		Uptime:       int64(uptime),
		MockCount:    len(s.engine.ListMocks()),
		RequestCount: int64(s.engine.RequestLogCount()),
		Protocols:    protocols,
		StartedAt:    time.Now().Add(-time.Duration(uptime) * time.Second),
	}
	if !s.engine.IsRunning() {
		resp.Status = "stopped"
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleDeploy(w http.ResponseWriter, r *http.Request) {
	var req DeployRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}

	if req.Replace {
		s.engine.ClearMocks()
	}

	deployed := 0
	for _, mock := range req.Mocks {
		if err := s.engine.AddMock(mock); err != nil {
			s.log.Warn("failed to deploy mock", "id", mock.ID, "error", err)
			continue
		}
		deployed++
	}

	writeJSON(w, http.StatusOK, DeployResponse{
		Deployed: deployed,
		Message:  fmt.Sprintf("deployed %d mocks", deployed),
	})
}

func (s *Server) handleUndeploy(w http.ResponseWriter, r *http.Request) {
	s.engine.ClearMocks()
	writeJSON(w, http.StatusOK, map[string]string{"message": "all mocks removed"})
}

func (s *Server) handleListMocks(w http.ResponseWriter, r *http.Request) {
	mocks := s.engine.ListMocks()
	writeJSON(w, http.StatusOK, MockListResponse{
		Mocks: mocks,
		Count: len(mocks),
	})
}

func (s *Server) handleGetMock(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	mock := s.engine.GetMock(id)
	if mock == nil {
		writeError(w, http.StatusNotFound, "not_found", "mock not found")
		return
	}
	writeJSON(w, http.StatusOK, mock)
}

func (s *Server) handleDeleteMock(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := s.engine.DeleteMock(id); err != nil {
		writeError(w, http.StatusNotFound, "not_found", "mock not found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleCreateMock(w http.ResponseWriter, r *http.Request) {
	var cfg config.MockConfiguration
	if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}

	// Check for duplicate ID
	if cfg.ID != "" && s.engine.GetMock(cfg.ID) != nil {
		writeError(w, http.StatusConflict, "duplicate_id", "mock with this ID already exists")
		return
	}

	if err := s.engine.AddMock(&cfg); err != nil {
		writeError(w, http.StatusBadRequest, "validation_error", err.Error())
		return
	}

	// Return the created mock
	created := s.engine.GetMock(cfg.ID)
	if created == nil {
		created = &cfg
	}
	writeJSON(w, http.StatusCreated, created)
}

func (s *Server) handleUpdateMock(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	// Check if mock exists
	existing := s.engine.GetMock(id)
	if existing == nil {
		writeError(w, http.StatusNotFound, "not_found", "mock not found")
		return
	}

	var cfg config.MockConfiguration
	if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}

	cfg.ID = id // Ensure ID matches path

	if err := s.engine.UpdateMock(id, &cfg); err != nil {
		writeError(w, http.StatusBadRequest, "validation_error", err.Error())
		return
	}

	// Return the updated mock
	updated := s.engine.GetMock(id)
	if updated == nil {
		updated = &cfg
	}
	writeJSON(w, http.StatusOK, updated)
}

func (s *Server) handleToggleMock(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	// Check if mock exists
	existing := s.engine.GetMock(id)
	if existing == nil {
		writeError(w, http.StatusNotFound, "not_found", "mock not found")
		return
	}

	var req ToggleMockRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}

	// Update the mock with new enabled status
	existing.Enabled = req.Enabled
	if err := s.engine.UpdateMock(id, existing); err != nil {
		writeError(w, http.StatusInternalServerError, "toggle_error", err.Error())
		return
	}

	// Return the updated mock
	updated := s.engine.GetMock(id)
	if updated == nil {
		updated = existing
	}
	writeJSON(w, http.StatusOK, updated)
}

func (s *Server) handleListRequests(w http.ResponseWriter, r *http.Request) {
	// Parse query params for filtering
	filter := &RequestLogFilter{
		Limit: 100, // default
	}

	// Parse limit
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if limit, err := strconv.Atoi(limitStr); err == nil && limit > 0 {
			filter.Limit = limit
		}
	}

	// Parse offset
	if offsetStr := r.URL.Query().Get("offset"); offsetStr != "" {
		if offset, err := strconv.Atoi(offsetStr); err == nil && offset >= 0 {
			filter.Offset = offset
		}
	}

	// Parse protocol filter
	if protocol := r.URL.Query().Get("protocol"); protocol != "" {
		filter.Protocol = protocol
	}

	// Parse method filter
	if method := r.URL.Query().Get("method"); method != "" {
		filter.Method = method
	}

	// Parse path filter
	if path := r.URL.Query().Get("path"); path != "" {
		filter.Path = path
	}

	// Parse matched mock ID filter
	if matched := r.URL.Query().Get("matched"); matched != "" {
		filter.MockID = matched
	}

	entries := s.engine.GetRequestLogs(filter)
	total := s.engine.RequestLogCount()

	// Convert to response type
	requests := make([]*RequestLogEntry, len(entries))
	for i, e := range entries {
		requests[i] = &RequestLogEntry{
			ID:            e.ID,
			Timestamp:     e.Timestamp,
			Protocol:      e.Protocol,
			Method:        e.Method,
			Path:          e.Path,
			MatchedMockID: e.MatchedMockID,
			StatusCode:    e.ResponseStatus,
			DurationMs:    e.DurationMs,
		}
	}

	writeJSON(w, http.StatusOK, RequestListResponse{
		Requests: requests,
		Count:    len(requests),
		Total:    total,
	})
}

func (s *Server) handleGetRequest(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	entry := s.engine.GetRequestLog(id)
	if entry == nil {
		writeError(w, http.StatusNotFound, "not_found", "request not found")
		return
	}
	// Convert headers from multi-value to single-value for API response
	headers := make(map[string]string, len(entry.Headers))
	for k, v := range entry.Headers {
		if len(v) > 0 {
			headers[k] = v[0]
		}
	}
	// Convert to API response type for consistent JSON field names
	// Include body and headers for single request detail view
	apiEntry := &RequestLogEntry{
		ID:            entry.ID,
		Timestamp:     entry.Timestamp,
		Protocol:      entry.Protocol,
		Method:        entry.Method,
		Path:          entry.Path,
		Headers:       headers,
		Body:          entry.Body,
		MatchedMockID: entry.MatchedMockID,
		StatusCode:    entry.ResponseStatus,
		DurationMs:    entry.DurationMs,
	}
	writeJSON(w, http.StatusOK, apiEntry)
}

func (s *Server) handleClearRequests(w http.ResponseWriter, r *http.Request) {
	count := s.engine.RequestLogCount()
	s.engine.ClearRequestLogs()
	writeJSON(w, http.StatusOK, map[string]any{
		"cleared": count,
		"message": "request logs cleared",
	})
}

func (s *Server) handleListProtocols(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.engine.ProtocolStatus())
}

// Chaos handlers

func (s *Server) handleGetChaos(w http.ResponseWriter, r *http.Request) {
	cfg := s.engine.GetChaosConfig()
	if cfg == nil {
		writeJSON(w, http.StatusOK, ChaosConfig{Enabled: false})
		return
	}
	writeJSON(w, http.StatusOK, cfg)
}

func (s *Server) handleSetChaos(w http.ResponseWriter, r *http.Request) {
	var cfg ChaosConfig
	if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}
	if err := s.engine.SetChaosConfig(&cfg); err != nil {
		writeError(w, http.StatusInternalServerError, "chaos_error", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, cfg)
}

func (s *Server) handleGetChaosStats(w http.ResponseWriter, r *http.Request) {
	stats := s.engine.GetChaosStats()
	if stats == nil {
		writeJSON(w, http.StatusOK, ChaosStats{FaultsByType: make(map[string]int64)})
		return
	}
	writeJSON(w, http.StatusOK, stats)
}

func (s *Server) handleResetChaosStats(w http.ResponseWriter, r *http.Request) {
	s.engine.ResetChaosStats()
	writeJSON(w, http.StatusOK, map[string]string{"message": "chaos stats reset"})
}

// State handlers

func (s *Server) handleGetState(w http.ResponseWriter, r *http.Request) {
	overview := s.engine.GetStateOverview()
	if overview == nil {
		writeJSON(w, http.StatusOK, StateOverview{
			Resources:    []StatefulResource{},
			ResourceList: []string{},
		})
		return
	}
	writeJSON(w, http.StatusOK, overview)
}

func (s *Server) handleResetState(w http.ResponseWriter, r *http.Request) {
	var req ResetStateRequest
	// Allow empty body to reset all resources, but reject malformed JSON
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && err != io.EOF {
		writeError(w, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}

	resp, err := s.engine.ResetState(req.Resource)
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleGetStateResource(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	resource, err := s.engine.GetStateResource(name)
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, resource)
}

func (s *Server) handleClearStateResource(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	count, err := s.engine.ClearStateResource(name)
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"cleared":  count,
		"resource": name,
		"message":  "resource cleared",
	})
}

// Protocol handler handlers

func (s *Server) handleListHandlers(w http.ResponseWriter, r *http.Request) {
	handlers := s.engine.ListProtocolHandlers()
	writeJSON(w, http.StatusOK, ProtocolHandlerListResponse{
		Handlers: handlers,
		Count:    len(handlers),
	})
}

func (s *Server) handleGetHandler(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	handler := s.engine.GetProtocolHandler(id)
	if handler == nil {
		writeError(w, http.StatusNotFound, "not_found", "handler not found")
		return
	}
	writeJSON(w, http.StatusOK, handler)
}

// SSE handlers

func (s *Server) handleListSSEConnections(w http.ResponseWriter, r *http.Request) {
	connections := s.engine.ListSSEConnections()
	writeJSON(w, http.StatusOK, SSEConnectionListResponse{
		Connections: connections,
		Count:       len(connections),
	})
}

func (s *Server) handleGetSSEConnection(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	conn := s.engine.GetSSEConnection(id)
	if conn == nil {
		writeError(w, http.StatusNotFound, "not_found", "SSE connection not found")
		return
	}
	writeJSON(w, http.StatusOK, conn)
}

func (s *Server) handleCloseSSEConnection(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := s.engine.CloseSSEConnection(id); err != nil {
		writeError(w, http.StatusNotFound, "not_found", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{
		"message": "SSE connection closed",
		"id":      id,
	})
}

func (s *Server) handleGetSSEStats(w http.ResponseWriter, r *http.Request) {
	stats := s.engine.GetSSEStats()
	if stats == nil {
		writeJSON(w, http.StatusOK, SSEStats{ConnectionsByMock: make(map[string]int)})
		return
	}
	writeJSON(w, http.StatusOK, stats)
}

// WebSocket handlers

func (s *Server) handleListWebSocketConnections(w http.ResponseWriter, r *http.Request) {
	connections := s.engine.ListWebSocketConnections()
	writeJSON(w, http.StatusOK, WebSocketConnectionListResponse{
		Connections: connections,
		Count:       len(connections),
	})
}

func (s *Server) handleGetWebSocketConnection(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	conn := s.engine.GetWebSocketConnection(id)
	if conn == nil {
		writeError(w, http.StatusNotFound, "not_found", "WebSocket connection not found")
		return
	}
	writeJSON(w, http.StatusOK, conn)
}

func (s *Server) handleCloseWebSocketConnection(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := s.engine.CloseWebSocketConnection(id); err != nil {
		writeError(w, http.StatusNotFound, "not_found", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{
		"message": "WebSocket connection closed",
		"id":      id,
	})
}

func (s *Server) handleGetWebSocketStats(w http.ResponseWriter, r *http.Request) {
	stats := s.engine.GetWebSocketStats()
	if stats == nil {
		writeJSON(w, http.StatusOK, WebSocketStats{ConnectionsByMock: make(map[string]int)})
		return
	}
	writeJSON(w, http.StatusOK, stats)
}

// Config handlers

func (s *Server) handleGetConfig(w http.ResponseWriter, r *http.Request) {
	cfg := s.engine.GetConfig()
	if cfg == nil {
		writeJSON(w, http.StatusOK, ConfigResponse{})
		return
	}
	writeJSON(w, http.StatusOK, cfg)
}

func (s *Server) handleExportMocks(w http.ResponseWriter, r *http.Request) {
	name := r.URL.Query().Get("name")
	if name == "" {
		name = "mockd-export"
	}

	mocks := s.engine.ListMocks()

	collection := &config.MockCollection{
		Version: "1.0",
		Name:    name,
		Mocks:   mocks,
	}

	writeJSON(w, http.StatusOK, collection)
}

func (s *Server) handleImportConfig(w http.ResponseWriter, r *http.Request) {
	var req ImportConfigRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}

	if req.Config == nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "config is required")
		return
	}

	// Clear existing mocks if replace is true
	if req.Replace {
		s.engine.ClearMocks()
	}

	// Import mocks
	imported := 0
	for _, mock := range req.Config.Mocks {
		if err := s.engine.AddMock(mock); err != nil {
			s.log.Warn("failed to import mock", "id", mock.ID, "error", err)
			continue
		}
		imported++
	}

	// Import stateful resources
	statefulCount := 0
	for _, res := range req.Config.StatefulResources {
		if res != nil {
			if err := s.engine.RegisterStatefulResource(res); err != nil {
				s.log.Warn("failed to import stateful resource", "name", res.Name, "error", err)
				continue
			}
			statefulCount++
		}
	}

	response := map[string]any{
		"imported": imported,
		"message":  fmt.Sprintf("imported %d mocks", imported),
	}
	if statefulCount > 0 {
		response["statefulResources"] = statefulCount
	}

	writeJSON(w, http.StatusOK, response)
}

// Helpers

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(ErrorResponse{
		Error:   code,
		Message: message,
	})
}
