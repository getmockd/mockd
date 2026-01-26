package cli

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/getmockd/mockd/pkg/admin"
	"github.com/getmockd/mockd/pkg/admin/engineclient"
	"github.com/getmockd/mockd/pkg/cli/internal/ports"
	"github.com/getmockd/mockd/pkg/config"
	"github.com/getmockd/mockd/pkg/engine"
	"github.com/getmockd/mockd/pkg/logging"
)

// upContext holds runtime state for the up command.
type upContext struct {
	cfg        *config.ProjectConfig
	configPath string
	detach     bool
	log        *slog.Logger
	ctx        context.Context
	cancel     context.CancelFunc

	// Running services
	admins  map[string]*admin.AdminAPI
	engines map[string]*engine.Server
}

// RunUp starts local admins and engines defined in the config file.
func RunUp(args []string) error {
	fs := flag.NewFlagSet("up", flag.ContinueOnError)

	var configFiles stringSliceFlag
	fs.Var(&configFiles, "config", "Config file path (can be specified multiple times)")
	fs.Var(&configFiles, "f", "Config file path (shorthand)")

	detach := fs.Bool("detach", false, "Run in background (daemon mode)")
	fs.BoolVar(detach, "d", false, "Run in background (shorthand)")

	logLevel := fs.String("log-level", "info", "Log level (debug, info, warn, error)")

	fs.Usage = func() {
		fmt.Fprint(os.Stderr, `Usage: mockd up [flags]

Start local admins and engines defined in mockd.yaml.

This command:
  1. Loads and validates the config
  2. Starts local admin servers (those without 'url' field)
  3. Starts local engine servers  
  4. Applies workspaces and mocks to local admins
  5. Runs in foreground (or background with -d)

Flags:
  -f, --config <path>    Config file path (can be specified multiple times)
                         If not specified, discovers mockd.yaml in current directory
  -d, --detach           Run in background (daemon mode)
      --log-level        Log level (debug, info, warn, error)

Environment Variables:
  MOCKD_CONFIG           Default config file path (if -f not specified)

Examples:
  # Start with config in current directory
  mockd up

  # Start with specific config
  mockd up -f ./mockd.yaml

  # Start in background
  mockd up -d

  # Merge multiple configs
  mockd up -f base.yaml -f production.yaml
`)
	}

	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return nil
		}
		return err
	}

	// Load config
	cfg, configPath, err := loadProjectConfig(configFiles)
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
		return fmt.Errorf("invalid configuration")
	}

	// Check port conflicts in config
	portResult := config.ValidatePortConflicts(cfg)
	if !portResult.IsValid() {
		fmt.Fprintln(os.Stderr, "Port conflicts in config:")
		for _, e := range portResult.Errors {
			fmt.Fprintf(os.Stderr, "  - %s\n", e.Error())
		}
		return fmt.Errorf("port conflicts")
	}

	// Check actual port availability
	if err := checkProjectPorts(cfg); err != nil {
		return err
	}

	// Create context
	ctx, cancel := context.WithCancel(context.Background())

	uctx := &upContext{
		cfg:        cfg,
		configPath: configPath,
		detach:     *detach,
		ctx:        ctx,
		cancel:     cancel,
		admins:     make(map[string]*admin.AdminAPI),
		engines:    make(map[string]*engine.Server),
	}

	// Initialize logger
	uctx.log = logging.New(logging.Config{
		Level:  logging.ParseLevel(*logLevel),
		Format: logging.FormatText,
	})

	// Print startup info
	fmt.Printf("Starting mockd from %s\n", configPath)
	printUpSummary(cfg)

	if *detach {
		// TODO: Implement proper daemonization
		fmt.Println("Detached mode not yet fully implemented. Running in foreground...")
	}

	return uctx.run()
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

	// Write PID file
	pidPath := defaultUpPIDPath()
	if err := uctx.writePIDFile(pidPath); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to write PID file: %v\n", err)
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

	// Start engines and connect them to their admins
	for _, engineCfg := range uctx.cfg.Engines {
		if err := uctx.startEngine(engineCfg); err != nil {
			return fmt.Errorf("starting engine '%s': %w", engineCfg.Name, err)
		}

		// Connect the engine to its admin
		if err := uctx.connectEngineToAdmin(engineCfg); err != nil {
			return fmt.Errorf("connecting engine '%s' to admin: %w", engineCfg.Name, err)
		}
	}

	return nil
}

func (uctx *upContext) startAdmin(adminCfg config.AdminConfig) error {
	fmt.Printf("Starting admin '%s' on port %d... ", adminCfg.Name, adminCfg.Port)

	// Build admin options
	opts := []admin.Option{}

	// Disable auth if configured
	if adminCfg.Auth != nil && adminCfg.Auth.Type == "none" {
		opts = append(opts, admin.WithAPIKeyDisabled())
	}

	// Create admin API
	adminAPI := admin.NewAdminAPI(adminCfg.Port, opts...)
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
	// Find the admin this engine should connect to
	adminAPI, ok := uctx.admins[engineCfg.Admin]
	if !ok {
		// Admin might be remote, skip connection
		return nil
	}

	// Find the engine
	srv, ok := uctx.engines[engineCfg.Name]
	if !ok {
		return fmt.Errorf("engine '%s' not found", engineCfg.Name)
	}

	// Connect admin to engine via the engine's management port
	engineURL := fmt.Sprintf("http://localhost:%d", srv.ManagementPort())
	engClient := engineclient.New(engineURL)

	// Set the engine client on the admin
	adminAPI.SetLocalEngine(engClient)

	fmt.Printf("Connected engine '%s' to admin '%s'\n", engineCfg.Name, engineCfg.Admin)
	return nil
}

func (uctx *upContext) stopAll() {
	// Stop engines first
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

	return fmt.Errorf("timeout waiting for engine health")
}

// loadAndApplyMocks loads mocks from the config (including file references and globs)
// and applies them to the appropriate engines via the admin API.
func (uctx *upContext) loadAndApplyMocks() error {
	if len(uctx.cfg.Mocks) == 0 {
		return nil
	}

	// Get base directory for resolving relative paths in mock file references
	baseDir := config.GetMockFileBaseDir(uctx.configPath)

	// Load all mocks (expanding file refs and globs)
	mocks, err := config.LoadAllMocks(uctx.cfg.Mocks, baseDir)
	if err != nil {
		return err
	}

	if len(mocks) == 0 {
		return nil
	}

	fmt.Printf("Loading %d mocks...\n", len(mocks))

	// TODO: Apply mocks to engines via admin API
	// For now, just log the loaded mocks
	for _, m := range mocks {
		if m.IsInline() {
			uctx.log.Debug("loaded mock", "id", m.ID, "type", m.Type, "workspace", m.Workspace)
		}
	}

	fmt.Printf("Loaded %d mocks\n", len(mocks))
	return nil
}
