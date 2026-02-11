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
	s.ensureDefaultWorkspace()

	s.fs.mu.RLock()
	defer s.fs.mu.RUnlock()

	result := make([]*store.Workspace, len(s.fs.data.Workspaces))
	copy(result, s.fs.data.Workspaces)
	return result, nil
}

func (s *workspaceStore) Get(ctx context.Context, id string) (*store.Workspace, error) {
	s.ensureDefaultWorkspace()

	s.fs.mu.RLock()
	defer s.fs.mu.RUnlock()

	for _, w := range s.fs.data.Workspaces {
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
// Acquires write lock only if the default workspace needs to be created.
func (s *workspaceStore) ensureDefaultWorkspace() {
	// Fast path: check under read lock
	s.fs.mu.RLock()
	for _, w := range s.fs.data.Workspaces {
		if w.ID == store.DefaultWorkspaceID {
			s.fs.mu.RUnlock()
			return
		}
	}
	s.fs.mu.RUnlock()

	// Slow path: acquire write lock and create default workspace
	s.fs.mu.Lock()
	defer s.fs.mu.Unlock()

	// Double-check under write lock (another goroutine may have created it)
	for _, w := range s.fs.data.Workspaces {
		if w.ID == store.DefaultWorkspaceID {
			return
		}
	}

	now := time.Now().Unix()
	defaultWS := &store.Workspace{
		ID:          store.DefaultWorkspaceID,
		Name:        "Default",
		Type:        store.WorkspaceTypeLocal,
		Description: "Default workspace",
		SyncStatus:  store.SyncStatusLocal,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	s.fs.data.Workspaces = append([]*store.Workspace{defaultWS}, s.fs.data.Workspaces...)
}
