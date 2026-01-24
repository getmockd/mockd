package mcp

import (
	"encoding/json"
	"time"
)

// ProtocolVersion is the MCP protocol version supported by this implementation.
const ProtocolVersion = "2025-06-18"

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

// Tool-Specific Parameter Types

// GetMockDataParams are parameters for get_mock_data tool.
type GetMockDataParams struct {
	Path        string            `json:"path"`
	Method      string            `json:"method,omitempty"`
	Headers     map[string]string `json:"headers,omitempty"`
	QueryParams map[string]string `json:"queryParams,omitempty"`
	Body        string            `json:"body,omitempty"`
}

// ListEndpointsParams are parameters for list_endpoints tool.
type ListEndpointsParams struct {
	Method     string `json:"method,omitempty"`
	PathPrefix string `json:"pathPrefix,omitempty"`
	Enabled    *bool  `json:"enabled,omitempty"`
}

// CreateEndpointParams are parameters for create_endpoint tool.
type CreateEndpointParams struct {
	Path        string         `json:"path"`
	Method      string         `json:"method"`
	Response    ResponseConfig `json:"response"`
	Description string         `json:"description,omitempty"`
	Priority    int            `json:"priority,omitempty"`
}

// ResponseConfig defines the response for a mock endpoint.
type ResponseConfig struct {
	Status  int               `json:"status,omitempty"`
	Headers map[string]string `json:"headers,omitempty"`
	Body    interface{}       `json:"body"`
	Delay   string            `json:"delay,omitempty"`
}

// UpdateEndpointParams are parameters for update_endpoint tool.
type UpdateEndpointParams struct {
	ID          string          `json:"id"`
	Response    *ResponseConfig `json:"response,omitempty"`
	Description string          `json:"description,omitempty"`
	Priority    *int            `json:"priority,omitempty"`
}

// DeleteEndpointParams are parameters for delete_endpoint tool.
type DeleteEndpointParams struct {
	ID string `json:"id"`
}

// ToggleEndpointParams are parameters for toggle_endpoint tool.
type ToggleEndpointParams struct {
	ID      string `json:"id"`
	Enabled bool   `json:"enabled"`
}

// StatefulListParams are parameters for stateful_list tool.
type StatefulListParams struct {
	Resource string            `json:"resource"`
	Limit    int               `json:"limit,omitempty"`
	Offset   int               `json:"offset,omitempty"`
	Filter   map[string]string `json:"filter,omitempty"`
	Sort     string            `json:"sort,omitempty"`
	Order    string            `json:"order,omitempty"`
}

// StatefulGetParams are parameters for stateful_get tool.
type StatefulGetParams struct {
	Resource string `json:"resource"`
	ID       string `json:"id"`
}

// StatefulCreateParams are parameters for stateful_create tool.
type StatefulCreateParams struct {
	Resource string                 `json:"resource"`
	Data     map[string]interface{} `json:"data"`
}

// StatefulResetParams are parameters for stateful_reset tool.
type StatefulResetParams struct {
	Resource string `json:"resource,omitempty"`
}

// GetRequestLogsParams are parameters for get_request_logs tool.
type GetRequestLogsParams struct {
	Limit      int    `json:"limit,omitempty"`
	Offset     int    `json:"offset,omitempty"`
	Method     string `json:"method,omitempty"`
	PathPrefix string `json:"pathPrefix,omitempty"`
	MockID     string `json:"mockId,omitempty"`
	StatusCode int    `json:"statusCode,omitempty"`
}

// ClearLogsParams are parameters for clear_logs tool.
type ClearLogsParams struct{}

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

// EndpointInfo represents information about a mock endpoint.
type EndpointInfo struct {
	ID          string    `json:"id"`
	Method      string    `json:"method"`
	Path        string    `json:"path"`
	Enabled     bool      `json:"enabled"`
	Description string    `json:"description,omitempty"`
	Priority    int       `json:"priority,omitempty"`
	CreatedAt   time.Time `json:"createdAt,omitempty"`
	UpdatedAt   time.Time `json:"updatedAt,omitempty"`
}

// StatefulListResult represents the result of a stateful_list operation.
type StatefulListResult struct {
	Data []map[string]interface{} `json:"data"`
	Meta PaginationMeta           `json:"meta"`
}

// PaginationMeta contains pagination information.
type PaginationMeta struct {
	Total  int `json:"total"`
	Limit  int `json:"limit"`
	Offset int `json:"offset"`
	Count  int `json:"count"`
}

// RequestLogEntry represents a captured request log entry.
type RequestLogEntry struct {
	ID          string              `json:"id"`
	Method      string              `json:"method"`
	Path        string              `json:"path"`
	Status      int                 `json:"status"`
	Duration    string              `json:"duration"`
	Timestamp   time.Time           `json:"timestamp"`
	MockID      string              `json:"mockId,omitempty"`
	Headers     map[string][]string `json:"headers,omitempty"`
	QueryParams map[string]string   `json:"queryParams,omitempty"`
	Body        string              `json:"body,omitempty"`
}
