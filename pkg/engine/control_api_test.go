package engine

import (
	"testing"

	"github.com/getmockd/mockd/pkg/config"
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
