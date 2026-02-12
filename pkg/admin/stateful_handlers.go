package admin

import (
	"net/http"

	"github.com/getmockd/mockd/pkg/admin/engineclient"
)

// handleStateOverview returns information about all stateful resources.
func (a *API) handleStateOverview(w http.ResponseWriter, r *http.Request, engine *engineclient.Client) {
	ctx := r.Context()

	overview, err := engine.GetStateOverview(ctx)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "engine_error", sanitizeEngineError(err, a.logger(), "get state overview"))
		return
	}

	writeJSON(w, http.StatusOK, overview)
}

// handleStateReset resets stateful resources to their seed data.
func (a *API) handleStateReset(w http.ResponseWriter, r *http.Request, engine *engineclient.Client) {
	ctx := r.Context()

	// Get optional resource filter from query param
	resourceName := r.URL.Query().Get("resource")

	if err := engine.ResetState(ctx, resourceName); err != nil {
		writeError(w, http.StatusServiceUnavailable, "engine_error", sanitizeEngineError(err, a.logger(), "reset state"))
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "reset"})
}

// handleResetStateResource resets a specific stateful resource to its seed data.
func (a *API) handleResetStateResource(w http.ResponseWriter, r *http.Request, engine *engineclient.Client) {
	ctx := r.Context()
	name := r.PathValue("name")
	if name == "" {
		writeError(w, http.StatusBadRequest, "missing_name", "Resource name is required")
		return
	}

	if err := engine.ResetState(ctx, name); err != nil {
		writeError(w, http.StatusServiceUnavailable, "engine_error", sanitizeEngineError(err, a.logger(), "reset state resource"))
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "reset", "resource": name})
}

// handleListStateResources returns a list of all registered stateful resources.
func (a *API) handleListStateResources(w http.ResponseWriter, r *http.Request, engine *engineclient.Client) {
	ctx := r.Context()

	overview, err := engine.GetStateOverview(ctx)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "engine_error", sanitizeEngineError(err, a.logger(), "list state resources"))
		return
	}

	writeJSON(w, http.StatusOK, overview.Resources)
}

// handleGetStateResource returns details about a specific stateful resource.
func (a *API) handleGetStateResource(w http.ResponseWriter, r *http.Request, engine *engineclient.Client) {
	ctx := r.Context()

	name := r.PathValue("name")
	if name == "" {
		writeError(w, http.StatusBadRequest, "missing_name", "Resource name is required")
		return
	}

	resource, err := engine.GetStateResource(ctx, name)
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", "Resource not found")
		return
	}

	writeJSON(w, http.StatusOK, resource)
}

// handleClearStateResource clears all items from a specific resource (does not restore seed data).
func (a *API) handleClearStateResource(w http.ResponseWriter, r *http.Request, engine *engineclient.Client) {
	ctx := r.Context()

	name := r.PathValue("name")
	if name == "" {
		writeError(w, http.StatusBadRequest, "missing_name", "Resource name is required")
		return
	}

	if err := engine.ClearStateResource(ctx, name); err != nil {
		writeError(w, http.StatusNotFound, "not_found", "Resource not found")
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{"cleared": true})
}
