package cli

import (
	"flag"
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/getmockd/mockd/internal/cliconfig"
	"github.com/getmockd/mockd/pkg/admin"
	"github.com/getmockd/mockd/pkg/config"
	"github.com/getmockd/mockd/pkg/engine"
)

// RunStart handles the start command.
func RunStart(args []string) error {
	fs := flag.NewFlagSet("start", flag.ContinueOnError)

	// Define flags with defaults from cliconfig
	port := fs.Int("port", cliconfig.DefaultPort, "HTTP server port")
	fs.IntVar(port, "p", cliconfig.DefaultPort, "HTTP server port (shorthand)")

	adminPort := fs.Int("admin-port", cliconfig.DefaultAdminPort, "Admin API port")
	fs.IntVar(adminPort, "a", cliconfig.DefaultAdminPort, "Admin API port (shorthand)")

	configFile := fs.String("config", "", "Path to mock configuration file")
	fs.StringVar(configFile, "c", "", "Path to mock configuration file (shorthand)")

	loadDir := fs.String("load", "", "Load mocks from directory")
	watch := fs.Bool("watch", false, "Watch for file changes (with --load)")
	validate := fs.Bool("validate", false, "Validate files before serving (with --load)")

	httpsPort := fs.Int("https-port", cliconfig.DefaultHTTPSPort, "HTTPS server port (0 = disabled)")
	readTimeout := fs.Int("read-timeout", cliconfig.DefaultReadTimeout, "Read timeout in seconds")
	writeTimeout := fs.Int("write-timeout", cliconfig.DefaultWriteTimeout, "Write timeout in seconds")
	maxLogEntries := fs.Int("max-log-entries", cliconfig.DefaultMaxLogEntries, "Maximum request log entries")
	autoCert := fs.Bool("auto-cert", cliconfig.DefaultAutoCert, "Auto-generate TLS certificate")

	fs.Usage = func() {
		fmt.Fprint(os.Stderr, `Usage: mockd start [flags]

Start the mock server.

Flags:
  -p, --port          HTTP server port (default: 8080)
  -a, --admin-port    Admin API port (default: 9090)
  -c, --config        Path to mock configuration file
      --load          Load mocks from directory
      --watch         Watch for file changes (with --load)
      --validate      Validate files before serving (with --load)
      --https-port    HTTPS server port (0 = disabled)
      --read-timeout  Read timeout in seconds (default: 30)
      --write-timeout Write timeout in seconds (default: 30)
      --max-log-entries Maximum request log entries (default: 1000)
      --auto-cert     Auto-generate TLS certificate (default: true)

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

	// Load config file if specified
	if *configFile != "" {
		if err := server.LoadConfig(*configFile, false); err != nil {
			return fmt.Errorf("failed to load config file: %w", err)
		}
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
					fmt.Fprintf(os.Stderr, "  • %s\n", e.Error())
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
				fmt.Fprintf(os.Stderr, "  • %s\n", e.Error())
			}
		}

		// Add loaded mocks to server
		for _, mock := range result.Collection.Mocks {
			if err := server.AddMock(mock); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to add mock %s: %v\n", mock.ID, err)
			}
		}

		fmt.Printf("Loaded %d mocks from %d files in %s\n", len(result.Collection.Mocks), result.FileCount, *loadDir)
	}

	// Start the mock server
	if err := server.Start(); err != nil {
		return fmt.Errorf("failed to start mock server: %w", err)
	}

	// Start file watcher if requested
	var watcher *config.Watcher
	if *watch && dirLoader != nil {
		watcher = config.NewWatcher(dirLoader)
		eventCh := watcher.Start()
		go handleWatchEvents(eventCh, dirLoader, server)
		fmt.Println("Watching for file changes...")
	}

	// Create and start the admin API
	adminAPI := admin.NewAdminAPI(server, *adminPort)
	if err := adminAPI.Start(); err != nil {
		server.Stop()
		return fmt.Errorf("failed to start admin API: %w", err)
	}

	// Print startup message
	printStartupMessage(*port, *adminPort, *httpsPort)

	// Wait for shutdown signal
	waitForShutdown(server, adminAPI)

	return nil
}

// checkPort checks if a port is available.
func checkPort(port int) error {
	ln, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return err
	}
	ln.Close()
	return nil
}

// formatPortError formats a port conflict error with suggestions.
func formatPortError(port int, err error) error {
	return fmt.Errorf(`port %d already in use

Suggestions:
  • Use a different port: mockd start --port %d
  • Check what's using the port: lsof -i :%d
  • Stop the other process and try again`, port, port+1, port)
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

// waitForShutdown blocks until a shutdown signal is received.
func waitForShutdown(server *engine.Server, adminAPI *admin.AdminAPI) {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	<-sigChan
	fmt.Println("\nShutting down...")

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

// handleWatchEvents processes file change events from the watcher.
func handleWatchEvents(eventCh <-chan config.WatchEvent, loader *config.DirectoryLoader, server *engine.Server) {
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

		// Update mocks in server
		for _, mock := range collection.Mocks {
			if err := server.AddMock(mock); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to update mock %s: %v\n", mock.ID, err)
			}
		}

		fmt.Printf("Reloaded %d mocks from %s\n", len(collection.Mocks), event.Path)
	}
}
