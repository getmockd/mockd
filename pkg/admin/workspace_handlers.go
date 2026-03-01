package admin

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	idgen "github.com/getmockd/mockd/internal/id"
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
	BasePath     string `json:"basePath"`
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
func (a *API) getWorkspaceStore() store.WorkspaceStore {
	return a.workspaceStore
}

// handleListWorkspaces returns all workspaces.
// GET /workspaces
func (a *API) handleListWorkspaces(w http.ResponseWriter, r *http.Request) {
	wsStore := a.getWorkspaceStore()
	ctx := r.Context()
	workspaces, err := wsStore.List(ctx)
	if err != nil {
		a.logger().Error("failed to list workspaces", "error", err)
		writeError(w, http.StatusInternalServerError, "store_error", ErrMsgInternalError)
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
func (a *API) handleGetWorkspace(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "missing_id", "Workspace ID is required")
		return
	}

	wsStore := a.getWorkspaceStore()
	ctx := r.Context()
	ws, err := wsStore.Get(ctx, id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "not_found", "Workspace not found")
			return
		}
		a.logger().Error("failed to get workspace", "error", err, "workspaceID", id)
		writeError(w, http.StatusInternalServerError, "store_error", ErrMsgInternalError)
		return
	}

	writeJSON(w, http.StatusOK, storeWorkspaceToDTO(ws))
}

// handleCreateWorkspace creates a new workspace.
// POST /workspaces
func (a *API) handleCreateWorkspace(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Name        string  `json:"name"`
		Type        *string `json:"type,omitempty"`
		Description string  `json:"description,omitempty"`
		BasePath    *string `json:"basePath,omitempty"`
		Path        string  `json:"path,omitempty"`
		URL         string  `json:"url,omitempty"`
		Branch      string  `json:"branch,omitempty"`
		ReadOnly    bool    `json:"readOnly,omitempty"`
		AutoSync    bool    `json:"autoSync,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeJSONDecodeError(w, err, a.logger())
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

	// Reject duplicate workspace names so repeated creates (e.g. `mockd up`)
	// are idempotent instead of silently creating duplicates.
	wsStore := a.getWorkspaceStore()
	ctx := r.Context()
	existing, err := wsStore.List(ctx)
	if err != nil {
		a.logger().Error("failed to list workspaces for duplicate check", "error", err)
		writeError(w, http.StatusInternalServerError, "store_error", ErrMsgInternalError)
		return
	}
	for _, ws := range existing {
		if ws.Name == input.Name {
			writeJSON(w, http.StatusConflict, ErrorResponse{
				Error:   "already_exists",
				Message: fmt.Sprintf("Workspace with name %q already exists", input.Name),
				Details: map[string]string{"existingId": ws.ID},
			})
			return
		}
	}

	// Generate ID and timestamps
	now := time.Now()
	id := "ws_" + idgen.Short()

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

	// Set BasePath for non-default workspaces
	if ws.ID != store.DefaultWorkspaceID {
		if input.BasePath != nil && *input.BasePath != "" {
			ws.BasePath = validateBasePath(*input.BasePath)
		} else if input.BasePath == nil {
			// Auto-generate from name
			ws.BasePath = "/" + SlugifyWorkspaceName(input.Name)
		}
		// If input.BasePath is explicitly empty string, leave ws.BasePath as ""

		// Validate basePath doesn't overlap with any peer workspace on the same engine.
		if conflict := checkBasePathConflict(ws.BasePath, ws.ID, existing); conflict != nil {
			writeJSON(w, http.StatusConflict, map[string]interface{}{
				"error":    "basepath_conflict",
				"message":  fmt.Sprintf("BasePath %q overlaps with workspace %q (basePath %q): %s", ws.BasePath, conflict.ExistingName, conflict.ExistingBasePath, conflict.Reason),
				"conflict": conflict,
			})
			return
		}
	}

	// For local workspaces, set default path if not provided
	if wsType == store.WorkspaceTypeLocal && ws.Path == "" {
		dataDir := a.dataDir
		if dataDir == "" {
			dataDir = store.DefaultDataDir()
		}
		ws.Path = filepath.Join(dataDir, "workspaces", id)
	}

	// Create workspace directory for local type
	if wsType == store.WorkspaceTypeLocal {
		if err := os.MkdirAll(ws.Path, 0700); err != nil {
			a.logger().Error("failed to create workspace directory", "error", err, "path", ws.Path)
			writeError(w, http.StatusInternalServerError, "filesystem_error", "Failed to create workspace directory")
			return
		}
	}

	// Persist workspace (wsStore and ctx already initialized for duplicate check above)
	if err := wsStore.Create(ctx, ws); err != nil {
		if errors.Is(err, store.ErrAlreadyExists) {
			writeError(w, http.StatusConflict, "already_exists", "Workspace with this ID already exists")
			return
		}
		a.logger().Error("failed to create workspace", "error", err, "workspaceID", ws.ID)
		writeError(w, http.StatusInternalServerError, "store_error", ErrMsgInternalError)
		return
	}

	writeJSON(w, http.StatusCreated, storeWorkspaceToDTO(ws))
}

// handleUpdateWorkspace updates an existing workspace.
// PUT /workspaces/{id}
func (a *API) handleUpdateWorkspace(w http.ResponseWriter, r *http.Request) {
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

	ctx := r.Context()

	// Get existing workspace
	ws, err := wsStore.Get(ctx, id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "not_found", "Workspace not found")
			return
		}
		a.logger().Error("failed to get workspace for update", "error", err, "workspaceID", id)
		writeError(w, http.StatusInternalServerError, "store_error", ErrMsgInternalError)
		return
	}

	var input struct {
		Name        *string `json:"name,omitempty"`
		Type        *string `json:"type,omitempty"`
		Description *string `json:"description,omitempty"`
		BasePath    *string `json:"basePath,omitempty"`
		Path        *string `json:"path,omitempty"`
		URL         *string `json:"url,omitempty"`
		Branch      *string `json:"branch,omitempty"`
		ReadOnly    *bool   `json:"readOnly,omitempty"`
		AutoSync    *bool   `json:"autoSync,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeJSONDecodeError(w, err, a.logger())
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
	if input.BasePath != nil {
		validated := validateBasePath(*input.BasePath)
		// Prevent non-default workspaces from claiming root (empty basePath)
		if validated == "" && id != store.DefaultWorkspaceID {
			writeError(w, http.StatusBadRequest, "validation_error", "basePath cannot be empty for non-default workspaces")
			return
		}
		// Check overlap with peer workspaces before accepting
		peers, listErr := wsStore.List(ctx)
		if listErr != nil {
			a.logger().Error("failed to list workspaces for overlap check", "error", listErr)
			writeError(w, http.StatusInternalServerError, "store_error", ErrMsgInternalError)
			return
		}
		if conflict := checkBasePathConflict(validated, id, peers); conflict != nil {
			writeJSON(w, http.StatusConflict, map[string]interface{}{
				"error":    "basepath_conflict",
				"message":  fmt.Sprintf("BasePath %q overlaps with workspace %q (basePath %q): %s", validated, conflict.ExistingName, conflict.ExistingBasePath, conflict.Reason),
				"conflict": conflict,
			})
			return
		}
		ws.BasePath = validated
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
		a.logger().Error("failed to update workspace", "error", err, "workspaceID", id)
		writeError(w, http.StatusInternalServerError, "store_error", ErrMsgInternalError)
		return
	}

	writeJSON(w, http.StatusOK, storeWorkspaceToDTO(ws))
}

// handleDeleteWorkspace deletes a workspace.
// DELETE /workspaces/{id}
func (a *API) handleDeleteWorkspace(w http.ResponseWriter, r *http.Request) {
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

	ctx := r.Context()

	// Check if workspace exists
	_, err := wsStore.Get(ctx, id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "not_found", "Workspace not found")
			return
		}
		a.logger().Error("failed to get workspace for delete", "error", err, "workspaceID", id)
		writeError(w, http.StatusInternalServerError, "store_error", ErrMsgInternalError)
		return
	}

	// Cascade-delete mocks belonging to this workspace
	if mockStore := a.getMockStore(); mockStore != nil {
		mocks, listErr := mockStore.List(ctx, &store.MockFilter{WorkspaceID: id})
		if listErr == nil {
			for _, m := range mocks {
				_ = mockStore.Delete(ctx, m.ID)
			}
		}
	}

	// Delete from store
	if err := wsStore.Delete(ctx, id); err != nil {
		a.logger().Error("failed to delete workspace", "error", err, "workspaceID", id)
		writeError(w, http.StatusInternalServerError, "store_error", ErrMsgInternalError)
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
		BasePath:    ws.BasePath,
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
