package cli

// MCPStopper is implemented by the MCP server to support graceful shutdown.
// Defined here to avoid circular imports between pkg/cli and pkg/mcp.
type MCPStopper interface {
	Stop() error
}

// MCPStartFunc creates and starts an MCP HTTP server.
// Set by cmd/mockd/mcp.go init() to break the circular dependency.
//
// Parameters:
//   - adminURL: URL of the running admin API (e.g., "http://localhost:4290")
//   - port: TCP port for the MCP HTTP server
//   - allowRemote: whether to accept non-localhost connections
//   - statefulStore: the engine's *stateful.StateStore (passed as interface{} to avoid import)
//   - logger: *slog.Logger for the MCP server
//
// Returns a stopper to gracefully shut down the MCP server.
var MCPStartFunc func(adminURL string, port int, allowRemote bool, statefulStore interface{}, logger interface{}) (MCPStopper, error)

// MCPRunStdioFunc runs the MCP server in stdio mode (for Claude Desktop, Cursor, etc.).
// Set by cmd/mockd/mcp.go init() to break the circular dependency.
var MCPRunStdioFunc func(args []string) error
