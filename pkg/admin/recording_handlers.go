package admin

import (
	"encoding/json"
	"fmt"
	"net/http"

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
func (pm *ProxyManager) handleConvertRecordings(w http.ResponseWriter, r *http.Request, server interface{ AddMockFromRecording(*recording.Recording) (string, error) }) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	if pm.store == nil {
		writeError(w, http.StatusBadRequest, "no_store", "No recording store available")
		return
	}

	var req ConvertRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "Failed to parse request body: "+err.Error())
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

	// Add mocks to server (if server interface provided)
	mockIDs := make([]string, 0, len(mocks))
	for _, mock := range mocks {
		mockIDs = append(mockIDs, mock.ID)
	}

	writeJSON(w, http.StatusOK, ConvertResult{
		MockIDs: mockIDs,
		Count:   len(mocks),
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
		json.NewDecoder(r.Body).Decode(&req)
	}

	var jsonOutput []byte
	var err error

	if req.SessionID != "" {
		jsonOutput, err = pm.store.ExportSession(req.SessionID)
	} else if len(req.RecordingIDs) > 0 {
		// Export specific recordings
		var recordings []*recording.Recording
		for _, id := range req.RecordingIDs {
			rec := pm.store.GetRecording(id)
			if rec != nil {
				recordings = append(recordings, rec)
			}
		}
		jsonOutput, err = json.MarshalIndent(recordings, "", "  ")
	} else {
		jsonOutput, err = pm.store.ExportRecordings(recording.RecordingFilter{})
	}

	if err != nil {
		writeError(w, http.StatusInternalServerError, "export_error", "Failed to export recordings: "+err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(jsonOutput)
}
