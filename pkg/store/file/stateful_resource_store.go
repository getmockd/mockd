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

// List returns all persisted stateful resource configs.
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

// Create persists a new stateful resource config.
func (s *statefulResourceStore) Create(ctx context.Context, res *config.StatefulResourceConfig) error {
	s.fs.mu.Lock()
	defer s.fs.mu.Unlock()

	if s.fs.cfg.ReadOnly {
		return store.ErrReadOnly
	}

	// Check for duplicate name
	for _, existing := range s.fs.data.StatefulResources {
		if existing.Name == res.Name {
			return store.ErrAlreadyExists
		}
	}

	s.fs.data.StatefulResources = append(s.fs.data.StatefulResources, res)
	s.fs.markDirty()
	return nil
}

// Delete removes a stateful resource config by name.
func (s *statefulResourceStore) Delete(ctx context.Context, name string) error {
	s.fs.mu.Lock()
	defer s.fs.mu.Unlock()

	if s.fs.cfg.ReadOnly {
		return store.ErrReadOnly
	}

	for i, res := range s.fs.data.StatefulResources {
		if res.Name == name {
			s.fs.data.StatefulResources = append(s.fs.data.StatefulResources[:i], s.fs.data.StatefulResources[i+1:]...)
			s.fs.markDirty()
			return nil
		}
	}
	return store.ErrNotFound
}

// DeleteAll removes all stateful resource configs.
func (s *statefulResourceStore) DeleteAll(ctx context.Context) error {
	s.fs.mu.Lock()
	defer s.fs.mu.Unlock()

	if s.fs.cfg.ReadOnly {
		return store.ErrReadOnly
	}

	s.fs.data.StatefulResources = nil
	s.fs.markDirty()
	return nil
}
