// Package engine provides the core mock server engine.
package engine

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/getmockd/mockd/internal/storage"
	"github.com/getmockd/mockd/pkg/config"
	"github.com/getmockd/mockd/pkg/stateful"
	mockdtls "github.com/getmockd/mockd/pkg/tls"
)

// Server is the main mock server engine.
type Server struct {
	cfg           *config.ServerConfiguration
	store         storage.MockStore
	statefulStore *stateful.StateStore
	logger        RequestLogger
	httpServer    *http.Server
	httpsServer   *http.Server
	handler       *Handler
	tlsConfig     *tls.Config
	mu            sync.RWMutex
	running       bool
	startTime     time.Time
}

// NewServer creates a new Server with the given configuration.
func NewServer(cfg *config.ServerConfiguration) *Server {
	if cfg == nil {
		cfg = config.DefaultServerConfiguration()
	}

	store := storage.NewInMemoryMockStore()
	statefulStore := stateful.NewStateStore()
	handler := NewHandler(store)
	handler.SetStatefulStore(statefulStore)

	maxLogEntries := cfg.MaxLogEntries
	if maxLogEntries <= 0 {
		maxLogEntries = 1000 // Default
	}
	logger := NewInMemoryRequestLogger(maxLogEntries)
	handler.SetLogger(logger)

	return &Server{
		cfg:           cfg,
		store:         store,
		statefulStore: statefulStore,
		logger:        logger,
		handler:       handler,
	}
}

// NewServerWithMocks creates a new Server with pre-loaded mocks.
func NewServerWithMocks(cfg *config.ServerConfiguration, mocks []*config.MockConfiguration) *Server {
	srv := NewServer(cfg)
	for _, mock := range mocks {
		if mock != nil {
			_ = srv.AddMock(mock)
		}
	}
	return srv
}

// Start starts the HTTP and HTTPS servers.
func (s *Server) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.running {
		return fmt.Errorf("server is already running")
	}

	// Start HTTP server if configured
	if s.cfg.HTTPPort > 0 {
		s.httpServer = &http.Server{
			Addr:         fmt.Sprintf(":%d", s.cfg.HTTPPort),
			Handler:      s.handler,
			ReadTimeout:  time.Duration(s.cfg.ReadTimeout) * time.Second,
			WriteTimeout: time.Duration(s.cfg.WriteTimeout) * time.Second,
		}

		go func() {
			if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				// Log error but don't crash
				fmt.Printf("HTTP server error: %v\n", err)
			}
		}()
	}

	// Start HTTPS server if configured
	if s.cfg.HTTPSPort > 0 {
		var err error
		s.tlsConfig, err = s.setupTLS()
		if err != nil {
			return fmt.Errorf("failed to setup TLS: %w", err)
		}

		s.httpsServer = &http.Server{
			Addr:         fmt.Sprintf(":%d", s.cfg.HTTPSPort),
			Handler:      s.handler,
			TLSConfig:    s.tlsConfig,
			ReadTimeout:  time.Duration(s.cfg.ReadTimeout) * time.Second,
			WriteTimeout: time.Duration(s.cfg.WriteTimeout) * time.Second,
		}

		go func() {
			if err := s.httpsServer.ListenAndServeTLS("", ""); err != nil && err != http.ErrServerClosed {
				// Log error but don't crash
				fmt.Printf("HTTPS server error: %v\n", err)
			}
		}()
	}

	s.running = true
	s.startTime = time.Now()
	return nil
}

// Stop gracefully shuts down the server.
func (s *Server) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var errs []error

	if s.httpServer != nil {
		if err := s.httpServer.Shutdown(ctx); err != nil {
			errs = append(errs, fmt.Errorf("HTTP shutdown: %w", err))
		}
	}

	if s.httpsServer != nil {
		if err := s.httpsServer.Shutdown(ctx); err != nil {
			errs = append(errs, fmt.Errorf("HTTPS shutdown: %w", err))
		}
	}

	s.running = false

	if len(errs) > 0 {
		return errs[0]
	}
	return nil
}

// setupTLS configures TLS for HTTPS.
func (s *Server) setupTLS() (*tls.Config, error) {
	var tlsCert tls.Certificate
	var err error

	// If auto-generate is enabled, generate a self-signed certificate
	if s.cfg.AutoGenerateCert {
		genCert, genErr := mockdtls.GenerateSelfSignedCert(mockdtls.DefaultCertificateConfig())
		if genErr != nil {
			return nil, fmt.Errorf("failed to generate certificate: %w", genErr)
		}

		tlsCert, err = mockdtls.CreateTLSCertificate(genCert.CertPEM, genCert.KeyPEM)
		if err != nil {
			return nil, fmt.Errorf("failed to create TLS certificate: %w", err)
		}
	} else {
		// Load certificate from files
		tlsCert, err = tls.LoadX509KeyPair(s.cfg.CertFile, s.cfg.KeyFile)
		if err != nil {
			return nil, fmt.Errorf("failed to load certificate: %w", err)
		}
	}

	return &tls.Config{
		Certificates: []tls.Certificate{tlsCert},
		MinVersion:   tls.VersionTLS12,
	}, nil
}

// AddMock adds a new mock configuration to the server.
func (s *Server) AddMock(mock *config.MockConfiguration) error {
	if mock == nil {
		return fmt.Errorf("mock cannot be nil")
	}

	// Generate ID if not provided
	if mock.ID == "" {
		mock.ID = generateID()
	}

	// Set timestamps
	now := time.Now()
	if mock.CreatedAt.IsZero() {
		mock.CreatedAt = now
	}
	mock.UpdatedAt = now

	// Set default enabled state
	if !mock.Enabled {
		mock.Enabled = true
	}

	// Validate
	if err := mock.Validate(); err != nil {
		return err
	}

	// Check for duplicate ID
	if s.store.Exists(mock.ID) {
		return fmt.Errorf("mock with ID %s already exists", mock.ID)
	}

	return s.store.Set(mock)
}

// UpdateMock updates an existing mock configuration.
func (s *Server) UpdateMock(id string, mock *config.MockConfiguration) error {
	if mock == nil {
		return fmt.Errorf("mock cannot be nil")
	}

	existing := s.store.Get(id)
	if existing == nil {
		return fmt.Errorf("mock with ID %s not found", id)
	}

	// Preserve ID and creation time
	mock.ID = id
	mock.CreatedAt = existing.CreatedAt
	mock.UpdatedAt = time.Now()

	// Validate
	if err := mock.Validate(); err != nil {
		return err
	}

	return s.store.Set(mock)
}

// DeleteMock removes a mock configuration.
func (s *Server) DeleteMock(id string) error {
	if !s.store.Delete(id) {
		return fmt.Errorf("mock with ID %s not found", id)
	}
	return nil
}

// GetMock retrieves a mock by ID.
func (s *Server) GetMock(id string) *config.MockConfiguration {
	return s.store.Get(id)
}

// ListMocks returns all mock configurations.
func (s *Server) ListMocks() []*config.MockConfiguration {
	return s.store.List()
}

// IsRunning returns whether the server is running.
func (s *Server) IsRunning() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.running
}

// Uptime returns the server uptime in seconds.
func (s *Server) Uptime() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if !s.running {
		return 0
	}
	return int(time.Since(s.startTime).Seconds())
}

// Config returns the server configuration.
func (s *Server) Config() *config.ServerConfiguration {
	return s.cfg
}

// Store returns the mock store (for admin API use).
func (s *Server) Store() storage.MockStore {
	return s.store
}

// SaveConfig saves the current mock configurations to a file.
func (s *Server) SaveConfig(path string, name string) error {
	mocks := s.store.List()
	return config.SaveMocksToFile(path, mocks, name)
}

// LoadConfig loads mock configurations from a file and adds them to the server.
// If replace is true, existing mocks are cleared first.
func (s *Server) LoadConfig(path string, replace bool) error {
	collection, err := config.LoadFromFile(path)
	if err != nil {
		return err
	}

	if replace {
		s.store.Clear()
	}

	// Load regular mocks
	for _, mock := range collection.Mocks {
		if mock != nil {
			if err := s.AddMock(mock); err != nil {
				// If not replacing and mock exists, skip it
				if !replace && s.store.Exists(mock.ID) {
					continue
				}
				return err
			}
		}
	}

	// Load stateful resources
	for _, res := range collection.StatefulResources {
		if res != nil {
			if err := s.RegisterStatefulResource(res); err != nil {
				return fmt.Errorf("failed to register stateful resource %s: %w", res.Name, err)
			}
		}
	}

	return nil
}

// ExportConfig exports the current configuration as a MockCollection.
func (s *Server) ExportConfig(name string) *config.MockCollection {
	return &config.MockCollection{
		Version: "1.0",
		Name:    name,
		Mocks:   s.store.List(),
	}
}

// ImportConfig imports a MockCollection, optionally replacing existing mocks.
func (s *Server) ImportConfig(collection *config.MockCollection, replace bool) error {
	if collection == nil {
		return fmt.Errorf("collection cannot be nil")
	}

	if err := collection.Validate(); err != nil {
		return err
	}

	if replace {
		s.store.Clear()
	}

	for _, mock := range collection.Mocks {
		if mock != nil {
			// Skip if not replacing and mock already exists
			if !replace && s.store.Exists(mock.ID) {
				continue
			}
			if err := s.store.Set(mock); err != nil {
				return err
			}
		}
	}

	return nil
}

// GetRequestLogs returns request logs, optionally filtered.
func (s *Server) GetRequestLogs(filter *RequestLogFilter) []*config.RequestLogEntry {
	if s.logger == nil {
		return nil
	}
	return s.logger.List(filter)
}

// GetRequestLog returns a single request log by ID.
func (s *Server) GetRequestLog(id string) *config.RequestLogEntry {
	if s.logger == nil {
		return nil
	}
	return s.logger.Get(id)
}

// ClearRequestLogs clears all request logs.
func (s *Server) ClearRequestLogs() {
	if s.logger != nil {
		s.logger.Clear()
	}
}

// RequestLogCount returns the number of request logs.
func (s *Server) RequestLogCount() int {
	if s.logger == nil {
		return 0
	}
	return s.logger.Count()
}

// Logger returns the request logger (for admin API use).
func (s *Server) Logger() RequestLogger {
	return s.logger
}

// StatefulStore returns the stateful resource store (for admin API use).
func (s *Server) StatefulStore() *stateful.StateStore {
	return s.statefulStore
}

// RegisterStatefulResource registers a stateful resource from config.
func (s *Server) RegisterStatefulResource(cfg *config.StatefulResourceConfig) error {
	return s.statefulStore.Register(&stateful.ResourceConfig{
		Name:        cfg.Name,
		BasePath:    cfg.BasePath,
		IDField:     cfg.IDField,
		ParentField: cfg.ParentField,
		SeedData:    cfg.SeedData,
	})
}
