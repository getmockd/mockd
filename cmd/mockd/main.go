// mockd CLI - Command-line interface for the mockd mock server
package main

import (
	"fmt"
	"os"

	"github.com/getmockd/mockd/pkg/cli"
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

Examples:
  # Start the server with defaults
  mockd start

  # Start with custom port and config file
  mockd start --port 3000 --config mocks.json

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
