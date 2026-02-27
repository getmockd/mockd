// Package cli provides the mockd CLI commands.
package cli

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/getmockd/mockd/pkg/config"
	"github.com/getmockd/mockd/pkg/engine"
	"github.com/getmockd/mockd/pkg/logging"

	"github.com/spf13/cobra"
)

// engineFlags holds all flags for the engine command.
type engineFlags struct {
	configPath   string
	port         int
	host         string
	printURL     bool
	logLevel     string
	logFormat    string
	readTimeout  int
	writeTimeout int
}

// engineFlagVals is the package-level instance bound to cobra flags.
var engineFlagVals engineFlags

var engineCmd = &cobra.Command{
	Use:   "engine",
	Short: "Run a headless mock engine (no admin API, for CI/CD)",
	Long: `Run a headless mock engine that serves mocks from a config file.

Unlike 'mockd serve' or 'mockd start', the engine command:
  - Does NOT start an admin API
  - Does NOT persist data to disk
  - Does NOT create PID files or support daemon mode
  - Loads all mocks from a single config file
  - Runs in the foreground until SIGTERM/SIGINT

This is ideal for CI/CD pipelines, Docker containers, and environments
where you need a lightweight, stateless mock server.`,
	Example: `  # Start with a config file
  mockd engine --config mocks.yaml

  # Auto-assign a port and print it
  mockd engine --config mocks.yaml --port 0 --print-url

  # JSON logs for CI parsing
  mockd engine --config mocks.yaml --log-format json

  # Custom timeouts
  mockd engine --config mocks.yaml --read-timeout 60 --write-timeout 60`,
	RunE: runEngine,
}

func init() {
	f := &engineFlagVals

	engineCmd.Flags().StringVarP(&f.configPath, "config", "c", "", "Path to mock config file (YAML or JSON) [required]")
	engineCmd.Flags().IntVarP(&f.port, "port", "p", 4280, "HTTP server port (0 = OS auto-assign)")
	engineCmd.Flags().StringVar(&f.host, "host", "0.0.0.0", "Bind address")
	engineCmd.Flags().BoolVar(&f.printURL, "print-url", false, "Print the server URL to stdout on startup")
	engineCmd.Flags().StringVar(&f.logLevel, "log-level", "warn", "Log level (debug, info, warn, error)")
	engineCmd.Flags().StringVar(&f.logFormat, "log-format", "text", "Log format (text, json)")
	engineCmd.Flags().IntVar(&f.readTimeout, "read-timeout", 30, "HTTP read timeout in seconds")
	engineCmd.Flags().IntVar(&f.writeTimeout, "write-timeout", 30, "HTTP write timeout in seconds")

	_ = engineCmd.MarkFlagRequired("config")

	rootCmd.AddCommand(engineCmd)
}

func runEngine(_ *cobra.Command, _ []string) error {
	f := &engineFlagVals

	// Validate config file exists
	if _, err := os.Stat(f.configPath); os.IsNotExist(err) {
		return fmt.Errorf("config file not found: %s", f.configPath)
	}

	// Load the mock collection from file
	collection, err := config.LoadFromFile(f.configPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Create logger
	log := logging.New(logging.Config{
		Level:  logging.ParseLevel(f.logLevel),
		Format: logging.ParseFormat(f.logFormat),
	})

	// Build server configuration — minimal, no admin, no management port
	serverCfg := &config.ServerConfiguration{
		HTTPPort:      f.port,
		HTTPAutoPort:  f.port == 0, // Enable auto-port only when explicitly 0
		ReadTimeout:   f.readTimeout,
		WriteTimeout:  f.writeTimeout,
		LogRequests:   true,
		MaxLogEntries: 1000,
	}

	// Create and configure the engine server
	srv := engine.NewServer(serverCfg)
	srv.SetLogger(log.With("component", "engine"))

	// Set base directory for resolving relative bodyFile paths
	srv.Handler().SetBaseDir(config.GetMockFileBaseDir(f.configPath))

	// Start the engine
	if err := srv.Start(); err != nil {
		if isAddrInUseError(err) {
			return fmt.Errorf("port %d is already in use — try --port 0 for auto-assign", f.port)
		}
		return fmt.Errorf("failed to start engine: %w", err)
	}
	defer func() { _ = srv.Stop() }()

	// Load mocks into the engine
	if err := srv.ImportConfig(collection, true); err != nil {
		return fmt.Errorf("failed to import mocks: %w", err)
	}

	// Resolve actual port (handles port-0 auto-assignment)
	actualPort := srv.HTTPPort()
	mockCount := len(collection.Mocks)
	configHash := computeConfigHash(collection)

	// Print URL if requested (to stdout for programmatic consumption)
	if f.printURL {
		fmt.Printf("http://%s:%d\n", f.host, actualPort)
	}

	// Print startup summary
	log.Info("engine started",
		"port", actualPort,
		"mocks", mockCount,
		"config", f.configPath,
		"configHash", configHash,
	)

	// Wait for shutdown signal
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	<-ctx.Done()

	log.Info("shutting down engine")

	return nil
}

// computeConfigHash returns a sha256 hash prefix of the serialized config.
func computeConfigHash(collection *config.MockCollection) string {
	data, err := json.Marshal(collection)
	if err != nil {
		return "unknown"
	}
	h := sha256.Sum256(data)
	return fmt.Sprintf("sha256:%x", h[:8])
}
