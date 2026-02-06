package mcp

import (
	"encoding/json"
	"time"

	types "github.com/getmockd/mockd/pkg/api/types"
)

// ProtocolVersion is the MCP protocol version advertised by this implementation.
const ProtocolVersion = "2025-06-18"

// SupportedProtocolVersions lists all protocol versions this server accepts.
// We accept multiple versions for compatibility with various MCP clients.
var SupportedProtocolVersions = []string{
	"2025-11-25", // OpenCode 1.1.52+
	"2025-06-18", // Current spec
	"2025-03-26", // Common
	"2024-11-05", // Older clients
}

// IsProtocolVersionSupported checks if a client's protocol version is supported.
func IsProtocolVersionSupported(version string) bool {
	for _, v := range SupportedProtocolVersions {
		if v == version {
			return true
		}
	}
	return false
}

// JSON-RPC 2.0 Types

// JSONRPCRequest represents an incoming JSON-RPC 2.0 request.
type JSONRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      interface{}     `json:"id,omitempty"` // Can be string, number, or null for notifications
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// IsNotification returns true if this is a notification (no ID).
func (r *JSONRPCRequest) IsNotification() bool {
	return r.ID == nil
}

// JSONRPCResponse represents an outgoing JSON-RPC 2.0 response.
type JSONRPCResponse struct {
	JSONRPC string        `json:"jsonrpc"`
	ID      interface{}   `json:"id"`
	Result  interface{}   `json:"result,omitempty"`
	Error   *JSONRPCError `json:"error,omitempty"`
}

// JSONRPCError represents a JSON-RPC 2.0 error object.
type JSONRPCError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// JSONRPCNotification represents a server-initiated notification.
type JSONRPCNotification struct {
	JSONRPC string      `json:"jsonrpc"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
}

// MCP Protocol Types

// InitializeParams represents parameters for the initialize request.
type InitializeParams struct {
	ProtocolVersion string             `json:"protocolVersion"`
	Capabilities    ClientCapabilities `json:"capabilities"`
	ClientInfo      ClientInfo         `json:"clientInfo"`
}

// InitializeResult represents the result of a successful initialize.
type InitializeResult struct {
	ProtocolVersion string             `json:"protocolVersion"`
	Capabilities    ServerCapabilities `json:"capabilities"`
	ServerInfo      ServerInfo         `json:"serverInfo"`
}

// ClientInfo identifies the MCP client.
type ClientInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// ServerInfo identifies the MCP server.
type ServerInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// ClientCapabilities describes client-supported features.
type ClientCapabilities struct {
	Roots       *RootsCapability       `json:"roots,omitempty"`
	Sampling    *SamplingCapability    `json:"sampling,omitempty"`
	Elicitation *ElicitationCapability `json:"elicitation,omitempty"`
}

// ServerCapabilities describes server-supported features.
type ServerCapabilities struct {
	Tools     *ToolsCapability     `json:"tools,omitempty"`
	Resources *ResourcesCapability `json:"resources,omitempty"`
	Prompts   *PromptsCapability   `json:"prompts,omitempty"`
}

// RootsCapability describes client filesystem roots capability.
type RootsCapability struct {
	ListChanged bool `json:"listChanged,omitempty"`
}

// SamplingCapability describes client LLM sampling capability.
type SamplingCapability struct{}

// ElicitationCapability describes client user info request capability.
type ElicitationCapability struct{}

// ToolsCapability describes server tools support.
type ToolsCapability struct {
	ListChanged bool `json:"listChanged,omitempty"`
}

// ResourcesCapability describes server resources support.
type ResourcesCapability struct {
	Subscribe   bool `json:"subscribe,omitempty"`
	ListChanged bool `json:"listChanged,omitempty"`
}

// PromptsCapability describes server prompts support.
type PromptsCapability struct {
	ListChanged bool `json:"listChanged,omitempty"`
}

// Tool Types

// ToolDefinition describes a tool exposed by the MCP server.
type ToolDefinition struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	InputSchema map[string]interface{} `json:"inputSchema"`
}

// ToolsListResult is the response for tools/list.
type ToolsListResult struct {
	Tools []ToolDefinition `json:"tools"`
}

// ToolCallParams are parameters for tools/call.
type ToolCallParams struct {
	Name      string                 `json:"name"`
	Arguments map[string]interface{} `json:"arguments,omitempty"`
}

// ToolResult is the result from tool execution.
type ToolResult struct {
	Content []ContentBlock `json:"content"`
	IsError bool           `json:"isError,omitempty"`
}

// ContentBlock represents a content item in tool results.
type ContentBlock struct {
	Type     string           `json:"type"`
	Text     string           `json:"text,omitempty"`
	MimeType string           `json:"mimeType,omitempty"`
	Blob     string           `json:"blob,omitempty"`
	Resource *ResourceContent `json:"resource,omitempty"`
}

// Resource Types

// ResourceDefinition describes a resource exposed by the MCP server.
type ResourceDefinition struct {
	URI         string `json:"uri"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	MimeType    string `json:"mimeType,omitempty"`
}

// ResourcesListResult is the response for resources/list.
type ResourcesListResult struct {
	Resources []ResourceDefinition `json:"resources"`
}

// ResourceReadParams are parameters for resources/read.
type ResourceReadParams struct {
	URI string `json:"uri"`
}

// ResourceReadResult is the response for resources/read.
type ResourceReadResult struct {
	Contents []ResourceContent `json:"contents"`
}

// ResourceContent represents the contents of a resource.
type ResourceContent struct {
	URI      string `json:"uri"`
	MimeType string `json:"mimeType,omitempty"`
	Text     string `json:"text,omitempty"`
	Blob     string `json:"blob,omitempty"`
}

// ResourceSubscribeParams are parameters for resources/subscribe.
type ResourceSubscribeParams struct {
	URI string `json:"uri"`
}

// ResourceUnsubscribeParams are parameters for resources/unsubscribe.
type ResourceUnsubscribeParams struct {
	URI string `json:"uri"`
}

// SSE Event Types

// SSEEvent represents a server-sent event.
type SSEEvent struct {
	ID    string `json:"id,omitempty"`
	Event string `json:"event,omitempty"`
	Data  string `json:"data"`
	Retry int    `json:"retry,omitempty"`
}

// =============================================================================
// Tool Parameter Types — Mock CRUD
// =============================================================================

// ListMocksParams are parameters for list_mocks tool.
type ListMocksParams struct {
	Type    string `json:"type,omitempty"`
	Enabled *bool  `json:"enabled,omitempty"`
}

// GetMockParams are parameters for get_mock tool.
type GetMockParams struct {
	ID string `json:"id"`
}

// CreateMockParams are parameters for create_mock tool.
// The protocol-specific fields (http, websocket, etc.) are passed through
// as-is to the admin API's POST /mocks endpoint.
type CreateMockParams struct {
	Type      string          `json:"type"`
	Name      string          `json:"name,omitempty"`
	HTTP      json.RawMessage `json:"http,omitempty"`
	WebSocket json.RawMessage `json:"websocket,omitempty"`
	GraphQL   json.RawMessage `json:"graphql,omitempty"`
	GRPC      json.RawMessage `json:"grpc,omitempty"`
	SOAP      json.RawMessage `json:"soap,omitempty"`
	MQTT      json.RawMessage `json:"mqtt,omitempty"`
	OAuth     json.RawMessage `json:"oauth,omitempty"`
}

// UpdateMockParams are parameters for update_mock tool.
type UpdateMockParams struct {
	ID        string          `json:"id"`
	Name      string          `json:"name,omitempty"`
	Enabled   *bool           `json:"enabled,omitempty"`
	HTTP      json.RawMessage `json:"http,omitempty"`
	WebSocket json.RawMessage `json:"websocket,omitempty"`
	GraphQL   json.RawMessage `json:"graphql,omitempty"`
	GRPC      json.RawMessage `json:"grpc,omitempty"`
	SOAP      json.RawMessage `json:"soap,omitempty"`
	MQTT      json.RawMessage `json:"mqtt,omitempty"`
	OAuth     json.RawMessage `json:"oauth,omitempty"`
}

// DeleteMockParams are parameters for delete_mock tool.
type DeleteMockParams struct {
	ID string `json:"id"`
}

// ToggleMockParams are parameters for toggle_mock tool.
type ToggleMockParams struct {
	ID      string `json:"id"`
	Enabled bool   `json:"enabled"`
}

// =============================================================================
// Tool Parameter Types — Context / Workspace
// =============================================================================

// SwitchContextParams are parameters for switch_context tool.
type SwitchContextParams struct {
	Name string `json:"name"`
}

// SwitchWorkspaceParams are parameters for switch_workspace tool.
type SwitchWorkspaceParams struct {
	ID string `json:"id"`
}

// =============================================================================
// Tool Parameter Types — Import / Export
// =============================================================================

// ImportMocksParams are parameters for import_mocks tool.
type ImportMocksParams struct {
	Content string `json:"content"`
	Format  string `json:"format,omitempty"`
	Replace bool   `json:"replace,omitempty"`
	DryRun  bool   `json:"dryRun,omitempty"`
}

// ExportMocksParams are parameters for export_mocks tool.
type ExportMocksParams struct {
	Format string `json:"format,omitempty"`
}

// =============================================================================
// Tool Parameter Types — Observability
// =============================================================================

// GetRequestLogsParams are parameters for get_request_logs tool.
type GetRequestLogsParams struct {
	Limit      int    `json:"limit,omitempty"`
	Offset     int    `json:"offset,omitempty"`
	Method     string `json:"method,omitempty"`
	PathPrefix string `json:"pathPrefix,omitempty"`
	MockID     string `json:"mockId,omitempty"`
}

// =============================================================================
// Tool Parameter Types — Stateful Resources
// =============================================================================

// ListStatefulItemsParams are parameters for list_stateful_items tool.
type ListStatefulItemsParams struct {
	Resource string `json:"resource"`
	Limit    int    `json:"limit,omitempty"`
	Offset   int    `json:"offset,omitempty"`
	Sort     string `json:"sort,omitempty"`
	Order    string `json:"order,omitempty"`
}

// GetStatefulItemParams are parameters for get_stateful_item tool.
type GetStatefulItemParams struct {
	Resource string `json:"resource"`
	ID       string `json:"id"`
}

// CreateStatefulItemParams are parameters for create_stateful_item tool.
type CreateStatefulItemParams struct {
	Resource string                 `json:"resource"`
	Data     map[string]interface{} `json:"data"`
}

// ResetStatefulDataParams are parameters for reset_stateful_data tool.
// Resource is required — no accidental full resets.
type ResetStatefulDataParams struct {
	Resource string `json:"resource"`
}

// =============================================================================
// Response Types
// =============================================================================

// MockSummary represents a protocol-agnostic mock summary for list_mocks.
type MockSummary struct {
	ID      string `json:"id"`
	Type    string `json:"type"`
	Name    string `json:"name,omitempty"`
	Enabled bool   `json:"enabled"`
	Summary string `json:"summary"` // Human-readable: "GET /api/users", "gRPC :50051", etc.
}

// ContextInfo represents context information returned by get_current_context.
// AuthToken is intentionally omitted to prevent credential leakage.
type ContextInfo struct {
	Name        string `json:"name"`
	AdminURL    string `json:"adminUrl"`
	Workspace   string `json:"workspace,omitempty"`
	Description string `json:"description,omitempty"`
}

// ContextResult is returned by get_current_context and switch_context.
type ContextResult struct {
	Current   string        `json:"current"`
	AdminURL  string        `json:"adminUrl"`
	Workspace string        `json:"workspace,omitempty"`
	Contexts  []ContextInfo `json:"contexts,omitempty"`
}

// StatefulListResult represents the result of a list_stateful_items operation.
type StatefulListResult struct {
	Data []map[string]interface{} `json:"data"`
	Meta PaginationMeta           `json:"meta"`
}

// PaginationMeta is an alias for the shared pagination metadata type.
type PaginationMeta = types.PaginationMeta

// RequestLogEntry represents a captured request log entry.
type RequestLogEntry struct {
	ID        string    `json:"id"`
	Method    string    `json:"method"`
	Path      string    `json:"path"`
	Status    int       `json:"status"`
	Duration  string    `json:"duration"`
	Timestamp time.Time `json:"timestamp"`
	MockID    string    `json:"mockId,omitempty"`
}

// Notification Types

// ResourceListChangedParams are parameters for notifications/resources/list_changed.
type ResourceListChangedParams struct{}

// ResourceUpdatedParams are parameters for notifications/resources/updated.
type ResourceUpdatedParams struct {
	URI string `json:"uri"`
}

// Session State

// SessionState represents the lifecycle state of an MCP session.
type SessionState int

const (
	SessionStateNew SessionState = iota
	SessionStateInitialized
	SessionStateReady
	SessionStateExpired
)

// String returns the string representation of the session state.
func (s SessionState) String() string {
	switch s {
	case SessionStateNew:
		return "new"
	case SessionStateInitialized:
		return "initialized"
	case SessionStateReady:
		return "ready"
	case SessionStateExpired:
		return "expired"
	default:
		return "unknown"
	}
}
