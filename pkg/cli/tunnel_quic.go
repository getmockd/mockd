package cli

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"syscall"

	quicclient "github.com/getmockd/mockd/pkg/tunnel/quic"
)

// RunTunnelQUIC handles the tunnel-quic command.
func RunTunnelQUIC(args []string) error {
	fs := flag.NewFlagSet("tunnel-quic", flag.ContinueOnError)

	// Relay configuration
	relayAddr := fs.String("relay", "relay.mockd.io", "Relay server address (host or host:port)")
	token := fs.String("token", os.Getenv("MOCKD_TOKEN"), "Authentication token")

	// Local service configuration
	localPort := fs.Int("port", 4280, "Local port to tunnel")
	fs.IntVar(localPort, "p", 4280, "Local port to tunnel (shorthand)")
	localHost := fs.String("local-host", "localhost", "Local host to forward to")

	// TLS options
	tlsInsecure := fs.Bool("insecure", false, "Skip TLS certificate verification (for testing)")

	fs.Usage = func() {
		fmt.Fprint(os.Stderr, `Usage: mockd tunnel-quic [flags]

Expose a local service via QUIC tunnel to the relay server.

This is a lightweight tunnel that forwards traffic to a local port.
Unlike 'mockd tunnel', this doesn't start a mock server - it just
forwards requests to an existing local service.

Flags:
      --relay       Relay server address (default: localhost:4443)
      --token       Authentication token (or set MOCKD_TOKEN)
  -p, --port        Local port to tunnel (default: 4280)
      --local-host  Local host to forward to (default: localhost)
      --insecure    Skip TLS verification (for testing)

Examples:
  # Tunnel local port 4280 to the relay
  mockd tunnel-quic --relay relay.mockd.io:4443 --port 4280

  # Tunnel with authentication
  mockd tunnel-quic --relay relay.mockd.io:4443 --token YOUR_TOKEN

  # For local testing with self-signed certs
  mockd tunnel-quic --relay localhost:4443 --insecure

Environment Variables:
  MOCKD_TOKEN       Authentication token (alternative to --token flag)
`)
	}

	if err := fs.Parse(args); err != nil {
		return err
	}

	// Default to port 443 if no port specified
	if !strings.Contains(*relayAddr, ":") {
		*relayAddr = *relayAddr + ":443"
	}

	// Build local target URL
	targetURL := fmt.Sprintf("http://%s:%d", *localHost, *localPort)
	target, err := url.Parse(targetURL)
	if err != nil {
		return fmt.Errorf("invalid target URL: %w", err)
	}

	// Create reverse proxy to forward requests to local service
	proxy := httputil.NewSingleHostReverseProxy(target)
	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		fmt.Printf("  proxy error: %v\n", err)
		http.Error(w, "Service unavailable", http.StatusServiceUnavailable)
	}

	// Setup logger
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	// Create QUIC client
	client := quicclient.NewClient(&quicclient.ClientConfig{
		RelayAddr:   *relayAddr,
		Token:       *token,
		LocalPort:   *localPort,
		Handler:     proxy,
		TLSInsecure: *tlsInsecure,
		Logger:      logger,
	})

	client.OnConnect = func(publicURL string) {
		fmt.Printf("\nTunnel connected!\n")
		fmt.Printf("Public URL: %s\n", publicURL)
		fmt.Printf("Local target: %s\n", targetURL)
		fmt.Println("\nPress Ctrl+C to stop")
	}

	client.OnDisconnect = func(err error) {
		if err != nil {
			fmt.Printf("\nTunnel disconnected: %v\n", err)
		} else {
			fmt.Println("\nTunnel disconnected")
		}
	}

	client.OnRequest = func(method, path string) {
		fmt.Printf("  %s %s\n", method, path)
	}

	// Set up context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Connect tunnel
	fmt.Printf("Connecting to relay at %s (QUIC)...\n", *relayAddr)
	if err := client.Connect(ctx); err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}

	// Handle shutdown signals
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	// Run tunnel in goroutine
	errCh := make(chan error, 1)
	go func() {
		errCh <- client.Run(ctx)
	}()

	// Wait for shutdown signal or error
	select {
	case <-sigCh:
		fmt.Println("\nShutting down...")
	case err := <-errCh:
		if err != nil && err != context.Canceled {
			fmt.Printf("\nTunnel error: %v\n", err)
		}
	}

	// Close client
	client.Close()

	// Print stats
	fmt.Printf("\nSession stats:\n")
	fmt.Printf("  Requests served: %d\n", client.RequestCount())

	fmt.Println("Goodbye!")
	return nil
}
