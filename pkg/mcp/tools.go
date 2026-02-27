package mcp

import "strings"

// ToolHandler is the signature for tool execution functions.
type ToolHandler func(args map[string]interface{}, session *MCPSession, server *Server) (*ToolResult, error)

// Tool represents a registered MCP tool.
type Tool struct {
	Definition ToolDefinition
	Handler    ToolHandler
}

// ToolRegistry manages all registered MCP tools.
// Tools are stored in a slice to preserve registration order for tools/list.
type ToolRegistry struct {
	tools  []*Tool
	byName map[string]*Tool
	server *Server
}

// NewToolRegistry creates a new tool registry and registers all built-in tools.
func NewToolRegistry(server *Server) *ToolRegistry {
	r := &ToolRegistry{
		tools:  make([]*Tool, 0, 16),
		byName: make(map[string]*Tool, 16),
		server: server,
	}

	r.registerBuiltinTools()
	return r
}

// registerBuiltinTools registers all 16 tools from tool_defs.go with their handlers.
func (r *ToolRegistry) registerBuiltinTools() {
	// Tool name → handler mapping.
	handlers := map[string]ToolHandler{
		// Mock CRUD (multiplexed)
		"manage_mock": handleManageMock,

		// Context / Workspace (multiplexed)
		"manage_context":   handleManageContext,
		"manage_workspace": handleManageWorkspace,

		// Import / Export
		"import_mocks": handleImportMocks,
		"export_mocks": handleExportMocks,

		// Observability
		"get_server_status":  handleGetServerStatus,
		"get_request_logs":   handleGetRequestLogs,
		"clear_request_logs": handleClearRequestLogs,

		// Chaos Engineering
		"get_chaos_config":  handleGetChaosConfig,
		"set_chaos_config":  handleSetChaosConfig,
		"reset_chaos_stats": handleResetChaosStats,

		// Verification
		"verify_mock":          handleVerifyMock,
		"get_mock_invocations": handleGetMockInvocations,
		"reset_verification":   handleResetVerification,

		// Stateful Resources (multiplexed)
		"manage_state": handleManageState,

		// Custom Operations (multiplexed)
		"manage_custom_operation": handleManageCustomOperation,
	}

	// Register in definition order (from tool_defs.go) to guarantee
	// consistent ordering in tools/list responses.
	for _, def := range allToolDefinitions() {
		handler, ok := handlers[def.Name]
		if !ok {
			continue
		}
		r.Register(&Tool{
			Definition: def,
			Handler:    handler,
		})
	}
}

// Register adds a tool to the registry.
func (r *ToolRegistry) Register(tool *Tool) {
	r.tools = append(r.tools, tool)
	r.byName[tool.Definition.Name] = tool
}

// Get retrieves a tool by name.
func (r *ToolRegistry) Get(name string) *Tool {
	return r.byName[name]
}

// List returns all tool definitions in registration order.
func (r *ToolRegistry) List() []ToolDefinition {
	defs := make([]ToolDefinition, 0, len(r.tools))
	for _, tool := range r.tools {
		defs = append(defs, tool.Definition)
	}
	return defs
}

// Execute executes a tool by name.
func (r *ToolRegistry) Execute(name string, args map[string]interface{}, session *MCPSession) (*ToolResult, error) {
	tool := r.byName[name]
	if tool == nil {
		return ToolResultError("tool not found: " + name), nil
	}
	return tool.Handler(args, session, r.server)
}

// =============================================================================
// Argument extraction helpers
// =============================================================================

func getString(args map[string]interface{}, key, defaultVal string) string {
	if v, ok := args[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return defaultVal
}

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

func getFloat(args map[string]interface{}, key string, defaultVal float64) float64 {
	if v, ok := args[key]; ok {
		switch n := v.(type) {
		case float64:
			return n
		case int:
			return float64(n)
		case int64:
			return float64(n)
		}
	}
	return defaultVal
}

func getBool(args map[string]interface{}, key string, defaultVal bool) bool {
	if v, ok := args[key]; ok {
		if b, ok := v.(bool); ok {
			return b
		}
	}
	return defaultVal
}

func getBoolPtr(args map[string]interface{}, key string) *bool {
	if v, ok := args[key]; ok {
		if b, ok := v.(bool); ok {
			return &b
		}
	}
	return nil
}

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

func getMap(args map[string]interface{}, key string) map[string]interface{} {
	if v, ok := args[key]; ok {
		if m, ok := v.(map[string]interface{}); ok {
			return m
		}
	}
	return nil
}

// =============================================================================
// Admin error helpers
// =============================================================================

// isConnectionError returns true if the error indicates the admin server is unreachable.
func isConnectionError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "connection refused") ||
		strings.Contains(msg, "no such host") ||
		strings.Contains(msg, "dial tcp") ||
		strings.Contains(msg, "connect: network is unreachable") ||
		strings.Contains(msg, "i/o timeout") ||
		strings.Contains(msg, "context deadline exceeded")
}

// adminError wraps an admin client error with an actionable message when the
// server is unreachable, or returns the original error string otherwise.
func adminError(err error, adminURL string) string {
	if isConnectionError(err) {
		return "mockd server unreachable at " + adminURL + " — start it with: mockd serve"
	}
	return err.Error()
}
