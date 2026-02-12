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

// CORSConfig defines Cross-Origin Resource Sharing settings.
type CORSConfig struct {
	// Enabled enables CORS handling. When false, no CORS headers are added.
	// Default: true (for development convenience)
	Enabled bool `json:"enabled" yaml:"enabled"`
	// AllowOrigins specifies allowed origins. Use "*" for any origin (not recommended for production).
	// Empty list defaults to localhost origins only.
	// Examples: ["https://example.com", "http://localhost:3000"]
	AllowOrigins []string `json:"allowOrigins,omitempty" yaml:"allowOrigins,omitempty"`
	// AllowMethods specifies allowed HTTP methods.
	// Default: ["GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS", "HEAD"]
	AllowMethods []string `json:"allowMethods,omitempty" yaml:"allowMethods,omitempty"`
	// AllowHeaders specifies allowed request headers.
	// Default: ["Content-Type", "Authorization", "X-Requested-With", "Accept", "Origin"]
	AllowHeaders []string `json:"allowHeaders,omitempty" yaml:"allowHeaders,omitempty"`
	// ExposeHeaders specifies headers that browsers are allowed to access.
	ExposeHeaders []string `json:"exposeHeaders,omitempty" yaml:"exposeHeaders,omitempty"`
	// AllowCredentials indicates whether credentials are allowed.
	// Cannot be used with AllowOrigins: ["*"]
	AllowCredentials bool `json:"allowCredentials,omitempty" yaml:"allowCredentials,omitempty"`
	// MaxAge is the preflight cache duration in seconds. Default: 86400 (24 hours)
	MaxAge int `json:"maxAge,omitempty" yaml:"maxAge,omitempty"`
}

// RateLimitConfig defines rate limiting settings for the mock engine.
type RateLimitConfig struct {
	// Enabled enables rate limiting. Default: false
	Enabled bool `json:"enabled" yaml:"enabled"`
	// RequestsPerSecond is the rate limit (tokens per second). Default: 1000
	RequestsPerSecond float64 `json:"requestsPerSecond,omitempty" yaml:"requestsPerSecond,omitempty"`
	// BurstSize is the maximum burst size. Default: 2000
	BurstSize int `json:"burstSize,omitempty" yaml:"burstSize,omitempty"`
	// MaxBuckets is the maximum number of per-IP buckets tracked concurrently.
	// Limits memory usage under high cardinality / spoofed source attacks.
	// Default: 10000 (from ratelimit.DefaultMaxBuckets)
	MaxBuckets int `json:"maxBuckets,omitempty" yaml:"maxBuckets,omitempty"`
	// TrustedProxies is a list of CIDR ranges or IPs for trusted proxies.
	// When set, X-Forwarded-For headers are trusted from these sources.
	TrustedProxies []string `json:"trustedProxies,omitempty" yaml:"trustedProxies,omitempty"`
}

// ServerConfiguration defines the mock server runtime settings and operational parameters.
type ServerConfiguration struct {
	// HTTPPort is the port for the HTTP server (0 = disabled)
	HTTPPort int `json:"httpPort,omitempty" yaml:"httpPort,omitempty"`
	// HTTPSPort is the port for the HTTPS server (0 = disabled)
	HTTPSPort int `json:"httpsPort,omitempty" yaml:"httpsPort,omitempty"`
	// AdminPort is the port for the admin API (required)
	AdminPort int `json:"adminPort" yaml:"adminPort"`
	// ManagementPort is the port for the Engine Management API (default: 4281)
	// This is an internal API used by the Admin server to communicate with the engine.
	ManagementPort int `json:"managementPort,omitempty" yaml:"managementPort,omitempty"`
	// TLS configures TLS/HTTPS settings
	TLS *TLSConfig `json:"tls,omitempty" yaml:"tls,omitempty"`
	// MTLS configures mutual TLS client certificate authentication
	MTLS *MTLSConfig `json:"mtls,omitempty" yaml:"mtls,omitempty"`
	// CORS configures Cross-Origin Resource Sharing. Default allows localhost only.
	CORS *CORSConfig `json:"cors,omitempty" yaml:"cors,omitempty"`
	// RateLimit configures rate limiting for the mock engine. Default: disabled.
	RateLimit *RateLimitConfig `json:"rateLimit,omitempty" yaml:"rateLimit,omitempty"`
	// LogRequests enables request logging
	LogRequests bool `json:"logRequests" yaml:"logRequests"`
	// MaxLogEntries is the maximum number of request log entries to retain
	MaxLogEntries int `json:"maxLogEntries,omitempty" yaml:"maxLogEntries,omitempty"`
	// MaxBodySize is the maximum request/response body size in bytes
	MaxBodySize int `json:"maxBodySize,omitempty" yaml:"maxBodySize,omitempty"`
	// ReadTimeout is the HTTP read timeout in seconds
	ReadTimeout int `json:"readTimeout,omitempty" yaml:"readTimeout,omitempty"`
	// WriteTimeout is the HTTP write timeout in seconds
	WriteTimeout int `json:"writeTimeout,omitempty" yaml:"writeTimeout,omitempty"`
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
	// SkipOriginVerify skips verification of the Origin header during WebSocket handshake.
	// Default: true (allows any origin for development/testing convenience).
	// Set to false to enforce that Origin matches the Host header.
	SkipOriginVerify *bool `json:"skipOriginVerify,omitempty" yaml:"skipOriginVerify,omitempty"`
}

// StatefulResourceConfig defines configuration for a stateful CRUD resource.
// This is the single canonical type used in YAML config, persistence, and API transport.
type StatefulResourceConfig struct {
	// Name is the unique resource name (e.g., "users", "products")
	Name string `json:"name" yaml:"name"`
	// Workspace is the workspace this resource belongs to (YAML config only, not persisted)
	Workspace string `json:"workspace,omitempty" yaml:"workspace,omitempty"`
	// BasePath is the URL path prefix (e.g., "/api/users")
	BasePath string `json:"basePath" yaml:"basePath"`
	// IDField is the field name for ID (default: "id")
	IDField string `json:"idField,omitempty" yaml:"idField,omitempty"`
	// ParentField is the field name for parent FK in nested resources
	ParentField string `json:"parentField,omitempty" yaml:"parentField,omitempty"`
	// MaxItems limits the number of items this resource can hold (0 = unlimited).
	// When the limit is reached, Create operations return an error.
	MaxItems int `json:"maxItems,omitempty" yaml:"maxItems,omitempty"`
	// SeedData is the initial data to load on startup/reset
	SeedData []map[string]interface{} `json:"seedData,omitempty" yaml:"seedData,omitempty"`
	// Validation defines validation rules for CRUD operations
	Validation *validation.StatefulValidation `json:"validation,omitempty" yaml:"validation,omitempty"`
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
		CORS:           DefaultCORSConfig(),
		RateLimit:      nil, // Rate limiting disabled by default
	}
}

// DefaultCORSConfig returns a CORSConfig with secure defaults (localhost only).
func DefaultCORSConfig() *CORSConfig {
	return &CORSConfig{
		Enabled: true,
		AllowOrigins: []string{
			"http://localhost:3000",
			"http://localhost:4290",
			"http://localhost:5173",
			"http://127.0.0.1:3000",
			"http://127.0.0.1:4290",
			"http://127.0.0.1:5173",
		},
		AllowMethods: []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS", "HEAD"},
		AllowHeaders: []string{"Content-Type", "Authorization", "X-Requested-With", "Accept", "Origin"},
		MaxAge:       86400,
	}
}

// IsWildcard returns true if the CORS config allows all origins.
func (c *CORSConfig) IsWildcard() bool {
	if c == nil {
		return false
	}
	for _, origin := range c.AllowOrigins {
		if origin == "*" {
			return true
		}
	}
	return false
}

// GetAllowOriginValue returns the appropriate Access-Control-Allow-Origin header value
// for the given request origin. Returns empty string if origin is not allowed.
func (c *CORSConfig) GetAllowOriginValue(requestOrigin string) string {
	if c == nil || !c.Enabled {
		return ""
	}

	// Check for wildcard
	for _, origin := range c.AllowOrigins {
		if origin == "*" {
			// Cannot use * with credentials
			if c.AllowCredentials {
				// Return the actual origin instead of *
				if requestOrigin != "" {
					return requestOrigin
				}
				return ""
			}
			return "*"
		}
	}

	// Check if request origin is in allowed list
	for _, allowed := range c.AllowOrigins {
		if allowed == requestOrigin {
			return requestOrigin
		}
	}

	return ""
}
