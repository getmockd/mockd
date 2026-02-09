// Package engine provides persistent store adapters.
package engine

import (
	"context"
	"errors"

	"github.com/getmockd/mockd/internal/storage"
	"github.com/getmockd/mockd/pkg/mock"
	"github.com/getmockd/mockd/pkg/store"
)

// PersistentMockStore wraps a store.MockStore to implement storage.MockStore.
// This adapter bridges the persistence layer (pkg/store) with the engine's
// internal storage interface (internal/storage), enabling automatic persistence
// of mock configurations.
type PersistentMockStore struct {
	store store.MockStore
	ctx   context.Context
}

// NewPersistentMockStore creates a new persistent mock store adapter.
func NewPersistentMockStore(s store.MockStore) *PersistentMockStore {
	return &PersistentMockStore{
		store: s,
		ctx:   context.Background(),
	}
}

// Ensure PersistentMockStore implements storage.MockStore
var _ storage.MockStore = (*PersistentMockStore)(nil)

// Get retrieves a mock by ID. Returns nil if not found.
func (p *PersistentMockStore) Get(id string) *mock.Mock {
	m, err := p.store.Get(p.ctx, id)
	if err != nil {
		return nil
	}
	return m
}

// Set stores or updates a mock configuration.
func (p *PersistentMockStore) Set(m *mock.Mock) error {
	// Check if it exists
	existing, err := p.store.Get(p.ctx, m.ID)
	if errors.Is(err, store.ErrNotFound) || existing == nil {
		return p.store.Create(p.ctx, m)
	}
	return p.store.Update(p.ctx, m)
}

// Delete removes a mock by ID. Returns true if deleted, false if not found.
func (p *PersistentMockStore) Delete(id string) bool {
	err := p.store.Delete(p.ctx, id)
	return err == nil
}

// List returns all stored mocks.
func (p *PersistentMockStore) List() []*mock.Mock {
	mocks, err := p.store.List(p.ctx, nil)
	if err != nil {
		return nil
	}
	return mocks
}

// ListByType returns all mocks of a specific type.
func (p *PersistentMockStore) ListByType(mockType mock.Type) []*mock.Mock {
	mocks, err := p.store.List(p.ctx, &store.MockFilter{Type: mockType})
	if err != nil {
		return nil
	}
	return mocks
}

// Count returns the number of stored mocks.
func (p *PersistentMockStore) Count() int {
	count, err := p.store.Count(p.ctx, "")
	if err != nil {
		return 0
	}
	return count
}

// Clear removes all stored mocks.
func (p *PersistentMockStore) Clear() {
	_ = p.store.DeleteAll(p.ctx)
}

// Exists checks if a mock with the given ID exists.
func (p *PersistentMockStore) Exists(id string) bool {
	m, err := p.store.Get(p.ctx, id)
	return err == nil && m != nil
}
