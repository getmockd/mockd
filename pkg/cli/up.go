package cli

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/getmockd/mockd/pkg/admin"
	"github.com/getmockd/mockd/pkg/admin/engineclient"
	"github.com/getmockd/mockd/pkg/cli/internal/output"
	"github.com/getmockd/mockd/pkg/cli/internal/ports"
	"github.com/getmockd/mockd/pkg/config"
	"github.com/getmockd/mockd/pkg/engine"
	"github.com/getmockd/mockd/pkg/logging"
	"github.com/getmockd/mockd/pkg/mock"
	"github.com/getmockd/mockd/pkg/store"
	"github.com/getmockd/mockd/pkg/tunnel"
	"github.com/spf13/cobra"
)

// upContext holds runtime state for the up command.
type upContext struct {
	cfg        *config.ProjectConfig
	configPath string
	detach     bool
	log        *slog.Logger
	ctx        context.Context
	cancel     context.CancelFunc

	// Running services (process-level)
	admins  map[string]*admin.API
	engines map[string]*engine.Server
	tunnels map[string]*tunnel.TunnelManager // engine name -> tunnel manager

	// HTTP clients for admin APIs (used for all control-plane operations)
	adminClients map[string]AdminClient // admin name -> HTTP client

	// Engine registration info (populated after HTTP registration)
	engineIDs map[string]string // engine name -> registered engine ID

	// Workspace IDs created via HTTP (workspace config name -> workspace ID)
	workspaceIDs map[string]string
}

var (
	upConfigFiles []string
	upDetach      bool
	upLogLevel    string
)

var upCmd = &cobra.Command{
	Use:   "up",
	Short: "Start local admins and engines defined in mockd.yaml",
	Long: `Start local admins and engines defined in mockd.yaml.

This command:
  1. Loads and validates the config
  2. Starts local admin servers (those without 'url' field)
  3. Starts local engine servers  
  4. Applies workspaces and mocks to local admins
  5. Runs in foreground (or background with -d)`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Load config
		cfg, configPath, err := loadProjectConfig(upConfigFiles)
		if err != nil {
			return err
		}

		// Validate config
		result := config.ValidateProjectConfig(cfg)
		if !result.IsValid() {
			fmt.Fprintln(os.Stderr, "Configuration validation failed:")
			for _, e := range result.Errors {
				fmt.Fprintf(os.Stderr, "  - %s\n", e.Error())
			}
			return errors.New("invalid configuration")
		}

		// Check port conflicts in config
		portResult := config.ValidatePortConflicts(cfg)
		if !portResult.IsValid() {
			fmt.Fprintln(os.Stderr, "Port conflicts in config:")
			for _, e := range portResult.Errors {
				fmt.Fprintf(os.Stderr, "  - %s\n", e.Error())
			}
			return errors.New("port conflicts")
		}

		// Check actual port availability
		if err := checkProjectPorts(cfg); err != nil {
			return err
		}

		// Create context
		ctx, cancel := context.WithCancel(context.Background())

		uctx := &upContext{
			cfg:          cfg,
			configPath:   configPath,
			detach:       upDetach,
			ctx:          ctx,
			cancel:       cancel,
			admins:       make(map[string]*admin.API),
			engines:      make(map[string]*engine.Server),
			tunnels:      make(map[string]*tunnel.TunnelManager),
			adminClients: make(map[string]AdminClient),
			engineIDs:    make(map[string]string),
			workspaceIDs: make(map[string]string),
		}

		// Initialize logger
		uctx.log = logging.New(logging.Config{
			Level:  logging.ParseLevel(upLogLevel),
			Format: logging.FormatText,
		})

		// Print startup info
		fmt.Printf("Starting mockd from %s\n", configPath)
		printUpSummary(cfg)

		if upDetach {
			// TODO: Implement proper daemonization
			fmt.Println("Detached mode not yet fully implemented. Running in foreground...")
		}

		return uctx.run()
	},
}

func init() {
	upCmd.Flags().StringSliceVarP(&upConfigFiles, "config", "f", nil, "Config file path (can be specified multiple times)")
	upCmd.Flags().BoolVarP(&upDetach, "detach", "d", false, "Run in background (daemon mode)")
	upCmd.Flags().StringVar(&upLogLevel, "log-level", "info", "Log level (debug, info, warn, error)")
	rootCmd.AddCommand(upCmd)
}

func loadProjectConfig(configFiles []string) (*config.ProjectConfig, string, error) {
	if len(configFiles) == 0 {
		path, err := config.DiscoverProjectConfig()
		if err != nil {
			return nil, "", err
		}
		cfg, err := config.LoadProjectConfig(path)
		if err != nil {
			return nil, "", fmt.Errorf("loading %s: %w", path, err)
		}
		return cfg, path, nil
	}

	if len(configFiles) == 1 {
		cfg, err := config.LoadProjectConfig(configFiles[0])
		if err != nil {
			return nil, "", fmt.Errorf("loading %s: %w", configFiles[0], err)
		}
		return cfg, configFiles[0], nil
	}

	cfg, err := config.LoadAndMergeProjectConfigs(configFiles)
	if err != nil {
		return nil, "", err
	}
	return cfg, strings.Join(configFiles, ", "), nil
}

func checkProjectPorts(cfg *config.ProjectConfig) error {
	// Check admin ports
	for _, a := range cfg.Admins {
		if a.IsLocal() {
			if err := ports.Check(a.Port); err != nil {
				return fmt.Errorf("admin '%s' port %d: %w", a.Name, a.Port, err)
			}
		}
	}

	// Check engine ports
	for _, e := range cfg.Engines {
		if e.HTTPPort > 0 {
			if err := ports.Check(e.HTTPPort); err != nil {
				return fmt.Errorf("engine '%s' HTTP port %d: %w", e.Name, e.HTTPPort, err)
			}
		}
		if e.HTTPSPort > 0 {
			if err := ports.Check(e.HTTPSPort); err != nil {
				return fmt.Errorf("engine '%s' HTTPS port %d: %w", e.Name, e.HTTPSPort, err)
			}
		}
		if e.GRPCPort > 0 {
			if err := ports.Check(e.GRPCPort); err != nil {
				return fmt.Errorf("engine '%s' gRPC port %d: %w", e.Name, e.GRPCPort, err)
			}
		}
	}

	return nil
}

func printUpSummary(cfg *config.ProjectConfig) {
	localAdmins := 0
	remoteAdmins := 0
	for _, a := range cfg.Admins {
		if a.IsLocal() {
			localAdmins++
		} else {
			remoteAdmins++
		}
	}

	fmt.Printf("  Admins: %d local", localAdmins)
	if remoteAdmins > 0 {
		fmt.Printf(", %d remote", remoteAdmins)
	}
	fmt.Println()
	fmt.Printf("  Engines: %d\n", len(cfg.Engines))
	fmt.Printf("  Workspaces: %d\n", len(cfg.Workspaces))

	mockCount := 0
	fileRefs := 0
	for _, m := range cfg.Mocks {
		if m.IsInline() {
			mockCount++
		} else {
			fileRefs++
		}
	}
	if mockCount > 0 || fileRefs > 0 {
		fmt.Printf("  Mocks: %d inline", mockCount)
		if fileRefs > 0 {
			fmt.Printf(", %d file refs", fileRefs)
		}
		fmt.Println()
	}

	tunnelCount := 0
	for _, e := range cfg.Engines {
		if e.Tunnel != nil && e.Tunnel.Enabled {
			tunnelCount++
		}
	}
	if tunnelCount > 0 {
		fmt.Printf("  Tunnels: %d\n", tunnelCount)
	}

	fmt.Println()
}

func (uctx *upContext) run() error {
	defer uctx.cancel()

	// Start all services
	if err := uctx.startAll(); err != nil {
		uctx.stopAll()
		return err
	}

	// Load and apply mocks from config (including file references and globs)
	if err := uctx.loadAndApplyMocks(); err != nil {
		uctx.stopAll()
		return fmt.Errorf("loading mocks: %w", err)
	}

	// Load and apply stateful resources from config
	if err := uctx.loadAndApplyStatefulResources(); err != nil {
		uctx.stopAll()
		return fmt.Errorf("loading stateful resources: %w", err)
	}

	// Write PID file
	pidPath := defaultUpPIDPath()
	if err := uctx.writePIDFile(pidPath); err != nil {
		output.Warn("failed to write PID file: %v", err)
	}
	defer func() { _ = os.Remove(pidPath) }()

	// Print running status
	uctx.printRunningStatus()

	// Set up signal handling
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	fmt.Println("Press Ctrl+C to stop")

	// Wait for shutdown signal
	<-sigCh
	fmt.Println("\nShutting down...")

	// Graceful shutdown
	uctx.stopAll()

	fmt.Println("All services stopped")
	return nil
}

func (uctx *upContext) writePIDFile(path string) error {
	services := []config.PIDFileService{}

	for _, adminCfg := range uctx.cfg.Admins {
		if !adminCfg.IsLocal() {
			continue
		}
		services = append(services, config.PIDFileService{
			Name: adminCfg.Name,
			Type: "admin",
			Port: adminCfg.Port,
			PID:  os.Getpid(),
		})
	}

	for _, engineCfg := range uctx.cfg.Engines {
		port := engineCfg.HTTPPort
		if port == 0 {
			port = engineCfg.HTTPSPort
		}
		if port == 0 {
			port = engineCfg.GRPCPort
		}
		services = append(services, config.PIDFileService{
			Name: engineCfg.Name,
			Type: "engine",
			Port: port,
			PID:  os.Getpid(),
		})
	}

	pidInfo := &config.PIDFile{
		PID:       os.Getpid(),
		StartedAt: time.Now().UTC().Format(time.RFC3339),
		Config:    uctx.configPath,
		Services:  services,
	}

	return writeUpPIDFile(path, pidInfo)
}

func (uctx *upContext) startAll() error {
	// Start local admins first
	for _, adminCfg := range uctx.cfg.Admins {
		if !adminCfg.IsLocal() {
			continue
		}

		if err := uctx.startAdmin(adminCfg); err != nil {
			return fmt.Errorf("starting admin '%s': %w", adminCfg.Name, err)
		}
	}

	// Build admin HTTP clients for all admins (local and remote)
	for _, adminCfg := range uctx.cfg.Admins {
		adminURL := uctx.resolveAdminURL(adminCfg)
		client := NewAdminClient(adminURL)
		uctx.adminClients[adminCfg.Name] = client
	}

	// Wait briefly for local admins to accept connections
	for _, adminCfg := range uctx.cfg.Admins {
		if !adminCfg.IsLocal() {
			continue
		}
		client := uctx.adminClients[adminCfg.Name]
		if err := uctx.waitForAdminHealth(client, 10*time.Second); err != nil {
			return fmt.Errorf("admin '%s' did not become healthy: %w", adminCfg.Name, err)
		}
	}

	// Start engines and register them with their admins via HTTP
	for _, engineCfg := range uctx.cfg.Engines {
		if err := uctx.startEngine(engineCfg); err != nil {
			return fmt.Errorf("starting engine '%s': %w", engineCfg.Name, err)
		}

		// Register the engine with its admin via HTTP
		if err := uctx.connectEngineToAdmin(engineCfg); err != nil {
			return fmt.Errorf("connecting engine '%s' to admin: %w", engineCfg.Name, err)
		}
	}

	// Create workspaces via HTTP and assign to engines
	if err := uctx.createWorkspaces(); err != nil {
		return fmt.Errorf("creating workspaces: %w", err)
	}

	// Start tunnels for engines that have tunnel config
	for _, engineCfg := range uctx.cfg.Engines {
		if engineCfg.Tunnel != nil && engineCfg.Tunnel.Enabled {
			if err := uctx.startTunnel(engineCfg); err != nil {
				// Tunnel failure is non-fatal -- log and continue
				output.Warn("tunnel for engine '%s' failed: %v", engineCfg.Name, err)
			}
		}
	}

	return nil
}

// resolveAdminURL returns the HTTP base URL for an admin.
func (uctx *upContext) resolveAdminURL(adminCfg config.AdminConfig) string {
	if adminCfg.URL != "" {
		return adminCfg.URL
	}
	return fmt.Sprintf("http://localhost:%d", adminCfg.Port)
}

// waitForAdminHealth waits for an admin API to report healthy status.
func (uctx *upContext) waitForAdminHealth(client AdminClient, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if err := client.Health(); err == nil {
			return nil
		}
		select {
		case <-uctx.ctx.Done():
			return uctx.ctx.Err()
		case <-time.After(100 * time.Millisecond):
			// Retry
		}
	}
	return errors.New("timeout waiting for admin health")
}

// createWorkspaces creates workspaces via HTTP and assigns them to engines.
func (uctx *upContext) createWorkspaces() error {
	if len(uctx.cfg.Workspaces) == 0 {
		return nil
	}

	for _, wsCfg := range uctx.cfg.Workspaces {
		// Find an admin to create the workspace on.
		// Use the admin of the first engine assigned to this workspace,
		// or the first local admin as fallback.
		adminName := uctx.findAdminForWorkspace(wsCfg)
		if adminName == "" {
			uctx.log.Warn("no admin found for workspace, skipping", "workspace", wsCfg.Name)
			continue
		}

		client, ok := uctx.adminClients[adminName]
		if !ok {
			continue
		}

		// Create workspace via HTTP: POST /workspaces
		result, err := client.CreateWorkspace(wsCfg.Name)
		if err != nil {
			// If workspace already exists (409), try to continue
			if apiErr, ok := err.(*APIError); ok && apiErr.StatusCode == 409 {
				uctx.log.Debug("workspace already exists, continuing", "name", wsCfg.Name)
				continue
			}
			return fmt.Errorf("creating workspace '%s': %w", wsCfg.Name, err)
		}
		uctx.workspaceIDs[wsCfg.Name] = result.ID
		fmt.Printf("Created workspace '%s' (id=%s)\n", wsCfg.Name, result.ID)

		// Assign workspace to its engines
		for _, engineName := range wsCfg.Engines {
			engineID, ok := uctx.engineIDs[engineName]
			if !ok {
				uctx.log.Warn("engine not registered, cannot assign workspace", "engine", engineName, "workspace", wsCfg.Name)
				continue
			}

			// Find the admin client for this engine
			engineAdmin := ""
			for _, eCfg := range uctx.cfg.Engines {
				if eCfg.Name == engineName {
					engineAdmin = eCfg.Admin
					break
				}
			}
			if engineAdmin == "" {
				continue
			}
			engAdminClient, ok := uctx.adminClients[engineAdmin]
			if !ok {
				continue
			}

			// Add workspace to engine via HTTP: POST /engines/{id}/workspaces
			if err := engAdminClient.AddEngineWorkspace(engineID, result.ID, wsCfg.Name); err != nil {
				uctx.log.Warn("failed to assign workspace to engine", "workspace", wsCfg.Name, "engine", engineName, "error", err)
			} else {
				fmt.Printf("Assigned workspace '%s' to engine '%s'\n", wsCfg.Name, engineName)
			}
		}
	}

	return nil
}

// findAdminForWorkspace finds an appropriate admin name for creating a workspace.
func (uctx *upContext) findAdminForWorkspace(wsCfg config.WorkspaceConfig) string {
	// If workspace has engines, use the admin of the first engine
	for _, engineName := range wsCfg.Engines {
		for _, eCfg := range uctx.cfg.Engines {
			if eCfg.Name == engineName {
				return eCfg.Admin
			}
		}
	}

	// Fallback: use first local admin
	for _, aCfg := range uctx.cfg.Admins {
		if aCfg.IsLocal() {
			return aCfg.Name
		}
	}
	return ""
}

func (uctx *upContext) startAdmin(adminCfg config.AdminConfig) error {
	fmt.Printf("Starting admin '%s' on port %d... ", adminCfg.Name, adminCfg.Port)

	// Build admin options
	opts := []admin.Option{
		// Enable localhost bypass so `mockd up` can register engines and
		// push workspaces/mocks via HTTP from the same machine.
		admin.WithAllowLocalhostBypass(true),
		admin.WithAPIKeyAllowLocalhost(true),
	}

	// Disable auth if configured
	if adminCfg.Auth != nil && adminCfg.Auth.Type == "none" {
		opts = append(opts, admin.WithAPIKeyDisabled())
	}
	// Honor admin persistence path when configured so `mockd up` projects can be isolated.
	if adminCfg.Persistence != nil && adminCfg.Persistence.Path != "" {
		opts = append(opts, admin.WithDataDir(adminCfg.Persistence.Path))
	}

	// Create admin API
	adminAPI := admin.NewAPI(adminCfg.Port, opts...)
	adminAPI.SetLogger(uctx.log.With("component", "admin", "name", adminCfg.Name))

	// Start admin
	if err := adminAPI.Start(); err != nil {
		fmt.Println("✗")
		return err
	}

	uctx.admins[adminCfg.Name] = adminAPI
	fmt.Println("✓")
	return nil
}

func (uctx *upContext) startEngine(engineCfg config.EngineConfig) error {
	// Find the admin this engine connects to
	var adminCfg *config.AdminConfig
	for i := range uctx.cfg.Admins {
		if uctx.cfg.Admins[i].Name == engineCfg.Admin {
			adminCfg = &uctx.cfg.Admins[i]
			break
		}
	}
	if adminCfg == nil {
		return fmt.Errorf("admin '%s' not found", engineCfg.Admin)
	}

	primaryPort := engineCfg.HTTPPort
	if primaryPort == 0 {
		primaryPort = engineCfg.HTTPSPort
	}
	if primaryPort == 0 {
		primaryPort = engineCfg.GRPCPort
	}

	fmt.Printf("Starting engine '%s' on port %d... ", engineCfg.Name, primaryPort)

	// Build server config
	serverCfg := &config.ServerConfiguration{
		HTTPPort:      engineCfg.HTTPPort,
		HTTPSPort:     engineCfg.HTTPSPort,
		AdminPort:     0, // Engine doesn't have its own admin port in new architecture
		LogRequests:   true,
		MaxLogEntries: 1000,
		ReadTimeout:   30,
		WriteTimeout:  30,
	}

	// Create engine server
	srv := engine.NewServer(serverCfg)
	srv.SetLogger(uctx.log.With("component", "engine", "name", engineCfg.Name))

	// Start server
	if err := srv.Start(); err != nil {
		fmt.Println("✗")
		return err
	}

	uctx.engines[engineCfg.Name] = srv

	// Wait for engine to be healthy
	engineURL := fmt.Sprintf("http://localhost:%d", srv.ManagementPort())
	engClient := engineclient.New(engineURL)
	if err := uctx.waitForHealth(engClient, 10*time.Second); err != nil {
		fmt.Println("✗")
		return fmt.Errorf("engine did not become healthy: %w", err)
	}

	fmt.Println("✓")
	return nil
}

func (uctx *upContext) connectEngineToAdmin(engineCfg config.EngineConfig) error {
	// Get the admin HTTP client
	client, ok := uctx.adminClients[engineCfg.Admin]
	if !ok {
		// Admin might be remote and not yet configured, skip
		return nil
	}

	// Find the engine
	srv, ok := uctx.engines[engineCfg.Name]
	if !ok {
		return fmt.Errorf("engine '%s' not found", engineCfg.Name)
	}

	// Register engine via HTTP: POST /engines/register
	mgmtPort := srv.ManagementPort()
	host := engineCfg.Host
	if host == "" {
		host = "localhost"
	}
	result, err := client.RegisterEngine(engineCfg.Name, host, mgmtPort)
	if err != nil {
		return fmt.Errorf("registering engine via HTTP: %w", err)
	}

	uctx.engineIDs[engineCfg.Name] = result.ID

	fmt.Printf("Registered engine '%s' with admin '%s' (id=%s)\n", engineCfg.Name, engineCfg.Admin, result.ID)
	return nil
}

func (uctx *upContext) stopAll() {
	// Stop tunnels first
	for name, tm := range uctx.tunnels {
		fmt.Printf("Stopping tunnel '%s'... ", name)
		if err := tm.Close(); err != nil {
			fmt.Printf("✗ (%v)\n", err)
		} else {
			fmt.Println("✓")
		}
	}

	// Stop engines
	for name, srv := range uctx.engines {
		fmt.Printf("Stopping engine '%s'... ", name)
		if err := srv.Stop(); err != nil {
			fmt.Printf("✗ (%v)\n", err)
		} else {
			fmt.Println("✓")
		}
	}

	// Stop admins
	for name, adminAPI := range uctx.admins {
		fmt.Printf("Stopping admin '%s'... ", name)
		if err := adminAPI.Stop(); err != nil {
			fmt.Printf("✗ (%v)\n", err)
		} else {
			fmt.Println("✓")
		}
	}
}

// startTunnel starts a tunnel for an engine based on its YAML config.
func (uctx *upContext) startTunnel(engineCfg config.EngineConfig) error {
	srv, ok := uctx.engines[engineCfg.Name]
	if !ok {
		return fmt.Errorf("engine '%s' not started", engineCfg.Name)
	}

	tc := engineCfg.Tunnel

	// Resolve relay address
	relayAddr := "relay.mockd.io:443"
	if tc.Relay != "" {
		relayAddr = tc.Relay
		if !strings.Contains(relayAddr, ":") {
			relayAddr += ":443"
		}
	}

	// Build store tunnel config for the admin
	tunnelCfg := &store.TunnelConfig{
		Enabled:      true,
		Subdomain:    tc.Subdomain,
		CustomDomain: tc.Domain,
		Expose: store.TunnelExposure{
			Mode: "all",
		},
	}

	// Apply exposure config from YAML
	if tc.Expose != nil {
		if tc.Expose.Mode != "" {
			tunnelCfg.Expose.Mode = tc.Expose.Mode
		}
		tunnelCfg.Expose.Workspaces = tc.Expose.Workspaces
		tunnelCfg.Expose.Folders = tc.Expose.Folders
		tunnelCfg.Expose.Mocks = tc.Expose.Mocks
		tunnelCfg.Expose.Types = tc.Expose.Types
		if tc.Expose.Exclude != nil {
			tunnelCfg.Expose.Exclude = &store.TunnelExclude{
				Workspaces: tc.Expose.Exclude.Workspaces,
				Folders:    tc.Expose.Exclude.Folders,
				Mocks:      tc.Expose.Exclude.Mocks,
			}
		}
	}

	// Apply auth config from YAML
	if tc.Auth != nil {
		tunnelCfg.Auth = &store.TunnelAuth{
			Type:       tc.Auth.Type,
			Token:      tc.Auth.Token,
			Username:   tc.Auth.Username,
			Password:   tc.Auth.Password,
			AllowedIPs: tc.Auth.AllowedIPs,
		}
	}

	// Store tunnel config on the admin (for local engine)
	if adminAPI, ok := uctx.admins[engineCfg.Admin]; ok {
		// The admin stores tunnel config so status queries work
		_ = adminAPI // Config stored via handleEnableTunnel; for up flow we set it directly below
	}

	fmt.Printf("Starting tunnel for engine '%s' (relay: %s)... ", engineCfg.Name, relayAddr)

	// Create tunnel manager
	tm := tunnel.NewTunnelManager(&tunnel.TunnelManagerConfig{
		Handler:   srv.Handler(),
		RelayAddr: relayAddr,
		Insecure:  tc.Insecure,
		Logger:    uctx.log.With("component", "tunnel", "engine", engineCfg.Name),
		OnStatusChange: func(status, publicURL, sessionID, transport string) {
			if status == "connected" {
				uctx.log.Info("tunnel connected",
					"engine", engineCfg.Name,
					"publicURL", publicURL,
				)
			}
		},
	})

	// Enable the tunnel
	if err := tm.Enable(tunnelCfg); err != nil {
		fmt.Println("✗")
		return err
	}

	uctx.tunnels[engineCfg.Name] = tm
	fmt.Println("✓")
	return nil
}

func (uctx *upContext) printRunningStatus() {
	fmt.Println("Services running:")

	for _, adminCfg := range uctx.cfg.Admins {
		if !adminCfg.IsLocal() {
			continue
		}
		fmt.Printf("  admin/%s    :%d  ready\n", adminCfg.Name, adminCfg.Port)
	}

	for _, engineCfg := range uctx.cfg.Engines {
		port := engineCfg.HTTPPort
		if port == 0 {
			port = engineCfg.HTTPSPort
		}
		if port == 0 {
			port = engineCfg.GRPCPort
		}
		fmt.Printf("  engine/%s   :%d  ready\n", engineCfg.Name, port)
	}

	// Show tunnel status
	for name, tm := range uctx.tunnels {
		connected, publicURL, _, _ := tm.Status()
		if connected {
			fmt.Printf("  tunnel/%s   %s  connected\n", name, publicURL)
		} else {
			fmt.Printf("  tunnel/%s   connecting...\n", name)
		}
	}

	fmt.Println()
}

// waitForHealth waits for the engine to report healthy status.
func (uctx *upContext) waitForHealth(client *engineclient.Client, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		err := client.Health(uctx.ctx)
		if err == nil {
			return nil
		}

		select {
		case <-uctx.ctx.Done():
			return uctx.ctx.Err()
		case <-time.After(100 * time.Millisecond):
			// Retry
		}
	}

	return errors.New("timeout waiting for engine health")
}

// loadAndApplyMocks loads mocks from the config (including file references and globs),
// converts them to the runtime mock format, and pushes them to the admin via HTTP.
func (uctx *upContext) loadAndApplyMocks() error {
	if len(uctx.cfg.Mocks) == 0 {
		return nil
	}

	// Get base directory for resolving relative paths in mock file references
	baseDir := config.GetMockFileBaseDir(uctx.configPath)

	// Load all mocks (expanding file refs and globs)
	entries, err := config.LoadAllMocks(uctx.cfg.Mocks, baseDir)
	if err != nil {
		return err
	}

	if len(entries) == 0 {
		return nil
	}

	fmt.Printf("Loading %d mocks...\n", len(entries))

	// Convert config mock entries to runtime mock.Mock objects
	var converted []*mock.Mock
	for _, entry := range entries {
		if !entry.IsInline() {
			continue
		}

		m, err := config.ConvertMockEntry(entry)
		if err != nil {
			uctx.log.Warn("failed to convert mock entry", "id", entry.ID, "error", err)
			continue
		}

		// Map workspace name to workspace ID
		if m.WorkspaceID != "" {
			if wsID, ok := uctx.workspaceIDs[m.WorkspaceID]; ok {
				m.WorkspaceID = wsID
			}
			// If workspace name doesn't map to an ID, keep the name —
			// the admin will use its default workspace handling
		}

		converted = append(converted, m)
	}

	if len(converted) == 0 {
		fmt.Println("No inline mocks to apply")
		return nil
	}

	// Group mocks by workspace to send them to the right admin
	// For now, send all mocks to the first available admin
	adminClient := uctx.findFirstAdminClient()
	if adminClient == nil {
		return errors.New("no admin client available to push mocks")
	}

	// Push mocks via HTTP: POST /mocks/bulk
	result, err := adminClient.BulkCreateMocks(converted, "")
	if err != nil {
		return fmt.Errorf("bulk creating mocks: %w", err)
	}

	fmt.Printf("Applied %d mocks via admin API\n", result.Created)
	if len(result.Warnings) > 0 {
		for _, w := range result.Warnings {
			fmt.Fprintf(os.Stderr, "  Warning: %s\n", w)
		}
	}

	return nil
}

// loadAndApplyStatefulResources pushes StatefulResourceConfig from the project config
// to the admin via ImportConfig.
func (uctx *upContext) loadAndApplyStatefulResources() error {
	if len(uctx.cfg.StatefulResources) == 0 {
		return nil
	}

	// Use the config entries directly (no conversion needed — StatefulResourceConfig
	// is now the single canonical type used in both YAML config and API transport).
	resources := make([]*config.StatefulResourceConfig, 0, len(uctx.cfg.StatefulResources))
	for i := range uctx.cfg.StatefulResources {
		resources = append(resources, &uctx.cfg.StatefulResources[i])
	}

	adminClient := uctx.findFirstAdminClient()
	if adminClient == nil {
		return errors.New("no admin client available to push stateful resources")
	}

	// Build a collection with only stateful resources.
	collection := &config.MockCollection{
		Version:           "1.0",
		Name:              "stateful-resources",
		StatefulResources: resources,
	}

	fmt.Printf("Loading %d stateful resources...\n", len(resources))

	result, err := adminClient.ImportConfig(collection, false)
	if err != nil {
		return fmt.Errorf("importing stateful resources: %w", err)
	}

	if result.StatefulResources > 0 {
		fmt.Printf("Applied %d stateful resources via admin API\n", result.StatefulResources)
	}

	return nil
}

// findFirstAdminClient returns the first available admin HTTP client.
func (uctx *upContext) findFirstAdminClient() AdminClient {
	// Prefer local admins
	for _, aCfg := range uctx.cfg.Admins {
		if aCfg.IsLocal() {
			if client, ok := uctx.adminClients[aCfg.Name]; ok {
				return client
			}
		}
	}
	// Fallback to any admin
	for _, client := range uctx.adminClients {
		return client
	}
	return nil
}
