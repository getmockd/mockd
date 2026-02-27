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
		a.logger().Error("failed to generate registration token", "error", err)
		writeError(w, http.StatusInternalServerError, "token_generation_failed", ErrMsgInternalError)
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
	if a.localEngine.Load() != nil {
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
	client := a.localEngine.Load()
	if client == nil {
		return nil
	}

	// Query the local engine's status
	status, err := client.Status(ctx)
	if err != nil {
		a.logger().Warn("failed to get local engine status", "error", err)
		// Return a basic entry even if status query fails
		registeredAt := a.startTime
		if registeredAt.IsZero() {
			registeredAt = time.Now()
		}
		return &store.Engine{
			ID:           LocalEngineID,
			Name:         "Local Engine",
			Host:         "localhost",
			Status:       store.EngineStatusOffline,
			RegisteredAt: registeredAt,
			LastSeen:     time.Now(),
			Workspaces:   []store.EngineWorkspace{},
		}
	}

	// Build the engine entry from status
	entry := &store.Engine{
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
		entry.ID = status.ID
	}
	if entry.Name == "" {
		entry.Name = "Local Engine"
	}

	// Extract port from HTTP protocol if available
	if httpProto, ok := status.Protocols["http"]; ok {
		entry.Port = httpProto.Port
	}

	return entry
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
		writeJSONDecodeError(w, err, a.logger())
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
	engineID := id.ULID()

	// Generate an engine-specific token for subsequent calls
	engineToken, err := a.generateEngineToken(engineID)
	if err != nil {
		a.logger().Error("failed to generate engine token", "error", err, "engineID", engineID)
		writeError(w, http.StatusInternalServerError, "token_generation_failed", ErrMsgInternalError)
		return
	}

	engine := &store.Engine{
		ID:           engineID,
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
		a.removeEngineToken(engineID) // Clean up the token on failure
		a.logger().Error("failed to register engine", "error", err, "engineID", engineID)
		writeError(w, http.StatusInternalServerError, "registration_failed", ErrMsgInternalError)
		return
	}

	// Auto-set localEngine for the first registered engine.
	// This allows all existing handler code that uses a.localEngine to work
	// when engines register via HTTP (e.g. from `mockd up`).
	engineURL := fmt.Sprintf("http://%s:%d", req.Host, req.Port)
	if a.localEngine.Load() == nil {
		a.localEngine.Store(engineclient.New(engineURL))
		a.logger().Info("auto-set localEngine from registration", "engineId", engineID, "url", engineURL)
	}

	// Sync the admin store (mocks + stateful resources) to the engine.
	// This runs on EVERY registration, not just the first, because:
	// - First registration: engine has no mocks, admin store may have persisted mocks
	// - Re-registration after crash: engine lost in-memory mocks, admin store has them
	go a.syncAdminStoreToEngine(engineclient.New(engineURL), "engine-registration")

	writeJSON(w, http.StatusCreated, RegisterEngineResponse{
		ID:             engineID,
		Token:          engineToken,
		ConfigEndpoint: "/engines/" + engineID + "/config",
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
	if err := decodeOptionalJSONBody(r, &req); err != nil {
		writeJSONDecodeError(w, err, a.logger())
		return
	}

	// Update heartbeat — returns whether the engine was previously offline.
	wasOffline, err := a.engineRegistry.Heartbeat(id)
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", "Engine not found")
		return
	}

	// On offline→online transition, sync the admin store to the engine.
	// This covers mocks created while the engine was unreachable.
	if wasOffline {
		a.logger().Info("engine came back online, syncing admin store", "engineID", id)
		engine, lookupErr := a.engineRegistry.Get(id)
		if lookupErr == nil {
			engineURL := fmt.Sprintf("http://%s:%d", engine.Host, engine.Port)
			go a.syncAdminStoreToEngine(engineclient.New(engineURL), "engine-reconnect")
		}
	}

	// If status was provided, update it
	if req.Status != "" {
		if err := a.engineRegistry.UpdateStatus(id, req.Status); err != nil {
			a.logger().Error("failed to update engine status", "error", err, "engineID", id)
			writeError(w, http.StatusInternalServerError, "update_failed", ErrMsgInternalError)
			return
		}
	}

	// Return updated engine
	engine, err := a.engineRegistry.Get(id)
	if err != nil {
		a.logger().Error("failed to get engine after heartbeat", "error", err, "engineID", id)
		writeError(w, http.StatusInternalServerError, "get_failed", ErrMsgInternalError)
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
		writeJSONDecodeError(w, err, a.logger())
		return
	}
	if req.WorkspaceID == "" {
		writeError(w, http.StatusBadRequest, "missing_workspace_id", "Workspace ID is required")
		return
	}

	if err := a.engineRegistry.AssignWorkspace(id, req.WorkspaceID); err != nil {
		writeError(w, http.StatusNotFound, "not_found", "Engine not found")
		return
	}

	// Return updated engine
	engine, err := a.engineRegistry.Get(id)
	if err != nil {
		a.logger().Error("failed to get engine after workspace assignment", "error", err, "engineID", id)
		writeError(w, http.StatusInternalServerError, "get_failed", ErrMsgInternalError)
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
		writeJSONDecodeError(w, err, a.logger())
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
			a.logger().Warn("workspace registered but failed to start", "workspaceId", ws.WorkspaceID, "error", startErr)
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
		writeJSONDecodeError(w, err, a.logger())
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

// syncAdminStoreToEngine pushes all mocks and stateful resources from the admin
// store to the given engine client. It uses replace=true so the engine's state
// matches the admin store exactly (admin is the single source of truth).
//
// This runs asynchronously after:
//   - Engine registration (first or re-registration after crash)
//   - Heartbeat offline→online transition (engine was unreachable, now back)
//
// Per-engine mutexes ensure concurrent syncs to different engines don't block
// each other, while duplicate syncs to the same engine are skipped.
func (a *API) syncAdminStoreToEngine(client *engineclient.Client, reason string) {
	if a.dataStore == nil || client == nil {
		return
	}

	// Use per-engine mutex so syncs to different engines run concurrently,
	// but duplicate syncs to the same engine are serialized / skipped.
	engineURL := client.BaseURL()
	mu := a.perEngineSync.get(engineURL)
	if !mu.TryLock() {
		a.logger().Debug("skipping engine sync, already in progress",
			"reason", reason, "engine", engineURL)
		return
	}
	defer mu.Unlock()

	ctx, cancel := context.WithTimeout(a.ctx, 30*time.Second)
	defer cancel()

	// Read all mocks from admin store.
	mocks, err := a.dataStore.Mocks().List(ctx, nil)
	if err != nil {
		a.logger().Warn("sync: failed to list mocks from admin store", "error", err, "reason", reason)
		return
	}

	// Read all stateful resources.
	var resources []*config.StatefulResourceConfig
	if a.dataStore.StatefulResources() != nil {
		resources, err = a.dataStore.StatefulResources().List(ctx)
		if err != nil {
			a.logger().Warn("sync: failed to list stateful resources", "error", err, "reason", reason)
			// Continue without resources — mocks are more important.
			resources = nil
		}
	}

	if len(mocks) == 0 && len(resources) == 0 {
		a.logger().Debug("sync: admin store is empty, nothing to push", "reason", reason)
		return
	}

	collection := &config.MockCollection{
		Version:           "1.0",
		Kind:              "MockCollection",
		Name:              "admin-store-sync",
		Mocks:             mocks,
		StatefulResources: resources,
	}

	a.logger().Info("syncing admin store to engine",
		"reason", reason,
		"mocks", len(mocks),
		"statefulResources", len(resources),
	)

	result, err := client.ImportConfig(ctx, collection, true)
	if err != nil {
		a.logger().Warn("sync: failed to push admin store to engine",
			"error", err, "reason", reason,
			"mocks", len(mocks), "statefulResources", len(resources),
		)
		return
	}

	a.logger().Info("sync complete",
		"reason", reason,
		"imported", result.Imported,
		"total", result.Total,
	)
}
