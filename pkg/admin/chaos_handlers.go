package admin

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/getmockd/mockd/pkg/admin/engineclient"
)

// handleGetChaos returns the current chaos configuration.
func (a *API) handleGetChaos(w http.ResponseWriter, r *http.Request, engine *engineclient.Client) {
	ctx := r.Context()

	chaosConfig, err := engine.GetChaos(ctx)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "engine_error", sanitizeEngineError(err, a.logger(), "get chaos config"))
		return
	}

	writeJSON(w, http.StatusOK, chaosConfig)
}

// handleSetChaos updates the chaos configuration.
func (a *API) handleSetChaos(w http.ResponseWriter, r *http.Request, engine *engineclient.Client) {
	ctx := r.Context()

	var config engineclient.ChaosConfig
	if err := json.NewDecoder(r.Body).Decode(&config); err != nil {
		writeJSONDecodeError(w, err, a.logger())
		return
	}

	if err := engine.SetChaos(ctx, &config); err != nil {
		// Surface validation errors from the engine instead of generic "unavailable"
		errMsg := err.Error()
		if strings.Contains(errMsg, "must be between") || strings.Contains(errMsg, "validation") {
			writeError(w, http.StatusBadRequest, "validation_error", errMsg)
			return
		}
		writeError(w, http.StatusServiceUnavailable, "engine_error", sanitizeEngineError(err, a.logger(), "set chaos config"))
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// handleGetChaosStats returns chaos injection statistics.
func (a *API) handleGetChaosStats(w http.ResponseWriter, r *http.Request, engine *engineclient.Client) {
	ctx := r.Context()

	stats, err := engine.GetChaosStats(ctx)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "engine_error", sanitizeEngineError(err, a.logger(), "get chaos stats"))
		return
	}

	writeJSON(w, http.StatusOK, stats)
}

// handleResetChaosStats resets chaos injection statistics.
func (a *API) handleResetChaosStats(w http.ResponseWriter, r *http.Request, engine *engineclient.Client) {
	ctx := r.Context()

	if err := engine.ResetChaosStats(ctx); err != nil {
		writeError(w, http.StatusServiceUnavailable, "engine_error", sanitizeEngineError(err, a.logger(), "reset chaos stats"))
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"message": "chaos stats reset"})
}
