package mcp

// allToolDefinitions returns all 19 tool definitions in display order.
// Tools are grouped by category: CRUD, Context, Import/Export, Observability, Stateful.
func allToolDefinitions() []ToolDefinition {
	return []ToolDefinition{
		// =====================================================================
		// Mock CRUD (6 tools)
		// =====================================================================
		defListMocks,
		defGetMock,
		defCreateMock,
		defUpdateMock,
		defDeleteMock,
		defToggleMock,

		// =====================================================================
		// Context / Workspace (4 tools)
		// =====================================================================
		defGetCurrentContext,
		defSwitchContext,
		defListWorkspaces,
		defSwitchWorkspace,

		// =====================================================================
		// Import / Export (2 tools)
		// =====================================================================
		defImportMocks,
		defExportMocks,

		// =====================================================================
		// Observability (3 tools)
		// =====================================================================
		defGetServerStatus,
		defGetRequestLogs,
		defClearRequestLogs,

		// =====================================================================
		// Stateful Resources (4 tools)
		// =====================================================================
		defListStatefulItems,
		defGetStatefulItem,
		defCreateStatefulItem,
		defResetStatefulData,
	}
}

// =============================================================================
// Mock CRUD Definitions
// =============================================================================

var defListMocks = ToolDefinition{
	Name:        "list_mocks",
	Description: "List all configured mocks across all protocols (HTTP, WebSocket, GraphQL, gRPC, SOAP, MQTT, OAuth). Returns ID, type, name, enabled status, and a summary for each mock. Use this to see what mocks exist.",
	InputSchema: map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"type": map[string]interface{}{
				"type":        "string",
				"description": "Filter by protocol type",
				"enum":        []string{"http", "websocket", "graphql", "grpc", "soap", "mqtt", "oauth"},
			},
			"enabled": map[string]interface{}{
				"type":        "boolean",
				"description": "Filter by enabled status",
			},
		},
	},
}

var defGetMock = ToolDefinition{
	Name:        "get_mock",
	Description: "Get the full configuration for a specific mock by ID. Returns the complete mock object including all protocol-specific settings. Use this to inspect a mock's details after you have its ID from list_mocks.",
	InputSchema: map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"id": map[string]interface{}{
				"type":        "string",
				"description": "Mock ID (e.g., http_060bff782a1de15f)",
			},
		},
		"required": []string{"id"},
	},
}

var defCreateMock = ToolDefinition{
	Name: "create_mock",
	Description: `Create a new mock for any supported protocol. The 'type' field determines which protocol spec to populate.

Minimal examples by protocol:

HTTP: {"type":"http","http":{"matcher":{"method":"GET","path":"/api/hello"},"response":{"statusCode":200,"body":"{\"msg\":\"hello\"}"}}}
WebSocket: {"type":"websocket","websocket":{"path":"/ws/chat","echoMode":true}}
GraphQL: {"type":"graphql","graphql":{"path":"/graphql","schema":"type Query { user: String }","resolvers":{"Query.user":{"response":"Alice"}}}}
gRPC: {"type":"grpc","grpc":{"port":50051,"protoFile":"./service.proto","reflection":true,"services":{"pkg.Svc":{"methods":{"Get":{"response":{}}}}}}}
MQTT: {"type":"mqtt","mqtt":{"port":1883,"topics":[{"topic":"sensors/temp","messages":[{"payload":"{\"temp\":72}"}]}]}}
SOAP: {"type":"soap","soap":{"path":"/soap","operations":{"GetWeather":{"response":"<Temp>72</Temp>"}}}}
OAuth: {"type":"oauth","oauth":{"issuer":"http://localhost:9999/oauth","clients":[{"clientId":"app","clientSecret":"secret"}]}}

For gRPC and MQTT, mocks on the same port are automatically merged.`,
	InputSchema: map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"type": map[string]interface{}{
				"type":        "string",
				"description": "Protocol type",
				"enum":        []string{"http", "websocket", "graphql", "grpc", "soap", "mqtt", "oauth"},
			},
			"name": map[string]interface{}{
				"type":        "string",
				"description": "Human-readable name for the mock",
			},
			"http": map[string]interface{}{
				"type":        "object",
				"description": "HTTP mock spec (required when type=http)",
				"properties": map[string]interface{}{
					"matcher": map[string]interface{}{
						"type":        "object",
						"description": "Request matching rules",
						"properties": map[string]interface{}{
							"method": map[string]interface{}{
								"type":        "string",
								"description": "HTTP method (GET, POST, PUT, DELETE, PATCH, HEAD, OPTIONS)",
							},
							"path": map[string]interface{}{
								"type":        "string",
								"description": "URL path pattern. Use {param} for path parameters (e.g., /api/users/{id})",
							},
							"headers": map[string]interface{}{
								"type":        "object",
								"description": "Headers to match (exact match)",
							},
							"queryParams": map[string]interface{}{
								"type":        "object",
								"description": "Query parameters to match",
							},
						},
					},
					"response": map[string]interface{}{
						"type":        "object",
						"description": "Response configuration",
						"properties": map[string]interface{}{
							"statusCode": map[string]interface{}{
								"type":    "integer",
								"default": 200,
							},
							"headers": map[string]interface{}{
								"type": "object",
							},
							"body": map[string]interface{}{
								"description": "Response body (string or JSON object)",
							},
							"delayMs": map[string]interface{}{
								"type":        "integer",
								"description": "Response delay in milliseconds",
								"default":     0,
							},
						},
					},
					"priority": map[string]interface{}{
						"type":        "integer",
						"description": "Match priority (higher wins)",
						"default":     0,
					},
				},
			},
			"websocket": map[string]interface{}{
				"type":        "object",
				"description": "WebSocket mock spec (required when type=websocket)",
			},
			"graphql": map[string]interface{}{
				"type":        "object",
				"description": "GraphQL mock spec (required when type=graphql). Resolvers are a map: {\"Query.user\": {\"response\": ...}}",
			},
			"grpc": map[string]interface{}{
				"type":        "object",
				"description": "gRPC mock spec (required when type=grpc)",
			},
			"soap": map[string]interface{}{
				"type":        "object",
				"description": "SOAP mock spec (required when type=soap)",
			},
			"mqtt": map[string]interface{}{
				"type":        "object",
				"description": "MQTT mock spec (required when type=mqtt)",
			},
			"oauth": map[string]interface{}{
				"type":        "object",
				"description": "OAuth mock spec (required when type=oauth)",
			},
		},
		"required": []string{"type"},
	},
}

var defUpdateMock = ToolDefinition{
	Name:        "update_mock",
	Description: "Update an existing mock's configuration. Fetches the current mock, merges provided fields, and saves. Works with any protocol type.",
	InputSchema: map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"id": map[string]interface{}{
				"type":        "string",
				"description": "Mock ID to update",
			},
			"name": map[string]interface{}{
				"type":        "string",
				"description": "New name",
			},
			"enabled": map[string]interface{}{
				"type":        "boolean",
				"description": "Enable or disable",
			},
			"http":      map[string]interface{}{"type": "object", "description": "Updated HTTP spec (partial merge)"},
			"websocket": map[string]interface{}{"type": "object", "description": "Updated WebSocket spec"},
			"graphql":   map[string]interface{}{"type": "object", "description": "Updated GraphQL spec"},
			"grpc":      map[string]interface{}{"type": "object", "description": "Updated gRPC spec"},
			"soap":      map[string]interface{}{"type": "object", "description": "Updated SOAP spec"},
			"mqtt":      map[string]interface{}{"type": "object", "description": "Updated MQTT spec"},
			"oauth":     map[string]interface{}{"type": "object", "description": "Updated OAuth spec"},
		},
		"required": []string{"id"},
	},
}

var defDeleteMock = ToolDefinition{
	Name:        "delete_mock",
	Description: "Delete a mock by ID. The mock will no longer respond to requests.",
	InputSchema: map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"id": map[string]interface{}{
				"type":        "string",
				"description": "Mock ID to delete",
			},
		},
		"required": []string{"id"},
	},
}

var defToggleMock = ToolDefinition{
	Name:        "toggle_mock",
	Description: "Enable or disable a mock without deleting it. Disabled mocks are preserved but do not respond to requests.",
	InputSchema: map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"id": map[string]interface{}{
				"type":        "string",
				"description": "Mock ID",
			},
			"enabled": map[string]interface{}{
				"type":        "boolean",
				"description": "true = enable, false = disable",
			},
		},
		"required": []string{"id", "enabled"},
	},
}

// =============================================================================
// Context / Workspace Definitions
// =============================================================================

var defGetCurrentContext = ToolDefinition{
	Name:        "get_current_context",
	Description: "Show the active context (admin server connection) and all available contexts. Use this FIRST to understand which mockd server you're connected to. Returns context name, admin URL, workspace, and a list of all configured contexts.",
	InputSchema: map[string]interface{}{
		"type":       "object",
		"properties": map[string]interface{}{},
	},
}

var defSwitchContext = ToolDefinition{
	Name:        "switch_context",
	Description: "Switch to a different context (admin server). This changes which mockd server this session communicates with. The change is session-scoped and does NOT persist to disk. Available contexts are defined in ~/.config/mockd/contexts.yaml.",
	InputSchema: map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"name": map[string]interface{}{
				"type":        "string",
				"description": "Context name to switch to (from get_current_context results)",
			},
		},
		"required": []string{"name"},
	},
}

var defListWorkspaces = ToolDefinition{
	Name:        "list_workspaces",
	Description: "List workspaces available on the current admin server. Workspaces isolate groups of mocks. Shows which workspace is currently active.",
	InputSchema: map[string]interface{}{
		"type":       "object",
		"properties": map[string]interface{}{},
	},
}

var defSwitchWorkspace = ToolDefinition{
	Name:        "switch_workspace",
	Description: "Switch the active workspace. Subsequent mock operations will be scoped to this workspace. The change is session-scoped and does NOT persist to disk.",
	InputSchema: map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"id": map[string]interface{}{
				"type":        "string",
				"description": "Workspace ID to switch to",
			},
		},
		"required": []string{"id"},
	},
}

// =============================================================================
// Import / Export Definitions
// =============================================================================

var defImportMocks = ToolDefinition{
	Name:        "import_mocks",
	Description: "Import mocks from inline content. Supports mockd YAML/JSON, OpenAPI, Postman, HAR, WireMock, and cURL formats. Format is auto-detected if not specified.",
	InputSchema: map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"content": map[string]interface{}{
				"type":        "string",
				"description": "Mock definition content (YAML, JSON, OpenAPI spec, Postman collection, HAR, WireMock, or cURL)",
			},
			"format": map[string]interface{}{
				"type":        "string",
				"description": "Format hint for parsing",
				"enum":        []string{"auto", "mockd", "openapi", "postman", "har", "wiremock", "curl"},
				"default":     "auto",
			},
			"replace": map[string]interface{}{
				"type":        "boolean",
				"description": "Replace all existing mocks (true) or merge with existing (false)",
				"default":     false,
			},
			"dryRun": map[string]interface{}{
				"type":        "boolean",
				"description": "Parse and validate without applying. Returns a summary of what would be imported.",
				"default":     false,
			},
		},
		"required": []string{"content"},
	},
}

var defExportMocks = ToolDefinition{
	Name:        "export_mocks",
	Description: "Export all current mocks as configuration. Returns the full mock configuration in the requested format.",
	InputSchema: map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"format": map[string]interface{}{
				"type":        "string",
				"description": "Output format",
				"enum":        []string{"yaml", "json"},
				"default":     "yaml",
			},
		},
	},
}

// =============================================================================
// Observability Definitions
// =============================================================================

var defGetServerStatus = ToolDefinition{
	Name:        "get_server_status",
	Description: "Get server health, ports, and statistics. Use this FIRST when debugging connectivity or port issues. Combines health check, stats, and port information into a single response.",
	InputSchema: map[string]interface{}{
		"type":       "object",
		"properties": map[string]interface{}{},
	},
}

var defGetRequestLogs = ToolDefinition{
	Name:        "get_request_logs",
	Description: "Retrieve captured HTTP request/response logs. Useful for verifying that expected API calls were made to the mock server.",
	InputSchema: map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"limit": map[string]interface{}{
				"type":        "integer",
				"description": "Maximum logs to return",
				"default":     100,
			},
			"offset": map[string]interface{}{
				"type":        "integer",
				"description": "Logs to skip",
				"default":     0,
			},
			"method": map[string]interface{}{
				"type":        "string",
				"description": "Filter by HTTP method",
			},
			"pathPrefix": map[string]interface{}{
				"type":        "string",
				"description": "Filter by path prefix",
			},
			"mockId": map[string]interface{}{
				"type":        "string",
				"description": "Filter by mock ID that handled the request",
			},
		},
	},
}

var defClearRequestLogs = ToolDefinition{
	Name:        "clear_request_logs",
	Description: "Clear all captured request logs. Useful for test isolation between test runs.",
	InputSchema: map[string]interface{}{
		"type":       "object",
		"properties": map[string]interface{}{},
	},
}

// =============================================================================
// Stateful Resource Definitions
// =============================================================================

var defListStatefulItems = ToolDefinition{
	Name:        "list_stateful_items",
	Description: "List items in a stateful mock resource with pagination. Stateful resources provide CRUD operations backed by in-memory data stores.",
	InputSchema: map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"resource": map[string]interface{}{
				"type":        "string",
				"description": "Stateful resource name (e.g., users, products)",
			},
			"limit": map[string]interface{}{
				"type":        "integer",
				"description": "Maximum items to return",
				"default":     100,
			},
			"offset": map[string]interface{}{
				"type":        "integer",
				"description": "Items to skip",
				"default":     0,
			},
			"sort": map[string]interface{}{
				"type":        "string",
				"description": "Field to sort by",
				"default":     "createdAt",
			},
			"order": map[string]interface{}{
				"type":        "string",
				"description": "Sort order",
				"enum":        []string{"asc", "desc"},
				"default":     "desc",
			},
		},
		"required": []string{"resource"},
	},
}

var defGetStatefulItem = ToolDefinition{
	Name:        "get_stateful_item",
	Description: "Get a specific item from a stateful resource by ID.",
	InputSchema: map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"resource": map[string]interface{}{
				"type":        "string",
				"description": "Stateful resource name",
			},
			"id": map[string]interface{}{
				"type":        "string",
				"description": "Item ID to retrieve",
			},
		},
		"required": []string{"resource", "id"},
	},
}

var defCreateStatefulItem = ToolDefinition{
	Name:        "create_stateful_item",
	Description: "Create a new item in a stateful resource. ID and timestamps are auto-generated if not provided.",
	InputSchema: map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"resource": map[string]interface{}{
				"type":        "string",
				"description": "Stateful resource name",
			},
			"data": map[string]interface{}{
				"type":        "object",
				"description": "Item data",
			},
		},
		"required": []string{"resource", "data"},
	},
}

var defResetStatefulData = ToolDefinition{
	Name:        "reset_stateful_data",
	Description: "Reset a stateful resource to its initial seed data state. Useful for test cleanup. The resource parameter is required to prevent accidental full resets.",
	InputSchema: map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"resource": map[string]interface{}{
				"type":        "string",
				"description": "Resource name to reset",
			},
		},
		"required": []string{"resource"},
	},
}
