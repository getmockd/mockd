// Package admin provides a REST API for managing mock configurations.
package admin

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/getmockd/mockd/pkg/engine"
)

// AdminAPI exposes a REST API for managing mock configurations.
type AdminAPI struct {
	server                 *engine.Server
	proxyManager           *ProxyManager
	streamRecordingManager *StreamRecordingManager
	httpServer             *http.Server
	port                   int
	startTime              time.Time
}

// NewAdminAPI creates a new AdminAPI.
func NewAdminAPI(server *engine.Server, port int) *AdminAPI {
	api := &AdminAPI{
		server:                 server,
		proxyManager:           NewProxyManager(),
		streamRecordingManager: NewStreamRecordingManager(),
		port:                   port,
	}

	mux := http.NewServeMux()
	api.registerRoutes(mux)

	api.httpServer = &http.Server{
		Addr:         fmt.Sprintf(":%d", port),
		Handler:      api.withMiddleware(mux),
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	return api
}

// InitializeStreamRecordings initializes the stream recording manager with the given data directory.
func (a *AdminAPI) InitializeStreamRecordings(dataDir string) error {
	return a.streamRecordingManager.Initialize(dataDir)
}

// StreamRecordingManager returns the stream recording manager.
func (a *AdminAPI) StreamRecordingManager() *StreamRecordingManager {
	return a.streamRecordingManager
}

// registerRoutes sets up all API routes.
func (a *AdminAPI) registerRoutes(mux *http.ServeMux) {
	// Health check
	mux.HandleFunc("GET /health", a.handleHealth)

	// Mock CRUD operations
	mux.HandleFunc("GET /mocks", a.handleListMocks)
	mux.HandleFunc("POST /mocks", a.handleCreateMock)
	mux.HandleFunc("GET /mocks/{id}", a.handleGetMock)
	mux.HandleFunc("PUT /mocks/{id}", a.handleUpdateMock)
	mux.HandleFunc("DELETE /mocks/{id}", a.handleDeleteMock)
	mux.HandleFunc("POST /mocks/{id}/toggle", a.handleToggleMock)

	// Configuration import/export
	mux.HandleFunc("GET /config", a.handleExportConfig)
	mux.HandleFunc("POST /config", a.handleImportConfig)

	// Request logging
	mux.HandleFunc("GET /requests", a.handleListRequests)
	mux.HandleFunc("GET /requests/{id}", a.handleGetRequest)
	mux.HandleFunc("DELETE /requests", a.handleClearRequests)

	// Proxy management
	mux.HandleFunc("POST /proxy/start", a.proxyManager.handleProxyStart)
	mux.HandleFunc("POST /proxy/stop", a.proxyManager.handleProxyStop)
	mux.HandleFunc("GET /proxy/status", a.proxyManager.handleProxyStatus)
	mux.HandleFunc("PUT /proxy/mode", a.proxyManager.handleProxyMode)
	mux.HandleFunc("GET /proxy/filters", a.proxyManager.handleGetFilters)
	mux.HandleFunc("PUT /proxy/filters", a.proxyManager.handleSetFilters)
	mux.HandleFunc("GET /proxy/ca", a.proxyManager.handleGetCA)
	mux.HandleFunc("POST /proxy/ca", a.proxyManager.handleGenerateCA)
	mux.HandleFunc("GET /proxy/ca/download", a.proxyManager.handleDownloadCA)

	// Recording management
	mux.HandleFunc("GET /recordings", a.proxyManager.handleListRecordings)
	mux.HandleFunc("DELETE /recordings", a.proxyManager.handleClearRecordings)
	mux.HandleFunc("GET /recordings/{id}", a.proxyManager.handleGetRecording)
	mux.HandleFunc("DELETE /recordings/{id}", a.proxyManager.handleDeleteRecording)
	mux.HandleFunc("POST /recordings/convert", a.handleConvertRecordings)
	mux.HandleFunc("POST /recordings/export", a.proxyManager.handleExportRecordings)

	// Session management
	mux.HandleFunc("GET /sessions", a.proxyManager.handleListSessions)
	mux.HandleFunc("POST /sessions", a.proxyManager.handleCreateSession)
	mux.HandleFunc("DELETE /sessions", a.proxyManager.handleDeleteSessions)
	mux.HandleFunc("GET /sessions/{id}", a.proxyManager.handleGetSession)
	mux.HandleFunc("DELETE /sessions/{id}", a.proxyManager.handleDeleteSession)

	// State management (stateful resources)
	mux.HandleFunc("GET /state", a.handleStateOverview)
	mux.HandleFunc("POST /state/reset", a.handleStateReset)
	mux.HandleFunc("GET /state/resources", a.handleListStateResources)
	mux.HandleFunc("GET /state/resources/{name}", a.handleGetStateResource)
	mux.HandleFunc("DELETE /state/resources/{name}", a.handleClearStateResource)

	// SSE connection management
	mux.HandleFunc("GET /sse/connections", a.handleListSSEConnections)
	mux.HandleFunc("GET /sse/connections/{id}", a.handleGetSSEConnection)
	mux.HandleFunc("DELETE /sse/connections/{id}", a.handleCloseSSEConnection)
	mux.HandleFunc("GET /sse/stats", a.handleGetSSEStats)

	// Mock-specific SSE endpoints
	mux.HandleFunc("GET /mocks/{id}/sse/connections", a.handleListMockSSEConnections)
	mux.HandleFunc("DELETE /mocks/{id}/sse/connections", a.handleCloseMockSSEConnections)
	mux.HandleFunc("GET /mocks/{id}/sse/buffer", a.handleGetMockSSEBuffer)
	mux.HandleFunc("DELETE /mocks/{id}/sse/buffer", a.handleClearMockSSEBuffer)

	// WebSocket connection management
	mux.HandleFunc("GET /admin/ws/connections", a.handleListWSConnections)
	mux.HandleFunc("GET /admin/ws/connections/{id}", a.handleGetWSConnection)
	mux.HandleFunc("DELETE /admin/ws/connections/{id}", a.handleDisconnectWS)
	mux.HandleFunc("POST /admin/ws/connections/{id}/send", a.handleSendWSMessage)
	mux.HandleFunc("POST /admin/ws/connections/{id}/groups", a.handleJoinWSGroup)
	mux.HandleFunc("DELETE /admin/ws/connections/{id}/groups", a.handleLeaveWSGroup)

	// WebSocket endpoint management
	mux.HandleFunc("GET /admin/ws/endpoints", a.handleListWSEndpoints)
	mux.HandleFunc("GET /admin/ws/endpoints/{path...}", a.handleGetWSEndpoint)

	// WebSocket broadcast
	mux.HandleFunc("POST /admin/ws/broadcast", a.handleWSBroadcast)

	// WebSocket statistics
	mux.HandleFunc("GET /admin/ws/stats", a.handleWSStats)

	// Stream recording management (WebSocket/SSE)
	mux.HandleFunc("GET /stream-recordings", a.streamRecordingManager.handleListStreamRecordings)
	mux.HandleFunc("GET /stream-recordings/stats", a.streamRecordingManager.handleGetStreamRecordingStats)
	mux.HandleFunc("GET /stream-recordings/sessions", a.streamRecordingManager.handleGetActiveSessions)
	mux.HandleFunc("POST /stream-recordings/start", a.streamRecordingManager.handleStartRecording)
	mux.HandleFunc("POST /stream-recordings/vacuum", a.streamRecordingManager.handleVacuum)
	mux.HandleFunc("GET /stream-recordings/{id}", a.streamRecordingManager.handleGetStreamRecording)
	mux.HandleFunc("DELETE /stream-recordings/{id}", a.streamRecordingManager.handleDeleteStreamRecording)
	mux.HandleFunc("POST /stream-recordings/{id}/stop", a.streamRecordingManager.handleStopRecording)
	mux.HandleFunc("POST /stream-recordings/{id}/export", a.streamRecordingManager.handleExportStreamRecording)
	mux.HandleFunc("POST /stream-recordings/{id}/convert", a.streamRecordingManager.handleConvertStreamRecording)
	mux.HandleFunc("POST /stream-recordings/{id}/replay", a.streamRecordingManager.handleStartReplay)

	// Replay session management
	mux.HandleFunc("GET /replay", a.streamRecordingManager.handleListReplaySessions)
	mux.HandleFunc("GET /replay/{id}", a.streamRecordingManager.handleGetReplayStatus)
	mux.HandleFunc("DELETE /replay/{id}", a.streamRecordingManager.handleStopReplay)
	mux.HandleFunc("POST /replay/{id}/advance", a.streamRecordingManager.handleAdvanceReplay)
}

// handleConvertRecordings wraps the convert handler to pass the server.
func (a *AdminAPI) handleConvertRecordings(w http.ResponseWriter, r *http.Request) {
	a.proxyManager.handleConvertRecordings(w, r, nil)
}

// withMiddleware wraps the handler with logging and CORS middleware.
func (a *AdminAPI) withMiddleware(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Add CORS headers
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		// Handle preflight requests
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		handler.ServeHTTP(w, r)
	})
}

// Start starts the admin API server.
func (a *AdminAPI) Start() error {
	a.startTime = time.Now()
	go func() {
		if err := a.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fmt.Printf("Admin API error: %v\n", err)
		}
	}()
	return nil
}

// Stop gracefully shuts down the admin API server.
func (a *AdminAPI) Stop() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return a.httpServer.Shutdown(ctx)
}

// Uptime returns the API uptime in seconds.
func (a *AdminAPI) Uptime() int {
	return int(time.Since(a.startTime).Seconds())
}
