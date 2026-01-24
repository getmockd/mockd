package admin

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/getmockd/mockd/pkg/recording"
)

// SessionSummary represents a session summary.
type SessionSummary struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	StartTime      string `json:"startTime"`
	EndTime        string `json:"endTime,omitempty"`
	RecordingCount int    `json:"recordingCount"`
}

// SessionCreateRequest represents a request to create a session.
type SessionCreateRequest struct {
	Name    string              `json:"name"`
	Filters *FilterConfigUpdate `json:"filters,omitempty"`
}

// SessionResponse represents a full session response.
type SessionResponse struct {
	ID         string                 `json:"id"`
	Name       string                 `json:"name"`
	StartTime  string                 `json:"startTime"`
	EndTime    string                 `json:"endTime,omitempty"`
	Filters    *FilterConfigUpdate    `json:"filters,omitempty"`
	Recordings []*recording.Recording `json:"recordings"`
}

// handleListSessions handles GET /sessions.
func (pm *ProxyManager) handleListSessions(w http.ResponseWriter, r *http.Request) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	if pm.store == nil {
		writeJSON(w, http.StatusOK, []SessionSummary{})
		return
	}

	sessions := pm.store.ListSessions()
	summaries := make([]SessionSummary, 0, len(sessions))

	for _, s := range sessions {
		summary := SessionSummary{
			ID:             s.ID,
			Name:           s.Name,
			StartTime:      s.StartTime.Format(time.RFC3339),
			RecordingCount: len(s.Recordings()),
		}
		if s.EndTime != nil && !s.EndTime.IsZero() {
			summary.EndTime = s.EndTime.Format(time.RFC3339)
		}
		summaries = append(summaries, summary)
	}

	writeJSON(w, http.StatusOK, summaries)
}

// handleCreateSession handles POST /sessions.
func (pm *ProxyManager) handleCreateSession(w http.ResponseWriter, r *http.Request) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	if pm.store == nil {
		writeError(w, http.StatusBadRequest, "no_store", "Proxy not running - start proxy first")
		return
	}

	var req SessionCreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "Failed to parse request body: "+err.Error())
		return
	}

	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "missing_name", "Session name is required")
		return
	}

	// Convert filters if provided
	var metadata map[string]interface{}
	if req.Filters != nil {
		metadata = map[string]interface{}{
			"includePaths": req.Filters.IncludePaths,
			"excludePaths": req.Filters.ExcludePaths,
			"includeHosts": req.Filters.IncludeHosts,
			"excludeHosts": req.Filters.ExcludeHosts,
		}
	}

	session := pm.store.CreateSession(req.Name, metadata)

	summary := SessionSummary{
		ID:             session.ID,
		Name:           session.Name,
		StartTime:      session.StartTime.Format(time.RFC3339),
		RecordingCount: 0,
	}

	writeJSON(w, http.StatusCreated, summary)
}

// handleDeleteSessions handles DELETE /sessions.
func (pm *ProxyManager) handleDeleteSessions(w http.ResponseWriter, r *http.Request) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	if pm.store == nil {
		writeJSON(w, http.StatusOK, map[string]int{"deleted": 0})
		return
	}

	sessions := pm.store.ListSessions()
	count := len(sessions)

	for _, s := range sessions {
		_ = pm.store.DeleteSession(s.ID)
	}

	writeJSON(w, http.StatusOK, map[string]int{"deleted": count})
}

// handleGetSession handles GET /sessions/{id}.
func (pm *ProxyManager) handleGetSession(w http.ResponseWriter, r *http.Request) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "missing_id", "Session ID is required")
		return
	}

	if pm.store == nil {
		writeError(w, http.StatusNotFound, "not_found", "Session not found")
		return
	}

	session := pm.store.GetSession(id)
	if session == nil {
		writeError(w, http.StatusNotFound, "not_found", "Session not found")
		return
	}

	resp := SessionResponse{
		ID:         session.ID,
		Name:       session.Name,
		StartTime:  session.StartTime.Format(time.RFC3339),
		Recordings: session.Recordings(),
	}
	if session.EndTime != nil && !session.EndTime.IsZero() {
		resp.EndTime = session.EndTime.Format(time.RFC3339)
	}

	writeJSON(w, http.StatusOK, resp)
}

// handleDeleteSession handles DELETE /sessions/{id}.
func (pm *ProxyManager) handleDeleteSession(w http.ResponseWriter, r *http.Request) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "missing_id", "Session ID is required")
		return
	}

	if pm.store == nil {
		writeError(w, http.StatusNotFound, "not_found", "Session not found")
		return
	}

	if err := pm.store.DeleteSession(id); err != nil {
		writeError(w, http.StatusNotFound, "not_found", "Session not found")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
