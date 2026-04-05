package integration

import (
	"bufio"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/getmockd/mockd/pkg/config"
	"github.com/getmockd/mockd/pkg/engine"
	"github.com/getmockd/mockd/pkg/mock"
	"github.com/getmockd/mockd/pkg/sse"
)

// ============================================================================
// SSE Test Helpers
// ============================================================================

// setupSSEMockServer creates a test server and adapter for SSE mock management.
func setupSSEMockServer(t *testing.T) (*httptest.Server, *engine.ControlAPIAdapter) {
	t.Helper()
	cfg := config.DefaultServerConfiguration()
	srv := engine.NewServer(cfg)
	adapter := engine.NewControlAPIAdapter(srv)

	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(func() { ts.Close() })
	return ts, adapter
}

// getSSEManager extracts the SSE ConnectionManager from the test server.
func getSSEManager(t *testing.T, ts *httptest.Server) *sse.SSEConnectionManager {
	t.Helper()
	handler := ts.Config.Handler
	require.NotNil(t, handler, "handler is nil")

	engineHandler, ok := handler.(*engine.Handler)
	require.True(t, ok, "handler is not *engine.Handler")

	sseHandler := engineHandler.SSEHandler()
	require.NotNil(t, sseHandler, "SSE handler is nil")

	mgr := sseHandler.GetManager()
	require.NotNil(t, mgr, "SSE connection manager is nil")
	return mgr
}

// newSSEMock creates a minimal SSE mock configuration that streams events
// with a keepalive to hold the connection open.
func newSSEMock(path string) *mock.Mock {
	return &mock.Mock{
		Type: mock.TypeHTTP,
		HTTP: &mock.HTTPSpec{
			Matcher: &mock.HTTPMatcher{
				Method: "GET",
				Path:   path,
			},
			SSE: &mock.SSEConfig{
				Generator: &mock.SSEEventGenerator{
					Type:  "sequence",
					Count: 0, // infinite
					Sequence: &mock.SSESequenceGenerator{
						Start:     1,
						Increment: 1,
					},
				},
				Lifecycle: mock.SSELifecycleConfig{
					KeepaliveInterval: 300, // 5min keepalive keeps connection alive
				},
			},
		},
	}
}

// connectSSE opens an SSE connection and waits until the first event is received,
// confirming the server has registered the connection. Returns a cancel func
// that closes the connection.
func connectSSE(t *testing.T, url string) context.CancelFunc {
	t.Helper()

	ctx, cancel := context.WithCancel(context.Background())

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	require.NoError(t, err)
	req.Header.Set("Accept", "text/event-stream")

	client := &http.Client{}
	resp, err := client.Do(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	// Read until we get at least one SSE event (data: line), confirming connection is tracked.
	ready := make(chan struct{})
	go func() {
		scanner := bufio.NewScanner(resp.Body)
		signalled := false
		for scanner.Scan() {
			line := scanner.Text()
			if !signalled && len(line) > 0 {
				close(ready)
				signalled = true
			}
		}
		// Body closed by context cancellation
	}()

	select {
	case <-ready:
		// Connection is active and streaming
	case <-time.After(5 * time.Second):
		cancel()
		t.Fatal("timeout waiting for SSE event")
	}

	return cancel
}

// waitForSSECount polls until the SSE manager reports the expected connection count
// or the timeout expires.
func waitForSSECount(t *testing.T, mgr *sse.SSEConnectionManager, expected int, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if mgr.Count() == expected {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("expected %d SSE connections, got %d", expected, mgr.Count())
}

// ============================================================================
// SSE Auto-Disconnect Tests
// ============================================================================

// TestSSE_MockUpdate_ClosesActiveConnections verifies that updating an SSE mock
// disconnects all active SSE clients so they reconnect with the new configuration.
func TestSSE_MockUpdate_ClosesActiveConnections(t *testing.T) {
	ts, adapter := setupSSEMockServer(t)
	mgr := getSSEManager(t, ts)

	// Create an SSE mock.
	mockCfg := newSSEMock("/sse/update-test")
	require.NoError(t, adapter.AddMock(mockCfg))

	// Connect an SSE client.
	cancelSSE := connectSSE(t, ts.URL+"/sse/update-test")
	defer cancelSSE()

	// Wait for the connection to be registered.
	waitForSSECount(t, mgr, 1, 2*time.Second)

	// Update the mock — this should disconnect all active SSE clients.
	mocks := adapter.ListMocks()
	require.Len(t, mocks, 1)
	updated := *mocks[0]
	updated.HTTP.SSE.Generator.Sequence.Start = 100
	require.NoError(t, adapter.UpdateMock(updated.ID, &updated))

	// Connections should be closed.
	waitForSSECount(t, mgr, 0, 2*time.Second)
}

// TestSSE_MockDelete_ClosesActiveConnections verifies that deleting an SSE mock
// disconnects all active SSE clients.
func TestSSE_MockDelete_ClosesActiveConnections(t *testing.T) {
	ts, adapter := setupSSEMockServer(t)
	mgr := getSSEManager(t, ts)

	mockCfg := newSSEMock("/sse/delete-test")
	require.NoError(t, adapter.AddMock(mockCfg))

	cancelSSE := connectSSE(t, ts.URL+"/sse/delete-test")
	defer cancelSSE()

	waitForSSECount(t, mgr, 1, 2*time.Second)

	// Delete the mock — SSE clients must be disconnected.
	mocks := adapter.ListMocks()
	require.Len(t, mocks, 1)
	require.NoError(t, adapter.DeleteMock(mocks[0].ID))

	waitForSSECount(t, mgr, 0, 2*time.Second)
}

// TestSSE_MockToggleDisable_ClosesActiveConnections verifies that disabling an SSE
// mock (toggle to enabled=false) disconnects all active SSE clients.
func TestSSE_MockToggleDisable_ClosesActiveConnections(t *testing.T) {
	ts, adapter := setupSSEMockServer(t)
	mgr := getSSEManager(t, ts)

	mockCfg := newSSEMock("/sse/toggle-test")
	require.NoError(t, adapter.AddMock(mockCfg))

	cancelSSE := connectSSE(t, ts.URL+"/sse/toggle-test")
	defer cancelSSE()

	waitForSSECount(t, mgr, 1, 2*time.Second)

	// Disable the mock — SSE clients must be disconnected.
	mocks := adapter.ListMocks()
	require.Len(t, mocks, 1)
	disabled := *mocks[0]
	enabled := false
	disabled.Enabled = &enabled
	require.NoError(t, adapter.UpdateMock(disabled.ID, &disabled))

	waitForSSECount(t, mgr, 0, 2*time.Second)
}

// TestSSE_ClearMocks_ClosesActiveConnections verifies that clearing all mocks
// disconnects all active SSE clients.
func TestSSE_ClearMocks_ClosesActiveConnections(t *testing.T) {
	ts, adapter := setupSSEMockServer(t)
	mgr := getSSEManager(t, ts)

	mockCfg := newSSEMock("/sse/clear-test")
	require.NoError(t, adapter.AddMock(mockCfg))

	cancelSSE := connectSSE(t, ts.URL+"/sse/clear-test")
	defer cancelSSE()

	waitForSSECount(t, mgr, 1, 2*time.Second)

	// Clear all mocks — SSE clients must be disconnected.
	adapter.ClearMocks()

	waitForSSECount(t, mgr, 0, 2*time.Second)
}

// TestSSE_MultipleConnections_AllDisconnected verifies that multiple SSE clients
// connected to the same mock are all disconnected when the mock is deleted.
func TestSSE_MultipleConnections_AllDisconnected(t *testing.T) {
	ts, adapter := setupSSEMockServer(t)
	mgr := getSSEManager(t, ts)

	mockCfg := newSSEMock("/sse/multi-test")
	require.NoError(t, adapter.AddMock(mockCfg))

	// Connect 3 SSE clients.
	for i := 0; i < 3; i++ {
		cancel := connectSSE(t, ts.URL+"/sse/multi-test")
		defer cancel()
	}

	waitForSSECount(t, mgr, 3, 2*time.Second)

	// Delete the mock — all 3 clients must be disconnected.
	mocks := adapter.ListMocks()
	require.Len(t, mocks, 1)
	require.NoError(t, adapter.DeleteMock(mocks[0].ID))

	waitForSSECount(t, mgr, 0, 2*time.Second)
}

// TestSSE_UpdateDoesNotAffectOtherMocks verifies that updating one SSE mock
// only disconnects clients on that mock, not clients on other SSE mocks.
func TestSSE_UpdateDoesNotAffectOtherMocks(t *testing.T) {
	ts, adapter := setupSSEMockServer(t)
	mgr := getSSEManager(t, ts)

	// Create two SSE mocks.
	mock1 := newSSEMock("/sse/mock1")
	require.NoError(t, adapter.AddMock(mock1))
	mock2 := newSSEMock("/sse/mock2")
	require.NoError(t, adapter.AddMock(mock2))

	// Connect a client to each.
	cancel1 := connectSSE(t, ts.URL+"/sse/mock1")
	defer cancel1()
	cancel2 := connectSSE(t, ts.URL+"/sse/mock2")
	defer cancel2()

	waitForSSECount(t, mgr, 2, 2*time.Second)

	// Update mock1 — only mock1 connections should drop.
	mocks := adapter.ListMocks()
	require.Len(t, mocks, 2)

	var target *mock.Mock
	for _, m := range mocks {
		if m.HTTP != nil && m.HTTP.Matcher != nil && m.HTTP.Matcher.Path == "/sse/mock1" {
			target = m
			break
		}
	}
	require.NotNil(t, target, "could not find /sse/mock1 mock")

	updated := *target
	updated.HTTP.SSE.Generator.Sequence.Start = 100
	require.NoError(t, adapter.UpdateMock(updated.ID, &updated))

	// mock1 connections should be closed, mock2 should remain.
	// Wait for mock1 connections to drop.
	time.Sleep(100 * time.Millisecond)
	assert.Equal(t, 1, mgr.Count(), "expected 1 remaining SSE connection (mock2)")
}
