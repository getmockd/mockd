// Example demonstrating how to load mocks from a JSON configuration file.
package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/getmockd/mockd/pkg/admin"
	"github.com/getmockd/mockd/pkg/config"
	"github.com/getmockd/mockd/pkg/engine"
)

func main() {
	// Get config file path from args or use default
	configPath := "mocks.json"
	if len(os.Args) > 1 {
		configPath = os.Args[1]
	}

	// Resolve absolute path
	absPath, err := filepath.Abs(configPath)
	if err != nil {
		log.Fatalf("Invalid config path: %v", err)
	}

	// Load mocks from file
	log.Printf("Loading mocks from: %s", absPath)
	mocks, err := config.LoadMocksFromFile(absPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}
	log.Printf("Loaded %d mocks", len(mocks))

	// Create server configuration
	cfg := &config.ServerConfiguration{
		HTTPPort:      4280,
		AdminPort:     4290,
		ReadTimeout:   30,
		WriteTimeout:  30,
		MaxLogEntries: 1000,
	}

	// Create server with pre-loaded mocks
	srv := engine.NewServerWithMocks(cfg, mocks)

	// Start the mock server
	if err := srv.Start(); err != nil {
		log.Fatalf("Failed to start mock server: %v", err)
	}

	// Start the admin API
	engineURL := fmt.Sprintf("http://localhost:%d", srv.ManagementPort())
	adminAPI := admin.NewAdminAPI(cfg.AdminPort, admin.WithLocalEngine(engineURL))
	if err := adminAPI.Start(); err != nil {
		log.Fatalf("Failed to start admin API: %v", err)
	}

	log.Printf("Mock server running on http://localhost:%d", cfg.HTTPPort)
	log.Printf("Admin API running on http://localhost:%d", cfg.AdminPort)
	log.Println("Press Ctrl+C to stop")

	// Wait for interrupt signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	log.Println("Shutting down...")

	// Optionally save config on exit
	if len(os.Args) > 2 && os.Args[2] == "--save-on-exit" {
		log.Printf("Saving config to: %s", absPath)
		if err := srv.SaveConfig(absPath, "saved-config"); err != nil {
			log.Printf("Warning: Failed to save config: %v", err)
		}
	}

	adminAPI.Stop()
	srv.Stop()
	log.Println("Goodbye!")
}
