package cli

import (
	"errors"

	"github.com/spf13/cobra"
)

// mcpCmd is the Cobra command for "mockd mcp".
var mcpCmd = &cobra.Command{
	Use:   "mcp",
	Short: "Start the MCP server in stdio mode for AI assistants",
	Long: `Start the Model Context Protocol (MCP) server in stdio mode.

This is used by AI assistants (Claude Desktop, Cursor, etc.) to
interact with mockd through the MCP protocol over stdin/stdout.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if MCPRunStdioFunc == nil {
			return errors.New("MCP support not available (binary was built without MCP wiring)")
		}
		return MCPRunStdioFunc(args)
	},
}

func init() {
	rootCmd.AddCommand(mcpCmd)
}
