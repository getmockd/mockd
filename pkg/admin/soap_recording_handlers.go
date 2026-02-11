package admin

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"sync"

	"github.com/getmockd/mockd/pkg/logging"
	"github.com/getmockd/mockd/pkg/recording"
	"github.com/getmockd/mockd/pkg/soap"
)

// SOAPRecordingManager manages SOAP recording operations for the Admin API.
type SOAPRecordingManager struct {
	mu       sync.RWMutex
	log      *slog.Logger
	store    *recording.SOAPStore
	handlers map[string]*soap.Handler // handler ID -> handler
}

// NewSOAPRecordingManager creates a new SOAP recording manager.
func NewSOAPRecordingManager() *SOAPRecordingManager {
	return &SOAPRecordingManager{
		log:      logging.Nop(),
		store:    recording.NewSOAPStore(1000),
		handlers: make(map[string]*soap.Handler),
	}
}

// SetLogger sets the logger under the manager's own lock.
func (m *SOAPRecordingManager) SetLogger(log *slog.Logger) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if log != nil {
		m.log = log
	} else {
		m.log = logging.Nop()
	}
}

// Store returns the SOAP recording store.
func (m *SOAPRecordingManager) Store() *recording.SOAPStore {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.store
}

// RegisterHandler registers a SOAP handler for recording management.
func (m *SOAPRecordingManager) RegisterHandler(handler *soap.Handler) {
	if handler == nil || handler.ID() == "" {
		return
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	m.handlers[handler.ID()] = handler
	// Set the recording store on the handler
	handler.SetRecordingStore(&soapStoreAdapter{store: m.store})
}

// UnregisterHandler removes a SOAP handler from recording management.
func (m *SOAPRecordingManager) UnregisterHandler(handlerID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if handler, ok := m.handlers[handlerID]; ok {
		handler.DisableRecording()
		delete(m.handlers, handlerID)
	}
}

// GetHandler returns a registered SOAP handler by ID.
func (m *SOAPRecordingManager) GetHandler(handlerID string) *soap.Handler {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.handlers[handlerID]
}

// ListHandlers returns all registered handler IDs.
func (m *SOAPRecordingManager) ListHandlers() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	ids := make([]string, 0, len(m.handlers))
	for id := range m.handlers {
		ids = append(ids, id)
	}
	return ids
}

// soapStoreAdapter adapts SOAPStore to the soap.SOAPRecordingStore interface.
type soapStoreAdapter struct {
	store *recording.SOAPStore
}

func (a *soapStoreAdapter) Add(data soap.SOAPRecordingData) error {
	// Convert from soap.SOAPRecordingData to recording.SOAPRecording
	rec := recording.NewSOAPRecording(data.Endpoint, data.Operation, data.SOAPVersion)
	rec.SetSOAPAction(data.SOAPAction)
	rec.SetRequestBody(data.RequestBody)
	rec.SetResponseBody(data.ResponseBody)
	rec.SetResponseStatus(data.ResponseStatus)
	rec.SetRequestHeaders(data.RequestHeaders)
	rec.SetResponseHeaders(data.ResponseHeaders)
	rec.SetDuration(data.Duration)
	if data.HasFault {
		rec.SetFault(data.FaultCode, data.FaultMessage)
	}
	return a.store.Add(rec)
}

// Request/Response types

// SOAPRecordingListResponse represents a list of SOAP recordings.
type SOAPRecordingListResponse struct {
	Recordings []*recording.SOAPRecording `json:"recordings"`
	Total      int                        `json:"total"`
	Limit      int                        `json:"limit,omitempty"`
	Offset     int                        `json:"offset,omitempty"`
}

// SOAPRecordingStatsResponse represents SOAP recording statistics.
type SOAPRecordingStatsResponse struct {
	*recording.SOAPRecordingStats
}

// SOAPHandlerStatusResponse represents the status of a SOAP handler.
type SOAPHandlerStatusResponse struct {
	ID               string `json:"id"`
	Path             string `json:"path"`
	RecordingEnabled bool   `json:"recordingEnabled"`
	OperationCount   int    `json:"operationCount"`
}

// SOAPRecordingStartResponse represents the response after starting recording.
type SOAPRecordingStartResponse struct {
	Message string `json:"message"`
	Enabled bool   `json:"enabled"`
}

// SOAPRecordingStopResponse represents the response after stopping recording.
type SOAPRecordingStopResponse struct {
	Message string `json:"message"`
	Enabled bool   `json:"enabled"`
}

// SOAPConvertRequest represents a request to convert recordings to mock config.
type SOAPConvertRequest struct {
	RecordingIDs   []string `json:"recordingIds,omitempty"`
	Endpoint       string   `json:"endpoint,omitempty"`
	Operation      string   `json:"operation,omitempty"`
	Deduplicate    bool     `json:"deduplicate,omitempty"`
	IncludeDelay   bool     `json:"includeDelay,omitempty"`
	PreserveFaults bool     `json:"preserveFaults,omitempty"`
}

// SOAPConvertResponse represents the result of converting recordings.
type SOAPConvertResponse struct {
	Config         *soap.SOAPConfig `json:"config"`
	OperationCount int              `json:"operationCount"`
	Total          int              `json:"total"`
	Warnings       []string         `json:"warnings,omitempty"`
}

// Handlers

// handleListSOAPRecordings handles GET /soap-recordings.
func (m *SOAPRecordingManager) handleListSOAPRecordings(w http.ResponseWriter, r *http.Request) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.store == nil {
		writeJSON(w, http.StatusOK, SOAPRecordingListResponse{
			Recordings: []*recording.SOAPRecording{},
			Total:      0,
		})
		return
	}

	filter := recording.SOAPRecordingFilter{}

	// Parse query parameters
	if endpoint := r.URL.Query().Get("endpoint"); endpoint != "" {
		filter.Endpoint = endpoint
	}
	if operation := r.URL.Query().Get("operation"); operation != "" {
		filter.Operation = operation
	}
	if soapAction := r.URL.Query().Get("soapAction"); soapAction != "" {
		filter.SOAPAction = soapAction
	}
	if hasFault := r.URL.Query().Get("hasFault"); hasFault != "" {
		b := hasFault == "true"
		filter.HasFault = &b
	}
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if limit, err := strconv.Atoi(limitStr); err == nil {
			filter.Limit = limit
		}
	}
	if offsetStr := r.URL.Query().Get("offset"); offsetStr != "" {
		if offset, err := strconv.Atoi(offsetStr); err == nil {
			filter.Offset = offset
		}
	}

	recordings, total := m.store.List(filter)

	writeJSON(w, http.StatusOK, SOAPRecordingListResponse{
		Recordings: recordings,
		Total:      total,
		Limit:      filter.Limit,
		Offset:     filter.Offset,
	})
}

// handleGetSOAPRecording handles GET /soap-recordings/{id}.
func (m *SOAPRecordingManager) handleGetSOAPRecording(w http.ResponseWriter, r *http.Request) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "missing_id", "Recording ID is required")
		return
	}

	if m.store == nil {
		writeError(w, http.StatusNotFound, "not_found", "Recording not found")
		return
	}

	rec := m.store.Get(id)
	if rec == nil {
		writeError(w, http.StatusNotFound, "not_found", "Recording not found")
		return
	}

	writeJSON(w, http.StatusOK, rec)
}

// handleDeleteSOAPRecording handles DELETE /soap-recordings/{id}.
func (m *SOAPRecordingManager) handleDeleteSOAPRecording(w http.ResponseWriter, r *http.Request) {
	m.mu.Lock()
	defer m.mu.Unlock()

	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "missing_id", "Recording ID is required")
		return
	}

	if m.store == nil {
		writeError(w, http.StatusNotFound, "not_found", "Recording not found")
		return
	}

	if err := m.store.Delete(id); err != nil {
		writeError(w, http.StatusNotFound, "not_found", "Recording not found")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// handleClearSOAPRecordings handles DELETE /soap-recordings.
func (m *SOAPRecordingManager) handleClearSOAPRecordings(w http.ResponseWriter, r *http.Request) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.store == nil {
		writeJSON(w, http.StatusOK, map[string]int{"deleted": 0})
		return
	}

	count := m.store.Clear()
	writeJSON(w, http.StatusOK, map[string]int{"deleted": count})
}

// handleGetSOAPRecordingStats handles GET /soap-recordings/stats.
func (m *SOAPRecordingManager) handleGetSOAPRecordingStats(w http.ResponseWriter, r *http.Request) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.store == nil {
		writeJSON(w, http.StatusOK, SOAPRecordingStatsResponse{
			SOAPRecordingStats: &recording.SOAPRecordingStats{},
		})
		return
	}

	stats := m.store.Stats()
	writeJSON(w, http.StatusOK, SOAPRecordingStatsResponse{
		SOAPRecordingStats: stats,
	})
}

// handleStartSOAPRecording handles POST /soap/{id}/record/start.
func (m *SOAPRecordingManager) handleStartSOAPRecording(w http.ResponseWriter, r *http.Request) {
	handlerID := r.PathValue("id")
	if handlerID == "" {
		writeError(w, http.StatusBadRequest, "missing_id", "Handler ID is required")
		return
	}

	m.mu.RLock()
	handler := m.handlers[handlerID]
	m.mu.RUnlock()

	if handler == nil {
		writeError(w, http.StatusNotFound, "not_found", fmt.Sprintf("SOAP handler '%s' not found", handlerID))
		return
	}

	handler.EnableRecording()

	writeJSON(w, http.StatusOK, SOAPRecordingStartResponse{
		Message: fmt.Sprintf("Recording enabled for SOAP handler '%s'", handlerID),
		Enabled: true,
	})
}

// handleStopSOAPRecording handles POST /soap/{id}/record/stop.
func (m *SOAPRecordingManager) handleStopSOAPRecording(w http.ResponseWriter, r *http.Request) {
	handlerID := r.PathValue("id")
	if handlerID == "" {
		writeError(w, http.StatusBadRequest, "missing_id", "Handler ID is required")
		return
	}

	m.mu.RLock()
	handler := m.handlers[handlerID]
	m.mu.RUnlock()

	if handler == nil {
		writeError(w, http.StatusNotFound, "not_found", fmt.Sprintf("SOAP handler '%s' not found", handlerID))
		return
	}

	handler.DisableRecording()

	writeJSON(w, http.StatusOK, SOAPRecordingStopResponse{
		Message: fmt.Sprintf("Recording disabled for SOAP handler '%s'", handlerID),
		Enabled: false,
	})
}

// handleGetSOAPHandlerStatus handles GET /soap/{id}/status.
func (m *SOAPRecordingManager) handleGetSOAPHandlerStatus(w http.ResponseWriter, r *http.Request) {
	handlerID := r.PathValue("id")
	if handlerID == "" {
		writeError(w, http.StatusBadRequest, "missing_id", "Handler ID is required")
		return
	}

	m.mu.RLock()
	handler := m.handlers[handlerID]
	m.mu.RUnlock()

	if handler == nil {
		writeError(w, http.StatusNotFound, "not_found", fmt.Sprintf("SOAP handler '%s' not found", handlerID))
		return
	}

	config := handler.GetConfig()
	operationCount := 0
	if config != nil && config.Operations != nil {
		operationCount = len(config.Operations)
	}

	path := ""
	if config != nil {
		path = config.Path
	}

	writeJSON(w, http.StatusOK, SOAPHandlerStatusResponse{
		ID:               handlerID,
		Path:             path,
		RecordingEnabled: handler.IsRecordingEnabled(),
		OperationCount:   operationCount,
	})
}

// handleListSOAPHandlers handles GET /soap.
func (m *SOAPRecordingManager) handleListSOAPHandlers(w http.ResponseWriter, r *http.Request) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	handlers := make([]SOAPHandlerStatusResponse, 0, len(m.handlers))
	for id, handler := range m.handlers {
		config := handler.GetConfig()
		operationCount := 0
		if config != nil && config.Operations != nil {
			operationCount = len(config.Operations)
		}

		path := ""
		if config != nil {
			path = config.Path
		}

		handlers = append(handlers, SOAPHandlerStatusResponse{
			ID:               id,
			Path:             path,
			RecordingEnabled: handler.IsRecordingEnabled(),
			OperationCount:   operationCount,
		})
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"handlers": handlers,
		"count":    len(handlers),
	})
}

// handleConvertSOAPRecording handles POST /soap-recordings/{id}/convert.
func (m *SOAPRecordingManager) handleConvertSOAPRecording(w http.ResponseWriter, r *http.Request) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "missing_id", "Recording ID is required")
		return
	}

	if m.store == nil {
		writeError(w, http.StatusNotFound, "not_found", "Recording not found")
		return
	}

	rec := m.store.Get(id)
	if rec == nil {
		writeError(w, http.StatusNotFound, "not_found", "Recording not found")
		return
	}

	// Parse options
	var req SOAPConvertRequest
	if r.Body != nil && r.ContentLength > 0 {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json", ErrMsgInvalidJSON)
			return
		}
	}

	opts := recording.SOAPConvertOptions{
		Deduplicate:    true,
		IncludeDelay:   req.IncludeDelay,
		PreserveFaults: req.PreserveFaults,
	}

	// Convert single recording
	result := recording.ConvertSOAPRecordings([]*recording.SOAPRecording{rec}, opts)

	writeJSON(w, http.StatusOK, SOAPConvertResponse{
		Config:         result.Config,
		OperationCount: result.OperationCount,
		Total:          result.Total,
		Warnings:       result.Warnings,
	})
}

// handleConvertSOAPRecordings handles POST /soap-recordings/convert.
func (m *SOAPRecordingManager) handleConvertSOAPRecordings(w http.ResponseWriter, r *http.Request) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.store == nil {
		writeError(w, http.StatusBadRequest, "no_store", "No recording store available")
		return
	}

	// Parse request
	var req SOAPConvertRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", ErrMsgInvalidJSON)
		return
	}

	// Get recordings to convert
	var recordings []*recording.SOAPRecording
	switch {
	case len(req.RecordingIDs) > 0:
		for _, id := range req.RecordingIDs {
			if rec := m.store.Get(id); rec != nil {
				recordings = append(recordings, rec)
			}
		}
	case req.Endpoint != "":
		recordings = m.store.ListByEndpoint(req.Endpoint)
	case req.Operation != "":
		recordings = m.store.ListByOperation(req.Operation)
	default:
		recordings, _ = m.store.List(recording.SOAPRecordingFilter{})
	}

	if len(recordings) == 0 {
		writeError(w, http.StatusBadRequest, "no_recordings", "No recordings to convert")
		return
	}

	opts := recording.SOAPConvertOptions{
		Deduplicate:    req.Deduplicate,
		IncludeDelay:   req.IncludeDelay,
		PreserveFaults: req.PreserveFaults,
	}

	result := recording.ConvertSOAPRecordings(recordings, opts)

	writeJSON(w, http.StatusOK, SOAPConvertResponse{
		Config:         result.Config,
		OperationCount: result.OperationCount,
		Total:          result.Total,
		Warnings:       result.Warnings,
	})
}

// handleExportSOAPRecordings handles POST /soap-recordings/export.
func (m *SOAPRecordingManager) handleExportSOAPRecordings(w http.ResponseWriter, r *http.Request) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.store == nil {
		writeError(w, http.StatusBadRequest, "no_store", "No recording store available")
		return
	}

	data, err := m.store.Export()
	if err != nil {
		m.log.Error("failed to export SOAP recordings", "error", err)
		writeError(w, http.StatusInternalServerError, "export_error", ErrMsgInternalError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", "attachment; filename=soap-recordings.json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}
