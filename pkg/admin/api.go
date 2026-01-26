// Package admin provides a REST API for managing mock configurations.
package admin

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/getmockd/mockd/pkg/admin/engineclient"
	"github.com/getmockd/mockd/pkg/engine"
	"github.com/getmockd/mockd/pkg/logging"
	"github.com/getmockd/mockd/pkg/metrics"
	"github.com/getmockd/mockd/pkg/store"
	"github.com/getmockd/mockd/pkg/store/file"
	"github.com/getmockd/mockd/pkg/tracing"
)

// EngineHeartbeatTimeout is the duration after which an engine is marked offline
// if no heartbeat is received.
const EngineHeartbeatTimeout = 30 * time.Second

// AdminAPI exposes a REST API for managing mock configurations.
type AdminAPI struct {
	// localEngine is the HTTP client for communicating with the local engine.
	localEngine *engineclient.Client

	proxyManager           *ProxyManager
	streamRecordingManager *StreamRecordingManager
	mqttRecordingManager   *MQTTRecordingManager
	soapRecordingManager   *SOAPRecordingManager
	workspaceStore         *store.WorkspaceFileStore
	engineRegistry         *store.EngineRegistry
	workspaceManager       *engine.WorkspaceManager
	dataStore              *file.FileStore // Persistent store for mocks and folders
	httpServer             *http.Server
	port                   int
	startTime              time.Time
	ctx                    context.Context
	cancel                 context.CancelFunc
	log                    *slog.Logger

	// Token management for engine authentication
	registrationTokens map[string]storedToken // token -> storedToken
	engineTokens       map[string]storedToken // engineID -> storedToken
	tokenMu            sync.RWMutex

	// Token expiration configuration (can be overridden)
	registrationTokenExpiration time.Duration
	engineTokenExpiration       time.Duration

	// API key authentication
	apiKeyAuth   *apiKeyAuth
	apiKeyConfig APIKeyConfig

	// Rate limiter for API protection
	rateLimiter *RateLimiter

	// CORS configuration
	corsConfig CORSConfig

	// Metrics registry for Prometheus metrics
	metricsRegistry *metrics.Registry

	// Tracer for distributed tracing (optional)
	tracer *tracing.Tracer

	// Custom data directory (for test isolation)
	dataDir string

	// Version string for status endpoint
	version string

	// AllowLocalhostBypass allows unauthenticated access from localhost (dev mode only)
	// Default is false - authentication is always required
	allowLocalhostBypass bool
}

// NewAdminAPI creates a new AdminAPI.
func NewAdminAPI(port int, opts ...Option) *AdminAPI {
	log := logging.Nop() // Default to no-op, can be set with SetLogger

	// Create context for background goroutines
	ctx, cancel := context.WithCancel(context.Background())

	// Create workspace manager for multi-workspace serving
	wsManager := engine.NewWorkspaceManager(nil)

	// Initialize metrics registry
	metricsRegistry := metrics.Init()

	api := &AdminAPI{
		proxyManager:                NewProxyManager(),
		streamRecordingManager:      NewStreamRecordingManager(),
		mqttRecordingManager:        NewMQTTRecordingManager(),
		soapRecordingManager:        NewSOAPRecordingManager(),
		engineRegistry:              store.NewEngineRegistry(),
		workspaceManager:            wsManager,
		port:                        port,
		ctx:                         ctx,
		cancel:                      cancel,
		log:                         log,
		registrationTokens:          make(map[string]storedToken),
		engineTokens:                make(map[string]storedToken),
		registrationTokenExpiration: RegistrationTokenExpiration,
		engineTokenExpiration:       EngineTokenExpiration,
		apiKeyConfig:                DefaultAPIKeyConfig(),
		metricsRegistry:             metricsRegistry,
	}

	// Apply options first so dataDir can be set
	for _, opt := range opts {
		opt(api)
	}

	// Initialize the workspace file store (after options to use dataDir if set)
	wsStore := store.NewWorkspaceFileStore(api.dataDir)
	if err := wsStore.Open(context.Background()); err != nil {
		// Log but don't fail - workspace features will be limited
		log.Warn("failed to initialize workspace store", "error", err)
	}
	api.workspaceStore = wsStore

	// Initialize the data store for mocks and folders (after options to use dataDir if set)
	var dataStore *file.FileStore
	if api.dataDir != "" {
		cfg := store.DefaultConfig()
		cfg.DataDir = api.dataDir
		dataStore = file.New(cfg)
	} else {
		dataStore = file.NewWithDefaults()
	}
	if err := dataStore.Open(context.Background()); err != nil {
		// Log but don't fail - store features will be limited
		log.Warn("failed to initialize data store", "error", err)
	}
	api.dataStore = dataStore

	// Initialize rate limiter with defaults if not provided via options
	if api.rateLimiter == nil {
		api.rateLimiter = NewRateLimiter(DefaultRateLimit, DefaultBurstSize)
	}

	// Initialize CORS config with defaults if not provided via options
	// Check if corsConfig is zero value (no AllowedOrigins set)
	if len(api.corsConfig.AllowedOrigins) == 0 && len(api.corsConfig.AllowedMethods) == 0 {
		api.corsConfig = DefaultCORSConfig()
	}

	// Initialize API key authentication
	logFn := func(msg string, args ...any) {
		log.Info(msg, args...)
	}
	apiKeyAuth, err := newAPIKeyAuth(api.apiKeyConfig, logFn)
	if err != nil {
		log.Error("failed to initialize API key auth", "error", err)
		// Continue without API key auth - will be insecure but functional
	}
	api.apiKeyAuth = apiKeyAuth

	// Initialize stream recording manager with data directory
	if err := api.streamRecordingManager.Initialize(api.dataDir); err != nil {
		log.Warn("failed to initialize stream recording manager", "error", err)
		// Continue without stream recording - feature will be unavailable
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

// MQTTRecordingManager returns the MQTT recording manager.
func (a *AdminAPI) MQTTRecordingManager() *MQTTRecordingManager {
	return a.mqttRecordingManager
}

// SOAPRecordingManager returns the SOAP recording manager.
func (a *AdminAPI) SOAPRecordingManager() *SOAPRecordingManager {
	return a.soapRecordingManager
}

// EngineRegistry returns the engine registry.
func (a *AdminAPI) EngineRegistry() *store.EngineRegistry {
	return a.engineRegistry
}

// WorkspaceManager returns the workspace manager for multi-workspace serving.
func (a *AdminAPI) WorkspaceManager() *engine.WorkspaceManager {
	return a.workspaceManager
}

// LocalEngine returns the local engine HTTP client.
// Returns nil if no local engine is configured.
func (a *AdminAPI) LocalEngine() *engineclient.Client {
	return a.localEngine
}

// SetLocalEngine sets the local engine client after the admin has started.
// This allows connecting an engine that was started after the admin.
func (a *AdminAPI) SetLocalEngine(client *engineclient.Client) {
	a.localEngine = client
}

// MetricsRegistry returns the metrics registry for Prometheus metrics.
func (a *AdminAPI) MetricsRegistry() *metrics.Registry {
	return a.metricsRegistry
}

// HasLocalEngine returns true if a local engine is configured.
func (a *AdminAPI) HasLocalEngine() bool {
	return a.localEngine != nil
}

// withMiddleware wraps the handler with rate limiting, logging, security headers, CORS, API key auth, and tracing middleware.
// Middleware order (outermost to innermost): Tracing -> Security Headers -> CORS -> API Key Auth -> Rate Limiting -> Handler
func (a *AdminAPI) withMiddleware(handler http.Handler) http.Handler {
	// Apply rate limiting first (innermost middleware)
	rateLimited := a.rateLimiter.Middleware(handler)

	// API key authentication wraps rate limiting
	authenticated := rateLimited
	if a.apiKeyAuth != nil {
		authenticated = a.apiKeyAuth.middleware(rateLimited)
	}

	// CORS middleware wraps API key auth
	corsHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")

		// Always set Vary header to indicate origin-dependent response
		w.Header().Add("Vary", "Origin")

		// Get the appropriate Allow-Origin value based on config
		allowOrigin := a.corsConfig.getAllowOriginValue(origin)
		if allowOrigin == "" {
			// Origin not allowed - process request but browser will block response
			// For preflight, we should return early without CORS headers
			if r.Method == "OPTIONS" {
				w.WriteHeader(http.StatusForbidden)
				return
			}
			authenticated.ServeHTTP(w, r)
			return
		}

		// Set CORS headers
		w.Header().Set("Access-Control-Allow-Origin", allowOrigin)
		w.Header().Set("Access-Control-Allow-Methods", a.corsConfig.getMethods())
		w.Header().Set("Access-Control-Allow-Headers", a.corsConfig.getHeaders())
		w.Header().Set("Access-Control-Max-Age", a.corsConfig.getMaxAge())

		// Set credentials header if enabled
		if a.corsConfig.AllowCredentials {
			w.Header().Set("Access-Control-Allow-Credentials", "true")
		}

		// Handle preflight requests (don't rate limit OPTIONS)
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		authenticated.ServeHTTP(w, r)
	})

	// Security headers middleware wraps CORS
	securityHandler := SecurityHeadersMiddleware(corsHandler)

	// Tracing middleware (outermost, captures full request lifecycle)
	// Only applied if a tracer is configured
	if a.tracer != nil {
		return a.tracingMiddleware(securityHandler)
	}

	return securityHandler
}

// skipTracingPaths contains paths that should not create traces.
// These are typically health checks and metrics endpoints.
var skipTracingPaths = map[string]bool{
	"/metrics":  true,
	"/health":   true,
	"/healthz":  true,
	"/ready":    true,
	"/readyz":   true,
	"/livez":    true,
	"/_/health": true,
}

// tracingMiddleware wraps a handler with distributed tracing support.
func (a *AdminAPI) tracingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Skip tracing for health/metrics endpoints to avoid noise
		if skipTracingPaths[r.URL.Path] {
			next.ServeHTTP(w, r)
			return
		}

		// Extract trace context from incoming request headers
		ctx := tracing.Extract(r.Context(), r.Header)

		// Create span name like "HTTP GET /path"
		spanName := fmt.Sprintf("HTTP %s %s", r.Method, r.URL.Path)

		// Start a new span
		ctx, span := a.tracer.Start(ctx, spanName)
		defer span.End()

		// Set HTTP request attributes
		span.SetAttribute("http.method", r.Method)
		span.SetAttribute("http.url", r.URL.String())
		span.SetAttribute("http.target", r.URL.Path)
		span.SetAttribute("http.host", r.Host)

		// Wrap response writer to capture status code
		wrapped := &adminStatusCapturingResponseWriter{
			ResponseWriter: w,
			statusCode:     http.StatusOK,
		}

		// Update request context with span
		r = r.WithContext(ctx)

		// Call the next handler
		next.ServeHTTP(wrapped, r)

		// Set response attributes
		span.SetAttribute("http.status_code", strconv.Itoa(wrapped.statusCode))

		// Set span status based on HTTP status code
		if wrapped.statusCode >= 400 && wrapped.statusCode < 500 {
			span.SetStatus(tracing.StatusError, fmt.Sprintf("HTTP client error: %d", wrapped.statusCode))
		} else if wrapped.statusCode >= 500 {
			span.SetStatus(tracing.StatusError, fmt.Sprintf("HTTP server error: %d", wrapped.statusCode))
		} else {
			span.SetStatus(tracing.StatusOK, "")
		}
	})
}

// adminStatusCapturingResponseWriter wraps http.ResponseWriter to capture the status code.
type adminStatusCapturingResponseWriter struct {
	http.ResponseWriter
	statusCode    int
	headerWritten bool
}

// WriteHeader captures the status code before writing the header.
func (w *adminStatusCapturingResponseWriter) WriteHeader(code int) {
	if !w.headerWritten {
		w.statusCode = code
		w.headerWritten = true
	}
	w.ResponseWriter.WriteHeader(code)
}

// Write captures status code if not already written (implicit 200 OK).
func (w *adminStatusCapturingResponseWriter) Write(b []byte) (int, error) {
	if !w.headerWritten {
		w.statusCode = http.StatusOK
		w.headerWritten = true
	}
	return w.ResponseWriter.Write(b)
}

// Unwrap returns the underlying ResponseWriter for http.ResponseController support.
func (w *adminStatusCapturingResponseWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}

// Tracer returns the tracer, if configured.
func (a *AdminAPI) Tracer() *tracing.Tracer {
	return a.tracer
}

// Start starts the admin API server.
func (a *AdminAPI) Start() error {
	a.startTime = time.Now()

	// Start the engine health check background goroutine
	a.engineRegistry.StartHealthCheck(a.ctx, EngineHeartbeatTimeout)

	// Start the token cleanup background goroutine
	go a.startTokenCleanup(a.ctx)

	a.log.Info("starting admin API", "port", a.port)
	go func() {
		if err := a.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			a.log.Error("admin API error", "error", err)
		}
	}()
	return nil
}

// SetLogger sets the operational logger for the admin API.
func (a *AdminAPI) SetLogger(log *slog.Logger) {
	if log != nil {
		a.log = log
	} else {
		a.log = logging.Nop()
	}
}

// Stop gracefully shuts down the admin API server.
func (a *AdminAPI) Stop() error {
	// Stop background goroutines
	a.cancel()
	a.engineRegistry.Stop()

	// Stop the rate limiter cleanup goroutine
	if a.rateLimiter != nil {
		a.rateLimiter.Stop()
	}

	// Stop all workspace servers
	if a.workspaceManager != nil {
		if err := a.workspaceManager.StopAll(); err != nil {
			a.log.Warn("error stopping workspace servers", "error", err)
		}
	}

	// Close the data store
	if a.dataStore != nil {
		if err := a.dataStore.Close(); err != nil {
			a.log.Warn("error closing data store", "error", err)
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return a.httpServer.Shutdown(ctx)
}

// Uptime returns the API uptime in seconds.
func (a *AdminAPI) Uptime() int {
	return int(time.Since(a.startTime).Seconds())
}
