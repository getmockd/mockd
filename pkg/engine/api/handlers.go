package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/getmockd/mockd/pkg/config"
	"github.com/getmockd/mockd/pkg/httputil"
	"github.com/getmockd/mockd/pkg/requestlog"
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
		protocols[k] = v
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
	limitedBody(w, r)
	var req DeployRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "invalid JSON in request body")
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
	limitedBody(w, r)
	var cfg config.MockConfiguration
	if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "invalid JSON in request body")
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
	limitedBody(w, r)
	id := r.PathValue("id")

	// Check if mock exists
	existing := s.engine.GetMock(id)
	if existing == nil {
		writeError(w, http.StatusNotFound, "not_found", "mock not found")
		return
	}

	var cfg config.MockConfiguration
	if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "invalid JSON in request body")
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
	limitedBody(w, r)
	id := r.PathValue("id")

	// Check if mock exists
	existing := s.engine.GetMock(id)
	if existing == nil {
		writeError(w, http.StatusNotFound, "not_found", "mock not found")
		return
	}

	var req ToggleMockRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "invalid JSON in request body")
		return
	}

	// Update the mock with new enabled status
	existing.Enabled = &req.Enabled
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
	filter := &requestlog.Filter{
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
		filter.MatchedID = matched
	}

	// Parse protocol-specific filters
	if v := r.URL.Query().Get("grpcService"); v != "" {
		filter.GRPCService = v
	}
	if v := r.URL.Query().Get("mqttTopic"); v != "" {
		filter.MQTTTopic = v
	}
	if v := r.URL.Query().Get("mqttClientId"); v != "" {
		filter.MQTTClientID = v
	}
	if v := r.URL.Query().Get("soapOperation"); v != "" {
		filter.SOAPOperation = v
	}
	if v := r.URL.Query().Get("graphqlOpType"); v != "" {
		filter.GraphQLOpType = v
	}
	if v := r.URL.Query().Get("wsConnectionId"); v != "" {
		filter.WSConnectionID = v
	}
	if v := r.URL.Query().Get("sseConnectionId"); v != "" {
		filter.SSEConnectionID = v
	}

	// Parse hasError filter
	if v := r.URL.Query().Get("hasError"); v != "" {
		hasError := v == "true"
		filter.HasError = &hasError
	}

	// Parse status code filter
	if v := r.URL.Query().Get("status"); v != "" {
		if code, err := strconv.Atoi(v); err == nil {
			filter.StatusCode = code
		}
	}

	entries := s.engine.GetRequestLogs(filter)
	total := s.engine.RequestLogCount()

	// Convert to response type — preserve all fields including protocol metadata
	requests := make([]*RequestLogEntry, len(entries))
	for i, e := range entries {
		requests[i] = entryToAPI(e)
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
	writeJSON(w, http.StatusOK, entryToAPI(entry))
}

// entryToAPI converts a requestlog.Entry to the API response type, preserving
// all fields including protocol-specific metadata end-to-end.
func entryToAPI(e *requestlog.Entry) *RequestLogEntry {
	return &RequestLogEntry{
		ID:            e.ID,
		Timestamp:     e.Timestamp,
		Protocol:      e.Protocol,
		Method:        e.Method,
		Path:          e.Path,
		QueryString:   e.QueryString,
		Headers:       e.Headers,
		Body:          e.Body,
		BodySize:      e.BodySize,
		RemoteAddr:    e.RemoteAddr,
		MatchedMockID: e.MatchedMockID,
		StatusCode:    e.ResponseStatus,
		ResponseBody:  e.ResponseBody,
		DurationMs:    e.DurationMs,
		Error:         e.Error,
		GRPC:          e.GRPC,
		WebSocket:     e.WebSocket,
		SSE:           e.SSE,
		MQTT:          e.MQTT,
		SOAP:          e.SOAP,
		GraphQL:       e.GraphQL,
	}
}

func (s *Server) handleClearRequests(w http.ResponseWriter, r *http.Request) {
	count := s.engine.RequestLogCount()
	s.engine.ClearRequestLogs()
	writeJSON(w, http.StatusOK, map[string]any{
		"cleared": count,
		"message": "request logs cleared",
	})
}

func (s *Server) handleClearRequestsByMockID(w http.ResponseWriter, r *http.Request) {
	mockID := r.PathValue("id")
	if mockID == "" {
		writeError(w, http.StatusBadRequest, "missing_id", "mock ID is required")
		return
	}
	count := s.engine.ClearRequestLogsByMockID(mockID)
	writeJSON(w, http.StatusOK, map[string]any{
		"cleared": count,
		"mockId":  mockID,
		"message": "invocations cleared",
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
	limitedBody(w, r)
	var cfg ChaosConfig
	if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "invalid JSON in request body")
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
	limitedBody(w, r)
	var req ResetStateRequest
	// Allow empty body to reset all resources, but reject malformed JSON
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && err != io.EOF {
		writeError(w, http.StatusBadRequest, "invalid_json", "invalid JSON in request body")
		return
	}

	resp, err := s.engine.ResetState(req.Resource)
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", "resource not found")
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleGetStateResource(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	resource, err := s.engine.GetStateResource(name)
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", "resource not found")
		return
	}
	writeJSON(w, http.StatusOK, resource)
}

func (s *Server) handleClearStateResource(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	count, err := s.engine.ClearStateResource(name)
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", "resource not found")
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
		writeError(w, http.StatusNotFound, "not_found", "SSE connection not found")
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
		writeError(w, http.StatusNotFound, "not_found", "WebSocket connection not found")
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
	limitedBody(w, r)
	var req ImportConfigRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "invalid JSON in request body")
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

	// Import mocks, collecting per-mock errors so the caller knows which
	// mocks failed and why (e.g., proto file not found, port in use).
	imported := 0
	var importErrors []map[string]string
	for i, mock := range req.Config.Mocks {
		if err := s.engine.AddMock(mock); err != nil {
			s.log.Warn("failed to import mock", "id", mock.ID, "index", i, "error", err)
			importErrors = append(importErrors, map[string]string{
				"index": strconv.Itoa(i),
				"id":    mock.ID,
				"error": err.Error(),
			})
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
		"total":    len(req.Config.Mocks),
		"message":  fmt.Sprintf("imported %d of %d mocks", imported, len(req.Config.Mocks)),
	}
	if statefulCount > 0 {
		response["statefulResources"] = statefulCount
	}
	if len(importErrors) > 0 {
		response["errors"] = importErrors
	}

	writeJSON(w, http.StatusOK, response)
}

// Helpers

// maxRequestBodySize is the maximum allowed request body size (10 MB).
// This prevents denial-of-service via oversized payloads on the control API.
const maxRequestBodySize = 10 * 1024 * 1024

// limitedBody wraps r.Body with http.MaxBytesReader to enforce body size limits.
// Must be called before reading r.Body in any handler that accepts a request body.
func limitedBody(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodySize)
}

// writeJSON writes a JSON response using the shared httputil package.
// This ensures Content-Type is always set correctly.
func writeJSON(w http.ResponseWriter, status int, v any) {
	httputil.WriteJSON(w, status, v)
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	httputil.WriteJSON(w, status, ErrorResponse{
		Error:   code,
		Message: message,
	})
}
