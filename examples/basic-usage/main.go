// Basic usage example demonstrating how to create a mock server programmatically.
package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/getmockd/mockd/pkg/admin"
	"github.com/getmockd/mockd/pkg/config"
	"github.com/getmockd/mockd/pkg/engine"
)

func main() {
	// Create server configuration
	cfg := &config.ServerConfiguration{
		HTTPPort:      8080,
		AdminPort:     9090,
		ReadTimeout:   30,
		WriteTimeout:  30,
		MaxLogEntries: 1000,
	}

	// Create the server
	srv := engine.NewServer(cfg)

	// Add some mocks
	srv.AddMock(&config.MockConfiguration{
		ID:       "get-users",
		Name:     "Get Users List",
		Priority: 0,
		Enabled:  true,
		Matcher: &config.RequestMatcher{
			Method: "GET",
			Path:   "/api/users",
		},
		Response: &config.ResponseDefinition{
			StatusCode: 200,
			Headers: map[string]string{
				"Content-Type": "application/json",
			},
			Body: `{"users": [{"id": 1, "name": "Alice"}, {"id": 2, "name": "Bob"}]}`,
		},
	})

	srv.AddMock(&config.MockConfiguration{
		ID:       "create-user",
		Name:     "Create User",
		Priority: 0,
		Enabled:  true,
		Matcher: &config.RequestMatcher{
			Method: "POST",
			Path:   "/api/users",
		},
		Response: &config.ResponseDefinition{
			StatusCode: 201,
			Headers: map[string]string{
				"Content-Type": "application/json",
				"Location":     "/api/users/new-id",
			},
			Body: `{"id": "new-id", "created": true}`,
		},
	})

	srv.AddMock(&config.MockConfiguration{
		ID:       "get-user-by-id",
		Name:     "Get User by ID",
		Priority: 0,
		Enabled:  true,
		Matcher: &config.RequestMatcher{
			Method: "GET",
			Path:   "/api/users/*",
		},
		Response: &config.ResponseDefinition{
			StatusCode: 200,
			Headers: map[string]string{
				"Content-Type": "application/json",
			},
			Body: `{"id": "123", "name": "Alice", "email": "alice@example.com"}`,
		},
	})

	// Start the mock server
	if err := srv.Start(); err != nil {
		log.Fatalf("Failed to start mock server: %v", err)
	}

	// Start the admin API
	adminAPI := admin.NewAdminAPI(srv, cfg.AdminPort)
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
