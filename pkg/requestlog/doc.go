// Package requestlog provides types and interfaces for capturing and storing
// request/response data for user inspection and debugging.
//
// This package serves mockd users who need to inspect what requests came in,
// which mocks matched, and what responses were sent. It is distinct from
// operational logging (which uses log/slog for platform debugging).
//
// # Core Types
//
// Entry is the central type representing a captured request/response pair.
// It supports multiple protocols (HTTP, gRPC, WebSocket, SSE, MQTT, SOAP, GraphQL)
// with protocol-specific metadata.
//
// # Store Interface
//
// Store defines the interface for request history storage, supporting:
//   - Recording new entries
//   - Querying by ID or with filters
//   - Subscribing to new entries in real-time
//   - Clearing history
//
// # Usage
//
// Protocol handlers create Entry instances and pass them to a Store implementation.
// The TUI and Admin API query the Store to display request history to users.
//
//	store := requestlog.NewMemoryStore(1000)
//	entry := &requestlog.Entry{
//	    Protocol: requestlog.ProtocolHTTP,
//	    Method:   "GET",
//	    Path:     "/api/users",
//	    // ...
//	}
//	store.Log(entry)
//
// # Package Design
//
// This is a leaf package with no internal dependencies, allowing it to be
// imported by any package without creating import cycles.
package requestlog
