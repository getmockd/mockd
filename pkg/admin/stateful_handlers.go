package admin

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

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
		if errors.Is(err, engineclient.ErrNotFound) {
			writeError(w, http.StatusNotFound, "not_found", "Resource not found")
			return
		}
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
		if errors.Is(err, engineclient.ErrNotFound) {
			writeError(w, http.StatusNotFound, "not_found", "Resource not found")
			return
		}
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

// handleListStatefulItems returns paginated items for a stateful resource.
func (a *API) handleListStatefulItems(w http.ResponseWriter, r *http.Request, engine *engineclient.Client) {
	ctx := r.Context()
	name := r.PathValue("name")
	if name == "" {
		writeError(w, http.StatusBadRequest, "missing_name", "Resource name is required")
		return
	}

	limit := 100
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			limit = n
		}
	}

	offset := 0
	if v := r.URL.Query().Get("offset"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			offset = n
		}
	}

	sort := r.URL.Query().Get("sort")
	if sort == "" {
		sort = "createdAt"
	}
	order := r.URL.Query().Get("order")
	if order == "" {
		order = "desc"
	}

	result, err := engine.ListStatefulItems(ctx, name, limit, offset, sort, order)
	if err != nil {
		if errors.Is(err, engineclient.ErrNotFound) {
			writeError(w, http.StatusNotFound, "not_found", "Resource not found")
			return
		}
		writeError(w, http.StatusServiceUnavailable, "engine_error", sanitizeEngineError(err, a.logger(), "list stateful items"))
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// handleGetStatefulItem returns a specific item from a stateful resource.
func (a *API) handleGetStatefulItem(w http.ResponseWriter, r *http.Request, engine *engineclient.Client) {
	ctx := r.Context()
	name := r.PathValue("name")
	if name == "" {
		writeError(w, http.StatusBadRequest, "missing_name", "Resource name is required")
		return
	}

	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "missing_id", "Item ID is required")
		return
	}

	item, err := engine.GetStatefulItem(ctx, name, id)
	if err != nil {
		if errors.Is(err, engineclient.ErrNotFound) {
			writeError(w, http.StatusNotFound, "not_found", "Item not found")
			return
		}
		writeError(w, http.StatusServiceUnavailable, "engine_error", sanitizeEngineError(err, a.logger(), "get stateful item"))
		return
	}

	writeJSON(w, http.StatusOK, item)
}

// handleCreateStatefulItem creates a new item in a stateful resource.
func (a *API) handleCreateStatefulItem(w http.ResponseWriter, r *http.Request, engine *engineclient.Client) {
	ctx := r.Context()
	name := r.PathValue("name")
	if name == "" {
		writeError(w, http.StatusBadRequest, "missing_name", "Resource name is required")
		return
	}

	var data map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "Invalid JSON in request body")
		return
	}

	item, err := engine.CreateStatefulItem(ctx, name, data)
	if err != nil {
		if errors.Is(err, engineclient.ErrNotFound) {
			writeError(w, http.StatusNotFound, "not_found", "Resource not found")
			return
		}
		if errors.Is(err, engineclient.ErrConflict) {
			writeError(w, http.StatusConflict, "conflict", sanitizeEngineError(err, a.logger(), "create stateful item"))
			return
		}
		if errors.Is(err, engineclient.ErrCapacity) {
			writeError(w, http.StatusInsufficientStorage, "capacity_exceeded", sanitizeEngineError(err, a.logger(), "create stateful item"))
			return
		}
		writeError(w, http.StatusBadRequest, "create_failed", sanitizeEngineError(err, a.logger(), "create stateful item"))
		return
	}

	writeJSON(w, http.StatusCreated, item)
}
