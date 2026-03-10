package engine

import (
	"sort"
	"testing"

	"github.com/getmockd/mockd/internal/storage"
	"github.com/getmockd/mockd/pkg/config"
	"github.com/getmockd/mockd/pkg/graphql"
	"github.com/getmockd/mockd/pkg/oauth"
	"github.com/getmockd/mockd/pkg/soap"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestHandler creates a Handler with an in-memory store for testing.
func newTestHandler() *Handler {
	return NewHandler(storage.NewInMemoryMockStore())
}

// ============================================================================
// GraphQL Handler Registration
// ============================================================================

func TestHandlerProtocol_GraphQL_RegisterAndGet(t *testing.T) {
	t.Parallel()

	h := newTestHandler()
	gqlHandler := &graphql.Handler{}

	h.RegisterGraphQLHandler("/graphql", gqlHandler)

	got := h.getGraphQLHandler("/graphql")
	assert.Same(t, gqlHandler, got, "should return the registered handler")
}

func TestHandlerProtocol_GraphQL_GetUnregistered(t *testing.T) {
	t.Parallel()

	h := newTestHandler()

	got := h.getGraphQLHandler("/graphql")
	assert.Nil(t, got, "unregistered path should return nil")
}

func TestHandlerProtocol_GraphQL_RegisterOverwrite(t *testing.T) {
	t.Parallel()

	h := newTestHandler()
	first := &graphql.Handler{}
	second := &graphql.Handler{}

	h.RegisterGraphQLHandler("/graphql", first)
	h.RegisterGraphQLHandler("/graphql", second)

	got := h.getGraphQLHandler("/graphql")
	assert.Same(t, second, got, "second registration should overwrite the first")
	assert.NotSame(t, first, got)
}

func TestHandlerProtocol_GraphQL_Unregister(t *testing.T) {
	t.Parallel()

	h := newTestHandler()
	h.RegisterGraphQLHandler("/graphql", &graphql.Handler{})

	h.UnregisterGraphQLHandler("/graphql")

	got := h.getGraphQLHandler("/graphql")
	assert.Nil(t, got, "handler should be nil after unregister")
}

func TestHandlerProtocol_GraphQL_UnregisterNonExistent(t *testing.T) {
	t.Parallel()

	h := newTestHandler()

	// Should not panic.
	assert.NotPanics(t, func() {
		h.UnregisterGraphQLHandler("/does-not-exist")
	})
}

func TestHandlerProtocol_GraphQL_ListEmpty(t *testing.T) {
	t.Parallel()

	h := newTestHandler()

	paths := h.ListGraphQLHandlerPaths()
	assert.Empty(t, paths, "empty handler should return no paths")
}

func TestHandlerProtocol_GraphQL_ListPaths(t *testing.T) {
	t.Parallel()

	h := newTestHandler()
	h.RegisterGraphQLHandler("/graphql", &graphql.Handler{})
	h.RegisterGraphQLHandler("/api/graphql", &graphql.Handler{})
	h.RegisterGraphQLHandler("/v2/graphql", &graphql.Handler{})

	paths := h.ListGraphQLHandlerPaths()
	sort.Strings(paths)

	expected := []string{"/api/graphql", "/graphql", "/v2/graphql"}
	assert.Equal(t, expected, paths)
}

func TestHandlerProtocol_GraphQL_RoundTrip(t *testing.T) {
	t.Parallel()

	h := newTestHandler()
	handler1 := &graphql.Handler{}
	handler2 := &graphql.Handler{}

	// Register two handlers
	h.RegisterGraphQLHandler("/a", handler1)
	h.RegisterGraphQLHandler("/b", handler2)
	require.Len(t, h.ListGraphQLHandlerPaths(), 2)

	// Unregister one
	h.UnregisterGraphQLHandler("/a")
	paths := h.ListGraphQLHandlerPaths()
	assert.Equal(t, []string{"/b"}, paths)
	assert.Nil(t, h.getGraphQLHandler("/a"))
	assert.Same(t, handler2, h.getGraphQLHandler("/b"))

	// Unregister the other
	h.UnregisterGraphQLHandler("/b")
	assert.Empty(t, h.ListGraphQLHandlerPaths())
}

// ============================================================================
// SOAP Handler Registration
// ============================================================================

func TestHandlerProtocol_SOAP_RegisterAndGet(t *testing.T) {
	t.Parallel()

	h := newTestHandler()
	soapHandler := &soap.Handler{}

	h.RegisterSOAPHandler("/soap", soapHandler)

	got := h.getSOAPHandler("/soap")
	assert.Same(t, soapHandler, got)
}

func TestHandlerProtocol_SOAP_GetUnregistered(t *testing.T) {
	t.Parallel()

	h := newTestHandler()

	got := h.getSOAPHandler("/soap")
	assert.Nil(t, got)
}

func TestHandlerProtocol_SOAP_RegisterOverwrite(t *testing.T) {
	t.Parallel()

	h := newTestHandler()
	first := &soap.Handler{}
	second := &soap.Handler{}

	h.RegisterSOAPHandler("/soap", first)
	h.RegisterSOAPHandler("/soap", second)

	got := h.getSOAPHandler("/soap")
	assert.Same(t, second, got, "second registration should overwrite the first")
}

func TestHandlerProtocol_SOAP_Unregister(t *testing.T) {
	t.Parallel()

	h := newTestHandler()
	h.RegisterSOAPHandler("/soap", &soap.Handler{})

	h.UnregisterSOAPHandler("/soap")

	assert.Nil(t, h.getSOAPHandler("/soap"))
}

func TestHandlerProtocol_SOAP_UnregisterNonExistent(t *testing.T) {
	t.Parallel()

	h := newTestHandler()

	assert.NotPanics(t, func() {
		h.UnregisterSOAPHandler("/nope")
	})
}

func TestHandlerProtocol_SOAP_ListEmpty(t *testing.T) {
	t.Parallel()

	h := newTestHandler()

	paths := h.ListSOAPHandlerPaths()
	assert.Empty(t, paths)
}

func TestHandlerProtocol_SOAP_ListPaths(t *testing.T) {
	t.Parallel()

	h := newTestHandler()
	h.RegisterSOAPHandler("/soap", &soap.Handler{})
	h.RegisterSOAPHandler("/api/soap", &soap.Handler{})

	paths := h.ListSOAPHandlerPaths()
	sort.Strings(paths)

	assert.Equal(t, []string{"/api/soap", "/soap"}, paths)
}

func TestHandlerProtocol_SOAP_RoundTrip(t *testing.T) {
	t.Parallel()

	h := newTestHandler()
	handler1 := &soap.Handler{}
	handler2 := &soap.Handler{}

	h.RegisterSOAPHandler("/a", handler1)
	h.RegisterSOAPHandler("/b", handler2)
	require.Len(t, h.ListSOAPHandlerPaths(), 2)

	h.UnregisterSOAPHandler("/a")
	paths := h.ListSOAPHandlerPaths()
	assert.Equal(t, []string{"/b"}, paths)
	assert.Nil(t, h.getSOAPHandler("/a"))
	assert.Same(t, handler2, h.getSOAPHandler("/b"))

	h.UnregisterSOAPHandler("/b")
	assert.Empty(t, h.ListSOAPHandlerPaths())
}

// ============================================================================
// OAuth Handler Registration
// ============================================================================

func TestHandlerProtocol_OAuth_RegisterWithPath(t *testing.T) {
	t.Parallel()

	h := newTestHandler()
	oauthHandler := &oauth.Handler{}
	cfg := &oauth.OAuthConfig{
		Issuer: "http://localhost:4280/oauth",
	}

	h.RegisterOAuthHandler(cfg, oauthHandler)

	// RegisterOAuthHandler extracts "/oauth" and registers 7 sub-paths.
	expectedPaths := []string{
		"/oauth/authorize",
		"/oauth/token",
		"/oauth/userinfo",
		"/oauth/revoke",
		"/oauth/introspect",
		"/oauth/.well-known/jwks.json",
		"/oauth/.well-known/openid-configuration",
	}
	for _, p := range expectedPaths {
		got := h.getOAuthHandler(p)
		assert.Same(t, oauthHandler, got, "path %s should be registered", p)
	}
}

func TestHandlerProtocol_OAuth_RegisterEmptyIssuer(t *testing.T) {
	t.Parallel()

	h := newTestHandler()
	oauthHandler := &oauth.Handler{}
	cfg := &oauth.OAuthConfig{Issuer: ""}

	h.RegisterOAuthHandler(cfg, oauthHandler)

	// Empty issuer → basePath = "" → routes at root.
	expectedPaths := []string{
		"/authorize",
		"/token",
		"/userinfo",
		"/revoke",
		"/introspect",
		"/.well-known/jwks.json",
		"/.well-known/openid-configuration",
	}
	for _, p := range expectedPaths {
		got := h.getOAuthHandler(p)
		assert.Same(t, oauthHandler, got, "path %s should be registered", p)
	}
}

func TestHandlerProtocol_OAuth_PathExtraction(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		issuer       string
		wantBasePath string // we verify by checking basePath + "/token"
	}{
		{
			name:         "standard localhost",
			issuer:       "http://localhost:4280/oauth",
			wantBasePath: "/oauth",
		},
		{
			name:         "https with deep path",
			issuer:       "https://auth.example.com/api/v1/auth",
			wantBasePath: "/api/v1/auth",
		},
		{
			name:         "no path in issuer",
			issuer:       "http://localhost:4280",
			wantBasePath: "", // no slash after host → empty basePath
		},
		{
			name:         "empty issuer",
			issuer:       "",
			wantBasePath: "",
		},
		{
			name:         "root path only",
			issuer:       "http://localhost:4280/",
			wantBasePath: "/",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			h := newTestHandler()
			oauthHandler := &oauth.Handler{}
			cfg := &oauth.OAuthConfig{Issuer: tt.issuer}

			h.RegisterOAuthHandler(cfg, oauthHandler)

			tokenPath := tt.wantBasePath + "/token"
			got := h.getOAuthHandler(tokenPath)
			assert.Same(t, oauthHandler, got,
				"issuer=%q: expected handler at %s", tt.issuer, tokenPath)
		})
	}
}

func TestHandlerProtocol_OAuth_Unregister(t *testing.T) {
	t.Parallel()

	h := newTestHandler()
	oauthHandler := &oauth.Handler{}
	issuer := "http://localhost:4280/oauth"
	cfg := &oauth.OAuthConfig{Issuer: issuer}

	h.RegisterOAuthHandler(cfg, oauthHandler)

	// Verify registered
	require.NotNil(t, h.getOAuthHandler("/oauth/token"))

	h.UnregisterOAuthHandler(issuer)

	// All 7 paths should be gone.
	paths := []string{
		"/oauth/authorize",
		"/oauth/token",
		"/oauth/userinfo",
		"/oauth/revoke",
		"/oauth/introspect",
		"/oauth/.well-known/jwks.json",
		"/oauth/.well-known/openid-configuration",
	}
	for _, p := range paths {
		assert.Nil(t, h.getOAuthHandler(p), "path %s should be unregistered", p)
	}
}

func TestHandlerProtocol_OAuth_UnregisterEmptyIssuer(t *testing.T) {
	t.Parallel()

	h := newTestHandler()
	cfg := &oauth.OAuthConfig{Issuer: ""}
	h.RegisterOAuthHandler(cfg, &oauth.Handler{})

	require.NotNil(t, h.getOAuthHandler("/token"))

	h.UnregisterOAuthHandler("")

	assert.Nil(t, h.getOAuthHandler("/token"))
	assert.Nil(t, h.getOAuthHandler("/authorize"))
}

func TestHandlerProtocol_OAuth_MultipleProviders(t *testing.T) {
	t.Parallel()

	h := newTestHandler()
	handler1 := &oauth.Handler{}
	handler2 := &oauth.Handler{}

	cfg1 := &oauth.OAuthConfig{Issuer: "http://localhost:4280/provider1"}
	cfg2 := &oauth.OAuthConfig{Issuer: "http://localhost:4280/provider2"}

	h.RegisterOAuthHandler(cfg1, handler1)
	h.RegisterOAuthHandler(cfg2, handler2)

	assert.Same(t, handler1, h.getOAuthHandler("/provider1/token"))
	assert.Same(t, handler2, h.getOAuthHandler("/provider2/token"))

	// Unregister one; the other should remain.
	h.UnregisterOAuthHandler("http://localhost:4280/provider1")
	assert.Nil(t, h.getOAuthHandler("/provider1/token"))
	assert.Same(t, handler2, h.getOAuthHandler("/provider2/token"))
}

// ============================================================================
// WebSocket Endpoint Registration
// ============================================================================

func TestHandlerProtocol_WebSocket_RegisterAndGet(t *testing.T) {
	t.Parallel()

	h := newTestHandler()
	wsCfg := &config.WebSocketEndpointConfig{
		Path: "/ws/chat",
	}

	err := h.RegisterWebSocketEndpoint(wsCfg)
	require.NoError(t, err)

	ep := h.wsManager.GetEndpoint("/ws/chat")
	require.NotNil(t, ep, "endpoint should be registered")
}

func TestHandlerProtocol_WebSocket_RegisterMultiple(t *testing.T) {
	t.Parallel()

	h := newTestHandler()

	err := h.RegisterWebSocketEndpoint(&config.WebSocketEndpointConfig{Path: "/ws/a"})
	require.NoError(t, err)
	err = h.RegisterWebSocketEndpoint(&config.WebSocketEndpointConfig{Path: "/ws/b"})
	require.NoError(t, err)

	assert.NotNil(t, h.wsManager.GetEndpoint("/ws/a"))
	assert.NotNil(t, h.wsManager.GetEndpoint("/ws/b"))
	assert.Nil(t, h.wsManager.GetEndpoint("/ws/c"), "unregistered path returns nil")
}

func TestHandlerProtocol_WebSocket_Unregister(t *testing.T) {
	t.Parallel()

	h := newTestHandler()
	wsCfg := &config.WebSocketEndpointConfig{Path: "/ws/test"}

	err := h.RegisterWebSocketEndpoint(wsCfg)
	require.NoError(t, err)
	require.NotNil(t, h.wsManager.GetEndpoint("/ws/test"))

	h.UnregisterWebSocketEndpoint("/ws/test")

	assert.Nil(t, h.wsManager.GetEndpoint("/ws/test"), "endpoint should be removed")
}

func TestHandlerProtocol_WebSocket_UnregisterNonExistent(t *testing.T) {
	t.Parallel()

	h := newTestHandler()

	assert.NotPanics(t, func() {
		h.UnregisterWebSocketEndpoint("/ws/nope")
	})
}

// ============================================================================
// GraphQL Subscription Handler Registration
// ============================================================================

func TestHandlerProtocol_GraphQLSubscription_RegisterAndGet(t *testing.T) {
	t.Parallel()

	h := newTestHandler()
	subHandler := &graphql.SubscriptionHandler{}

	h.RegisterGraphQLSubscriptionHandler("/graphql/ws", subHandler)

	got := h.getGraphQLSubscriptionHandler("/graphql/ws")
	assert.Same(t, subHandler, got)
}

func TestHandlerProtocol_GraphQLSubscription_GetUnregistered(t *testing.T) {
	t.Parallel()

	h := newTestHandler()

	got := h.getGraphQLSubscriptionHandler("/graphql/ws")
	assert.Nil(t, got)
}

// ============================================================================
// Cross-protocol isolation
// ============================================================================

func TestHandlerProtocol_CrossProtocolIsolation(t *testing.T) {
	t.Parallel()

	h := newTestHandler()

	// Register at the same path for different protocols.
	gqlH := &graphql.Handler{}
	soapH := &soap.Handler{}

	h.RegisterGraphQLHandler("/api", gqlH)
	h.RegisterSOAPHandler("/api", soapH)

	// Each protocol's getter returns only its own handler.
	assert.Same(t, gqlH, h.getGraphQLHandler("/api"))
	assert.Same(t, soapH, h.getSOAPHandler("/api"))

	// Unregistering one doesn't affect the other.
	h.UnregisterGraphQLHandler("/api")
	assert.Nil(t, h.getGraphQLHandler("/api"))
	assert.Same(t, soapH, h.getSOAPHandler("/api"), "SOAP handler should be unaffected")
}

// ============================================================================
// List consistency after register/unregister cycles
// ============================================================================

func TestHandlerProtocol_ListConsistencyAfterCycles(t *testing.T) {
	t.Parallel()

	h := newTestHandler()

	// Cycle: register → unregister → re-register
	h.RegisterGraphQLHandler("/a", &graphql.Handler{})
	h.RegisterGraphQLHandler("/b", &graphql.Handler{})
	h.UnregisterGraphQLHandler("/a")
	h.RegisterGraphQLHandler("/c", &graphql.Handler{})
	h.UnregisterGraphQLHandler("/b")
	h.RegisterGraphQLHandler("/a", &graphql.Handler{})

	paths := h.ListGraphQLHandlerPaths()
	sort.Strings(paths)
	assert.Equal(t, []string{"/a", "/c"}, paths)

	// Same cycle for SOAP
	h.RegisterSOAPHandler("/x", &soap.Handler{})
	h.RegisterSOAPHandler("/y", &soap.Handler{})
	h.UnregisterSOAPHandler("/x")

	soapPaths := h.ListSOAPHandlerPaths()
	assert.Equal(t, []string{"/y"}, soapPaths)
}
