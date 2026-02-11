// mockd CLI - Command-line interface for the mockd mock server
package main

import (
	"fmt"
	"os"

	"github.com/getmockd/mockd/pkg/cli"
)

// Build-time variables set via ldflags
var (
	Version   = "dev"
	Commit    = "unknown"
	BuildDate = "unknown"
)

// Command represents a registered CLI command.
type Command struct {
	Name     string
	Short    string
	Category string
	Run      func(args []string) error
	Hidden   bool
}

// Registry holds all registered commands.
type Registry struct {
	commands map[string]*Command
	ordered  []*Command
}

func newRegistry() *Registry {
	return &Registry{commands: make(map[string]*Command)}
}

func (r *Registry) register(cmd *Command) {
	r.commands[cmd.Name] = cmd
	r.ordered = append(r.ordered, cmd)
}

func (r *Registry) lookup(name string) (*Command, bool) {
	cmd, ok := r.commands[name]
	return cmd, ok
}

func (r *Registry) isCommand(name string) bool {
	_, ok := r.commands[name]
	return ok
}

// buildRegistry creates the command registry with all CLI commands.
func buildRegistry() *Registry {
	reg := newRegistry()

	// Core
	reg.register(&Command{Name: "up", Short: "Start services from mockd.yaml (Docker Compose style)", Category: "Core", Run: cli.RunUp})
	reg.register(&Command{Name: "down", Short: "Stop services started by 'mockd up'", Category: "Core", Run: cli.RunDown})
	reg.register(&Command{Name: "ps", Short: "Show running services", Category: "Core", Run: cli.RunPs})
	reg.register(&Command{Name: "serve", Short: "Start the mock server (default command)", Category: "Core", Run: cli.RunServe})
	reg.register(&Command{Name: "init", Short: "Create a starter config file", Category: "Core", Run: cli.RunInit})
	reg.register(&Command{Name: "start", Short: "Start the mock server (alias for serve)", Category: "Core", Run: cli.RunStart})
	reg.register(&Command{Name: "stop", Short: "Stop a running mockd server", Category: "Core", Run: cli.RunStop})
	reg.register(&Command{Name: "status", Short: "Show status of running mockd server", Category: "Core", Run: cli.RunStatus})
	reg.register(&Command{Name: "ports", Short: "Show all ports in use by mockd", Category: "Core", Run: cli.RunPorts})
	reg.register(&Command{Name: "validate", Short: "Validate config file without starting services", Category: "Core", Run: cli.RunValidate})

	// Configuration
	reg.register(&Command{Name: "context", Short: "Manage contexts (admin server + workspace pairs)", Category: "Configuration", Run: cli.RunContext})
	reg.register(&Command{Name: "workspace", Short: "Manage workspaces within the current context", Category: "Configuration", Run: cli.RunWorkspace})
	reg.register(&Command{Name: "config", Short: "Show effective configuration", Category: "Configuration", Run: cli.RunConfig})

	// Mock Management
	reg.register(&Command{Name: "add", Short: "Add a new mock endpoint", Category: "Mock Management", Run: cli.RunAdd})
	reg.register(&Command{Name: "new", Short: "Create mocks from templates (crud, auth, etc.)", Category: "Mock Management", Run: cli.RunNew})
	reg.register(&Command{Name: "list", Short: "List all configured mocks", Category: "Mock Management", Run: cli.RunList})
	reg.register(&Command{Name: "get", Short: "Get details of a specific mock", Category: "Mock Management", Run: cli.RunGet})
	reg.register(&Command{Name: "delete", Short: "Delete a mock by ID", Category: "Mock Management", Run: cli.RunDelete})
	reg.register(&Command{Name: "import", Short: "Import mocks from a configuration file", Category: "Mock Management", Run: cli.RunImport})
	reg.register(&Command{Name: "export", Short: "Export current mocks to stdout or file", Category: "Mock Management", Run: cli.RunExport})
	reg.register(&Command{Name: "logs", Short: "View request logs", Category: "Mock Management", Run: cli.RunLogs})

	// Utilities
	reg.register(&Command{Name: "completion", Short: "Generate shell completion scripts", Category: "Utilities", Run: cli.RunCompletion})
	reg.register(&Command{
		Name: "version", Short: "Show version information", Category: "Utilities",
		Run: func(args []string) error {
			return cli.RunVersion(cli.BuildInfo{Version: Version, Commit: Commit, BuildDate: BuildDate}, args)
		},
	})
	reg.register(&Command{Name: "doctor", Short: "Diagnose common setup issues", Category: "Utilities", Run: cli.RunDoctor})
	reg.register(&Command{Name: "help", Short: "Show help for a topic or command", Category: "Utilities", Run: cli.RunHelp})

	// Proxy
	reg.register(&Command{Name: "proxy", Short: "Manage the MITM proxy for recording API traffic", Category: "Proxy", Run: cli.RunProxy})
	reg.register(&Command{Name: "recordings", Short: "Manage recorded API traffic", Category: "Proxy", Run: cli.RunRecordings})
	reg.register(&Command{Name: "stream-recordings", Short: "Manage WebSocket and SSE stream recordings", Category: "Proxy", Run: cli.RunStreamRecordings})

	// AI
	reg.register(&Command{Name: "generate", Short: "Generate mocks from OpenAPI or natural language (with --ai)", Category: "AI", Run: cli.RunGenerate})
	reg.register(&Command{Name: "enhance", Short: "Enhance existing mocks with AI-generated data", Category: "AI", Run: cli.RunEnhance})
	reg.register(&Command{Name: "convert", Short: "Convert recordings to mock definitions", Category: "AI", Run: cli.RunConvert})

	// GraphQL
	reg.register(&Command{Name: "graphql", Short: "Validate schemas and execute GraphQL queries", Category: "GraphQL", Run: cli.RunGraphQL})

	// Chaos
	reg.register(&Command{Name: "chaos", Short: "Manage chaos injection (enable, disable, status)", Category: "Chaos", Run: cli.RunChaos})

	// gRPC
	reg.register(&Command{Name: "grpc", Short: "Manage and test gRPC endpoints", Category: "gRPC", Run: cli.RunGRPC})

	// MQTT
	reg.register(&Command{Name: "mqtt", Short: "Publish, subscribe, and manage MQTT broker", Category: "MQTT", Run: cli.RunMQTT})

	// WebSocket
	reg.register(&Command{Name: "websocket", Short: "Connect, send, and listen to WebSocket endpoints", Category: "WebSocket", Run: cli.RunWebSocket})

	// SOAP
	reg.register(&Command{Name: "soap", Short: "Validate WSDL files and call SOAP operations", Category: "SOAP", Run: cli.RunSOAP})

	// MCP
	reg.register(&Command{Name: "mcp", Short: "Start MCP server for AI assistants (stdio transport)", Category: "MCP", Run: cli.RunMCP})

	// Templates
	reg.register(&Command{Name: "templates", Short: "List and add templates from the official library", Category: "Templates", Run: cli.RunTemplates})

	// Cloud
	reg.register(&Command{Name: "tunnel", Short: "Expose local mocks via secure tunnel", Category: "Cloud", Run: cli.RunTunnel})
	reg.register(&Command{Name: "tunnel-quic", Short: "Expose local mocks via QUIC tunnel", Category: "Cloud", Run: cli.RunTunnelQUIC, Hidden: true})

	return reg
}

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	reg := buildRegistry()

	// Determine command and args
	command := ""
	var cmdArgs []string

	switch {
	case len(args) == 0:
		// No args at all, run serve
		command = "serve"
		cmdArgs = []string{}
	case args[0] == "" || args[0][0] == '-':
		first := args[0]
		// Flag passed directly (e.g., --help, --version, --port), handle global flags or run serve
		switch first {
		case "--help", "-h":
			printUsage(reg)
			return nil
		case "--version", "-v":
			return cli.RunVersion(cli.BuildInfo{
				Version:   Version,
				Commit:    Commit,
				BuildDate: BuildDate,
			}, nil)
		default:
			// Other flags, run serve with them
			command = "serve"
			cmdArgs = args
		}
	case reg.isCommand(args[0]):
		command = args[0]
		cmdArgs = args[1:]
	default:
		// Unknown argument, try serve
		command = "serve"
		cmdArgs = args
	}

	cmd, ok := reg.lookup(command)
	if !ok {
		return fmt.Errorf("unknown command: %s\n\nRun 'mockd --help' for usage", command)
	}
	return cmd.Run(cmdArgs)
}

func printUsage(reg *Registry) {
	fmt.Print("mockd - Local-first mock server for API development\n\n")
	fmt.Print("Usage:\n")
	fmt.Print("  mockd                          Start mock server with defaults\n")
	fmt.Print("  mockd <command> [flags]        Run a specific command\n")
	fmt.Print("  mockd --help                   Show this help message\n\n")

	// Group commands by category in display order.
	categories := []string{
		"Core", "Configuration", "Mock Management", "Utilities",
		"Proxy", "AI", "GraphQL", "Chaos", "gRPC", "MQTT",
		"WebSocket", "SOAP", "MCP", "Templates", "Cloud",
	}

	groups := make(map[string][]*Command)
	for _, cmd := range reg.ordered {
		if !cmd.Hidden {
			groups[cmd.Category] = append(groups[cmd.Category], cmd)
		}
	}

	for _, cat := range categories {
		cmds := groups[cat]
		if len(cmds) == 0 {
			continue
		}
		fmt.Printf("%s:\n", cat)
		for _, cmd := range cmds {
			fmt.Printf("  %-24s %s\n", cmd.Name, cmd.Short)
		}
		fmt.Println()
	}

	fmt.Print(`Global Flags:
  -h, --help      Show this help message
  -v, --version   Show version information

Examples:
  # Create a starter config and start the server
  mockd init
  mockd serve --config mockd.yaml

  # Start the server with defaults
  mockd start

  # Connect to a remote mockd server
  mockd context add staging --admin-url https://staging:4290 --use
  mockd list  # uses staging server

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
