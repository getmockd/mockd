package admin

import (
	"encoding/json"
	"net/http"

	"github.com/getmockd/mockd/pkg/admin/engineclient"
)

// handleStateOverview returns information about all stateful resources.
func (a *API) handleStateOverview(w http.ResponseWriter, r *http.Request, engine *engineclient.Client) {
	ctx := r.Context()

	overview, err := engine.GetStateOverview(ctx)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "engine_error", err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(overview)
}

// handleStateReset resets stateful resources to their seed data.
func (a *API) handleStateReset(w http.ResponseWriter, r *http.Request, engine *engineclient.Client) {
	ctx := r.Context()

	// Get optional resource filter from query param
	resourceName := r.URL.Query().Get("resource")

	if err := engine.ResetState(ctx, resourceName); err != nil {
		writeError(w, http.StatusInternalServerError, "engine_error", err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "reset"})
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
		writeError(w, http.StatusInternalServerError, "engine_error", err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "reset", "resource": name})
}

// handleListStateResources returns a list of all registered stateful resources.
func (a *API) handleListStateResources(w http.ResponseWriter, r *http.Request, engine *engineclient.Client) {
	ctx := r.Context()

	overview, err := engine.GetStateOverview(ctx)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "engine_error", err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(overview.Resources)
}

// handleGetStateResource returns details about a specific stateful resource.
func (a *API) handleGetStateResource(w http.ResponseWriter, r *http.Request, engine *engineclient.Client) {
	ctx := r.Context()

	name := r.PathValue("name")
	if name == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "resource name required"})
		return
	}

	resource, err := engine.GetStateResource(ctx, name)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error(), "resource": name})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resource)
}

// handleClearStateResource clears all items from a specific resource (does not restore seed data).
func (a *API) handleClearStateResource(w http.ResponseWriter, r *http.Request, engine *engineclient.Client) {
	ctx := r.Context()

	name := r.PathValue("name")
	if name == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "resource name required"})
		return
	}

	if err := engine.ClearStateResource(ctx, name); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error(), "resource": name})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{"cleared": true})
}
