// Package engine provides the core mock server engine.
package engine

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/getmockd/mockd/pkg/config"
	"github.com/getmockd/mockd/pkg/graphql"
	"github.com/getmockd/mockd/pkg/logging"
	"github.com/getmockd/mockd/pkg/mock"
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
// It loads mocks from the unified mock store.
func (cl *ConfigLoader) LoadFromStore(ctx context.Context, persistentStore store.Store) error {
	if persistentStore == nil {
		return nil // No store configured, nothing to load
	}

	// Load all mocks from the unified store
	mocks, err := persistentStore.Mocks().List(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to load mocks: %w", err)
	}

	// Register mocks by type using mockManager
	for _, m := range mocks {
		if !m.Enabled {
			continue
		}
		switch m.Type {
		case mock.MockTypeHTTP:
			// HTTP mocks are handled automatically via PersistentMockStore adapter
			// They're matched in the request handler via store.List()
		case mock.MockTypeWebSocket:
			if m.WebSocket != nil {
				if err := cl.server.mockManager.registerWebSocketMock(m); err != nil {
					cl.log.Warn("failed to load WebSocket mock", "name", m.Name, "error", err)
				}
			}
		case mock.MockTypeGraphQL:
			if m.GraphQL != nil {
				if err := cl.server.mockManager.registerGraphQLMock(m); err != nil {
					cl.log.Warn("failed to load GraphQL mock", "name", m.Name, "error", err)
				}
			}
		case mock.MockTypeGRPC:
			// gRPC servers are port-based and require explicit startup via API
		case mock.MockTypeSOAP:
			if m.SOAP != nil {
				if err := cl.server.mockManager.registerSOAPMock(m); err != nil {
					cl.log.Warn("failed to load SOAP mock", "name", m.Name, "error", err)
				}
			}
		case mock.MockTypeMQTT:
			if m.MQTT != nil {
				if err := cl.server.mockManager.registerMQTTMock(m); err != nil {
					cl.log.Warn("failed to load MQTT mock", "name", m.Name, "error", err)
				}
			}
		case mock.MockTypeOAuth:
			if m.OAuth != nil {
				if err := cl.server.mockManager.registerOAuthMock(m); err != nil {
					cl.log.Warn("failed to load OAuth mock", "name", m.Name, "error", err)
				}
			}
		}
	}

	return nil
}

// LoadFromFile loads mock configurations from a file and adds them to the server.
// If replace is true, existing mocks are cleared first.
func (cl *ConfigLoader) LoadFromFile(path string, replace bool) error {
	collection, err := config.LoadFromFile(path)
	if err != nil {
		return err
	}

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
	}

	// Load regular mocks
	for _, m := range collection.Mocks {
		if m != nil {
			// Check for duplicate before attempting to add
			if !replace && store.Exists(m.ID) {
				fmt.Fprintf(os.Stderr, "Warning: skipping mock with duplicate ID: %s\n", m.ID)
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

	// Load WebSocket endpoints
	for _, ws := range collection.WebSocketEndpoints {
		if ws != nil {
			if err := cl.server.registerWebSocketEndpoint(ws); err != nil {
				return fmt.Errorf("failed to register WebSocket endpoint %s: %w", ws.Path, err)
			}
		}
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
		}
	}

	return nil
}

// SaveToFile saves the current mock configurations to a file.
func (cl *ConfigLoader) SaveToFile(path string, name string) error {
	mocks := cl.server.Store().ListByType(mock.MockTypeHTTP)
	return config.SaveMocksToFile(path, mocks, name)
}

// Export exports the current configuration as a MockCollection.
func (cl *ConfigLoader) Export(name string) *config.MockCollection {
	mocks := cl.server.Store().ListByType(mock.MockTypeHTTP)
	return &config.MockCollection{
		Version: "1.0",
		Name:    name,
		Mocks:   mocks,
	}
}

// Import imports a MockCollection, optionally replacing existing mocks.
func (cl *ConfigLoader) Import(collection *config.MockCollection, replace bool) error {
	if collection == nil {
		return fmt.Errorf("collection cannot be nil")
	}

	if err := collection.Validate(); err != nil {
		return err
	}

	store := cl.server.Store()
	if replace {
		store.Clear()
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
					continue
				}
				return fmt.Errorf("failed to register stateful resource %s: %w", res.Name, err)
			}
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

// registerStatefulResource registers a stateful resource from config.
func (cl *ConfigLoader) registerStatefulResource(cfg *config.StatefulResourceConfig) error {
	return cl.server.statefulStore.Register(&stateful.ResourceConfig{
		Name:        cfg.Name,
		BasePath:    cfg.BasePath,
		IDField:     cfg.IDField,
		ParentField: cfg.ParentField,
		SeedData:    cfg.SeedData,
	})
}
