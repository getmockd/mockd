package admin

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/getmockd/mockd/pkg/admin/engineclient"
	"github.com/getmockd/mockd/pkg/store"
)

// WebSocketConnectionListResponse represents a list of WebSocket connections with stats.
type WebSocketConnectionListResponse struct {
	Connections []*engineclient.WebSocketConnection `json:"connections"`
	Stats       engineclient.WebSocketStats         `json:"stats"`
}

// handleListWebSocketConnections handles GET /websocket/connections.
func (a *API) handleListWebSocketConnections(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	engine := a.localEngine.Load()
	if engine == nil {
		writeJSON(w, http.StatusOK, WebSocketConnectionListResponse{
			Connections: []*engineclient.WebSocketConnection{},
			Stats:       engineclient.WebSocketStats{ConnectionsByMock: make(map[string]int)},
		})
		return
	}

	stats, err := engine.GetWebSocketStats(ctx)
	if err != nil {
		a.logger().Error("failed to get WebSocket stats", "error", err)
		status, code, msg := mapWebSocketEngineError(err, a.logger(), "get WebSocket stats")
		writeError(w, status, code, msg)
		return
	}

	// Stats and connections are fetched in two separate calls; they are not
	// atomically consistent. Connections may change between the two requests,
	// so the counts in stats may not exactly match the length of the returned
	// connection list. This is intentional — correctness is not required here.
	connections, err := engine.ListWebSocketConnections(ctx)
	if err != nil {
		a.logger().Error("failed to list WebSocket connections", "error", err)
		status, code, msg := mapWebSocketEngineError(err, a.logger(), "list WebSocket connections")
		writeError(w, status, code, msg)
		return
	}

	if connections == nil {
		connections = []*engineclient.WebSocketConnection{}
	}

	connsByMock := stats.ConnectionsByMock
	if connsByMock == nil {
		connsByMock = make(map[string]int)
	}

	writeJSON(w, http.StatusOK, WebSocketConnectionListResponse{
		Connections: connections,
		Stats: engineclient.WebSocketStats{
			TotalConnections:  stats.TotalConnections,
			ActiveConnections: stats.ActiveConnections,
			TotalMessagesSent: stats.TotalMessagesSent,
			TotalMessagesRecv: stats.TotalMessagesRecv,
			ConnectionsByMock: connsByMock,
		},
	})
}

// handleGetWebSocketConnection handles GET /websocket/connections/{id}.
func (a *API) handleGetWebSocketConnection(w http.ResponseWriter, r *http.Request) {
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

	conn, err := engine.GetWebSocketConnection(ctx, id)
	if err != nil {
		if errors.Is(err, engineclient.ErrNotFound) {
			writeError(w, http.StatusNotFound, "not_found", "Connection not found")
			return
		}
		a.logger().Error("failed to get WebSocket connection", "error", err, "connectionID", id)
		status, code, msg := mapWebSocketEngineError(err, a.logger(), "get WebSocket connection")
		writeError(w, status, code, msg)
		return
	}

	writeJSON(w, http.StatusOK, conn)
}

// handleCloseWebSocketConnection handles DELETE /websocket/connections/{id}.
//
//nolint:dupl // intentionally parallel structure with other protocol close handlers
func (a *API) handleCloseWebSocketConnection(w http.ResponseWriter, r *http.Request) {
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

	err := engine.CloseWebSocketConnection(ctx, id)
	if err != nil {
		if errors.Is(err, engineclient.ErrNotFound) {
			writeError(w, http.StatusNotFound, "not_found", "Connection not found")
			return
		}
		a.logger().Error("failed to close WebSocket connection", "error", err, "connectionID", id)
		status, code, msg := mapWebSocketEngineError(err, a.logger(), "close WebSocket connection")
		writeError(w, status, code, msg)
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"message":    "Connection closed",
		"connection": id,
	})
}

// handleGetWebSocketStats handles GET /websocket/stats.
func (a *API) handleGetWebSocketStats(w http.ResponseWriter, r *http.Request) {
	engine := a.localEngine.Load()
	if engine == nil {
		writeJSON(w, http.StatusOK, engineclient.WebSocketStats{ConnectionsByMock: make(map[string]int)})
		return
	}
	a.handleGetStats(w, r, newWSStatsProvider(engine))
}

func mapWebSocketEngineError(err error, log *slog.Logger, operation string) (int, string, string) {
	return http.StatusServiceUnavailable, "engine_error", sanitizeEngineError(err, log, operation)
}

// handleSendToWebSocketConnection handles POST /websocket/connections/{id}/send.
// It forwards a text or binary message to a specific active connection through the engine.
func (a *API) handleSendToWebSocketConnection(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "missing_id", "Connection ID is required")
		return
	}

	var req WebSocketSendRequest
	if err := decodeOptionalJSONBody(r, &req); err != nil {
		writeJSONDecodeError(w, err, a.logger())
		return
	}
	if req.Type == "" {
		req.Type = "text"
	}
	if req.Type != "text" && req.Type != "binary" {
		writeError(w, http.StatusBadRequest, "invalid_type", `Type must be "text" or "binary"`)
		return
	}

	engine := a.localEngine.Load()
	if engine == nil {
		writeError(w, http.StatusNotFound, "not_found", "Connection not found")
		return
	}

	err := engine.SendToWebSocketConnection(ctx, id, req.Type, req.Data)
	if err != nil {
		if errors.Is(err, engineclient.ErrNotFound) {
			writeError(w, http.StatusNotFound, "not_found", "Connection not found")
			return
		}
		a.logger().Error("failed to send to WebSocket connection", "error", err, "connectionID", id)
		status, code, msg := mapWebSocketEngineError(err, a.logger(), "send to WebSocket connection")
		writeError(w, status, code, msg)
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"message":    "Message sent",
		"connection": id,
		"type":       req.Type,
	})
}

// handleListMockWebSocketConnections handles GET /mocks/{id}/websocket/connections.
//
//nolint:dupl // intentionally parallel structure with other protocol mock-scoped handlers
func (a *API) handleListMockWebSocketConnections(w http.ResponseWriter, r *http.Request, engine *engineclient.Client) {
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

	// Get all WebSocket connections and filter by mock
	connections, err := engine.ListWebSocketConnections(ctx)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "engine_error", sanitizeEngineError(err, a.logger(), "list WebSocket connections for mock"))
		return
	}

	var filtered []*engineclient.WebSocketConnection
	for _, conn := range connections {
		if conn.MockID == mockID {
			filtered = append(filtered, conn)
		}
	}
	if filtered == nil {
		filtered = []*engineclient.WebSocketConnection{}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"connections": filtered,
		"count":       len(filtered),
		"mockId":      mockID,
	})
}

// handleCloseMockWebSocketConnections handles DELETE /mocks/{id}/websocket/connections.
//
//nolint:dupl // intentionally parallel structure with other protocol mock-scoped handlers
func (a *API) handleCloseMockWebSocketConnections(w http.ResponseWriter, r *http.Request, engine *engineclient.Client) {
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

	// Get all WebSocket connections and close those for this mock
	connections, err := engine.ListWebSocketConnections(ctx)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "engine_error", sanitizeEngineError(err, a.logger(), "list WebSocket connections for close"))
		return
	}

	closed := 0
	for _, conn := range connections {
		if conn.MockID == mockID {
			if err := engine.CloseWebSocketConnection(ctx, conn.ID); err == nil {
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
