package file

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/getmockd/mockd/pkg/config"
	"github.com/getmockd/mockd/pkg/mock"
	"github.com/getmockd/mockd/pkg/store"
)

// ============================================================================
// Test Helpers
// ============================================================================

// newTestStore creates a FileStore backed by a temp directory.
// It opens the store and registers cleanup to close it and remove the dir.
func newTestStore(t *testing.T) *FileStore {
	t.Helper()
	dir := t.TempDir()
	fs := New(store.Config{
		DataDir:   dir,
		ConfigDir: filepath.Join(dir, "config"),
		CacheDir:  filepath.Join(dir, "cache"),
		StateDir:  filepath.Join(dir, "state"),
	})
	if err := fs.Open(context.Background()); err != nil {
		t.Fatalf("Open() failed: %v", err)
	}
	t.Cleanup(func() { _ = fs.Close() })
	return fs
}

// newReadOnlyStore creates a read-only FileStore.
func newReadOnlyStore(t *testing.T) *FileStore {
	t.Helper()
	dir := t.TempDir()
	fs := New(store.Config{
		DataDir:   dir,
		ConfigDir: filepath.Join(dir, "config"),
		CacheDir:  filepath.Join(dir, "cache"),
		StateDir:  filepath.Join(dir, "state"),
		ReadOnly:  true,
	})
	if err := fs.Open(context.Background()); err != nil {
		t.Fatalf("Open() failed: %v", err)
	}
	t.Cleanup(func() { _ = fs.Close() })
	return fs
}

func boolPtr(b bool) *bool    { return &b }
func strPtr(s string) *string { return &s }

func makeMock(id, name string, mockType mock.Type) *mock.Mock {
	return &mock.Mock{
		ID:   id,
		Name: name,
		Type: mockType,
	}
}

// ============================================================================
// FileStore Core — Open / Close / Save / Persistence
// ============================================================================

func TestFileStore_Open_CreatesDirectories(t *testing.T) {
	dir := t.TempDir()
	subDirs := map[string]string{
		"data":   filepath.Join(dir, "data"),
		"config": filepath.Join(dir, "config"),
		"cache":  filepath.Join(dir, "cache"),
		"state":  filepath.Join(dir, "state"),
	}
	fs := New(store.Config{
		DataDir:   subDirs["data"],
		ConfigDir: subDirs["config"],
		CacheDir:  subDirs["cache"],
		StateDir:  subDirs["state"],
	})
	defer func() { _ = fs.Close() }()

	if err := fs.Open(context.Background()); err != nil {
		t.Fatalf("Open() failed: %v", err)
	}

	for name, path := range subDirs {
		info, err := os.Stat(path)
		if err != nil {
			t.Errorf("directory %s not created: %v", name, err)
			continue
		}
		if !info.IsDir() {
			t.Errorf("expected %s to be a directory", name)
		}
	}
}

func TestFileStore_Open_NoDataFile_StartsFresh(t *testing.T) {
	fs := newTestStore(t)
	mocks, err := fs.Mocks().List(context.Background(), nil)
	if err != nil {
		t.Fatalf("List() failed: %v", err)
	}
	if len(mocks) != 0 {
		t.Errorf("expected 0 mocks, got %d", len(mocks))
	}
}

func TestFileStore_Open_LoadsExistingData(t *testing.T) {
	dir := t.TempDir()
	cfg := store.Config{
		DataDir:   dir,
		ConfigDir: filepath.Join(dir, "config"),
		CacheDir:  filepath.Join(dir, "cache"),
		StateDir:  filepath.Join(dir, "state"),
	}

	// Write initial data
	data := storeData{
		Version: 1,
		Mocks: []*mock.Mock{
			{ID: "m1", Name: "Test Mock", Type: mock.TypeHTTP},
		},
	}
	raw, _ := json.Marshal(data)
	os.MkdirAll(dir, 0700)
	os.WriteFile(filepath.Join(dir, "data.json"), raw, 0600)

	// Open store and verify data loaded
	fs := New(cfg)
	defer func() { _ = fs.Close() }()
	if err := fs.Open(context.Background()); err != nil {
		t.Fatalf("Open() failed: %v", err)
	}

	m, err := fs.Mocks().Get(context.Background(), "m1")
	if err != nil {
		t.Fatalf("Get() failed: %v", err)
	}
	if m.Name != "Test Mock" {
		t.Errorf("expected name 'Test Mock', got %q", m.Name)
	}
}

func TestFileStore_Open_InvalidJSON_ReturnsError(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "data.json"), []byte("{invalid"), 0600)

	fs := New(store.Config{DataDir: dir, ConfigDir: dir, CacheDir: dir, StateDir: dir})
	defer func() { _ = fs.Close() }()

	err := fs.Open(context.Background())
	if err == nil {
		t.Fatal("expected error for invalid JSON, got nil")
	}
}

func TestFileStore_ForceSave_PersistsToDisk(t *testing.T) {
	dir := t.TempDir()
	cfg := store.Config{
		DataDir:   dir,
		ConfigDir: filepath.Join(dir, "config"),
		CacheDir:  filepath.Join(dir, "cache"),
		StateDir:  filepath.Join(dir, "state"),
	}
	fs := New(cfg)
	defer func() { _ = fs.Close() }()
	if err := fs.Open(context.Background()); err != nil {
		t.Fatalf("Open() failed: %v", err)
	}

	// Create a mock
	ctx := context.Background()
	err := fs.Mocks().Create(ctx, &mock.Mock{ID: "save-test", Name: "SaveTest", Type: mock.TypeHTTP})
	if err != nil {
		t.Fatalf("Create() failed: %v", err)
	}

	// Force save
	if err := fs.ForceSave(); err != nil {
		t.Fatalf("ForceSave() failed: %v", err)
	}

	// Verify file exists and contains the mock
	raw, err := os.ReadFile(filepath.Join(dir, "data.json"))
	if err != nil {
		t.Fatalf("read data.json: %v", err)
	}
	var saved storeData
	if err := json.Unmarshal(raw, &saved); err != nil {
		t.Fatalf("unmarshal data.json: %v", err)
	}
	if len(saved.Mocks) != 1 || saved.Mocks[0].ID != "save-test" {
		t.Errorf("expected 1 mock with ID 'save-test', got %+v", saved.Mocks)
	}
	if saved.Version != dataVersion {
		t.Errorf("expected version %d, got %d", dataVersion, saved.Version)
	}
}

func TestFileStore_ForceSave_ReadOnly_ReturnsError(t *testing.T) {
	fs := newReadOnlyStore(t)
	err := fs.ForceSave()
	if !errors.Is(err, store.ErrReadOnly) {
		t.Errorf("expected ErrReadOnly, got %v", err)
	}
}

func TestFileStore_Close_MultipleCalls_Safe(t *testing.T) {
	fs := newTestStore(t)
	// Close is called by cleanup, but call it again explicitly
	if err := fs.Close(); err != nil {
		t.Fatalf("first Close() failed: %v", err)
	}
	// Second close should not panic (closeOnce protects)
	if err := fs.Close(); err != nil {
		t.Fatalf("second Close() failed: %v", err)
	}
}

func TestFileStore_Close_SavesDirtyData(t *testing.T) {
	dir := t.TempDir()
	cfg := store.Config{
		DataDir:   dir,
		ConfigDir: filepath.Join(dir, "config"),
		CacheDir:  filepath.Join(dir, "cache"),
		StateDir:  filepath.Join(dir, "state"),
	}
	fs := New(cfg)
	if err := fs.Open(context.Background()); err != nil {
		t.Fatalf("Open() failed: %v", err)
	}

	// Create a mock (makes dirty)
	ctx := context.Background()
	_ = fs.Mocks().Create(ctx, &mock.Mock{ID: "close-test", Name: "CloseTest", Type: mock.TypeHTTP})

	// Close (should trigger final save)
	if err := fs.Close(); err != nil {
		t.Fatalf("Close() failed: %v", err)
	}

	// Verify data persisted
	raw, err := os.ReadFile(filepath.Join(dir, "data.json"))
	if err != nil {
		t.Fatalf("read data.json: %v", err)
	}
	var saved storeData
	json.Unmarshal(raw, &saved)
	if len(saved.Mocks) != 1 {
		t.Errorf("expected 1 mock after close, got %d", len(saved.Mocks))
	}
}

func TestFileStore_DataDir(t *testing.T) {
	dir := t.TempDir()
	fs := New(store.Config{DataDir: dir})
	defer func() { _ = fs.Close() }()
	if fs.DataDir() != dir {
		t.Errorf("expected DataDir %q, got %q", dir, fs.DataDir())
	}
}

func TestFileStore_LastSyncTime(t *testing.T) {
	fs := newTestStore(t)
	if got := fs.LastSyncTime(); got != 0 {
		t.Errorf("expected LastSyncTime 0, got %d", got)
	}
}

func TestFileStore_Sync_NoOp(t *testing.T) {
	fs := newTestStore(t)
	if err := fs.Sync(context.Background()); err != nil {
		t.Errorf("Sync() should be no-op, got %v", err)
	}
}

func TestFileStore_ChangeListener(t *testing.T) {
	fs := newTestStore(t)
	ctx := context.Background()

	var received []store.ChangeEvent
	var mu sync.Mutex

	fs.AddChangeListener(func(event store.ChangeEvent) {
		mu.Lock()
		defer mu.Unlock()
		received = append(received, event)
	})

	_ = fs.Mocks().Create(ctx, &mock.Mock{ID: "listener-test", Name: "ListenerTest", Type: mock.TypeHTTP})

	// Wait a bit for async listener goroutine
	time.Sleep(50 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	if len(received) == 0 {
		t.Fatal("expected at least 1 change event, got 0")
	}
	if received[0].Collection != "mocks" || received[0].Operation != "create" {
		t.Errorf("expected mocks/create event, got %s/%s", received[0].Collection, received[0].Operation)
	}
}

func TestFileStore_ChangeListener_PanicRecovery(t *testing.T) {
	fs := newTestStore(t)
	ctx := context.Background()

	// Register a panicking listener — should not crash the store
	fs.AddChangeListener(func(event store.ChangeEvent) {
		panic("listener panic")
	})

	// This should not panic
	err := fs.Mocks().Create(ctx, &mock.Mock{ID: "panic-test", Name: "PanicTest", Type: mock.TypeHTTP})
	if err != nil {
		t.Fatalf("Create() failed: %v", err)
	}
	// Wait for goroutine to finish
	time.Sleep(50 * time.Millisecond)
}

// ============================================================================
// Transaction Tests
// ============================================================================

func TestTransaction_Commit_SavesToDisk(t *testing.T) {
	dir := t.TempDir()
	cfg := store.Config{
		DataDir:   dir,
		ConfigDir: filepath.Join(dir, "config"),
		CacheDir:  filepath.Join(dir, "cache"),
		StateDir:  filepath.Join(dir, "state"),
	}
	fs := New(cfg)
	defer func() { _ = fs.Close() }()
	if err := fs.Open(context.Background()); err != nil {
		t.Fatalf("Open() failed: %v", err)
	}
	ctx := context.Background()

	tx, err := fs.Begin(ctx)
	if err != nil {
		t.Fatalf("Begin() failed: %v", err)
	}

	_ = fs.Mocks().Create(ctx, &mock.Mock{ID: "tx-1", Name: "TxMock", Type: mock.TypeHTTP})

	if err := tx.Commit(); err != nil {
		t.Fatalf("Commit() failed: %v", err)
	}

	// Verify file persisted
	raw, _ := os.ReadFile(filepath.Join(dir, "data.json"))
	var saved storeData
	json.Unmarshal(raw, &saved)
	if len(saved.Mocks) != 1 {
		t.Errorf("expected 1 mock after commit, got %d", len(saved.Mocks))
	}
}

func TestTransaction_Rollback_DiscardsChanges(t *testing.T) {
	dir := t.TempDir()
	cfg := store.Config{
		DataDir:   dir,
		ConfigDir: filepath.Join(dir, "config"),
		CacheDir:  filepath.Join(dir, "cache"),
		StateDir:  filepath.Join(dir, "state"),
	}
	fs := New(cfg)
	defer func() { _ = fs.Close() }()
	if err := fs.Open(context.Background()); err != nil {
		t.Fatalf("Open() failed: %v", err)
	}
	ctx := context.Background()

	// Save initial state
	_ = fs.ForceSave()

	// Start transaction, create a mock
	tx, _ := fs.Begin(ctx)
	_ = fs.Mocks().Create(ctx, &mock.Mock{ID: "rollback-1", Name: "RollbackMock", Type: mock.TypeHTTP})

	// Rollback
	if err := tx.Rollback(); err != nil {
		t.Fatalf("Rollback() failed: %v", err)
	}

	// Mock should be gone
	mocks, _ := fs.Mocks().List(ctx, nil)
	if len(mocks) != 0 {
		t.Errorf("expected 0 mocks after rollback, got %d", len(mocks))
	}
}

// ============================================================================
// MockStore Tests
// ============================================================================

func TestMockStore_CRUD(t *testing.T) {
	fs := newTestStore(t)
	ctx := context.Background()
	ms := fs.Mocks()

	// Create
	m := &mock.Mock{ID: "http_1", Name: "Get Users", Type: mock.TypeHTTP}
	if err := ms.Create(ctx, m); err != nil {
		t.Fatalf("Create() failed: %v", err)
	}

	// Verify timestamps were set
	if m.CreatedAt.IsZero() {
		t.Error("CreatedAt not set")
	}
	if m.UpdatedAt.IsZero() {
		t.Error("UpdatedAt not set")
	}
	if m.MetaSortKey == 0 {
		t.Error("MetaSortKey not set")
	}
	if m.MetaSortKey >= 0 {
		t.Error("MetaSortKey should be negative (reverse chronological)")
	}

	// Get
	got, err := ms.Get(ctx, "http_1")
	if err != nil {
		t.Fatalf("Get() failed: %v", err)
	}
	if got.Name != "Get Users" {
		t.Errorf("expected name 'Get Users', got %q", got.Name)
	}

	// Update
	got.Name = "Get All Users"
	oldCreatedAt := got.CreatedAt
	if err := ms.Update(ctx, got); err != nil {
		t.Fatalf("Update() failed: %v", err)
	}
	updated, _ := ms.Get(ctx, "http_1")
	if updated.Name != "Get All Users" {
		t.Errorf("expected updated name, got %q", updated.Name)
	}
	if updated.CreatedAt != oldCreatedAt {
		t.Error("CreatedAt should be preserved on update")
	}

	// Delete
	if err := ms.Delete(ctx, "http_1"); err != nil {
		t.Fatalf("Delete() failed: %v", err)
	}
	_, err = ms.Get(ctx, "http_1")
	if !errors.Is(err, store.ErrNotFound) {
		t.Errorf("expected ErrNotFound after delete, got %v", err)
	}
}

func TestMockStore_Create_DuplicateID(t *testing.T) {
	fs := newTestStore(t)
	ctx := context.Background()
	ms := fs.Mocks()

	_ = ms.Create(ctx, &mock.Mock{ID: "dup", Name: "First", Type: mock.TypeHTTP})
	err := ms.Create(ctx, &mock.Mock{ID: "dup", Name: "Second", Type: mock.TypeHTTP})
	if !errors.Is(err, store.ErrAlreadyExists) {
		t.Errorf("expected ErrAlreadyExists, got %v", err)
	}
}

func TestMockStore_Create_PreservesExistingCreatedAt(t *testing.T) {
	fs := newTestStore(t)
	ctx := context.Background()
	ms := fs.Mocks()

	custom := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	m := &mock.Mock{ID: "custom-time", Name: "Custom", Type: mock.TypeHTTP, CreatedAt: custom}
	_ = ms.Create(ctx, m)

	if !m.CreatedAt.Equal(custom) {
		t.Errorf("expected CreatedAt %v, got %v", custom, m.CreatedAt)
	}
}

func TestMockStore_Create_PreservesExistingMetaSortKey(t *testing.T) {
	fs := newTestStore(t)
	ctx := context.Background()
	ms := fs.Mocks()

	m := &mock.Mock{ID: "custom-sort", Name: "Custom", Type: mock.TypeHTTP, MetaSortKey: 42.0}
	_ = ms.Create(ctx, m)

	if m.MetaSortKey != 42.0 {
		t.Errorf("expected MetaSortKey 42, got %f", m.MetaSortKey)
	}
}

func TestMockStore_Update_PreservesCreatedAtWhenZero(t *testing.T) {
	fs := newTestStore(t)
	ctx := context.Background()
	ms := fs.Mocks()

	_ = ms.Create(ctx, &mock.Mock{ID: "m1", Name: "Original", Type: mock.TypeHTTP})
	original, _ := ms.Get(ctx, "m1")

	// Update with zero CreatedAt — should preserve original
	updated := &mock.Mock{ID: "m1", Name: "Updated", Type: mock.TypeHTTP}
	_ = ms.Update(ctx, updated)

	got, _ := ms.Get(ctx, "m1")
	if got.CreatedAt != original.CreatedAt {
		t.Errorf("CreatedAt should be preserved, got %v (expected %v)", got.CreatedAt, original.CreatedAt)
	}
}

func TestMockStore_Update_NotFound(t *testing.T) {
	fs := newTestStore(t)
	ctx := context.Background()
	err := fs.Mocks().Update(ctx, &mock.Mock{ID: "nonexistent"})
	if !errors.Is(err, store.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestMockStore_Get_NotFound(t *testing.T) {
	fs := newTestStore(t)
	ctx := context.Background()
	_, err := fs.Mocks().Get(ctx, "nonexistent")
	if !errors.Is(err, store.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestMockStore_Delete_NotFound(t *testing.T) {
	fs := newTestStore(t)
	ctx := context.Background()
	err := fs.Mocks().Delete(ctx, "nonexistent")
	if !errors.Is(err, store.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestMockStore_List_EmptyStore(t *testing.T) {
	fs := newTestStore(t)
	ctx := context.Background()
	mocks, err := fs.Mocks().List(ctx, nil)
	if err != nil {
		t.Fatalf("List() failed: %v", err)
	}
	if mocks == nil {
		t.Error("List() should return empty slice, not nil")
	}
	if len(mocks) != 0 {
		t.Errorf("expected 0 mocks, got %d", len(mocks))
	}
}

func TestMockStore_List_ReturnsCopy(t *testing.T) {
	fs := newTestStore(t)
	ctx := context.Background()
	ms := fs.Mocks()

	_ = ms.Create(ctx, &mock.Mock{ID: "m1", Name: "Mock1", Type: mock.TypeHTTP})
	_ = ms.Create(ctx, &mock.Mock{ID: "m2", Name: "Mock2", Type: mock.TypeHTTP})

	list1, _ := ms.List(ctx, nil)
	list2, _ := ms.List(ctx, nil)

	// Modifying list1 should not affect list2 or the store
	list1[0] = nil
	if list2[0] == nil {
		t.Error("List() should return a copy, not reference to internal slice")
	}
}

func TestMockStore_List_FilterByType(t *testing.T) {
	fs := newTestStore(t)
	ctx := context.Background()
	ms := fs.Mocks()

	_ = ms.Create(ctx, &mock.Mock{ID: "h1", Name: "HTTP", Type: mock.TypeHTTP})
	_ = ms.Create(ctx, &mock.Mock{ID: "g1", Name: "GraphQL", Type: mock.TypeGraphQL})
	_ = ms.Create(ctx, &mock.Mock{ID: "h2", Name: "HTTP2", Type: mock.TypeHTTP})

	result, _ := ms.List(ctx, &store.MockFilter{Type: mock.TypeHTTP})
	if len(result) != 2 {
		t.Errorf("expected 2 HTTP mocks, got %d", len(result))
	}

	result, _ = ms.List(ctx, &store.MockFilter{Type: mock.TypeGraphQL})
	if len(result) != 1 {
		t.Errorf("expected 1 GraphQL mock, got %d", len(result))
	}
}

func TestMockStore_List_FilterByWorkspaceID(t *testing.T) {
	fs := newTestStore(t)
	ctx := context.Background()
	ms := fs.Mocks()

	m1 := &mock.Mock{ID: "w1", Name: "WS1", Type: mock.TypeHTTP, WorkspaceID: "ws-a"}
	m2 := &mock.Mock{ID: "w2", Name: "WS2", Type: mock.TypeHTTP, WorkspaceID: "ws-b"}
	_ = ms.Create(ctx, m1)
	_ = ms.Create(ctx, m2)

	result, _ := ms.List(ctx, &store.MockFilter{WorkspaceID: "ws-a"})
	if len(result) != 1 {
		t.Errorf("expected 1 mock for ws-a, got %d", len(result))
	}
}

func TestMockStore_List_FilterByParentID(t *testing.T) {
	fs := newTestStore(t)
	ctx := context.Background()
	ms := fs.Mocks()

	root := &mock.Mock{ID: "root1", Name: "Root", Type: mock.TypeHTTP}
	child := &mock.Mock{ID: "child1", Name: "Child", Type: mock.TypeHTTP, ParentID: "fld_1"}
	_ = ms.Create(ctx, root)
	_ = ms.Create(ctx, child)

	// Root level only (ParentID == "")
	result, _ := ms.List(ctx, &store.MockFilter{ParentID: strPtr("")})
	if len(result) != 1 || result[0].ID != "root1" {
		t.Errorf("expected 1 root mock, got %d", len(result))
	}

	// Specific folder
	result, _ = ms.List(ctx, &store.MockFilter{ParentID: strPtr("fld_1")})
	if len(result) != 1 || result[0].ID != "child1" {
		t.Errorf("expected 1 child mock, got %d", len(result))
	}

	// nil ParentID = no filter
	result, _ = ms.List(ctx, &store.MockFilter{})
	if len(result) != 2 {
		t.Errorf("expected 2 mocks with no parent filter, got %d", len(result))
	}
}

func TestMockStore_List_FilterByEnabled(t *testing.T) {
	fs := newTestStore(t)
	ctx := context.Background()
	ms := fs.Mocks()

	enabled := &mock.Mock{ID: "e1", Name: "Enabled", Type: mock.TypeHTTP, Enabled: boolPtr(true)}
	disabled := &mock.Mock{ID: "d1", Name: "Disabled", Type: mock.TypeHTTP, Enabled: boolPtr(false)}
	nilEnabled := &mock.Mock{ID: "n1", Name: "NilEnabled", Type: mock.TypeHTTP} // nil Enabled = treated as true
	_ = ms.Create(ctx, enabled)
	_ = ms.Create(ctx, disabled)
	_ = ms.Create(ctx, nilEnabled)

	result, _ := ms.List(ctx, &store.MockFilter{Enabled: boolPtr(true)})
	if len(result) != 2 { // e1 and n1
		t.Errorf("expected 2 enabled mocks, got %d", len(result))
	}

	result, _ = ms.List(ctx, &store.MockFilter{Enabled: boolPtr(false)})
	if len(result) != 1 { // d1
		t.Errorf("expected 1 disabled mock, got %d", len(result))
	}
}

func TestMockStore_List_FilterBySearch(t *testing.T) {
	fs := newTestStore(t)
	ctx := context.Background()
	ms := fs.Mocks()

	_ = ms.Create(ctx, &mock.Mock{ID: "s1", Name: "User API", Type: mock.TypeHTTP})
	_ = ms.Create(ctx, &mock.Mock{ID: "s2", Name: "Product API", Type: mock.TypeHTTP})
	_ = ms.Create(ctx, &mock.Mock{ID: "s3", Name: "Order Service", Type: mock.TypeHTTP})

	result, _ := ms.List(ctx, &store.MockFilter{Search: "api"})
	if len(result) != 2 {
		t.Errorf("expected 2 mocks matching 'api', got %d", len(result))
	}

	result, _ = ms.List(ctx, &store.MockFilter{Search: "ORDER"})
	if len(result) != 1 {
		t.Errorf("expected 1 mock matching 'ORDER' (case-insensitive), got %d", len(result))
	}
}

func TestMockStore_List_CombinedFilters(t *testing.T) {
	fs := newTestStore(t)
	ctx := context.Background()
	ms := fs.Mocks()

	_ = ms.Create(ctx, &mock.Mock{ID: "cf1", Name: "HTTP Enabled", Type: mock.TypeHTTP, Enabled: boolPtr(true), WorkspaceID: "ws-a"})
	_ = ms.Create(ctx, &mock.Mock{ID: "cf2", Name: "HTTP Disabled", Type: mock.TypeHTTP, Enabled: boolPtr(false), WorkspaceID: "ws-a"})
	_ = ms.Create(ctx, &mock.Mock{ID: "cf3", Name: "GQL Enabled", Type: mock.TypeGraphQL, Enabled: boolPtr(true), WorkspaceID: "ws-a"})
	_ = ms.Create(ctx, &mock.Mock{ID: "cf4", Name: "HTTP Enabled B", Type: mock.TypeHTTP, Enabled: boolPtr(true), WorkspaceID: "ws-b"})

	result, _ := ms.List(ctx, &store.MockFilter{
		Type:        mock.TypeHTTP,
		Enabled:     boolPtr(true),
		WorkspaceID: "ws-a",
	})
	if len(result) != 1 || result[0].ID != "cf1" {
		t.Errorf("expected 1 mock (cf1), got %d", len(result))
	}
}

func TestMockStore_DeleteByType(t *testing.T) {
	fs := newTestStore(t)
	ctx := context.Background()
	ms := fs.Mocks()

	_ = ms.Create(ctx, &mock.Mock{ID: "h1", Name: "HTTP", Type: mock.TypeHTTP})
	_ = ms.Create(ctx, &mock.Mock{ID: "g1", Name: "GQL", Type: mock.TypeGraphQL})
	_ = ms.Create(ctx, &mock.Mock{ID: "h2", Name: "HTTP2", Type: mock.TypeHTTP})

	err := ms.DeleteByType(ctx, mock.TypeHTTP)
	if err != nil {
		t.Fatalf("DeleteByType() failed: %v", err)
	}

	mocks, _ := ms.List(ctx, nil)
	if len(mocks) != 1 || mocks[0].ID != "g1" {
		t.Errorf("expected only GQL mock remaining, got %+v", mocks)
	}
}

func TestMockStore_DeleteByType_NoMatch(t *testing.T) {
	fs := newTestStore(t)
	ctx := context.Background()
	ms := fs.Mocks()

	_ = ms.Create(ctx, &mock.Mock{ID: "h1", Name: "HTTP", Type: mock.TypeHTTP})
	err := ms.DeleteByType(ctx, mock.TypeGraphQL)
	if err != nil {
		t.Fatalf("DeleteByType() with no matches should succeed, got %v", err)
	}
	mocks, _ := ms.List(ctx, nil)
	if len(mocks) != 1 {
		t.Errorf("expected 1 mock remaining, got %d", len(mocks))
	}
}

func TestMockStore_DeleteAll(t *testing.T) {
	fs := newTestStore(t)
	ctx := context.Background()
	ms := fs.Mocks()

	_ = ms.Create(ctx, &mock.Mock{ID: "h1", Type: mock.TypeHTTP})
	_ = ms.Create(ctx, &mock.Mock{ID: "g1", Type: mock.TypeGraphQL})

	if err := ms.DeleteAll(ctx); err != nil {
		t.Fatalf("DeleteAll() failed: %v", err)
	}

	count, _ := ms.Count(ctx, "")
	if count != 0 {
		t.Errorf("expected 0 mocks after DeleteAll, got %d", count)
	}
}

func TestMockStore_Count(t *testing.T) {
	fs := newTestStore(t)
	ctx := context.Background()
	ms := fs.Mocks()

	_ = ms.Create(ctx, &mock.Mock{ID: "h1", Type: mock.TypeHTTP})
	_ = ms.Create(ctx, &mock.Mock{ID: "g1", Type: mock.TypeGraphQL})
	_ = ms.Create(ctx, &mock.Mock{ID: "h2", Type: mock.TypeHTTP})

	total, _ := ms.Count(ctx, "")
	if total != 3 {
		t.Errorf("expected total 3, got %d", total)
	}

	httpCount, _ := ms.Count(ctx, mock.TypeHTTP)
	if httpCount != 2 {
		t.Errorf("expected 2 HTTP, got %d", httpCount)
	}

	gqlCount, _ := ms.Count(ctx, mock.TypeGraphQL)
	if gqlCount != 1 {
		t.Errorf("expected 1 GraphQL, got %d", gqlCount)
	}

	wsCount, _ := ms.Count(ctx, mock.TypeWebSocket)
	if wsCount != 0 {
		t.Errorf("expected 0 WebSocket, got %d", wsCount)
	}
}

func TestMockStore_BulkCreate(t *testing.T) {
	fs := newTestStore(t)
	ctx := context.Background()
	ms := fs.Mocks()

	mocks := []*mock.Mock{
		{ID: "b1", Name: "Bulk1", Type: mock.TypeHTTP},
		{ID: "b2", Name: "Bulk2", Type: mock.TypeHTTP},
		{ID: "b3", Name: "Bulk3", Type: mock.TypeGraphQL},
	}
	if err := ms.BulkCreate(ctx, mocks); err != nil {
		t.Fatalf("BulkCreate() failed: %v", err)
	}

	count, _ := ms.Count(ctx, "")
	if count != 3 {
		t.Errorf("expected 3 mocks, got %d", count)
	}

	// Verify timestamps set
	for _, m := range mocks {
		if m.CreatedAt.IsZero() {
			t.Errorf("mock %s: CreatedAt not set", m.ID)
		}
	}
}

func TestMockStore_BulkCreate_DuplicateExisting(t *testing.T) {
	fs := newTestStore(t)
	ctx := context.Background()
	ms := fs.Mocks()

	_ = ms.Create(ctx, &mock.Mock{ID: "existing", Type: mock.TypeHTTP})

	err := ms.BulkCreate(ctx, []*mock.Mock{
		{ID: "new1", Type: mock.TypeHTTP},
		{ID: "existing", Type: mock.TypeHTTP}, // duplicate
	})
	if !errors.Is(err, store.ErrAlreadyExists) {
		t.Errorf("expected ErrAlreadyExists, got %v", err)
	}
}

func TestMockStore_BulkCreate_DuplicateWithinBatch(t *testing.T) {
	fs := newTestStore(t)
	ctx := context.Background()
	ms := fs.Mocks()

	err := ms.BulkCreate(ctx, []*mock.Mock{
		{ID: "dup", Name: "First", Type: mock.TypeHTTP},
		{ID: "dup", Name: "Second", Type: mock.TypeHTTP}, // duplicate within batch
	})
	if !errors.Is(err, store.ErrAlreadyExists) {
		t.Errorf("expected ErrAlreadyExists for intra-batch duplicate, got %v", err)
	}
}

func TestMockStore_BulkUpdate(t *testing.T) {
	fs := newTestStore(t)
	ctx := context.Background()
	ms := fs.Mocks()

	_ = ms.BulkCreate(ctx, []*mock.Mock{
		{ID: "u1", Name: "Original1", Type: mock.TypeHTTP},
		{ID: "u2", Name: "Original2", Type: mock.TypeHTTP},
	})

	err := ms.BulkUpdate(ctx, []*mock.Mock{
		{ID: "u1", Name: "Updated1", Type: mock.TypeHTTP},
		{ID: "u2", Name: "Updated2", Type: mock.TypeHTTP},
	})
	if err != nil {
		t.Fatalf("BulkUpdate() failed: %v", err)
	}

	m1, _ := ms.Get(ctx, "u1")
	if m1.Name != "Updated1" {
		t.Errorf("expected 'Updated1', got %q", m1.Name)
	}
}

func TestMockStore_BulkUpdate_NotFound(t *testing.T) {
	fs := newTestStore(t)
	ctx := context.Background()

	err := fs.Mocks().BulkUpdate(ctx, []*mock.Mock{
		{ID: "nonexistent", Name: "Ghost", Type: mock.TypeHTTP},
	})
	if !errors.Is(err, store.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestMockStore_ReadOnly(t *testing.T) {
	fs := newReadOnlyStore(t)
	ctx := context.Background()
	ms := fs.Mocks()

	tests := []struct {
		name string
		fn   func() error
	}{
		{"Create", func() error { return ms.Create(ctx, &mock.Mock{ID: "x", Type: mock.TypeHTTP}) }},
		{"Update", func() error { return ms.Update(ctx, &mock.Mock{ID: "x", Type: mock.TypeHTTP}) }},
		{"Delete", func() error { return ms.Delete(ctx, "x") }},
		{"DeleteByType", func() error { return ms.DeleteByType(ctx, mock.TypeHTTP) }},
		{"DeleteAll", func() error { return ms.DeleteAll(ctx) }},
		{"BulkCreate", func() error { return ms.BulkCreate(ctx, []*mock.Mock{{ID: "x", Type: mock.TypeHTTP}}) }},
		{"BulkUpdate", func() error { return ms.BulkUpdate(ctx, []*mock.Mock{{ID: "x", Type: mock.TypeHTTP}}) }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.fn(); !errors.Is(err, store.ErrReadOnly) {
				t.Errorf("expected ErrReadOnly, got %v", err)
			}
		})
	}
}

// ============================================================================
// WorkspaceStore Tests
// ============================================================================

func TestWorkspaceStore_DefaultWorkspace(t *testing.T) {
	fs := newTestStore(t)
	ctx := context.Background()
	ws := fs.Workspaces()

	// List should auto-create default workspace
	workspaces, err := ws.List(ctx)
	if err != nil {
		t.Fatalf("List() failed: %v", err)
	}
	if len(workspaces) == 0 {
		t.Fatal("expected at least 1 workspace (default)")
	}
	if workspaces[0].ID != store.DefaultWorkspaceID {
		t.Errorf("expected first workspace to be default, got %q", workspaces[0].ID)
	}
	if workspaces[0].Name != "Default" {
		t.Errorf("expected name 'Default', got %q", workspaces[0].Name)
	}
}

func TestWorkspaceStore_Get_DefaultWorkspace(t *testing.T) {
	fs := newTestStore(t)
	ctx := context.Background()

	ws, err := fs.Workspaces().Get(ctx, store.DefaultWorkspaceID)
	if err != nil {
		t.Fatalf("Get() failed: %v", err)
	}
	if ws.ID != store.DefaultWorkspaceID {
		t.Errorf("expected ID %q, got %q", store.DefaultWorkspaceID, ws.ID)
	}
}

func TestWorkspaceStore_CRUD(t *testing.T) {
	fs := newTestStore(t)
	ctx := context.Background()
	ws := fs.Workspaces()

	// Create
	custom := &store.Workspace{
		ID:          "custom-ws",
		Name:        "Custom Workspace",
		Type:        store.WorkspaceTypeCloud,
		Description: "A custom workspace",
	}
	if err := ws.Create(ctx, custom); err != nil {
		t.Fatalf("Create() failed: %v", err)
	}
	if custom.CreatedAt == 0 {
		t.Error("CreatedAt not set")
	}
	if custom.UpdatedAt == 0 {
		t.Error("UpdatedAt not set")
	}

	// Get
	got, err := ws.Get(ctx, "custom-ws")
	if err != nil {
		t.Fatalf("Get() failed: %v", err)
	}
	if got.Name != "Custom Workspace" {
		t.Errorf("expected name 'Custom Workspace', got %q", got.Name)
	}

	// Update
	got.Name = "Updated Workspace"
	if err := ws.Update(ctx, got); err != nil {
		t.Fatalf("Update() failed: %v", err)
	}
	updated, _ := ws.Get(ctx, "custom-ws")
	if updated.Name != "Updated Workspace" {
		t.Errorf("expected updated name, got %q", updated.Name)
	}

	// Delete
	if err := ws.Delete(ctx, "custom-ws"); err != nil {
		t.Fatalf("Delete() failed: %v", err)
	}
	_, err = ws.Get(ctx, "custom-ws")
	if !errors.Is(err, store.ErrNotFound) {
		t.Errorf("expected ErrNotFound after delete, got %v", err)
	}
}

func TestWorkspaceStore_Create_DuplicateID(t *testing.T) {
	fs := newTestStore(t)
	ctx := context.Background()
	ws := fs.Workspaces()

	_ = ws.Create(ctx, &store.Workspace{ID: "dup-ws", Name: "First"})
	err := ws.Create(ctx, &store.Workspace{ID: "dup-ws", Name: "Second"})
	if !errors.Is(err, store.ErrAlreadyExists) {
		t.Errorf("expected ErrAlreadyExists, got %v", err)
	}
}

func TestWorkspaceStore_Delete_DefaultWorkspace_Blocked(t *testing.T) {
	fs := newTestStore(t)
	ctx := context.Background()

	// Ensure default exists
	_, _ = fs.Workspaces().List(ctx)

	err := fs.Workspaces().Delete(ctx, store.DefaultWorkspaceID)
	if !errors.Is(err, store.ErrReadOnly) {
		t.Errorf("expected ErrReadOnly when deleting default workspace, got %v", err)
	}
}

func TestWorkspaceStore_Delete_NotFound(t *testing.T) {
	fs := newTestStore(t)
	ctx := context.Background()
	err := fs.Workspaces().Delete(ctx, "nonexistent")
	if !errors.Is(err, store.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestWorkspaceStore_Update_NotFound(t *testing.T) {
	fs := newTestStore(t)
	ctx := context.Background()
	err := fs.Workspaces().Update(ctx, &store.Workspace{ID: "nonexistent"})
	if !errors.Is(err, store.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestWorkspaceStore_ReadOnly(t *testing.T) {
	fs := newReadOnlyStore(t)
	ctx := context.Background()
	ws := fs.Workspaces()

	err := ws.Create(ctx, &store.Workspace{ID: "x"})
	if !errors.Is(err, store.ErrReadOnly) {
		t.Errorf("Create: expected ErrReadOnly, got %v", err)
	}
	err = ws.Update(ctx, &store.Workspace{ID: "x"})
	if !errors.Is(err, store.ErrReadOnly) {
		t.Errorf("Update: expected ErrReadOnly, got %v", err)
	}
	err = ws.Delete(ctx, "x")
	if !errors.Is(err, store.ErrReadOnly) {
		t.Errorf("Delete: expected ErrReadOnly, got %v", err)
	}
}

func TestWorkspaceStore_EnsureDefaultWorkspace_Concurrent(t *testing.T) {
	fs := newTestStore(t)
	ctx := context.Background()

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			workspaces, err := fs.Workspaces().List(ctx)
			if err != nil {
				t.Errorf("List() failed: %v", err)
				return
			}
			// Should always have at least the default
			found := false
			for _, w := range workspaces {
				if w.ID == store.DefaultWorkspaceID {
					found = true
				}
			}
			if !found {
				t.Error("default workspace not found")
			}
		}()
	}
	wg.Wait()

	// Should only have 1 default workspace, not multiple
	workspaces, _ := fs.Workspaces().List(ctx)
	count := 0
	for _, w := range workspaces {
		if w.ID == store.DefaultWorkspaceID {
			count++
		}
	}
	if count != 1 {
		t.Errorf("expected exactly 1 default workspace, got %d", count)
	}
}

// ============================================================================
// FolderStore Tests
// ============================================================================

func TestFolderStore_CRUD(t *testing.T) {
	fs := newTestStore(t)
	ctx := context.Background()
	fld := fs.Folders()

	folder := &config.Folder{ID: "fld_1", Name: "Test Folder"}
	if err := fld.Create(ctx, folder); err != nil {
		t.Fatalf("Create() failed: %v", err)
	}

	got, err := fld.Get(ctx, "fld_1")
	if err != nil {
		t.Fatalf("Get() failed: %v", err)
	}
	if got.Name != "Test Folder" {
		t.Errorf("expected 'Test Folder', got %q", got.Name)
	}

	got.Name = "Updated Folder"
	if err := fld.Update(ctx, got); err != nil {
		t.Fatalf("Update() failed: %v", err)
	}
	updated, _ := fld.Get(ctx, "fld_1")
	if updated.Name != "Updated Folder" {
		t.Errorf("expected 'Updated Folder', got %q", updated.Name)
	}

	if err := fld.Delete(ctx, "fld_1"); err != nil {
		t.Fatalf("Delete() failed: %v", err)
	}
	_, err = fld.Get(ctx, "fld_1")
	if !errors.Is(err, store.ErrNotFound) {
		t.Errorf("expected ErrNotFound after delete, got %v", err)
	}
}

func TestFolderStore_List_NoFilter(t *testing.T) {
	fs := newTestStore(t)
	ctx := context.Background()
	fld := fs.Folders()

	_ = fld.Create(ctx, &config.Folder{ID: "f1", Name: "F1"})
	_ = fld.Create(ctx, &config.Folder{ID: "f2", Name: "F2"})

	result, _ := fld.List(ctx, nil)
	if len(result) != 2 {
		t.Errorf("expected 2 folders, got %d", len(result))
	}
}

func TestFolderStore_List_FilterByWorkspaceID(t *testing.T) {
	fs := newTestStore(t)
	ctx := context.Background()
	fld := fs.Folders()

	_ = fld.Create(ctx, &config.Folder{
		ID:         "f1",
		Name:       "F1",
		EntityMeta: config.EntityMeta{WorkspaceID: "ws-a"},
	})
	_ = fld.Create(ctx, &config.Folder{
		ID:         "f2",
		Name:       "F2",
		EntityMeta: config.EntityMeta{WorkspaceID: "ws-b"},
	})
	_ = fld.Create(ctx, &config.Folder{
		ID:   "f3",
		Name: "F3 no ws",
		// Empty WorkspaceID — treated as "local" for backward compat
	})

	result, _ := fld.List(ctx, &store.FolderFilter{WorkspaceID: "ws-a"})
	if len(result) != 1 {
		t.Errorf("expected 1 folder for ws-a, got %d", len(result))
	}

	// Empty workspace ID treated as "local"
	result, _ = fld.List(ctx, &store.FolderFilter{WorkspaceID: store.DefaultWorkspaceID})
	if len(result) != 1 || result[0].ID != "f3" {
		t.Errorf("expected f3 (empty wsID mapped to 'local'), got %+v", result)
	}
}

func TestFolderStore_List_FilterByParentID(t *testing.T) {
	fs := newTestStore(t)
	ctx := context.Background()
	fld := fs.Folders()

	_ = fld.Create(ctx, &config.Folder{ID: "root1", Name: "Root"})
	_ = fld.Create(ctx, &config.Folder{
		ID:               "child1",
		Name:             "Child",
		OrganizationMeta: config.OrganizationMeta{ParentID: "root1"},
	})

	// Root level
	result, _ := fld.List(ctx, &store.FolderFilter{ParentID: strPtr("")})
	if len(result) != 1 || result[0].ID != "root1" {
		t.Errorf("expected 1 root folder, got %d", len(result))
	}

	// Children of root1
	result, _ = fld.List(ctx, &store.FolderFilter{ParentID: strPtr("root1")})
	if len(result) != 1 || result[0].ID != "child1" {
		t.Errorf("expected 1 child folder, got %d", len(result))
	}
}

func TestFolderStore_DeleteAll(t *testing.T) {
	fs := newTestStore(t)
	ctx := context.Background()
	fld := fs.Folders()

	_ = fld.Create(ctx, &config.Folder{ID: "f1", Name: "F1"})
	_ = fld.Create(ctx, &config.Folder{ID: "f2", Name: "F2"})

	if err := fld.DeleteAll(ctx); err != nil {
		t.Fatalf("DeleteAll() failed: %v", err)
	}
	result, _ := fld.List(ctx, nil)
	if len(result) != 0 {
		t.Errorf("expected 0 folders after DeleteAll, got %d", len(result))
	}
}

func TestFolderStore_ReadOnly(t *testing.T) {
	fs := newReadOnlyStore(t)
	ctx := context.Background()
	fld := fs.Folders()

	if err := fld.Create(ctx, &config.Folder{ID: "x"}); !errors.Is(err, store.ErrReadOnly) {
		t.Errorf("Create: expected ErrReadOnly, got %v", err)
	}
	if err := fld.Update(ctx, &config.Folder{ID: "x"}); !errors.Is(err, store.ErrReadOnly) {
		t.Errorf("Update: expected ErrReadOnly, got %v", err)
	}
	if err := fld.Delete(ctx, "x"); !errors.Is(err, store.ErrReadOnly) {
		t.Errorf("Delete: expected ErrReadOnly, got %v", err)
	}
	if err := fld.DeleteAll(ctx); !errors.Is(err, store.ErrReadOnly) {
		t.Errorf("DeleteAll: expected ErrReadOnly, got %v", err)
	}
}

// ============================================================================
// RecordingStore Tests
// ============================================================================

func TestRecordingStore_CRUD(t *testing.T) {
	fs := newTestStore(t)
	ctx := context.Background()
	rs := fs.Recordings()

	rec := &store.Recording{ID: "rec_1", Name: "Recording 1", Protocol: "http"}
	if err := rs.Create(ctx, rec); err != nil {
		t.Fatalf("Create() failed: %v", err)
	}

	got, err := rs.Get(ctx, "rec_1")
	if err != nil {
		t.Fatalf("Get() failed: %v", err)
	}
	if got.Name != "Recording 1" {
		t.Errorf("expected 'Recording 1', got %q", got.Name)
	}

	got.Name = "Updated Recording"
	if err := rs.Update(ctx, got); err != nil {
		t.Fatalf("Update() failed: %v", err)
	}
	updated, _ := rs.Get(ctx, "rec_1")
	if updated.Name != "Updated Recording" {
		t.Errorf("expected 'Updated Recording', got %q", updated.Name)
	}

	if err := rs.Delete(ctx, "rec_1"); err != nil {
		t.Fatalf("Delete() failed: %v", err)
	}
	_, err = rs.Get(ctx, "rec_1")
	if !errors.Is(err, store.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestRecordingStore_List(t *testing.T) {
	fs := newTestStore(t)
	ctx := context.Background()
	rs := fs.Recordings()

	_ = rs.Create(ctx, &store.Recording{ID: "r1"})
	_ = rs.Create(ctx, &store.Recording{ID: "r2"})

	result, _ := rs.List(ctx)
	if len(result) != 2 {
		t.Errorf("expected 2 recordings, got %d", len(result))
	}
}

func TestRecordingStore_DeleteAll(t *testing.T) {
	fs := newTestStore(t)
	ctx := context.Background()
	rs := fs.Recordings()

	_ = rs.Create(ctx, &store.Recording{ID: "r1"})
	if err := rs.DeleteAll(ctx); err != nil {
		t.Fatalf("DeleteAll() failed: %v", err)
	}
	result, _ := rs.List(ctx)
	if len(result) != 0 {
		t.Errorf("expected 0, got %d", len(result))
	}
}

func TestRecordingStore_Get_NotFound(t *testing.T) {
	fs := newTestStore(t)
	_, err := fs.Recordings().Get(context.Background(), "nope")
	if !errors.Is(err, store.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestRecordingStore_ReadOnly(t *testing.T) {
	fs := newReadOnlyStore(t)
	ctx := context.Background()
	rs := fs.Recordings()

	if err := rs.Create(ctx, &store.Recording{ID: "x"}); !errors.Is(err, store.ErrReadOnly) {
		t.Errorf("Create: expected ErrReadOnly, got %v", err)
	}
	if err := rs.Update(ctx, &store.Recording{ID: "x"}); !errors.Is(err, store.ErrReadOnly) {
		t.Errorf("Update: expected ErrReadOnly, got %v", err)
	}
	if err := rs.Delete(ctx, "x"); !errors.Is(err, store.ErrReadOnly) {
		t.Errorf("Delete: expected ErrReadOnly, got %v", err)
	}
	if err := rs.DeleteAll(ctx); !errors.Is(err, store.ErrReadOnly) {
		t.Errorf("DeleteAll: expected ErrReadOnly, got %v", err)
	}
}

// ============================================================================
// RequestLogStore Tests
// ============================================================================

func TestRequestLogStore_AppendAndList(t *testing.T) {
	fs := newTestStore(t)
	ctx := context.Background()
	rl := fs.RequestLog()

	for i := 0; i < 5; i++ {
		_ = rl.Append(ctx, &store.RequestLogEntry{
			ID:     "log_" + string(rune('a'+i)),
			Method: "GET",
			Path:   "/test",
		})
	}

	count, _ := rl.Count(ctx)
	if count != 5 {
		t.Errorf("expected 5 entries, got %d", count)
	}
}

func TestRequestLogStore_List_LimitOffset(t *testing.T) {
	fs := newTestStore(t)
	ctx := context.Background()
	rl := fs.RequestLog()

	for i := 0; i < 10; i++ {
		_ = rl.Append(ctx, &store.RequestLogEntry{ID: "log_" + string(rune('0'+i))})
	}

	// limit=3, offset=2
	result, _ := rl.List(ctx, 3, 2)
	if len(result) != 3 {
		t.Errorf("expected 3 entries, got %d", len(result))
	}

	// offset beyond length
	result, _ = rl.List(ctx, 10, 100)
	if len(result) != 0 {
		t.Errorf("expected 0 entries for offset beyond length, got %d", len(result))
	}

	// limit=0 means no limit
	result, _ = rl.List(ctx, 0, 0)
	if len(result) != 10 {
		t.Errorf("expected 10 entries with limit=0, got %d", len(result))
	}

	// negative limit means no limit
	result, _ = rl.List(ctx, -1, 0)
	if len(result) != 10 {
		t.Errorf("expected 10 entries with limit=-1, got %d", len(result))
	}
}

func TestRequestLogStore_Append_MaxEntries(t *testing.T) {
	fs := newTestStore(t)
	ctx := context.Background()
	rl := fs.RequestLog()

	// Fill to max (10,000)
	for i := 0; i < 10001; i++ {
		_ = rl.Append(ctx, &store.RequestLogEntry{ID: "log_" + string(rune(i))})
	}

	count, _ := rl.Count(ctx)
	if count != 10000 {
		t.Errorf("expected 10000 entries (capped), got %d", count)
	}
}

func TestRequestLogStore_Get(t *testing.T) {
	fs := newTestStore(t)
	ctx := context.Background()
	rl := fs.RequestLog()

	_ = rl.Append(ctx, &store.RequestLogEntry{ID: "find-me", Method: "POST", Path: "/api"})

	got, err := rl.Get(ctx, "find-me")
	if err != nil {
		t.Fatalf("Get() failed: %v", err)
	}
	if got.Method != "POST" {
		t.Errorf("expected POST, got %q", got.Method)
	}
}

func TestRequestLogStore_Get_NotFound(t *testing.T) {
	fs := newTestStore(t)
	_, err := fs.RequestLog().Get(context.Background(), "nope")
	if !errors.Is(err, store.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestRequestLogStore_Clear(t *testing.T) {
	fs := newTestStore(t)
	ctx := context.Background()
	rl := fs.RequestLog()

	_ = rl.Append(ctx, &store.RequestLogEntry{ID: "log1"})
	_ = rl.Append(ctx, &store.RequestLogEntry{ID: "log2"})

	if err := rl.Clear(ctx); err != nil {
		t.Fatalf("Clear() failed: %v", err)
	}
	count, _ := rl.Count(ctx)
	if count != 0 {
		t.Errorf("expected 0 after clear, got %d", count)
	}
}

func TestRequestLogStore_ReadOnly(t *testing.T) {
	fs := newReadOnlyStore(t)
	ctx := context.Background()
	rl := fs.RequestLog()

	if err := rl.Append(ctx, &store.RequestLogEntry{ID: "x"}); !errors.Is(err, store.ErrReadOnly) {
		t.Errorf("Append: expected ErrReadOnly, got %v", err)
	}
	if err := rl.Clear(ctx); !errors.Is(err, store.ErrReadOnly) {
		t.Errorf("Clear: expected ErrReadOnly, got %v", err)
	}
}

// ============================================================================
// PreferencesStore Tests
// ============================================================================

func TestPreferencesStore_DefaultValues(t *testing.T) {
	fs := newTestStore(t)
	ctx := context.Background()

	prefs, err := fs.Preferences().Get(ctx)
	if err != nil {
		t.Fatalf("Get() failed: %v", err)
	}
	if prefs.Theme != "system" {
		t.Errorf("expected theme 'system', got %q", prefs.Theme)
	}
	if !prefs.AutoScroll {
		t.Error("expected AutoScroll true")
	}
	if prefs.PollingInterval != 2000 {
		t.Errorf("expected PollingInterval 2000, got %d", prefs.PollingInterval)
	}
	if !prefs.MinimizeToTray {
		t.Error("expected MinimizeToTray true")
	}
	if prefs.DefaultMockPort != 4280 {
		t.Errorf("expected DefaultMockPort 4280, got %d", prefs.DefaultMockPort)
	}
	if prefs.DefaultAdminPort != 4290 {
		t.Errorf("expected DefaultAdminPort 4290, got %d", prefs.DefaultAdminPort)
	}
}

func TestPreferencesStore_SetAndGet(t *testing.T) {
	fs := newTestStore(t)
	ctx := context.Background()
	ps := fs.Preferences()

	custom := &store.Preferences{
		Theme:            "dark",
		AutoScroll:       false,
		PollingInterval:  5000,
		DefaultMockPort:  9090,
		DefaultAdminPort: 9091,
	}
	if err := ps.Set(ctx, custom); err != nil {
		t.Fatalf("Set() failed: %v", err)
	}

	got, _ := ps.Get(ctx)
	if got.Theme != "dark" {
		t.Errorf("expected theme 'dark', got %q", got.Theme)
	}
	if got.PollingInterval != 5000 {
		t.Errorf("expected 5000, got %d", got.PollingInterval)
	}
}

func TestPreferencesStore_ReadOnly(t *testing.T) {
	fs := newReadOnlyStore(t)
	ctx := context.Background()

	err := fs.Preferences().Set(ctx, &store.Preferences{Theme: "dark"})
	if !errors.Is(err, store.ErrReadOnly) {
		t.Errorf("expected ErrReadOnly, got %v", err)
	}
}

// ============================================================================
// StatefulResourceStore Tests
// ============================================================================

func TestStatefulResourceStore_CRUD(t *testing.T) {
	fs := newTestStore(t)
	ctx := context.Background()
	srs := fs.StatefulResources()

	res := &config.StatefulResourceConfig{
		Name:     "users",
		BasePath: "/api/users",
	}
	if err := srs.Create(ctx, res); err != nil {
		t.Fatalf("Create() failed: %v", err)
	}

	list, _ := srs.List(ctx)
	if len(list) != 1 {
		t.Errorf("expected 1, got %d", len(list))
	}
	if list[0].Name != "users" {
		t.Errorf("expected 'users', got %q", list[0].Name)
	}

	// Delete by name
	if err := srs.Delete(ctx, "users"); err != nil {
		t.Fatalf("Delete() failed: %v", err)
	}
	list, _ = srs.List(ctx)
	if len(list) != 0 {
		t.Errorf("expected 0 after delete, got %d", len(list))
	}
}

func TestStatefulResourceStore_Create_DuplicateName(t *testing.T) {
	fs := newTestStore(t)
	ctx := context.Background()
	srs := fs.StatefulResources()

	_ = srs.Create(ctx, &config.StatefulResourceConfig{Name: "users", BasePath: "/api/users"})
	err := srs.Create(ctx, &config.StatefulResourceConfig{Name: "users", BasePath: "/api/v2/users"})
	if !errors.Is(err, store.ErrAlreadyExists) {
		t.Errorf("expected ErrAlreadyExists, got %v", err)
	}
}

func TestStatefulResourceStore_Delete_NotFound(t *testing.T) {
	fs := newTestStore(t)
	ctx := context.Background()
	err := fs.StatefulResources().Delete(ctx, "nonexistent")
	if !errors.Is(err, store.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestStatefulResourceStore_DeleteAll(t *testing.T) {
	fs := newTestStore(t)
	ctx := context.Background()
	srs := fs.StatefulResources()

	_ = srs.Create(ctx, &config.StatefulResourceConfig{Name: "users"})
	_ = srs.Create(ctx, &config.StatefulResourceConfig{Name: "products"})

	if err := srs.DeleteAll(ctx); err != nil {
		t.Fatalf("DeleteAll() failed: %v", err)
	}
	list, _ := srs.List(ctx)
	if len(list) != 0 {
		t.Errorf("expected 0, got %d", len(list))
	}
}

func TestStatefulResourceStore_List_EmptyReturnsEmptySlice(t *testing.T) {
	fs := newTestStore(t)
	ctx := context.Background()
	list, err := fs.StatefulResources().List(ctx)
	if err != nil {
		t.Fatalf("List() failed: %v", err)
	}
	if list == nil {
		t.Error("List() should return empty slice, not nil")
	}
	if len(list) != 0 {
		t.Errorf("expected 0, got %d", len(list))
	}
}

func TestStatefulResourceStore_ReadOnly(t *testing.T) {
	fs := newReadOnlyStore(t)
	ctx := context.Background()
	srs := fs.StatefulResources()

	if err := srs.Create(ctx, &config.StatefulResourceConfig{Name: "x"}); !errors.Is(err, store.ErrReadOnly) {
		t.Errorf("Create: expected ErrReadOnly, got %v", err)
	}
	if err := srs.Delete(ctx, "x"); !errors.Is(err, store.ErrReadOnly) {
		t.Errorf("Delete: expected ErrReadOnly, got %v", err)
	}
	if err := srs.DeleteAll(ctx); !errors.Is(err, store.ErrReadOnly) {
		t.Errorf("DeleteAll: expected ErrReadOnly, got %v", err)
	}
}

// ============================================================================
// Persistence Round-Trip Tests (Save + Load)
// ============================================================================

func TestFileStore_Persistence_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	cfg := store.Config{
		DataDir:   dir,
		ConfigDir: filepath.Join(dir, "config"),
		CacheDir:  filepath.Join(dir, "cache"),
		StateDir:  filepath.Join(dir, "state"),
	}
	ctx := context.Background()

	// Session 1: Create data
	fs1 := New(cfg)
	_ = fs1.Open(ctx)
	_ = fs1.Mocks().Create(ctx, &mock.Mock{ID: "persist-1", Name: "Persistent", Type: mock.TypeHTTP})
	_ = fs1.Workspaces().Create(ctx, &store.Workspace{ID: "ws-persist", Name: "Persistent WS"})
	_ = fs1.Folders().Create(ctx, &config.Folder{ID: "fld-persist", Name: "Persistent Folder"})
	_ = fs1.Recordings().Create(ctx, &store.Recording{ID: "rec-persist", Protocol: "http"})
	_ = fs1.RequestLog().Append(ctx, &store.RequestLogEntry{ID: "log-persist", Method: "GET"})
	_ = fs1.StatefulResources().Create(ctx, &config.StatefulResourceConfig{Name: "persist-res", BasePath: "/api/test"})
	_ = fs1.Preferences().Set(ctx, &store.Preferences{Theme: "dark", DefaultMockPort: 4280})
	_ = fs1.Close()

	// Session 2: Verify data survived
	fs2 := New(cfg)
	if err := fs2.Open(ctx); err != nil {
		t.Fatalf("Session 2 Open() failed: %v", err)
	}
	defer func() { _ = fs2.Close() }()

	// Mock
	m, err := fs2.Mocks().Get(ctx, "persist-1")
	if err != nil {
		t.Errorf("mock not persisted: %v", err)
	} else if m.Name != "Persistent" {
		t.Errorf("mock name wrong: %q", m.Name)
	}

	// Workspace
	ws, err := fs2.Workspaces().Get(ctx, "ws-persist")
	if err != nil {
		t.Errorf("workspace not persisted: %v", err)
	} else if ws.Name != "Persistent WS" {
		t.Errorf("workspace name wrong: %q", ws.Name)
	}

	// Folder
	fld, err := fs2.Folders().Get(ctx, "fld-persist")
	if err != nil {
		t.Errorf("folder not persisted: %v", err)
	} else if fld.Name != "Persistent Folder" {
		t.Errorf("folder name wrong: %q", fld.Name)
	}

	// Recording
	rec, err := fs2.Recordings().Get(ctx, "rec-persist")
	if err != nil {
		t.Errorf("recording not persisted: %v", err)
	} else if rec.Protocol != "http" {
		t.Errorf("recording protocol wrong: %q", rec.Protocol)
	}

	// Request log
	logEntry, err := fs2.RequestLog().Get(ctx, "log-persist")
	if err != nil {
		t.Errorf("request log not persisted: %v", err)
	} else if logEntry.Method != "GET" {
		t.Errorf("log method wrong: %q", logEntry.Method)
	}

	// Stateful resources
	srs, _ := fs2.StatefulResources().List(ctx)
	if len(srs) != 1 || srs[0].Name != "persist-res" {
		t.Errorf("stateful resources not persisted correctly: %+v", srs)
	}

	// Preferences
	prefs, _ := fs2.Preferences().Get(ctx)
	if prefs.Theme != "dark" {
		t.Errorf("preferences not persisted: theme=%q", prefs.Theme)
	}
}

// ============================================================================
// Concurrency Tests
// ============================================================================

func TestMockStore_ConcurrentCRUD(t *testing.T) {
	fs := newTestStore(t)
	ctx := context.Background()
	ms := fs.Mocks()

	var wg sync.WaitGroup

	// Concurrent creates
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			id := "conc_" + string(rune('A'+i%26)) + string(rune('0'+i/26))
			_ = ms.Create(ctx, &mock.Mock{ID: id, Name: "Concurrent", Type: mock.TypeHTTP})
		}(i)
	}
	wg.Wait()

	count, _ := ms.Count(ctx, "")
	if count != 50 {
		t.Errorf("expected 50 mocks after concurrent creates, got %d", count)
	}

	// Concurrent reads + writes
	for i := 0; i < 50; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			_, _ = ms.List(ctx, nil)
		}()
		go func() {
			defer wg.Done()
			_, _ = ms.Count(ctx, "")
		}()
	}
	wg.Wait()
}

// ============================================================================
// Atomic Write Test
// ============================================================================

func TestFileStore_AtomicWrite_NoTempFileLeftOver(t *testing.T) {
	dir := t.TempDir()
	cfg := store.Config{
		DataDir:   dir,
		ConfigDir: filepath.Join(dir, "config"),
		CacheDir:  filepath.Join(dir, "cache"),
		StateDir:  filepath.Join(dir, "state"),
	}
	fs := New(cfg)
	defer func() { _ = fs.Close() }()
	_ = fs.Open(context.Background())

	_ = fs.Mocks().Create(context.Background(), &mock.Mock{ID: "atomic", Type: mock.TypeHTTP})
	_ = fs.ForceSave()

	// Verify no .tmp file left
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if filepath.Ext(e.Name()) == ".tmp" {
			t.Errorf("temp file left over: %s", e.Name())
		}
	}

	// Verify data.json exists
	if _, err := os.Stat(filepath.Join(dir, "data.json")); err != nil {
		t.Errorf("data.json not found: %v", err)
	}
}
