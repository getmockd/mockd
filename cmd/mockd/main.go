// mockd CLI - Command-line interface for the mockd mock server
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/getmockd/mockd/pkg/admin"
	"github.com/getmockd/mockd/pkg/cli"
	"github.com/getmockd/mockd/pkg/config"
	"github.com/getmockd/mockd/pkg/engine"
	"github.com/getmockd/mockd/pkg/tui"
)

// Version is set at build time via ldflags
var Version = "dev"

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	// Check for TUI flag first (opt-in for Phase 1)
	if len(args) > 0 && args[0] == "--tui" {
		// Parse TUI-specific flags
		tuiArgs := args[1:]
		serve := false
		adminURL := "http://localhost:9090"

		// Parse flags
		for i := 0; i < len(tuiArgs); i++ {
			switch tuiArgs[i] {
			case "--serve":
				serve = true
			case "--admin":
				if i+1 < len(tuiArgs) {
					adminURL = tuiArgs[i+1]
					i++ // Skip next arg
				} else {
					return fmt.Errorf("--admin requires a URL argument")
				}
			case "--help", "-h":
				printTUIUsage()
				return nil
			}
		}

		// Handle different modes
		if serve {
			// Hybrid mode: start embedded server + TUI
			return runHybridMode(adminURL)
		} else {
			// Remote mode: connect to existing server
			return tui.RunWithAdminURL(adminURL)
		}
	}

	// Check for CI/no-TUI flags
	if len(args) > 0 && (args[0] == "--ci" || args[0] == "--no-tui") {
		// Remove the flag and continue with normal CLI
		args = args[1:]
	}

	if len(args) == 0 {
		printUsage()
		return nil
	}

	// Check for global flags first
	if args[0] == "--help" || args[0] == "-h" {
		printUsage()
		return nil
	}
	if args[0] == "--version" || args[0] == "-v" {
		fmt.Printf("mockd version %s\n", Version)
		return nil
	}

	// Route to subcommand
	command := args[0]
	cmdArgs := args[1:]

	switch command {
	case "version":
		return cli.RunVersion(Version, cmdArgs)
	case "start":
		return cli.RunStart(cmdArgs)
	case "serve":
		return cli.RunServe(cmdArgs)
	case "add":
		return cli.RunAdd(cmdArgs)
	case "new":
		return cli.RunNew(cmdArgs)
	case "list":
		return cli.RunList(cmdArgs)
	case "get":
		return cli.RunGet(cmdArgs)
	case "delete":
		return cli.RunDelete(cmdArgs)
	case "import":
		return cli.RunImport(cmdArgs)
	case "export":
		return cli.RunExport(cmdArgs)
	case "logs":
		return cli.RunLogs(cmdArgs)
	case "config":
		return cli.RunConfig(cmdArgs)
	case "completion":
		return cli.RunCompletion(cmdArgs)
	case "proxy":
		return cli.RunProxy(cmdArgs)
	case "recordings":
		return cli.RunRecordings(cmdArgs)
	case "tunnel":
		return cli.RunTunnel(cmdArgs)
	case "templates":
		return cli.RunTemplates(cmdArgs)
	case "stream-recordings":
		return cli.RunStreamRecordings(cmdArgs)
	case "help", "--help", "-h":
		printUsage()
		return nil
	default:
		return fmt.Errorf("unknown command: %s\n\nRun 'mockd --help' for usage", command)
	}
}

func printUsage() {
	fmt.Print(`mockd - Local-first HTTP/HTTPS mock server

Usage:
  mockd <command> [flags]

Commands:
  start       Start the mock server
  serve       Start with runtime/pull mode support
  add         Add a new mock endpoint
  new         Create mocks from templates (crud, auth, etc.)
  list        List all configured mocks
  get         Get details of a specific mock
  delete      Delete a mock by ID
  import      Import mocks from a configuration file
  export      Export current mocks to stdout or file
  logs        View request logs
  config      Show effective configuration
  completion  Generate shell completion scripts
  version     Show version information

Proxy Commands:
  proxy             Manage the MITM proxy for recording API traffic
  recordings        Manage recorded API traffic

Stream Recording Commands:
  stream-recordings Manage WebSocket and SSE stream recordings

Template Commands:
  templates   List and add templates from the official library

Cloud Commands:
  tunnel      Expose local mocks via secure tunnel

Global Flags:
  -h, --help      Show this help message
  -v, --version   Show version information
  --tui           Launch interactive TUI
  --ci            Run in headless/CI mode (disable TUI)
  --no-tui        Alias for --ci

TUI Flags (when using --tui):
  --serve         Start embedded mock server with TUI (hybrid mode)
  --admin <url>   Admin API URL (default: http://localhost:9090)

Examples:
  # Start the server with defaults
  mockd start

  # Start with custom port and config file
  mockd start --port 3000 --config mocks.json

  # Launch TUI connected to local server
  mockd --tui

  # Launch TUI with embedded server (hybrid mode)
  mockd --tui --serve

  # Connect TUI to remote server
  mockd --tui --admin http://remote:9090

  # Create CRUD mocks from template
  mockd new -t crud --resource users -o users.yaml

  # Expose local mocks via tunnel
  mockd tunnel --token YOUR_TOKEN

  # List available templates
  mockd templates list

  # Add a template
  mockd templates add services/openai/chat-completions -o openai.yaml

  # Add a mock endpoint
  mockd add --path /api/users --status 200 --body '{"users": []}'

  # Start proxy in record mode
  mockd proxy start --mode record

  # List recorded traffic
  mockd recordings list

  # Convert recordings to mocks
  mockd recordings convert -o mocks.json

Run 'mockd <command> --help' for more information on a command.
`)
}

func printTUIUsage() {
	fmt.Print(`mockd TUI - Interactive Terminal UI for mockd

Usage:
  mockd --tui [flags]

Modes:
  Remote (default):  Connect to existing mockd server
  Hybrid:            Start embedded server + TUI

Flags:
  --serve          Start embedded mock server with TUI (hybrid mode)
  --admin <url>    Admin API URL (default: http://localhost:9090)
  -h, --help       Show this help message

Examples:
  # Connect to local server (default)
  mockd --tui

  # Start embedded server + TUI (hybrid mode)
  mockd --tui --serve

  # Connect to remote server
  mockd --tui --admin http://remote:9090

  # Hybrid mode with custom admin URL
  mockd --tui --serve --admin http://localhost:9999

Navigation:
  1-7    Switch views (Dashboard, Mocks, Proxy, Streams, Traffic, Connections, Logs)
  ?      Toggle help
  q      Quit

`)
}

func runHybridMode(adminURL string) error {
	// Parse admin URL to get port
	// For simplicity, use default ports
	cfg := config.DefaultServerConfiguration()
	cfg.HTTPPort = 8080
	cfg.AdminPort = 9090

	// Create and start server
	srv := engine.NewServer(cfg)
	if err := srv.Start(); err != nil {
		return fmt.Errorf("failed to start server: %w", err)
	}

	// Start admin API
	adminSrv := admin.NewAdminAPI(srv, cfg.AdminPort)
	if err := adminSrv.Start(); err != nil {
		return fmt.Errorf("failed to start admin API: %w", err)
	}

	// Give servers a moment to start
	time.Sleep(500 * time.Millisecond)

	fmt.Printf("Mock server started on port %d\n", cfg.HTTPPort)
	fmt.Printf("Admin API started on port %d\n", cfg.AdminPort)
	fmt.Println("Starting TUI...")
	time.Sleep(500 * time.Millisecond)

	// Setup cleanup on exit
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle signals for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-sigChan
		cancel()
	}()

	// Run TUI in foreground
	tuiErr := tui.RunWithAdminURL(adminURL)

	// TUI exited, shutdown servers
	fmt.Println("\nShutting down servers...")

	if err := adminSrv.Stop(); err != nil {
		fmt.Fprintf(os.Stderr, "Error shutting down admin server: %v\n", err)
	}

	if err := srv.Stop(); err != nil {
		fmt.Fprintf(os.Stderr, "Error shutting down mock server: %v\n", err)
	}

	// Use ctx to avoid unused warning
	_ = ctx

	return tuiErr
}
