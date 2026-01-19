// Package store provides a file-based workspace metadata store.
package store

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// WorkspaceFileStore is a file-based implementation of WorkspaceStore.
// It stores workspace metadata in a JSON file and manages workspace directories.
type WorkspaceFileStore struct {
	mu         sync.RWMutex
	dataDir    string // Base data directory (e.g., ~/.local/share/mockd)
	filePath   string // Path to workspaces.json
	workspaces []*Workspace
	loaded     bool
}

// NewWorkspaceFileStore creates a new file-based workspace store.
// If dataDir is empty, it uses DefaultDataDir().
func NewWorkspaceFileStore(dataDir string) *WorkspaceFileStore {
	if dataDir == "" {
		dataDir = DefaultDataDir()
	}
	return &WorkspaceFileStore{
		dataDir:  dataDir,
		filePath: filepath.Join(dataDir, "workspaces.json"),
	}
}

// Open initializes the store by loading data from disk.
// Creates the data directory and default workspace if they don't exist.
func (s *WorkspaceFileStore) Open(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Ensure data directory exists
	if err := os.MkdirAll(s.dataDir, 0700); err != nil {
		return err
	}

	// Try to load existing data
	data, err := os.ReadFile(s.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			// No file yet, start with empty list
			s.workspaces = nil
			s.loaded = true
			s.ensureDefaultWorkspaceLocked()
			return s.saveLocked()
		}
		return err
	}

	// Parse workspaces
	var stored struct {
		Workspaces []*Workspace `json:"workspaces"`
	}
	if err := json.Unmarshal(data, &stored); err != nil {
		return err
	}

	s.workspaces = stored.Workspaces
	s.loaded = true
	s.ensureDefaultWorkspaceLocked()
	return nil
}

// Close is a no-op for the file store.
func (s *WorkspaceFileStore) Close() error {
	return nil
}

// List returns all workspaces.
func (s *WorkspaceFileStore) List(ctx context.Context) ([]*Workspace, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if !s.loaded {
		return nil, ErrNotFound
	}

	s.ensureDefaultWorkspaceLocked()
	result := make([]*Workspace, len(s.workspaces))
	copy(result, s.workspaces)
	return result, nil
}

// Get returns a workspace by ID.
func (s *WorkspaceFileStore) Get(ctx context.Context, id string) (*Workspace, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if !s.loaded {
		return nil, ErrNotFound
	}

	for _, ws := range s.workspaces {
		if ws.ID == id {
			return ws, nil
		}
	}

	// Check for default workspace
	if id == DefaultWorkspaceID {
		s.ensureDefaultWorkspaceLocked()
		for _, ws := range s.workspaces {
			if ws.ID == id {
				return ws, nil
			}
		}
	}

	return nil, ErrNotFound
}

// Create creates a new workspace.
// For local workspaces, it also creates the workspace directory.
func (s *WorkspaceFileStore) Create(ctx context.Context, workspace *Workspace) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Check for duplicate ID
	for _, ws := range s.workspaces {
		if ws.ID == workspace.ID {
			return ErrAlreadyExists
		}
	}

	// Set timestamps if not set
	now := time.Now().Unix()
	if workspace.CreatedAt == 0 {
		workspace.CreatedAt = now
	}
	workspace.UpdatedAt = now

	// For local workspaces, create the directory
	if workspace.Type == WorkspaceTypeLocal {
		// Set default path if not provided
		if workspace.Path == "" {
			workspace.Path = filepath.Join(s.dataDir, "workspaces", workspace.ID)
		}

		// Create the workspace directory
		if err := os.MkdirAll(workspace.Path, 0700); err != nil {
			return err
		}
	}

	s.workspaces = append(s.workspaces, workspace)
	return s.saveLocked()
}

// Update updates an existing workspace.
func (s *WorkspaceFileStore) Update(ctx context.Context, workspace *Workspace) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i, ws := range s.workspaces {
		if ws.ID == workspace.ID {
			workspace.UpdatedAt = time.Now().Unix()
			s.workspaces[i] = workspace
			return s.saveLocked()
		}
	}

	return ErrNotFound
}

// Delete deletes a workspace by ID.
// Cannot delete the default workspace.
func (s *WorkspaceFileStore) Delete(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Cannot delete the default workspace
	if id == DefaultWorkspaceID {
		return ErrReadOnly
	}

	for i, ws := range s.workspaces {
		if ws.ID == id {
			s.workspaces = append(s.workspaces[:i], s.workspaces[i+1:]...)
			return s.saveLocked()
		}
	}

	return ErrNotFound
}

// DataDir returns the base data directory.
func (s *WorkspaceFileStore) DataDir() string {
	return s.dataDir
}

// ensureDefaultWorkspaceLocked ensures the default local workspace exists.
// Must be called with s.mu held (at least RLock).
func (s *WorkspaceFileStore) ensureDefaultWorkspaceLocked() {
	for _, ws := range s.workspaces {
		if ws.ID == DefaultWorkspaceID {
			return
		}
	}

	// Create default workspace
	now := time.Now().Unix()
	defaultPath := filepath.Join(s.dataDir, "workspaces", DefaultWorkspaceID)
	defaultWS := &Workspace{
		ID:          DefaultWorkspaceID,
		Name:        "Default",
		Type:        WorkspaceTypeLocal,
		Description: "Default workspace",
		Path:        defaultPath,
		SyncStatus:  SyncStatusLocal,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	// Prepend so default is first
	s.workspaces = append([]*Workspace{defaultWS}, s.workspaces...)

	// Create the directory (ignore errors here, will be created on first use)
	_ = os.MkdirAll(defaultPath, 0700)
}

// saveLocked saves the workspaces to disk.
// Must be called with s.mu held (write lock).
func (s *WorkspaceFileStore) saveLocked() error {
	stored := struct {
		Workspaces []*Workspace `json:"workspaces"`
	}{
		Workspaces: s.workspaces,
	}

	data, err := json.MarshalIndent(stored, "", "  ")
	if err != nil {
		return err
	}

	// Atomic write: write to temp file, then rename
	tmpFile := s.filePath + ".tmp"
	if err := os.WriteFile(tmpFile, data, 0600); err != nil {
		return err
	}

	if err := os.Rename(tmpFile, s.filePath); err != nil {
		os.Remove(tmpFile)
		return err
	}

	return nil
}

// Ensure WorkspaceFileStore implements WorkspaceStore
var _ WorkspaceStore = (*WorkspaceFileStore)(nil)
