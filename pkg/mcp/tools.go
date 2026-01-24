package mcp

// ToolHandler is the signature for tool execution functions.
type ToolHandler func(args map[string]interface{}, session *MCPSession, server *Server) (*ToolResult, error)

// Tool represents a registered MCP tool.
type Tool struct {
	Definition ToolDefinition
	Handler    ToolHandler
}

// ToolRegistry manages all registered MCP tools.
type ToolRegistry struct {
	tools  map[string]*Tool
	server *Server
}

// NewToolRegistry creates a new tool registry and registers all built-in tools.
func NewToolRegistry(server *Server) *ToolRegistry {
	r := &ToolRegistry{
		tools:  make(map[string]*Tool),
		server: server,
	}

	// Register all built-in tools
	r.registerBuiltinTools()

	return r
}

// registerBuiltinTools registers all built-in tools.
func (r *ToolRegistry) registerBuiltinTools() {
	// Mock Data Tools
	r.Register(&Tool{
		Definition: ToolDefinition{
			Name:        "get_mock_data",
			Description: "Retrieve the mock response data for a given API endpoint path and HTTP method. Returns the response body that would be returned if an HTTP request was made to the mock server.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"path": map[string]interface{}{
						"type":        "string",
						"description": "API endpoint path (e.g., /api/users, /api/products/123)",
					},
					"method": map[string]interface{}{
						"type":        "string",
						"description": "HTTP method (GET, POST, PUT, DELETE, PATCH)",
						"enum":        []string{"GET", "POST", "PUT", "DELETE", "PATCH", "HEAD", "OPTIONS"},
						"default":     "GET",
					},
					"headers": map[string]interface{}{
						"type":        "object",
						"description": "Optional request headers for matching",
						"additionalProperties": map[string]interface{}{
							"type": "string",
						},
					},
					"queryParams": map[string]interface{}{
						"type":        "object",
						"description": "Optional query parameters for matching",
						"additionalProperties": map[string]interface{}{
							"type": "string",
						},
					},
					"body": map[string]interface{}{
						"type":        "string",
						"description": "Optional request body for matching",
					},
				},
				"required": []string{"path"},
			},
		},
		Handler: handleGetMockData,
	})

	// Discovery Tools
	r.Register(&Tool{
		Definition: ToolDefinition{
			Name:        "list_endpoints",
			Description: "List all configured mock endpoints with their paths, methods, and status. Useful for discovering what APIs are available to mock.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"method": map[string]interface{}{
						"type":        "string",
						"description": "Filter by HTTP method",
						"enum":        []string{"GET", "POST", "PUT", "DELETE", "PATCH", "HEAD", "OPTIONS"},
					},
					"pathPrefix": map[string]interface{}{
						"type":        "string",
						"description": "Filter by path prefix (e.g., /api/users)",
					},
					"enabled": map[string]interface{}{
						"type":        "boolean",
						"description": "Filter by enabled status",
					},
				},
			},
		},
		Handler: handleListEndpoints,
	})

	// Management Tools
	r.Register(&Tool{
		Definition: ToolDefinition{
			Name:        "create_endpoint",
			Description: "Create a new mock endpoint that will respond to HTTP requests matching the specified path and method.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"path": map[string]interface{}{
						"type":        "string",
						"description": "API endpoint path (e.g., /api/products)",
					},
					"method": map[string]interface{}{
						"type":        "string",
						"description": "HTTP method to match",
						"enum":        []string{"GET", "POST", "PUT", "DELETE", "PATCH", "HEAD", "OPTIONS"},
					},
					"response": map[string]interface{}{
						"type":        "object",
						"description": "Response configuration",
						"properties": map[string]interface{}{
							"status": map[string]interface{}{
								"type":        "integer",
								"description": "HTTP status code",
								"default":     200,
							},
							"headers": map[string]interface{}{
								"type":        "object",
								"description": "Response headers",
								"additionalProperties": map[string]interface{}{
									"type": "string",
								},
							},
							"body": map[string]interface{}{
								"description": "Response body (string or JSON object)",
							},
							"delay": map[string]interface{}{
								"type":        "string",
								"description": "Response delay (e.g., 100ms, 2s)",
								"default":     "0s",
							},
						},
						"required": []string{"body"},
					},
					"description": map[string]interface{}{
						"type":        "string",
						"description": "Human-readable description of the mock",
					},
					"priority": map[string]interface{}{
						"type":        "integer",
						"description": "Match priority (higher wins)",
						"default":     0,
					},
				},
				"required": []string{"path", "method", "response"},
			},
		},
		Handler: handleCreateEndpoint,
	})

	r.Register(&Tool{
		Definition: ToolDefinition{
			Name:        "update_endpoint",
			Description: "Update an existing mock endpoint's response configuration, priority, or description.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"id": map[string]interface{}{
						"type":        "string",
						"description": "Mock endpoint ID",
					},
					"response": map[string]interface{}{
						"type":        "object",
						"description": "New response configuration",
						"properties": map[string]interface{}{
							"status":  map[string]interface{}{"type": "integer"},
							"headers": map[string]interface{}{"type": "object"},
							"body":    map[string]interface{}{},
							"delay":   map[string]interface{}{"type": "string"},
						},
					},
					"description": map[string]interface{}{
						"type":        "string",
						"description": "New description",
					},
					"priority": map[string]interface{}{
						"type":        "integer",
						"description": "New priority",
					},
				},
				"required": []string{"id"},
			},
		},
		Handler: handleUpdateEndpoint,
	})

	r.Register(&Tool{
		Definition: ToolDefinition{
			Name:        "delete_endpoint",
			Description: "Remove a mock endpoint. Requests to this path/method will no longer be mocked.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"id": map[string]interface{}{
						"type":        "string",
						"description": "Mock endpoint ID to delete",
					},
				},
				"required": []string{"id"},
			},
		},
		Handler: handleDeleteEndpoint,
	})

	r.Register(&Tool{
		Definition: ToolDefinition{
			Name:        "toggle_endpoint",
			Description: "Enable or disable a mock endpoint without deleting it. Disabled mocks are preserved but do not respond to requests.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"id": map[string]interface{}{
						"type":        "string",
						"description": "Mock endpoint ID",
					},
					"enabled": map[string]interface{}{
						"type":        "boolean",
						"description": "Whether to enable (true) or disable (false) the mock",
					},
				},
				"required": []string{"id", "enabled"},
			},
		},
		Handler: handleToggleEndpoint,
	})

	// Stateful Tools
	r.Register(&Tool{
		Definition: ToolDefinition{
			Name:        "stateful_list",
			Description: "List items in a stateful mock resource with optional pagination and filtering.",
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
					"filter": map[string]interface{}{
						"type":        "object",
						"description": "Field filters (exact match)",
						"additionalProperties": map[string]interface{}{
							"type": "string",
						},
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
		},
		Handler: handleStatefulList,
	})

	r.Register(&Tool{
		Definition: ToolDefinition{
			Name:        "stateful_get",
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
		},
		Handler: handleStatefulGet,
	})

	r.Register(&Tool{
		Definition: ToolDefinition{
			Name:        "stateful_create",
			Description: "Create a new item in a stateful resource. ID and timestamps are auto-generated.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"resource": map[string]interface{}{
						"type":        "string",
						"description": "Stateful resource name",
					},
					"data": map[string]interface{}{
						"type":        "object",
						"description": "Item data (id will be auto-generated if not provided)",
					},
				},
				"required": []string{"resource", "data"},
			},
		},
		Handler: handleStatefulCreate,
	})

	r.Register(&Tool{
		Definition: ToolDefinition{
			Name:        "stateful_reset",
			Description: "Reset a stateful resource to its initial seed data state. Useful for test cleanup.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"resource": map[string]interface{}{
						"type":        "string",
						"description": "Resource name to reset. If omitted, all stateful resources are reset.",
					},
				},
			},
		},
		Handler: handleStatefulReset,
	})

	// Observability Tools
	r.Register(&Tool{
		Definition: ToolDefinition{
			Name:        "get_request_logs",
			Description: "Retrieve captured HTTP request logs. Useful for verifying that expected API calls were made.",
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
					"statusCode": map[string]interface{}{
						"type":        "integer",
						"description": "Filter by response status code",
					},
				},
			},
		},
		Handler: handleGetRequestLogs,
	})

	r.Register(&Tool{
		Definition: ToolDefinition{
			Name:        "clear_logs",
			Description: "Clear all captured request logs. Useful for test isolation between test runs.",
			InputSchema: map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			},
		},
		Handler: handleClearLogs,
	})
}

// Register adds a tool to the registry.
func (r *ToolRegistry) Register(tool *Tool) {
	r.tools[tool.Definition.Name] = tool
}

// Get retrieves a tool by name.
func (r *ToolRegistry) Get(name string) *Tool {
	return r.tools[name]
}

// List returns all tool definitions.
func (r *ToolRegistry) List() []ToolDefinition {
	defs := make([]ToolDefinition, 0, len(r.tools))
	for _, tool := range r.tools {
		defs = append(defs, tool.Definition)
	}
	return defs
}

// Execute executes a tool by name.
func (r *ToolRegistry) Execute(name string, args map[string]interface{}, session *MCPSession) (*ToolResult, error) {
	tool := r.tools[name]
	if tool == nil {
		return ToolResultError("tool not found: " + name), nil
	}

	return tool.Handler(args, session, r.server)
}

// Helper functions for argument extraction

// getString extracts a string argument with default.
func getString(args map[string]interface{}, key, defaultVal string) string {
	if v, ok := args[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return defaultVal
}

// getInt extracts an int argument with default.
func getInt(args map[string]interface{}, key string, defaultVal int) int {
	if v, ok := args[key]; ok {
		switch n := v.(type) {
		case float64:
			return int(n)
		case int:
			return n
		case int64:
			return int(n)
		}
	}
	return defaultVal
}

// getBool extracts a bool argument with default.
func getBool(args map[string]interface{}, key string, defaultVal bool) bool {
	if v, ok := args[key]; ok {
		if b, ok := v.(bool); ok {
			return b
		}
	}
	return defaultVal
}

// getBoolPtr extracts an optional bool argument.
func getBoolPtr(args map[string]interface{}, key string) *bool {
	if v, ok := args[key]; ok {
		if b, ok := v.(bool); ok {
			return &b
		}
	}
	return nil
}

// getStringMap extracts a map[string]string argument.
func getStringMap(args map[string]interface{}, key string) map[string]string {
	if v, ok := args[key]; ok {
		if m, ok := v.(map[string]interface{}); ok {
			result := make(map[string]string)
			for k, val := range m {
				if s, ok := val.(string); ok {
					result[k] = s
				}
			}
			return result
		}
	}
	return nil
}

// getMap extracts a map[string]interface{} argument.
func getMap(args map[string]interface{}, key string) map[string]interface{} {
	if v, ok := args[key]; ok {
		if m, ok := v.(map[string]interface{}); ok {
			return m
		}
	}
	return nil
}
