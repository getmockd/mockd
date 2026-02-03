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

	"github.com/getmockd/mockd/pkg/tunnel/protocol"
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

	// Tunnel auth (incoming request protection)
	authToken := fs.String("auth-token", "", "Require this token from callers (via X-Tunnel-Token header or ?token= param)")
	authBasic := fs.String("auth-basic", "", "Require HTTP Basic auth from callers (format: user:pass)")
	allowIPs := fs.String("allow-ips", "", "Restrict access to these IPs/CIDRs (comma-separated, e.g. 10.0.0.0/8,192.168.1.50)")
	authHeader := fs.String("auth-header", "", "Custom header name for token auth (default: X-Tunnel-Token)")

	// TLS options
	tlsInsecure := fs.Bool("insecure", false, "Skip TLS certificate verification (for testing)")

	fs.Usage = func() {
		fmt.Fprint(os.Stderr, `Usage: mockd tunnel-quic [flags]

Expose a local service via QUIC tunnel to the relay server.

This is a lightweight tunnel that forwards traffic to a local port.
Unlike 'mockd tunnel', this doesn't start a mock server - it just
forwards requests to an existing local service.

Flags:
      --relay        Relay server address (default: relay.mockd.io)
      --token        Authentication token (or set MOCKD_TOKEN)
  -p, --port         Local port to tunnel (default: 4280)
      --local-host   Local host to forward to (default: localhost)
      --insecure     Skip TLS verification (for testing)

Tunnel Auth (protect your tunnel URL from unauthorized callers):
      --auth-token   Require token from callers (X-Tunnel-Token header or ?token= param)
      --auth-basic   Require HTTP Basic auth (format: user:pass)
      --allow-ips    Restrict to IPs/CIDRs (comma-separated, e.g. 10.0.0.0/8,192.168.1.50)
      --auth-header  Custom header name for token auth (default: X-Tunnel-Token)

Examples:
  # Tunnel local port 4280 to the relay
  mockd tunnel-quic --relay relay.mockd.io --port 4280

  # Tunnel with agent authentication
  mockd tunnel-quic --relay relay.mockd.io --token YOUR_TOKEN

  # Protect tunnel URL with token auth
  mockd tunnel-quic --token YOUR_TOKEN --auth-token SECRET123
  # Callers must include: curl -H "X-Tunnel-Token: SECRET123" https://abc.tunnel.mockd.io

  # Protect with IP allowlist
  mockd tunnel-quic --token YOUR_TOKEN --allow-ips 10.0.0.0/8,192.168.1.0/24

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

	// Build tunnel auth config from flags
	var tunnelAuth *protocol.TunnelAuth
	switch {
	case *authToken != "":
		tunnelAuth = &protocol.TunnelAuth{
			Type:        "token",
			Token:       *authToken,
			TokenHeader: *authHeader,
		}
	case *authBasic != "":
		parts := strings.SplitN(*authBasic, ":", 2)
		if len(parts) != 2 {
			return fmt.Errorf("--auth-basic must be in format user:pass")
		}
		tunnelAuth = &protocol.TunnelAuth{
			Type:     "basic",
			Username: parts[0],
			Password: parts[1],
		}
	case *allowIPs != "":
		cidrs := strings.Split(*allowIPs, ",")
		for i := range cidrs {
			cidrs[i] = strings.TrimSpace(cidrs[i])
		}
		tunnelAuth = &protocol.TunnelAuth{
			Type:       "ip",
			AllowedIPs: cidrs,
		}
	}

	// Create QUIC client
	client := quicclient.NewClient(&quicclient.ClientConfig{
		RelayAddr:   *relayAddr,
		Token:       *token,
		LocalPort:   *localPort,
		Handler:     proxy,
		TLSInsecure: *tlsInsecure,
		TunnelAuth:  tunnelAuth,
		Logger:      logger,
	})

	client.OnConnect = func(publicURL string) {
		fmt.Printf("\nTunnel connected!\n")
		fmt.Printf("Public URL: %s\n", publicURL)
		fmt.Printf("Local target: %s\n", targetURL)
		if tunnelAuth != nil {
			switch tunnelAuth.Type {
			case "token":
				h := tunnelAuth.EffectiveTokenHeader()
				fmt.Printf("Auth: token (header: %s)\n", h)
			case "basic":
				fmt.Printf("Auth: basic (user: %s)\n", tunnelAuth.Username)
			case "ip":
				fmt.Printf("Auth: IP allowlist (%s)\n", strings.Join(tunnelAuth.AllowedIPs, ", "))
			}
		} else {
			fmt.Printf("Auth: none (tunnel URL is public)\n")
		}
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
	_ = client.Close()

	// Print stats
	fmt.Printf("\nSession stats:\n")
	fmt.Printf("  Requests served: %d\n", client.RequestCount())

	fmt.Println("Goodbye!")
	return nil
}
