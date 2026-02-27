package mcp

// Annotation helpers to reduce boilerplate.
var (
	readOnlyAnnotations = map[string]interface{}{
		"readOnlyHint":   true,
		"idempotentHint": true,
	}
	destructiveAnnotations = map[string]interface{}{
		"destructiveHint": true,
		"idempotentHint":  true,
	}
	idempotentAnnotations = map[string]interface{}{
		"idempotentHint": true,
	}
)

// allToolDefinitions returns all 16 tool definitions in display order.
// Tools are grouped by category: Mock, Context, Workspace, Import/Export,
// Observability, Chaos, Verification, Stateful, Custom Operations.
func allToolDefinitions() []ToolDefinition {
	return []ToolDefinition{
		// =====================================================================
		// Mock CRUD (1 multiplexed tool)
		// =====================================================================
		defManageMock,

		// =====================================================================
		// Context / Workspace (2 multiplexed tools)
		// =====================================================================
		defManageContext,
		defManageWorkspace,

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
		// Chaos Engineering (3 tools)
		// =====================================================================
		defGetChaosConfig,
		defSetChaosConfig,
		defResetChaosStats,

		// =====================================================================
		// Verification (3 tools)
		// =====================================================================
		defVerifyMock,
		defGetMockInvocations,
		defResetVerification,

		// =====================================================================
		// Stateful Resources (1 multiplexed tool)
		// =====================================================================
		defManageState,

		// =====================================================================
		// Custom Operations (1 multiplexed tool)
		// =====================================================================
		defManageCustomOperation,
	}
}

// =============================================================================
// Mock CRUD Definition (multiplexed)
// =============================================================================

var defManageMock = ToolDefinition{
	Name: "manage_mock",
	Description: `Create, retrieve, update, delete, list, or toggle mock endpoints. Use 'action' to specify the operation. For list, optionally filter by type or enabled status. For get/update/delete/toggle, provide the mock ID. For create, provide type and protocol-specific configuration.

Examples:
  List:   {"action":"list","type":"http"}
  Get:    {"action":"get","id":"http_060bff782a1de15f"}
  Create: {"action":"create","type":"http","http":{"matcher":{"method":"GET","path":"/api/hello"},"response":{"statusCode":200,"body":"{\"msg\":\"hello\"}"}}}
  Update: {"action":"update","id":"http_060bff782a1de15f","http":{"response":{"statusCode":201}}}
  Delete: {"action":"delete","id":"http_060bff782a1de15f"}
  Toggle: {"action":"toggle","id":"http_060bff782a1de15f","enabled":false}`,
	InputSchema: map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"action": map[string]interface{}{
				"type":        "string",
				"description": "Operation to perform",
				"enum":        []string{"list", "get", "create", "update", "delete", "toggle"},
			},
			"id": map[string]interface{}{
				"type":        "string",
				"description": "Mock ID (required for get/update/delete/toggle)",
			},
			"type": map[string]interface{}{
				"type":        "string",
				"description": "Protocol type for create or list filter",
				"enum":        []string{"http", "websocket", "graphql", "grpc", "soap", "mqtt", "oauth"},
			},
			"enabled": map[string]interface{}{
				"type":        "boolean",
				"description": "For toggle: set specific state. For list: filter by enabled status.",
			},
			"name": map[string]interface{}{
				"type":        "string",
				"description": "Mock name (create/update)",
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
				"description": "WebSocket config (create/update)",
			},
			"graphql": map[string]interface{}{
				"type":        "object",
				"description": "GraphQL config (create/update)",
			},
			"grpc": map[string]interface{}{
				"type":        "object",
				"description": "gRPC config (create/update)",
			},
			"soap": map[string]interface{}{
				"type":        "object",
				"description": "SOAP config (create/update)",
			},
			"mqtt": map[string]interface{}{
				"type":        "object",
				"description": "MQTT config (create/update)",
			},
			"oauth": map[string]interface{}{
				"type":        "object",
				"description": "OAuth config (create/update)",
			},
		},
		"required": []string{"action"},
	},
}

// =============================================================================
// Context / Workspace Definitions (multiplexed)
// =============================================================================

var defManageContext = ToolDefinition{
	Name:        "manage_context",
	Description: "View or switch the active admin server context. Use 'get' to see the current context and all available contexts. Use 'switch' to change which mockd server this session communicates with.",
	InputSchema: map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"action": map[string]interface{}{
				"type":        "string",
				"description": "Operation to perform",
				"enum":        []string{"get", "switch"},
			},
			"name": map[string]interface{}{
				"type":        "string",
				"description": "Context name (required for switch)",
			},
		},
		"required": []string{"action"},
	},
}

var defManageWorkspace = ToolDefinition{
	Name:        "manage_workspace",
	Description: "List available workspaces or switch the active workspace. Workspaces isolate mock configurations. Use 'list' to see all workspaces, 'switch' to route subsequent operations to a specific workspace.",
	InputSchema: map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"action": map[string]interface{}{
				"type":        "string",
				"description": "Operation to perform",
				"enum":        []string{"list", "switch"},
			},
			"id": map[string]interface{}{
				"type":        "string",
				"description": "Workspace ID (required for switch)",
			},
			"name": map[string]interface{}{
				"type":        "string",
				"description": "Workspace name (for switch, alternative to ID)",
			},
		},
		"required": []string{"action"},
	},
}

// =============================================================================
// Import / Export Definitions
// =============================================================================

var defImportMocks = ToolDefinition{
	Name:        "import_mocks",
	Description: "Import mocks from inline content. Supports OpenAPI, Postman, HAR, WireMock, cURL, and mockd YAML/JSON formats. Format is auto-detected if not specified. Use dryRun=true to preview without applying.",
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
	Description: "Export all current mocks as YAML or JSON configuration. Returns the full mock collection for backup, sharing, or version control.",
	Annotations: readOnlyAnnotations,
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
	Annotations: readOnlyAnnotations,
}

var defGetRequestLogs = ToolDefinition{
	Name:        "get_request_logs",
	Description: "Retrieve captured request/response logs. Filter by method, path, mock ID, or protocol. Use this to verify that expected API calls were made to the mock server.",
	Annotations: readOnlyAnnotations,
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
			"protocol": map[string]interface{}{
				"type":        "string",
				"description": "Filter by protocol type",
				"enum":        []string{"http", "grpc", "mqtt", "soap", "graphql", "websocket", "sse"},
			},
		},
	},
}

var defClearRequestLogs = ToolDefinition{
	Name:        "clear_request_logs",
	Description: "Permanently remove all captured request/response logs. Use this for test isolation between test runs. This action cannot be undone.",
	InputSchema: map[string]interface{}{
		"type":       "object",
		"properties": map[string]interface{}{},
	},
	Annotations: destructiveAnnotations,
}

// =============================================================================
// Stateful Resource Definition (multiplexed)
// =============================================================================

var defManageState = ToolDefinition{
	Name: "manage_state",
	Description: `Manage stateful mock resources — CRUD collections that persist data across requests. Use 'overview' to see all resources, 'list_items' to browse items in a resource, 'get_item' for a specific item, 'create_item' to add data, or 'reset' to restore seed data.

Examples:
  Overview:    {"action":"overview"}
  List items:  {"action":"list_items","resource":"users","limit":10}
  Get item:    {"action":"get_item","resource":"users","item_id":"abc123"}
  Create item: {"action":"create_item","resource":"users","data":{"name":"Alice"}}
  Reset:       {"action":"reset","resource":"users"}`,
	InputSchema: map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"action": map[string]interface{}{
				"type":        "string",
				"description": "Operation to perform",
				"enum":        []string{"overview", "list_items", "get_item", "create_item", "reset"},
			},
			"resource": map[string]interface{}{
				"type":        "string",
				"description": "Resource name (required for list_items/get_item/create_item/reset)",
			},
			"item_id": map[string]interface{}{
				"type":        "string",
				"description": "Item ID (required for get_item)",
			},
			"data": map[string]interface{}{
				"type":        "object",
				"description": "Item data (required for create_item)",
			},
			"limit": map[string]interface{}{
				"type":        "integer",
				"description": "Max items for list_items",
				"default":     50,
			},
			"offset": map[string]interface{}{
				"type":        "integer",
				"description": "Pagination offset for list_items",
				"default":     0,
			},
			"sort": map[string]interface{}{
				"type":        "string",
				"description": "Sort field for list_items",
				"default":     "createdAt",
			},
			"order": map[string]interface{}{
				"type":        "string",
				"description": "Sort order: asc or desc",
				"enum":        []string{"asc", "desc"},
				"default":     "desc",
			},
		},
		"required": []string{"action"},
	},
}

// =============================================================================
// Chaos Engineering Definitions
// =============================================================================

var defGetChaosConfig = ToolDefinition{
	Name:        "get_chaos_config",
	Description: "Retrieve the current chaos fault injection configuration including latency, error rate, and bandwidth throttle settings. Returns the active chaos config and injection statistics. Use this to check what chaos rules are active before modifying them.",
	InputSchema: map[string]interface{}{
		"type":       "object",
		"properties": map[string]interface{}{},
	},
	Annotations: readOnlyAnnotations,
}

var defSetChaosConfig = ToolDefinition{
	Name:        "set_chaos_config",
	Description: "Configure chaos fault injection rules. Set latency ranges, error rates with specific HTTP status codes, and bandwidth throttling. Pass enabled=false to disable all chaos. For pre-built configurations, use named profiles like \"slow-api\", \"flaky\", or \"offline\".",
	Annotations: idempotentAnnotations,
	InputSchema: map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"enabled": map[string]interface{}{
				"type":        "boolean",
				"description": "Enable or disable chaos injection",
			},
			"latency_ms": map[string]interface{}{
				"type":        "integer",
				"description": "Fixed latency in milliseconds",
			},
			"latency_min_ms": map[string]interface{}{
				"type":        "integer",
				"description": "Minimum random latency in milliseconds",
			},
			"latency_max_ms": map[string]interface{}{
				"type":        "integer",
				"description": "Maximum random latency in milliseconds",
			},
			"error_rate": map[string]interface{}{
				"type":        "number",
				"description": "Error rate 0.0-1.0 (e.g., 0.2 = 20% of requests fail)",
			},
			"error_codes": map[string]interface{}{
				"type":        "array",
				"description": "HTTP status codes to return on error (e.g., [500, 502, 503])",
				"items":       map[string]interface{}{"type": "integer"},
			},
			"bandwidth_bytes_per_sec": map[string]interface{}{
				"type":        "integer",
				"description": "Bandwidth throttle in bytes/sec",
			},
			"profile": map[string]interface{}{
				"type":        "string",
				"description": "Named chaos profile",
				"enum":        []string{"slow-api", "degraded", "flaky", "offline", "timeout", "rate-limited", "mobile-3g", "satellite", "dns-flaky", "overloaded"},
			},
		},
		"required": []string{"enabled"},
	},
}

var defResetChaosStats = ToolDefinition{
	Name:        "reset_chaos_stats",
	Description: "Reset chaos injection statistics counters to zero without changing the active chaos configuration. Use this to start fresh measurement after modifying chaos rules.",
	InputSchema: map[string]interface{}{
		"type":       "object",
		"properties": map[string]interface{}{},
	},
	Annotations: destructiveAnnotations,
}

// =============================================================================
// Verification Definitions
// =============================================================================

var defVerifyMock = ToolDefinition{
	Name:        "verify_mock",
	Description: "Check whether a mock was called the expected number of times. Returns pass/fail status, actual call count, and invocation details. Optionally assert with expected_count, at_least, or at_most parameters. Use this to assert your application is making the right API calls.",
	Annotations: readOnlyAnnotations,
	InputSchema: map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"id": map[string]interface{}{
				"type":        "string",
				"description": "Mock ID to verify",
			},
			"expected_count": map[string]interface{}{
				"type":        "integer",
				"description": "Expected number of invocations (exact match)",
			},
			"at_least": map[string]interface{}{
				"type":        "integer",
				"description": "Minimum invocations expected",
			},
			"at_most": map[string]interface{}{
				"type":        "integer",
				"description": "Maximum invocations expected",
			},
		},
		"required": []string{"id"},
	},
}

var defGetMockInvocations = ToolDefinition{
	Name:        "get_mock_invocations",
	Description: "List all recorded invocations (request/response pairs) for a specific mock. Shows method, path, headers, body, and timestamp for each call. Use this to debug what requests actually hit a mock.",
	Annotations: readOnlyAnnotations,
	InputSchema: map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"id": map[string]interface{}{
				"type":        "string",
				"description": "Mock ID",
			},
			"limit": map[string]interface{}{
				"type":        "integer",
				"description": "Max invocations to return (default 50)",
				"default":     50,
			},
		},
		"required": []string{"id"},
	},
}

var defResetVerification = ToolDefinition{
	Name:        "reset_verification",
	Description: "Clear verification data (invocation records and counters) for a specific mock or all mocks. Use this to reset counters before running a new test scenario.",
	Annotations: destructiveAnnotations,
	InputSchema: map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"id": map[string]interface{}{
				"type":        "string",
				"description": "Mock ID to reset. Omit to reset ALL mocks.",
			},
		},
	},
}

// =============================================================================
// Custom Operations Definition
// =============================================================================

var defManageCustomOperation = ToolDefinition{
	Name:        "manage_custom_operation",
	Description: "Manage custom operations on stateful resources. Use 'list' to see all operations, 'get' for details, 'register' to create new ones, 'delete' to remove, or 'execute' to run an operation with input data.",
	InputSchema: map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"action": map[string]interface{}{
				"type":        "string",
				"description": "Operation action",
				"enum":        []string{"list", "get", "register", "delete", "execute"},
			},
			"name": map[string]interface{}{
				"type":        "string",
				"description": "Operation name (required for get, delete, execute)",
			},
			"definition": map[string]interface{}{
				"type":        "object",
				"description": "Operation definition (required for register). Must include name, steps, and optionally consistency and response.",
			},
			"input": map[string]interface{}{
				"type":        "object",
				"description": "Input data for execute action",
			},
		},
		"required": []string{"action"},
	},
}
