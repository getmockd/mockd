package api

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/getmockd/mockd/pkg/config"
	"github.com/getmockd/mockd/pkg/logging"
	"github.com/getmockd/mockd/pkg/requestlog"
)

// Server is the Engine Control API server.
type Server struct {
	engine     EngineController
	httpServer *http.Server
	port       int
	log        *slog.Logger
}

// EngineController is the interface the API uses to control the engine.
// This is implemented by engine.Server.
type EngineController interface {
	// Status
	IsRunning() bool
	Uptime() int

	// Mocks
	AddMock(cfg *config.MockConfiguration) error
	UpdateMock(id string, cfg *config.MockConfiguration) error
	DeleteMock(id string) error
	GetMock(id string) *config.MockConfiguration
	ListMocks() []*config.MockConfiguration
	ClearMocks()

	// Request logs
	GetRequestLogs(filter *requestlog.Filter) []*requestlog.Entry
	GetRequestLog(id string) *requestlog.Entry
	RequestLogCount() int
	ClearRequestLogs()
	ClearRequestLogsByMockID(mockID string) int

	// Protocol status
	ProtocolStatus() map[string]ProtocolStatusInfo

	// Chaos injection
	GetChaosConfig() *ChaosConfig
	SetChaosConfig(cfg *ChaosConfig) error
	GetChaosStats() *ChaosStats
	ResetChaosStats()
	GetStatefulFaultStats() *StatefulFaultStats
	TripCircuitBreaker(key string) error
	ResetCircuitBreaker(key string) error

	// Stateful resources
	GetStateOverview() *StateOverview
	GetStateResource(name string) (*StatefulResource, error)
	ClearStateResource(name string) (int, error)
	ResetState(resourceName string) (*ResetStateResponse, error)
	RegisterStatefulResource(cfg *config.StatefulResourceConfig) error
	ListStatefulItems(name string, limit, offset int, sort, order string) (*StatefulItemsResponse, error)
	GetStatefulItem(resourceName, itemID string) (map[string]interface{}, error)
	CreateStatefulItem(resourceName string, data map[string]interface{}) (map[string]interface{}, error)

	// Custom operations
	ListCustomOperations() []CustomOperationInfo
	GetCustomOperation(name string) (*CustomOperationDetail, error)
	RegisterCustomOperation(cfg *config.CustomOperationConfig) error
	DeleteCustomOperation(name string) error
	ExecuteCustomOperation(name string, input map[string]interface{}) (map[string]interface{}, error)

	// Protocol handlers
	ListProtocolHandlers() []*ProtocolHandler
	GetProtocolHandler(id string) *ProtocolHandler

	// SSE connections
	ListSSEConnections() []*SSEConnection
	GetSSEConnection(id string) *SSEConnection
	CloseSSEConnection(id string) error
	GetSSEStats() *SSEStats

	// WebSocket connections
	ListWebSocketConnections() []*WebSocketConnection
	GetWebSocketConnection(id string) *WebSocketConnection
	CloseWebSocketConnection(id string) error
	GetWebSocketStats() *WebSocketStats

	// Config
	GetConfig() *ConfigResponse
}

// NewServer creates a new Engine Control API server.
//
// The server binds to 127.0.0.1 ONLY to prevent remote access. This is
// an internal API with NO authentication, NO authorization, and NO rate
// limiting. It is designed to be called exclusively by the Admin API via
// [engineclient.Client]. Do not expose this server on a public interface
// or call it directly from user-facing code.
func NewServer(engine EngineController, port int) *Server {
	s := &Server{
		engine: engine,
		port:   port,
		log:    logging.Nop(),
	}

	mux := http.NewServeMux()
	s.registerRoutes(mux)

	s.httpServer = &http.Server{
		Addr:         fmt.Sprintf("127.0.0.1:%d", port),
		Handler:      s.withMiddleware(mux),
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	return s
}

// SetLogger sets the logger.
func (s *Server) SetLogger(log *slog.Logger) {
	if log != nil {
		s.log = log
	}
}

// Start starts the control API server.
// Uses synchronous Listen to catch port-in-use errors immediately.
func (s *Server) Start() error {
	s.log.Info("starting engine control API", "port", s.port)

	ln, err := net.Listen("tcp", s.httpServer.Addr)
	if err != nil {
		return fmt.Errorf("failed to listen on engine control port %d: %w", s.port, err)
	}
	go func() {
		if err := s.httpServer.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			s.log.Error("control API server error", "error", err)
		}
	}()
	return nil
}

// Stop stops the control API server.
func (s *Server) Stop(ctx context.Context) error {
	return s.httpServer.Shutdown(ctx)
}

// Port returns the port the server is running on.
func (s *Server) Port() int {
	return s.port
}

func (s *Server) registerRoutes(mux *http.ServeMux) {
	// Health & Status
	mux.HandleFunc("GET /health", s.handleHealth)
	mux.HandleFunc("GET /status", s.handleStatus)

	// Deployment
	mux.HandleFunc("POST /deploy", s.handleDeploy)
	mux.HandleFunc("DELETE /deploy", s.handleUndeploy)

	// Mocks
	mux.HandleFunc("GET /mocks", s.handleListMocks)
	mux.HandleFunc("POST /mocks", s.handleCreateMock)
	mux.HandleFunc("GET /mocks/{id}", s.handleGetMock)
	mux.HandleFunc("PUT /mocks/{id}", s.handleUpdateMock)
	mux.HandleFunc("DELETE /mocks/{id}", s.handleDeleteMock)
	mux.HandleFunc("POST /mocks/{id}/toggle", s.handleToggleMock)

	// Request logs
	mux.HandleFunc("GET /requests", s.handleListRequests)
	mux.HandleFunc("GET /requests/{id}", s.handleGetRequest)
	mux.HandleFunc("DELETE /requests", s.handleClearRequests)
	mux.HandleFunc("DELETE /requests/mock/{id}", s.handleClearRequestsByMockID)

	// Protocols
	mux.HandleFunc("GET /protocols", s.handleListProtocols)

	// Chaos injection
	mux.HandleFunc("GET /chaos", s.handleGetChaos)
	mux.HandleFunc("PUT /chaos", s.handleSetChaos)
	mux.HandleFunc("GET /chaos/stats", s.handleGetChaosStats)
	mux.HandleFunc("POST /chaos/stats/reset", s.handleResetChaosStats)
	mux.HandleFunc("GET /chaos/faults", s.handleGetStatefulFaultStats)
	mux.HandleFunc("POST /chaos/circuit-breakers/{key}/trip", s.handleTripCircuitBreaker)
	mux.HandleFunc("POST /chaos/circuit-breakers/{key}/reset", s.handleResetCircuitBreaker)

	// State management
	mux.HandleFunc("GET /state", s.handleGetState)
	mux.HandleFunc("POST /state/reset", s.handleResetState)
	mux.HandleFunc("POST /state/resources", s.handleRegisterStatefulResource)
	mux.HandleFunc("GET /state/resources/{name}", s.handleGetStateResource)
	mux.HandleFunc("DELETE /state/resources/{name}", s.handleClearStateResource)
	mux.HandleFunc("GET /state/resources/{name}/items", s.handleListStatefulItems)
	mux.HandleFunc("GET /state/resources/{name}/items/{id}", s.handleGetStatefulItem)
	mux.HandleFunc("POST /state/resources/{name}/items", s.handleCreateStatefulItem)

	// Custom operations
	mux.HandleFunc("GET /state/operations", s.handleListCustomOperations)
	mux.HandleFunc("GET /state/operations/{name}", s.handleGetCustomOperation)
	mux.HandleFunc("POST /state/operations", s.handleRegisterCustomOperation)
	mux.HandleFunc("DELETE /state/operations/{name}", s.handleDeleteCustomOperation)
	mux.HandleFunc("POST /state/operations/{name}/execute", s.handleExecuteCustomOperation)

	// Protocol handlers
	mux.HandleFunc("GET /handlers", s.handleListHandlers)
	mux.HandleFunc("GET /handlers/{id}", s.handleGetHandler)

	// SSE connections
	mux.HandleFunc("GET /sse/connections", s.handleListSSEConnections)
	mux.HandleFunc("GET /sse/connections/{id}", s.handleGetSSEConnection)
	mux.HandleFunc("DELETE /sse/connections/{id}", s.handleCloseSSEConnection)
	mux.HandleFunc("GET /sse/stats", s.handleGetSSEStats)

	// WebSocket connections
	mux.HandleFunc("GET /websocket/connections", s.handleListWebSocketConnections)
	mux.HandleFunc("GET /websocket/connections/{id}", s.handleGetWebSocketConnection)
	mux.HandleFunc("DELETE /websocket/connections/{id}", s.handleCloseWebSocketConnection)
	mux.HandleFunc("GET /websocket/stats", s.handleGetWebSocketStats)

	// Config
	mux.HandleFunc("GET /config", s.handleGetConfig)
	mux.HandleFunc("POST /config", s.handleImportConfig)
	mux.HandleFunc("GET /export", s.handleExportMocks)
}

// withMiddleware wraps the handler with any necessary middleware.
// Note: Content-Type is now set per-response by httputil.WriteJSON,
// so this middleware only serves as an extension point.
func (s *Server) withMiddleware(handler http.Handler) http.Handler {
	return handler
}
