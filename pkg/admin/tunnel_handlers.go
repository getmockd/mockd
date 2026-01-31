// Tunnel management handlers for the Admin API.

package admin

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/getmockd/mockd/pkg/store"
)

// ============================================================================
// Request / Response types
// ============================================================================

// TunnelEnableRequest is the body for POST /engines/{id}/tunnel/enable.
type TunnelEnableRequest struct {
	Expose       store.TunnelExposure `json:"expose"`
	Subdomain    string               `json:"subdomain,omitempty"`
	CustomDomain string               `json:"customDomain,omitempty"`
	Auth         *store.TunnelAuth    `json:"auth,omitempty"`
}

// TunnelEnableResponse is the response for POST /engines/{id}/tunnel/enable.
type TunnelEnableResponse struct {
	Enabled   bool                 `json:"enabled"`
	PublicURL string               `json:"publicUrl"`
	Subdomain string               `json:"subdomain"`
	Status    string               `json:"status"`
	Expose    store.TunnelExposure `json:"expose"`
}

// TunnelStatusResponse is the response for GET /engines/{id}/tunnel/status.
type TunnelStatusResponse struct {
	Enabled           bool                  `json:"enabled"`
	Status            string                `json:"status"` // "connected","disconnected","connecting","error"
	PublicURL         string                `json:"publicUrl,omitempty"`
	Subdomain         string                `json:"subdomain,omitempty"`
	SessionID         string                `json:"sessionId,omitempty"`
	ConnectedAt       *time.Time            `json:"connectedAt,omitempty"`
	Transport         string                `json:"transport,omitempty"` // "quic","websocket"
	Stats             *store.TunnelStats    `json:"stats,omitempty"`
	Expose            *store.TunnelExposure `json:"expose,omitempty"`
	ResolvedMockCount int                   `json:"resolvedMockCount,omitempty"`
}

// TunnelConfigUpdateRequest is the body for PUT /engines/{id}/tunnel/config.
type TunnelConfigUpdateRequest struct {
	Expose       *store.TunnelExposure `json:"expose,omitempty"`
	Subdomain    *string               `json:"subdomain,omitempty"`
	CustomDomain *string               `json:"customDomain,omitempty"`
	Auth         *store.TunnelAuth     `json:"auth,omitempty"`
}

// TunnelPreviewRequest is the body for POST /engines/{id}/tunnel/preview.
type TunnelPreviewRequest struct {
	Expose store.TunnelExposure `json:"expose"`
}

// TunnelPreviewMock is a single mock in a preview result.
type TunnelPreviewMock struct {
	ID        string `json:"id"`
	Type      string `json:"type"`
	Name      string `json:"name"`
	Workspace string `json:"workspace"`
	Folder    string `json:"folder,omitempty"`
}

// TunnelPreviewResponse is the response for POST /engines/{id}/tunnel/preview.
type TunnelPreviewResponse struct {
	MockCount int                 `json:"mockCount"`
	Mocks     []TunnelPreviewMock `json:"mocks"`
	Protocols map[string]int      `json:"protocols"` // e.g. {"http": 12, "grpc": 2}
}

// TunnelListItem is a single tunnel in the list response.
type TunnelListItem struct {
	EngineID   string `json:"engineId"`
	EngineName string `json:"engineName"`
	PublicURL  string `json:"publicUrl"`
	Status     string `json:"status"`
	Transport  string `json:"transport"`
	MockCount  int    `json:"mockCount"`
	Uptime     string `json:"uptime"`
}

// TunnelListResponse is the response for GET /tunnels.
type TunnelListResponse struct {
	Tunnels []TunnelListItem `json:"tunnels"`
	Total   int              `json:"total"`
}

// ============================================================================
// Handlers
// ============================================================================

// handleEnableTunnel handles POST /engines/{id}/tunnel/enable.
func (a *AdminAPI) handleEnableTunnel(w http.ResponseWriter, r *http.Request) {
	engineID := r.PathValue("id")
	if engineID == "" {
		writeError(w, http.StatusBadRequest, "missing_id", "Engine ID is required")
		return
	}

	// For local engine, use LocalEngineID
	engine, err := a.resolveEngine(engineID)
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", "Engine not found")
		return
	}

	var req TunnelEnableRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "Failed to parse request body: "+err.Error())
		return
	}

	// Default exposure mode
	if req.Expose.Mode == "" {
		req.Expose.Mode = "all"
	}

	// Validate exposure mode
	switch req.Expose.Mode {
	case "all", "selected", "none":
		// ok
	default:
		writeError(w, http.StatusBadRequest, "invalid_mode", "Exposure mode must be 'all', 'selected', or 'none'")
		return
	}

	// Build tunnel config
	cfg := &store.TunnelConfig{
		Enabled:      true,
		Subdomain:    req.Subdomain,
		CustomDomain: req.CustomDomain,
		Expose:       req.Expose,
		Auth:         req.Auth,
		Status:       "disconnected", // Will become "connecting" when engine connects
	}

	// Generate subdomain if not provided
	if cfg.Subdomain == "" && cfg.CustomDomain == "" {
		cfg.Subdomain = generateSubdomain(engine.ID)
	}

	// Build the public URL
	if cfg.CustomDomain != "" {
		cfg.PublicURL = fmt.Sprintf("https://%s", cfg.CustomDomain)
	} else {
		cfg.PublicURL = fmt.Sprintf("https://%s.tunnel.mockd.io", cfg.Subdomain)
	}

	// Store tunnel config on the engine
	if engineID == LocalEngineID {
		// For local engine, store in admin's local tunnel state
		a.setLocalTunnelConfig(cfg)
	} else {
		if err := a.engineRegistry.SetTunnelConfig(engineID, cfg); err != nil {
			writeError(w, http.StatusInternalServerError, "store_failed", "Failed to store tunnel config: "+err.Error())
			return
		}
	}

	writeJSON(w, http.StatusOK, TunnelEnableResponse{
		Enabled:   true,
		PublicURL: cfg.PublicURL,
		Subdomain: cfg.Subdomain,
		Status:    cfg.Status,
		Expose:    cfg.Expose,
	})
}

// handleDisableTunnel handles POST /engines/{id}/tunnel/disable.
func (a *AdminAPI) handleDisableTunnel(w http.ResponseWriter, r *http.Request) {
	engineID := r.PathValue("id")
	if engineID == "" {
		writeError(w, http.StatusBadRequest, "missing_id", "Engine ID is required")
		return
	}

	if engineID == LocalEngineID {
		a.setLocalTunnelConfig(nil)
	} else {
		if err := a.engineRegistry.SetTunnelConfig(engineID, nil); err != nil {
			writeError(w, http.StatusNotFound, "not_found", "Engine not found")
			return
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"enabled": false,
		"status":  "disconnected",
	})
}

// handleGetTunnelConfig handles GET /engines/{id}/tunnel/config.
func (a *AdminAPI) handleGetTunnelConfig(w http.ResponseWriter, r *http.Request) {
	engineID := r.PathValue("id")
	if engineID == "" {
		writeError(w, http.StatusBadRequest, "missing_id", "Engine ID is required")
		return
	}

	cfg, err := a.getTunnelConfig(engineID)
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", "Engine not found")
		return
	}

	if cfg == nil {
		writeJSON(w, http.StatusOK, map[string]any{
			"enabled": false,
		})
		return
	}

	writeJSON(w, http.StatusOK, cfg)
}

// handleUpdateTunnelConfig handles PUT /engines/{id}/tunnel/config.
func (a *AdminAPI) handleUpdateTunnelConfig(w http.ResponseWriter, r *http.Request) {
	engineID := r.PathValue("id")
	if engineID == "" {
		writeError(w, http.StatusBadRequest, "missing_id", "Engine ID is required")
		return
	}

	existing, err := a.getTunnelConfig(engineID)
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", "Engine not found")
		return
	}

	if existing == nil {
		writeError(w, http.StatusConflict, "not_enabled", "Tunnel is not enabled. Use POST /engines/{id}/tunnel/enable first.")
		return
	}

	var req TunnelConfigUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "Failed to parse request body: "+err.Error())
		return
	}

	// Apply partial updates
	if req.Expose != nil {
		existing.Expose = *req.Expose
	}
	if req.Subdomain != nil {
		existing.Subdomain = *req.Subdomain
		if existing.CustomDomain == "" {
			existing.PublicURL = fmt.Sprintf("https://%s.tunnel.mockd.io", existing.Subdomain)
		}
	}
	if req.CustomDomain != nil {
		existing.CustomDomain = *req.CustomDomain
		if *req.CustomDomain != "" {
			existing.PublicURL = fmt.Sprintf("https://%s", *req.CustomDomain)
		}
	}
	if req.Auth != nil {
		existing.Auth = req.Auth
	}

	// Store updated config
	if engineID == LocalEngineID {
		a.setLocalTunnelConfig(existing)
	} else {
		if err := a.engineRegistry.SetTunnelConfig(engineID, existing); err != nil {
			writeError(w, http.StatusInternalServerError, "store_failed", "Failed to store tunnel config: "+err.Error())
			return
		}
	}

	writeJSON(w, http.StatusOK, existing)
}

// handleGetTunnelStatus handles GET /engines/{id}/tunnel/status.
func (a *AdminAPI) handleGetTunnelStatus(w http.ResponseWriter, r *http.Request) {
	engineID := r.PathValue("id")
	if engineID == "" {
		writeError(w, http.StatusBadRequest, "missing_id", "Engine ID is required")
		return
	}

	cfg, err := a.getTunnelConfig(engineID)
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", "Engine not found")
		return
	}

	if cfg == nil {
		writeJSON(w, http.StatusOK, TunnelStatusResponse{
			Enabled: false,
			Status:  "disconnected",
		})
		return
	}

	// Count resolved mocks for the current exposure config
	resolvedMocks := a.previewExposedMocks(r, cfg.Expose)

	writeJSON(w, http.StatusOK, TunnelStatusResponse{
		Enabled:     cfg.Enabled,
		Status:      cfg.Status,
		PublicURL:   cfg.PublicURL,
		Subdomain:   cfg.Subdomain,
		SessionID:   cfg.SessionID,
		ConnectedAt: cfg.ConnectedAt,
		Transport:   cfg.Transport,
		// Stats will be populated when TunnelManager reports them
		Expose:            &cfg.Expose,
		ResolvedMockCount: len(resolvedMocks),
	})
}

// handleTunnelPreview handles POST /engines/{id}/tunnel/preview.
func (a *AdminAPI) handleTunnelPreview(w http.ResponseWriter, r *http.Request) {
	engineID := r.PathValue("id")
	if engineID == "" {
		writeError(w, http.StatusBadRequest, "missing_id", "Engine ID is required")
		return
	}

	// Verify engine exists
	if _, err := a.resolveEngine(engineID); err != nil {
		writeError(w, http.StatusNotFound, "not_found", "Engine not found")
		return
	}

	var req TunnelPreviewRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "Failed to parse request body: "+err.Error())
		return
	}

	// Default mode
	if req.Expose.Mode == "" {
		req.Expose.Mode = "all"
	}

	// For now, return a summary of what would be exposed.
	// Full mock enumeration requires the engine's mock inventory.
	// We'll do a simple count from the data store.
	mocks := a.previewExposedMocks(r, req.Expose)

	protocols := make(map[string]int)
	for _, m := range mocks {
		protocols[m.Type]++
	}

	writeJSON(w, http.StatusOK, TunnelPreviewResponse{
		MockCount: len(mocks),
		Mocks:     mocks,
		Protocols: protocols,
	})
}

// handleListTunnels handles GET /tunnels.
func (a *AdminAPI) handleListTunnels(w http.ResponseWriter, r *http.Request) {
	var items []TunnelListItem

	// Check local engine tunnel
	if a.localEngine != nil {
		cfg := a.getLocalTunnelConfig()
		if cfg != nil && cfg.Enabled {
			uptime := ""
			if cfg.ConnectedAt != nil {
				uptime = time.Since(*cfg.ConnectedAt).Round(time.Second).String()
			}
			mockCount := len(a.previewExposedMocks(r, cfg.Expose))
			items = append(items, TunnelListItem{
				EngineID:   LocalEngineID,
				EngineName: "Local Engine",
				PublicURL:  cfg.PublicURL,
				Status:     cfg.Status,
				Transport:  cfg.Transport,
				MockCount:  mockCount,
				Uptime:     uptime,
			})
		}
	}

	// Check registered engines
	tunnelEngines := a.engineRegistry.ListTunnels()
	for _, eng := range tunnelEngines {
		if eng.Tunnel == nil {
			continue
		}
		uptime := ""
		if eng.Tunnel.ConnectedAt != nil {
			uptime = time.Since(*eng.Tunnel.ConnectedAt).Round(time.Second).String()
		}
		mockCount := len(a.previewExposedMocks(r, eng.Tunnel.Expose))
		items = append(items, TunnelListItem{
			EngineID:   eng.ID,
			EngineName: eng.Name,
			PublicURL:  eng.Tunnel.PublicURL,
			Status:     eng.Tunnel.Status,
			Transport:  eng.Tunnel.Transport,
			MockCount:  mockCount,
			Uptime:     uptime,
		})
	}

	writeJSON(w, http.StatusOK, TunnelListResponse{
		Tunnels: items,
		Total:   len(items),
	})
}

// ============================================================================
// Internal helpers
// ============================================================================

// resolveEngine resolves an engine by ID, supporting "local" for co-located engine.
func (a *AdminAPI) resolveEngine(id string) (*store.Engine, error) {
	if id == LocalEngineID {
		if a.localEngine == nil {
			return nil, store.ErrNotFound
		}
		return &store.Engine{
			ID:   LocalEngineID,
			Name: "Local Engine",
			Host: "localhost",
		}, nil
	}
	return a.engineRegistry.Get(id)
}

// getTunnelConfig retrieves tunnel config for an engine (local or registry).
func (a *AdminAPI) getTunnelConfig(engineID string) (*store.TunnelConfig, error) {
	if engineID == LocalEngineID {
		return a.getLocalTunnelConfig(), nil
	}
	return a.engineRegistry.GetTunnelConfig(engineID)
}

// getLocalTunnelConfig returns the local engine's tunnel config.
func (a *AdminAPI) getLocalTunnelConfig() *store.TunnelConfig {
	a.tunnelMu.RLock()
	defer a.tunnelMu.RUnlock()
	return a.localTunnel
}

// setLocalTunnelConfig stores the local engine's tunnel config.
func (a *AdminAPI) setLocalTunnelConfig(cfg *store.TunnelConfig) {
	a.tunnelMu.Lock()
	defer a.tunnelMu.Unlock()
	a.localTunnel = cfg
}

// generateSubdomain generates a random subdomain from the engine ID.
func generateSubdomain(engineID string) string {
	// Use first 8 chars of engine ID or a hash
	if len(engineID) >= 8 {
		return engineID[:8]
	}
	return engineID
}

// previewExposedMocks returns the mocks that would be exposed given an exposure config.
// For MVP this does a simple enumeration from the data store.
func (a *AdminAPI) previewExposedMocks(r *http.Request, expose store.TunnelExposure) []TunnelPreviewMock {
	if expose.Mode == "none" {
		return []TunnelPreviewMock{}
	}

	// Get all mocks from the data store
	allMocks, err := a.dataStore.Mocks().List(r.Context(), nil)
	if err != nil {
		return []TunnelPreviewMock{}
	}

	var result []TunnelPreviewMock
	for _, m := range allMocks {
		mockType := "http"
		if m.Type != "" {
			mockType = string(m.Type)
		}

		// For "selected" mode, apply include filters first
		if expose.Mode == "selected" {
			included := false

			// Check workspace filter
			if len(expose.Workspaces) > 0 {
				for _, ws := range expose.Workspaces {
					if m.WorkspaceID == ws {
						included = true
						break
					}
				}
			}

			// Check folder filter
			if !included && len(expose.Folders) > 0 {
				for _, f := range expose.Folders {
					if m.ParentID == f {
						included = true
						break
					}
				}
			}

			// Check mock ID filter
			if !included && len(expose.Mocks) > 0 {
				for _, id := range expose.Mocks {
					if m.ID == id {
						included = true
						break
					}
				}
			}

			// Check type filter
			if !included && len(expose.Types) > 0 {
				for _, t := range expose.Types {
					if mockType == t {
						included = true
						break
					}
				}
			}

			// If no include filters matched, skip this mock
			if !included {
				continue
			}
		}

		// Apply exclusions (for both "all" and "selected" modes)
		if expose.Exclude != nil {
			if isExcluded(expose.Exclude, m.WorkspaceID, m.ParentID, m.ID) {
				continue
			}
		}

		result = append(result, TunnelPreviewMock{
			ID:        m.ID,
			Type:      mockType,
			Name:      m.Name,
			Workspace: m.WorkspaceID,
			Folder:    m.ParentID,
		})
	}

	return result
}

// isExcluded checks whether a mock matches any exclusion rule.
func isExcluded(excl *store.TunnelExclude, workspaceID, parentID, mockID string) bool {
	for _, ws := range excl.Workspaces {
		if workspaceID == ws {
			return true
		}
	}
	for _, f := range excl.Folders {
		if parentID == f {
			return true
		}
	}
	for _, id := range excl.Mocks {
		if mockID == id {
			return true
		}
	}
	return false
}
