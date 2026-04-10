//nolint:dupl // Structural similarity with stateful_resource_store.go is intentional; different types.
package file

import (
	"context"

	"github.com/getmockd/mockd/pkg/config"
	"github.com/getmockd/mockd/pkg/store"
)

// customOperationStore implements store.CustomOperationStore for file-based storage.
type customOperationStore struct {
	fs *FileStore
}

// List returns all persisted custom operation configs across every workspace.
// Each entry's Workspace field identifies its bucket.
func (s *customOperationStore) List(ctx context.Context) ([]*config.CustomOperationConfig, error) {
	s.fs.mu.RLock()
	defer s.fs.mu.RUnlock()

	if s.fs.data.CustomOperations == nil {
		return []*config.CustomOperationConfig{}, nil
	}

	result := make([]*config.CustomOperationConfig, len(s.fs.data.CustomOperations))
	copy(result, s.fs.data.CustomOperations)
	return result, nil
}

// Create persists a new custom operation config. Identity is (workspace, name);
// two workspaces may each register an operation with the same name.
func (s *customOperationStore) Create(ctx context.Context, op *config.CustomOperationConfig) error {
	s.fs.mu.Lock()
	defer s.fs.mu.Unlock()

	if s.fs.cfg.ReadOnly {
		return store.ErrReadOnly
	}

	// Check for duplicate (workspace, name).
	for _, existing := range s.fs.data.CustomOperations {
		if existing.Workspace == op.Workspace && existing.Name == op.Name {
			return store.ErrAlreadyExists
		}
	}

	s.fs.data.CustomOperations = append(s.fs.data.CustomOperations, op)
	s.fs.markDirty()
	return nil
}

// Delete removes a custom operation config from the given workspace by name.
func (s *customOperationStore) Delete(ctx context.Context, workspaceID, name string) error {
	s.fs.mu.Lock()
	defer s.fs.mu.Unlock()

	if s.fs.cfg.ReadOnly {
		return store.ErrReadOnly
	}

	for i, op := range s.fs.data.CustomOperations {
		if op.Workspace == workspaceID && op.Name == name {
			s.fs.data.CustomOperations = append(s.fs.data.CustomOperations[:i], s.fs.data.CustomOperations[i+1:]...)
			s.fs.markDirty()
			return nil
		}
	}
	return store.ErrNotFound
}

// DeleteAll removes every custom operation config in the given workspace.
// Operations in other workspaces are left untouched.
func (s *customOperationStore) DeleteAll(ctx context.Context, workspaceID string) error {
	s.fs.mu.Lock()
	defer s.fs.mu.Unlock()

	if s.fs.cfg.ReadOnly {
		return store.ErrReadOnly
	}

	kept := s.fs.data.CustomOperations[:0]
	removed := false
	for _, op := range s.fs.data.CustomOperations {
		if op.Workspace == workspaceID {
			removed = true
			continue
		}
		kept = append(kept, op)
	}
	s.fs.data.CustomOperations = kept
	if removed {
		s.fs.markDirty()
	}
	return nil
}
