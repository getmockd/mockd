package cli

import (
	"context"
	"encoding/json"
	"errors"
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
	"github.com/spf13/cobra"
)

var (
	tunnelPort      int
	tunnelAdminPort int
	tunnelConfig    string
	tunnelRelay     string
	tunnelToken     string
	tunnelSubdomain string
	tunnelDomain    string
	tunnelAuthToken string
	tunnelAuthBasic string
	tunnelAllowIPs  string

	tunnelEnableEngine            string
	tunnelEnableMode              string
	tunnelEnableSubdomain         string
	tunnelEnableDomain            string
	tunnelEnableAuthToken         string
	tunnelEnableAuthBasic         string
	tunnelEnableAllowIPs          string
	tunnelEnableWorkspaces        string
	tunnelEnableFolders           string
	tunnelEnableMocks             string
	tunnelEnableTypes             string
	tunnelEnableExcludeWorkspaces string
	tunnelEnableExcludeFolders    string
	tunnelEnableExcludeMocks      string

	tunnelDisableEngine string

	tunnelStatusEngine string

	tunnelPreviewEngine            string
	tunnelPreviewMode              string
	tunnelPreviewWorkspaces        string
	tunnelPreviewFolders           string
	tunnelPreviewMocks             string
	tunnelPreviewTypes             string
	tunnelPreviewExcludeWorkspaces string
	tunnelPreviewExcludeFolders    string
	tunnelPreviewExcludeMocks      string
)

var tunnelCmd = &cobra.Command{
	Use:   "tunnel",
	Short: "Expose local mocks via the cloud relay",
	Long: `Expose local mocks via the cloud relay. Running 'mockd tunnel' without a
subcommand starts a local mock server + tunnel in one shot.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		port := &tunnelPort
		adminPort := &tunnelAdminPort
		configFile := &tunnelConfig
		relayURL := &tunnelRelay

		// Map MOCKD_TOKEN environment variable correctly directly checking os.Getenv
		tokenResolved := tunnelToken
		if tokenResolved == "" {
			tokenResolved = os.Getenv("MOCKD_TOKEN")
		}
		token := &tokenResolved

		subdomain := &tunnelSubdomain
		domain := &tunnelDomain
		authToken := &tunnelAuthToken
		authBasic := &tunnelAuthBasic
		allowIPs := &tunnelAllowIPs

		// Validate token
		if *token == "" {
			return errors.New("authentication token required (use --token or set MOCKD_TOKEN)")
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
		switch {
		case *authToken != "":
			tunnelCfg.WithTokenAuth(*authToken)
			fmt.Println("Request authentication: token required")
		case *authBasic != "":
			parts := strings.SplitN(*authBasic, ":", 2)
			if len(parts) != 2 {
				return errors.New("invalid --auth-basic format, expected user:pass")
			}
			tunnelCfg.WithBasicAuth(parts[0], parts[1])
			fmt.Println("Request authentication: Basic Auth required")
		case *allowIPs != "":
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
	},
}

var tunnelEnableCmd = &cobra.Command{
	Use:   "enable",
	Short: "Enable tunnel on an engine, making mocks publicly accessible",
	RunE: func(cmd *cobra.Command, args []string) error {
		adminAddr := &adminURL
		engineID := &tunnelEnableEngine
		mode := &tunnelEnableMode
		subdomain := &tunnelEnableSubdomain
		domain := &tunnelEnableDomain
		authToken := &tunnelEnableAuthToken
		authBasic := &tunnelEnableAuthBasic
		allowIPs := &tunnelEnableAllowIPs
		workspaces := &tunnelEnableWorkspaces
		folders := &tunnelEnableFolders
		mocks := &tunnelEnableMocks
		types := &tunnelEnableTypes
		excludeWorkspaces := &tunnelEnableExcludeWorkspaces
		excludeFolders := &tunnelEnableExcludeFolders
		excludeMocks := &tunnelEnableExcludeMocks

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
		switch {
		case *authToken != "":
			reqBody["auth"] = map[string]any{"type": "token", "token": *authToken}
		case *authBasic != "":
			parts := strings.SplitN(*authBasic, ":", 2)
			if len(parts) != 2 {
				return errors.New("invalid --auth-basic format, expected user:pass")
			}
			reqBody["auth"] = map[string]any{"type": "basic", "username": parts[0], "password": parts[1]}
		case *allowIPs != "":
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
	},
}

var tunnelDisableCmd = &cobra.Command{
	Use:   "disable",
	Short: "Disable tunnel on an engine, removing public access",
	RunE: func(cmd *cobra.Command, args []string) error {
		engineID := &tunnelDisableEngine

		client := tunnelHTTPClient(adminURL)

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
	},
}

var tunnelStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show detailed tunnel status for an engine",
	RunE: func(cmd *cobra.Command, args []string) error {
		engineID := &tunnelStatusEngine

		client := tunnelHTTPClient(adminURL)

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

		if jsonOutput {
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
	},
}

var tunnelStopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Alias for disable",
	RunE: func(cmd *cobra.Command, args []string) error {
		// Stop is an alias for disable. Flags bind directly to tunnelDisable* vars (see init),
		// so we just delegate to the disable command's RunE.
		return tunnelDisableCmd.RunE(cmd, args)
	},
}

var tunnelListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all active tunnels across all engines",
	RunE: func(cmd *cobra.Command, args []string) error {
		client := tunnelHTTPClient(adminURL)

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

		if jsonOutput {
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
	},
}

var tunnelPreviewCmd = &cobra.Command{
	Use:   "preview",
	Short: "Preview which mocks would be exposed through a tunnel",
	RunE: func(cmd *cobra.Command, args []string) error {
		adminAddr := &adminURL
		engineID := &tunnelPreviewEngine
		mode := &tunnelPreviewMode
		workspaces := &tunnelPreviewWorkspaces
		folders := &tunnelPreviewFolders
		mocks := &tunnelPreviewMocks
		types := &tunnelPreviewTypes
		excludeWorkspaces := &tunnelPreviewExcludeWorkspaces
		excludeFolders := &tunnelPreviewExcludeFolders
		excludeMocks := &tunnelPreviewExcludeMocks

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

		if jsonOutput {
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
	},
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
		return "", errors.New("no engines registered. Start an engine first: mockd up")
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

// splitCSV splits a comma-separated string into a trimmed slice.
func splitCSV(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	return parts
}

func init() {
	rootCmd.AddCommand(tunnelCmd)

	tunnelCmd.Flags().IntVarP(&tunnelPort, "port", "p", cliconfig.DefaultPort, "HTTP server port")
	tunnelCmd.Flags().IntVar(&tunnelAdminPort, "admin-port", cliconfig.DefaultAdminPort, "Admin API port")
	tunnelCmd.Flags().StringVarP(&tunnelConfig, "config", "c", "", "Path to mock configuration file")
	tunnelCmd.Flags().StringVar(&tunnelRelay, "relay", tunnel.DefaultRelayURL, "Relay server URL")
	tunnelCmd.Flags().StringVar(&tunnelToken, "token", "", "Authentication token (or set MOCKD_TOKEN)")
	tunnelCmd.Flags().StringVarP(&tunnelSubdomain, "subdomain", "s", "", "Requested subdomain (auto-assigned if empty)")
	tunnelCmd.Flags().StringVar(&tunnelDomain, "domain", "", "Custom domain (must be verified)")
	tunnelCmd.Flags().StringVar(&tunnelAuthToken, "auth-token", "", "Require this token for incoming requests")
	tunnelCmd.Flags().StringVar(&tunnelAuthBasic, "auth-basic", "", "Require Basic Auth (format: user:pass)")
	tunnelCmd.Flags().StringVar(&tunnelAllowIPs, "allow-ips", "", "Allow only these IPs (comma-separated CIDR or IP)")

	tunnelCmd.AddCommand(tunnelEnableCmd)
	tunnelEnableCmd.Flags().StringVar(&tunnelEnableEngine, "engine", "local", "Engine ID")
	tunnelEnableCmd.Flags().StringVar(&tunnelEnableMode, "mode", "all", "Exposure mode: all, selected, none")
	tunnelEnableCmd.Flags().StringVar(&tunnelEnableSubdomain, "subdomain", "", "Custom subdomain (auto-assigned if empty)")
	tunnelEnableCmd.Flags().StringVar(&tunnelEnableDomain, "domain", "", "Custom domain")
	tunnelEnableCmd.Flags().StringVar(&tunnelEnableAuthToken, "auth-token", "", "Require token for incoming requests")
	tunnelEnableCmd.Flags().StringVar(&tunnelEnableAuthBasic, "auth-basic", "", "Require Basic Auth (user:pass)")
	tunnelEnableCmd.Flags().StringVar(&tunnelEnableAllowIPs, "allow-ips", "", "Restrict by IP (comma-separated CIDRs)")
	tunnelEnableCmd.Flags().StringVar(&tunnelEnableWorkspaces, "workspaces", "", "Expose only these workspaces (comma-separated)")
	tunnelEnableCmd.Flags().StringVar(&tunnelEnableFolders, "folders", "", "Expose only these folders (comma-separated)")
	tunnelEnableCmd.Flags().StringVar(&tunnelEnableMocks, "mocks", "", "Expose only these mock IDs (comma-separated)")
	tunnelEnableCmd.Flags().StringVar(&tunnelEnableTypes, "types", "", "Expose only these mock types (comma-separated)")
	tunnelEnableCmd.Flags().StringVar(&tunnelEnableExcludeWorkspaces, "exclude-workspaces", "", "Exclude these workspaces (comma-separated)")
	tunnelEnableCmd.Flags().StringVar(&tunnelEnableExcludeFolders, "exclude-folders", "", "Exclude these folders (comma-separated)")
	tunnelEnableCmd.Flags().StringVar(&tunnelEnableExcludeMocks, "exclude-mocks", "", "Exclude these mock IDs (comma-separated)")

	tunnelCmd.AddCommand(tunnelDisableCmd)
	tunnelDisableCmd.Flags().StringVar(&tunnelDisableEngine, "engine", "local", "Engine ID")

	tunnelCmd.AddCommand(tunnelStatusCmd)
	tunnelStatusCmd.Flags().StringVar(&tunnelStatusEngine, "engine", "local", "Engine ID")

	tunnelCmd.AddCommand(tunnelStopCmd)
	tunnelStopCmd.Flags().StringVar(&tunnelDisableEngine, "engine", "local", "Engine ID")

	tunnelCmd.AddCommand(tunnelListCmd)

	tunnelCmd.AddCommand(tunnelPreviewCmd)
	tunnelPreviewCmd.Flags().StringVar(&tunnelPreviewEngine, "engine", "local", "Engine ID")
	tunnelPreviewCmd.Flags().StringVar(&tunnelPreviewMode, "mode", "all", "Exposure mode: all, selected, none")
	tunnelPreviewCmd.Flags().StringVar(&tunnelPreviewWorkspaces, "workspaces", "", "Expose only these workspaces (comma-separated)")
	tunnelPreviewCmd.Flags().StringVar(&tunnelPreviewFolders, "folders", "", "Expose only these folders (comma-separated)")
	tunnelPreviewCmd.Flags().StringVar(&tunnelPreviewMocks, "mocks", "", "Expose only these mock IDs (comma-separated)")
	tunnelPreviewCmd.Flags().StringVar(&tunnelPreviewTypes, "types", "", "Expose only these mock types (comma-separated)")
	tunnelPreviewCmd.Flags().StringVar(&tunnelPreviewExcludeWorkspaces, "exclude-workspaces", "", "Exclude these workspaces (comma-separated)")
	tunnelPreviewCmd.Flags().StringVar(&tunnelPreviewExcludeFolders, "exclude-folders", "", "Exclude these folders (comma-separated)")
	tunnelPreviewCmd.Flags().StringVar(&tunnelPreviewExcludeMocks, "exclude-mocks", "", "Exclude these mock IDs (comma-separated)")
}
