package admin

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/getmockd/mockd/pkg/admin/engineclient"
	"github.com/getmockd/mockd/pkg/config"
	"github.com/getmockd/mockd/pkg/mock"
	"github.com/getmockd/mockd/pkg/recording"
	"github.com/getmockd/mockd/pkg/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// newFakeRecordingStore creates a recording.Store pre-populated with a session.
func newFakeRecordingStore() *recording.Store {
	s := recording.NewStore()
	s.CreateSession("test-session", nil)
	return s
}

// addRecordingToStore adds a single recording to the active session.
func addRecordingToStore(s *recording.Store, id, method, path string, status int, body string) {
	rec := &recording.Recording{
		ID:        id,
		Timestamp: time.Now(),
		Request: recording.RecordedRequest{
			Method: method,
			Path:   path,
			URL:    "http://localhost" + path,
		},
		Response: recording.RecordedResponse{
			StatusCode: status,
			Body:       []byte(body),
		},
	}
	_ = s.AddRecording(rec)
}

// ---------------------------------------------------------------------------
// MockCreatorFunc unit tests
// ---------------------------------------------------------------------------

// TestMockCreatorFunc_DualWrite verifies that mockCreator() writes to the
// admin store AND pushes to the engine — the core architectural invariant.
func TestMockCreatorFunc_DualWrite(t *testing.T) {
	server := newMockEngineServer()
	defer server.Close()

	api := NewAPI(0,
		WithDataDir(t.TempDir()),
		WithLocalEngineClient(server.client()),
	)

	create := api.mockCreator()
	ctx := context.Background()

	enabled := true
	m := &config.MockConfiguration{
		Name:    "dual-write-test",
		Type:    mock.TypeHTTP,
		Enabled: &enabled,
		HTTP: &mock.HTTPSpec{
			Matcher: &mock.HTTPMatcher{Method: "GET", Path: "/test"},
		},
	}

	created, err := create(ctx, m)
	require.NoError(t, err)
	require.NotEmpty(t, created.ID, "created mock should have an ID")

	// Verify mock exists in the admin store.
	storeMock, err := api.dataStore.Mocks().Get(ctx, created.ID)
	require.NoError(t, err, "mock must be in admin store after dual-write")
	assert.Equal(t, "dual-write-test", storeMock.Name)

	// Verify mock exists in the engine.
	assert.True(t, server.hasMock(created.ID), "mock must be in engine after dual-write")
}

// TestMockCreatorFunc_StoreOnlyWhenNoEngine verifies that when no engine is
// connected, the mock is still persisted in the store.
func TestMockCreatorFunc_StoreOnlyWhenNoEngine(t *testing.T) {
	api := NewAPI(0, WithDataDir(t.TempDir()))

	create := api.mockCreator()
	ctx := context.Background()

	enabled := true
	m := &config.MockConfiguration{
		Name:    "store-only-test",
		Type:    mock.TypeHTTP,
		Enabled: &enabled,
	}

	created, err := create(ctx, m)
	require.NoError(t, err)
	require.NotEmpty(t, created.ID)

	// Verify mock is in the admin store.
	storeMock, err := api.dataStore.Mocks().Get(ctx, created.ID)
	require.NoError(t, err, "mock must be in admin store even without engine")
	assert.Equal(t, "store-only-test", storeMock.Name)
}

// TestMockCreatorFunc_RollbackOnEngineFailure verifies that when the engine
// rejects a mock, the admin store is rolled back.
func TestMockCreatorFunc_RollbackOnEngineFailure(t *testing.T) {
	// Engine that always rejects POST /mocks.
	rejectEngine := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" && r.URL.Path == "/mocks" {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(ErrorResponse{
				Error:   "validation_error",
				Message: "invalid mock",
			})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer rejectEngine.Close()

	api := NewAPI(0,
		WithDataDir(t.TempDir()),
		WithLocalEngineClient(engineclient.New(rejectEngine.URL)),
	)

	create := api.mockCreator()
	ctx := context.Background()

	enabled := true
	m := &config.MockConfiguration{
		ID:      "should-rollback",
		Name:    "rollback-test",
		Type:    mock.TypeHTTP,
		Enabled: &enabled,
	}

	_, err := create(ctx, m)
	require.Error(t, err, "should fail when engine rejects")
	assert.Contains(t, err.Error(), "engine create failed")

	// Verify mock was rolled back from the store.
	_, storeErr := api.dataStore.Mocks().Get(ctx, "should-rollback")
	assert.ErrorIs(t, storeErr, store.ErrNotFound,
		"mock must be rolled back from store on engine failure")
}

// TestMockCreatorFunc_SetsDefaults verifies that the creator fills in ID,
// workspace, and timestamps.
func TestMockCreatorFunc_SetsDefaults(t *testing.T) {
	api := NewAPI(0, WithDataDir(t.TempDir()))

	create := api.mockCreator()
	ctx := context.Background()

	m := &config.MockConfiguration{
		Name: "defaults-test",
		Type: mock.TypeHTTP,
	}

	created, err := create(ctx, m)
	require.NoError(t, err)

	assert.NotEmpty(t, created.ID, "ID should be auto-generated")
	assert.True(t, strings.HasPrefix(created.ID, "http_"),
		"ID should be type-prefixed, got: %s", created.ID)
	assert.Equal(t, store.DefaultWorkspaceID, created.WorkspaceID)
	assert.False(t, created.CreatedAt.IsZero(), "createdAt should be set")
	assert.False(t, created.UpdatedAt.IsZero(), "updatedAt should be set")
}

// ---------------------------------------------------------------------------
// End-to-end recording conversion dual-write tests
// ---------------------------------------------------------------------------

// TestConvertRecordings_DualWrite: POST /recordings/convert produces mocks
// that exist in both the admin store and the engine.
func TestConvertRecordings_DualWrite(t *testing.T) {
	server := newMockEngineServer()
	defer server.Close()

	api := NewAPI(0,
		WithDataDir(t.TempDir()),
		WithLocalEngineClient(server.client()),
	)

	// Seed the proxy manager with a recording.
	recStore := newFakeRecordingStore()
	addRecordingToStore(recStore, "rec-1", "GET", "/api/users", 200, `[{"id":1}]`)
	api.proxyManager.mu.Lock()
	api.proxyManager.store = recStore
	api.proxyManager.mu.Unlock()

	body := `{"recordingIds":["rec-1"],"deduplicate":false,"includeHeaders":false}`
	req := httptest.NewRequest("POST", "/recordings/convert", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	api.handleConvertRecordings(rec, req)

	require.Equal(t, http.StatusOK, rec.Code, "body: %s", rec.Body.String())

	var result ConvertResult
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &result))
	require.Equal(t, 1, result.Count, "should convert 1 mock")
	require.Len(t, result.MockIDs, 1)

	mockID := result.MockIDs[0]

	// Verify mock is in the admin store.
	ctx := context.Background()
	storeMock, err := api.dataStore.Mocks().Get(ctx, mockID)
	require.NoError(t, err, "converted mock must be in admin store")
	assert.Equal(t, mock.TypeHTTP, storeMock.Type)

	// Verify mock is in the engine.
	assert.True(t, server.hasMock(mockID), "converted mock must be in engine")
}

// TestConvertSingleRecording_DualWrite: POST /recordings/{id}/to-mock with
// addToServer=true writes to both store and engine.
func TestConvertSingleRecording_DualWrite(t *testing.T) {
	server := newMockEngineServer()
	defer server.Close()

	api := NewAPI(0,
		WithDataDir(t.TempDir()),
		WithLocalEngineClient(server.client()),
	)

	recStore := newFakeRecordingStore()
	addRecordingToStore(recStore, "rec-single", "POST", "/api/orders", 201, `{"id":"order-1"}`)
	api.proxyManager.mu.Lock()
	api.proxyManager.store = recStore
	api.proxyManager.mu.Unlock()

	body := `{"addToServer":true,"includeHeaders":false}`
	req := httptest.NewRequest("POST", "/recordings/rec-single/to-mock", strings.NewReader(body))
	req.SetPathValue("id", "rec-single")
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	api.handleConvertSingleRecording(rec, req)

	require.Equal(t, http.StatusOK, rec.Code, "body: %s", rec.Body.String())

	var result SingleConvertResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &result))
	require.NotNil(t, result.Mock)
	require.NotEmpty(t, result.Mock.ID)

	mockID := result.Mock.ID

	// Verify mock is in the admin store.
	ctx := context.Background()
	storeMock, err := api.dataStore.Mocks().Get(ctx, mockID)
	require.NoError(t, err, "single-convert mock must be in admin store")
	assert.Equal(t, mock.TypeHTTP, storeMock.Type)

	// Verify mock is in the engine.
	assert.True(t, server.hasMock(mockID), "single-convert mock must be in engine")
}

// TestConvertSingleRecording_NoAddToServer: when addToServer is false the
// mock is returned as a preview — NOT written to store or engine.
func TestConvertSingleRecording_NoAddToServer(t *testing.T) {
	server := newMockEngineServer()
	defer server.Close()

	api := NewAPI(0,
		WithDataDir(t.TempDir()),
		WithLocalEngineClient(server.client()),
	)

	recStore := newFakeRecordingStore()
	addRecordingToStore(recStore, "rec-preview", "GET", "/preview", 200, `{}`)
	api.proxyManager.mu.Lock()
	api.proxyManager.store = recStore
	api.proxyManager.mu.Unlock()

	req := httptest.NewRequest("POST", "/recordings/rec-preview/to-mock", strings.NewReader(`{}`))
	req.SetPathValue("id", "rec-preview")
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	api.handleConvertSingleRecording(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	var result SingleConvertResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &result))
	require.NotNil(t, result.Mock)

	// Mock should NOT be in the store (preview only).
	ctx := context.Background()
	_, err := api.dataStore.Mocks().Get(ctx, result.Mock.ID)
	assert.ErrorIs(t, err, store.ErrNotFound,
		"preview mock should not be in store")

	// Mock should NOT be in the engine.
	assert.Equal(t, 0, server.mockCount(), "preview mock should not be in engine")
}

// TestConvertSession_DualWrite: POST /recordings/sessions/{id}/to-mocks with
// addToServer=true writes all converted mocks to both store and engine.
func TestConvertSession_DualWrite(t *testing.T) {
	server := newMockEngineServer()
	defer server.Close()

	api := NewAPI(0,
		WithDataDir(t.TempDir()),
		WithLocalEngineClient(server.client()),
	)

	recStore := newFakeRecordingStore()
	addRecordingToStore(recStore, "sess-rec-1", "GET", "/api/users", 200, `[]`)
	addRecordingToStore(recStore, "sess-rec-2", "POST", "/api/users", 201, `{"id":1}`)
	api.proxyManager.mu.Lock()
	api.proxyManager.store = recStore
	api.proxyManager.mu.Unlock()

	// Get the session ID from the store.
	sessions := recStore.ListSessions()
	require.NotEmpty(t, sessions)
	sessionID := sessions[0].ID

	body := `{"addToServer":true,"duplicates":"all"}`
	req := httptest.NewRequest("POST", "/recordings/sessions/"+sessionID+"/to-mocks", strings.NewReader(body))
	req.SetPathValue("id", sessionID)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	api.handleConvertSession(rec, req)

	require.Equal(t, http.StatusOK, rec.Code, "body: %s", rec.Body.String())

	var result SessionConvertResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &result))
	require.GreaterOrEqual(t, len(result.MockIDs), 2, "should convert at least 2 mocks")
	assert.Equal(t, result.Added, len(result.MockIDs), "all mocks should be added")

	// Verify each mock is in both store and engine.
	ctx := context.Background()
	for _, mockID := range result.MockIDs {
		_, err := api.dataStore.Mocks().Get(ctx, mockID)
		assert.NoError(t, err, "session mock %s must be in admin store", mockID)

		assert.True(t, server.hasMock(mockID), "session mock %s must be in engine", mockID)
	}
}
