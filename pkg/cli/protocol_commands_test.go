package cli

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// =============================================================================
// wsStatusCmd — WebSocket status via admin API
// =============================================================================

func TestWSStatus_NoMocks(t *testing.T) {
	// Cannot use t.Parallel() — modifies globals: adminURL, jsonOutput, os.Stdout

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && r.URL.Path == "/mocks" {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"mocks": []interface{}{},
				"count": 0,
			})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer ts.Close()

	oldAdminURL := adminURL
	oldJSON := jsonOutput
	adminURL = ts.URL
	jsonOutput = false
	defer func() {
		adminURL = oldAdminURL
		jsonOutput = oldJSON
	}()

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := wsStatusCmd.RunE(wsStatusCmd, nil)

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	buf.ReadFrom(r)

	if err != nil {
		t.Fatalf("wsStatusCmd.RunE() returned error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "no mocks configured") {
		t.Errorf("expected 'no mocks configured' in output, got: %s", output)
	}
}

func TestWSStatus_WithMocks(t *testing.T) {
	// Cannot use t.Parallel() — modifies globals: adminURL, jsonOutput, os.Stdout

	enabled := true
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && r.URL.Path == "/mocks" {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"mocks": []map[string]interface{}{
					{
						"id":      "ws_001",
						"type":    "websocket",
						"enabled": enabled,
						"websocket": map[string]interface{}{
							"path": "/ws/chat",
						},
					},
					{
						"id":      "ws_002",
						"type":    "websocket",
						"enabled": enabled,
						"websocket": map[string]interface{}{
							"path": "/ws/events",
						},
					},
					{
						"id":   "http_001",
						"type": "http",
						"http": map[string]interface{}{
							"matcher":  map[string]interface{}{"method": "GET", "path": "/api"},
							"response": map[string]interface{}{"statusCode": 200},
						},
					},
				},
				"count": 3,
			})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer ts.Close()

	oldAdminURL := adminURL
	oldJSON := jsonOutput
	adminURL = ts.URL
	jsonOutput = false
	defer func() {
		adminURL = oldAdminURL
		jsonOutput = oldJSON
	}()

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := wsStatusCmd.RunE(wsStatusCmd, nil)

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	buf.ReadFrom(r)

	if err != nil {
		t.Fatalf("wsStatusCmd.RunE() returned error: %v", err)
	}

	output := buf.String()
	// Should report 2 websocket mocks (not 3, HTTP one is filtered out)
	if !strings.Contains(output, "2 mock(s) configured") {
		t.Errorf("expected '2 mock(s) configured' in output, got: %s", output)
	}
	if !strings.Contains(output, "/ws/chat") {
		t.Errorf("expected '/ws/chat' in output, got: %s", output)
	}
	if !strings.Contains(output, "/ws/events") {
		t.Errorf("expected '/ws/events' in output, got: %s", output)
	}
	if !strings.Contains(output, "ws_001") {
		t.Errorf("expected 'ws_001' in output, got: %s", output)
	}
}

func TestWSStatus_ServerError(t *testing.T) {
	// Cannot use t.Parallel() — modifies globals: adminURL, jsonOutput

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error":   "internal_error",
			"message": "engine unavailable",
		})
	}))
	defer ts.Close()

	oldAdminURL := adminURL
	oldJSON := jsonOutput
	adminURL = ts.URL
	jsonOutput = false
	defer func() {
		adminURL = oldAdminURL
		jsonOutput = oldJSON
	}()

	err := wsStatusCmd.RunE(wsStatusCmd, nil)
	if err == nil {
		t.Fatal("wsStatusCmd.RunE() should return error for 500 response")
	}
	if !strings.Contains(err.Error(), "failed to get WebSocket status") {
		t.Errorf("error message should mention WebSocket status, got: %v", err)
	}
}

// =============================================================================
// runMQTTStatus — MQTT status via admin API
// =============================================================================

func TestMQTTStatus_Running(t *testing.T) {
	// Cannot use t.Parallel() — modifies globals: adminURL, jsonOutput, os.Stdout

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && r.URL.Path == "/mqtt/status" {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"running":     true,
				"port":        float64(1883),
				"clientCount": float64(3),
				"topicCount":  float64(5),
				"tlsEnabled":  true,
				"authEnabled": true,
			})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer ts.Close()

	oldAdminURL := adminURL
	oldJSON := jsonOutput
	adminURL = ts.URL
	jsonOutput = false
	defer func() {
		adminURL = oldAdminURL
		jsonOutput = oldJSON
	}()

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := runMQTTStatus(nil, nil)

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	buf.ReadFrom(r)

	if err != nil {
		t.Fatalf("runMQTTStatus() returned error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "MQTT broker: running") {
		t.Errorf("expected 'MQTT broker: running' in output, got: %s", output)
	}
	if !strings.Contains(output, "Port: 1883") {
		t.Errorf("expected 'Port: 1883' in output, got: %s", output)
	}
	if !strings.Contains(output, "Connected clients: 3") {
		t.Errorf("expected 'Connected clients: 3' in output, got: %s", output)
	}
	if !strings.Contains(output, "Configured topics: 5") {
		t.Errorf("expected 'Configured topics: 5' in output, got: %s", output)
	}
	if !strings.Contains(output, "TLS: enabled") {
		t.Errorf("expected 'TLS: enabled' in output, got: %s", output)
	}
	if !strings.Contains(output, "Authentication: enabled") {
		t.Errorf("expected 'Authentication: enabled' in output, got: %s", output)
	}
}

func TestMQTTStatus_NotRunning(t *testing.T) {
	// Cannot use t.Parallel() — modifies globals: adminURL, jsonOutput, os.Stdout

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && r.URL.Path == "/mqtt/status" {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"running": false,
			})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer ts.Close()

	oldAdminURL := adminURL
	oldJSON := jsonOutput
	adminURL = ts.URL
	jsonOutput = false
	defer func() {
		adminURL = oldAdminURL
		jsonOutput = oldJSON
	}()

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := runMQTTStatus(nil, nil)

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	buf.ReadFrom(r)

	if err != nil {
		t.Fatalf("runMQTTStatus() returned error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "MQTT broker: not running") {
		t.Errorf("expected 'MQTT broker: not running' in output, got: %s", output)
	}
}

func TestMQTTStatus_ServerError(t *testing.T) {
	// Cannot use t.Parallel() — modifies globals: adminURL, jsonOutput

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error":   "internal_error",
			"message": "mqtt not available",
		})
	}))
	defer ts.Close()

	oldAdminURL := adminURL
	oldJSON := jsonOutput
	adminURL = ts.URL
	jsonOutput = false
	defer func() {
		adminURL = oldAdminURL
		jsonOutput = oldJSON
	}()

	err := runMQTTStatus(nil, nil)
	if err == nil {
		t.Fatal("runMQTTStatus() should return error for 500 response")
	}
	if !strings.Contains(err.Error(), "failed to get MQTT status") {
		t.Errorf("error message should mention MQTT status, got: %v", err)
	}
}

// =============================================================================
// printMQTTPublishInstructions — stdout output test
// =============================================================================

func TestPrintMQTTPublishInstructions(t *testing.T) {
	tests := []struct {
		name         string
		broker       string
		topic        string
		message      string
		username     string
		password     string
		qos          int
		retain       bool
		wantContains []string
	}{
		{
			name:    "basic",
			broker:  "localhost:1883",
			topic:   "test/topic",
			message: `{"temp":72}`,
			wantContains: []string{
				"mosquitto_pub is not installed",
				"brew install mosquitto",
				"apt install mosquitto-clients",
				"mosquitto_pub -h localhost -p 1883 -t 'test/topic'",
				`{"temp":72}`,
			},
		},
		{
			name:     "with_auth",
			broker:   "broker.io:8883",
			topic:    "secure/topic",
			message:  "hello",
			username: "user1",
			password: "pass1",
			wantContains: []string{
				"-h broker.io -p 8883",
				"-u 'user1'",
				"-P 'pass1'",
			},
		},
		{
			name:    "with_qos_and_retain",
			broker:  "localhost:1883",
			topic:   "retained/topic",
			message: "persisted",
			qos:     2,
			retain:  true,
			wantContains: []string{
				"-q 2",
				"-r",
			},
		},
		{
			name:    "qos_zero_omitted",
			broker:  "localhost:1883",
			topic:   "basic/topic",
			message: "msg",
			qos:     0,
			retain:  false,
			// qos 0 and no retain should not appear in command
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Capture stdout
			oldStdout := os.Stdout
			r, w, _ := os.Pipe()
			os.Stdout = w

			err := printMQTTPublishInstructions(tt.broker, tt.topic, tt.message, tt.username, tt.password, tt.qos, tt.retain)

			w.Close()
			os.Stdout = oldStdout

			var buf bytes.Buffer
			buf.ReadFrom(r)
			output := buf.String()

			// Should always return an error (mosquitto_pub not found)
			if err == nil {
				t.Fatal("printMQTTPublishInstructions() should return error")
			}
			if err.Error() != "mosquitto_pub not found" {
				t.Errorf("error = %q, want 'mosquitto_pub not found'", err.Error())
			}

			for _, want := range tt.wantContains {
				if !strings.Contains(output, want) {
					t.Errorf("output missing %q, got:\n%s", want, output)
				}
			}

			// For qos 0, the -q flag should NOT appear
			if tt.qos == 0 && strings.Contains(output, "-q 0") {
				t.Errorf("qos 0 should not produce -q flag, got:\n%s", output)
			}
			// For retain false, the -r flag should NOT appear
			if !tt.retain && strings.Contains(output, " -r") {
				t.Errorf("retain=false should not produce -r flag, got:\n%s", output)
			}
		})
	}
}

// =============================================================================
// printMQTTSubscribeInstructions — stdout output test
// =============================================================================

func TestPrintMQTTSubscribeInstructions(t *testing.T) {
	tests := []struct {
		name         string
		broker       string
		topic        string
		username     string
		password     string
		qos          int
		count        int
		wantContains []string
	}{
		{
			name:   "basic",
			broker: "localhost:1883",
			topic:  "sensor/#",
			wantContains: []string{
				"mosquitto_sub is not installed",
				"brew install mosquitto",
				"apt install mosquitto-clients",
				"mosquitto_sub -h localhost -p 1883 -t 'sensor/#' -v",
			},
		},
		{
			name:     "with_auth_and_count",
			broker:   "myhost:9999",
			topic:    "events",
			username: "admin",
			password: "secret",
			qos:      1,
			count:    10,
			wantContains: []string{
				"-h myhost -p 9999",
				"-u 'admin'",
				"-P 'secret'",
				"-q 1",
				"-C 10",
			},
		},
		{
			name:   "count_zero_omitted",
			broker: "localhost:1883",
			topic:  "test",
			count:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Capture stdout
			oldStdout := os.Stdout
			r, w, _ := os.Pipe()
			os.Stdout = w

			err := printMQTTSubscribeInstructions(tt.broker, tt.topic, tt.username, tt.password, tt.qos, tt.count, 0)

			w.Close()
			os.Stdout = oldStdout

			var buf bytes.Buffer
			buf.ReadFrom(r)
			output := buf.String()

			// Should always return an error (mosquitto_sub not found)
			if err == nil {
				t.Fatal("printMQTTSubscribeInstructions() should return error")
			}
			if err.Error() != "mosquitto_sub not found" {
				t.Errorf("error = %q, want 'mosquitto_sub not found'", err.Error())
			}

			for _, want := range tt.wantContains {
				if !strings.Contains(output, want) {
					t.Errorf("output missing %q, got:\n%s", want, output)
				}
			}

			// count 0 should not produce -C flag
			if tt.count == 0 && strings.Contains(output, "-C") {
				t.Errorf("count=0 should not produce -C flag, got:\n%s", output)
			}
		})
	}
}

// =============================================================================
// printGRPCCallInstructions — stdout output test
// =============================================================================

func TestPrintGRPCCallInstructions(t *testing.T) {
	tests := []struct {
		name          string
		endpoint      string
		serviceMethod string
		body          string
		metadata      string
		plaintext     bool
		wantContains  []string
	}{
		{
			name:          "basic_plaintext",
			endpoint:      "localhost:50051",
			serviceMethod: "greet.Greeter/SayHello",
			body:          `{"name":"World"}`,
			plaintext:     true,
			wantContains: []string{
				"grpcurl is not installed",
				"brew install grpcurl",
				"go install github.com/fullstorydev/grpcurl",
				"grpcurl -plaintext",
				"-d '{\"name\":\"World\"}'",
				"localhost:50051",
				"greet.Greeter/SayHello",
			},
		},
		{
			name:          "no_plaintext",
			endpoint:      "api.example.com:443",
			serviceMethod: "api.Service/Method",
			body:          `{}`,
			plaintext:     false,
			wantContains: []string{
				"api.example.com:443",
				"api.Service/Method",
			},
		},
		{
			name:          "with_metadata",
			endpoint:      "localhost:50051",
			serviceMethod: "svc/Method",
			body:          `{"id":1}`,
			metadata:      "authorization:Bearer tok123,x-request-id:abc",
			plaintext:     true,
			wantContains: []string{
				"-H 'authorization: Bearer tok123'",
				"-H 'x-request-id: abc'",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Capture stdout
			oldStdout := os.Stdout
			r, w, _ := os.Pipe()
			os.Stdout = w

			err := printGRPCCallInstructions(tt.endpoint, tt.serviceMethod, tt.body, tt.metadata, tt.plaintext)

			w.Close()
			os.Stdout = oldStdout

			var buf bytes.Buffer
			buf.ReadFrom(r)
			output := buf.String()

			// Should always return an error (grpcurl not found)
			if err == nil {
				t.Fatal("printGRPCCallInstructions() should return error")
			}
			if err.Error() != "grpcurl not found" {
				t.Errorf("error = %q, want 'grpcurl not found'", err.Error())
			}

			for _, want := range tt.wantContains {
				if !strings.Contains(output, want) {
					t.Errorf("output missing %q, got:\n%s", want, output)
				}
			}

			// No plaintext flag when plaintext=false
			if !tt.plaintext && strings.Contains(output, "-plaintext") {
				t.Errorf("plaintext=false should not produce -plaintext flag, got:\n%s", output)
			}
		})
	}
}

// =============================================================================
// runGraphQLValidate — schema validation with temp files
// =============================================================================

func TestGraphQLValidate_ValidSchema(t *testing.T) {
	// Cannot use t.Parallel() — modifies globals: jsonOutput, os.Stdout

	schema := `
type Query {
  user(id: ID!): User
  users: [User!]!
}

type User {
  id: ID!
  name: String!
  email: String!
}
`
	dir := t.TempDir()
	schemaFile := filepath.Join(dir, "schema.graphql")
	if err := os.WriteFile(schemaFile, []byte(schema), 0600); err != nil {
		t.Fatalf("failed to write schema file: %v", err)
	}

	oldJSON := jsonOutput
	jsonOutput = false
	defer func() { jsonOutput = oldJSON }()

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := runGraphQLValidate(nil, []string{schemaFile})

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	buf.ReadFrom(r)

	if err != nil {
		t.Fatalf("runGraphQLValidate() returned error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "Schema valid") {
		t.Errorf("expected 'Schema valid' in output, got: %s", output)
	}
	if !strings.Contains(output, "Queries: 2") {
		t.Errorf("expected 'Queries: 2' in output, got: %s", output)
	}
}

func TestGraphQLValidate_InvalidSchema(t *testing.T) {
	schema := `
type Query {
  user(id: ID!): NonExistentType
}
`
	dir := t.TempDir()
	schemaFile := filepath.Join(dir, "bad.graphql")
	if err := os.WriteFile(schemaFile, []byte(schema), 0600); err != nil {
		t.Fatalf("failed to write schema file: %v", err)
	}

	err := runGraphQLValidate(nil, []string{schemaFile})
	if err == nil {
		t.Fatal("runGraphQLValidate() should return error for invalid schema")
	}
	if !strings.Contains(err.Error(), "schema validation failed") {
		t.Errorf("error message should mention 'schema validation failed', got: %v", err)
	}
}

func TestGraphQLValidate_NoQueryType(t *testing.T) {
	// A schema without Query type should fail validation
	schema := `
type User {
  id: ID!
  name: String!
}
`
	dir := t.TempDir()
	schemaFile := filepath.Join(dir, "no_query.graphql")
	if err := os.WriteFile(schemaFile, []byte(schema), 0600); err != nil {
		t.Fatalf("failed to write schema file: %v", err)
	}

	err := runGraphQLValidate(nil, []string{schemaFile})
	if err == nil {
		t.Fatal("runGraphQLValidate() should return error for schema without Query type")
	}
}

func TestGraphQLValidate_MissingFile(t *testing.T) {
	err := runGraphQLValidate(nil, []string{"/nonexistent/schema.graphql"})
	if err == nil {
		t.Fatal("runGraphQLValidate() should return error for missing file")
	}
	if !strings.Contains(err.Error(), "failed to read schema file") {
		t.Errorf("error message should mention 'failed to read schema file', got: %v", err)
	}
}

func TestGraphQLValidate_NoArgs(t *testing.T) {
	err := runGraphQLValidate(nil, nil)
	if err == nil {
		t.Fatal("runGraphQLValidate() should return error with no args")
	}
	if err.Error() != "schema file is required" {
		t.Errorf("error = %q, want 'schema file is required'", err.Error())
	}
}

func TestGraphQLValidate_WithMutations(t *testing.T) {
	// Cannot use t.Parallel() — modifies globals: jsonOutput, os.Stdout

	schema := `
type Query {
  user(id: ID!): User
}

type Mutation {
  createUser(name: String!): User
  deleteUser(id: ID!): Boolean
}

type User {
  id: ID!
  name: String!
}
`
	dir := t.TempDir()
	schemaFile := filepath.Join(dir, "with_mutations.graphql")
	if err := os.WriteFile(schemaFile, []byte(schema), 0600); err != nil {
		t.Fatalf("failed to write schema file: %v", err)
	}

	oldJSON := jsonOutput
	jsonOutput = false
	defer func() { jsonOutput = oldJSON }()

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := runGraphQLValidate(nil, []string{schemaFile})

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	buf.ReadFrom(r)

	if err != nil {
		t.Fatalf("runGraphQLValidate() returned error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "Mutations: 2") {
		t.Errorf("expected 'Mutations: 2' in output, got: %s", output)
	}
}

// =============================================================================
// runGraphQLQuery — query against httptest server
// =============================================================================

func TestGraphQLQuery_Success(t *testing.T) {
	// Cannot use t.Parallel() — modifies globals: graphqlVariables, graphqlOperationName,
	//   graphqlHeaders, graphqlPretty, os.Stdout

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		ct := r.Header.Get("Content-Type")
		if ct != "application/json" {
			t.Errorf("Content-Type = %q, want application/json", ct)
		}

		// Decode the GraphQL request to verify it was sent correctly
		var req map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("failed to decode request: %v", err)
		}
		query, ok := req["query"].(string)
		if !ok || query == "" {
			t.Error("request missing 'query' field")
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"data": map[string]interface{}{
				"user": map[string]interface{}{
					"name": "Alice",
				},
			},
		})
	}))
	defer ts.Close()

	// Save and restore globals
	oldVars := graphqlVariables
	oldOpName := graphqlOperationName
	oldHeaders := graphqlHeaders
	oldPretty := graphqlPretty
	graphqlVariables = ""
	graphqlOperationName = ""
	graphqlHeaders = ""
	graphqlPretty = false
	defer func() {
		graphqlVariables = oldVars
		graphqlOperationName = oldOpName
		graphqlHeaders = oldHeaders
		graphqlPretty = oldPretty
	}()

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := runGraphQLQuery(nil, []string{ts.URL, `{ user(id: "1") { name } }`})

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	buf.ReadFrom(r)

	if err != nil {
		t.Fatalf("runGraphQLQuery() returned error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "Alice") {
		t.Errorf("expected 'Alice' in output, got: %s", output)
	}
}

func TestGraphQLQuery_WithVariables(t *testing.T) {
	// Cannot use t.Parallel() — modifies globals

	var receivedReq map[string]interface{}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&receivedReq)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"data": map[string]interface{}{
				"user": map[string]interface{}{"name": "Bob"},
			},
		})
	}))
	defer ts.Close()

	oldVars := graphqlVariables
	oldOpName := graphqlOperationName
	oldHeaders := graphqlHeaders
	oldPretty := graphqlPretty
	graphqlVariables = `{"id":"42"}`
	graphqlOperationName = "GetUser"
	graphqlHeaders = ""
	graphqlPretty = false
	defer func() {
		graphqlVariables = oldVars
		graphqlOperationName = oldOpName
		graphqlHeaders = oldHeaders
		graphqlPretty = oldPretty
	}()

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := runGraphQLQuery(nil, []string{ts.URL, `query GetUser($id: ID!) { user(id: $id) { name } }`})

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	buf.ReadFrom(r)

	if err != nil {
		t.Fatalf("runGraphQLQuery() returned error: %v", err)
	}

	// Verify variables were sent
	if receivedReq == nil {
		t.Fatal("server did not receive request")
	}
	vars, ok := receivedReq["variables"].(map[string]interface{})
	if !ok {
		t.Fatal("request missing 'variables' field")
	}
	if vars["id"] != "42" {
		t.Errorf("variables.id = %v, want '42'", vars["id"])
	}
	if receivedReq["operationName"] != "GetUser" {
		t.Errorf("operationName = %v, want 'GetUser'", receivedReq["operationName"])
	}
}

func TestGraphQLQuery_WithErrors(t *testing.T) {
	// Cannot use t.Parallel() — modifies globals

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"data": nil,
			"errors": []map[string]interface{}{
				{"message": "Field 'unknown' not found"},
			},
		})
	}))
	defer ts.Close()

	oldVars := graphqlVariables
	oldOpName := graphqlOperationName
	oldHeaders := graphqlHeaders
	oldPretty := graphqlPretty
	graphqlVariables = ""
	graphqlOperationName = ""
	graphqlHeaders = ""
	graphqlPretty = false
	defer func() {
		graphqlVariables = oldVars
		graphqlOperationName = oldOpName
		graphqlHeaders = oldHeaders
		graphqlPretty = oldPretty
	}()

	// Capture stdout (the response is still printed)
	oldStdout := os.Stdout
	_, w, _ := os.Pipe()
	os.Stdout = w

	err := runGraphQLQuery(nil, []string{ts.URL, `{ unknown }`})

	w.Close()
	os.Stdout = oldStdout

	if err == nil {
		t.Fatal("runGraphQLQuery() should return error when GraphQL response has errors")
	}
	if !strings.Contains(err.Error(), "1 error(s)") {
		t.Errorf("error = %v, want mention of '1 error(s)'", err)
	}
}

func TestGraphQLQuery_PrettyPrint(t *testing.T) {
	// Cannot use t.Parallel() — modifies globals

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// Return compact JSON
		w.Write([]byte(`{"data":{"user":{"name":"Alice","age":30}}}`))
	}))
	defer ts.Close()

	oldVars := graphqlVariables
	oldOpName := graphqlOperationName
	oldHeaders := graphqlHeaders
	oldPretty := graphqlPretty
	graphqlVariables = ""
	graphqlOperationName = ""
	graphqlHeaders = ""
	graphqlPretty = true
	defer func() {
		graphqlVariables = oldVars
		graphqlOperationName = oldOpName
		graphqlHeaders = oldHeaders
		graphqlPretty = oldPretty
	}()

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := runGraphQLQuery(nil, []string{ts.URL, `{ user { name age } }`})

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	buf.ReadFrom(r)

	if err != nil {
		t.Fatalf("runGraphQLQuery() returned error: %v", err)
	}

	output := buf.String()
	// Pretty-printed JSON should have indentation
	if !strings.Contains(output, "  ") {
		t.Errorf("expected indented output with pretty=true, got: %s", output)
	}
}

func TestGraphQLQuery_NoArgs(t *testing.T) {
	err := runGraphQLQuery(nil, nil)
	if err == nil {
		t.Fatal("runGraphQLQuery() should return error with no args")
	}
	if err.Error() != "endpoint and query are required" {
		t.Errorf("error = %q, want 'endpoint and query are required'", err.Error())
	}
}

func TestGraphQLQuery_InvalidVariablesJSON(t *testing.T) {
	// Cannot use t.Parallel() — modifies globals

	oldVars := graphqlVariables
	graphqlVariables = "not-valid-json"
	defer func() { graphqlVariables = oldVars }()

	err := runGraphQLQuery(nil, []string{"http://localhost:9999", "{ user { name } }"})
	if err == nil {
		t.Fatal("runGraphQLQuery() should return error for invalid variables JSON")
	}
	if !strings.Contains(err.Error(), "invalid variables JSON") {
		t.Errorf("error = %v, want mention of 'invalid variables JSON'", err)
	}
}

func TestGraphQLQuery_WithCustomHeaders(t *testing.T) {
	// Cannot use t.Parallel() — modifies globals

	var receivedAuth string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"data": map[string]interface{}{"ok": true},
		})
	}))
	defer ts.Close()

	oldVars := graphqlVariables
	oldOpName := graphqlOperationName
	oldHeaders := graphqlHeaders
	oldPretty := graphqlPretty
	graphqlVariables = ""
	graphqlOperationName = ""
	graphqlHeaders = "Authorization:Bearer mytoken"
	graphqlPretty = false
	defer func() {
		graphqlVariables = oldVars
		graphqlOperationName = oldOpName
		graphqlHeaders = oldHeaders
		graphqlPretty = oldPretty
	}()

	// Capture stdout
	oldStdout := os.Stdout
	_, w, _ := os.Pipe()
	os.Stdout = w

	err := runGraphQLQuery(nil, []string{ts.URL, `{ ok }`})

	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("runGraphQLQuery() returned error: %v", err)
	}

	if receivedAuth != "Bearer mytoken" {
		t.Errorf("Authorization header = %q, want 'Bearer mytoken'", receivedAuth)
	}
}
