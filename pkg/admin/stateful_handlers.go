package admin

import (
	"encoding/json"
	"net/http"
)

// handleStateOverview returns information about all stateful resources.
func (a *AdminAPI) handleStateOverview(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if a.localEngine == nil {
		writeError(w, http.StatusServiceUnavailable, "no_engine", "No engine connected")
		return
	}

	overview, err := a.localEngine.GetStateOverview(ctx)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "engine_error", err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(overview)
}

// handleStateReset resets stateful resources to their seed data.
func (a *AdminAPI) handleStateReset(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if a.localEngine == nil {
		writeError(w, http.StatusServiceUnavailable, "no_engine", "No engine connected")
		return
	}

	// Get optional resource filter from query param
	resourceName := r.URL.Query().Get("resource")

	if err := a.localEngine.ResetState(ctx, resourceName); err != nil {
		writeError(w, http.StatusInternalServerError, "engine_error", err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "reset"})
}

// handleListStateResources returns a list of all registered stateful resources.
func (a *AdminAPI) handleListStateResources(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if a.localEngine == nil {
		writeError(w, http.StatusServiceUnavailable, "no_engine", "No engine connected")
		return
	}

	overview, err := a.localEngine.GetStateOverview(ctx)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "engine_error", err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(overview.Resources)
}

// handleGetStateResource returns details about a specific stateful resource.
func (a *AdminAPI) handleGetStateResource(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if a.localEngine == nil {
		writeError(w, http.StatusServiceUnavailable, "no_engine", "No engine connected")
		return
	}

	name := r.PathValue("name")
	if name == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "resource name required"})
		return
	}

	resource, err := a.localEngine.GetStateResource(ctx, name)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error(), "resource": name})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(resource)
}

// handleClearStateResource clears all items from a specific resource (does not restore seed data).
func (a *AdminAPI) handleClearStateResource(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if a.localEngine == nil {
		writeError(w, http.StatusServiceUnavailable, "no_engine", "No engine connected")
		return
	}

	name := r.PathValue("name")
	if name == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "resource name required"})
		return
	}

	if err := a.localEngine.ClearStateResource(ctx, name); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error(), "resource": name})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{"cleared": true})
}
