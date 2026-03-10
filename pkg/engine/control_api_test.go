package engine

import (
	"testing"

	"github.com/getmockd/mockd/pkg/config"
	"github.com/getmockd/mockd/pkg/engine/api"
	"github.com/getmockd/mockd/pkg/mock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ============================================================================
// ControlAPIAdapter Tests
// ============================================================================

// newTestAdapter creates a ControlAPIAdapter backed by a real Server for testing.
func newTestAdapter() *ControlAPIAdapter {
	srv := NewServer(nil)
	return NewControlAPIAdapter(srv)
}

// validTestMock returns a minimal valid HTTP mock configuration.
func validTestMock(id string) *config.MockConfiguration {
	return &config.MockConfiguration{
		ID:      id,
		Type:    mock.TypeHTTP,
		Enabled: boolPtr(true),
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
}

func TestControlAPIAdapter_IsRunning(t *testing.T) {
	t.Parallel()
	adapter := newTestAdapter()

	// A newly created server that has not been started should not be running.
	assert.False(t, adapter.IsRunning(), "expected server to not be running before Start")
}

func TestControlAPIAdapter_Uptime(t *testing.T) {
	t.Parallel()
	adapter := newTestAdapter()

	// Uptime should be 0 for a server that hasn't started.
	assert.Equal(t, 0, adapter.Uptime(), "expected uptime to be 0 for a non-running server")
}

func TestControlAPIAdapter_MockCRUD(t *testing.T) {
	t.Parallel()
	adapter := newTestAdapter()

	m := validTestMock("crud-mock-1")

	// Add
	err := adapter.AddMock(m)
	require.NoError(t, err, "AddMock should succeed for a valid mock")

	// Get
	got := adapter.GetMock("crud-mock-1")
	require.NotNil(t, got, "GetMock should return the mock we just added")
	assert.Equal(t, "crud-mock-1", got.ID)
	assert.Equal(t, mock.TypeHTTP, got.Type)

	// List includes it
	mocks := adapter.ListMocks()
	require.Len(t, mocks, 1, "ListMocks should return exactly 1 mock")
	assert.Equal(t, "crud-mock-1", mocks[0].ID)

	// Delete
	err = adapter.DeleteMock("crud-mock-1")
	require.NoError(t, err, "DeleteMock should succeed for an existing mock")

	// Get returns nil after delete
	assert.Nil(t, adapter.GetMock("crud-mock-1"), "GetMock should return nil after deletion")

	// List is empty
	assert.Empty(t, adapter.ListMocks(), "ListMocks should be empty after deletion")
}

func TestControlAPIAdapter_AddMock_InvalidReturnsError(t *testing.T) {
	t.Parallel()
	adapter := newTestAdapter()

	// Mock with empty type — should fail validation.
	invalid := &config.MockConfiguration{
		ID:   "bad-mock",
		Type: "", // missing type
	}

	err := adapter.AddMock(invalid)
	assert.Error(t, err, "AddMock with empty type should return an error")
}

func TestControlAPIAdapter_DeleteMock_NotFoundReturnsError(t *testing.T) {
	t.Parallel()
	adapter := newTestAdapter()

	err := adapter.DeleteMock("nonexistent-id")
	assert.Error(t, err, "DeleteMock on nonexistent ID should return an error")
}

func TestControlAPIAdapter_StatefulOverview_Empty(t *testing.T) {
	t.Parallel()
	adapter := newTestAdapter()

	overview := adapter.GetStateOverview()
	require.NotNil(t, overview, "GetStateOverview should not be nil with initialized server")
	assert.Equal(t, 0, overview.Total, "Total resources should be 0 when none registered")
	assert.Equal(t, 0, overview.TotalItems, "TotalItems should be 0 when none registered")
	assert.Empty(t, overview.ResourceList, "ResourceList should be empty")
	assert.Empty(t, overview.Resources, "Resources should be empty")
}

func TestControlAPIAdapter_StatefulResource_Lifecycle(t *testing.T) {
	t.Parallel()
	adapter := newTestAdapter()

	// Register a resource
	res := &config.StatefulResourceConfig{
		Name:    "users",
		IDField: "id",
		SeedData: []map[string]interface{}{
			{"id": "u1", "name": "Alice"},
		},
	}
	err := adapter.RegisterStatefulResource(res)
	require.NoError(t, err, "RegisterStatefulResource should succeed")

	// Overview shows it
	overview := adapter.GetStateOverview()
	require.NotNil(t, overview)
	assert.Equal(t, 1, overview.Total, "Total resources should be 1 after registration")
	assert.Equal(t, 1, overview.TotalItems, "TotalItems should be 1 (seed data)")
	assert.Contains(t, overview.ResourceList, "users")
	require.Len(t, overview.Resources, 1)
	assert.Equal(t, "users", overview.Resources[0].Name)
	assert.Equal(t, 1, overview.Resources[0].ItemCount)

	// List items
	itemsResp, err := adapter.ListStatefulItems("users", 10, 0, "createdAt", "desc")
	require.NoError(t, err, "ListStatefulItems should succeed")
	require.NotNil(t, itemsResp)
	require.Len(t, itemsResp.Data, 1, "should have 1 seed item")
	assert.Equal(t, "u1", itemsResp.Data[0]["id"])
	assert.Equal(t, "Alice", itemsResp.Data[0]["name"])

	// Create item
	created, err := adapter.CreateStatefulItem("users", map[string]interface{}{
		"id":   "u2",
		"name": "Bob",
	})
	require.NoError(t, err, "CreateStatefulItem should succeed")
	require.NotNil(t, created)
	assert.Equal(t, "u2", created["id"])
	assert.Equal(t, "Bob", created["name"])

	// List items again — should have 2
	itemsResp, err = adapter.ListStatefulItems("users", 10, 0, "createdAt", "desc")
	require.NoError(t, err)
	assert.Len(t, itemsResp.Data, 2, "should have 2 items after create")
}

func TestControlAPIAdapter_ListStatefulItems_NotFound(t *testing.T) {
	t.Parallel()
	adapter := newTestAdapter()

	_, err := adapter.ListStatefulItems("nonexistent", 10, 0, "", "")
	assert.Error(t, err, "ListStatefulItems on nonexistent resource should error")
	assert.Contains(t, err.Error(), "not found")
}

func TestControlAPIAdapter_CreateStatefulItem_NotFound(t *testing.T) {
	t.Parallel()
	adapter := newTestAdapter()

	_, err := adapter.CreateStatefulItem("nonexistent", map[string]interface{}{"id": "x"})
	assert.Error(t, err, "CreateStatefulItem on nonexistent resource should error")
	assert.Contains(t, err.Error(), "not found")
}

func TestControlAPIAdapter_GetConfig(t *testing.T) {
	t.Parallel()
	adapter := newTestAdapter()

	cfg := adapter.GetConfig()
	require.NotNil(t, cfg, "GetConfig should not be nil")
	// Default config uses port 4280
	assert.Equal(t, 4280, cfg.HTTPPort, "default HTTP port should be 4280")
	assert.Equal(t, 1000, cfg.MaxLogEntries, "default MaxLogEntries should be 1000")
}

// ============================================================================
// Chaos Methods — nil middleware chain (NewServer(nil) doesn't set it)
// ============================================================================

func TestControlAPIAdapter_GetChaosConfig_NilMiddlewareChain(t *testing.T) {
	t.Parallel()
	adapter := newTestAdapter()

	// With no middleware chain, ChaosInjector() is nil → GetChaosConfig returns nil.
	cfg := adapter.GetChaosConfig()
	assert.Nil(t, cfg, "GetChaosConfig should return nil when middleware chain is not initialized")
}

func TestControlAPIAdapter_SetChaosConfig_NilMiddlewareChain(t *testing.T) {
	t.Parallel()
	adapter := newTestAdapter()

	// Disabling chaos (nil config) should hit SetChaosInjector(nil)
	// which returns an error because the middleware chain is nil.
	err := adapter.SetChaosConfig(nil)
	assert.Error(t, err, "SetChaosConfig(nil) should error when middleware chain is nil")

	// Disabling chaos (enabled=false) should also try to set nil injector.
	err = adapter.SetChaosConfig(&api.ChaosConfig{Enabled: false})
	assert.Error(t, err, "SetChaosConfig(enabled=false) should error when middleware chain is nil")

	// Enabling chaos should fail because middleware chain is nil.
	err = adapter.SetChaosConfig(&api.ChaosConfig{
		Enabled: true,
		ErrorRate: &api.ErrorRateConfig{
			Probability: 0.5,
			StatusCodes: []int{500},
		},
	})
	assert.Error(t, err, "SetChaosConfig(enabled=true) should error when middleware chain is nil")
}

func TestControlAPIAdapter_GetChaosStats_NilInjector(t *testing.T) {
	t.Parallel()
	adapter := newTestAdapter()

	stats := adapter.GetChaosStats()
	assert.Nil(t, stats, "GetChaosStats should return nil when no chaos injector is set")
}

func TestControlAPIAdapter_ResetChaosStats_NilInjector(t *testing.T) {
	t.Parallel()
	adapter := newTestAdapter()

	// Should not panic when there's no injector.
	assert.NotPanics(t, func() {
		adapter.ResetChaosStats()
	}, "ResetChaosStats should not panic when no chaos injector is set")
}

func TestControlAPIAdapter_GetStatefulFaultStats_NilInjector(t *testing.T) {
	t.Parallel()
	adapter := newTestAdapter()

	stats := adapter.GetStatefulFaultStats()
	assert.Nil(t, stats, "GetStatefulFaultStats should return nil when no chaos injector is set")
}

func TestControlAPIAdapter_TripCircuitBreaker_NilInjector(t *testing.T) {
	t.Parallel()
	adapter := newTestAdapter()

	err := adapter.TripCircuitBreaker("0:0")
	assert.Error(t, err, "TripCircuitBreaker should error when chaos is not enabled")
	assert.Contains(t, err.Error(), "chaos is not enabled")
}

func TestControlAPIAdapter_ResetCircuitBreaker_NilInjector(t *testing.T) {
	t.Parallel()
	adapter := newTestAdapter()

	err := adapter.ResetCircuitBreaker("0:0")
	assert.Error(t, err, "ResetCircuitBreaker should error when chaos is not enabled")
	assert.Contains(t, err.Error(), "chaos is not enabled")
}

// ============================================================================
// Protocol Handlers
// ============================================================================

func TestControlAPIAdapter_ListProtocolHandlers_Fresh(t *testing.T) {
	t.Parallel()
	adapter := newTestAdapter()

	// NewServer(nil) creates a ProtocolRegistry via ProtocolManager.
	// On a fresh server with no protocol handlers registered, the list should be empty.
	handlers := adapter.ListProtocolHandlers()
	assert.Empty(t, handlers, "ListProtocolHandlers should be empty on a fresh server")
}

func TestControlAPIAdapter_GetProtocolHandler_NotFound(t *testing.T) {
	t.Parallel()
	adapter := newTestAdapter()

	handler := adapter.GetProtocolHandler("nonexistent")
	assert.Nil(t, handler, "GetProtocolHandler should return nil for a nonexistent handler")
}

// ============================================================================
// SSE Methods
// ============================================================================

func TestControlAPIAdapter_ListSSEConnections_Fresh(t *testing.T) {
	t.Parallel()
	adapter := newTestAdapter()

	conns := adapter.ListSSEConnections()
	assert.Empty(t, conns, "ListSSEConnections should be empty on a fresh server")
}

func TestControlAPIAdapter_GetSSEConnection_NotFound(t *testing.T) {
	t.Parallel()
	adapter := newTestAdapter()

	conn := adapter.GetSSEConnection("nonexistent")
	assert.Nil(t, conn, "GetSSEConnection should return nil for nonexistent connection")
}

func TestControlAPIAdapter_CloseSSEConnection_NotFound(t *testing.T) {
	t.Parallel()
	adapter := newTestAdapter()

	// Closing a nonexistent connection — should return an error (not found or similar).
	err := adapter.CloseSSEConnection("nonexistent")
	assert.Error(t, err, "CloseSSEConnection should error for nonexistent connection")
}

func TestControlAPIAdapter_GetSSEStats_Fresh(t *testing.T) {
	t.Parallel()
	adapter := newTestAdapter()

	stats := adapter.GetSSEStats()
	// SSEHandler and manager are initialized by NewServer, so stats should be non-nil.
	require.NotNil(t, stats, "GetSSEStats should not be nil on a fresh server")
	assert.Equal(t, int64(0), stats.TotalConnections, "TotalConnections should be 0")
	assert.Equal(t, 0, stats.ActiveConnections, "ActiveConnections should be 0")
	assert.Equal(t, int64(0), stats.TotalEventsSent, "TotalEventsSent should be 0")
	assert.Equal(t, int64(0), stats.TotalBytesSent, "TotalBytesSent should be 0")
}

// ============================================================================
// WebSocket Methods
// ============================================================================

func TestControlAPIAdapter_ListWebSocketConnections_Fresh(t *testing.T) {
	t.Parallel()
	adapter := newTestAdapter()

	conns := adapter.ListWebSocketConnections()
	assert.Empty(t, conns, "ListWebSocketConnections should be empty on a fresh server")
}

func TestControlAPIAdapter_GetWebSocketConnection_NotFound(t *testing.T) {
	t.Parallel()
	adapter := newTestAdapter()

	conn := adapter.GetWebSocketConnection("nonexistent")
	assert.Nil(t, conn, "GetWebSocketConnection should return nil for nonexistent connection")
}

func TestControlAPIAdapter_CloseWebSocketConnection_NotFound(t *testing.T) {
	t.Parallel()
	adapter := newTestAdapter()

	err := adapter.CloseWebSocketConnection("nonexistent")
	assert.Error(t, err, "CloseWebSocketConnection should error for nonexistent connection")
}

func TestControlAPIAdapter_GetWebSocketStats_Fresh(t *testing.T) {
	t.Parallel()
	adapter := newTestAdapter()

	stats := adapter.GetWebSocketStats()
	// WebSocketManager is initialized by NewServer, so stats should be non-nil.
	require.NotNil(t, stats, "GetWebSocketStats should not be nil on a fresh server")
	assert.Equal(t, int64(0), stats.TotalConnections, "TotalConnections should be 0")
	assert.Equal(t, 0, stats.ActiveConnections, "ActiveConnections should be 0")
	assert.Equal(t, int64(0), stats.TotalMessagesSent, "TotalMessagesSent should be 0")
	assert.Equal(t, int64(0), stats.TotalMessagesRecv, "TotalMessagesRecv should be 0")
}

// ============================================================================
// Custom Operations
// ============================================================================

func TestControlAPIAdapter_ListCustomOperations_Fresh(t *testing.T) {
	t.Parallel()
	adapter := newTestAdapter()

	// StatefulBridge is initialized by NewServer, but no custom ops are registered.
	ops := adapter.ListCustomOperations()
	assert.Nil(t, ops, "ListCustomOperations should be nil when no ops are registered")
}

func TestControlAPIAdapter_GetCustomOperation_NotFound(t *testing.T) {
	t.Parallel()
	adapter := newTestAdapter()

	detail, err := adapter.GetCustomOperation("nonexistent")
	assert.Error(t, err, "GetCustomOperation should error for nonexistent operation")
	assert.Nil(t, detail)
	assert.Contains(t, err.Error(), "not found")
}

func TestControlAPIAdapter_RegisterCustomOperation_NilConfig(t *testing.T) {
	t.Parallel()
	adapter := newTestAdapter()

	err := adapter.RegisterCustomOperation(nil)
	assert.Error(t, err, "RegisterCustomOperation with nil config should error")
	assert.Contains(t, err.Error(), "name")
}

func TestControlAPIAdapter_RegisterCustomOperation_EmptyName(t *testing.T) {
	t.Parallel()
	adapter := newTestAdapter()

	err := adapter.RegisterCustomOperation(&config.CustomOperationConfig{
		Name: "",
		Steps: []config.CustomStepConfig{
			{Type: "set", Var: "x", Value: "1"},
		},
	})
	assert.Error(t, err, "RegisterCustomOperation with empty name should error")
}

func TestControlAPIAdapter_CustomOperation_Lifecycle(t *testing.T) {
	t.Parallel()
	adapter := newTestAdapter()

	// First, register a stateful resource that the custom op can reference.
	err := adapter.RegisterStatefulResource(&config.StatefulResourceConfig{
		Name:    "orders",
		IDField: "id",
		SeedData: []map[string]interface{}{
			{"id": "o1", "status": "pending", "amount": 100},
		},
	})
	require.NoError(t, err, "RegisterStatefulResource should succeed")

	// Register a custom operation that reads an order and sets a var.
	opCfg := &config.CustomOperationConfig{
		Name: "confirm-order",
		Steps: []config.CustomStepConfig{
			{Type: "read", Resource: "orders", ID: "input.order_id", As: "order"},
			{Type: "update", Resource: "orders", ID: "input.order_id", Set: map[string]string{
				"status": `"confirmed"`,
			}},
		},
		Response: map[string]string{
			"order_id": "order.id",
			"status":   `"confirmed"`,
		},
	}
	err = adapter.RegisterCustomOperation(opCfg)
	require.NoError(t, err, "RegisterCustomOperation should succeed")

	// List should now contain the operation.
	ops := adapter.ListCustomOperations()
	require.Len(t, ops, 1, "ListCustomOperations should return 1 operation")
	assert.Equal(t, "confirm-order", ops[0].Name)
	assert.Equal(t, 2, ops[0].StepCount)

	// Get should return details.
	detail, err := adapter.GetCustomOperation("confirm-order")
	require.NoError(t, err)
	require.NotNil(t, detail)
	assert.Equal(t, "confirm-order", detail.Name)
	assert.Len(t, detail.Steps, 2)
	assert.Equal(t, "read", detail.Steps[0].Type)
	assert.Equal(t, "update", detail.Steps[1].Type)
	assert.Contains(t, detail.Response, "order_id")

	// Execute the operation.
	result, err := adapter.ExecuteCustomOperation("confirm-order", map[string]interface{}{
		"order_id": "o1",
	})
	require.NoError(t, err, "ExecuteCustomOperation should succeed")
	require.NotNil(t, result)

	// Verify the order was updated in the store.
	items, err := adapter.ListStatefulItems("orders", 10, 0, "createdAt", "desc")
	require.NoError(t, err)
	require.Len(t, items.Data, 1)
	assert.Equal(t, "confirmed", items.Data[0]["status"])

	// Delete the custom operation.
	err = adapter.DeleteCustomOperation("confirm-order")
	require.NoError(t, err, "DeleteCustomOperation should succeed")

	// List should be empty again.
	ops = adapter.ListCustomOperations()
	assert.Nil(t, ops, "ListCustomOperations should be nil after deleting the only operation")
}

func TestControlAPIAdapter_DeleteCustomOperation_NotFound(t *testing.T) {
	t.Parallel()
	adapter := newTestAdapter()

	err := adapter.DeleteCustomOperation("nonexistent")
	assert.Error(t, err, "DeleteCustomOperation should error for nonexistent operation")
	assert.Contains(t, err.Error(), "not found")
}

func TestControlAPIAdapter_ExecuteCustomOperation_NotFound(t *testing.T) {
	t.Parallel()
	adapter := newTestAdapter()

	result, err := adapter.ExecuteCustomOperation("nonexistent", map[string]interface{}{})
	assert.Error(t, err, "ExecuteCustomOperation should error for nonexistent operation")
	assert.Nil(t, result)
}

// ============================================================================
// ClearRequestLogsByMockID
// ============================================================================

func TestControlAPIAdapter_ClearRequestLogsByMockID_NoLogs(t *testing.T) {
	t.Parallel()
	adapter := newTestAdapter()

	// On a fresh server with no request logs, clearing by mock ID should return 0.
	count := adapter.ClearRequestLogsByMockID("some-mock-id")
	assert.Equal(t, 0, count, "ClearRequestLogsByMockID should return 0 when no logs exist")
}

// ============================================================================
// ProtocolStatus
// ============================================================================

func TestControlAPIAdapter_ProtocolStatus_Fresh(t *testing.T) {
	t.Parallel()
	adapter := newTestAdapter()

	status := adapter.ProtocolStatus()
	require.NotNil(t, status, "ProtocolStatus should not be nil")
	// On a fresh server, at minimum the HTTP protocol should be present.
	// The exact contents depend on what protocols are registered.
	// Just verify no panic and a non-nil map.
	assert.IsType(t, map[string]api.ProtocolStatusInfo{}, status)
}

// ============================================================================
// Stateful Resource Extras
// ============================================================================

func TestControlAPIAdapter_GetStateResource_NotFound(t *testing.T) {
	t.Parallel()
	adapter := newTestAdapter()

	res, err := adapter.GetStateResource("nonexistent")
	assert.Error(t, err, "GetStateResource should error for nonexistent resource")
	assert.Nil(t, res)
}

func TestControlAPIAdapter_GetStateResource_Found(t *testing.T) {
	t.Parallel()
	adapter := newTestAdapter()

	err := adapter.RegisterStatefulResource(&config.StatefulResourceConfig{
		Name:    "products",
		IDField: "sku",
		SeedData: []map[string]interface{}{
			{"sku": "p1", "name": "Widget"},
			{"sku": "p2", "name": "Gadget"},
		},
	})
	require.NoError(t, err)

	res, err := adapter.GetStateResource("products")
	require.NoError(t, err)
	require.NotNil(t, res)
	assert.Equal(t, "products", res.Name)
	assert.Equal(t, "sku", res.IDField)
	assert.Equal(t, 2, res.ItemCount)
	assert.Equal(t, 2, res.SeedCount)
}

func TestControlAPIAdapter_ClearStateResource_NotFound(t *testing.T) {
	t.Parallel()
	adapter := newTestAdapter()

	count, err := adapter.ClearStateResource("nonexistent")
	assert.Error(t, err, "ClearStateResource should error for nonexistent resource")
	assert.Equal(t, 0, count)
}

func TestControlAPIAdapter_ResetState(t *testing.T) {
	t.Parallel()
	adapter := newTestAdapter()

	// Register a resource with seed data.
	err := adapter.RegisterStatefulResource(&config.StatefulResourceConfig{
		Name:    "carts",
		IDField: "id",
		SeedData: []map[string]interface{}{
			{"id": "c1", "items": 3},
		},
	})
	require.NoError(t, err)

	// Add an extra item.
	_, err = adapter.CreateStatefulItem("carts", map[string]interface{}{
		"id": "c2", "items": 5,
	})
	require.NoError(t, err)

	// Verify 2 items.
	items, err := adapter.ListStatefulItems("carts", 10, 0, "", "")
	require.NoError(t, err)
	assert.Len(t, items.Data, 2)

	// Reset to seed data.
	resp, err := adapter.ResetState("carts")
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.True(t, resp.Reset)

	// Verify only seed data remains.
	items, err = adapter.ListStatefulItems("carts", 10, 0, "", "")
	require.NoError(t, err)
	assert.Len(t, items.Data, 1, "should only have seed data after reset")
}

func TestControlAPIAdapter_GetStatefulItem_NotFoundResource(t *testing.T) {
	t.Parallel()
	adapter := newTestAdapter()

	item, err := adapter.GetStatefulItem("nonexistent", "id1")
	assert.Error(t, err)
	assert.Nil(t, item)
	assert.Contains(t, err.Error(), "not found")
}

func TestControlAPIAdapter_GetStatefulItem_NotFoundItem(t *testing.T) {
	t.Parallel()
	adapter := newTestAdapter()

	err := adapter.RegisterStatefulResource(&config.StatefulResourceConfig{
		Name:    "items",
		IDField: "id",
	})
	require.NoError(t, err)

	item, err := adapter.GetStatefulItem("items", "nonexistent")
	assert.Error(t, err)
	assert.Nil(t, item)
	assert.Contains(t, err.Error(), "not found")
}

func TestControlAPIAdapter_GetStatefulItem_Found(t *testing.T) {
	t.Parallel()
	adapter := newTestAdapter()

	err := adapter.RegisterStatefulResource(&config.StatefulResourceConfig{
		Name:    "books",
		IDField: "isbn",
		SeedData: []map[string]interface{}{
			{"isbn": "978-1", "title": "Go in Action"},
		},
	})
	require.NoError(t, err)

	item, err := adapter.GetStatefulItem("books", "978-1")
	require.NoError(t, err)
	require.NotNil(t, item)
	// ToJSON() maps the ID field to "id" regardless of the resource's idField name.
	assert.Equal(t, "978-1", item["id"])
	assert.Equal(t, "Go in Action", item["title"])
}

// ============================================================================
// UpdateMock and ListMocks by Type
// ============================================================================

func TestControlAPIAdapter_UpdateMock(t *testing.T) {
	t.Parallel()
	adapter := newTestAdapter()

	m := validTestMock("update-test")
	err := adapter.AddMock(m)
	require.NoError(t, err)

	// Update the response body.
	updated := validTestMock("update-test")
	updated.HTTP.Response.Body = `{"ok":false}`
	err = adapter.UpdateMock("update-test", updated)
	require.NoError(t, err)

	got := adapter.GetMock("update-test")
	require.NotNil(t, got)
	assert.Equal(t, `{"ok":false}`, got.HTTP.Response.Body)
}

func TestControlAPIAdapter_ClearMocks(t *testing.T) {
	t.Parallel()
	adapter := newTestAdapter()

	_ = adapter.AddMock(validTestMock("clear-1"))
	_ = adapter.AddMock(validTestMock("clear-2"))
	assert.Len(t, adapter.ListMocks(), 2)

	adapter.ClearMocks()
	assert.Empty(t, adapter.ListMocks(), "ListMocks should be empty after ClearMocks")
}

// ============================================================================
// Request Logs
// ============================================================================

func TestControlAPIAdapter_RequestLogs_Fresh(t *testing.T) {
	t.Parallel()
	adapter := newTestAdapter()

	// Fresh server should have no logs.
	logs := adapter.GetRequestLogs(nil)
	assert.Empty(t, logs, "GetRequestLogs should be empty on a fresh server")
	assert.Equal(t, 0, adapter.RequestLogCount(), "RequestLogCount should be 0")

	// GetRequestLog for a nonexistent ID should return nil.
	entry := adapter.GetRequestLog("nonexistent")
	assert.Nil(t, entry)

	// ClearRequestLogs should not panic on empty.
	assert.NotPanics(t, func() {
		adapter.ClearRequestLogs()
	})
}

func TestControlAPIAdapter_RegisterStatefulResource_NilConfig(t *testing.T) {
	t.Parallel()
	adapter := newTestAdapter()

	err := adapter.RegisterStatefulResource(nil)
	assert.Error(t, err, "RegisterStatefulResource with nil config should error")
}
