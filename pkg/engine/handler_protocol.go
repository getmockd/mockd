// Protocol handler registration for GraphQL, OAuth, SOAP, and WebSocket.

package engine

import (
	"net/http"
	"strings"

	"github.com/getmockd/mockd/pkg/config"
	"github.com/getmockd/mockd/pkg/graphql"
	"github.com/getmockd/mockd/pkg/oauth"
	"github.com/getmockd/mockd/pkg/soap"
	"github.com/getmockd/mockd/pkg/sse"
	"github.com/getmockd/mockd/pkg/websocket"
)

// SSEHandler returns the SSE handler for admin API access.
func (h *Handler) SSEHandler() *sse.SSEHandler { return h.sseHandler }

// ChunkedHandler returns the chunked transfer handler.
func (h *Handler) ChunkedHandler() *sse.ChunkedHandler { return h.chunkedHandler }

// WebSocketManager returns the WebSocket connection manager.
func (h *Handler) WebSocketManager() *websocket.ConnectionManager { return h.wsManager }

// handleWebSocket handles WebSocket upgrade requests.
func (h *Handler) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	endpoint := h.wsManager.GetEndpoint(r.URL.Path)
	if endpoint == nil {
		http.Error(w, `{"error": "websocket_endpoint_not_found", "path": "`+r.URL.Path+`"}`, http.StatusNotFound)
		return
	}
	if err := endpoint.HandleUpgrade(w, r); err != nil {
		return
	}
}

// RegisterWebSocketEndpoint registers a WebSocket endpoint from config.
func (h *Handler) RegisterWebSocketEndpoint(cfg *config.WebSocketEndpointConfig) error {
	endpoint, err := websocket.EndpointFromConfig(cfg)
	if err != nil {
		return err
	}
	// Wire template engine so WebSocket responses can use template variables
	if h.templateEngine != nil {
		endpoint.SetTemplateEngine(h.templateEngine)
	}
	h.wsManager.RegisterEndpoint(endpoint)
	return nil
}

// RegisterGraphQLHandler registers a GraphQL handler at the specified path.
func (h *Handler) RegisterGraphQLHandler(path string, handler *graphql.Handler) {
	h.graphqlMu.Lock()
	defer h.graphqlMu.Unlock()
	h.graphqlHandlers[path] = handler
}

// UnregisterGraphQLHandler removes a GraphQL handler at the specified path.
func (h *Handler) UnregisterGraphQLHandler(path string) {
	h.graphqlMu.Lock()
	defer h.graphqlMu.Unlock()
	delete(h.graphqlHandlers, path)
}

// ListGraphQLHandlerPaths returns all registered GraphQL handler paths.
func (h *Handler) ListGraphQLHandlerPaths() []string {
	h.graphqlMu.RLock()
	defer h.graphqlMu.RUnlock()
	paths := make([]string, 0, len(h.graphqlHandlers))
	for path := range h.graphqlHandlers {
		paths = append(paths, path)
	}
	return paths
}

// RegisterGraphQLSubscriptionHandler registers a GraphQL subscription handler at the specified path.
func (h *Handler) RegisterGraphQLSubscriptionHandler(path string, handler *graphql.SubscriptionHandler) {
	h.graphqlSubMu.Lock()
	defer h.graphqlSubMu.Unlock()
	h.graphqlSubs[path] = handler
}

// RegisterOAuthHandler registers OAuth handlers at the provider's configured paths.
func (h *Handler) RegisterOAuthHandler(cfg *oauth.OAuthConfig, handler *oauth.Handler) {
	h.oauthMu.Lock()
	defer h.oauthMu.Unlock()

	basePath := ""
	if cfg.Issuer != "" {
		if idx := strings.Index(cfg.Issuer, "://"); idx != -1 {
			remainder := cfg.Issuer[idx+3:]
			if pathIdx := strings.Index(remainder, "/"); pathIdx != -1 {
				basePath = remainder[pathIdx:]
			}
		}
	}

	h.oauthHandlers[basePath+"/authorize"] = handler
	h.oauthHandlers[basePath+"/token"] = handler
	h.oauthHandlers[basePath+"/userinfo"] = handler
	h.oauthHandlers[basePath+"/revoke"] = handler
	h.oauthHandlers[basePath+"/introspect"] = handler
	h.oauthHandlers[basePath+"/.well-known/jwks.json"] = handler
	h.oauthHandlers[basePath+"/.well-known/openid-configuration"] = handler
}

// RegisterSOAPHandler registers a SOAP handler at the specified path.
func (h *Handler) RegisterSOAPHandler(path string, handler *soap.Handler) {
	h.soapMu.Lock()
	defer h.soapMu.Unlock()
	h.soapHandlers[path] = handler
}

// UnregisterSOAPHandler removes a SOAP handler at the specified path.
func (h *Handler) UnregisterSOAPHandler(path string) {
	h.soapMu.Lock()
	defer h.soapMu.Unlock()
	delete(h.soapHandlers, path)
}

// UnregisterOAuthHandler removes all OAuth handler routes for a given issuer base path.
func (h *Handler) UnregisterOAuthHandler(issuer string) {
	h.oauthMu.Lock()
	defer h.oauthMu.Unlock()

	basePath := ""
	if issuer != "" {
		if idx := strings.Index(issuer, "://"); idx != -1 {
			remainder := issuer[idx+3:]
			if pathIdx := strings.Index(remainder, "/"); pathIdx != -1 {
				basePath = remainder[pathIdx:]
			}
		}
	}

	delete(h.oauthHandlers, basePath+"/authorize")
	delete(h.oauthHandlers, basePath+"/token")
	delete(h.oauthHandlers, basePath+"/userinfo")
	delete(h.oauthHandlers, basePath+"/revoke")
	delete(h.oauthHandlers, basePath+"/introspect")
	delete(h.oauthHandlers, basePath+"/.well-known/jwks.json")
	delete(h.oauthHandlers, basePath+"/.well-known/openid-configuration")
}

// UnregisterWebSocketEndpoint removes a WebSocket endpoint by path.
func (h *Handler) UnregisterWebSocketEndpoint(path string) {
	h.wsManager.UnregisterEndpoint(path)
}

// ListSOAPHandlerPaths returns all registered SOAP handler paths.
func (h *Handler) ListSOAPHandlerPaths() []string {
	h.soapMu.RLock()
	defer h.soapMu.RUnlock()
	paths := make([]string, 0, len(h.soapHandlers))
	for path := range h.soapHandlers {
		paths = append(paths, path)
	}
	return paths
}

// getGraphQLHandler returns the GraphQL handler for a path, if any.
func (h *Handler) getGraphQLHandler(path string) *graphql.Handler {
	h.graphqlMu.RLock()
	defer h.graphqlMu.RUnlock()
	return h.graphqlHandlers[path]
}

// getGraphQLSubscriptionHandler returns the GraphQL subscription handler for a path, if any.
func (h *Handler) getGraphQLSubscriptionHandler(path string) *graphql.SubscriptionHandler {
	h.graphqlSubMu.RLock()
	defer h.graphqlSubMu.RUnlock()
	return h.graphqlSubs[path]
}

// getOAuthHandler returns the OAuth handler for a path, if any.
func (h *Handler) getOAuthHandler(path string) *oauth.Handler {
	h.oauthMu.RLock()
	defer h.oauthMu.RUnlock()
	return h.oauthHandlers[path]
}

// getSOAPHandler returns the SOAP handler for a path, if any.
func (h *Handler) getSOAPHandler(path string) *soap.Handler {
	h.soapMu.RLock()
	defer h.soapMu.RUnlock()
	return h.soapHandlers[path]
}

// routeOAuthRequest routes an OAuth request to the appropriate handler method.
func (h *Handler) routeOAuthRequest(w http.ResponseWriter, r *http.Request, handler *oauth.Handler) {
	path := r.URL.Path
	switch {
	case strings.HasSuffix(path, "/authorize"):
		handler.HandleAuthorize(w, r)
	case strings.HasSuffix(path, "/token"):
		handler.HandleToken(w, r)
	case strings.HasSuffix(path, "/userinfo"):
		handler.HandleUserInfo(w, r)
	case strings.HasSuffix(path, "/revoke"):
		handler.HandleRevoke(w, r)
	case strings.HasSuffix(path, "/introspect"):
		handler.HandleIntrospect(w, r)
	case strings.HasSuffix(path, "/.well-known/jwks.json"):
		handler.HandleJWKS(w, r)
	case strings.HasSuffix(path, "/.well-known/openid-configuration"):
		handler.HandleOpenIDConfig(w, r)
	default:
		http.Error(w, `{"error": "not_found", "error_description": "unknown OAuth endpoint"}`, http.StatusNotFound)
	}
}
