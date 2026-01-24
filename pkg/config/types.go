// Package config provides configuration types and utilities for the mock server engine.
package config

import (
	"time"

	"github.com/getmockd/mockd/pkg/audit"
	"github.com/getmockd/mockd/pkg/chaos"
	"github.com/getmockd/mockd/pkg/graphql"
	"github.com/getmockd/mockd/pkg/grpc"
	"github.com/getmockd/mockd/pkg/mock"
	"github.com/getmockd/mockd/pkg/mqtt"
	"github.com/getmockd/mockd/pkg/oauth"
	"github.com/getmockd/mockd/pkg/soap"
	"github.com/getmockd/mockd/pkg/validation"
)

// EntityMeta contains common metadata for all stored entities.
// Embed this in entity types for consistent workspace/sync support.
type EntityMeta struct {
	WorkspaceID string `json:"workspaceId,omitempty" yaml:"workspaceId,omitempty"` // Source workspace, defaults to "local"
	SyncVersion int64  `json:"syncVersion,omitempty" yaml:"syncVersion,omitempty"` // For CRDT/conflict resolution
}

// OrganizationMeta contains folder organization metadata for sortable/folderable entities.
// Embed this in entity types that support folder organization.
type OrganizationMeta struct {
	// ParentID is the folder ID this item belongs to ("" = root level)
	ParentID string `json:"parentId,omitempty" yaml:"parentId,omitempty"`
	// MetaSortKey is used for manual ordering within a folder (negative timestamp = newest first)
	MetaSortKey float64 `json:"metaSortKey,omitempty" yaml:"metaSortKey,omitempty"`
}

// Folder represents an organizational container for grouping mocks and endpoints.
type Folder struct {
	EntityMeta       `json:",inline" yaml:",inline"`
	OrganizationMeta `json:",inline" yaml:",inline"`

	// ID is a unique identifier for the folder (prefixed: fld_xxx)
	ID string `json:"id" yaml:"id"`
	// Name is the display name
	Name string `json:"name" yaml:"name"`
	// Description is an optional longer description
	Description string `json:"description,omitempty" yaml:"description,omitempty"`
	// CreatedAt is when the folder was created
	CreatedAt time.Time `json:"createdAt" yaml:"createdAt"`
	// UpdatedAt is when the folder was last modified
	UpdatedAt time.Time `json:"updatedAt" yaml:"updatedAt"`
}

// MockConfiguration is an alias for mock.Mock for backward compatibility.
// New code should use mock.Mock directly.
type MockConfiguration = mock.Mock

// TLSConfig defines TLS/HTTPS configuration for the server.
type TLSConfig struct {
	// Enabled enables TLS/HTTPS on the server
	Enabled bool `json:"enabled" yaml:"enabled"`
	// CertFile is the path to the TLS certificate file
	CertFile string `json:"certFile,omitempty" yaml:"certFile,omitempty"`
	// KeyFile is the path to the TLS private key file
	KeyFile string `json:"keyFile,omitempty" yaml:"keyFile,omitempty"`
	// AutoGenerateCert enables auto-generation of self-signed certificate
	AutoGenerateCert bool `json:"autoGenerateCert,omitempty" yaml:"autoGenerateCert,omitempty"`
}

// MTLSConfig defines mutual TLS (mTLS) configuration for client certificate authentication.
type MTLSConfig struct {
	// Enabled enables mTLS client certificate verification
	Enabled bool `json:"enabled" yaml:"enabled"`
	// ClientAuth specifies the client authentication policy:
	// - "none": no client certificate requested
	// - "request": client certificate requested but not required
	// - "require": client certificate required but not verified
	// - "verify-if-given": verify client certificate if provided
	// - "require-and-verify": require and verify client certificate
	ClientAuth string `json:"clientAuth,omitempty" yaml:"clientAuth,omitempty"`
	// CACertFile is the path to the CA certificate file for verifying client certificates
	CACertFile string `json:"caCertFile,omitempty" yaml:"caCertFile,omitempty"`
	// CACertFiles is a list of CA certificate file paths for verifying client certificates
	CACertFiles []string `json:"caCertFiles,omitempty" yaml:"caCertFiles,omitempty"`
	// AllowedCNs restricts access to clients with specific Common Names (optional)
	AllowedCNs []string `json:"allowedCNs,omitempty" yaml:"allowedCNs,omitempty"`
	// AllowedOUs restricts access to clients with specific Organizational Units (optional)
	AllowedOUs []string `json:"allowedOUs,omitempty" yaml:"allowedOUs,omitempty"`
}

// ServerConfiguration defines the mock server runtime settings and operational parameters.
type ServerConfiguration struct {
	// HTTPPort is the port for the HTTP server (0 = disabled)
	HTTPPort int `json:"httpPort,omitempty"`
	// HTTPSPort is the port for the HTTPS server (0 = disabled)
	HTTPSPort int `json:"httpsPort,omitempty"`
	// AdminPort is the port for the admin API (required)
	AdminPort int `json:"adminPort"`
	// ManagementPort is the port for the Engine Management API (default: 4281)
	// This is an internal API used by the Admin server to communicate with the engine.
	ManagementPort int `json:"managementPort,omitempty"`
	// TLS configures TLS/HTTPS settings
	TLS *TLSConfig `json:"tls,omitempty" yaml:"tls,omitempty"`
	// MTLS configures mutual TLS client certificate authentication
	MTLS *MTLSConfig `json:"mtls,omitempty" yaml:"mtls,omitempty"`
	// LogRequests enables request logging
	LogRequests bool `json:"logRequests"`
	// MaxLogEntries is the maximum number of request log entries to retain
	MaxLogEntries int `json:"maxLogEntries,omitempty"`
	// MaxBodySize is the maximum request/response body size in bytes
	MaxBodySize int `json:"maxBodySize,omitempty"`
	// ReadTimeout is the HTTP read timeout in seconds
	ReadTimeout int `json:"readTimeout,omitempty"`
	// WriteTimeout is the HTTP write timeout in seconds
	WriteTimeout int `json:"writeTimeout,omitempty"`
	// Audit configures audit logging for request/response tracking
	Audit *audit.AuditConfig `json:"audit,omitempty" yaml:"audit,omitempty"`

	// GraphQL defines GraphQL mock endpoint configurations
	GraphQL []*graphql.GraphQLConfig `json:"graphql,omitempty" yaml:"graphql,omitempty"`
	// GRPC defines gRPC mock endpoint configurations
	GRPC []*grpc.GRPCConfig `json:"grpc,omitempty" yaml:"grpc,omitempty"`
	// OAuth defines OAuth/OIDC mock provider configurations
	OAuth []*oauth.OAuthConfig `json:"oauth,omitempty" yaml:"oauth,omitempty"`
	// SOAP defines SOAP mock endpoint configurations
	SOAP []*soap.SOAPConfig `json:"soap,omitempty" yaml:"soap,omitempty"`
	// Validation configures OpenAPI request/response validation
	Validation *validation.ValidationConfig `json:"validation,omitempty" yaml:"validation,omitempty"`
	// Chaos configures chaos/fault injection for testing resilience
	Chaos *chaos.ChaosConfig `json:"chaos,omitempty" yaml:"chaos,omitempty"`
	// MQTT defines MQTT broker configurations
	MQTT []*mqtt.MQTTConfig `json:"mqtt,omitempty" yaml:"mqtt,omitempty"`
}

// MockCollection is a container for a set of mock configurations, typically loaded from a single config file.
type MockCollection struct {
	// Version is the config format version (e.g., "1.0")
	Version string `json:"version" yaml:"version"`
	// Kind identifies the config type (e.g., "MockCollection")
	Kind string `json:"kind,omitempty" yaml:"kind,omitempty"`
	// Metadata contains collection metadata
	Metadata *CollectionMetadata `json:"metadata,omitempty" yaml:"metadata,omitempty"`
	// Name is the collection name/description (prefer metadata.name for new configs)
	Name string `json:"name,omitempty" yaml:"name,omitempty"`
	// Mocks is an array of mock definitions
	Mocks []*MockConfiguration `json:"mocks" yaml:"mocks"`
	// ServerConfig contains server settings (if embedded)
	ServerConfig *ServerConfiguration `json:"serverConfig,omitempty" yaml:"serverConfig,omitempty"`
	// StatefulResources defines stateful CRUD resources
	StatefulResources []*StatefulResourceConfig `json:"statefulResources,omitempty" yaml:"statefulResources,omitempty"`
	// WebSocketEndpoints defines WebSocket endpoints
	WebSocketEndpoints []*WebSocketEndpointConfig `json:"websocketEndpoints,omitempty" yaml:"websocketEndpoints,omitempty"`
}

// CollectionMetadata contains metadata about a mock collection.
type CollectionMetadata struct {
	// Name is the human-readable name
	Name string `json:"name,omitempty" yaml:"name,omitempty"`
	// Description explains what this collection is for
	Description string `json:"description,omitempty" yaml:"description,omitempty"`
	// Tags are labels for categorization
	Tags []string `json:"tags,omitempty" yaml:"tags,omitempty"`
}

// WebSocketEndpointConfig defines configuration for a WebSocket endpoint.
type WebSocketEndpointConfig struct {
	EntityMeta       `json:",inline" yaml:",inline"`
	OrganizationMeta `json:",inline" yaml:",inline"`

	// ID is a unique identifier for the endpoint
	ID string `json:"id,omitempty" yaml:"id,omitempty"`
	// Name is a human-readable name for the endpoint
	Name string `json:"name,omitempty" yaml:"name,omitempty"`
	// Path is the URL path for WebSocket upgrade (e.g., "/ws/chat")
	Path string `json:"path" yaml:"path"`
	// Subprotocols lists supported subprotocols for negotiation
	Subprotocols []string `json:"subprotocols,omitempty" yaml:"subprotocols,omitempty"`
	// RequireSubprotocol rejects connections without a matching subprotocol
	RequireSubprotocol bool `json:"requireSubprotocol,omitempty" yaml:"requireSubprotocol,omitempty"`
	// Matchers contains message matching rules for conditional responses
	Matchers []*mock.WSMatcherConfig `json:"matchers,omitempty" yaml:"matchers,omitempty"`
	// DefaultResponse is sent when no matcher matches
	DefaultResponse *mock.WSMessageResponse `json:"defaultResponse,omitempty" yaml:"defaultResponse,omitempty"`
	// Scenario defines a scripted message sequence
	Scenario *mock.WSScenarioConfig `json:"scenario,omitempty" yaml:"scenario,omitempty"`
	// Heartbeat configures ping/pong keepalive
	Heartbeat *mock.WSHeartbeatConfig `json:"heartbeat,omitempty" yaml:"heartbeat,omitempty"`
	// MaxMessageSize is the maximum message size in bytes (default: 65536)
	MaxMessageSize int64 `json:"maxMessageSize,omitempty" yaml:"maxMessageSize,omitempty"`
	// IdleTimeout closes connections after inactivity (e.g., "5m")
	IdleTimeout string `json:"idleTimeout,omitempty" yaml:"idleTimeout,omitempty"`
	// MaxConnections limits concurrent connections (default: 0 = unlimited)
	MaxConnections int `json:"maxConnections,omitempty" yaml:"maxConnections,omitempty"`
	// EchoMode enables automatic echo of received messages
	EchoMode *bool `json:"echoMode,omitempty" yaml:"echoMode,omitempty"`
	// Enabled indicates whether the endpoint is active (default: true)
	Enabled *bool `json:"enabled,omitempty" yaml:"enabled,omitempty"`
}

// StatefulResourceConfig defines configuration for a stateful CRUD resource.
type StatefulResourceConfig struct {
	// Name is the unique resource name (e.g., "users", "products")
	Name string `json:"name" yaml:"name"`
	// BasePath is the URL path prefix (e.g., "/api/users")
	BasePath string `json:"basePath" yaml:"basePath"`
	// IDField is the field name for ID (default: "id")
	IDField string `json:"idField,omitempty" yaml:"idField,omitempty"`
	// ParentField is the field name for parent FK in nested resources
	ParentField string `json:"parentField,omitempty" yaml:"parentField,omitempty"`
	// SeedData is the initial data to load on startup/reset
	SeedData []map[string]interface{} `json:"seedData,omitempty" yaml:"seedData,omitempty"`
}

// DefaultServerConfiguration returns a ServerConfiguration with sensible defaults.
func DefaultServerConfiguration() *ServerConfiguration {
	return &ServerConfiguration{
		HTTPPort:       4280,
		HTTPSPort:      0,
		AdminPort:      4290,
		ManagementPort: 4281,
		LogRequests:    true,
		MaxLogEntries:  1000,
		MaxBodySize:    10 * 1024 * 1024, // 10MB
		ReadTimeout:    30,
		WriteTimeout:   30,
	}
}
