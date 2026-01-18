package engine

import (
	"context"
	"errors"

	"github.com/getmockd/mockd/pkg/chaos"
	"github.com/getmockd/mockd/pkg/config"
	"github.com/getmockd/mockd/pkg/engine/api"
	"github.com/getmockd/mockd/pkg/protocol"
	"github.com/getmockd/mockd/pkg/requestlog"
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
// Converts the api filter type to the engine filter type.
func (a *ControlAPIAdapter) GetRequestLogs(filter *api.RequestLogFilter) []*requestlog.Entry {
	if filter == nil {
		return a.server.GetRequestLogs(nil)
	}
	// Convert api filter to engine filter
	engineFilter := &RequestLogFilter{
		Limit:     filter.Limit,
		Offset:    filter.Offset,
		Method:    filter.Method,
		Path:      filter.Path,
		MatchedID: filter.MockID,
		Protocol:  filter.Protocol,
	}
	return a.server.GetRequestLogs(engineFilter)
}

// GetRequestLog implements api.EngineController.
func (a *ControlAPIAdapter) GetRequestLog(id string) *requestlog.Entry {
	return a.server.GetRequestLog(id)
}

// RequestLogCount implements api.EngineController.
func (a *ControlAPIAdapter) RequestLogCount() int {
	return a.server.RequestLogCount()
}

// ClearRequestLogs implements api.EngineController.
func (a *ControlAPIAdapter) ClearRequestLogs() {
	a.server.ClearRequestLogs()
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

	apiCfg := &api.ChaosConfig{
		Enabled: cfg.Enabled,
	}

	// Convert global rules
	if cfg.GlobalRules != nil {
		if cfg.GlobalRules.Latency != nil {
			apiCfg.Latency = &api.LatencyConfig{
				Min:         cfg.GlobalRules.Latency.Min,
				Max:         cfg.GlobalRules.Latency.Max,
				Probability: cfg.GlobalRules.Latency.Probability,
			}
		}
		if cfg.GlobalRules.ErrorRate != nil {
			apiCfg.ErrorRate = &api.ErrorRateConfig{
				Probability: cfg.GlobalRules.ErrorRate.Probability,
				StatusCodes: cfg.GlobalRules.ErrorRate.StatusCodes,
				DefaultCode: cfg.GlobalRules.ErrorRate.DefaultCode,
			}
		}
		if cfg.GlobalRules.Bandwidth != nil {
			apiCfg.Bandwidth = &api.BandwidthConfig{
				BytesPerSecond: cfg.GlobalRules.Bandwidth.BytesPerSecond,
				Probability:    cfg.GlobalRules.Bandwidth.Probability,
			}
		}
	}

	// Convert rules
	for _, rule := range cfg.Rules {
		apiCfg.Rules = append(apiCfg.Rules, api.ChaosRuleConfig{
			PathPattern: rule.PathPattern,
			Methods:     rule.Methods,
			Probability: rule.Probability,
		})
	}

	return apiCfg
}

// SetChaosConfig implements api.EngineController.
func (a *ControlAPIAdapter) SetChaosConfig(cfg *api.ChaosConfig) error {
	if cfg == nil || !cfg.Enabled {
		// Disable chaos by setting nil injector
		return a.server.SetChaosInjector(nil)
	}

	// Build chaos config
	chaosCfg := &chaos.ChaosConfig{
		Enabled: cfg.Enabled,
	}

	// Convert global rules
	if cfg.Latency != nil || cfg.ErrorRate != nil || cfg.Bandwidth != nil {
		chaosCfg.GlobalRules = &chaos.GlobalChaosRules{}
		if cfg.Latency != nil {
			chaosCfg.GlobalRules.Latency = &chaos.LatencyFault{
				Min:         cfg.Latency.Min,
				Max:         cfg.Latency.Max,
				Probability: cfg.Latency.Probability,
			}
		}
		if cfg.ErrorRate != nil {
			chaosCfg.GlobalRules.ErrorRate = &chaos.ErrorRateFault{
				Probability: cfg.ErrorRate.Probability,
				StatusCodes: cfg.ErrorRate.StatusCodes,
				DefaultCode: cfg.ErrorRate.DefaultCode,
			}
		}
		if cfg.Bandwidth != nil {
			chaosCfg.GlobalRules.Bandwidth = &chaos.BandwidthFault{
				BytesPerSecond: cfg.Bandwidth.BytesPerSecond,
				Probability:    cfg.Bandwidth.Probability,
			}
		}
	}

	// Convert rules
	for _, rule := range cfg.Rules {
		chaosCfg.Rules = append(chaosCfg.Rules, chaos.ChaosRule{
			PathPattern: rule.PathPattern,
			Methods:     rule.Methods,
			Probability: rule.Probability,
		})
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

// GetStateOverview implements api.EngineController.
func (a *ControlAPIAdapter) GetStateOverview() *api.StateOverview {
	store := a.server.StatefulStore()
	if store == nil {
		return nil
	}

	overview := store.Overview()
	if overview == nil {
		return nil
	}

	// Get detailed resource info
	var resources []api.StatefulResource
	for _, name := range overview.ResourceList {
		info, err := store.ResourceInfo(name)
		if err == nil && info != nil {
			resources = append(resources, api.StatefulResource{
				Name:        info.Name,
				BasePath:    info.BasePath,
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
func (a *ControlAPIAdapter) GetStateResource(name string) (*api.StatefulResource, error) {
	store := a.server.StatefulStore()
	if store == nil {
		return nil, ErrStatefulStoreNotInitialized
	}

	info, err := store.ResourceInfo(name)
	if err != nil {
		return nil, err
	}

	return &api.StatefulResource{
		Name:        info.Name,
		BasePath:    info.BasePath,
		ItemCount:   info.ItemCount,
		SeedCount:   info.SeedCount,
		IDField:     info.IDField,
		ParentField: info.ParentField,
	}, nil
}

// ClearStateResource implements api.EngineController.
func (a *ControlAPIAdapter) ClearStateResource(name string) (int, error) {
	store := a.server.StatefulStore()
	if store == nil {
		return 0, ErrStatefulStoreNotInitialized
	}

	return store.ClearResource(name)
}

// ResetState implements api.EngineController.
func (a *ControlAPIAdapter) ResetState(resourceName string) (*api.ResetStateResponse, error) {
	store := a.server.StatefulStore()
	if store == nil {
		return nil, ErrStatefulStoreNotInitialized
	}

	resp, err := store.Reset(resourceName)
	if err != nil {
		return nil, err
	}

	return &api.ResetStateResponse{
		Reset:     resp.Reset,
		Resources: resp.Resources,
		Message:   resp.Message,
	}, nil
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
		ActiveConnections: stats.TotalConnections,
		TotalMessagesSent: stats.TotalMessagesSent,
		TotalMessagesRecv: stats.TotalMessagesReceived,
		ConnectionsByMock: connectionsByMock,
	}
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
