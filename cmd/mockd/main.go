// mockd CLI - Command-line interface for the mockd mock server
package main

import (
	"fmt"
	"os"
	"slices"

	"github.com/getmockd/mockd/pkg/cli"
)

// Build-time variables set via ldflags
var (
	Version   = "dev"
	Commit    = "unknown"
	BuildDate = "unknown"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	// Determine command and args
	command := ""
	var cmdArgs []string

	if len(args) == 0 {
		// No args at all, run serve
		command = "serve"
		cmdArgs = []string{}
	} else {
		first := args[0]
		if first == "" || first[0] == '-' {
			// Flag passed directly (e.g., --help, --version, --port), handle global flags or run serve
			if first == "--help" || first == "-h" {
				printUsage()
				return nil
			}
			if first == "--version" || first == "-v" {
				return cli.RunVersion(cli.BuildInfo{
					Version:   Version,
					Commit:    Commit,
					BuildDate: BuildDate,
				}, nil)
			}
			// Other flags, run serve with them
			command = "serve"
			cmdArgs = args
		} else if isCommand(first) {
			command = first
			cmdArgs = args[1:]
		} else {
			// Unknown argument, try serve
			command = "serve"
			cmdArgs = args
		}
	}

	switch command {
	case "version":
		return cli.RunVersion(cli.BuildInfo{
			Version:   Version,
			Commit:    Commit,
			BuildDate: BuildDate,
		}, cmdArgs)
	case "start":
		return cli.RunStart(cmdArgs)
	case "serve":
		return cli.RunServe(cmdArgs)
	case "stop":
		return cli.RunStop(cmdArgs)
	case "status":
		return cli.RunStatus(cmdArgs)
	case "add":
		return cli.RunAdd(cmdArgs)
	case "new":
		return cli.RunNew(cmdArgs)
	case "init":
		return cli.RunInit(cmdArgs)
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
	case "graphql":
		return cli.RunGraphQL(cmdArgs)
	case "chaos":
		return cli.RunChaos(cmdArgs)
	case "grpc":
		return cli.RunGRPC(cmdArgs)
	case "mqtt":
		return cli.RunMQTT(cmdArgs)
	case "websocket":
		return cli.RunWebSocket(cmdArgs)
	case "soap":
		return cli.RunSOAP(cmdArgs)
	case "convert":
		return cli.RunConvert(cmdArgs)
	case "generate":
		return cli.RunGenerate(cmdArgs)
	case "enhance":
		return cli.RunEnhance(cmdArgs)
	case "help":
		return cli.RunHelp(cmdArgs)
	case "doctor":
		return cli.RunDoctor(cmdArgs)
	case "context":
		return cli.RunContext(cmdArgs)
	case "workspace":
		return cli.RunWorkspace(cmdArgs)
	case "--help", "-h":
		printUsage()
		return nil
	default:
		return fmt.Errorf("unknown command: %s\n\nRun 'mockd --help' for usage", command)
	}
}

func printUsage() {
	fmt.Print(`mockd - Local-first HTTP/HTTPS mock server

Usage:
  mockd                          Start mock server with defaults
  mockd <command> [flags]        Run a specific command

Commands:
  serve       Start the mock server (default command)
  init        Create a starter config file
  start       Start the mock server (alias for serve)
  stop        Stop a running mockd server
  status      Show status of running mockd server
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
  doctor      Diagnose common setup issues

Proxy Commands:
  proxy             Manage the MITM proxy for recording API traffic
  recordings        Manage recorded API traffic
  convert           Convert recordings to mock definitions

AI Commands:
  generate          Generate mocks from OpenAPI or natural language (with --ai)
  enhance           Enhance existing mocks with AI-generated data

Stream Recording Commands:
  stream-recordings Manage WebSocket and SSE stream recordings

GraphQL Commands:
  graphql     Validate schemas and execute GraphQL queries

Chaos Engineering Commands:
  chaos       Manage chaos injection (enable, disable, status)

gRPC Commands:
  grpc        Manage and test gRPC endpoints

MQTT Commands:
  mqtt        Publish, subscribe, and manage MQTT broker

WebSocket Commands:
  websocket   Connect, send, and listen to WebSocket endpoints

SOAP Commands:
  soap        Validate WSDL files and call SOAP operations

Template Commands:
  templates   List and add templates from the official library

Cloud Commands:
  tunnel      Expose local mocks via secure tunnel

Context Commands:
  context     Manage contexts (admin server + workspace pairs)
  workspace   Manage workspaces within the current context

Global Flags:
  -h, --help      Show this help message
  -v, --version   Show version information

Examples:
  # Create a starter config and start the server
  mockd init
  mockd serve --config mockd.yaml

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

Additional Help Topics:
  mockd help config        Configuration file format
  mockd help matching      Request matching patterns
  mockd help templating    Template variable reference
  mockd help formats       Import/export formats
  mockd help websocket     WebSocket mock configuration
  mockd help graphql       GraphQL mock configuration
  mockd help grpc          gRPC mock configuration
  mockd help mqtt          MQTT broker configuration
  mockd help soap          SOAP/WSDL mock configuration
  mockd help sse           Server-Sent Events configuration

Run 'mockd <command> --help' for more information on a command.
`)
}

// isCommand checks if the given string is a known mockd command.
func isCommand(s string) bool {
	commands := []string{
		"start", "serve", "add", "new", "list", "get", "delete",
		"import", "export", "logs", "config", "completion", "version",
		"proxy", "recordings", "convert", "generate", "enhance",
		"stream-recordings", "graphql", "chaos", "grpc", "mqtt", "soap",
		"templates", "tunnel", "init", "help", "status", "stop", "doctor",
		"websocket", "context", "workspace",
	}
	return slices.Contains(commands, s)
}
