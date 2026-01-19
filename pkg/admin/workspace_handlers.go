package admin

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/getmockd/mockd/pkg/store"
)

// ============================================================================
// Workspace CRUD Handlers
// ============================================================================

// WorkspaceDTO represents a workspace for API responses.
type WorkspaceDTO struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Type         string `json:"type"`
	Description  string `json:"description,omitempty"`
	Path         string `json:"path,omitempty"`
	URL          string `json:"url,omitempty"`
	Branch       string `json:"branch,omitempty"`
	ReadOnly     bool   `json:"readOnly,omitempty"`
	SyncStatus   string `json:"syncStatus,omitempty"`
	LastSyncedAt string `json:"lastSyncedAt,omitempty"`
	AutoSync     bool   `json:"autoSync,omitempty"`
	CreatedAt    string `json:"createdAt,omitempty"`
	UpdatedAt    string `json:"updatedAt,omitempty"`
}

// getWorkspaceStore returns the workspace store to use.
// Uses the file-based workspace store.
func (a *AdminAPI) getWorkspaceStore() store.WorkspaceStore {
	return a.workspaceStore
}

// handleListWorkspaces returns all workspaces.
// GET /workspaces
func (a *AdminAPI) handleListWorkspaces(w http.ResponseWriter, r *http.Request) {
	wsStore := a.getWorkspaceStore()
	if wsStore == nil {
		// Fallback to default workspace only
		defaultWS := &WorkspaceDTO{
			ID:        store.DefaultWorkspaceID,
			Name:      "Default",
			Type:      string(store.WorkspaceTypeLocal),
			CreatedAt: time.Now().Format(time.RFC3339),
			UpdatedAt: time.Now().Format(time.RFC3339),
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"workspaces": []*WorkspaceDTO{defaultWS},
			"count":      1,
		})
		return
	}

	ctx := r.Context()
	workspaces, err := wsStore.List(ctx)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "store_error", err.Error())
		return
	}

	// Convert to DTOs
	dtos := make([]*WorkspaceDTO, 0, len(workspaces))
	for _, ws := range workspaces {
		dtos = append(dtos, storeWorkspaceToDTO(ws))
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"workspaces": dtos,
		"count":      len(dtos),
	})
}

// handleGetWorkspace returns a specific workspace.
// GET /workspaces/{id}
func (a *AdminAPI) handleGetWorkspace(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "missing_id", "Workspace ID is required")
		return
	}

	wsStore := a.getWorkspaceStore()
	if wsStore == nil {
		// No store - only "local" workspace exists
		if id == store.DefaultWorkspaceID {
			writeJSON(w, http.StatusOK, &WorkspaceDTO{
				ID:        store.DefaultWorkspaceID,
				Name:      "Default",
				Type:      string(store.WorkspaceTypeLocal),
				CreatedAt: time.Now().Format(time.RFC3339),
				UpdatedAt: time.Now().Format(time.RFC3339),
			})
			return
		}
		writeError(w, http.StatusNotFound, "not_found", "Workspace not found")
		return
	}

	ctx := r.Context()
	ws, err := wsStore.Get(ctx, id)
	if err != nil {
		if err == store.ErrNotFound {
			writeError(w, http.StatusNotFound, "not_found", "Workspace not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "store_error", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, storeWorkspaceToDTO(ws))
}

// handleCreateWorkspace creates a new workspace.
// POST /workspaces
func (a *AdminAPI) handleCreateWorkspace(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Name        string  `json:"name"`
		Type        *string `json:"type,omitempty"`
		Description string  `json:"description,omitempty"`
		Path        string  `json:"path,omitempty"`
		URL         string  `json:"url,omitempty"`
		Branch      string  `json:"branch,omitempty"`
		ReadOnly    bool    `json:"readOnly,omitempty"`
		AutoSync    bool    `json:"autoSync,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}

	// Validate required fields
	if input.Name == "" {
		writeError(w, http.StatusBadRequest, "validation_error", "name is required")
		return
	}

	// Set defaults
	wsType := store.WorkspaceTypeLocal
	if input.Type != nil {
		wsType = store.WorkspaceType(*input.Type)
	}

	// Validate workspace type - only "local" is currently supported
	switch wsType {
	case store.WorkspaceTypeLocal:
		// Supported
	case store.WorkspaceTypeGit, store.WorkspaceTypeCloud:
		writeError(w, http.StatusNotImplemented, "not_supported", fmt.Sprintf("workspace type %q is not yet supported", wsType))
		return
	case store.WorkspaceTypeConfig:
		writeError(w, http.StatusBadRequest, "validation_error", "config workspaces are read-only and cannot be created")
		return
	default:
		writeError(w, http.StatusBadRequest, "validation_error", fmt.Sprintf("invalid type: %s", wsType))
		return
	}

	// Generate ID and timestamps
	now := time.Now()
	id := fmt.Sprintf("ws_%x", now.UnixNano())

	// Create workspace model
	ws := &store.Workspace{
		ID:          id,
		Name:        input.Name,
		Type:        wsType,
		Description: input.Description,
		Path:        input.Path,
		URL:         input.URL,
		Branch:      input.Branch,
		ReadOnly:    input.ReadOnly,
		AutoSync:    input.AutoSync,
		SyncStatus:  store.SyncStatusLocal,
		CreatedAt:   now.Unix(),
		UpdatedAt:   now.Unix(),
	}

	// For local workspaces, set default path if not provided
	if wsType == store.WorkspaceTypeLocal && ws.Path == "" {
		dataDir := store.DefaultDataDir()
		ws.Path = filepath.Join(dataDir, "workspaces", id)
	}

	// Create workspace directory for local type
	if wsType == store.WorkspaceTypeLocal {
		if err := os.MkdirAll(ws.Path, 0700); err != nil {
			writeError(w, http.StatusInternalServerError, "filesystem_error", fmt.Sprintf("failed to create workspace directory: %v", err))
			return
		}
	}

	// Get workspace store
	wsStore := a.getWorkspaceStore()
	if wsStore == nil {
		writeError(w, http.StatusInternalServerError, "store_error", "workspace store not initialized")
		return
	}

	// Persist workspace
	ctx := r.Context()
	if err := wsStore.Create(ctx, ws); err != nil {
		if err == store.ErrAlreadyExists {
			writeError(w, http.StatusConflict, "already_exists", "Workspace with this ID already exists")
			return
		}
		writeError(w, http.StatusInternalServerError, "store_error", err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, storeWorkspaceToDTO(ws))
}

// handleUpdateWorkspace updates an existing workspace.
// PUT /workspaces/{id}
func (a *AdminAPI) handleUpdateWorkspace(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "missing_id", "Workspace ID is required")
		return
	}

	// Cannot modify the default workspace
	if id == store.DefaultWorkspaceID {
		writeError(w, http.StatusBadRequest, "validation_error", "cannot modify the default workspace")
		return
	}

	wsStore := a.getWorkspaceStore()
	if wsStore == nil {
		writeError(w, http.StatusInternalServerError, "store_error", "workspace store not initialized")
		return
	}

	ctx := r.Context()

	// Get existing workspace
	ws, err := wsStore.Get(ctx, id)
	if err != nil {
		if err == store.ErrNotFound {
			writeError(w, http.StatusNotFound, "not_found", "Workspace not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "store_error", err.Error())
		return
	}

	var input struct {
		Name        *string `json:"name,omitempty"`
		Type        *string `json:"type,omitempty"`
		Description *string `json:"description,omitempty"`
		Path        *string `json:"path,omitempty"`
		URL         *string `json:"url,omitempty"`
		Branch      *string `json:"branch,omitempty"`
		ReadOnly    *bool   `json:"readOnly,omitempty"`
		AutoSync    *bool   `json:"autoSync,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}

	// Apply updates
	if input.Name != nil {
		ws.Name = *input.Name
	}
	if input.Type != nil {
		wsType := store.WorkspaceType(*input.Type)
		// Only allow local type for now
		switch wsType {
		case store.WorkspaceTypeLocal:
			ws.Type = wsType
		case store.WorkspaceTypeGit, store.WorkspaceTypeCloud:
			writeError(w, http.StatusNotImplemented, "not_supported", fmt.Sprintf("workspace type %q is not yet supported", wsType))
			return
		default:
			writeError(w, http.StatusBadRequest, "validation_error", fmt.Sprintf("invalid type: %s", wsType))
			return
		}
	}
	if input.Description != nil {
		ws.Description = *input.Description
	}
	if input.Path != nil {
		ws.Path = *input.Path
	}
	if input.URL != nil {
		ws.URL = *input.URL
	}
	if input.Branch != nil {
		ws.Branch = *input.Branch
	}
	if input.ReadOnly != nil {
		ws.ReadOnly = *input.ReadOnly
	}
	if input.AutoSync != nil {
		ws.AutoSync = *input.AutoSync
	}

	ws.UpdatedAt = time.Now().Unix()

	// Update in store
	if err := wsStore.Update(ctx, ws); err != nil {
		writeError(w, http.StatusInternalServerError, "store_error", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, storeWorkspaceToDTO(ws))
}

// handleDeleteWorkspace deletes a workspace.
// DELETE /workspaces/{id}
func (a *AdminAPI) handleDeleteWorkspace(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "missing_id", "Workspace ID is required")
		return
	}

	// Cannot delete the default workspace
	if id == store.DefaultWorkspaceID {
		writeError(w, http.StatusBadRequest, "validation_error", "cannot delete the default workspace")
		return
	}

	wsStore := a.getWorkspaceStore()
	if wsStore == nil {
		writeError(w, http.StatusInternalServerError, "store_error", "workspace store not initialized")
		return
	}

	ctx := r.Context()

	// Check if workspace exists
	_, err := wsStore.Get(ctx, id)
	if err != nil {
		if err == store.ErrNotFound {
			writeError(w, http.StatusNotFound, "not_found", "Workspace not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "store_error", err.Error())
		return
	}

	// Delete from store
	if err := wsStore.Delete(ctx, id); err != nil {
		writeError(w, http.StatusInternalServerError, "store_error", err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// storeWorkspaceToDTO converts a store.Workspace to a DTO.
func storeWorkspaceToDTO(ws *store.Workspace) *WorkspaceDTO {
	dto := &WorkspaceDTO{
		ID:          ws.ID,
		Name:        ws.Name,
		Type:        string(ws.Type),
		Description: ws.Description,
		Path:        ws.Path,
		URL:         ws.URL,
		Branch:      ws.Branch,
		ReadOnly:    ws.ReadOnly,
		AutoSync:    ws.AutoSync,
		CreatedAt:   time.Unix(ws.CreatedAt, 0).Format(time.RFC3339),
		UpdatedAt:   time.Unix(ws.UpdatedAt, 0).Format(time.RFC3339),
	}

	if ws.SyncStatus != "" {
		dto.SyncStatus = string(ws.SyncStatus)
	}
	if ws.LastSyncedAt > 0 {
		dto.LastSyncedAt = time.Unix(ws.LastSyncedAt, 0).Format(time.RFC3339)
	}

	return dto
}
