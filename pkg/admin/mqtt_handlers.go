package admin

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/getmockd/mockd/pkg/admin/engineclient"
)

// MQTTConnectionListResponse represents a list of MQTT connections with stats.
type MQTTConnectionListResponse struct {
	Connections []*engineclient.MQTTConnection `json:"connections"`
	Stats       engineclient.MQTTStats         `json:"stats"`
}

// handleListMQTTConnections handles GET /mqtt/connections.
func (a *API) handleListMQTTConnections(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	engine := a.localEngine.Load()
	if engine == nil {
		writeJSON(w, http.StatusOK, MQTTConnectionListResponse{
			Connections: []*engineclient.MQTTConnection{},
			Stats:       engineclient.MQTTStats{SubscriptionsByClient: make(map[string]int)},
		})
		return
	}

	stats, err := engine.GetMQTTStats(ctx)
	if err != nil {
		a.logger().Error("failed to get MQTT stats", "error", err)
		status, code, msg := mapMQTTEngineError(err, a.logger(), "get MQTT stats")
		writeError(w, status, code, msg)
		return
	}

	connections, err := engine.ListMQTTConnections(ctx)
	if err != nil {
		a.logger().Error("failed to list MQTT connections", "error", err)
		status, code, msg := mapMQTTEngineError(err, a.logger(), "list MQTT connections")
		writeError(w, status, code, msg)
		return
	}

	if connections == nil {
		connections = []*engineclient.MQTTConnection{}
	}

	subsByClient := stats.SubscriptionsByClient
	if subsByClient == nil {
		subsByClient = make(map[string]int)
	}

	writeJSON(w, http.StatusOK, MQTTConnectionListResponse{
		Connections: connections,
		Stats: engineclient.MQTTStats{
			ConnectedClients:      stats.ConnectedClients,
			TotalSubscriptions:    stats.TotalSubscriptions,
			TopicCount:            stats.TopicCount,
			Port:                  stats.Port,
			TLSEnabled:            stats.TLSEnabled,
			AuthEnabled:           stats.AuthEnabled,
			SubscriptionsByClient: subsByClient,
		},
	})
}

// handleGetMQTTConnection handles GET /mqtt/connections/{id}.
func (a *API) handleGetMQTTConnection(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "missing_id", "Client ID is required")
		return
	}

	engine := a.localEngine.Load()
	if engine == nil {
		writeError(w, http.StatusNotFound, "not_found", "Connection not found")
		return
	}

	conn, err := engine.GetMQTTConnection(ctx, id)
	if err != nil {
		if errors.Is(err, engineclient.ErrNotFound) {
			writeError(w, http.StatusNotFound, "not_found", "Connection not found")
			return
		}
		a.logger().Error("failed to get MQTT connection", "error", err, "clientID", id)
		status, code, msg := mapMQTTEngineError(err, a.logger(), "get MQTT connection")
		writeError(w, status, code, msg)
		return
	}

	writeJSON(w, http.StatusOK, conn)
}

// handleCloseMQTTConnection handles DELETE /mqtt/connections/{id}.
//
//nolint:dupl // intentionally parallel structure with other protocol close handlers
func (a *API) handleCloseMQTTConnection(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "missing_id", "Client ID is required")
		return
	}

	engine := a.localEngine.Load()
	if engine == nil {
		writeError(w, http.StatusNotFound, "not_found", "Connection not found")
		return
	}

	err := engine.CloseMQTTConnection(ctx, id)
	if err != nil {
		if errors.Is(err, engineclient.ErrNotFound) {
			writeError(w, http.StatusNotFound, "not_found", "Connection not found")
			return
		}
		a.logger().Error("failed to close MQTT connection", "error", err, "clientID", id)
		status, code, msg := mapMQTTEngineError(err, a.logger(), "close MQTT connection")
		writeError(w, status, code, msg)
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"message":    "Connection closed",
		"connection": id,
	})
}

// handleGetMQTTStats handles GET /mqtt/stats.
func (a *API) handleGetMQTTStats(w http.ResponseWriter, r *http.Request) {
	engine := a.localEngine.Load()
	if engine == nil {
		writeJSON(w, http.StatusOK, engineclient.MQTTStats{SubscriptionsByClient: make(map[string]int)})
		return
	}
	a.handleGetStats(w, r, newMQTTStatsProvider(engine))
}

func mapMQTTEngineError(err error, log *slog.Logger, operation string) (int, string, string) {
	return http.StatusServiceUnavailable, "engine_error", sanitizeEngineError(err, log, operation)
}
