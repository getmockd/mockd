package admin

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/getmockd/mockd/pkg/admin/engineclient"
)

// GRPCStreamListResponse represents a list of gRPC streams with stats.
type GRPCStreamListResponse struct {
	Streams []*engineclient.GRPCStream `json:"streams"`
	Stats   engineclient.GRPCStats     `json:"stats"`
}

// handleListGRPCStreams handles GET /grpc/connections.
//
//nolint:dupl // intentionally parallel structure with other protocol list handlers
func (a *API) handleListGRPCStreams(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	engine := a.localEngine.Load()
	if engine == nil {
		writeJSON(w, http.StatusOK, GRPCStreamListResponse{
			Streams: []*engineclient.GRPCStream{},
			Stats:   engineclient.GRPCStats{StreamsByMethod: make(map[string]int)},
		})
		return
	}

	stats, err := engine.GetGRPCStats(ctx)
	if err != nil {
		a.logger().Error("failed to get gRPC stats", "error", err)
		status, code, msg := mapGRPCEngineError(err, a.logger(), "get gRPC stats")
		writeError(w, status, code, msg)
		return
	}

	streams, err := engine.ListGRPCStreams(ctx)
	if err != nil {
		a.logger().Error("failed to list gRPC streams", "error", err)
		status, code, msg := mapGRPCEngineError(err, a.logger(), "list gRPC streams")
		writeError(w, status, code, msg)
		return
	}

	if streams == nil {
		streams = []*engineclient.GRPCStream{}
	}

	byMethod := stats.StreamsByMethod
	if byMethod == nil {
		byMethod = make(map[string]int)
	}

	writeJSON(w, http.StatusOK, GRPCStreamListResponse{
		Streams: streams,
		Stats: engineclient.GRPCStats{
			ActiveStreams:      stats.ActiveStreams,
			TotalStreams:       stats.TotalStreams,
			TotalRPCs:          stats.TotalRPCs,
			TotalMessagesSent:  stats.TotalMessagesSent,
			TotalMessagesRecv:  stats.TotalMessagesRecv,
			StreamsByMethod:    byMethod,
		},
	})
}

// handleGetGRPCStream handles GET /grpc/connections/{id}.
func (a *API) handleGetGRPCStream(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "missing_id", "Stream ID is required")
		return
	}

	engine := a.localEngine.Load()
	if engine == nil {
		writeError(w, http.StatusNotFound, "not_found", "Stream not found")
		return
	}

	stream, err := engine.GetGRPCStream(ctx, id)
	if err != nil {
		if errors.Is(err, engineclient.ErrNotFound) {
			writeError(w, http.StatusNotFound, "not_found", "Stream not found")
			return
		}
		a.logger().Error("failed to get gRPC stream", "error", err, "streamID", id)
		status, code, msg := mapGRPCEngineError(err, a.logger(), "get gRPC stream")
		writeError(w, status, code, msg)
		return
	}

	writeJSON(w, http.StatusOK, stream)
}

// handleCancelGRPCStream handles DELETE /grpc/connections/{id}.
//
//nolint:dupl // intentionally parallel structure with other protocol close handlers
func (a *API) handleCancelGRPCStream(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "missing_id", "Stream ID is required")
		return
	}

	engine := a.localEngine.Load()
	if engine == nil {
		writeError(w, http.StatusNotFound, "not_found", "Stream not found")
		return
	}

	err := engine.CancelGRPCStream(ctx, id)
	if err != nil {
		if errors.Is(err, engineclient.ErrNotFound) {
			writeError(w, http.StatusNotFound, "not_found", "Stream not found")
			return
		}
		a.logger().Error("failed to cancel gRPC stream", "error", err, "streamID", id)
		status, code, msg := mapGRPCEngineError(err, a.logger(), "cancel gRPC stream")
		writeError(w, status, code, msg)
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"message": "Stream cancelled",
		"stream":  id,
	})
}

// handleGetGRPCStats handles GET /grpc/stats.
func (a *API) handleGetGRPCStats(w http.ResponseWriter, r *http.Request) {
	engine := a.localEngine.Load()
	if engine == nil {
		writeJSON(w, http.StatusOK, engineclient.GRPCStats{StreamsByMethod: make(map[string]int)})
		return
	}
	a.handleGetStats(w, r, newGRPCStatsProvider(engine))
}

func mapGRPCEngineError(err error, log *slog.Logger, operation string) (int, string, string) {
	return http.StatusServiceUnavailable, "engine_error", sanitizeEngineError(err, log, operation)
}

// grpcStatsProvider implements statsProvider for gRPC statistics.
type grpcStatsProvider struct {
	engine *engineclient.Client
}

func newGRPCStatsProvider(engine *engineclient.Client) *grpcStatsProvider {
	return &grpcStatsProvider{engine: engine}
}

func (p *grpcStatsProvider) GetStats(ctx context.Context) (interface{}, error) {
	stats, err := p.engine.GetGRPCStats(ctx)
	if err != nil {
		return nil, err
	}

	byMethod := stats.StreamsByMethod
	if byMethod == nil {
		byMethod = make(map[string]int)
	}

	return engineclient.GRPCStats{
		ActiveStreams:      stats.ActiveStreams,
		TotalStreams:       stats.TotalStreams,
		TotalRPCs:          stats.TotalRPCs,
		TotalMessagesSent:  stats.TotalMessagesSent,
		TotalMessagesRecv:  stats.TotalMessagesRecv,
		StreamsByMethod:    byMethod,
	}, nil
}

func (p *grpcStatsProvider) MapError(err error, log *slog.Logger, operation string) (int, string, string) {
	return mapGRPCEngineError(err, log, operation)
}

func (p *grpcStatsProvider) ProtocolName() string { return "gRPC" }
