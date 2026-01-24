// Package engine provides the core mock server engine.
package engine

import (
	"context"
	"crypto/tls"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/getmockd/mockd/internal/storage"
	"github.com/getmockd/mockd/pkg/chaos"
	"github.com/getmockd/mockd/pkg/config"
	"github.com/getmockd/mockd/pkg/grpc"
	"github.com/getmockd/mockd/pkg/logging"
	"github.com/getmockd/mockd/pkg/mock"
	"github.com/getmockd/mockd/pkg/mqtt"
	"github.com/getmockd/mockd/pkg/protocol"
	"github.com/getmockd/mockd/pkg/requestlog"
	"github.com/getmockd/mockd/pkg/stateful"
	"github.com/getmockd/mockd/pkg/store"
	"github.com/getmockd/mockd/pkg/tracing"
	"github.com/getmockd/mockd/pkg/validation"
)

// findFreePort finds a free port starting from the given port.
// It checks up to 100 ports from the starting port.
func findFreePort(startPort int) int {
	for port := startPort; port < startPort+100; port++ {
		listener, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
		if err == nil {
			_ = listener.Close()
			return port
		}
	}
	// Fallback to a random port if no port in range is available
	//nolint:gosec // G102: binding to all interfaces is intentional for mock server
	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		return startPort // Return start port as last resort
	}
	defer func() { _ = listener.Close() }()
	tcpAddr, ok := listener.Addr().(*net.TCPAddr)
	if !ok {
		return startPort
	}
	return tcpAddr.Port
}

// Server is the main mock server engine.
type Server struct {
	cfg             *config.ServerConfiguration
	persistentStore store.Store // Optional persistent storage backend
	statefulStore   *stateful.StateStore
	requestLogger   RequestLogger // For request history (user-facing)
	log             *slog.Logger  // For operational logging (developer-facing)
	httpServer      *http.Server
	httpsServer     *http.Server
	handler         *Handler
	httpHandler     http.Handler // The actual handler used by servers (may be wrapped with middleware)
	middlewareChain *MiddlewareChain
	tlsManager      *TLSManager
	tlsConfig       *tls.Config
	mu              sync.RWMutex
	running         bool
	startTime       time.Time

	// Protocol handler registry for unified handler management
	protocolRegistry *protocol.Registry

	// Protocol manager for all protocol handlers
	protocolManager *ProtocolManager

	// Mock manager for mock CRUD operations
	mockManager *MockManager

	// Config loader for loading/saving mock configurations
	configLoader *ConfigLoader

	// Control API for engine management
	controlAPI *ControlAPI

	// Tracer for distributed tracing (optional)
	tracer *tracing.Tracer
}

// ServerOption is a functional option for configuring a Server.
type ServerOption func(*Server)

// WithStore sets a persistent store backend for the server.
// The store is used for persisting mock configurations and other data.
func WithStore(persistentStore store.Store) ServerOption {
	return func(s *Server) {
		s.persistentStore = persistentStore
	}
}

// WithLogger sets the operational logger for the server.
func WithLogger(log *slog.Logger) ServerOption {
	return func(s *Server) {
		if log != nil {
			s.log = log
		}
	}
}

// WithTracer sets the tracer for distributed tracing.
// When set, tracing middleware will be applied to capture request spans.
func WithTracer(t *tracing.Tracer) ServerOption {
	return func(s *Server) {
		s.tracer = t
	}
}

// NewServer creates a new Server with the given configuration.
// Optional ServerOption functions can be passed to customize the server.
func NewServer(cfg *config.ServerConfiguration, opts ...ServerOption) *Server {
	if cfg == nil {
		cfg = config.DefaultServerConfiguration()
	}

	// Create initial server with default values
	s := &Server{
		cfg: cfg,
		log: logging.Nop(), // Default to no-op, can be set with SetLogger
	}

	// Apply options first to capture any persistent store
	for _, opt := range opts {
		opt(s)
	}

	// Initialize mock store based on whether a persistent store was provided
	var mockStore storage.MockStore
	if s.persistentStore != nil {
		mockStore = NewPersistentMockStore(s.persistentStore.Mocks())
	} else {
		mockStore = storage.NewInMemoryMockStore()
	}

	// Initialize core components
	statefulStore := stateful.NewStateStore()
	handler := NewHandler(mockStore)
	handler.SetStatefulStore(statefulStore)

	maxLogEntries := cfg.MaxLogEntries
	if maxLogEntries <= 0 {
		maxLogEntries = 1000 // Default
	}
	logger := NewInMemoryRequestLogger(maxLogEntries)
	handler.SetLogger(logger)

	pm := NewProtocolManager()
	pm.SetRequestLogger(logger)

	mockManager := NewMockManager(mockStore, handler, pm)

	// Set remaining server fields
	s.statefulStore = statefulStore
	s.requestLogger = logger
	s.handler = handler
	s.protocolRegistry = pm.Registry()
	s.protocolManager = pm
	s.tlsManager = NewTLSManagerFromServerConfig(cfg)
	s.mockManager = mockManager

	// Initialize config loader (needs server reference)
	s.configLoader = NewConfigLoader(s)

	// Wire up WebSocket and SSE loggers
	handler.WebSocketManager().SetRequestLogger(logger)
	handler.SSEHandler().SetRequestLogger(logger)

	// Initialize management API if port is configured
	managementPort := cfg.ManagementPort
	if managementPort == 0 {
		// Find a free port for the management API starting from 4281
		managementPort = findFreePort(4281)
	}
	s.controlAPI = NewControlAPI(s, managementPort)

	return s
}

// NewServerWithMocks creates a new Server with pre-loaded mocks.
//
// Note: Consider using NewServer followed by HTTP API calls for more control,
// or use this convenience function when you have a set of mocks to load at startup.
func NewServerWithMocks(cfg *config.ServerConfiguration, mocks []*config.MockConfiguration, opts ...ServerOption) *Server {
	srv := NewServer(cfg, opts...)
	for _, mock := range mocks {
		if mock != nil {
			if err := srv.addMock(mock); err != nil {
				srv.log.Warn("failed to add mock at startup", "id", mock.ID, "error", err)
			}
		}
	}
	return srv
}

// NewServerWithStore creates a new Server with a persistent store backend.
// The store is used for persisting mock configurations and other data.
// If the store is nil, an in-memory store is used (no persistence).
//
// Deprecated: Use NewServer with WithStore option instead:
//
//	NewServer(cfg, WithStore(persistentStore))
func NewServerWithStore(cfg *config.ServerConfiguration, persistentStore store.Store) *Server {
	return NewServer(cfg, WithStore(persistentStore))
}

// SetStore sets the persistent store backend.
// This can be called after server creation to enable persistence.
func (s *Server) SetStore(persistentStore store.Store) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.persistentStore = persistentStore

	// If we have a persistent store, wrap it for mock storage
	if persistentStore != nil {
		mockStore := NewPersistentMockStore(persistentStore.Mocks())
		s.mockManager.SetStore(mockStore)
		s.handler.SetStore(mockStore)
	}
}

// PersistentStore returns the persistent store backend, if any.
func (s *Server) PersistentStore() store.Store {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.persistentStore
}

// LoadFromStore loads all configurations from the persistent store into the engine.
// This should be called after setting the store and before starting the server.
// It loads mocks from the unified mock store.
func (s *Server) LoadFromStore(ctx context.Context) error {
	return s.configLoader.LoadFromStore(ctx, s.persistentStore)
}

// Start starts the HTTP and HTTPS servers.
func (s *Server) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.running {
		return fmt.Errorf("server is already running")
	}

	// Initialize middleware chain (handles validation, chaos, audit, and tracing)
	var mcOpts []MiddlewareChainOption
	if s.tracer != nil {
		mcOpts = append(mcOpts, WithChainTracer(s.tracer))
	}
	mc, err := NewMiddlewareChain(s.cfg, mcOpts...)
	if err != nil {
		return err
	}
	s.middlewareChain = mc

	// Wrap handler with middleware chain
	s.httpHandler = s.middlewareChain.Wrap(s.handler)

	// Build protocol config from server config
	protocolCfg := &ProtocolConfig{
		GraphQL: s.cfg.GraphQL,
		OAuth:   s.cfg.OAuth,
		SOAP:    s.cfg.SOAP,
		GRPC:    s.cfg.GRPC,
		MQTT:    s.cfg.MQTT,
	}

	// Start all protocol handlers via the protocol manager
	if err := s.protocolManager.StartAll(context.Background(), protocolCfg, s.handler); err != nil {
		return err
	}

	// Start HTTP server if configured
	if s.cfg.HTTPPort > 0 {
		s.httpServer = &http.Server{
			Addr:         fmt.Sprintf(":%d", s.cfg.HTTPPort),
			Handler:      s.httpHandler,
			ReadTimeout:  time.Duration(s.cfg.ReadTimeout) * time.Second,
			WriteTimeout: time.Duration(s.cfg.WriteTimeout) * time.Second,
		}

		s.log.Info("starting HTTP server", "port", s.cfg.HTTPPort)
		go func() {
			if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				s.log.Error("HTTP server error", "error", err)
			}
		}()
	}

	// Start HTTPS server if configured
	if s.cfg.HTTPSPort > 0 {
		var err error
		s.tlsConfig, err = s.tlsManager.BuildConfig()
		if err != nil {
			return fmt.Errorf("failed to setup TLS: %w", err)
		}

		s.httpsServer = &http.Server{
			Addr:         fmt.Sprintf(":%d", s.cfg.HTTPSPort),
			Handler:      s.httpHandler,
			TLSConfig:    s.tlsConfig,
			ReadTimeout:  time.Duration(s.cfg.ReadTimeout) * time.Second,
			WriteTimeout: time.Duration(s.cfg.WriteTimeout) * time.Second,
		}

		go func() {
			if err := s.httpsServer.ListenAndServeTLS("", ""); err != nil && err != http.ErrServerClosed {
				s.log.Error("HTTPS server error", "error", err)
			}
		}()
	}

	// Start the control API
	if s.controlAPI != nil {
		if err := s.controlAPI.Start(); err != nil {
			s.log.Error("control API server error", "error", err)
			// Non-fatal: log but continue
		}
	}

	s.running = true
	s.startTime = time.Now()
	s.log.Info("engine started", "http_port", s.cfg.HTTPPort, "https_port", s.cfg.HTTPSPort)
	return nil
}

// Stop gracefully shuts down the server.
func (s *Server) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var errs []error

	// Stop the control API
	if s.controlAPI != nil {
		if err := s.controlAPI.Stop(ctx); err != nil {
			errs = append(errs, fmt.Errorf("control API shutdown: %w", err))
		}
	}

	// Stop all protocol handlers via the protocol manager
	if protocolErrs := s.protocolManager.StopAll(ctx, 5*time.Second); len(protocolErrs) > 0 {
		errs = append(errs, protocolErrs...)
	}

	if s.httpServer != nil {
		if err := s.httpServer.Shutdown(ctx); err != nil {
			errs = append(errs, fmt.Errorf("HTTP shutdown: %w", err))
		}
	}

	if s.httpsServer != nil {
		if err := s.httpsServer.Shutdown(ctx); err != nil {
			errs = append(errs, fmt.Errorf("HTTPS shutdown: %w", err))
		}
	}

	// Close middleware chain (handles audit logger cleanup)
	if s.middlewareChain != nil {
		if err := s.middlewareChain.Close(); err != nil {
			errs = append(errs, fmt.Errorf("middleware chain close: %w", err))
		}
		s.middlewareChain = nil
	}

	s.running = false

	if len(errs) > 0 {
		return errs[0]
	}
	return nil
}

// addMock adds a new mock configuration to the server.
// This is an internal method - external callers should use the HTTP API.
func (s *Server) addMock(cfg *config.MockConfiguration) error {
	return s.mockManager.Add(cfg)
}

// updateMock updates an existing mock configuration.
// This is an internal method - external callers should use the HTTP API.
func (s *Server) updateMock(id string, cfg *config.MockConfiguration) error {
	return s.mockManager.Update(id, cfg)
}

// deleteMock removes a mock configuration.
// This is an internal method - external callers should use the HTTP API.
func (s *Server) deleteMock(id string) error {
	return s.mockManager.Delete(id)
}

// getMock retrieves a mock by ID.
// This is an internal method - external callers should use the HTTP API.
func (s *Server) getMock(id string) *config.MockConfiguration {
	return s.mockManager.Get(id)
}

// listMocks returns all mock configurations (all types).
// This is an internal method - external callers should use the HTTP API.
func (s *Server) listMocks() []*config.MockConfiguration {
	return s.mockManager.List()
}

// listHTTPMocks returns only HTTP mock configurations.
// This is an internal method - external callers should use the HTTP API.
func (s *Server) listHTTPMocks() []*config.MockConfiguration {
	return s.mockManager.ListByType(mock.MockTypeHTTP)
}

// IsRunning returns whether the server is running.
func (s *Server) IsRunning() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.running
}

// Uptime returns the server uptime in seconds.
func (s *Server) Uptime() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if !s.running {
		return 0
	}
	return int(time.Since(s.startTime).Seconds())
}

// Config returns the server configuration.
func (s *Server) Config() *config.ServerConfiguration {
	return s.cfg
}

// ManagementPort returns the port the management API is running on.
// This is the internal HTTP API used by the Admin server to manage mocks.
func (s *Server) ManagementPort() int {
	if s.controlAPI != nil {
		return s.controlAPI.Port()
	}
	return 0
}

// Store returns the mock store (for admin API use).
func (s *Server) Store() storage.MockStore {
	return s.mockManager.Store()
}

// SaveConfig saves the current mock configurations to a file.
func (s *Server) SaveConfig(path string, name string) error {
	return s.configLoader.SaveToFile(path, name)
}

// LoadConfig loads mock configurations from a file and adds them to the server.
// If replace is true, existing mocks are cleared first.
func (s *Server) LoadConfig(path string, replace bool) error {
	return s.configLoader.LoadFromFile(path, replace)
}

// LoadConfigFromBytes loads mock configurations from JSON bytes and adds them to the server.
// If replace is true, existing mocks are cleared first.
func (s *Server) LoadConfigFromBytes(data []byte, replace bool) error {
	return s.configLoader.LoadFromBytes(data, replace)
}

// ExportConfig exports the current configuration as a MockCollection.
func (s *Server) ExportConfig(name string) *config.MockCollection {
	return s.configLoader.Export(name)
}

// ImportConfig imports a MockCollection, optionally replacing existing mocks.
func (s *Server) ImportConfig(collection *config.MockCollection, replace bool) error {
	return s.configLoader.Import(collection, replace)
}

// GetRequestLogs returns request logs, optionally filtered.
func (s *Server) GetRequestLogs(filter *RequestLogFilter) []*requestlog.Entry {
	if s.requestLogger == nil {
		return nil
	}
	return s.requestLogger.List(filter)
}

// GetRequestLog returns a single request log by ID.
func (s *Server) GetRequestLog(id string) *requestlog.Entry {
	if s.requestLogger == nil {
		return nil
	}
	return s.requestLogger.Get(id)
}

// ClearRequestLogs clears all request logs.
func (s *Server) ClearRequestLogs() {
	if s.requestLogger != nil {
		s.requestLogger.Clear()
	}
}

// RequestLogCount returns the number of request logs.
func (s *Server) RequestLogCount() int {
	if s.requestLogger == nil {
		return 0
	}
	return s.requestLogger.Count()
}

// Logger returns the request logger (for admin API use).
func (s *Server) Logger() RequestLogger {
	return s.requestLogger
}

// SetLogger sets the operational logger for the server and its handler.
// This logger is used for server-level events (startup, errors, warnings).
func (s *Server) SetLogger(log *slog.Logger) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if log != nil {
		s.log = log
		// Propagate to handler with "handler" sub-component
		if s.handler != nil {
			s.handler.SetOperationalLogger(log.With("subcomponent", "handler"))
		}
	} else {
		s.log = logging.Nop()
	}
}

// StatefulStore returns the stateful resource store (for admin API use).
func (s *Server) StatefulStore() *stateful.StateStore {
	return s.statefulStore
}

// registerStatefulResource registers a stateful resource from config.
// This is an internal method - external callers should use the HTTP API.
func (s *Server) registerStatefulResource(cfg *config.StatefulResourceConfig) error {
	return s.statefulStore.Register(&stateful.ResourceConfig{
		Name:        cfg.Name,
		BasePath:    cfg.BasePath,
		IDField:     cfg.IDField,
		ParentField: cfg.ParentField,
		SeedData:    cfg.SeedData,
	})
}

// Handler returns the request handler (for admin API use).
func (s *Server) Handler() *Handler {
	return s.handler
}

// registerWebSocketEndpoint registers a WebSocket endpoint from config.
// This is an internal method - external callers should use the HTTP API.
func (s *Server) registerWebSocketEndpoint(cfg *config.WebSocketEndpointConfig) error {
	return s.handler.RegisterWebSocketEndpoint(cfg)
}

// GraphQLEndpoints returns the paths of all registered GraphQL endpoints.
func (s *Server) GraphQLEndpoints() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Return paths from config
	var endpoints []string
	if s.cfg.GraphQL != nil {
		for _, gqlCfg := range s.cfg.GraphQL {
			if gqlCfg != nil && gqlCfg.Enabled {
				endpoints = append(endpoints, gqlCfg.Path)
			}
		}
	}

	return endpoints
}

// GRPCPorts returns the ports of all running gRPC servers.
func (s *Server) GRPCPorts() []int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var ports []int
	if s.cfg.GRPC != nil {
		for _, grpcCfg := range s.cfg.GRPC {
			if grpcCfg != nil && grpcCfg.Enabled {
				ports = append(ports, grpcCfg.Port)
			}
		}
	}

	return ports
}

// GRPCServers returns all running gRPC servers.
func (s *Server) GRPCServers() []*grpc.Server {
	return s.protocolManager.GRPCServers()
}

// GetGRPCServer returns a gRPC server by ID.
func (s *Server) GetGRPCServer(id string) *grpc.Server {
	return s.protocolManager.GetGRPCServer(id)
}

// StartGRPCServer dynamically starts a gRPC server with the given configuration.
// Returns the server instance or an error if startup fails.
func (s *Server) StartGRPCServer(cfg *grpc.GRPCConfig) (*grpc.Server, error) {
	return s.protocolManager.StartGRPCServer(cfg)
}

// StopGRPCServer stops a running gRPC server by ID.
func (s *Server) StopGRPCServer(id string) error {
	return s.protocolManager.StopGRPCServer(id)
}

// MQTTPorts returns the ports of all running MQTT brokers.
func (s *Server) MQTTPorts() []int {
	return s.protocolManager.MQTTPorts()
}

// StartMQTTBroker dynamically starts an MQTT broker with the given configuration.
// Returns the broker instance or an error if the broker could not be started.
func (s *Server) StartMQTTBroker(cfg *mqtt.MQTTConfig) (*mqtt.Broker, error) {
	return s.protocolManager.StartMQTTBroker(cfg)
}

// StopMQTTBroker stops a running MQTT broker by ID.
func (s *Server) StopMQTTBroker(id string) error {
	return s.protocolManager.StopMQTTBroker(id)
}

// GetMQTTBroker returns a running MQTT broker by ID.
func (s *Server) GetMQTTBroker(id string) *mqtt.Broker {
	return s.protocolManager.GetMQTTBroker(id)
}

// GetMQTTBrokers returns all MQTT brokers (running or not).
func (s *Server) GetMQTTBrokers() []*mqtt.Broker {
	return s.protocolManager.GetMQTTBrokers()
}

// OAuthIssuers returns the issuers of all running OAuth providers.
func (s *Server) OAuthIssuers() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var issuers []string
	if s.cfg.OAuth != nil {
		for _, oauthCfg := range s.cfg.OAuth {
			if oauthCfg != nil && oauthCfg.Enabled {
				issuers = append(issuers, oauthCfg.Issuer)
			}
		}
	}

	return issuers
}

// SOAPEndpoints returns the paths of all registered SOAP endpoints.
func (s *Server) SOAPEndpoints() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var endpoints []string
	if s.cfg.SOAP != nil {
		for _, soapCfg := range s.cfg.SOAP {
			if soapCfg != nil && soapCfg.Enabled {
				endpoints = append(endpoints, soapCfg.Path)
			}
		}
	}

	return endpoints
}

// ChaosEnabled returns whether chaos injection is enabled.
func (s *Server) ChaosEnabled() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.middlewareChain == nil {
		return false
	}
	return s.middlewareChain.ChaosEnabled()
}

// ValidationEnabled returns whether OpenAPI validation is enabled.
func (s *Server) ValidationEnabled() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.middlewareChain == nil {
		return false
	}
	return s.middlewareChain.ValidationEnabled()
}

// ChaosInjector returns the chaos injector (for admin API use).
func (s *Server) ChaosInjector() *chaos.Injector {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.middlewareChain == nil {
		return nil
	}
	return s.middlewareChain.ChaosInjector()
}

// SetChaosInjector sets the chaos injector for dynamic chaos injection.
// Unlike startup-time configuration, this updates the injector dynamically
// and requires the middlewareChain to be in place.
func (s *Server) SetChaosInjector(injector *chaos.Injector) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.middlewareChain == nil {
		return fmt.Errorf("middleware chain not initialized")
	}
	s.middlewareChain.SetChaosInjector(injector)
	return nil
}

// Validator returns the OpenAPI validator (for admin API use).
func (s *Server) Validator() *validation.OpenAPIValidator {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.middlewareChain == nil {
		return nil
	}
	return s.middlewareChain.Validator()
}

// Tracer returns the tracer, if configured.
func (s *Server) Tracer() *tracing.Tracer {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.tracer
}

// ProtocolRegistry returns the protocol handler registry.
// Returns nil if no registry has been set.
func (s *Server) ProtocolRegistry() *protocol.Registry {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.protocolRegistry
}

// SetProtocolRegistry sets the protocol handler registry.
func (s *Server) SetProtocolRegistry(reg *protocol.Registry) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.protocolRegistry = reg
}

// clearMocks removes all mocks from the engine.
// This is an internal method - external callers should use the HTTP API.
func (s *Server) clearMocks() {
	s.mockManager.Clear()
}

// ProtocolStatus returns the status of all protocols.
func (s *Server) ProtocolStatus() map[string]ProtocolStatusInfo {
	status := make(map[string]ProtocolStatusInfo)

	// HTTP is always enabled if server is running
	httpStatus := "stopped"
	if s.running {
		httpStatus = "running"
	}
	status["http"] = ProtocolStatusInfo{
		Enabled: s.running,
		Port:    s.cfg.HTTPPort,
		Status:  httpStatus,
	}

	// HTTPS status
	if s.cfg.HTTPSPort > 0 {
		httpsStatus := "stopped"
		if s.running {
			httpsStatus = "running"
		}
		status["https"] = ProtocolStatusInfo{
			Enabled: s.running && s.cfg.HTTPSPort > 0,
			Port:    s.cfg.HTTPSPort,
			Status:  httpsStatus,
		}
	}

	// WebSocket status (shares HTTP port)
	status["websocket"] = ProtocolStatusInfo{
		Enabled:     s.running,
		Port:        s.cfg.HTTPPort,
		Connections: s.handler.WebSocketManager().ConnectionCount(),
		Status:      httpStatus,
	}

	// SSE status (shares HTTP port)
	status["sse"] = ProtocolStatusInfo{
		Enabled:     s.running,
		Port:        s.cfg.HTTPPort,
		Connections: s.handler.SSEHandler().ConnectionCount(),
		Status:      httpStatus,
	}

	// gRPC status
	grpcServers := s.protocolManager.GRPCServers()
	for _, srv := range grpcServers {
		if srv != nil && srv.IsRunning() {
			status["grpc"] = ProtocolStatusInfo{
				Enabled: true,
				Port:    srv.Config().Port,
				Status:  "running",
			}
			break
		}
	}

	// MQTT status
	mqttBrokers := s.protocolManager.GetMQTTBrokers()
	for _, broker := range mqttBrokers {
		if broker != nil && broker.IsRunning() {
			stats := broker.GetStats()
			status["mqtt"] = ProtocolStatusInfo{
				Enabled:     true,
				Port:        broker.Config().Port,
				Connections: stats.ClientCount,
				Status:      "running",
			}
			break
		}
	}

	// GraphQL status (shares HTTP port)
	graphqlHandlers := s.protocolManager.GraphQLHandlers()
	if len(graphqlHandlers) > 0 && s.running {
		status["graphql"] = ProtocolStatusInfo{
			Enabled: true,
			Port:    s.cfg.HTTPPort,
			Status:  "running",
		}
	}

	// SOAP status (shares HTTP port)
	soapHandlers := s.protocolManager.SOAPHandlers()
	if len(soapHandlers) > 0 && s.running {
		status["soap"] = ProtocolStatusInfo{
			Enabled: true,
			Port:    s.cfg.HTTPPort,
			Status:  "running",
		}
	}

	// OAuth status (shares HTTP port)
	oauthHandlers := s.protocolManager.OAuthHandlers()
	if len(oauthHandlers) > 0 && s.running {
		status["oauth"] = ProtocolStatusInfo{
			Enabled: true,
			Port:    s.cfg.HTTPPort,
			Status:  "running",
		}
	}

	return status
}

// ProtocolStatusInfo contains status information for a protocol.
type ProtocolStatusInfo struct {
	Enabled     bool   `json:"enabled"`
	Port        int    `json:"port,omitempty"`
	Connections int    `json:"connections,omitempty"`
	Status      string `json:"status,omitempty"`
}
