package file

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/getmockd/mockd/pkg/config"
	"github.com/getmockd/mockd/pkg/store"
)

// ============================================================================
// CustomOperationStore — Dedicated Test File
//
// This file extends the basic CRUD tests in store_test.go with more granular
// tests for persistence, multi-step operations, and edge cases.
// ============================================================================

func TestCustomOperationStore_CreateAndGet(t *testing.T) {
	fs := newTestStore(t)
	ctx := context.Background()
	cos := fs.CustomOperations()

	op := &config.CustomOperationConfig{
		Name:        "TransferFunds",
		Consistency: "atomic",
		Steps: []config.CustomStepConfig{
			{Type: "read", Resource: "accounts", ID: "input.sourceId", As: "source"},
			{Type: "read", Resource: "accounts", ID: "input.destId", As: "dest"},
			{Type: "validate", Condition: "source.balance >= input.amount", ErrorMessage: "Insufficient funds"},
			{Type: "update", Resource: "accounts", ID: "input.sourceId", Set: map[string]string{"balance": "source.balance - input.amount"}},
			{Type: "update", Resource: "accounts", ID: "input.destId", Set: map[string]string{"balance": "dest.balance + input.amount"}},
		},
		Response: map[string]string{
			"status":     `"completed"`,
			"newBalance": "source.balance - input.amount",
		},
	}
	if err := cos.Create(ctx, op); err != nil {
		t.Fatalf("Create() failed: %v", err)
	}

	// Verify full data via List
	list, err := cos.List(ctx)
	if err != nil {
		t.Fatalf("List() failed: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 operation, got %d", len(list))
	}

	got := list[0]
	if got.Name != "TransferFunds" {
		t.Errorf("Name = %q, want %q", got.Name, "TransferFunds")
	}
	if got.Consistency != "atomic" {
		t.Errorf("Consistency = %q, want %q", got.Consistency, "atomic")
	}
	if len(got.Steps) != 5 {
		t.Errorf("Steps count = %d, want 5", len(got.Steps))
	}
	if got.Steps[0].Type != "read" {
		t.Errorf("Step[0].Type = %q, want %q", got.Steps[0].Type, "read")
	}
	if got.Steps[0].Resource != "accounts" {
		t.Errorf("Step[0].Resource = %q, want %q", got.Steps[0].Resource, "accounts")
	}
	if got.Steps[0].As != "source" {
		t.Errorf("Step[0].As = %q, want %q", got.Steps[0].As, "source")
	}
	if got.Steps[2].Type != "validate" {
		t.Errorf("Step[2].Type = %q, want %q", got.Steps[2].Type, "validate")
	}
	if got.Steps[2].ErrorMessage != "Insufficient funds" {
		t.Errorf("Step[2].ErrorMessage = %q, want %q", got.Steps[2].ErrorMessage, "Insufficient funds")
	}
	if len(got.Response) != 2 {
		t.Errorf("Response count = %d, want 2", len(got.Response))
	}
	if got.Response["status"] != `"completed"` {
		t.Errorf("Response[status] = %q, want %q", got.Response["status"], `"completed"`)
	}
}

func TestCustomOperationStore_CreateDuplicate(t *testing.T) {
	fs := newTestStore(t)
	ctx := context.Background()
	cos := fs.CustomOperations()

	_ = cos.Create(ctx, &config.CustomOperationConfig{Name: "MyOp"})
	err := cos.Create(ctx, &config.CustomOperationConfig{Name: "MyOp"})
	if !errors.Is(err, store.ErrAlreadyExists) {
		t.Errorf("expected ErrAlreadyExists, got %v", err)
	}
}

func TestCustomOperationStore_DeleteByName(t *testing.T) {
	fs := newTestStore(t)
	ctx := context.Background()
	cos := fs.CustomOperations()

	_ = cos.Create(ctx, &config.CustomOperationConfig{Name: "Op1"})
	_ = cos.Create(ctx, &config.CustomOperationConfig{Name: "Op2"})
	_ = cos.Create(ctx, &config.CustomOperationConfig{Name: "Op3"})

	// Delete the middle one (default workspace)
	if err := cos.Delete(ctx, "", "Op2"); err != nil {
		t.Fatalf("Delete(Op2) failed: %v", err)
	}

	list, _ := cos.List(ctx)
	if len(list) != 2 {
		t.Fatalf("expected 2 operations after delete, got %d", len(list))
	}

	names := make(map[string]bool)
	for _, op := range list {
		names[op.Name] = true
	}
	if names["Op2"] {
		t.Error("Op2 should have been deleted")
	}
	if !names["Op1"] || !names["Op3"] {
		t.Errorf("expected Op1 and Op3 to remain, got %v", names)
	}
}

func TestCustomOperationStore_DeleteNotFound(t *testing.T) {
	fs := newTestStore(t)
	ctx := context.Background()
	err := fs.CustomOperations().Delete(ctx, "", "nonexistent")
	if !errors.Is(err, store.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestCustomOperationStore_DeleteAll_MultipleItems(t *testing.T) {
	fs := newTestStore(t)
	ctx := context.Background()
	cos := fs.CustomOperations()

	_ = cos.Create(ctx, &config.CustomOperationConfig{Name: "A"})
	_ = cos.Create(ctx, &config.CustomOperationConfig{Name: "B"})
	_ = cos.Create(ctx, &config.CustomOperationConfig{Name: "C"})

	if err := cos.DeleteAll(ctx, ""); err != nil {
		t.Fatalf("DeleteAll() failed: %v", err)
	}

	list, _ := cos.List(ctx)
	if len(list) != 0 {
		t.Errorf("expected 0 after DeleteAll, got %d", len(list))
	}
}

// TestCustomOperationStore_WorkspaceIsolation is a regression test for issue #12.
// Two workspaces must be able to register custom operations with the same name,
// and Delete/DeleteAll must be scoped to one workspace at a time.
func TestCustomOperationStore_WorkspaceIsolation(t *testing.T) {
	fs := newTestStore(t)
	ctx := context.Background()
	cos := fs.CustomOperations()

	// Same name, different workspaces — both must succeed.
	if err := cos.Create(ctx, &config.CustomOperationConfig{Name: "CancelOrder", Workspace: "ws-a"}); err != nil {
		t.Fatalf("Create ws-a: %v", err)
	}
	if err := cos.Create(ctx, &config.CustomOperationConfig{Name: "CancelOrder", Workspace: "ws-b"}); err != nil {
		t.Fatalf("Create ws-b: %v", err)
	}
	// Same (workspace, name) is a duplicate.
	if err := cos.Create(ctx, &config.CustomOperationConfig{Name: "CancelOrder", Workspace: "ws-a"}); !errors.Is(err, store.ErrAlreadyExists) {
		t.Errorf("expected ErrAlreadyExists, got %v", err)
	}

	// Delete only touches the named workspace.
	if err := cos.Delete(ctx, "ws-a", "CancelOrder"); err != nil {
		t.Fatalf("Delete ws-a: %v", err)
	}
	list, _ := cos.List(ctx)
	if len(list) != 1 || list[0].Workspace != "ws-b" {
		t.Errorf("expected only ws-b CancelOrder to remain, got %+v", list)
	}

	// DeleteAll is also workspace-scoped.
	_ = cos.Create(ctx, &config.CustomOperationConfig{Name: "Refund", Workspace: "ws-b"})
	_ = cos.Create(ctx, &config.CustomOperationConfig{Name: "Refund", Workspace: "ws-c"})
	if err := cos.DeleteAll(ctx, "ws-b"); err != nil {
		t.Fatalf("DeleteAll ws-b: %v", err)
	}
	list, _ = cos.List(ctx)
	for _, op := range list {
		if op.Workspace == "ws-b" {
			t.Errorf("ws-b operation %q should have been deleted", op.Name)
		}
	}
	if len(list) != 1 || list[0].Workspace != "ws-c" {
		t.Errorf("expected only ws-c Refund to remain, got %+v", list)
	}
}

func TestCustomOperationStore_ListEmpty(t *testing.T) {
	fs := newTestStore(t)
	ctx := context.Background()
	list, err := fs.CustomOperations().List(ctx)
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

func TestCustomOperationStore_ListReturnsCopy(t *testing.T) {
	fs := newTestStore(t)
	ctx := context.Background()
	cos := fs.CustomOperations()

	_ = cos.Create(ctx, &config.CustomOperationConfig{Name: "Op1"})

	list1, _ := cos.List(ctx)
	list2, _ := cos.List(ctx)

	// Modifying list1 should not affect list2
	list1[0] = nil
	if list2[0] == nil {
		t.Error("List() should return a copy, not reference to internal slice")
	}
}

func TestCustomOperationStore_ReadOnly_AllMethods(t *testing.T) {
	fs := newReadOnlyStore(t)
	ctx := context.Background()
	cos := fs.CustomOperations()

	if err := cos.Create(ctx, &config.CustomOperationConfig{Name: "x"}); !errors.Is(err, store.ErrReadOnly) {
		t.Errorf("Create: expected ErrReadOnly, got %v", err)
	}
	if err := cos.Delete(ctx, "", "x"); !errors.Is(err, store.ErrReadOnly) {
		t.Errorf("Delete: expected ErrReadOnly, got %v", err)
	}
	if err := cos.DeleteAll(ctx, ""); !errors.Is(err, store.ErrReadOnly) {
		t.Errorf("DeleteAll: expected ErrReadOnly, got %v", err)
	}

	// List should still work in read-only mode
	list, err := cos.List(ctx)
	if err != nil {
		t.Errorf("List should work in read-only mode, got %v", err)
	}
	if list == nil {
		t.Error("List should return empty slice, not nil")
	}
}

// ============================================================================
// Persistence Round-Trip Tests
// ============================================================================

func TestCustomOperationStore_PersistenceRoundTrip(t *testing.T) {
	dir := t.TempDir()
	cfg := store.Config{
		DataDir:   dir,
		ConfigDir: filepath.Join(dir, "config"),
		CacheDir:  filepath.Join(dir, "cache"),
		StateDir:  filepath.Join(dir, "state"),
	}
	ctx := context.Background()

	// Session 1: Create operations
	fs1 := New(cfg)
	if err := fs1.Open(ctx); err != nil {
		t.Fatalf("Session 1 Open() failed: %v", err)
	}

	cos1 := fs1.CustomOperations()
	_ = cos1.Create(ctx, &config.CustomOperationConfig{
		Name:        "TransferFunds",
		Consistency: "atomic",
		Steps: []config.CustomStepConfig{
			{Type: "read", Resource: "accounts", ID: "input.sourceId", As: "source"},
			{Type: "update", Resource: "accounts", ID: "input.sourceId",
				Set: map[string]string{"balance": "source.balance - input.amount"}},
		},
		Response: map[string]string{"status": `"done"`},
	})
	_ = cos1.Create(ctx, &config.CustomOperationConfig{
		Name: "VerifyUser",
		Steps: []config.CustomStepConfig{
			{Type: "read", Resource: "users", ID: "input.userId", As: "user"},
			{Type: "update", Resource: "users", ID: "input.userId",
				Set: map[string]string{"verified": "true"}},
		},
	})
	if err := fs1.Close(); err != nil {
		t.Fatalf("Session 1 Close() failed: %v", err)
	}

	// Verify data.json exists and contains operations
	raw, err := os.ReadFile(filepath.Join(dir, "data.json"))
	if err != nil {
		t.Fatalf("read data.json: %v", err)
	}
	var diskData map[string]interface{}
	if err := json.Unmarshal(raw, &diskData); err != nil {
		t.Fatalf("unmarshal data.json: %v", err)
	}
	ops, ok := diskData["customOperations"].([]interface{})
	if !ok {
		t.Fatalf("expected customOperations array in data.json, got %T", diskData["customOperations"])
	}
	if len(ops) != 2 {
		t.Fatalf("expected 2 operations in data.json, got %d", len(ops))
	}

	// Session 2: Verify data loaded correctly
	fs2 := New(cfg)
	if err := fs2.Open(ctx); err != nil {
		t.Fatalf("Session 2 Open() failed: %v", err)
	}
	defer func() { _ = fs2.Close() }()

	cos2 := fs2.CustomOperations()
	list, err := cos2.List(ctx)
	if err != nil {
		t.Fatalf("Session 2 List() failed: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("expected 2 operations from disk, got %d", len(list))
	}

	// Find TransferFunds
	var transfer *config.CustomOperationConfig
	for _, op := range list {
		if op.Name == "TransferFunds" {
			transfer = op
			break
		}
	}
	if transfer == nil {
		t.Fatal("TransferFunds not found in loaded data")
	}
	if transfer.Consistency != "atomic" {
		t.Errorf("Consistency = %q, want %q", transfer.Consistency, "atomic")
	}
	if len(transfer.Steps) != 2 {
		t.Errorf("Steps count = %d, want 2", len(transfer.Steps))
	}
	if transfer.Steps[0].Type != "read" || transfer.Steps[0].As != "source" {
		t.Errorf("Step[0] not loaded correctly: %+v", transfer.Steps[0])
	}
	if transfer.Steps[1].Set == nil || transfer.Steps[1].Set["balance"] != "source.balance - input.amount" {
		t.Errorf("Step[1].Set not loaded correctly: %+v", transfer.Steps[1])
	}
	if transfer.Response["status"] != `"done"` {
		t.Errorf("Response[status] = %q, want %q", transfer.Response["status"], `"done"`)
	}
}

func TestCustomOperationStore_PersistenceAfterDelete(t *testing.T) {
	dir := t.TempDir()
	cfg := store.Config{
		DataDir:   dir,
		ConfigDir: filepath.Join(dir, "config"),
		CacheDir:  filepath.Join(dir, "cache"),
		StateDir:  filepath.Join(dir, "state"),
	}
	ctx := context.Background()

	// Session 1: Create then delete an operation
	fs1 := New(cfg)
	_ = fs1.Open(ctx)
	cos1 := fs1.CustomOperations()
	_ = cos1.Create(ctx, &config.CustomOperationConfig{Name: "ToKeep"})
	_ = cos1.Create(ctx, &config.CustomOperationConfig{Name: "ToDelete"})
	_ = cos1.Delete(ctx, "", "ToDelete")
	_ = fs1.Close()

	// Session 2: Only ToKeep should remain
	fs2 := New(cfg)
	_ = fs2.Open(ctx)
	defer func() { _ = fs2.Close() }()

	list, _ := fs2.CustomOperations().List(ctx)
	if len(list) != 1 {
		t.Fatalf("expected 1 operation, got %d", len(list))
	}
	if list[0].Name != "ToKeep" {
		t.Errorf("expected ToKeep, got %q", list[0].Name)
	}
}

func TestCustomOperationStore_MultipleStepTypes(t *testing.T) {
	fs := newTestStore(t)
	ctx := context.Background()
	cos := fs.CustomOperations()

	op := &config.CustomOperationConfig{
		Name: "ComplexOp",
		Steps: []config.CustomStepConfig{
			{Type: "read", Resource: "users", ID: "input.userId", As: "user"},
			{Type: "validate", Condition: "user.active == true", ErrorMessage: "User is inactive", ErrorStatus: 403},
			{Type: "set", Var: "greeting", Value: `"Hello " + user.name`},
			{Type: "create", Resource: "logs", Set: map[string]string{"action": `"login"`, "userId": "user.id"}},
			{Type: "list", Resource: "orders", As: "orders", Filter: map[string]string{"userId": "user.id"}},
			{Type: "update", Resource: "users", ID: "input.userId", Set: map[string]string{"lastLogin": `"now"`}},
			{Type: "delete", Resource: "sessions", ID: "input.oldSessionId"},
		},
		Response: map[string]string{
			"message":    "greeting",
			"orderCount": "len(orders)",
		},
	}
	if err := cos.Create(ctx, op); err != nil {
		t.Fatalf("Create() failed: %v", err)
	}

	// Verify all step types and fields were stored
	list, _ := cos.List(ctx)
	if len(list) != 1 {
		t.Fatalf("expected 1, got %d", len(list))
	}

	got := list[0]
	if len(got.Steps) != 7 {
		t.Fatalf("expected 7 steps, got %d", len(got.Steps))
	}

	// Validate step (step 1)
	if got.Steps[1].ErrorStatus != 403 {
		t.Errorf("Step[1].ErrorStatus = %d, want 403", got.Steps[1].ErrorStatus)
	}

	// Set step (step 2)
	if got.Steps[2].Var != "greeting" {
		t.Errorf("Step[2].Var = %q, want %q", got.Steps[2].Var, "greeting")
	}
	if got.Steps[2].Value != `"Hello " + user.name` {
		t.Errorf("Step[2].Value = %q, want %q", got.Steps[2].Value, `"Hello " + user.name`)
	}

	// List step (step 4)
	if got.Steps[4].Filter == nil || got.Steps[4].Filter["userId"] != "user.id" {
		t.Errorf("Step[4].Filter not stored correctly: %+v", got.Steps[4].Filter)
	}

	// Delete step (step 6)
	if got.Steps[6].Type != "delete" || got.Steps[6].Resource != "sessions" {
		t.Errorf("Step[6] not stored correctly: %+v", got.Steps[6])
	}
}

func TestCustomOperationStore_CreateWithEmptySteps(t *testing.T) {
	fs := newTestStore(t)
	ctx := context.Background()
	cos := fs.CustomOperations()

	// Operations with no steps (e.g., pure response mapping)
	op := &config.CustomOperationConfig{
		Name:     "NoopOp",
		Steps:    []config.CustomStepConfig{},
		Response: map[string]string{"result": `"ok"`},
	}
	if err := cos.Create(ctx, op); err != nil {
		t.Fatalf("Create() failed: %v", err)
	}

	list, _ := cos.List(ctx)
	if len(list) != 1 {
		t.Fatalf("expected 1, got %d", len(list))
	}
	if len(list[0].Steps) != 0 {
		t.Errorf("expected 0 steps, got %d", len(list[0].Steps))
	}
}

func TestCustomOperationStore_CreateWithNoResponse(t *testing.T) {
	fs := newTestStore(t)
	ctx := context.Background()
	cos := fs.CustomOperations()

	op := &config.CustomOperationConfig{
		Name: "VoidOp",
		Steps: []config.CustomStepConfig{
			{Type: "set", Var: "x", Value: "1"},
		},
		// No Response map
	}
	if err := cos.Create(ctx, op); err != nil {
		t.Fatalf("Create() failed: %v", err)
	}

	list, _ := cos.List(ctx)
	if len(list) != 1 {
		t.Fatalf("expected 1, got %d", len(list))
	}
	if len(list[0].Response) != 0 {
		t.Errorf("expected nil or empty Response, got %v", list[0].Response)
	}
}

func TestCustomOperationStore_OrderPreservedAcrossCreates(t *testing.T) {
	fs := newTestStore(t)
	ctx := context.Background()
	cos := fs.CustomOperations()

	names := []string{"Alpha", "Beta", "Gamma", "Delta"}
	for _, name := range names {
		_ = cos.Create(ctx, &config.CustomOperationConfig{Name: name})
	}

	list, _ := cos.List(ctx)
	if len(list) != 4 {
		t.Fatalf("expected 4, got %d", len(list))
	}

	for i, name := range names {
		if list[i].Name != name {
			t.Errorf("list[%d].Name = %q, want %q (insertion order)", i, list[i].Name, name)
		}
	}
}
