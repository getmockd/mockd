package admin

import (
	"encoding/json"
	"net/http"

	"github.com/getmockd/mockd/pkg/admin/engineclient"
)

// handleGetChaos returns the current chaos configuration.
func (a *AdminAPI) handleGetChaos(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if a.localEngine == nil {
		writeError(w, http.StatusServiceUnavailable, "no_engine", "No engine connected")
		return
	}

	chaosConfig, err := a.localEngine.GetChaos(ctx)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "engine_error", err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(chaosConfig)
}

// handleSetChaos updates the chaos configuration.
func (a *AdminAPI) handleSetChaos(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if a.localEngine == nil {
		writeError(w, http.StatusServiceUnavailable, "no_engine", "No engine connected")
		return
	}

	var config engineclient.ChaosConfig
	if err := json.NewDecoder(r.Body).Decode(&config); err != nil {
		http.Error(w, `{"error":"invalid JSON: `+err.Error()+`"}`, http.StatusBadRequest)
		return
	}

	if err := a.localEngine.SetChaos(ctx, &config); err != nil {
		http.Error(w, `{"error":"failed to set chaos config: `+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"status":"ok"}`))
}
