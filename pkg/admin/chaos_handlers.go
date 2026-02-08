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
		writeError(w, http.StatusServiceUnavailable, "engine_error", sanitizeEngineError(err, a.log, "get chaos config"))
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
		writeError(w, http.StatusBadRequest, "invalid_json", sanitizeJSONError(err, a.log))
		return
	}

	if err := engine.SetChaos(ctx, &config); err != nil {
		writeError(w, http.StatusServiceUnavailable, "engine_error", sanitizeEngineError(err, a.log, "set chaos config"))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"status":"ok"}`))
}
