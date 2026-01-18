// Package storage provides mock storage abstractions and implementations.
package storage

import (
	"github.com/getmockd/mockd/pkg/mock"
)

// FilteredMockStore wraps a MockStore and filters by workspaceID.
// This provides a live view of the underlying store filtered to a specific workspace.
// All reads are filtered, writes automatically set the workspaceID.
type FilteredMockStore struct {
	underlying  MockStore
	workspaceID string
}

// NewFilteredMockStore creates a new filtered store wrapper.
func NewFilteredMockStore(store MockStore, workspaceID string) *FilteredMockStore {
	return &FilteredMockStore{
		underlying:  store,
		workspaceID: workspaceID,
	}
}

// Get retrieves a mock by ID, only if it belongs to this workspace.
func (f *FilteredMockStore) Get(id string) *mock.Mock {
	m := f.underlying.Get(id)
	if m == nil {
		return nil
	}
	// Only return if it belongs to this workspace
	if m.WorkspaceID != f.workspaceID {
		return nil
	}
	return m
}

// Set stores a mock, automatically setting the workspaceID.
func (f *FilteredMockStore) Set(m *mock.Mock) error {
	m.WorkspaceID = f.workspaceID
	return f.underlying.Set(m)
}

// Delete removes a mock by ID, only if it belongs to this workspace.
func (f *FilteredMockStore) Delete(id string) bool {
	// Check it belongs to this workspace first
	m := f.underlying.Get(id)
	if m == nil || m.WorkspaceID != f.workspaceID {
		return false
	}
	return f.underlying.Delete(id)
}

// List returns all mocks belonging to this workspace.
func (f *FilteredMockStore) List() []*mock.Mock {
	all := f.underlying.List()
	filtered := make([]*mock.Mock, 0)
	for _, m := range all {
		if m != nil && m.WorkspaceID == f.workspaceID {
			filtered = append(filtered, m)
		}
	}
	return filtered
}

// ListByType returns all mocks of a specific type belonging to this workspace.
func (f *FilteredMockStore) ListByType(mockType mock.MockType) []*mock.Mock {
	all := f.underlying.ListByType(mockType)
	filtered := make([]*mock.Mock, 0)
	for _, m := range all {
		if m != nil && m.WorkspaceID == f.workspaceID {
			filtered = append(filtered, m)
		}
	}
	return filtered
}

// Count returns the number of mocks in this workspace.
func (f *FilteredMockStore) Count() int {
	return len(f.List())
}

// Clear removes all mocks belonging to this workspace.
func (f *FilteredMockStore) Clear() {
	for _, m := range f.List() {
		f.underlying.Delete(m.ID)
	}
}

// Exists checks if a mock with the given ID exists in this workspace.
func (f *FilteredMockStore) Exists(id string) bool {
	return f.Get(id) != nil
}

// WorkspaceID returns the workspace ID this store is filtered to.
func (f *FilteredMockStore) WorkspaceID() string {
	return f.workspaceID
}

// Underlying returns the underlying unfiltered store.
func (f *FilteredMockStore) Underlying() MockStore {
	return f.underlying
}
