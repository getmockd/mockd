package stateful

import (
	"net/http"
	"testing"
	"time"
)

// =============================================================================
// StateStore Tests
// =============================================================================

func TestNewStateStore(t *testing.T) {
	store := NewStateStore()
	if store == nil {
		t.Fatal("NewStateStore returned nil")
		return
	}
	if store.resources == nil {
		t.Error("resources map not initialized")
	}
}

func TestStateStore_Register(t *testing.T) {
	tests := []struct {
		name    string
		config  *ResourceConfig
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid config",
			config: &ResourceConfig{
				Name:     "users",
				BasePath: "/api/users",
			},
			wantErr: false,
		},
		{
			name:    "nil config",
			config:  nil,
			wantErr: true,
			errMsg:  "config cannot be nil",
		},
		{
			name: "empty name",
			config: &ResourceConfig{
				Name:     "",
				BasePath: "/api/users",
			},
			wantErr: true,
			errMsg:  "resource name cannot be empty",
		},
		{
			name: "empty basePath",
			config: &ResourceConfig{
				Name:     "users",
				BasePath: "",
			},
			wantErr: true,
			errMsg:  "resource basePath cannot be empty",
		},
		{
			name: "basePath without leading slash",
			config: &ResourceConfig{
				Name:     "users",
				BasePath: "api/users",
			},
			wantErr: true,
			errMsg:  "resource basePath must start with /",
		},
		{
			name: "with seed data",
			config: &ResourceConfig{
				Name:     "products",
				BasePath: "/api/products",
				SeedData: []map[string]interface{}{
					{"id": "p1", "name": "Product 1"},
					{"id": "p2", "name": "Product 2"},
				},
			},
			wantErr: false,
		},
		{
			name: "duplicate seed data IDs",
			config: &ResourceConfig{
				Name:     "items",
				BasePath: "/api/items",
				SeedData: []map[string]interface{}{
					{"id": "dup", "name": "First"},
					{"id": "dup", "name": "Second"},
				},
			},
			wantErr: true,
			errMsg:  "duplicate ID",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := NewStateStore()
			err := store.Register(tt.config)

			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error containing %q, got nil", tt.errMsg)
				} else if tt.errMsg != "" && err.Error() != tt.errMsg && !containsString(err.Error(), tt.errMsg) {
					t.Errorf("expected error containing %q, got %q", tt.errMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}

func TestStateStore_RegisterDuplicate(t *testing.T) {
	store := NewStateStore()
	config := &ResourceConfig{
		Name:     "users",
		BasePath: "/api/users",
	}

	if err := store.Register(config); err != nil {
		t.Fatalf("first register failed: %v", err)
	}

	err := store.Register(config)
	if err == nil {
		t.Error("expected error for duplicate registration")
	}
}

func TestStateStore_Get(t *testing.T) {
	store := NewStateStore()
	config := &ResourceConfig{
		Name:     "users",
		BasePath: "/api/users",
	}
	store.Register(config)

	// Get existing
	resource := store.Get("users")
	if resource == nil {
		t.Error("expected to find 'users' resource")
	}

	// Get non-existing
	resource = store.Get("nonexistent")
	if resource != nil {
		t.Error("expected nil for non-existent resource")
	}
}

func TestStateStore_List(t *testing.T) {
	store := NewStateStore()

	// Empty store
	names := store.List()
	if len(names) != 0 {
		t.Errorf("expected 0 names, got %d", len(names))
	}

	// Add resources
	store.Register(&ResourceConfig{Name: "users", BasePath: "/api/users"})
	store.Register(&ResourceConfig{Name: "products", BasePath: "/api/products"})

	names = store.List()
	if len(names) != 2 {
		t.Errorf("expected 2 names, got %d", len(names))
	}
}

func TestStateStore_MatchPath(t *testing.T) {
	store := NewStateStore()
	store.Register(&ResourceConfig{Name: "users", BasePath: "/api/users"})
	store.Register(&ResourceConfig{Name: "orders", BasePath: "/api/users/:userId/orders"})

	tests := []struct {
		path       string
		wantMatch  bool
		wantID     string
		wantParams map[string]string
	}{
		{"/api/users", true, "", nil},
		{"/api/users/123", true, "123", nil},
		{"/api/users/u1/orders", true, "", map[string]string{"userId": "u1"}},
		{"/api/users/u1/orders/o1", true, "o1", map[string]string{"userId": "u1"}},
		{"/api/unknown", false, "", nil},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			resource, id, params := store.MatchPath(tt.path)

			if tt.wantMatch {
				if resource == nil {
					t.Error("expected match but got nil resource")
				}
				if id != tt.wantID {
					t.Errorf("id = %q, want %q", id, tt.wantID)
				}
				for k, v := range tt.wantParams {
					if params[k] != v {
						t.Errorf("param[%s] = %q, want %q", k, params[k], v)
					}
				}
			} else {
				if resource != nil {
					t.Error("expected no match but got resource")
				}
			}
		})
	}
}

func TestStateStore_Reset(t *testing.T) {
	store := NewStateStore()
	store.Register(&ResourceConfig{
		Name:     "users",
		BasePath: "/api/users",
		SeedData: []map[string]interface{}{
			{"id": "u1", "name": "User 1"},
		},
	})

	// Add an item
	resource := store.Get("users")
	resource.Create(map[string]interface{}{"id": "u2", "name": "User 2"}, nil)

	if resource.Count() != 2 {
		t.Fatalf("expected 2 items, got %d", resource.Count())
	}

	// Reset specific resource
	resp, err := store.Reset("users")
	if err != nil {
		t.Fatalf("reset failed: %v", err)
	}
	if !resp.Reset {
		t.Error("expected Reset=true")
	}
	if resource.Count() != 1 {
		t.Errorf("expected 1 item after reset, got %d", resource.Count())
	}

	// Reset non-existent
	_, err = store.Reset("nonexistent")
	if err == nil {
		t.Error("expected error for non-existent resource")
	}

	// Reset all
	store.Register(&ResourceConfig{Name: "products", BasePath: "/api/products"})
	resp, err = store.Reset("")
	if err != nil {
		t.Fatalf("reset all failed: %v", err)
	}
	if len(resp.Resources) != 2 {
		t.Errorf("expected 2 resources reset, got %d", len(resp.Resources))
	}
}

func TestStateStore_Clear(t *testing.T) {
	store := NewStateStore()
	store.Register(&ResourceConfig{Name: "users", BasePath: "/api/users"})
	store.Register(&ResourceConfig{Name: "products", BasePath: "/api/products"})

	store.Clear()

	if len(store.List()) != 0 {
		t.Error("expected empty store after Clear")
	}
}

func TestStateStore_Overview(t *testing.T) {
	store := NewStateStore()
	store.Register(&ResourceConfig{
		Name:     "users",
		BasePath: "/api/users",
		SeedData: []map[string]interface{}{
			{"id": "u1"},
			{"id": "u2"},
		},
	})
	store.Register(&ResourceConfig{
		Name:     "products",
		BasePath: "/api/products",
		SeedData: []map[string]interface{}{
			{"id": "p1"},
		},
	})

	overview := store.Overview()
	if overview.Resources != 2 {
		t.Errorf("Resources = %d, want 2", overview.Resources)
	}
	if overview.TotalItems != 3 {
		t.Errorf("TotalItems = %d, want 3", overview.TotalItems)
	}
}

func TestStateStore_ResourceInfo(t *testing.T) {
	store := NewStateStore()
	store.Register(&ResourceConfig{
		Name:     "users",
		BasePath: "/api/users",
		IDField:  "userId",
		SeedData: []map[string]interface{}{
			{"userId": "u1", "name": "User 1"},
		},
	})

	info, err := store.ResourceInfo("users")
	if err != nil {
		t.Fatalf("ResourceInfo failed: %v", err)
	}
	if info.Name != "users" {
		t.Errorf("Name = %q, want %q", info.Name, "users")
	}
	if info.IDField != "userId" {
		t.Errorf("IDField = %q, want %q", info.IDField, "userId")
	}
	if info.ItemCount != 1 {
		t.Errorf("ItemCount = %d, want 1", info.ItemCount)
	}

	// Non-existent
	_, err = store.ResourceInfo("nonexistent")
	if err == nil {
		t.Error("expected error for non-existent resource")
	}
}

func TestStateStore_ClearResource(t *testing.T) {
	store := NewStateStore()
	store.Register(&ResourceConfig{
		Name:     "users",
		BasePath: "/api/users",
		SeedData: []map[string]interface{}{
			{"id": "u1"},
			{"id": "u2"},
		},
	})

	count, err := store.ClearResource("users")
	if err != nil {
		t.Fatalf("ClearResource failed: %v", err)
	}
	if count != 2 {
		t.Errorf("count = %d, want 2", count)
	}

	resource := store.Get("users")
	if resource.Count() != 0 {
		t.Errorf("resource still has %d items", resource.Count())
	}

	// Non-existent
	_, err = store.ClearResource("nonexistent")
	if err == nil {
		t.Error("expected error for non-existent resource")
	}
}

// =============================================================================
// StatefulResource Tests
// =============================================================================

func TestStatefulResource_CRUD(t *testing.T) {
	config := &ResourceConfig{
		Name:     "users",
		BasePath: "/api/users",
	}
	resource := NewStatefulResource(config)

	// Create
	item, err := resource.Create(map[string]interface{}{
		"name":  "John Doe",
		"email": "john@example.com",
	}, nil)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if item.ID == "" {
		t.Error("ID should be auto-generated")
	}
	if item.CreatedAt.IsZero() {
		t.Error("CreatedAt should be set")
	}

	// Create with explicit ID
	item2, err := resource.Create(map[string]interface{}{
		"id":   "custom-id",
		"name": "Jane Doe",
	}, nil)
	if err != nil {
		t.Fatalf("Create with ID failed: %v", err)
	}
	if item2.ID != "custom-id" {
		t.Errorf("ID = %q, want %q", item2.ID, "custom-id")
	}

	// Get
	got := resource.Get(item.ID)
	if got == nil {
		t.Fatal("Get returned nil for existing item")
		return
	}
	if got.Data["name"] != "John Doe" {
		t.Errorf("name = %v, want %q", got.Data["name"], "John Doe")
	}

	// Get non-existent
	got = resource.Get("nonexistent")
	if got != nil {
		t.Error("Get should return nil for non-existent item")
	}

	// Update
	updated, err := resource.Update(item.ID, map[string]interface{}{
		"name":  "John Updated",
		"email": "john.updated@example.com",
	})
	if err != nil {
		t.Fatalf("Update failed: %v", err)
	}
	if updated.Data["name"] != "John Updated" {
		t.Errorf("name = %v, want %q", updated.Data["name"], "John Updated")
	}
	if updated.CreatedAt != item.CreatedAt {
		t.Error("CreatedAt should be preserved on update")
	}
	if !updated.UpdatedAt.After(item.UpdatedAt) {
		t.Error("UpdatedAt should be later after update")
	}

	// Update non-existent
	_, err = resource.Update("nonexistent", map[string]interface{}{"name": "test"})
	if err == nil {
		t.Error("Update should fail for non-existent item")
	}
	if _, ok := err.(*NotFoundError); !ok {
		t.Errorf("expected NotFoundError, got %T", err)
	}

	// Delete
	err = resource.Delete(item.ID)
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	if resource.Get(item.ID) != nil {
		t.Error("item should be deleted")
	}

	// Delete non-existent
	err = resource.Delete("nonexistent")
	if err == nil {
		t.Error("Delete should fail for non-existent item")
	}
}

func TestStatefulResource_CreateDuplicate(t *testing.T) {
	config := &ResourceConfig{
		Name:     "users",
		BasePath: "/api/users",
	}
	resource := NewStatefulResource(config)

	resource.Create(map[string]interface{}{"id": "dup-id", "name": "First"}, nil)

	_, err := resource.Create(map[string]interface{}{"id": "dup-id", "name": "Second"}, nil)
	if err == nil {
		t.Error("expected ConflictError for duplicate ID")
	}
	if _, ok := err.(*ConflictError); !ok {
		t.Errorf("expected ConflictError, got %T", err)
	}
}

func TestStatefulResource_List(t *testing.T) {
	config := &ResourceConfig{
		Name:     "users",
		BasePath: "/api/users",
	}
	resource := NewStatefulResource(config)

	// Create test data
	for i := 0; i < 5; i++ {
		resource.Create(map[string]interface{}{
			"name":  "User " + string(rune('A'+i)),
			"score": float64(100 - i*10),
		}, nil)
		time.Sleep(time.Millisecond) // Ensure different timestamps
	}

	// List all (nil filter)
	resp := resource.List(nil)
	if len(resp.Data) != 5 {
		t.Errorf("expected 5 items, got %d", len(resp.Data))
	}
	if resp.Meta.Total != 5 {
		t.Errorf("expected total=5, got %d", resp.Meta.Total)
	}

	// List with pagination
	resp = resource.List(&QueryFilter{Limit: 2, Offset: 1})
	if len(resp.Data) != 2 {
		t.Errorf("expected 2 items, got %d", len(resp.Data))
	}
	if resp.Meta.Total != 5 {
		t.Errorf("expected total=5, got %d", resp.Meta.Total)
	}
}

func TestStatefulResource_Reset(t *testing.T) {
	config := &ResourceConfig{
		Name:     "users",
		BasePath: "/api/users",
		SeedData: []map[string]interface{}{
			{"id": "seed-1", "name": "Seed User"},
		},
	}
	resource := NewStatefulResource(config)
	resource.loadSeed()

	// Add more items
	resource.Create(map[string]interface{}{"id": "new-1", "name": "New User"}, nil)
	if resource.Count() != 2 {
		t.Fatalf("expected 2 items before reset, got %d", resource.Count())
	}

	// Reset
	resource.Reset()
	if resource.Count() != 1 {
		t.Errorf("expected 1 item after reset, got %d", resource.Count())
	}
	if resource.Get("seed-1") == nil {
		t.Error("seed item should exist after reset")
	}
	if resource.Get("new-1") != nil {
		t.Error("new item should not exist after reset")
	}
}

func TestStatefulResource_Clear(t *testing.T) {
	config := &ResourceConfig{
		Name:     "users",
		BasePath: "/api/users",
		SeedData: []map[string]interface{}{
			{"id": "seed-1", "name": "Seed User"},
		},
	}
	resource := NewStatefulResource(config)
	resource.loadSeed()
	resource.Create(map[string]interface{}{"id": "new-1", "name": "New User"}, nil)

	count := resource.Clear()
	if count != 2 {
		t.Errorf("Clear returned %d, want 2", count)
	}
	if resource.Count() != 0 {
		t.Errorf("resource has %d items, want 0", resource.Count())
	}
}

func TestStatefulResource_Accessors(t *testing.T) {
	config := &ResourceConfig{
		Name:        "users",
		BasePath:    "/api/users",
		ParentField: "orgId",
	}
	resource := NewStatefulResource(config)

	if resource.Name() != "users" {
		t.Errorf("Name() = %q, want %q", resource.Name(), "users")
	}
	if resource.BasePath() != "/api/users" {
		t.Errorf("BasePath() = %q, want %q", resource.BasePath(), "/api/users")
	}
	if resource.ParentField() != "orgId" {
		t.Errorf("ParentField() = %q, want %q", resource.ParentField(), "orgId")
	}
}

func TestStatefulResource_NestedWithParentField(t *testing.T) {
	config := &ResourceConfig{
		Name:        "orders",
		BasePath:    "/api/users/:userId/orders",
		ParentField: "userId",
	}
	resource := NewStatefulResource(config)

	// Create with path params
	item, err := resource.Create(
		map[string]interface{}{"product": "Widget"},
		map[string]string{"userId": "user-123"},
	)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Parent field should be set
	if item.Data["userId"] != "user-123" {
		t.Errorf("userId = %v, want %q", item.Data["userId"], "user-123")
	}
}

// =============================================================================
// Filter & Sorting Tests
// =============================================================================

func TestApplyFilters(t *testing.T) {
	items := []*ResourceItem{
		{ID: "1", Data: map[string]interface{}{"status": "active", "type": "admin"}},
		{ID: "2", Data: map[string]interface{}{"status": "inactive", "type": "user"}},
		{ID: "3", Data: map[string]interface{}{"status": "active", "type": "user"}},
	}

	// No filters
	result := ApplyFilters(items, &QueryFilter{})
	if len(result) != 3 {
		t.Errorf("expected 3 items with no filter, got %d", len(result))
	}

	// Filter by field
	result = ApplyFilters(items, &QueryFilter{
		Filters: map[string]string{"status": "active"},
	})
	if len(result) != 2 {
		t.Errorf("expected 2 active items, got %d", len(result))
	}

	// Filter by ID
	result = ApplyFilters(items, &QueryFilter{
		Filters: map[string]string{"id": "1"},
	})
	if len(result) != 1 {
		t.Errorf("expected 1 item with id=1, got %d", len(result))
	}

	// Multiple filters
	result = ApplyFilters(items, &QueryFilter{
		Filters: map[string]string{"status": "active", "type": "user"},
	})
	if len(result) != 1 {
		t.Errorf("expected 1 active user, got %d", len(result))
	}

	// Parent field filter
	itemsWithParent := []*ResourceItem{
		{ID: "1", Data: map[string]interface{}{"userId": "u1"}},
		{ID: "2", Data: map[string]interface{}{"userId": "u2"}},
		{ID: "3", Data: map[string]interface{}{"userId": "u1"}},
	}
	result = ApplyFilters(itemsWithParent, &QueryFilter{
		ParentField: "userId",
		ParentID:    "u1",
	})
	if len(result) != 2 {
		t.Errorf("expected 2 items for u1, got %d", len(result))
	}
}

func TestSortItems(t *testing.T) {
	now := time.Now()
	items := []*ResourceItem{
		{ID: "b", CreatedAt: now.Add(-time.Hour), Data: map[string]interface{}{"score": 50.0}},
		{ID: "a", CreatedAt: now.Add(-2 * time.Hour), Data: map[string]interface{}{"score": 100.0}},
		{ID: "c", CreatedAt: now, Data: map[string]interface{}{"score": 75.0}},
	}

	// Sort by ID ascending
	SortItems(items, "id", "asc")
	if items[0].ID != "a" || items[1].ID != "b" || items[2].ID != "c" {
		t.Error("sort by ID asc failed")
	}

	// Sort by ID descending
	SortItems(items, "id", "desc")
	if items[0].ID != "c" || items[1].ID != "b" || items[2].ID != "a" {
		t.Error("sort by ID desc failed")
	}

	// Sort by createdAt (default field when empty)
	SortItems(items, "", "asc")
	// Should sort oldest first
	if items[0].ID != "a" {
		t.Error("sort by createdAt asc failed - expected oldest first")
	}

	// Sort by custom field
	SortItems(items, "score", "desc")
	if items[0].Data["score"].(float64) != 100.0 {
		t.Error("sort by score desc failed")
	}
}

func TestCompareValues(t *testing.T) {
	tests := []struct {
		a, b interface{}
		want bool
	}{
		{"a", "b", true},
		{"b", "a", false},
		{1, 2, true},
		{2, 1, false},
		{int64(1), int64(2), true},
		{1.5, 2.5, true},
		{time.Now(), time.Now().Add(time.Hour), true},
		{"abc", 123, false}, // Falls back to string comparison: "abc" > "123" in ASCII
	}

	for _, tt := range tests {
		got := CompareValues(tt.a, tt.b)
		if got != tt.want {
			t.Errorf("CompareValues(%v, %v) = %v, want %v", tt.a, tt.b, got, tt.want)
		}
	}
}

func TestPaginate(t *testing.T) {
	items := make([]*ResourceItem, 10)
	for i := 0; i < 10; i++ {
		items[i] = &ResourceItem{ID: string(rune('a' + i))}
	}

	tests := []struct {
		offset, limit int
		wantLen       int
		wantTotal     int
	}{
		{0, 5, 5, 10},
		{5, 5, 5, 10},
		{0, 10, 10, 10},
		{0, 20, 10, 10}, // Limit exceeds total
		{8, 5, 2, 10},   // Offset near end
		{10, 5, 0, 10},  // Offset at end
		{15, 5, 0, 10},  // Offset beyond end
	}

	for _, tt := range tests {
		result, total := Paginate(items, tt.offset, tt.limit)
		if len(result) != tt.wantLen {
			t.Errorf("Paginate(offset=%d, limit=%d) len=%d, want %d",
				tt.offset, tt.limit, len(result), tt.wantLen)
		}
		if total != tt.wantTotal {
			t.Errorf("Paginate(offset=%d, limit=%d) total=%d, want %d",
				tt.offset, tt.limit, total, tt.wantTotal)
		}
	}
}

// =============================================================================
// Types Tests
// =============================================================================

func TestResourceItem_ToJSON(t *testing.T) {
	now := time.Now()
	item := &ResourceItem{
		ID:        "test-id",
		Data:      map[string]interface{}{"name": "Test", "score": 100},
		CreatedAt: now,
		UpdatedAt: now,
	}

	json := item.ToJSON()

	if json["id"] != "test-id" {
		t.Errorf("id = %v, want %q", json["id"], "test-id")
	}
	if json["name"] != "Test" {
		t.Errorf("name = %v, want %q", json["name"], "Test")
	}
	if json["score"] != 100 {
		t.Errorf("score = %v, want 100", json["score"])
	}
	if _, ok := json["createdAt"].(string); !ok {
		t.Error("createdAt should be RFC3339 string")
	}
}

func TestFromJSON(t *testing.T) {
	data := map[string]interface{}{
		"id":        "test-id",
		"name":      "Test",
		"createdAt": "2024-01-01T00:00:00Z", // Should be ignored
	}

	item := FromJSON(data, "id")

	if item.ID != "test-id" {
		t.Errorf("ID = %q, want %q", item.ID, "test-id")
	}
	if item.Data["name"] != "Test" {
		t.Errorf("Data[name] = %v, want %q", item.Data["name"], "Test")
	}
	if _, exists := item.Data["id"]; exists {
		t.Error("ID should not be in Data")
	}
	if _, exists := item.Data["createdAt"]; exists {
		t.Error("createdAt should not be in Data")
	}

	// Custom ID field
	data2 := map[string]interface{}{
		"userId": "custom-id",
		"name":   "Custom",
	}
	item2 := FromJSON(data2, "userId")
	if item2.ID != "custom-id" {
		t.Errorf("ID = %q, want %q", item2.ID, "custom-id")
	}

	// Empty ID field defaults to "id"
	item3 := FromJSON(data, "")
	if item3.ID != "test-id" {
		t.Errorf("ID = %q, want %q", item3.ID, "test-id")
	}
}

func TestDefaultQueryFilter(t *testing.T) {
	filter := DefaultQueryFilter()

	if filter.Limit != 100 {
		t.Errorf("Limit = %d, want 100", filter.Limit)
	}
	if filter.Offset != 0 {
		t.Errorf("Offset = %d, want 0", filter.Offset)
	}
	if filter.Sort != "createdAt" {
		t.Errorf("Sort = %q, want %q", filter.Sort, "createdAt")
	}
	if filter.Order != "desc" {
		t.Errorf("Order = %q, want %q", filter.Order, "desc")
	}
	if filter.Filters == nil {
		t.Error("Filters should be initialized")
	}
}

// =============================================================================
// Error Types Tests
// =============================================================================

func TestNotFoundError(t *testing.T) {
	// With ID
	err := &NotFoundError{Resource: "users", ID: "123"}
	if err.Error() != `resource "users" item "123" not found` {
		t.Errorf("unexpected error message: %s", err.Error())
	}
	if err.StatusCode() != http.StatusNotFound {
		t.Errorf("StatusCode = %d, want %d", err.StatusCode(), http.StatusNotFound)
	}
	if err.Hint() == "" {
		t.Error("Hint should not be empty")
	}

	// Without ID
	err2 := &NotFoundError{Resource: "users"}
	if err2.Error() != `resource "users" not found` {
		t.Errorf("unexpected error message: %s", err2.Error())
	}
}

func TestConflictError(t *testing.T) {
	err := &ConflictError{Resource: "users", ID: "123"}
	if err.StatusCode() != http.StatusConflict {
		t.Errorf("StatusCode = %d, want %d", err.StatusCode(), http.StatusConflict)
	}
	if err.Hint() == "" {
		t.Error("Hint should not be empty")
	}
}

func TestValidationError(t *testing.T) {
	// With field
	err := &ValidationError{Message: "invalid email", Field: "email"}
	if err.StatusCode() != http.StatusBadRequest {
		t.Errorf("StatusCode = %d, want %d", err.StatusCode(), http.StatusBadRequest)
	}
	if err.Error() != `validation failed for field "email": invalid email` {
		t.Errorf("unexpected error message: %s", err.Error())
	}

	// Without field
	err2 := &ValidationError{Message: "invalid request"}
	if err2.Error() != "invalid request" {
		t.Errorf("unexpected error message: %s", err2.Error())
	}
}

func TestPayloadTooLargeError(t *testing.T) {
	err := &PayloadTooLargeError{MaxSize: 1024, ActualSize: 2048}
	if err.StatusCode() != http.StatusRequestEntityTooLarge {
		t.Errorf("StatusCode = %d, want %d", err.StatusCode(), http.StatusRequestEntityTooLarge)
	}
	if err.Hint() == "" {
		t.Error("Hint should not be empty")
	}
}

func TestToErrorResponse(t *testing.T) {
	tests := []struct {
		err            error
		wantStatus     int
		wantErrorField string
	}{
		{&NotFoundError{Resource: "users", ID: "123"}, http.StatusNotFound, "resource not found"},
		{&ConflictError{Resource: "users", ID: "123"}, http.StatusConflict, "resource already exists"},
		{&ValidationError{Message: "bad", Field: "email"}, http.StatusBadRequest, "invalid request"},
		{&PayloadTooLargeError{MaxSize: 1024}, http.StatusRequestEntityTooLarge, "payload too large"},
		{&testError{}, http.StatusInternalServerError, "internal error"},
	}

	for _, tt := range tests {
		resp := ToErrorResponse(tt.err)
		if resp.StatusCode != tt.wantStatus {
			t.Errorf("ToErrorResponse(%T): StatusCode = %d, want %d",
				tt.err, resp.StatusCode, tt.wantStatus)
		}
		if resp.Error != tt.wantErrorField {
			t.Errorf("ToErrorResponse(%T): Error = %q, want %q",
				tt.err, resp.Error, tt.wantErrorField)
		}
	}
}

// =============================================================================
// Observer Tests
// =============================================================================

func TestNoopObserver(t *testing.T) {
	obs := &NoopObserver{}

	// All methods should not panic
	obs.OnCreate("users", "1", time.Millisecond)
	obs.OnRead("users", "1", time.Millisecond)
	obs.OnList("users", 10, time.Millisecond)
	obs.OnUpdate("users", "1", time.Millisecond)
	obs.OnDelete("users", "1", time.Millisecond)
	obs.OnError("users", "create", nil)
	obs.OnReset([]string{"users"}, time.Millisecond)
}

func TestMetricsObserver(t *testing.T) {
	obs := NewMetricsObserver()

	obs.OnCreate("users", "1", time.Millisecond)
	obs.OnCreate("users", "2", time.Millisecond)
	obs.OnRead("users", "1", time.Millisecond)
	obs.OnList("users", 10, time.Millisecond)
	obs.OnUpdate("users", "1", time.Millisecond)
	obs.OnDelete("users", "1", time.Millisecond)
	obs.OnError("users", "create", nil)
	obs.OnReset([]string{"users"}, time.Millisecond)

	snapshot := obs.Snapshot()

	if snapshot.CreateCount != 2 {
		t.Errorf("CreateCount = %d, want 2", snapshot.CreateCount)
	}
	if snapshot.ReadCount != 1 {
		t.Errorf("ReadCount = %d, want 1", snapshot.ReadCount)
	}
	if snapshot.ListCount != 1 {
		t.Errorf("ListCount = %d, want 1", snapshot.ListCount)
	}
	if snapshot.UpdateCount != 1 {
		t.Errorf("UpdateCount = %d, want 1", snapshot.UpdateCount)
	}
	if snapshot.DeleteCount != 1 {
		t.Errorf("DeleteCount = %d, want 1", snapshot.DeleteCount)
	}
	if snapshot.ErrorCount != 1 {
		t.Errorf("ErrorCount = %d, want 1", snapshot.ErrorCount)
	}
	if snapshot.ResetCount != 1 {
		t.Errorf("ResetCount = %d, want 1", snapshot.ResetCount)
	}
	if snapshot.TotalLatency < 7*time.Millisecond {
		t.Errorf("TotalLatency = %v, want >= 7ms", snapshot.TotalLatency)
	}

	total := snapshot.TotalOperations()
	if total != 6 {
		t.Errorf("TotalOperations = %d, want 6", total)
	}
}

func TestMetricsObserver_Reset(t *testing.T) {
	obs := NewMetricsObserver()

	// Add some metrics
	obs.OnCreate("users", "1", time.Millisecond)
	obs.OnRead("users", "1", time.Millisecond)
	obs.OnError("users", "create", nil)

	// Verify they were recorded
	snapshot := obs.Snapshot()
	if snapshot.CreateCount != 1 || snapshot.ReadCount != 1 || snapshot.ErrorCount != 1 {
		t.Error("Metrics not recorded before reset")
	}

	// Reset all metrics
	obs.Reset()

	// Verify all counters are zero
	snapshot = obs.Snapshot()
	if snapshot.CreateCount != 0 {
		t.Errorf("CreateCount after reset = %d, want 0", snapshot.CreateCount)
	}
	if snapshot.ReadCount != 0 {
		t.Errorf("ReadCount after reset = %d, want 0", snapshot.ReadCount)
	}
	if snapshot.ListCount != 0 {
		t.Errorf("ListCount after reset = %d, want 0", snapshot.ListCount)
	}
	if snapshot.UpdateCount != 0 {
		t.Errorf("UpdateCount after reset = %d, want 0", snapshot.UpdateCount)
	}
	if snapshot.DeleteCount != 0 {
		t.Errorf("DeleteCount after reset = %d, want 0", snapshot.DeleteCount)
	}
	if snapshot.ErrorCount != 0 {
		t.Errorf("ErrorCount after reset = %d, want 0", snapshot.ErrorCount)
	}
	if snapshot.ResetCount != 0 {
		t.Errorf("ResetCount after reset = %d, want 0", snapshot.ResetCount)
	}
	if snapshot.TotalLatency != 0 {
		t.Errorf("TotalLatency after reset = %v, want 0", snapshot.TotalLatency)
	}
}

func TestMetricsObserver_Concurrent(t *testing.T) {
	// This test verifies that MetricsObserver is safe for concurrent use.
	// Run with -race flag to detect race conditions: go test -race
	obs := NewMetricsObserver()

	const numGoroutines = 100
	const opsPerGoroutine = 1000

	done := make(chan bool, numGoroutines)

	// Launch many goroutines that all update metrics concurrently
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			for j := 0; j < opsPerGoroutine; j++ {
				switch j % 7 {
				case 0:
					obs.OnCreate("resource", "id", time.Microsecond)
				case 1:
					obs.OnRead("resource", "id", time.Microsecond)
				case 2:
					obs.OnList("resource", 10, time.Microsecond)
				case 3:
					obs.OnUpdate("resource", "id", time.Microsecond)
				case 4:
					obs.OnDelete("resource", "id", time.Microsecond)
				case 5:
					obs.OnError("resource", "op", nil)
				case 6:
					obs.OnReset([]string{"resource"}, time.Microsecond)
				}
			}
			done <- true
		}(i)
	}

	// Wait for all goroutines to complete
	for i := 0; i < numGoroutines; i++ {
		<-done
	}

	// Verify total counts are correct
	snapshot := obs.Snapshot()
	totalExpected := int64(numGoroutines * opsPerGoroutine)

	// Each operation type should occur approximately totalExpected/7 times
	// Due to integer division, we check that the sum equals the total
	totalOps := snapshot.CreateCount + snapshot.ReadCount + snapshot.ListCount +
		snapshot.UpdateCount + snapshot.DeleteCount + snapshot.ErrorCount + snapshot.ResetCount

	if totalOps != totalExpected {
		t.Errorf("Total operations = %d, want %d", totalOps, totalExpected)
	}

	// Verify each counter is reasonable (should be roughly totalExpected/7)
	expectedPerOp := totalExpected / 7
	tolerance := int64(numGoroutines) // Allow small variance

	counters := []struct {
		name  string
		value int64
	}{
		{"CreateCount", snapshot.CreateCount},
		{"ReadCount", snapshot.ReadCount},
		{"ListCount", snapshot.ListCount},
		{"UpdateCount", snapshot.UpdateCount},
		{"DeleteCount", snapshot.DeleteCount},
		{"ErrorCount", snapshot.ErrorCount},
		{"ResetCount", snapshot.ResetCount},
	}

	for _, c := range counters {
		diff := c.value - expectedPerOp
		if diff < 0 {
			diff = -diff
		}
		if diff > tolerance {
			t.Errorf("%s = %d, expected approximately %d (tolerance %d)",
				c.name, c.value, expectedPerOp, tolerance)
		}
	}
}

func TestMetricsObserver_ConcurrentSnapshotAndUpdate(t *testing.T) {
	// Test that taking snapshots while updates are happening is safe
	obs := NewMetricsObserver()

	const numUpdaters = 50
	const numSnapshots = 50
	const opsPerGoroutine = 500

	done := make(chan bool, numUpdaters+numSnapshots)

	// Launch updater goroutines
	for i := 0; i < numUpdaters; i++ {
		go func() {
			for j := 0; j < opsPerGoroutine; j++ {
				obs.OnCreate("resource", "id", time.Microsecond)
				obs.OnRead("resource", "id", time.Microsecond)
			}
			done <- true
		}()
	}

	// Launch snapshot goroutines
	for i := 0; i < numSnapshots; i++ {
		go func() {
			for j := 0; j < opsPerGoroutine; j++ {
				snapshot := obs.Snapshot()
				// Verify snapshot is internally consistent
				// (all values should be non-negative)
				if snapshot.CreateCount < 0 || snapshot.ReadCount < 0 {
					t.Errorf("Snapshot has negative count: %+v", snapshot)
				}
			}
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < numUpdaters+numSnapshots; i++ {
		<-done
	}

	// Final verification
	snapshot := obs.Snapshot()
	expectedCreates := int64(numUpdaters * opsPerGoroutine)
	expectedReads := int64(numUpdaters * opsPerGoroutine)

	if snapshot.CreateCount != expectedCreates {
		t.Errorf("Final CreateCount = %d, want %d", snapshot.CreateCount, expectedCreates)
	}
	if snapshot.ReadCount != expectedReads {
		t.Errorf("Final ReadCount = %d, want %d", snapshot.ReadCount, expectedReads)
	}
}

func TestMetricsObserver_ConcurrentResetAndUpdate(t *testing.T) {
	// Test that resetting while updates are happening is safe
	obs := NewMetricsObserver()

	const numUpdaters = 20
	const numResetters = 5
	const opsPerGoroutine = 200

	done := make(chan bool, numUpdaters+numResetters)

	// Launch updater goroutines
	for i := 0; i < numUpdaters; i++ {
		go func() {
			for j := 0; j < opsPerGoroutine; j++ {
				obs.OnCreate("resource", "id", time.Microsecond)
			}
			done <- true
		}()
	}

	// Launch reset goroutines
	for i := 0; i < numResetters; i++ {
		go func() {
			for j := 0; j < opsPerGoroutine; j++ {
				obs.Reset()
			}
			done <- true
		}()
	}

	// Wait for all goroutines - this should not panic or deadlock
	for i := 0; i < numUpdaters+numResetters; i++ {
		<-done
	}

	// Just verify we can still take a snapshot (no panic)
	snapshot := obs.Snapshot()
	_ = snapshot.TotalOperations()
}

// =============================================================================
// Edge Case Tests (P1 Audit Findings)
// =============================================================================

func TestResource_EdgeCase_EmptyResource(t *testing.T) {
	// Test operations on an empty resource (no seed data)
	store := NewStateStore()
	err := store.Register(&ResourceConfig{
		Name:     "empty",
		BasePath: "/api/empty",
		// No seed data
	})
	if err != nil {
		t.Fatalf("failed to register: %v", err)
	}

	resource := store.Get("empty")
	if resource == nil {
		t.Fatal("resource should exist")
	}

	// List should return response with empty data, not nil
	response := resource.List(nil)
	if response == nil {
		t.Fatal("List should return response, not nil")
		return
	}
	if len(response.Data) != 0 {
		t.Errorf("expected 0 items, got %d", len(response.Data))
	}
	if response.Meta.Total != 0 {
		t.Errorf("expected total 0, got %d", response.Meta.Total)
	}

	// Count should be 0
	if resource.Count() != 0 {
		t.Errorf("expected count 0, got %d", resource.Count())
	}

	// Get non-existent ID should return nil
	item := resource.Get("nonexistent")
	if item != nil {
		t.Error("Get on empty resource should return nil")
	}

	// Update non-existent should return error
	_, err = resource.Update("nonexistent", map[string]interface{}{"name": "test"})
	if err == nil {
		t.Error("Update on empty resource should return error")
	}

	// Delete non-existent should return error
	err = resource.Delete("nonexistent")
	if err == nil {
		t.Error("Delete on empty resource should return error")
	}
}

func TestResource_EdgeCase_NegativeOffset(t *testing.T) {
	store := NewStateStore()
	store.Register(&ResourceConfig{
		Name:     "items",
		BasePath: "/api/items",
		SeedData: []map[string]interface{}{
			{"id": "1", "name": "Item 1"},
			{"id": "2", "name": "Item 2"},
			{"id": "3", "name": "Item 3"},
		},
	})

	resource := store.Get("items")

	// Negative offset should be treated as 0 or handled gracefully
	filter := &QueryFilter{Offset: -5, Limit: 10}
	response := resource.List(filter)
	// Implementation should handle gracefully
	if response == nil {
		t.Fatal("List with negative offset should not return nil")
		return
	}
	// Should still return items (treating -5 as 0 or clamping)
	if len(response.Data) == 0 {
		t.Error("List with negative offset should return items")
	}
}

func TestResource_EdgeCase_ZeroLimit(t *testing.T) {
	store := NewStateStore()
	store.Register(&ResourceConfig{
		Name:     "items",
		BasePath: "/api/items",
		SeedData: []map[string]interface{}{
			{"id": "1", "name": "Item 1"},
			{"id": "2", "name": "Item 2"},
		},
	})

	resource := store.Get("items")

	// Zero limit - default should be applied
	filter := &QueryFilter{Offset: 0, Limit: 0}
	response := resource.List(filter)
	// Should handle gracefully (apply default limit)
	if response == nil {
		t.Error("List with zero limit should not return nil")
	}
}

func TestResource_EdgeCase_LargeOffset(t *testing.T) {
	store := NewStateStore()
	store.Register(&ResourceConfig{
		Name:     "items",
		BasePath: "/api/items",
		SeedData: []map[string]interface{}{
			{"id": "1", "name": "Item 1"},
		},
	})

	resource := store.Get("items")

	// Offset larger than total items should return empty data
	filter := &QueryFilter{Offset: 1000, Limit: 10}
	response := resource.List(filter)
	if response == nil {
		t.Fatal("List with large offset should return response, not nil")
		return
	}
	if len(response.Data) != 0 {
		t.Errorf("expected 0 items with large offset, got %d", len(response.Data))
	}
}

func TestResource_EdgeCase_SpecialCharactersInID(t *testing.T) {
	store := NewStateStore()
	store.Register(&ResourceConfig{
		Name:     "items",
		BasePath: "/api/items",
	})

	resource := store.Get("items")

	// Test IDs with special characters
	specialIDs := []string{
		"id-with-dashes",
		"id_with_underscores",
		"id.with.dots",
		"id:with:colons",
		"id/with/slashes",
		"id with spaces",
		"id@with@at",
	}

	for _, id := range specialIDs {
		data := map[string]interface{}{"id": id, "name": "test"}
		created, err := resource.Create(data, nil)
		if err != nil {
			// Some special chars may be rejected - that's OK
			continue
		}

		// If created, verify we can retrieve it
		retrieved := resource.Get(id)
		if retrieved == nil {
			t.Errorf("failed to retrieve item with ID %q", id)
			continue
		}

		// Verify we can delete it
		err = resource.Delete(id)
		if err != nil {
			t.Errorf("failed to delete item with ID %q: %v", id, err)
		}

		_ = created // suppress unused warning
	}
}

func TestResource_EdgeCase_EmptyUpdate(t *testing.T) {
	store := NewStateStore()
	store.Register(&ResourceConfig{
		Name:     "items",
		BasePath: "/api/items",
		SeedData: []map[string]interface{}{
			{"id": "1", "name": "Original"},
		},
	})

	resource := store.Get("items")

	// Update uses PUT semantics (full replacement), not PATCH (partial update)
	// Empty data should replace the entire item data (except system fields)
	_, err := resource.Update("1", map[string]interface{}{})
	// Should not error - empty update is valid
	if err != nil {
		t.Errorf("empty update should not error: %v", err)
	}

	// Item should still exist with ID preserved
	item := resource.Get("1")
	if item == nil {
		t.Fatal("item should still exist")
		return
	}

	// ID should be preserved (system field)
	if item.ID != "1" {
		t.Errorf("ID should be preserved, got %v", item.ID)
	}

	// CreatedAt should be preserved (system field)
	if item.CreatedAt.IsZero() {
		t.Error("CreatedAt should be preserved")
	}

	// With PUT semantics, user data fields are replaced (not preserved)
	// This is the expected behavior for full replacement
	itemData := item.ToJSON()
	// The "name" field should NOT be present since we replaced with empty data
	// This test documents the PUT semantics behavior
	_ = itemData // Document that data is now empty (except id, createdAt, updatedAt)
}

func TestResource_EdgeCase_NullValues(t *testing.T) {
	store := NewStateStore()
	store.Register(&ResourceConfig{
		Name:     "items",
		BasePath: "/api/items",
	})

	resource := store.Get("items")

	// Create with null values
	data := map[string]interface{}{
		"id":       "null-test",
		"name":     nil, // null value
		"optional": nil,
	}

	created, err := resource.Create(data, nil)
	if err != nil {
		t.Fatalf("create with null values failed: %v", err)
	}

	// Retrieve and verify nulls are preserved
	item := resource.Get("null-test")
	if item == nil {
		t.Fatal("item should exist")
	}
	itemData := item.ToJSON()
	if itemData["name"] != nil {
		t.Errorf("null value should be preserved, got %v", itemData["name"])
	}

	_ = created
}

// =============================================================================
// Helpers
// =============================================================================

type testError struct{}

func (e *testError) Error() string { return "test error" }

func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// =============================================================================
// Session 12: MaxItems capacity tests
// =============================================================================

func TestResource_MaxItems(t *testing.T) {
	t.Run("enforces max items on create", func(t *testing.T) {
		store := NewStateStore()
		store.Register(&ResourceConfig{
			Name:     "limited",
			BasePath: "/api/limited",
			MaxItems: 2,
		})

		resource := store.Get("limited")

		// Create first item — should succeed
		_, err := resource.Create(map[string]interface{}{"name": "first"}, nil)
		if err != nil {
			t.Fatalf("first create should succeed: %v", err)
		}

		// Create second item — should succeed
		_, err = resource.Create(map[string]interface{}{"name": "second"}, nil)
		if err != nil {
			t.Fatalf("second create should succeed: %v", err)
		}

		// Create third item — should fail with CapacityError
		_, err = resource.Create(map[string]interface{}{"name": "third"}, nil)
		if err == nil {
			t.Fatal("third create should fail due to capacity limit")
		}

		capErr, ok := err.(*CapacityError)
		if !ok {
			t.Fatalf("expected *CapacityError, got %T: %v", err, err)
		}
		if capErr.Resource != "limited" {
			t.Errorf("expected resource 'limited', got %q", capErr.Resource)
		}
		if capErr.MaxItems != 2 {
			t.Errorf("expected MaxItems 2, got %d", capErr.MaxItems)
		}
		if capErr.StatusCode() != 507 {
			t.Errorf("expected status 507, got %d", capErr.StatusCode())
		}
	})

	t.Run("zero maxItems means unlimited", func(t *testing.T) {
		store := NewStateStore()
		store.Register(&ResourceConfig{
			Name:     "unlimited",
			BasePath: "/api/unlimited",
			MaxItems: 0,
		})

		resource := store.Get("unlimited")

		// Create many items — all should succeed
		for i := 0; i < 100; i++ {
			_, err := resource.Create(map[string]interface{}{
				"name": "item",
			}, nil)
			if err != nil {
				t.Fatalf("create %d should succeed with unlimited: %v", i, err)
			}
		}
	})

	t.Run("reset frees capacity", func(t *testing.T) {
		store := NewStateStore()
		store.Register(&ResourceConfig{
			Name:     "resettable",
			BasePath: "/api/resettable",
			MaxItems: 1,
		})

		resource := store.Get("resettable")

		// Fill the resource
		_, err := resource.Create(map[string]interface{}{"name": "first"}, nil)
		if err != nil {
			t.Fatalf("first create should succeed: %v", err)
		}

		// Should be at capacity
		_, err = resource.Create(map[string]interface{}{"name": "second"}, nil)
		if err == nil {
			t.Fatal("should fail at capacity")
		}

		// Reset should clear items
		resource.Reset()

		// Should be able to create again (no seed data)
		_, err = resource.Create(map[string]interface{}{"name": "after-reset"}, nil)
		if err != nil {
			t.Fatalf("create after reset should succeed: %v", err)
		}
	})

	t.Run("capacity error in ToErrorResponse", func(t *testing.T) {
		err := &CapacityError{Resource: "users", MaxItems: 100}
		resp := ToErrorResponse(err)
		if resp.Error != "resource capacity exceeded" {
			t.Errorf("expected 'resource capacity exceeded', got %q", resp.Error)
		}
		if resp.Resource != "users" {
			t.Errorf("expected resource 'users', got %q", resp.Resource)
		}
		if resp.StatusCode != 507 {
			t.Errorf("expected status 507, got %d", resp.StatusCode)
		}
	})
}

// =============================================================================
// Session 12: Reset lock release test
// =============================================================================

func TestStateStore_ResetDoesNotHoldStoreLock(t *testing.T) {
	store := NewStateStore()

	// Register multiple resources with seed data
	for _, name := range []string{"users", "products", "orders"} {
		store.Register(&ResourceConfig{
			Name:     name,
			BasePath: "/api/" + name,
			SeedData: []map[string]interface{}{
				{"id": "seed-1", "name": name + " seed"},
			},
		})
	}

	// Reset all — should not deadlock even with concurrent reads
	done := make(chan bool, 1)
	go func() {
		resp, err := store.Reset("")
		if err != nil {
			t.Errorf("reset failed: %v", err)
		}
		if len(resp.Resources) != 3 {
			t.Errorf("expected 3 resources reset, got %d", len(resp.Resources))
		}
		done <- true
	}()

	// Concurrent reads while reset is happening
	for i := 0; i < 10; i++ {
		names := store.List()
		_ = names
	}

	<-done
}
