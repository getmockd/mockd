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
`)
	}

	if err := fs.Parse(args); err != nil {
		return err
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
