package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/getmockd/mockd/pkg/admin"
	"github.com/getmockd/mockd/pkg/admin/engineclient"
	"github.com/getmockd/mockd/pkg/cli/internal/output"
	"github.com/getmockd/mockd/pkg/cli/internal/ports"
	"github.com/getmockd/mockd/pkg/config"
	"github.com/getmockd/mockd/pkg/engine"
	"github.com/getmockd/mockd/pkg/logging"
	"github.com/getmockd/mockd/pkg/recording"
	"github.com/getmockd/mockd/pkg/store"
	"github.com/getmockd/mockd/pkg/store/file"
	"github.com/spf13/cobra"
)

var (
	startLoadDir     string
	startWatch       bool
	startValidate    bool
	startEngineName  string
	startRegisterURL string
	startLogLevel    string
	startLogFormat   string
	startDetach      bool
	startPidFile     string
	startServerFlags ServerFlags
)

// startCmd represents the start command
var startCmd = &cobra.Command{
	Use:   "start",
	Short: "Start the mock server",
	Long: `Start the mock server.

By default, the server starts on port 4280 and the admin API on port 4290.
You can configure the server using flags, environment variables, or a config file.`,
	Example: `  # Start with defaults
  mockd start

  # Start with config file on custom port
  mockd start --config mocks.json --port 3000

  # Start with HTTPS enabled
  mockd start --https-port 8443

  # Load mocks from directory
  mockd start --load ./mocks/

  # Load with hot reload
  mockd start --load ./mocks/ --watch

  # Validate mocks before serving
  mockd start --load ./mocks/ --validate

  # Start in engine mode, registering with an admin server
  mockd start --engine-name "dev-engine" --register-url http://admin.example.com:4290

  # Start with TLS using certificate files
  mockd start --tls-cert server.crt --tls-key server.key --https-port 8443

  # Start with mTLS enabled
  mockd start --mtls-enabled --mtls-ca ca.crt --tls-cert server.crt --tls-key server.key

  # Start with audit logging
  mockd start --audit-enabled --audit-file audit.log --audit-level debug`,
	RunE: runStart,
}

func init() {
	rootCmd.AddCommand(startCmd)

	// Since RegisterServerFlags takes a flag.FlagSet, we'll manually bind them for now,
	// or create a helper for Cobra flags. For speed, mapping directly here:
	// Server Flags
	startCmd.Flags().IntVarP(&startServerFlags.Port, "port", "p", 4280, "HTTP server port")
	startCmd.Flags().IntVarP(&startServerFlags.AdminPort, "admin-port", "a", 4290, "Admin API port")
	startCmd.Flags().StringVarP(&startServerFlags.ConfigFile, "config", "c", "", "Path to mock configuration file")
	startCmd.Flags().IntVar(&startServerFlags.HTTPSPort, "https-port", 0, "HTTPS server port (0 = disabled)")
	startCmd.Flags().IntVar(&startServerFlags.ReadTimeout, "read-timeout", 30, "Read timeout in seconds")
	startCmd.Flags().IntVar(&startServerFlags.WriteTimeout, "write-timeout", 30, "Write timeout in seconds")
	startCmd.Flags().IntVar(&startServerFlags.MaxLogEntries, "max-log-entries", 1000, "Maximum request log entries")
	startCmd.Flags().BoolVar(&startServerFlags.AutoCert, "auto-cert", true, "Auto-generate TLS certificate")

	// TLS flags
	startCmd.Flags().StringVar(&startServerFlags.TLSCert, "tls-cert", "", "Path to TLS certificate file")
	startCmd.Flags().StringVar(&startServerFlags.TLSKey, "tls-key", "", "Path to TLS private key file")
	startCmd.Flags().BoolVar(&startServerFlags.TLSAuto, "tls-auto", false, "Auto-generate self-signed certificate")

	// mTLS flags
	startCmd.Flags().BoolVar(&startServerFlags.MTLSEnabled, "mtls-enabled", false, "Enable mTLS client certificate validation")
	startCmd.Flags().StringVar(&startServerFlags.MTLSClientAuth, "mtls-client-auth", "require-and-verify", "Client auth mode (none, request, require, verify-if-given, require-and-verify)")
	startCmd.Flags().StringVar(&startServerFlags.MTLSCA, "mtls-ca", "", "Path to CA certificate for client validation")
	// (Note: custom StringSlice variable for allowed CNs is string typed in the struct, so use string directly)
	startCmd.Flags().StringVar(&startServerFlags.MTLSAllowedCNs, "mtls-allowed-cns", "", "Comma-separated list of allowed Common Names")

	// Audit flags
	startCmd.Flags().BoolVar(&startServerFlags.AuditEnabled, "audit-enabled", false, "Enable audit logging")
	startCmd.Flags().StringVar(&startServerFlags.AuditFile, "audit-file", "", "Path to audit log file")
	startCmd.Flags().StringVar(&startServerFlags.AuditLevel, "audit-level", "info", "Audit log level")

	// GraphQL flags
	startCmd.Flags().StringVar(&startServerFlags.GraphQLSchema, "graphql-schema", "", "Path to GraphQL schema file")
	startCmd.Flags().StringVar(&startServerFlags.GraphQLPath, "graphql-path", "/graphql", "GraphQL endpoint path")

	// OAuth flags
	startCmd.Flags().BoolVar(&startServerFlags.OAuthEnabled, "oauth-enabled", false, "Enable OAuth provider")
	startCmd.Flags().StringVar(&startServerFlags.OAuthIssuer, "oauth-issuer", "", "OAuth issuer URL")
	startCmd.Flags().IntVar(&startServerFlags.OAuthPort, "oauth-port", 0, "OAuth server port")

	// Chaos flags
	startCmd.Flags().BoolVar(&startServerFlags.ChaosEnabled, "chaos-enabled", false, "Enable chaos injection")
	startCmd.Flags().StringVar(&startServerFlags.ChaosLatency, "chaos-latency", "", "Add random latency (e.g., '10ms-100ms')")
	startCmd.Flags().Float64Var(&startServerFlags.ChaosErrorRate, "chaos-error-rate", 0, "Error rate (0.0-1.0)")

	// Validation flags
	startCmd.Flags().StringVar(&startServerFlags.ValidateSpec, "validate-spec", "", "Path to OpenAPI spec for request validation")
	startCmd.Flags().BoolVar(&startServerFlags.ValidateFail, "validate-fail", false, "Fail on validation error")

	// Storage flags
	startCmd.Flags().StringVar(&startServerFlags.DataDir, "data-dir", "", "Data directory for persistent storage (default: ~/.local/share/mockd)")
	startCmd.Flags().BoolVar(&startServerFlags.NoAuth, "no-auth", false, "Disable API key authentication on admin API")

	// Start-specific flags
	startCmd.Flags().StringVar(&startLoadDir, "load", "", "Load mocks from directory")
	startCmd.Flags().BoolVar(&startWatch, "watch", false, "Watch for file changes (with --load)")
	startCmd.Flags().BoolVar(&startValidate, "validate", false, "Validate files before serving (with --load)")
	startCmd.Flags().StringVar(&startEngineName, "engine-name", "", "Name for this engine when registering with admin")
	startCmd.Flags().StringVar(&startRegisterURL, "register-url", "", "Admin server URL to register with (enables engine mode)")
	startCmd.Flags().StringVar(&startLogLevel, "log-level", "info", "Log level (debug, info, warn, error)")
	startCmd.Flags().StringVar(&startLogFormat, "log-format", "text", "Log format (text, json)")
	startCmd.Flags().BoolVarP(&startDetach, "detach", "d", false, "Run server in background (daemon mode)")
	startCmd.Flags().StringVar(&startPidFile, "pid-file", DefaultPIDPath(), "Path to PID file")
}

//nolint:gocyclo
func runStart(cmd *cobra.Command, args []string) error {
	sf := startServerFlags

	// Handle detach mode (daemon) - re-exec as child and exit
	if startDetach && os.Getenv("MOCKD_CHILD") == "" {
		return daemonize(args, startPidFile, sf.Port, sf.AdminPort)
	}

	// Validate --watch requires --load
	if startWatch && startLoadDir == "" {
		return errors.New("--watch requires --load to be specified")
	}

	// Initialize structured logger
	log := logging.New(logging.Config{
		Level:  logging.ParseLevel(startLogLevel),
		Format: logging.ParseFormat(startLogFormat),
	})

	// Check for port conflicts
	if err := ports.Check(sf.Port); err != nil {
		return formatPortError(sf.Port, err)
	}
	if err := ports.Check(sf.AdminPort); err != nil {
		return formatPortError(sf.AdminPort, err)
	}
	if sf.HTTPSPort > 0 {
		if err := ports.Check(sf.HTTPSPort); err != nil {
			return formatPortError(sf.HTTPSPort, err)
		}
	}

	// Build server configuration using shared builders
	serverCfg := BuildServerConfig(&sf)

	// Configure chaos if enabled
	if chaosCfg := BuildChaosConfig(&sf); chaosCfg != nil {
		serverCfg.Chaos = chaosCfg
		fmt.Println("Chaos injection enabled")
	}

	// Create and start the mock server
	server := engine.NewServer(serverCfg)

	// Initialize persistent store for endpoint persistence (GraphQL, gRPC, SOAP, MQTT, etc.)
	// Skip persistent store when --config is provided to avoid loading stale mocks
	var persistentStore *file.FileStore
	if sf.ConfigFile == "" {
		// Only use persistent store when no config file is specified
		storeCfg := store.DefaultConfig()
		if sf.DataDir != "" {
			storeCfg.DataDir = sf.DataDir
		}
		persistentStore = file.New(storeCfg)
		if err := persistentStore.Open(context.Background()); err != nil {
			output.Warn("failed to initialize persistent store: %v", err)
		} else {
			server.SetStore(persistentStore)
			// Ensure store is closed on shutdown
			defer func() { _ = persistentStore.Close() }()
		}
	}

	// Load config file if specified
	if sf.ConfigFile != "" {
		if err := server.LoadConfig(sf.ConfigFile, false); err != nil {
			return fmt.Errorf("failed to load config file: %w", err)
		}
	}

	// Start the mock server first (before loading mocks via HTTP)
	if err := server.Start(); err != nil {
		if isAddrInUseError(err) {
			return fmt.Errorf("port %d is already in use — try a different port with --port or check what's using it: lsof -i :%d", sf.Port, sf.Port)
		}
		return fmt.Errorf("failed to start mock server: %w", err)
	}

	// Create engine client for loading mocks via HTTP
	engineURL := fmt.Sprintf("http://localhost:%d", server.ManagementPort())
	engClient := engineclient.New(engineURL)

	// Wait for engine management API to be healthy before loading mocks
	ctx := context.Background()
	if err := waitForEngineHealth(ctx, engClient, 10*time.Second); err != nil {
		_ = server.Stop()
		return fmt.Errorf("engine management API did not become healthy: %w", err)
	}

	// Load mocks from directory if specified
	var dirLoader *config.DirectoryLoader
	if startLoadDir != "" {
		dirLoader = config.NewDirectoryLoader(startLoadDir)

		// Validate files if requested
		if startValidate {
			validationErrs, err := dirLoader.Validate()
			if err != nil {
				return fmt.Errorf("failed to validate directory: %w", err)
			}
			if len(validationErrs) > 0 {
				fmt.Fprintf(os.Stderr, "Validation errors:\n")
				for _, e := range validationErrs {
					fmt.Fprintf(os.Stderr, "  - %s\n", e.Error())
				}
				return fmt.Errorf("validation failed with %d errors", len(validationErrs))
			}
		}

		// Load mocks
		result, err := dirLoader.Load()
		if err != nil {
			return fmt.Errorf("failed to load from directory: %w", err)
		}

		// Report any non-fatal errors
		if len(result.Errors) > 0 {
			fmt.Fprintf(os.Stderr, "Warnings while loading:\n")
			for _, e := range result.Errors {
				fmt.Fprintf(os.Stderr, "  - %s\n", e.Error())
			}
		}

		// Add loaded mocks to engine via HTTP
		for _, mock := range result.Collection.Mocks {
			if _, err := engClient.CreateMock(ctx, mock); err != nil {
				output.Warn("failed to add mock %s: %v", mock.ID, err)
			}
		}

		fmt.Printf("Loaded %d mocks from %d files in %s\n", len(result.Collection.Mocks), result.FileCount, startLoadDir)
	}

	// Start file watcher if requested
	var watcher *config.Watcher
	if startWatch && dirLoader != nil {
		watcher = config.NewWatcher(dirLoader)
		eventCh := watcher.Start()
		go handleWatchEvents(eventCh, dirLoader, engClient)
		fmt.Println("Watching for file changes...")
	}
	// Ensure watcher cleanup on shutdown
	defer func() {
		if watcher != nil {
			watcher.Stop()
		}
	}()

	// Create and start the admin API
	adminOpts := []admin.Option{
		admin.WithLocalEngine(engineURL),
		admin.WithWorkspaceManager(engine.NewWorkspaceManager(nil)),
	}
	if sf.NoAuth {
		adminOpts = append(adminOpts, admin.WithAPIKeyDisabled())
	}
	if sf.DataDir != "" {
		adminOpts = append(adminOpts, admin.WithDataDir(sf.DataDir))
	}
	adminAPI := admin.NewAPI(sf.AdminPort, adminOpts...)
	adminAPI.SetLogger(log.With("component", "admin"))
	if err := adminAPI.Start(); err != nil {
		_ = server.Stop()
		if isAddrInUseError(err) {
			return fmt.Errorf("admin port %d is already in use — try a different port with --admin-port or check what's using it: lsof -i :%d", sf.AdminPort, sf.AdminPort)
		}
		return fmt.Errorf("failed to start admin API: %w", err)
	}

	// Wire stream recording to WebSocket and SSE handlers
	if recMgr := adminAPI.StreamRecordingManager(); recMgr != nil {
		if recStore := recMgr.Store(); recStore != nil {
			hookFactory := recording.NewFileStoreHookFactory(recStore)
			wsManager := server.Handler().WebSocketManager()
			wsManager.SetRecordingHookFactory(hookFactory)
			sseHandler := server.Handler().SSEHandler()
			sseHandler.SetRecordingHookFactory(hookFactory.CreateSSEHookFactory())
		}
	}

	// Register with remote admin if engine mode is enabled
	if startRegisterURL != "" {
		name := startEngineName
		if name == "" {
			hostname, _ := os.Hostname()
			name = "engine-" + hostname
		}

		// Create workspace manager for serving remote workspaces
		wsManager := engine.NewWorkspaceManager(nil)

		// Create engine client to communicate with admin
		engineClient := engine.NewEngineClient(&engine.EngineClientConfig{
			AdminURL:     startRegisterURL,
			EngineName:   name,
			// Register the engine control API port so the admin can address the engine.
			LocalPort:    server.ManagementPort(),
			PollInterval: 10 * time.Second,
		}, wsManager)

		// Start the engine client (registers and starts polling)
		if err := engineClient.Start(ctx); err != nil {
			return fmt.Errorf("failed to start engine client: %w", err)
		}

		// Ensure cleanup on shutdown
		defer engineClient.Stop()
		defer func() { _ = wsManager.StopAll() }()

		fmt.Printf("Engine mode: connected to admin at %s\n", startRegisterURL)
	}

	// Write PID file for process management
	if startPidFile != "" {
		mocksCount, _ := engClient.ListMocks(ctx)
		pidMocksLoaded := len(mocksCount)
		if stateOverview, err := engClient.GetStateOverview(ctx); err == nil {
			pidMocksLoaded += stateOverview.Total
		}
		if err := writePIDFileForServe(startPidFile, "dev", sf.Port, sf.HTTPSPort, sf.AdminPort, sf.ConfigFile, pidMocksLoaded); err != nil {
			output.Warn("failed to write PID file: %v", err)
		}
		defer func() {
			if err := RemovePIDFile(startPidFile); err != nil {
				output.Warn("failed to remove PID file: %v", err)
			}
		}()
	}

	// Print startup message
	printStartupMessage(sf.Port, sf.AdminPort, sf.HTTPSPort)

	// Wait for shutdown signal using shared function
	WaitForShutdown(server, adminAPI)

	return nil
}

// formatPortError formats a port availability error with suggestions.
func formatPortError(port int, err error) error {
	if err != nil {
		if isPermissionDeniedError(err) {
			return fmt.Errorf("could not bind port %d to check availability: %v", port, err)
		}
		if !isAddrInUseError(err) {
			return fmt.Errorf("failed to check port %d availability: %w", port, err)
		}
	}

	return fmt.Errorf(`port %d already in use

Suggestions:
  - Use a different port: mockd start --port %d
  - Check what's using the port: lsof -i :%d
  - Stop the other process and try again`, port, port+1, port)
}

// printStartupMessage prints the server startup information.
func printStartupMessage(httpPort, adminPort, httpsPort int) {
	fmt.Printf("Mock server running on http://localhost:%d\n", httpPort)
	if httpsPort > 0 {
		fmt.Printf("HTTPS server running on https://localhost:%d\n", httpsPort)
	}
	fmt.Printf("Admin API running on http://localhost:%d\n", adminPort)
	fmt.Println("Press Ctrl+C to stop")
}

// handleWatchEvents processes file change events from the watcher.
func handleWatchEvents(eventCh <-chan config.WatchEvent, loader *config.DirectoryLoader, engClient *engineclient.Client) {
	ctx := context.Background()
	for event := range eventCh {
		if event.Error != nil {
			fmt.Fprintf(os.Stderr, "Watch error: %v\n", event.Error)
			continue
		}

		fmt.Printf("File changed: %s (%s)\n", event.Path, event.Type)

		// Reload the changed file
		collection, err := loader.ReloadFile(event.Path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to reload %s: %v\n", event.Path, err)
			continue
		}

		// Update mocks in engine via HTTP
		for _, mock := range collection.Mocks {
			if _, err := engClient.CreateMock(ctx, mock); err != nil {
				output.Warn("failed to update mock %s: %v", mock.ID, err)
			}
		}

		fmt.Printf("Reloaded %d mocks from %s\n", len(collection.Mocks), event.Path)
	}
}

// waitForEngineHealth waits for the engine control API to become healthy.
func waitForEngineHealth(ctx context.Context, client *engineclient.Client, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if err := client.Health(ctx); err == nil {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(100 * time.Millisecond):
			// retry
		}
	}
	return fmt.Errorf("timeout waiting for engine health after %v", timeout)
}
