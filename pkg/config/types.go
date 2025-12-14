// Package config provides configuration types and utilities for the mock server engine.
package config

import (
	"time"
)

// MockConfiguration represents a single mock endpoint definition with its matching criteria and response specification.
type MockConfiguration struct {
	// ID is a unique identifier for the mock (UUID v4)
	ID string `json:"id" yaml:"id"`
	// Name is a human-readable name for the mock
	Name string `json:"name,omitempty" yaml:"name,omitempty"`
	// Description is a longer description of the mock
	Description string `json:"description,omitempty" yaml:"description,omitempty"`
	// Priority determines matching order - higher priority mocks match first when multiple could match
	Priority int `json:"priority,omitempty" yaml:"priority,omitempty"`
	// Matcher defines criteria for matching incoming requests
	Matcher *RequestMatcher `json:"matcher" yaml:"matcher"`
	// Response defines the response to return when matched (mutually exclusive with SSE and Chunked)
	Response *ResponseDefinition `json:"response,omitempty" yaml:"response,omitempty"`
	// SSE defines Server-Sent Events streaming response configuration (mutually exclusive with Response and Chunked)
	SSE *SSEConfig `json:"sse,omitempty" yaml:"sse,omitempty"`
	// Chunked defines HTTP chunked transfer encoding response configuration (mutually exclusive with Response and SSE)
	Chunked *ChunkedConfig `json:"chunked,omitempty" yaml:"chunked,omitempty"`
	// Enabled indicates whether this mock is active
	Enabled bool `json:"enabled" yaml:"enabled"`
	// CreatedAt is when the mock was created
	CreatedAt time.Time `json:"createdAt" yaml:"createdAt"`
	// UpdatedAt is when the mock was last modified
	UpdatedAt time.Time `json:"updatedAt" yaml:"updatedAt"`
}

// SSEConfig defines the configuration for an SSE streaming endpoint
type SSEConfig struct {
	// Events defines the sequence of events to send
	Events []SSEEventDef `json:"events,omitempty" yaml:"events,omitempty"`

	// Generator configures dynamic event generation (mutually exclusive with Events)
	Generator *SSEEventGenerator `json:"generator,omitempty" yaml:"generator,omitempty"`

	// Timing controls delay between events
	Timing SSETimingConfig `json:"timing" yaml:"timing"`

	// Lifecycle controls connection behavior
	Lifecycle SSELifecycleConfig `json:"lifecycle" yaml:"lifecycle"`

	// RateLimit optionally throttles event delivery
	RateLimit *SSERateLimitConfig `json:"rateLimit,omitempty" yaml:"rateLimit,omitempty"`

	// Resume enables Last-Event-ID resumption
	Resume SSEResumeConfig `json:"resume" yaml:"resume"`

	// Template uses a built-in template (e.g., "openai-chat")
	Template string `json:"template,omitempty" yaml:"template,omitempty"`

	// TemplateParams provides parameters for the template
	TemplateParams map[string]interface{} `json:"templateParams,omitempty" yaml:"templateParams,omitempty"`
}

// SSEEventDef defines a single event in the stream
type SSEEventDef struct {
	// Type is the event type (optional, maps to "event:" field)
	Type string `json:"type,omitempty"`

	// Data is the event payload (required, maps to "data:" field)
	Data interface{} `json:"data"`

	// ID is the event identifier (optional, maps to "id:" field)
	ID string `json:"id,omitempty"`

	// Retry suggests reconnection interval in ms (optional)
	Retry int `json:"retry,omitempty"`

	// Comment sends a comment line before the event (optional)
	Comment string `json:"comment,omitempty"`

	// Delay overrides the timing config for this specific event (ms)
	Delay *int `json:"delay,omitempty"`
}

// SSEEventGenerator configures dynamic event generation
type SSEEventGenerator struct {
	// Type specifies the generator type: "sequence", "random", "template"
	Type string `json:"type"`

	// Count is the total number of events to generate (0 = unlimited)
	Count int `json:"count,omitempty"`

	// Sequence generates incrementing values
	Sequence *SSESequenceGenerator `json:"sequence,omitempty"`

	// Random generates random data based on schema
	Random *SSERandomGenerator `json:"random,omitempty"`

	// Template repeats events from a list
	Template *SSETemplateGenerator `json:"template,omitempty"`
}

// SSESequenceGenerator produces incrementing numeric events
type SSESequenceGenerator struct {
	Start     int    `json:"start"`
	Increment int    `json:"increment"`
	Format    string `json:"format,omitempty"`
}

// SSERandomGenerator produces random data events
type SSERandomGenerator struct {
	// Schema defines the JSON structure with random placeholders
	Schema map[string]interface{} `json:"schema"`
}

// SSETemplateGenerator repeats events from a list
type SSETemplateGenerator struct {
	Events []SSEEventDef `json:"events"`
	Repeat int           `json:"repeat,omitempty"`
}

// SSETimingConfig controls event delivery timing
type SSETimingConfig struct {
	// FixedDelay sets a constant delay between events (ms)
	FixedDelay *int `json:"fixedDelay,omitempty"`

	// RandomDelay sets a random delay range between events (ms)
	RandomDelay *SSERandomDelayConfig `json:"randomDelay,omitempty"`

	// PerEventDelays sets specific delays for each event (ms)
	PerEventDelays []int `json:"perEventDelays,omitempty"`

	// Burst configures burst delivery mode
	Burst *SSEBurstConfig `json:"burst,omitempty"`

	// InitialDelay before first event (ms)
	InitialDelay int `json:"initialDelay,omitempty"`
}

// SSERandomDelayConfig defines a random delay range
type SSERandomDelayConfig struct {
	Min int `json:"min"`
	Max int `json:"max"`
}

// SSEBurstConfig defines burst delivery mode
type SSEBurstConfig struct {
	Count    int `json:"count"`
	Interval int `json:"interval"`
	Pause    int `json:"pause"`
}

// SSELifecycleConfig controls connection behavior
type SSELifecycleConfig struct {
	// KeepaliveInterval in seconds (0 = disabled)
	KeepaliveInterval int `json:"keepaliveInterval,omitempty"`

	// MaxEvents closes connection after this many events (0 = unlimited)
	MaxEvents int `json:"maxEvents,omitempty"`

	// Timeout closes connection after this duration of no events (seconds)
	Timeout int `json:"timeout,omitempty"`

	// ConnectionTimeout is max connection duration (seconds)
	ConnectionTimeout int `json:"connectionTimeout,omitempty"`

	// Termination defines how the stream ends
	Termination SSETerminationConfig `json:"termination,omitempty"`

	// SimulateDisconnect triggers abrupt disconnect after N events
	SimulateDisconnect *int `json:"simulateDisconnect,omitempty"`
}

// SSETerminationConfig defines how the stream ends
type SSETerminationConfig struct {
	Type       string       `json:"type,omitempty"`
	FinalEvent *SSEEventDef `json:"finalEvent,omitempty"`
	ErrorEvent *SSEEventDef `json:"errorEvent,omitempty"`
	CloseDelay int          `json:"closeDelay,omitempty"`
}

// SSEResumeConfig controls Last-Event-ID resumption
type SSEResumeConfig struct {
	Enabled    bool `json:"enabled"`
	BufferSize int  `json:"bufferSize,omitempty"`
	MaxAge     int  `json:"maxAge,omitempty"`
}

// SSERateLimitConfig controls event delivery rate
type SSERateLimitConfig struct {
	EventsPerSecond float64 `json:"eventsPerSecond"`
	BurstSize       int     `json:"burstSize,omitempty"`
	Strategy        string  `json:"strategy,omitempty"`
	Headers         bool    `json:"headers,omitempty"`
}

// ChunkedConfig configures HTTP chunked transfer encoding
type ChunkedConfig struct {
	// ChunkSize in bytes (0 = auto)
	ChunkSize int `json:"chunkSize,omitempty"`

	// ChunkDelay between chunks (ms)
	ChunkDelay int `json:"chunkDelay,omitempty"`

	// Data to send (will be split into chunks)
	Data string `json:"data,omitempty"`

	// DataFile path to file containing data
	DataFile string `json:"dataFile,omitempty"`

	// Format: "raw", "ndjson"
	Format string `json:"format,omitempty"`

	// NDJSONItems for ndjson format
	NDJSONItems []interface{} `json:"ndjsonItems,omitempty"`
}

// RequestMatcher defines criteria used to match incoming HTTP requests to mock configurations.
type RequestMatcher struct {
	// Method is the HTTP method to match (GET, POST, etc.) - exact match
	Method string `json:"method,omitempty" yaml:"method,omitempty"`
	// Path is the URL path to match - supports exact match or wildcards
	Path string `json:"path,omitempty" yaml:"path,omitempty"`
	// PathPattern is a regex pattern for path matching (future feature)
	PathPattern string `json:"pathPattern,omitempty" yaml:"pathPattern,omitempty"`
	// Headers are required headers - all must match
	Headers map[string]string `json:"headers,omitempty" yaml:"headers,omitempty"`
	// QueryParams are required query parameters - all must match
	QueryParams map[string]string `json:"queryParams,omitempty" yaml:"queryParams,omitempty"`
	// BodyContains requires the body to contain this substring
	BodyContains string `json:"bodyContains,omitempty" yaml:"bodyContains,omitempty"`
	// BodyEquals requires the body to exactly match this string
	BodyEquals string `json:"bodyEquals,omitempty" yaml:"bodyEquals,omitempty"`
	// BodyPattern is a regex pattern for body matching (future feature)
	BodyPattern string `json:"bodyPattern,omitempty" yaml:"bodyPattern,omitempty"`
}

// ResponseDefinition specifies the HTTP response to return when a request matches a mock.
type ResponseDefinition struct {
	// StatusCode is the HTTP status code (100-599)
	StatusCode int `json:"statusCode" yaml:"statusCode"`
	// Headers are response headers to set
	Headers map[string]string `json:"headers,omitempty" yaml:"headers,omitempty"`
	// Body is the response body content
	Body string `json:"body" yaml:"body"`
	// BodyFile is a path to file containing response body (future feature)
	BodyFile string `json:"bodyFile,omitempty" yaml:"bodyFile,omitempty"`
	// DelayMs is an artificial delay before responding (milliseconds)
	DelayMs int `json:"delayMs,omitempty" yaml:"delayMs,omitempty"`
}

// ServerConfiguration defines the mock server runtime settings and operational parameters.
type ServerConfiguration struct {
	// HTTPPort is the port for the HTTP server (0 = disabled)
	HTTPPort int `json:"httpPort,omitempty"`
	// HTTPSPort is the port for the HTTPS server (0 = disabled)
	HTTPSPort int `json:"httpsPort,omitempty"`
	// AdminPort is the port for the admin API (required)
	AdminPort int `json:"adminPort"`
	// CertFile is the path to the TLS certificate (for HTTPS)
	CertFile string `json:"certFile,omitempty"`
	// KeyFile is the path to the TLS private key (for HTTPS)
	KeyFile string `json:"keyFile,omitempty"`
	// AutoGenerateCert enables auto-generation of self-signed cert if HTTPS is enabled
	AutoGenerateCert bool `json:"autoGenerateCert"`
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
}

// RequestLogEntry captures complete details of an incoming HTTP request for debugging and inspection.
type RequestLogEntry struct {
	// ID is a unique identifier for the log entry (UUID v4)
	ID string `json:"id"`
	// Timestamp is when the request was received
	Timestamp time.Time `json:"timestamp"`
	// Method is the HTTP method
	Method string `json:"method"`
	// Path is the request URL path
	Path string `json:"path"`
	// QueryString is the raw query string
	QueryString string `json:"queryString,omitempty"`
	// Headers are the request headers (multi-value)
	Headers map[string][]string `json:"headers"`
	// Body is the request body content (truncated if > 10KB)
	Body string `json:"body,omitempty"`
	// BodySize is the original body size in bytes
	BodySize int `json:"bodySize"`
	// RemoteAddr is the client IP address
	RemoteAddr string `json:"remoteAddr"`
	// MatchedMockID is the ID of mock that matched (empty if no match)
	MatchedMockID string `json:"matchedMockID,omitempty"`
	// ResponseStatus is the status code returned
	ResponseStatus int `json:"responseStatus"`
	// DurationMs is the request processing time in milliseconds
	DurationMs int `json:"durationMs"`
}

// MockCollection is a container for a set of mock configurations, typically loaded from a single config file.
type MockCollection struct {
	// Version is the config format version (e.g., "1.0")
	Version string `json:"version" yaml:"version"`
	// Kind identifies the config type (e.g., "MockCollection")
	Kind string `json:"kind,omitempty" yaml:"kind,omitempty"`
	// Metadata contains collection metadata
	Metadata *CollectionMetadata `json:"metadata,omitempty" yaml:"metadata,omitempty"`
	// Name is the collection name/description (deprecated, use metadata.name)
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
	// Path is the URL path for WebSocket upgrade (e.g., "/ws/chat")
	Path string `json:"path" yaml:"path"`
	// Subprotocols lists supported subprotocols for negotiation
	Subprotocols []string `json:"subprotocols,omitempty" yaml:"subprotocols,omitempty"`
	// RequireSubprotocol rejects connections without a matching subprotocol
	RequireSubprotocol bool `json:"requireSubprotocol,omitempty" yaml:"requireSubprotocol,omitempty"`
	// Matchers contains message matching rules for conditional responses
	Matchers []*WSMatcherConfig `json:"matchers,omitempty" yaml:"matchers,omitempty"`
	// DefaultResponse is sent when no matcher matches
	DefaultResponse *WSMessageResponse `json:"defaultResponse,omitempty" yaml:"defaultResponse,omitempty"`
	// Scenario defines a scripted message sequence
	Scenario *WSScenarioConfig `json:"scenario,omitempty" yaml:"scenario,omitempty"`
	// Heartbeat configures ping/pong keepalive
	Heartbeat *WSHeartbeatConfig `json:"heartbeat,omitempty" yaml:"heartbeat,omitempty"`
	// MaxMessageSize is the maximum message size in bytes (default: 65536)
	MaxMessageSize int64 `json:"maxMessageSize,omitempty" yaml:"maxMessageSize,omitempty"`
	// IdleTimeout closes connections after inactivity (e.g., "5m")
	IdleTimeout string `json:"idleTimeout,omitempty" yaml:"idleTimeout,omitempty"`
	// MaxConnections limits concurrent connections (default: 0 = unlimited)
	MaxConnections int `json:"maxConnections,omitempty" yaml:"maxConnections,omitempty"`
	// EchoMode enables automatic echo of received messages
	EchoMode *bool `json:"echoMode,omitempty" yaml:"echoMode,omitempty"`
}

// WSMatcherConfig defines a WebSocket message matcher.
type WSMatcherConfig struct {
	// Match defines the matching criteria
	Match *WSMatchCriteria `json:"match" yaml:"match"`
	// Response is the response to send when matched
	Response *WSMessageResponse `json:"response,omitempty" yaml:"response,omitempty"`
	// NoResponse if true, matches but doesn't respond
	NoResponse bool `json:"noResponse,omitempty" yaml:"noResponse,omitempty"`
}

// WSMatchCriteria defines how to match a WebSocket message.
type WSMatchCriteria struct {
	// Type is the match type: "exact", "regex", "json", "contains", "prefix", "suffix"
	Type string `json:"type" yaml:"type"`
	// Value is the match value
	Value string `json:"value,omitempty" yaml:"value,omitempty"`
	// Path is the JSON path for json type (e.g., "$.action")
	Path string `json:"path,omitempty" yaml:"path,omitempty"`
	// MessageType restricts to specific message types: "text", "binary"
	MessageType string `json:"messageType,omitempty" yaml:"messageType,omitempty"`
}

// WSMessageResponse defines a response to send for a matched message.
type WSMessageResponse struct {
	// Type is the message type: "text", "binary", "json"
	Type string `json:"type" yaml:"type"`
	// Value is the response content
	Value interface{} `json:"value" yaml:"value"`
	// Delay is the wait time before sending (e.g., "500ms")
	Delay string `json:"delay,omitempty" yaml:"delay,omitempty"`
}

// WSScenarioConfig defines a scripted WebSocket message sequence.
type WSScenarioConfig struct {
	// Name is the scenario name
	Name string `json:"name" yaml:"name"`
	// Steps is the ordered list of scenario steps
	Steps []*WSScenarioStepConfig `json:"steps" yaml:"steps"`
	// Loop restarts the scenario on completion
	Loop bool `json:"loop,omitempty" yaml:"loop,omitempty"`
	// ResetOnReconnect resets to step 0 on reconnect (default: true)
	ResetOnReconnect *bool `json:"resetOnReconnect,omitempty" yaml:"resetOnReconnect,omitempty"`
}

// WSScenarioStepConfig defines a single step in a WebSocket scenario.
type WSScenarioStepConfig struct {
	// Type is the step type: "send", "expect", "wait"
	Type string `json:"type" yaml:"type"`
	// Message is the message to send (for "send" type)
	Message *WSMessageResponse `json:"message,omitempty" yaml:"message,omitempty"`
	// Match is the expected message pattern (for "expect" type)
	Match *WSMatchCriteria `json:"match,omitempty" yaml:"match,omitempty"`
	// Duration is the wait duration (for "wait" type, e.g., "1s")
	Duration string `json:"duration,omitempty" yaml:"duration,omitempty"`
	// Timeout is the maximum wait for "expect" (default: "30s")
	Timeout string `json:"timeout,omitempty" yaml:"timeout,omitempty"`
	// Optional can be skipped if timeout reached
	Optional bool `json:"optional,omitempty" yaml:"optional,omitempty"`
}

// WSHeartbeatConfig configures WebSocket ping/pong keepalive.
type WSHeartbeatConfig struct {
	// Enabled enables heartbeat pings
	Enabled bool `json:"enabled" yaml:"enabled"`
	// Interval is the time between pings (e.g., "30s")
	Interval string `json:"interval,omitempty" yaml:"interval,omitempty"`
	// Timeout is the maximum wait for pong response (e.g., "10s")
	Timeout string `json:"timeout,omitempty" yaml:"timeout,omitempty"`
}

// StatefulResourceConfig defines configuration for a stateful CRUD resource.
type StatefulResourceConfig struct {
	// Name is the unique resource name (e.g., "users", "products")
	Name string `json:"name"`
	// BasePath is the URL path prefix (e.g., "/api/users")
	BasePath string `json:"basePath"`
	// IDField is the field name for ID (default: "id")
	IDField string `json:"idField,omitempty"`
	// ParentField is the field name for parent FK in nested resources
	ParentField string `json:"parentField,omitempty"`
	// SeedData is the initial data to load on startup/reset
	SeedData []map[string]interface{} `json:"seedData,omitempty"`
}

// DefaultServerConfiguration returns a ServerConfiguration with sensible defaults.
func DefaultServerConfiguration() *ServerConfiguration {
	return &ServerConfiguration{
		HTTPPort:         8080,
		HTTPSPort:        0,
		AdminPort:        9090,
		AutoGenerateCert: true,
		LogRequests:      true,
		MaxLogEntries:    1000,
		MaxBodySize:      10 * 1024 * 1024, // 10MB
		ReadTimeout:      30,
		WriteTimeout:     30,
	}
}
