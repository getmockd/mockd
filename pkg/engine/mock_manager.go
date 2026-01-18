// Package engine provides the core mock server engine.
package engine

import (
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/getmockd/mockd/internal/id"
	"github.com/getmockd/mockd/internal/storage"
	"github.com/getmockd/mockd/pkg/config"
	"github.com/getmockd/mockd/pkg/graphql"
	"github.com/getmockd/mockd/pkg/grpc"
	"github.com/getmockd/mockd/pkg/logging"
	"github.com/getmockd/mockd/pkg/mock"
	"github.com/getmockd/mockd/pkg/mqtt"
	"github.com/getmockd/mockd/pkg/oauth"
	"github.com/getmockd/mockd/pkg/soap"
)

// MockManager handles mock CRUD operations and protocol-specific registration.
type MockManager struct {
	mu              sync.RWMutex
	store           storage.MockStore
	handler         *Handler
	protocolManager *ProtocolManager
	log             *slog.Logger
}

// NewMockManager creates a new mock manager.
func NewMockManager(store storage.MockStore, handler *Handler, pm *ProtocolManager) *MockManager {
	return &MockManager{
		store:           store,
		handler:         handler,
		protocolManager: pm,
		log:             logging.Nop(),
	}
}

// SetLogger sets the logger.
func (mm *MockManager) SetLogger(log *slog.Logger) {
	mm.mu.Lock()
	defer mm.mu.Unlock()
	if log != nil {
		mm.log = log
	}
}

// Add adds a new mock configuration.
func (mm *MockManager) Add(cfg *config.MockConfiguration) error {
	if cfg == nil {
		return fmt.Errorf("mock cannot be nil")
	}

	// Generate ID if not provided
	if cfg.ID == "" {
		cfg.ID = id.UUID()
	}

	// Set timestamps
	now := time.Now()
	if cfg.CreatedAt.IsZero() {
		cfg.CreatedAt = now
	}
	cfg.UpdatedAt = now

	// Set default enabled state
	if !cfg.Enabled {
		cfg.Enabled = true
	}

	// Validate
	if err := cfg.Validate(); err != nil {
		return err
	}

	mm.mu.Lock()
	defer mm.mu.Unlock()

	// Check for duplicate ID
	if mm.store.Exists(cfg.ID) {
		return fmt.Errorf("mock with ID %s already exists", cfg.ID)
	}

	// Store in mock store
	if err := mm.store.Set(cfg); err != nil {
		return err
	}

	// Register protocol-specific handlers based on mock type
	mm.registerHandlerLocked(cfg)

	return nil
}

// Update updates an existing mock.
func (mm *MockManager) Update(id string, cfg *config.MockConfiguration) error {
	if cfg == nil {
		return fmt.Errorf("mock cannot be nil")
	}

	mm.mu.Lock()
	defer mm.mu.Unlock()

	existing := mm.store.Get(id)
	if existing == nil {
		return fmt.Errorf("mock with ID %s not found", id)
	}

	// Preserve ID and creation time
	cfg.ID = id
	cfg.CreatedAt = existing.CreatedAt
	cfg.UpdatedAt = time.Now()

	// Validate
	if err := cfg.Validate(); err != nil {
		return err
	}

	// Store directly (MockConfiguration is now an alias for mock.Mock)
	return mm.store.Set(cfg)
}

// Delete removes a mock by ID.
func (mm *MockManager) Delete(id string) error {
	mm.mu.Lock()
	defer mm.mu.Unlock()
	if !mm.store.Delete(id) {
		return fmt.Errorf("mock with ID %s not found", id)
	}
	return nil
}

// Get retrieves a mock by ID.
func (mm *MockManager) Get(id string) *config.MockConfiguration {
	mm.mu.RLock()
	defer mm.mu.RUnlock()
	return mm.store.Get(id)
}

// List returns all mocks.
func (mm *MockManager) List() []*config.MockConfiguration {
	mm.mu.RLock()
	defer mm.mu.RUnlock()
	return mm.store.List()
}

// ListByType returns mocks of a specific type.
func (mm *MockManager) ListByType(t mock.MockType) []*config.MockConfiguration {
	mm.mu.RLock()
	defer mm.mu.RUnlock()
	return mm.store.ListByType(t)
}

// Store returns the underlying mock store.
func (mm *MockManager) Store() storage.MockStore {
	mm.mu.RLock()
	defer mm.mu.RUnlock()
	return mm.store
}

// SetStore updates the underlying mock store.
func (mm *MockManager) SetStore(store storage.MockStore) {
	mm.mu.Lock()
	defer mm.mu.Unlock()
	mm.store = store
}

// Exists checks if a mock with the given ID exists.
func (mm *MockManager) Exists(id string) bool {
	mm.mu.RLock()
	defer mm.mu.RUnlock()
	return mm.store.Exists(id)
}

// Clear removes all mocks.
func (mm *MockManager) Clear() {
	mm.mu.Lock()
	defer mm.mu.Unlock()
	mm.store.Clear()
}

// Count returns the number of mocks.
func (mm *MockManager) Count() int {
	mm.mu.RLock()
	defer mm.mu.RUnlock()
	return mm.store.Count()
}

// registerHandler registers the appropriate protocol handler for a mock.
// This is the public version that acquires the lock before calling registerHandlerLocked.
func (mm *MockManager) registerHandler(cfg *config.MockConfiguration) {
	mm.mu.Lock()
	defer mm.mu.Unlock()
	mm.registerHandlerLocked(cfg)
}

// registerHandlerLocked registers the appropriate protocol handler for a mock.
// This is called after a mock is stored to wire up WebSocket, GraphQL, etc. handlers.
// MUST be called while holding mm.mu lock.
func (mm *MockManager) registerHandlerLocked(cfg *config.MockConfiguration) {
	if cfg == nil || !cfg.Enabled {
		return
	}

	switch cfg.Type {
	case mock.MockTypeWebSocket:
		if cfg.WebSocket != nil {
			if err := mm.registerWebSocketMock(cfg); err != nil {
				mm.log.Warn("failed to register WebSocket handler", "name", cfg.Name, "error", err)
			}
		}
	case mock.MockTypeGraphQL:
		if cfg.GraphQL != nil {
			if err := mm.registerGraphQLMock(cfg); err != nil {
				mm.log.Warn("failed to register GraphQL handler", "name", cfg.Name, "error", err)
			}
		}
	case mock.MockTypeSOAP:
		if cfg.SOAP != nil {
			if err := mm.registerSOAPMock(cfg); err != nil {
				mm.log.Warn("failed to register SOAP handler", "name", cfg.Name, "error", err)
			}
		}
	case mock.MockTypeMQTT:
		if cfg.MQTT != nil {
			if err := mm.registerMQTTMock(cfg); err != nil {
				mm.log.Warn("failed to register MQTT handler", "name", cfg.Name, "error", err)
			}
		}
	case mock.MockTypeGRPC:
		if cfg.GRPC != nil {
			if err := mm.registerGRPCMock(cfg); err != nil {
				mm.log.Warn("failed to register gRPC handler", "name", cfg.Name, "error", err)
			}
		}
	case mock.MockTypeOAuth:
		if cfg.OAuth != nil {
			if err := mm.registerOAuthMock(cfg); err != nil {
				mm.log.Warn("failed to register OAuth handler", "name", cfg.Name, "error", err)
			}
		}
		// HTTP mocks are handled by the default request handler via store lookup
		// gRPC mocks require explicit port binding via API
	}
}

// registerWebSocketMock registers a WebSocket mock from the unified mock struct.
func (mm *MockManager) registerWebSocketMock(m *mock.Mock) error {
	if m.WebSocket == nil {
		return fmt.Errorf("WebSocket spec is nil")
	}

	// Convert mock.WebSocketSpec to config.WebSocketEndpointConfig
	ws := m.WebSocket

	// Convert []WSMatcherConfig to []*WSMatcherConfig
	var matchers []*mock.WSMatcherConfig
	for i := range ws.Matchers {
		matchers = append(matchers, &ws.Matchers[i])
	}

	cfg := &config.WebSocketEndpointConfig{
		ID:                 m.ID,
		Name:               m.Name,
		Path:               ws.Path,
		Subprotocols:       ws.Subprotocols,
		RequireSubprotocol: ws.RequireSubprotocol,
		Matchers:           matchers,
		DefaultResponse:    ws.DefaultResponse,
		Scenario:           ws.Scenario,
		Heartbeat:          ws.Heartbeat,
		MaxMessageSize:     ws.MaxMessageSize,
		IdleTimeout:        ws.IdleTimeout,
		MaxConnections:     ws.MaxConnections,
		EchoMode:           ws.EchoMode,
	}

	return mm.handler.RegisterWebSocketEndpoint(cfg)
}

// registerGraphQLMock registers a GraphQL mock from the unified mock struct.
func (mm *MockManager) registerGraphQLMock(m *mock.Mock) error {
	if m.GraphQL == nil {
		return fmt.Errorf("GraphQL spec is nil")
	}

	gqlSpec := m.GraphQL

	// Convert mock.GraphQLSpec to graphql.GraphQLConfig
	cfg := &graphql.GraphQLConfig{
		ID:            m.ID,
		Name:          m.Name,
		ParentID:      m.ParentID,
		MetaSortKey:   m.MetaSortKey,
		Path:          gqlSpec.Path,
		Schema:        gqlSpec.Schema,
		SchemaFile:    gqlSpec.SchemaFile,
		Introspection: gqlSpec.Introspection,
		Enabled:       m.Enabled,
	}

	// Convert resolvers
	if gqlSpec.Resolvers != nil {
		cfg.Resolvers = make(map[string]graphql.ResolverConfig)
		for path, resolver := range gqlSpec.Resolvers {
			cfg.Resolvers[path] = graphql.ResolverConfig{
				Response: resolver.Response,
				Delay:    resolver.Delay,
			}
			// Note: Match and Error fields would need conversion if used
		}
	}

	// Parse schema and create executor
	var schema *graphql.Schema
	var err error
	if cfg.Schema != "" {
		schema, err = graphql.ParseSchema(cfg.Schema)
	} else if cfg.SchemaFile != "" {
		schema, err = graphql.ParseSchemaFile(cfg.SchemaFile)
	} else {
		return fmt.Errorf("GraphQL mock requires schema or schemaFile")
	}
	if err != nil {
		return fmt.Errorf("failed to parse GraphQL schema: %w", err)
	}

	// Create executor and handler
	executor := graphql.NewExecutor(schema, cfg)
	handler := graphql.NewHandler(executor, cfg)

	// Register with the HTTP handler
	mm.handler.RegisterGraphQLHandler(cfg.Path, handler)

	mm.log.Info("registered GraphQL handler", "path", cfg.Path, "name", cfg.Name)
	return nil
}

// registerSOAPMock registers a SOAP mock from the unified mock struct.
func (mm *MockManager) registerSOAPMock(m *mock.Mock) error {
	if m.SOAP == nil {
		return fmt.Errorf("SOAP spec is nil")
	}

	soapSpec := m.SOAP

	// Convert mock.SOAPSpec to soap.SOAPConfig
	cfg := &soap.SOAPConfig{
		ID:       m.ID,
		Name:     m.Name,
		Path:     soapSpec.Path,
		WSDL:     soapSpec.WSDL,
		WSDLFile: soapSpec.WSDLFile,
		Enabled:  m.Enabled,
	}

	// Convert operations
	if soapSpec.Operations != nil {
		cfg.Operations = make(map[string]soap.OperationConfig)
		for name, op := range soapSpec.Operations {
			soapOp := soap.OperationConfig{
				SOAPAction: op.SOAPAction,
				Response:   op.Response,
				Delay:      op.Delay,
			}
			// Convert fault if present
			if op.Fault != nil {
				soapOp.Fault = &soap.SOAPFault{
					Code:    op.Fault.Code,
					Message: op.Fault.Message,
					Detail:  op.Fault.Detail,
				}
			}
			// Convert match if present
			if op.Match != nil {
				soapOp.Match = &soap.SOAPMatch{
					XPath: op.Match.XPath,
				}
			}
			cfg.Operations[name] = soapOp
		}
	}

	// Create handler and register
	handler := soap.NewHandler(cfg)
	mm.handler.RegisterSOAPHandler(cfg.Path, handler)

	mm.log.Info("registered SOAP handler", "path", cfg.Path, "name", cfg.Name)
	return nil
}

// registerMQTTMock registers an MQTT mock and starts the broker.
func (mm *MockManager) registerMQTTMock(m *mock.Mock) error {
	if m.MQTT == nil {
		return fmt.Errorf("MQTT spec is nil")
	}

	if mm.protocolManager == nil {
		mm.log.Warn("MQTT mock stored but broker not started (no protocol manager)",
			"name", m.Name, "port", m.MQTT.Port)
		return nil
	}

	mqttSpec := m.MQTT

	// Convert mock.MQTTSpec to mqtt.MQTTConfig
	cfg := &mqtt.MQTTConfig{
		ID:      m.ID,
		Name:    m.Name,
		Port:    mqttSpec.Port,
		Enabled: m.Enabled,
	}

	// Convert TLS config
	if mqttSpec.TLS != nil {
		cfg.TLS = &mqtt.MQTTTLSConfig{
			Enabled:  mqttSpec.TLS.Enabled,
			CertFile: mqttSpec.TLS.CertFile,
			KeyFile:  mqttSpec.TLS.KeyFile,
		}
	}

	// Convert Auth config
	if mqttSpec.Auth != nil {
		cfg.Auth = &mqtt.MQTTAuthConfig{
			Enabled: mqttSpec.Auth.Enabled,
		}
		for _, user := range mqttSpec.Auth.Users {
			mqttUser := mqtt.MQTTUser{
				Username: user.Username,
				Password: user.Password,
			}
			for _, rule := range user.ACL {
				mqttUser.ACL = append(mqttUser.ACL, mqtt.ACLRule{
					Topic:  rule.Topic,
					Access: rule.Access,
				})
			}
			cfg.Auth.Users = append(cfg.Auth.Users, mqttUser)
		}
	}

	// Convert Topics
	for _, topic := range mqttSpec.Topics {
		mqttTopic := mqtt.TopicConfig{
			Topic:  topic.Topic,
			QoS:    topic.QoS,
			Retain: topic.Retain,
		}
		// Convert Messages
		for _, msg := range topic.Messages {
			mqttTopic.Messages = append(mqttTopic.Messages, mqtt.MessageConfig{
				Payload:  msg.Payload,
				Delay:    msg.Delay,
				Repeat:   msg.Repeat,
				Interval: msg.Interval,
			})
		}
		// Convert OnPublish handler
		if topic.OnPublish != nil {
			mqttTopic.OnPublish = &mqtt.PublishHandler{
				Forward: topic.OnPublish.Forward,
			}
			if topic.OnPublish.Response != nil {
				mqttTopic.OnPublish.Response = &mqtt.MessageConfig{
					Payload:  topic.OnPublish.Response.Payload,
					Delay:    topic.OnPublish.Response.Delay,
					Repeat:   topic.OnPublish.Response.Repeat,
					Interval: topic.OnPublish.Response.Interval,
				}
			}
		}
		// Convert DeviceSimulation settings
		if topic.DeviceSimulation != nil {
			mqttTopic.DeviceSimulation = &mqtt.DeviceSimulationSettings{
				Enabled:         topic.DeviceSimulation.Enabled,
				DeviceCount:     topic.DeviceSimulation.DeviceCount,
				DeviceIDPattern: topic.DeviceSimulation.DeviceIDPattern,
			}
		}
		cfg.Topics = append(cfg.Topics, mqttTopic)
	}

	// Start the broker via ProtocolManager
	broker, err := mm.protocolManager.StartMQTTBroker(cfg)
	if err != nil {
		return fmt.Errorf("failed to start MQTT broker: %w", err)
	}

	mm.log.Info("started MQTT broker", "name", m.Name, "port", broker.Port())
	return nil
}

// registerGRPCMock registers a gRPC mock and starts the server.
func (mm *MockManager) registerGRPCMock(m *mock.Mock) error {
	if m.GRPC == nil {
		return fmt.Errorf("gRPC spec is nil")
	}

	if mm.protocolManager == nil {
		mm.log.Warn("gRPC mock stored but server not started (no protocol manager)",
			"name", m.Name, "port", m.GRPC.Port)
		return nil
	}

	grpcSpec := m.GRPC

	// gRPC requires proto files to parse the schema
	if grpcSpec.ProtoFile == "" && len(grpcSpec.ProtoFiles) == 0 {
		mm.log.Warn("gRPC mock has no proto file, cannot start server",
			"name", m.Name, "port", grpcSpec.Port)
		return nil
	}

	// Convert mock.GRPCSpec to grpc.GRPCConfig
	cfg := &grpc.GRPCConfig{
		ID:          m.ID,
		Name:        m.Name,
		Port:        grpcSpec.Port,
		ProtoFile:   grpcSpec.ProtoFile,
		ProtoFiles:  grpcSpec.ProtoFiles,
		ImportPaths: grpcSpec.ImportPaths,
		Reflection:  grpcSpec.Reflection,
		Enabled:     m.Enabled,
	}

	// Convert Services
	if grpcSpec.Services != nil {
		cfg.Services = make(map[string]grpc.ServiceConfig)
		for svcName, svc := range grpcSpec.Services {
			grpcSvc := grpc.ServiceConfig{
				Methods: make(map[string]grpc.MethodConfig),
			}
			for methodName, method := range svc.Methods {
				grpcMethod := grpc.MethodConfig{
					Response:    method.Response,
					Delay:       method.Delay,
					StreamDelay: method.StreamDelay,
				}
				// Convert responses slice
				for _, resp := range method.Responses {
					grpcMethod.Responses = append(grpcMethod.Responses, resp)
				}
				// Convert error config
				if method.Error != nil {
					grpcMethod.Error = &grpc.GRPCErrorConfig{
						Code:    method.Error.Code,
						Message: method.Error.Message,
						Details: method.Error.Details,
					}
				}
				// Convert match config
				if method.Match != nil {
					grpcMethod.Match = &grpc.MethodMatch{
						Metadata: method.Match.Metadata,
						Request:  method.Match.Request,
					}
				}
				grpcSvc.Methods[methodName] = grpcMethod
			}
			cfg.Services[svcName] = grpcSvc
		}
	}

	// Start the server via ProtocolManager
	server, err := mm.protocolManager.StartGRPCServer(cfg)
	if err != nil {
		return fmt.Errorf("failed to start gRPC server: %w", err)
	}

	mm.log.Info("started gRPC server", "name", m.Name, "port", server.Port())
	return nil
}

// registerOAuthMock registers an OAuth/OIDC mock provider.
func (mm *MockManager) registerOAuthMock(m *mock.Mock) error {
	if m.OAuth == nil {
		return fmt.Errorf("OAuth spec is nil")
	}

	if mm.handler == nil {
		mm.log.Warn("OAuth mock stored but handler not available",
			"name", m.Name, "issuer", m.OAuth.Issuer)
		return nil
	}

	oauthSpec := m.OAuth

	// Convert mock.OAuthSpec to oauth.OAuthConfig
	cfg := &oauth.OAuthConfig{
		ID:            m.ID,
		Issuer:        oauthSpec.Issuer,
		TokenExpiry:   oauthSpec.TokenExpiry,
		RefreshExpiry: oauthSpec.RefreshExpiry,
		DefaultScopes: oauthSpec.DefaultScopes,
		Enabled:       m.Enabled,
	}

	// Set defaults if not specified
	if cfg.TokenExpiry == "" {
		cfg.TokenExpiry = "1h"
	}
	if cfg.RefreshExpiry == "" {
		cfg.RefreshExpiry = "7d"
	}
	if len(cfg.DefaultScopes) == 0 {
		cfg.DefaultScopes = []string{"openid", "profile", "email"}
	}

	// Convert clients
	for _, client := range oauthSpec.Clients {
		cfg.Clients = append(cfg.Clients, oauth.ClientConfig{
			ClientID:     client.ClientID,
			ClientSecret: client.ClientSecret,
			RedirectURIs: client.RedirectURIs,
			GrantTypes:   client.GrantTypes,
		})
	}

	// Convert users
	for _, user := range oauthSpec.Users {
		claims := make(map[string]interface{})
		for k, v := range user.Claims {
			claims[k] = v
		}
		cfg.Users = append(cfg.Users, oauth.UserConfig{
			Username: user.Username,
			Password: user.Password,
			Claims:   claims,
		})
	}

	// Create OAuth provider and handler
	provider, err := oauth.NewProvider(cfg)
	if err != nil {
		return fmt.Errorf("failed to create OAuth provider: %w", err)
	}

	handler := oauth.NewHandler(provider)
	mm.handler.RegisterOAuthHandler(cfg, handler)

	mm.log.Info("registered OAuth provider", "name", m.Name, "issuer", oauthSpec.Issuer)
	return nil
}
