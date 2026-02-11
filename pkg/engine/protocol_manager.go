// Package engine provides the core mock server engine.
package engine

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/getmockd/mockd/pkg/graphql"
	"github.com/getmockd/mockd/pkg/grpc"
	"github.com/getmockd/mockd/pkg/logging"
	"github.com/getmockd/mockd/pkg/mqtt"
	"github.com/getmockd/mockd/pkg/oauth"
	"github.com/getmockd/mockd/pkg/protocol"
	"github.com/getmockd/mockd/pkg/soap"
)

// ProtocolManager manages the lifecycle of all protocol handlers.
type ProtocolManager struct {
	registry      *protocol.Registry
	requestLogger RequestLogger
	log           *slog.Logger
	mu            sync.RWMutex

	// Protocol handlers
	graphqlHandlers    []*graphql.Handler
	graphqlSubHandlers []*graphql.SubscriptionHandler
	grpcServers        []*grpc.Server
	oauthProviders     []*oauth.Provider
	oauthHandlers      []*oauth.Handler
	soapHandlers       []*soap.Handler
	mqttBrokers        []*mqtt.Broker
}

// NewProtocolManager creates a new protocol manager.
func NewProtocolManager() *ProtocolManager {
	return &ProtocolManager{
		registry: protocol.NewRegistry(),
		log:      logging.Nop(),
	}
}

// SetLogger sets the logger for the protocol manager.
func (pm *ProtocolManager) SetLogger(log *slog.Logger) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	if log != nil {
		pm.log = log
	} else {
		pm.log = logging.Nop()
	}
}

// SetRequestLogger sets the request logger for all protocol handlers.
func (pm *ProtocolManager) SetRequestLogger(logger RequestLogger) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	pm.requestLogger = logger
}

// Registry returns the protocol handler registry.
func (pm *ProtocolManager) Registry() *protocol.Registry {
	return pm.registry
}

// StartAll starts all configured protocol handlers.
// handler is passed so GraphQL and SOAP handlers can be registered with the HTTP mux.
func (pm *ProtocolManager) StartAll(ctx context.Context, cfg *ProtocolConfig, handler *Handler) error {
	// Start GraphQL
	if err := pm.startGraphQL(cfg, handler); err != nil {
		return fmt.Errorf("failed to start GraphQL: %w", err)
	}
	// Start OAuth
	if err := pm.startOAuth(cfg, handler); err != nil {
		return fmt.Errorf("failed to start OAuth: %w", err)
	}
	// Start SOAP
	if err := pm.startSOAP(cfg, handler); err != nil {
		return fmt.Errorf("failed to start SOAP: %w", err)
	}
	// Start gRPC
	if err := pm.startGRPC(ctx, cfg); err != nil {
		return fmt.Errorf("failed to start gRPC: %w", err)
	}
	// Start MQTT
	if err := pm.startMQTT(ctx, cfg); err != nil {
		return fmt.Errorf("failed to start MQTT: %w", err)
	}
	return nil
}

// ProtocolConfig contains protocol-specific configurations.
// This is extracted from config.ServerConfiguration for the ProtocolManager.
type ProtocolConfig struct {
	GraphQL []*graphql.GraphQLConfig
	OAuth   []*oauth.OAuthConfig
	SOAP    []*soap.SOAPConfig
	GRPC    []*grpc.GRPCConfig
	MQTT    []*mqtt.MQTTConfig
}

// StopAll stops all protocol handlers.
func (pm *ProtocolManager) StopAll(ctx context.Context, timeout time.Duration) []error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	var errs []error

	// Stop MQTT brokers first (they may have active connections)
	for _, broker := range pm.mqttBrokers {
		if err := broker.Stop(ctx, timeout); err != nil {
			errs = append(errs, fmt.Errorf("MQTT broker stop: %w", err))
		}
	}
	pm.mqttBrokers = nil

	// Stop gRPC servers
	for _, server := range pm.grpcServers {
		if err := server.Stop(ctx, timeout); err != nil {
			errs = append(errs, fmt.Errorf("gRPC server stop: %w", err))
		}
	}
	pm.grpcServers = nil

	// Close GraphQL subscription handlers
	for _, subHandler := range pm.graphqlSubHandlers {
		subHandler.CloseAll("server shutting down")
	}
	pm.graphqlSubHandlers = nil
	pm.graphqlHandlers = nil

	// Clear OAuth and SOAP handlers
	pm.oauthProviders = nil
	pm.oauthHandlers = nil
	pm.soapHandlers = nil

	return errs
}

// startGraphQL initializes and registers GraphQL endpoints.
func (pm *ProtocolManager) startGraphQL(cfg *ProtocolConfig, handler *Handler) error {
	if cfg.GraphQL == nil {
		return nil
	}

	pm.mu.Lock()
	defer pm.mu.Unlock()

	for _, gqlCfg := range cfg.GraphQL {
		if gqlCfg == nil || !gqlCfg.Enabled {
			continue
		}

		// Create GraphQL handler
		gqlHandler, err := graphql.Endpoint(gqlCfg)
		if err != nil {
			return fmt.Errorf("failed to create GraphQL endpoint %s: %w", gqlCfg.Path, err)
		}

		// Set request logger for unified logging
		if pm.requestLogger != nil {
			gqlHandler.SetRequestLogger(pm.requestLogger)
		}

		pm.graphqlHandlers = append(pm.graphqlHandlers, gqlHandler)

		// Register with protocol registry for unified handler management
		if err := pm.registry.Register(gqlHandler); err != nil {
			pm.log.Warn("failed to register GraphQL handler with protocol registry", "path", gqlCfg.Path, "error", err)
		}

		// Register the handler at the configured path
		handler.RegisterGraphQLHandler(gqlCfg.Path, gqlHandler)

		// Create subscription handler if subscriptions are configured
		if len(gqlCfg.Subscriptions) > 0 {
			// Parse schema for subscription handler
			var schema *graphql.Schema
			var err error
			if gqlCfg.Schema != "" {
				schema, err = graphql.ParseSchema(gqlCfg.Schema)
			} else if gqlCfg.SchemaFile != "" {
				schema, err = graphql.ParseSchemaFile(gqlCfg.SchemaFile)
			}
			if err != nil {
				return fmt.Errorf("failed to parse GraphQL schema for subscriptions: %w", err)
			}

			subHandler := graphql.NewSubscriptionHandler(schema, gqlCfg)
			pm.graphqlSubHandlers = append(pm.graphqlSubHandlers, subHandler)

			// Register subscription handler at path/ws or path with WebSocket upgrade
			wsPath := gqlCfg.Path
			if wsPath[len(wsPath)-1] != '/' {
				wsPath += "/ws"
			} else {
				wsPath += "ws"
			}
			handler.RegisterGraphQLSubscriptionHandler(wsPath, subHandler)
		}
	}

	return nil
}

// startOAuth initializes and registers OAuth providers.
func (pm *ProtocolManager) startOAuth(cfg *ProtocolConfig, handler *Handler) error {
	if cfg.OAuth == nil {
		return nil
	}

	pm.mu.Lock()
	defer pm.mu.Unlock()

	for _, oauthCfg := range cfg.OAuth {
		if oauthCfg == nil || !oauthCfg.Enabled {
			continue
		}

		// Create OAuth provider
		provider, err := oauth.NewProvider(oauthCfg)
		if err != nil {
			return fmt.Errorf("failed to create OAuth provider: %w", err)
		}

		pm.oauthProviders = append(pm.oauthProviders, provider)

		// Create OAuth handler
		oauthHandler := oauth.NewHandler(provider)
		pm.oauthHandlers = append(pm.oauthHandlers, oauthHandler)

		// Register OAuth endpoints
		handler.RegisterOAuthHandler(oauthCfg, oauthHandler)
	}

	return nil
}

// startSOAP initializes and registers SOAP handlers.
func (pm *ProtocolManager) startSOAP(cfg *ProtocolConfig, handler *Handler) error {
	if cfg.SOAP == nil {
		return nil
	}

	pm.mu.Lock()
	defer pm.mu.Unlock()

	for _, soapCfg := range cfg.SOAP {
		if soapCfg == nil || !soapCfg.Enabled {
			continue
		}

		// Create SOAP handler
		soapHandler, err := soap.NewHandler(soapCfg)
		if err != nil {
			pm.log.Error("failed to create SOAP handler", "path", soapCfg.Path, "error", err)
			continue
		}

		// Set request logger for unified logging
		if pm.requestLogger != nil {
			soapHandler.SetRequestLogger(pm.requestLogger)
		}

		pm.soapHandlers = append(pm.soapHandlers, soapHandler)

		// Register with protocol registry for unified handler management
		if err := pm.registry.Register(soapHandler); err != nil {
			pm.log.Warn("failed to register SOAP handler with protocol registry", "path", soapCfg.Path, "error", err)
		}

		// Register the handler at the configured path
		handler.RegisterSOAPHandler(soapCfg.Path, soapHandler)
	}

	return nil
}

// startGRPC starts all configured gRPC servers.
func (pm *ProtocolManager) startGRPC(ctx context.Context, cfg *ProtocolConfig) error {
	if cfg.GRPC == nil {
		return nil
	}

	pm.mu.Lock()
	defer pm.mu.Unlock()

	for _, grpcCfg := range cfg.GRPC {
		if grpcCfg == nil || !grpcCfg.Enabled {
			continue
		}

		// Parse proto files
		protoFiles := grpcCfg.GetProtoFiles()
		schema, err := grpc.ParseProtoFiles(protoFiles, grpcCfg.ImportPaths)
		if err != nil {
			return fmt.Errorf("failed to parse proto files for gRPC server on port %d: %w", grpcCfg.Port, err)
		}

		// Create gRPC server
		server, err := grpc.NewServer(grpcCfg, schema)
		if err != nil {
			return fmt.Errorf("failed to create gRPC server on port %d: %w", grpcCfg.Port, err)
		}

		// Set request logger for unified logging
		if pm.requestLogger != nil {
			server.SetRequestLogger(pm.requestLogger)
		}

		// Start the server
		if err := server.Start(ctx); err != nil {
			return fmt.Errorf("failed to start gRPC server on port %d: %w", grpcCfg.Port, err)
		}

		// Register with protocol registry for unified handler management
		if err := pm.registry.Register(server); err != nil {
			pm.log.Warn("failed to register gRPC server with protocol registry", "port", grpcCfg.Port, "error", err)
		}

		pm.grpcServers = append(pm.grpcServers, server)
	}

	return nil
}

// startMQTT starts all configured MQTT brokers.
func (pm *ProtocolManager) startMQTT(ctx context.Context, cfg *ProtocolConfig) error {
	if cfg.MQTT == nil {
		return nil
	}

	pm.mu.Lock()
	defer pm.mu.Unlock()

	for _, mqttCfg := range cfg.MQTT {
		if mqttCfg == nil || !mqttCfg.Enabled {
			continue
		}

		// Create MQTT broker
		broker, err := mqtt.NewBroker(mqttCfg)
		if err != nil {
			return fmt.Errorf("failed to create MQTT broker on port %d: %w", mqttCfg.Port, err)
		}

		// Set request logger for unified logging
		if pm.requestLogger != nil {
			broker.SetRequestLogger(pm.requestLogger)
		}

		// Start the broker
		if err := broker.Start(ctx); err != nil {
			return fmt.Errorf("failed to start MQTT broker on port %d: %w", mqttCfg.Port, err)
		}

		// Register with protocol registry for unified handler management
		if err := pm.registry.Register(broker); err != nil {
			pm.log.Warn("failed to register MQTT broker with protocol registry", "port", mqttCfg.Port, "error", err)
		}

		pm.mqttBrokers = append(pm.mqttBrokers, broker)
	}

	return nil
}

// GRPCServers returns all running gRPC servers.
func (pm *ProtocolManager) GRPCServers() []*grpc.Server {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	result := make([]*grpc.Server, len(pm.grpcServers))
	copy(result, pm.grpcServers)
	return result
}

// GetGRPCServer returns a gRPC server by ID.
func (pm *ProtocolManager) GetGRPCServer(id string) *grpc.Server {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	for _, srv := range pm.grpcServers {
		if srv != nil && srv.ID() == id {
			return srv
		}
	}
	return nil
}

// StartGRPCServer dynamically starts a gRPC server with the given configuration.
// Returns the server instance or an error if startup fails.
func (pm *ProtocolManager) StartGRPCServer(cfg *grpc.GRPCConfig) (*grpc.Server, error) {
	if cfg == nil {
		return nil, errors.New("gRPC config cannot be nil")
	}

	pm.mu.Lock()
	defer pm.mu.Unlock()

	// Check if a server with this ID is already running
	for _, srv := range pm.grpcServers {
		if srv != nil && srv.ID() == cfg.ID {
			if srv.IsRunning() {
				return srv, nil // Already running
			}
			// Remove the stopped server from the list
			pm.removeGRPCServerLocked(cfg.ID)
			break
		}
	}

	// Check for port conflicts
	for _, srv := range pm.grpcServers {
		if srv != nil && srv.IsRunning() && srv.Config().Port == cfg.Port {
			return nil, fmt.Errorf("port %d is already in use by gRPC server %s", cfg.Port, srv.ID())
		}
	}

	// Parse proto files
	protoFiles := cfg.GetProtoFiles()
	if len(protoFiles) == 0 {
		return nil, errors.New("no proto files specified")
	}

	schema, err := grpc.ParseProtoFiles(protoFiles, cfg.ImportPaths)
	if err != nil {
		return nil, fmt.Errorf("failed to parse proto files: %w", err)
	}

	// Create gRPC server
	server, err := grpc.NewServer(cfg, schema)
	if err != nil {
		return nil, fmt.Errorf("failed to create gRPC server: %w", err)
	}

	// Set request logger for unified logging
	if pm.requestLogger != nil {
		server.SetRequestLogger(pm.requestLogger)
	}

	// Start the server
	if err := server.Start(context.Background()); err != nil {
		return nil, fmt.Errorf("failed to start gRPC server on port %d: %w", cfg.Port, err)
	}

	// Register with protocol registry for unified handler management
	if err := pm.registry.Register(server); err != nil {
		pm.log.Warn("failed to register gRPC server with protocol registry", "port", cfg.Port, "error", err)
	}

	pm.grpcServers = append(pm.grpcServers, server)
	return server, nil
}

// StopGRPCServer stops a running gRPC server by ID.
func (pm *ProtocolManager) StopGRPCServer(id string) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	for i, srv := range pm.grpcServers {
		if srv != nil && srv.ID() == id {
			if err := srv.Stop(context.Background(), 5*time.Second); err != nil {
				return fmt.Errorf("failed to stop gRPC server %s: %w", id, err)
			}
			// Unregister from protocol registry
			if err := pm.registry.Unregister(id); err != nil {
				pm.log.Warn("failed to unregister gRPC server from protocol registry", "id", id, "error", err)
			}
			// Remove from slice
			pm.grpcServers = append(pm.grpcServers[:i], pm.grpcServers[i+1:]...)
			return nil
		}
	}

	return nil // Server not found, nothing to stop
}

// removeGRPCServerLocked removes a gRPC server from the list (caller must hold the lock).
func (pm *ProtocolManager) removeGRPCServerLocked(id string) {
	for i, srv := range pm.grpcServers {
		if srv != nil && srv.ID() == id {
			pm.grpcServers = append(pm.grpcServers[:i], pm.grpcServers[i+1:]...)
			return
		}
	}
}

// MQTTPorts returns the ports of all running MQTT brokers.
func (pm *ProtocolManager) MQTTPorts() []int {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	var ports []int
	for _, broker := range pm.mqttBrokers {
		if broker != nil && broker.IsRunning() {
			ports = append(ports, broker.Config().Port)
		}
	}

	return ports
}

// StartMQTTBroker dynamically starts an MQTT broker with the given configuration.
// Each mock gets its own broker on its specified port.
// Returns the broker instance or an error if the broker could not be started.
func (pm *ProtocolManager) StartMQTTBroker(cfg *mqtt.MQTTConfig) (*mqtt.Broker, error) {
	if cfg == nil {
		return nil, errors.New("MQTT config cannot be nil")
	}

	pm.mu.Lock()
	defer pm.mu.Unlock()

	// Check if a broker with this ID is already running
	for _, broker := range pm.mqttBrokers {
		if broker != nil && broker.ID() == cfg.ID {
			if broker.IsRunning() {
				return broker, nil // Already running
			}
			// Remove the stopped broker from the list
			pm.removeBrokerLocked(cfg.ID)
			break
		}
	}

	// Check for port conflicts - one port = one mock
	for _, broker := range pm.mqttBrokers {
		if broker != nil && broker.IsRunning() && broker.Config().Port == cfg.Port {
			return nil, fmt.Errorf("port %d is already in use by MQTT broker %s", cfg.Port, broker.ID())
		}
	}

	// Create the broker
	broker, err := mqtt.NewBroker(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create MQTT broker: %w", err)
	}

	// Set request logger for unified logging
	if pm.requestLogger != nil {
		broker.SetRequestLogger(pm.requestLogger)
	}

	// Start the broker
	if err := broker.Start(context.Background()); err != nil {
		return nil, fmt.Errorf("failed to start MQTT broker on port %d: %w", cfg.Port, err)
	}

	// Register with protocol registry for unified handler management
	if err := pm.registry.Register(broker); err != nil {
		pm.log.Warn("failed to register MQTT broker with protocol registry", "port", cfg.Port, "error", err)
	}

	pm.mqttBrokers = append(pm.mqttBrokers, broker)
	return broker, nil
}

// StopMQTTBroker stops a running MQTT broker by ID.
func (pm *ProtocolManager) StopMQTTBroker(id string) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	for i, broker := range pm.mqttBrokers {
		if broker != nil && broker.ID() == id {
			if err := broker.Stop(context.Background(), 5*time.Second); err != nil {
				return fmt.Errorf("failed to stop MQTT broker %s: %w", id, err)
			}
			// Unregister from protocol registry
			if err := pm.registry.Unregister(id); err != nil {
				pm.log.Warn("failed to unregister MQTT broker from protocol registry", "id", id, "error", err)
			}
			// Remove from slice
			pm.mqttBrokers = append(pm.mqttBrokers[:i], pm.mqttBrokers[i+1:]...)
			return nil
		}
	}

	return nil // Broker not found, nothing to stop
}

// GetMQTTBroker returns a running MQTT broker by ID.
func (pm *ProtocolManager) GetMQTTBroker(id string) *mqtt.Broker {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	for _, broker := range pm.mqttBrokers {
		if broker != nil && broker.ID() == id {
			return broker
		}
	}
	return nil
}

// GetMQTTBrokers returns all MQTT brokers (running or not).
func (pm *ProtocolManager) GetMQTTBrokers() []*mqtt.Broker {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	result := make([]*mqtt.Broker, len(pm.mqttBrokers))
	copy(result, pm.mqttBrokers)
	return result
}

// removeBrokerLocked removes a broker from the list (caller must hold the lock).
func (pm *ProtocolManager) removeBrokerLocked(id string) {
	for i, broker := range pm.mqttBrokers {
		if broker != nil && broker.ID() == id {
			pm.mqttBrokers = append(pm.mqttBrokers[:i], pm.mqttBrokers[i+1:]...)
			return
		}
	}
}

// GraphQLHandlers returns all GraphQL handlers.
func (pm *ProtocolManager) GraphQLHandlers() []*graphql.Handler {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	result := make([]*graphql.Handler, len(pm.graphqlHandlers))
	copy(result, pm.graphqlHandlers)
	return result
}

// AddGraphQLHandler adds a GraphQL handler to the manager.
// This is used when loading mocks from config files.
func (pm *ProtocolManager) AddGraphQLHandler(handler *graphql.Handler) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	pm.graphqlHandlers = append(pm.graphqlHandlers, handler)
}

// AddGraphQLSubscriptionHandler adds a GraphQL subscription handler to the manager.
// This is used when loading subscriptions from config files so they are tracked for shutdown.
func (pm *ProtocolManager) AddGraphQLSubscriptionHandler(handler *graphql.SubscriptionHandler) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	pm.graphqlSubHandlers = append(pm.graphqlSubHandlers, handler)
}

// SOAPHandlers returns all SOAP handlers.
func (pm *ProtocolManager) SOAPHandlers() []*soap.Handler {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	result := make([]*soap.Handler, len(pm.soapHandlers))
	copy(result, pm.soapHandlers)
	return result
}

// OAuthProviders returns all OAuth providers.
func (pm *ProtocolManager) OAuthProviders() []*oauth.Provider {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	result := make([]*oauth.Provider, len(pm.oauthProviders))
	copy(result, pm.oauthProviders)
	return result
}

// OAuthHandlers returns all OAuth handlers.
func (pm *ProtocolManager) OAuthHandlers() []*oauth.Handler {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	result := make([]*oauth.Handler, len(pm.oauthHandlers))
	copy(result, pm.oauthHandlers)
	return result
}
