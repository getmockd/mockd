// Package admin provides a REST API for managing mock configurations.
package admin

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/getmockd/mockd/pkg/admin/engineclient"
	"github.com/getmockd/mockd/pkg/logging"
	"github.com/getmockd/mockd/pkg/metrics"
	"github.com/getmockd/mockd/pkg/ratelimit"
	"github.com/getmockd/mockd/pkg/store"
	"github.com/getmockd/mockd/pkg/store/file"
	"github.com/getmockd/mockd/pkg/tracing"
	"github.com/getmockd/mockd/pkg/workspace"
)

// EngineHeartbeatTimeout is the duration after which an engine is marked offline
// if no heartbeat is received.
const EngineHeartbeatTimeout = 30 * time.Second

// DefaultRateLimit is the default requests per second limit for the admin API.
const DefaultRateLimit float64 = 100

// DefaultBurstSize is the default burst size for the admin API.
const DefaultBurstSize int = 200

// API exposes a REST API for managing mock configurations.
type API struct {
	// localEngine is the HTTP client for communicating with the local engine.
	// Stored as atomic.Pointer to prevent data races between SetLocalEngine /
	// handleRegisterEngine (writers) and handler goroutines (readers).
	localEngine atomic.Pointer[engineclient.Client]

	proxyManager           *ProxyManager
	streamRecordingManager *StreamRecordingManager
	mqttRecordingManager   *MQTTRecordingManager
	soapRecordingManager   *SOAPRecordingManager
	workspaceStore         *store.WorkspaceFileStore
	engineRegistry         *store.EngineRegistry
	workspaceManager       workspace.Manager
	dataStore              *file.FileStore // Persistent store for mocks and folders
	httpServer             *http.Server
	port                   int
	startTime              time.Time
	ctx                    context.Context
	cancel                 context.CancelFunc
	log                    atomic.Pointer[slog.Logger]

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
	rateLimiter *ratelimit.PerIPLimiter

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

	// Tunnel state for local engine
	localTunnel *store.TunnelConfig
	tunnelMu    sync.RWMutex
}

// NewAPI creates a new API.
func NewAPI(port int, opts ...Option) *API {
	// Create context for background goroutines
	ctx, cancel := context.WithCancel(context.Background())

	// Initialize metrics registry
	metricsRegistry := metrics.Init()

	api := &API{
		proxyManager:                NewProxyManager(),
		streamRecordingManager:      NewStreamRecordingManager(),
		mqttRecordingManager:        NewMQTTRecordingManager(),
		soapRecordingManager:        NewSOAPRecordingManager(),
		engineRegistry:              store.NewEngineRegistry(),
		port:                        port,
		ctx:                         ctx,
		cancel:                      cancel,
		registrationTokens:          make(map[string]storedToken),
		engineTokens:                make(map[string]storedToken),
		registrationTokenExpiration: RegistrationTokenExpiration,
		engineTokenExpiration:       EngineTokenExpiration,
		apiKeyConfig:                DefaultAPIKeyConfig(),
		metricsRegistry:             metricsRegistry,
	}

	// Store default nop logger (can be replaced with SetLogger before Start)
	api.log.Store(logging.Nop())

	// Apply options first so dataDir can be set
	for _, opt := range opts {
		opt(api)
	}

	// Initialize the workspace file store (after options to use dataDir if set)
	wsStore := store.NewWorkspaceFileStore(api.dataDir)
	if err := wsStore.Open(context.Background()); err != nil {
		// Log but don't fail - workspace features will be limited
		api.logger().Warn("failed to initialize workspace store", "error", err)
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
		api.logger().Warn("failed to initialize data store", "error", err)
	}
	api.dataStore = dataStore

	// Initialize rate limiter with defaults if not provided via options
	if api.rateLimiter == nil {
		api.rateLimiter = ratelimit.NewPerIPLimiter(ratelimit.PerIPConfig{
			Rate:  DefaultRateLimit,
			Burst: DefaultBurstSize,
		})
	}

	// Initialize CORS config with defaults if not provided via options
	// Check if corsConfig is zero value (no AllowedOrigins set)
	if len(api.corsConfig.AllowedOrigins) == 0 && len(api.corsConfig.AllowedMethods) == 0 {
		api.corsConfig = DefaultCORSConfig()
	}

	// Initialize API key authentication.
	// The logFn closure reads the logger dynamically via api.logger() so
	// that it picks up any logger set later via SetLogger (C3 fix).
	logFn := func(msg string, args ...any) {
		api.logger().Info(msg, args...)
	}
	apiKeyAuth, err := newAPIKeyAuth(api.apiKeyConfig, logFn)
	if err != nil {
		api.logger().Error("failed to initialize API key auth", "error", err)
		// Continue without API key auth - will be insecure but functional
	}
	api.apiKeyAuth = apiKeyAuth

	// Initialize stream recording manager with data directory
	if err := api.streamRecordingManager.Initialize(api.dataDir); err != nil {
		api.logger().Warn("failed to initialize stream recording manager", "error", err)
		// Continue without stream recording - feature will be unavailable
	}

	mux := http.NewServeMux()
	api.registerRoutes(mux)

	api.httpServer = &http.Server{
		Addr:         fmt.Sprintf(":%d", port),
		Handler:      api.withMiddleware(mux),
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	return api
}

// InitializeStreamRecordings initializes the stream recording manager with the given data directory.
func (a *API) InitializeStreamRecordings(dataDir string) error {
	return a.streamRecordingManager.Initialize(dataDir)
}

// StreamRecordingManager returns the stream recording manager.
func (a *API) StreamRecordingManager() *StreamRecordingManager {
	return a.streamRecordingManager
}

// MQTTRecordingManager returns the MQTT recording manager.
func (a *API) MQTTRecordingManager() *MQTTRecordingManager {
	return a.mqttRecordingManager
}

// SOAPRecordingManager returns the SOAP recording manager.
func (a *API) SOAPRecordingManager() *SOAPRecordingManager {
	return a.soapRecordingManager
}

// EngineRegistry returns the engine registry.
func (a *API) EngineRegistry() *store.EngineRegistry {
	return a.engineRegistry
}

// WorkspaceManager returns the workspace manager for multi-workspace serving.
// Returns nil if no workspace manager was configured via WithWorkspaceManager.
func (a *API) WorkspaceManager() workspace.Manager {
	return a.workspaceManager
}

// LocalEngine returns the local engine HTTP client.
// Returns nil if no local engine is configured.
// Safe to call from any goroutine.
func (a *API) LocalEngine() *engineclient.Client {
	return a.localEngine.Load()
}

// SetLocalEngine atomically sets the local engine client after the admin has
// started. This allows connecting an engine that was started after the admin.
// Safe to call concurrently with handler goroutines that read via localEngine.Load().
func (a *API) SetLocalEngine(client *engineclient.Client) {
	a.localEngine.Store(client)
}

// MetricsRegistry returns the metrics registry for Prometheus metrics.
func (a *API) MetricsRegistry() *metrics.Registry {
	return a.metricsRegistry
}

// HasLocalEngine returns true if a local engine is configured.
// Safe to call from any goroutine.
func (a *API) HasLocalEngine() bool {
	return a.localEngine.Load() != nil
}

// maxRequestBodySize is the default maximum request body size for admin API handlers (2MB).
// Individual handlers that need larger bodies (e.g., config import, bulk create) override this.
const maxRequestBodySize = 2 << 20

// withMiddleware wraps the handler with rate limiting, logging, security headers, CORS, API key auth, and tracing middleware.
// Middleware order (outermost to innermost): Tracing -> Security Headers -> CORS -> API Key Auth -> Rate Limiting -> Body Limit -> Handler
func (a *API) withMiddleware(handler http.Handler) http.Handler {
	// Body size limit (innermost — protects all handlers from oversized request bodies)
	bodyCapped := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodySize)
		handler.ServeHTTP(w, r)
	})

	// Apply rate limiting
	rateLimited := ratelimit.Middleware(a.rateLimiter, ratelimit.WithTextResponse())(bodyCapped)

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
func (a *API) tracingMiddleware(next http.Handler) http.Handler {
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
func (a *API) Tracer() *tracing.Tracer {
	return a.tracer
}

// Start starts the admin API server.
func (a *API) Start() error {
	a.startTime = time.Now()

	// Start the engine health check background goroutine
	a.engineRegistry.StartHealthCheck(a.ctx, EngineHeartbeatTimeout)

	// Start the token cleanup background goroutine
	go a.startTokenCleanup(a.ctx)

	a.logger().Info("starting admin API", "port", a.port)
	go func() {
		if err := a.httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			a.logger().Error("admin API error", "error", err)
		}
	}()
	return nil
}

// logger returns the current logger from the atomic pointer.
// This is safe to call from any goroutine concurrently with SetLogger.
func (a *API) logger() *slog.Logger {
	return a.log.Load()
}

// SetLogger atomically sets the operational logger for the admin API and
// propagates it to all manager subsystems via their own lock-protected
// SetLogger methods. Safe to call concurrently with handler goroutines
// that read the logger via the logger() accessor.
func (a *API) SetLogger(log *slog.Logger) {
	if log == nil {
		log = logging.Nop()
	}
	a.log.Store(log)

	// Fan out to managers via their lock-protected SetLogger methods so
	// that concurrent handler goroutines never observe a torn pointer write.
	a.proxyManager.SetLogger(log.With("component", "proxy"))
	a.streamRecordingManager.SetLogger(log.With("component", "stream-recording"))
	a.mqttRecordingManager.SetLogger(log.With("component", "mqtt-recording"))
	a.soapRecordingManager.SetLogger(log.With("component", "soap-recording"))
}

// Stop gracefully shuts down the admin API server.
func (a *API) Stop() error {
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
			a.logger().Warn("error stopping workspace servers", "error", err)
		}
	}

	// Close the data store
	if a.dataStore != nil {
		if err := a.dataStore.Close(); err != nil {
			a.logger().Warn("error closing data store", "error", err)
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return a.httpServer.Shutdown(ctx)
}

// Uptime returns the API uptime in seconds.
func (a *API) Uptime() int {
	return int(time.Since(a.startTime).Seconds())
}
