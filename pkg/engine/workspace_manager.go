// Package engine provides the core mock server engine.
package engine

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/getmockd/mockd/internal/id"
	"github.com/getmockd/mockd/internal/storage"
	"github.com/getmockd/mockd/pkg/config"
	"github.com/getmockd/mockd/pkg/logging"
	"github.com/getmockd/mockd/pkg/mock"
	"github.com/getmockd/mockd/pkg/store"
	"github.com/getmockd/mockd/pkg/workspace"
)

// WorkspaceServerStatus is an alias for the shared workspace.ServerStatus type.
// Kept for backward compatibility with existing consumers within pkg/engine.
type WorkspaceServerStatus = workspace.ServerStatus

const (
	WorkspaceServerStatusStopped  = workspace.ServerStatusStopped
	WorkspaceServerStatusRunning  = workspace.ServerStatusRunning
	WorkspaceServerStatusStarting = workspace.ServerStatusStarting
	WorkspaceServerStatusError    = workspace.ServerStatusError
)

// WorkspaceServer represents an isolated HTTP server for a single workspace.
type WorkspaceServer struct {
	WorkspaceID   string
	WorkspaceName string
	HTTPPort      int
	GRPCPort      int
	MQTTPort      int

	status     WorkspaceServerStatus
	statusMsg  string
	httpServer *http.Server
	handler    *Handler
	store      storage.MockStore
	logger     RequestLogger
	log        *slog.Logger
	startTime  time.Time
	mu         sync.RWMutex

	// Reference to parent manager for fetching mocks
	manager *WorkspaceManager
}

// WorkspaceManager manages multiple workspace servers.
type WorkspaceManager struct {
	workspaces map[string]*WorkspaceServer // workspaceID -> server
	mu         sync.RWMutex
	log        *slog.Logger

	// Mock source - where to get mocks for workspaces
	// In admin mode, this could be backed by a persistent store per workspace
	// For now, we use a central mock store and filter by workspace
	centralStore storage.MockStore

	// Callback to fetch mocks for a workspace (set by admin API)
	mockFetcher WorkspaceMockFetcher

	// Configuration
	defaultReadTimeout  time.Duration
	defaultWriteTimeout time.Duration
	maxLogEntries       int
}

// WorkspaceMockFetcher is an alias for the shared workspace.MockFetcher type.
type WorkspaceMockFetcher = workspace.MockFetcher

// WorkspaceManagerConfig is an alias for the shared workspace.ManagerConfig type.
type WorkspaceManagerConfig = workspace.ManagerConfig

// DefaultWorkspaceManagerConfig returns sensible defaults.
func DefaultWorkspaceManagerConfig() *WorkspaceManagerConfig {
	return workspace.DefaultManagerConfig()
}

// NewWorkspaceManager creates a new workspace manager.
func NewWorkspaceManager(cfg *WorkspaceManagerConfig) *WorkspaceManager {
	if cfg == nil {
		cfg = DefaultWorkspaceManagerConfig()
	}

	return &WorkspaceManager{
		workspaces:          make(map[string]*WorkspaceServer),
		log:                 logging.Nop(),
		defaultReadTimeout:  cfg.DefaultReadTimeout,
		defaultWriteTimeout: cfg.DefaultWriteTimeout,
		maxLogEntries:       cfg.MaxLogEntries,
	}
}

// SetLogger sets the operational logger for the workspace manager.
func (m *WorkspaceManager) SetLogger(log *slog.Logger) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if log != nil {
		m.log = log
	} else {
		m.log = logging.Nop()
	}
}

// SetMockFetcher sets the function used to fetch mocks for workspaces.
func (m *WorkspaceManager) SetMockFetcher(fetcher WorkspaceMockFetcher) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.mockFetcher = fetcher
}

// SetCentralStore sets the central mock store (used for filtering).
func (m *WorkspaceManager) SetCentralStore(store storage.MockStore) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.centralStore = store
}

// StartWorkspace creates and starts a server for the given workspace.
func (m *WorkspaceManager) StartWorkspace(ctx context.Context, ws *store.EngineWorkspace) error {
	if ws == nil {
		return errors.New("workspace is required")
	}
	if strings.TrimSpace(ws.WorkspaceID) == "" {
		return errors.New("workspace ID is required")
	}
	if ws.HTTPPort <= 0 || ws.HTTPPort > 65535 {
		return fmt.Errorf("invalid HTTP port %d for workspace %s", ws.HTTPPort, ws.WorkspaceID)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if already running
	if existing, ok := m.workspaces[ws.WorkspaceID]; ok {
		if existing.Status() == WorkspaceServerStatusRunning {
			return fmt.Errorf("workspace %s is already running on port %d", ws.WorkspaceID, existing.HTTPPort)
		}
		// Stop existing server before restarting
		_ = existing.Stop()
	}

	// Create workspace server
	server := &WorkspaceServer{
		WorkspaceID:   ws.WorkspaceID,
		WorkspaceName: ws.WorkspaceName,
		HTTPPort:      ws.HTTPPort,
		GRPCPort:      ws.GRPCPort,
		MQTTPort:      ws.MQTTPort,
		status:        WorkspaceServerStatusStopped,
		log:           m.log,
		manager:       m,
	}

	// Initialize the server
	if err := server.init(m.maxLogEntries); err != nil {
		return fmt.Errorf("failed to initialize workspace server: %w", err)
	}

	// Load mocks for this workspace
	if err := server.loadMocks(ctx); err != nil {
		return fmt.Errorf("failed to load mocks for workspace: %w", err)
	}

	// Start the server
	if err := server.Start(); err != nil {
		return fmt.Errorf("failed to start workspace server: %w", err)
	}

	m.workspaces[ws.WorkspaceID] = server
	return nil
}

// StopWorkspace stops the server for the given workspace.
func (m *WorkspaceManager) StopWorkspace(workspaceID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	server, ok := m.workspaces[workspaceID]
	if !ok {
		return fmt.Errorf("workspace %s not found", workspaceID)
	}

	if err := server.Stop(); err != nil {
		return fmt.Errorf("failed to stop workspace server: %w", err)
	}

	return nil
}

// RemoveWorkspace stops and removes the server for the given workspace.
func (m *WorkspaceManager) RemoveWorkspace(workspaceID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	server, ok := m.workspaces[workspaceID]
	if !ok {
		return nil // Not an error if not found
	}

	if server.Status() == WorkspaceServerStatusRunning {
		_ = server.Stop()
	}

	delete(m.workspaces, workspaceID)
	return nil
}

// GetWorkspace returns the workspace server for the given ID.
func (m *WorkspaceManager) GetWorkspace(workspaceID string) workspace.Server {
	m.mu.RLock()
	defer m.mu.RUnlock()
	server, ok := m.workspaces[workspaceID]
	if !ok {
		return nil
	}
	return server
}

// ListWorkspaces returns all workspace servers.
func (m *WorkspaceManager) ListWorkspaces() []workspace.Server {
	m.mu.RLock()
	defer m.mu.RUnlock()

	servers := make([]workspace.Server, 0, len(m.workspaces))
	for _, server := range m.workspaces {
		servers = append(servers, server)
	}
	return servers
}

// ReloadWorkspace reloads mocks for a workspace from the source.
func (m *WorkspaceManager) ReloadWorkspace(ctx context.Context, workspaceID string) error {
	m.mu.RLock()
	server, ok := m.workspaces[workspaceID]
	m.mu.RUnlock()

	if !ok {
		return fmt.Errorf("workspace %s not found", workspaceID)
	}

	return server.loadMocks(ctx)
}

// StopAll stops all workspace servers.
func (m *WorkspaceManager) StopAll() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	var errs []error
	for _, server := range m.workspaces {
		if err := server.Stop(); err != nil {
			errs = append(errs, err)
		}
	}

	return errors.Join(errs...)
}

// GetWorkspaceStatus returns detailed status info for a workspace.
func (m *WorkspaceManager) GetWorkspaceStatus(workspaceID string) *WorkspaceStatusInfo {
	m.mu.RLock()
	server, ok := m.workspaces[workspaceID]
	m.mu.RUnlock()

	if !ok {
		return nil
	}

	return server.StatusInfo()
}

// WorkspaceStatusInfo is an alias for the shared workspace.StatusInfo type.
type WorkspaceStatusInfo = workspace.StatusInfo

// --- WorkspaceServer methods ---

// init initializes the workspace server's internal components.
func (s *WorkspaceServer) init(maxLogEntries int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Use filtered store if we have a central store - provides live view of mocks
	// Otherwise fall back to isolated in-memory store
	if s.manager != nil && s.manager.centralStore != nil {
		s.store = storage.NewFilteredMockStore(s.manager.centralStore, s.WorkspaceID)
	} else {
		s.store = storage.NewInMemoryMockStore()
	}

	// Create handler with the workspace store
	s.handler = NewHandler(s.store)

	// Create request logger
	if maxLogEntries <= 0 {
		maxLogEntries = 1000
	}
	s.logger = NewInMemoryRequestLogger(maxLogEntries)
	s.handler.SetLogger(s.logger)

	return nil
}

// loadMocks loads mocks for this workspace from the manager's source.
func (s *WorkspaceServer) loadMocks(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.manager == nil {
		return nil
	}

	// Try mock fetcher first
	if s.manager.mockFetcher != nil {
		cfgs, err := s.manager.mockFetcher(ctx, s.WorkspaceID)
		if err != nil {
			return err
		}

		// Clear and reload
		s.store.Clear()
		for _, cfg := range cfgs {
			if cfg != nil && (cfg.Enabled == nil || *cfg.Enabled) {
				// MockConfiguration is now an alias for mock.Mock
				_ = s.store.Set(cfg)
			}
		}
		return nil
	}

	// Fallback: filter from central store by workspace ID
	if s.manager.centralStore != nil {
		allMocks := s.manager.centralStore.List()
		s.store.Clear()
		for _, m := range allMocks {
			if m != nil && (m.Enabled == nil || *m.Enabled) && m.WorkspaceID == s.WorkspaceID {
				_ = s.store.Set(m)
			}
		}
	}

	return nil
}

// Start starts the HTTP server for this workspace.
// It binds the listening socket synchronously so that when Start returns
// successfully the port is confirmed open and the server is truly running.
func (s *WorkspaceServer) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.status == WorkspaceServerStatusRunning {
		return errors.New("server is already running")
	}

	s.status = WorkspaceServerStatusStarting

	// Use manager's configured timeouts, falling back to 30s defaults.
	readTimeout := 30 * time.Second
	writeTimeout := 30 * time.Second
	if s.manager != nil {
		if s.manager.defaultReadTimeout > 0 {
			readTimeout = s.manager.defaultReadTimeout
		}
		if s.manager.defaultWriteTimeout > 0 {
			writeTimeout = s.manager.defaultWriteTimeout
		}
	}

	// Create HTTP server
	s.httpServer = &http.Server{
		Addr:         fmt.Sprintf(":%d", s.HTTPPort),
		Handler:      s.handler,
		ReadTimeout:  readTimeout,
		WriteTimeout: writeTimeout,
	}

	// Bind the listener synchronously so we know the port is available
	// before reporting Running status.
	ln, err := net.Listen("tcp", s.httpServer.Addr)
	if err != nil {
		s.status = WorkspaceServerStatusError
		s.statusMsg = err.Error()
		return fmt.Errorf("failed to listen on port %d: %w", s.HTTPPort, err)
	}

	// Serve in background using the already-bound listener
	go func() {
		if err := s.httpServer.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			s.mu.Lock()
			s.status = WorkspaceServerStatusError
			s.statusMsg = err.Error()
			s.mu.Unlock()
			s.log.Error("workspace server error", "workspaceId", s.WorkspaceID, "port", s.HTTPPort, "error", err)
		}
	}()

	s.status = WorkspaceServerStatusRunning
	s.startTime = time.Now()
	s.statusMsg = ""

	return nil
}

// Stop gracefully shuts down the HTTP server.
func (s *WorkspaceServer) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.status != WorkspaceServerStatusRunning && s.status != WorkspaceServerStatusStarting {
		return nil
	}

	if s.httpServer != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := s.httpServer.Shutdown(ctx); err != nil {
			s.status = WorkspaceServerStatusError
			s.statusMsg = err.Error()
			return err
		}
	}

	s.status = WorkspaceServerStatusStopped
	s.statusMsg = ""
	return nil
}

// Status returns the current status of the server.
func (s *WorkspaceServer) Status() WorkspaceServerStatus {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.status
}

// StatusInfo returns detailed status information.
func (s *WorkspaceServer) StatusInfo() *WorkspaceStatusInfo {
	s.mu.RLock()
	defer s.mu.RUnlock()

	uptime := 0
	if s.status == WorkspaceServerStatusRunning && !s.startTime.IsZero() {
		uptime = int(time.Since(s.startTime).Seconds())
	}

	mockCount := 0
	if s.store != nil {
		mockCount = s.store.Count()
	}

	requestCount := 0
	if s.logger != nil {
		requestCount = s.logger.Count()
	}

	return &WorkspaceStatusInfo{
		WorkspaceID:   s.WorkspaceID,
		WorkspaceName: s.WorkspaceName,
		HTTPPort:      s.HTTPPort,
		GRPCPort:      s.GRPCPort,
		MQTTPort:      s.MQTTPort,
		Status:        s.status,
		StatusMessage: s.statusMsg,
		MockCount:     mockCount,
		RequestCount:  requestCount,
		Uptime:        uptime,
	}
}

// Handler returns the HTTP handler for this workspace.
func (s *WorkspaceServer) Handler() *Handler {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.handler
}

// Store returns the mock store for this workspace.
func (s *WorkspaceServer) Store() storage.MockStore {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.store
}

// Logger returns the request logger for this workspace.
func (s *WorkspaceServer) Logger() RequestLogger {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.logger
}

// AddMock adds a mock to this workspace's store.
func (s *WorkspaceServer) AddMock(cfg *config.MockConfiguration) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.store == nil {
		return errors.New("workspace server not initialized")
	}

	// Ensure mock has workspace ID set
	cfg.WorkspaceID = s.WorkspaceID

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

	// Validate
	if err := cfg.Validate(); err != nil {
		return err
	}

	// Store directly (MockConfiguration is now an alias for mock.Mock)
	return s.store.Set(cfg)
}

// DeleteMock removes a mock from this workspace's store.
func (s *WorkspaceServer) DeleteMock(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.store == nil {
		return errors.New("workspace server not initialized")
	}

	if !s.store.Delete(id) {
		return fmt.Errorf("mock with ID %s not found", id)
	}
	return nil
}

// ListMocks returns all HTTP mocks in this workspace.
func (s *WorkspaceServer) ListMocks() []*config.MockConfiguration {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.store == nil {
		return nil
	}
	return s.store.ListByType(mock.TypeHTTP)
}

// GetMock retrieves a mock by ID.
func (s *WorkspaceServer) GetMock(id string) *config.MockConfiguration {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.store == nil {
		return nil
	}
	return s.store.Get(id)
}

// ClearMocks removes all mocks from this workspace.
func (s *WorkspaceServer) ClearMocks() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.store != nil {
		s.store.Clear()
	}
}
