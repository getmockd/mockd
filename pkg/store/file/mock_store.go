package file

import (
	"context"
	"strings"
	"time"

	"github.com/getmockd/mockd/pkg/mock"
	"github.com/getmockd/mockd/pkg/store"
)

// mockStore implements store.MockStore for file-based storage.
type mockStore struct {
	fs *FileStore
}

// List returns all mocks matching the filter.
func (s *mockStore) List(ctx context.Context, filter *store.MockFilter) ([]*mock.Mock, error) {
	s.fs.mu.RLock()
	defer s.fs.mu.RUnlock()

	if s.fs.data.Mocks == nil {
		return []*mock.Mock{}, nil
	}

	// If no filter, return all
	if filter == nil {
		result := make([]*mock.Mock, len(s.fs.data.Mocks))
		copy(result, s.fs.data.Mocks)
		return result, nil
	}

	// Apply filters
	var result []*mock.Mock
	for _, m := range s.fs.data.Mocks {
		if !s.matchesFilter(m, filter) {
			continue
		}
		result = append(result, m)
	}

	return result, nil
}

// matchesFilter checks if a mock matches the given filter.
func (s *mockStore) matchesFilter(m *mock.Mock, filter *store.MockFilter) bool {
	// Filter by workspace
	if filter.WorkspaceID != "" && m.WorkspaceID != filter.WorkspaceID {
		return false
	}

	// Filter by type
	if filter.Type != "" && m.Type != filter.Type {
		return false
	}

	// Filter by parent folder
	if filter.ParentID != nil {
		if *filter.ParentID == "" {
			// Root level only
			if m.ParentID != "" {
				return false
			}
		} else if m.ParentID != *filter.ParentID {
			return false
		}
	}

	// Filter by enabled state
	if filter.Enabled != nil {
		mEnabled := m.Enabled == nil || *m.Enabled
		if mEnabled != *filter.Enabled {
			return false
		}
	}

	// Filter by search query
	if filter.Search != "" {
		query := strings.ToLower(filter.Search)
		nameMatch := strings.Contains(strings.ToLower(m.Name), query)
		pathMatch := strings.Contains(strings.ToLower(m.GetPath()), query)
		if !nameMatch && !pathMatch {
			return false
		}
	}

	return true
}

// Get returns a single mock by ID.
func (s *mockStore) Get(ctx context.Context, id string) (*mock.Mock, error) {
	s.fs.mu.RLock()
	defer s.fs.mu.RUnlock()

	for _, m := range s.fs.data.Mocks {
		if m.ID == id {
			return m, nil
		}
	}
	return nil, store.ErrNotFound
}

// Create creates a new mock.
func (s *mockStore) Create(ctx context.Context, m *mock.Mock) error {
	s.fs.mu.Lock()
	defer s.fs.mu.Unlock()

	if s.fs.cfg.ReadOnly {
		return store.ErrReadOnly
	}

	// Check for duplicate ID
	for _, existing := range s.fs.data.Mocks {
		if existing.ID == m.ID {
			return store.ErrAlreadyExists
		}
	}

	// Set timestamps
	now := time.Now()
	if m.CreatedAt.IsZero() {
		m.CreatedAt = now
	}
	m.UpdatedAt = now

	// Set default metaSortKey if not set
	if m.MetaSortKey == 0 {
		m.MetaSortKey = float64(-now.UnixMilli())
	}

	s.fs.data.Mocks = append(s.fs.data.Mocks, m)
	s.fs.markDirty()
	s.fs.notify("mocks", "create", m.ID, m)
	return nil
}

// Update updates an existing mock.
func (s *mockStore) Update(ctx context.Context, m *mock.Mock) error {
	s.fs.mu.Lock()
	defer s.fs.mu.Unlock()

	if s.fs.cfg.ReadOnly {
		return store.ErrReadOnly
	}

	for i, existing := range s.fs.data.Mocks {
		if existing.ID == m.ID {
			m.UpdatedAt = time.Now()
			// Preserve createdAt if not set
			if m.CreatedAt.IsZero() {
				m.CreatedAt = existing.CreatedAt
			}
			s.fs.data.Mocks[i] = m
			s.fs.markDirty()
			s.fs.notify("mocks", "update", m.ID, m)
			return nil
		}
	}
	return store.ErrNotFound
}

// Delete deletes a mock by ID.
func (s *mockStore) Delete(ctx context.Context, id string) error {
	s.fs.mu.Lock()
	defer s.fs.mu.Unlock()

	if s.fs.cfg.ReadOnly {
		return store.ErrReadOnly
	}

	for i, m := range s.fs.data.Mocks {
		if m.ID == id {
			s.fs.data.Mocks = append(s.fs.data.Mocks[:i], s.fs.data.Mocks[i+1:]...)
			s.fs.markDirty()
			s.fs.notify("mocks", "delete", id, nil)
			return nil
		}
	}
	return store.ErrNotFound
}

// DeleteByType deletes all mocks of a specific type.
func (s *mockStore) DeleteByType(ctx context.Context, mockType mock.Type) error {
	s.fs.mu.Lock()
	defer s.fs.mu.Unlock()

	if s.fs.cfg.ReadOnly {
		return store.ErrReadOnly
	}

	var remaining []*mock.Mock
	var deletedIDs []string
	for _, m := range s.fs.data.Mocks {
		if m.Type == mockType {
			deletedIDs = append(deletedIDs, m.ID)
		} else {
			remaining = append(remaining, m)
		}
	}

	if len(deletedIDs) > 0 {
		s.fs.data.Mocks = remaining
		s.fs.markDirty()
		for _, id := range deletedIDs {
			s.fs.notify("mocks", "delete", id, nil)
		}
	}
	return nil
}

// DeleteAll deletes all mocks.
func (s *mockStore) DeleteAll(ctx context.Context) error {
	s.fs.mu.Lock()
	defer s.fs.mu.Unlock()

	if s.fs.cfg.ReadOnly {
		return store.ErrReadOnly
	}

	s.fs.data.Mocks = nil
	s.fs.markDirty()
	s.fs.notify("mocks", "deleteAll", "", nil)
	return nil
}

// Count returns the total number of mocks, optionally filtered by type.
func (s *mockStore) Count(ctx context.Context, mockType mock.Type) (int, error) {
	s.fs.mu.RLock()
	defer s.fs.mu.RUnlock()

	if mockType == "" {
		return len(s.fs.data.Mocks), nil
	}

	count := 0
	for _, m := range s.fs.data.Mocks {
		if m.Type == mockType {
			count++
		}
	}
	return count, nil
}

// BulkCreate creates multiple mocks in a single operation.
// The operation is atomic: either all mocks are created or none are.
func (s *mockStore) BulkCreate(ctx context.Context, mocks []*mock.Mock) error {
	s.fs.mu.Lock()
	defer s.fs.mu.Unlock()

	if s.fs.cfg.ReadOnly {
		return store.ErrReadOnly
	}

	// Phase 1: Validate all IDs before inserting any (atomic check)
	existingIDs := make(map[string]bool)
	for _, m := range s.fs.data.Mocks {
		existingIDs[m.ID] = true
	}
	for _, m := range mocks {
		if existingIDs[m.ID] {
			return store.ErrAlreadyExists
		}
		existingIDs[m.ID] = true // detect intra-batch duplicates
	}

	// Phase 2: All IDs valid — insert all mocks
	now := time.Now()
	for _, m := range mocks {
		if m.CreatedAt.IsZero() {
			m.CreatedAt = now
		}
		m.UpdatedAt = now
		if m.MetaSortKey == 0 {
			m.MetaSortKey = float64(-now.UnixMilli())
		}

		s.fs.data.Mocks = append(s.fs.data.Mocks, m)
	}

	s.fs.markDirty()
	for _, m := range mocks {
		s.fs.notify("mocks", "create", m.ID, m)
	}
	return nil
}

// BulkUpdate updates multiple mocks in a single operation.
// The operation is atomic: either all mocks are updated or none are.
func (s *mockStore) BulkUpdate(ctx context.Context, mocks []*mock.Mock) error {
	s.fs.mu.Lock()
	defer s.fs.mu.Unlock()

	if s.fs.cfg.ReadOnly {
		return store.ErrReadOnly
	}

	mockIndex := make(map[string]int)
	for i, m := range s.fs.data.Mocks {
		mockIndex[m.ID] = i
	}

	// Phase 1: Validate all mocks exist before mutating any
	for _, m := range mocks {
		if _, exists := mockIndex[m.ID]; !exists {
			return store.ErrNotFound
		}
	}

	// Phase 2: All mocks exist — apply updates
	now := time.Now()
	for _, m := range mocks {
		idx := mockIndex[m.ID]
		if m.CreatedAt.IsZero() {
			m.CreatedAt = s.fs.data.Mocks[idx].CreatedAt
		}
		m.UpdatedAt = now
		s.fs.data.Mocks[idx] = m
	}

	s.fs.markDirty()
	for _, m := range mocks {
		s.fs.notify("mocks", "update", m.ID, m)
	}
	return nil
}
