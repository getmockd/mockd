package admin

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/getmockd/mockd/pkg/admin/engineclient"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ============================================================================
// handleListWebSocketConnections
// ============================================================================

func TestHandleListWebSocketConnections_NoEngine_ReturnsEmptyList(t *testing.T) {
	api := NewAPI(0, WithDataDir(t.TempDir()))
	defer func() { _ = api.Stop() }()

	req := httptest.NewRequest(http.MethodGet, "/websocket/connections", nil)
	rec := httptest.NewRecorder()

	api.handleListWebSocketConnections(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var resp WebSocketConnectionListResponse
	err := json.Unmarshal(rec.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Empty(t, resp.Connections)
	assert.NotNil(t, resp.Stats.ConnectionsByMock)
}

func TestHandleListWebSocketConnections_EngineUnavailable_Returns503(t *testing.T) {
	api := NewAPI(0, WithDataDir(t.TempDir()), WithLocalEngineClient(engineclient.New("http://127.0.0.1:1")))
	defer func() { _ = api.Stop() }()

	req := httptest.NewRequest(http.MethodGet, "/websocket/connections", nil)
	rec := httptest.NewRecorder()

	api.handleListWebSocketConnections(rec, req)

	assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
}

// ============================================================================
// handleGetWebSocketConnection
// ============================================================================

func TestHandleGetWebSocketConnection_MissingID_Returns400(t *testing.T) {
	api := NewAPI(0, WithDataDir(t.TempDir()))
	defer func() { _ = api.Stop() }()

	req := httptest.NewRequest(http.MethodGet, "/websocket/connections/", nil)
	// PathValue "id" intentionally not set → empty string
	rec := httptest.NewRecorder()

	api.handleGetWebSocketConnection(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestHandleGetWebSocketConnection_NoEngine_Returns404(t *testing.T) {
	api := NewAPI(0, WithDataDir(t.TempDir()))
	defer func() { _ = api.Stop() }()

	req := httptest.NewRequest(http.MethodGet, "/websocket/connections/conn-1", nil)
	req.SetPathValue("id", "conn-1")
	rec := httptest.NewRecorder()

	api.handleGetWebSocketConnection(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestHandleGetWebSocketConnection_EngineUnavailable_Returns503(t *testing.T) {
	api := NewAPI(0, WithDataDir(t.TempDir()), WithLocalEngineClient(engineclient.New("http://127.0.0.1:1")))
	defer func() { _ = api.Stop() }()

	req := httptest.NewRequest(http.MethodGet, "/websocket/connections/conn-1", nil)
	req.SetPathValue("id", "conn-1")
	rec := httptest.NewRecorder()

	api.handleGetWebSocketConnection(rec, req)

	assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
}

// ============================================================================
// handleCloseWebSocketConnection
// ============================================================================

func TestHandleCloseWebSocketConnection_MissingID_Returns400(t *testing.T) {
	api := NewAPI(0, WithDataDir(t.TempDir()))
	defer func() { _ = api.Stop() }()

	req := httptest.NewRequest(http.MethodDelete, "/websocket/connections/", nil)
	// PathValue "id" intentionally not set → empty string
	rec := httptest.NewRecorder()

	api.handleCloseWebSocketConnection(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestHandleCloseWebSocketConnection_NoEngine_Returns404(t *testing.T) {
	api := NewAPI(0, WithDataDir(t.TempDir()))
	defer func() { _ = api.Stop() }()

	req := httptest.NewRequest(http.MethodDelete, "/websocket/connections/conn-1", nil)
	req.SetPathValue("id", "conn-1")
	rec := httptest.NewRecorder()

	api.handleCloseWebSocketConnection(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestHandleCloseWebSocketConnection_EngineUnavailable_Returns503(t *testing.T) {
	api := NewAPI(0, WithDataDir(t.TempDir()), WithLocalEngineClient(engineclient.New("http://127.0.0.1:1")))
	defer func() { _ = api.Stop() }()

	req := httptest.NewRequest(http.MethodDelete, "/websocket/connections/conn-1", nil)
	req.SetPathValue("id", "conn-1")
	rec := httptest.NewRecorder()

	api.handleCloseWebSocketConnection(rec, req)

	assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
}

// ============================================================================
// handleGetWebSocketStats
// ============================================================================

func TestHandleGetWebSocketStats_NoEngine_ReturnsEmptyStats(t *testing.T) {
	api := NewAPI(0, WithDataDir(t.TempDir()))
	defer func() { _ = api.Stop() }()

	req := httptest.NewRequest(http.MethodGet, "/websocket/stats", nil)
	rec := httptest.NewRecorder()

	api.handleGetWebSocketStats(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var stats engineclient.WebSocketStats
	err := json.Unmarshal(rec.Body.Bytes(), &stats)
	require.NoError(t, err)
	assert.NotNil(t, stats.ConnectionsByMock)
}

func TestHandleGetWebSocketStats_EngineUnavailable_Returns503(t *testing.T) {
	api := NewAPI(0, WithDataDir(t.TempDir()), WithLocalEngineClient(engineclient.New("http://127.0.0.1:1")))
	defer func() { _ = api.Stop() }()

	req := httptest.NewRequest(http.MethodGet, "/websocket/stats", nil)
	rec := httptest.NewRecorder()

	api.handleGetWebSocketStats(rec, req)

	assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
}
