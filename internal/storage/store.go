// Package storage provides mock storage abstractions and implementations.
package storage

import (
	"github.com/getmockd/mockd/pkg/mock"
)

// MockStore defines the interface for storing and retrieving mock configurations.
type MockStore interface {
	// Get retrieves a mock by ID. Returns nil if not found.
	Get(id string) *mock.Mock

	// Set stores or updates a mock configuration.
	Set(m *mock.Mock) error

	// Delete removes a mock by ID. Returns true if deleted, false if not found.
	Delete(id string) bool

	// List returns all stored mocks.
	List() []*mock.Mock

	// ListByType returns all mocks of a specific type.
	ListByType(mockType mock.MockType) []*mock.Mock

	// Count returns the number of stored mocks.
	Count() int

	// Clear removes all stored mocks.
	Clear()

	// Exists checks if a mock with the given ID exists.
	Exists(id string) bool
}
