package main

import (
	"flag"
	"fmt"
	"log/slog"
	"os"

	"github.com/getmockd/mockd/pkg/cli"
	"github.com/getmockd/mockd/pkg/cliconfig"
	"github.com/getmockd/mockd/pkg/mcp"
	"github.com/getmockd/mockd/pkg/stateful"
)

func init() {
	// Wire MCP factories into pkg/cli to break circular imports.
	cli.MCPRunStdioFunc = runMCPStdio
	cli.MCPStartFunc = startMCPHTTP
}

// runMCPStdio runs the MCP server in stdio mode.
// This is what Claude Desktop, Cursor, etc. spawn when configured with:
//
//	{ "command": "mockd", "args": ["mcp"] }
func runMCPStdio(args []string) error {
	fs := flag.NewFlagSet("mcp", flag.ContinueOnError)
	var adminURL string
	var logLevel string
	fs.StringVar(&adminURL, "admin-url", "", "Admin API URL (default: auto-detect from context/config)")
	fs.StringVar(&logLevel, "log-level", "warn", "Log level for stderr (debug, info, warn, error)")
	fs.Usage = func() {
		fmt.Fprint(os.Stderr, `Usage: mockd mcp [flags]

Start the MCP (Model Context Protocol) server in stdio mode.
Reads JSON-RPC from stdin, writes responses to stdout.

This is the primary way to connect AI assistants like Claude Desktop
to a running mockd server.

Prerequisites:
  A mockd server must be running (mockd serve or mockd start).

Claude Desktop config (~/.config/claude/claude_desktop_config.json):

  {
    "mcpServers": {
      "mockd": {
        "command": "mockd",
        "args": ["mcp"]
      }
    }
  }

Flags:
      --admin-url   Admin API URL (default: auto-detect from context/config)
      --log-level   Log level for stderr (debug, info, warn, error) (default: warn)
`)
	}

	if err := fs.Parse(args); err != nil {
		return err
	}

	// Resolve admin URL from context/config/default if not specified.
	if adminURL == "" {
		cfg := cliconfig.ResolveClientConfigSimple("")
		adminURL = cfg.AdminURL
	}
	if adminURL == "" {
		adminURL = cliconfig.DefaultAdminURL(cliconfig.DefaultAdminPort)
	}

	// Create admin client (talks to the running mockd admin API).
	adminClient := cli.NewAdminClientWithAuth(adminURL)

	// Create MCP config — stdio doesn't use HTTP, but the server needs a config.
	mcpCfg := mcp.DefaultConfig()
	mcpCfg.Enabled = true
	mcpCfg.AdminURL = adminURL

	// Create MCP server (provides dispatch/tools/resources, not HTTP).
	server := mcp.NewServer(mcpCfg, adminClient, nil) // nil stateful store — tools that need it will return an error

	// Logger writes to stderr so stdout stays clean for the protocol.
	level := slog.LevelWarn
	switch logLevel {
	case "debug":
		level = slog.LevelDebug
	case "info":
		level = slog.LevelInfo
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	}
	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level}))
	server.SetLogger(log)

	// Run the stdio transport — blocks until EOF.
	stdio := mcp.NewStdioServer(server)
	stdio.SetLogger(log)
	return stdio.Run()
}

// startMCPHTTP creates and starts the MCP HTTP server for embedding in mockd serve.
func startMCPHTTP(adminURL string, port int, allowRemote bool, storeIface interface{}, logIface interface{}) (cli.MCPStopper, error) {
	mcpCfg := mcp.DefaultConfig()
	mcpCfg.Enabled = true
	mcpCfg.Port = port
	mcpCfg.AllowRemote = allowRemote
	mcpCfg.AdminURL = adminURL

	adminClient := cli.NewAdminClient(adminURL)

	// Type-assert the stateful store (passed as interface{} to avoid circular imports).
	var store *stateful.StateStore
	if s, ok := storeIface.(*stateful.StateStore); ok {
		store = s
	}

	server := mcp.NewServer(mcpCfg, adminClient, store)

	// Type-assert the logger.
	if log, ok := logIface.(*slog.Logger); ok && log != nil {
		server.SetLogger(log)
	}

	if err := server.Start(); err != nil {
		return nil, err
	}
	return server, nil
}
