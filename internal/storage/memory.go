package storage

import (
	"sort"
	"sync"

	"github.com/getmockd/mockd/pkg/config"
)

// InMemoryMockStore is a thread-safe in-memory implementation of MockStore.
type InMemoryMockStore struct {
	mu    sync.RWMutex
	mocks map[string]*config.MockConfiguration
}

// NewInMemoryMockStore creates a new InMemoryMockStore.
func NewInMemoryMockStore() *InMemoryMockStore {
	return &InMemoryMockStore{
		mocks: make(map[string]*config.MockConfiguration),
	}
}

// Get retrieves a mock by ID. Returns nil if not found.
func (s *InMemoryMockStore) Get(id string) *config.MockConfiguration {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.mocks[id]
}

// Set stores or updates a mock configuration.
func (s *InMemoryMockStore) Set(mock *config.MockConfiguration) error {
	if mock == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.mocks[mock.ID] = mock
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
func (s *InMemoryMockStore) List() []*config.MockConfiguration {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*config.MockConfiguration, 0, len(s.mocks))
	for _, mock := range s.mocks {
		result = append(result, mock)
	}

	// Sort by priority (descending) then by creation time (ascending)
	sort.Slice(result, func(i, j int) bool {
		if result[i].Priority != result[j].Priority {
			return result[i].Priority > result[j].Priority
		}
		return result[i].CreatedAt.Before(result[j].CreatedAt)
	})

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
	s.mocks = make(map[string]*config.MockConfiguration)
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
