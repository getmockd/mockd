package admin

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/getmockd/mockd/pkg/admin/engineclient"
	"github.com/getmockd/mockd/pkg/sse"
)

// statsProvider defines the interface for retrieving and formatting statistics.
type statsProvider interface {
	// GetStats retrieves statistics from the engine.
	GetStats(ctx context.Context) (interface{}, error)
	// MapError converts an error to HTTP status, code, and message.
	MapError(err error, log *slog.Logger, operation string) (int, string, string)
}

// handleGetStats is a generic handler for retrieving statistics.
// The caller must guard against a nil engine and handle the empty-stats
// response before constructing the provider and invoking this function.
func (a *API) handleGetStats(w http.ResponseWriter, r *http.Request, provider statsProvider) {
	ctx := r.Context()

	stats, err := provider.GetStats(ctx)
	if err != nil {
		a.logger().Error("failed to get stats", "error", err)
		status, code, msg := provider.MapError(err, a.logger(), "get stats")
		writeError(w, status, code, msg)
		return
	}

	writeJSON(w, http.StatusOK, stats)
}

// sseStatsProvider implements statsProvider for SSE statistics.
type sseStatsProvider struct {
	engine *engineclient.Client
}

// newSSEStatsProvider creates a new SSE statistics provider.
func newSSEStatsProvider(engine *engineclient.Client) *sseStatsProvider {
	return &sseStatsProvider{engine: engine}
}

// GetStats retrieves SSE statistics from the engine.
func (p *sseStatsProvider) GetStats(ctx context.Context) (interface{}, error) {
	stats, err := p.engine.GetSSEStats(ctx)
	if err != nil {
		return nil, err
	}

	connsByMock := stats.ConnectionsByMock
	if connsByMock == nil {
		connsByMock = make(map[string]int)
	}

	return sse.ConnectionStats{
		ActiveConnections: stats.ActiveConnections,
		TotalConnections:  stats.TotalConnections,
		TotalEventsSent:   stats.TotalEventsSent,
		TotalBytesSent:    stats.TotalBytesSent,
		ConnectionsByMock: connsByMock,
	}, nil
}

// MapError converts an SSE engine error to HTTP status, code, and message.
func (p *sseStatsProvider) MapError(err error, log *slog.Logger, operation string) (int, string, string) {
	return mapSSEEngineError(err, log, operation)
}

// wsStatsProvider implements statsProvider for WebSocket statistics.
type wsStatsProvider struct {
	engine *engineclient.Client
}

// newWSStatsProvider creates a new WebSocket statistics provider.
func newWSStatsProvider(engine *engineclient.Client) *wsStatsProvider {
	return &wsStatsProvider{engine: engine}
}

// GetStats retrieves WebSocket statistics from the engine.
func (p *wsStatsProvider) GetStats(ctx context.Context) (interface{}, error) {
	stats, err := p.engine.GetWebSocketStats(ctx)
	if err != nil {
		return nil, err
	}

	connsByMock := stats.ConnectionsByMock
	if connsByMock == nil {
		connsByMock = make(map[string]int)
	}

	return engineclient.WebSocketStats{
		TotalConnections:  stats.TotalConnections,
		ActiveConnections: stats.ActiveConnections,
		TotalMessagesSent: stats.TotalMessagesSent,
		TotalMessagesRecv: stats.TotalMessagesRecv,
		ConnectionsByMock: connsByMock,
	}, nil
}

// MapError converts a WebSocket engine error to HTTP status, code, and message.
func (p *wsStatsProvider) MapError(err error, log *slog.Logger, operation string) (int, string, string) {
	return mapWebSocketEngineError(err, log, operation)
}
