// Package engine provides the core mock server engine for handling HTTP/HTTPS requests.
//
// # Architecture
//
// The mockd architecture has two main components:
//
//	┌─────────────────────────────────────────────────────────────────┐
//	│                        MOCKD ARCHITECTURE                        │
//	├─────────────────────────────────────────────────────────────────┤
//	│                                                                  │
//	│  ┌──────────────────────────────────────────────────────────┐   │
//	│  │                    Admin Server (:4290)                   │   │
//	│  │            (Public HTTP API - User-facing)                │   │
//	│  │                                                           │   │
//	│  │  Package: admin/                                          │   │
//	│  │  Role: CRUD mocks, view logs, configure engine            │   │
//	│  └──────────────────────────────────────────────────────────┘   │
//	│                              │                                   │
//	│                              │ engineclient.Client               │
//	│                              │ (HTTP client for mgmt API)        │
//	│                              ▼                                   │
//	│  ┌──────────────────────────────────────────────────────────┐   │
//	│  │                       Engine                              │   │
//	│  │                                                           │   │
//	│  │   ┌─────────────────────┐  ┌───────────────────────────┐ │   │
//	│  │   │   Mock Server       │  │   Management API          │ │   │
//	│  │   │   (:4280)           │  │   (ManagementPort)        │ │   │
//	│  │   │                     │  │                           │ │   │
//	│  │   │   DATA PLANE        │  │   INTERNAL                │ │   │
//	│  │   │   Serves mocks      │  │   Runtime configuration   │ │   │
//	│  │   └─────────────────────┘  └───────────────────────────┘ │   │
//	│  └──────────────────────────────────────────────────────────┘   │
//	└─────────────────────────────────────────────────────────────────┘
//
// The engine package provides:
//   - Server: The main mock server that handles HTTP and HTTPS requests
//   - Handler: The HTTP handler that matches requests against configured mocks
//   - RequestLogger: Request logging for debugging and inspection
//   - Management API: Internal HTTP API for runtime configuration
//
// # Basic Usage
//
// Mocks are managed via the engine's management API using an HTTP client.
// This ensures all mock operations go through the same interface:
//
//	cfg := &config.ServerConfiguration{
//	    HTTPPort:  4280,
//	    AdminPort: 4290,
//	}
//
//	// Create and start the server
//	srv := engine.NewServer(cfg)
//	srv.Start()
//	defer srv.Stop()
//
//	// Use HTTP client to manage mocks
//	client := engineclient.New(fmt.Sprintf("http://localhost:%d", srv.ManagementPort()))
//	client.CreateMock(ctx, &config.MockConfiguration{
//	    ID:   "test-mock",
//	    Type: mock.MockTypeHTTP,
//	    HTTP: &mock.HTTPSpec{
//	        Matcher:  &mock.HTTPMatcher{Method: "GET", Path: "/api/test"},
//	        Response: &mock.HTTPResponse{StatusCode: 200, Body: "ok"},
//	    },
//	})
//
// For loading mocks from configuration files at startup, use NewServerWithMocks:
//
//	mocks, _ := config.LoadMocksFromFile("mocks.json")
//	srv := engine.NewServerWithMocks(cfg, mocks)
//
// # Features
//
// The server supports:
//   - HTTP and HTTPS (with auto-generated self-signed certificates)
//   - Multi-criteria request matching with scoring
//   - Priority-based mock selection
//   - Request logging for debugging
//   - Dynamic mock management via HTTP API
package engine
