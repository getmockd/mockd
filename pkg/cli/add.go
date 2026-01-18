package cli

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/getmockd/mockd/internal/cliconfig"
	"github.com/getmockd/mockd/pkg/cli/internal/flags"
	"github.com/getmockd/mockd/pkg/cli/internal/parse"
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

	// MQTT flags
	topic := fs.String("topic", "", "MQTT topic pattern")
	payload := fs.String("payload", "", "MQTT response payload")
	qos := fs.Int("qos", 0, "MQTT QoS level (0, 1, 2)")

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

WebSocket Flags (--type websocket):
      --path          WebSocket path (required)
      --message       Default response message (JSON)
      --echo          Enable echo mode

GraphQL Flags (--type graphql):
      --path          GraphQL endpoint path (default: /graphql)
      --operation     Operation name (required)
      --op-type       Operation type: query or mutation (default: query)
      --response      JSON response data

gRPC Flags (--type grpc):
      --service       Service name, e.g., greeter.Greeter (required)
      --rpc-method    RPC method name (required)
      --response      JSON response data

MQTT Flags (--type mqtt):
      --topic         Topic pattern (required)
      --payload       Response payload
      --qos           QoS level: 0, 1, or 2 (default: 0)

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

  # gRPC mock
  mockd add --type grpc --service greeter.Greeter --rpc-method SayHello \
    --response '{"message": "Hello, World!"}'

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
	validTypes := map[string]mock.MockType{
		"http":      mock.MockTypeHTTP,
		"websocket": mock.MockTypeWebSocket,
		"graphql":   mock.MockTypeGraphQL,
		"grpc":      mock.MockTypeGRPC,
		"mqtt":      mock.MockTypeMQTT,
		"soap":      mock.MockTypeSOAP,
	}

	mockTypeEnum, ok := validTypes[mt]
	if !ok {
		return fmt.Errorf("invalid mock type: %s\n\nValid types: http, websocket, graphql, grpc, mqtt, soap", *mockType)
	}

	// Build mock configuration based on type
	var m *config.MockConfiguration
	var err error

	switch mockTypeEnum {
	case mock.MockTypeHTTP:
		m, err = buildHTTPMock(*name, *path, *method, *status, *body, *bodyFile, *priority, *delay, headers, matchHeaders, matchQueries)
	case mock.MockTypeWebSocket:
		m, err = buildWebSocketMock(*name, *path, *message, *echo)
	case mock.MockTypeGraphQL:
		m, err = buildGraphQLMock(*name, *path, *operation, *opType, *response)
	case mock.MockTypeGRPC:
		m, err = buildGRPCMock(*name, *service, *rpcMethod, *response)
	case mock.MockTypeMQTT:
		m, err = buildMQTTMock(*name, *topic, *payload, *qos)
	case mock.MockTypeSOAP:
		m, err = buildSOAPMock(*name, *path, *operation, *soapAction, *response)
	}

	if err != nil {
		return err
	}

	// Create admin client and add mock
	client := NewAdminClient(*adminURL)
	created, err := client.CreateMock(m)
	if err != nil {
		return fmt.Errorf("%s", FormatConnectionError(err))
	}

	// Output result based on mock type
	return outputResult(created, mockTypeEnum, *jsonOutput)
}

// buildHTTPMock creates an HTTP mock configuration.
func buildHTTPMock(name, path, method string, status int, body, bodyFile string, priority, delay int, headers, matchHeaders, matchQueries flags.StringSlice) (*config.MockConfiguration, error) {
	if path == "" {
		return nil, fmt.Errorf(`--path is required for HTTP mocks

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

	m := &config.MockConfiguration{
		Name:    name,
		Type:    mock.MockTypeHTTP,
		Enabled: true,
		HTTP: &mock.HTTPSpec{
			Priority: priority,
			Matcher: &mock.HTTPMatcher{
				Method: strings.ToUpper(method),
				Path:   path,
			},
			Response: &mock.HTTPResponse{
				StatusCode: status,
				Body:       responseBody,
				DelayMs:    delay,
			},
		},
	}

	if len(matchHeadersMap) > 0 {
		m.HTTP.Matcher.Headers = matchHeadersMap
	}
	if len(matchQueryMap) > 0 {
		m.HTTP.Matcher.QueryParams = matchQueryMap
	}
	if len(responseHeaders) > 0 {
		m.HTTP.Response.Headers = responseHeaders
	}

	return m, nil
}

// buildWebSocketMock creates a WebSocket mock configuration.
func buildWebSocketMock(name, path, message string, echo bool) (*config.MockConfiguration, error) {
	if path == "" {
		return nil, fmt.Errorf(`--path is required for WebSocket mocks

Usage: mockd add --type websocket --path /ws/endpoint [flags]

Run 'mockd add --help' for more options`)
	}

	m := &config.MockConfiguration{
		Name:    name,
		Type:    mock.MockTypeWebSocket,
		Enabled: true,
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
		return nil, fmt.Errorf(`--operation is required for GraphQL mocks

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

	m := &config.MockConfiguration{
		Name:    name,
		Type:    mock.MockTypeGraphQL,
		Enabled: true,
		GraphQL: &mock.GraphQLSpec{
			Path:      path,
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

// buildGRPCMock creates a gRPC mock configuration.
func buildGRPCMock(name, service, rpcMethod, response string) (*config.MockConfiguration, error) {
	if service == "" {
		return nil, fmt.Errorf(`--service is required for gRPC mocks

Usage: mockd add --type grpc --service greeter.Greeter --rpc-method SayHello [flags]

Run 'mockd add --help' for more options`)
	}

	if rpcMethod == "" {
		return nil, fmt.Errorf(`--rpc-method is required for gRPC mocks

Usage: mockd add --type grpc --service greeter.Greeter --rpc-method SayHello [flags]

Run 'mockd add --help' for more options`)
	}

	m := &config.MockConfiguration{
		Name:    name,
		Type:    mock.MockTypeGRPC,
		Enabled: true,
		GRPC: &mock.GRPCSpec{
			Services: make(map[string]mock.ServiceConfig),
		},
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
func buildMQTTMock(name, topic, payload string, qos int) (*config.MockConfiguration, error) {
	if topic == "" {
		return nil, fmt.Errorf(`--topic is required for MQTT mocks

Usage: mockd add --type mqtt --topic sensors/temperature [flags]

Run 'mockd add --help' for more options`)
	}

	// Validate QoS
	if qos < 0 || qos > 2 {
		return nil, fmt.Errorf("invalid --qos: %d (must be 0, 1, or 2)", qos)
	}

	m := &config.MockConfiguration{
		Name:    name,
		Type:    mock.MockTypeMQTT,
		Enabled: true,
		MQTT: &mock.MQTTSpec{
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
		return nil, fmt.Errorf(`--operation is required for SOAP mocks

Usage: mockd add --type soap --operation GetWeather [flags]

Run 'mockd add --help' for more options`)
	}

	// Default path
	if path == "" {
		path = "/soap"
	}

	m := &config.MockConfiguration{
		Name:    name,
		Type:    mock.MockTypeSOAP,
		Enabled: true,
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

// outputResult formats and prints the created mock result.
func outputResult(created *config.MockConfiguration, mockType mock.MockType, jsonOutput bool) error {
	if jsonOutput {
		return outputJSONResult(created, mockType)
	}

	fmt.Printf("Created mock: %s\n", created.ID)
	fmt.Printf("  Type: %s\n", created.Type)

	switch mockType {
	case mock.MockTypeHTTP:
		if created.HTTP != nil {
			if created.HTTP.Matcher != nil {
				fmt.Printf("  Method: %s\n", created.HTTP.Matcher.Method)
				fmt.Printf("  Path:   %s\n", created.HTTP.Matcher.Path)
			}
			if created.HTTP.Response != nil {
				fmt.Printf("  Status: %d\n", created.HTTP.Response.StatusCode)
			}
		}
	case mock.MockTypeWebSocket:
		if created.WebSocket != nil {
			fmt.Printf("  Path: %s\n", created.WebSocket.Path)
			if created.WebSocket.EchoMode != nil && *created.WebSocket.EchoMode {
				fmt.Printf("  Echo: enabled\n")
			}
		}
	case mock.MockTypeGraphQL:
		if created.GraphQL != nil {
			fmt.Printf("  Path: %s\n", created.GraphQL.Path)
			for op := range created.GraphQL.Resolvers {
				fmt.Printf("  Operation: %s\n", op)
			}
		}
	case mock.MockTypeGRPC:
		if created.GRPC != nil {
			for svc, cfg := range created.GRPC.Services {
				fmt.Printf("  Service: %s\n", svc)
				for method := range cfg.Methods {
					fmt.Printf("  Method: %s\n", method)
				}
			}
		}
	case mock.MockTypeMQTT:
		if created.MQTT != nil && len(created.MQTT.Topics) > 0 {
			fmt.Printf("  Topic: %s\n", created.MQTT.Topics[0].Topic)
			fmt.Printf("  QoS: %d\n", created.MQTT.Topics[0].QoS)
		}
	case mock.MockTypeSOAP:
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

// outputJSONResult outputs the created mock in JSON format.
func outputJSONResult(created *config.MockConfiguration, mockType mock.MockType) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")

	switch mockType {
	case mock.MockTypeHTTP:
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
		return enc.Encode(struct {
			ID         string `json:"id"`
			Type       string `json:"type"`
			Method     string `json:"method"`
			Path       string `json:"path"`
			StatusCode int    `json:"statusCode"`
		}{
			ID:         created.ID,
			Type:       string(created.Type),
			Method:     createdMethod,
			Path:       createdPath,
			StatusCode: createdStatus,
		})

	case mock.MockTypeWebSocket:
		wsPath := ""
		echoEnabled := false
		if created.WebSocket != nil {
			wsPath = created.WebSocket.Path
			if created.WebSocket.EchoMode != nil {
				echoEnabled = *created.WebSocket.EchoMode
			}
		}
		return enc.Encode(struct {
			ID   string `json:"id"`
			Type string `json:"type"`
			Path string `json:"path"`
			Echo bool   `json:"echo"`
		}{
			ID:   created.ID,
			Type: string(created.Type),
			Path: wsPath,
			Echo: echoEnabled,
		})

	case mock.MockTypeGraphQL:
		gqlPath := ""
		var operations []string
		if created.GraphQL != nil {
			gqlPath = created.GraphQL.Path
			for op := range created.GraphQL.Resolvers {
				operations = append(operations, op)
			}
		}
		return enc.Encode(struct {
			ID         string   `json:"id"`
			Type       string   `json:"type"`
			Path       string   `json:"path"`
			Operations []string `json:"operations"`
		}{
			ID:         created.ID,
			Type:       string(created.Type),
			Path:       gqlPath,
			Operations: operations,
		})

	case mock.MockTypeGRPC:
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
		return enc.Encode(struct {
			ID       string   `json:"id"`
			Type     string   `json:"type"`
			Services []string `json:"services"`
			Methods  []string `json:"methods"`
		}{
			ID:       created.ID,
			Type:     string(created.Type),
			Services: services,
			Methods:  methods,
		})

	case mock.MockTypeMQTT:
		mqttTopic := ""
		mqttQoS := 0
		if created.MQTT != nil && len(created.MQTT.Topics) > 0 {
			mqttTopic = created.MQTT.Topics[0].Topic
			mqttQoS = created.MQTT.Topics[0].QoS
		}
		return enc.Encode(struct {
			ID    string `json:"id"`
			Type  string `json:"type"`
			Topic string `json:"topic"`
			QoS   int    `json:"qos"`
		}{
			ID:    created.ID,
			Type:  string(created.Type),
			Topic: mqttTopic,
			QoS:   mqttQoS,
		})

	case mock.MockTypeSOAP:
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
		return enc.Encode(struct {
			ID          string   `json:"id"`
			Type        string   `json:"type"`
			Path        string   `json:"path"`
			Operations  []string `json:"operations"`
			SOAPActions []string `json:"soapActions,omitempty"`
		}{
			ID:          created.ID,
			Type:        string(created.Type),
			Path:        soapPath,
			Operations:  operations,
			SOAPActions: soapActions,
		})
	}

	// Fallback for unknown types
	return enc.Encode(created)
}
