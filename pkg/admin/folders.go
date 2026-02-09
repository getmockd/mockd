// Package admin provides a REST API for managing mock configurations.
package admin

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/getmockd/mockd/pkg/config"
	"github.com/getmockd/mockd/pkg/store"
)

// CreateFolderRequest is the request body for creating a folder.
type CreateFolderRequest struct {
	Name        string  `json:"name"`
	ParentID    string  `json:"parentId,omitempty"`
	MetaSortKey float64 `json:"metaSortKey,omitempty"`
	Description string  `json:"description,omitempty"`
	WorkspaceID string  `json:"workspaceId,omitempty"`
}

// UpdateFolderRequest is the request body for updating a folder.
type UpdateFolderRequest struct {
	Name        *string  `json:"name,omitempty"`
	ParentID    *string  `json:"parentId,omitempty"`
	MetaSortKey *float64 `json:"metaSortKey,omitempty"`
	Description *string  `json:"description,omitempty"`
}

// getFolderStore returns the folder store to use.
func (a *API) getFolderStore() store.FolderStore {
	if a.dataStore == nil {
		return nil
	}
	return a.dataStore.Folders()
}

// handleListFolders returns all folders, optionally filtered by workspace.
func (a *API) handleListFolders(w http.ResponseWriter, r *http.Request) {
	folderStore := a.getFolderStore()
	if folderStore == nil {
		writeError(w, http.StatusNotImplemented, "not_implemented", "Folder management requires persistent storage - coming soon")
		return
	}

	// Build filter from query params
	var filter *store.FolderFilter
	query := r.URL.Query()
	workspaceID := query.Get("workspaceId")
	if workspaceID != "" {
		filter = &store.FolderFilter{WorkspaceID: workspaceID}
	}

	folders, err := folderStore.List(r.Context(), filter)
	if err != nil {
		a.log.Error("failed to list folders", "error", err)
		writeError(w, http.StatusInternalServerError, "store_error", ErrMsgInternalError)
		return
	}
	writeJSON(w, http.StatusOK, folders)
}

// handleGetFolder returns a single folder by ID.
func (a *API) handleGetFolder(w http.ResponseWriter, r *http.Request) {
	folderStore := a.getFolderStore()
	if folderStore == nil {
		writeError(w, http.StatusNotImplemented, "not_implemented", "Folder management requires persistent storage - coming soon")
		return
	}

	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "missing_id", "folder ID is required")
		return
	}

	folder, err := folderStore.Get(r.Context(), id)
	if err != nil {
		if err == store.ErrNotFound {
			writeError(w, http.StatusNotFound, "not_found", "folder not found")
			return
		}
		a.log.Error("failed to get folder", "id", id, "error", err)
		writeError(w, http.StatusInternalServerError, "store_error", ErrMsgInternalError)
		return
	}

	writeJSON(w, http.StatusOK, folder)
}

// handleCreateFolder creates a new folder.
func (a *API) handleCreateFolder(w http.ResponseWriter, r *http.Request) {
	folderStore := a.getFolderStore()
	if folderStore == nil {
		writeError(w, http.StatusNotImplemented, "not_implemented", "Folder management requires persistent storage - coming soon")
		return
	}

	var req CreateFolderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "invalid request body")
		return
	}

	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "missing_name", "folder name is required")
		return
	}

	ctx := r.Context()

	// Determine workspaceId first: use request body, then query param, then default
	workspaceID := req.WorkspaceID
	if workspaceID == "" {
		workspaceID = r.URL.Query().Get("workspaceId")
	}
	if workspaceID == "" {
		workspaceID = store.DefaultWorkspaceID
	}

	// Validate parent exists and is in the same workspace if specified
	if req.ParentID != "" {
		parent, err := folderStore.Get(ctx, req.ParentID)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_parent", "parent folder not found")
			return
		}
		// Validate parent is in the same workspace
		parentWsID := parent.WorkspaceID
		if parentWsID == "" {
			parentWsID = store.DefaultWorkspaceID
		}
		if parentWsID != workspaceID {
			writeError(w, http.StatusBadRequest, "invalid_parent", "parent folder must be in the same workspace")
			return
		}
	}

	now := time.Now()
	id, err := generateFolderID()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "id_generation_failed", "failed to generate folder ID")
		return
	}

	// Default metaSortKey to negative timestamp if not provided
	metaSortKey := req.MetaSortKey
	if metaSortKey == 0 {
		metaSortKey = -float64(now.UnixMilli())
	}

	folder := &config.Folder{
		ID:          id,
		Name:        req.Name,
		Description: req.Description,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	folder.ParentID = req.ParentID
	folder.MetaSortKey = metaSortKey
	folder.WorkspaceID = workspaceID

	if err := folderStore.Create(ctx, folder); err != nil {
		a.log.Error("failed to create folder", "error", err)
		writeError(w, http.StatusInternalServerError, "store_error", ErrMsgInternalError)
		return
	}
	writeJSON(w, http.StatusCreated, folder)
}

// handleUpdateFolder updates an existing folder.
func (a *API) handleUpdateFolder(w http.ResponseWriter, r *http.Request) {
	folderStore := a.getFolderStore()
	if folderStore == nil {
		writeError(w, http.StatusNotImplemented, "not_implemented", "Folder management requires persistent storage - coming soon")
		return
	}

	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "missing_id", "folder ID is required")
		return
	}

	ctx := r.Context()
	existing, err := folderStore.Get(ctx, id)
	if err != nil {
		if err == store.ErrNotFound {
			writeError(w, http.StatusNotFound, "not_found", "folder not found")
			return
		}
		a.log.Error("failed to get folder for update", "id", id, "error", err)
		writeError(w, http.StatusInternalServerError, "store_error", ErrMsgInternalError)
		return
	}

	var req UpdateFolderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "invalid request body")
		return
	}

	// Validate parent if changing
	if req.ParentID != nil && *req.ParentID != "" {
		// Can't be own parent
		if *req.ParentID == id {
			writeError(w, http.StatusBadRequest, "invalid_parent", "folder cannot be its own parent")
			return
		}
		// Can't move to a descendant - check using folder store
		if isDescendant(ctx, folderStore, *req.ParentID, id) {
			writeError(w, http.StatusBadRequest, "invalid_parent", "cannot move folder to its own descendant")
			return
		}
		// Parent must exist
		if _, err := folderStore.Get(ctx, *req.ParentID); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_parent", "parent folder not found")
			return
		}
	}

	// Apply updates
	if req.Name != nil {
		existing.Name = *req.Name
	}
	if req.ParentID != nil {
		existing.ParentID = *req.ParentID
	}
	if req.MetaSortKey != nil {
		existing.MetaSortKey = *req.MetaSortKey
	}
	if req.Description != nil {
		existing.Description = *req.Description
	}
	existing.UpdatedAt = time.Now()

	if err := folderStore.Update(ctx, existing); err != nil {
		a.log.Error("failed to update folder", "id", id, "error", err)
		writeError(w, http.StatusInternalServerError, "store_error", ErrMsgInternalError)
		return
	}
	writeJSON(w, http.StatusOK, existing)
}

// isDescendant checks if targetID is a descendant of ancestorID using the folder store.
func isDescendant(ctx context.Context, folderStore store.FolderStore, targetID, ancestorID string) bool {
	current := targetID
	visited := make(map[string]bool)

	for current != "" {
		if visited[current] {
			return false // Cycle detected
		}
		visited[current] = true

		if current == ancestorID {
			return true
		}

		f, err := folderStore.Get(ctx, current)
		if err != nil {
			break
		}
		current = f.ParentID
	}
	return false
}

// handleDeleteFolder deletes a folder.
func (a *API) handleDeleteFolder(w http.ResponseWriter, r *http.Request) {
	folderStore := a.getFolderStore()
	if folderStore == nil {
		writeError(w, http.StatusNotImplemented, "not_implemented", "Folder management requires persistent storage - coming soon")
		return
	}

	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "missing_id", "folder ID is required")
		return
	}

	ctx := r.Context()
	if err := folderStore.Delete(ctx, id); err != nil {
		if err == store.ErrNotFound {
			writeError(w, http.StatusNotFound, "not_found", "folder not found")
			return
		}
		a.log.Error("failed to delete folder", "id", id, "error", err)
		writeError(w, http.StatusInternalServerError, "store_error", ErrMsgInternalError)
		return
	}

	// Move orphaned children to root
	folders, _ := folderStore.List(ctx, nil)
	for _, f := range folders {
		if f.ParentID == id {
			f.ParentID = ""
			f.UpdatedAt = time.Now()
			_ = folderStore.Update(ctx, f)
		}
	}

	w.WriteHeader(http.StatusNoContent)
}

// generateFolderID generates a unique folder ID.
func generateFolderID() (string, error) {
	hex, err := generateRandomHex(16)
	if err != nil {
		return "", fmt.Errorf("failed to generate folder ID: %w", err)
	}
	return "fld_" + hex, nil
}
