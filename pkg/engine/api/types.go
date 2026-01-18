package api

import (
	"time"

	"github.com/getmockd/mockd/pkg/config"
)

// DeployRequest is sent by Admin to deploy mocks to an engine.
type DeployRequest struct {
	Mocks   []*config.MockConfiguration `json:"mocks"`
	Replace bool                        `json:"replace,omitempty"` // Replace all existing mocks
}

// DeployResponse confirms deployment.
type DeployResponse struct {
	Deployed int    `json:"deployed"`
	Message  string `json:"message,omitempty"`
}

// StatusResponse returns engine status.
type StatusResponse struct {
	ID           string                    `json:"id"`
	Name         string                    `json:"name,omitempty"`
	Status       string                    `json:"status"` // "running", "stopped"
	Uptime       int64                     `json:"uptime"` // seconds
	MockCount    int                       `json:"mockCount"`
	RequestCount int64                     `json:"requestCount"`
	Protocols    map[string]ProtocolStatus `json:"protocols"`
	StartedAt    time.Time                 `json:"startedAt"`
}

// ProtocolStatus is status for a single protocol.
type ProtocolStatus struct {
	Enabled     bool   `json:"enabled"`
	Port        int    `json:"port,omitempty"`
	Connections int    `json:"connections,omitempty"`
	Status      string `json:"status,omitempty"`
}

// HealthResponse is a simple health check response.
type HealthResponse struct {
	Status    string    `json:"status"`
	Timestamp time.Time `json:"timestamp"`
}

// MockListResponse lists mocks on the engine.
type MockListResponse struct {
	Mocks []*config.MockConfiguration `json:"mocks"`
	Count int                         `json:"count"`
}

// RequestLogEntry is a logged request.
type RequestLogEntry struct {
	ID            string            `json:"id"`
	Timestamp     time.Time         `json:"timestamp"`
	Protocol      string            `json:"protocol"`
	Method        string            `json:"method,omitempty"`
	Path          string            `json:"path"`
	Headers       map[string]string `json:"headers,omitempty"`
	Body          string            `json:"body,omitempty"`
	MatchedMockID string            `json:"matchedMockId,omitempty"`
	StatusCode    int               `json:"statusCode,omitempty"`
	DurationMs    int               `json:"durationMs"`
}

// RequestListResponse lists request logs.
type RequestListResponse struct {
	Requests []*RequestLogEntry `json:"requests"`
	Count    int                `json:"count"`
	Total    int                `json:"total"`
}

// ErrorResponse is a standard error response.
type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
}

// RequestLogFilter is used to filter request logs.
type RequestLogFilter struct {
	Limit    int    // Maximum number of entries
	Offset   int    // Starting offset for pagination
	Method   string // Filter by HTTP method
	Path     string // Filter by path (substring match)
	MockID   string // Filter by matched mock ID
	Protocol string // Filter by protocol (http, websocket, etc.)
}

// ProtocolStatusInfo contains status information for a protocol.
type ProtocolStatusInfo struct {
	Enabled     bool   `json:"enabled"`
	Port        int    `json:"port,omitempty"`
	Connections int    `json:"connections,omitempty"`
	Status      string `json:"status,omitempty"`
}

// ChaosConfig represents chaos injection configuration for the API.
type ChaosConfig struct {
	Enabled   bool              `json:"enabled"`
	Latency   *LatencyConfig    `json:"latency,omitempty"`
	ErrorRate *ErrorRateConfig  `json:"errorRate,omitempty"`
	Bandwidth *BandwidthConfig  `json:"bandwidth,omitempty"`
	Rules     []ChaosRuleConfig `json:"rules,omitempty"`
}

// LatencyConfig configures latency injection.
type LatencyConfig struct {
	Min         string  `json:"min"`
	Max         string  `json:"max"`
	Probability float64 `json:"probability"`
}

// ErrorRateConfig configures error injection.
type ErrorRateConfig struct {
	Probability float64 `json:"probability"`
	StatusCodes []int   `json:"statusCodes,omitempty"`
	DefaultCode int     `json:"defaultCode,omitempty"`
}

// BandwidthConfig configures bandwidth throttling.
type BandwidthConfig struct {
	BytesPerSecond int     `json:"bytesPerSecond"`
	Probability    float64 `json:"probability"`
}

// ChaosRuleConfig represents a path-specific chaos rule.
type ChaosRuleConfig struct {
	PathPattern string   `json:"pathPattern"`
	Methods     []string `json:"methods,omitempty"`
	Probability float64  `json:"probability,omitempty"`
}

// ChaosStats represents chaos injection statistics.
type ChaosStats struct {
	TotalRequests    int64            `json:"totalRequests"`
	InjectedFaults   int64            `json:"injectedFaults"`
	LatencyInjected  int64            `json:"latencyInjected"`
	ErrorsInjected   int64            `json:"errorsInjected"`
	TimeoutsInjected int64            `json:"timeoutsInjected"`
	FaultsByType     map[string]int64 `json:"faultsByType"`
}

// StatefulResource represents a stateful mock resource for the API.
type StatefulResource struct {
	Name        string `json:"name"`
	BasePath    string `json:"basePath"`
	ItemCount   int    `json:"itemCount"`
	SeedCount   int    `json:"seedCount"`
	IDField     string `json:"idField"`
	ParentField string `json:"parentField,omitempty"`
}

// StateOverview represents an overview of all stateful resources.
type StateOverview struct {
	Resources    []StatefulResource `json:"resources"`
	Total        int                `json:"total"`
	TotalItems   int                `json:"totalItems"`
	ResourceList []string           `json:"resourceList"`
}

// ResetStateRequest is the request body for resetting state.
type ResetStateRequest struct {
	Resource string `json:"resource,omitempty"` // If empty, reset all
}

// ResetStateResponse is the response from a state reset operation.
type ResetStateResponse struct {
	Reset     bool     `json:"reset"`
	Resources []string `json:"resources"`
	Message   string   `json:"message"`
}

// ProtocolHandler represents a running protocol handler.
type ProtocolHandler struct {
	ID          string `json:"id"`
	Type        string `json:"type"` // http, websocket, sse, grpc, mqtt, graphql
	Port        int    `json:"port,omitempty"`
	Path        string `json:"path,omitempty"`
	Status      string `json:"status"`
	Connections int    `json:"connections"`
	Version     string `json:"version,omitempty"`
}

// ProtocolHandlerListResponse lists all protocol handlers.
type ProtocolHandlerListResponse struct {
	Handlers []*ProtocolHandler `json:"handlers"`
	Count    int                `json:"count"`
}

// SSEConnection represents an active SSE connection.
type SSEConnection struct {
	ID          string    `json:"id"`
	MockID      string    `json:"mockId"`
	Path        string    `json:"path"`
	ClientIP    string    `json:"clientIp"`
	UserAgent   string    `json:"userAgent,omitempty"`
	ConnectedAt time.Time `json:"connectedAt"`
	EventsSent  int64     `json:"eventsSent"`
	BytesSent   int64     `json:"bytesSent"`
	Status      string    `json:"status"`
}

// SSEConnectionListResponse lists SSE connections.
type SSEConnectionListResponse struct {
	Connections []*SSEConnection `json:"connections"`
	Count       int              `json:"count"`
}

// SSEStats represents SSE statistics.
type SSEStats struct {
	TotalConnections  int64          `json:"totalConnections"`
	ActiveConnections int            `json:"activeConnections"`
	TotalEventsSent   int64          `json:"totalEventsSent"`
	TotalBytesSent    int64          `json:"totalBytesSent"`
	ConnectionErrors  int64          `json:"connectionErrors"`
	ConnectionsByMock map[string]int `json:"connectionsByMock"`
}

// WebSocketConnection represents an active WebSocket connection.
type WebSocketConnection struct {
	ID            string    `json:"id"`
	MockID        string    `json:"mockId"`
	Path          string    `json:"path"`
	ClientIP      string    `json:"clientIp"`
	ConnectedAt   time.Time `json:"connectedAt"`
	MessagesSent  int64     `json:"messagesSent"`
	MessagesRecv  int64     `json:"messagesRecv"`
	BytesSent     int64     `json:"bytesSent"`
	BytesReceived int64     `json:"bytesReceived"`
	Status        string    `json:"status"`
}

// WebSocketConnectionListResponse lists WebSocket connections.
type WebSocketConnectionListResponse struct {
	Connections []*WebSocketConnection `json:"connections"`
	Count       int                    `json:"count"`
}

// WebSocketStats represents WebSocket statistics.
type WebSocketStats struct {
	TotalConnections  int64          `json:"totalConnections"`
	ActiveConnections int            `json:"activeConnections"`
	TotalMessagesSent int64          `json:"totalMessagesSent"`
	TotalMessagesRecv int64          `json:"totalMessagesRecv"`
	ConnectionsByMock map[string]int `json:"connectionsByMock"`
}

// ConfigResponse represents the server configuration.
type ConfigResponse struct {
	HTTPPort       int `json:"httpPort"`
	HTTPSPort      int `json:"httpsPort,omitempty"`
	ManagementPort int `json:"managementPort"`
	MaxLogEntries  int `json:"maxLogEntries"`
	ReadTimeout    int `json:"readTimeout"`
	WriteTimeout   int `json:"writeTimeout"`
}

// ToggleMockRequest represents a request to toggle a mock's enabled status.
type ToggleMockRequest struct {
	Enabled bool `json:"enabled"`
}

// ImportConfigRequest represents a request to import configuration.
type ImportConfigRequest struct {
	Config  *config.MockCollection `json:"config"`
	Replace bool                   `json:"replace,omitempty"`
}
