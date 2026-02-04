package cli

import "fmt"

// RunMCP handles the "mockd mcp" command.
// It starts the MCP server in stdio mode for use with AI assistants.
func RunMCP(args []string) error {
	if MCPRunStdioFunc == nil {
		return fmt.Errorf("MCP support not available (binary was built without MCP wiring)")
	}
	return MCPRunStdioFunc(args)
}
