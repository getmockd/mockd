package cli

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/getmockd/mockd/pkg/cli/internal/flags"
	"github.com/getmockd/mockd/pkg/cli/internal/output"
	"github.com/getmockd/mockd/pkg/cli/internal/parse"
	"github.com/getmockd/mockd/pkg/cliconfig"
	"github.com/getmockd/mockd/pkg/config"
	"github.com/getmockd/mockd/pkg/mock"
)

// RunAdd handles the add command.
func RunAdd(args []string) error {
	fs := flag.NewFlagSet("add", flag.ContinueOnError)

	// Mock type (new)
	mockType := fs.String("type", "http", "Mock type (http, websocket, graphql, grpc, mqtt, soap)")
	fs.StringVar(mockType, "t", "http", "Mock type (shorthand)")

	// Common flags
	name := fs.String("name", "", "Mock display name")
	fs.StringVar(name, "n", "", "Mock display name (shorthand)")

	// Admin URL and output format
	adminURL := fs.String("admin-url", cliconfig.GetAdminURL(), "Admin API base URL")
	jsonOutput := fs.Bool("json", false, "Output in JSON format")

	// HTTP flags
	method := fs.String("method", "GET", "HTTP method to match")
	fs.StringVar(method, "m", "GET", "HTTP method to match (shorthand)")
	path := fs.String("path", "", "URL path to match (required for http, websocket, graphql, soap)")
	status := fs.Int("status", 200, "Response status code")
	fs.IntVar(status, "s", 200, "Response status code (shorthand)")
	body := fs.String("body", "", "Response body")
	fs.StringVar(body, "b", "", "Response body (shorthand)")
	bodyFile := fs.String("body-file", "", "Read response body from file")
	var headers flags.StringSlice
	fs.Var(&headers, "header", "Response header (key:value), repeatable")
	fs.Var(&headers, "H", "Response header (key:value), repeatable (shorthand)")
	var matchHeaders flags.StringSlice
	fs.Var(&matchHeaders, "match-header", "Required request header (key:value), repeatable")
	var matchQueries flags.StringSlice
	fs.Var(&matchQueries, "match-query", "Required query param (key:value), repeatable")
	priority := fs.Int("priority", 0, "Mock priority (higher = matched first)")
	delay := fs.Int("delay", 0, "Response delay in milliseconds")

	// SSE flags (for HTTP mocks with SSE streaming)
	sse := fs.Bool("sse", false, "Enable Server-Sent Events streaming")
	var sseEvents flags.StringSlice
	fs.Var(&sseEvents, "sse-event", "SSE event (type:data or just data), repeatable")
	sseDelay := fs.Int("sse-delay", 100, "Delay between SSE events in milliseconds")
	sseTemplate := fs.String("sse-template", "", "SSE template: openai, notification")
	sseRepeat := fs.Int("sse-repeat", 1, "Number of times to repeat SSE events (0 = infinite)")
	sseKeepalive := fs.Int("sse-keepalive", 0, "SSE keepalive interval in milliseconds (0 = disabled)")

	// WebSocket flags
	message := fs.String("message", "", "Default response message (JSON) for WebSocket")
	echo := fs.Bool("echo", false, "Enable echo mode for WebSocket")

	// GraphQL flags
	operation := fs.String("operation", "", "Operation name (required for graphql, soap)")
	opType := fs.String("op-type", "query", "GraphQL operation type (query/mutation)")
	response := fs.String("response", "", "JSON response data (for graphql, grpc)")

	// gRPC flags
	service := fs.String("service", "", "gRPC service name (e.g., greeter.Greeter)")
	rpcMethod := fs.String("rpc-method", "", "gRPC method name")
	grpcPort := fs.Int("grpc-port", 50051, "gRPC server port")
	var protoFiles flags.StringSlice
	fs.Var(&protoFiles, "proto", "Path to .proto file (required for gRPC, repeatable)")
	var protoPaths flags.StringSlice
	fs.Var(&protoPaths, "proto-path", "Import path for proto dependencies (repeatable)")

	// MQTT flags
	topic := fs.String("topic", "", "MQTT topic pattern")
	payload := fs.String("payload", "", "MQTT response payload")
	qos := fs.Int("qos", 0, "MQTT QoS level (0, 1, 2)")
	mqttPort := fs.Int("mqtt-port", 1883, "MQTT broker port")

	// SOAP flags
	soapAction := fs.String("soap-action", "", "SOAPAction header value")

	fs.Usage = func() {
		fmt.Fprint(os.Stderr, `Usage: mockd add [flags]

Add a new mock endpoint.

Global Flags:
  -t, --type          Mock type: http, websocket, graphql, grpc, mqtt, soap (default: http)
  -n, --name          Mock display name
      --admin-url     Admin API base URL (default: http://localhost:4290)
      --json          Output in JSON format

HTTP Flags (--type http):
  -m, --method        HTTP method to match (default: GET)
      --path          URL path to match (required)
  -s, --status        Response status code (default: 200)
  -b, --body          Response body
      --body-file     Read response body from file
  -H, --header        Response header (key:value), repeatable
      --match-header  Required request header (key:value), repeatable
      --match-query   Required query param (key:value), repeatable
      --priority      Mock priority (higher = matched first)
      --delay         Response delay in milliseconds

SSE Flags (Server-Sent Events, use with --type http):
      --sse           Enable SSE streaming response
      --sse-event     SSE event (type:data), repeatable. Examples:
                        --sse-event 'message:hello'
                        --sse-event 'update:{"count":1}'
      --sse-delay     Delay between events in ms (default: 100)
      --sse-template  Use built-in template: openai-chat, notification-stream
      --sse-repeat    Repeat events N times (0 = infinite, default: 1)
      --sse-keepalive Keepalive interval in ms (0 = disabled)

WebSocket Flags (--type websocket):
      --path          WebSocket path (required)
      --message       Default response message (JSON)
      --echo          Enable echo mode

GraphQL Flags (--type graphql):
      --path          GraphQL endpoint path (default: /graphql)
      --operation     Operation name (required)
      --op-type       Operation type: query or mutation (default: query)
      --response      JSON response data
  NOTE: For full GraphQL with schema validation, use YAML config file.

gRPC Flags (--type grpc):
      --proto         Path to .proto file (required, repeatable for multiple files)
      --proto-path    Import path for proto dependencies (repeatable)
      --service       Service name, e.g., myapp.UserService (required)
      --rpc-method    RPC method name (required)
      --response      JSON response data
      --grpc-port     gRPC server port (default: 50051)
                      If a gRPC mock already exists on this port, the new
                      service/method is merged into it automatically.
                      For multiple services, use YAML config with services array.

MQTT Flags (--type mqtt):
      --topic         Topic pattern (required)
      --payload       Response payload
      --qos           QoS level: 0, 1, or 2 (default: 0)
      --mqtt-port     MQTT broker port (default: 1883)
                      If an MQTT mock already exists on this port, the new
                      topic is merged into it automatically.
                      For multiple topics, use YAML config with topics array.

SOAP Flags (--type soap):
      --path          SOAP endpoint path (default: /soap)
      --operation     SOAP operation name (required)
      --soap-action   SOAPAction header value
      --response      XML response body

Examples:
  # Simple HTTP GET mock
  mockd add --path /api/users --status 200 --body '[]'

  # HTTP POST with JSON response
  mockd add -m POST --path /api/users -s 201 \
    -b '{"id": "new-id", "created": true}' \
    -H "Content-Type:application/json"

  # SSE endpoint with custom events
  mockd add --path /events --sse \
    --sse-event 'connected:{"status":"ok"}' \
    --sse-event 'update:{"count":1}' \
    --sse-event 'update:{"count":2}' \
    --sse-delay 500

  # SSE with OpenAI-style streaming (for LLM mock)
  mockd add --path /v1/chat/completions --sse --sse-template openai-chat

  # Infinite SSE stream (heartbeat style)
  mockd add --path /stream --sse \
    --sse-event 'ping:{}' --sse-delay 1000 --sse-repeat 0

  # WebSocket mock with echo mode
  mockd add --type websocket --path /ws/chat --echo

  # WebSocket mock with default response
  mockd add --type websocket --path /ws/events \
    --message '{"type": "connected", "status": "ok"}'

  # GraphQL query mock
  mockd add --type graphql --operation getUser \
    --response '{"data": {"user": {"id": "1", "name": "Alice"}}}'

  # GraphQL mutation mock
  mockd add --type graphql --operation createUser --op-type mutation \
    --response '{"data": {"createUser": {"id": "new-id"}}}'

  # gRPC mock (proto file required)
  mockd add --type grpc \
    --proto ./service.proto \
    --service myapp.UserService \
    --rpc-method GetUser \
    --response '{"id": "1", "name": "Alice"}'

  # gRPC with import paths for proto dependencies
  mockd add --type grpc \
    --proto ./api/v1/service.proto \
    --proto-path ./api \
    --service myapp.v1.UserService \
    --rpc-method GetUser \
    --response '{"id": "1", "name": "Alice"}'

  # MQTT mock
  mockd add --type mqtt --topic sensors/temperature --payload '{"temp": 72.5}' --qos 1

  # SOAP mock
  mockd add --type soap --operation GetWeather --soap-action "http://example.com/GetWeather" \
    --response '<GetWeatherResponse><Temperature>72</Temperature></GetWeatherResponse>'
`)
	}

	if err := fs.Parse(args); err != nil {
		return err
	}

	// Normalize mock type
	mt := strings.ToLower(*mockType)

	// Validate mock type
	validTypes := map[string]mock.Type{
		"http":      mock.TypeHTTP,
		"websocket": mock.TypeWebSocket,
		"graphql":   mock.TypeGraphQL,
		"grpc":      mock.TypeGRPC,
		"mqtt":      mock.TypeMQTT,
		"soap":      mock.TypeSOAP,
	}

	mockTypeEnum, ok := validTypes[mt]
	if !ok {
		return fmt.Errorf("invalid mock type: %s\n\nValid types: http, websocket, graphql, grpc, mqtt, soap", *mockType)
	}

	// Build mock configuration based on type
	var m *config.MockConfiguration
	var err error

	switch mockTypeEnum { //nolint:exhaustive // OAuth not yet supported in CLI add
	case mock.TypeHTTP:
		m, err = buildHTTPMock(*name, *path, *method, *status, *body, *bodyFile, *priority, *delay, headers, matchHeaders, matchQueries,
			*sse, sseEvents, *sseDelay, *sseTemplate, *sseRepeat, *sseKeepalive)
	case mock.TypeWebSocket:
		m, err = buildWebSocketMock(*name, *path, *message, *echo)
	case mock.TypeGraphQL:
		m, err = buildGraphQLMock(*name, *path, *operation, *opType, *response)
	case mock.TypeGRPC:
		m, err = buildGRPCMock(*name, *service, *rpcMethod, *response, *grpcPort, protoFiles, protoPaths)
	case mock.TypeMQTT:
		m, err = buildMQTTMock(*name, *topic, *payload, *qos, *mqttPort)
	case mock.TypeSOAP:
		m, err = buildSOAPMock(*name, *path, *operation, *soapAction, *response)
	}

	if err != nil {
		return err
	}

	// Create admin client and add mock
	client := NewAdminClientWithAuth(*adminURL)
	result, err := client.CreateMock(m)
	if err != nil {
		return fmt.Errorf("%s", FormatConnectionError(err))
	}

	// Output result based on mock type and action (created vs merged)
	return outputResult(result, mockTypeEnum, *jsonOutput)
}

// buildHTTPMock creates an HTTP mock configuration.
func buildHTTPMock(name, path, method string, status int, body, bodyFile string, priority, delay int,
	headers, matchHeaders, matchQueries flags.StringSlice,
	sse bool, sseEvents flags.StringSlice, sseDelay int, sseTemplate string, sseRepeat, sseKeepalive int) (*config.MockConfiguration, error) {

	if path == "" {
		return nil, errors.New(`--path is required for HTTP mocks

Usage: mockd add --path /api/endpoint [flags]

Run 'mockd add --help' for more options`)
	}

	// Read body from file if specified
	responseBody := body
	if bodyFile != "" {
		data, err := os.ReadFile(bodyFile)
		if err != nil {
			return nil, fmt.Errorf("failed to read body file: %w", err)
		}
		responseBody = string(data)
	}

	// Parse response headers
	responseHeaders := make(map[string]string)
	for _, h := range headers {
		key, value, ok := parse.KeyValue(h, ':')
		if !ok {
			return nil, fmt.Errorf("invalid header format: %s (expected key:value)", h)
		}
		responseHeaders[key] = value
	}

	// Parse match headers
	matchHeadersMap := make(map[string]string)
	for _, h := range matchHeaders {
		key, value, ok := parse.KeyValue(h, ':')
		if !ok {
			return nil, fmt.Errorf("invalid match-header format: %s (expected key:value)", h)
		}
		matchHeadersMap[key] = value
	}

	// Parse match query params
	matchQueryMap := make(map[string]string)
	for _, q := range matchQueries {
		key, value, ok := parse.KeyValue(q, ':')
		if !ok {
			return nil, fmt.Errorf("invalid match-query format: %s (expected key:value)", q)
		}
		matchQueryMap[key] = value
	}

	enabled := true
	m := &config.MockConfiguration{
		Name:    name,
		Type:    mock.TypeHTTP,
		Enabled: &enabled,
		HTTP: &mock.HTTPSpec{
			Priority: priority,
			Matcher: &mock.HTTPMatcher{
				Method: strings.ToUpper(method),
				Path:   path,
			},
		},
	}

	// Build SSE config if enabled
	if sse || len(sseEvents) > 0 || sseTemplate != "" {
		sseConfig, err := buildSSEConfig(sseEvents, sseDelay, sseTemplate, sseRepeat, sseKeepalive)
		if err != nil {
			return nil, err
		}
		m.HTTP.SSE = sseConfig
	} else {
		// Only set Response for non-SSE mocks
		m.HTTP.Response = &mock.HTTPResponse{
			StatusCode: status,
			Body:       responseBody,
			DelayMs:    delay,
		}
		if len(responseHeaders) > 0 {
			m.HTTP.Response.Headers = responseHeaders
		}
	}

	if len(matchHeadersMap) > 0 {
		m.HTTP.Matcher.Headers = matchHeadersMap
	}
	if len(matchQueryMap) > 0 {
		m.HTTP.Matcher.QueryParams = matchQueryMap
	}

	return m, nil
}

// buildSSEConfig creates an SSE configuration from CLI flags.
//
//nolint:unparam // error is always nil but kept for future validation
func buildSSEConfig(events flags.StringSlice, delayMs int, template string, repeat, keepaliveMs int) (*mock.SSEConfig, error) {
	cfg := &mock.SSEConfig{
		Timing: mock.SSETimingConfig{
			FixedDelay: &delayMs,
		},
		Lifecycle: mock.SSELifecycleConfig{
			Termination: mock.SSETerminationConfig{
				Type: "complete",
			},
		},
	}

	// Set keepalive if specified
	if keepaliveMs > 0 {
		cfg.Lifecycle.KeepaliveInterval = keepaliveMs
	}

	// Handle template-based generation
	if template != "" {
		cfg.Template = template
		return cfg, nil
	}

	// Parse events from CLI
	if len(events) == 0 {
		// Default to a simple "connected" event
		cfg.Events = []mock.SSEEventDef{
			{Type: "message", Data: map[string]interface{}{"status": "connected"}},
		}
		return cfg, nil
	}

	parsedEvents := make([]mock.SSEEventDef, 0, len(events))
	for _, e := range events {
		eventType, data, hasType := parse.KeyValue(e, ':')
		if !hasType {
			// No type specified, use "message" as default
			parsedEvents = append(parsedEvents, mock.SSEEventDef{
				Type: "message",
				Data: e,
			})
		} else {
			// Try to parse data as JSON
			var jsonData interface{}
			if err := json.Unmarshal([]byte(data), &jsonData); err != nil {
				// Not JSON, use as string
				parsedEvents = append(parsedEvents, mock.SSEEventDef{
					Type: eventType,
					Data: data,
				})
			} else {
				parsedEvents = append(parsedEvents, mock.SSEEventDef{
					Type: eventType,
					Data: jsonData,
				})
			}
		}
	}
	cfg.Events = parsedEvents

	// Handle repeat (infinite streaming)
	if repeat == 0 {
		// Infinite repeat - use generator with template
		cfg.Generator = &mock.SSEEventGenerator{
			Type: "template",
			Template: &mock.SSETemplateGenerator{
				Events: parsedEvents,
				Repeat: 0, // 0 means infinite in template generator
			},
		}
		cfg.Events = nil // Clear events since we're using generator
	} else if repeat > 1 {
		// Repeat N times using generator
		cfg.Generator = &mock.SSEEventGenerator{
			Type: "template",
			Template: &mock.SSETemplateGenerator{
				Events: parsedEvents,
				Repeat: repeat,
			},
		}
		cfg.Events = nil
	}

	return cfg, nil
}

// buildWebSocketMock creates a WebSocket mock configuration.
func buildWebSocketMock(name, path, message string, echo bool) (*config.MockConfiguration, error) {
	if path == "" {
		return nil, errors.New(`--path is required for WebSocket mocks

Usage: mockd add --type websocket --path /ws/endpoint [flags]

Run 'mockd add --help' for more options`)
	}

	wsEnabled := true
	m := &config.MockConfiguration{
		Name:    name,
		Type:    mock.TypeWebSocket,
		Enabled: &wsEnabled,
		WebSocket: &mock.WebSocketSpec{
			Path: path,
		},
	}

	// Set echo mode
	if echo {
		m.WebSocket.EchoMode = &echo
	}

	// Set default response message
	if message != "" {
		m.WebSocket.DefaultResponse = &mock.WSMessageResponse{
			Type:  "json",
			Value: json.RawMessage(message),
		}
	}

	return m, nil
}

// buildGraphQLMock creates a GraphQL mock configuration.
func buildGraphQLMock(name, path, operation, opType, response string) (*config.MockConfiguration, error) {
	if operation == "" {
		return nil, errors.New(`--operation is required for GraphQL mocks

Usage: mockd add --type graphql --operation getUser [flags]

Run 'mockd add --help' for more options`)
	}

	// Validate operation type
	opType = strings.ToLower(opType)
	if opType != "query" && opType != "mutation" {
		return nil, fmt.Errorf("invalid --op-type: %s (must be 'query' or 'mutation')", opType)
	}

	// Default path
	if path == "" {
		path = "/graphql"
	}

	// Build resolver key based on operation type
	// Capitalize first letter of opType (e.g., "query" -> "Query")
	opTypeCapitalized := strings.ToUpper(opType[:1]) + opType[1:]
	resolverKey := fmt.Sprintf("%s.%s", opTypeCapitalized, operation)

	// Auto-generate a minimal schema for CLI-created GraphQL mocks
	schema := generateMinimalGraphQLSchema(operation, opType, response)

	gqlEnabled := true
	m := &config.MockConfiguration{
		Name:    name,
		Type:    mock.TypeGraphQL,
		Enabled: &gqlEnabled,
		GraphQL: &mock.GraphQLSpec{
			Path:      path,
			Schema:    schema,
			Resolvers: make(map[string]mock.ResolverConfig),
		},
	}

	// Parse response if provided
	if response != "" {
		var responseData interface{}
		if err := json.Unmarshal([]byte(response), &responseData); err != nil {
			return nil, fmt.Errorf("invalid JSON response: %w", err)
		}
		m.GraphQL.Resolvers[resolverKey] = mock.ResolverConfig{
			Response: responseData,
		}
	} else {
		m.GraphQL.Resolvers[resolverKey] = mock.ResolverConfig{}
	}

	return m, nil
}

// generateMinimalGraphQLSchema creates a minimal schema for CLI-created GraphQL mocks.
func generateMinimalGraphQLSchema(operation, opType, _ string) string {
	// Generate a simple schema that supports the operation
	// The response type is JSON (dynamic), so we use a scalar
	var sb strings.Builder

	sb.WriteString("scalar JSON\n\n")

	if opType == "query" {
		sb.WriteString("type Query {\n")
		sb.WriteString(fmt.Sprintf("  %s: JSON\n", operation))
		sb.WriteString("}\n")
	} else {
		sb.WriteString("type Query {\n")
		sb.WriteString("  _empty: String\n") // GraphQL requires at least one Query field
		sb.WriteString("}\n\n")
		sb.WriteString("type Mutation {\n")
		sb.WriteString(fmt.Sprintf("  %s: JSON\n", operation))
		sb.WriteString("}\n")
	}

	return sb.String()
}

// buildGRPCMock creates a gRPC mock configuration.
func buildGRPCMock(name, service, rpcMethod, response string, port int, protoFiles, protoPaths flags.StringSlice) (*config.MockConfiguration, error) {
	// Proto file is required for gRPC mocks
	if len(protoFiles) == 0 {
		return nil, errors.New(`--proto is required for gRPC mocks

gRPC mocks require a .proto file to define the service schema.

Usage: mockd add --type grpc --proto ./service.proto --service myapp.UserService --rpc-method GetUser [flags]

Examples:
  # Basic gRPC mock
  mockd add --type grpc \
    --proto ./service.proto \
    --service myapp.UserService \
    --rpc-method GetUser \
    --response '{"id": "1", "name": "Alice"}'

  # With import paths for proto dependencies
  mockd add --type grpc \
    --proto ./api/v1/service.proto \
    --proto-path ./api \
    --service myapp.v1.UserService \
    --rpc-method GetUser \
    --response '{"id": "1"}'

Run 'mockd add --help' for more options`)
	}

	if service == "" {
		return nil, errors.New(`--service is required for gRPC mocks

Usage: mockd add --type grpc --proto ./service.proto --service myapp.UserService --rpc-method GetUser [flags]

Run 'mockd add --help' for more options`)
	}

	if rpcMethod == "" {
		return nil, errors.New(`--rpc-method is required for gRPC mocks

Usage: mockd add --type grpc --proto ./service.proto --service myapp.UserService --rpc-method GetUser [flags]

Run 'mockd add --help' for more options`)
	}

	// Verify proto files exist
	for _, protoFile := range protoFiles {
		if _, err := os.Stat(protoFile); os.IsNotExist(err) {
			return nil, fmt.Errorf("proto file not found: %s", protoFile)
		}
	}

	grpcEnabled := true
	m := &config.MockConfiguration{
		Name:    name,
		Type:    mock.TypeGRPC,
		Enabled: &grpcEnabled,
		GRPC: &mock.GRPCSpec{
			Port:       port,
			Reflection: true, // Enable reflection for grpcurl/grpcui support
			Services:   make(map[string]mock.ServiceConfig),
		},
	}

	// Set proto file(s)
	if len(protoFiles) == 1 {
		m.GRPC.ProtoFile = protoFiles[0]
	} else {
		m.GRPC.ProtoFiles = protoFiles
	}

	// Set import paths if provided
	if len(protoPaths) > 0 {
		m.GRPC.ImportPaths = protoPaths
	}

	methodConfig := mock.MethodConfig{}

	// Parse response if provided
	if response != "" {
		var responseData interface{}
		if err := json.Unmarshal([]byte(response), &responseData); err != nil {
			return nil, fmt.Errorf("invalid JSON response: %w", err)
		}
		methodConfig.Response = responseData
	}

	m.GRPC.Services[service] = mock.ServiceConfig{
		Methods: map[string]mock.MethodConfig{
			rpcMethod: methodConfig,
		},
	}

	return m, nil
}

// buildMQTTMock creates an MQTT mock configuration.
func buildMQTTMock(name, topic, payload string, qos, port int) (*config.MockConfiguration, error) {
	if topic == "" {
		return nil, errors.New(`--topic is required for MQTT mocks

Usage: mockd add --type mqtt --topic sensors/temperature [flags]

Run 'mockd add --help' for more options`)
	}

	// Validate QoS
	if qos < 0 || qos > 2 {
		return nil, fmt.Errorf("invalid --qos: %d (must be 0, 1, or 2)", qos)
	}

	mqttEnabled := true
	m := &config.MockConfiguration{
		Name:    name,
		Type:    mock.TypeMQTT,
		Enabled: &mqttEnabled,
		MQTT: &mock.MQTTSpec{
			Port: port,
			Topics: []mock.TopicConfig{
				{
					Topic: topic,
					QoS:   qos,
				},
			},
		},
	}

	// Add payload as a message if provided
	if payload != "" {
		m.MQTT.Topics[0].Messages = []mock.MessageConfig{
			{
				Payload: payload,
			},
		}
	}

	return m, nil
}

// buildSOAPMock creates a SOAP mock configuration.
func buildSOAPMock(name, path, operation, soapAction, response string) (*config.MockConfiguration, error) {
	if operation == "" {
		return nil, errors.New(`--operation is required for SOAP mocks

Usage: mockd add --type soap --operation GetWeather [flags]

Run 'mockd add --help' for more options`)
	}

	// Default path
	if path == "" {
		path = "/soap"
	}

	soapEnabled := true
	m := &config.MockConfiguration{
		Name:    name,
		Type:    mock.TypeSOAP,
		Enabled: &soapEnabled,
		SOAP: &mock.SOAPSpec{
			Path:       path,
			Operations: make(map[string]mock.OperationConfig),
		},
	}

	opConfig := mock.OperationConfig{
		Response: response,
	}

	if soapAction != "" {
		opConfig.SOAPAction = soapAction
	}

	m.SOAP.Operations[operation] = opConfig

	return m, nil
}

// outputResult formats and prints the created or merged mock result.
func outputResult(result *CreateMockResult, mockType mock.Type, jsonOutput bool) error { //nolint:gocyclo // CLI output handler for all mock types
	if jsonOutput {
		return outputJSONResult(result, mockType)
	}

	created := result.Mock

	// Handle merge case
	if result.IsMerge() {
		fmt.Printf("Merged into mock: %s\n", created.ID)
		fmt.Printf("  Type: %s\n", created.Type)

		switch mockType { //nolint:exhaustive // only gRPC and MQTT have merge output
		case mock.TypeGRPC:
			if len(result.AddedServices) > 0 {
				fmt.Printf("  Added:\n")
				for _, svc := range result.AddedServices {
					fmt.Printf("    - %s\n", svc)
				}
			}
			if len(result.TotalServices) > 0 {
				fmt.Printf("  Total services:\n")
				for _, svc := range result.TotalServices {
					fmt.Printf("    - %s\n", svc)
				}
			}
		case mock.TypeMQTT:
			if len(result.AddedTopics) > 0 {
				fmt.Printf("  Added:\n")
				for _, topic := range result.AddedTopics {
					fmt.Printf("    - %s\n", topic)
				}
			}
			if len(result.TotalTopics) > 0 {
				fmt.Printf("  Total topics:\n")
				for _, topic := range result.TotalTopics {
					fmt.Printf("    - %s\n", topic)
				}
			}
		}
		return nil
	}

	// Standard create case
	fmt.Printf("Created mock: %s\n", created.ID)
	fmt.Printf("  Type: %s\n", created.Type)

	switch mockType { //nolint:exhaustive // OAuth not yet supported in CLI add output
	case mock.TypeHTTP:
		if created.HTTP != nil {
			if created.HTTP.Matcher != nil {
				fmt.Printf("  Method: %s\n", created.HTTP.Matcher.Method)
				fmt.Printf("  Path:   %s\n", created.HTTP.Matcher.Path)
			}
			if created.HTTP.Response != nil {
				fmt.Printf("  Status: %d\n", created.HTTP.Response.StatusCode)
			}
		}
	case mock.TypeWebSocket:
		if created.WebSocket != nil {
			fmt.Printf("  Path: %s\n", created.WebSocket.Path)
			if created.WebSocket.EchoMode != nil && *created.WebSocket.EchoMode {
				fmt.Printf("  Echo: enabled\n")
			}
		}
	case mock.TypeGraphQL:
		if created.GraphQL != nil {
			fmt.Printf("  Path: %s\n", created.GraphQL.Path)
			for op := range created.GraphQL.Resolvers {
				fmt.Printf("  Operation: %s\n", op)
			}
		}
	case mock.TypeGRPC:
		if created.GRPC != nil {
			for svc, cfg := range created.GRPC.Services {
				fmt.Printf("  Service: %s\n", svc)
				for method := range cfg.Methods {
					fmt.Printf("  Method: %s\n", method)
				}
			}
		}
	case mock.TypeMQTT:
		if created.MQTT != nil && len(created.MQTT.Topics) > 0 {
			fmt.Printf("  Topic: %s\n", created.MQTT.Topics[0].Topic)
			fmt.Printf("  QoS: %d\n", created.MQTT.Topics[0].QoS)
		}
	case mock.TypeSOAP:
		if created.SOAP != nil {
			fmt.Printf("  Path: %s\n", created.SOAP.Path)
			for op, cfg := range created.SOAP.Operations {
				fmt.Printf("  Operation: %s\n", op)
				if cfg.SOAPAction != "" {
					fmt.Printf("  SOAPAction: %s\n", cfg.SOAPAction)
				}
			}
		}
	}

	return nil
}

// outputJSONResult outputs the created or merged mock in JSON format.
func outputJSONResult(result *CreateMockResult, mockType mock.Type) error {
	created := result.Mock

	// For merge results, include merge-specific information
	if result.IsMerge() {
		switch mockType { //nolint:exhaustive // only gRPC and MQTT have merge JSON output
		case mock.TypeGRPC:
			return output.JSON(struct {
				ID            string   `json:"id"`
				Type          string   `json:"type"`
				Action        string   `json:"action"`
				AddedServices []string `json:"addedServices,omitempty"`
				TotalServices []string `json:"totalServices,omitempty"`
			}{
				ID:            created.ID,
				Type:          string(created.Type),
				Action:        result.Action,
				AddedServices: result.AddedServices,
				TotalServices: result.TotalServices,
			})
		case mock.TypeMQTT:
			return output.JSON(struct {
				ID          string   `json:"id"`
				Type        string   `json:"type"`
				Action      string   `json:"action"`
				AddedTopics []string `json:"addedTopics,omitempty"`
				TotalTopics []string `json:"totalTopics,omitempty"`
			}{
				ID:          created.ID,
				Type:        string(created.Type),
				Action:      result.Action,
				AddedTopics: result.AddedTopics,
				TotalTopics: result.TotalTopics,
			})
		}
	}

	// Standard create output
	switch mockType { //nolint:exhaustive // OAuth not yet supported in CLI JSON output
	case mock.TypeHTTP:
		createdMethod := ""
		createdPath := ""
		createdStatus := 0
		if created.HTTP != nil && created.HTTP.Matcher != nil {
			createdMethod = created.HTTP.Matcher.Method
			createdPath = created.HTTP.Matcher.Path
		}
		if created.HTTP != nil && created.HTTP.Response != nil {
			createdStatus = created.HTTP.Response.StatusCode
		}
		return output.JSON(struct {
			ID         string `json:"id"`
			Type       string `json:"type"`
			Action     string `json:"action"`
			Method     string `json:"method"`
			Path       string `json:"path"`
			StatusCode int    `json:"statusCode"`
		}{
			ID:         created.ID,
			Type:       string(created.Type),
			Action:     result.Action,
			Method:     createdMethod,
			Path:       createdPath,
			StatusCode: createdStatus,
		})

	case mock.TypeWebSocket:
		wsPath := ""
		echoEnabled := false
		if created.WebSocket != nil {
			wsPath = created.WebSocket.Path
			if created.WebSocket.EchoMode != nil {
				echoEnabled = *created.WebSocket.EchoMode
			}
		}
		return output.JSON(struct {
			ID     string `json:"id"`
			Type   string `json:"type"`
			Action string `json:"action"`
			Path   string `json:"path"`
			Echo   bool   `json:"echo"`
		}{
			ID:     created.ID,
			Type:   string(created.Type),
			Action: result.Action,
			Path:   wsPath,
			Echo:   echoEnabled,
		})

	case mock.TypeGraphQL:
		gqlPath := ""
		var operations []string
		if created.GraphQL != nil {
			gqlPath = created.GraphQL.Path
			for op := range created.GraphQL.Resolvers {
				operations = append(operations, op)
			}
		}
		return output.JSON(struct {
			ID         string   `json:"id"`
			Type       string   `json:"type"`
			Action     string   `json:"action"`
			Path       string   `json:"path"`
			Operations []string `json:"operations"`
		}{
			ID:         created.ID,
			Type:       string(created.Type),
			Action:     result.Action,
			Path:       gqlPath,
			Operations: operations,
		})

	case mock.TypeGRPC:
		var services []string
		var methods []string
		if created.GRPC != nil {
			for svc, cfg := range created.GRPC.Services {
				services = append(services, svc)
				for method := range cfg.Methods {
					methods = append(methods, method)
				}
			}
		}
		return output.JSON(struct {
			ID       string   `json:"id"`
			Type     string   `json:"type"`
			Action   string   `json:"action"`
			Services []string `json:"services"`
			Methods  []string `json:"methods"`
		}{
			ID:       created.ID,
			Type:     string(created.Type),
			Action:   result.Action,
			Services: services,
			Methods:  methods,
		})

	case mock.TypeMQTT:
		mqttTopic := ""
		mqttQoS := 0
		if created.MQTT != nil && len(created.MQTT.Topics) > 0 {
			mqttTopic = created.MQTT.Topics[0].Topic
			mqttQoS = created.MQTT.Topics[0].QoS
		}
		return output.JSON(struct {
			ID     string `json:"id"`
			Type   string `json:"type"`
			Action string `json:"action"`
			Topic  string `json:"topic"`
			QoS    int    `json:"qos"`
		}{
			ID:     created.ID,
			Type:   string(created.Type),
			Action: result.Action,
			Topic:  mqttTopic,
			QoS:    mqttQoS,
		})

	case mock.TypeSOAP:
		soapPath := ""
		var operations []string
		var soapActions []string
		if created.SOAP != nil {
			soapPath = created.SOAP.Path
			for op, cfg := range created.SOAP.Operations {
				operations = append(operations, op)
				if cfg.SOAPAction != "" {
					soapActions = append(soapActions, cfg.SOAPAction)
				}
			}
		}
		return output.JSON(struct {
			ID          string   `json:"id"`
			Type        string   `json:"type"`
			Action      string   `json:"action"`
			Path        string   `json:"path"`
			Operations  []string `json:"operations"`
			SOAPActions []string `json:"soapActions,omitempty"`
		}{
			ID:          created.ID,
			Type:        string(created.Type),
			Action:      result.Action,
			Path:        soapPath,
			Operations:  operations,
			SOAPActions: soapActions,
		})
	}

	// Fallback for unknown types
	return output.JSON(created)
}
