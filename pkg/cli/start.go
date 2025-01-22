package cli

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/getmockd/mockd/pkg/admin"
	"github.com/getmockd/mockd/pkg/admin/engineclient"
	"github.com/getmockd/mockd/pkg/cli/internal/ports"
	"github.com/getmockd/mockd/pkg/config"
	"github.com/getmockd/mockd/pkg/engine"
	"github.com/getmockd/mockd/pkg/store"
	"github.com/getmockd/mockd/pkg/store/file"
)

// RunStart handles the start command.
func RunStart(args []string) error {
	fs := flag.NewFlagSet("start", flag.ContinueOnError)

	// Use shared server flags
	var sf ServerFlags
	RegisterServerFlags(fs, &sf)

	// Start-specific flags
	loadDir := fs.String("load", "", "Load mocks from directory")
	watch := fs.Bool("watch", false, "Watch for file changes (with --load)")
	validate := fs.Bool("validate", false, "Validate files before serving (with --load)")

	// Engine mode flags
	engineName := fs.String("engine-name", "", "Name for this engine when registering with admin")
	adminURL := fs.String("admin-url", "", "Admin server URL to register with (enables engine mode)")

	fs.Usage = func() {
		fmt.Fprint(os.Stderr, `Usage: mockd start [flags]

Start the mock server.

Flags:
  -p, --port          HTTP server port (default: 4280)
  -a, --admin-port    Admin API port (default: 4290)
  -c, --config        Path to mock configuration file
      --load          Load mocks from directory
      --watch         Watch for file changes (with --load)
      --validate      Validate files before serving (with --load)
      --https-port    HTTPS server port (0 = disabled)
      --read-timeout  Read timeout in seconds (default: 30)
      --write-timeout Write timeout in seconds (default: 30)
      --max-log-entries Maximum request log entries (default: 1000)
      --auto-cert     Auto-generate TLS certificate (default: true)

TLS flags:
      --tls-cert      Path to TLS certificate file
      --tls-key       Path to TLS private key file
      --tls-auto      Auto-generate self-signed certificate

mTLS flags:
      --mtls-enabled  Enable mTLS client certificate validation
      --mtls-client-auth Client auth mode (none, request, require, verify-if-given, require-and-verify)
      --mtls-ca       Path to CA certificate for client validation
      --mtls-allowed-cns Comma-separated list of allowed Common Names

Audit flags:
      --audit-enabled Enable audit logging
      --audit-file    Path to audit log file
      --audit-level   Log level (debug, info, warn, error)

GraphQL flags:
      --graphql-schema Path to GraphQL schema file
      --graphql-path   GraphQL endpoint path (default: /graphql)

OAuth flags:
      --oauth-enabled   Enable OAuth provider
      --oauth-issuer    OAuth issuer URL
      --oauth-port      OAuth server port

Chaos flags:
      --chaos-enabled   Enable chaos injection
      --chaos-latency   Add random latency (e.g., "10ms-100ms")
      --chaos-error-rate Error rate (0.0-1.0)

Validation flags:
      --validate-spec   Path to OpenAPI spec for request validation
      --validate-fail   Fail on validation error (default: false)

Storage flags:
      --data-dir      Data directory for persistent storage (default: ~/.local/share/mockd)
      --no-auth       Disable API key authentication on admin API

Examples:
  # Start with defaults
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
  mockd start --engine-name "dev-engine" --admin-url http://admin.example.com:4290

  # Start with TLS using certificate files
  mockd start --tls-cert server.crt --tls-key server.key --https-port 8443

  # Start with mTLS enabled
  mockd start --mtls-enabled --mtls-ca ca.crt --tls-cert server.crt --tls-key server.key

  # Start with audit logging
  mockd start --audit-enabled --audit-file audit.log --audit-level debug
`)
	}

	if err := fs.Parse(args); err != nil {
		return err
	}

	// Validate --watch requires --load
	if *watch && *loadDir == "" {
		return fmt.Errorf("--watch requires --load to be specified")
	}

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
			fmt.Fprintf(os.Stderr, "Warning: failed to initialize persistent store: %v\n", err)
		} else {
			server.SetStore(persistentStore)
			// Ensure store is closed on shutdown
			defer persistentStore.Close()
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
		return fmt.Errorf("failed to start mock server: %w", err)
	}

	// Create engine client for loading mocks via HTTP
	engineURL := fmt.Sprintf("http://localhost:%d", server.ManagementPort())
	engClient := engineclient.New(engineURL)

	// Wait for engine management API to be healthy before loading mocks
	ctx := context.Background()
	if err := waitForEngineHealth(ctx, engClient, 10*time.Second); err != nil {
		server.Stop()
		return fmt.Errorf("engine management API did not become healthy: %w", err)
	}

	// Load mocks from directory if specified
	var dirLoader *config.DirectoryLoader
	if *loadDir != "" {
		dirLoader = config.NewDirectoryLoader(*loadDir)

		// Validate files if requested
		if *validate {
			errors, err := dirLoader.Validate()
			if err != nil {
				return fmt.Errorf("failed to validate directory: %w", err)
			}
			if len(errors) > 0 {
				fmt.Fprintf(os.Stderr, "Validation errors:\n")
				for _, e := range errors {
					fmt.Fprintf(os.Stderr, "  - %s\n", e.Error())
				}
				return fmt.Errorf("validation failed with %d errors", len(errors))
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
				fmt.Fprintf(os.Stderr, "Warning: failed to add mock %s: %v\n", mock.ID, err)
			}
		}

		fmt.Printf("Loaded %d mocks from %d files in %s\n", len(result.Collection.Mocks), result.FileCount, *loadDir)
	}

	// Start file watcher if requested
	if *watch && dirLoader != nil {
		watcher := config.NewWatcher(dirLoader)
		eventCh := watcher.Start()
		go handleWatchEvents(eventCh, dirLoader, engClient)
		fmt.Println("Watching for file changes...")
	}

	// Create and start the admin API
	adminOpts := []admin.Option{admin.WithLocalEngine(engineURL)}
	if sf.NoAuth {
		adminOpts = append(adminOpts, admin.WithAPIKeyDisabled())
	}
	if sf.DataDir != "" {
		adminOpts = append(adminOpts, admin.WithDataDir(sf.DataDir))
	}
	adminAPI := admin.NewAdminAPI(sf.AdminPort, adminOpts...)
	if err := adminAPI.Start(); err != nil {
		server.Stop()
		return fmt.Errorf("failed to start admin API: %w", err)
	}

	// Register with remote admin if engine mode is enabled
	if *adminURL != "" {
		name := *engineName
		if name == "" {
			hostname, _ := os.Hostname()
			name = fmt.Sprintf("engine-%s", hostname)
		}

		// Create workspace manager for serving remote workspaces
		wsManager := engine.NewWorkspaceManager(nil)

		// Create engine client to communicate with admin
		engineClient := engine.NewEngineClient(&engine.EngineClientConfig{
			AdminURL:     *adminURL,
			EngineName:   name,
			LocalPort:    sf.Port,
			PollInterval: 10 * time.Second,
		}, wsManager)

		// Start the engine client (registers and starts polling)
		ctx := context.Background()
		if err := engineClient.Start(ctx); err != nil {
			return fmt.Errorf("failed to start engine client: %w", err)
		}

		// Ensure cleanup on shutdown
		defer engineClient.Stop()
		defer wsManager.StopAll()

		fmt.Printf("Engine mode: connected to admin at %s\n", *adminURL)
	}

	// Print startup message
	printStartupMessage(sf.Port, sf.AdminPort, sf.HTTPSPort)

	// Wait for shutdown signal using shared function
	WaitForShutdown(server, adminAPI)

	return nil
}

// formatPortError formats a port conflict error with suggestions.
func formatPortError(port int, err error) error {
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
				fmt.Fprintf(os.Stderr, "Warning: failed to update mock %s: %v\n", mock.ID, err)
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
