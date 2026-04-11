package admin

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/getmockd/mockd/pkg/admin/engineclient"
	"github.com/getmockd/mockd/pkg/store"
)

// SSEConnectionListResponse represents a list of SSE connections.
type SSEConnectionListResponse struct {
	Connections []*engineclient.SSEConnection `json:"connections"`
	Stats       engineclient.SSEStats         `json:"stats"`
}

// handleListSSEConnections handles GET /sse/connections.
//
//nolint:dupl // intentionally parallel structure with other protocol list handlers
func (a *API) handleListSSEConnections(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	engine := a.localEngine.Load()
	if engine == nil {
		writeJSON(w, http.StatusOK, SSEConnectionListResponse{
			Connections: []*engineclient.SSEConnection{},
			Stats:       engineclient.SSEStats{ConnectionsByMock: make(map[string]int)},
		})
		return
	}

	stats, err := engine.GetSSEStats(ctx)
	if err != nil {
		a.logger().Error("failed to get SSE stats", "error", err)
		status, code, msg := mapSSEEngineError(err, a.logger(), "get SSE stats")
		writeError(w, status, code, msg)
		return
	}

	connections, err := engine.ListSSEConnections(ctx)
	if err != nil {
		a.logger().Error("failed to list SSE connections", "error", err)
		status, code, msg := mapSSEEngineError(err, a.logger(), "list SSE connections")
		writeError(w, status, code, msg)
		return
	}

	if connections == nil {
		connections = []*engineclient.SSEConnection{}
	}

	connsByMock := stats.ConnectionsByMock
	if connsByMock == nil {
		connsByMock = make(map[string]int)
	}

	writeJSON(w, http.StatusOK, SSEConnectionListResponse{
		Connections: connections,
		Stats: engineclient.SSEStats{
			TotalConnections:  stats.TotalConnections,
			ActiveConnections: stats.ActiveConnections,
			TotalEventsSent:   stats.TotalEventsSent,
			TotalBytesSent:    stats.TotalBytesSent,
			ConnectionErrors:  stats.ConnectionErrors,
			ConnectionsByMock: connsByMock,
		},
	})
}

// handleGetSSEConnection handles GET /sse/connections/{id}.
func (a *API) handleGetSSEConnection(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "missing_id", "Connection ID is required")
		return
	}

	engine := a.localEngine.Load()
	if engine == nil {
		writeError(w, http.StatusNotFound, "not_found", "Connection not found")
		return
	}

	conn, err := engine.GetSSEConnection(ctx, id)
	if err != nil {
		if errors.Is(err, engineclient.ErrNotFound) {
			writeError(w, http.StatusNotFound, "not_found", "Connection not found")
			return
		}
		a.logger().Error("failed to get SSE connection", "error", err, "connectionID", id)
		status, code, msg := mapSSEEngineError(err, a.logger(), "get SSE connection")
		writeError(w, status, code, msg)
		return
	}

	writeJSON(w, http.StatusOK, conn)
}

// handleCloseSSEConnection handles DELETE /sse/connections/{id}.
func (a *API) handleCloseSSEConnection(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "missing_id", "Connection ID is required")
		return
	}

	engine := a.localEngine.Load()
	if engine == nil {
		writeError(w, http.StatusNotFound, "not_found", "Connection not found")
		return
	}

	// Parse optional request body
	var req struct{}
	if err := decodeOptionalJSONBody(r, &req); err != nil {
		writeJSONDecodeError(w, err, a.logger())
		return
	}

	err := engine.CloseSSEConnection(ctx, id)
	if err != nil {
		if errors.Is(err, engineclient.ErrNotFound) {
			writeError(w, http.StatusNotFound, "not_found", "Connection not found")
			return
		}
		a.logger().Error("failed to close SSE connection", "error", err, "connectionID", id)
		status, code, msg := mapSSEEngineError(err, a.logger(), "close SSE connection")
		writeError(w, status, code, msg)
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"message":    "Connection closed",
		"connection": id,
	})
}

// handleGetSSEStats handles GET /sse/stats.
func (a *API) handleGetSSEStats(w http.ResponseWriter, r *http.Request) {
	engine := a.localEngine.Load()
	if engine == nil {
		writeJSON(w, http.StatusOK, engineclient.SSEStats{ConnectionsByMock: make(map[string]int)})
		return
	}
	a.handleGetStats(w, r, newSSEStatsProvider(engine))
}

// handleListMockSSEConnections handles GET /mocks/{id}/sse/connections.
//
//nolint:dupl // intentionally parallel structure with other protocol mock-scoped handlers
func (a *API) handleListMockSSEConnections(w http.ResponseWriter, r *http.Request, engine *engineclient.Client) {
	ctx := r.Context()
	mockID := r.PathValue("id")
	if mockID == "" {
		writeError(w, http.StatusBadRequest, "missing_id", "Mock ID is required")
		return
	}

	// Verify mock exists in the admin store (single source of truth).
	if mockStore := a.getMockStore(); mockStore != nil {
		if _, err := mockStore.Get(ctx, mockID); err != nil {
			if errors.Is(err, store.ErrNotFound) {
				writeError(w, http.StatusNotFound, "not_found", "Mock not found")
				return
			}
			writeError(w, http.StatusInternalServerError, "store_error", ErrMsgInternalError)
			return
		}
	}

	// Get all SSE connections and filter by mock
	connections, err := engine.ListSSEConnections(ctx)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "engine_error", sanitizeEngineError(err, a.logger(), "list SSE connections for mock"))
		return
	}

	var filtered []*engineclient.SSEConnection
	for _, conn := range connections {
		if conn.MockID == mockID {
			filtered = append(filtered, conn)
		}
	}

	if filtered == nil {
		filtered = []*engineclient.SSEConnection{}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"connections": filtered,
		"count":       len(filtered),
		"mockId":      mockID,
	})
}

// handleCloseMockSSEConnections handles DELETE /mocks/{id}/sse/connections.
//
//nolint:dupl // intentionally parallel structure with other protocol mock-scoped handlers
func (a *API) handleCloseMockSSEConnections(w http.ResponseWriter, r *http.Request, engine *engineclient.Client) {
	ctx := r.Context()
	mockID := r.PathValue("id")
	if mockID == "" {
		writeError(w, http.StatusBadRequest, "missing_id", "Mock ID is required")
		return
	}

	// Verify mock exists in the admin store (single source of truth).
	if mockStore := a.getMockStore(); mockStore != nil {
		if _, err := mockStore.Get(ctx, mockID); err != nil {
			if errors.Is(err, store.ErrNotFound) {
				writeError(w, http.StatusNotFound, "not_found", "Mock not found")
				return
			}
			writeError(w, http.StatusInternalServerError, "store_error", ErrMsgInternalError)
			return
		}
	}

	// Get all SSE connections and close those for this mock
	connections, err := engine.ListSSEConnections(ctx)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "engine_error", sanitizeEngineError(err, a.logger(), "list SSE connections for close"))
		return
	}

	closed := 0
	for _, conn := range connections {
		if conn.MockID == mockID {
			if err := engine.CloseSSEConnection(ctx, conn.ID); err == nil {
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
func (a *API) handleGetMockSSEBuffer(w http.ResponseWriter, r *http.Request) {
	mockID := r.PathValue("id")
	if mockID == "" {
		writeError(w, http.StatusBadRequest, "missing_id", "Mock ID is required")
		return
	}

	writeError(w, http.StatusNotImplemented, "not_implemented", "SSE buffer access requires direct engine access - coming soon")
}

// handleClearMockSSEBuffer handles DELETE /mocks/{id}/sse/buffer.
// Note: Buffer access requires direct engine access - not yet available via HTTP.
func (a *API) handleClearMockSSEBuffer(w http.ResponseWriter, r *http.Request) {
	mockID := r.PathValue("id")
	if mockID == "" {
		writeError(w, http.StatusBadRequest, "missing_id", "Mock ID is required")
		return
	}

	writeError(w, http.StatusNotImplemented, "not_implemented", "SSE buffer access requires direct engine access - coming soon")
}

func mapSSEEngineError(err error, log *slog.Logger, operation string) (int, string, string) {
	return http.StatusServiceUnavailable, "engine_error", sanitizeEngineError(err, log, operation)
}
