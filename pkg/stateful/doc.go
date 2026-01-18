// Package stateful provides in-memory stateful CRUD resource management for the mock server.
//
// The stateful package enables the mock server to simulate realistic API behavior
// where data persists across requests within a session. It supports:
//
//   - Full CRUD operations (Create, Read, Update, Delete)
//   - Auto-generated UUIDs and timestamps
//   - Seed data initialization from configuration
//   - State reset for test isolation
//   - Query filtering and pagination
//   - Nested resource relationships
//
// Core Types:
//
//   - StateStore: Global container for all stateful resources
//   - StatefulResource: A named collection that maintains state (e.g., "users", "products")
//   - ResourceItem: A single record within a stateful resource
//
// Thread Safety:
//
// All operations are thread-safe using sync.RWMutex at both the store and resource level.
// Read operations can proceed concurrently, while write operations are serialized per resource.
//
// Usage:
//
//	store := stateful.NewStateStore()
//	config := &stateful.ResourceConfig{
//	    Name:     "users",
//	    BasePath: "/api/users",
//	    SeedData: []map[string]interface{}{
//	        {"id": "1", "name": "Alice"},
//	    },
//	}
//	store.Register(config)
//
//	// CRUD operations
//	item, err := store.Get("users").Create(data)
//	item, err := store.Get("users").Get("1")
//	items := store.Get("users").List(nil)
//	item, err := store.Get("users").Update("1", data)
//	err := store.Get("users").Delete("1")
//
//	// State management
//	store.Reset("") // Reset all resources
//	store.Reset("users") // Reset specific resource
package stateful
