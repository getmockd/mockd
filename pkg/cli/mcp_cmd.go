package cli

import (
	"errors"
	"os"

	"github.com/spf13/cobra"
)

// mcpCmd is the Cobra command for "mockd mcp".
// Flags are parsed by a flag.NewFlagSet in the wiring layer (cmd/mockd/mcp.go)
// rather than by cobra, so we use DisableFlagParsing to pass them through.
var mcpCmd = &cobra.Command{
	Use:   "mcp",
	Short: "Start the MCP server in stdio mode for AI assistants",
	Long: `Start the Model Context Protocol (MCP) server in stdio mode.

This is used by AI assistants (Claude Desktop, Cursor, etc.) to
interact with mockd through the MCP protocol over stdin/stdout.

By default, connects to a running mockd server. If no server is running,
auto-starts a background daemon so the MCP session works immediately.

Use --standalone for a dedicated embedded server tied to this session.`,
	DisableFlagParsing: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Handle --help / -h manually since cobra flag parsing is disabled.
		for _, a := range args {
			if a == "--help" || a == "-h" {
				cmd.Help()
				os.Exit(0)
			}
		}
		if MCPRunStdioFunc == nil {
			return errors.New("MCP support not available (binary was built without MCP wiring)")
		}
		return MCPRunStdioFunc(args)
	},
}

func init() {
	rootCmd.AddCommand(mcpCmd)
}
