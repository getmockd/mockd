package admin

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/getmockd/mockd/pkg/admin/engineclient"
	"github.com/getmockd/mockd/pkg/config"
	"github.com/getmockd/mockd/pkg/recording"
)

// RecordingListResponse represents a list of recordings.
type RecordingListResponse struct {
	Recordings []*recording.Recording `json:"recordings"`
	Total      int                    `json:"total"`
	Limit      int                    `json:"limit"`
	Offset     int                    `json:"offset"`
}

// ConvertRequest represents a request to convert recordings to mocks.
type ConvertRequest struct {
	RecordingIDs   []string `json:"recordingIds,omitempty"`
	SessionID      string   `json:"sessionId,omitempty"`
	Deduplicate    bool     `json:"deduplicate"`
	IncludeHeaders bool     `json:"includeHeaders"`
}

// ConvertResult represents the result of converting recordings.
type ConvertResult struct {
	MockIDs []string `json:"mockIds"`
	Count   int      `json:"count"`
}

// ExportRequest represents a request to export recordings.
type ExportRequest struct {
	SessionID    string   `json:"sessionId,omitempty"`
	RecordingIDs []string `json:"recordingIds,omitempty"`
	Format       string   `json:"format,omitempty"` // "json" (default) or "yaml"
}

// handleListRecordings handles GET /recordings.
func (pm *ProxyManager) handleListRecordings(w http.ResponseWriter, r *http.Request) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	if pm.store == nil {
		writeJSON(w, http.StatusOK, RecordingListResponse{
			Recordings: []*recording.Recording{},
			Total:      0,
		})
		return
	}

	filter := recording.RecordingFilter{}

	// Parse query parameters
	if session := r.URL.Query().Get("session"); session != "" {
		filter.SessionID = session
	}
	if method := r.URL.Query().Get("method"); method != "" {
		filter.Method = method
	}
	if path := r.URL.Query().Get("path"); path != "" {
		filter.Path = path
	}
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		var limit int
		if _, err := fmt.Sscanf(limitStr, "%d", &limit); err == nil {
			filter.Limit = limit
		}
	}

	recordings, total := pm.store.ListRecordings(filter)

	writeJSON(w, http.StatusOK, RecordingListResponse{
		Recordings: recordings,
		Total:      total,
		Limit:      filter.Limit,
	})
}

// handleClearRecordings handles DELETE /recordings.
func (pm *ProxyManager) handleClearRecordings(w http.ResponseWriter, r *http.Request) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	if pm.store == nil {
		writeJSON(w, http.StatusOK, map[string]int{"deleted": 0})
		return
	}

	count := pm.store.Clear()
	writeJSON(w, http.StatusOK, map[string]int{"deleted": count})
}

// handleGetRecording handles GET /recordings/{id}.
func (pm *ProxyManager) handleGetRecording(w http.ResponseWriter, r *http.Request) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "missing_id", "Recording ID is required")
		return
	}

	if pm.store == nil {
		writeError(w, http.StatusNotFound, "not_found", "Recording not found")
		return
	}

	rec := pm.store.GetRecording(id)
	if rec == nil {
		writeError(w, http.StatusNotFound, "not_found", "Recording not found")
		return
	}

	writeJSON(w, http.StatusOK, rec)
}

// handleDeleteRecording handles DELETE /recordings/{id}.
func (pm *ProxyManager) handleDeleteRecording(w http.ResponseWriter, r *http.Request) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "missing_id", "Recording ID is required")
		return
	}

	if pm.store == nil {
		writeError(w, http.StatusNotFound, "not_found", "Recording not found")
		return
	}

	if err := pm.store.DeleteRecording(id); err != nil {
		writeError(w, http.StatusNotFound, "not_found", "Recording not found")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// handleConvertRecordings handles POST /recordings/convert.
func (pm *ProxyManager) handleConvertRecordings(w http.ResponseWriter, r *http.Request, client *engineclient.Client) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	if pm.store == nil {
		writeError(w, http.StatusBadRequest, "no_store", "No recording store available")
		return
	}

	var req ConvertRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", ErrMsgInvalidJSON)
		return
	}

	// Get recordings to convert
	var recordings []*recording.Recording
	if len(req.RecordingIDs) > 0 {
		// Convert specific recordings
		for _, id := range req.RecordingIDs {
			rec := pm.store.GetRecording(id)
			if rec != nil {
				recordings = append(recordings, rec)
			}
		}
	} else if req.SessionID != "" {
		// Convert all recordings from session
		filter := recording.RecordingFilter{SessionID: req.SessionID}
		recordings, _ = pm.store.ListRecordings(filter)
	} else {
		// Convert all recordings
		recordings, _ = pm.store.ListRecordings(recording.RecordingFilter{})
	}

	if len(recordings) == 0 {
		writeError(w, http.StatusBadRequest, "no_recordings", "No recordings to convert")
		return
	}

	// Convert to mocks
	opts := recording.ConvertOptions{
		Deduplicate:    req.Deduplicate,
		IncludeHeaders: req.IncludeHeaders,
	}
	mocks := recording.ToMocks(recordings, opts)

	// Add mocks to engine via HTTP client
	mockIDs := make([]string, 0, len(mocks))
	addedCount := 0
	ctx := r.Context()
	for _, mock := range mocks {
		if client != nil {
			if _, err := client.CreateMock(ctx, mock); err == nil {
				addedCount++
			}
		}
		mockIDs = append(mockIDs, mock.ID)
	}

	writeJSON(w, http.StatusOK, ConvertResult{
		MockIDs: mockIDs,
		Count:   addedCount,
	})
}

// handleExportRecordings handles POST /recordings/export.
func (pm *ProxyManager) handleExportRecordings(w http.ResponseWriter, r *http.Request) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	if pm.store == nil {
		writeError(w, http.StatusBadRequest, "no_store", "No recording store available")
		return
	}

	var req ExportRequest
	if r.Body != nil {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json", ErrMsgInvalidJSON)
			return
		}
	}

	// Gather recordings to export
	var recordings []*recording.Recording

	if req.SessionID != "" {
		recordings, _ = pm.store.ListRecordings(recording.RecordingFilter{SessionID: req.SessionID})
	} else if len(req.RecordingIDs) > 0 {
		for _, id := range req.RecordingIDs {
			rec := pm.store.GetRecording(id)
			if rec != nil {
				recordings = append(recordings, rec)
			}
		}
	} else {
		recordings, _ = pm.store.ListRecordings(recording.RecordingFilter{})
	}

	// Marshal to requested format
	var output []byte
	var err error
	contentType := "application/json"

	if strings.EqualFold(req.Format, "yaml") {
		output, err = yaml.Marshal(recordings)
		contentType = "application/x-yaml"
	} else {
		output, err = json.MarshalIndent(recordings, "", "  ")
	}

	if err != nil {
		log.Printf("Failed to export recordings: %v\n", err)
		writeError(w, http.StatusInternalServerError, "export_error", "Failed to export recordings")
		return
	}

	w.Header().Set("Content-Type", contentType)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(output)
}

// SingleConvertRequest represents a request to convert a single recording.
type SingleConvertRequest struct {
	IncludeHeaders bool `json:"includeHeaders"`
	SmartMatch     bool `json:"smartMatch"`
	AddToServer    bool `json:"addToServer"`
}

// SingleConvertResponse represents the result of converting a single recording.
type SingleConvertResponse struct {
	Mock     *config.MockConfiguration        `json:"mock"`
	Warnings []recording.SensitiveDataWarning `json:"warnings,omitempty"`
}

// handleConvertSingleRecording handles POST /recordings/{id}/to-mock.
func (pm *ProxyManager) handleConvertSingleRecording(w http.ResponseWriter, r *http.Request, client *engineclient.Client) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "missing_id", "Recording ID is required")
		return
	}

	if pm.store == nil {
		writeError(w, http.StatusBadRequest, "no_store", "No recording store available")
		return
	}

	rec := pm.store.GetRecording(id)
	if rec == nil {
		writeError(w, http.StatusNotFound, "not_found", "Recording not found")
		return
	}

	// Parse request body for options
	var req SingleConvertRequest
	if r.Body != nil && r.ContentLength > 0 {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json", ErrMsgInvalidJSON)
			return
		}
	}

	opts := recording.ConvertOptions{
		IncludeHeaders: req.IncludeHeaders,
		SmartMatch:     req.SmartMatch,
	}

	// Check for sensitive data warnings
	warnings := recording.CheckSensitiveData(rec)

	// Convert to mock
	mock := recording.ToMock(rec, opts)

	// Apply smart matching if enabled
	if req.SmartMatch && mock.HTTP != nil && mock.HTTP.Matcher != nil {
		mock.HTTP.Matcher.Path = recording.SmartPathMatcher(mock.HTTP.Matcher.Path)
	}

	// Check if we should add to engine (from JSON body or query param)
	addToServer := req.AddToServer || r.URL.Query().Get("add") == "true"
	if addToServer && client != nil {
		if _, err := client.CreateMock(r.Context(), mock); err != nil {
			log.Printf("Failed to add mock to engine: %v\n", err)
			writeError(w, http.StatusInternalServerError, "add_error", ErrMsgInternalError)
			return
		}
	}

	writeJSON(w, http.StatusOK, SingleConvertResponse{
		Mock:     mock,
		Warnings: warnings,
	})
}

// SessionConvertRequest represents a request to convert session recordings.
type SessionConvertRequest struct {
	PathFilter   string `json:"pathFilter,omitempty"`   // Glob pattern like /api/*
	MethodFilter string `json:"methodFilter,omitempty"` // Comma-separated: GET,POST
	StatusFilter string `json:"statusFilter,omitempty"` // 2xx, 4xx, or specific codes
	Duplicates   string `json:"duplicates,omitempty"`   // "first", "last", "all"
	AddToServer  bool   `json:"addToServer,omitempty"`  // Add mocks directly
	SmartMatch   bool   `json:"smartMatch,omitempty"`   // Convert /users/123 to /users/{id}
}

// SessionConvertResponse represents the result of converting session recordings.
type SessionConvertResponse struct {
	Mocks    []*config.MockConfiguration      `json:"mocks"`
	MockIDs  []string                         `json:"mockIds"`
	Warnings []recording.SensitiveDataWarning `json:"warnings,omitempty"`
	Filtered int                              `json:"filtered"`
	Total    int                              `json:"total"`
	Added    int                              `json:"added"`
}

// handleConvertSession handles POST /recordings/sessions/{id}/to-mocks.
func (pm *ProxyManager) handleConvertSession(w http.ResponseWriter, r *http.Request, client *engineclient.Client) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	sessionID := r.PathValue("id")
	if sessionID == "" {
		writeError(w, http.StatusBadRequest, "missing_id", "Session ID is required")
		return
	}

	if pm.store == nil {
		writeError(w, http.StatusBadRequest, "no_store", "No recording store available")
		return
	}

	// Handle "latest" as a special session ID
	var session *recording.Session
	if sessionID == "latest" {
		session = pm.store.ActiveSession()
		if session == nil {
			sessions := pm.store.ListSessions()
			if len(sessions) > 0 {
				session = sessions[len(sessions)-1]
			}
		}
	} else {
		session = pm.store.GetSession(sessionID)
	}

	if session == nil {
		writeError(w, http.StatusNotFound, "not_found", "Session not found")
		return
	}

	// Parse request body for options
	var req SessionConvertRequest
	if r.Body != nil && r.ContentLength > 0 {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json", ErrMsgInvalidJSON)
			return
		}
	}

	// Build filter options
	statusCodes, statusRange := recording.ParseStatusFilter(req.StatusFilter)

	opts := recording.SessionConvertOptions{
		ConvertOptions: recording.ConvertOptions{
			Deduplicate: req.Duplicates != "all",
			SmartMatch:  req.SmartMatch,
		},
		Filter: recording.FilterOptions{
			PathPattern: req.PathFilter,
			Methods:     recording.ParseMethodFilter(req.MethodFilter),
			StatusCodes: statusCodes,
			StatusRange: statusRange,
		},
		Duplicates:  req.Duplicates,
		AddToServer: req.AddToServer,
	}

	if opts.Duplicates == "" {
		opts.Duplicates = "first"
	}

	// Convert with options
	result := recording.ConvertSessionWithOptions(session, opts)

	// Add to server if requested
	addedCount := 0
	mockIDs := make([]string, 0, len(result.Mocks))

	ctx := r.Context()
	for _, mock := range result.Mocks {
		mockIDs = append(mockIDs, mock.ID)
		if req.AddToServer && client != nil {
			if _, err := client.CreateMock(ctx, mock); err == nil {
				addedCount++
			}
		}
	}

	writeJSON(w, http.StatusOK, SessionConvertResponse{
		Mocks:    result.Mocks,
		MockIDs:  mockIDs,
		Warnings: result.Warnings,
		Filtered: result.Filtered,
		Total:    result.Total,
		Added:    addedCount,
	})
}

// handleCheckSensitiveData handles GET /recordings/{id}/check-sensitive.
func (pm *ProxyManager) handleCheckSensitiveData(w http.ResponseWriter, r *http.Request) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "missing_id", "Recording ID is required")
		return
	}

	if pm.store == nil {
		writeError(w, http.StatusBadRequest, "no_store", "No recording store available")
		return
	}

	rec := pm.store.GetRecording(id)
	if rec == nil {
		writeError(w, http.StatusNotFound, "not_found", "Recording not found")
		return
	}

	warnings := recording.CheckSensitiveData(rec)

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"recordingId": id,
		"warnings":    warnings,
		"count":       len(warnings),
	})
}

// handlePreviewSmartMatch handles POST /recordings/{id}/preview-smart-match.
func (pm *ProxyManager) handlePreviewSmartMatch(w http.ResponseWriter, r *http.Request) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "missing_id", "Recording ID is required")
		return
	}

	if pm.store == nil {
		writeError(w, http.StatusBadRequest, "no_store", "No recording store available")
		return
	}

	rec := pm.store.GetRecording(id)
	if rec == nil {
		writeError(w, http.StatusNotFound, "not_found", "Recording not found")
		return
	}

	originalPath := rec.Request.Path
	smartPath := recording.SmartPathMatcher(originalPath)

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"recordingId":  id,
		"originalPath": originalPath,
		"smartPath":    smartPath,
		"changed":      originalPath != smartPath,
	})
}
