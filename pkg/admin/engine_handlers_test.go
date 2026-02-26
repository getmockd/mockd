package admin

import (
	"bytes"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/getmockd/mockd/pkg/config"
	"github.com/getmockd/mockd/pkg/mock"
	"github.com/getmockd/mockd/pkg/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHandleAssignWorkspace_ValidatesWorkspaceID(t *testing.T) {
	api := NewAPI(0, WithAPIKeyDisabled())
	if err := api.engineRegistry.Register(&store.Engine{
		ID:   "eng-1",
		Name: "test-engine",
		Host: "localhost",
		Port: 4280,
	}); err != nil {
		t.Fatalf("register engine: %v", err)
	}

	req := httptest.NewRequest(http.MethodPut, "/engines/eng-1/workspace", bytes.NewBufferString(`{}`))
	req.SetPathValue("id", "eng-1")
	rec := httptest.NewRecorder()

	api.handleAssignWorkspace(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}

	var resp ErrorResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Error != "missing_workspace_id" {
		t.Fatalf("error = %q, want %q", resp.Error, "missing_workspace_id")
	}
}

// TestSyncAdminStoreToEngine verifies the core sync mechanism.
func TestSyncAdminStoreToEngine(t *testing.T) {
	t.Run("syncs mocks and stateful resources to engine", func(t *testing.T) {
		server := newMockEngineServer()
		defer server.Close()

		api := NewAPI(0,
			WithDataDir(t.TempDir()),
			WithLocalEngineClient(server.client()),
		)

		// Populate admin store with mocks and stateful resources.
		ctx := t.Context()
		require.NoError(t, api.dataStore.Mocks().Create(ctx, &config.MockConfiguration{
			ID:   "sync-mock-1",
			Name: "Sync Mock 1",
			Type: mock.TypeHTTP,
		}))
		require.NoError(t, api.dataStore.Mocks().Create(ctx, &config.MockConfiguration{
			ID:   "sync-mock-2",
			Name: "Sync Mock 2",
			Type: mock.TypeHTTP,
		}))
		require.NoError(t, api.dataStore.StatefulResources().Create(ctx, &config.StatefulResourceConfig{
			Name:     "users",
			BasePath: "/api/users",
		}))

		// Run sync.
		api.syncAdminStoreToEngine(server.client(), "test")

		// Verify engine received the mocks (replace=true clears first, then adds).
		assert.Len(t, server.mocks, 2, "engine should have exactly 2 mocks after sync")
		assert.Contains(t, server.mocks, "sync-mock-1")
		assert.Contains(t, server.mocks, "sync-mock-2")
	})

	t.Run("skips sync when admin store is empty", func(t *testing.T) {
		server := newMockEngineServer()
		defer server.Close()

		// Pre-populate engine with a mock that should NOT be cleared
		// (sync with empty store skips entirely — no replace=true call).
		server.mocks["pre-existing"] = &config.MockConfiguration{ID: "pre-existing"}

		api := NewAPI(0, WithDataDir(t.TempDir()))

		api.syncAdminStoreToEngine(server.client(), "test-empty")

		// Engine should still have its pre-existing mock (sync was skipped).
		assert.Len(t, server.mocks, 1)
		assert.Contains(t, server.mocks, "pre-existing")
	})

	t.Run("handles nil client gracefully", func(t *testing.T) {
		api := NewAPI(0, WithDataDir(t.TempDir()))

		// Should not panic.
		api.syncAdminStoreToEngine(nil, "test-nil-client")
	})

	t.Run("handles nil data store gracefully", func(t *testing.T) {
		server := newMockEngineServer()
		defer server.Close()

		api := NewAPI(0)
		api.dataStore = nil

		// Should not panic.
		api.syncAdminStoreToEngine(server.client(), "test-nil-store")
	})

	t.Run("concurrent sync attempts are skipped", func(t *testing.T) {
		server := newMockEngineServer()
		defer server.Close()

		api := NewAPI(0, WithDataDir(t.TempDir()))

		ctx := t.Context()
		require.NoError(t, api.dataStore.Mocks().Create(ctx, &config.MockConfiguration{
			ID:   "concurrent-mock",
			Type: mock.TypeHTTP,
		}))

		// Acquire the sync lock manually to simulate an in-progress sync.
		api.engineSyncMu.Lock()

		done := make(chan struct{})
		go func() {
			// This should return immediately (TryLock fails).
			api.syncAdminStoreToEngine(server.client(), "concurrent-attempt")
			close(done)
		}()

		// Wait for the goroutine to finish (should be near-instant since it skips).
		select {
		case <-done:
			// Good — it returned without blocking.
		case <-time.After(2 * time.Second):
			t.Fatal("sync goroutine blocked — TryLock should have failed")
		}

		api.engineSyncMu.Unlock()
	})
}

// TestRegistrationTriggersSyncToEngine verifies that engine registration
// always pushes admin store mocks to the engine.
func TestRegistrationTriggersSyncToEngine(t *testing.T) {
	t.Run("first registration syncs admin store", func(t *testing.T) {
		server := newMockEngineServer()
		defer server.Close()

		api := NewAPI(0,
			WithDataDir(t.TempDir()),
			WithAPIKeyDisabled(),
		)
		defer api.Stop()

		// Populate admin store BEFORE engine registers.
		ctx := t.Context()
		require.NoError(t, api.dataStore.Mocks().Create(ctx, &config.MockConfiguration{
			ID:   "pre-reg-mock",
			Name: "Pre-Registration Mock",
			Type: mock.TypeHTTP,
		}))

		// Simulate engine registration.
		body, _ := json.Marshal(map[string]interface{}{
			"name": "test-engine",
			"host": server.Listener.Addr().(*net.TCPAddr).IP.String(),
			"port": server.Listener.Addr().(*net.TCPAddr).Port,
		})
		req := httptest.NewRequest(http.MethodPost, "/engines/register", bytes.NewReader(body))
		rec := httptest.NewRecorder()

		api.handleRegisterEngine(rec, req)
		require.Equal(t, http.StatusCreated, rec.Code, "body: %s", rec.Body.String())

		// Give the async sync goroutine time to complete.
		assert.Eventually(t, func() bool {
			return len(server.mocks) >= 1
		}, 5*time.Second, 50*time.Millisecond,
			"engine should have received admin store mocks via sync")
		assert.Contains(t, server.mocks, "pre-reg-mock")
	})

	t.Run("re-registration also syncs (localEngine already set)", func(t *testing.T) {
		server := newMockEngineServer()
		defer server.Close()

		api := NewAPI(0,
			WithDataDir(t.TempDir()),
			WithAPIKeyDisabled(),
			WithLocalEngineClient(server.client()), // localEngine already set
		)
		defer api.Stop()

		// Populate admin store.
		ctx := t.Context()
		require.NoError(t, api.dataStore.Mocks().Create(ctx, &config.MockConfiguration{
			ID:   "re-reg-mock",
			Name: "Re-Registration Mock",
			Type: mock.TypeHTTP,
		}))

		// Register — even though localEngine is already set, sync should fire.
		body, _ := json.Marshal(map[string]interface{}{
			"name": "restarted-engine",
			"host": server.Listener.Addr().(*net.TCPAddr).IP.String(),
			"port": server.Listener.Addr().(*net.TCPAddr).Port,
		})
		req := httptest.NewRequest(http.MethodPost, "/engines/register", bytes.NewReader(body))
		rec := httptest.NewRecorder()

		api.handleRegisterEngine(rec, req)
		require.Equal(t, http.StatusCreated, rec.Code)

		assert.Eventually(t, func() bool {
			return len(server.mocks) >= 1
		}, 5*time.Second, 50*time.Millisecond,
			"re-registration should sync admin store to engine")
		assert.Contains(t, server.mocks, "re-reg-mock")
	})
}

// TestHeartbeatOfflineToOnlineTriggesSync verifies that a heartbeat that
// transitions an engine from offline to online triggers a store sync.
func TestHeartbeatOfflineToOnlineTriggesSync(t *testing.T) {
	t.Run("offline-to-online heartbeat triggers sync", func(t *testing.T) {
		server := newMockEngineServer()
		defer server.Close()

		addr := server.Listener.Addr().(*net.TCPAddr)

		api := NewAPI(0,
			WithDataDir(t.TempDir()),
			WithAPIKeyDisabled(),
		)
		defer api.Stop()

		// Register an engine and set it to offline.
		engineID := "heartbeat-engine"
		require.NoError(t, api.engineRegistry.Register(&store.Engine{
			ID:     engineID,
			Name:   "heartbeat-test",
			Host:   addr.IP.String(),
			Port:   addr.Port,
			Status: store.EngineStatusOnline,
		}))
		require.NoError(t, api.engineRegistry.UpdateStatus(engineID, store.EngineStatusOffline))

		// Add a mock to admin store while engine is "offline".
		ctx := t.Context()
		require.NoError(t, api.dataStore.Mocks().Create(ctx, &config.MockConfiguration{
			ID:   "while-offline-mock",
			Name: "Created While Offline",
			Type: mock.TypeHTTP,
		}))

		// Send heartbeat — should trigger offline→online sync.
		req := httptest.NewRequest(http.MethodPost, "/engines/"+engineID+"/heartbeat", nil)
		req.SetPathValue("id", engineID)
		rec := httptest.NewRecorder()

		api.handleEngineHeartbeat(rec, req)
		require.Equal(t, http.StatusOK, rec.Code, "body: %s", rec.Body.String())

		// Verify engine is back online.
		engine, err := api.engineRegistry.Get(engineID)
		require.NoError(t, err)
		assert.Equal(t, store.EngineStatusOnline, engine.Status)

		// Give async sync time to complete.
		assert.Eventually(t, func() bool {
			return len(server.mocks) >= 1
		}, 5*time.Second, 50*time.Millisecond,
			"offline→online heartbeat should sync admin store to engine")
		assert.Contains(t, server.mocks, "while-offline-mock")
	})

	t.Run("online-to-online heartbeat does NOT trigger sync", func(t *testing.T) {
		server := newMockEngineServer()
		defer server.Close()

		addr := server.Listener.Addr().(*net.TCPAddr)

		api := NewAPI(0,
			WithDataDir(t.TempDir()),
			WithAPIKeyDisabled(),
		)
		defer api.Stop()

		// Register an engine (already online).
		engineID := "online-engine"
		require.NoError(t, api.engineRegistry.Register(&store.Engine{
			ID:     engineID,
			Name:   "online-test",
			Host:   addr.IP.String(),
			Port:   addr.Port,
			Status: store.EngineStatusOnline,
		}))

		// Add a mock to admin store.
		ctx := t.Context()
		require.NoError(t, api.dataStore.Mocks().Create(ctx, &config.MockConfiguration{
			ID:   "should-not-sync",
			Type: mock.TypeHTTP,
		}))

		// Send heartbeat while already online — should NOT trigger sync.
		req := httptest.NewRequest(http.MethodPost, "/engines/"+engineID+"/heartbeat", nil)
		req.SetPathValue("id", engineID)
		rec := httptest.NewRecorder()

		api.handleEngineHeartbeat(rec, req)
		require.Equal(t, http.StatusOK, rec.Code)

		// Give a moment for any async goroutine to run (there shouldn't be one).
		time.Sleep(200 * time.Millisecond)

		// Engine should have NO mocks (no sync triggered).
		assert.Empty(t, server.mocks, "online→online heartbeat should not trigger sync")
	})
}
