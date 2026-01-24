package cli

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/getmockd/mockd/pkg/cli/templates"
	"github.com/getmockd/mockd/pkg/config"
	"github.com/getmockd/mockd/pkg/mock"
	"gopkg.in/yaml.v3"
)

// RunInit handles the init command for creating a starter config file.
func RunInit(args []string) error {
	fs := flag.NewFlagSet("init", flag.ContinueOnError)

	force := fs.Bool("force", false, "Overwrite existing config file")
	output := fs.String("output", "mockd.yaml", "Output filename")
	fs.StringVar(output, "o", "mockd.yaml", "Output filename (shorthand)")
	format := fs.String("format", "", "Output format: yaml or json (default: inferred from filename)")
	interactive := fs.Bool("interactive", false, "Interactive mode - prompts for configuration")
	fs.BoolVar(interactive, "i", false, "Interactive mode (shorthand)")
	template := fs.String("template", "default", "Template to use (use 'list' to see available templates)")
	fs.StringVar(template, "t", "default", "Template to use (shorthand)")

	fs.Usage = func() {
		fmt.Fprint(os.Stderr, `Usage: mockd init [flags]

Create a starter mockd configuration file with example mocks.

Flags:
      --force           Overwrite existing config file
  -o, --output          Output filename (default: mockd.yaml)
      --format          Output format: yaml or json (default: inferred from filename)
  -i, --interactive     Interactive mode - prompts for configuration
  -t, --template        Template to use (default: default)

Templates:
  default          Basic HTTP mocks (hello, echo, health)
  crud             Full REST CRUD API for resources
  websocket-chat   Chat room WebSocket endpoint with echo
  graphql-api      GraphQL API with User CRUD resolvers
  grpc-service     gRPC Greeter service with reflection
  mqtt-iot         MQTT broker with IoT sensor topics

Examples:
  # Create default mockd.yaml
  mockd init

  # List available templates
  mockd init --template list

  # Use CRUD API template
  mockd init --template crud

  # Use WebSocket template with custom output
  mockd init -t websocket-chat -o websocket.yaml

  # Interactive setup
  mockd init -i

  # Create with custom filename
  mockd init -o my-mocks.yaml

  # Create JSON config
  mockd init --format json -o mocks.json

  # Overwrite existing config
  mockd init --force
`)
	}

	if err := fs.Parse(args); err != nil {
		return err
	}

	// Handle template list command
	if *template == "list" {
		fmt.Print(templates.FormatList())
		return nil
	}

	// Determine output format
	outputFormat := strings.ToLower(*format)
	if outputFormat == "" {
		// Infer from filename extension
		ext := strings.ToLower(filepath.Ext(*output))
		if ext == ".json" {
			outputFormat = "json"
		} else {
			outputFormat = "yaml"
		}
	}

	// Validate format
	if outputFormat != "yaml" && outputFormat != "json" {
		return fmt.Errorf("invalid format: %s (must be yaml or json)", outputFormat)
	}

	// Check if file already exists
	if _, err := os.Stat(*output); err == nil {
		if !*force {
			return fmt.Errorf("file already exists: %s\n\nUse --force to overwrite", *output)
		}
	}

	// Build the config - either interactively, from template, or with defaults
	var data []byte
	var err error

	if *interactive {
		collection, err := runInteractiveInit()
		if err != nil {
			return err
		}
		// Generate output for interactive mode
		if outputFormat == "json" {
			data, err = json.MarshalIndent(collection, "", "  ")
			if err != nil {
				return fmt.Errorf("failed to generate JSON: %w", err)
			}
			data = append(data, '\n')
		} else {
			data, err = generateYAMLWithComments(collection)
			if err != nil {
				return fmt.Errorf("failed to generate YAML: %w", err)
			}
		}
	} else {
		// Use template
		if !templates.Exists(*template) {
			return fmt.Errorf("unknown template: %s\n\nRun 'mockd init --template list' to see available templates", *template)
		}

		data, err = templates.Get(*template)
		if err != nil {
			return fmt.Errorf("failed to load template: %w", err)
		}

		// Convert to JSON if requested
		if outputFormat == "json" {
			// Parse YAML template and convert to JSON
			var collection config.MockCollection
			if err := yaml.Unmarshal(data, &collection); err != nil {
				return fmt.Errorf("failed to parse template: %w", err)
			}
			data, err = json.MarshalIndent(collection, "", "  ")
			if err != nil {
				return fmt.Errorf("failed to generate JSON: %w", err)
			}
			data = append(data, '\n')
		}
	}

	// Write the file
	if err := os.WriteFile(*output, data, 0644); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	// Print success message
	tmpl, _ := templates.GetTemplate(*template)
	fmt.Printf("Created %s", *output)
	if tmpl != nil && *template != "default" {
		fmt.Printf(" (template: %s)", tmpl.Name)
	}
	fmt.Println()
	fmt.Println()
	fmt.Println("Next steps:")
	fmt.Printf("  mockd serve --config %s\n", *output)

	// Template-specific hints
	switch *template {
	case "crud":
		fmt.Println("  curl http://localhost:4280/api/resources")
	case "websocket-chat":
		fmt.Println("  wscat -c ws://localhost:4280/ws/chat")
	case "graphql-api":
		fmt.Println("  curl -X POST http://localhost:4280/graphql -H 'Content-Type: application/json' -d '{\"query\": \"{ users { id name } }\"}'")
	case "grpc-service":
		fmt.Println("  grpcurl -plaintext localhost:50051 list")
	case "mqtt-iot":
		//nolint:misspell // mosquitto is the correct name of the MQTT broker software
		fmt.Println("  mosquitto_sub -h localhost -p 1883 -t 'sensors/#'")
	default:
		fmt.Println("  curl http://localhost:4280/hello")
	}

	return nil
}

// generateYAMLWithComments generates YAML output with header comments.
func generateYAMLWithComments(collection *config.MockCollection) ([]byte, error) {
	// Generate the YAML content
	yamlData, err := yaml.Marshal(collection)
	if err != nil {
		return nil, err
	}

	// Add header comments
	header := `# mockd.yaml
# Generated by: mockd init
# Documentation: https://mockd.io/docs
#
# Start server:  mockd serve --config mockd.yaml
# Test endpoint: curl http://localhost:4280/hello

`
	return append([]byte(header), yamlData...), nil
}

// runInteractiveInit prompts the user for configuration options.
func runInteractiveInit() (*config.MockCollection, error) {
	reader := bufio.NewReader(os.Stdin)

	fmt.Println("mockd Interactive Setup")
	fmt.Println("========================")
	fmt.Println()

	// Protocol selection
	fmt.Println("Select protocol type:")
	fmt.Println("  1. HTTP (REST API)")
	fmt.Println("  2. WebSocket")
	fmt.Println("  3. GraphQL")
	fmt.Println("  4. gRPC")
	fmt.Println("  5. MQTT")
	fmt.Println("  6. SOAP")
	fmt.Println()
	fmt.Print("Choice [1]: ")

	choiceInput, _ := reader.ReadString('\n')
	choice := strings.TrimSpace(choiceInput)
	if choice == "" {
		choice = "1"
	}

	fmt.Println()

	switch choice {
	case "1":
		return interactiveHTTP(reader)
	case "2":
		return interactiveWebSocket(reader)
	case "3":
		return interactiveGraphQL(reader)
	case "4":
		return interactiveGRPC(reader)
	case "5":
		return interactiveMQTT(reader)
	case "6":
		return interactiveSOAP(reader)
	default:
		return interactiveHTTP(reader)
	}
}

// interactiveHTTP prompts for HTTP mock configuration.
func interactiveHTTP(reader *bufio.Reader) (*config.MockCollection, error) {
	fmt.Println("HTTP Mock Configuration")
	fmt.Println("-----------------------")
	fmt.Println()

	// Prompt for endpoint path
	fmt.Print("Endpoint path [/hello]: ")
	pathInput, _ := reader.ReadString('\n')
	path := strings.TrimSpace(pathInput)
	if path == "" {
		path = "/hello"
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}

	// Prompt for HTTP method
	fmt.Print("HTTP method [GET]: ")
	methodInput, _ := reader.ReadString('\n')
	method := strings.ToUpper(strings.TrimSpace(methodInput))
	if method == "" {
		method = "GET"
	}

	// Prompt for response status
	fmt.Print("Response status code [200]: ")
	statusInput, _ := reader.ReadString('\n')
	statusStr := strings.TrimSpace(statusInput)
	status := 200
	if statusStr != "" {
		if parsed, err := strconv.Atoi(statusStr); err == nil {
			status = parsed
		}
	}

	// Prompt for response body
	fmt.Print("Response body (JSON) [{\"message\": \"Hello!\"}]: ")
	bodyInput, _ := reader.ReadString('\n')
	body := strings.TrimSpace(bodyInput)
	if body == "" {
		body = `{"message": "Hello!"}`
	}

	// Prompt for mock name
	fmt.Print("Mock name [My API]: ")
	nameInput, _ := reader.ReadString('\n')
	name := strings.TrimSpace(nameInput)
	if name == "" {
		name = "My API"
	}

	id := strings.ToLower(strings.ReplaceAll(name, " ", "-"))

	fmt.Println()
	fmt.Println("Creating HTTP mock configuration...")

	return &config.MockCollection{
		Version: "1.0",
		Mocks: []*mock.Mock{
			{
				ID:      id,
				Name:    name,
				Type:    mock.MockTypeHTTP,
				Enabled: true,
				HTTP: &mock.HTTPSpec{
					Matcher: &mock.HTTPMatcher{
						Method: method,
						Path:   path,
					},
					Response: &mock.HTTPResponse{
						StatusCode: status,
						Headers: map[string]string{
							"Content-Type": "application/json",
						},
						Body: body,
					},
				},
			},
		},
	}, nil
}

// interactiveWebSocket prompts for WebSocket mock configuration.
func interactiveWebSocket(reader *bufio.Reader) (*config.MockCollection, error) {
	fmt.Println("WebSocket Mock Configuration")
	fmt.Println("----------------------------")
	fmt.Println()

	// Prompt for path
	fmt.Print("WebSocket path [/ws]: ")
	pathInput, _ := reader.ReadString('\n')
	path := strings.TrimSpace(pathInput)
	if path == "" {
		path = "/ws"
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}

	// Prompt for echo mode
	fmt.Print("Enable echo mode? [Y/n]: ")
	echoInput, _ := reader.ReadString('\n')
	echoStr := strings.ToLower(strings.TrimSpace(echoInput))
	echoMode := echoStr == "" || echoStr == "y" || echoStr == "yes"

	// Prompt for mock name
	fmt.Print("Mock name [WebSocket Endpoint]: ")
	nameInput, _ := reader.ReadString('\n')
	name := strings.TrimSpace(nameInput)
	if name == "" {
		name = "WebSocket Endpoint"
	}

	id := strings.ToLower(strings.ReplaceAll(name, " ", "-"))

	fmt.Println()
	fmt.Println("Creating WebSocket mock configuration...")

	return &config.MockCollection{
		Version: "1.0",
		Mocks: []*mock.Mock{
			{
				ID:      id,
				Name:    name,
				Type:    mock.MockTypeWebSocket,
				Enabled: true,
				WebSocket: &mock.WebSocketSpec{
					Path:     path,
					EchoMode: &echoMode,
				},
			},
		},
	}, nil
}

// interactiveGraphQL prompts for GraphQL mock configuration.
func interactiveGraphQL(reader *bufio.Reader) (*config.MockCollection, error) {
	fmt.Println("GraphQL Mock Configuration")
	fmt.Println("--------------------------")
	fmt.Println()

	// Prompt for path
	fmt.Print("GraphQL endpoint path [/graphql]: ")
	pathInput, _ := reader.ReadString('\n')
	path := strings.TrimSpace(pathInput)
	if path == "" {
		path = "/graphql"
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}

	// Prompt for introspection
	fmt.Print("Enable introspection? [Y/n]: ")
	introInput, _ := reader.ReadString('\n')
	introStr := strings.ToLower(strings.TrimSpace(introInput))
	introspection := introStr == "" || introStr == "y" || introStr == "yes"

	// Prompt for mock name
	fmt.Print("Mock name [GraphQL API]: ")
	nameInput, _ := reader.ReadString('\n')
	name := strings.TrimSpace(nameInput)
	if name == "" {
		name = "GraphQL API"
	}

	id := strings.ToLower(strings.ReplaceAll(name, " ", "-"))

	fmt.Println()
	fmt.Println("Creating GraphQL mock configuration...")

	schema := `type Query {
  hello: String
  users: [User!]!
}

type User {
  id: ID!
  name: String!
  email: String
}`

	return &config.MockCollection{
		Version: "1.0",
		Mocks: []*mock.Mock{
			{
				ID:      id,
				Name:    name,
				Type:    mock.MockTypeGraphQL,
				Enabled: true,
				GraphQL: &mock.GraphQLSpec{
					Path:          path,
					Schema:        schema,
					Introspection: introspection,
					Resolvers: map[string]mock.ResolverConfig{
						"Query.hello": {Response: "Hello from GraphQL!"},
						"Query.users": {Response: []map[string]any{
							{"id": "1", "name": "Alice", "email": "alice@example.com"},
							{"id": "2", "name": "Bob", "email": "bob@example.com"},
						}},
					},
				},
			},
		},
	}, nil
}

// interactiveGRPC prompts for gRPC mock configuration.
func interactiveGRPC(reader *bufio.Reader) (*config.MockCollection, error) {
	fmt.Println("gRPC Mock Configuration")
	fmt.Println("-----------------------")
	fmt.Println()

	// Prompt for port
	fmt.Print("gRPC port [50051]: ")
	portInput, _ := reader.ReadString('\n')
	portStr := strings.TrimSpace(portInput)
	port := 50051
	if portStr != "" {
		if parsed, err := strconv.Atoi(portStr); err == nil {
			port = parsed
		}
	}

	// Prompt for service name
	fmt.Print("Service name [greeter.Greeter]: ")
	serviceInput, _ := reader.ReadString('\n')
	service := strings.TrimSpace(serviceInput)
	if service == "" {
		service = "greeter.Greeter"
	}

	// Prompt for method name
	fmt.Print("Method name [SayHello]: ")
	methodInput, _ := reader.ReadString('\n')
	method := strings.TrimSpace(methodInput)
	if method == "" {
		method = "SayHello"
	}

	// Prompt for reflection
	fmt.Print("Enable reflection? [Y/n]: ")
	reflInput, _ := reader.ReadString('\n')
	reflStr := strings.ToLower(strings.TrimSpace(reflInput))
	reflection := reflStr == "" || reflStr == "y" || reflStr == "yes"

	// Prompt for mock name
	fmt.Print("Mock name [gRPC Service]: ")
	nameInput, _ := reader.ReadString('\n')
	name := strings.TrimSpace(nameInput)
	if name == "" {
		name = "gRPC Service"
	}

	id := strings.ToLower(strings.ReplaceAll(name, " ", "-"))

	fmt.Println()
	fmt.Println("Creating gRPC mock configuration...")

	return &config.MockCollection{
		Version: "1.0",
		Mocks: []*mock.Mock{
			{
				ID:      id,
				Name:    name,
				Type:    mock.MockTypeGRPC,
				Enabled: true,
				GRPC: &mock.GRPCSpec{
					Port:       port,
					Reflection: reflection,
					Services: map[string]mock.ServiceConfig{
						service: {
							Methods: map[string]mock.MethodConfig{
								method: {
									Response: map[string]any{
										"message": "Hello from gRPC!",
									},
								},
							},
						},
					},
				},
			},
		},
	}, nil
}

// interactiveMQTT prompts for MQTT mock configuration.
func interactiveMQTT(reader *bufio.Reader) (*config.MockCollection, error) {
	fmt.Println("MQTT Broker Configuration")
	fmt.Println("-------------------------")
	fmt.Println()

	// Prompt for port
	fmt.Print("MQTT port [1883]: ")
	portInput, _ := reader.ReadString('\n')
	portStr := strings.TrimSpace(portInput)
	port := 1883
	if portStr != "" {
		if parsed, err := strconv.Atoi(portStr); err == nil {
			port = parsed
		}
	}

	// Prompt for topic
	fmt.Print("Topic pattern [sensors/temperature]: ")
	topicInput, _ := reader.ReadString('\n')
	topic := strings.TrimSpace(topicInput)
	if topic == "" {
		topic = "sensors/temperature"
	}

	// Prompt for mock name
	fmt.Print("Mock name [MQTT Broker]: ")
	nameInput, _ := reader.ReadString('\n')
	name := strings.TrimSpace(nameInput)
	if name == "" {
		name = "MQTT Broker"
	}

	id := strings.ToLower(strings.ReplaceAll(name, " ", "-"))

	fmt.Println()
	fmt.Println("Creating MQTT mock configuration...")

	return &config.MockCollection{
		Version: "1.0",
		Mocks: []*mock.Mock{
			{
				ID:      id,
				Name:    name,
				Type:    mock.MockTypeMQTT,
				Enabled: true,
				MQTT: &mock.MQTTSpec{
					Port: port,
					Topics: []mock.TopicConfig{
						{
							Topic: topic,
							QoS:   1,
							Messages: []mock.MessageConfig{
								{
									Payload:  `{"value": 25.5, "unit": "C", "timestamp": "{{now}}"}`,
									Interval: "5s",
								},
							},
						},
					},
				},
			},
		},
	}, nil
}

// interactiveSOAP prompts for SOAP mock configuration.
func interactiveSOAP(reader *bufio.Reader) (*config.MockCollection, error) {
	fmt.Println("SOAP Mock Configuration")
	fmt.Println("-----------------------")
	fmt.Println()

	// Prompt for path
	fmt.Print("SOAP endpoint path [/soap/service]: ")
	pathInput, _ := reader.ReadString('\n')
	path := strings.TrimSpace(pathInput)
	if path == "" {
		path = "/soap/service"
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}

	// Prompt for operation name
	fmt.Print("Operation name [GetUser]: ")
	opInput, _ := reader.ReadString('\n')
	operation := strings.TrimSpace(opInput)
	if operation == "" {
		operation = "GetUser"
	}

	// Prompt for mock name
	fmt.Print("Mock name [SOAP Service]: ")
	nameInput, _ := reader.ReadString('\n')
	name := strings.TrimSpace(nameInput)
	if name == "" {
		name = "SOAP Service"
	}

	id := strings.ToLower(strings.ReplaceAll(name, " ", "-"))

	fmt.Println()
	fmt.Println("Creating SOAP mock configuration...")

	return &config.MockCollection{
		Version: "1.0",
		Mocks: []*mock.Mock{
			{
				ID:      id,
				Name:    name,
				Type:    mock.MockTypeSOAP,
				Enabled: true,
				SOAP: &mock.SOAPSpec{
					Path: path,
					Operations: map[string]mock.OperationConfig{
						operation: {
							Response: `<` + operation + `Response>
  <Result>
    <Id>1</Id>
    <Name>Example</Name>
  </Result>
</` + operation + `Response>`,
						},
					},
				},
			},
		},
	}, nil
}
