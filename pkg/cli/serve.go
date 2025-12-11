package cli

import (
	"context"
	"flag"
	"fmt"
	"net"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/getmockd/mockd/internal/cliconfig"
	"github.com/getmockd/mockd/internal/runtime"
	"github.com/getmockd/mockd/pkg/admin"
	"github.com/getmockd/mockd/pkg/config"
	"github.com/getmockd/mockd/pkg/engine"
)

// RunServe handles the serve command (enhanced start with runtime mode support).
func RunServe(args []string) error {
	fs := flag.NewFlagSet("serve", flag.ContinueOnError)

	// Standard server flags
	port := fs.Int("port", cliconfig.DefaultPort, "HTTP server port")
	fs.IntVar(port, "p", cliconfig.DefaultPort, "HTTP server port (shorthand)")

	adminPort := fs.Int("admin-port", cliconfig.DefaultAdminPort, "Admin API port")
	fs.IntVar(adminPort, "a", cliconfig.DefaultAdminPort, "Admin API port (shorthand)")

	configFile := fs.String("config", "", "Path to mock configuration file")
	fs.StringVar(configFile, "c", "", "Path to mock configuration file (shorthand)")

	httpsPort := fs.Int("https-port", cliconfig.DefaultHTTPSPort, "HTTPS server port (0 = disabled)")
	readTimeout := fs.Int("read-timeout", cliconfig.DefaultReadTimeout, "Read timeout in seconds")
	writeTimeout := fs.Int("write-timeout", cliconfig.DefaultWriteTimeout, "Write timeout in seconds")
	maxLogEntries := fs.Int("max-log-entries", cliconfig.DefaultMaxLogEntries, "Maximum request log entries")
	autoCert := fs.Bool("auto-cert", cliconfig.DefaultAutoCert, "Auto-generate TLS certificate")

	// Runtime mode flags
	register := fs.Bool("register", false, "Register with control plane as a runtime")
	controlPlane := fs.String("control-plane", "https://api.mockd.io", "Control plane URL")
	token := fs.String("token", "", "Runtime token (or set MOCKD_RUNTIME_TOKEN env var)")
	name := fs.String("name", "", "Runtime name (required with --register)")
	labels := fs.String("labels", "", "Runtime labels as key=value pairs (comma-separated)")

	// Pull mode flags
	pull := fs.String("pull", "", "Pull and serve mocks from mockd:// URI")
	cacheDir := fs.String("cache", "", "Local cache directory for pulled mocks")

	fs.Usage = func() {
		fmt.Fprint(os.Stderr, `Usage: mockd serve [flags]

Start the mock server. Can operate in three modes:

1. Local mode (default): Serve mocks from local configuration
2. Runtime mode (--register): Register with control plane and receive deployments
3. Pull mode (--pull): Pull mocks from cloud and serve locally

Flags:
  -p, --port          HTTP server port (default: 8080)
  -a, --admin-port    Admin API port (default: 9090)
  -c, --config        Path to mock configuration file
      --https-port    HTTPS server port (0 = disabled)
      --read-timeout  Read timeout in seconds (default: 30)
      --write-timeout Write timeout in seconds (default: 30)
      --max-log-entries Maximum request log entries (default: 1000)
      --auto-cert     Auto-generate TLS certificate (default: true)

Runtime mode (register with control plane):
      --register      Register with control plane as a runtime
      --control-plane Control plane URL (default: https://api.mockd.io)
      --token         Runtime token (or MOCKD_RUNTIME_TOKEN env var)
      --name          Runtime name (required with --register)
      --labels        Runtime labels (key=value,key2=value2)

Pull mode (serve mocks from cloud):
      --pull          mockd:// URI to pull and serve
      --cache         Local cache directory for pulled mocks

Examples:
  # Start with defaults
  mockd serve

  # Start with config file on custom port
  mockd serve --config mocks.json --port 3000

  # Register as a runtime
  mockd serve --register --name ci-runner-1 --token $MOCKD_RUNTIME_TOKEN

  # Pull and serve from cloud
  mockd serve --pull mockd://acme/payment-api
`)
	}

	if err := fs.Parse(args); err != nil {
		return err
	}

	// Get token from env var if not provided via flag
	if *token == "" {
		*token = os.Getenv("MOCKD_RUNTIME_TOKEN")
	}

	// Validate flags for different modes
	if *register && *pull != "" {
		return fmt.Errorf("cannot use --register and --pull together")
	}

	if *register {
		if *name == "" {
			return fmt.Errorf("--name is required when using --register")
		}
		if *token == "" {
			return fmt.Errorf("--token is required when using --register (or set MOCKD_RUNTIME_TOKEN)")
		}
	}

	if *pull != "" && *token == "" {
		*token = os.Getenv("MOCKD_TOKEN")
		if *token == "" {
			return fmt.Errorf("--token is required when using --pull (or set MOCKD_TOKEN)")
		}
	}

	// Check for port conflicts
	if err := checkPort(*port); err != nil {
		return formatPortError(*port, err)
	}
	if err := checkPort(*adminPort); err != nil {
		return formatPortError(*adminPort, err)
	}
	if *httpsPort > 0 {
		if err := checkPort(*httpsPort); err != nil {
			return formatPortError(*httpsPort, err)
		}
	}

	// Build server configuration
	serverCfg := &config.ServerConfiguration{
		HTTPPort:         *port,
		HTTPSPort:        *httpsPort,
		AdminPort:        *adminPort,
		ReadTimeout:      *readTimeout,
		WriteTimeout:     *writeTimeout,
		MaxLogEntries:    *maxLogEntries,
		AutoGenerateCert: *autoCert,
		LogRequests:      true,
	}

	// Create and start the mock server
	server := engine.NewServer(serverCfg)

	// Handle different modes
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var runtimeClient *runtime.Client

	if *register {
		// Runtime mode: register with control plane
		runtimeClient = runtime.NewClient(runtime.Config{
			ControlPlaneURL: *controlPlane,
			Token:           *token,
			Name:            *name,
			URL:             fmt.Sprintf("http://localhost:%d", *port), // TODO: Make configurable
			Labels:          parseLabels(*labels),
			Version:         "dev", // TODO: Get actual version
		})

		regResp, err := runtimeClient.Register(ctx)
		if err != nil {
			return fmt.Errorf("failed to register with control plane: %w", err)
		}

		fmt.Printf("Registered as runtime %s (ID: %s)\n", regResp.Name, regResp.ID)

		// Pull initial deployments
		if err := runtimeClient.PullDeployments(ctx); err != nil {
			fmt.Printf("Warning: failed to pull initial deployments: %v\n", err)
		}

		// Start heartbeat loop in background
		go func() {
			if err := runtimeClient.HeartbeatLoop(ctx); err != nil && ctx.Err() == nil {
				fmt.Printf("Heartbeat loop error: %v\n", err)
			}
		}()
	} else if *pull != "" {
		// Pull mode: fetch mocks from cloud
		pullClient := runtime.NewClient(runtime.Config{
			ControlPlaneURL: *controlPlane,
			Token:           *token,
		})

		content, err := pullClient.Pull(ctx, *pull)
		if err != nil {
			return fmt.Errorf("failed to pull mocks: %w", err)
		}

		// Cache the content if cache dir specified
		if *cacheDir != "" {
			if err := cachePulledContent(*cacheDir, *pull, content); err != nil {
				fmt.Printf("Warning: failed to cache content: %v\n", err)
			}
		}

		// Load pulled content into server
		if err := server.LoadConfigFromBytes(content, false); err != nil {
			return fmt.Errorf("failed to load pulled mocks: %w", err)
		}

		fmt.Printf("Pulled mocks from %s\n", *pull)
	} else if *configFile != "" {
		// Local mode with config file
		if err := server.LoadConfig(*configFile, false); err != nil {
			return fmt.Errorf("failed to load config file: %w", err)
		}
	}

	// Start the mock server
	if err := server.Start(); err != nil {
		return fmt.Errorf("failed to start mock server: %w", err)
	}

	// Create and start the admin API
	adminAPI := admin.NewAdminAPI(server, *adminPort)
	if err := adminAPI.Start(); err != nil {
		server.Stop()
		return fmt.Errorf("failed to start admin API: %w", err)
	}

	// Print startup message
	printServeStartupMessage(*port, *adminPort, *httpsPort, *register, *pull)

	// Wait for shutdown signal
	waitForServeShutdown(ctx, cancel, server, adminAPI, runtimeClient)

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
func printServeStartupMessage(httpPort, adminPort, httpsPort int, isRuntime bool, pullURI string) {
	fmt.Printf("Mock server running on http://localhost:%d\n", httpPort)
	if httpsPort > 0 {
		fmt.Printf("HTTPS server running on https://localhost:%d\n", httpsPort)
	}
	fmt.Printf("Admin API running on http://localhost:%d\n", adminPort)

	if isRuntime {
		fmt.Println("Mode: Runtime (connected to control plane)")
	} else if pullURI != "" {
		fmt.Printf("Mode: Pull (serving from %s)\n", pullURI)
	} else {
		fmt.Println("Mode: Local")
	}

	fmt.Println("Press Ctrl+C to stop")
}

// waitForServeShutdown blocks until a shutdown signal is received.
func waitForServeShutdown(ctx context.Context, cancel context.CancelFunc, server *engine.Server, adminAPI *admin.AdminAPI, runtimeClient *runtime.Client) {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	<-sigChan
	fmt.Println("\nShutting down...")

	// Cancel context to stop heartbeat loop
	cancel()

	// Deregister from control plane if in runtime mode
	if runtimeClient != nil {
		fmt.Println("Deregistering from control plane...")
		// TODO: Implement graceful deregistration
	}

	// Stop admin API first
	if err := adminAPI.Stop(); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: admin API shutdown error: %v\n", err)
	}

	// Stop mock server
	if err := server.Stop(); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: server shutdown error: %v\n", err)
	}

	fmt.Println("Server stopped")
}

// checkPort checks if a port is available (moved from start.go for reuse).
func checkServePort(port int) error {
	ln, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return err
	}
	ln.Close()
	return nil
}
