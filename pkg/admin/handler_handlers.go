package admin

import (
	"net/http"

	"github.com/getmockd/mockd/pkg/admin/engineclient"
)

// HandlersListResponse represents the response for listing handlers.
type HandlersListResponse struct {
	Handlers []*engineclient.ProtocolHandler `json:"handlers"`
	Total    int                             `json:"total"`
}

// BroadcastRequest represents a request to broadcast a message.
type BroadcastRequest struct {
	Type    string            `json:"type,omitempty"`
	Data    string            `json:"data"`
	Headers map[string]string `json:"headers,omitempty"`
	Group   string            `json:"group,omitempty"`
	ConnIDs []string          `json:"connectionIds,omitempty"`
}

// GET /handlers
func (a *AdminAPI) handleListHandlers(w http.ResponseWriter, r *http.Request) {
	if a.localEngine == nil {
		writeJSON(w, http.StatusOK, HandlersListResponse{Handlers: []*engineclient.ProtocolHandler{}, Total: 0})
		return
	}

	handlers, err := a.localEngine.ListHandlers(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "engine_error", "Failed to list handlers: "+err.Error())
		return
	}

	if handlers == nil {
		handlers = []*engineclient.ProtocolHandler{}
	}

	writeJSON(w, http.StatusOK, HandlersListResponse{Handlers: handlers, Total: len(handlers)})
}

// GET /handlers/{id}
func (a *AdminAPI) handleGetHandler(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "missing_id", "handler id is required")
		return
	}

	if a.localEngine == nil {
		writeError(w, http.StatusServiceUnavailable, "no_engine", "No engine configured")
		return
	}

	handler, err := a.localEngine.GetHandler(r.Context(), id)
	if err != nil {
		if err == engineclient.ErrNotFound {
			writeError(w, http.StatusNotFound, "not_found", "handler not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "engine_error", "Failed to get handler: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, handler)
}

// GET /handlers/{id}/health
func (a *AdminAPI) handleGetHandlerHealth(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "missing_id", "handler id is required")
		return
	}

	if a.localEngine == nil {
		writeError(w, http.StatusServiceUnavailable, "no_engine", "No engine configured")
		return
	}

	handler, err := a.localEngine.GetHandler(r.Context(), id)
	if err != nil {
		if err == engineclient.ErrNotFound {
			writeError(w, http.StatusNotFound, "not_found", "handler not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "engine_error", "Failed to get handler: "+err.Error())
		return
	}

	// Return basic health info from handler status
	writeJSON(w, http.StatusOK, map[string]string{
		"status": handler.Status,
		"id":     handler.ID,
	})
}

// GET /handlers/{id}/stats
func (a *AdminAPI) handleGetHandlerStats(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "missing_id", "handler id is required")
		return
	}

	if a.localEngine == nil {
		writeError(w, http.StatusServiceUnavailable, "no_engine", "No engine configured")
		return
	}

	handler, err := a.localEngine.GetHandler(r.Context(), id)
	if err != nil {
		if err == engineclient.ErrNotFound {
			writeError(w, http.StatusNotFound, "not_found", "handler not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "engine_error", "Failed to get handler: "+err.Error())
		return
	}

	// Return basic stats from handler
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"connections": handler.Connections,
		"status":      handler.Status,
	})
}

// POST /handlers/{id}/start
func (a *AdminAPI) handleStartHandler(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "missing_id", "handler id is required")
		return
	}

	// TODO: Implement handler start via engine API
	writeError(w, http.StatusNotImplemented, "not_implemented", "Handler start/stop control is not yet available via HTTP API")
}

// POST /handlers/{id}/stop
func (a *AdminAPI) handleStopHandler(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "missing_id", "handler id is required")
		return
	}

	// TODO: Implement handler stop via engine API
	writeError(w, http.StatusNotImplemented, "not_implemented", "Handler start/stop control is not yet available via HTTP API")
}

// POST /handlers/{id}/recording/enable
func (a *AdminAPI) handleEnableHandlerRecording(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "missing_id", "handler id is required")
		return
	}

	// TODO: Implement recording control via engine API
	writeError(w, http.StatusNotImplemented, "not_implemented", "Handler recording control is not yet available via HTTP API")
}

// POST /handlers/{id}/recording/disable
func (a *AdminAPI) handleDisableHandlerRecording(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "missing_id", "handler id is required")
		return
	}

	// TODO: Implement recording control via engine API
	writeError(w, http.StatusNotImplemented, "not_implemented", "Handler recording control is not yet available via HTTP API")
}

// GET /handlers/{id}/connections
func (a *AdminAPI) handleListHandlerConnections(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "missing_id", "handler id is required")
		return
	}

	// TODO: Implement connection listing via engine API
	writeError(w, http.StatusNotImplemented, "not_implemented", "Handler connection listing is not yet available via HTTP API")
}

// DELETE /handlers/{id}/connections/{connId}
func (a *AdminAPI) handleCloseHandlerConnection(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "missing_id", "handler id is required")
		return
	}

	connID := r.PathValue("connId")
	if connID == "" {
		writeError(w, http.StatusBadRequest, "missing_conn_id", "connection id is required")
		return
	}

	// TODO: Implement connection close via engine API
	writeError(w, http.StatusNotImplemented, "not_implemented", "Handler connection management is not yet available via HTTP API")
}

// POST /handlers/{id}/broadcast
func (a *AdminAPI) handleBroadcastHandler(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "missing_id", "handler id is required")
		return
	}

	// TODO: Implement broadcast via engine API
	writeError(w, http.StatusNotImplemented, "not_implemented", "Handler broadcast is not yet available via HTTP API")
}
