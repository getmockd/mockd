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

// GET /handlers
func (a *API) handleListHandlers(w http.ResponseWriter, r *http.Request) {
	if a.localEngine == nil {
		writeJSON(w, http.StatusOK, HandlersListResponse{Handlers: []*engineclient.ProtocolHandler{}, Total: 0})
		return
	}

	handlers, err := a.localEngine.ListHandlers(r.Context())
	if err != nil {
		a.log.Error("failed to list handlers", "error", err)
		writeError(w, http.StatusInternalServerError, "engine_error", ErrMsgEngineUnavailable)
		return
	}

	if handlers == nil {
		handlers = []*engineclient.ProtocolHandler{}
	}

	writeJSON(w, http.StatusOK, HandlersListResponse{Handlers: handlers, Total: len(handlers)})
}

// GET /handlers/{id}
func (a *API) handleGetHandler(w http.ResponseWriter, r *http.Request, engine *engineclient.Client) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "missing_id", "handler id is required")
		return
	}

	handler, err := engine.GetHandler(r.Context(), id)
	if err != nil {
		if err == engineclient.ErrNotFound {
			writeError(w, http.StatusNotFound, "not_found", "handler not found")
			return
		}
		writeError(w, http.StatusServiceUnavailable, "engine_error", sanitizeEngineError(err, a.log, "get handler"))
		return
	}

	writeJSON(w, http.StatusOK, handler)
}

// GET /handlers/{id}/health
func (a *API) handleGetHandlerHealth(w http.ResponseWriter, r *http.Request, engine *engineclient.Client) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "missing_id", "handler id is required")
		return
	}

	handler, err := engine.GetHandler(r.Context(), id)
	if err != nil {
		if err == engineclient.ErrNotFound {
			writeError(w, http.StatusNotFound, "not_found", "handler not found")
			return
		}
		writeError(w, http.StatusServiceUnavailable, "engine_error", sanitizeEngineError(err, a.log, "get handler health"))
		return
	}

	// Return basic health info from handler status
	writeJSON(w, http.StatusOK, map[string]string{
		"status": handler.Status,
		"id":     handler.ID,
	})
}

// GET /handlers/{id}/stats
func (a *API) handleGetHandlerStats(w http.ResponseWriter, r *http.Request, engine *engineclient.Client) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "missing_id", "handler id is required")
		return
	}

	handler, err := engine.GetHandler(r.Context(), id)
	if err != nil {
		if err == engineclient.ErrNotFound {
			writeError(w, http.StatusNotFound, "not_found", "handler not found")
			return
		}
		writeError(w, http.StatusServiceUnavailable, "engine_error", sanitizeEngineError(err, a.log, "get handler stats"))
		return
	}

	// Return basic stats from handler
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"connections": handler.Connections,
		"status":      handler.Status,
	})
}
