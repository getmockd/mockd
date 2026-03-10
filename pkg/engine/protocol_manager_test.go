package engine

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"syscall"
	"testing"
	"time"

	"github.com/getmockd/mockd/pkg/graphql"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ============================================================================
// NewProtocolManager
// ============================================================================

func TestProtocolManager_New(t *testing.T) {
	t.Parallel()

	pm := NewProtocolManager()
	require.NotNil(t, pm, "NewProtocolManager should return non-nil")
	assert.NotNil(t, pm.registry, "registry should be initialised")
	assert.NotNil(t, pm.log, "logger should default to nop")
}

// ============================================================================
// Setters
// ============================================================================

func TestProtocolManager_SetLogger(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		logger *slog.Logger
	}{
		{name: "non-nil logger", logger: slog.New(slog.NewTextHandler(io.Discard, nil))},
		{name: "nil logger falls back to nop", logger: nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			pm := NewProtocolManager()
			// Should not panic.
			pm.SetLogger(tt.logger)
			assert.NotNil(t, pm.log, "log should never be nil after SetLogger")
		})
	}
}

func TestProtocolManager_SetRequestLogger(t *testing.T) {
	t.Parallel()

	pm := NewProtocolManager()
	rl := NewInMemoryRequestLogger(10)
	// Should not panic.
	pm.SetRequestLogger(rl)
	assert.Equal(t, rl, pm.requestLogger)
}

// ============================================================================
// Registry
// ============================================================================

func TestProtocolManager_Registry(t *testing.T) {
	t.Parallel()

	pm := NewProtocolManager()
	reg := pm.Registry()
	require.NotNil(t, reg, "Registry() must return non-nil protocol registry")
}

// ============================================================================
// GraphQL accessors
// ============================================================================

func TestProtocolManager_GraphQLHandlers_EmptyByDefault(t *testing.T) {
	t.Parallel()

	pm := NewProtocolManager()
	handlers := pm.GraphQLHandlers()
	assert.NotNil(t, handlers, "GraphQLHandlers should return non-nil slice")
	assert.Empty(t, handlers, "GraphQLHandlers should be empty on new manager")
}

func TestProtocolManager_AddGraphQLHandler(t *testing.T) {
	t.Parallel()

	pm := NewProtocolManager()

	// Create a minimal GraphQL handler via the Endpoint helper.
	cfg := &graphql.GraphQLConfig{
		ID:      "test-gql",
		Path:    "/graphql",
		Enabled: true,
		Schema:  `type Query { hello: String }`,
	}
	handler, err := graphql.Endpoint(cfg)
	require.NoError(t, err)

	pm.AddGraphQLHandler(handler)

	handlers := pm.GraphQLHandlers()
	require.Len(t, handlers, 1, "should have 1 handler after add")
	assert.Equal(t, handler, handlers[0])
}

func TestProtocolManager_AddGraphQLSubscriptionHandler(t *testing.T) {
	t.Parallel()

	pm := NewProtocolManager()

	cfg := &graphql.GraphQLConfig{
		ID:      "test-sub",
		Path:    "/graphql",
		Enabled: true,
	}
	subHandler := graphql.NewSubscriptionHandler(nil, cfg)
	require.NotNil(t, subHandler)

	pm.AddGraphQLSubscriptionHandler(subHandler)

	// Verify via internal field (white-box).
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	require.Len(t, pm.graphqlSubHandlers, 1)
	assert.Equal(t, subHandler, pm.graphqlSubHandlers[0])
}

// ============================================================================
// OAuth accessors
// ============================================================================

func TestProtocolManager_OAuthProviders_EmptyByDefault(t *testing.T) {
	t.Parallel()

	pm := NewProtocolManager()
	providers := pm.OAuthProviders()
	assert.NotNil(t, providers)
	assert.Empty(t, providers)
}

func TestProtocolManager_OAuthHandlers_EmptyByDefault(t *testing.T) {
	t.Parallel()

	pm := NewProtocolManager()
	handlers := pm.OAuthHandlers()
	assert.NotNil(t, handlers)
	assert.Empty(t, handlers)
}

// ============================================================================
// SOAP accessors
// ============================================================================

func TestProtocolManager_SOAPHandlers_EmptyByDefault(t *testing.T) {
	t.Parallel()

	pm := NewProtocolManager()
	handlers := pm.SOAPHandlers()
	assert.NotNil(t, handlers)
	assert.Empty(t, handlers)
}

// ============================================================================
// gRPC accessors
// ============================================================================

func TestProtocolManager_GRPCServers_EmptyByDefault(t *testing.T) {
	t.Parallel()

	pm := NewProtocolManager()
	servers := pm.GRPCServers()
	assert.NotNil(t, servers)
	assert.Empty(t, servers)
}

func TestProtocolManager_GetGRPCServer_NonExistent(t *testing.T) {
	t.Parallel()

	pm := NewProtocolManager()
	srv := pm.GetGRPCServer("does-not-exist")
	assert.Nil(t, srv, "GetGRPCServer should return nil for unknown ID")
}

func TestProtocolManager_StopGRPCServer_NonExistent(t *testing.T) {
	t.Parallel()

	pm := NewProtocolManager()
	err := pm.StopGRPCServer("no-such-server")
	// Implementation returns nil when server is not found.
	assert.NoError(t, err, "StopGRPCServer should not error for non-existent ID")
}

// ============================================================================
// MQTT accessors
// ============================================================================

func TestProtocolManager_GetMQTTBrokers_EmptyByDefault(t *testing.T) {
	t.Parallel()

	pm := NewProtocolManager()
	brokers := pm.GetMQTTBrokers()
	assert.NotNil(t, brokers)
	assert.Empty(t, brokers)
}

func TestProtocolManager_GetMQTTBroker_NonExistent(t *testing.T) {
	t.Parallel()

	pm := NewProtocolManager()
	broker := pm.GetMQTTBroker("no-such-broker")
	assert.Nil(t, broker, "GetMQTTBroker should return nil for unknown ID")
}

func TestProtocolManager_MQTTPorts_EmptyByDefault(t *testing.T) {
	t.Parallel()

	pm := NewProtocolManager()
	ports := pm.MQTTPorts()
	assert.Empty(t, ports, "MQTTPorts should be empty on new manager")
}

func TestProtocolManager_StopMQTTBroker_NonExistent(t *testing.T) {
	t.Parallel()

	pm := NewProtocolManager()
	err := pm.StopMQTTBroker("no-such-broker")
	// Implementation returns nil when broker is not found.
	assert.NoError(t, err, "StopMQTTBroker should not error for non-existent ID")
}

// ============================================================================
// StopAll — empty manager
// ============================================================================

func TestProtocolManager_StopAll_EmptyManager(t *testing.T) {
	t.Parallel()

	pm := NewProtocolManager()
	ctx := context.Background()
	errs := pm.StopAll(ctx, 5*time.Second)
	assert.Empty(t, errs, "StopAll on empty manager should return no errors")
}

// ============================================================================
// StartAll — empty ProtocolConfig
// ============================================================================

func TestProtocolManager_StartAll_EmptyConfig(t *testing.T) {
	t.Parallel()

	pm := NewProtocolManager()
	cfg := &ProtocolConfig{} // all slices nil
	ctx := context.Background()

	err := pm.StartAll(ctx, cfg, nil)
	assert.NoError(t, err, "StartAll with empty config should succeed")
}

// ============================================================================
// isAddrInUse
// ============================================================================

func TestProtocolManager_IsAddrInUse(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "EADDRINUSE via pointer errno in OpError",
			err: func() error {
				errno := syscall.EADDRINUSE
				return &net.OpError{
					Op:  "listen",
					Net: "tcp",
					Addr: &net.TCPAddr{
						IP:   net.IPv4(0, 0, 0, 0),
						Port: 5000,
					},
					Err: &errno,
				}
			}(),
			want: true,
		},
		{
			name: "different errno via pointer",
			err: func() error {
				errno := syscall.ECONNREFUSED
				return &net.OpError{
					Op:  "listen",
					Net: "tcp",
					Err: &errno,
				}
			}(),
			want: false,
		},
		{
			name: "plain error",
			err:  errors.New("something else"),
			want: false,
		},
		{
			name: "nil error",
			err:  nil,
			want: false,
		},
		{
			name: "OpError without SyscallError",
			err: &net.OpError{
				Op:  "listen",
				Net: "tcp",
				Err: errors.New("random inner"),
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := isAddrInUse(tt.err)
			assert.Equal(t, tt.want, got)
		})
	}
}

// ============================================================================
// StopAll clears all handler slices
// ============================================================================

func TestProtocolManager_StopAll_ClearsHandlers(t *testing.T) {
	t.Parallel()

	pm := NewProtocolManager()

	// Add a GraphQL handler so we can verify it gets cleared.
	cfg := &graphql.GraphQLConfig{
		ID:      "clear-test",
		Path:    "/graphql",
		Enabled: true,
		Schema:  `type Query { hello: String }`,
	}
	handler, err := graphql.Endpoint(cfg)
	require.NoError(t, err)
	pm.AddGraphQLHandler(handler)

	// Add a subscription handler.
	subHandler := graphql.NewSubscriptionHandler(nil, cfg)
	pm.AddGraphQLSubscriptionHandler(subHandler)

	require.Len(t, pm.GraphQLHandlers(), 1, "precondition: 1 handler before StopAll")

	errs := pm.StopAll(context.Background(), 5*time.Second)
	assert.Empty(t, errs)
	assert.Empty(t, pm.GraphQLHandlers(), "GraphQL handlers should be cleared after StopAll")
	assert.Empty(t, pm.GRPCServers(), "gRPC servers should be cleared after StopAll")
	assert.Empty(t, pm.GetMQTTBrokers(), "MQTT brokers should be cleared after StopAll")
	assert.Empty(t, pm.OAuthProviders(), "OAuth providers should be cleared after StopAll")
	assert.Empty(t, pm.OAuthHandlers(), "OAuth handlers should be cleared after StopAll")
	assert.Empty(t, pm.SOAPHandlers(), "SOAP handlers should be cleared after StopAll")
}

// ============================================================================
// Multiple AddGraphQLHandler calls accumulate
// ============================================================================

func TestProtocolManager_AddMultipleGraphQLHandlers(t *testing.T) {
	t.Parallel()

	pm := NewProtocolManager()

	for i := 0; i < 3; i++ {
		cfg := &graphql.GraphQLConfig{
			ID:      "gql-" + string(rune('a'+i)),
			Path:    "/graphql",
			Enabled: true,
			Schema:  `type Query { hello: String }`,
		}
		h, err := graphql.Endpoint(cfg)
		require.NoError(t, err)
		pm.AddGraphQLHandler(h)
	}

	assert.Len(t, pm.GraphQLHandlers(), 3, "should accumulate 3 handlers")
}

// ============================================================================
// GraphQLHandlers returns a copy (mutation safety)
// ============================================================================

func TestProtocolManager_GraphQLHandlers_ReturnsCopy(t *testing.T) {
	t.Parallel()

	pm := NewProtocolManager()
	cfg := &graphql.GraphQLConfig{
		ID:      "copy-test",
		Path:    "/graphql",
		Enabled: true,
		Schema:  `type Query { hello: String }`,
	}
	h, err := graphql.Endpoint(cfg)
	require.NoError(t, err)
	pm.AddGraphQLHandler(h)

	slice1 := pm.GraphQLHandlers()
	slice2 := pm.GraphQLHandlers()

	// Mutating the returned slice should not affect the manager.
	slice1[0] = nil
	assert.NotNil(t, slice2[0], "returned slices should be independent copies")
	assert.NotNil(t, pm.GraphQLHandlers()[0], "internal slice should be unaffected")
}
