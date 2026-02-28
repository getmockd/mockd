// Package types provides shared API types used across admin and engine packages.
// This eliminates duplicate type definitions and ensures consistent API contracts.
package types

import (
	"encoding/json"
	"time"

	"github.com/getmockd/mockd/pkg/config"
	"github.com/getmockd/mockd/pkg/requestlog"
)

// --- General Responses ---

// ErrorResponse is a standard error response used across all APIs.
type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
	Details any    `json:"details,omitempty"`
}

// HealthResponse is a simple health check response.
type HealthResponse struct {
	Status    string    `json:"status"`
	Uptime    int       `json:"uptime,omitempty"`
	Timestamp time.Time `json:"timestamp,omitempty"`
}

// MessageResponse is a simple message response.
type MessageResponse struct {
	Message string `json:"message"`
}

// CountResponse is a response with a count field.
type CountResponse struct {
	Message string `json:"message,omitempty"`
	Count   int    `json:"count,omitempty"`
	Cleared int    `json:"cleared,omitempty"`
}

// ToggleRequest represents a request to toggle an item's enabled status.
type ToggleRequest struct {
	Enabled bool `json:"enabled"`
}

// PaginatedResponse is a generic paginated response wrapper.
type PaginatedResponse[T any] struct {
	Items  []T `json:"items"`
	Count  int `json:"count"`
	Total  int `json:"total,omitempty"`
	Offset int `json:"offset,omitempty"`
	Limit  int `json:"limit,omitempty"`
}

// PaginationMeta contains pagination metadata for collection responses.
type PaginationMeta struct {
	Total  int `json:"total"`
	Limit  int `json:"limit"`
	Offset int `json:"offset"`
	Count  int `json:"count"`
}

// --- Server Status ---

// ProtocolStatus represents status for a single protocol.
type ProtocolStatus struct {
	Enabled     bool   `json:"enabled"`
	Port        int    `json:"port,omitempty"`
	Connections int    `json:"connections,omitempty"`
	Status      string `json:"status,omitempty"`
}

// ServerStatus represents detailed server status.
type ServerStatus struct {
	Status       string                    `json:"status"`
	ID           string                    `json:"id,omitempty"`
	Name         string                    `json:"name,omitempty"`
	HTTPPort     int                       `json:"httpPort"`
	HTTPSPort    int                       `json:"httpsPort,omitempty"`
	AdminPort    int                       `json:"adminPort,omitempty"`
	Uptime       int64                     `json:"uptime"`
	MockCount    int                       `json:"mockCount"`
	ActiveMocks  int                       `json:"activeMocks,omitempty"`
	RequestCount int64                     `json:"requestCount"`
	TLSEnabled   bool                      `json:"tlsEnabled,omitempty"`
	Version      string                    `json:"version,omitempty"`
	Protocols    map[string]ProtocolStatus `json:"protocols,omitempty"`
	StartedAt    time.Time                 `json:"startedAt,omitempty"`
}

// StatusResponse returns engine status (used by engine control API).
type StatusResponse struct {
	ID           string                    `json:"id"`
	Name         string                    `json:"name,omitempty"`
	Status       string                    `json:"status"`
	Uptime       int64                     `json:"uptime"`
	MockCount    int                       `json:"mockCount"`
	RequestCount int64                     `json:"requestCount"`
	Protocols    map[string]ProtocolStatus `json:"protocols"`
	StartedAt    time.Time                 `json:"startedAt"`
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

// --- Mock Types ---

// MockListResponse lists mocks with count.
type MockListResponse struct {
	Mocks []*config.MockConfiguration `json:"mocks"`
	Count int                         `json:"count"`
}

// DeployRequest is sent to deploy mocks to an engine.
type DeployRequest struct {
	Mocks   []*config.MockConfiguration `json:"mocks"`
	Replace bool                        `json:"replace,omitempty"`
}

// DeployResponse confirms deployment.
type DeployResponse struct {
	Deployed int    `json:"deployed"`
	Message  string `json:"message,omitempty"`
}

// --- Request Logs ---

// RequestLogEntry represents a logged request.
// All fields from requestlog.Entry are preserved end-to-end so that protocol-
// specific metadata (gRPC service, MQTT topic, GraphQL operation, etc.) is
// visible to consumers of the Admin API and Engine Control API.
type RequestLogEntry struct {
	ID            string              `json:"id"`
	Timestamp     time.Time           `json:"timestamp"`
	Protocol      string              `json:"protocol"`
	Method        string              `json:"method,omitempty"`
	Path          string              `json:"path"`
	QueryString   string              `json:"queryString,omitempty"`
	Headers       map[string][]string `json:"headers,omitempty"`
	Body          string              `json:"body,omitempty"`
	BodySize      int                 `json:"bodySize,omitempty"`
	RemoteAddr    string              `json:"remoteAddr,omitempty"`
	MatchedMockID string              `json:"matchedMockId,omitempty"`
	StatusCode    int                 `json:"statusCode,omitempty"`
	ResponseBody  string              `json:"responseBody,omitempty"`
	DurationMs    int                 `json:"durationMs"`
	Error         string              `json:"error,omitempty"`

	// Near-miss debugging data (populated for unmatched requests).
	NearMisses []requestlog.NearMissInfo `json:"nearMisses,omitempty"`

	// Protocol-specific metadata (only one populated based on Protocol).
	GRPC      *requestlog.GRPCMeta      `json:"grpc,omitempty"`
	WebSocket *requestlog.WebSocketMeta `json:"websocket,omitempty"`
	SSE       *requestlog.SSEMeta       `json:"sse,omitempty"`
	MQTT      *requestlog.MQTTMeta      `json:"mqtt,omitempty"`
	SOAP      *requestlog.SOAPMeta      `json:"soap,omitempty"`
	GraphQL   *requestlog.GraphQLMeta   `json:"graphql,omitempty"`
}

// RequestListResponse lists request logs.
type RequestListResponse struct {
	Requests []*RequestLogEntry `json:"requests"`
	Count    int                `json:"count"`
	Total    int                `json:"total"`
}

// RequestLogFilter is deprecated — use requestlog.Filter directly.
// Kept temporarily for backwards compatibility with external consumers.
type RequestLogFilter = requestlog.Filter

// --- Import / Export ---

// ImportConfigRequest represents a request to import configuration.
type ImportConfigRequest struct {
	Config  *config.MockCollection `json:"config"`
	Replace bool                   `json:"replace,omitempty"`
}

// ImportConfigResponse represents the response from a config import.
type ImportConfigResponse struct {
	Message           string `json:"message"`
	Imported          int    `json:"imported"`
	Total             int    `json:"total"`
	StatefulResources int    `json:"statefulResources,omitempty"`
}

// --- Chaos Injection ---

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
	PathPattern string             `json:"pathPattern"`
	Methods     []string           `json:"methods,omitempty"`
	Faults      []ChaosFaultConfig `json:"faults,omitempty"`
	Probability float64            `json:"probability,omitempty"`
}

// ChaosFaultConfig represents a fault within a chaos rule.
type ChaosFaultConfig struct {
	Type        string         `json:"type"`
	Probability float64        `json:"probability"`
	Config      map[string]any `json:"config,omitempty"`
}

// knownFaultFields lists the JSON keys that map to struct fields on ChaosFaultConfig.
// Everything else is treated as a fault-specific config parameter and gets merged
// into the Config map so users don't have to nest them under "config".
var knownFaultFields = map[string]bool{
	"type":        true,
	"probability": true,
	"config":      true,
}

// UnmarshalJSON accepts both the canonical format (config params nested under "config")
// and the flat format (config params alongside "type" and "probability").
//
// Canonical:  {"type":"circuit_breaker","probability":1.0,"config":{"tripAfter":3}}
// Flat:       {"type":"circuit_breaker","probability":1.0,"tripAfter":3}
//
// When both forms are present for the same key, the explicit "config" value wins.
func (f *ChaosFaultConfig) UnmarshalJSON(data []byte) error {
	// Use an alias to avoid infinite recursion.
	type Alias ChaosFaultConfig
	var alias Alias
	if err := json.Unmarshal(data, &alias); err != nil {
		return err
	}

	// Decode the full blob to catch any extra keys.
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	// Collect unknown top-level keys as config params.
	var extras map[string]any
	for key, val := range raw {
		if knownFaultFields[key] {
			continue
		}
		if extras == nil {
			extras = make(map[string]any)
		}
		var decoded any
		if err := json.Unmarshal(val, &decoded); err != nil {
			return err
		}
		extras[key] = decoded
	}

	// Merge: extras form the base, explicit "config" keys override.
	if len(extras) > 0 {
		if alias.Config == nil {
			alias.Config = extras
		} else {
			// Explicit "config" wins — layer it on top of the extras.
			for k, v := range extras {
				if _, exists := alias.Config[k]; !exists {
					alias.Config[k] = v
				}
			}
		}
	}

	*f = ChaosFaultConfig(alias)
	return nil
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

// StatefulFaultStats contains stats for all stateful chaos faults.
type StatefulFaultStats struct {
	CircuitBreakers         map[string]CircuitBreakerStatus         `json:"circuitBreakers,omitempty"`
	RetryAfterTrackers      map[string]RetryAfterStatus             `json:"retryAfterTrackers,omitempty"`
	ProgressiveDegradations map[string]ProgressiveDegradationStatus `json:"progressiveDegradations,omitempty"`
}

// CircuitBreakerStatus represents the current state of a circuit breaker.
type CircuitBreakerStatus struct {
	State                string `json:"state"`
	ConsecutiveFailures  int    `json:"consecutiveFailures"`
	ConsecutiveSuccesses int    `json:"consecutiveSuccesses"`
	TotalRequests        int64  `json:"totalRequests"`
	TotalTrips           int64  `json:"totalTrips"`
	TotalRejected        int64  `json:"totalRejected"`
	TotalPassed          int64  `json:"totalPassed"`
	TotalHalfOpen        int64  `json:"totalHalfOpen"`
	StateChanges         int64  `json:"stateChanges"`
	OpenedAt             string `json:"openedAt,omitempty"`
}

// RetryAfterStatus represents the current state of a retry-after tracker.
type RetryAfterStatus struct {
	IsLimited    bool   `json:"isLimited"`
	StatusCode   int    `json:"statusCode"`
	RetryAfterMs int64  `json:"retryAfterMs"`
	TotalLimited int64  `json:"totalLimited"`
	TotalPassed  int64  `json:"totalPassed"`
	LimitedAt    string `json:"limitedAt,omitempty"`
}

// ProgressiveDegradationStatus represents the state of a progressive degradation tracker.
type ProgressiveDegradationStatus struct {
	RequestCount   int64 `json:"requestCount"`
	CurrentDelayMs int64 `json:"currentDelayMs"`
	MaxDelayMs     int64 `json:"maxDelayMs"`
	ErrorAfter     int   `json:"errorAfter"`
	ResetAfter     int   `json:"resetAfter"`
	TotalErrors    int64 `json:"totalErrors"`
	TotalResets    int64 `json:"totalResets"`
	IsErroring     bool  `json:"isErroring"`
}

// --- Stateful Resources ---

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
	Resource string `json:"resource,omitempty"`
}

// ResetStateResponse is the response from a state reset operation.
type ResetStateResponse struct {
	Reset     bool     `json:"reset"`
	Resources []string `json:"resources"`
	Message   string   `json:"message"`
}

// StatefulItemsResponse is the paginated response for listing items in a stateful resource.
type StatefulItemsResponse struct {
	Data []map[string]interface{} `json:"data"`
	Meta PaginationMeta           `json:"meta"`
}

// --- Custom Operations ---

// CustomOperationInfo is a summary of a registered custom operation.
type CustomOperationInfo struct {
	Name        string `json:"name"`
	StepCount   int    `json:"stepCount"`
	Consistency string `json:"consistency,omitempty"`
}

// CustomOperationDetail is the full definition of a custom operation.
type CustomOperationDetail struct {
	Name        string                `json:"name"`
	Consistency string                `json:"consistency,omitempty"`
	Steps       []CustomOperationStep `json:"steps"`
	Response    map[string]string     `json:"response,omitempty"`
}

// CustomOperationStep describes a single step in a custom operation.
type CustomOperationStep struct {
	Type     string            `json:"type"`
	Resource string            `json:"resource,omitempty"`
	ID       string            `json:"id,omitempty"`
	As       string            `json:"as,omitempty"`
	Set      map[string]string `json:"set,omitempty"`
	Var      string            `json:"var,omitempty"`
	Value    string            `json:"value,omitempty"`
}

// --- Protocol Handlers ---

// ProtocolHandler represents a running protocol handler.
type ProtocolHandler struct {
	ID          string `json:"id"`
	Type        string `json:"type"`
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

// --- SSE ---

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

// --- WebSocket ---

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
