package engine

import (
	"context"
	"errors"
	"fmt"

	"github.com/getmockd/mockd/pkg/api/types"
	"github.com/getmockd/mockd/pkg/chaos"
	"github.com/getmockd/mockd/pkg/config"
	"github.com/getmockd/mockd/pkg/engine/api"
	"github.com/getmockd/mockd/pkg/protocol"
	"github.com/getmockd/mockd/pkg/requestlog"
	"github.com/getmockd/mockd/pkg/stateful"
	"github.com/getmockd/mockd/pkg/websocket"
)

// Errors returned by the control API adapter.
var (
	ErrStatefulStoreNotInitialized    = errors.New("stateful store not initialized")
	ErrSSEHandlerNotInitialized       = errors.New("SSE handler not initialized")
	ErrWebSocketHandlerNotInitialized = errors.New("WebSocket handler not initialized")
)

// ControlAPIAdapter adapts engine.Server to the api.EngineController interface.
// This breaks the import cycle by providing an adapter that the api package can use.
type ControlAPIAdapter struct {
	server *Server
}

// NewControlAPIAdapter creates a new adapter for the given server.
func NewControlAPIAdapter(s *Server) *ControlAPIAdapter {
	return &ControlAPIAdapter{server: s}
}

// IsRunning implements api.EngineController.
func (a *ControlAPIAdapter) IsRunning() bool {
	return a.server.IsRunning()
}

// Uptime implements api.EngineController.
func (a *ControlAPIAdapter) Uptime() int {
	return a.server.Uptime()
}

// AddMock implements api.EngineController.
func (a *ControlAPIAdapter) AddMock(cfg *config.MockConfiguration) error {
	return a.server.addMock(cfg)
}

// UpdateMock implements api.EngineController.
func (a *ControlAPIAdapter) UpdateMock(id string, cfg *config.MockConfiguration) error {
	return a.server.updateMock(id, cfg)
}

// DeleteMock implements api.EngineController.
func (a *ControlAPIAdapter) DeleteMock(id string) error {
	return a.server.deleteMock(id)
}

// GetMock implements api.EngineController.
func (a *ControlAPIAdapter) GetMock(id string) *config.MockConfiguration {
	return a.server.getMock(id)
}

// ListMocks implements api.EngineController.
func (a *ControlAPIAdapter) ListMocks() []*config.MockConfiguration {
	return a.server.listMocks()
}

// ClearMocks implements api.EngineController.
func (a *ControlAPIAdapter) ClearMocks() {
	a.server.clearMocks()
}

// GetRequestLogs implements api.EngineController.
func (a *ControlAPIAdapter) GetRequestLogs(filter *requestlog.Filter) []*requestlog.Entry {
	return a.server.GetRequestLogs(filter)
}

// GetRequestLog implements api.EngineController.
func (a *ControlAPIAdapter) GetRequestLog(id string) *requestlog.Entry {
	return a.server.GetRequestLog(id)
}

// RequestLogCount implements api.EngineController.
func (a *ControlAPIAdapter) RequestLogCount() int {
	return a.server.RequestLogCount()
}

// RequestLogCountFiltered implements api.EngineController.
func (a *ControlAPIAdapter) RequestLogCountFiltered(filter *requestlog.Filter) int {
	return a.server.RequestLogCountFiltered(filter)
}

// ClearRequestLogs implements api.EngineController.
func (a *ControlAPIAdapter) ClearRequestLogs() {
	a.server.ClearRequestLogs()
}

// ClearRequestLogsByMockID implements api.EngineController.
func (a *ControlAPIAdapter) ClearRequestLogsByMockID(mockID string) int {
	logger := a.server.Logger()
	if logger == nil {
		return 0
	}
	// Check if logger supports ClearByMockID
	if clearer, ok := logger.(interface{ ClearByMockID(string) }); ok {
		// Get count before clearing
		if counter, ok2 := logger.(interface{ CountByMockID(string) int }); ok2 {
			count := counter.CountByMockID(mockID)
			clearer.ClearByMockID(mockID)
			return count
		}
		clearer.ClearByMockID(mockID)
		return -1 // Unknown count
	}
	return 0
}

// ProtocolStatus implements api.EngineController.
// Converts the engine status type to the api status type.
func (a *ControlAPIAdapter) ProtocolStatus() map[string]api.ProtocolStatusInfo {
	engineStatus := a.server.ProtocolStatus()
	result := make(map[string]api.ProtocolStatusInfo, len(engineStatus))
	for k, v := range engineStatus {
		result[k] = api.ProtocolStatusInfo{
			Enabled:     v.Enabled,
			Port:        v.Port,
			Connections: v.Connections,
			Status:      v.Status,
		}
	}
	return result
}

// GetChaosConfig implements api.EngineController.
func (a *ControlAPIAdapter) GetChaosConfig() *api.ChaosConfig {
	injector := a.server.ChaosInjector()
	if injector == nil {
		return nil
	}

	cfg := injector.GetConfig()
	if cfg == nil {
		return nil
	}

	apiCfg := types.ChaosConfigFromInternal(cfg)
	return &apiCfg
}

// SetChaosConfig implements api.EngineController.
func (a *ControlAPIAdapter) SetChaosConfig(cfg *api.ChaosConfig) error {
	if cfg == nil || !cfg.Enabled {
		// Disable chaos by setting nil injector
		return a.server.SetChaosInjector(nil)
	}

	// Convert API config to internal chaos config using canonical converter
	chaosCfg := types.ChaosConfigToInternal(cfg)

	// Clamp probability/rate values to [0.0, 1.0] before validating
	chaosCfg.Clamp()

	// Validate configuration before creating injector
	if err := chaosCfg.Validate(); err != nil {
		return err
	}

	// Create injector
	injector, err := chaos.NewInjector(chaosCfg)
	if err != nil {
		return err
	}

	return a.server.SetChaosInjector(injector)
}

// GetChaosStats implements api.EngineController.
func (a *ControlAPIAdapter) GetChaosStats() *api.ChaosStats {
	injector := a.server.ChaosInjector()
	if injector == nil {
		return nil
	}

	stats := injector.GetStats()
	faultsByType := make(map[string]int64)
	for k, v := range stats.FaultsByType {
		faultsByType[string(k)] = v
	}

	return &api.ChaosStats{
		TotalRequests:    stats.TotalRequests,
		InjectedFaults:   stats.InjectedFaults,
		LatencyInjected:  stats.LatencyInjected,
		ErrorsInjected:   stats.ErrorsInjected,
		TimeoutsInjected: stats.TimeoutsInjected,
		FaultsByType:     faultsByType,
	}
}

// ResetChaosStats implements api.EngineController.
func (a *ControlAPIAdapter) ResetChaosStats() {
	injector := a.server.ChaosInjector()
	if injector != nil {
		injector.ResetStats()
	}
}

// GetStatefulFaultStats implements api.EngineController.
func (a *ControlAPIAdapter) GetStatefulFaultStats() *api.StatefulFaultStats {
	injector := a.server.ChaosInjector()
	if injector == nil {
		return nil
	}

	result := &api.StatefulFaultStats{}

	// Collect circuit breaker stats
	cbs := injector.GetCircuitBreakers()
	if len(cbs) > 0 {
		result.CircuitBreakers = make(map[string]api.CircuitBreakerStatus, len(cbs))
		for key, cb := range cbs {
			s := cb.Stats()
			result.CircuitBreakers[key] = api.CircuitBreakerStatus{
				State:                s.State,
				ConsecutiveFailures:  s.ConsecutiveFailures,
				ConsecutiveSuccesses: s.ConsecutiveSuccesses,
				TotalRequests:        s.TotalRequests,
				TotalTrips:           s.TotalTrips,
				TotalRejected:        s.TotalRejected,
				TotalPassed:          s.TotalPassed,
				TotalHalfOpen:        s.TotalHalfOpen,
				StateChanges:         s.StateChanges,
				OpenedAt:             s.OpenedAt,
			}
		}
	}

	// Collect retry-after tracker stats
	rts := injector.GetRetryTrackers()
	if len(rts) > 0 {
		result.RetryAfterTrackers = make(map[string]api.RetryAfterStatus, len(rts))
		for key, rt := range rts {
			s := rt.Stats()
			result.RetryAfterTrackers[key] = api.RetryAfterStatus{
				IsLimited:    s.IsLimited,
				StatusCode:   s.StatusCode,
				RetryAfterMs: s.RetryAfterMs,
				TotalLimited: s.TotalLimited,
				TotalPassed:  s.TotalPassed,
				LimitedAt:    s.LimitedAt,
			}
		}
	}

	// Collect progressive degradation stats
	pds := injector.GetProgressives()
	if len(pds) > 0 {
		result.ProgressiveDegradations = make(map[string]api.ProgressiveDegradationStatus, len(pds))
		for key, pd := range pds {
			s := pd.Stats()
			result.ProgressiveDegradations[key] = api.ProgressiveDegradationStatus{
				RequestCount:   s.RequestCount,
				CurrentDelayMs: s.CurrentDelayMs,
				MaxDelayMs:     s.MaxDelayMs,
				ErrorAfter:     s.ErrorAfter,
				ResetAfter:     s.ResetAfter,
				TotalErrors:    s.TotalErrors,
				TotalResets:    s.TotalResets,
				IsErroring:     s.IsErroring,
			}
		}
	}

	return result
}

// TripCircuitBreaker implements api.EngineController.
func (a *ControlAPIAdapter) TripCircuitBreaker(key string) error {
	injector := a.server.ChaosInjector()
	if injector == nil {
		return errors.New("chaos is not enabled")
	}
	cbs := injector.GetCircuitBreakers()
	cb, ok := cbs[key]
	if !ok {
		return fmt.Errorf("circuit breaker %q not found", key)
	}
	cb.Trip()
	return nil
}

// ResetCircuitBreaker implements api.EngineController.
func (a *ControlAPIAdapter) ResetCircuitBreaker(key string) error {
	injector := a.server.ChaosInjector()
	if injector == nil {
		return errors.New("chaos is not enabled")
	}
	cbs := injector.GetCircuitBreakers()
	cb, ok := cbs[key]
	if !ok {
		return fmt.Errorf("circuit breaker %q not found", key)
	}
	cb.Reset()
	return nil
}

// GetStateOverview implements api.EngineController.
func (a *ControlAPIAdapter) GetStateOverview(workspaceID string) *api.StateOverview {
	store := a.server.StatefulStore()
	if store == nil {
		return nil
	}

	overview := store.Overview(workspaceID)
	if overview == nil {
		return nil
	}

	// Get detailed resource info
	var resources []api.StatefulResource
	for _, name := range overview.ResourceList {
		info, err := store.ResourceInfo(workspaceID, name)
		if err == nil && info != nil {
			resources = append(resources, api.StatefulResource{
				Name:        info.Name,
				ItemCount:   info.ItemCount,
				SeedCount:   info.SeedCount,
				IDField:     info.IDField,
				ParentField: info.ParentField,
			})
		}
	}

	return &api.StateOverview{
		Resources:    resources,
		Total:        overview.Resources,
		TotalItems:   overview.TotalItems,
		ResourceList: overview.ResourceList,
	}
}

// GetStateResource implements api.EngineController.
func (a *ControlAPIAdapter) GetStateResource(workspaceID string, name string) (*api.StatefulResource, error) {
	store := a.server.StatefulStore()
	if store == nil {
		return nil, ErrStatefulStoreNotInitialized
	}

	info, err := store.ResourceInfo(workspaceID, name)
	if err != nil {
		return nil, err
	}

	return &api.StatefulResource{
		Name:        info.Name,
		ItemCount:   info.ItemCount,
		SeedCount:   info.SeedCount,
		IDField:     info.IDField,
		ParentField: info.ParentField,
	}, nil
}

// ClearStateResource implements api.EngineController.
func (a *ControlAPIAdapter) ClearStateResource(workspaceID string, name string) (int, error) {
	store := a.server.StatefulStore()
	if store == nil {
		return 0, ErrStatefulStoreNotInitialized
	}

	return store.ClearResource(workspaceID, name)
}

// ResetState implements api.EngineController.
func (a *ControlAPIAdapter) ResetState(workspaceID string, resourceName string) (*api.ResetStateResponse, error) {
	store := a.server.StatefulStore()
	if store == nil {
		return nil, ErrStatefulStoreNotInitialized
	}

	resp, err := store.Reset(workspaceID, resourceName)
	if err != nil {
		return nil, err
	}

	return &api.ResetStateResponse{
		Reset:     resp.Reset,
		Resources: resp.Resources,
		Message:   resp.Message,
	}, nil
}

// RegisterStatefulResource implements api.EngineController.
func (a *ControlAPIAdapter) RegisterStatefulResource(workspaceID string, cfg *config.StatefulResourceConfig) error {
	if cfg == nil {
		return errors.New("config cannot be nil")
	}
	return a.server.registerStatefulResource(workspaceID, cfg)
}

// DeleteStatefulResource implements api.EngineController.
func (a *ControlAPIAdapter) DeleteStatefulResource(workspaceID string, name string) error {
	store := a.server.StatefulStore()
	if store == nil {
		return ErrStatefulStoreNotInitialized
	}
	return store.Unregister(workspaceID, name)
}

// ListStatefulItems implements api.EngineController.
func (a *ControlAPIAdapter) ListStatefulItems(workspaceID string, name string, limit, offset int, sort, order string) (*api.StatefulItemsResponse, error) {
	store := a.server.StatefulStore()
	if store == nil {
		return nil, ErrStatefulStoreNotInitialized
	}

	resource := store.Get(workspaceID, name)
	if resource == nil {
		return nil, errors.New("stateful resource not found: " + name)
	}

	filter := &stateful.QueryFilter{
		Limit:   limit,
		Offset:  offset,
		Sort:    sort,
		Order:   order,
		Filters: make(map[string]string),
	}

	result := resource.List(filter)

	return &api.StatefulItemsResponse{
		Data: result.Data,
		Meta: types.PaginationMeta{
			Total:  result.Meta.Total,
			Limit:  result.Meta.Limit,
			Offset: result.Meta.Offset,
			Count:  result.Meta.Count,
		},
	}, nil
}

// GetStatefulItem implements api.EngineController.
func (a *ControlAPIAdapter) GetStatefulItem(workspaceID string, resourceName, itemID string) (map[string]interface{}, error) {
	store := a.server.StatefulStore()
	if store == nil {
		return nil, ErrStatefulStoreNotInitialized
	}

	resource := store.Get(workspaceID, resourceName)
	if resource == nil {
		return nil, errors.New("stateful resource not found: " + resourceName)
	}

	item := resource.Get(itemID)
	if item == nil {
		return nil, errors.New("item not found: " + itemID + " in resource " + resourceName)
	}

	return item.ToJSON(), nil
}

// CreateStatefulItem implements api.EngineController.
func (a *ControlAPIAdapter) CreateStatefulItem(workspaceID string, resourceName string, data map[string]interface{}) (map[string]interface{}, error) {
	store := a.server.StatefulStore()
	if store == nil {
		return nil, ErrStatefulStoreNotInitialized
	}

	resource := store.Get(workspaceID, resourceName)
	if resource == nil {
		return nil, errors.New("stateful resource not found: " + resourceName)
	}

	item, err := resource.Create(data, nil)
	if err != nil {
		return nil, err
	}

	return item.ToJSON(), nil
}

// ListProtocolHandlers implements api.EngineController.
func (a *ControlAPIAdapter) ListProtocolHandlers() []*api.ProtocolHandler {
	registry := a.server.ProtocolRegistry()
	if registry == nil {
		return nil
	}

	handlers := registry.List()
	var result []*api.ProtocolHandler

	for _, h := range handlers {
		meta := h.Metadata()
		handler := &api.ProtocolHandler{
			ID:      meta.ID,
			Type:    string(meta.Protocol),
			Status:  "running",
			Version: meta.Version,
		}

		// Get port if the handler is a standalone server (MQTT, gRPC, etc.)
		if ss, ok := h.(protocol.StandaloneServer); ok {
			handler.Port = ss.Port()
		}

		// Get connection count if the handler supports it
		if cm, ok := h.(protocol.ConnectionManager); ok {
			handler.Connections = cm.ConnectionCount()
		}

		result = append(result, handler)
	}

	return result
}

// GetProtocolHandler implements api.EngineController.
func (a *ControlAPIAdapter) GetProtocolHandler(id string) *api.ProtocolHandler {
	registry := a.server.ProtocolRegistry()
	if registry == nil {
		return nil
	}

	h, exists := registry.Get(id)
	if !exists {
		return nil
	}

	meta := h.Metadata()
	handler := &api.ProtocolHandler{
		ID:      meta.ID,
		Type:    string(meta.Protocol),
		Status:  "running",
		Version: meta.Version,
	}

	// Get connection count if the handler supports it
	if cm, ok := h.(protocol.ConnectionManager); ok {
		handler.Connections = cm.ConnectionCount()
	}

	return handler
}

// ListSSEConnections implements api.EngineController.
func (a *ControlAPIAdapter) ListSSEConnections() []*api.SSEConnection {
	handler := a.server.Handler()
	if handler == nil {
		return nil
	}

	sseHandler := handler.SSEHandler()
	if sseHandler == nil {
		return nil
	}

	manager := sseHandler.GetManager()
	if manager == nil {
		return nil
	}

	streams := manager.GetConnections()
	var result []*api.SSEConnection

	for _, stream := range streams {
		conn := &api.SSEConnection{
			ID:          stream.ID,
			MockID:      stream.MockID,
			Path:        stream.Path,
			ClientIP:    stream.ClientIP,
			UserAgent:   stream.UserAgent,
			ConnectedAt: stream.StartTime,
			EventsSent:  stream.EventsSent,
			BytesSent:   stream.BytesSent,
			Status:      string(stream.Status),
		}
		result = append(result, conn)
	}

	return result
}

// GetSSEConnection implements api.EngineController.
func (a *ControlAPIAdapter) GetSSEConnection(id string) *api.SSEConnection {
	handler := a.server.Handler()
	if handler == nil {
		return nil
	}

	sseHandler := handler.SSEHandler()
	if sseHandler == nil {
		return nil
	}

	manager := sseHandler.GetManager()
	if manager == nil {
		return nil
	}

	stream := manager.Get(id)
	if stream == nil {
		return nil
	}

	return &api.SSEConnection{
		ID:          stream.ID,
		MockID:      stream.MockID,
		Path:        stream.Path,
		ClientIP:    stream.ClientIP,
		UserAgent:   stream.UserAgent,
		ConnectedAt: stream.StartTime,
		EventsSent:  stream.EventsSent,
		BytesSent:   stream.BytesSent,
		Status:      string(stream.Status),
	}
}

// CloseSSEConnection implements api.EngineController.
func (a *ControlAPIAdapter) CloseSSEConnection(id string) error {
	handler := a.server.Handler()
	if handler == nil {
		return ErrSSEHandlerNotInitialized
	}

	sseHandler := handler.SSEHandler()
	if sseHandler == nil {
		return ErrSSEHandlerNotInitialized
	}

	manager := sseHandler.GetManager()
	if manager == nil {
		return ErrSSEHandlerNotInitialized
	}

	return manager.Close(id, true, nil)
}

// GetSSEStats implements api.EngineController.
func (a *ControlAPIAdapter) GetSSEStats() *api.SSEStats {
	handler := a.server.Handler()
	if handler == nil {
		return nil
	}

	sseHandler := handler.SSEHandler()
	if sseHandler == nil {
		return nil
	}

	manager := sseHandler.GetManager()
	if manager == nil {
		return nil
	}

	stats := manager.Stats()
	return &api.SSEStats{
		TotalConnections:  stats.TotalConnections,
		ActiveConnections: stats.ActiveConnections,
		TotalEventsSent:   stats.TotalEventsSent,
		TotalBytesSent:    stats.TotalBytesSent,
		ConnectionErrors:  stats.ConnectionErrors,
		ConnectionsByMock: stats.ConnectionsByMock,
	}
}

// ListWebSocketConnections implements api.EngineController.
func (a *ControlAPIAdapter) ListWebSocketConnections() []*api.WebSocketConnection {
	handler := a.server.Handler()
	if handler == nil {
		return nil
	}

	wsManager := handler.WebSocketManager()
	if wsManager == nil {
		return nil
	}

	infos := wsManager.ListConnectionInfos("", "")
	var result []*api.WebSocketConnection

	for _, info := range infos {
		conn := &api.WebSocketConnection{
			ID:           info.ID,
			Path:         info.EndpointPath,
			ConnectedAt:  info.ConnectedAt,
			MessagesSent: info.MessagesSent,
			MessagesRecv: info.MessagesReceived,
			Status:       "connected",
		}
		result = append(result, conn)
	}

	return result
}

// GetWebSocketConnection implements api.EngineController.
func (a *ControlAPIAdapter) GetWebSocketConnection(id string) *api.WebSocketConnection {
	handler := a.server.Handler()
	if handler == nil {
		return nil
	}

	wsManager := handler.WebSocketManager()
	if wsManager == nil {
		return nil
	}

	info, err := wsManager.GetConnectionInfo(id)
	if err != nil || info == nil {
		return nil
	}

	return &api.WebSocketConnection{
		ID:           info.ID,
		Path:         info.EndpointPath,
		ConnectedAt:  info.ConnectedAt,
		MessagesSent: info.MessagesSent,
		MessagesRecv: info.MessagesReceived,
		Status:       "connected",
	}
}

// CloseWebSocketConnection implements api.EngineController.
func (a *ControlAPIAdapter) CloseWebSocketConnection(id string) error {
	handler := a.server.Handler()
	if handler == nil {
		return ErrWebSocketHandlerNotInitialized
	}

	wsManager := handler.WebSocketManager()
	if wsManager == nil {
		return ErrWebSocketHandlerNotInitialized
	}

	return wsManager.CloseConnection(id, "closed by API")
}

// SendToWebSocketConnection implements api.EngineController.
// msgType must be "text" or "binary"; data is the raw message payload.
func (a *ControlAPIAdapter) SendToWebSocketConnection(id string, msgType string, data []byte) error {
	handler := a.server.Handler()
	if handler == nil {
		return ErrWebSocketHandlerNotInitialized
	}

	wsManager := handler.WebSocketManager()
	if wsManager == nil {
		return ErrWebSocketHandlerNotInitialized
	}

	mt := websocket.MessageText
	if msgType == "binary" {
		mt = websocket.MessageBinary
	}

	return wsManager.SendToConnection(id, mt, data)
}

// GetWebSocketStats implements api.EngineController.
func (a *ControlAPIAdapter) GetWebSocketStats() *api.WebSocketStats {
	handler := a.server.Handler()
	if handler == nil {
		return nil
	}

	wsManager := handler.WebSocketManager()
	if wsManager == nil {
		return nil
	}

	stats := wsManager.WebSocketStats()
	if stats == nil {
		return nil
	}

	// Convert endpoint connections to mock connections for API consistency
	connectionsByMock := make(map[string]int)
	for endpoint, count := range stats.ConnectionsByEndpoint {
		connectionsByMock[endpoint] = count
	}

	return &api.WebSocketStats{
		TotalConnections:  int64(stats.TotalConnections),
		ActiveConnections: stats.TotalConnections, // WS manager tracks only current live connections
		TotalMessagesSent: stats.TotalMessagesSent,
		TotalMessagesRecv: stats.TotalMessagesReceived,
		ConnectionsByMock: connectionsByMock,
	}
}

// ErrMQTTBrokerNotInitialized is returned when no MQTT broker is available.
var ErrMQTTBrokerNotInitialized = errors.New("MQTT broker not initialized")

// ListMQTTConnections implements api.EngineController.
func (a *ControlAPIAdapter) ListMQTTConnections() []*api.MQTTConnection {
	brokers := a.server.GetMQTTBrokers()
	var result []*api.MQTTConnection

	for _, broker := range brokers {
		if broker == nil || !broker.IsRunning() {
			continue
		}
		infos := broker.ListClientInfos()
		for _, info := range infos {
			if info.Closed {
				continue
			}
			conn := &api.MQTTConnection{
				ID:              info.ID,
				BrokerID:        info.BrokerID,
				ConnectedAt:     info.ConnectedAt,
				Subscriptions:   info.Subscriptions,
				ProtocolVersion: info.ProtocolVersion,
				Username:        info.Username,
				RemoteAddr:      info.RemoteAddr,
				Status:          "connected",
			}
			if conn.Subscriptions == nil {
				conn.Subscriptions = []string{}
			}
			result = append(result, conn)
		}
	}

	return result
}

// GetMQTTConnection implements api.EngineController.
func (a *ControlAPIAdapter) GetMQTTConnection(id string) *api.MQTTConnection {
	brokers := a.server.GetMQTTBrokers()

	for _, broker := range brokers {
		if broker == nil || !broker.IsRunning() {
			continue
		}
		info := broker.GetClientInfo(id)
		if info == nil {
			continue
		}
		subs := info.Subscriptions
		if subs == nil {
			subs = []string{}
		}
		return &api.MQTTConnection{
			ID:              info.ID,
			BrokerID:        info.BrokerID,
			ConnectedAt:     info.ConnectedAt,
			Subscriptions:   subs,
			ProtocolVersion: info.ProtocolVersion,
			Username:        info.Username,
			RemoteAddr:      info.RemoteAddr,
			Status:          "connected",
		}
	}

	return nil
}

// CloseMQTTConnection implements api.EngineController.
func (a *ControlAPIAdapter) CloseMQTTConnection(id string) error {
	brokers := a.server.GetMQTTBrokers()

	for _, broker := range brokers {
		if broker == nil || !broker.IsRunning() {
			continue
		}
		if err := broker.DisconnectClient(id); err == nil {
			return nil
		}
	}

	return fmt.Errorf("MQTT client %q not found", id)
}

// GetMQTTStats implements api.EngineController.
func (a *ControlAPIAdapter) GetMQTTStats() *api.MQTTStats {
	brokers := a.server.GetMQTTBrokers()
	if len(brokers) == 0 {
		return nil
	}

	stats := &api.MQTTStats{
		SubscriptionsByClient: make(map[string]int),
	}

	for _, broker := range brokers {
		if broker == nil || !broker.IsRunning() {
			continue
		}
		connected, totalSubs, subsByClient := broker.GetConnectionStats()
		stats.ConnectedClients += connected
		stats.TotalSubscriptions += totalSubs
		for k, v := range subsByClient {
			stats.SubscriptionsByClient[k] = v
		}
		cfg := broker.Config()
		if cfg != nil {
			stats.TopicCount += len(cfg.Topics)
			stats.Port = cfg.Port
			stats.TLSEnabled = stats.TLSEnabled || (cfg.TLS != nil && cfg.TLS.Enabled)
			stats.AuthEnabled = stats.AuthEnabled || (cfg.Auth != nil && cfg.Auth.Enabled)
		}
	}

	return stats
}

// ErrGRPCServerNotInitialized is returned when no gRPC server is available.
var ErrGRPCServerNotInitialized = errors.New("gRPC server not initialized")

// ListGRPCStreams implements api.EngineController.
func (a *ControlAPIAdapter) ListGRPCStreams() []*api.GRPCStream {
	servers := a.server.GRPCServers()
	var result []*api.GRPCStream

	for _, srv := range servers {
		if srv == nil || !srv.IsRunning() {
			continue
		}
		for _, info := range srv.StreamTracker().List() {
			result = append(result, &api.GRPCStream{
				ID:           info.ID,
				Method:       info.Method,
				StreamType:   string(info.StreamType),
				ClientAddr:   info.ClientAddr,
				ConnectedAt:  info.ConnectedAt,
				MessagesSent: info.MessagesSent,
				MessagesRecv: info.MessagesRecv,
			})
		}
	}

	return result
}

// GetGRPCStream implements api.EngineController.
func (a *ControlAPIAdapter) GetGRPCStream(id string) *api.GRPCStream {
	servers := a.server.GRPCServers()

	for _, srv := range servers {
		if srv == nil || !srv.IsRunning() {
			continue
		}
		if info := srv.StreamTracker().Get(id); info != nil {
			return &api.GRPCStream{
				ID:           info.ID,
				Method:       info.Method,
				StreamType:   string(info.StreamType),
				ClientAddr:   info.ClientAddr,
				ConnectedAt:  info.ConnectedAt,
				MessagesSent: info.MessagesSent,
				MessagesRecv: info.MessagesRecv,
			}
		}
	}

	return nil
}

// CancelGRPCStream implements api.EngineController.
func (a *ControlAPIAdapter) CancelGRPCStream(id string) error {
	servers := a.server.GRPCServers()

	for _, srv := range servers {
		if srv == nil || !srv.IsRunning() {
			continue
		}
		if err := srv.StreamTracker().Cancel(id); err == nil {
			return nil
		}
	}

	return fmt.Errorf("gRPC stream %s not found", id)
}

// GetGRPCStats implements api.EngineController.
func (a *ControlAPIAdapter) GetGRPCStats() *api.GRPCStats {
	servers := a.server.GRPCServers()
	if len(servers) == 0 {
		return nil
	}

	stats := &api.GRPCStats{
		StreamsByMethod: make(map[string]int),
	}

	for _, srv := range servers {
		if srv == nil || !srv.IsRunning() {
			continue
		}
		s := srv.StreamTracker().Stats()
		stats.ActiveStreams += s.ActiveStreams
		stats.TotalStreams += s.TotalStreams
		stats.TotalRPCs += s.TotalRPCs
		stats.TotalMessagesSent += s.TotalMsgSent
		stats.TotalMessagesRecv += s.TotalMsgRecv
		for method, count := range s.StreamsByMethod {
			stats.StreamsByMethod[method] += count
		}
	}

	return stats
}

// GetConfig implements api.EngineController.
func (a *ControlAPIAdapter) GetConfig() *api.ConfigResponse {
	cfg := a.server.Config()
	if cfg == nil {
		return nil
	}

	return &api.ConfigResponse{
		HTTPPort:       cfg.HTTPPort,
		HTTPSPort:      cfg.HTTPSPort,
		ManagementPort: cfg.ManagementPort,
		MaxLogEntries:  cfg.MaxLogEntries,
		ReadTimeout:    cfg.ReadTimeout,
		WriteTimeout:   cfg.WriteTimeout,
	}
}

// ListCustomOperations implements api.EngineController.
func (a *ControlAPIAdapter) ListCustomOperations(workspaceID string) []api.CustomOperationInfo {
	bridge := a.server.StatefulBridge()
	if bridge == nil {
		return nil
	}

	ops := bridge.ListCustomOperations(workspaceID)
	if len(ops) == 0 {
		return nil
	}

	result := make([]api.CustomOperationInfo, 0, len(ops))
	for name, op := range ops {
		mode, _ := stateful.CustomOperationConsistency(op)
		result = append(result, api.CustomOperationInfo{
			Name:        name,
			StepCount:   len(op.Steps),
			Consistency: string(mode),
		})
	}
	return result
}

// GetCustomOperation implements api.EngineController.
func (a *ControlAPIAdapter) GetCustomOperation(workspaceID string, name string) (*api.CustomOperationDetail, error) {
	bridge := a.server.StatefulBridge()
	if bridge == nil {
		return nil, errors.New("stateful bridge not initialized")
	}

	op := bridge.GetCustomOperation(workspaceID, name)
	if op == nil {
		return nil, errors.New("custom operation not found: " + name)
	}
	mode, err := stateful.CustomOperationConsistency(op)
	if err != nil {
		return nil, err
	}

	steps := make([]api.CustomOperationStep, 0, len(op.Steps))
	for _, s := range op.Steps {
		steps = append(steps, api.CustomOperationStep{
			Type:     string(s.Type),
			Resource: s.Resource,
			ID:       s.ID,
			As:       s.As,
			Set:      s.Set,
			Var:      s.Var,
			Value:    s.Value,
		})
	}

	return &api.CustomOperationDetail{
		Name:        name,
		Consistency: string(mode),
		Steps:       steps,
		Response:    op.Response,
	}, nil
}

// RegisterCustomOperation implements api.EngineController.
func (a *ControlAPIAdapter) RegisterCustomOperation(workspaceID string, cfg *config.CustomOperationConfig) error {
	bridge := a.server.StatefulBridge()
	if bridge == nil {
		return errors.New("stateful bridge not initialized")
	}

	if cfg == nil || cfg.Name == "" {
		return errors.New("custom operation config must have a name")
	}

	customOp, err := convertCustomOperation(cfg)
	if err != nil {
		return err
	}
	bridge.RegisterCustomOperation(workspaceID, cfg.Name, customOp)
	return nil
}

// DeleteCustomOperation implements api.EngineController.
func (a *ControlAPIAdapter) DeleteCustomOperation(workspaceID string, name string) error {
	bridge := a.server.StatefulBridge()
	if bridge == nil {
		return errors.New("stateful bridge not initialized")
	}

	op := bridge.GetCustomOperation(workspaceID, name)
	if op == nil {
		return errors.New("custom operation not found: " + name)
	}

	bridge.DeleteCustomOperation(workspaceID, name)
	return nil
}

// ExecuteCustomOperation implements api.EngineController.
func (a *ControlAPIAdapter) ExecuteCustomOperation(workspaceID string, name string, input map[string]interface{}) (map[string]interface{}, error) {
	bridge := a.server.StatefulBridge()
	if bridge == nil {
		return nil, errors.New("stateful bridge not initialized")
	}

	result := bridge.Execute(context.Background(), &stateful.OperationRequest{
		Action:        stateful.ActionCustom,
		OperationName: name,
		Data:          input,
		WorkspaceID:   workspaceID,
	})

	if result.Error != nil {
		return nil, result.Error
	}

	if result.Item != nil {
		return result.Item.ToJSON(), nil
	}

	return map[string]interface{}{"success": true}, nil
}

// ControlAPI represents a control API server associated with an engine.
type ControlAPI struct {
	server  *api.Server
	adapter *ControlAPIAdapter
}

// NewControlAPI creates a new control API for the given engine server.
func NewControlAPI(s *Server, port int) *ControlAPI {
	adapter := NewControlAPIAdapter(s)
	apiServer := api.NewServer(adapter, port)
	return &ControlAPI{
		server:  apiServer,
		adapter: adapter,
	}
}

// Start starts the control API server.
func (c *ControlAPI) Start() error {
	return c.server.Start()
}

// Stop stops the control API server.
func (c *ControlAPI) Stop(ctx context.Context) error {
	return c.server.Stop(ctx)
}

// Port returns the port the control API is running on.
func (c *ControlAPI) Port() int {
	return c.server.Port()
}
