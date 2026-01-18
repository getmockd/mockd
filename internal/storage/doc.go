// Package storage provides mock storage abstractions and implementations.
//
// It defines the MockStore interface for storing, retrieving, and managing
// mock configurations, along with concrete implementations.
//
// Key types:
//
//   - MockStore: Interface defining the contract for mock storage backends
//   - InMemoryMockStore: Thread-safe in-memory implementation of MockStore
//   - FilteredStore: Wrapper that applies filters to an underlying store
//
// The MockStore interface supports:
//
//   - CRUD operations (Get, Set, Delete)
//   - Listing mocks (all or by type)
//   - Existence checking and counting
//   - Bulk clear operations
//
// The InMemoryMockStore implementation is safe for concurrent access and
// is the default storage backend for the mock server.
package storage
