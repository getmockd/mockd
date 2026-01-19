package store

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// =============================================================================
// Config & Directory Tests
// =============================================================================

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	if config.Backend != BackendFile {
		t.Errorf("Backend = %q, want %q", config.Backend, BackendFile)
	}
	if config.DataDir == "" {
		t.Error("DataDir should not be empty")
	}
	if config.ConfigDir == "" {
		t.Error("ConfigDir should not be empty")
	}
	if config.CacheDir == "" {
		t.Error("CacheDir should not be empty")
	}
	if config.StateDir == "" {
		t.Error("StateDir should not be empty")
	}
}

func TestDefaultDataDir(t *testing.T) {
	// Save and restore XDG env var
	original := os.Getenv("XDG_DATA_HOME")
	defer os.Setenv("XDG_DATA_HOME", original)

	// Test with XDG_DATA_HOME set
	os.Setenv("XDG_DATA_HOME", "/custom/data")
	dir := DefaultDataDir()
	if dir != "/custom/data/mockd" {
		t.Errorf("with XDG_DATA_HOME: got %q, want %q", dir, "/custom/data/mockd")
	}

	// Test without XDG_DATA_HOME (uses default)
	os.Unsetenv("XDG_DATA_HOME")
	dir = DefaultDataDir()
	if dir == "" {
		t.Error("DefaultDataDir should not return empty string")
	}
	// Should contain 'mockd'
	if filepath.Base(dir) != "mockd" {
		t.Errorf("dir should end with 'mockd', got %q", dir)
	}
}

func TestDefaultConfigDir(t *testing.T) {
	original := os.Getenv("XDG_CONFIG_HOME")
	defer os.Setenv("XDG_CONFIG_HOME", original)

	os.Setenv("XDG_CONFIG_HOME", "/custom/config")
	dir := DefaultConfigDir()
	if dir != "/custom/config/mockd" {
		t.Errorf("with XDG_CONFIG_HOME: got %q, want %q", dir, "/custom/config/mockd")
	}

	os.Unsetenv("XDG_CONFIG_HOME")
	dir = DefaultConfigDir()
	if dir == "" {
		t.Error("DefaultConfigDir should not return empty string")
	}
}

func TestDefaultCacheDir(t *testing.T) {
	original := os.Getenv("XDG_CACHE_HOME")
	defer os.Setenv("XDG_CACHE_HOME", original)

	os.Setenv("XDG_CACHE_HOME", "/custom/cache")
	dir := DefaultCacheDir()
	if dir != "/custom/cache/mockd" {
		t.Errorf("with XDG_CACHE_HOME: got %q, want %q", dir, "/custom/cache/mockd")
	}

	os.Unsetenv("XDG_CACHE_HOME")
	dir = DefaultCacheDir()
	if dir == "" {
		t.Error("DefaultCacheDir should not return empty string")
	}
}

func TestDefaultStateDir(t *testing.T) {
	original := os.Getenv("XDG_STATE_HOME")
	defer os.Setenv("XDG_STATE_HOME", original)

	os.Setenv("XDG_STATE_HOME", "/custom/state")
	dir := DefaultStateDir()
	if dir != "/custom/state/mockd" {
		t.Errorf("with XDG_STATE_HOME: got %q, want %q", dir, "/custom/state/mockd")
	}

	os.Unsetenv("XDG_STATE_HOME")
	dir = DefaultStateDir()
	if dir == "" {
		t.Error("DefaultStateDir should not return empty string")
	}
}

func TestBackendConstants(t *testing.T) {
	// Verify constants exist and have expected values
	if BackendFile != "file" {
		t.Errorf("BackendFile = %q, want %q", BackendFile, "file")
	}
	if BackendSQLite != "sqlite" {
		t.Errorf("BackendSQLite = %q, want %q", BackendSQLite, "sqlite")
	}
	if BackendMemory != "memory" {
		t.Errorf("BackendMemory = %q, want %q", BackendMemory, "memory")
	}
}

func TestErrors(t *testing.T) {
	if ErrNotFound.Error() != "not found" {
		t.Errorf("ErrNotFound = %q, want %q", ErrNotFound.Error(), "not found")
	}
	if ErrAlreadyExists.Error() != "already exists" {
		t.Errorf("ErrAlreadyExists = %q, want %q", ErrAlreadyExists.Error(), "already exists")
	}
	if ErrInvalidID.Error() != "invalid id" {
		t.Errorf("ErrInvalidID = %q, want %q", ErrInvalidID.Error(), "invalid id")
	}
	if ErrReadOnly.Error() != "store is read-only" {
		t.Errorf("ErrReadOnly = %q, want %q", ErrReadOnly.Error(), "store is read-only")
	}
}

// =============================================================================
// EngineRegistry Tests
// =============================================================================

func TestNewEngineRegistry(t *testing.T) {
	reg := NewEngineRegistry()
	if reg == nil {
		t.Fatal("NewEngineRegistry returned nil")
	}
	if reg.engines == nil {
		t.Error("engines map not initialized")
	}
	if reg.Count() != 0 {
		t.Errorf("Count = %d, want 0", reg.Count())
	}
}

func TestEngineRegistry_Register(t *testing.T) {
	reg := NewEngineRegistry()
	engine := &Engine{
		ID:   "engine-1",
		Name: "Test Engine",
		Host: "localhost",
		Port: 8080,
	}

	err := reg.Register(engine)
	if err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	// Verify registration
	if reg.Count() != 1 {
		t.Errorf("Count = %d, want 1", reg.Count())
	}

	// Verify engine state was updated
	got, _ := reg.Get("engine-1")
	if got.Status != EngineStatusOnline {
		t.Errorf("Status = %q, want %q", got.Status, EngineStatusOnline)
	}
	if got.LastSeen.IsZero() {
		t.Error("LastSeen should be set")
	}
	if got.RegisteredAt.IsZero() {
		t.Error("RegisteredAt should be set")
	}
	if got.PortRangeStart != DefaultPortRangeStart {
		t.Errorf("PortRangeStart = %d, want %d", got.PortRangeStart, DefaultPortRangeStart)
	}
	if got.PortRangeEnd != DefaultPortRangeEnd {
		t.Errorf("PortRangeEnd = %d, want %d", got.PortRangeEnd, DefaultPortRangeEnd)
	}

	// Re-register should update
	engine2 := &Engine{
		ID:      "engine-1",
		Name:    "Updated Engine",
		Host:    "localhost",
		Port:    9090,
		Version: "2.0.0",
	}
	reg.Register(engine2)
	got, _ = reg.Get("engine-1")
	if got.Name != "Updated Engine" {
		t.Errorf("Name = %q, want %q", got.Name, "Updated Engine")
	}
}

func TestEngineRegistry_Unregister(t *testing.T) {
	reg := NewEngineRegistry()
	reg.Register(&Engine{ID: "engine-1", Name: "Test"})

	err := reg.Unregister("engine-1")
	if err != nil {
		t.Fatalf("Unregister failed: %v", err)
	}

	if reg.Count() != 0 {
		t.Errorf("Count = %d, want 0", reg.Count())
	}

	// Unregister non-existent should return error
	err = reg.Unregister("nonexistent")
	if err != ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestEngineRegistry_Get(t *testing.T) {
	reg := NewEngineRegistry()
	reg.Register(&Engine{
		ID:   "engine-1",
		Name: "Test",
		Workspaces: []EngineWorkspace{
			{WorkspaceID: "ws-1", HTTPPort: 9001},
		},
	})

	got, err := reg.Get("engine-1")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if got.ID != "engine-1" {
		t.Errorf("ID = %q, want %q", got.ID, "engine-1")
	}

	// Should return a copy (not modify original)
	got.Name = "Modified"
	original, _ := reg.Get("engine-1")
	if original.Name == "Modified" {
		t.Error("Get should return a copy, not the original")
	}

	// Get non-existent
	_, err = reg.Get("nonexistent")
	if err != ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestEngineRegistry_List(t *testing.T) {
	reg := NewEngineRegistry()

	// Empty list
	list := reg.List()
	if len(list) != 0 {
		t.Errorf("expected empty list, got %d items", len(list))
	}

	// Add engines
	reg.Register(&Engine{ID: "engine-1", Name: "E1"})
	reg.Register(&Engine{ID: "engine-2", Name: "E2"})

	list = reg.List()
	if len(list) != 2 {
		t.Errorf("expected 2 items, got %d", len(list))
	}
}

func TestEngineRegistry_UpdateStatus(t *testing.T) {
	reg := NewEngineRegistry()
	reg.Register(&Engine{ID: "engine-1", Name: "Test"})

	err := reg.UpdateStatus("engine-1", EngineStatusOffline)
	if err != nil {
		t.Fatalf("UpdateStatus failed: %v", err)
	}

	got, _ := reg.Get("engine-1")
	if got.Status != EngineStatusOffline {
		t.Errorf("Status = %q, want %q", got.Status, EngineStatusOffline)
	}

	// Update non-existent
	err = reg.UpdateStatus("nonexistent", EngineStatusOnline)
	if err != ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestEngineRegistry_Heartbeat(t *testing.T) {
	reg := NewEngineRegistry()
	engine := &Engine{ID: "engine-1", Name: "Test"}
	reg.Register(engine)

	// Set to offline
	reg.UpdateStatus("engine-1", EngineStatusOffline)

	time.Sleep(10 * time.Millisecond)

	// Heartbeat should set back to online and update LastSeen
	err := reg.Heartbeat("engine-1")
	if err != nil {
		t.Fatalf("Heartbeat failed: %v", err)
	}

	got, _ := reg.Get("engine-1")
	if got.Status != EngineStatusOnline {
		t.Errorf("Status = %q, want %q", got.Status, EngineStatusOnline)
	}

	// Heartbeat non-existent
	err = reg.Heartbeat("nonexistent")
	if err != ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestEngineRegistry_AssignWorkspace(t *testing.T) {
	reg := NewEngineRegistry()
	reg.Register(&Engine{ID: "engine-1", Name: "Test"})

	err := reg.AssignWorkspace("engine-1", "workspace-1")
	if err != nil {
		t.Fatalf("AssignWorkspace failed: %v", err)
	}

	got, _ := reg.Get("engine-1")
	if got.WorkspaceID != "workspace-1" {
		t.Errorf("WorkspaceID = %q, want %q", got.WorkspaceID, "workspace-1")
	}

	// Non-existent engine
	err = reg.AssignWorkspace("nonexistent", "ws")
	if err != ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestEngineRegistry_CountByStatus(t *testing.T) {
	reg := NewEngineRegistry()
	reg.Register(&Engine{ID: "e1"})
	reg.Register(&Engine{ID: "e2"})
	reg.Register(&Engine{ID: "e3"})
	reg.UpdateStatus("e2", EngineStatusOffline)

	online := reg.CountByStatus(EngineStatusOnline)
	if online != 2 {
		t.Errorf("online count = %d, want 2", online)
	}

	offline := reg.CountByStatus(EngineStatusOffline)
	if offline != 1 {
		t.Errorf("offline count = %d, want 1", offline)
	}
}

// =============================================================================
// Engine Workspace Management Tests
// =============================================================================

func TestEngine_AddWorkspace(t *testing.T) {
	engine := &Engine{
		ID:             "engine-1",
		PortRangeStart: 9000,
		PortRangeEnd:   9999,
		Workspaces:     []EngineWorkspace{},
	}

	// Add new workspace with explicit ports
	ws := engine.AddWorkspace("ws-1", "Workspace 1", 9001, 9002, 9003)
	if ws.WorkspaceID != "ws-1" {
		t.Errorf("WorkspaceID = %q, want %q", ws.WorkspaceID, "ws-1")
	}
	if ws.HTTPPort != 9001 {
		t.Errorf("HTTPPort = %d, want 9001", ws.HTTPPort)
	}
	if ws.GRPCPort != 9002 {
		t.Errorf("GRPCPort = %d, want 9002", ws.GRPCPort)
	}

	// Add workspace with auto-assigned port
	ws2 := engine.AddWorkspace("ws-2", "Workspace 2", 0, 0, 0)
	if ws2.HTTPPort == 0 {
		t.Error("HTTPPort should be auto-assigned")
	}
	// Should be 9000 (first in range not used)
	if ws2.HTTPPort != 9000 {
		t.Errorf("HTTPPort = %d, want 9000 (auto-assigned)", ws2.HTTPPort)
	}

	// Update existing workspace
	ws3 := engine.AddWorkspace("ws-1", "Workspace 1 Updated", 9010, 0, 0)
	if ws3.HTTPPort != 9010 {
		t.Errorf("HTTPPort = %d, want 9010 (updated)", ws3.HTTPPort)
	}
	if ws3.GRPCPort != 9002 {
		t.Errorf("GRPCPort = %d, want 9002 (preserved)", ws3.GRPCPort)
	}
}

func TestEngine_RemoveWorkspace(t *testing.T) {
	engine := &Engine{
		ID: "engine-1",
		Workspaces: []EngineWorkspace{
			{WorkspaceID: "ws-1"},
			{WorkspaceID: "ws-2"},
		},
	}

	removed := engine.RemoveWorkspace("ws-1")
	if !removed {
		t.Error("RemoveWorkspace should return true")
	}
	if len(engine.Workspaces) != 1 {
		t.Errorf("expected 1 workspace, got %d", len(engine.Workspaces))
	}

	// Remove non-existent
	removed = engine.RemoveWorkspace("nonexistent")
	if removed {
		t.Error("RemoveWorkspace should return false for non-existent")
	}
}

func TestEngine_GetWorkspace(t *testing.T) {
	engine := &Engine{
		ID: "engine-1",
		Workspaces: []EngineWorkspace{
			{WorkspaceID: "ws-1", HTTPPort: 9001},
		},
	}

	ws := engine.GetWorkspace("ws-1")
	if ws == nil {
		t.Fatal("GetWorkspace returned nil")
	}
	if ws.HTTPPort != 9001 {
		t.Errorf("HTTPPort = %d, want 9001", ws.HTTPPort)
	}

	// Non-existent
	ws = engine.GetWorkspace("nonexistent")
	if ws != nil {
		t.Error("GetWorkspace should return nil for non-existent")
	}
}

func TestEngine_FindAvailablePort(t *testing.T) {
	engine := &Engine{
		ID:             "engine-1",
		PortRangeStart: 9000,
		PortRangeEnd:   9005,
		Workspaces: []EngineWorkspace{
			{HTTPPort: 9000},
			{HTTPPort: 9001},
			{HTTPPort: 9002},
		},
	}

	port := engine.FindAvailablePort()
	if port != 9003 {
		t.Errorf("FindAvailablePort = %d, want 9003", port)
	}

	// Fill up all ports
	engine.Workspaces = append(engine.Workspaces,
		EngineWorkspace{HTTPPort: 9003},
		EngineWorkspace{HTTPPort: 9004},
		EngineWorkspace{HTTPPort: 9005},
	)
	port = engine.FindAvailablePort()
	if port != 0 {
		t.Errorf("FindAvailablePort = %d, want 0 (no available)", port)
	}
}

func TestEngine_GetUsedPorts(t *testing.T) {
	engine := &Engine{
		ID: "engine-1",
		Workspaces: []EngineWorkspace{
			{HTTPPort: 9000, GRPCPort: 9100, MQTTPort: 9200},
			{HTTPPort: 9001},
		},
	}

	used := engine.GetUsedPorts()
	if !used[9000] {
		t.Error("9000 should be marked as used")
	}
	if !used[9100] {
		t.Error("9100 should be marked as used")
	}
	if !used[9200] {
		t.Error("9200 should be marked as used")
	}
	if !used[9001] {
		t.Error("9001 should be marked as used")
	}
	if used[9002] {
		t.Error("9002 should not be marked as used")
	}
}

func TestEngine_UpdateWorkspace(t *testing.T) {
	engine := &Engine{
		ID: "engine-1",
		Workspaces: []EngineWorkspace{
			{WorkspaceID: "ws-1", HTTPPort: 9000},
		},
	}

	ws := engine.UpdateWorkspace("ws-1", 9001, 9002, 9003)
	if ws == nil {
		t.Fatal("UpdateWorkspace returned nil")
	}
	if ws.HTTPPort != 9001 {
		t.Errorf("HTTPPort = %d, want 9001", ws.HTTPPort)
	}
	if ws.GRPCPort != 9002 {
		t.Errorf("GRPCPort = %d, want 9002", ws.GRPCPort)
	}

	// Non-existent
	ws = engine.UpdateWorkspace("nonexistent", 0, 0, 0)
	if ws != nil {
		t.Error("UpdateWorkspace should return nil for non-existent")
	}
}

func TestEngine_SyncWorkspace(t *testing.T) {
	engine := &Engine{
		ID: "engine-1",
		Workspaces: []EngineWorkspace{
			{WorkspaceID: "ws-1"},
		},
	}

	ws := engine.SyncWorkspace("ws-1")
	if ws == nil {
		t.Fatal("SyncWorkspace returned nil")
	}
	if ws.LastSynced.IsZero() {
		t.Error("LastSynced should be set")
	}

	// Non-existent
	ws = engine.SyncWorkspace("nonexistent")
	if ws != nil {
		t.Error("SyncWorkspace should return nil for non-existent")
	}
}

func TestEngine_Copy(t *testing.T) {
	original := &Engine{
		ID:   "engine-1",
		Name: "Test",
		Workspaces: []EngineWorkspace{
			{WorkspaceID: "ws-1", HTTPPort: 9000},
		},
	}

	cpy := original.Copy()

	// Verify copy is independent
	cpy.Name = "Modified"
	cpy.Workspaces[0].HTTPPort = 9999

	if original.Name != "Test" {
		t.Error("Copy modified original Name")
	}
	if original.Workspaces[0].HTTPPort != 9000 {
		t.Error("Copy modified original Workspaces")
	}
}

// =============================================================================
// EngineRegistry Workspace Methods Tests
// =============================================================================

func TestEngineRegistry_WorkspaceMethods(t *testing.T) {
	reg := NewEngineRegistry()
	reg.Register(&Engine{ID: "engine-1", Name: "Test"})

	// AddWorkspaceToEngine
	ws, err := reg.AddWorkspaceToEngine("engine-1", "ws-1", "Workspace 1", 9001, 9002, 0)
	if err != nil {
		t.Fatalf("AddWorkspaceToEngine failed: %v", err)
	}
	if ws.HTTPPort != 9001 {
		t.Errorf("HTTPPort = %d, want 9001", ws.HTTPPort)
	}

	// GetWorkspaceFromEngine
	ws, err = reg.GetWorkspaceFromEngine("engine-1", "ws-1")
	if err != nil {
		t.Fatalf("GetWorkspaceFromEngine failed: %v", err)
	}
	if ws.WorkspaceID != "ws-1" {
		t.Errorf("WorkspaceID = %q, want %q", ws.WorkspaceID, "ws-1")
	}

	// UpdateWorkspaceInEngine
	ws, err = reg.UpdateWorkspaceInEngine("engine-1", "ws-1", 9010, 0, 0)
	if err != nil {
		t.Fatalf("UpdateWorkspaceInEngine failed: %v", err)
	}
	if ws.HTTPPort != 9010 {
		t.Errorf("HTTPPort = %d, want 9010", ws.HTTPPort)
	}

	// SyncWorkspaceInEngine
	ws, err = reg.SyncWorkspaceInEngine("engine-1", "ws-1")
	if err != nil {
		t.Fatalf("SyncWorkspaceInEngine failed: %v", err)
	}
	if ws.LastSynced.IsZero() {
		t.Error("LastSynced should be set")
	}

	// UpdateWorkspaceStatus
	err = reg.UpdateWorkspaceStatus("engine-1", "ws-1", "running")
	if err != nil {
		t.Fatalf("UpdateWorkspaceStatus failed: %v", err)
	}
	ws, _ = reg.GetWorkspaceFromEngine("engine-1", "ws-1")
	if ws.Status != "running" {
		t.Errorf("Status = %q, want %q", ws.Status, "running")
	}

	// RemoveWorkspaceFromEngine
	err = reg.RemoveWorkspaceFromEngine("engine-1", "ws-1")
	if err != nil {
		t.Fatalf("RemoveWorkspaceFromEngine failed: %v", err)
	}
	_, err = reg.GetWorkspaceFromEngine("engine-1", "ws-1")
	if err != ErrNotFound {
		t.Errorf("expected ErrNotFound after removal, got %v", err)
	}
}

func TestEngineRegistry_WorkspaceMethods_Errors(t *testing.T) {
	reg := NewEngineRegistry()
	reg.Register(&Engine{ID: "engine-1", Name: "Test"})

	// Non-existent engine
	_, err := reg.AddWorkspaceToEngine("nonexistent", "ws-1", "", 0, 0, 0)
	if err != ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}

	_, err = reg.GetWorkspaceFromEngine("nonexistent", "ws-1")
	if err != ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}

	_, err = reg.UpdateWorkspaceInEngine("nonexistent", "ws-1", 0, 0, 0)
	if err != ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}

	_, err = reg.SyncWorkspaceInEngine("nonexistent", "ws-1")
	if err != ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}

	err = reg.UpdateWorkspaceStatus("nonexistent", "ws-1", "")
	if err != ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}

	err = reg.RemoveWorkspaceFromEngine("nonexistent", "ws-1")
	if err != ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}

	// Non-existent workspace on existing engine
	_, err = reg.GetWorkspaceFromEngine("engine-1", "nonexistent")
	if err != ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}

	_, err = reg.UpdateWorkspaceInEngine("engine-1", "nonexistent", 0, 0, 0)
	if err != ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}

	_, err = reg.SyncWorkspaceInEngine("engine-1", "nonexistent")
	if err != ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}

	err = reg.UpdateWorkspaceStatus("engine-1", "nonexistent", "")
	if err != ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}

	err = reg.RemoveWorkspaceFromEngine("engine-1", "nonexistent")
	if err != ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestEngineRegistry_HealthCheck(t *testing.T) {
	reg := NewEngineRegistry()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	reg.Register(&Engine{ID: "engine-1", Name: "Test"})

	// Start health check with short timeout
	reg.StartHealthCheck(ctx, 50*time.Millisecond)

	// Wait for health check to mark engine offline
	time.Sleep(100 * time.Millisecond)

	engine, _ := reg.Get("engine-1")
	if engine.Status != EngineStatusOffline {
		t.Errorf("Status = %q, want %q (after timeout)", engine.Status, EngineStatusOffline)
	}

	// Stop health check
	reg.Stop()
}

// =============================================================================
// WorkspaceFileStore Tests
// =============================================================================

func TestWorkspaceFileStore_CRUD(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "mockd-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	store := NewWorkspaceFileStore(tmpDir)
	ctx := context.Background()

	// Open
	err = store.Open(ctx)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}

	// List should have default workspace
	list, err := store.List(ctx)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(list) != 1 {
		t.Errorf("expected 1 default workspace, got %d", len(list))
	}
	if list[0].ID != DefaultWorkspaceID {
		t.Errorf("default workspace ID = %q, want %q", list[0].ID, DefaultWorkspaceID)
	}

	// Get default workspace
	ws, err := store.Get(ctx, DefaultWorkspaceID)
	if err != nil {
		t.Fatalf("Get default failed: %v", err)
	}
	if ws.Name != "Default" {
		t.Errorf("Name = %q, want %q", ws.Name, "Default")
	}

	// Create new workspace
	newWS := &Workspace{
		ID:   "custom-ws",
		Name: "Custom Workspace",
		Type: WorkspaceTypeLocal,
	}
	err = store.Create(ctx, newWS)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if newWS.CreatedAt == 0 {
		t.Error("CreatedAt should be set")
	}
	if newWS.Path == "" {
		t.Error("Path should be auto-set for local workspace")
	}

	// Get created workspace
	ws, err = store.Get(ctx, "custom-ws")
	if err != nil {
		t.Fatalf("Get custom failed: %v", err)
	}
	if ws.Name != "Custom Workspace" {
		t.Errorf("Name = %q, want %q", ws.Name, "Custom Workspace")
	}

	// Update workspace
	ws.Name = "Updated Workspace"
	err = store.Update(ctx, ws)
	if err != nil {
		t.Fatalf("Update failed: %v", err)
	}
	ws, _ = store.Get(ctx, "custom-ws")
	if ws.Name != "Updated Workspace" {
		t.Errorf("Name = %q, want %q", ws.Name, "Updated Workspace")
	}

	// Delete workspace
	err = store.Delete(ctx, "custom-ws")
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	_, err = store.Get(ctx, "custom-ws")
	if err != ErrNotFound {
		t.Errorf("expected ErrNotFound after delete, got %v", err)
	}

	// Close
	err = store.Close()
	if err != nil {
		t.Fatalf("Close failed: %v", err)
	}
}

func TestWorkspaceFileStore_Errors(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "mockd-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	store := NewWorkspaceFileStore(tmpDir)
	ctx := context.Background()
	store.Open(ctx)

	// Create duplicate
	ws := &Workspace{ID: "test-ws", Name: "Test", Type: WorkspaceTypeLocal}
	store.Create(ctx, ws)
	err = store.Create(ctx, ws)
	if err != ErrAlreadyExists {
		t.Errorf("expected ErrAlreadyExists, got %v", err)
	}

	// Delete default workspace
	err = store.Delete(ctx, DefaultWorkspaceID)
	if err != ErrReadOnly {
		t.Errorf("expected ErrReadOnly for default workspace, got %v", err)
	}

	// Update non-existent
	err = store.Update(ctx, &Workspace{ID: "nonexistent"})
	if err != ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}

	// Delete non-existent
	err = store.Delete(ctx, "nonexistent")
	if err != ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}

	// Get non-existent
	_, err = store.Get(ctx, "nonexistent")
	if err != ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestWorkspaceFileStore_Persistence(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "mockd-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	ctx := context.Background()

	// Create and populate store
	store1 := NewWorkspaceFileStore(tmpDir)
	store1.Open(ctx)
	store1.Create(ctx, &Workspace{
		ID:   "persistent-ws",
		Name: "Persistent",
		Type: WorkspaceTypeLocal,
	})
	store1.Close()

	// Re-open store
	store2 := NewWorkspaceFileStore(tmpDir)
	err = store2.Open(ctx)
	if err != nil {
		t.Fatalf("Second Open failed: %v", err)
	}

	// Verify data persisted
	ws, err := store2.Get(ctx, "persistent-ws")
	if err != nil {
		t.Fatalf("Get after reopen failed: %v", err)
	}
	if ws.Name != "Persistent" {
		t.Errorf("Name = %q, want %q", ws.Name, "Persistent")
	}
}

func TestWorkspaceFileStore_DefaultPath(t *testing.T) {
	store := NewWorkspaceFileStore("")
	// Should not panic and use default
	if store.DataDir() == "" {
		t.Error("DataDir should not be empty")
	}
}

func TestWorkspaceFileStore_ListBeforeOpen(t *testing.T) {
	store := NewWorkspaceFileStore("/nonexistent")
	ctx := context.Background()

	// List before Open should return error
	_, err := store.List(ctx)
	if err != ErrNotFound {
		t.Errorf("expected ErrNotFound before Open, got %v", err)
	}
}

// =============================================================================
// Interface Type Tests
// =============================================================================

func TestWorkspaceTypeConstants(t *testing.T) {
	if WorkspaceTypeLocal != "local" {
		t.Errorf("WorkspaceTypeLocal = %q, want %q", WorkspaceTypeLocal, "local")
	}
	if WorkspaceTypeGit != "git" {
		t.Errorf("WorkspaceTypeGit = %q, want %q", WorkspaceTypeGit, "git")
	}
	if WorkspaceTypeCloud != "cloud" {
		t.Errorf("WorkspaceTypeCloud = %q, want %q", WorkspaceTypeCloud, "cloud")
	}
	if WorkspaceTypeConfig != "config" {
		t.Errorf("WorkspaceTypeConfig = %q, want %q", WorkspaceTypeConfig, "config")
	}
}

func TestSyncStatusConstants(t *testing.T) {
	if SyncStatusSynced != "synced" {
		t.Errorf("SyncStatusSynced = %q, want %q", SyncStatusSynced, "synced")
	}
	if SyncStatusPending != "pending" {
		t.Errorf("SyncStatusPending = %q, want %q", SyncStatusPending, "pending")
	}
	if SyncStatusError != "error" {
		t.Errorf("SyncStatusError = %q, want %q", SyncStatusError, "error")
	}
	if SyncStatusLocal != "local" {
		t.Errorf("SyncStatusLocal = %q, want %q", SyncStatusLocal, "local")
	}
}

func TestEngineStatusConstants(t *testing.T) {
	if EngineStatusOnline != "online" {
		t.Errorf("EngineStatusOnline = %q, want %q", EngineStatusOnline, "online")
	}
	if EngineStatusOffline != "offline" {
		t.Errorf("EngineStatusOffline = %q, want %q", EngineStatusOffline, "offline")
	}
}
