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
// handleListGRPCStreams
// ============================================================================

func TestHandleListGRPCStreams_NoEngine_ReturnsEmptyList(t *testing.T) {
	api := NewAPI(0, WithDataDir(t.TempDir()))
	defer func() { _ = api.Stop() }()

	req := httptest.NewRequest(http.MethodGet, "/grpc/connections", nil)
	rec := httptest.NewRecorder()

	api.handleListGRPCStreams(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var resp GRPCStreamListResponse
	err := json.Unmarshal(rec.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Empty(t, resp.Streams)
	assert.NotNil(t, resp.Stats.StreamsByMethod)
}

func TestHandleListGRPCStreams_WithEngine_ReturnsStreamsAndStats(t *testing.T) {
	mockEngine := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/grpc/stats":
			_, _ = w.Write([]byte(`{"activeStreams":1,"totalStreams":5,"totalRPCs":100,"totalMessagesSent":50,"totalMessagesRecv":30,"streamsByMethod":{"/pkg.Svc/Stream":1}}`))
		case r.Method == http.MethodGet && r.URL.Path == "/grpc/connections":
			_, _ = w.Write([]byte(`{"streams":[{"id":"grpc-stream-1","method":"/pkg.Svc/Stream","streamType":"server_stream","clientAddr":"127.0.0.1:5000","connectedAt":"2026-04-05T00:00:00Z","messagesSent":10,"messagesRecv":1}],"count":1}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer mockEngine.Close()

	api := NewAPI(0, WithDataDir(t.TempDir()), WithLocalEngineClient(engineclient.New(mockEngine.URL)))
	defer func() { _ = api.Stop() }()

	req := httptest.NewRequest(http.MethodGet, "/grpc/connections", nil)
	rec := httptest.NewRecorder()

	api.handleListGRPCStreams(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var resp GRPCStreamListResponse
	err := json.Unmarshal(rec.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Len(t, resp.Streams, 1)
	assert.Equal(t, "grpc-stream-1", resp.Streams[0].ID)
	assert.Equal(t, "/pkg.Svc/Stream", resp.Streams[0].Method)
	assert.Equal(t, 1, resp.Stats.ActiveStreams)
	assert.Equal(t, int64(100), resp.Stats.TotalRPCs)
}

// ============================================================================
// handleGetGRPCStream
// ============================================================================

func TestHandleGetGRPCStream_MissingID_ReturnsBadRequest(t *testing.T) {
	api := NewAPI(0, WithDataDir(t.TempDir()))
	defer func() { _ = api.Stop() }()

	req := httptest.NewRequest(http.MethodGet, "/grpc/connections/", nil)
	rec := httptest.NewRecorder()

	api.handleGetGRPCStream(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestHandleGetGRPCStream_NoEngine_ReturnsNotFound(t *testing.T) {
	api := NewAPI(0, WithDataDir(t.TempDir()))
	defer func() { _ = api.Stop() }()

	req := httptest.NewRequest(http.MethodGet, "/grpc/connections/stream-1", nil)
	req.SetPathValue("id", "stream-1")
	rec := httptest.NewRecorder()

	api.handleGetGRPCStream(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestHandleGetGRPCStream_Found_ReturnsStream(t *testing.T) {
	mockEngine := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"stream-1","method":"/pkg.Svc/Chat","streamType":"bidi","connectedAt":"2026-04-05T00:00:00Z","messagesSent":5,"messagesRecv":3}`))
	}))
	defer mockEngine.Close()

	api := NewAPI(0, WithDataDir(t.TempDir()), WithLocalEngineClient(engineclient.New(mockEngine.URL)))
	defer func() { _ = api.Stop() }()

	req := httptest.NewRequest(http.MethodGet, "/grpc/connections/stream-1", nil)
	req.SetPathValue("id", "stream-1")
	rec := httptest.NewRecorder()

	api.handleGetGRPCStream(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var stream engineclient.GRPCStream
	err := json.Unmarshal(rec.Body.Bytes(), &stream)
	require.NoError(t, err)
	assert.Equal(t, "stream-1", stream.ID)
	assert.Equal(t, "bidi", stream.StreamType)
}

// ============================================================================
// handleCancelGRPCStream
// ============================================================================

func TestHandleCancelGRPCStream_MissingID_ReturnsBadRequest(t *testing.T) {
	api := NewAPI(0, WithDataDir(t.TempDir()))
	defer func() { _ = api.Stop() }()

	req := httptest.NewRequest(http.MethodDelete, "/grpc/connections/", nil)
	rec := httptest.NewRecorder()

	api.handleCancelGRPCStream(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestHandleCancelGRPCStream_NoEngine_ReturnsNotFound(t *testing.T) {
	api := NewAPI(0, WithDataDir(t.TempDir()))
	defer func() { _ = api.Stop() }()

	req := httptest.NewRequest(http.MethodDelete, "/grpc/connections/stream-1", nil)
	req.SetPathValue("id", "stream-1")
	rec := httptest.NewRecorder()

	api.handleCancelGRPCStream(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestHandleCancelGRPCStream_Success(t *testing.T) {
	mockEngine := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"message":"gRPC stream cancelled","id":"stream-1"}`))
	}))
	defer mockEngine.Close()

	api := NewAPI(0, WithDataDir(t.TempDir()), WithLocalEngineClient(engineclient.New(mockEngine.URL)))
	defer func() { _ = api.Stop() }()

	req := httptest.NewRequest(http.MethodDelete, "/grpc/connections/stream-1", nil)
	req.SetPathValue("id", "stream-1")
	rec := httptest.NewRecorder()

	api.handleCancelGRPCStream(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}

// ============================================================================
// handleGetGRPCStats
// ============================================================================

func TestHandleGetGRPCStats_NoEngine_ReturnsEmptyStats(t *testing.T) {
	api := NewAPI(0, WithDataDir(t.TempDir()))
	defer func() { _ = api.Stop() }()

	req := httptest.NewRequest(http.MethodGet, "/grpc/stats", nil)
	rec := httptest.NewRecorder()

	api.handleGetGRPCStats(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var stats engineclient.GRPCStats
	err := json.Unmarshal(rec.Body.Bytes(), &stats)
	require.NoError(t, err)
	assert.NotNil(t, stats.StreamsByMethod)
}

func TestHandleGetGRPCStats_WithEngine_ReturnsStats(t *testing.T) {
	mockEngine := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"activeStreams":3,"totalStreams":10,"totalRPCs":500,"totalMessagesSent":200,"totalMessagesRecv":150,"streamsByMethod":{"/pkg.Svc/A":2,"/pkg.Svc/B":1}}`))
	}))
	defer mockEngine.Close()

	api := NewAPI(0, WithDataDir(t.TempDir()), WithLocalEngineClient(engineclient.New(mockEngine.URL)))
	defer func() { _ = api.Stop() }()

	req := httptest.NewRequest(http.MethodGet, "/grpc/stats", nil)
	rec := httptest.NewRecorder()

	api.handleGetGRPCStats(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var stats engineclient.GRPCStats
	err := json.Unmarshal(rec.Body.Bytes(), &stats)
	require.NoError(t, err)
	assert.Equal(t, 3, stats.ActiveStreams)
	assert.Equal(t, int64(500), stats.TotalRPCs)
	assert.Len(t, stats.StreamsByMethod, 2)
}
