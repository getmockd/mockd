package admin

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/getmockd/mockd/pkg/admin/engineclient"
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
	provider := newWSStatsProvider(engine)
	a.handleGetStats(w, r, provider)
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
		writeError(w, http.StatusBadRequest, "invalid_body", "Invalid JSON in request body")
		return
	}
	if req.Type == "" {
		req.Type = "text"
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
