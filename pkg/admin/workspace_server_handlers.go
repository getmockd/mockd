package admin

import (
	"context"
	"net/http"

	"github.com/getmockd/mockd/pkg/config"
	"github.com/getmockd/mockd/pkg/store"
	"github.com/getmockd/mockd/pkg/workspace"
)

// WorkspaceServerStatusResponse represents the status of a workspace server.
type WorkspaceServerStatusResponse struct {
	WorkspaceID   string                 `json:"workspaceId"`
	WorkspaceName string                 `json:"workspaceName"`
	HTTPPort      int                    `json:"httpPort"`
	GRPCPort      int                    `json:"grpcPort,omitempty"`
	MQTTPort      int                    `json:"mqttPort,omitempty"`
	Status        workspace.ServerStatus `json:"status"`
	StatusMessage string                 `json:"statusMessage,omitempty"`
	MockCount     int                    `json:"mockCount"`
	RequestCount  int                    `json:"requestCount"`
	Uptime        int                    `json:"uptime"`
}

// WorkspaceServerListResponse represents a list of workspace servers.
type WorkspaceServerListResponse struct {
	Servers []*WorkspaceServerStatusResponse `json:"servers"`
	Total   int                              `json:"total"`
}

// handleStartWorkspaceServer handles POST /engines/{id}/workspaces/{workspaceId}/start
func (a *API) handleStartWorkspaceServer(w http.ResponseWriter, r *http.Request) {
	engineID := r.PathValue("id")
	workspaceID := r.PathValue("workspaceId")

	if engineID == "" {
		writeError(w, http.StatusBadRequest, "missing_id", "Engine ID is required")
		return
	}
	if workspaceID == "" {
		writeError(w, http.StatusBadRequest, "missing_workspace_id", "Workspace ID is required")
		return
	}

	// Get the engine
	eng, err := a.engineRegistry.Get(engineID)
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", "Engine not found")
		return
	}

	// Find the workspace in the engine
	var ws *store.EngineWorkspace
	for i := range eng.Workspaces {
		if eng.Workspaces[i].WorkspaceID == workspaceID {
			ws = &eng.Workspaces[i]
			break
		}
	}

	if ws == nil {
		writeError(w, http.StatusNotFound, "not_found", "Workspace not assigned to this engine")
		return
	}

	if a.workspaceManager == nil {
		writeError(w, http.StatusServiceUnavailable, "not_configured", "Workspace manager not configured")
		return
	}

	// Set up mock fetcher if not already configured
	a.workspaceManager.SetMockFetcher(a.fetchMocksForWorkspace)

	// Start the workspace server
	if err := a.workspaceManager.StartWorkspace(r.Context(), ws); err != nil {
		a.logger().Error("failed to start workspace server", "error", err, "workspaceID", workspaceID, "engineID", engineID)
		writeError(w, http.StatusInternalServerError, "start_failed", "Failed to start workspace server")
		return
	}

	// Update workspace status in registry
	_ = a.engineRegistry.UpdateWorkspaceStatus(engineID, workspaceID, "running")

	// Get status
	status := a.workspaceManager.GetWorkspaceStatus(workspaceID)
	if status == nil {
		writeJSON(w, http.StatusOK, map[string]string{"status": "started"})
		return
	}

	writeJSON(w, http.StatusOK, &WorkspaceServerStatusResponse{
		WorkspaceID:   status.WorkspaceID,
		WorkspaceName: status.WorkspaceName,
		HTTPPort:      status.HTTPPort,
		GRPCPort:      status.GRPCPort,
		MQTTPort:      status.MQTTPort,
		Status:        status.Status,
		StatusMessage: status.StatusMessage,
		MockCount:     status.MockCount,
		RequestCount:  status.RequestCount,
		Uptime:        status.Uptime,
	})
}

// handleStopWorkspaceServer handles POST /engines/{id}/workspaces/{workspaceId}/stop
func (a *API) handleStopWorkspaceServer(w http.ResponseWriter, r *http.Request) {
	engineID := r.PathValue("id")
	workspaceID := r.PathValue("workspaceId")

	if engineID == "" {
		writeError(w, http.StatusBadRequest, "missing_id", "Engine ID is required")
		return
	}
	if workspaceID == "" {
		writeError(w, http.StatusBadRequest, "missing_workspace_id", "Workspace ID is required")
		return
	}

	// Verify engine and workspace exist
	eng, err := a.engineRegistry.Get(engineID)
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", "Engine not found")
		return
	}

	found := false
	for _, ws := range eng.Workspaces {
		if ws.WorkspaceID == workspaceID {
			found = true
			break
		}
	}
	if !found {
		writeError(w, http.StatusNotFound, "not_found", "Workspace not assigned to this engine")
		return
	}

	if a.workspaceManager == nil {
		writeError(w, http.StatusServiceUnavailable, "not_configured", "Workspace manager not configured")
		return
	}

	// Stop the workspace server
	if err := a.workspaceManager.StopWorkspace(workspaceID); err != nil {
		a.logger().Error("failed to stop workspace server", "error", err, "workspaceID", workspaceID, "engineID", engineID)
		writeError(w, http.StatusInternalServerError, "stop_failed", "Failed to stop workspace server")
		return
	}

	// Update workspace status in registry
	_ = a.engineRegistry.UpdateWorkspaceStatus(engineID, workspaceID, "stopped")

	w.WriteHeader(http.StatusNoContent)
}

// handleGetWorkspaceServerStatus handles GET /engines/{id}/workspaces/{workspaceId}/status
func (a *API) handleGetWorkspaceServerStatus(w http.ResponseWriter, r *http.Request) {
	engineID := r.PathValue("id")
	workspaceID := r.PathValue("workspaceId")

	if engineID == "" {
		writeError(w, http.StatusBadRequest, "missing_id", "Engine ID is required")
		return
	}
	if workspaceID == "" {
		writeError(w, http.StatusBadRequest, "missing_workspace_id", "Workspace ID is required")
		return
	}

	// Verify engine exists
	eng, err := a.engineRegistry.Get(engineID)
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", "Engine not found")
		return
	}

	// Find the workspace in the engine
	var registeredWS *store.EngineWorkspace
	for i := range eng.Workspaces {
		if eng.Workspaces[i].WorkspaceID == workspaceID {
			registeredWS = &eng.Workspaces[i]
			break
		}
	}

	if registeredWS == nil {
		writeError(w, http.StatusNotFound, "not_found", "Workspace not assigned to this engine")
		return
	}

	if a.workspaceManager == nil {
		writeJSON(w, http.StatusOK, &WorkspaceServerStatusResponse{
			WorkspaceID:   registeredWS.WorkspaceID,
			WorkspaceName: registeredWS.WorkspaceName,
			HTTPPort:      registeredWS.HTTPPort,
			GRPCPort:      registeredWS.GRPCPort,
			MQTTPort:      registeredWS.MQTTPort,
			Status:        workspace.ServerStatusStopped,
			MockCount:     registeredWS.MockCount,
		})
		return
	}

	// Get runtime status from workspace manager
	status := a.workspaceManager.GetWorkspaceStatus(workspaceID)
	if status == nil {
		// Not running, return info from registry
		writeJSON(w, http.StatusOK, &WorkspaceServerStatusResponse{
			WorkspaceID:   registeredWS.WorkspaceID,
			WorkspaceName: registeredWS.WorkspaceName,
			HTTPPort:      registeredWS.HTTPPort,
			GRPCPort:      registeredWS.GRPCPort,
			MQTTPort:      registeredWS.MQTTPort,
			Status:        workspace.ServerStatusStopped,
			MockCount:     registeredWS.MockCount,
		})
		return
	}

	writeJSON(w, http.StatusOK, &WorkspaceServerStatusResponse{
		WorkspaceID:   status.WorkspaceID,
		WorkspaceName: status.WorkspaceName,
		HTTPPort:      status.HTTPPort,
		GRPCPort:      status.GRPCPort,
		MQTTPort:      status.MQTTPort,
		Status:        status.Status,
		StatusMessage: status.StatusMessage,
		MockCount:     status.MockCount,
		RequestCount:  status.RequestCount,
		Uptime:        status.Uptime,
	})
}

// handleReloadWorkspaceServer handles POST /engines/{id}/workspaces/{workspaceId}/reload
func (a *API) handleReloadWorkspaceServer(w http.ResponseWriter, r *http.Request) {
	engineID := r.PathValue("id")
	workspaceID := r.PathValue("workspaceId")

	if engineID == "" {
		writeError(w, http.StatusBadRequest, "missing_id", "Engine ID is required")
		return
	}
	if workspaceID == "" {
		writeError(w, http.StatusBadRequest, "missing_workspace_id", "Workspace ID is required")
		return
	}

	// Verify engine and workspace exist
	eng, err := a.engineRegistry.Get(engineID)
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", "Engine not found")
		return
	}

	found := false
	for _, ws := range eng.Workspaces {
		if ws.WorkspaceID == workspaceID {
			found = true
			break
		}
	}
	if !found {
		writeError(w, http.StatusNotFound, "not_found", "Workspace not assigned to this engine")
		return
	}

	if a.workspaceManager == nil {
		writeError(w, http.StatusServiceUnavailable, "not_configured", "Workspace manager not configured")
		return
	}

	// Check if workspace server is running
	server := a.workspaceManager.GetWorkspace(workspaceID)
	if server == nil || server.Status() != workspace.ServerStatusRunning {
		writeError(w, http.StatusBadRequest, "not_running", "Workspace server is not running")
		return
	}

	// Reload mocks
	if err := a.workspaceManager.ReloadWorkspace(r.Context(), workspaceID); err != nil {
		a.logger().Error("failed to reload workspace", "error", err, "workspaceID", workspaceID, "engineID", engineID)
		writeError(w, http.StatusInternalServerError, "reload_failed", "Failed to reload workspace")
		return
	}

	// Get updated status
	status := a.workspaceManager.GetWorkspaceStatus(workspaceID)
	if status == nil {
		writeJSON(w, http.StatusOK, map[string]string{"status": "reloaded"})
		return
	}

	writeJSON(w, http.StatusOK, &WorkspaceServerStatusResponse{
		WorkspaceID:   status.WorkspaceID,
		WorkspaceName: status.WorkspaceName,
		HTTPPort:      status.HTTPPort,
		GRPCPort:      status.GRPCPort,
		MQTTPort:      status.MQTTPort,
		Status:        status.Status,
		StatusMessage: status.StatusMessage,
		MockCount:     status.MockCount,
		RequestCount:  status.RequestCount,
		Uptime:        status.Uptime,
	})
}

// fetchMocksForWorkspace is the mock fetcher function used by the workspace manager.
// It fetches mocks from the engine via HTTP client filtered by workspace ID.
func (a *API) fetchMocksForWorkspace(ctx context.Context, workspaceID string) ([]*config.MockConfiguration, error) {
	// Get mocks from the engine via HTTP client
	if a.localEngine == nil {
		return nil, nil
	}

	allMocks, err := a.localEngine.ListMocks(ctx)
	if err != nil {
		return nil, err
	}

	// Filter by workspace ID
	var filtered []*config.MockConfiguration
	for _, mock := range allMocks {
		if mock != nil && mock.WorkspaceID == workspaceID {
			filtered = append(filtered, mock)
		}
	}

	return filtered, nil
}

// ListWorkspaceServers returns all running workspace servers.
func (a *API) ListWorkspaceServers() *WorkspaceServerListResponse {
	if a.workspaceManager == nil {
		return &WorkspaceServerListResponse{
			Servers: []*WorkspaceServerStatusResponse{},
			Total:   0,
		}
	}

	servers := a.workspaceManager.ListWorkspaces()

	response := &WorkspaceServerListResponse{
		Servers: make([]*WorkspaceServerStatusResponse, 0, len(servers)),
		Total:   len(servers),
	}

	for _, server := range servers {
		status := server.StatusInfo()
		response.Servers = append(response.Servers, &WorkspaceServerStatusResponse{
			WorkspaceID:   status.WorkspaceID,
			WorkspaceName: status.WorkspaceName,
			HTTPPort:      status.HTTPPort,
			GRPCPort:      status.GRPCPort,
			MQTTPort:      status.MQTTPort,
			Status:        status.Status,
			StatusMessage: status.StatusMessage,
			MockCount:     status.MockCount,
			RequestCount:  status.RequestCount,
			Uptime:        status.Uptime,
		})
	}

	return response
}
