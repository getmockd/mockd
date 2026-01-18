// Package audit provides audit logging infrastructure for the mock server engine.
// It captures detailed request/response information for debugging, compliance,
// and observability purposes.
package audit

import (
	"net/http"
	"time"
)

// Event constants define the types of events that can be logged.
const (
	EventRequestReceived  = "request.received"
	EventResponseSent     = "response.sent"
	EventMockMatched      = "mock.matched"
	EventMockNotFound     = "mock.not_found"
	EventProxyForwarded   = "proxy.forwarded"
	EventProxyResponse    = "proxy.response"
	EventWebSocketOpen    = "websocket.open"
	EventWebSocketClose   = "websocket.close"
	EventWebSocketMessage = "websocket.message"
	EventSSEStreamStart   = "sse.stream_start"
	EventSSEStreamEnd     = "sse.stream_end"
	EventSSEEventSent     = "sse.event_sent"
	EventError            = "error"
)

// AuditEntry represents a single audit log record capturing an event
// that occurred during request processing.
type AuditEntry struct {
	// Sequence is a monotonically increasing sequence number for ordering entries.
	Sequence int64 `json:"sequence"`

	// Timestamp is when the event occurred.
	Timestamp time.Time `json:"timestamp"`

	// TraceID is a unique identifier that correlates related events
	// across a single request/response cycle.
	TraceID string `json:"traceId"`

	// Event is the type of event being logged (e.g., "request.received").
	Event string `json:"event"`

	// Request contains information about the incoming HTTP request.
	Request *RequestInfo `json:"request,omitempty"`

	// Response contains information about the outgoing HTTP response.
	Response *ResponseInfo `json:"response,omitempty"`

	// Mock contains information about the matched mock configuration.
	Mock *MockInfo `json:"mock,omitempty"`

	// Client contains information about the client making the request.
	Client *ClientInfo `json:"client,omitempty"`

	// Metadata contains additional contextual information.
	Metadata *EntryMetadata `json:"metadata,omitempty"`
}

// RequestInfo captures details about an incoming HTTP request.
type RequestInfo struct {
	// Method is the HTTP method (GET, POST, etc.).
	Method string `json:"method"`

	// Path is the request URL path.
	Path string `json:"path"`

	// Query is the raw query string.
	Query string `json:"query,omitempty"`

	// Headers are the request headers.
	// Note: Sensitive headers may be redacted in future versions.
	Headers http.Header `json:"headers,omitempty"`

	// BodySize is the size of the request body in bytes.
	BodySize int64 `json:"bodySize,omitempty"`

	// BodyPreview is a truncated preview of the request body.
	// Large bodies are truncated to prevent log bloat.
	BodyPreview string `json:"bodyPreview,omitempty"`

	// ContentType is the Content-Type header value.
	ContentType string `json:"contentType,omitempty"`
}

// ResponseInfo captures details about an outgoing HTTP response.
type ResponseInfo struct {
	// StatusCode is the HTTP status code.
	StatusCode int `json:"statusCode"`

	// Headers are the response headers.
	Headers http.Header `json:"headers,omitempty"`

	// BodySize is the size of the response body in bytes.
	BodySize int64 `json:"bodySize,omitempty"`

	// BodyPreview is a truncated preview of the response body.
	BodyPreview string `json:"bodyPreview,omitempty"`

	// ContentType is the Content-Type header value.
	ContentType string `json:"contentType,omitempty"`

	// DurationMs is the time taken to generate the response in milliseconds.
	DurationMs int64 `json:"durationMs,omitempty"`
}

// MockInfo captures details about the matched mock configuration.
type MockInfo struct {
	// ID is the unique identifier of the matched mock.
	ID string `json:"id"`

	// Name is the human-readable name of the mock.
	Name string `json:"name,omitempty"`

	// Priority is the mock's priority value.
	Priority int `json:"priority,omitempty"`

	// MatchScore indicates how well the request matched (for debugging).
	MatchScore float64 `json:"matchScore,omitempty"`

	// MatchedCriteria lists which matcher criteria were satisfied.
	MatchedCriteria []string `json:"matchedCriteria,omitempty"`
}

// ClientInfo captures details about the client making the request.
type ClientInfo struct {
	// RemoteAddr is the client's IP address and port.
	RemoteAddr string `json:"remoteAddr"`

	// UserAgent is the User-Agent header value.
	UserAgent string `json:"userAgent,omitempty"`

	// TLS indicates whether the connection used TLS.
	TLS bool `json:"tls,omitempty"`

	// TLSVersion is the TLS protocol version (e.g., "TLS 1.3").
	TLSVersion string `json:"tlsVersion,omitempty"`

	// ClientCertCN is the Common Name from the client certificate (mTLS).
	ClientCertCN string `json:"clientCertCn,omitempty"`
}

// EntryMetadata contains additional contextual information for an audit entry.
type EntryMetadata struct {
	// ServerID identifies the mock server instance.
	ServerID string `json:"serverId,omitempty"`

	// SessionID identifies the client session (if applicable).
	SessionID string `json:"sessionId,omitempty"`

	// DeploymentID identifies the deployment configuration.
	DeploymentID string `json:"deploymentId,omitempty"`

	// Error contains error details if the event represents an error.
	Error *ErrorInfo `json:"error,omitempty"`

	// Tags are arbitrary key-value pairs for additional context.
	Tags map[string]string `json:"tags,omitempty"`

	// Duration is the total processing time for the request in nanoseconds.
	Duration int64 `json:"duration,omitempty"`
}

// ErrorInfo captures details about an error that occurred.
type ErrorInfo struct {
	// Code is a machine-readable error code.
	Code string `json:"code,omitempty"`

	// Message is a human-readable error description.
	Message string `json:"message"`

	// Details contains additional error context.
	Details map[string]interface{} `json:"details,omitempty"`
}

// NewAuditEntry creates a new AuditEntry with the current timestamp.
func NewAuditEntry(event string, traceID string) *AuditEntry {
	return &AuditEntry{
		Timestamp: time.Now(),
		TraceID:   traceID,
		Event:     event,
	}
}

// WithRequest adds request information to the audit entry.
func (e *AuditEntry) WithRequest(req *RequestInfo) *AuditEntry {
	e.Request = req
	return e
}

// WithResponse adds response information to the audit entry.
func (e *AuditEntry) WithResponse(resp *ResponseInfo) *AuditEntry {
	e.Response = resp
	return e
}

// WithMock adds mock information to the audit entry.
func (e *AuditEntry) WithMock(mock *MockInfo) *AuditEntry {
	e.Mock = mock
	return e
}

// WithClient adds client information to the audit entry.
func (e *AuditEntry) WithClient(client *ClientInfo) *AuditEntry {
	e.Client = client
	return e
}

// WithMetadata adds metadata to the audit entry.
func (e *AuditEntry) WithMetadata(meta *EntryMetadata) *AuditEntry {
	e.Metadata = meta
	return e
}
