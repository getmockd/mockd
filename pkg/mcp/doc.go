// Package mcp implements the Model Context Protocol (MCP) server for mockd.
//
// MCP enables AI agents to discover, query, and manage mock API endpoints
// across all supported protocols (HTTP, WebSocket, GraphQL, gRPC, SOAP, MQTT, OAuth)
// through a standardized JSON-RPC 2.0 based protocol.
//
// # Protocol Version
//
// This implementation follows MCP protocol version 2025-06-18 with Streamable HTTP
// and stdio transports.
//
// # Tools (19 total)
//
// Mock CRUD:
//   - list_mocks, get_mock, create_mock, update_mock, delete_mock, toggle_mock
//
// Context / Workspace:
//   - get_current_context, switch_context, list_workspaces, switch_workspace
//
// Import / Export:
//   - import_mocks, export_mocks
//
// Observability:
//   - get_server_status, get_request_logs, clear_request_logs
//
// Stateful Resources:
//   - list_stateful_items, get_stateful_item, create_stateful_item, reset_stateful_data
//
// # Resources
//
// Resources use the mock:// URI scheme:
//   - mock:///path#METHOD - Individual mock endpoints
//   - mock://stateful/name - Stateful resources
//   - mock://logs - Request log summary
//   - mock://config - Server configuration
//   - mock://context - Current context info
//
// # Transports
//
// Stdio (primary): mockd mcp — newline-delimited JSON-RPC over stdin/stdout.
// HTTP (secondary): mockd serve --mcp — Streamable HTTP on :9091/mcp.
//
// # Security
//
// By default, the HTTP transport only accepts connections from localhost.
// Auth tokens from contexts.yaml are never exposed in tool responses.
package mcp
