package cli

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/getmockd/mockd/pkg/tunnel/protocol"
	quicclient "github.com/getmockd/mockd/pkg/tunnel/quic"
)

// mqttFlag implements flag.Value for repeatable --mqtt flags.
// Accepts "PORT" or "PORT:NAME" format (e.g., "1883" or "1883:sensors").
type mqttFlag []protocol.ProtocolPort

func (f *mqttFlag) String() string {
	if f == nil {
		return ""
	}
	var parts []string
	for _, p := range *f {
		if p.Name != "" {
			parts = append(parts, fmt.Sprintf("%d:%s", p.Port, p.Name))
		} else {
			parts = append(parts, strconv.Itoa(p.Port))
		}
	}
	return strings.Join(parts, ", ")
}

func (f *mqttFlag) Set(s string) error {
	parts := strings.SplitN(s, ":", 2)

	port, err := strconv.Atoi(parts[0])
	if err != nil {
		return fmt.Errorf("invalid MQTT port %q: %w", parts[0], err)
	}
	if port < 1 || port > 65535 {
		return fmt.Errorf("MQTT port %d out of range (1-65535)", port)
	}

	pp := protocol.ProtocolPort{
		Type: "mqtt",
		Port: port,
	}
	if len(parts) == 2 {
		pp.Name = parts[1]
		if pp.Name == "" {
			return fmt.Errorf("MQTT broker name cannot be empty after ':'")
		}
	}

	*f = append(*f, pp)
	return nil
}

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

	// MQTT broker ports (repeatable)
	var mqttPorts mqttFlag
	fs.Var(&mqttPorts, "mqtt", "MQTT broker port (repeatable, format: PORT or PORT:NAME)")

	// TLS options
	tlsInsecure := fs.Bool("insecure", false, "Skip TLS certificate verification (for testing)")

	fs.Usage = func() {
		fmt.Fprint(os.Stderr, `Usage: mockd tunnel-quic [flags]

Expose a local service via QUIC tunnel to the relay server.

This is a lightweight tunnel that forwards traffic to a local port.
Unlike 'mockd tunnel', this doesn't start a mock server - it just
forwards requests to an existing local service.

All protocols (HTTP, gRPC, WebSocket, SSE, MQTT) are tunneled through
a single port (443) using ALPN-based routing.

Flags:
      --relay        Relay server address (default: relay.mockd.io)
      --token        Authentication token (or set MOCKD_TOKEN)
  -p, --port         Local HTTP/gRPC/WebSocket port (default: 4280)
      --mqtt         Local MQTT broker port (repeatable, format: PORT or PORT:NAME)
      --local-host   Local host to forward to (default: localhost)
      --insecure     Skip TLS verification (for testing)

Tunnel Auth (protect your tunnel URL from unauthorized callers):
      --auth-token   Require token from callers (X-Tunnel-Token header or ?token= param)
      --auth-basic   Require HTTP Basic auth (format: user:pass)
      --allow-ips    Restrict to IPs/CIDRs (comma-separated, e.g. 10.0.0.0/8,192.168.1.50)
      --auth-header  Custom header name for token auth (default: X-Tunnel-Token)

Examples:
  # Tunnel local port 4280 (HTTP, gRPC, WebSocket all work automatically)
  mockd tunnel-quic --port 4280

  # Tunnel HTTP + MQTT broker
  mockd tunnel-quic --port 8080 --mqtt 1883

  # Multiple named MQTT brokers (each gets a subdomain)
  mockd tunnel-quic --port 8080 --mqtt 1883:sensors --mqtt 1884:alerts
  # sensors.abc123.tunnel.mockd.io:443 (ALPN: mqtt) → localhost:1883
  # alerts.abc123.tunnel.mockd.io:443  (ALPN: mqtt) → localhost:1884

  # Protect tunnel URL with token auth
  mockd tunnel-quic --auth-token SECRET123
  # curl -H "X-Tunnel-Token: SECRET123" https://abc123.tunnel.mockd.io

  # For local testing with self-signed certs
  mockd tunnel-quic --relay localhost:4443 --insecure

MQTT clients connect with ALPN negotiation:
  mosquitto_pub -h abc123.tunnel.mockd.io -p 443 --alpn mqtt \
    --capath /etc/ssl/certs -t sensors/temp -m '{"val":22}'

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

	// Auto-fetch anonymous JWT if no token provided
	if *token == "" {
		fmt.Println("No token provided, fetching anonymous tunnel token...")
		anonymousToken, err := fetchAnonymousToken()
		if err != nil {
			return fmt.Errorf("failed to fetch anonymous token: %w", err)
		}
		*token = anonymousToken
		fmt.Println("Anonymous token acquired (2h session, 100MB bandwidth)")
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

	// Build protocol list from flags
	protocols := make([]protocol.ProtocolPort, 0, 1+len(mqttPorts))
	protocols = append(protocols, protocol.ProtocolPort{
		Type: "http",
		Port: *localPort,
	})
	protocols = append(protocols, mqttPorts...)

	// Create QUIC client
	client := quicclient.NewClient(&quicclient.ClientConfig{
		RelayAddr:   *relayAddr,
		Token:       *token,
		LocalPort:   *localPort,
		Handler:     proxy,
		TLSInsecure: *tlsInsecure,
		TunnelAuth:  tunnelAuth,
		Protocols:   protocols,
		Logger:      logger,
	})

	client.OnConnect = func(publicURL string) {
		fmt.Printf("\nTunnel connected!\n")
		fmt.Printf("  HTTP:  %s → %s\n", publicURL, targetURL)

		// Show MQTT endpoints
		for _, p := range mqttPorts {
			// Extract the hostname from the public URL for MQTT display
			mqttHost := strings.TrimPrefix(publicURL, "https://")
			mqttHost = strings.TrimPrefix(mqttHost, "http://")
			if p.Name != "" {
				fmt.Printf("  MQTT:  mqtts://%s.%s:443 → localhost:%d (ALPN: mqtt)\n", p.Name, mqttHost, p.Port)
			} else {
				fmt.Printf("  MQTT:  mqtts://%s:443 → localhost:%d (ALPN: mqtt)\n", mqttHost, p.Port)
			}
		}

		if tunnelAuth != nil {
			switch tunnelAuth.Type {
			case "token":
				h := tunnelAuth.EffectiveTokenHeader()
				fmt.Printf("  Auth:  token (header: %s)\n", h)
			case "basic":
				fmt.Printf("  Auth:  basic (user: %s)\n", tunnelAuth.Username)
			case "ip":
				fmt.Printf("  Auth:  IP allowlist (%s)\n", strings.Join(tunnelAuth.AllowedIPs, ", "))
			}
		} else {
			fmt.Printf("  Auth:  none (tunnel URL is public)\n")
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

// tokenAPIURL is the endpoint for anonymous tunnel token requests.
const tokenAPIURL = "https://api.mockd.io/api/v1/tunnels/anonymous"

// tokenResponse matches the JSON response from the tunnel token API.
type tokenResponse struct {
	Token     string `json:"token"`
	ExpiresAt string `json:"expires_at"`
	Tier      string `json:"tier"`
}

// fetchAnonymousToken calls the mockd API to get a free anonymous tunnel JWT.
func fetchAnonymousToken() (string, error) {
	client := &http.Client{Timeout: 10 * time.Second}

	resp, err := client.Post(tokenAPIURL, "application/json", nil)
	if err != nil {
		return "", fmt.Errorf("HTTP request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 16*1024))
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("API returned %d: %s", resp.StatusCode, string(body))
	}

	var tokenResp tokenResponse
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}

	if tokenResp.Token == "" {
		return "", fmt.Errorf("API returned empty token")
	}

	return tokenResp.Token, nil
}
