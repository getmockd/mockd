package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/getmockd/mockd/pkg/cli"
	"github.com/getmockd/mockd/pkg/logging"
	"github.com/getmockd/mockd/pkg/stateful"
)

// ServerVersion is the mockd server version.
const ServerVersion = "0.2.0"

// Server is the MCP protocol server.
type Server struct {
	config        *Config
	adminClient   cli.AdminClient
	statefulStore *stateful.StateStore
	sessions      *SessionManager
	tools         *ToolRegistry
	resources     *ResourceProvider
	httpServer    *http.Server
	stopCh        chan struct{}
	mu            sync.RWMutex
	running       bool
	log           *slog.Logger
}

// NewServer creates a new MCP server.
// The adminClient is used for mock operations via HTTP.
// statefulStore is optional and used for stateful resource operations.
func NewServer(cfg *Config, adminClient cli.AdminClient, statefulStore *stateful.StateStore) *Server {
	if cfg == nil {
		cfg = DefaultConfig()
	}

	// Create admin client if not provided but AdminURL is configured
	if adminClient == nil && cfg.AdminURL != "" {
		adminClient = cli.NewAdminClient(cfg.AdminURL)
	}

	s := &Server{
		config:        cfg,
		adminClient:   adminClient,
		statefulStore: statefulStore,
		sessions:      NewSessionManager(cfg),
		stopCh:        make(chan struct{}),
		log:           logging.Nop(),
	}

	// Initialize tool registry with handlers
	s.tools = NewToolRegistry(s)

	// Initialize resource provider
	s.resources = NewResourceProvider(adminClient, statefulStore)

	return s
}

// Start starts the MCP HTTP server.
func (s *Server) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.running {
		return fmt.Errorf("MCP server is already running")
	}

	if err := s.config.Validate(); err != nil {
		return fmt.Errorf("invalid MCP config: %w", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc(s.config.Path, s.handleMCP)

	s.httpServer = &http.Server{
		Addr:         s.config.Address(),
		Handler:      s.withMiddleware(mux),
		ReadTimeout:  s.config.ReadTimeout,
		WriteTimeout: s.config.WriteTimeout,
	}

	// Start session cleanup routine
	s.sessions.StartCleanupRoutine(time.Minute, s.stopCh)

	go func() {
		if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			s.log.Error("MCP server error", "error", err)
		}
	}()

	s.running = true
	return nil
}

// Stop gracefully shuts down the MCP server.
func (s *Server) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return nil
	}

	close(s.stopCh)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := s.httpServer.Shutdown(ctx); err != nil {
		return fmt.Errorf("MCP server shutdown: %w", err)
	}

	s.sessions.Close()
	s.running = false
	return nil
}

// Handler returns the HTTP handler for the MCP server.
// This is useful for testing without starting the HTTP server.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc(s.config.Path, s.handleMCP)
	return s.withMiddleware(mux)
}

// withMiddleware wraps the handler with CORS and origin validation.
func (s *Server) withMiddleware(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Localhost check if remote not allowed
		if !s.config.AllowRemote {
			if !isLocalhost(r.RemoteAddr) {
				http.Error(w, "Remote access not allowed", http.StatusForbidden)
				return
			}
		}

		// Origin validation
		origin := r.Header.Get("Origin")
		if origin != "" && !s.isOriginAllowed(origin) {
			http.Error(w, "Origin not allowed", http.StatusForbidden)
			return
		}

		// CORS headers
		if origin != "" {
			w.Header().Set("Access-Control-Allow-Origin", origin)
		} else {
			w.Header().Set("Access-Control-Allow-Origin", "*")
		}
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Accept, Mcp-Session-Id, MCP-Protocol-Version, Last-Event-ID")
		w.Header().Set("Access-Control-Expose-Headers", "Mcp-Session-Id")

		// Handle preflight
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		handler.ServeHTTP(w, r)
	})
}

// isLocalhost checks if the remote address is localhost.
func isLocalhost(remoteAddr string) bool {
	// Empty address is allowed (test environment or internal calls)
	if remoteAddr == "" {
		return true
	}

	// Remove port if present
	host := remoteAddr
	if idx := strings.LastIndex(remoteAddr, ":"); idx != -1 {
		host = remoteAddr[:idx]
	}
	// Handle IPv6 addresses in brackets
	host = strings.TrimPrefix(host, "[")
	host = strings.TrimSuffix(host, "]")

	return host == "127.0.0.1" || host == "localhost" || host == "::1"
}

// isOriginAllowed checks if the origin is in the allowed list.
func (s *Server) isOriginAllowed(origin string) bool {
	for _, allowed := range s.config.AllowedOrigins {
		if allowed == "*" {
			return true
		}
		if matchOrigin(origin, allowed) {
			return true
		}
	}
	return false
}

// matchOrigin matches an origin against a pattern (supports * wildcard for port).
func matchOrigin(origin, pattern string) bool {
	if origin == pattern {
		return true
	}

	// Handle wildcard patterns like "http://localhost:*"
	if strings.HasSuffix(pattern, ":*") {
		prefix := strings.TrimSuffix(pattern, "*")
		// Match origin starting with prefix followed by digits
		if strings.HasPrefix(origin, prefix) {
			rest := origin[len(prefix):]
			// Check if rest is all digits (port number)
			for _, c := range rest {
				if c < '0' || c > '9' {
					return false
				}
			}
			return len(rest) > 0
		}
	}

	return false
}

// handleMCP is the main handler for MCP requests.
func (s *Server) handleMCP(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		// SSE stream
		s.handleSSE(w, r)
	case "POST":
		// JSON-RPC request
		s.handleJSONRPC(w, r)
	case "DELETE":
		// Session termination
		s.handleSessionDelete(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleJSONRPC handles JSON-RPC POST requests.
func (s *Server) handleJSONRPC(w http.ResponseWriter, r *http.Request) {
	// Parse the request
	req, parseErr := ParseRequest(r.Body)
	if parseErr != nil {
		s.writeError(w, nil, parseErr)
		return
	}

	// Get or create session
	sessionID := r.Header.Get("Mcp-Session-Id")
	var session *MCPSession

	// Initialize is special - creates a new session
	if req.Method == "initialize" {
		var err error
		session, err = s.sessions.Create()
		if err != nil {
			s.writeError(w, req.ID, InternalError(err))
			return
		}
		// Return session ID in header
		w.Header().Set("Mcp-Session-Id", session.ID)
	} else {
		// All other methods require an existing session
		if sessionID == "" {
			s.writeError(w, req.ID, SessionRequiredError())
			return
		}
		session = s.sessions.Get(sessionID)
		if session == nil {
			s.writeError(w, req.ID, SessionExpiredError(sessionID))
			return
		}
		session.Touch()
	}

	// Dispatch the request
	result, err := s.dispatch(session, req)

	// Handle notification (no response needed)
	if req.IsNotification() {
		w.WriteHeader(http.StatusAccepted)
		return
	}

	// Handle error
	if err != nil {
		s.writeError(w, req.ID, err)
		return
	}

	// Write success response
	s.writeSuccess(w, req.ID, result)
}

// dispatch routes the request to the appropriate handler.
func (s *Server) dispatch(session *MCPSession, req *JSONRPCRequest) (interface{}, *JSONRPCError) {
	switch req.Method {
	// Lifecycle methods
	case "initialize":
		return s.handleInitialize(session, req.Params)
	case "initialized":
		return s.handleInitialized(session)
	case "ping":
		return s.handlePing()

	// Tool methods
	case "tools/list":
		return s.handleToolsList(session)
	case "tools/call":
		return s.handleToolsCall(session, req.Params)

	// Resource methods
	case "resources/list":
		return s.handleResourcesList(session)
	case "resources/read":
		return s.handleResourcesRead(session, req.Params)
	case "resources/subscribe":
		return s.handleResourcesSubscribe(session, req.Params)
	case "resources/unsubscribe":
		return s.handleResourcesUnsubscribe(session, req.Params)

	default:
		return nil, MethodNotFoundError(req.Method)
	}
}

// handleInitialize handles the initialize request.
func (s *Server) handleInitialize(session *MCPSession, params json.RawMessage) (interface{}, *JSONRPCError) {
	initParams, err := UnmarshalParamsRequired[InitializeParams](params)
	if err != nil {
		return nil, err
	}

	// Validate protocol version
	if initParams.ProtocolVersion != ProtocolVersion {
		return nil, ProtocolVersionError(initParams.ProtocolVersion, ProtocolVersion)
	}

	// Store client info
	session.ProtocolVersion = initParams.ProtocolVersion
	session.ClientInfo = initParams.ClientInfo
	session.Capabilities = initParams.Capabilities
	session.SetState(SessionStateInitialized)

	// Return server capabilities
	return &InitializeResult{
		ProtocolVersion: ProtocolVersion,
		Capabilities: ServerCapabilities{
			Tools: &ToolsCapability{
				ListChanged: false,
			},
			Resources: &ResourcesCapability{
				Subscribe:   true,
				ListChanged: true,
			},
		},
		ServerInfo: ServerInfo{
			Name:    "mockd",
			Version: ServerVersion,
		},
	}, nil
}

// handleInitialized handles the initialized notification.
func (s *Server) handleInitialized(session *MCPSession) (interface{}, *JSONRPCError) {
	if session.GetState() != SessionStateInitialized {
		return nil, NotInitializedError()
	}
	session.SetState(SessionStateReady)
	return nil, nil
}

// handlePing handles the ping request.
func (s *Server) handlePing() (interface{}, *JSONRPCError) {
	return map[string]interface{}{}, nil
}

// handleToolsList returns the list of available tools.
func (s *Server) handleToolsList(session *MCPSession) (interface{}, *JSONRPCError) {
	if session.GetState() != SessionStateReady {
		return nil, NotInitializedError()
	}

	return &ToolsListResult{
		Tools: s.tools.List(),
	}, nil
}

// handleToolsCall executes a tool.
func (s *Server) handleToolsCall(session *MCPSession, params json.RawMessage) (interface{}, *JSONRPCError) {
	if session.GetState() != SessionStateReady {
		return nil, NotInitializedError()
	}

	callParams, err := UnmarshalParamsRequired[ToolCallParams](params)
	if err != nil {
		return nil, err
	}

	result, toolErr := s.tools.Execute(callParams.Name, callParams.Arguments, session)
	if toolErr != nil {
		return result, nil // Tool errors are returned in the result, not as JSON-RPC errors
	}

	return result, nil
}

// handleResourcesList returns the list of available resources.
func (s *Server) handleResourcesList(session *MCPSession) (interface{}, *JSONRPCError) {
	if session.GetState() != SessionStateReady {
		return nil, NotInitializedError()
	}

	return &ResourcesListResult{
		Resources: s.resources.List(),
	}, nil
}

// handleResourcesRead reads a resource.
func (s *Server) handleResourcesRead(session *MCPSession, params json.RawMessage) (interface{}, *JSONRPCError) {
	if session.GetState() != SessionStateReady {
		return nil, NotInitializedError()
	}

	readParams, err := UnmarshalParamsRequired[ResourceReadParams](params)
	if err != nil {
		return nil, err
	}

	contents, readErr := s.resources.Read(readParams.URI)
	if readErr != nil {
		return nil, readErr
	}

	return &ResourceReadResult{
		Contents: contents,
	}, nil
}

// handleResourcesSubscribe subscribes to a resource.
func (s *Server) handleResourcesSubscribe(session *MCPSession, params json.RawMessage) (interface{}, *JSONRPCError) {
	if session.GetState() != SessionStateReady {
		return nil, NotInitializedError()
	}

	subParams, err := UnmarshalParamsRequired[ResourceSubscribeParams](params)
	if err != nil {
		return nil, err
	}

	session.Subscribe(subParams.URI)
	return map[string]interface{}{}, nil
}

// handleResourcesUnsubscribe unsubscribes from a resource.
func (s *Server) handleResourcesUnsubscribe(session *MCPSession, params json.RawMessage) (interface{}, *JSONRPCError) {
	if session.GetState() != SessionStateReady {
		return nil, NotInitializedError()
	}

	unsubParams, err := UnmarshalParamsRequired[ResourceUnsubscribeParams](params)
	if err != nil {
		return nil, err
	}

	session.Unsubscribe(unsubParams.URI)
	return map[string]interface{}{}, nil
}

// handleSSE handles SSE GET requests.
func (s *Server) handleSSE(w http.ResponseWriter, r *http.Request) {
	sessionID := r.Header.Get("Mcp-Session-Id")
	if sessionID == "" {
		http.Error(w, "Session required", http.StatusBadRequest)
		return
	}

	session := s.sessions.Get(sessionID)
	if session == nil {
		http.Error(w, "Session not found", http.StatusNotFound)
		return
	}

	if session.GetState() != SessionStateReady {
		http.Error(w, "Session not ready", http.StatusBadRequest)
		return
	}

	// Check for Flusher support
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming not supported", http.StatusInternalServerError)
		return
	}

	// Set SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	// Handle Last-Event-ID for resumption
	lastEventID := r.Header.Get("Last-Event-ID")
	_ = lastEventID // TODO: implement event replay from buffer

	// Event loop
	ctx := r.Context()
	eventID := int64(0)

	for {
		select {
		case <-ctx.Done():
			return

		case notif, ok := <-session.EventChannel:
			if !ok {
				return
			}

			eventID++
			data, _ := json.Marshal(notif)

			fmt.Fprintf(w, "id: %d\n", eventID)
			fmt.Fprintf(w, "event: message\n")
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
		}
	}
}

// handleSessionDelete handles session termination.
func (s *Server) handleSessionDelete(w http.ResponseWriter, r *http.Request) {
	sessionID := r.Header.Get("Mcp-Session-Id")
	if sessionID == "" {
		http.Error(w, "Session required", http.StatusBadRequest)
		return
	}

	s.sessions.Delete(sessionID)
	w.WriteHeader(http.StatusNoContent)
}

// writeError writes a JSON-RPC error response.
func (s *Server) writeError(w http.ResponseWriter, id interface{}, err *JSONRPCError) {
	resp := ErrorResponse(id, err)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK) // JSON-RPC errors are returned with 200 OK
	json.NewEncoder(w).Encode(resp)
}

// writeSuccess writes a JSON-RPC success response.
func (s *Server) writeSuccess(w http.ResponseWriter, id interface{}, result interface{}) {
	resp := SuccessResponse(id, result)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// NotifyResourceListChanged broadcasts a resource list changed notification.
func (s *Server) NotifyResourceListChanged() {
	s.sessions.Broadcast(ResourceListChangedNotification())
}

// NotifyResourceUpdated broadcasts a resource updated notification.
func (s *Server) NotifyResourceUpdated(uri string) {
	s.sessions.BroadcastToSubscribers(uri, ResourceUpdatedNotification(uri))
}

// AdminClient returns the admin client.
func (s *Server) AdminClient() cli.AdminClient {
	return s.adminClient
}

// StatefulStore returns the stateful store.
func (s *Server) StatefulStore() *stateful.StateStore {
	return s.statefulStore
}

// Sessions returns the session manager.
func (s *Server) Sessions() *SessionManager {
	return s.sessions
}

// SetLogger sets the operational logger for the server.
func (s *Server) SetLogger(log *slog.Logger) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if log != nil {
		s.log = log
	} else {
		s.log = logging.Nop()
	}
}
