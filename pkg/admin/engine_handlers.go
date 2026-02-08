package admin

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/getmockd/mockd/internal/id"
	"github.com/getmockd/mockd/pkg/admin/engineclient"
	"github.com/getmockd/mockd/pkg/config"
	"github.com/getmockd/mockd/pkg/store"
)

// EngineListResponse represents a list of engines response.
type EngineListResponse struct {
	Engines []*store.Engine `json:"engines"`
	Total   int             `json:"total"`
}

// RegisterEngineRequest represents a request to register an engine.
type RegisterEngineRequest struct {
	Name        string `json:"name"`
	Host        string `json:"host"`
	Port        int    `json:"port"`
	Version     string `json:"version,omitempty"`
	Fingerprint string `json:"fingerprint,omitempty"` // Machine fingerprint for identity verification
}

// RegisterEngineResponse represents a response after registering an engine.
type RegisterEngineResponse struct {
	ID             string `json:"id"`
	Token          string `json:"token,omitempty"` // Engine-specific token for subsequent calls
	ConfigEndpoint string `json:"configEndpoint"`
}

// TokenListResponse represents a list of registration tokens.
type TokenListResponse struct {
	Tokens []string `json:"tokens"`
	Total  int      `json:"total"`
}

// GenerateTokenResponse represents a response after generating a registration token.
type GenerateTokenResponse struct {
	Token string `json:"token"`
}

// HeartbeatRequest represents a heartbeat request from an engine.
type HeartbeatRequest struct {
	Status store.EngineStatus `json:"status,omitempty"`
}

// AssignWorkspaceRequest represents a request to assign a workspace to an engine.
type AssignWorkspaceRequest struct {
	WorkspaceID string `json:"workspaceId"`
}

// AddEngineWorkspaceRequest represents a request to add a workspace to an engine.
type AddEngineWorkspaceRequest struct {
	WorkspaceID   string `json:"workspaceId"`
	WorkspaceName string `json:"workspaceName,omitempty"`
	HTTPPort      int    `json:"httpPort,omitempty"`
	GRPCPort      int    `json:"grpcPort,omitempty"`
	MQTTPort      int    `json:"mqttPort,omitempty"`
	AutoStart     bool   `json:"autoStart,omitempty"` // If true, start the workspace server immediately
}

// UpdateEngineWorkspaceRequest represents a request to update workspace ports.
type UpdateEngineWorkspaceRequest struct {
	HTTPPort int `json:"httpPort,omitempty"`
	GRPCPort int `json:"grpcPort,omitempty"`
	MQTTPort int `json:"mqttPort,omitempty"`
}

// EngineConfigResponse represents the configuration for an engine.
type EngineConfigResponse struct {
	EngineID   string                       `json:"engineId"`
	Workspaces []EngineWorkspaceConfigEntry `json:"workspaces"`
}

// EngineWorkspaceConfigEntry represents a workspace config entry in the engine config response.
type EngineWorkspaceConfigEntry struct {
	WorkspaceID   string `json:"workspaceId"`
	WorkspaceName string `json:"workspaceName"`
	HTTPPort      int    `json:"httpPort"`
	GRPCPort      int    `json:"grpcPort,omitempty"`
	MQTTPort      int    `json:"mqttPort,omitempty"`
}

// handleGenerateRegistrationToken handles POST /admin/tokens/registration.
func (a *API) handleGenerateRegistrationToken(w http.ResponseWriter, r *http.Request) {
	token, err := a.GenerateRegistrationToken()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "token_generation_failed", "Failed to generate registration token: "+err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, GenerateTokenResponse{
		Token: token,
	})
}

// handleListRegistrationTokens handles GET /admin/tokens/registration.
func (a *API) handleListRegistrationTokens(w http.ResponseWriter, r *http.Request) {
	tokens := a.ListRegistrationTokens()
	writeJSON(w, http.StatusOK, TokenListResponse{
		Tokens: tokens,
		Total:  len(tokens),
	})
}

// LocalEngineID is the well-known ID for the local engine when running in co-located mode.
const LocalEngineID = "local"

// handleListEngines handles GET /engines.
func (a *API) handleListEngines(w http.ResponseWriter, r *http.Request) {
	engines := a.engineRegistry.List()

	// Include local engine if configured
	if a.localEngine != nil {
		localEngine := a.buildLocalEngineEntry(r.Context())
		if localEngine != nil {
			// Prepend local engine to the list
			engines = append([]*store.Engine{localEngine}, engines...)
		}
	}

	writeJSON(w, http.StatusOK, EngineListResponse{
		Engines: engines,
		Total:   len(engines),
	})
}

// buildLocalEngineEntry creates a store.Engine representation of the local engine.
// Returns nil if the local engine status cannot be retrieved.
func (a *API) buildLocalEngineEntry(ctx context.Context) *store.Engine {
	if a.localEngine == nil {
		return nil
	}

	// Query the local engine's status
	status, err := a.localEngine.Status(ctx)
	if err != nil {
		a.log.Warn("failed to get local engine status", "error", err)
		// Return a basic entry even if status query fails
		return &store.Engine{
			ID:           LocalEngineID,
			Name:         "Local Engine",
			Host:         "localhost",
			Status:       store.EngineStatusOffline,
			RegisteredAt: a.startTime,
			LastSeen:     time.Now(),
			Workspaces:   []store.EngineWorkspace{},
		}
	}

	// Build the engine entry from status
	engine := &store.Engine{
		ID:           LocalEngineID,
		Name:         status.Name,
		Host:         "localhost",
		Status:       store.EngineStatusOnline,
		RegisteredAt: status.StartedAt,
		LastSeen:     time.Now(),
		Workspaces:   []store.EngineWorkspace{},
	}

	// Use ID from status if available, otherwise keep "local"
	if status.ID != "" {
		engine.ID = status.ID
	}
	if engine.Name == "" {
		engine.Name = "Local Engine"
	}

	// Extract port from HTTP protocol if available
	if httpProto, ok := status.Protocols["http"]; ok {
		engine.Port = httpProto.Port
	}

	return engine
}

// handleRegisterEngine handles POST /engines/register.
func (a *API) handleRegisterEngine(w http.ResponseWriter, r *http.Request) {
	// Check if localhost bypass is allowed AND request is from localhost,
	// or if API key auth is disabled (trusted network / dev mode).
	localhostBypass := a.allowLocalhostBypass && isLocalhost(r)
	authDisabled := !a.apiKeyConfig.Enabled

	if !localhostBypass && !authDisabled {
		token := getBearerToken(r)
		if token == "" {
			writeError(w, http.StatusUnauthorized, "missing_token", "Authorization token required")
			return
		}
		if !a.ValidateRegistrationToken(token) {
			writeError(w, http.StatusUnauthorized, "invalid_token", "Invalid or expired registration token")
			return
		}
	}

	var req RegisterEngineRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "Failed to parse request body: "+err.Error())
		return
	}

	// Validate required fields
	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "missing_name", "Engine name is required")
		return
	}
	if req.Host == "" {
		writeError(w, http.StatusBadRequest, "missing_host", "Engine host is required")
		return
	}
	if req.Port <= 0 {
		writeError(w, http.StatusBadRequest, "invalid_port", "Engine port must be positive")
		return
	}

	// Generate a new engine ID
	id := id.ULID()

	// Generate an engine-specific token for subsequent calls
	engineToken, err := a.generateEngineToken(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "token_generation_failed", "Failed to generate engine token: "+err.Error())
		return
	}

	engine := &store.Engine{
		ID:           id,
		Name:         req.Name,
		Host:         req.Host,
		Port:         req.Port,
		Version:      req.Version,
		Fingerprint:  req.Fingerprint,
		Status:       store.EngineStatusOnline,
		RegisteredAt: time.Now(),
		LastSeen:     time.Now(),
		Token:        engineToken,
	}

	if err := a.engineRegistry.Register(engine); err != nil {
		a.removeEngineToken(id) // Clean up the token on failure
		writeError(w, http.StatusInternalServerError, "registration_failed", "Failed to register engine: "+err.Error())
		return
	}

	// Auto-set localEngine for the first registered engine.
	// This allows all existing handler code that uses a.localEngine to work
	// when engines register via HTTP (e.g. from `mockd up`).
	if a.localEngine == nil {
		engineURL := fmt.Sprintf("http://%s:%d", req.Host, req.Port)
		a.localEngine = engineclient.New(engineURL)
		a.log.Info("auto-set localEngine from registration", "engineId", id, "url", engineURL)

		// Push any persisted stateful resources to the newly connected engine.
		// Mocks are handled separately (via BulkCreate from the CLI or re-import),
		// but stateful resources need to be restored here so they survive restarts.
		go a.syncPersistedStatefulResources()
	}

	writeJSON(w, http.StatusCreated, RegisterEngineResponse{
		ID:             id,
		Token:          engineToken,
		ConfigEndpoint: "/engines/" + id + "/config",
	})
}

// handleGetEngine handles GET /engines/{id}.
func (a *API) handleGetEngine(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "missing_id", "Engine ID is required")
		return
	}

	engine, err := a.engineRegistry.Get(id)
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", "Engine not found")
		return
	}

	writeJSON(w, http.StatusOK, engine)
}

// handleUnregisterEngine handles DELETE /engines/{id}.
func (a *API) handleUnregisterEngine(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "missing_id", "Engine ID is required")
		return
	}

	// Check if localhost bypass is allowed AND request is from localhost,
	// or if API key auth is disabled (trusted network / dev mode).
	localhostBypass := a.allowLocalhostBypass && isLocalhost(r)
	authDisabled := !a.apiKeyConfig.Enabled

	if !localhostBypass && !authDisabled {
		token := getBearerToken(r)
		if token == "" {
			writeError(w, http.StatusUnauthorized, "missing_token", "Authorization token required")
			return
		}
		if !a.ValidateEngineToken(id, token) {
			writeError(w, http.StatusUnauthorized, "invalid_token", "Invalid engine token")
			return
		}
	}

	if err := a.engineRegistry.Unregister(id); err != nil {
		writeError(w, http.StatusNotFound, "not_found", "Engine not found")
		return
	}

	// Clean up the engine token
	a.removeEngineToken(id)

	w.WriteHeader(http.StatusNoContent)
}

// handleEngineHeartbeat handles POST /engines/{id}/heartbeat.
func (a *API) handleEngineHeartbeat(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "missing_id", "Engine ID is required")
		return
	}

	// Check if localhost bypass is allowed AND request is from localhost,
	// or if API key auth is disabled (trusted network / dev mode).
	localhostBypass := a.allowLocalhostBypass && isLocalhost(r)
	authDisabled := !a.apiKeyConfig.Enabled

	if !localhostBypass && !authDisabled {
		token := getBearerToken(r)
		if token == "" {
			writeError(w, http.StatusUnauthorized, "missing_token", "Authorization token required")
			return
		}
		if !a.ValidateEngineToken(id, token) {
			writeError(w, http.StatusUnauthorized, "invalid_token", "Invalid engine token")
			return
		}
	}

	// Optionally parse status from request body
	var req HeartbeatRequest
	if r.ContentLength > 0 {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json", "Failed to parse request body: "+err.Error())
			return
		}
	}

	// Update heartbeat
	if err := a.engineRegistry.Heartbeat(id); err != nil {
		writeError(w, http.StatusNotFound, "not_found", "Engine not found")
		return
	}

	// If status was provided, update it
	if req.Status != "" {
		if err := a.engineRegistry.UpdateStatus(id, req.Status); err != nil {
			writeError(w, http.StatusInternalServerError, "update_failed", "Failed to update status: "+err.Error())
			return
		}
	}

	// Return updated engine
	engine, err := a.engineRegistry.Get(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "get_failed", "Failed to get engine: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, engine)
}

// handleAssignWorkspace handles PUT /engines/{id}/workspace.
func (a *API) handleAssignWorkspace(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "missing_id", "Engine ID is required")
		return
	}

	// Check if localhost bypass is allowed AND request is from localhost,
	// or if API key auth is disabled (trusted network / dev mode).
	localhostBypass := a.allowLocalhostBypass && isLocalhost(r)
	authDisabled := !a.apiKeyConfig.Enabled

	if !localhostBypass && !authDisabled {
		token := getBearerToken(r)
		if token == "" {
			writeError(w, http.StatusUnauthorized, "missing_token", "Authorization token required")
			return
		}
		if !a.ValidateEngineToken(id, token) {
			writeError(w, http.StatusUnauthorized, "invalid_token", "Invalid engine token")
			return
		}
	}

	var req AssignWorkspaceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "Failed to parse request body: "+err.Error())
		return
	}

	if err := a.engineRegistry.AssignWorkspace(id, req.WorkspaceID); err != nil {
		writeError(w, http.StatusNotFound, "not_found", "Engine not found")
		return
	}

	// Return updated engine
	engine, err := a.engineRegistry.Get(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "get_failed", "Failed to get engine: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, engine)
}

// handleAddEngineWorkspace handles POST /engines/{id}/workspaces.
func (a *API) handleAddEngineWorkspace(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "missing_id", "Engine ID is required")
		return
	}

	var req AddEngineWorkspaceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "Failed to parse request body: "+err.Error())
		return
	}

	if req.WorkspaceID == "" {
		writeError(w, http.StatusBadRequest, "missing_workspace_id", "Workspace ID is required")
		return
	}

	// Check for port conflicts with mocks in other workspaces on this engine
	conflicts := a.checkWorkspaceEnginePortConflicts(r.Context(), id, req.WorkspaceID)
	if len(conflicts) > 0 {
		writeJSON(w, http.StatusConflict, map[string]interface{}{
			"error":     "port_conflict",
			"message":   "Workspace has mocks with ports that conflict with existing workspaces on this engine",
			"conflicts": conflicts,
		})
		return
	}

	// Get workspace name from store if not provided
	workspaceName := req.WorkspaceName
	if workspaceName == "" {
		ws, err := a.workspaceStore.Get(r.Context(), req.WorkspaceID)
		if err == nil && ws != nil {
			workspaceName = ws.Name
		} else {
			workspaceName = req.WorkspaceID // Fallback to ID
		}
	}

	ws, err := a.engineRegistry.AddWorkspaceToEngine(id, req.WorkspaceID, workspaceName, req.HTTPPort, req.GRPCPort, req.MQTTPort)
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", "Engine not found")
		return
	}

	// Auto-start the workspace server if requested
	if req.AutoStart && a.workspaceManager != nil {
		// Set up mock fetcher
		a.workspaceManager.SetMockFetcher(a.fetchMocksForWorkspace)

		// Start the workspace server
		if startErr := a.workspaceManager.StartWorkspace(r.Context(), ws); startErr != nil {
			// Log the error but don't fail the request - workspace is registered but not started
			a.log.Warn("workspace registered but failed to start", "workspaceId", ws.WorkspaceID, "error", startErr)
			ws.Status = "error"
		} else {
			ws.Status = "running"
			// Update status in registry
			_ = a.engineRegistry.UpdateWorkspaceStatus(id, ws.WorkspaceID, "running")
		}
	}

	writeJSON(w, http.StatusCreated, ws)
}

// handleRemoveEngineWorkspace handles DELETE /engines/{id}/workspaces/{workspaceId}.
func (a *API) handleRemoveEngineWorkspace(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	workspaceID := r.PathValue("workspaceId")

	if id == "" {
		writeError(w, http.StatusBadRequest, "missing_id", "Engine ID is required")
		return
	}
	if workspaceID == "" {
		writeError(w, http.StatusBadRequest, "missing_workspace_id", "Workspace ID is required")
		return
	}

	// Stop the workspace server if running
	if a.workspaceManager != nil {
		_ = a.workspaceManager.RemoveWorkspace(workspaceID)
	}

	if err := a.engineRegistry.RemoveWorkspaceFromEngine(id, workspaceID); err != nil {
		writeError(w, http.StatusNotFound, "not_found", "Engine or workspace not found")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// handleUpdateEngineWorkspace handles PUT /engines/{id}/workspaces/{workspaceId}.
func (a *API) handleUpdateEngineWorkspace(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	workspaceID := r.PathValue("workspaceId")

	if id == "" {
		writeError(w, http.StatusBadRequest, "missing_id", "Engine ID is required")
		return
	}
	if workspaceID == "" {
		writeError(w, http.StatusBadRequest, "missing_workspace_id", "Workspace ID is required")
		return
	}

	var req UpdateEngineWorkspaceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "Failed to parse request body: "+err.Error())
		return
	}

	ws, err := a.engineRegistry.UpdateWorkspaceInEngine(id, workspaceID, req.HTTPPort, req.GRPCPort, req.MQTTPort)
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", "Engine or workspace not found")
		return
	}

	writeJSON(w, http.StatusOK, ws)
}

// handleSyncEngineWorkspace handles POST /engines/{id}/workspaces/{workspaceId}/sync.
func (a *API) handleSyncEngineWorkspace(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	workspaceID := r.PathValue("workspaceId")

	if id == "" {
		writeError(w, http.StatusBadRequest, "missing_id", "Engine ID is required")
		return
	}
	if workspaceID == "" {
		writeError(w, http.StatusBadRequest, "missing_workspace_id", "Workspace ID is required")
		return
	}

	ws, err := a.engineRegistry.SyncWorkspaceInEngine(id, workspaceID)
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", "Engine or workspace not found")
		return
	}

	writeJSON(w, http.StatusOK, ws)
}

// handleGetEngineConfig handles GET /engines/{id}/config.
func (a *API) handleGetEngineConfig(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "missing_id", "Engine ID is required")
		return
	}

	engine, err := a.engineRegistry.Get(id)
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", "Engine not found")
		return
	}

	// Build the config response with workspace assignments
	workspaces := make([]EngineWorkspaceConfigEntry, 0, len(engine.Workspaces))
	for _, ws := range engine.Workspaces {
		workspaces = append(workspaces, EngineWorkspaceConfigEntry{
			WorkspaceID:   ws.WorkspaceID,
			WorkspaceName: ws.WorkspaceName,
			HTTPPort:      ws.HTTPPort,
			GRPCPort:      ws.GRPCPort,
			MQTTPort:      ws.MQTTPort,
		})
	}

	response := EngineConfigResponse{
		EngineID:   engine.ID,
		Workspaces: workspaces,
	}

	writeJSON(w, http.StatusOK, response)
}

// syncPersistedStatefulResources pushes stateful resources from the file store
// to the engine. This runs asynchronously after the first engine registers so
// that resources imported in a previous admin session are restored.
func (a *API) syncPersistedStatefulResources() {
	if a.dataStore == nil || a.localEngine == nil {
		return
	}

	ctx, cancel := context.WithTimeout(a.ctx, 15*time.Second)
	defer cancel()

	resources, err := a.dataStore.StatefulResources().List(ctx)
	if err != nil {
		a.log.Warn("failed to load persisted stateful resources", "error", err)
		return
	}
	if len(resources) == 0 {
		return
	}

	a.log.Info("restoring persisted stateful resources to engine", "count", len(resources))

	// Build a minimal collection with only stateful resources (no mocks).
	collection := &config.MockCollection{
		Version:           "1.0",
		Name:              "persisted-stateful-resources",
		StatefulResources: resources,
	}

	if err := a.localEngine.ImportConfig(ctx, collection, false); err != nil {
		a.log.Warn("failed to sync persisted stateful resources to engine", "error", err)
	}
}
