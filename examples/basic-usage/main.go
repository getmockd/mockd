// Basic usage example demonstrating how to create a mock server programmatically.
//
// This example shows the recommended pattern:
// 1. Create and start the engine server
// 2. Use the HTTP-based engine client to add mocks
// 3. Start the admin API for runtime management
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/getmockd/mockd/pkg/admin"
	"github.com/getmockd/mockd/pkg/admin/engineclient"
	"github.com/getmockd/mockd/pkg/config"
	"github.com/getmockd/mockd/pkg/engine"
	"github.com/getmockd/mockd/pkg/mock"
)

func main() {
	// Create server configuration
	cfg := &config.ServerConfiguration{
		HTTPPort:      4280,
		AdminPort:     4290,
		ReadTimeout:   30,
		WriteTimeout:  30,
		MaxLogEntries: 1000,
	}

	// Create and start the server
	srv := engine.NewServer(cfg)
	if err := srv.Start(); err != nil {
		log.Fatalf("Failed to start mock server: %v", err)
	}

	// Create an HTTP client to communicate with the engine's management API
	engineURL := fmt.Sprintf("http://localhost:%d", srv.ManagementPort())
	client := engineclient.New(engineURL, engineclient.WithTimeout(10*time.Second))

	// Wait for engine to be ready
	ctx := context.Background()
	if err := waitForEngine(ctx, client); err != nil {
		log.Fatalf("Engine failed to become ready: %v", err)
	}

	// Add mocks via the HTTP client
	enabled := true
	mocks := []*config.MockConfiguration{
		{
			ID:      "get-users",
			Name:    "Get Users List",
			Type:    mock.MockTypeHTTP,
			Enabled: &enabled,
			HTTP: &mock.HTTPSpec{
				Priority: 0,
				Matcher: &mock.HTTPMatcher{
					Method: "GET",
					Path:   "/api/users",
				},
				Response: &mock.HTTPResponse{
					StatusCode: 200,
					Headers: map[string]string{
						"Content-Type": "application/json",
					},
					Body: `{"users": [{"id": 1, "name": "Alice"}, {"id": 2, "name": "Bob"}]}`,
				},
			},
		},
		{
			ID:      "create-user",
			Name:    "Create User",
			Type:    mock.MockTypeHTTP,
			Enabled: &enabled,
			HTTP: &mock.HTTPSpec{
				Priority: 0,
				Matcher: &mock.HTTPMatcher{
					Method: "POST",
					Path:   "/api/users",
				},
				Response: &mock.HTTPResponse{
					StatusCode: 201,
					Headers: map[string]string{
						"Content-Type": "application/json",
						"Location":     "/api/users/new-id",
					},
					Body: `{"id": "new-id", "created": true}`,
				},
			},
		},
		{
			ID:      "get-user-by-id",
			Name:    "Get User by ID",
			Type:    mock.MockTypeHTTP,
			Enabled: &enabled,
			HTTP: &mock.HTTPSpec{
				Priority: 0,
				Matcher: &mock.HTTPMatcher{
					Method: "GET",
					Path:   "/api/users/*",
				},
				Response: &mock.HTTPResponse{
					StatusCode: 200,
					Headers: map[string]string{
						"Content-Type": "application/json",
					},
					Body: `{"id": "123", "name": "Alice", "email": "alice@example.com"}`,
				},
			},
		},
	}

	// Create each mock via the HTTP API
	for _, m := range mocks {
		if _, err := client.CreateMock(ctx, m); err != nil {
			log.Fatalf("Failed to create mock %s: %v", m.ID, err)
		}
		log.Printf("Created mock: %s", m.Name)
	}

	// Start the admin API with engine client
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
	adminAPI.Stop()
	srv.Stop()
	log.Println("Goodbye!")
}

// waitForEngine polls the engine health endpoint until it's ready.
func waitForEngine(ctx context.Context, client *engineclient.Client) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			if err := client.Health(ctx); err == nil {
				return nil
			}
			time.Sleep(50 * time.Millisecond)
		}
	}
}
