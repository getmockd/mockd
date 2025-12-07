package admin

import (
	"encoding/json"
	"net/http"
)

// handleStateOverview returns information about all stateful resources.
func (a *AdminAPI) handleStateOverview(w http.ResponseWriter, r *http.Request) {
	store := a.server.StatefulStore()
	if store == nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(map[string]string{"error": "stateful store not available"})
		return
	}

	overview := store.Overview()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(overview)
}

// handleStateReset resets stateful resources to their seed data.
func (a *AdminAPI) handleStateReset(w http.ResponseWriter, r *http.Request) {
	store := a.server.StatefulStore()
	if store == nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(map[string]string{"error": "stateful store not available"})
		return
	}

	resourceName := r.URL.Query().Get("resource")

	result, err := store.Reset(resourceName)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(result)
}

// handleListStateResources returns a list of all registered stateful resources.
func (a *AdminAPI) handleListStateResources(w http.ResponseWriter, r *http.Request) {
	store := a.server.StatefulStore()
	if store == nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(map[string]string{"error": "stateful store not available"})
		return
	}

	names := store.List()
	resources := make([]interface{}, 0, len(names))

	for _, name := range names {
		if info, err := store.ResourceInfo(name); err == nil {
			resources = append(resources, info)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(resources)
}

// handleGetStateResource returns details about a specific stateful resource.
func (a *AdminAPI) handleGetStateResource(w http.ResponseWriter, r *http.Request) {
	store := a.server.StatefulStore()
	if store == nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(map[string]string{"error": "stateful store not available"})
		return
	}

	name := r.PathValue("name")
	if name == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "resource name required"})
		return
	}

	info, err := store.ResourceInfo(name)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error(), "resource": name})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(info)
}

// handleClearStateResource clears all items from a specific resource (does not restore seed data).
func (a *AdminAPI) handleClearStateResource(w http.ResponseWriter, r *http.Request) {
	store := a.server.StatefulStore()
	if store == nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(map[string]string{"error": "stateful store not available"})
		return
	}

	name := r.PathValue("name")
	if name == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "resource name required"})
		return
	}

	count, err := store.ClearResource(name)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error(), "resource": name})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{"cleared": count})
}
