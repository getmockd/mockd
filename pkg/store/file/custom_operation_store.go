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

// List returns all persisted custom operation configs.
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

// Create persists a new custom operation config.
func (s *customOperationStore) Create(ctx context.Context, op *config.CustomOperationConfig) error {
	s.fs.mu.Lock()
	defer s.fs.mu.Unlock()

	if s.fs.cfg.ReadOnly {
		return store.ErrReadOnly
	}

	// Check for duplicate name
	for _, existing := range s.fs.data.CustomOperations {
		if existing.Name == op.Name {
			return store.ErrAlreadyExists
		}
	}

	s.fs.data.CustomOperations = append(s.fs.data.CustomOperations, op)
	s.fs.markDirty()
	return nil
}

// Delete removes a custom operation config by name.
func (s *customOperationStore) Delete(ctx context.Context, name string) error {
	s.fs.mu.Lock()
	defer s.fs.mu.Unlock()

	if s.fs.cfg.ReadOnly {
		return store.ErrReadOnly
	}

	for i, op := range s.fs.data.CustomOperations {
		if op.Name == name {
			s.fs.data.CustomOperations = append(s.fs.data.CustomOperations[:i], s.fs.data.CustomOperations[i+1:]...)
			s.fs.markDirty()
			return nil
		}
	}
	return store.ErrNotFound
}

// DeleteAll removes all custom operation configs.
func (s *customOperationStore) DeleteAll(ctx context.Context) error {
	s.fs.mu.Lock()
	defer s.fs.mu.Unlock()

	if s.fs.cfg.ReadOnly {
		return store.ErrReadOnly
	}

	s.fs.data.CustomOperations = nil
	s.fs.markDirty()
	return nil
}
