package engineclient

import (
	"errors"
	"time"

	"github.com/getmockd/mockd/pkg/config"
)

// ErrNotFound is returned when a resource is not found.
var ErrNotFound = errors.New("not found")

// ErrDuplicate is returned when a resource already exists.
var ErrDuplicate = errors.New("resource already exists")

// DeployRequest is sent to deploy mocks to an engine.
type DeployRequest struct {
	Mocks   []*config.MockConfiguration `json:"mocks"`
	Replace bool                        `json:"replace,omitempty"`
}

// DeployResponse is returned after deployment.
type DeployResponse struct {
	Deployed int    `json:"deployed"`
	Message  string `json:"message,omitempty"`
}

// StatusResponse is the engine status.
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

// ProtocolStatus is the status of a protocol handler.
type ProtocolStatus struct {
	Enabled     bool   `json:"enabled"`
	Port        int    `json:"port,omitempty"`
	Connections int    `json:"connections,omitempty"`
	Status      string `json:"status,omitempty"`
}

// MockListResponse lists mocks.
type MockListResponse struct {
	Mocks []*config.MockConfiguration `json:"mocks"`
	Count int                         `json:"count"`
}

// RequestFilter filters request logs.
type RequestFilter struct {
	Limit    int
	Offset   int
	Protocol string
	Method   string
	Path     string
	MockID   string // Filter by matched mock ID
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

// ErrorResponse is an error from the engine.
type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
}

// ChaosConfig for chaos injection.
type ChaosConfig struct {
	Enabled   bool             `json:"enabled"`
	Latency   *LatencyConfig   `json:"latency,omitempty"`
	ErrorRate *ErrorRateConfig `json:"errorRate,omitempty"`
}

// LatencyConfig configures latency injection.
type LatencyConfig struct {
	Min         string  `json:"min"`
	Max         string  `json:"max"`
	Probability float64 `json:"probability"`
}

// ErrorRateConfig configures error rate injection.
type ErrorRateConfig struct {
	Probability float64 `json:"probability"`
	StatusCodes []int   `json:"statusCodes,omitempty"`
	DefaultCode int     `json:"defaultCode,omitempty"`
}

// StatefulResource represents a stateful resource.
type StatefulResource struct {
	Name        string `json:"name"`
	BasePath    string `json:"basePath"`
	ItemCount   int    `json:"itemCount"`
	SeedCount   int    `json:"seedCount"`
	IDField     string `json:"idField"`
	ParentField string `json:"parentField,omitempty"`
}

// StateOverview provides an overview of all stateful resources.
type StateOverview struct {
	Resources    []StatefulResource `json:"resources"`
	Total        int                `json:"total"`
	TotalItems   int                `json:"totalItems"`
	ResourceList []string           `json:"resourceList"`
}

// ProtocolHandler represents a protocol handler.
type ProtocolHandler struct {
	ID          string `json:"id"`
	Type        string `json:"type"`
	Port        int    `json:"port,omitempty"`
	Path        string `json:"path,omitempty"`
	Status      string `json:"status"`
	Connections int    `json:"connections"`
}

// SSEConnection represents an SSE connection.
type SSEConnection struct {
	ID          string    `json:"id"`
	MockID      string    `json:"mockId"`
	Path        string    `json:"path"`
	ConnectedAt time.Time `json:"connectedAt"`
}

// SSEStats provides SSE statistics.
type SSEStats struct {
	TotalConnections  int `json:"totalConnections"`
	ActiveConnections int `json:"activeConnections"`
}
