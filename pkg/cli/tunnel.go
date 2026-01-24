package cli

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/getmockd/mockd/pkg/admin"
	"github.com/getmockd/mockd/pkg/cli/internal/ports"
	"github.com/getmockd/mockd/pkg/cliconfig"
	"github.com/getmockd/mockd/pkg/config"
	"github.com/getmockd/mockd/pkg/engine"
	"github.com/getmockd/mockd/pkg/tunnel"
)

// RunTunnel handles the tunnel command.
func RunTunnel(args []string) error {
	if len(args) > 0 {
		switch args[0] {
		case "status":
			return runTunnelStatus(args[1:])
		case "stop":
			return runTunnelStop(args[1:])
		}
	}

	return runTunnelStart(args)
}

// runTunnelStart starts a tunnel connection.
func runTunnelStart(args []string) error {
	fs := flag.NewFlagSet("tunnel", flag.ContinueOnError)

	// Server configuration
	port := fs.Int("port", cliconfig.DefaultPort, "HTTP server port")
	fs.IntVar(port, "p", cliconfig.DefaultPort, "HTTP server port (shorthand)")
	adminPort := fs.Int("admin-port", cliconfig.DefaultAdminPort, "Admin API port")
	configFile := fs.String("config", "", "Path to mock configuration file")
	fs.StringVar(configFile, "c", "", "Path to mock configuration file (shorthand)")

	// Tunnel configuration
	relayURL := fs.String("relay", tunnel.DefaultRelayURL, "Relay server URL")
	token := fs.String("token", os.Getenv("MOCKD_TOKEN"), "Authentication token (or set MOCKD_TOKEN)")
	subdomain := fs.String("subdomain", "", "Requested subdomain (auto-assigned if empty)")
	fs.StringVar(subdomain, "s", "", "Requested subdomain (shorthand)")
	domain := fs.String("domain", "", "Custom domain (must be verified)")

	// Authentication for incoming requests (optional protection)
	authToken := fs.String("auth-token", "", "Require this token for incoming requests")
	authBasic := fs.String("auth-basic", "", "Require Basic Auth (format: user:pass)")
	allowIPs := fs.String("allow-ips", "", "Allow only these IPs (comma-separated CIDR or IP)")

	fs.Usage = func() {
		fmt.Fprint(os.Stderr, `Usage: mockd tunnel [flags]
       mockd tunnel status
       mockd tunnel stop

Expose local mocks via the cloud relay. The tunnel creates a public URL that
forwards requests to your local mock server.

Flags:
  -p, --port        HTTP server port (default: 4280)
      --admin-port  Admin API port (default: 4290)
  -c, --config      Path to mock configuration file
      --relay       Relay server URL (default: wss://relay.mockd.io/ws)
      --token       Authentication token (or set MOCKD_TOKEN env var)
  -s, --subdomain   Requested subdomain (auto-assigned if empty)
      --domain      Custom domain (must be verified in cloud dashboard)

Authentication (optional - protect incoming requests):
      --auth-token  Require this token in X-Auth-Token header
      --auth-basic  Require HTTP Basic Auth (format: user:pass)
      --allow-ips   Allow only these IPs (comma-separated CIDR or IP)

Subcommands:
  status    Show current tunnel status and metrics
  stop      Stop the running tunnel

Examples:
  # Start tunnel with auto-assigned subdomain
  mockd tunnel --token YOUR_TOKEN

  # Start tunnel with custom subdomain
  mockd tunnel --token YOUR_TOKEN --subdomain my-api

  # Start tunnel with config file
  mockd tunnel --config mocks.json --token YOUR_TOKEN

  # Start tunnel with custom domain (must verify in cloud dashboard first)
  mockd tunnel --token YOUR_TOKEN --domain mocks.acme.com

  # Protect tunnel with token authentication
  mockd tunnel --token YOUR_TOKEN --auth-token secret123

  # Protect tunnel with HTTP Basic Auth
  mockd tunnel --token YOUR_TOKEN --auth-basic admin:password

  # Restrict tunnel access by IP
  mockd tunnel --token YOUR_TOKEN --allow-ips "10.0.0.0/8,192.168.1.0/24"

  # Check tunnel status
  mockd tunnel status

Environment Variables:
  MOCKD_TOKEN       Authentication token (alternative to --token flag)
`)
	}

	if err := fs.Parse(args); err != nil {
		return err
	}

	// Validate token
	if *token == "" {
		return fmt.Errorf("authentication token required (use --token or set MOCKD_TOKEN)")
	}

	// Check for port conflicts
	if err := ports.Check(*port); err != nil {
		return formatPortError(*port, err)
	}
	if err := ports.Check(*adminPort); err != nil {
		return formatPortError(*adminPort, err)
	}

	// Build server configuration
	serverCfg := &config.ServerConfiguration{
		HTTPPort:      *port,
		AdminPort:     *adminPort,
		ReadTimeout:   30,
		WriteTimeout:  30,
		MaxLogEntries: 1000,
		LogRequests:   true,
	}

	// Create the mock server
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
	engineURL := fmt.Sprintf("http://localhost:%d", server.ManagementPort())
	adminAPI := admin.NewAdminAPI(*adminPort, admin.WithLocalEngine(engineURL))
	if err := adminAPI.Start(); err != nil {
		_ = server.Stop()
		return fmt.Errorf("failed to start admin API: %w", err)
	}

	// Build tunnel configuration
	tunnelCfg := tunnel.DefaultConfig().
		WithRelayURL(*relayURL).
		WithToken(*token).
		WithSubdomain(*subdomain).
		WithCustomDomain(*domain)

	// Configure request authentication if specified
	if *authToken != "" {
		tunnelCfg.WithTokenAuth(*authToken)
		fmt.Println("Request authentication: token required")
	} else if *authBasic != "" {
		parts := strings.SplitN(*authBasic, ":", 2)
		if len(parts) != 2 {
			return fmt.Errorf("invalid --auth-basic format, expected user:pass")
		}
		tunnelCfg.WithBasicAuth(parts[0], parts[1])
		fmt.Println("Request authentication: Basic Auth required")
	} else if *allowIPs != "" {
		ips := strings.Split(*allowIPs, ",")
		for i := range ips {
			ips[i] = strings.TrimSpace(ips[i])
		}
		tunnelCfg.WithIPAuth(ips)
		fmt.Printf("Request authentication: IP whitelist (%d entries)\n", len(ips))
	}

	tunnelCfg.OnConnect = func(publicURL string) {
		fmt.Printf("\nTunnel connected!\n")
		fmt.Printf("Public URL: %s\n", publicURL)
		fmt.Printf("Local server: http://localhost:%d\n", *port)
		fmt.Printf("Admin API: http://localhost:%d\n", *adminPort)
		fmt.Println("\nPress Ctrl+C to stop")
	}

	tunnelCfg.OnDisconnect = func(err error) {
		if err != nil {
			fmt.Printf("\nTunnel disconnected: %v\n", err)
		} else {
			fmt.Println("\nTunnel disconnected")
		}
	}

	tunnelCfg.OnRequest = func(method, path string) {
		fmt.Printf("  %s %s\n", method, path)
	}

	// Create tunnel client with engine handler
	engineHandler := tunnel.NewEngineHandler(server.Handler(), tunnelCfg.Auth)
	tunnelClient, err := tunnel.NewClient(tunnelCfg, engineHandler)
	if err != nil {
		_ = adminAPI.Stop()
		_ = server.Stop()
		return fmt.Errorf("failed to create tunnel client: %w", err)
	}

	// Set up context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Connect tunnel
	fmt.Printf("Connecting to relay at %s...\n", *relayURL)
	if err := tunnelClient.Connect(ctx); err != nil {
		_ = adminAPI.Stop()
		_ = server.Stop()
		return fmt.Errorf("failed to connect tunnel: %w", err)
	}

	// Wait for shutdown signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	<-sigChan
	fmt.Println("\nShutting down...")

	// Disconnect tunnel
	tunnelClient.Disconnect()

	// Print final stats
	stats := tunnelClient.Stats()
	fmt.Printf("\nSession stats:\n")
	fmt.Printf("  Requests served: %d\n", stats.RequestsServed)
	fmt.Printf("  Bytes in: %d\n", stats.BytesIn)
	fmt.Printf("  Bytes out: %d\n", stats.BytesOut)
	fmt.Printf("  Uptime: %s\n", stats.Uptime())
	if stats.RequestsServed > 0 {
		fmt.Printf("  Avg latency: %.2f ms\n", stats.AvgLatencyMs())
	}

	// Stop admin API
	if err := adminAPI.Stop(); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: admin API shutdown error: %v\n", err)
	}

	// Stop mock server
	if err := server.Stop(); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: server shutdown error: %v\n", err)
	}

	fmt.Println("Goodbye!")
	return nil
}

// runTunnelStatus shows the current tunnel status.
func runTunnelStatus(args []string) error {
	fs := flag.NewFlagSet("tunnel status", flag.ContinueOnError)
	adminAddr := fs.String("admin-url", "http://localhost:4290", "Admin API address")

	fs.Usage = func() {
		fmt.Fprint(os.Stderr, `Usage: mockd tunnel status [flags]

Show the current tunnel connection status.

Flags:
      --admin-url   Admin API address (default: http://localhost:4290)
`)
	}

	if err := fs.Parse(args); err != nil {
		return err
	}

	// Check if server is running
	client := NewAdminClientWithAuth(*adminAddr)
	if err := client.Health(); err != nil {
		return fmt.Errorf("mockd server not running (admin API not reachable)")
	}

	// TODO: Add tunnel status endpoint to admin API
	fmt.Println("Server is running. Tunnel status check not yet implemented.")
	fmt.Println("Use 'mockd tunnel' to start a new tunnel connection.")
	return nil
}

// runTunnelStop stops a running tunnel.
func runTunnelStop(args []string) error {
	fs := flag.NewFlagSet("tunnel stop", flag.ContinueOnError)
	adminAddr := fs.String("admin-url", "http://localhost:4290", "Admin API address")

	fs.Usage = func() {
		fmt.Fprint(os.Stderr, `Usage: mockd tunnel stop [flags]

Stop the running tunnel connection.

Flags:
      --admin-url   Admin API address (default: http://localhost:4290)
`)
	}

	if err := fs.Parse(args); err != nil {
		return err
	}

	// Check if server is running
	client := NewAdminClientWithAuth(*adminAddr)
	if err := client.Health(); err != nil {
		return fmt.Errorf("mockd server not running (admin API not reachable)")
	}

	// TODO: Add tunnel stop endpoint to admin API
	fmt.Println("Tunnel stop not yet implemented via CLI.")
	fmt.Println("Use Ctrl+C in the terminal where 'mockd tunnel' is running.")
	return nil
}
