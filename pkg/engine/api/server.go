package api

import (
	"context"
	"fmt"
	"log/slog"
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
	GetRequestLogs(filter *RequestLogFilter) []*requestlog.Entry
	GetRequestLog(id string) *requestlog.Entry
	RequestLogCount() int
	ClearRequestLogs()

	// Protocol status
	ProtocolStatus() map[string]ProtocolStatusInfo

	// Chaos injection
	GetChaosConfig() *ChaosConfig
	SetChaosConfig(cfg *ChaosConfig) error
	GetChaosStats() *ChaosStats
	ResetChaosStats()

	// Stateful resources
	GetStateOverview() *StateOverview
	GetStateResource(name string) (*StatefulResource, error)
	ClearStateResource(name string) (int, error)
	ResetState(resourceName string) (*ResetStateResponse, error)

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
func NewServer(engine EngineController, port int) *Server {
	s := &Server{
		engine: engine,
		port:   port,
		log:    logging.Nop(),
	}

	mux := http.NewServeMux()
	s.registerRoutes(mux)

	s.httpServer = &http.Server{
		Addr:         fmt.Sprintf(":%d", port),
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
func (s *Server) Start() error {
	s.log.Info("starting engine control API", "port", s.port)
	go func() {
		if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
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

	// Protocols
	mux.HandleFunc("GET /protocols", s.handleListProtocols)

	// Chaos injection
	mux.HandleFunc("GET /chaos", s.handleGetChaos)
	mux.HandleFunc("PUT /chaos", s.handleSetChaos)
	mux.HandleFunc("GET /chaos/stats", s.handleGetChaosStats)
	mux.HandleFunc("POST /chaos/stats/reset", s.handleResetChaosStats)

	// State management
	mux.HandleFunc("GET /state", s.handleGetState)
	mux.HandleFunc("POST /state/reset", s.handleResetState)
	mux.HandleFunc("GET /state/resources/{name}", s.handleGetStateResource)
	mux.HandleFunc("DELETE /state/resources/{name}", s.handleClearStateResource)

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

func (s *Server) withMiddleware(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		handler.ServeHTTP(w, r)
	})
}
