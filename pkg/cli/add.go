package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/getmockd/mockd/pkg/cli/internal/flags"
	"github.com/getmockd/mockd/pkg/cli/internal/output"
	"github.com/getmockd/mockd/pkg/cli/internal/parse"
	"github.com/getmockd/mockd/pkg/config"
	"github.com/getmockd/mockd/pkg/mock"
	"github.com/spf13/cobra"
)

var (
	addMockType       string
	addName           string
	addAllowDuplicate bool
	addStateful       string

	addMethod       string
	addPath         string
	addStatus       int
	addBody         string
	addBodyFile     string
	addHeaders      flags.StringSlice
	addMatchHeaders flags.StringSlice
	addMatchQueries flags.StringSlice
	addBodyContains string
	addPathPattern  string
	addPriority     int
	addDelay        int

	addSSE          bool
	addSSEEvents    flags.StringSlice
	addSSEDelay     int
	addSSETemplate  string
	addSSERepeat    int
	addSSEKeepalive int

	addMessage string
	addEcho    bool

	addOperation string
	addOpType    string
	addResponse  string

	addService    string
	addRPCMethod  string
	addGRPCPort   int
	addProtoFiles flags.StringSlice
	addProtoPaths flags.StringSlice

	addTopic    string
	addPayload  string
	addQoS      int
	addMQTTPort int

	addSoapAction        string
	addActionAlias       string
	addMutation          bool
	addStatefulOperation string

	addIssuer        string
	addClientID      string
	addClientSecret  string
	addOAuthUser     string
	addOAuthPassword string
)

var addCmd = &cobra.Command{
	Use:   "add [type]",
	Short: "Add a new mock endpoint",
	Long: `Add a new mock endpoint.

The mock type can be specified as a positional argument, a flag, or via
the protocol subcommands:

  mockd add http --path /api/hello          # positional type
  mockd add --type http --path /api/hello   # flag type
  mockd http add --path /api/hello          # subcommand

Valid types: http, websocket, graphql, grpc, mqtt, soap, oauth`,
	Args: cobra.MaximumNArgs(1),
	RunE: runAdd,
}

func init() {
	rootCmd.AddCommand(addCmd)

	addCmd.Flags().StringVarP(&addMockType, "type", "t", "http", "Mock type (http, websocket, graphql, grpc, mqtt, soap, oauth)")
	addCmd.Flags().StringVarP(&addName, "name", "n", "", "Mock display name")
	addCmd.Flags().BoolVar(&addAllowDuplicate, "allow-duplicate", false, "Create a new mock even if one already exists on the same path")
	addCmd.Flags().StringVar(&addStateful, "stateful", "", "Create a stateful CRUD resource (value is the resource name, e.g., users)")

	addCmd.Flags().StringVarP(&addMethod, "method", "m", "GET", "HTTP method to match")
	addCmd.Flags().StringVar(&addPath, "path", "", "URL path to match (required for http, websocket, graphql, soap)")
	addCmd.Flags().IntVarP(&addStatus, "status", "s", 200, "Response status code")
	addCmd.Flags().StringVarP(&addBody, "body", "b", "", "Response body")
	addCmd.Flags().StringVar(&addBodyFile, "body-file", "", "Read response body from file")
	addCmd.Flags().VarP(&addHeaders, "header", "H", "Response header (key:value), repeatable")
	addCmd.Flags().Var(&addMatchHeaders, "match-header", "Required request header (key:value), repeatable")
	addCmd.Flags().Var(&addMatchQueries, "match-query", "Required query param (key=value or key:value), repeatable")
	addCmd.Flags().StringVar(&addBodyContains, "match-body-contains", "", "Match requests whose body contains this string")
	addCmd.Flags().StringVar(&addPathPattern, "path-pattern", "", "Regex path pattern for matching (alternative to --path)")
	addCmd.Flags().IntVar(&addPriority, "priority", 0, "Mock priority (higher = matched first)")
	addCmd.Flags().IntVar(&addDelay, "delay", 0, "Response delay in milliseconds")

	addCmd.Flags().BoolVar(&addSSE, "sse", false, "Enable Server-Sent Events streaming")
	addCmd.Flags().Var(&addSSEEvents, "sse-event", "SSE event (type:data or just data), repeatable")
	addCmd.Flags().IntVar(&addSSEDelay, "sse-delay", 100, "Delay between SSE events in milliseconds")
	addCmd.Flags().StringVar(&addSSETemplate, "sse-template", "", "SSE template: openai, notification")
	addCmd.Flags().IntVar(&addSSERepeat, "sse-repeat", 1, "Number of times to repeat SSE events (0 = infinite)")
	addCmd.Flags().IntVar(&addSSEKeepalive, "sse-keepalive", 0, "SSE keepalive interval in seconds (0 = disabled, min 5)")

	addCmd.Flags().StringVar(&addMessage, "message", "", "Default response message (JSON) for WebSocket")
	addCmd.Flags().BoolVar(&addEcho, "echo", false, "Enable echo mode for WebSocket")

	addCmd.Flags().StringVar(&addOperation, "operation", "", "Operation name (required for graphql, soap)")
	addCmd.Flags().StringVar(&addActionAlias, "action", "", "SOAP action name (alias for --operation)")
	addCmd.Flags().StringVar(&addOpType, "op-type", "query", "GraphQL operation type (query/mutation)")
	addCmd.Flags().BoolVar(&addMutation, "mutation", false, "Create a Mutation resolver instead of Query (GraphQL)")
	addCmd.Flags().StringVar(&addResponse, "response", "", "JSON response data (for graphql, grpc)")

	addCmd.Flags().StringVar(&addService, "service", "", "gRPC service name (e.g., greeter.Greeter)")
	addCmd.Flags().StringVar(&addRPCMethod, "rpc-method", "", "gRPC method name")
	addCmd.Flags().IntVar(&addGRPCPort, "grpc-port", 50051, "gRPC server port")
	addCmd.Flags().Var(&addProtoFiles, "proto", "Path to .proto file (required for gRPC, repeatable)")
	addCmd.Flags().Var(&addProtoPaths, "proto-path", "Import path for proto dependencies (repeatable)")

	addCmd.Flags().StringVar(&addTopic, "topic", "", "MQTT topic pattern")
	addCmd.Flags().StringVar(&addPayload, "payload", "", "MQTT response payload")
	addCmd.Flags().IntVar(&addQoS, "qos", 0, "MQTT QoS level (0, 1, 2)")
	addCmd.Flags().IntVar(&addMQTTPort, "mqtt-port", 1883, "MQTT broker port")

	addCmd.Flags().StringVar(&addStatefulOperation, "stateful-operation", "", "Wire to a custom stateful operation (e.g., TransferFunds)")

	addCmd.Flags().StringVar(&addSoapAction, "soap-action", "", "SOAPAction header value")

	addCmd.Flags().StringVar(&addIssuer, "issuer", "", "OAuth issuer URL (default: http://localhost:4280)")
	addCmd.Flags().StringVar(&addClientID, "client-id", "test-client", "OAuth client ID")
	addCmd.Flags().StringVar(&addClientSecret, "client-secret", "test-secret", "OAuth client secret")
	addCmd.Flags().StringVar(&addOAuthUser, "oauth-user", "testuser", "OAuth test username")
	addCmd.Flags().StringVar(&addOAuthPassword, "oauth-password", "password", "OAuth test password")
}

func runAdd(cmd *cobra.Command, args []string) error {
	// Accept positional type: "mockd add http --path ..."
	// Only override if --type wasn't explicitly set by the user
	if len(args) == 1 && !cmd.Flags().Changed("type") {
		addMockType = args[0]
	}

	// Resolve flag aliases:
	// --action is an alias for --operation (SOAP compatibility)
	if addActionAlias != "" && addOperation == "" {
		addOperation = addActionAlias
	}
	// --mutation sets op-type to "mutation" (GraphQL shorthand)
	if addMutation {
		addOpType = "mutation"
	}

	// Normalize mock type
	mt := strings.ToLower(addMockType)

	// Validate mock type
	validTypes := map[string]mock.Type{
		"http":      mock.TypeHTTP,
		"websocket": mock.TypeWebSocket,
		"graphql":   mock.TypeGraphQL,
		"grpc":      mock.TypeGRPC,
		"mqtt":      mock.TypeMQTT,
		"soap":      mock.TypeSOAP,
		"oauth":     mock.TypeOAuth,
	}

	mockTypeEnum, ok := validTypes[mt]
	if !ok {
		return fmt.Errorf("invalid mock type: %s\n\nValid types: http, websocket, graphql, grpc, mqtt, soap, oauth", addMockType)
	}

	// Handle --stateful: create a stateful CRUD resource instead of a mock
	if addStateful != "" {
		return runAddStateful(addStateful, addPath)
	}

	// Mutual exclusivity: --path and --path-pattern
	if addPath != "" && addPathPattern != "" {
		return errors.New("--path and --path-pattern are mutually exclusive")
	}

	// Build mock configuration based on type
	var m *config.MockConfiguration
	var err error

	switch mockTypeEnum {
	case mock.TypeHTTP:
		m, err = buildHTTPMock(addName, addPath, addMethod, addStatus, addBody, addBodyFile, addPriority, addDelay, addHeaders, addMatchHeaders, addMatchQueries,
			addSSE, addSSEEvents, addSSEDelay, addSSETemplate, addSSERepeat, addSSEKeepalive, addBodyContains, addPathPattern, addStatefulOperation)
	case mock.TypeWebSocket:
		m, err = buildWebSocketMock(addName, addPath, addMessage, addEcho)
	case mock.TypeGraphQL:
		m, err = buildGraphQLMock(addName, addPath, addOperation, addOpType, addResponse)
	case mock.TypeGRPC:
		m, err = buildGRPCMock(addName, addService, addRPCMethod, addResponse, addGRPCPort, addProtoFiles, addProtoPaths)
	case mock.TypeMQTT:
		m, err = buildMQTTMock(addName, addTopic, addPayload, addQoS, addMQTTPort)
	case mock.TypeSOAP:
		m, err = buildSOAPMock(addName, addPath, addOperation, addSoapAction, addResponse, soapAddStatefulRes, soapAddStatefulAction)
	case mock.TypeOAuth:
		m = buildOAuthMock(addName, addIssuer, addClientID, addClientSecret, addOAuthUser, addOAuthPassword)
	}

	if err != nil {
		return err
	}

	// Create admin client
	client := NewAdminClientWithAuth(adminURL)

	// For HTTP mocks: check for existing mock on same method+path (upsert behavior)
	if mockTypeEnum == mock.TypeHTTP && !addAllowDuplicate && m.HTTP != nil && m.HTTP.Matcher != nil {
		existingID, err := findExistingHTTPMock(client, m.HTTP.Matcher.Method, m.HTTP.Matcher.Path)
		if err != nil {
			// Connection error — fall through to create which will give a better error
			return fmt.Errorf("%s", FormatConnectionError(err))
		}
		if existingID != "" {
			// Update existing mock instead of creating a duplicate
			updated, err := client.UpdateMock(existingID, m)
			if err != nil {
				return fmt.Errorf("failed to update mock: %s", FormatConnectionError(err))
			}
			return outputResult(&CreateMockResult{
				Mock:   updated,
				Action: "updated",
			}, mockTypeEnum, jsonOutput)
		}
	}

	// For SOAP mocks: merge operations into existing mock on same path
	if mockTypeEnum == mock.TypeSOAP && !addAllowDuplicate && m.SOAP != nil {
		result, merged, err := mergeSOAPIfExists(client, m)
		if err != nil {
			return fmt.Errorf("%s", FormatConnectionError(err))
		}
		if merged {
			return outputResult(result, mockTypeEnum, jsonOutput)
		}
	}

	// Create new mock
	result, err := client.CreateMock(m)
	if err != nil {
		return fmt.Errorf("%s", FormatConnectionError(err))
	}

	// Output result based on mock type and action (created vs merged)
	return outputResult(result, mockTypeEnum, jsonOutput)
}

// findExistingHTTPMock looks for an existing mock with the same method+path.
// Returns the mock ID if found, empty string if not.
func findExistingHTTPMock(client AdminClient, method, path string) (string, error) {
	mocks, err := client.ListMocks()
	if err != nil {
		return "", err
	}

	for _, m := range mocks {
		if m.Type != mock.TypeHTTP && m.Type != "" {
			continue
		}
		if m.HTTP == nil || m.HTTP.Matcher == nil {
			continue
		}
		if m.HTTP.Matcher.Method == method && m.HTTP.Matcher.Path == path {
			return m.ID, nil
		}
	}
	return "", nil
}

// findExistingSOAPMock looks for an existing SOAP mock on the same path.
// Returns the mock ID if found, empty string if not.
func findExistingSOAPMock(client AdminClient, path string) (string, error) {
	mocks, err := client.ListMocks()
	if err != nil {
		return "", err
	}

	for _, m := range mocks {
		if m.Type != mock.TypeSOAP {
			continue
		}
		if m.SOAP != nil && m.SOAP.Path == path {
			return m.ID, nil
		}
	}
	return "", nil
}

// mergeSOAPIfExists merges new operations into an existing SOAP mock on the same path.
// Returns (result, true, nil) if merged, (nil, false, nil) if no existing mock found.
func mergeSOAPIfExists(client AdminClient, newMock *config.MockConfiguration) (*CreateMockResult, bool, error) {
	existingID, err := findExistingSOAPMock(client, newMock.SOAP.Path)
	if err != nil {
		return nil, false, err
	}
	if existingID == "" {
		return nil, false, nil
	}

	// Fetch the full existing mock to get its operations
	existing, err := client.GetMock(existingID)
	if err != nil {
		return nil, false, fmt.Errorf("failed to get existing SOAP mock: %w", err)
	}

	if existing.SOAP == nil {
		return nil, false, nil
	}
	if existing.SOAP.Operations == nil {
		existing.SOAP.Operations = make(map[string]mock.OperationConfig)
	}

	// Merge new operations into the existing mock
	addedOps := []string{}
	for opName, opConfig := range newMock.SOAP.Operations {
		if _, exists := existing.SOAP.Operations[opName]; !exists {
			addedOps = append(addedOps, opName)
		}
		existing.SOAP.Operations[opName] = opConfig
	}

	// Collect all operation names after merge
	totalOps := make([]string, 0, len(existing.SOAP.Operations))
	for opName := range existing.SOAP.Operations {
		totalOps = append(totalOps, opName)
	}

	// Update the existing mock with merged operations
	updated, err := client.UpdateMock(existingID, existing)
	if err != nil {
		return nil, false, fmt.Errorf("failed to update mock: %w", err)
	}

	result := &CreateMockResult{
		Mock:            updated,
		Action:          "merged",
		Message:         fmt.Sprintf("Merged %d operation(s) into existing SOAP mock %s", len(addedOps), existingID),
		AddedOperations: addedOps,
		TotalOperations: totalOps,
	}

	return result, true, nil
}

// buildHTTPMock creates an HTTP mock configuration.
func buildHTTPMock(name, path, method string, status int, body, bodyFile string, priority, delay int,
	headers, matchHeaders, matchQueries flags.StringSlice,
	sse bool, sseEvents flags.StringSlice, sseDelay int, sseTemplate string, sseRepeat, sseKeepalive int,
	bodyContains, pathPattern, statefulOperation string) (*config.MockConfiguration, error) {

	if path == "" && pathPattern == "" {
		return nil, errors.New(`--path or --path-pattern is required for HTTP mocks

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

	// Parse match query params — accept both key=value and key:value
	matchQueryMap := make(map[string]string)
	for _, q := range matchQueries {
		key, value, ok := parse.KeyValue(q, '=', ':')
		if !ok {
			return nil, fmt.Errorf("invalid match-query format: %s (expected key=value or key:value)", q)
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

	// Handle stateful operation: mutually exclusive with response/sse
	if statefulOperation != "" {
		if sse || len(sseEvents) > 0 || sseTemplate != "" {
			return nil, errors.New("--stateful-operation and --sse are mutually exclusive")
		}
		if body != "" || bodyFile != "" {
			return nil, errors.New("--stateful-operation and --body/--body-file are mutually exclusive (the operation result becomes the response)")
		}
		m.HTTP.StatefulOperation = statefulOperation
	} else if sse || len(sseEvents) > 0 || sseTemplate != "" {
		// Build SSE config if enabled
		sseConfig, err := buildSSEConfig(sseEvents, sseDelay, sseTemplate, sseRepeat, sseKeepalive)
		if err != nil {
			return nil, err
		}
		m.HTTP.SSE = sseConfig
	} else {
		// Only set Response for non-SSE, non-stateful-operation mocks
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
	if bodyContains != "" {
		m.HTTP.Matcher.BodyContains = bodyContains
	}
	if pathPattern != "" {
		m.HTTP.Matcher.PathPattern = pathPattern
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
func buildSOAPMock(name, path, operation, soapAction, response, statefulResource, statefulAction string) (*config.MockConfiguration, error) {
	if operation == "" {
		return nil, errors.New(`--operation is required for SOAP mocks

Usage: mockd add --type soap --operation GetWeather [flags]

Run 'mockd add --help' for more options`)
	}

	// Validate stateful flags: both or neither
	if (statefulResource != "") != (statefulAction != "") {
		return nil, errors.New("--stateful-resource and --stateful-action must be used together")
	}

	// Validate stateful action values
	if statefulAction != "" {
		validActions := map[string]bool{
			"list": true, "get": true, "create": true,
			"update": true, "delete": true, "custom": true,
		}
		if !validActions[statefulAction] {
			return nil, fmt.Errorf("invalid --stateful-action %q: must be one of list, get, create, update, delete, custom", statefulAction)
		}
	}

	// When stateful fields are set, response is optional
	if statefulResource == "" && response == "" {
		// No stateful and no response — default response is fine (empty string)
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

	if statefulResource != "" {
		opConfig.StatefulResource = statefulResource
		opConfig.StatefulAction = statefulAction
	}

	m.SOAP.Operations[operation] = opConfig

	return m, nil
}

// buildOAuthMock creates an OAuth/OIDC mock configuration.
func buildOAuthMock(name, issuer, clientID, clientSecret, username, password string) *config.MockConfiguration {
	// Default issuer
	if issuer == "" {
		issuer = "http://localhost:4280"
	}

	// Default name
	if name == "" {
		name = "OAuth Mock"
	}

	oauthEnabled := true
	m := &config.MockConfiguration{
		Name:    name,
		Type:    mock.TypeOAuth,
		Enabled: &oauthEnabled,
		OAuth: &mock.OAuthSpec{
			Issuer:        issuer,
			TokenExpiry:   "1h",
			RefreshExpiry: "7d",
			DefaultScopes: []string{"openid", "profile", "email"},
			Clients: []mock.OAuthClient{
				{
					ClientID:     clientID,
					ClientSecret: clientSecret,
					RedirectURIs: []string{"http://localhost:3000/callback"},
					GrantTypes:   []string{"authorization_code", "client_credentials", "refresh_token", "password"},
				},
			},
			Users: []mock.OAuthUser{
				{
					Username: username,
					Password: password,
					Claims: map[string]interface{}{
						"sub":   username,
						"email": username + "@example.com",
						"name":  username,
					},
				},
			},
		},
	}

	return m
}

// runAddStateful creates a stateful CRUD resource via the admin API.
func runAddStateful(name, basePath string) error {
	if basePath == "" {
		// Default basePath from name: "users" -> "/api/users"
		basePath = "/api/" + name
	}

	// Build a config with just the stateful resource
	collection := &config.MockCollection{
		Version: "1.0",
		Mocks:   []*config.MockConfiguration{},
		StatefulResources: []*config.StatefulResourceConfig{
			{
				Name:     name,
				BasePath: basePath,
			},
		},
	}

	client := NewAdminClientWithAuth(adminURL)
	_, err := client.ImportConfig(collection, false)
	if err != nil {
		return fmt.Errorf("%s", FormatConnectionError(err))
	}

	if jsonOutput {
		return output.JSON(struct {
			Resource  string   `json:"resource"`
			BasePath  string   `json:"basePath"`
			Action    string   `json:"action"`
			Endpoints []string `json:"endpoints"`
		}{
			Resource: name,
			BasePath: basePath,
			Action:   "created",
			Endpoints: []string{
				"GET    " + basePath,
				"POST   " + basePath,
				"GET    " + basePath + "/{id}",
				"PUT    " + basePath + "/{id}",
				"DELETE " + basePath + "/{id}",
			},
		})
	}

	fmt.Printf("Created stateful resource: %s\n", name)
	fmt.Printf("  Base path: %s\n", basePath)
	fmt.Printf("  Endpoints:\n")
	fmt.Printf("    GET    %s        — List all %s\n", basePath, name)
	fmt.Printf("    POST   %s        — Create a %s\n", basePath, singularize(name))
	fmt.Printf("    GET    %s/{id}   — Get a %s by ID\n", basePath, singularize(name))
	fmt.Printf("    PUT    %s/{id}   — Update a %s\n", basePath, singularize(name))
	fmt.Printf("    DELETE %s/{id}   — Delete a %s\n", basePath, singularize(name))
	return nil
}

// singularize does a naive singularization (removes trailing "s").
func singularize(name string) string {
	if strings.HasSuffix(name, "ies") {
		return name[:len(name)-3] + "y"
	}
	if strings.HasSuffix(name, "ses") || strings.HasSuffix(name, "xes") || strings.HasSuffix(name, "zes") {
		return name[:len(name)-2]
	}
	if strings.HasSuffix(name, "s") && !strings.HasSuffix(name, "ss") {
		return name[:len(name)-1]
	}
	return name
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

		switch mockType { //nolint:exhaustive // only gRPC, MQTT, and SOAP have merge output
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
		case mock.TypeSOAP:
			if len(result.AddedOperations) > 0 {
				fmt.Printf("  Added:\n")
				for _, op := range result.AddedOperations {
					fmt.Printf("    - %s\n", op)
				}
			}
			if len(result.TotalOperations) > 0 {
				fmt.Printf("  Total operations:\n")
				for _, op := range result.TotalOperations {
					fmt.Printf("    - %s\n", op)
				}
			}
		}
		return nil
	}

	// Standard create/update case
	if result.Action == "updated" {
		fmt.Printf("Updated mock: %s\n", created.ID)
	} else {
		fmt.Printf("Created mock: %s\n", created.ID)
	}
	fmt.Printf("  Type: %s\n", created.Type)

	switch mockType { //nolint:exhaustive // only protocol types with specific output
	case mock.TypeHTTP:
		if created.HTTP != nil {
			if created.HTTP.Matcher != nil {
				fmt.Printf("  Method: %s\n", created.HTTP.Matcher.Method)
				fmt.Printf("  Path:   %s\n", created.HTTP.Matcher.Path)
			}
			if created.HTTP.StatefulOperation != "" {
				fmt.Printf("  Operation: %s\n", created.HTTP.StatefulOperation)
			} else if created.HTTP.Response != nil {
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
	case mock.TypeOAuth:
		if created.OAuth != nil {
			fmt.Printf("  Issuer: %s\n", created.OAuth.Issuer)
			if len(created.OAuth.Clients) > 0 {
				fmt.Printf("  Client ID: %s\n", created.OAuth.Clients[0].ClientID)
			}
			if len(created.OAuth.Users) > 0 {
				fmt.Printf("  User: %s\n", created.OAuth.Users[0].Username)
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
		switch mockType { //nolint:exhaustive // only gRPC, MQTT, and SOAP have merge JSON output
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
		case mock.TypeSOAP:
			return output.JSON(struct {
				ID              string   `json:"id"`
				Type            string   `json:"type"`
				Action          string   `json:"action"`
				Path            string   `json:"path"`
				AddedOperations []string `json:"addedOperations,omitempty"`
				TotalOperations []string `json:"totalOperations,omitempty"`
			}{
				ID:              created.ID,
				Type:            string(created.Type),
				Action:          result.Action,
				Path:            created.SOAP.Path,
				AddedOperations: result.AddedOperations,
				TotalOperations: result.TotalOperations,
			})
		}
	}

	// Standard create output
	switch mockType { //nolint:exhaustive // only protocol types with specific JSON output
	case mock.TypeHTTP:
		createdMethod := ""
		createdPath := ""
		createdStatus := 0
		createdOperation := ""
		if created.HTTP != nil && created.HTTP.Matcher != nil {
			createdMethod = created.HTTP.Matcher.Method
			createdPath = created.HTTP.Matcher.Path
		}
		if created.HTTP != nil && created.HTTP.Response != nil {
			createdStatus = created.HTTP.Response.StatusCode
		}
		if created.HTTP != nil {
			createdOperation = created.HTTP.StatefulOperation
		}
		return output.JSON(struct {
			ID                string `json:"id"`
			Type              string `json:"type"`
			Action            string `json:"action"`
			Method            string `json:"method"`
			Path              string `json:"path"`
			StatusCode        int    `json:"statusCode,omitempty"`
			StatefulOperation string `json:"statefulOperation,omitempty"`
		}{
			ID:                created.ID,
			Type:              string(created.Type),
			Action:            result.Action,
			Method:            createdMethod,
			Path:              createdPath,
			StatusCode:        createdStatus,
			StatefulOperation: createdOperation,
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

	case mock.TypeOAuth:
		oauthIssuer := ""
		oauthClientID := ""
		oauthUsername := ""
		if created.OAuth != nil {
			oauthIssuer = created.OAuth.Issuer
			if len(created.OAuth.Clients) > 0 {
				oauthClientID = created.OAuth.Clients[0].ClientID
			}
			if len(created.OAuth.Users) > 0 {
				oauthUsername = created.OAuth.Users[0].Username
			}
		}
		return output.JSON(struct {
			ID       string `json:"id"`
			Type     string `json:"type"`
			Action   string `json:"action"`
			Issuer   string `json:"issuer"`
			ClientID string `json:"clientId,omitempty"`
			Username string `json:"username,omitempty"`
		}{
			ID:       created.ID,
			Type:     string(created.Type),
			Action:   result.Action,
			Issuer:   oauthIssuer,
			ClientID: oauthClientID,
			Username: oauthUsername,
		})
	}

	// Fallback for unknown types
	return output.JSON(created)
}
