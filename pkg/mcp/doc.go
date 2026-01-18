// Package mcp implements the Model Context Protocol (MCP) server for mockd.
//
// MCP (Model Context Protocol) enables AI agents to discover, query, and manage
// mock API endpoints through a standardized JSON-RPC 2.0 based protocol.
// This allows LLM applications like Claude Desktop to programmatically interact
// with mockd's mocking capabilities.
//
// # Protocol Version
//
// This implementation follows MCP protocol version 2025-06-18 with Streamable HTTP transport.
//
// # Features
//
// The MCP server provides:
//   - Tool discovery and execution (tools/list, tools/call)
//   - Resource discovery and reading (resources/list, resources/read)
//   - SSE streaming for server-initiated notifications
//   - Session management with Mcp-Session-Id headers
//
// # Tools
//
// Available tools include:
//   - get_mock_data: Retrieve mock response for API endpoint
//   - list_endpoints: List all configured mock endpoints
//   - create_endpoint: Create a new mock endpoint
//   - update_endpoint: Update existing mock configuration
//   - delete_endpoint: Remove a mock endpoint
//   - toggle_endpoint: Enable/disable a mock endpoint
//   - stateful_list: List items in a stateful resource
//   - stateful_get: Get a specific stateful item
//   - stateful_create: Create a new stateful item
//   - stateful_reset: Reset stateful resource to seed data
//   - get_request_logs: Retrieve captured request logs
//   - clear_logs: Clear all request logs
//
// # Resources
//
// Resources use the mock:// URI scheme:
//   - mock:///path - Static mock endpoints
//   - mock://stateful/name - Stateful resources
//   - mock://logs - Request logs
//   - mock://config - Server configuration
//
// # Usage
//
// Create and start an MCP server:
//
//	cfg := &mcp.Config{
//	    Enabled: true,
//	    Port:    9091,
//	    Path:    "/mcp",
//	}
//	adminClient := cli.NewAdminClient("http://localhost:4290")
//	server := mcp.NewServer(cfg, adminClient, statefulStore)
//	server.Start()
//
// # Security
//
// By default, the MCP server only accepts connections from localhost.
// Set AllowRemote to true to accept remote connections (use with caution).
// Origin header validation is performed to protect against DNS rebinding attacks.
package mcp
