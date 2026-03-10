package engine

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"testing"
	"time"

	"github.com/getmockd/mockd/pkg/config"
	"github.com/getmockd/mockd/pkg/mock"
	"github.com/getmockd/mockd/pkg/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStartWorkspace_ValidatesInput(t *testing.T) {
	t.Parallel()

	mgr := NewWorkspaceManager(nil)

	tests := []struct {
		name string
		ws   *store.EngineWorkspace
	}{
		{
			name: "nil workspace",
			ws:   nil,
		},
		{
			name: "missing workspace ID",
			ws: &store.EngineWorkspace{
				WorkspaceID: "   ",
				HTTPPort:    9000,
			},
		},
		{
			name: "invalid zero port",
			ws: &store.EngineWorkspace{
				WorkspaceID: "ws-1",
				HTTPPort:    0,
			},
		},
		{
			name: "invalid high port",
			ws: &store.EngineWorkspace{
				WorkspaceID: "ws-1",
				HTTPPort:    70000,
			},
		},
		{
			name: "invalid gRPC port",
			ws: &store.EngineWorkspace{
				WorkspaceID: "ws-1",
				HTTPPort:    9000,
				GRPCPort:    -1,
			},
		},
		{
			name: "invalid MQTT port",
			ws: &store.EngineWorkspace{
				WorkspaceID: "ws-1",
				HTTPPort:    9000,
				MQTTPort:    70000,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if err := mgr.StartWorkspace(context.Background(), tt.ws); err == nil {
				t.Fatalf("expected validation error, got nil")
			}
		})
	}
}

// --- NewWorkspaceManager ---

func TestNewWorkspaceManager_DefaultConfig(t *testing.T) {
	t.Parallel()
	mgr := NewWorkspaceManager(nil)
	require.NotNil(t, mgr)
	assert.NotNil(t, mgr.workspaces) // map is initialized
	assert.NotNil(t, mgr.log)        // logger is set (nop)
}

func TestNewWorkspaceManager_CustomConfig(t *testing.T) {
	t.Parallel()
	cfg := &WorkspaceManagerConfig{
		DefaultReadTimeout:  5 * time.Second,
		DefaultWriteTimeout: 10 * time.Second,
		MaxLogEntries:       500,
	}
	mgr := NewWorkspaceManager(cfg)
	require.NotNil(t, mgr)
	assert.Equal(t, 5*time.Second, mgr.defaultReadTimeout)
	assert.Equal(t, 10*time.Second, mgr.defaultWriteTimeout)
	assert.Equal(t, 500, mgr.maxLogEntries)
}

// --- SetLogger, SetMockFetcher, SetCentralStore ---

func TestWorkspaceManager_SetLogger_NoPanic(t *testing.T) {
	t.Parallel()
	mgr := NewWorkspaceManager(nil)

	// Setting a real logger should not panic
	assert.NotPanics(t, func() {
		mgr.SetLogger(slog.Default())
	})

	// Setting nil should not panic (reverts to nop)
	assert.NotPanics(t, func() {
		mgr.SetLogger(nil)
	})
	assert.NotNil(t, mgr.log)
}

func TestWorkspaceManager_SetMockFetcher_NoPanic(t *testing.T) {
	t.Parallel()
	mgr := NewWorkspaceManager(nil)

	fetcher := func(_ context.Context, _ string) ([]*config.MockConfiguration, error) {
		return nil, nil
	}
	assert.NotPanics(t, func() {
		mgr.SetMockFetcher(fetcher)
	})
	assert.NotNil(t, mgr.mockFetcher)

	// Setting nil fetcher
	assert.NotPanics(t, func() {
		mgr.SetMockFetcher(nil)
	})
	assert.Nil(t, mgr.mockFetcher)
}

func TestWorkspaceManager_SetCentralStore_NoPanic(t *testing.T) {
	t.Parallel()
	mgr := NewWorkspaceManager(nil)

	assert.NotPanics(t, func() {
		mgr.SetCentralStore(nil)
	})
	assert.Nil(t, mgr.centralStore)
}

// --- ListWorkspaces on empty manager ---

func TestWorkspaceManager_ListWorkspaces_Empty(t *testing.T) {
	t.Parallel()
	mgr := NewWorkspaceManager(nil)

	servers := mgr.ListWorkspaces()
	assert.NotNil(t, servers) // returns empty slice, not nil
	assert.Empty(t, servers)
}

// --- GetWorkspace for non-existent ---

func TestWorkspaceManager_GetWorkspace_NonExistent(t *testing.T) {
	t.Parallel()
	mgr := NewWorkspaceManager(nil)

	ws := mgr.GetWorkspace("does-not-exist")
	assert.Nil(t, ws)
}

// --- StopAll on empty manager ---

func TestWorkspaceManager_StopAll_Empty(t *testing.T) {
	t.Parallel()
	mgr := NewWorkspaceManager(nil)

	err := mgr.StopAll()
	assert.NoError(t, err)
}

// --- RemoveWorkspace non-existent ---

func TestWorkspaceManager_RemoveWorkspace_NonExistent(t *testing.T) {
	t.Parallel()
	mgr := NewWorkspaceManager(nil)

	err := mgr.RemoveWorkspace("does-not-exist")
	assert.NoError(t, err) // returns nil for not found
}

// --- GetWorkspaceStatus non-existent ---

func TestWorkspaceManager_GetWorkspaceStatus_NonExistent(t *testing.T) {
	t.Parallel()
	mgr := NewWorkspaceManager(nil)

	status := mgr.GetWorkspaceStatus("does-not-exist")
	assert.Nil(t, status)
}

// --- Full lifecycle ---

// newTestWorkspace creates an EngineWorkspace using a free port.
func newTestWorkspace(id string) *store.EngineWorkspace {
	return &store.EngineWorkspace{
		WorkspaceID:   id,
		WorkspaceName: "Test " + id,
		HTTPPort:      getFreePort(),
	}
}

func TestWorkspaceManager_FullLifecycle(t *testing.T) {
	t.Parallel()
	mgr := NewWorkspaceManager(nil)
	ctx := context.Background()

	ws := newTestWorkspace("lifecycle-ws")

	// Start workspace
	err := mgr.StartWorkspace(ctx, ws)
	require.NoError(t, err)
	t.Cleanup(func() { _ = mgr.StopAll() })

	// Verify it appears in ListWorkspaces
	servers := mgr.ListWorkspaces()
	require.Len(t, servers, 1)
	assert.Equal(t, WorkspaceServerStatusRunning, servers[0].Status())

	// GetWorkspace returns non-nil
	server := mgr.GetWorkspace("lifecycle-ws")
	require.NotNil(t, server)
	assert.Equal(t, WorkspaceServerStatusRunning, server.Status())

	// GetWorkspaceStatus returns running status
	status := mgr.GetWorkspaceStatus("lifecycle-ws")
	require.NotNil(t, status)
	assert.Equal(t, WorkspaceServerStatusRunning, status.Status)
	assert.Equal(t, ws.HTTPPort, status.HTTPPort)
	assert.Equal(t, "Test lifecycle-ws", status.WorkspaceName)

	// Verify the HTTP server is actually listening
	resp, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d/health-check-test", ws.HTTPPort))
	require.NoError(t, err)
	resp.Body.Close()

	// Stop workspace
	err = mgr.StopWorkspace("lifecycle-ws")
	require.NoError(t, err)

	// Status should be stopped
	status = mgr.GetWorkspaceStatus("lifecycle-ws")
	require.NotNil(t, status)
	assert.Equal(t, WorkspaceServerStatusStopped, status.Status)

	// Remove workspace
	err = mgr.RemoveWorkspace("lifecycle-ws")
	require.NoError(t, err)

	// Now it should be gone
	assert.Nil(t, mgr.GetWorkspace("lifecycle-ws"))
	assert.Empty(t, mgr.ListWorkspaces())
}

func TestWorkspaceManager_StopWorkspace_NotFound(t *testing.T) {
	t.Parallel()
	mgr := NewWorkspaceManager(nil)

	err := mgr.StopWorkspace("nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestWorkspaceManager_StartWorkspace_DuplicateRunning(t *testing.T) {
	t.Parallel()
	mgr := NewWorkspaceManager(nil)
	ctx := context.Background()

	ws := newTestWorkspace("dup-ws")

	err := mgr.StartWorkspace(ctx, ws)
	require.NoError(t, err)
	t.Cleanup(func() { _ = mgr.StopAll() })

	// Starting the same workspace again while running should fail
	err = mgr.StartWorkspace(ctx, ws)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already running")
}

func TestWorkspaceManager_StopAll_WithRunning(t *testing.T) {
	t.Parallel()
	mgr := NewWorkspaceManager(nil)
	ctx := context.Background()

	ws1 := newTestWorkspace("stopall-ws1")
	ws2 := newTestWorkspace("stopall-ws2")

	require.NoError(t, mgr.StartWorkspace(ctx, ws1))
	require.NoError(t, mgr.StartWorkspace(ctx, ws2))
	assert.Len(t, mgr.ListWorkspaces(), 2)

	err := mgr.StopAll()
	assert.NoError(t, err)

	// Both should be stopped
	s1 := mgr.GetWorkspaceStatus("stopall-ws1")
	s2 := mgr.GetWorkspaceStatus("stopall-ws2")
	require.NotNil(t, s1)
	require.NotNil(t, s2)
	assert.Equal(t, WorkspaceServerStatusStopped, s1.Status)
	assert.Equal(t, WorkspaceServerStatusStopped, s2.Status)
}

func TestWorkspaceManager_RemoveWorkspace_StopsIfRunning(t *testing.T) {
	t.Parallel()
	mgr := NewWorkspaceManager(nil)
	ctx := context.Background()

	ws := newTestWorkspace("remove-running-ws")

	require.NoError(t, mgr.StartWorkspace(ctx, ws))
	assert.Equal(t, WorkspaceServerStatusRunning, mgr.GetWorkspace("remove-running-ws").Status())

	// RemoveWorkspace should stop it and remove it
	err := mgr.RemoveWorkspace("remove-running-ws")
	assert.NoError(t, err)
	assert.Nil(t, mgr.GetWorkspace("remove-running-ws"))
}

// --- Workspace server mock CRUD ---

func TestWorkspaceServer_MockCRUD(t *testing.T) {
	t.Parallel()
	mgr := NewWorkspaceManager(nil)
	ctx := context.Background()

	ws := newTestWorkspace("crud-ws")
	require.NoError(t, mgr.StartWorkspace(ctx, ws))
	t.Cleanup(func() { _ = mgr.StopAll() })

	// Get the concrete WorkspaceServer (GetWorkspace returns workspace.Server interface)
	server := mgr.workspaces["crud-ws"]
	require.NotNil(t, server)

	// Initially no mocks
	mocks := server.ListMocks()
	assert.Empty(t, mocks)

	// Add a mock
	enabled := true
	mockCfg := &config.MockConfiguration{
		Type:    mock.TypeHTTP,
		Enabled: &enabled,
		HTTP: &mock.HTTPSpec{
			Matcher: &mock.HTTPMatcher{
				Method: "GET",
				Path:   "/api/test",
			},
			Response: &mock.HTTPResponse{
				StatusCode: 200,
				Body:       `{"ok":true}`,
			},
		},
	}

	err := server.AddMock(mockCfg)
	require.NoError(t, err)
	assert.NotEmpty(t, mockCfg.ID)                  // ID was generated
	assert.Equal(t, "crud-ws", mockCfg.WorkspaceID) // workspace ID was set

	// List mocks — should have 1
	mocks = server.ListMocks()
	require.Len(t, mocks, 1)
	assert.Equal(t, mockCfg.ID, mocks[0].ID)

	// Get mock by ID
	got := server.GetMock(mockCfg.ID)
	require.NotNil(t, got)
	assert.Equal(t, "/api/test", got.HTTP.Matcher.Path)

	// Get non-existent mock
	assert.Nil(t, server.GetMock("no-such-id"))

	// Delete mock
	err = server.DeleteMock(mockCfg.ID)
	assert.NoError(t, err)
	assert.Empty(t, server.ListMocks())

	// Delete non-existent mock
	err = server.DeleteMock("no-such-id")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestWorkspaceServer_ClearMocks(t *testing.T) {
	t.Parallel()
	mgr := NewWorkspaceManager(nil)
	ctx := context.Background()

	ws := newTestWorkspace("clear-ws")
	require.NoError(t, mgr.StartWorkspace(ctx, ws))
	t.Cleanup(func() { _ = mgr.StopAll() })

	server := mgr.workspaces["clear-ws"]
	require.NotNil(t, server)

	enabled := true
	for i := 0; i < 3; i++ {
		err := server.AddMock(&config.MockConfiguration{
			Type:    mock.TypeHTTP,
			Enabled: &enabled,
			HTTP: &mock.HTTPSpec{
				Matcher: &mock.HTTPMatcher{
					Method: "GET",
					Path:   fmt.Sprintf("/api/item/%d", i),
				},
				Response: &mock.HTTPResponse{
					StatusCode: 200,
					Body:       `{}`,
				},
			},
		})
		require.NoError(t, err)
	}
	require.Len(t, server.ListMocks(), 3)

	server.ClearMocks()
	assert.Empty(t, server.ListMocks())
}

// --- Mock fetcher callback ---

func TestWorkspaceManager_MockFetcher(t *testing.T) {
	t.Parallel()
	mgr := NewWorkspaceManager(nil)
	ctx := context.Background()

	fetcherCalled := false
	enabled := true
	mgr.SetMockFetcher(func(_ context.Context, wsID string) ([]*config.MockConfiguration, error) {
		fetcherCalled = true
		assert.Equal(t, "fetcher-ws", wsID)
		return []*config.MockConfiguration{
			{
				ID:      "fetched-mock-1",
				Type:    mock.TypeHTTP,
				Enabled: &enabled,
				HTTP: &mock.HTTPSpec{
					Matcher: &mock.HTTPMatcher{
						Method: "GET",
						Path:   "/api/fetched",
					},
					Response: &mock.HTTPResponse{
						StatusCode: 200,
						Body:       `{"source":"fetcher"}`,
					},
				},
			},
		}, nil
	})

	ws := newTestWorkspace("fetcher-ws")
	err := mgr.StartWorkspace(ctx, ws)
	require.NoError(t, err)
	t.Cleanup(func() { _ = mgr.StopAll() })

	assert.True(t, fetcherCalled, "mock fetcher should have been called")

	// The fetched mock should be in the workspace store
	server := mgr.workspaces["fetcher-ws"]
	require.NotNil(t, server)

	// store.List returns all mocks; the fetcher added one HTTP mock
	allMocks := server.Store().List()
	require.Len(t, allMocks, 1)
	assert.Equal(t, "fetched-mock-1", allMocks[0].ID)
}

// --- WorkspaceServer accessors ---

func TestWorkspaceServer_Accessors(t *testing.T) {
	t.Parallel()
	mgr := NewWorkspaceManager(nil)
	ctx := context.Background()

	ws := newTestWorkspace("accessor-ws")
	require.NoError(t, mgr.StartWorkspace(ctx, ws))
	t.Cleanup(func() { _ = mgr.StopAll() })

	server := mgr.workspaces["accessor-ws"]
	require.NotNil(t, server)

	assert.NotNil(t, server.Handler(), "Handler() should return non-nil after init")
	assert.NotNil(t, server.Store(), "Store() should return non-nil after init")
	assert.NotNil(t, server.Logger(), "Logger() should return non-nil after init")
}

// --- StatusInfo details ---

func TestWorkspaceServer_StatusInfo(t *testing.T) {
	t.Parallel()
	mgr := NewWorkspaceManager(nil)
	ctx := context.Background()

	ws := newTestWorkspace("statusinfo-ws")
	require.NoError(t, mgr.StartWorkspace(ctx, ws))
	t.Cleanup(func() { _ = mgr.StopAll() })

	info := mgr.GetWorkspaceStatus("statusinfo-ws")
	require.NotNil(t, info)

	assert.Equal(t, "statusinfo-ws", info.WorkspaceID)
	assert.Equal(t, "Test statusinfo-ws", info.WorkspaceName)
	assert.Equal(t, ws.HTTPPort, info.HTTPPort)
	assert.Equal(t, WorkspaceServerStatusRunning, info.Status)
	assert.Equal(t, 0, info.MockCount)    // no mocks added
	assert.Equal(t, 0, info.RequestCount) // no requests made
	assert.GreaterOrEqual(t, info.Uptime, 0)
}

// --- ReloadWorkspace ---

func TestWorkspaceManager_ReloadWorkspace(t *testing.T) {
	t.Parallel()
	mgr := NewWorkspaceManager(nil)
	ctx := context.Background()

	// ReloadWorkspace on non-existent should error
	err := mgr.ReloadWorkspace(ctx, "nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")

	// Start a workspace, then reload it (no fetcher = no-op reload)
	ws := newTestWorkspace("reload-ws")
	require.NoError(t, mgr.StartWorkspace(ctx, ws))
	t.Cleanup(func() { _ = mgr.StopAll() })

	err = mgr.ReloadWorkspace(ctx, "reload-ws")
	assert.NoError(t, err)
}
