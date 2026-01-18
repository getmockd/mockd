package storage

import (
	"sort"
	"sync"

	"github.com/getmockd/mockd/pkg/mock"
)

// InMemoryMockStore is a thread-safe in-memory implementation of MockStore.
type InMemoryMockStore struct {
	mu    sync.RWMutex
	mocks map[string]*mock.Mock
}

// NewInMemoryMockStore creates a new InMemoryMockStore.
func NewInMemoryMockStore() *InMemoryMockStore {
	return &InMemoryMockStore{
		mocks: make(map[string]*mock.Mock),
	}
}

// Get retrieves a mock by ID. Returns nil if not found.
func (s *InMemoryMockStore) Get(id string) *mock.Mock {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.mocks[id]
}

// Set stores or updates a mock configuration.
func (s *InMemoryMockStore) Set(m *mock.Mock) error {
	if m == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.mocks[m.ID] = m
	return nil
}

// Delete removes a mock by ID. Returns true if deleted, false if not found.
func (s *InMemoryMockStore) Delete(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.mocks[id]; exists {
		delete(s.mocks, id)
		return true
	}
	return false
}

// List returns all stored mocks, sorted by priority (descending) then by creation time.
func (s *InMemoryMockStore) List() []*mock.Mock {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*mock.Mock, 0, len(s.mocks))
	for _, m := range s.mocks {
		result = append(result, m)
	}

	// Sort by priority (descending for HTTP mocks) then by creation time (ascending)
	sort.Slice(result, func(i, j int) bool {
		pi, pj := 0, 0
		if result[i].HTTP != nil {
			pi = result[i].HTTP.Priority
		}
		if result[j].HTTP != nil {
			pj = result[j].HTTP.Priority
		}
		if pi != pj {
			return pi > pj
		}
		return result[i].CreatedAt.Before(result[j].CreatedAt)
	})

	return result
}

// ListByType returns all mocks of a specific type.
func (s *InMemoryMockStore) ListByType(mockType mock.MockType) []*mock.Mock {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*mock.Mock
	for _, m := range s.mocks {
		if m.Type == mockType {
			result = append(result, m)
		}
	}
	return result
}

// Count returns the number of stored mocks.
func (s *InMemoryMockStore) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.mocks)
}

// Clear removes all stored mocks.
func (s *InMemoryMockStore) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.mocks = make(map[string]*mock.Mock)
}

// Exists checks if a mock with the given ID exists.
func (s *InMemoryMockStore) Exists(id string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, exists := s.mocks[id]
	return exists
}

// Ensure InMemoryMockStore implements MockStore.
var _ MockStore = (*InMemoryMockStore)(nil)
