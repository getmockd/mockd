package storage

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/getmockd/mockd/pkg/mock"
)

// --- Helper ---

func boolPtr(b bool) *bool { return &b }

func newMock(id string, mockType mock.Type) *mock.Mock {
	return &mock.Mock{
		ID:        id,
		Type:      mockType,
		Enabled:   boolPtr(true),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}

func newHTTPMock(id string, priority int) *mock.Mock {
	m := newMock(id, mock.TypeHTTP)
	m.HTTP = &mock.HTTPSpec{
		Priority: priority,
	}
	return m
}

func newWorkspaceMock(id, workspaceID string) *mock.Mock {
	m := newMock(id, mock.TypeHTTP)
	m.WorkspaceID = workspaceID
	return m
}

// --- InMemoryMockStore Tests ---

func TestNewInMemoryMockStore(t *testing.T) {
	store := NewInMemoryMockStore()
	if store == nil {
		t.Fatal("NewInMemoryMockStore() returned nil")
	}
	if store.Count() != 0 {
		t.Errorf("new store Count() = %d, want 0", store.Count())
	}
}

func TestInMemory_SetAndGet(t *testing.T) {
	store := NewInMemoryMockStore()
	m := newMock("test-1", mock.TypeHTTP)

	if err := store.Set(m); err != nil {
		t.Fatalf("Set() error = %v", err)
	}

	got := store.Get("test-1")
	if got == nil {
		t.Fatal("Get() returned nil")
	}
	if got.ID != "test-1" {
		t.Errorf("Get().ID = %q, want %q", got.ID, "test-1")
	}
}

func TestInMemory_SetNil(t *testing.T) {
	store := NewInMemoryMockStore()
	if err := store.Set(nil); err != nil {
		t.Errorf("Set(nil) error = %v, want nil", err)
	}
	if store.Count() != 0 {
		t.Errorf("Count() after Set(nil) = %d, want 0", store.Count())
	}
}

func TestInMemory_SetOverwrite(t *testing.T) {
	store := NewInMemoryMockStore()
	m1 := newMock("test-1", mock.TypeHTTP)
	m1.Name = "original"
	_ = store.Set(m1)

	m2 := newMock("test-1", mock.TypeGraphQL)
	m2.Name = "updated"
	_ = store.Set(m2)

	got := store.Get("test-1")
	if got.Name != "updated" {
		t.Errorf("Get().Name = %q, want %q after overwrite", got.Name, "updated")
	}
	if got.Type != mock.TypeGraphQL {
		t.Errorf("Get().Type = %v, want %v after overwrite", got.Type, mock.TypeGraphQL)
	}
	if store.Count() != 1 {
		t.Errorf("Count() = %d, want 1 after overwrite", store.Count())
	}
}

func TestInMemory_GetNotFound(t *testing.T) {
	store := NewInMemoryMockStore()
	got := store.Get("nonexistent")
	if got != nil {
		t.Errorf("Get(nonexistent) = %v, want nil", got)
	}
}

func TestInMemory_Delete(t *testing.T) {
	store := NewInMemoryMockStore()
	_ = store.Set(newMock("test-1", mock.TypeHTTP))

	deleted := store.Delete("test-1")
	if !deleted {
		t.Error("Delete() = false, want true")
	}
	if store.Get("test-1") != nil {
		t.Error("Get() after Delete() returned non-nil")
	}
	if store.Count() != 0 {
		t.Errorf("Count() after Delete() = %d, want 0", store.Count())
	}
}

func TestInMemory_DeleteNotFound(t *testing.T) {
	store := NewInMemoryMockStore()
	deleted := store.Delete("nonexistent")
	if deleted {
		t.Error("Delete(nonexistent) = true, want false")
	}
}

func TestInMemory_List_Empty(t *testing.T) {
	store := NewInMemoryMockStore()
	list := store.List()
	if len(list) != 0 {
		t.Errorf("List() length = %d, want 0", len(list))
	}
}

func TestInMemory_List_SortedByPriority(t *testing.T) {
	store := NewInMemoryMockStore()
	// Add in wrong order
	_ = store.Set(newHTTPMock("low", 1))
	_ = store.Set(newHTTPMock("high", 100))
	_ = store.Set(newHTTPMock("mid", 50))

	list := store.List()
	if len(list) != 3 {
		t.Fatalf("List() length = %d, want 3", len(list))
	}
	// Should be sorted by priority descending
	if list[0].ID != "high" {
		t.Errorf("List()[0].ID = %q, want %q (highest priority)", list[0].ID, "high")
	}
	if list[1].ID != "mid" {
		t.Errorf("List()[1].ID = %q, want %q", list[1].ID, "mid")
	}
	if list[2].ID != "low" {
		t.Errorf("List()[2].ID = %q, want %q (lowest priority)", list[2].ID, "low")
	}
}

func TestInMemory_List_SortedByCreatedAtWhenSamePriority(t *testing.T) {
	store := NewInMemoryMockStore()
	m1 := newHTTPMock("first", 10)
	m1.CreatedAt = time.Now().Add(-2 * time.Hour)
	m2 := newHTTPMock("second", 10)
	m2.CreatedAt = time.Now().Add(-1 * time.Hour)
	m3 := newHTTPMock("third", 10)
	m3.CreatedAt = time.Now()

	_ = store.Set(m3) // add out of order
	_ = store.Set(m1)
	_ = store.Set(m2)

	list := store.List()
	if len(list) != 3 {
		t.Fatalf("List() length = %d, want 3", len(list))
	}
	// Same priority → sorted by creation time ascending
	if list[0].ID != "first" {
		t.Errorf("List()[0].ID = %q, want %q (earliest)", list[0].ID, "first")
	}
	if list[1].ID != "second" {
		t.Errorf("List()[1].ID = %q, want %q", list[1].ID, "second")
	}
	if list[2].ID != "third" {
		t.Errorf("List()[2].ID = %q, want %q (latest)", list[2].ID, "third")
	}
}

func TestInMemory_List_NonHTTPMocksHaveZeroPriority(t *testing.T) {
	store := NewInMemoryMockStore()
	// Non-HTTP mocks have nil HTTP field → priority 0
	grpc := newMock("grpc-1", mock.TypeGRPC)
	grpc.CreatedAt = time.Now().Add(-1 * time.Hour)
	http := newHTTPMock("http-1", 10) // higher priority
	http.CreatedAt = time.Now()

	_ = store.Set(grpc)
	_ = store.Set(http)

	list := store.List()
	// HTTP (priority 10) should come before gRPC (priority 0)
	if list[0].ID != "http-1" {
		t.Errorf("List()[0].ID = %q, want %q", list[0].ID, "http-1")
	}
}

func TestInMemory_ListByType(t *testing.T) {
	store := NewInMemoryMockStore()
	_ = store.Set(newMock("http-1", mock.TypeHTTP))
	_ = store.Set(newMock("grpc-1", mock.TypeGRPC))
	_ = store.Set(newMock("http-2", mock.TypeHTTP))
	_ = store.Set(newMock("ws-1", mock.TypeWebSocket))

	httpMocks := store.ListByType(mock.TypeHTTP)
	if len(httpMocks) != 2 {
		t.Errorf("ListByType(HTTP) = %d mocks, want 2", len(httpMocks))
	}

	grpcMocks := store.ListByType(mock.TypeGRPC)
	if len(grpcMocks) != 1 {
		t.Errorf("ListByType(GRPC) = %d mocks, want 1", len(grpcMocks))
	}

	soapMocks := store.ListByType(mock.TypeSOAP)
	if len(soapMocks) != 0 {
		t.Errorf("ListByType(SOAP) = %d mocks, want 0", len(soapMocks))
	}
}

func TestInMemory_Count(t *testing.T) {
	store := NewInMemoryMockStore()
	if store.Count() != 0 {
		t.Errorf("Count() = %d, want 0", store.Count())
	}

	_ = store.Set(newMock("a", mock.TypeHTTP))
	if store.Count() != 1 {
		t.Errorf("Count() = %d, want 1", store.Count())
	}

	_ = store.Set(newMock("b", mock.TypeHTTP))
	if store.Count() != 2 {
		t.Errorf("Count() = %d, want 2", store.Count())
	}

	store.Delete("a")
	if store.Count() != 1 {
		t.Errorf("Count() after delete = %d, want 1", store.Count())
	}
}

func TestInMemory_Clear(t *testing.T) {
	store := NewInMemoryMockStore()
	for i := 0; i < 10; i++ {
		_ = store.Set(newMock(fmt.Sprintf("mock-%d", i), mock.TypeHTTP))
	}
	if store.Count() != 10 {
		t.Fatalf("Count() = %d, want 10", store.Count())
	}

	store.Clear()
	if store.Count() != 0 {
		t.Errorf("Count() after Clear() = %d, want 0", store.Count())
	}
	if list := store.List(); len(list) != 0 {
		t.Errorf("List() after Clear() = %d items, want 0", len(list))
	}
}

func TestInMemory_Exists(t *testing.T) {
	store := NewInMemoryMockStore()
	_ = store.Set(newMock("exists", mock.TypeHTTP))

	if !store.Exists("exists") {
		t.Error("Exists(exists) = false, want true")
	}
	if store.Exists("nope") {
		t.Error("Exists(nope) = true, want false")
	}
}

func TestInMemory_Concurrent(t *testing.T) {
	store := NewInMemoryMockStore()
	const goroutines = 50
	const ops = 100
	var wg sync.WaitGroup

	// Concurrent writes
	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			for i := 0; i < ops; i++ {
				id := fmt.Sprintf("mock-%d-%d", g, i)
				_ = store.Set(newMock(id, mock.TypeHTTP))
			}
		}(g)
	}
	wg.Wait()

	if store.Count() != goroutines*ops {
		t.Errorf("Count() = %d, want %d", store.Count(), goroutines*ops)
	}

	// Concurrent reads
	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			for i := 0; i < ops; i++ {
				id := fmt.Sprintf("mock-%d-%d", g, i)
				_ = store.Get(id)
			}
		}(g)
	}
	wg.Wait()

	// Concurrent mixed operations (read + write + delete)
	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			_ = store.List()
			_ = store.Count()
			store.Delete(fmt.Sprintf("mock-%d-0", g))
			_ = store.Exists(fmt.Sprintf("mock-%d-1", g))
			_ = store.ListByType(mock.TypeHTTP)
		}(g)
	}
	wg.Wait()
}

func TestInMemory_ImplementsMockStore(t *testing.T) {
	var _ MockStore = (*InMemoryMockStore)(nil)
}

// --- FilteredMockStore Tests ---

func TestNewFilteredMockStore(t *testing.T) {
	underlying := NewInMemoryMockStore()
	filtered := NewFilteredMockStore(underlying, "ws-1")
	if filtered == nil {
		t.Fatal("NewFilteredMockStore() returned nil")
	}
	if filtered.WorkspaceID() != "ws-1" {
		t.Errorf("WorkspaceID() = %q, want %q", filtered.WorkspaceID(), "ws-1")
	}
	if filtered.Underlying() != underlying {
		t.Error("Underlying() does not match")
	}
}

func TestFiltered_SetStampsWorkspaceID(t *testing.T) {
	underlying := NewInMemoryMockStore()
	filtered := NewFilteredMockStore(underlying, "ws-1")

	m := newMock("test-1", mock.TypeHTTP)
	if err := filtered.Set(m); err != nil {
		t.Fatalf("Set() error = %v", err)
	}

	// The mock in the underlying store should have the workspace ID stamped
	got := underlying.Get("test-1")
	if got == nil {
		t.Fatal("underlying Get() returned nil")
	}
	if got.WorkspaceID != "ws-1" {
		t.Errorf("stored mock.WorkspaceID = %q, want %q", got.WorkspaceID, "ws-1")
	}
}

func TestFiltered_SetDoesNotMutateCaller(t *testing.T) {
	underlying := NewInMemoryMockStore()
	filtered := NewFilteredMockStore(underlying, "ws-1")

	m := newMock("test-1", mock.TypeHTTP)
	m.WorkspaceID = "original"

	if err := filtered.Set(m); err != nil {
		t.Fatalf("Set() error = %v", err)
	}

	// The caller's mock should NOT be mutated
	if m.WorkspaceID != "original" {
		t.Errorf("caller mock.WorkspaceID = %q, want %q (should not be mutated)", m.WorkspaceID, "original")
	}
}

func TestFiltered_GetFiltersWorkspace(t *testing.T) {
	underlying := NewInMemoryMockStore()
	_ = underlying.Set(newWorkspaceMock("mock-1", "ws-1"))
	_ = underlying.Set(newWorkspaceMock("mock-2", "ws-2"))

	ws1 := NewFilteredMockStore(underlying, "ws-1")
	ws2 := NewFilteredMockStore(underlying, "ws-2")

	// ws-1 should only see mock-1
	if ws1.Get("mock-1") == nil {
		t.Error("ws-1 Get(mock-1) = nil, want non-nil")
	}
	if ws1.Get("mock-2") != nil {
		t.Error("ws-1 Get(mock-2) = non-nil, want nil (different workspace)")
	}

	// ws-2 should only see mock-2
	if ws2.Get("mock-2") == nil {
		t.Error("ws-2 Get(mock-2) = nil, want non-nil")
	}
	if ws2.Get("mock-1") != nil {
		t.Error("ws-2 Get(mock-1) = non-nil, want nil (different workspace)")
	}
}

func TestFiltered_GetNotFound(t *testing.T) {
	underlying := NewInMemoryMockStore()
	filtered := NewFilteredMockStore(underlying, "ws-1")

	if filtered.Get("nonexistent") != nil {
		t.Error("Get(nonexistent) = non-nil, want nil")
	}
}

func TestFiltered_DeleteOnlyOwnWorkspace(t *testing.T) {
	underlying := NewInMemoryMockStore()
	_ = underlying.Set(newWorkspaceMock("mock-1", "ws-1"))
	_ = underlying.Set(newWorkspaceMock("mock-2", "ws-2"))

	ws1 := NewFilteredMockStore(underlying, "ws-1")

	// Should NOT be able to delete mock from different workspace
	if ws1.Delete("mock-2") {
		t.Error("Delete(mock-2) from ws-1 = true, want false (wrong workspace)")
	}
	// Underlying should still have it
	if underlying.Get("mock-2") == nil {
		t.Error("mock-2 was deleted from underlying despite wrong workspace")
	}

	// Should be able to delete own mock
	if !ws1.Delete("mock-1") {
		t.Error("Delete(mock-1) from ws-1 = false, want true")
	}
}

func TestFiltered_DeleteNotFound(t *testing.T) {
	underlying := NewInMemoryMockStore()
	filtered := NewFilteredMockStore(underlying, "ws-1")

	if filtered.Delete("nonexistent") {
		t.Error("Delete(nonexistent) = true, want false")
	}
}

func TestFiltered_List(t *testing.T) {
	underlying := NewInMemoryMockStore()
	_ = underlying.Set(newWorkspaceMock("ws1-a", "ws-1"))
	_ = underlying.Set(newWorkspaceMock("ws1-b", "ws-1"))
	_ = underlying.Set(newWorkspaceMock("ws2-a", "ws-2"))

	ws1 := NewFilteredMockStore(underlying, "ws-1")
	list := ws1.List()
	if len(list) != 2 {
		t.Errorf("ws-1 List() = %d items, want 2", len(list))
	}

	ws2 := NewFilteredMockStore(underlying, "ws-2")
	list = ws2.List()
	if len(list) != 1 {
		t.Errorf("ws-2 List() = %d items, want 1", len(list))
	}

	ws3 := NewFilteredMockStore(underlying, "ws-3")
	list = ws3.List()
	if len(list) != 0 {
		t.Errorf("ws-3 List() = %d items, want 0", len(list))
	}
}

func TestFiltered_ListByType(t *testing.T) {
	underlying := NewInMemoryMockStore()

	httpMock := newWorkspaceMock("http-1", "ws-1")
	httpMock.Type = mock.TypeHTTP
	_ = underlying.Set(httpMock)

	grpcMock := newWorkspaceMock("grpc-1", "ws-1")
	grpcMock.Type = mock.TypeGRPC
	_ = underlying.Set(grpcMock)

	otherHTTP := newWorkspaceMock("http-2", "ws-2")
	otherHTTP.Type = mock.TypeHTTP
	_ = underlying.Set(otherHTTP)

	ws1 := NewFilteredMockStore(underlying, "ws-1")

	httpList := ws1.ListByType(mock.TypeHTTP)
	if len(httpList) != 1 {
		t.Errorf("ws-1 ListByType(HTTP) = %d, want 1", len(httpList))
	}

	grpcList := ws1.ListByType(mock.TypeGRPC)
	if len(grpcList) != 1 {
		t.Errorf("ws-1 ListByType(GRPC) = %d, want 1", len(grpcList))
	}
}

func TestFiltered_Count(t *testing.T) {
	underlying := NewInMemoryMockStore()
	_ = underlying.Set(newWorkspaceMock("a", "ws-1"))
	_ = underlying.Set(newWorkspaceMock("b", "ws-1"))
	_ = underlying.Set(newWorkspaceMock("c", "ws-2"))

	ws1 := NewFilteredMockStore(underlying, "ws-1")
	if ws1.Count() != 2 {
		t.Errorf("ws-1 Count() = %d, want 2", ws1.Count())
	}

	ws2 := NewFilteredMockStore(underlying, "ws-2")
	if ws2.Count() != 1 {
		t.Errorf("ws-2 Count() = %d, want 1", ws2.Count())
	}
}

func TestFiltered_Clear(t *testing.T) {
	underlying := NewInMemoryMockStore()
	_ = underlying.Set(newWorkspaceMock("ws1-a", "ws-1"))
	_ = underlying.Set(newWorkspaceMock("ws1-b", "ws-1"))
	_ = underlying.Set(newWorkspaceMock("ws2-a", "ws-2"))

	ws1 := NewFilteredMockStore(underlying, "ws-1")
	ws1.Clear()

	// ws-1 mocks should be gone
	if ws1.Count() != 0 {
		t.Errorf("ws-1 Count() after Clear() = %d, want 0", ws1.Count())
	}

	// ws-2 mocks should be untouched
	if underlying.Get("ws2-a") == nil {
		t.Error("ws-2 mock was deleted by ws-1 Clear()")
	}
	if underlying.Count() != 1 {
		t.Errorf("underlying Count() = %d, want 1 (ws-2 mock only)", underlying.Count())
	}
}

func TestFiltered_Exists(t *testing.T) {
	underlying := NewInMemoryMockStore()
	_ = underlying.Set(newWorkspaceMock("mock-1", "ws-1"))
	_ = underlying.Set(newWorkspaceMock("mock-2", "ws-2"))

	ws1 := NewFilteredMockStore(underlying, "ws-1")
	if !ws1.Exists("mock-1") {
		t.Error("ws-1 Exists(mock-1) = false, want true")
	}
	if ws1.Exists("mock-2") {
		t.Error("ws-1 Exists(mock-2) = true, want false (wrong workspace)")
	}
	if ws1.Exists("nonexistent") {
		t.Error("ws-1 Exists(nonexistent) = true, want false")
	}
}

func TestFiltered_ImplementsMockStore(t *testing.T) {
	var _ MockStore = (*FilteredMockStore)(nil)
}

func TestFiltered_Concurrent(t *testing.T) {
	underlying := NewInMemoryMockStore()
	ws1 := NewFilteredMockStore(underlying, "ws-1")
	ws2 := NewFilteredMockStore(underlying, "ws-2")

	const goroutines = 25
	const ops = 50
	var wg sync.WaitGroup

	// Concurrent writes to both workspaces
	for g := 0; g < goroutines; g++ {
		wg.Add(2)
		go func(g int) {
			defer wg.Done()
			for i := 0; i < ops; i++ {
				_ = ws1.Set(newMock(fmt.Sprintf("ws1-%d-%d", g, i), mock.TypeHTTP))
			}
		}(g)
		go func(g int) {
			defer wg.Done()
			for i := 0; i < ops; i++ {
				_ = ws2.Set(newMock(fmt.Sprintf("ws2-%d-%d", g, i), mock.TypeHTTP))
			}
		}(g)
	}
	wg.Wait()

	// Each workspace should see only its mocks
	ws1Count := ws1.Count()
	ws2Count := ws2.Count()
	total := underlying.Count()

	if ws1Count != goroutines*ops {
		t.Errorf("ws-1 Count() = %d, want %d", ws1Count, goroutines*ops)
	}
	if ws2Count != goroutines*ops {
		t.Errorf("ws-2 Count() = %d, want %d", ws2Count, goroutines*ops)
	}
	if total != goroutines*ops*2 {
		t.Errorf("underlying Count() = %d, want %d", total, goroutines*ops*2)
	}

	// Concurrent reads from both workspaces
	for g := 0; g < goroutines; g++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			_ = ws1.List()
			_ = ws1.Count()
		}()
		go func() {
			defer wg.Done()
			_ = ws2.List()
			_ = ws2.Count()
		}()
	}
	wg.Wait()
}

// --- Edge Cases ---

func TestInMemory_EmptyStringID(t *testing.T) {
	store := NewInMemoryMockStore()
	m := newMock("", mock.TypeHTTP)
	if err := store.Set(m); err != nil {
		t.Fatalf("Set(empty ID) error = %v", err)
	}
	got := store.Get("")
	if got == nil {
		t.Error("Get(empty ID) = nil, want non-nil")
	}
	if store.Count() != 1 {
		t.Errorf("Count() = %d, want 1", store.Count())
	}
}

func TestInMemory_ClearThenReuse(t *testing.T) {
	store := NewInMemoryMockStore()
	_ = store.Set(newMock("a", mock.TypeHTTP))
	store.Clear()
	_ = store.Set(newMock("b", mock.TypeHTTP))

	if store.Count() != 1 {
		t.Errorf("Count() = %d, want 1", store.Count())
	}
	if store.Get("a") != nil {
		t.Error("Get(a) after Clear() should be nil")
	}
	if store.Get("b") == nil {
		t.Error("Get(b) after re-add should not be nil")
	}
}

func TestFiltered_TwoWorkspacesIsolated(t *testing.T) {
	underlying := NewInMemoryMockStore()
	ws1 := NewFilteredMockStore(underlying, "ws-1")
	ws2 := NewFilteredMockStore(underlying, "ws-2")

	// Add mock via ws-1
	_ = ws1.Set(newMock("shared-id", mock.TypeHTTP))

	// ws-2 should NOT see it
	if ws2.Get("shared-id") != nil {
		t.Error("ws-2 can see ws-1's mock — isolation broken")
	}

	// Add mock with same ID via ws-2 — this overwrites in underlying but with ws-2 stamp
	_ = ws2.Set(newMock("shared-id", mock.TypeHTTP))

	// Now ws-2 should see it, ws-1 should NOT (workspace was overwritten)
	if ws2.Get("shared-id") == nil {
		t.Error("ws-2 cannot see its own mock")
	}
	if ws1.Get("shared-id") != nil {
		t.Error("ws-1 can still see mock after ws-2 overwrote it")
	}
}

func TestFiltered_ClearEmptyWorkspace(t *testing.T) {
	underlying := NewInMemoryMockStore()
	_ = underlying.Set(newWorkspaceMock("mock-1", "ws-1"))

	ws2 := NewFilteredMockStore(underlying, "ws-2")
	ws2.Clear() // should be a no-op

	if underlying.Count() != 1 {
		t.Errorf("underlying Count() = %d, want 1 after empty workspace Clear()", underlying.Count())
	}
}
