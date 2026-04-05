package admin

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/getmockd/mockd/pkg/admin/engineclient"
)

// ============================================================================
// handleListMQTTConnections
// ============================================================================

func TestHandleListMQTTConnections_NoEngine_ReturnsEmptyList(t *testing.T) {
	api := NewAPI(0, WithDataDir(t.TempDir()))
	defer func() { _ = api.Stop() }()

	req := httptest.NewRequest(http.MethodGet, "/mqtt/connections", nil)
	rec := httptest.NewRecorder()

	api.handleListMQTTConnections(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var resp MQTTConnectionListResponse
	err := json.Unmarshal(rec.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Empty(t, resp.Connections)
	assert.NotNil(t, resp.Stats.SubscriptionsByClient)
}

func TestHandleListMQTTConnections_WithEngine_ReturnsConnectionsAndStats(t *testing.T) {
	mockEngine := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/mqtt/stats":
			_, _ = w.Write([]byte(`{"connectedClients":2,"totalSubscriptions":3,"topicCount":5,"port":1883,"tlsEnabled":false,"authEnabled":true,"subscriptionsByClient":{"client-1":2,"client-2":1}}`))
		case r.Method == http.MethodGet && r.URL.Path == "/mqtt/connections":
			_, _ = w.Write([]byte(`{"connections":[{"id":"client-1","brokerId":"broker-1","connectedAt":"2026-04-05T00:00:00Z","subscriptions":["sensors/#"],"protocolVersion":4,"status":"connected"}],"count":1}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer mockEngine.Close()

	api := NewAPI(0, WithDataDir(t.TempDir()), WithLocalEngineClient(engineclient.New(mockEngine.URL)))
	defer func() { _ = api.Stop() }()

	req := httptest.NewRequest(http.MethodGet, "/mqtt/connections", nil)
	rec := httptest.NewRecorder()

	api.handleListMQTTConnections(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var resp MQTTConnectionListResponse
	err := json.Unmarshal(rec.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Len(t, resp.Connections, 1)
	assert.Equal(t, "client-1", resp.Connections[0].ID)
	assert.Equal(t, "broker-1", resp.Connections[0].BrokerID)
	assert.Equal(t, []string{"sensors/#"}, resp.Connections[0].Subscriptions)
	assert.Equal(t, 2, resp.Stats.ConnectedClients)
	assert.Equal(t, 3, resp.Stats.TotalSubscriptions)
	assert.True(t, resp.Stats.AuthEnabled)
}

func TestHandleListMQTTConnections_EngineUnavailable_Returns503(t *testing.T) {
	api := NewAPI(0, WithDataDir(t.TempDir()), WithLocalEngineClient(engineclient.New("http://127.0.0.1:1")))
	defer func() { _ = api.Stop() }()

	req := httptest.NewRequest(http.MethodGet, "/mqtt/connections", nil)
	rec := httptest.NewRecorder()

	api.handleListMQTTConnections(rec, req)

	assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
}

// ============================================================================
// handleGetMQTTConnection
// ============================================================================

func TestHandleGetMQTTConnection_MissingID_Returns400(t *testing.T) {
	api := NewAPI(0, WithDataDir(t.TempDir()))
	defer func() { _ = api.Stop() }()

	req := httptest.NewRequest(http.MethodGet, "/mqtt/connections/", nil)
	rec := httptest.NewRecorder()

	api.handleGetMQTTConnection(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestHandleGetMQTTConnection_NoEngine_Returns404(t *testing.T) {
	api := NewAPI(0, WithDataDir(t.TempDir()))
	defer func() { _ = api.Stop() }()

	req := httptest.NewRequest(http.MethodGet, "/mqtt/connections/client-1", nil)
	req.SetPathValue("id", "client-1")
	rec := httptest.NewRecorder()

	api.handleGetMQTTConnection(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestHandleGetMQTTConnection_Found_ReturnsConnection(t *testing.T) {
	mockEngine := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"client-1","brokerId":"broker-1","connectedAt":"2026-04-05T00:00:00Z","subscriptions":["topic/a"],"protocolVersion":5,"status":"connected"}`))
	}))
	defer mockEngine.Close()

	api := NewAPI(0, WithDataDir(t.TempDir()), WithLocalEngineClient(engineclient.New(mockEngine.URL)))
	defer func() { _ = api.Stop() }()

	req := httptest.NewRequest(http.MethodGet, "/mqtt/connections/client-1", nil)
	req.SetPathValue("id", "client-1")
	rec := httptest.NewRecorder()

	api.handleGetMQTTConnection(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var conn engineclient.MQTTConnection
	err := json.Unmarshal(rec.Body.Bytes(), &conn)
	require.NoError(t, err)
	assert.Equal(t, "client-1", conn.ID)
	assert.Equal(t, byte(5), conn.ProtocolVersion)
}

func TestHandleGetMQTTConnection_NotFound_Returns404(t *testing.T) {
	mockEngine := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":"not_found","message":"MQTT connection not found"}`))
	}))
	defer mockEngine.Close()

	api := NewAPI(0, WithDataDir(t.TempDir()), WithLocalEngineClient(engineclient.New(mockEngine.URL)))
	defer func() { _ = api.Stop() }()

	req := httptest.NewRequest(http.MethodGet, "/mqtt/connections/nonexistent", nil)
	req.SetPathValue("id", "nonexistent")
	rec := httptest.NewRecorder()

	api.handleGetMQTTConnection(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)
}

// ============================================================================
// handleCloseMQTTConnection
// ============================================================================

func TestHandleCloseMQTTConnection_MissingID_Returns400(t *testing.T) {
	api := NewAPI(0, WithDataDir(t.TempDir()))
	defer func() { _ = api.Stop() }()

	req := httptest.NewRequest(http.MethodDelete, "/mqtt/connections/", nil)
	rec := httptest.NewRecorder()

	api.handleCloseMQTTConnection(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestHandleCloseMQTTConnection_NoEngine_Returns404(t *testing.T) {
	api := NewAPI(0, WithDataDir(t.TempDir()))
	defer func() { _ = api.Stop() }()

	req := httptest.NewRequest(http.MethodDelete, "/mqtt/connections/client-1", nil)
	req.SetPathValue("id", "client-1")
	rec := httptest.NewRecorder()

	api.handleCloseMQTTConnection(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestHandleCloseMQTTConnection_Success(t *testing.T) {
	mockEngine := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"message":"MQTT connection closed","id":"client-1"}`))
	}))
	defer mockEngine.Close()

	api := NewAPI(0, WithDataDir(t.TempDir()), WithLocalEngineClient(engineclient.New(mockEngine.URL)))
	defer func() { _ = api.Stop() }()

	req := httptest.NewRequest(http.MethodDelete, "/mqtt/connections/client-1", nil)
	req.SetPathValue("id", "client-1")
	rec := httptest.NewRecorder()

	api.handleCloseMQTTConnection(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}

// ============================================================================
// handleGetMQTTStats
// ============================================================================

func TestHandleGetMQTTStats_NoEngine_ReturnsEmptyStats(t *testing.T) {
	api := NewAPI(0, WithDataDir(t.TempDir()))
	defer func() { _ = api.Stop() }()

	req := httptest.NewRequest(http.MethodGet, "/mqtt/stats", nil)
	rec := httptest.NewRecorder()

	api.handleGetMQTTStats(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var stats engineclient.MQTTStats
	err := json.Unmarshal(rec.Body.Bytes(), &stats)
	require.NoError(t, err)
	assert.NotNil(t, stats.SubscriptionsByClient)
}

func TestHandleGetMQTTStats_WithEngine_ReturnsStats(t *testing.T) {
	mockEngine := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"connectedClients":3,"totalSubscriptions":5,"topicCount":2,"port":1883,"subscriptionsByClient":{"c1":2,"c2":3}}`))
	}))
	defer mockEngine.Close()

	api := NewAPI(0, WithDataDir(t.TempDir()), WithLocalEngineClient(engineclient.New(mockEngine.URL)))
	defer func() { _ = api.Stop() }()

	req := httptest.NewRequest(http.MethodGet, "/mqtt/stats", nil)
	rec := httptest.NewRecorder()

	api.handleGetMQTTStats(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var stats engineclient.MQTTStats
	err := json.Unmarshal(rec.Body.Bytes(), &stats)
	require.NoError(t, err)
	assert.Equal(t, 3, stats.ConnectedClients)
	assert.Equal(t, 5, stats.TotalSubscriptions)
	assert.Equal(t, 2, stats.SubscriptionsByClient["c1"])
}
