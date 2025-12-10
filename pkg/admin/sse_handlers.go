package admin

import (
	"encoding/json"
	"net/http"

	"github.com/getmockd/mockd/pkg/sse"
)

// SSEConnectionListResponse represents a list of SSE connections.
type SSEConnectionListResponse struct {
	Connections []sse.SSEStreamInfo  `json:"connections"`
	Stats       sse.ConnectionStats  `json:"stats"`
}

// handleListSSEConnections handles GET /sse/connections.
func (a *AdminAPI) handleListSSEConnections(w http.ResponseWriter, r *http.Request) {
	handler := a.server.Handler()
	if handler == nil || handler.SSEHandler() == nil {
		writeJSON(w, http.StatusOK, SSEConnectionListResponse{
			Connections: []sse.SSEStreamInfo{},
			Stats:       sse.ConnectionStats{ConnectionsByMock: make(map[string]int)},
		})
		return
	}

	manager := handler.SSEHandler().GetManager()
	info := manager.GetConnectionInfo()
	stats := manager.Stats()

	writeJSON(w, http.StatusOK, SSEConnectionListResponse{
		Connections: info,
		Stats:       stats,
	})
}

// handleGetSSEConnection handles GET /sse/connections/{id}.
func (a *AdminAPI) handleGetSSEConnection(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "missing_id", "Connection ID is required")
		return
	}

	handler := a.server.Handler()
	if handler == nil || handler.SSEHandler() == nil {
		writeError(w, http.StatusNotFound, "not_found", "Connection not found")
		return
	}

	manager := handler.SSEHandler().GetManager()
	stream := manager.Get(id)
	if stream == nil {
		writeError(w, http.StatusNotFound, "not_found", "Connection not found")
		return
	}

	info := sse.SSEStreamInfo{
		ID:         stream.ID,
		MockID:     stream.MockID,
		ClientIP:   stream.ClientIP,
		StartTime:  stream.StartTime,
		EventsSent: stream.EventsSent,
		Status:     stream.Status,
	}

	writeJSON(w, http.StatusOK, info)
}

// CloseConnectionRequest represents a request to close an SSE connection.
type CloseConnectionRequest struct {
	Graceful bool `json:"graceful,omitempty"`
}

// handleCloseSSEConnection handles DELETE /sse/connections/{id}.
func (a *AdminAPI) handleCloseSSEConnection(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "missing_id", "Connection ID is required")
		return
	}

	handler := a.server.Handler()
	if handler == nil || handler.SSEHandler() == nil {
		writeError(w, http.StatusNotFound, "not_found", "Connection not found")
		return
	}

	manager := handler.SSEHandler().GetManager()

	// Parse optional request body
	var req CloseConnectionRequest
	_ = json.NewDecoder(r.Body).Decode(&req)

	err := manager.Close(id, req.Graceful, nil)
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", "Connection not found")
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"message":    "Connection closed",
		"connection": id,
		"graceful":   req.Graceful,
	})
}

// handleGetSSEStats handles GET /sse/stats.
func (a *AdminAPI) handleGetSSEStats(w http.ResponseWriter, r *http.Request) {
	handler := a.server.Handler()
	if handler == nil || handler.SSEHandler() == nil {
		writeJSON(w, http.StatusOK, sse.ConnectionStats{
			ConnectionsByMock: make(map[string]int),
		})
		return
	}

	manager := handler.SSEHandler().GetManager()
	stats := manager.Stats()

	writeJSON(w, http.StatusOK, stats)
}

// handleListMockSSEConnections handles GET /mocks/{id}/sse/connections.
func (a *AdminAPI) handleListMockSSEConnections(w http.ResponseWriter, r *http.Request) {
	mockID := r.PathValue("id")
	if mockID == "" {
		writeError(w, http.StatusBadRequest, "missing_id", "Mock ID is required")
		return
	}

	// Verify mock exists
	mock := a.server.GetMock(mockID)
	if mock == nil {
		writeError(w, http.StatusNotFound, "not_found", "Mock not found")
		return
	}

	handler := a.server.Handler()
	if handler == nil || handler.SSEHandler() == nil {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"connections": []sse.SSEStreamInfo{},
			"count":       0,
		})
		return
	}

	manager := handler.SSEHandler().GetManager()
	streams := manager.GetConnectionsByMock(mockID)

	info := make([]sse.SSEStreamInfo, 0, len(streams))
	for _, stream := range streams {
		info = append(info, sse.SSEStreamInfo{
			ID:         stream.ID,
			MockID:     stream.MockID,
			ClientIP:   stream.ClientIP,
			StartTime:  stream.StartTime,
			EventsSent: stream.EventsSent,
			Status:     stream.Status,
		})
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"connections": info,
		"count":       len(info),
		"mockId":      mockID,
	})
}

// handleCloseMockSSEConnections handles DELETE /mocks/{id}/sse/connections.
func (a *AdminAPI) handleCloseMockSSEConnections(w http.ResponseWriter, r *http.Request) {
	mockID := r.PathValue("id")
	if mockID == "" {
		writeError(w, http.StatusBadRequest, "missing_id", "Mock ID is required")
		return
	}

	// Verify mock exists
	mock := a.server.GetMock(mockID)
	if mock == nil {
		writeError(w, http.StatusNotFound, "not_found", "Mock not found")
		return
	}

	handler := a.server.Handler()
	if handler == nil || handler.SSEHandler() == nil {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"message": "No connections to close",
			"closed":  0,
			"mockId":  mockID,
		})
		return
	}

	manager := handler.SSEHandler().GetManager()
	closed := manager.CloseByMock(mockID)

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"message": "Connections closed",
		"closed":  closed,
		"mockId":  mockID,
	})
}

// handleGetMockSSEBuffer handles GET /mocks/{id}/sse/buffer.
func (a *AdminAPI) handleGetMockSSEBuffer(w http.ResponseWriter, r *http.Request) {
	mockID := r.PathValue("id")
	if mockID == "" {
		writeError(w, http.StatusBadRequest, "missing_id", "Mock ID is required")
		return
	}

	// Verify mock exists
	mock := a.server.GetMock(mockID)
	if mock == nil {
		writeError(w, http.StatusNotFound, "not_found", "Mock not found")
		return
	}

	handler := a.server.Handler()
	if handler == nil || handler.SSEHandler() == nil {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"mockId": mockID,
			"size":   0,
			"events": []interface{}{},
		})
		return
	}

	buffer := handler.SSEHandler().GetBuffer(mockID)
	if buffer == nil {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"mockId": mockID,
			"size":   0,
			"events": []interface{}{},
		})
		return
	}

	stats := buffer.Stats()
	events := buffer.GetLatest(100) // Get last 100 events

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"mockId": mockID,
		"size":   stats.Size,
		"stats":  stats,
		"events": events,
	})
}

// handleClearMockSSEBuffer handles DELETE /mocks/{id}/sse/buffer.
func (a *AdminAPI) handleClearMockSSEBuffer(w http.ResponseWriter, r *http.Request) {
	mockID := r.PathValue("id")
	if mockID == "" {
		writeError(w, http.StatusBadRequest, "missing_id", "Mock ID is required")
		return
	}

	// Verify mock exists
	mock := a.server.GetMock(mockID)
	if mock == nil {
		writeError(w, http.StatusNotFound, "not_found", "Mock not found")
		return
	}

	handler := a.server.Handler()
	if handler == nil || handler.SSEHandler() == nil {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"message": "Buffer cleared",
			"mockId":  mockID,
		})
		return
	}

	handler.SSEHandler().ClearBuffer(mockID)

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"message": "Buffer cleared",
		"mockId":  mockID,
	})
}
