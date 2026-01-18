package requestlog

import "time"

// Protocol constants for request logging.
const (
	ProtocolHTTP      = "http"
	ProtocolGRPC      = "grpc"
	ProtocolWebSocket = "websocket"
	ProtocolSSE       = "sse"
	ProtocolMQTT      = "mqtt"
	ProtocolSOAP      = "soap"
	ProtocolGraphQL   = "graphql"
)

// Entry captures complete details of a request/response for debugging and inspection.
// Supports multiple protocols: HTTP, gRPC, WebSocket, SSE, MQTT, SOAP, GraphQL.
type Entry struct {
	// ID is a unique identifier for the log entry.
	ID string `json:"id"`

	// Timestamp is when the request was received.
	Timestamp time.Time `json:"timestamp"`

	// Protocol identifies the protocol type (http, grpc, websocket, sse, mqtt, soap, graphql).
	Protocol string `json:"protocol"`

	// Method is the HTTP method (or gRPC method name, MQTT topic, etc.).
	Method string `json:"method"`

	// Path is the request URL path (or gRPC service/method, MQTT topic, etc.).
	Path string `json:"path"`

	// QueryString is the raw query string (HTTP only).
	QueryString string `json:"queryString,omitempty"`

	// Headers are the request headers/metadata (multi-value).
	Headers map[string][]string `json:"headers,omitempty"`

	// Body is the request body content (truncated if > 10KB).
	Body string `json:"body,omitempty"`

	// BodySize is the original body size in bytes.
	BodySize int `json:"bodySize"`

	// RemoteAddr is the client IP address.
	RemoteAddr string `json:"remoteAddr"`

	// MatchedMockID is the ID of mock that matched (empty if no match).
	MatchedMockID string `json:"matchedMockID,omitempty"`

	// ResponseStatus is the status code returned (HTTP status, gRPC code, etc.).
	ResponseStatus int `json:"responseStatus"`

	// ResponseBody is the response body content (truncated if > 10KB).
	ResponseBody string `json:"responseBody,omitempty"`

	// DurationMs is the request processing time in milliseconds.
	DurationMs int `json:"durationMs"`

	// Error contains error message if the request failed.
	Error string `json:"error,omitempty"`

	// Protocol-specific metadata (only one will be populated based on Protocol).
	GRPC      *GRPCMeta      `json:"grpc,omitempty"`
	WebSocket *WebSocketMeta `json:"websocket,omitempty"`
	SSE       *SSEMeta       `json:"sse,omitempty"`
	MQTT      *MQTTMeta      `json:"mqtt,omitempty"`
	SOAP      *SOAPMeta      `json:"soap,omitempty"`
	GraphQL   *GraphQLMeta   `json:"graphql,omitempty"`
}
