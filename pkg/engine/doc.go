// Package engine provides the core mock server engine for handling HTTP/HTTPS requests.
//
// The engine package is the main entry point for creating and managing a mock server.
// It provides:
//   - Server: The main mock server that handles HTTP and HTTPS requests
//   - Handler: The HTTP handler that matches requests against configured mocks
//   - RequestLogger: Request logging for debugging and inspection
//
// Basic usage:
//
//	cfg := &config.ServerConfiguration{
//	    HTTPPort:  8080,
//	    AdminPort: 9090,
//	}
//
//	srv := engine.NewServer(cfg)
//	srv.AddMock(&config.MockConfiguration{
//	    Matcher:  &config.RequestMatcher{Method: "GET", Path: "/api/test"},
//	    Response: &config.ResponseDefinition{StatusCode: 200, Body: "ok"},
//	})
//
//	srv.Start()
//	defer srv.Stop()
//
// The server supports:
//   - HTTP and HTTPS (with auto-generated self-signed certificates)
//   - Multi-criteria request matching with scoring
//   - Priority-based mock selection
//   - Request logging for debugging
//   - Dynamic mock management via admin API
package engine
