package cli

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/getmockd/mockd/internal/runtime"
	"github.com/getmockd/mockd/pkg/admin"
	"github.com/getmockd/mockd/pkg/admin/engineclient"
	"github.com/getmockd/mockd/pkg/audit"
	"github.com/getmockd/mockd/pkg/chaos"
	"github.com/getmockd/mockd/pkg/cli/internal/output"
	"github.com/getmockd/mockd/pkg/cli/internal/ports"
	"github.com/getmockd/mockd/pkg/cliconfig"
	"github.com/getmockd/mockd/pkg/config"
	"github.com/getmockd/mockd/pkg/engine"
	"github.com/getmockd/mockd/pkg/graphql"
	"github.com/getmockd/mockd/pkg/logging"
	"github.com/getmockd/mockd/pkg/mqtt"
	"github.com/getmockd/mockd/pkg/oauth"
	"github.com/getmockd/mockd/pkg/recording"
	"github.com/getmockd/mockd/pkg/store"
	"github.com/getmockd/mockd/pkg/store/file"
	"github.com/getmockd/mockd/pkg/tracing"
	"github.com/getmockd/mockd/pkg/validation"

	"github.com/spf13/cobra"
)

// shutdownTimeout is the maximum time to wait for graceful shutdown.
const shutdownTimeout = 30 * time.Second

// serveFlagVals is the package-level instance bound to cobra flags.
var serveFlagVals serveFlags

// serveCmd represents the serve command — the full-featured foreground server.
var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the full-featured mock server (foreground)",
	Long: `Start the mock server. Can operate in three modes:

1. Local mode (default): Serve mocks from local configuration
2. Runtime mode (--register): Register with control plane and receive deployments
3. Pull mode (--pull): Pull mocks from cloud and serve locally

The serve command is the full-featured server entrypoint with support for MCP,
CORS, rate limiting, distributed tracing, log aggregation, and more.
Use 'mockd start' for a simpler startup experience.`,
	Example: `  # Start with defaults
  mockd serve

  # Start with config file on custom port
  mockd serve --config mocks.json --port 3000

  # Register as a runtime
  mockd serve --register --name ci-runner-1 --token $MOCKD_RUNTIME_TOKEN

  # Pull and serve from cloud
  mockd serve --pull mockd://acme/payment-api

  # Start with TLS using certificate files
  mockd serve --tls-cert server.crt --tls-key server.key --https-port 8443

  # Start with mTLS enabled
  mockd serve --mtls-enabled --mtls-ca ca.crt --tls-cert server.crt --tls-key server.key

  # Start with distributed tracing (send traces to Jaeger via OTLP)
  mockd serve --otlp-endpoint http://localhost:4318/v1/traces

  # Start with MCP server enabled (for AI assistants via HTTP)
  mockd serve --mcp

  # Start in daemon/background mode
  mockd serve -d`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Get token from env var if not provided via flag
		if serveFlagVals.token == "" {
			serveFlagVals.token = os.Getenv("MOCKD_RUNTIME_TOKEN")
		}
		return runServeWithFlags(&serveFlagVals)
	},
}

//nolint:funlen // flag registration is inherently long
func initServeCmd() {
	rootCmd.AddCommand(serveCmd)

	f := &serveFlagVals

	// Standard server flags
	serveCmd.Flags().IntVarP(&f.port, "port", "p", cliconfig.DefaultPort, "HTTP server port")
	serveCmd.Flags().IntVarP(&f.adminPort, "admin-port", "a", cliconfig.DefaultAdminPort, "Admin API port")
	serveCmd.Flags().StringVarP(&f.configFile, "config", "c", "", "Path to mock configuration file")
	serveCmd.Flags().IntVar(&f.httpsPort, "https-port", cliconfig.DefaultHTTPSPort, "HTTPS server port (0 = disabled)")
	serveCmd.Flags().IntVar(&f.readTimeout, "read-timeout", cliconfig.DefaultReadTimeout, "Read timeout in seconds")
	serveCmd.Flags().IntVar(&f.writeTimeout, "write-timeout", cliconfig.DefaultWriteTimeout, "Write timeout in seconds")
	serveCmd.Flags().IntVar(&f.requestTimeout, "request-timeout", 0, "Request timeout in seconds (sets both read and write timeout)")
	serveCmd.Flags().IntVar(&f.maxLogEntries, "max-log-entries", cliconfig.DefaultMaxLogEntries, "Maximum request log entries")
	serveCmd.Flags().IntVar(&f.maxConnections, "max-connections", 0, "Maximum concurrent HTTP connections (0 = unlimited)")
	serveCmd.Flags().BoolVar(&f.autoCert, "auto-cert", cliconfig.DefaultAutoCert, "Auto-generate TLS certificate")

	// TLS flags
	serveCmd.Flags().StringVar(&f.tlsCert, "tls-cert", "", "Path to TLS certificate file")
	serveCmd.Flags().StringVar(&f.tlsKey, "tls-key", "", "Path to TLS private key file")
	serveCmd.Flags().BoolVar(&f.tlsAuto, "tls-auto", false, "Auto-generate self-signed certificate")

	// mTLS flags
	serveCmd.Flags().BoolVar(&f.mtlsEnabled, "mtls-enabled", false, "Enable mTLS client certificate validation")
	serveCmd.Flags().StringVar(&f.mtlsClientAuth, "mtls-client-auth", "require-and-verify", "Client auth mode (none, request, require, verify-if-given, require-and-verify)")
	serveCmd.Flags().StringVar(&f.mtlsCA, "mtls-ca", "", "Path to CA certificate for client validation")
	serveCmd.Flags().StringVar(&f.mtlsAllowedCNs, "mtls-allowed-cns", "", "Comma-separated list of allowed Common Names")

	// Audit flags
	serveCmd.Flags().BoolVar(&f.auditEnabled, "audit-enabled", false, "Enable audit logging")
	serveCmd.Flags().StringVar(&f.auditFile, "audit-file", "", "Path to audit log file")
	serveCmd.Flags().StringVar(&f.auditLevel, "audit-level", "info", "Audit log level (debug, info, warn, error)")

	// Runtime mode flags
	serveCmd.Flags().BoolVar(&f.register, "register", false, "Register with control plane as a runtime")
	serveCmd.Flags().StringVar(&f.controlPlane, "control-plane", "https://api.mockd.io", "Control plane URL")
	serveCmd.Flags().StringVar(&f.token, "token", "", "Runtime token (or set MOCKD_RUNTIME_TOKEN env var)")
	serveCmd.Flags().StringVar(&f.name, "name", "", "Runtime name (required with --register)")
	serveCmd.Flags().StringVar(&f.labels, "labels", "", "Runtime labels as key=value pairs (comma-separated)")

	// Pull mode flags
	serveCmd.Flags().StringVar(&f.pull, "pull", "", "Pull and serve mocks from mockd:// URI")
	serveCmd.Flags().StringVar(&f.cacheDir, "cache", "", "Local cache directory for pulled mocks")

	// GraphQL flags
	serveCmd.Flags().StringVar(&f.graphqlSchema, "graphql-schema", "", "Path to GraphQL schema file")
	serveCmd.Flags().StringVar(&f.graphqlPath, "graphql-path", "/graphql", "GraphQL endpoint path")

	// OAuth flags
	serveCmd.Flags().BoolVar(&f.oauthEnabled, "oauth-enabled", false, "Enable OAuth provider")
	serveCmd.Flags().StringVar(&f.oauthIssuer, "oauth-issuer", "", "OAuth issuer URL")
	serveCmd.Flags().IntVar(&f.oauthPort, "oauth-port", 0, "OAuth server port")

	// Chaos flags
	serveCmd.Flags().BoolVar(&f.chaosEnabled, "chaos-enabled", false, "Enable chaos injection")
	serveCmd.Flags().StringVar(&f.chaosLatency, "chaos-latency", "", "Add random latency (e.g., '10ms-100ms')")
	serveCmd.Flags().Float64Var(&f.chaosErrorRate, "chaos-error-rate", 0, "Error rate (0.0-1.0)")
	serveCmd.Flags().StringVar(&f.chaosProfile, "chaos-profile", "", "Apply a built-in chaos profile at startup (e.g., slow-api, flaky, offline)")

	// Validation flags
	serveCmd.Flags().StringVar(&f.validateSpec, "validate-spec", "", "Path to OpenAPI spec for request validation")
	serveCmd.Flags().BoolVar(&f.validateFail, "validate-fail", false, "Fail on validation error")

	// Storage flags
	serveCmd.Flags().StringVar(&f.dataDir, "data-dir", "", "Data directory for persistent storage")
	serveCmd.Flags().BoolVar(&f.noAuth, "no-auth", false, "Disable API key authentication on admin API")

	// Daemon/detach flags
	serveCmd.Flags().BoolVarP(&f.detach, "detach", "d", false, "Run server in background (daemon mode)")
	serveCmd.Flags().StringVar(&f.pidFile, "pid-file", DefaultPIDPath(), "Path to PID file")

	// Logging flags
	serveCmd.Flags().StringVar(&f.logLevel, "log-level", "info", "Log level (debug, info, warn, error)")
	serveCmd.Flags().StringVar(&f.logFormat, "log-format", "text", "Log format (text, json)")
	serveCmd.Flags().StringVar(&f.lokiEndpoint, "loki-endpoint", "", "Loki endpoint for log aggregation")

	// Tracing flags
	serveCmd.Flags().StringVar(&f.otlpEndpoint, "otlp-endpoint", "", "OTLP HTTP endpoint for distributed tracing")
	serveCmd.Flags().Float64Var(&f.traceSampler, "trace-sampler", 1.0, "Trace sampling ratio (0.0-1.0)")

	// MCP flags
	serveCmd.Flags().BoolVar(&f.mcpEnabled, "mcp", false, "Enable MCP (Model Context Protocol) HTTP server")
	serveCmd.Flags().IntVar(&f.mcpPort, "mcp-port", 9091, "MCP server port")
	serveCmd.Flags().BoolVar(&f.mcpAllowRemote, "mcp-allow-remote", false, "Allow remote MCP connections")

	// CORS flags
	serveCmd.Flags().StringVar(&f.corsOrigins, "cors-origins", "", "Comma-separated CORS allowed origins")

	// Rate limiting flags
	serveCmd.Flags().Float64Var(&f.rateLimit, "rate-limit", 0, "Rate limit in requests per second (0 = disabled)")

	// Persistence flags
	serveCmd.Flags().BoolVar(&f.noPersist, "no-persist", false, "Disable persistent storage (mocks are lost on restart)")
}

func init() {
	initServeCmd()
}

// serveFlags holds all parsed command-line flags for the serve command.
type serveFlags struct {
	// Standard server flags
	port           int
	adminPort      int
	configFile     string
	httpsPort      int
	readTimeout    int
	writeTimeout   int
	requestTimeout int
	maxLogEntries  int
	maxConnections int
	autoCert       bool

	// TLS flags
	tlsCert string
	tlsKey  string
	tlsAuto bool

	// mTLS flags
	mtlsEnabled    bool
	mtlsClientAuth string
	mtlsCA         string
	mtlsAllowedCNs string

	// Audit flags
	auditEnabled bool
	auditFile    string
	auditLevel   string

	// Runtime mode flags
	register     bool
	controlPlane string
	token        string
	name         string
	labels       string

	// Pull mode flags
	pull     string
	cacheDir string

	// GraphQL flags
	graphqlSchema string
	graphqlPath   string

	// OAuth flags
	oauthEnabled bool
	oauthIssuer  string
	oauthPort    int

	// Chaos flags
	chaosEnabled   bool
	chaosLatency   string
	chaosErrorRate float64
	chaosProfile   string

	// Validation flags
	validateSpec string
	validateFail bool

	// Storage flags
	dataDir string
	noAuth  bool

	// Daemon/detach flags
	detach  bool
	pidFile string

	// Logging flags
	logLevel     string
	logFormat    string
	lokiEndpoint string

	// Tracing flags
	otlpEndpoint string
	traceSampler float64

	// MCP flags
	mcpEnabled     bool
	mcpPort        int
	mcpAllowRemote bool

	// CORS flags
	corsOrigins string

	// Rate limiting flags
	rateLimit float64

	// Persistence flags
	noPersist bool
}

// serveContext holds all runtime state for the serve command.
type serveContext struct {
	flags         *serveFlags
	serverCfg     *config.ServerConfiguration
	server        *engine.Server
	adminAPI      *admin.API
	runtimeClient *runtime.Client
	mqttBroker    *mqtt.Broker
	chaosConfig   *engineclient.ChaosConfig // Chaos config to apply after engine starts
	mcpServer     MCPStopper
	store         *file.FileStore
	log           *slog.Logger
	tracer        *tracing.Tracer
	ctx           context.Context
	cancel        context.CancelFunc
}

// runServeWithFlags is the core serve logic called by the cobra command.
func runServeWithFlags(flags *serveFlags) error {
	// Handle detach mode (daemon) - re-exec as child and exit
	if flags.detach && os.Getenv("MOCKD_CHILD") == "" {
		return daemonize(nil, flags.pidFile, flags.port, flags.adminPort)
	}

	// Validate flags for different modes
	if err := validateServeFlags(flags); err != nil {
		return err
	}

	// Check for port conflicts
	if err := checkPortConflicts(flags); err != nil {
		return err
	}

	// Build server configuration from flags
	serverCfg, err := buildServerConfiguration(flags)
	if err != nil {
		return err
	}

	// Create serve context to hold all runtime state
	ctx, cancel := context.WithCancel(context.Background())
	sctx := &serveContext{
		flags:     flags,
		serverCfg: serverCfg,
		ctx:       ctx,
		cancel:    cancel,
	}
	defer cancel()

	// Initialize structured logger
	sctx.log = logging.New(logging.Config{
		Level:  logging.ParseLevel(flags.logLevel),
		Format: logging.ParseFormat(flags.logFormat),
	})

	// Add Loki handler if endpoint is configured
	if flags.lokiEndpoint != "" {
		lokiHandler := logging.NewLokiHandler(flags.lokiEndpoint,
			logging.WithLokiLabels(map[string]string{
				"service": "mockd",
				"port":    strconv.Itoa(flags.port),
			}),
			logging.WithLokiLevel(logging.ParseLevel(flags.logLevel)),
		)
		// Create a multi-handler that writes to both stdout and Loki
		sctx.log = slog.New(logging.NewMultiHandler(sctx.log.Handler(), lokiHandler))
		sctx.log.Info("log aggregation enabled", "endpoint", flags.lokiEndpoint)
	}

	// Initialize distributed tracer if OTLP endpoint is configured
	if flags.otlpEndpoint != "" {
		sctx.tracer = initializeTracer(flags.otlpEndpoint, flags.traceSampler)
		sctx.log.Info("distributed tracing enabled", "endpoint", flags.otlpEndpoint, "sampler", flags.traceSampler)
	}

	// Create and configure the mock server
	var engineOpts []engine.ServerOption
	if sctx.tracer != nil {
		engineOpts = append(engineOpts, engine.WithTracer(sctx.tracer))
	}
	sctx.server = engine.NewServer(serverCfg, engineOpts...)
	sctx.server.SetLogger(sctx.log.With("component", "engine"))

	// Initialize persistent store if needed
	if err := initializePersistentStore(sctx); err != nil {
		return err
	}
	if sctx.store != nil {
		defer func() { _ = sctx.store.Close() }()
	}

	// Configure protocol handlers (MQTT, chaos injection)
	if err := configureProtocolHandlers(sctx); err != nil {
		return err
	}

	// Start all servers (engine + admin) BEFORE loading mocks.
	// Config loading goes through admin so mocks are written to both the
	// persistent store and the engine — keeping them in sync from the start.
	if err := startServers(sctx); err != nil {
		return err
	}

	// Load mocks through admin (must happen after servers are up)
	if err := handleOperatingMode(sctx); err != nil {
		_ = sctx.server.Stop()
		_ = sctx.adminAPI.Stop()
		return err
	}

	// PID file, MCP server, startup message (needs mock counts)
	if err := postStartup(sctx); err != nil {
		return err
	}

	// Run main event loop (blocks until shutdown signal)
	return runMainLoop(sctx)
}

// validateServeFlags validates flag combinations for different operating modes.

// validateServeFlags validates flag combinations for different operating modes.
func validateServeFlags(f *serveFlags) error {
	// Validate port ranges
	if f.port < 0 || f.port > 65535 {
		return fmt.Errorf("invalid port %d: must be between 0 and 65535", f.port)
	}
	if f.adminPort < 0 || f.adminPort > 65535 {
		return fmt.Errorf("invalid admin port %d: must be between 0 and 65535", f.adminPort)
	}
	if f.httpsPort < 0 || f.httpsPort > 65535 {
		return fmt.Errorf("invalid HTTPS port %d: must be between 0 and 65535", f.httpsPort)
	}
	if f.mcpEnabled && (f.mcpPort < 0 || f.mcpPort > 65535) {
		return fmt.Errorf("invalid MCP port %d: must be between 0 and 65535", f.mcpPort)
	}

	if f.register && f.pull != "" {
		return errors.New("cannot use --register and --pull together")
	}

	if f.register {
		if f.name == "" {
			return errors.New("--name is required when using --register")
		}
		if f.token == "" {
			return errors.New("--token is required when using --register (or set MOCKD_RUNTIME_TOKEN)")
		}
	}

	if f.pull != "" && f.token == "" {
		f.token = os.Getenv("MOCKD_TOKEN")
		if f.token == "" {
			return errors.New("--token is required when using --pull (or set MOCKD_TOKEN)")
		}
	}

	// Validate chaos flag combinations
	if f.chaosProfile != "" {
		if _, ok := chaos.GetProfile(f.chaosProfile); !ok {
			names := chaos.ProfileNames()
			return fmt.Errorf("unknown chaos profile %q — available profiles: %s", f.chaosProfile, strings.Join(names, ", "))
		}
		if f.chaosEnabled || f.chaosLatency != "" || f.chaosErrorRate > 0 {
			return errors.New("--chaos-profile cannot be combined with --chaos-enabled, --chaos-latency, or --chaos-error-rate")
		}
	}

	return nil
}

// checkPortConflicts verifies that requested ports are available.
func checkPortConflicts(f *serveFlags) error {
	if err := ports.Check(f.port); err != nil {
		return formatPortError(f.port, err)
	}
	if err := ports.Check(f.adminPort); err != nil {
		return formatPortError(f.adminPort, err)
	}
	if f.httpsPort > 0 {
		if err := ports.Check(f.httpsPort); err != nil {
			return formatPortError(f.httpsPort, err)
		}
	}
	if f.mcpEnabled {
		if err := ports.Check(f.mcpPort); err != nil {
			return formatPortError(f.mcpPort, err)
		}
	}
	return nil
}

// buildServerConfiguration creates the server configuration from parsed flags.
//
//nolint:unparam // error is always nil but kept for future validation
func buildServerConfiguration(f *serveFlags) (*config.ServerConfiguration, error) {
	readTimeout := f.readTimeout
	writeTimeout := f.writeTimeout
	// --request-timeout is a convenience flag that sets both read and write timeout
	if f.requestTimeout > 0 {
		readTimeout = f.requestTimeout
		writeTimeout = f.requestTimeout
	}

	serverCfg := &config.ServerConfiguration{
		HTTPPort:       f.port,
		HTTPSPort:      f.httpsPort,
		AdminPort:      f.adminPort,
		ReadTimeout:    readTimeout,
		WriteTimeout:   writeTimeout,
		MaxConnections: f.maxConnections,
		MaxLogEntries:  f.maxLogEntries,
		LogRequests:    true,
	}

	// Configure TLS if any TLS flags are set or HTTPS port is configured
	if f.tlsCert != "" || f.tlsKey != "" || f.tlsAuto || f.httpsPort > 0 {
		serverCfg.TLS = &config.TLSConfig{
			Enabled:          true,
			CertFile:         f.tlsCert,
			KeyFile:          f.tlsKey,
			AutoGenerateCert: f.tlsAuto || f.autoCert,
		}
	}

	// Configure mTLS if enabled
	if f.mtlsEnabled {
		var allowedCNs []string
		if f.mtlsAllowedCNs != "" {
			for _, cn := range strings.Split(f.mtlsAllowedCNs, ",") {
				cn = strings.TrimSpace(cn)
				if cn != "" {
					allowedCNs = append(allowedCNs, cn)
				}
			}
		}
		serverCfg.MTLS = &config.MTLSConfig{
			Enabled:    true,
			ClientAuth: f.mtlsClientAuth,
			CACertFile: f.mtlsCA,
			AllowedCNs: allowedCNs,
		}
	}

	// Configure audit if enabled
	if f.auditEnabled {
		serverCfg.Audit = &audit.AuditConfig{
			Enabled:    true,
			Level:      f.auditLevel,
			OutputFile: f.auditFile,
		}
	}

	// Configure GraphQL if schema specified
	if f.graphqlSchema != "" {
		serverCfg.GraphQL = []*graphql.GraphQLConfig{{
			ID:            "cli-graphql",
			Path:          f.graphqlPath,
			SchemaFile:    f.graphqlSchema,
			Introspection: true,
			Enabled:       true,
		}}
	}

	// Configure OAuth if enabled
	if f.oauthEnabled {
		issuer := f.oauthIssuer
		if issuer == "" {
			issuer = fmt.Sprintf("http://localhost:%d", f.oauthPort)
		}
		serverCfg.OAuth = []*oauth.OAuthConfig{{
			ID:      "cli-oauth",
			Issuer:  issuer,
			Enabled: true,
		}}
	}

	// Configure validation if spec specified
	if f.validateSpec != "" {
		serverCfg.Validation = &validation.ValidationConfig{
			Enabled:         true,
			SpecFile:        f.validateSpec,
			ValidateRequest: true,
			FailOnError:     f.validateFail,
		}
	}

	// Configure CORS if origins specified
	if f.corsOrigins != "" {
		origins := strings.Split(f.corsOrigins, ",")
		for i := range origins {
			origins[i] = strings.TrimSpace(origins[i])
		}
		serverCfg.CORS = &config.CORSConfig{
			Enabled:      true,
			AllowOrigins: origins,
			AllowMethods: []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS", "HEAD"},
			AllowHeaders: []string{"*"},
		}
	}

	// Configure rate limiting if specified
	if f.rateLimit > 0 {
		serverCfg.RateLimit = &config.RateLimitConfig{
			Enabled:           true,
			RequestsPerSecond: f.rateLimit,
			BurstSize:         int(f.rateLimit * 2),
		}
		if serverCfg.RateLimit.BurstSize < 1 {
			serverCfg.RateLimit.BurstSize = 1
		}
	}

	return serverCfg, nil
}

// initializePersistentStore sets up the file store for endpoint persistence.
//
//nolint:unparam // error is always nil but kept for future validation
func initializePersistentStore(sctx *serveContext) error {
	f := sctx.flags

	// When --config is provided, the config file IS the source of truth
	// When no --config, use ~/.local/share/mockd/data.json as the persistent store
	// When --no-persist is set, skip persistent storage entirely
	if f.configFile != "" || f.register || f.pull != "" || f.noPersist {
		return nil
	}

	storeCfg := store.DefaultConfig()
	if f.dataDir != "" {
		storeCfg.DataDir = f.dataDir
	}

	sctx.store = file.New(storeCfg)
	if err := sctx.store.Open(sctx.ctx); err != nil {
		output.Warn("failed to initialize persistent store: %v", err)
		sctx.store = nil
		return nil
	}

	sctx.server.SetStore(sctx.store)

	// Load mocks from store and register protocol handlers (GraphQL, SOAP, etc.)
	if err := sctx.server.LoadFromStore(sctx.ctx); err != nil {
		output.Warn("failed to load from store: %v", err)
	}

	return nil
}

// configureProtocolHandlers builds the chaos config from flags.
// The config is applied to the engine in startServers after the engine is healthy.
// Note: MQTT/gRPC are now started dynamically when mocks are added via the admin API.
func configureProtocolHandlers(sctx *serveContext) error {
	f := sctx.flags

	// Build chaos config from --chaos-profile or individual flags
	if f.chaosProfile != "" {
		profile, _ := chaos.GetProfile(f.chaosProfile) // already validated in validateServeFlags
		apiCfg := chaosProfileToAPIConfig(&profile.Config)
		sctx.chaosConfig = &apiCfg
	} else if f.chaosEnabled {
		cfg := &engineclient.ChaosConfig{Enabled: true}
		if f.chaosLatency != "" {
			min, max := ParseLatencyRange(f.chaosLatency)
			cfg.Latency = &engineclient.LatencyConfig{
				Min:         min,
				Max:         max,
				Probability: 1.0,
			}
		}
		if f.chaosErrorRate > 0 {
			cfg.ErrorRate = &engineclient.ErrorRateConfig{
				Probability: f.chaosErrorRate,
				DefaultCode: 500,
			}
		}
		sctx.chaosConfig = cfg
	}

	return nil
}

// chaosProfileToAPIConfig converts a chaos.ChaosConfig to the API-level
// engineclient.ChaosConfig for applying via the engine control API.
// This mirrors admin/chaos_handlers.go chaosConfigToAPI.
func chaosProfileToAPIConfig(src *chaos.ChaosConfig) engineclient.ChaosConfig {
	cfg := engineclient.ChaosConfig{
		Enabled: src.Enabled,
	}
	if src.GlobalRules != nil {
		if src.GlobalRules.Latency != nil {
			cfg.Latency = &engineclient.LatencyConfig{
				Min:         src.GlobalRules.Latency.Min,
				Max:         src.GlobalRules.Latency.Max,
				Probability: src.GlobalRules.Latency.Probability,
			}
		}
		if src.GlobalRules.ErrorRate != nil {
			cfg.ErrorRate = &engineclient.ErrorRateConfig{
				Probability: src.GlobalRules.ErrorRate.Probability,
				DefaultCode: src.GlobalRules.ErrorRate.DefaultCode,
			}
			if len(src.GlobalRules.ErrorRate.StatusCodes) > 0 {
				cfg.ErrorRate.StatusCodes = make([]int, len(src.GlobalRules.ErrorRate.StatusCodes))
				copy(cfg.ErrorRate.StatusCodes, src.GlobalRules.ErrorRate.StatusCodes)
			}
		}
		if src.GlobalRules.Bandwidth != nil {
			cfg.Bandwidth = &engineclient.BandwidthConfig{
				BytesPerSecond: src.GlobalRules.Bandwidth.BytesPerSecond,
				Probability:    src.GlobalRules.Bandwidth.Probability,
			}
		}
	}
	return cfg
}

// handleOperatingMode handles runtime/pull/local modes and loads mocks.
func handleOperatingMode(sctx *serveContext) error {
	f := sctx.flags

	switch {
	case f.register:
		return handleRuntimeMode(sctx)
	case f.pull != "":
		return handlePullMode(sctx)
	case f.configFile != "":
		return handleLocalMode(sctx)
	default:
		return nil
	}
}

// handleRuntimeMode registers with control plane and sets up heartbeat.
func handleRuntimeMode(sctx *serveContext) error {
	f := sctx.flags

	sctx.runtimeClient = runtime.NewClient(runtime.Config{
		ControlPlaneURL: f.controlPlane,
		Token:           f.token,
		Name:            f.name,
		URL:             fmt.Sprintf("http://localhost:%d", f.port),
		Labels:          parseLabels(f.labels),
		Version:         "dev",
	})

	regResp, err := sctx.runtimeClient.Register(sctx.ctx)
	if err != nil {
		return fmt.Errorf("failed to register with control plane: %w", err)
	}

	fmt.Printf("Registered as runtime %s (ID: %s)\n", regResp.Name, regResp.ID)

	// Pull initial deployments
	if err := sctx.runtimeClient.PullDeployments(sctx.ctx); err != nil {
		output.Warn("failed to pull initial deployments: %v", err)
	}

	// Start heartbeat loop in background
	go func() {
		if err := sctx.runtimeClient.HeartbeatLoop(sctx.ctx); err != nil && sctx.ctx.Err() == nil {
			fmt.Printf("Heartbeat loop error: %v\n", err)
		}
	}()

	return nil
}

// handlePullMode fetches mocks from cloud and loads them.
func handlePullMode(sctx *serveContext) error {
	f := sctx.flags

	pullClient := runtime.NewClient(runtime.Config{
		ControlPlaneURL: f.controlPlane,
		Token:           f.token,
	})

	content, err := pullClient.Pull(sctx.ctx, f.pull)
	if err != nil {
		return fmt.Errorf("failed to pull mocks: %w", err)
	}

	// Cache the content if cache dir specified
	if f.cacheDir != "" {
		if err := cachePulledContent(f.cacheDir, f.pull, content); err != nil {
			output.Warn("failed to cache content: %v", err)
		}
	}

	// Parse pulled content
	collection, err := config.ParseJSON(content)
	if err != nil {
		return fmt.Errorf("failed to parse pulled mocks: %w", err)
	}

	// Import through admin — writes to persistent store + pushes to engine.
	if _, err := sctx.adminAPI.ImportConfigDirect(sctx.ctx, collection, false); err != nil {
		return fmt.Errorf("failed to import pulled mocks: %w", err)
	}

	fmt.Printf("Pulled mocks from %s\n", f.pull)
	return nil
}

// handleLocalMode loads mocks from a local configuration file through the admin
// API so they are written to both the persistent store and the engine.
func handleLocalMode(sctx *serveContext) error {
	// Parse the YAML config file
	collection, err := config.LoadFromFile(sctx.flags.configFile)
	if err != nil {
		return fmt.Errorf("failed to load config file: %w", err)
	}

	// Set the handler's base directory so relative bodyFile paths resolve
	// relative to the config file location.
	baseDir := config.GetMockFileBaseDir(sctx.flags.configFile)
	sctx.server.Handler().SetBaseDir(baseDir)

	// Import through admin — writes to persistent store + pushes to engine.
	if _, err := sctx.adminAPI.ImportConfigDirect(sctx.ctx, collection, false); err != nil {
		return fmt.Errorf("failed to import config: %w", err)
	}
	return nil
}

// startServers starts the mock server and admin API.
func startServers(sctx *serveContext) error {
	f := sctx.flags

	// Start the mock server
	if err := sctx.server.Start(); err != nil {
		if isAddrInUseError(err) {
			return fmt.Errorf("port %d is already in use — try a different port with --port or check what's using it: lsof -i :%d", f.port, f.port)
		}
		return fmt.Errorf("failed to start mock server: %w", err)
	}

	// Create and start the admin API
	engineURL := fmt.Sprintf("http://localhost:%d", sctx.server.ManagementPort())
	adminOpts := []admin.Option{
		admin.WithLocalEngine(engineURL),
		admin.WithWorkspaceManager(engine.NewWorkspaceManager(nil)),
	}
	if f.noAuth {
		adminOpts = append(adminOpts, admin.WithAPIKeyDisabled())
	}
	if f.dataDir != "" {
		adminOpts = append(adminOpts, admin.WithDataDir(f.dataDir))
	}
	if sctx.tracer != nil {
		adminOpts = append(adminOpts, admin.WithTracer(sctx.tracer))
	}

	sctx.adminAPI = admin.NewAPI(f.adminPort, adminOpts...)
	sctx.adminAPI.SetLogger(sctx.log.With("component", "admin"))
	if err := sctx.adminAPI.Start(); err != nil {
		_ = sctx.server.Stop()
		if isAddrInUseError(err) {
			return fmt.Errorf("admin port %d is already in use — try a different port with --admin-port or check what's using it: lsof -i :%d", f.adminPort, f.adminPort)
		}
		return fmt.Errorf("failed to start admin API: %w", err)
	}

	// Wire stream recording to WebSocket and SSE handlers
	// This allows recording sessions started via the admin API to capture frames/events
	if recMgr := sctx.adminAPI.StreamRecordingManager(); recMgr != nil {
		if recStore := recMgr.Store(); recStore != nil {
			hookFactory := recording.NewFileStoreHookFactory(recStore)

			// Wire WebSocket recording
			wsManager := sctx.server.Handler().WebSocketManager()
			wsManager.SetRecordingHookFactory(hookFactory)

			// Wire SSE recording
			sseHandler := sctx.server.Handler().SSEHandler()
			sseHandler.SetRecordingHookFactory(hookFactory.CreateSSEHookFactory())
		}
	}

	// Create engine client and wait for health
	engClient := engineclient.New(engineURL)
	if err := waitForEngineHealth(sctx.ctx, engClient, 10*time.Second); err != nil {
		_ = sctx.server.Stop()
		_ = sctx.adminAPI.Stop()
		return fmt.Errorf("engine control API did not become healthy: %w", err)
	}

	// Apply chaos config to engine (from --chaos-profile or --chaos-enabled flags)
	if sctx.chaosConfig != nil {
		if err := engClient.SetChaos(sctx.ctx, sctx.chaosConfig); err != nil {
			_ = sctx.server.Stop()
			_ = sctx.adminAPI.Stop()
			return fmt.Errorf("failed to apply chaos config: %w", err)
		}
		if sctx.flags.chaosProfile != "" {
			fmt.Printf("Chaos profile %q applied\n", sctx.flags.chaosProfile)
		} else {
			fmt.Println("Chaos injection enabled")
		}
	}

	return nil
}

// postStartup runs after both servers are up and mocks are loaded.
// It writes the PID file, starts MCP, and prints the startup message.
func postStartup(sctx *serveContext) error {
	f := sctx.flags

	engineURL := fmt.Sprintf("http://localhost:%d", sctx.server.ManagementPort())
	engClient := engineclient.New(engineURL)

	// Write PID file for process management (both foreground and detach modes)
	if f.pidFile != "" {
		mocksCount, _ := engClient.ListMocks(sctx.ctx)
		pidMocksLoaded := len(mocksCount)
		if stateOverview, err := engClient.GetStateOverview(sctx.ctx); err == nil {
			pidMocksLoaded += stateOverview.Total
		}
		if err := writePIDFileForServe(f.pidFile, "dev", f.port, f.httpsPort, f.adminPort, f.configFile, pidMocksLoaded); err != nil {
			_ = sctx.server.Stop()
			_ = sctx.adminAPI.Stop()
			return fmt.Errorf("failed to write PID file: %w", err)
		}
	}

	// Start MCP server if enabled
	if f.mcpEnabled && MCPStartFunc != nil {
		adminURL := fmt.Sprintf("http://localhost:%d", f.adminPort)
		mcpServer, err := MCPStartFunc(
			adminURL,
			f.mcpPort,
			f.mcpAllowRemote,
			sctx.server.StatefulStore(),
			sctx.log.With("component", "mcp"),
		)
		if err != nil {
			_ = sctx.server.Stop()
			_ = sctx.adminAPI.Stop()
			return fmt.Errorf("failed to start MCP server: %w", err)
		}
		sctx.mcpServer = mcpServer
	}

	// Print startup message — count mocks and stateful resources
	mocks, _ := engClient.ListMocks(sctx.ctx)
	mocksLoaded := len(mocks)
	statefulCount := 0
	if stateOverview, err := engClient.GetStateOverview(sctx.ctx); err == nil {
		statefulCount = stateOverview.Total
	}
	printServeStartupMessage(f.port, f.adminPort, f.httpsPort, f.mcpEnabled, f.mcpPort, f.register, f.pull, mocksLoaded, statefulCount)

	return nil
}

// runMainLoop handles the main event loop and graceful shutdown.
func runMainLoop(sctx *serveContext) error {
	f := sctx.flags

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Wait for shutdown signal
	<-sigChan
	fmt.Println("\nShutting down...")

	// Cancel context to stop background goroutines (heartbeat, etc.)
	sctx.cancel()

	// Create shutdown context with timeout
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer shutdownCancel()

	// Remove PID file if it was written
	if f.pidFile != "" {
		if err := RemovePIDFile(f.pidFile); err != nil {
			output.Warn("failed to remove PID file: %v", err)
		}
	}

	// Deregister from control plane if in runtime mode
	if sctx.runtimeClient != nil {
		fmt.Println("Deregistering from control plane...")
		// TODO(FEAT-002): Implement graceful deregistration from control plane
		// This should:
		// 1. Send a deregistration request to the control plane API
		// 2. Wait for acknowledgment with a timeout (e.g., 5 seconds)
		// 3. Handle errors gracefully - log but don't block shutdown
		// See: runtimeClient should expose a Deregister() method
	}

	// Stop MCP server if running
	if sctx.mcpServer != nil {
		if err := sctx.mcpServer.Stop(); err != nil {
			output.Warn("MCP server shutdown error: %v", err)
		}
	}

	// Stop admin API first (uses internal 5s timeout)
	if sctx.adminAPI != nil {
		if err := sctx.adminAPI.Stop(); err != nil {
			output.Warn("admin API shutdown error: %v", err)
		}
	}

	// Stop mock server (uses internal 5s timeout)
	if sctx.server != nil {
		if err := sctx.server.Stop(); err != nil {
			output.Warn("server shutdown error: %v", err)
		}
	}

	// Stop MQTT broker if running
	if sctx.mqttBroker != nil {
		if err := sctx.mqttBroker.Stop(shutdownCtx, shutdownTimeout); err != nil {
			output.Warn("MQTT broker shutdown error: %v", err)
		}
	}

	// Shutdown tracer (flush remaining spans)
	if sctx.tracer != nil {
		if err := sctx.tracer.Shutdown(shutdownCtx); err != nil {
			output.Warn("tracer shutdown error: %v", err)
		}
	}

	fmt.Println("Server stopped")
	return nil
}

// parseLabels parses comma-separated key=value pairs into a map.
func parseLabels(labelsStr string) map[string]string {
	if labelsStr == "" {
		return nil
	}

	labels := make(map[string]string)
	pairs := strings.Split(labelsStr, ",")
	for _, pair := range pairs {
		kv := strings.SplitN(strings.TrimSpace(pair), "=", 2)
		if len(kv) == 2 {
			labels[strings.TrimSpace(kv[0])] = strings.TrimSpace(kv[1])
		}
	}
	return labels
}

// cachePulledContent caches pulled content to the local filesystem.
func cachePulledContent(cacheDir, uri string, content []byte) error {
	// TODO: Implement local caching
	_ = cacheDir
	_ = uri
	_ = content
	return nil
}

// printServeStartupMessage prints the server startup information.
func printServeStartupMessage(httpPort, adminPort, httpsPort int, mcpEnabled bool, mcpPort int, isRuntime bool, pullURI string, mocksLoaded, statefulCount int) {
	if mocksLoaded == 0 && statefulCount == 0 && !isRuntime && pullURI == "" {
		// No mocks configured - show welcome message
		printWelcomeMessage(httpPort, adminPort)
	} else {
		// Normal startup message
		parts := []string{}
		if mocksLoaded > 0 {
			parts = append(parts, fmt.Sprintf("%d mocks", mocksLoaded))
		}
		if statefulCount > 0 {
			parts = append(parts, fmt.Sprintf("%d stateful resources", statefulCount))
		}
		if len(parts) == 0 {
			parts = append(parts, "no mocks")
		}
		fmt.Printf("mockd server started (%s)\n", strings.Join(parts, ", "))
		fmt.Println()
		fmt.Printf("  Mock server: http://localhost:%d\n", httpPort)
		if httpsPort > 0 {
			fmt.Printf("  HTTPS:       https://localhost:%d\n", httpsPort)
		}
		fmt.Printf("  Admin API:   http://localhost:%d\n", adminPort)
		if mcpEnabled {
			fmt.Printf("  MCP server:  http://localhost:%d/mcp\n", mcpPort)
		}
		fmt.Println()

		if isRuntime {
			fmt.Println("Mode: Runtime (connected to control plane)")
		} else if pullURI != "" {
			fmt.Printf("Mode: Pull (serving from %s)\n", pullURI)
		}

		fmt.Println("Press Ctrl+C to stop")
	}
}

// printWelcomeMessage prints a helpful welcome message when starting with no mocks.
func printWelcomeMessage(mockPort, adminPort int) {
	fmt.Println("mockd server started")
	fmt.Println()
	fmt.Printf("  Mock server: http://localhost:%d\n", mockPort)
	fmt.Printf("  Admin API:   http://localhost:%d\n", adminPort)
	fmt.Println()
	fmt.Println("No mocks configured. Quick start options:")
	fmt.Println()
	fmt.Println("  # Create a config file")
	fmt.Println("  mockd init")
	fmt.Println("  mockd serve --config mockd.yaml")
	fmt.Println()
	fmt.Println("  # Or add a mock directly")
	fmt.Printf("  mockd add --path /hello --body '{\"message\": \"Hello!\"}'\n")
	fmt.Printf("  curl http://localhost:%d/hello\n", mockPort)
	fmt.Println()
	fmt.Println("Press Ctrl+C to stop")
}

// daemonize re-executes the current process as a background daemon.
func daemonize(_ []string, pidFilePath string, httpPort, adminPort int) error {
	// Build the command with same arguments
	cmd := exec.Command(os.Args[0], os.Args[1:]...)
	cmd.Env = append(os.Environ(), "MOCKD_CHILD=1")

	// Detach from terminal
	cmd.Stdout = nil
	cmd.Stderr = nil
	cmd.Stdin = nil

	// Start the child process
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start daemon: %w", err)
	}

	// Wait briefly for child to start and write PID file
	time.Sleep(500 * time.Millisecond)

	// Verify the daemon started by checking PID file
	pidInfo, err := ReadPIDFile(pidFilePath)
	if err != nil {
		output.Warn("daemon may have failed to start (could not read PID file: %v)", err)
		return nil
	}

	if !pidInfo.IsRunning() {
		return errors.New("daemon process exited immediately")
	}

	// Print success message
	fmt.Printf("mockd started in background (PID %d)\n", pidInfo.PID)
	fmt.Printf("Admin API:   http://localhost:%d\n", adminPort)
	fmt.Printf("Mock server: http://localhost:%d\n", httpPort)

	// Show loaded mocks count if any
	if pidInfo.Config.MocksLoaded > 0 {
		fmt.Printf("Loaded %d mocks\n", pidInfo.Config.MocksLoaded)
	}

	return nil
}

// writePIDFileForServe writes the PID file with server component information.
func writePIDFileForServe(pidFilePath string, version string, httpPort, httpsPort, adminPort int, configFile string, mocksLoaded int) error {
	pidInfo := &PIDFile{
		PID:       os.Getpid(),
		StartTime: time.Now(),
		Version:   version,
		Components: ComponentsInfo{
			Admin: ComponentStatus{
				Enabled: true,
				Port:    adminPort,
				Host:    "localhost",
			},
			Engine: ComponentStatus{
				Enabled:   true,
				Port:      httpPort,
				Host:      "localhost",
				HTTPSPort: httpsPort,
			},
		},
		Config: ConfigInfo{
			File:        configFile,
			MocksLoaded: mocksLoaded,
		},
	}

	return WritePIDFile(pidFilePath, pidInfo)
}

// initializeTracer creates a new tracer with OTLP exporter for distributed tracing.
func initializeTracer(endpoint string, samplerRatio float64) *tracing.Tracer {
	exporter := tracing.NewOTLPExporter(endpoint)

	var sampler tracing.Sampler
	switch {
	case samplerRatio >= 1.0:
		sampler = tracing.AlwaysSample{}
	case samplerRatio <= 0:
		sampler = tracing.NeverSample{}
	default:
		sampler = tracing.NewRatioSampler(samplerRatio)
	}

	return tracing.NewTracer("mockd",
		tracing.WithExporter(exporter),
		tracing.WithSampler(sampler),
		tracing.WithBatchSize(10), // Export spans in smaller batches for lower latency
	)
}
