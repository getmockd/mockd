package admin

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/getmockd/mockd/pkg/admin/engineclient"
	"github.com/getmockd/mockd/pkg/chaos"
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

// --- Chaos Profiles ---

// chaosProfileResponse is the API-level representation of a chaos profile.
type chaosProfileResponse struct {
	Name        string                   `json:"name"`
	Description string                   `json:"description"`
	Config      engineclient.ChaosConfig `json:"config"`
}

// handleListChaosProfiles returns all available built-in chaos profiles.
func (a *API) handleListChaosProfiles(w http.ResponseWriter, r *http.Request) {
	profiles := chaos.ListProfiles()

	resp := make([]chaosProfileResponse, 0, len(profiles))
	for _, p := range profiles {
		resp = append(resp, chaosProfileResponse{
			Name:        p.Name,
			Description: p.Description,
			Config:      chaosConfigToAPI(&p.Config),
		})
	}

	writeJSON(w, http.StatusOK, resp)
}

// handleGetChaosProfile returns a specific chaos profile by name.
func (a *API) handleGetChaosProfile(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" {
		writeError(w, http.StatusBadRequest, "missing_name", "profile name is required")
		return
	}

	p, ok := chaos.GetProfile(name)
	if !ok {
		writeError(w, http.StatusNotFound, "not_found", "chaos profile not found: "+name)
		return
	}

	writeJSON(w, http.StatusOK, chaosProfileResponse{
		Name:        p.Name,
		Description: p.Description,
		Config:      chaosConfigToAPI(&p.Config),
	})
}

// handleApplyChaosProfile applies a named chaos profile to the engine.
func (a *API) handleApplyChaosProfile(w http.ResponseWriter, r *http.Request, engine *engineclient.Client) {
	ctx := r.Context()

	name := r.PathValue("name")
	if name == "" {
		writeError(w, http.StatusBadRequest, "missing_name", "profile name is required")
		return
	}

	p, ok := chaos.GetProfile(name)
	if !ok {
		writeError(w, http.StatusNotFound, "not_found", "chaos profile not found: "+name)
		return
	}

	apiConfig := chaosConfigToAPI(&p.Config)
	if err := engine.SetChaos(ctx, &apiConfig); err != nil {
		errMsg := err.Error()
		if strings.Contains(errMsg, "must be between") || strings.Contains(errMsg, "validation") {
			writeError(w, http.StatusBadRequest, "validation_error", errMsg)
			return
		}
		writeError(w, http.StatusServiceUnavailable, "engine_error", sanitizeEngineError(err, a.logger(), "apply chaos profile"))
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status":  "ok",
		"profile": name,
		"message": "chaos profile applied: " + p.Description,
	})
}

// chaosConfigToAPI converts an internal chaos.ChaosConfig to the API-level
// engineclient.ChaosConfig used for admin-engine communication.
func chaosConfigToAPI(src *chaos.ChaosConfig) engineclient.ChaosConfig {
	cfg := engineclient.ChaosConfig{
		Enabled: src.Enabled,
	}

	if src.GlobalRules != nil {
		if src.GlobalRules.Latency != nil {
			cfg.Latency = &engineclient.LatencyConfig{
				Min:         src.GlobalRules.Latency.Min,
				Max:         src.GlobalRules.Latency.Max,
				Probability: src.GlobalRules.Latency.Probability,
			}
		}
		if src.GlobalRules.ErrorRate != nil {
			cfg.ErrorRate = &engineclient.ErrorRateConfig{
				Probability: src.GlobalRules.ErrorRate.Probability,
				DefaultCode: src.GlobalRules.ErrorRate.DefaultCode,
			}
			if len(src.GlobalRules.ErrorRate.StatusCodes) > 0 {
				cfg.ErrorRate.StatusCodes = make([]int, len(src.GlobalRules.ErrorRate.StatusCodes))
				copy(cfg.ErrorRate.StatusCodes, src.GlobalRules.ErrorRate.StatusCodes)
			}
		}
		if src.GlobalRules.Bandwidth != nil {
			cfg.Bandwidth = &engineclient.BandwidthConfig{
				BytesPerSecond: src.GlobalRules.Bandwidth.BytesPerSecond,
				Probability:    src.GlobalRules.Bandwidth.Probability,
			}
		}
	}

	return cfg
}
