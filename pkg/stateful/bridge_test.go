package stateful

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupBridgeTest(t *testing.T) (*Bridge, *MetricsObserver) {
	t.Helper()
	store := NewStateStore()
	obs := NewMetricsObserver()
	store.SetObserver(obs)

	err := store.Register(&ResourceConfig{
		Name:     "users",
		BasePath: "/api/users",
		SeedData: []map[string]interface{}{
			{"id": "u1", "name": "Alice", "email": "alice@example.com"},
			{"id": "u2", "name": "Bob", "email": "bob@example.com"},
		},
	})
	require.NoError(t, err)

	bridge := NewBridge(store)
	return bridge, obs
}

// --- Get tests ---

func TestBridge_Get_Success(t *testing.T) {
	bridge, obs := setupBridgeTest(t)

	result := bridge.Execute(context.Background(), &OperationRequest{
		Resource:   "users",
		Action:     ActionGet,
		ResourceID: "u1",
	})

	assert.Equal(t, StatusSuccess, result.Status)
	assert.Nil(t, result.Error)
	assert.NotNil(t, result.Item)
	assert.Equal(t, "u1", result.Item.ID)
	assert.Equal(t, "Alice", result.Item.Data["name"])
	assert.Equal(t, int64(1), obs.Snapshot().ReadCount)
}

func TestBridge_Get_NotFound(t *testing.T) {
	bridge, obs := setupBridgeTest(t)

	result := bridge.Execute(context.Background(), &OperationRequest{
		Resource:   "users",
		Action:     ActionGet,
		ResourceID: "nonexistent",
	})

	assert.Equal(t, StatusNotFound, result.Status)
	assert.Error(t, result.Error)
	assert.IsType(t, &NotFoundError{}, result.Error)
	assert.Equal(t, int64(1), obs.Snapshot().ErrorCount)
}

func TestBridge_Get_MissingID(t *testing.T) {
	bridge, _ := setupBridgeTest(t)

	result := bridge.Execute(context.Background(), &OperationRequest{
		Resource: "users",
		Action:   ActionGet,
		// ResourceID intentionally omitted
	})

	assert.Equal(t, StatusValidationError, result.Status)
	assert.Error(t, result.Error)
}

func TestBridge_Get_ResourceNotFound(t *testing.T) {
	bridge, obs := setupBridgeTest(t)

	result := bridge.Execute(context.Background(), &OperationRequest{
		Resource:   "nonexistent",
		Action:     ActionGet,
		ResourceID: "u1",
	})

	assert.Equal(t, StatusNotFound, result.Status)
	assert.Error(t, result.Error)
	assert.Equal(t, int64(1), obs.Snapshot().ErrorCount)
}

// --- List tests ---

func TestBridge_List_Success(t *testing.T) {
	bridge, obs := setupBridgeTest(t)

	result := bridge.Execute(context.Background(), &OperationRequest{
		Resource: "users",
		Action:   ActionList,
	})

	assert.Equal(t, StatusSuccess, result.Status)
	assert.Nil(t, result.Error)
	assert.NotNil(t, result.List)
	assert.Equal(t, 2, result.List.Meta.Total)
	assert.Len(t, result.List.Data, 2)
	assert.Equal(t, int64(1), obs.Snapshot().ListCount)
}

func TestBridge_List_WithFilter(t *testing.T) {
	bridge, _ := setupBridgeTest(t)

	result := bridge.Execute(context.Background(), &OperationRequest{
		Resource: "users",
		Action:   ActionList,
		Filter: &QueryFilter{
			Limit:   1,
			Offset:  0,
			Sort:    "name",
			Order:   "asc",
			Filters: map[string]string{},
		},
	})

	assert.Equal(t, StatusSuccess, result.Status)
	assert.NotNil(t, result.List)
	assert.Len(t, result.List.Data, 1)
	assert.Equal(t, 2, result.List.Meta.Total)
}

// --- Create tests ---

func TestBridge_Create_Success(t *testing.T) {
	bridge, obs := setupBridgeTest(t)

	result := bridge.Execute(context.Background(), &OperationRequest{
		Resource: "users",
		Action:   ActionCreate,
		Data:     map[string]interface{}{"name": "Charlie", "email": "charlie@example.com"},
	})

	assert.Equal(t, StatusCreated, result.Status)
	assert.Nil(t, result.Error)
	assert.NotNil(t, result.Item)
	assert.Equal(t, "Charlie", result.Item.Data["name"])
	assert.NotEmpty(t, result.Item.ID)
	assert.Equal(t, int64(1), obs.Snapshot().CreateCount)
}

func TestBridge_Create_WithID(t *testing.T) {
	bridge, _ := setupBridgeTest(t)

	result := bridge.Execute(context.Background(), &OperationRequest{
		Resource: "users",
		Action:   ActionCreate,
		Data:     map[string]interface{}{"id": "u3", "name": "Charlie"},
	})

	assert.Equal(t, StatusCreated, result.Status)
	assert.Equal(t, "u3", result.Item.ID)
}

func TestBridge_Create_DuplicateID(t *testing.T) {
	bridge, obs := setupBridgeTest(t)

	result := bridge.Execute(context.Background(), &OperationRequest{
		Resource: "users",
		Action:   ActionCreate,
		Data:     map[string]interface{}{"id": "u1", "name": "Duplicate"},
	})

	assert.Equal(t, StatusConflict, result.Status)
	assert.Error(t, result.Error)
	assert.IsType(t, &ConflictError{}, result.Error)
	assert.Equal(t, int64(1), obs.Snapshot().ErrorCount)
}

func TestBridge_Create_NilData(t *testing.T) {
	bridge, _ := setupBridgeTest(t)

	result := bridge.Execute(context.Background(), &OperationRequest{
		Resource: "users",
		Action:   ActionCreate,
		// Data intentionally nil
	})

	// Should succeed with auto-generated ID and empty data
	assert.Equal(t, StatusCreated, result.Status)
	assert.NotNil(t, result.Item)
	assert.NotEmpty(t, result.Item.ID)
}

// --- Update tests ---

func TestBridge_Update_Success(t *testing.T) {
	bridge, obs := setupBridgeTest(t)

	result := bridge.Execute(context.Background(), &OperationRequest{
		Resource:   "users",
		Action:     ActionUpdate,
		ResourceID: "u1",
		Data:       map[string]interface{}{"name": "Alice Updated", "email": "alice-new@example.com"},
	})

	assert.Equal(t, StatusSuccess, result.Status)
	assert.Nil(t, result.Error)
	assert.NotNil(t, result.Item)
	assert.Equal(t, "u1", result.Item.ID)
	assert.Equal(t, "Alice Updated", result.Item.Data["name"])
	assert.Equal(t, int64(1), obs.Snapshot().UpdateCount)
}

func TestBridge_Update_NotFound(t *testing.T) {
	bridge, _ := setupBridgeTest(t)

	result := bridge.Execute(context.Background(), &OperationRequest{
		Resource:   "users",
		Action:     ActionUpdate,
		ResourceID: "nonexistent",
		Data:       map[string]interface{}{"name": "Ghost"},
	})

	assert.Equal(t, StatusNotFound, result.Status)
	assert.Error(t, result.Error)
}

func TestBridge_Update_MissingID(t *testing.T) {
	bridge, _ := setupBridgeTest(t)

	result := bridge.Execute(context.Background(), &OperationRequest{
		Resource: "users",
		Action:   ActionUpdate,
		Data:     map[string]interface{}{"name": "No ID"},
	})

	assert.Equal(t, StatusValidationError, result.Status)
}

// --- Patch tests ---

func TestBridge_Patch_Success(t *testing.T) {
	bridge, obs := setupBridgeTest(t)

	result := bridge.Execute(context.Background(), &OperationRequest{
		Resource:   "users",
		Action:     ActionPatch,
		ResourceID: "u1",
		Data:       map[string]interface{}{"email": "alice-patched@example.com"},
	})

	assert.Equal(t, StatusSuccess, result.Status)
	assert.Nil(t, result.Error)
	assert.NotNil(t, result.Item)
	assert.Equal(t, "Alice", result.Item.Data["name"])                      // preserved
	assert.Equal(t, "alice-patched@example.com", result.Item.Data["email"]) // updated
	assert.Equal(t, int64(1), obs.Snapshot().UpdateCount)
}

func TestBridge_Patch_NotFound(t *testing.T) {
	bridge, _ := setupBridgeTest(t)

	result := bridge.Execute(context.Background(), &OperationRequest{
		Resource:   "users",
		Action:     ActionPatch,
		ResourceID: "nonexistent",
		Data:       map[string]interface{}{"name": "Ghost"},
	})

	assert.Equal(t, StatusNotFound, result.Status)
}

// --- Delete tests ---

func TestBridge_Delete_Success(t *testing.T) {
	bridge, obs := setupBridgeTest(t)

	result := bridge.Execute(context.Background(), &OperationRequest{
		Resource:   "users",
		Action:     ActionDelete,
		ResourceID: "u1",
	})

	assert.Equal(t, StatusSuccess, result.Status)
	assert.Nil(t, result.Error)
	assert.Equal(t, int64(1), obs.Snapshot().DeleteCount)

	// Verify it's gone
	getResult := bridge.Execute(context.Background(), &OperationRequest{
		Resource:   "users",
		Action:     ActionGet,
		ResourceID: "u1",
	})
	assert.Equal(t, StatusNotFound, getResult.Status)
}

func TestBridge_Delete_NotFound(t *testing.T) {
	bridge, _ := setupBridgeTest(t)

	result := bridge.Execute(context.Background(), &OperationRequest{
		Resource:   "users",
		Action:     ActionDelete,
		ResourceID: "nonexistent",
	})

	assert.Equal(t, StatusNotFound, result.Status)
	assert.Error(t, result.Error)
}

func TestBridge_Delete_MissingID(t *testing.T) {
	bridge, _ := setupBridgeTest(t)

	result := bridge.Execute(context.Background(), &OperationRequest{
		Resource: "users",
		Action:   ActionDelete,
	})

	assert.Equal(t, StatusValidationError, result.Status)
}

// --- Edge cases ---

func TestBridge_NilRequest(t *testing.T) {
	bridge, _ := setupBridgeTest(t)

	result := bridge.Execute(context.Background(), nil)
	assert.Equal(t, StatusError, result.Status)
	assert.Error(t, result.Error)
}

func TestBridge_UnsupportedAction(t *testing.T) {
	bridge, _ := setupBridgeTest(t)

	result := bridge.Execute(context.Background(), &OperationRequest{
		Resource: "users",
		Action:   "invalid",
	})

	assert.Equal(t, StatusError, result.Status)
	assert.Error(t, result.Error)
}

func TestBridge_CRUD_Lifecycle(t *testing.T) {
	bridge, obs := setupBridgeTest(t)
	ctx := context.Background()

	// Create
	createResult := bridge.Execute(ctx, &OperationRequest{
		Resource: "users",
		Action:   ActionCreate,
		Data:     map[string]interface{}{"id": "lifecycle-1", "name": "Test User"},
	})
	require.Equal(t, StatusCreated, createResult.Status)

	// Read
	getResult := bridge.Execute(ctx, &OperationRequest{
		Resource:   "users",
		Action:     ActionGet,
		ResourceID: "lifecycle-1",
	})
	require.Equal(t, StatusSuccess, getResult.Status)
	assert.Equal(t, "Test User", getResult.Item.Data["name"])

	// Update
	updateResult := bridge.Execute(ctx, &OperationRequest{
		Resource:   "users",
		Action:     ActionUpdate,
		ResourceID: "lifecycle-1",
		Data:       map[string]interface{}{"name": "Updated User"},
	})
	require.Equal(t, StatusSuccess, updateResult.Status)
	assert.Equal(t, "Updated User", updateResult.Item.Data["name"])

	// Patch
	patchResult := bridge.Execute(ctx, &OperationRequest{
		Resource:   "users",
		Action:     ActionPatch,
		ResourceID: "lifecycle-1",
		Data:       map[string]interface{}{"email": "test@example.com"},
	})
	require.Equal(t, StatusSuccess, patchResult.Status)
	assert.Equal(t, "Updated User", patchResult.Item.Data["name"]) // preserved
	assert.Equal(t, "test@example.com", patchResult.Item.Data["email"])

	// List (should have 3: 2 seed + 1 created)
	listResult := bridge.Execute(ctx, &OperationRequest{
		Resource: "users",
		Action:   ActionList,
	})
	require.Equal(t, StatusSuccess, listResult.Status)
	assert.Equal(t, 3, listResult.List.Meta.Total)

	// Delete
	deleteResult := bridge.Execute(ctx, &OperationRequest{
		Resource:   "users",
		Action:     ActionDelete,
		ResourceID: "lifecycle-1",
	})
	require.Equal(t, StatusSuccess, deleteResult.Status)

	// Verify observer captured all operations
	snap := obs.Snapshot()
	assert.Equal(t, int64(1), snap.CreateCount)
	assert.Equal(t, int64(1), snap.ReadCount)
	assert.Equal(t, int64(2), snap.UpdateCount) // update + patch
	assert.Equal(t, int64(1), snap.DeleteCount)
	assert.Equal(t, int64(1), snap.ListCount)
}

// --- ErrorCode tests ---

func TestErrorCode_NotFoundError(t *testing.T) {
	err := &NotFoundError{Resource: "users", ID: "1"}
	assert.Equal(t, ErrCodeNotFound, err.ErrorCode())
	assert.Equal(t, 404, err.StatusCode())
	assert.Equal(t, "NOT_FOUND", err.ErrorCode().String())
}

func TestErrorCode_ConflictError(t *testing.T) {
	err := &ConflictError{Resource: "users", ID: "1"}
	assert.Equal(t, ErrCodeConflict, err.ErrorCode())
	assert.Equal(t, 409, err.StatusCode())
	assert.Equal(t, "CONFLICT", err.ErrorCode().String())
}

func TestErrorCode_ValidationError(t *testing.T) {
	err := &ValidationError{Message: "bad input", Field: "name"}
	assert.Equal(t, ErrCodeValidation, err.ErrorCode())
	assert.Equal(t, 400, err.StatusCode())
}

func TestErrorCode_PayloadTooLargeError(t *testing.T) {
	err := &PayloadTooLargeError{MaxSize: 1024}
	assert.Equal(t, ErrCodePayloadTooLarge, err.ErrorCode())
	assert.Equal(t, 413, err.StatusCode())
}

func TestErrorCode_CapacityError(t *testing.T) {
	err := &CapacityError{Resource: "users", MaxItems: 100}
	assert.Equal(t, ErrCodeCapacityExceeded, err.ErrorCode())
	assert.Equal(t, 507, err.StatusCode())
}

func TestGetErrorCode_WithErrorCodeError(t *testing.T) {
	err := &NotFoundError{Resource: "users"}
	assert.Equal(t, ErrCodeNotFound, GetErrorCode(err))
}

func TestGetErrorCode_WithGenericError(t *testing.T) {
	err := assert.AnError
	assert.Equal(t, ErrCodeInternal, GetErrorCode(err))
}

func TestErrorCode_String_All(t *testing.T) {
	tests := []struct {
		code     ErrorCode
		expected string
	}{
		{ErrCodeNotFound, "NOT_FOUND"},
		{ErrCodeConflict, "CONFLICT"},
		{ErrCodeValidation, "VALIDATION_ERROR"},
		{ErrCodePayloadTooLarge, "PAYLOAD_TOO_LARGE"},
		{ErrCodeCapacityExceeded, "CAPACITY_EXCEEDED"},
		{ErrCodeInternal, "INTERNAL_ERROR"},
		{ErrorCode(99), "UNKNOWN"},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.expected, tt.code.String())
	}
}

func TestBridge_Store(t *testing.T) {
	bridge, _ := setupBridgeTest(t)
	assert.NotNil(t, bridge.Store())
	assert.Equal(t, 2, bridge.Store().Get("users").Count())
}

// --- Bridge-only resources (no basePath, no HTTP) ---

func TestBridge_BridgeOnlyResource_CRUD(t *testing.T) {
	// Resource without basePath — accessible via Bridge but NOT via HTTP path matching
	store := NewStateStore()
	err := store.Register(&ResourceConfig{
		Name: "internal-data",
		// No BasePath — bridge-only
	})
	require.NoError(t, err)

	bridge := NewBridge(store)
	ctx := context.Background()

	// Create via Bridge works
	result := bridge.Execute(ctx, &OperationRequest{
		Resource: "internal-data",
		Action:   ActionCreate,
		Data:     map[string]interface{}{"key": "value"},
	})
	require.Equal(t, StatusCreated, result.Status)
	require.NotNil(t, result.Item)
	itemID := result.Item.ID

	// Get via Bridge works
	getResult := bridge.Execute(ctx, &OperationRequest{
		Resource:   "internal-data",
		Action:     ActionGet,
		ResourceID: itemID,
	})
	require.Equal(t, StatusSuccess, getResult.Status)
	assert.Equal(t, "value", getResult.Item.Data["key"])

	// HTTP path matching returns false (no HTTP routing)
	resource := store.Get("internal-data")
	require.NotNil(t, resource)
	_, _, matched := resource.MatchPath("/internal-data")
	assert.False(t, matched, "bridge-only resource should not match HTTP paths")
	_, _, matched2 := resource.MatchPath("/api/internal-data/" + itemID)
	assert.False(t, matched2, "bridge-only resource should not match any path")

	// Store.MatchPath also skips it
	r, _, _ := store.MatchPath("/internal-data")
	assert.Nil(t, r, "store should not match bridge-only resources via path")
}

func TestBridge_BridgeOnlyResource_Registration(t *testing.T) {
	store := NewStateStore()

	// Empty basePath is allowed
	err := store.Register(&ResourceConfig{
		Name: "bridge-only",
	})
	require.NoError(t, err)

	// Non-slash basePath still rejected
	err = store.Register(&ResourceConfig{
		Name:     "bad-path",
		BasePath: "no-slash",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "must start with /")
}
