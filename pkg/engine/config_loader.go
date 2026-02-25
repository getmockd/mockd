// Package engine provides the core mock server engine.
package engine

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/getmockd/mockd/pkg/config"
	"github.com/getmockd/mockd/pkg/graphql"
	"github.com/getmockd/mockd/pkg/logging"
	"github.com/getmockd/mockd/pkg/stateful"
	"github.com/getmockd/mockd/pkg/store"
)

// ConfigLoader handles loading and saving mock configurations.
type ConfigLoader struct {
	server *Server
	log    *slog.Logger
}

// NewConfigLoader creates a new config loader.
func NewConfigLoader(server *Server) *ConfigLoader {
	return &ConfigLoader{
		server: server,
		log:    logging.Nop(),
	}
}

// SetLogger sets the logger.
func (cl *ConfigLoader) SetLogger(log *slog.Logger) {
	if log != nil {
		cl.log = log
	}
}

// LoadFromStore loads all configurations from the persistent store into the engine.
// This should be called after setting the store and before starting the server.
// It loads mocks from the unified mock store and starts all protocol handlers,
// and also restores persisted stateful resource definitions.
func (cl *ConfigLoader) LoadFromStore(ctx context.Context, persistentStore store.Store) error {
	if persistentStore == nil {
		return nil // No store configured, nothing to load
	}

	// Load all mocks from the unified store
	mocks, err := persistentStore.Mocks().List(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to load mocks: %w", err)
	}

	// Register all enabled mocks using the unified registerHandler which handles
	// all protocol types (HTTP, WebSocket, GraphQL, gRPC, MQTT, SOAP, OAuth, SSE).
	// HTTP mocks are also matched via store lookup, but registerHandler is a no-op for them.
	for _, m := range mocks {
		if m.Enabled != nil && !*m.Enabled {
			continue
		}
		// registerHandler logs warnings for failures but doesn't return errors
		// to allow loading to continue for other mocks
		cl.server.mockManager.registerHandler(m)
	}

	// Load persisted stateful resource definitions.
	// These are registered with the engine's stateful store so their CRUD
	// endpoints become available immediately (seeded with initial data).
	resources, err := persistentStore.StatefulResources().List(ctx)
	if err != nil {
		cl.log.Warn("failed to load persisted stateful resources", "error", err)
	} else {
		for _, res := range resources {
			if res == nil {
				continue
			}
			if err := cl.registerStatefulResource(res); err != nil {
				cl.log.Warn("failed to restore stateful resource",
					"name", res.Name, "error", err)
			}
		}
		if len(resources) > 0 {
			cl.log.Info("restored persisted stateful resources", "count", len(resources))
		}
	}

	return nil
}

// LoadFromFile loads mock configurations from a file and adds them to the server.
// If replace is true, existing mocks are cleared first.
// The config file's directory is set as the handler's baseDir so that relative
// bodyFile paths resolve correctly.
func (cl *ConfigLoader) LoadFromFile(path string, replace bool) error {
	collection, err := config.LoadFromFile(path)
	if err != nil {
		return err
	}

	// Set the handler's base directory to the config file's directory so that
	// relative bodyFile paths are resolved relative to the config file location.
	baseDir := config.GetMockFileBaseDir(path)
	cl.server.handler.SetBaseDir(baseDir)

	return cl.loadCollection(collection, replace)
}

// LoadFromBytes loads mock configurations from JSON bytes and adds them to the server.
// If replace is true, existing mocks are cleared first.
func (cl *ConfigLoader) LoadFromBytes(data []byte, replace bool) error {
	collection, err := config.ParseJSON(data)
	if err != nil {
		return err
	}

	return cl.loadCollection(collection, replace)
}

// loadCollection is a helper that loads a collection into the server.
func (cl *ConfigLoader) loadCollection(collection *config.MockCollection, replace bool) error {
	store := cl.server.Store()
	if replace {
		store.Clear()
		if cl.server.statefulBridge != nil {
			cl.server.statefulBridge.ClearCustomOperations()
		}
	}

	// Load regular mocks
	for _, m := range collection.Mocks {
		if m != nil {
			// Check for duplicate before attempting to add
			if !replace && store.Exists(m.ID) {
				cl.log.Warn("skipping mock with duplicate ID", "id", m.ID)
				continue
			}
			if err := cl.server.addMock(m); err != nil {
				return err
			}
		}
	}

	// Load stateful resources
	for _, res := range collection.StatefulResources {
		if res != nil {
			if err := cl.server.registerStatefulResource(res); err != nil {
				return fmt.Errorf("failed to register stateful resource %s: %w", res.Name, err)
			}
		}
	}

	// Load custom operations and register them on the stateful bridge.
	// Custom operations compose reads, writes, and expression-evaluated transforms
	// against stateful resources (e.g., TransferFunds).
	if cl.server.statefulBridge != nil {
		for _, opCfg := range collection.CustomOperations {
			if opCfg == nil {
				continue
			}
			customOp, err := convertCustomOperation(opCfg)
			if err != nil {
				return fmt.Errorf("invalid custom operation %s: %w", opCfg.Name, err)
			}
			cl.server.statefulBridge.RegisterCustomOperation(opCfg.Name, customOp)
			cl.log.Info("registered custom operation", "name", opCfg.Name, "steps", len(opCfg.Steps))
		}
	}

	// Load WebSocket endpoints
	for _, ws := range collection.WebSocketEndpoints {
		if ws != nil {
			if err := cl.server.registerWebSocketEndpoint(ws); err != nil {
				return fmt.Errorf("failed to register WebSocket endpoint %s: %w", ws.Path, err)
			}
		}
	}

	// Merge collection's ServerConfig into the running server configuration.
	// This allows YAML config files to set server-level settings (rate limiting,
	// CORS, etc.) that are otherwise only configurable via CLI flags.
	// CLI flags take precedence: only fields not already set by flags are merged.
	if collection.ServerConfig != nil {
		cl.mergeServerConfig(collection.ServerConfig)
	}

	// Load GraphQL endpoints from collection's ServerConfig
	if collection.ServerConfig != nil {
		for _, gqlCfg := range collection.ServerConfig.GraphQL {
			if gqlCfg == nil || !gqlCfg.Enabled {
				continue
			}

			// Create GraphQL handler
			gqlHandler, err := graphql.Endpoint(gqlCfg)
			if err != nil {
				return fmt.Errorf("failed to create GraphQL endpoint %s: %w", gqlCfg.Path, err)
			}

			cl.server.protocolManager.AddGraphQLHandler(gqlHandler)

			// Register the handler at the configured path
			cl.server.handler.RegisterGraphQLHandler(gqlCfg.Path, gqlHandler)

			// Create subscription handler if subscriptions are configured
			if len(gqlCfg.Subscriptions) > 0 {
				// Parse schema for subscription handler
				var schema *graphql.Schema
				var schemaErr error
				if gqlCfg.Schema != "" {
					schema, schemaErr = graphql.ParseSchema(gqlCfg.Schema)
				} else if gqlCfg.SchemaFile != "" {
					schema, schemaErr = graphql.ParseSchemaFile(gqlCfg.SchemaFile)
				}
				if schemaErr != nil {
					return fmt.Errorf("failed to parse GraphQL schema for subscriptions: %w", schemaErr)
				}

				subHandler := graphql.NewSubscriptionHandler(schema, gqlCfg)
				cl.server.protocolManager.AddGraphQLSubscriptionHandler(subHandler)

				// Register subscription handler at path/ws (or path + ws when path ends with slash).
				wsPath, pathErr := buildGraphQLSubscriptionPath(gqlCfg.Path)
				if pathErr != nil {
					return fmt.Errorf("invalid GraphQL path for subscriptions: %w", pathErr)
				}
				cl.server.handler.RegisterGraphQLSubscriptionHandler(wsPath, subHandler)
			}
		}
	}

	return nil
}

// SaveToFile saves the current mock configurations to a file.
// All protocol types (HTTP, WebSocket, GraphQL, gRPC, MQTT, SOAP, OAuth)
// are included in the export.
func (cl *ConfigLoader) SaveToFile(path string, name string) error {
	mocks := cl.server.Store().List()
	return config.SaveMocksToFile(path, mocks, name)
}

// Export exports the current configuration as a MockCollection.
// All protocol types (HTTP, WebSocket, GraphQL, gRPC, MQTT, SOAP, OAuth)
// are included in the export, along with stateful resources and custom operations.
func (cl *ConfigLoader) Export(name string) *config.MockCollection {
	mocks := cl.server.Store().List()
	collection := &config.MockCollection{
		Version: "1.0",
		Name:    name,
		Mocks:   mocks,
	}

	// Include stateful resource definitions
	if cl.server.statefulStore != nil {
		resources := cl.server.statefulStore.ListConfigs()
		if len(resources) > 0 {
			collection.StatefulResources = resources
		}
	}

	// Include custom operation definitions
	if cl.server.statefulBridge != nil {
		customOps := cl.server.statefulBridge.ListCustomOperations()
		if len(customOps) > 0 {
			configs := make([]*config.CustomOperationConfig, 0, len(customOps))
			for name, op := range customOps {
				cfg := &config.CustomOperationConfig{
					Name:        name,
					Consistency: string(op.Consistency),
					Response:    op.Response,
				}
				for _, s := range op.Steps {
					cfg.Steps = append(cfg.Steps, config.CustomStepConfig{
						Type:     string(s.Type),
						Resource: s.Resource,
						ID:       s.ID,
						As:       s.As,
						Set:      s.Set,
						Var:      s.Var,
						Value:    s.Value,
					})
				}
				configs = append(configs, cfg)
			}
			collection.CustomOperations = configs
		}
	}

	return collection
}

// Import imports a MockCollection, optionally replacing existing mocks.
func (cl *ConfigLoader) Import(collection *config.MockCollection, replace bool) error {
	if collection == nil {
		return errors.New("collection cannot be nil")
	}

	if err := collection.Validate(); err != nil {
		return err
	}

	store := cl.server.Store()
	if replace {
		store.Clear()
		if cl.server.statefulBridge != nil {
			cl.server.statefulBridge.ClearCustomOperations()
		}
	}

	for _, cfg := range collection.Mocks {
		if cfg != nil {
			// Skip if not replacing and mock already exists
			if !replace && store.Exists(cfg.ID) {
				continue
			}
			// Store directly (MockConfiguration is now an alias for mock.Mock)
			if err := store.Set(cfg); err != nil {
				return err
			}
			// Register protocol-specific handlers (MQTT brokers, WebSocket, GraphQL, etc.)
			cl.server.mockManager.registerHandler(cfg)
		}
	}

	// Import stateful resources
	for _, res := range collection.StatefulResources {
		if res != nil {
			if err := cl.registerStatefulResource(res); err != nil {
				// Skip if resource already exists (similar to mocks)
				if !replace {
					cl.log.Warn("skipping stateful resource (already registered)",
						"name", res.Name, "error", err)
					continue
				}
				return fmt.Errorf("failed to register stateful resource %s: %w", res.Name, err)
			}
		}
	}

	// Import custom operations
	if cl.server.statefulBridge != nil {
		for _, opCfg := range collection.CustomOperations {
			if opCfg == nil {
				continue
			}
			customOp, err := convertCustomOperation(opCfg)
			if err != nil {
				return fmt.Errorf("invalid custom operation %s: %w", opCfg.Name, err)
			}
			cl.server.statefulBridge.RegisterCustomOperation(opCfg.Name, customOp)
		}
	}

	// Import WebSocket endpoints
	for _, ws := range collection.WebSocketEndpoints {
		if ws != nil {
			if err := cl.server.registerWebSocketEndpoint(ws); err != nil {
				return fmt.Errorf("failed to register WebSocket endpoint %s: %w", ws.Path, err)
			}
		}
	}

	return nil
}

// mergeServerConfig merges a collection's ServerConfig into the running server
// configuration. CLI flags take precedence — only fields not already set by
// flags are merged from the YAML/JSON config file.
func (cl *ConfigLoader) mergeServerConfig(src *config.ServerConfiguration) {
	dst := cl.server.cfg

	// Rate limiting: merge if not already configured via CLI flags
	if dst.RateLimit == nil && src.RateLimit != nil {
		dst.RateLimit = src.RateLimit
		cl.log.Info("rate limiting configured from config file",
			"enabled", src.RateLimit.Enabled,
			"rps", src.RateLimit.RequestsPerSecond,
			"burst", src.RateLimit.BurstSize)
	}

	// CORS: merge if not already configured
	if dst.CORS == nil && src.CORS != nil {
		dst.CORS = src.CORS
		cl.log.Info("CORS configured from config file")
	}

	// TLS: merge if not already configured via CLI flags
	if dst.TLS == nil && src.TLS != nil {
		dst.TLS = src.TLS
		cl.log.Info("TLS configured from config file")
	}

	// mTLS: merge if not already configured
	if dst.MTLS == nil && src.MTLS != nil {
		dst.MTLS = src.MTLS
		cl.log.Info("mTLS configured from config file")
	}

	// OAuth: merge if not already configured via CLI flags
	if len(dst.OAuth) == 0 && len(src.OAuth) > 0 {
		dst.OAuth = src.OAuth
		cl.log.Info("OAuth configured from config file", "count", len(src.OAuth))
	}
}

// registerStatefulResource registers a stateful resource from config.
func (cl *ConfigLoader) registerStatefulResource(cfg *config.StatefulResourceConfig) error {
	return cl.server.statefulStore.Register(cfg)
}

// convertCustomOperation converts a config.CustomOperationConfig to a stateful.CustomOperation.
func convertCustomOperation(cfg *config.CustomOperationConfig) (*stateful.CustomOperation, error) {
	steps := make([]stateful.Step, 0, len(cfg.Steps))
	for _, s := range cfg.Steps {
		steps = append(steps, stateful.Step{
			Type:     stateful.StepType(s.Type),
			Resource: s.Resource,
			ID:       s.ID,
			As:       s.As,
			Set:      s.Set,
			Var:      s.Var,
			Value:    s.Value,
		})
	}
	op := &stateful.CustomOperation{
		Name:        cfg.Name,
		Consistency: stateful.ConsistencyMode(cfg.Consistency),
		Steps:       steps,
		Response:    cfg.Response,
	}
	if _, err := stateful.NormalizeCustomOperation(op); err != nil {
		return nil, err
	}
	return op, nil
}
