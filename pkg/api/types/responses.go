// Package types provides shared API types used across admin and engine packages.
// This eliminates duplicate type definitions and ensures consistent API contracts.
package types

import (
	"time"

	"github.com/getmockd/mockd/pkg/config"
)

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

// MockListResponse lists mocks with count.
type MockListResponse struct {
	Mocks []*config.MockConfiguration `json:"mocks"`
	Count int                         `json:"count"`
}

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

// PaginatedResponse is a generic paginated response wrapper.
type PaginatedResponse[T any] struct {
	Items  []T `json:"items"`
	Count  int `json:"count"`
	Total  int `json:"total,omitempty"`
	Offset int `json:"offset,omitempty"`
	Limit  int `json:"limit,omitempty"`
}

// RequestLogEntry represents a logged request.
type RequestLogEntry struct {
	ID            string            `json:"id"`
	Timestamp     time.Time         `json:"timestamp"`
	Protocol      string            `json:"protocol"`
	Method        string            `json:"method,omitempty"`
	Path          string            `json:"path"`
	QueryString   string            `json:"queryString,omitempty"`
	Headers       map[string]string `json:"headers,omitempty"`
	Body          string            `json:"body,omitempty"`
	BodySize      int               `json:"bodySize,omitempty"`
	RemoteAddr    string            `json:"remoteAddr,omitempty"`
	MatchedMockID string            `json:"matchedMockId,omitempty"`
	StatusCode    int               `json:"statusCode,omitempty"`
	DurationMs    int               `json:"durationMs"`
	Error         string            `json:"error,omitempty"`
}

// RequestListResponse lists request logs.
type RequestListResponse struct {
	Requests []*RequestLogEntry `json:"requests"`
	Count    int                `json:"count"`
	Total    int                `json:"total"`
}

// RequestLogFilter is used to filter request logs.
type RequestLogFilter struct {
	Limit    int    `json:"limit,omitempty"`
	Offset   int    `json:"offset,omitempty"`
	Method   string `json:"method,omitempty"`
	Path     string `json:"path,omitempty"`
	MockID   string `json:"mockId,omitempty"`
	Protocol string `json:"protocol,omitempty"`
}

// ToggleRequest represents a request to toggle an item's enabled status.
type ToggleRequest struct {
	Enabled bool `json:"enabled"`
}

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
