package mock

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// UnmarshalJSON Regression Tests (Bug 3.1)
// =============================================================================

func TestMock_UnmarshalJSON_LegacyFormat(t *testing.T) {
	tests := []struct {
		name        string
		json        string
		wantType    MockType
		wantPath    string
		wantMethod  string
		wantErr     bool
		description string
	}{
		{
			name: "legacy with matcher and response",
			json: `{
				"id": "test-1",
				"enabled": true,
				"matcher": {
					"method": "GET",
					"path": "/api/users"
				},
				"response": {
					"statusCode": 200,
					"body": "{\"users\":[]}"
				}
			}`,
			wantType:    MockTypeHTTP,
			wantPath:    "/api/users",
			wantMethod:  "GET",
			description: "Standard legacy format with matcher at top level",
		},
		{
			name: "legacy with pathPattern (not path)",
			json: `{
				"id": "test-2",
				"enabled": true,
				"matcher": {
					"method": "GET",
					"pathPattern": "/api/users/[0-9]+"
				},
				"response": {
					"statusCode": 200,
					"body": "{}"
				}
			}`,
			wantType:    MockTypeHTTP,
			wantPath:    "/api/users/[0-9]+",
			wantMethod:  "GET",
			description: "Legacy with pathPattern instead of path",
		},
		{
			name: "legacy with headers only (no path/method)",
			json: `{
				"id": "test-3",
				"enabled": true,
				"matcher": {
					"headers": {"X-Custom": "value"}
				},
				"response": {
					"statusCode": 200,
					"body": "ok"
				}
			}`,
			wantType:    MockTypeHTTP,
			wantPath:    "",
			wantMethod:  "",
			description: "Legacy with only headers in matcher",
		},
		{
			name: "legacy preserves priority",
			json: `{
				"id": "test-4",
				"enabled": true,
				"priority": 100,
				"matcher": {
					"method": "POST",
					"path": "/api/data"
				},
				"response": {
					"statusCode": 201,
					"body": "{}"
				}
			}`,
			wantType:    MockTypeHTTP,
			wantPath:    "/api/data",
			wantMethod:  "POST",
			description: "Legacy format preserves priority field",
		},
		{
			name: "legacy with all metadata",
			json: `{
				"id": "test-5",
				"name": "Test Mock",
				"description": "A test description",
				"enabled": true,
				"parentId": "folder-1",
				"metaSortKey": 1.5,
				"workspaceId": "ws-123",
				"syncVersion": 42,
				"createdAt": "2024-01-01T00:00:00Z",
				"updatedAt": "2024-01-02T00:00:00Z",
				"matcher": {
					"method": "GET",
					"path": "/test"
				},
				"response": {
					"statusCode": 200,
					"body": "ok"
				}
			}`,
			wantType:    MockTypeHTTP,
			wantPath:    "/test",
			wantMethod:  "GET",
			description: "Legacy with all metadata fields preserved",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var m Mock
			err := json.Unmarshal([]byte(tt.json), &m)

			if tt.wantErr {
				require.Error(t, err, tt.description)
				return
			}

			require.NoError(t, err, tt.description)
			assert.Equal(t, tt.wantType, m.Type, "Type should be set correctly")
			assert.NotNil(t, m.HTTP, "HTTP spec should be populated")
			assert.Equal(t, tt.wantPath, m.GetPath(), "Path should match")
			assert.Equal(t, tt.wantMethod, m.GetMethod(), "Method should match")
		})
	}
}

func TestMock_UnmarshalJSON_NewFormat(t *testing.T) {
	tests := []struct {
		name        string
		json        string
		wantType    MockType
		wantPath    string
		wantErr     bool
		description string
	}{
		{
			name: "new format with explicit type",
			json: `{
				"id": "test-1",
				"type": "http",
				"enabled": true,
				"http": {
					"matcher": {
						"method": "GET",
						"path": "/api/v2/users"
					},
					"response": {
						"statusCode": 200,
						"body": "{}"
					}
				}
			}`,
			wantType:    MockTypeHTTP,
			wantPath:    "/api/v2/users",
			description: "New format with type field",
		},
		{
			name: "new format with http spec (no type field)",
			json: `{
				"id": "test-2",
				"enabled": true,
				"http": {
					"matcher": {
						"method": "POST",
						"path": "/api/data"
					},
					"response": {
						"statusCode": 201,
						"body": "{}"
					}
				}
			}`,
			wantType:    "",
			wantPath:    "", // GetPath() returns empty because type is not set
			description: "New format with http spec but no type - type not auto-inferred, GetPath returns empty",
		},
		{
			name: "websocket type",
			json: `{
				"id": "ws-1",
				"type": "websocket",
				"enabled": true,
				"websocket": {
					"path": "/ws/events"
				}
			}`,
			wantType:    MockTypeWebSocket,
			wantPath:    "/ws/events",
			description: "WebSocket mock type",
		},
		{
			name: "grpc type",
			json: `{
				"id": "grpc-1",
				"type": "grpc",
				"enabled": true,
				"grpc": {
					"port": 50051,
					"protoFile": "service.proto"
				}
			}`,
			wantType:    MockTypeGRPC,
			wantPath:    ":50051",
			description: "gRPC mock type",
		},
		{
			name: "graphql type",
			json: `{
				"id": "gql-1",
				"type": "graphql",
				"enabled": true,
				"graphql": {
					"path": "/graphql",
					"schema": "type Query { hello: String }"
				}
			}`,
			wantType:    MockTypeGraphQL,
			wantPath:    "/graphql",
			description: "GraphQL mock type",
		},
		{
			name: "soap type",
			json: `{
				"id": "soap-1",
				"type": "soap",
				"enabled": true,
				"soap": {
					"path": "/soap/service"
				}
			}`,
			wantType:    MockTypeSOAP,
			wantPath:    "/soap/service",
			description: "SOAP mock type",
		},
		{
			name: "mqtt type",
			json: `{
				"id": "mqtt-1",
				"type": "mqtt",
				"enabled": true,
				"mqtt": {
					"port": 1883
				}
			}`,
			wantType:    MockTypeMQTT,
			wantPath:    ":1883",
			description: "MQTT mock type",
		},
		{
			name: "oauth type",
			json: `{
				"id": "oauth-1",
				"type": "oauth",
				"enabled": true,
				"oauth": {
					"issuer": "http://localhost:9999/oauth",
					"clients": [{"clientId": "test-app"}]
				}
			}`,
			wantType:    MockTypeOAuth,
			wantPath:    "http://localhost:9999/oauth",
			description: "OAuth mock type",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var m Mock
			err := json.Unmarshal([]byte(tt.json), &m)

			if tt.wantErr {
				require.Error(t, err, tt.description)
				return
			}

			require.NoError(t, err, tt.description)
			assert.Equal(t, tt.wantType, m.Type, "Type should match")
			assert.Equal(t, tt.wantPath, m.GetPath(), "Path should match")
		})
	}
}

func TestMock_UnmarshalJSON_Ambiguous_NewFormatWins(t *testing.T) {
	// When both "type" and "matcher" are present, new format should win
	// (type field takes precedence)
	jsonData := `{
		"id": "ambiguous-1",
		"type": "http",
		"enabled": true,
		"matcher": {
			"method": "GET",
			"path": "/legacy-path"
		},
		"http": {
			"matcher": {
				"method": "POST",
				"path": "/new-path"
			},
			"response": {
				"statusCode": 200,
				"body": "{}"
			}
		}
	}`

	var m Mock
	err := json.Unmarshal([]byte(jsonData), &m)
	require.NoError(t, err)

	// New format should win because type field is present
	assert.Equal(t, MockTypeHTTP, m.Type)
	assert.Equal(t, "/new-path", m.GetPath())
	assert.Equal(t, "POST", m.GetMethod())
}

func TestMock_UnmarshalJSON_InvalidJSON(t *testing.T) {
	tests := []struct {
		name string
		json string
	}{
		{"empty string", ""},
		{"not json", "not json at all"},
		{"unclosed brace", `{"id": "test"`},
		{"invalid field type", `{"id": 123}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var m Mock
			err := json.Unmarshal([]byte(tt.json), &m)
			assert.Error(t, err)
		})
	}
}

// =============================================================================
// SSEConfig Validation Tests (Bug 2.4)
// =============================================================================

func TestSSEConfig_Validate_MutualExclusivity(t *testing.T) {
	event := SSEEventDef{Data: "test"}
	generator := &SSEEventGenerator{Type: "sequence"}

	tests := []struct {
		name      string
		config    SSEConfig
		wantErr   bool
		errSubstr string
	}{
		{
			name: "events and generator - error",
			config: SSEConfig{
				Events:    []SSEEventDef{event},
				Generator: generator,
			},
			wantErr:   true,
			errSubstr: "mutually exclusive",
		},
		{
			name: "events and template - error",
			config: SSEConfig{
				Events:   []SSEEventDef{event},
				Template: "some-template",
			},
			wantErr:   true,
			errSubstr: "mutually exclusive",
		},
		{
			name: "generator and template - error",
			config: SSEConfig{
				Generator: generator,
				Template:  "some-template",
			},
			wantErr:   true,
			errSubstr: "mutually exclusive",
		},
		{
			name: "all three - error",
			config: SSEConfig{
				Events:    []SSEEventDef{event},
				Generator: generator,
				Template:  "some-template",
			},
			wantErr:   true,
			errSubstr: "mutually exclusive",
		},
		{
			name: "only events - ok",
			config: SSEConfig{
				Events: []SSEEventDef{event},
			},
			wantErr: false,
		},
		{
			name: "only generator - ok",
			config: SSEConfig{
				Generator: generator,
			},
			wantErr: false,
		},
		{
			name: "only template - ok",
			config: SSEConfig{
				Template: "some-template",
			},
			wantErr: false,
		},
		{
			name:      "none specified - error",
			config:    SSEConfig{},
			wantErr:   true,
			errSubstr: "one of events, generator, or template is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errSubstr)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestSSEConfig_Validate_EventData(t *testing.T) {
	tests := []struct {
		name    string
		config  SSEConfig
		wantErr bool
	}{
		{
			name: "event with nil data - error",
			config: SSEConfig{
				Events: []SSEEventDef{{Data: nil}},
			},
			wantErr: true,
		},
		{
			name: "event with string data - ok",
			config: SSEConfig{
				Events: []SSEEventDef{{Data: "test"}},
			},
			wantErr: false,
		},
		{
			name: "event with object data - ok",
			config: SSEConfig{
				Events: []SSEEventDef{{Data: map[string]string{"key": "value"}}},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestSSEConfig_Validate_Timing(t *testing.T) {
	event := SSEEventDef{Data: "test"}

	tests := []struct {
		name    string
		config  SSEConfig
		wantErr bool
	}{
		{
			name: "valid random delay",
			config: SSEConfig{
				Events: []SSEEventDef{event},
				Timing: SSETimingConfig{
					RandomDelay: &SSERandomDelayConfig{Min: 100, Max: 500},
				},
			},
			wantErr: false,
		},
		{
			name: "random delay min negative - error",
			config: SSEConfig{
				Events: []SSEEventDef{event},
				Timing: SSETimingConfig{
					RandomDelay: &SSERandomDelayConfig{Min: -1, Max: 500},
				},
			},
			wantErr: true,
		},
		{
			name: "random delay max less than min - error",
			config: SSEConfig{
				Events: []SSEEventDef{event},
				Timing: SSETimingConfig{
					RandomDelay: &SSERandomDelayConfig{Min: 500, Max: 100},
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestSSEConfig_Validate_Lifecycle(t *testing.T) {
	event := SSEEventDef{Data: "test"}

	tests := []struct {
		name    string
		config  SSEConfig
		wantErr bool
	}{
		{
			name: "keepalive disabled (0) - ok",
			config: SSEConfig{
				Events:    []SSEEventDef{event},
				Lifecycle: SSELifecycleConfig{KeepaliveInterval: 0},
			},
			wantErr: false,
		},
		{
			name: "keepalive 5 seconds - ok",
			config: SSEConfig{
				Events:    []SSEEventDef{event},
				Lifecycle: SSELifecycleConfig{KeepaliveInterval: 5},
			},
			wantErr: false,
		},
		{
			name: "keepalive too short (4 seconds) - error",
			config: SSEConfig{
				Events:    []SSEEventDef{event},
				Lifecycle: SSELifecycleConfig{KeepaliveInterval: 4},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestSSEConfig_Validate_RateLimit(t *testing.T) {
	event := SSEEventDef{Data: "test"}

	tests := []struct {
		name    string
		config  SSEConfig
		wantErr bool
	}{
		{
			name: "valid rate limit",
			config: SSEConfig{
				Events:    []SSEEventDef{event},
				RateLimit: &SSERateLimitConfig{EventsPerSecond: 10},
			},
			wantErr: false,
		},
		{
			name: "zero events per second - error",
			config: SSEConfig{
				Events:    []SSEEventDef{event},
				RateLimit: &SSERateLimitConfig{EventsPerSecond: 0},
			},
			wantErr: true,
		},
		{
			name: "negative events per second - error",
			config: SSEConfig{
				Events:    []SSEEventDef{event},
				RateLimit: &SSERateLimitConfig{EventsPerSecond: -1},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestSSEConfig_Validate_Resume(t *testing.T) {
	event := SSEEventDef{Data: "test"}

	tests := []struct {
		name    string
		config  SSEConfig
		wantErr bool
	}{
		{
			name: "resume enabled with buffer - ok",
			config: SSEConfig{
				Events: []SSEEventDef{event},
				Resume: SSEResumeConfig{Enabled: true, BufferSize: 100},
			},
			wantErr: false,
		},
		{
			name: "resume enabled without buffer - error",
			config: SSEConfig{
				Events: []SSEEventDef{event},
				Resume: SSEResumeConfig{Enabled: true, BufferSize: 0},
			},
			wantErr: true,
		},
		{
			name: "resume disabled without buffer - ok",
			config: SSEConfig{
				Events: []SSEEventDef{event},
				Resume: SSEResumeConfig{Enabled: false, BufferSize: 0},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// =============================================================================
// ChunkedConfig Validation Tests (Bug 2.5)
// =============================================================================

func TestChunkedConfig_Validate_MutualExclusivity(t *testing.T) {
	tests := []struct {
		name      string
		config    ChunkedConfig
		wantErr   bool
		errSubstr string
	}{
		{
			name: "data and dataFile - error",
			config: ChunkedConfig{
				Data:     "some data",
				DataFile: "/path/to/file",
			},
			wantErr:   true,
			errSubstr: "mutually exclusive",
		},
		{
			name: "data and ndjsonItems - error",
			config: ChunkedConfig{
				Data:        "some data",
				NDJSONItems: []any{"item1", "item2"},
			},
			wantErr:   true,
			errSubstr: "mutually exclusive",
		},
		{
			name: "dataFile and ndjsonItems - error",
			config: ChunkedConfig{
				DataFile:    "/path/to/file",
				NDJSONItems: []any{"item1"},
			},
			wantErr:   true,
			errSubstr: "mutually exclusive",
		},
		{
			name: "all three - error",
			config: ChunkedConfig{
				Data:        "some data",
				DataFile:    "/path/to/file",
				NDJSONItems: []any{"item1"},
			},
			wantErr:   true,
			errSubstr: "mutually exclusive",
		},
		{
			name: "only data - ok",
			config: ChunkedConfig{
				Data: "some data",
			},
			wantErr: false,
		},
		{
			name: "only dataFile - ok",
			config: ChunkedConfig{
				DataFile: "/path/to/file",
			},
			wantErr: false,
		},
		{
			name: "only ndjsonItems - ok",
			config: ChunkedConfig{
				NDJSONItems: []any{"item1", "item2"},
			},
			wantErr: false,
		},
		{
			name:      "none specified - error",
			config:    ChunkedConfig{},
			wantErr:   true,
			errSubstr: "one of data, dataFile, or ndjsonItems is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errSubstr)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestChunkedConfig_Validate_ChunkSettings(t *testing.T) {
	tests := []struct {
		name    string
		config  ChunkedConfig
		wantErr bool
	}{
		{
			name: "valid chunk settings",
			config: ChunkedConfig{
				Data:       "some data",
				ChunkSize:  1024,
				ChunkDelay: 100,
			},
			wantErr: false,
		},
		{
			name: "negative chunk size - error",
			config: ChunkedConfig{
				Data:      "some data",
				ChunkSize: -1,
			},
			wantErr: true,
		},
		{
			name: "negative chunk delay - error",
			config: ChunkedConfig{
				Data:       "some data",
				ChunkDelay: -1,
			},
			wantErr: true,
		},
		{
			name: "zero chunk size and delay - ok",
			config: ChunkedConfig{
				Data:       "some data",
				ChunkSize:  0,
				ChunkDelay: 0,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// =============================================================================
// Mock Lifecycle Tests
// =============================================================================

func TestMock_Validate_RequiresID(t *testing.T) {
	m := Mock{
		Type: MockTypeHTTP,
		HTTP: &HTTPSpec{
			Matcher:  &HTTPMatcher{Path: "/test"},
			Response: &HTTPResponse{StatusCode: 200},
		},
	}

	err := m.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "id is required")
}

func TestMock_Validate_RequiresType(t *testing.T) {
	m := Mock{
		ID: "test-id",
	}

	err := m.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "type is required")
}

func TestMock_Validate_HTTPRequiresHTTPConfig(t *testing.T) {
	m := Mock{
		ID:   "test-id",
		Type: MockTypeHTTP,
	}

	err := m.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "http spec is required")
}

func TestMock_Validate_HTTPRequiresMatcher(t *testing.T) {
	m := Mock{
		ID:   "test-id",
		Type: MockTypeHTTP,
		HTTP: &HTTPSpec{
			Response: &HTTPResponse{StatusCode: 200},
		},
	}

	err := m.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "matcher is required")
}

func TestMock_Validate_HTTPRequiresResponse(t *testing.T) {
	m := Mock{
		ID:   "test-id",
		Type: MockTypeHTTP,
		HTTP: &HTTPSpec{
			Matcher: &HTTPMatcher{Path: "/test"},
		},
	}

	err := m.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "one of response, sse, or chunked is required")
}

func TestMock_Validate_HTTPOnlyOneResponseType(t *testing.T) {
	m := Mock{
		ID:   "test-id",
		Type: MockTypeHTTP,
		HTTP: &HTTPSpec{
			Matcher:  &HTTPMatcher{Path: "/test"},
			Response: &HTTPResponse{StatusCode: 200},
			SSE:      &SSEConfig{Events: []SSEEventDef{{Data: "test"}}},
		},
	}

	err := m.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "only one of response, sse, or chunked may be specified")
}

func TestMock_Validate_ValidHTTPMock(t *testing.T) {
	m := Mock{
		ID:      "test-id",
		Type:    MockTypeHTTP,
		Enabled: true,
		HTTP: &HTTPSpec{
			Matcher:  &HTTPMatcher{Method: "GET", Path: "/api/test"},
			Response: &HTTPResponse{StatusCode: 200, Body: "ok"},
		},
	}

	err := m.Validate()
	assert.NoError(t, err)
}

func TestMock_Validate_WebSocketRequiresPath(t *testing.T) {
	m := Mock{
		ID:        "ws-1",
		Type:      MockTypeWebSocket,
		WebSocket: &WebSocketSpec{},
	}

	err := m.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "path is required")
}

func TestMock_Validate_WebSocketPathMustStartWithSlash(t *testing.T) {
	m := Mock{
		ID:        "ws-1",
		Type:      MockTypeWebSocket,
		WebSocket: &WebSocketSpec{Path: "ws/events"},
	}

	err := m.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "path must start with /")
}

func TestMock_Validate_ValidWebSocketMock(t *testing.T) {
	m := Mock{
		ID:        "ws-1",
		Type:      MockTypeWebSocket,
		WebSocket: &WebSocketSpec{Path: "/ws/events"},
	}

	err := m.Validate()
	assert.NoError(t, err)
}

func TestMock_Validate_UnknownType(t *testing.T) {
	m := Mock{
		ID:   "test-id",
		Type: MockType("unknown"),
	}

	err := m.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown mock type")
}

// =============================================================================
// JSON Round-trip Tests
// =============================================================================

func TestMock_JSON_RoundTrip_HTTP(t *testing.T) {
	original := Mock{
		ID:          "http-1",
		Type:        MockTypeHTTP,
		Name:        "Test HTTP Mock",
		Description: "A test mock",
		Enabled:     true,
		ParentID:    "folder-1",
		MetaSortKey: 1.5,
		WorkspaceID: "ws-local",
		SyncVersion: 42,
		CreatedAt:   time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		UpdatedAt:   time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC),
		HTTP: &HTTPSpec{
			Priority: 10,
			Matcher: &HTTPMatcher{
				Method:      "POST",
				Path:        "/api/users",
				Headers:     map[string]string{"Content-Type": "application/json"},
				QueryParams: map[string]string{"version": "2"},
			},
			Response: &HTTPResponse{
				StatusCode: 201,
				Headers:    map[string]string{"X-Custom": "header"},
				Body:       `{"id": "new-user"}`,
				DelayMs:    100,
			},
		},
	}

	// Marshal to JSON
	data, err := json.Marshal(original)
	require.NoError(t, err)

	// Unmarshal back
	var restored Mock
	err = json.Unmarshal(data, &restored)
	require.NoError(t, err)

	// Compare
	assert.Equal(t, original.ID, restored.ID)
	assert.Equal(t, original.Type, restored.Type)
	assert.Equal(t, original.Name, restored.Name)
	assert.Equal(t, original.Description, restored.Description)
	assert.Equal(t, original.Enabled, restored.Enabled)
	assert.Equal(t, original.ParentID, restored.ParentID)
	assert.Equal(t, original.MetaSortKey, restored.MetaSortKey)
	assert.Equal(t, original.WorkspaceID, restored.WorkspaceID)
	assert.Equal(t, original.SyncVersion, restored.SyncVersion)
	assert.True(t, original.CreatedAt.Equal(restored.CreatedAt))
	assert.True(t, original.UpdatedAt.Equal(restored.UpdatedAt))

	require.NotNil(t, restored.HTTP)
	assert.Equal(t, original.HTTP.Priority, restored.HTTP.Priority)

	require.NotNil(t, restored.HTTP.Matcher)
	assert.Equal(t, original.HTTP.Matcher.Method, restored.HTTP.Matcher.Method)
	assert.Equal(t, original.HTTP.Matcher.Path, restored.HTTP.Matcher.Path)
	assert.Equal(t, original.HTTP.Matcher.Headers, restored.HTTP.Matcher.Headers)
	assert.Equal(t, original.HTTP.Matcher.QueryParams, restored.HTTP.Matcher.QueryParams)

	require.NotNil(t, restored.HTTP.Response)
	assert.Equal(t, original.HTTP.Response.StatusCode, restored.HTTP.Response.StatusCode)
	assert.Equal(t, original.HTTP.Response.Headers, restored.HTTP.Response.Headers)
	assert.Equal(t, original.HTTP.Response.Body, restored.HTTP.Response.Body)
	assert.Equal(t, original.HTTP.Response.DelayMs, restored.HTTP.Response.DelayMs)
}

func TestMock_JSON_RoundTrip_WebSocket(t *testing.T) {
	original := Mock{
		ID:      "ws-1",
		Type:    MockTypeWebSocket,
		Name:    "Test WebSocket Mock",
		Enabled: true,
		WebSocket: &WebSocketSpec{
			Path:           "/ws/events",
			Subprotocols:   []string{"graphql-ws", "subscriptions-transport-ws"},
			MaxMessageSize: 65536,
			IdleTimeout:    "30s",
			MaxConnections: 100,
			Heartbeat: &WSHeartbeatConfig{
				Enabled:  true,
				Interval: "15s",
				Timeout:  "5s",
			},
		},
	}

	data, err := json.Marshal(original)
	require.NoError(t, err)

	var restored Mock
	err = json.Unmarshal(data, &restored)
	require.NoError(t, err)

	assert.Equal(t, original.ID, restored.ID)
	assert.Equal(t, original.Type, restored.Type)

	require.NotNil(t, restored.WebSocket)
	assert.Equal(t, original.WebSocket.Path, restored.WebSocket.Path)
	assert.Equal(t, original.WebSocket.Subprotocols, restored.WebSocket.Subprotocols)
	assert.Equal(t, original.WebSocket.MaxMessageSize, restored.WebSocket.MaxMessageSize)
	assert.Equal(t, original.WebSocket.IdleTimeout, restored.WebSocket.IdleTimeout)
	assert.Equal(t, original.WebSocket.MaxConnections, restored.WebSocket.MaxConnections)

	require.NotNil(t, restored.WebSocket.Heartbeat)
	assert.Equal(t, original.WebSocket.Heartbeat.Enabled, restored.WebSocket.Heartbeat.Enabled)
	assert.Equal(t, original.WebSocket.Heartbeat.Interval, restored.WebSocket.Heartbeat.Interval)
	assert.Equal(t, original.WebSocket.Heartbeat.Timeout, restored.WebSocket.Heartbeat.Timeout)
}

func TestMock_JSON_RoundTrip_SSE(t *testing.T) {
	fixedDelay := 100
	original := Mock{
		ID:      "sse-1",
		Type:    MockTypeHTTP,
		Name:    "Test SSE Mock",
		Enabled: true,
		HTTP: &HTTPSpec{
			Matcher: &HTTPMatcher{
				Method: "GET",
				Path:   "/events",
			},
			SSE: &SSEConfig{
				Events: []SSEEventDef{
					{Type: "message", Data: "hello", ID: "1"},
					{Type: "update", Data: map[string]string{"key": "value"}, ID: "2"},
				},
				Timing: SSETimingConfig{
					FixedDelay:   &fixedDelay,
					InitialDelay: 50,
				},
				Lifecycle: SSELifecycleConfig{
					MaxEvents:         100,
					KeepaliveInterval: 15,
				},
			},
		},
	}

	data, err := json.Marshal(original)
	require.NoError(t, err)

	var restored Mock
	err = json.Unmarshal(data, &restored)
	require.NoError(t, err)

	assert.Equal(t, original.ID, restored.ID)
	assert.Equal(t, original.Type, restored.Type)

	require.NotNil(t, restored.HTTP)
	require.NotNil(t, restored.HTTP.SSE)
	assert.Len(t, restored.HTTP.SSE.Events, 2)
	assert.Equal(t, "message", restored.HTTP.SSE.Events[0].Type)
	assert.Equal(t, "update", restored.HTTP.SSE.Events[1].Type)

	require.NotNil(t, restored.HTTP.SSE.Timing.FixedDelay)
	assert.Equal(t, 100, *restored.HTTP.SSE.Timing.FixedDelay)
	assert.Equal(t, 50, restored.HTTP.SSE.Timing.InitialDelay)
	assert.Equal(t, 100, restored.HTTP.SSE.Lifecycle.MaxEvents)
	assert.Equal(t, 15, restored.HTTP.SSE.Lifecycle.KeepaliveInterval)
}

// =============================================================================
// HTTPMatcher Validation Tests
// =============================================================================

func TestHTTPMatcher_Validate_AtLeastOneCriteria(t *testing.T) {
	m := &HTTPMatcher{}
	err := m.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "at least one matching criterion must be specified")
}

func TestHTTPMatcher_Validate_InvalidMethod(t *testing.T) {
	m := &HTTPMatcher{Method: "INVALID"}
	err := m.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid HTTP method")
}

func TestHTTPMatcher_Validate_ValidMethods(t *testing.T) {
	methods := []string{"GET", "POST", "PUT", "DELETE", "PATCH", "HEAD", "OPTIONS"}
	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			m := &HTTPMatcher{Method: method, Path: "/test"}
			err := m.Validate()
			assert.NoError(t, err)
		})
	}
}

func TestHTTPMatcher_Validate_PathMustStartWithSlash(t *testing.T) {
	m := &HTTPMatcher{Path: "api/users"}
	err := m.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "path must start with /")
}

func TestHTTPMatcher_Validate_PathAndPathPatternMutuallyExclusive(t *testing.T) {
	m := &HTTPMatcher{Path: "/api/users", PathPattern: "/api/users/[0-9]+"}
	err := m.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cannot specify both path and pathPattern")
}

func TestHTTPMatcher_Validate_InvalidPathPatternRegex(t *testing.T) {
	m := &HTTPMatcher{PathPattern: "[invalid"}
	err := m.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid regex pattern")
}

func TestHTTPMatcher_Validate_InvalidBodyPatternRegex(t *testing.T) {
	m := &HTTPMatcher{Path: "/test", BodyPattern: "[invalid"}
	err := m.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid regex pattern")
}

func TestHTTPMatcher_Validate_BodyEqualsAndBodyContainsMutuallyExclusive(t *testing.T) {
	m := &HTTPMatcher{Path: "/test", BodyEquals: "exact", BodyContains: "partial"}
	err := m.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cannot specify both bodyEquals and bodyContains")
}

func TestHTTPMatcher_Validate_InvalidJSONPath(t *testing.T) {
	m := &HTTPMatcher{
		Path:         "/test",
		BodyJSONPath: map[string]interface{}{"[invalid": "value"},
	}
	err := m.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid JSONPath expression")
}

func TestHTTPMatcher_Validate_InvalidHeaderName(t *testing.T) {
	m := &HTTPMatcher{
		Path:    "/test",
		Headers: map[string]string{"Invalid Header": "value"},
	}
	err := m.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid header name")
}

// =============================================================================
// HTTPResponse Validation Tests
// =============================================================================

func TestHTTPResponse_Validate_InvalidStatusCode(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		wantErr    bool
	}{
		{"too low", 99, true},
		{"min valid", 100, false},
		{"200 OK", 200, false},
		{"404 Not Found", 404, false},
		{"500 Server Error", 500, false},
		{"max valid", 599, false},
		{"too high", 600, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &HTTPResponse{StatusCode: tt.statusCode}
			err := r.Validate()
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestHTTPResponse_Validate_BodyAndBodyFileMutuallyExclusive(t *testing.T) {
	r := &HTTPResponse{
		StatusCode: 200,
		Body:       "inline body",
		BodyFile:   "/path/to/file",
	}
	err := r.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cannot specify both body and bodyFile")
}

func TestHTTPResponse_Validate_DelayMs(t *testing.T) {
	tests := []struct {
		name    string
		delayMs int
		wantErr bool
	}{
		{"negative", -1, true},
		{"zero", 0, false},
		{"positive", 100, false},
		{"max", 30000, false},
		{"over max", 30001, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &HTTPResponse{StatusCode: 200, DelayMs: tt.delayMs}
			err := r.Validate()
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// =============================================================================
// GetSpec and GetPath Tests
// =============================================================================

func TestMock_GetSpec(t *testing.T) {
	tests := []struct {
		name     string
		mock     Mock
		wantNil  bool
		specType string
	}{
		{
			name:     "HTTP",
			mock:     Mock{Type: MockTypeHTTP, HTTP: &HTTPSpec{}},
			specType: "*mock.HTTPSpec",
		},
		{
			name:     "WebSocket",
			mock:     Mock{Type: MockTypeWebSocket, WebSocket: &WebSocketSpec{}},
			specType: "*mock.WebSocketSpec",
		},
		{
			name:     "GraphQL",
			mock:     Mock{Type: MockTypeGraphQL, GraphQL: &GraphQLSpec{}},
			specType: "*mock.GraphQLSpec",
		},
		{
			name:     "gRPC",
			mock:     Mock{Type: MockTypeGRPC, GRPC: &GRPCSpec{}},
			specType: "*mock.GRPCSpec",
		},
		{
			name:     "SOAP",
			mock:     Mock{Type: MockTypeSOAP, SOAP: &SOAPSpec{}},
			specType: "*mock.SOAPSpec",
		},
		{
			name:     "MQTT",
			mock:     Mock{Type: MockTypeMQTT, MQTT: &MQTTSpec{}},
			specType: "*mock.MQTTSpec",
		},
		{
			name:     "OAuth",
			mock:     Mock{Type: MockTypeOAuth, OAuth: &OAuthSpec{}},
			specType: "*mock.OAuthSpec",
		},
		{
			name:    "Unknown type",
			mock:    Mock{Type: MockType("unknown")},
			wantNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spec := tt.mock.GetSpec()
			if tt.wantNil {
				assert.Nil(t, spec)
			} else {
				assert.NotNil(t, spec)
			}
		})
	}
}

func TestMock_GetPath(t *testing.T) {
	tests := []struct {
		name     string
		mock     Mock
		wantPath string
	}{
		{
			name: "HTTP with path",
			mock: Mock{
				Type: MockTypeHTTP,
				HTTP: &HTTPSpec{Matcher: &HTTPMatcher{Path: "/api/users"}},
			},
			wantPath: "/api/users",
		},
		{
			name: "HTTP with pathPattern",
			mock: Mock{
				Type: MockTypeHTTP,
				HTTP: &HTTPSpec{Matcher: &HTTPMatcher{PathPattern: "/api/users/[0-9]+"}},
			},
			wantPath: "/api/users/[0-9]+",
		},
		{
			name: "HTTP path takes precedence over pathPattern",
			mock: Mock{
				Type: MockTypeHTTP,
				HTTP: &HTTPSpec{Matcher: &HTTPMatcher{Path: "/exact", PathPattern: "/pattern"}},
			},
			wantPath: "/exact",
		},
		{
			name:     "WebSocket",
			mock:     Mock{Type: MockTypeWebSocket, WebSocket: &WebSocketSpec{Path: "/ws/events"}},
			wantPath: "/ws/events",
		},
		{
			name:     "GraphQL",
			mock:     Mock{Type: MockTypeGraphQL, GraphQL: &GraphQLSpec{Path: "/graphql"}},
			wantPath: "/graphql",
		},
		{
			name:     "gRPC",
			mock:     Mock{Type: MockTypeGRPC, GRPC: &GRPCSpec{Port: 50051}},
			wantPath: ":50051",
		},
		{
			name:     "gRPC zero port",
			mock:     Mock{Type: MockTypeGRPC, GRPC: &GRPCSpec{Port: 0}},
			wantPath: "",
		},
		{
			name:     "SOAP",
			mock:     Mock{Type: MockTypeSOAP, SOAP: &SOAPSpec{Path: "/soap/service"}},
			wantPath: "/soap/service",
		},
		{
			name:     "MQTT",
			mock:     Mock{Type: MockTypeMQTT, MQTT: &MQTTSpec{Port: 1883}},
			wantPath: ":1883",
		},
		{
			name:     "OAuth",
			mock:     Mock{Type: MockTypeOAuth, OAuth: &OAuthSpec{Issuer: "http://localhost:9999/oauth"}},
			wantPath: "http://localhost:9999/oauth",
		},
		{
			name:     "nil spec",
			mock:     Mock{Type: MockTypeHTTP, HTTP: nil},
			wantPath: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.wantPath, tt.mock.GetPath())
		})
	}
}

func TestMock_GetMethod(t *testing.T) {
	tests := []struct {
		name       string
		mock       Mock
		wantMethod string
	}{
		{
			name: "HTTP with method",
			mock: Mock{
				Type: MockTypeHTTP,
				HTTP: &HTTPSpec{Matcher: &HTTPMatcher{Method: "POST"}},
			},
			wantMethod: "POST",
		},
		{
			name: "HTTP without method",
			mock: Mock{
				Type: MockTypeHTTP,
				HTTP: &HTTPSpec{Matcher: &HTTPMatcher{Path: "/test"}},
			},
			wantMethod: "",
		},
		{
			name:       "non-HTTP type",
			mock:       Mock{Type: MockTypeWebSocket},
			wantMethod: "",
		},
		{
			name:       "nil HTTP spec",
			mock:       Mock{Type: MockTypeHTTP, HTTP: nil},
			wantMethod: "",
		},
		{
			name:       "nil matcher",
			mock:       Mock{Type: MockTypeHTTP, HTTP: &HTTPSpec{Matcher: nil}},
			wantMethod: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.wantMethod, tt.mock.GetMethod())
		})
	}
}
