package admin

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/getmockd/mockd/pkg/admin/engineclient"
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

// ============================================================================
// handleSendToWebSocketConnection
// ============================================================================

func TestHandleSendToWebSocketConnection_MissingID_Returns400(t *testing.T) {
	api := NewAPI(0, WithDataDir(t.TempDir()))
	defer func() { _ = api.Stop() }()

	req := httptest.NewRequest(http.MethodPost, "/websocket/connections//send", strings.NewReader(`{"type":"text","data":"hello"}`))
	// PathValue "id" intentionally not set → empty string
	rec := httptest.NewRecorder()

	api.handleSendToWebSocketConnection(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestHandleSendToWebSocketConnection_InvalidJSON_Returns400(t *testing.T) {
	api := NewAPI(0, WithDataDir(t.TempDir()), WithLocalEngineClient(engineclient.New("http://127.0.0.1:1")))
	defer func() { _ = api.Stop() }()

	req := httptest.NewRequest(http.MethodPost, "/websocket/connections/conn-1/send", strings.NewReader(`{invalid`))
	req.SetPathValue("id", "conn-1")
	rec := httptest.NewRecorder()

	api.handleSendToWebSocketConnection(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestHandleSendToWebSocketConnection_NoEngine_Returns404(t *testing.T) {
	api := NewAPI(0, WithDataDir(t.TempDir()))
	defer func() { _ = api.Stop() }()

	req := httptest.NewRequest(http.MethodPost, "/websocket/connections/conn-1/send",
		strings.NewReader(`{"type":"text","data":"hello"}`))
	req.SetPathValue("id", "conn-1")
	rec := httptest.NewRecorder()

	api.handleSendToWebSocketConnection(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestHandleSendToWebSocketConnection_EngineUnavailable_Returns503(t *testing.T) {
	api := NewAPI(0, WithDataDir(t.TempDir()), WithLocalEngineClient(engineclient.New("http://127.0.0.1:1")))
	defer func() { _ = api.Stop() }()

	req := httptest.NewRequest(http.MethodPost, "/websocket/connections/conn-1/send",
		strings.NewReader(`{"type":"text","data":"hello"}`))
	req.SetPathValue("id", "conn-1")
	rec := httptest.NewRecorder()

	api.handleSendToWebSocketConnection(rec, req)

	assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
}

func TestHandleSendToWebSocketConnection_EmptyBody_DefaultsToText(t *testing.T) {
	// No engine — expects 404, but verifies empty body doesn't fail decode
	api := NewAPI(0, WithDataDir(t.TempDir()))
	defer func() { _ = api.Stop() }()

	req := httptest.NewRequest(http.MethodPost, "/websocket/connections/conn-1/send", strings.NewReader(""))
	req.SetPathValue("id", "conn-1")
	rec := httptest.NewRecorder()

	api.handleSendToWebSocketConnection(rec, req)

	// No engine → 404, not a 400 parse error
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestHandleSendToWebSocketConnection_Success_Returns200(t *testing.T) {
	// Spin up a minimal mock engine that accepts the send call and returns 200.
	mockEngine := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/send") {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"message":"Message sent","connection":"conn-1","type":"text"}`))
			return
		}
		http.NotFound(w, r)
	}))
	defer mockEngine.Close()

	api := NewAPI(0, WithDataDir(t.TempDir()), WithLocalEngineClient(engineclient.New(mockEngine.URL)))
	defer func() { _ = api.Stop() }()

	body := strings.NewReader(`{"type":"text","data":"hello"}`)
	req := httptest.NewRequest(http.MethodPost, "/websocket/connections/conn-1/send", body)
	req.SetPathValue("id", "conn-1")
	rec := httptest.NewRecorder()

	api.handleSendToWebSocketConnection(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var resp map[string]interface{}
	err := json.Unmarshal(rec.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, "Message sent", resp["message"])
	assert.Equal(t, "conn-1", resp["connection"])
	assert.Equal(t, "text", resp["type"])
}
