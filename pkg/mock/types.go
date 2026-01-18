// Package mock provides the unified Mock type that represents all mock types
// (HTTP, WebSocket, GraphQL, gRPC, SOAP, MQTT) with a common interface.
package mock

import (
	"encoding/json"
	"fmt"
	"time"
)

// MockType represents the type of mock.
type MockType string

const (
	MockTypeHTTP      MockType = "http"
	MockTypeWebSocket MockType = "websocket"
	MockTypeGraphQL   MockType = "graphql"
	MockTypeGRPC      MockType = "grpc"
	MockTypeSOAP      MockType = "soap"
	MockTypeMQTT      MockType = "mqtt"
	MockTypeOAuth     MockType = "oauth"
)

// Mock represents a unified mock definition that can be any of the supported types.
// The Type field determines which Spec field is populated.
type Mock struct {
	// ID is a unique identifier for the mock (UUID or prefixed ID)
	ID string `json:"id" yaml:"id"`

	// Type determines the mock type and which spec field is populated
	Type MockType `json:"type" yaml:"type"`

	// Name is a human-readable name for the mock
	Name string `json:"name,omitempty" yaml:"name,omitempty"`

	// Description is an optional longer description
	Description string `json:"description,omitempty" yaml:"description,omitempty"`

	// Enabled indicates whether this mock is active
	Enabled bool `json:"enabled" yaml:"enabled"`

	// ParentID is the folder ID this mock belongs to ("" = root level)
	ParentID string `json:"parentId,omitempty" yaml:"parentId,omitempty"`

	// MetaSortKey is used for manual ordering within a folder
	MetaSortKey float64 `json:"metaSortKey,omitempty" yaml:"metaSortKey,omitempty"`

	// WorkspaceID is the source workspace (defaults to "local")
	WorkspaceID string `json:"workspaceId,omitempty" yaml:"workspaceId,omitempty"`

	// SyncVersion is used for CRDT/conflict resolution
	SyncVersion int64 `json:"syncVersion,omitempty" yaml:"syncVersion,omitempty"`

	// CreatedAt is when the mock was created
	CreatedAt time.Time `json:"createdAt" yaml:"createdAt"`

	// UpdatedAt is when the mock was last modified
	UpdatedAt time.Time `json:"updatedAt" yaml:"updatedAt"`

	// Type-specific specs - exactly one is populated based on Type
	HTTP      *HTTPSpec      `json:"http,omitempty" yaml:"http,omitempty"`
	WebSocket *WebSocketSpec `json:"websocket,omitempty" yaml:"websocket,omitempty"`
	GraphQL   *GraphQLSpec   `json:"graphql,omitempty" yaml:"graphql,omitempty"`
	GRPC      *GRPCSpec      `json:"grpc,omitempty" yaml:"grpc,omitempty"`
	SOAP      *SOAPSpec      `json:"soap,omitempty" yaml:"soap,omitempty"`
	MQTT      *MQTTSpec      `json:"mqtt,omitempty" yaml:"mqtt,omitempty"`
	OAuth     *OAuthSpec     `json:"oauth,omitempty" yaml:"oauth,omitempty"`
}

// UnmarshalJSON handles both the legacy format (matcher/response at top level)
// and the new unified format (type field with nested spec).
func (m *Mock) UnmarshalJSON(data []byte) error {
	// First, try to detect which format we're dealing with
	var probe struct {
		Type    MockType `json:"type"`
		Matcher *struct {
			Method string `json:"method"`
			Path   string `json:"path"`
		} `json:"matcher"`
	}
	if err := json.Unmarshal(data, &probe); err != nil {
		return err
	}

	// If we have a matcher field but no type, it's legacy HTTP format
	if probe.Matcher != nil && probe.Type == "" {
		return m.unmarshalLegacyHTTP(data)
	}

	// Otherwise, use standard unmarshaling with an alias to avoid recursion
	type MockAlias Mock
	alias := (*MockAlias)(m)
	return json.Unmarshal(data, alias)
}

// unmarshalLegacyHTTP handles the old format where matcher/response are at the top level.
func (m *Mock) unmarshalLegacyHTTP(data []byte) error {
	// Define a struct for the legacy format
	var legacy struct {
		ID          string    `json:"id"`
		Name        string    `json:"name,omitempty"`
		Description string    `json:"description,omitempty"`
		Enabled     bool      `json:"enabled"`
		ParentID    string    `json:"parentId,omitempty"`
		MetaSortKey float64   `json:"metaSortKey,omitempty"`
		WorkspaceID string    `json:"workspaceId,omitempty"`
		SyncVersion int64     `json:"syncVersion,omitempty"`
		CreatedAt   time.Time `json:"createdAt"`
		UpdatedAt   time.Time `json:"updatedAt"`
		Priority    int       `json:"priority,omitempty"`

		// Legacy HTTP fields at top level
		Matcher  *HTTPMatcher  `json:"matcher"`
		Response *HTTPResponse `json:"response"`
	}

	if err := json.Unmarshal(data, &legacy); err != nil {
		return err
	}

	// Convert to new unified format
	m.ID = legacy.ID
	m.Type = MockTypeHTTP
	m.Name = legacy.Name
	m.Description = legacy.Description
	m.Enabled = legacy.Enabled
	m.ParentID = legacy.ParentID
	m.MetaSortKey = legacy.MetaSortKey
	m.WorkspaceID = legacy.WorkspaceID
	m.SyncVersion = legacy.SyncVersion
	m.CreatedAt = legacy.CreatedAt
	m.UpdatedAt = legacy.UpdatedAt

	// Build the HTTP spec
	m.HTTP = &HTTPSpec{
		Priority: legacy.Priority,
		Matcher:  legacy.Matcher,
		Response: legacy.Response,
	}

	return nil
}

// GetSpec returns the type-specific spec as an interface.
func (m *Mock) GetSpec() interface{} {
	switch m.Type {
	case MockTypeHTTP:
		return m.HTTP
	case MockTypeWebSocket:
		return m.WebSocket
	case MockTypeGraphQL:
		return m.GraphQL
	case MockTypeGRPC:
		return m.GRPC
	case MockTypeSOAP:
		return m.SOAP
	case MockTypeMQTT:
		return m.MQTT
	case MockTypeOAuth:
		return m.OAuth
	default:
		return nil
	}
}

// GetPath returns the path/endpoint for this mock (for display purposes).
func (m *Mock) GetPath() string {
	switch m.Type {
	case MockTypeHTTP:
		if m.HTTP != nil && m.HTTP.Matcher != nil {
			if m.HTTP.Matcher.Path != "" {
				return m.HTTP.Matcher.Path
			}
			return m.HTTP.Matcher.PathPattern
		}
	case MockTypeWebSocket:
		if m.WebSocket != nil {
			return m.WebSocket.Path
		}
	case MockTypeGraphQL:
		if m.GraphQL != nil {
			return m.GraphQL.Path
		}
	case MockTypeGRPC:
		if m.GRPC != nil {
			return formatPort(m.GRPC.Port)
		}
	case MockTypeSOAP:
		if m.SOAP != nil {
			return m.SOAP.Path
		}
	case MockTypeMQTT:
		if m.MQTT != nil {
			return formatPort(m.MQTT.Port)
		}
	case MockTypeOAuth:
		if m.OAuth != nil {
			return m.OAuth.Issuer
		}
	}
	return ""
}

// GetMethod returns the HTTP method (only applicable for HTTP mocks).
func (m *Mock) GetMethod() string {
	if m.Type == MockTypeHTTP && m.HTTP != nil && m.HTTP.Matcher != nil {
		return m.HTTP.Matcher.Method
	}
	return ""
}

func formatPort(port int) string {
	if port == 0 {
		return ""
	}
	return fmt.Sprintf(":%d", port)
}

// ============================================================================
// HTTP Spec
// ============================================================================

// HTTPSpec contains HTTP-specific mock configuration.
type HTTPSpec struct {
	// Priority determines matching order - higher priority mocks match first
	Priority int `json:"priority,omitempty" yaml:"priority,omitempty"`

	// Matcher defines criteria for matching incoming requests
	Matcher *HTTPMatcher `json:"matcher" yaml:"matcher"`

	// Response defines the response to return when matched
	Response *HTTPResponse `json:"response,omitempty" yaml:"response,omitempty"`

	// SSE defines Server-Sent Events streaming response configuration
	SSE *SSEConfig `json:"sse,omitempty" yaml:"sse,omitempty"`

	// Chunked defines HTTP chunked transfer encoding response configuration
	Chunked *ChunkedConfig `json:"chunked,omitempty" yaml:"chunked,omitempty"`
}

// HTTPMatcher defines criteria used to match incoming HTTP requests.
type HTTPMatcher struct {
	Method       string                 `json:"method,omitempty" yaml:"method,omitempty"`
	Path         string                 `json:"path,omitempty" yaml:"path,omitempty"`
	PathPattern  string                 `json:"pathPattern,omitempty" yaml:"pathPattern,omitempty"`
	Headers      map[string]string      `json:"headers,omitempty" yaml:"headers,omitempty"`
	QueryParams  map[string]string      `json:"queryParams,omitempty" yaml:"queryParams,omitempty"`
	BodyContains string                 `json:"bodyContains,omitempty" yaml:"bodyContains,omitempty"`
	BodyEquals   string                 `json:"bodyEquals,omitempty" yaml:"bodyEquals,omitempty"`
	BodyPattern  string                 `json:"bodyPattern,omitempty" yaml:"bodyPattern,omitempty"`
	BodyJSONPath map[string]interface{} `json:"bodyJsonPath,omitempty" yaml:"bodyJsonPath,omitempty"`
	MTLS         *MTLSMatch             `json:"mtls,omitempty" yaml:"mtls,omitempty"`
}

// MTLSMatch defines mTLS client certificate matching criteria.
type MTLSMatch struct {
	RequireAuth bool      `json:"requireAuth,omitempty" yaml:"requireAuth,omitempty"`
	CN          string    `json:"cn,omitempty" yaml:"cn,omitempty"`
	CNPattern   string    `json:"cnPattern,omitempty" yaml:"cnPattern,omitempty"`
	OU          string    `json:"ou,omitempty" yaml:"ou,omitempty"`
	O           string    `json:"o,omitempty" yaml:"o,omitempty"`
	Fingerprint string    `json:"fingerprint,omitempty" yaml:"fingerprint,omitempty"`
	Issuer      string    `json:"issuer,omitempty" yaml:"issuer,omitempty"`
	SAN         *SANMatch `json:"san,omitempty" yaml:"san,omitempty"`
}

// SANMatch defines Subject Alternative Name matching criteria.
type SANMatch struct {
	DNS   string `json:"dns,omitempty" yaml:"dns,omitempty"`
	Email string `json:"email,omitempty" yaml:"email,omitempty"`
	IP    string `json:"ip,omitempty" yaml:"ip,omitempty"`
}

// HTTPResponse specifies the HTTP response to return.
type HTTPResponse struct {
	StatusCode int               `json:"statusCode" yaml:"statusCode"`
	Headers    map[string]string `json:"headers,omitempty" yaml:"headers,omitempty"`
	Body       string            `json:"body" yaml:"body"`
	BodyFile   string            `json:"bodyFile,omitempty" yaml:"bodyFile,omitempty"`
	DelayMs    int               `json:"delayMs,omitempty" yaml:"delayMs,omitempty"`
}

// SSEConfig defines Server-Sent Events configuration.
// Imported from config package - keeping reference here for completeness.
type SSEConfig struct {
	Events         []SSEEventDef       `json:"events,omitempty" yaml:"events,omitempty"`
	Generator      *SSEEventGenerator  `json:"generator,omitempty" yaml:"generator,omitempty"`
	Timing         SSETimingConfig     `json:"timing" yaml:"timing"`
	Lifecycle      SSELifecycleConfig  `json:"lifecycle" yaml:"lifecycle"`
	RateLimit      *SSERateLimitConfig `json:"rateLimit,omitempty" yaml:"rateLimit,omitempty"`
	Resume         SSEResumeConfig     `json:"resume" yaml:"resume"`
	Template       string              `json:"template,omitempty" yaml:"template,omitempty"`
	TemplateParams map[string]any      `json:"templateParams,omitempty" yaml:"templateParams,omitempty"`
}

// SSEEventDef defines a single SSE event.
type SSEEventDef struct {
	Type    string `json:"type,omitempty"`
	Data    any    `json:"data"`
	ID      string `json:"id,omitempty"`
	Retry   int    `json:"retry,omitempty"`
	Comment string `json:"comment,omitempty"`
	Delay   *int   `json:"delay,omitempty"`
}

// SSEEventGenerator configures dynamic event generation.
type SSEEventGenerator struct {
	Type     string                `json:"type"`
	Count    int                   `json:"count,omitempty"`
	Sequence *SSESequenceGenerator `json:"sequence,omitempty"`
	Random   *SSERandomGenerator   `json:"random,omitempty"`
	Template *SSETemplateGenerator `json:"template,omitempty"`
}

// SSESequenceGenerator produces incrementing numeric events.
type SSESequenceGenerator struct {
	Start     int    `json:"start"`
	Increment int    `json:"increment"`
	Format    string `json:"format,omitempty"`
}

// SSERandomGenerator produces random data events.
type SSERandomGenerator struct {
	Schema map[string]any `json:"schema"`
}

// SSETemplateGenerator repeats events from a list.
type SSETemplateGenerator struct {
	Events []SSEEventDef `json:"events"`
	Repeat int           `json:"repeat,omitempty"`
}

// SSETimingConfig controls event delivery timing.
type SSETimingConfig struct {
	FixedDelay     *int                  `json:"fixedDelay,omitempty"`
	RandomDelay    *SSERandomDelayConfig `json:"randomDelay,omitempty"`
	PerEventDelays []int                 `json:"perEventDelays,omitempty"`
	Burst          *SSEBurstConfig       `json:"burst,omitempty"`
	InitialDelay   int                   `json:"initialDelay,omitempty"`
}

// SSERandomDelayConfig defines a random delay range.
type SSERandomDelayConfig struct {
	Min int `json:"min"`
	Max int `json:"max"`
}

// SSEBurstConfig defines burst delivery mode.
type SSEBurstConfig struct {
	Count    int `json:"count"`
	Interval int `json:"interval"`
	Pause    int `json:"pause"`
}

// SSELifecycleConfig controls connection behavior.
type SSELifecycleConfig struct {
	KeepaliveInterval  int                  `json:"keepaliveInterval,omitempty"`
	MaxEvents          int                  `json:"maxEvents,omitempty"`
	Timeout            int                  `json:"timeout,omitempty"`
	ConnectionTimeout  int                  `json:"connectionTimeout,omitempty"`
	Termination        SSETerminationConfig `json:"termination,omitempty"`
	SimulateDisconnect *int                 `json:"simulateDisconnect,omitempty"`
}

// SSETerminationConfig defines how the stream ends.
type SSETerminationConfig struct {
	Type       string       `json:"type,omitempty"`
	FinalEvent *SSEEventDef `json:"finalEvent,omitempty"`
	ErrorEvent *SSEEventDef `json:"errorEvent,omitempty"`
	CloseDelay int          `json:"closeDelay,omitempty"`
}

// SSEResumeConfig controls Last-Event-ID resumption.
type SSEResumeConfig struct {
	Enabled    bool `json:"enabled"`
	BufferSize int  `json:"bufferSize,omitempty"`
	MaxAge     int  `json:"maxAge,omitempty"`
}

// SSERateLimitConfig controls event delivery rate.
type SSERateLimitConfig struct {
	EventsPerSecond float64 `json:"eventsPerSecond"`
	BurstSize       int     `json:"burstSize,omitempty"`
	Strategy        string  `json:"strategy,omitempty"`
	Headers         bool    `json:"headers,omitempty"`
}

// ChunkedConfig configures HTTP chunked transfer encoding.
type ChunkedConfig struct {
	ChunkSize   int    `json:"chunkSize,omitempty"`
	ChunkDelay  int    `json:"chunkDelay,omitempty"`
	Data        string `json:"data,omitempty"`
	DataFile    string `json:"dataFile,omitempty"`
	Format      string `json:"format,omitempty"`
	NDJSONItems []any  `json:"ndjsonItems,omitempty"`
}

// ============================================================================
// WebSocket Spec
// ============================================================================

// WebSocketSpec contains WebSocket-specific mock configuration.
type WebSocketSpec struct {
	Path               string             `json:"path" yaml:"path"`
	Subprotocols       []string           `json:"subprotocols,omitempty" yaml:"subprotocols,omitempty"`
	RequireSubprotocol bool               `json:"requireSubprotocol,omitempty" yaml:"requireSubprotocol,omitempty"`
	Matchers           []WSMatcherConfig  `json:"matchers,omitempty" yaml:"matchers,omitempty"`
	DefaultResponse    *WSMessageResponse `json:"defaultResponse,omitempty" yaml:"defaultResponse,omitempty"`
	Scenario           *WSScenarioConfig  `json:"scenario,omitempty" yaml:"scenario,omitempty"`
	Heartbeat          *WSHeartbeatConfig `json:"heartbeat,omitempty" yaml:"heartbeat,omitempty"`
	MaxMessageSize     int64              `json:"maxMessageSize,omitempty" yaml:"maxMessageSize,omitempty"`
	IdleTimeout        string             `json:"idleTimeout,omitempty" yaml:"idleTimeout,omitempty"`
	MaxConnections     int                `json:"maxConnections,omitempty" yaml:"maxConnections,omitempty"`
	EchoMode           *bool              `json:"echoMode,omitempty" yaml:"echoMode,omitempty"`
}

// WSMatcherConfig defines a WebSocket message matcher.
type WSMatcherConfig struct {
	Match      *WSMatchCriteria   `json:"match" yaml:"match"`
	Response   *WSMessageResponse `json:"response,omitempty" yaml:"response,omitempty"`
	NoResponse bool               `json:"noResponse,omitempty" yaml:"noResponse,omitempty"`
}

// WSMatchCriteria defines how to match a WebSocket message.
type WSMatchCriteria struct {
	Type        string `json:"type" yaml:"type"`
	Value       string `json:"value,omitempty" yaml:"value,omitempty"`
	Path        string `json:"path,omitempty" yaml:"path,omitempty"`
	MessageType string `json:"messageType,omitempty" yaml:"messageType,omitempty"`
}

// WSMessageResponse defines a response to send for a matched message.
type WSMessageResponse struct {
	Type  string `json:"type" yaml:"type"`
	Value any    `json:"value" yaml:"value"`
	Delay string `json:"delay,omitempty" yaml:"delay,omitempty"`
}

// WSScenarioConfig defines a scripted WebSocket message sequence.
type WSScenarioConfig struct {
	Name             string                 `json:"name" yaml:"name"`
	Steps            []WSScenarioStepConfig `json:"steps" yaml:"steps"`
	Loop             bool                   `json:"loop,omitempty" yaml:"loop,omitempty"`
	ResetOnReconnect *bool                  `json:"resetOnReconnect,omitempty" yaml:"resetOnReconnect,omitempty"`
}

// WSScenarioStepConfig defines a single step in a WebSocket scenario.
type WSScenarioStepConfig struct {
	Type     string             `json:"type" yaml:"type"`
	Message  *WSMessageResponse `json:"message,omitempty" yaml:"message,omitempty"`
	Match    *WSMatchCriteria   `json:"match,omitempty" yaml:"match,omitempty"`
	Duration string             `json:"duration,omitempty" yaml:"duration,omitempty"`
	Timeout  string             `json:"timeout,omitempty" yaml:"timeout,omitempty"`
	Optional bool               `json:"optional,omitempty" yaml:"optional,omitempty"`
}

// WSHeartbeatConfig configures WebSocket ping/pong keepalive.
type WSHeartbeatConfig struct {
	Enabled  bool   `json:"enabled" yaml:"enabled"`
	Interval string `json:"interval,omitempty" yaml:"interval,omitempty"`
	Timeout  string `json:"timeout,omitempty" yaml:"timeout,omitempty"`
}

// ============================================================================
// GraphQL Spec
// ============================================================================

// GraphQLSpec contains GraphQL-specific mock configuration.
type GraphQLSpec struct {
	Path          string                        `json:"path" yaml:"path"`
	Schema        string                        `json:"schema,omitempty" yaml:"schema,omitempty"`
	SchemaFile    string                        `json:"schemaFile,omitempty" yaml:"schemaFile,omitempty"`
	Introspection bool                          `json:"introspection" yaml:"introspection"`
	Resolvers     map[string]ResolverConfig     `json:"resolvers,omitempty" yaml:"resolvers,omitempty"`
	Subscriptions map[string]SubscriptionConfig `json:"subscriptions,omitempty" yaml:"subscriptions,omitempty"`
}

// ResolverConfig configures how a GraphQL field is resolved.
type ResolverConfig struct {
	Response any                 `json:"response,omitempty" yaml:"response,omitempty"`
	Delay    string              `json:"delay,omitempty" yaml:"delay,omitempty"`
	Match    *ResolverMatch      `json:"match,omitempty" yaml:"match,omitempty"`
	Error    *GraphQLErrorConfig `json:"error,omitempty" yaml:"error,omitempty"`
}

// ResolverMatch specifies matching conditions for a resolver.
type ResolverMatch struct {
	Args map[string]any `json:"args,omitempty" yaml:"args,omitempty"`
}

// GraphQLErrorConfig configures a GraphQL error response.
type GraphQLErrorConfig struct {
	Message    string         `json:"message"`
	Path       []string       `json:"path,omitempty"`
	Extensions map[string]any `json:"extensions,omitempty"`
}

// SubscriptionConfig configures GraphQL subscriptions.
type SubscriptionConfig struct {
	// TODO: Add subscription-specific fields
}

// ============================================================================
// gRPC Spec
// ============================================================================

// GRPCSpec contains gRPC-specific mock configuration.
type GRPCSpec struct {
	Port        int                      `json:"port" yaml:"port"`
	ProtoFile   string                   `json:"protoFile,omitempty" yaml:"protoFile,omitempty"`
	ProtoFiles  []string                 `json:"protoFiles,omitempty" yaml:"protoFiles,omitempty"`
	ImportPaths []string                 `json:"importPaths,omitempty" yaml:"importPaths,omitempty"`
	Services    map[string]ServiceConfig `json:"services,omitempty" yaml:"services,omitempty"`
	Reflection  bool                     `json:"reflection" yaml:"reflection"`
}

// ServiceConfig configures mock responses for a gRPC service.
type ServiceConfig struct {
	Methods map[string]MethodConfig `json:"methods,omitempty" yaml:"methods,omitempty"`
}

// MethodConfig configures how a gRPC method responds to requests.
type MethodConfig struct {
	Response    any              `json:"response,omitempty" yaml:"response,omitempty"`
	Responses   []any            `json:"responses,omitempty" yaml:"responses,omitempty"`
	Delay       string           `json:"delay,omitempty" yaml:"delay,omitempty"`
	StreamDelay string           `json:"streamDelay,omitempty" yaml:"streamDelay,omitempty"`
	Error       *GRPCErrorConfig `json:"error,omitempty" yaml:"error,omitempty"`
	Match       *MethodMatch     `json:"match,omitempty" yaml:"match,omitempty"`
}

// MethodMatch defines conditions for matching incoming gRPC requests.
type MethodMatch struct {
	Metadata map[string]string `json:"metadata,omitempty" yaml:"metadata,omitempty"`
	Request  map[string]any    `json:"request,omitempty" yaml:"request,omitempty"`
}

// GRPCErrorConfig defines a gRPC error response.
type GRPCErrorConfig struct {
	Code    string         `json:"code" yaml:"code"`
	Message string         `json:"message" yaml:"message"`
	Details map[string]any `json:"details,omitempty" yaml:"details,omitempty"`
}

// ============================================================================
// SOAP Spec
// ============================================================================

// SOAPSpec contains SOAP-specific mock configuration.
type SOAPSpec struct {
	Path       string                     `json:"path" yaml:"path"`
	WSDLFile   string                     `json:"wsdlFile,omitempty" yaml:"wsdlFile,omitempty"`
	WSDL       string                     `json:"wsdl,omitempty" yaml:"wsdl,omitempty"`
	Operations map[string]OperationConfig `json:"operations,omitempty" yaml:"operations,omitempty"`
}

// OperationConfig configures a single SOAP operation.
type OperationConfig struct {
	SOAPAction string     `json:"soapAction,omitempty" yaml:"soapAction,omitempty"`
	Response   string     `json:"response" yaml:"response"`
	Delay      string     `json:"delay,omitempty" yaml:"delay,omitempty"`
	Fault      *SOAPFault `json:"fault,omitempty" yaml:"fault,omitempty"`
	Match      *SOAPMatch `json:"match,omitempty" yaml:"match,omitempty"`
}

// SOAPMatch defines XPath-based request matching conditions.
type SOAPMatch struct {
	XPath map[string]string `json:"xpath,omitempty" yaml:"xpath,omitempty"`
}

// SOAPFault defines a SOAP fault response.
type SOAPFault struct {
	Code    string `json:"code" yaml:"code"`
	Message string `json:"message" yaml:"message"`
	Detail  string `json:"detail,omitempty" yaml:"detail,omitempty"`
}

// ============================================================================
// MQTT Spec
// ============================================================================

// MQTTSpec contains MQTT-specific mock configuration.
type MQTTSpec struct {
	Port   int             `json:"port" yaml:"port"`
	TLS    *MQTTTLSConfig  `json:"tls,omitempty" yaml:"tls,omitempty"`
	Auth   *MQTTAuthConfig `json:"auth,omitempty" yaml:"auth,omitempty"`
	Topics []TopicConfig   `json:"topics,omitempty" yaml:"topics,omitempty"`
}

// MQTTTLSConfig configures TLS for the MQTT broker.
type MQTTTLSConfig struct {
	Enabled  bool   `json:"enabled" yaml:"enabled"`
	CertFile string `json:"certFile" yaml:"certFile"`
	KeyFile  string `json:"keyFile" yaml:"keyFile"`
}

// MQTTAuthConfig configures authentication for the MQTT broker.
type MQTTAuthConfig struct {
	Enabled bool       `json:"enabled" yaml:"enabled"`
	Users   []MQTTUser `json:"users,omitempty" yaml:"users,omitempty"`
}

// MQTTUser represents an authenticated MQTT user.
type MQTTUser struct {
	Username string    `json:"username" yaml:"username"`
	Password string    `json:"password" yaml:"password"`
	ACL      []ACLRule `json:"acl,omitempty" yaml:"acl,omitempty"`
}

// ACLRule defines access control for topics.
type ACLRule struct {
	Topic  string `json:"topic" yaml:"topic"`
	Access string `json:"access" yaml:"access"`
}

// TopicConfig configures a mock topic.
type TopicConfig struct {
	Topic            string                    `json:"topic" yaml:"topic"`
	QoS              int                       `json:"qos,omitempty" yaml:"qos,omitempty"`
	Retain           bool                      `json:"retain,omitempty" yaml:"retain,omitempty"`
	Messages         []MessageConfig           `json:"messages,omitempty" yaml:"messages,omitempty"`
	OnPublish        *PublishHandler           `json:"onPublish,omitempty" yaml:"onPublish,omitempty"`
	DeviceSimulation *DeviceSimulationSettings `json:"deviceSimulation,omitempty" yaml:"deviceSimulation,omitempty"`
}

// DeviceSimulationSettings configures per-topic device simulation.
type DeviceSimulationSettings struct {
	Enabled         bool   `json:"enabled" yaml:"enabled"`
	DeviceCount     int    `json:"deviceCount" yaml:"deviceCount"`
	DeviceIDPattern string `json:"deviceIdPattern" yaml:"deviceIdPattern"`
}

// MessageConfig configures a message to be published.
type MessageConfig struct {
	Payload  string `json:"payload" yaml:"payload"`
	Delay    string `json:"delay,omitempty" yaml:"delay,omitempty"`
	Repeat   bool   `json:"repeat,omitempty" yaml:"repeat,omitempty"`
	Interval string `json:"interval,omitempty" yaml:"interval,omitempty"`
}

// PublishHandler configures behavior when a message is received.
type PublishHandler struct {
	Response *MessageConfig `json:"response,omitempty" yaml:"response,omitempty"`
	Forward  string         `json:"forward,omitempty" yaml:"forward,omitempty"`
}

// OAuthSpec contains OAuth/OIDC mock provider configuration.
type OAuthSpec struct {
	// Issuer is the OAuth issuer URL (e.g., http://localhost:9999/oauth)
	Issuer string `json:"issuer" yaml:"issuer"`

	// TokenExpiry is the access token lifetime (e.g., "1h", "30m")
	TokenExpiry string `json:"tokenExpiry,omitempty" yaml:"tokenExpiry,omitempty"`

	// RefreshExpiry is the refresh token lifetime (e.g., "7d", "24h")
	RefreshExpiry string `json:"refreshExpiry,omitempty" yaml:"refreshExpiry,omitempty"`

	// DefaultScopes are the default scopes to include if none requested
	DefaultScopes []string `json:"defaultScopes,omitempty" yaml:"defaultScopes,omitempty"`

	// Clients are the registered OAuth clients
	Clients []OAuthClient `json:"clients,omitempty" yaml:"clients,omitempty"`

	// Users are test users for resource owner password credentials flow
	Users []OAuthUser `json:"users,omitempty" yaml:"users,omitempty"`
}

// OAuthClient defines an OAuth client configuration.
type OAuthClient struct {
	ClientID     string   `json:"clientId" yaml:"clientId"`
	ClientSecret string   `json:"clientSecret" yaml:"clientSecret"`
	RedirectURIs []string `json:"redirectUris,omitempty" yaml:"redirectUris,omitempty"`
	GrantTypes   []string `json:"grantTypes,omitempty" yaml:"grantTypes,omitempty"`
}

// OAuthUser defines a test user for the resource owner password credentials flow.
type OAuthUser struct {
	Username string            `json:"username" yaml:"username"`
	Password string            `json:"password" yaml:"password"`
	Claims   map[string]string `json:"claims,omitempty" yaml:"claims,omitempty"`
}
