package admin

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/getmockd/mockd/pkg/admin/engineclient"
	"github.com/getmockd/mockd/pkg/sse"
)

// SSEConnectionListResponse represents a list of SSE connections.
type SSEConnectionListResponse struct {
	Connections []sse.SSEStreamInfo `json:"connections"`
	Stats       sse.ConnectionStats `json:"stats"`
}

// handleListSSEConnections handles GET /sse/connections.
func (a *AdminAPI) handleListSSEConnections(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if a.localEngine == nil {
		writeJSON(w, http.StatusOK, SSEConnectionListResponse{
			Connections: []sse.SSEStreamInfo{},
			Stats:       sse.ConnectionStats{ConnectionsByMock: make(map[string]int)},
		})
		return
	}

	stats, err := a.localEngine.GetSSEStats(ctx)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "engine_error", err.Error())
		return
	}

	connections, err := a.localEngine.ListSSEConnections(ctx)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "engine_error", err.Error())
		return
	}

	// Convert engine client connections to SSEStreamInfo
	info := make([]sse.SSEStreamInfo, 0, len(connections))
	for _, conn := range connections {
		info = append(info, sse.SSEStreamInfo{
			ID:       conn.ID,
			MockID:   conn.MockID,
			ClientIP: conn.ClientIP,
		})
	}

	connsByMock := stats.ConnectionsByMock
	if connsByMock == nil {
		connsByMock = make(map[string]int)
	}

	writeJSON(w, http.StatusOK, SSEConnectionListResponse{
		Connections: info,
		Stats: sse.ConnectionStats{
			ActiveConnections: stats.ActiveConnections,
			TotalConnections:  stats.TotalConnections,
			TotalEventsSent:   stats.TotalEventsSent,
			TotalBytesSent:    stats.TotalBytesSent,
			ConnectionsByMock: connsByMock,
		},
	})
}

// handleGetSSEConnection handles GET /sse/connections/{id}.
func (a *AdminAPI) handleGetSSEConnection(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "missing_id", "Connection ID is required")
		return
	}

	if a.localEngine == nil {
		writeError(w, http.StatusNotFound, "not_found", "Connection not found")
		return
	}

	conn, err := a.localEngine.GetSSEConnection(ctx, id)
	if err != nil {
		if errors.Is(err, engineclient.ErrNotFound) {
			writeError(w, http.StatusNotFound, "not_found", "Connection not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "engine_error", err.Error())
		return
	}

	info := sse.SSEStreamInfo{
		ID:     conn.ID,
		MockID: conn.MockID,
	}

	writeJSON(w, http.StatusOK, info)
}

// CloseConnectionRequest represents a request to close an SSE connection.
type CloseConnectionRequest struct {
	Graceful bool `json:"graceful,omitempty"`
}

// handleCloseSSEConnection handles DELETE /sse/connections/{id}.
func (a *AdminAPI) handleCloseSSEConnection(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "missing_id", "Connection ID is required")
		return
	}

	if a.localEngine == nil {
		writeError(w, http.StatusNotFound, "not_found", "Connection not found")
		return
	}

	// Parse optional request body
	var req CloseConnectionRequest
	_ = json.NewDecoder(r.Body).Decode(&req)

	err := a.localEngine.CloseSSEConnection(ctx, id)
	if err != nil {
		if errors.Is(err, engineclient.ErrNotFound) {
			writeError(w, http.StatusNotFound, "not_found", "Connection not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "engine_error", err.Error())
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
	ctx := r.Context()

	if a.localEngine == nil {
		writeJSON(w, http.StatusOK, sse.ConnectionStats{
			ConnectionsByMock: make(map[string]int),
		})
		return
	}

	stats, err := a.localEngine.GetSSEStats(ctx)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "engine_error", err.Error())
		return
	}

	connsByMock := stats.ConnectionsByMock
	if connsByMock == nil {
		connsByMock = make(map[string]int)
	}
	writeJSON(w, http.StatusOK, sse.ConnectionStats{
		ActiveConnections: stats.ActiveConnections,
		TotalConnections:  stats.TotalConnections,
		TotalEventsSent:   stats.TotalEventsSent,
		TotalBytesSent:    stats.TotalBytesSent,
		ConnectionsByMock: connsByMock,
	})
}

// handleListMockSSEConnections handles GET /mocks/{id}/sse/connections.
func (a *AdminAPI) handleListMockSSEConnections(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	mockID := r.PathValue("id")
	if mockID == "" {
		writeError(w, http.StatusBadRequest, "missing_id", "Mock ID is required")
		return
	}

	if a.localEngine == nil {
		writeError(w, http.StatusServiceUnavailable, "no_engine", "No engine connected")
		return
	}

	// Verify mock exists
	_, err := a.localEngine.GetMock(ctx, mockID)
	if err != nil {
		if errors.Is(err, engineclient.ErrNotFound) {
			writeError(w, http.StatusNotFound, "not_found", "Mock not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "engine_error", err.Error())
		return
	}

	// Get all SSE connections and filter by mock
	connections, err := a.localEngine.ListSSEConnections(ctx)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "engine_error", err.Error())
		return
	}

	info := make([]sse.SSEStreamInfo, 0)
	for _, conn := range connections {
		if conn.MockID == mockID {
			info = append(info, sse.SSEStreamInfo{
				ID:     conn.ID,
				MockID: conn.MockID,
			})
		}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"connections": info,
		"count":       len(info),
		"mockId":      mockID,
	})
}

// handleCloseMockSSEConnections handles DELETE /mocks/{id}/sse/connections.
func (a *AdminAPI) handleCloseMockSSEConnections(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	mockID := r.PathValue("id")
	if mockID == "" {
		writeError(w, http.StatusBadRequest, "missing_id", "Mock ID is required")
		return
	}

	if a.localEngine == nil {
		writeError(w, http.StatusServiceUnavailable, "no_engine", "No engine connected")
		return
	}

	// Verify mock exists
	_, err := a.localEngine.GetMock(ctx, mockID)
	if err != nil {
		if errors.Is(err, engineclient.ErrNotFound) {
			writeError(w, http.StatusNotFound, "not_found", "Mock not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "engine_error", err.Error())
		return
	}

	// Get all SSE connections and close those for this mock
	connections, err := a.localEngine.ListSSEConnections(ctx)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "engine_error", err.Error())
		return
	}

	closed := 0
	for _, conn := range connections {
		if conn.MockID == mockID {
			if err := a.localEngine.CloseSSEConnection(ctx, conn.ID); err == nil {
				closed++
			}
		}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"message": "Connections closed",
		"closed":  closed,
		"mockId":  mockID,
	})
}

// handleGetMockSSEBuffer handles GET /mocks/{id}/sse/buffer.
// Note: Buffer access requires direct engine access - not yet available via HTTP.
func (a *AdminAPI) handleGetMockSSEBuffer(w http.ResponseWriter, r *http.Request) {
	mockID := r.PathValue("id")
	if mockID == "" {
		writeError(w, http.StatusBadRequest, "missing_id", "Mock ID is required")
		return
	}

	writeError(w, http.StatusNotImplemented, "not_implemented", "SSE buffer access requires direct engine access - coming soon")
}

// handleClearMockSSEBuffer handles DELETE /mocks/{id}/sse/buffer.
// Note: Buffer access requires direct engine access - not yet available via HTTP.
func (a *AdminAPI) handleClearMockSSEBuffer(w http.ResponseWriter, r *http.Request) {
	mockID := r.PathValue("id")
	if mockID == "" {
		writeError(w, http.StatusBadRequest, "missing_id", "Mock ID is required")
		return
	}

	writeError(w, http.StatusNotImplemented, "not_implemented", "SSE buffer access requires direct engine access - coming soon")
}
