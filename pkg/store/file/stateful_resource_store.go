//nolint:dupl // Structural similarity with custom_operation_store.go is intentional; different types.
package file

import (
	"context"

	"github.com/getmockd/mockd/pkg/config"
	"github.com/getmockd/mockd/pkg/store"
)

// statefulResourceStore implements store.StatefulResourceStore for file-based storage.
type statefulResourceStore struct {
	fs *FileStore
}

// List returns all persisted stateful resource configs across every workspace.
// Each entry's Workspace field identifies its bucket.
func (s *statefulResourceStore) List(ctx context.Context) ([]*config.StatefulResourceConfig, error) {
	s.fs.mu.RLock()
	defer s.fs.mu.RUnlock()

	if s.fs.data.StatefulResources == nil {
		return []*config.StatefulResourceConfig{}, nil
	}

	result := make([]*config.StatefulResourceConfig, len(s.fs.data.StatefulResources))
	copy(result, s.fs.data.StatefulResources)
	return result, nil
}

// Create persists a new stateful resource config. Identity is (workspace, name);
// two workspaces may each register a resource with the same name.
func (s *statefulResourceStore) Create(ctx context.Context, res *config.StatefulResourceConfig) error {
	s.fs.mu.Lock()
	defer s.fs.mu.Unlock()

	if s.fs.cfg.ReadOnly {
		return store.ErrReadOnly
	}

	// Check for duplicate (workspace, name).
	for _, existing := range s.fs.data.StatefulResources {
		if existing.Workspace == res.Workspace && existing.Name == res.Name {
			return store.ErrAlreadyExists
		}
	}

	s.fs.data.StatefulResources = append(s.fs.data.StatefulResources, res)
	s.fs.markDirty()
	return nil
}

// Delete removes a stateful resource config from the given workspace by name.
func (s *statefulResourceStore) Delete(ctx context.Context, workspaceID, name string) error {
	s.fs.mu.Lock()
	defer s.fs.mu.Unlock()

	if s.fs.cfg.ReadOnly {
		return store.ErrReadOnly
	}

	for i, res := range s.fs.data.StatefulResources {
		if res.Workspace == workspaceID && res.Name == name {
			s.fs.data.StatefulResources = append(s.fs.data.StatefulResources[:i], s.fs.data.StatefulResources[i+1:]...)
			s.fs.markDirty()
			return nil
		}
	}
	return store.ErrNotFound
}

// DeleteAll removes every stateful resource config in the given workspace.
// Resources in other workspaces are left untouched.
func (s *statefulResourceStore) DeleteAll(ctx context.Context, workspaceID string) error {
	s.fs.mu.Lock()
	defer s.fs.mu.Unlock()

	if s.fs.cfg.ReadOnly {
		return store.ErrReadOnly
	}

	kept := s.fs.data.StatefulResources[:0]
	removed := false
	for _, res := range s.fs.data.StatefulResources {
		if res.Workspace == workspaceID {
			removed = true
			continue
		}
		kept = append(kept, res)
	}
	s.fs.data.StatefulResources = kept
	if removed {
		s.fs.markDirty()
	}
	return nil
}
