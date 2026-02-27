// Package mock provides the unified Mock type that represents all mock types
// (HTTP, WebSocket, GraphQL, gRPC, SOAP, MQTT) with a common interface.
package mock

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/getmockd/mockd/pkg/validation"
	"gopkg.in/yaml.v3"
)

// Type represents the type of mock.
type Type string

const (
	TypeHTTP      Type = "http"
	TypeWebSocket Type = "websocket"
	TypeGraphQL   Type = "graphql"
	TypeGRPC      Type = "grpc"
	TypeSOAP      Type = "soap"
	TypeMQTT      Type = "mqtt"
	TypeOAuth     Type = "oauth"
)

// Mock represents a unified mock definition that can be any of the supported types.
// The Type field determines which Spec field is populated.
type Mock struct {
	// ID is a unique identifier for the mock (UUID or prefixed ID)
	ID string `json:"id" yaml:"id"`

	// Type determines the mock type and which spec field is populated
	Type Type `json:"type" yaml:"type"`

	// Name is a human-readable name for the mock
	Name string `json:"name,omitempty" yaml:"name,omitempty"`

	// Description is an optional longer description
	Description string `json:"description,omitempty" yaml:"description,omitempty"`

	// Enabled indicates whether this mock is active
	Enabled *bool `json:"enabled,omitempty" yaml:"enabled,omitempty"`

	// ParentID is the folder ID this mock belongs to ("" = root level)
	ParentID string `json:"parentId,omitempty" yaml:"parentId,omitempty"`

	// FolderID is an alias for ParentID, used by UI clients.
	// On unmarshal, if FolderID is set and ParentID is empty, ParentID is populated from FolderID.
	// On marshal, FolderID is omitted (parentId is the canonical field).
	FolderID string `json:"folderId,omitempty" yaml:"folderId,omitempty"`

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
	// Probe for format detection using RawMessage for field presence
	var probe struct {
		Type    Type            `json:"type"`
		Matcher json.RawMessage `json:"matcher"`
		HTTP    json.RawMessage `json:"http"`
	}
	if err := json.Unmarshal(data, &probe); err != nil {
		return err
	}

	// Legacy format: has "matcher" at top level, no "type", no "http" spec
	isLegacyFormat := len(probe.Matcher) > 0 && probe.Type == "" && len(probe.HTTP) == 0

	if isLegacyFormat {
		return m.unmarshalLegacyHTTP(data)
	}

	// Use standard unmarshaling with an alias to avoid recursion
	type MockAlias Mock
	alias := (*MockAlias)(m)
	if err := json.Unmarshal(data, alias); err != nil {
		return err
	}

	// Reconcile FolderID -> ParentID: if folderId was provided but parentId was not,
	// copy folderId into parentId so downstream code only needs to check ParentID.
	if m.FolderID != "" && m.ParentID == "" {
		m.ParentID = m.FolderID
	}
	// Clear FolderID so it doesn't get re-serialized (parentId is canonical)
	m.FolderID = ""

	return nil
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
	m.Type = TypeHTTP
	m.Name = legacy.Name
	m.Description = legacy.Description
	m.Enabled = &legacy.Enabled
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
	case TypeHTTP:
		return m.HTTP
	case TypeWebSocket:
		return m.WebSocket
	case TypeGraphQL:
		return m.GraphQL
	case TypeGRPC:
		return m.GRPC
	case TypeSOAP:
		return m.SOAP
	case TypeMQTT:
		return m.MQTT
	case TypeOAuth:
		return m.OAuth
	default:
		return nil
	}
}

// GetPath returns the path/endpoint for this mock (for display purposes).
func (m *Mock) GetPath() string {
	switch m.Type {
	case TypeHTTP:
		if m.HTTP != nil && m.HTTP.Matcher != nil {
			if m.HTTP.Matcher.Path != "" {
				return m.HTTP.Matcher.Path
			}
			return m.HTTP.Matcher.PathPattern
		}
	case TypeWebSocket:
		if m.WebSocket != nil {
			return m.WebSocket.Path
		}
	case TypeGraphQL:
		if m.GraphQL != nil {
			return m.GraphQL.Path
		}
	case TypeGRPC:
		if m.GRPC != nil {
			return formatPort(m.GRPC.Port)
		}
	case TypeSOAP:
		if m.SOAP != nil {
			return m.SOAP.Path
		}
	case TypeMQTT:
		if m.MQTT != nil {
			return formatPort(m.MQTT.Port)
		}
	case TypeOAuth:
		if m.OAuth != nil {
			return m.OAuth.Issuer
		}
	}
	return ""
}

// GetMethod returns the HTTP method (only applicable for HTTP mocks).
func (m *Mock) GetMethod() string {
	if m.Type == TypeHTTP && m.HTTP != nil && m.HTTP.Matcher != nil {
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

	// Validation defines request validation rules (runs after matching, before response)
	Validation *validation.RequestValidation `json:"validation,omitempty" yaml:"validation,omitempty"`

	// StatefulOperation routes this mock through a registered custom operation.
	// When set, the JSON request body becomes the operation input, and the
	// operation result is returned as JSON. This allows HTTP endpoints like
	// POST /api/transfer to execute multi-step custom operations (e.g., TransferFunds).
	StatefulOperation string `json:"statefulOperation,omitempty" yaml:"statefulOperation,omitempty"`
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

// UnmarshalJSON handles the Body field accepting both a string and a JSON object/array.
// When body is a JSON object (e.g., {"id": 1}) or array, it is marshaled to a JSON string.
// This lets config files use: body: {"id": 1} instead of body: '{"id": 1}'.
func (r *HTTPResponse) UnmarshalJSON(data []byte) error {
	// Use a proxy struct with Body as json.RawMessage so we can inspect it.
	var proxy struct {
		StatusCode int               `json:"statusCode"`
		Headers    map[string]string `json:"headers,omitempty"`
		Body       json.RawMessage   `json:"body"`
		BodyFile   string            `json:"bodyFile,omitempty"`
		DelayMs    int               `json:"delayMs,omitempty"`
	}
	if err := json.Unmarshal(data, &proxy); err != nil {
		return err
	}

	r.StatusCode = proxy.StatusCode
	r.Headers = proxy.Headers
	r.BodyFile = proxy.BodyFile
	r.DelayMs = proxy.DelayMs

	// Handle body: could be string, object, array, number, boolean, or null
	if len(proxy.Body) == 0 {
		r.Body = ""
		return nil
	}

	// Try to unmarshal as string first (most common case)
	var s string
	if err := json.Unmarshal(proxy.Body, &s); err == nil {
		r.Body = s
		return nil
	}

	// Not a string — it's an object, array, number, or boolean.
	// Store the raw JSON as the body string.
	r.Body = string(proxy.Body)
	return nil
}

// UnmarshalYAML handles the Body field accepting both a string and a YAML object/array.
// When body is a YAML mapping or sequence, it is marshaled to a JSON string.
// This lets config files use: body: { id: 1 } instead of body: '{"id": 1}'.
func (r *HTTPResponse) UnmarshalYAML(value *yaml.Node) error {
	// First, find the body node manually from the mapping, then decode the rest
	// with a simple alias to avoid recursion.
	if value.Kind != yaml.MappingNode {
		return fmt.Errorf("expected mapping node, got %d", value.Kind)
	}

	// Extract and handle body specially, decode everything else with alias
	type httpResponseAlias HTTPResponse
	var alias httpResponseAlias

	// Walk the mapping to find the body node
	var bodyNode *yaml.Node
	for i := 0; i+1 < len(value.Content); i += 2 {
		keyNode := value.Content[i]
		if keyNode.Value == "body" {
			bodyNode = value.Content[i+1]
			// Temporarily replace the body value with a placeholder scalar
			// so the default decoder doesn't choke on object bodies
			orig := *bodyNode
			value.Content[i+1] = &yaml.Node{Kind: yaml.ScalarNode, Value: "", Tag: "!!str"}
			if err := value.Decode(&alias); err != nil {
				return err
			}
			// Restore original node
			*value.Content[i+1] = orig
			bodyNode = &orig
			goto handleBody
		}
	}

	// No body field found — just decode normally
	if err := value.Decode(&alias); err != nil {
		return err
	}
	*r = HTTPResponse(alias)
	return nil

handleBody:
	*r = HTTPResponse(alias)

	// Scalar values (strings, numbers, booleans): store as-is
	if bodyNode.Kind == yaml.ScalarNode {
		r.Body = bodyNode.Value
		return nil
	}

	// Mapping or sequence: decode to interface{}, then marshal to JSON string
	var bodyObj interface{}
	if err := bodyNode.Decode(&bodyObj); err != nil {
		return fmt.Errorf("failed to decode body: %w", err)
	}

	bodyJSON, err := json.Marshal(bodyObj)
	if err != nil {
		return fmt.Errorf("failed to marshal body to JSON: %w", err)
	}
	r.Body = string(bodyJSON)
	return nil
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
	Type    string `json:"type,omitempty" yaml:"type,omitempty"`
	Data    any    `json:"data" yaml:"data"`
	ID      string `json:"id,omitempty" yaml:"id,omitempty"`
	Retry   int    `json:"retry,omitempty" yaml:"retry,omitempty"`
	Comment string `json:"comment,omitempty" yaml:"comment,omitempty"`
	Delay   *int   `json:"delay,omitempty" yaml:"delay,omitempty"`
}

// SSEEventGenerator configures dynamic event generation.
type SSEEventGenerator struct {
	Type     string                `json:"type" yaml:"type"`
	Count    int                   `json:"count,omitempty" yaml:"count,omitempty"`
	Sequence *SSESequenceGenerator `json:"sequence,omitempty" yaml:"sequence,omitempty"`
	Random   *SSERandomGenerator   `json:"random,omitempty" yaml:"random,omitempty"`
	Template *SSETemplateGenerator `json:"template,omitempty" yaml:"template,omitempty"`
}

// SSESequenceGenerator produces incrementing numeric events.
type SSESequenceGenerator struct {
	Start     int    `json:"start" yaml:"start"`
	Increment int    `json:"increment" yaml:"increment"`
	Format    string `json:"format,omitempty" yaml:"format,omitempty"`
}

// SSERandomGenerator produces random data events.
type SSERandomGenerator struct {
	Schema map[string]any `json:"schema" yaml:"schema"`
}

// SSETemplateGenerator repeats events from a list.
type SSETemplateGenerator struct {
	Events []SSEEventDef `json:"events" yaml:"events"`
	Repeat int           `json:"repeat,omitempty" yaml:"repeat,omitempty"`
}

// SSETimingConfig controls event delivery timing.
type SSETimingConfig struct {
	FixedDelay     *int                  `json:"fixedDelay,omitempty" yaml:"fixedDelay,omitempty"`
	RandomDelay    *SSERandomDelayConfig `json:"randomDelay,omitempty" yaml:"randomDelay,omitempty"`
	PerEventDelays []int                 `json:"perEventDelays,omitempty" yaml:"perEventDelays,omitempty"`
	Burst          *SSEBurstConfig       `json:"burst,omitempty" yaml:"burst,omitempty"`
	InitialDelay   int                   `json:"initialDelay,omitempty" yaml:"initialDelay,omitempty"`
}

// SSERandomDelayConfig defines a random delay range.
type SSERandomDelayConfig struct {
	Min int `json:"min" yaml:"min"`
	Max int `json:"max" yaml:"max"`
}

// SSEBurstConfig defines burst delivery mode.
type SSEBurstConfig struct {
	Count    int `json:"count" yaml:"count"`
	Interval int `json:"interval" yaml:"interval"`
	Pause    int `json:"pause" yaml:"pause"`
}

// SSELifecycleConfig controls connection behavior.
type SSELifecycleConfig struct {
	KeepaliveInterval  int                  `json:"keepaliveInterval,omitempty" yaml:"keepaliveInterval,omitempty"`
	MaxEvents          int                  `json:"maxEvents,omitempty" yaml:"maxEvents,omitempty"`
	Timeout            int                  `json:"timeout,omitempty" yaml:"timeout,omitempty"`
	ConnectionTimeout  int                  `json:"connectionTimeout,omitempty" yaml:"connectionTimeout,omitempty"`
	Termination        SSETerminationConfig `json:"termination,omitempty" yaml:"termination,omitempty"`
	SimulateDisconnect *int                 `json:"simulateDisconnect,omitempty" yaml:"simulateDisconnect,omitempty"`
}

// SSETerminationConfig defines how the stream ends.
type SSETerminationConfig struct {
	Type       string       `json:"type,omitempty" yaml:"type,omitempty"`
	FinalEvent *SSEEventDef `json:"finalEvent,omitempty" yaml:"finalEvent,omitempty"`
	ErrorEvent *SSEEventDef `json:"errorEvent,omitempty" yaml:"errorEvent,omitempty"`
	CloseDelay int          `json:"closeDelay,omitempty" yaml:"closeDelay,omitempty"`
}

// SSEResumeConfig controls Last-Event-ID resumption.
type SSEResumeConfig struct {
	Enabled    bool `json:"enabled" yaml:"enabled"`
	BufferSize int  `json:"bufferSize,omitempty" yaml:"bufferSize,omitempty"`
	MaxAge     int  `json:"maxAge,omitempty" yaml:"maxAge,omitempty"`
}

// SSERateLimitConfig controls event delivery rate.
type SSERateLimitConfig struct {
	EventsPerSecond float64 `json:"eventsPerSecond" yaml:"eventsPerSecond"`
	BurstSize       int     `json:"burstSize,omitempty" yaml:"burstSize,omitempty"`
	Strategy        string  `json:"strategy,omitempty" yaml:"strategy,omitempty"`
	Headers         bool    `json:"headers,omitempty" yaml:"headers,omitempty"`
}

// ChunkedConfig configures HTTP chunked transfer encoding.
type ChunkedConfig struct {
	ChunkSize   int    `json:"chunkSize,omitempty" yaml:"chunkSize,omitempty"`
	ChunkDelay  int    `json:"chunkDelay,omitempty" yaml:"chunkDelay,omitempty"`
	Data        string `json:"data,omitempty" yaml:"data,omitempty"`
	DataFile    string `json:"dataFile,omitempty" yaml:"dataFile,omitempty"`
	Format      string `json:"format,omitempty" yaml:"format,omitempty"`
	NDJSONItems []any  `json:"ndjsonItems,omitempty" yaml:"ndjsonItems,omitempty"`
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
	// Events is a list of events to stream to the client.
	Events []SubscriptionEventConfig `json:"events,omitempty" yaml:"events,omitempty"`
	// Timing configures the timing behavior for events.
	Timing *SubscriptionTimingConfig `json:"timing,omitempty" yaml:"timing,omitempty"`
}

// SubscriptionEventConfig configures a single subscription event.
type SubscriptionEventConfig struct {
	// Data is the event payload to send.
	Data any `json:"data" yaml:"data"`
	// Delay is the delay before sending this event (e.g., "100ms", "2s").
	Delay string `json:"delay,omitempty" yaml:"delay,omitempty"`
}

// SubscriptionTimingConfig configures timing behavior for subscription events.
type SubscriptionTimingConfig struct {
	// FixedDelay is a fixed delay between events (e.g., "100ms", "1s").
	FixedDelay string `json:"fixedDelay,omitempty" yaml:"fixedDelay,omitempty"`
	// RandomDelay is a random delay range between events (e.g., "100ms-500ms").
	RandomDelay string `json:"randomDelay,omitempty" yaml:"randomDelay,omitempty"`
	// Repeat indicates whether to repeat the events after the sequence completes.
	Repeat bool `json:"repeat,omitempty" yaml:"repeat,omitempty"`
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

	// StatefulResource is the name of the stateful resource this operation reads/writes.
	StatefulResource string `json:"statefulResource,omitempty" yaml:"statefulResource,omitempty"`
	// StatefulAction is the CRUD action: "get", "list", "create", "update", "patch", "delete".
	StatefulAction string `json:"statefulAction,omitempty" yaml:"statefulAction,omitempty"`
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

	// DefaultClaims are claims added to all tokens (e.g., iss, aud)
	DefaultClaims map[string]interface{} `json:"defaultClaims,omitempty" yaml:"defaultClaims,omitempty"`

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
	Username string                 `json:"username" yaml:"username"`
	Password string                 `json:"password" yaml:"password"`
	Claims   map[string]interface{} `json:"claims,omitempty" yaml:"claims,omitempty"`
}
