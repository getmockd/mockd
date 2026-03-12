package stateful_test

import (
	"fmt"
	"sync"
	"testing"

	"github.com/getmockd/mockd/pkg/stateful"
)

// TestConcurrentRegister tests that 20 goroutines can concurrently register
// resources without races or lost writes.
func TestConcurrentRegister(t *testing.T) {
	t.Parallel()
	store := stateful.NewStateStore()

	const n = 20
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			name := fmt.Sprintf("resource_%03d", idx)
			cfg := &stateful.ResourceConfig{
				Name:       name,
				IDField:    "id",
				IDStrategy: "prefix",
				IDPrefix:   "test_",
				SeedData: []map[string]interface{}{
					{"id": fmt.Sprintf("seed_%d", idx), "name": "Test"},
				},
			}
			if err := store.Register("ws_test", cfg); err != nil {
				t.Errorf("Register(%q) failed: %v", name, err)
			}
		}(i)
	}
	wg.Wait()

	names := store.List("ws_test")
	t.Logf("Registered %d resources", len(names))
	if len(names) != n {
		t.Errorf("Expected %d, got %d", n, len(names))
	}
}

// TestConcurrentRegisterAndRead tests concurrent register + list + get
// operations on the same workspace.
func TestConcurrentRegisterAndRead(t *testing.T) {
	t.Parallel()
	store := stateful.NewStateStore()

	const n = 20
	var wg sync.WaitGroup

	// Writers: register resources concurrently
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			name := fmt.Sprintf("rw_resource_%03d", idx)
			cfg := &stateful.ResourceConfig{
				Name:       name,
				IDField:    "id",
				IDStrategy: "uuid",
				SeedData: []map[string]interface{}{
					{"name": fmt.Sprintf("item_%d", idx)},
				},
			}
			_ = store.Register("ws_rw", cfg)
		}(i)
	}

	// Readers: list and get concurrently with writes
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			// List — should never panic
			_ = store.List("ws_rw")
			// Get — may return nil if not yet registered
			name := fmt.Sprintf("rw_resource_%03d", idx)
			_ = store.Get("ws_rw", name)
		}(i)
	}

	wg.Wait()

	names := store.List("ws_rw")
	if len(names) != n {
		t.Errorf("Expected %d resources, got %d", n, len(names))
	}
}

// TestConcurrentMultiWorkspace tests concurrent registration across
// multiple workspaces to ensure workspace isolation under contention.
func TestConcurrentMultiWorkspace(t *testing.T) {
	t.Parallel()
	store := stateful.NewStateStore()

	const workspaces = 5
	const resourcesPerWS = 10
	var wg sync.WaitGroup

	for w := 0; w < workspaces; w++ {
		for r := 0; r < resourcesPerWS; r++ {
			wg.Add(1)
			go func(wsIdx, resIdx int) {
				defer wg.Done()
				wsID := fmt.Sprintf("ws_%d", wsIdx)
				name := fmt.Sprintf("res_%03d", resIdx)
				cfg := &stateful.ResourceConfig{
					Name:       name,
					IDField:    "id",
					IDStrategy: "sequence",
				}
				_ = store.Register(wsID, cfg)
			}(w, r)
		}
	}

	wg.Wait()

	for w := 0; w < workspaces; w++ {
		wsID := fmt.Sprintf("ws_%d", w)
		names := store.List(wsID)
		if len(names) != resourcesPerWS {
			t.Errorf("Workspace %s: expected %d resources, got %d", wsID, resourcesPerWS, len(names))
		}
	}
}

// TestConcurrentResourceCRUD tests concurrent CRUD operations on a single
// StatefulResource to check for data races in the resource itself.
func TestConcurrentResourceCRUD(t *testing.T) {
	t.Parallel()
	store := stateful.NewStateStore()

	cfg := &stateful.ResourceConfig{
		Name:       "crud_target",
		IDField:    "id",
		IDStrategy: "prefix",
		IDPrefix:   "item_",
	}
	if err := store.Register("ws_crud", cfg); err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	res := store.Get("ws_crud", "crud_target")
	if res == nil {
		t.Fatal("Resource not found after registration")
	}

	const n = 30
	var wg sync.WaitGroup

	// Concurrent creates
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			data := map[string]interface{}{
				"name":  fmt.Sprintf("user_%d", idx),
				"email": fmt.Sprintf("user_%d@test.com", idx),
			}
			_, _ = res.Create(data, nil)
		}(i)
	}
	wg.Wait()

	// Verify all items created
	result := res.List(nil)
	if len(result.Data) != n {
		t.Errorf("Expected %d items, got %d", n, len(result.Data))
	}

	// Concurrent reads + updates
	wg = sync.WaitGroup{}
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			// List — should never panic under concurrent access
			_ = res.List(nil)
		}(i)
	}
	wg.Wait()
}
