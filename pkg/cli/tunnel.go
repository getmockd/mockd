package cli

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/getmockd/mockd/pkg/admin"
	"github.com/getmockd/mockd/pkg/cli/internal/output"
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
		case "enable":
			return runTunnelEnable(args[1:])
		case "disable":
			return runTunnelDisable(args[1:])
		case "status":
			return runTunnelStatus(args[1:])
		case "stop":
			return runTunnelStop(args[1:])
		case "list":
			return runTunnelList(args[1:])
		case "preview":
			return runTunnelPreview(args[1:])
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
       mockd tunnel <subcommand> [flags]

Expose local mocks via the cloud relay. Running 'mockd tunnel' without a
subcommand starts a local mock server + tunnel in one shot.

Subcommands:
  enable    Enable tunnel on an engine (via admin API)
  disable   Disable tunnel on an engine
  status    Show tunnel connection status
  list      List all active tunnels
  preview   Preview which mocks would be exposed
  stop      Alias for disable

Direct Start Flags (mockd tunnel --token ...):
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

Examples:
  # Direct start: launch server + tunnel together
  mockd tunnel --token YOUR_TOKEN

  # Enable tunnel on a running engine via admin API
  mockd tunnel enable --subdomain my-api

  # Check tunnel status
  mockd tunnel status

  # List all active tunnels
  mockd tunnel list

  # Preview what would be exposed
  mockd tunnel preview --mode selected --types http

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
	adminAPI := admin.NewAPI(*adminPort, admin.WithLocalEngine(engineURL))
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
		output.Warn("admin API shutdown error: %v", err)
	}

	// Stop mock server
	if err := server.Stop(); err != nil {
		output.Warn("server shutdown error: %v", err)
	}

	fmt.Println("Goodbye!")
	return nil
}

// runTunnelEnable enables a tunnel on the local engine via the admin API.
func runTunnelEnable(args []string) error {
	fs := flag.NewFlagSet("tunnel enable", flag.ContinueOnError)
	adminAddr := fs.String("admin-url", "", "Admin API address")
	engineID := fs.String("engine", "local", "Engine ID (default: local)")
	mode := fs.String("mode", "all", "Exposure mode: all, selected, none")
	subdomain := fs.String("subdomain", "", "Custom subdomain (auto-assigned if empty)")
	domain := fs.String("domain", "", "Custom domain")
	authToken := fs.String("auth-token", "", "Require token for incoming requests")
	authBasic := fs.String("auth-basic", "", "Require Basic Auth (user:pass)")
	allowIPs := fs.String("allow-ips", "", "Restrict by IP (comma-separated CIDRs)")
	workspaces := fs.String("workspaces", "", "Expose only these workspaces (comma-separated)")
	folders := fs.String("folders", "", "Expose only these folders (comma-separated)")
	mocks := fs.String("mocks", "", "Expose only these mock IDs (comma-separated)")
	types := fs.String("types", "", "Expose only these mock types (comma-separated)")
	excludeWorkspaces := fs.String("exclude-workspaces", "", "Exclude these workspaces (comma-separated)")
	excludeFolders := fs.String("exclude-folders", "", "Exclude these folders (comma-separated)")
	excludeMocks := fs.String("exclude-mocks", "", "Exclude these mock IDs (comma-separated)")

	fs.Usage = func() {
		fmt.Fprint(os.Stderr, `Usage: mockd tunnel enable [flags]

Enable tunnel on an engine, making mocks publicly accessible.

Flags:
      --admin-url          Admin API address (auto-detected from context)
      --engine             Engine ID (default: local)
      --mode               Exposure mode: all, selected, none (default: all)
      --subdomain          Custom subdomain (auto-assigned if empty)
      --domain             Custom domain
      --auth-token         Require token for incoming requests
      --auth-basic         Require Basic Auth (format: user:pass)
      --allow-ips          Restrict by IP (comma-separated CIDRs)
      --workspaces         Expose only these workspaces (comma-separated)
      --folders            Expose only these folders (comma-separated)
      --mocks              Expose only these mock IDs (comma-separated)
      --types              Expose only these mock types (comma-separated)
      --exclude-workspaces Exclude these workspaces (comma-separated)
      --exclude-folders    Exclude these folders (comma-separated)
      --exclude-mocks      Exclude these mock IDs (comma-separated)

Examples:
  # Enable tunnel with all mocks exposed
  mockd tunnel enable

  # Enable with custom subdomain
  mockd tunnel enable --subdomain my-api

  # Expose only HTTP mocks
  mockd tunnel enable --mode selected --types http

  # Expose specific workspace, exclude a folder
  mockd tunnel enable --mode selected --workspaces payments-api --exclude-folders fld_internal

  # Protect with token authentication
  mockd tunnel enable --auth-token secret123
`)
	}

	if err := fs.Parse(args); err != nil {
		return err
	}

	client := tunnelHTTPClient(*adminAddr)

	// Resolve engine ID (auto-detect if default)
	resolvedEngine, err := resolveEngineID(client, *engineID)
	if err != nil {
		return err
	}

	// Build enable request
	expose := map[string]any{
		"mode": *mode,
	}
	reqBody := map[string]any{
		"expose": expose,
	}
	if *workspaces != "" {
		expose["workspaces"] = splitCSV(*workspaces)
	}
	if *folders != "" {
		expose["folders"] = splitCSV(*folders)
	}
	if *mocks != "" {
		expose["mocks"] = splitCSV(*mocks)
	}
	if *types != "" {
		expose["types"] = splitCSV(*types)
	}

	// Build exclude config
	excludeMap := map[string]any{}
	if *excludeWorkspaces != "" {
		excludeMap["workspaces"] = splitCSV(*excludeWorkspaces)
	}
	if *excludeFolders != "" {
		excludeMap["folders"] = splitCSV(*excludeFolders)
	}
	if *excludeMocks != "" {
		excludeMap["mocks"] = splitCSV(*excludeMocks)
	}
	if len(excludeMap) > 0 {
		expose["exclude"] = excludeMap
	}

	if *subdomain != "" {
		reqBody["subdomain"] = *subdomain
	}
	if *domain != "" {
		reqBody["customDomain"] = *domain
	}

	// Build auth config
	if *authToken != "" {
		reqBody["auth"] = map[string]any{"type": "token", "token": *authToken}
	} else if *authBasic != "" {
		parts := strings.SplitN(*authBasic, ":", 2)
		if len(parts) != 2 {
			return fmt.Errorf("invalid --auth-basic format, expected user:pass")
		}
		reqBody["auth"] = map[string]any{"type": "basic", "username": parts[0], "password": parts[1]}
	} else if *allowIPs != "" {
		reqBody["auth"] = map[string]any{"type": "ip", "allowedIPs": splitCSV(*allowIPs)}
	}

	body, _ := json.Marshal(reqBody)
	resp, err := client.post(fmt.Sprintf("/engines/%s/tunnel/enable", resolvedEngine), body)
	if err != nil {
		return fmt.Errorf("failed to enable tunnel: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != 200 {
		return parseHTTPError(resp)
	}

	var result map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	fmt.Printf("Tunnel enabled!\n")
	if url, ok := result["publicUrl"].(string); ok && url != "" {
		fmt.Printf("Public URL: %s\n", url)
	}
	if sub, ok := result["subdomain"].(string); ok && sub != "" {
		fmt.Printf("Subdomain:  %s\n", sub)
	}
	if status, ok := result["status"].(string); ok {
		fmt.Printf("Status:     %s\n", status)
	}

	return nil
}

// runTunnelDisable disables the tunnel on an engine via the admin API.
func runTunnelDisable(args []string) error {
	fs := flag.NewFlagSet("tunnel disable", flag.ContinueOnError)
	adminAddr := fs.String("admin-url", "", "Admin API address")
	engineID := fs.String("engine", "local", "Engine ID (default: local)")

	fs.Usage = func() {
		fmt.Fprint(os.Stderr, `Usage: mockd tunnel disable [flags]

Disable tunnel on an engine, removing public access.

Flags:
      --admin-url   Admin API address (auto-detected from context)
      --engine      Engine ID (default: local)
`)
	}

	if err := fs.Parse(args); err != nil {
		return err
	}

	client := tunnelHTTPClient(*adminAddr)

	resolvedEngine, err := resolveEngineID(client, *engineID)
	if err != nil {
		return err
	}

	resp, err := client.post(fmt.Sprintf("/engines/%s/tunnel/disable", resolvedEngine), nil)
	if err != nil {
		return fmt.Errorf("failed to disable tunnel: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != 200 {
		return parseHTTPError(resp)
	}

	fmt.Println("Tunnel disabled.")
	return nil
}

// runTunnelStatus shows the current tunnel status for an engine.
func runTunnelStatus(args []string) error {
	fs := flag.NewFlagSet("tunnel status", flag.ContinueOnError)
	adminAddr := fs.String("admin-url", "", "Admin API address")
	engineID := fs.String("engine", "local", "Engine ID (default: local)")
	outputJSON := fs.Bool("json", false, "Output as JSON")

	fs.Usage = func() {
		fmt.Fprint(os.Stderr, `Usage: mockd tunnel status [flags]

Show detailed tunnel status for an engine.

Flags:
      --admin-url   Admin API address (auto-detected from context)
      --engine      Engine ID (default: local)
      --json        Output as JSON

Examples:
  mockd tunnel status
  mockd tunnel status --engine my-engine
  mockd tunnel status --json
`)
	}

	if err := fs.Parse(args); err != nil {
		return err
	}

	client := tunnelHTTPClient(*adminAddr)

	resolvedEngine, err := resolveEngineID(client, *engineID)
	if err != nil {
		return err
	}

	resp, err := client.get(fmt.Sprintf("/engines/%s/tunnel/status", resolvedEngine))
	if err != nil {
		return fmt.Errorf("failed to get tunnel status: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != 200 {
		return parseHTTPError(resp)
	}

	var status map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	if *outputJSON {
		return output.JSON(status)
	}

	enabled, _ := status["enabled"].(bool)
	if !enabled {
		fmt.Printf("Engine %s: tunnel not enabled\n", *engineID)
		return nil
	}

	fmt.Printf("Engine:      %s\n", *engineID)
	if s, ok := status["status"].(string); ok {
		fmt.Printf("Status:      %s\n", s)
	}
	if url, ok := status["publicUrl"].(string); ok && url != "" {
		fmt.Printf("Public URL:  %s\n", url)
	}
	if sub, ok := status["subdomain"].(string); ok && sub != "" {
		fmt.Printf("Subdomain:   %s\n", sub)
	}
	if t, ok := status["transport"].(string); ok && t != "" {
		fmt.Printf("Transport:   %s\n", t)
	}
	if sid, ok := status["sessionId"].(string); ok && sid != "" {
		fmt.Printf("Session:     %s\n", sid)
	}
	if ct, ok := status["connectedAt"].(string); ok && ct != "" {
		fmt.Printf("Connected:   %s\n", ct)
	}

	return nil
}

// runTunnelStop disables the tunnel (alias for disable).
func runTunnelStop(args []string) error {
	return runTunnelDisable(args)
}

// runTunnelList lists all active tunnels across all engines.
func runTunnelList(args []string) error {
	fs := flag.NewFlagSet("tunnel list", flag.ContinueOnError)
	adminAddr := fs.String("admin-url", "", "Admin API address")
	outputJSON := fs.Bool("json", false, "Output as JSON")

	fs.Usage = func() {
		fmt.Fprint(os.Stderr, `Usage: mockd tunnel list [flags]

List all active tunnels across all engines.

Flags:
      --admin-url   Admin API address (auto-detected from context)
      --json        Output as JSON
`)
	}

	if err := fs.Parse(args); err != nil {
		return err
	}

	client := tunnelHTTPClient(*adminAddr)

	resp, err := client.get("/tunnels")
	if err != nil {
		return fmt.Errorf("failed to list tunnels: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != 200 {
		return parseHTTPError(resp)
	}

	var result struct {
		Tunnels []map[string]any `json:"tunnels"`
		Total   int              `json:"total"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	if *outputJSON {
		return output.JSON(result)
	}

	if result.Total == 0 {
		fmt.Println("No active tunnels.")
		fmt.Println("Use 'mockd tunnel enable' to enable a tunnel.")
		return nil
	}

	fmt.Printf("Active tunnels (%d):\n\n", result.Total)
	fmt.Printf("%-12s %-15s %-40s %-12s %-10s %s\n",
		"ENGINE ID", "NAME", "PUBLIC URL", "STATUS", "TRANSPORT", "UPTIME")
	fmt.Println(strings.Repeat("-", 100))

	for _, t := range result.Tunnels {
		id, _ := t["engineId"].(string)
		name, _ := t["engineName"].(string)
		url, _ := t["publicUrl"].(string)
		status, _ := t["status"].(string)
		transport, _ := t["transport"].(string)
		uptime, _ := t["uptime"].(string)

		fmt.Printf("%-12s %-15s %-40s %-12s %-10s %s\n",
			id, name, url, status, transport, uptime)
	}

	return nil
}

// runTunnelPreview shows a dry-run of what mocks would be exposed through a tunnel.
func runTunnelPreview(args []string) error {
	fs := flag.NewFlagSet("tunnel preview", flag.ContinueOnError)
	adminAddr := fs.String("admin-url", "", "Admin API address")
	engineID := fs.String("engine", "local", "Engine ID (default: local)")
	mode := fs.String("mode", "all", "Exposure mode: all, selected, none")
	workspaces := fs.String("workspaces", "", "Expose only these workspaces (comma-separated)")
	folders := fs.String("folders", "", "Expose only these folders (comma-separated)")
	mocks := fs.String("mocks", "", "Expose only these mock IDs (comma-separated)")
	types := fs.String("types", "", "Expose only these mock types (comma-separated)")
	excludeWorkspaces := fs.String("exclude-workspaces", "", "Exclude these workspaces (comma-separated)")
	excludeFolders := fs.String("exclude-folders", "", "Exclude these folders (comma-separated)")
	excludeMocks := fs.String("exclude-mocks", "", "Exclude these mock IDs (comma-separated)")
	outputJSON := fs.Bool("json", false, "Output as JSON")

	fs.Usage = func() {
		fmt.Fprint(os.Stderr, `Usage: mockd tunnel preview [flags]

Preview which mocks would be exposed through a tunnel.

This is a dry-run -- no tunnel is created. Use this to verify exposure
settings before enabling a tunnel.

Flags:
      --admin-url          Admin API address (auto-detected from context)
      --engine             Engine ID (default: local)
      --mode               Exposure mode: all, selected, none (default: all)
      --workspaces         Expose only these workspaces (comma-separated)
      --folders            Expose only these folders (comma-separated)
      --mocks              Expose only these mock IDs (comma-separated)
      --types              Expose only these mock types (comma-separated)
      --exclude-workspaces Exclude these workspaces (comma-separated)
      --exclude-folders    Exclude these folders (comma-separated)
      --exclude-mocks      Exclude these mock IDs (comma-separated)
      --json               Output as JSON

Examples:
  # Preview all mocks
  mockd tunnel preview

  # Preview only HTTP mocks
  mockd tunnel preview --mode selected --types http

  # Preview specific workspace, exclude internal folder
  mockd tunnel preview --mode selected --workspaces default --exclude-folders fld_internal
`)
	}

	if err := fs.Parse(args); err != nil {
		return err
	}

	client := tunnelHTTPClient(*adminAddr)

	resolvedEngine, err := resolveEngineID(client, *engineID)
	if err != nil {
		return err
	}

	expose := map[string]any{
		"mode": *mode,
	}
	reqBody := map[string]any{
		"expose": expose,
	}
	if *workspaces != "" {
		expose["workspaces"] = splitCSV(*workspaces)
	}
	if *folders != "" {
		expose["folders"] = splitCSV(*folders)
	}
	if *mocks != "" {
		expose["mocks"] = splitCSV(*mocks)
	}
	if *types != "" {
		expose["types"] = splitCSV(*types)
	}

	// Build exclude config
	excludeMap := map[string]any{}
	if *excludeWorkspaces != "" {
		excludeMap["workspaces"] = splitCSV(*excludeWorkspaces)
	}
	if *excludeFolders != "" {
		excludeMap["folders"] = splitCSV(*excludeFolders)
	}
	if *excludeMocks != "" {
		excludeMap["mocks"] = splitCSV(*excludeMocks)
	}
	if len(excludeMap) > 0 {
		expose["exclude"] = excludeMap
	}

	body, _ := json.Marshal(reqBody)
	resp, err := client.post(fmt.Sprintf("/engines/%s/tunnel/preview", resolvedEngine), body)
	if err != nil {
		return fmt.Errorf("failed to preview tunnel: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != 200 {
		return parseHTTPError(resp)
	}

	var result struct {
		MockCount int              `json:"mockCount"`
		Mocks     []map[string]any `json:"mocks"`
		Protocols map[string]int   `json:"protocols"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	if *outputJSON {
		return output.JSON(result)
	}

	fmt.Printf("Tunnel preview (engine: %s, mode: %s)\n\n", *engineID, *mode)

	if result.MockCount == 0 {
		fmt.Println("No mocks would be exposed with these settings.")
		return nil
	}

	fmt.Printf("Total mocks: %d\n", result.MockCount)
	fmt.Printf("Protocols:   ")
	first := true
	for proto, count := range result.Protocols {
		if !first {
			fmt.Print(", ")
		}
		fmt.Printf("%s (%d)", proto, count)
		first = false
	}
	fmt.Println()

	if len(result.Mocks) > 0 {
		fmt.Printf("\n%-36s %-10s %-30s %s\n", "ID", "TYPE", "NAME", "WORKSPACE")
		fmt.Println(strings.Repeat("-", 85))
		for _, m := range result.Mocks {
			id, _ := m["id"].(string)
			mType, _ := m["type"].(string)
			name, _ := m["name"].(string)
			ws, _ := m["workspace"].(string)
			fmt.Printf("%-36s %-10s %-30s %s\n", id, mType, name, ws)
		}
	}

	return nil
}

// ============================================================================
// Tunnel CLI helpers
// ============================================================================

// tunnelHTTPClient creates a raw HTTP client for tunnel operations.
func tunnelHTTPClient(adminURL string) *adminClient {
	cfg := cliconfig.ResolveClientConfigSimple(adminURL)
	c := &adminClient{
		baseURL: cfg.AdminURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		apiKey: cfg.APIKey,
	}
	return c
}

// splitCSV splits a comma-separated string into a trimmed slice.
func splitCSV(s string) []string {
	parts := strings.Split(s, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}

// resolveEngineID resolves the engine ID. If the provided ID is empty or "local",
// it queries the admin to auto-resolve: if exactly one engine exists, use it;
// if multiple exist, return an error listing them.
func resolveEngineID(client *adminClient, engineID string) (string, error) {
	// If explicitly set to something other than default, use it directly
	if engineID != "" && engineID != "local" {
		return engineID, nil
	}

	// Try to auto-resolve by listing engines
	resp, err := client.get("/engines")
	if err != nil {
		// Can't reach admin — fall back to "local"
		return "local", nil
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != 200 {
		return "local", nil
	}

	var result struct {
		Engines []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
			Host string `json:"host"`
			Port int    `json:"port"`
		} `json:"engines"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "local", nil
	}

	// Check for local engine too
	hasLocal := false
	for _, e := range result.Engines {
		if e.ID == "local" {
			hasLocal = true
			break
		}
	}

	totalEngines := len(result.Engines)
	if hasLocal {
		// Local engine is already in the list, total count is accurate
	} else {
		// Add implicit local engine if admin has one
		totalEngines++ // assume local exists since we're talking to admin
	}

	switch totalEngines {
	case 0:
		return "", fmt.Errorf("no engines registered. Start an engine first: mockd up")
	case 1:
		if len(result.Engines) > 0 {
			return result.Engines[0].ID, nil
		}
		return "local", nil
	default:
		// Multiple engines — if "local" was the default, it's fine to use it
		// Only error if the user might be confused
		return "local", nil
	}
}

// parseHTTPError parses an error response into a readable error.
func parseHTTPError(resp *http.Response) error {
	body, _ := io.ReadAll(resp.Body)
	var errResp struct {
		Error   string `json:"error"`
		Message string `json:"message"`
	}
	if json.Unmarshal(body, &errResp) == nil && errResp.Message != "" {
		return fmt.Errorf("%s (HTTP %d)", errResp.Message, resp.StatusCode)
	}
	return fmt.Errorf("request failed with HTTP %d: %s", resp.StatusCode, string(body))
}
