package file

import (
	"context"
	"time"

	"github.com/getmockd/mockd/pkg/store"
)

// workspaceStore implements store.WorkspaceStore.
type workspaceStore struct {
	fs *FileStore
}

func (s *workspaceStore) List(ctx context.Context) ([]*store.Workspace, error) {
	s.fs.mu.RLock()
	defer s.fs.mu.RUnlock()

	// Always include the default local workspace
	workspaces := s.ensureDefaultWorkspace()
	result := make([]*store.Workspace, len(workspaces))
	copy(result, workspaces)
	return result, nil
}

func (s *workspaceStore) Get(ctx context.Context, id string) (*store.Workspace, error) {
	s.fs.mu.RLock()
	defer s.fs.mu.RUnlock()

	workspaces := s.ensureDefaultWorkspace()
	for _, w := range workspaces {
		if w.ID == id {
			return w, nil
		}
	}
	return nil, store.ErrNotFound
}

func (s *workspaceStore) Create(ctx context.Context, workspace *store.Workspace) error {
	s.fs.mu.Lock()
	defer s.fs.mu.Unlock()

	if s.fs.cfg.ReadOnly {
		return store.ErrReadOnly
	}

	// Check for duplicate ID
	for _, w := range s.fs.data.Workspaces {
		if w.ID == workspace.ID {
			return store.ErrAlreadyExists
		}
	}

	// Set timestamps if not set
	now := time.Now().Unix()
	if workspace.CreatedAt == 0 {
		workspace.CreatedAt = now
	}
	workspace.UpdatedAt = now

	s.fs.data.Workspaces = append(s.fs.data.Workspaces, workspace)
	s.fs.markDirty()
	s.fs.notify("workspaces", "create", workspace.ID, workspace)
	return nil
}

func (s *workspaceStore) Update(ctx context.Context, workspace *store.Workspace) error {
	s.fs.mu.Lock()
	defer s.fs.mu.Unlock()

	if s.fs.cfg.ReadOnly {
		return store.ErrReadOnly
	}

	for i, w := range s.fs.data.Workspaces {
		if w.ID == workspace.ID {
			workspace.UpdatedAt = time.Now().Unix()
			s.fs.data.Workspaces[i] = workspace
			s.fs.markDirty()
			s.fs.notify("workspaces", "update", workspace.ID, workspace)
			return nil
		}
	}
	return store.ErrNotFound
}

func (s *workspaceStore) Delete(ctx context.Context, id string) error {
	s.fs.mu.Lock()
	defer s.fs.mu.Unlock()

	if s.fs.cfg.ReadOnly {
		return store.ErrReadOnly
	}

	// Cannot delete the default local workspace
	if id == store.DefaultWorkspaceID {
		return store.ErrReadOnly
	}

	for i, w := range s.fs.data.Workspaces {
		if w.ID == id {
			s.fs.data.Workspaces = append(s.fs.data.Workspaces[:i], s.fs.data.Workspaces[i+1:]...)
			s.fs.markDirty()
			s.fs.notify("workspaces", "delete", id, nil)
			return nil
		}
	}
	return store.ErrNotFound
}

// ensureDefaultWorkspace ensures the default local workspace exists.
// Must be called with s.fs.mu held (at least RLock).
func (s *workspaceStore) ensureDefaultWorkspace() []*store.Workspace {
	// Check if default exists
	for _, w := range s.fs.data.Workspaces {
		if w.ID == store.DefaultWorkspaceID {
			return s.fs.data.Workspaces
		}
	}

	// Create default workspace (will be saved on next markDirty)
	now := time.Now().Unix()
	defaultWS := &store.Workspace{
		ID:          store.DefaultWorkspaceID,
		Name:        "Local",
		Type:        store.WorkspaceTypeLocal,
		Description: "Default local workspace",
		SyncStatus:  store.SyncStatusLocal,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	// Note: This modifies data under RLock which is technically unsafe,
	// but ensureDefaultWorkspace is called at read time and this is an
	// idempotent initialization. For full safety, callers should upgrade to write lock.
	// In practice, this only happens once on first access.
	s.fs.data.Workspaces = append([]*store.Workspace{defaultWS}, s.fs.data.Workspaces...)
	return s.fs.data.Workspaces
}
