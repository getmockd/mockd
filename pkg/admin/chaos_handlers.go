package admin

import (
	"encoding/json"
	"net/http"

	"github.com/getmockd/mockd/pkg/admin/engineclient"
)

// handleGetChaos returns the current chaos configuration.
func (a *API) handleGetChaos(w http.ResponseWriter, r *http.Request, engine *engineclient.Client) {
	ctx := r.Context()

	chaosConfig, err := engine.GetChaos(ctx)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "engine_error", err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(chaosConfig)
}

// handleSetChaos updates the chaos configuration.
func (a *API) handleSetChaos(w http.ResponseWriter, r *http.Request, engine *engineclient.Client) {
	ctx := r.Context()

	var config engineclient.ChaosConfig
	if err := json.NewDecoder(r.Body).Decode(&config); err != nil {
		http.Error(w, `{"error":"invalid JSON: `+err.Error()+`"}`, http.StatusBadRequest)
		return
	}

	if err := engine.SetChaos(ctx, &config); err != nil {
		http.Error(w, `{"error":"failed to set chaos config: `+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"status":"ok"}`))
}
