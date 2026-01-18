package protocol

import (
	"context"
	"time"
)

// Handler is the base interface that ALL protocol handlers must implement.
// This provides the universal contract for identification and lifecycle management.
//
// Every protocol handler (gRPC, MQTT, WebSocket, SSE, GraphQL, SOAP, etc.)
// must implement this interface to be managed by the engine and exposed
// through the Admin API.
type Handler interface {
	// Metadata returns descriptive information about the handler.
	// This includes the unique ID, protocol type, version, and capabilities.
	Metadata() Metadata

	// Start activates the handler.
	// For standalone servers (gRPC, MQTT), this starts listening on a port.
	// For HTTP-based handlers (GraphQL, SOAP, SSE), this prepares for serving.
	// The context can be used for cancellation during startup.
	Start(ctx context.Context) error

	// Stop gracefully shuts down the handler.
	// Implementations should drain connections, flush buffers, and
	// release resources within the timeout period.
	// If the timeout expires, the handler should force shutdown.
	Stop(ctx context.Context, timeout time.Duration) error

	// Health returns the current health status of the handler.
	// This is used by the Admin API for health check endpoints and
	// by the engine for monitoring handler state.
	Health(ctx context.Context) HealthStatus
}

// Metadata provides descriptive information about a protocol handler.
// This struct is returned by Handler.Metadata() and used by the Admin API
// to expose handler information and capabilities.
type Metadata struct {
	// ID is the unique identifier for this handler instance.
	// This is required and must be unique within a registry.
	ID string `json:"id"`

	// Name is a human-readable name for display purposes.
	// Optional - if empty, the ID may be used for display.
	Name string `json:"name,omitempty"`

	// Protocol identifies the protocol type (grpc, mqtt, websocket, etc.).
	Protocol Protocol `json:"protocol"`

	// Version is the handler implementation version.
	// Optional - useful for tracking handler updates.
	Version string `json:"version,omitempty"`

	// Capabilities lists the features this handler supports.
	// These are used for capability detection via type assertions
	// and for filtering handlers in the Admin API.
	Capabilities []Capability `json:"capabilities"`

	// TransportType indicates the underlying transport mechanism.
	TransportType TransportType `json:"transportType"`

	// ConnectionModel describes the connection lifecycle pattern.
	ConnectionModel ConnectionModel `json:"connectionModel"`

	// CommunicationPattern describes the message flow pattern.
	CommunicationPattern CommunicationPattern `json:"communicationPattern"`
}

// HasCapability returns true if the metadata includes the given capability.
// This is useful for checking capabilities without type assertions.
func (m Metadata) HasCapability(cap Capability) bool {
	for _, c := range m.Capabilities {
		if c == cap {
			return true
		}
	}
	return false
}

// HealthStatus represents the health of a handler.
// Returned by Handler.Health() and used by Admin API health endpoints.
type HealthStatus struct {
	// Status is the overall health state (healthy, degraded, unhealthy, unknown).
	Status HealthState `json:"status"`

	// Message provides additional context about the health status.
	// Optional - useful for explaining degraded or unhealthy states.
	Message string `json:"message,omitempty"`

	// CheckedAt is when the health check was performed.
	CheckedAt time.Time `json:"checkedAt"`

	// Details contains protocol-specific health information.
	// Optional - can include connection counts, queue depths, etc.
	Details any `json:"details,omitempty"`
}

// HealthState is the health status enum.
type HealthState string

// HealthState constants for all possible health states.
const (
	// HealthHealthy indicates the handler is fully operational.
	HealthHealthy HealthState = "healthy"

	// HealthDegraded indicates the handler is operational but with issues.
	// For example, high latency or some failed connections.
	HealthDegraded HealthState = "degraded"

	// HealthUnhealthy indicates the handler is not operational.
	// For example, cannot accept connections or process requests.
	HealthUnhealthy HealthState = "unhealthy"

	// HealthUnknown indicates the health status cannot be determined.
	// For example, health check timed out or threw an error.
	HealthUnknown HealthState = "unknown"
)

// String returns the string representation of the health state.
func (h HealthState) String() string {
	return string(h)
}
