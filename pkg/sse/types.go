package sse

import (
	"context"
	"net/http"
	"sync"
	"time"
)

// SSEConfig defines the configuration for an SSE streaming endpoint
type SSEConfig struct {
	// Events defines the sequence of events to send
	// If empty, Generator must be configured
	Events []SSEEventDef `json:"events,omitempty"`

	// Generator configures dynamic event generation
	// Mutually exclusive with Events
	Generator *EventGenerator `json:"generator,omitempty"`

	// Timing controls delay between events
	Timing TimingConfig `json:"timing"`

	// Lifecycle controls connection behavior
	Lifecycle LifecycleConfig `json:"lifecycle"`

	// RateLimit optionally throttles event delivery
	RateLimit *RateLimitConfig `json:"rateLimit,omitempty"`

	// Resume enables Last-Event-ID resumption
	Resume ResumeConfig `json:"resume"`

	// Template uses a built-in template (e.g., "openai-chat")
	// If set, other fields provide template parameters
	Template string `json:"template,omitempty"`

	// TemplateParams provides parameters for the template
	TemplateParams map[string]interface{} `json:"templateParams,omitempty"`
}

// SSEEventDef defines a single event in the stream
type SSEEventDef struct {
	// Type is the event type (optional, maps to "event:" field)
	Type string `json:"type,omitempty"`

	// Data is the event payload (required, maps to "data:" field)
	// Can be string or any JSON-serializable value
	Data interface{} `json:"data"`

	// ID is the event identifier (optional, maps to "id:" field)
	// If empty and resume is enabled, auto-generated as sequence number
	ID string `json:"id,omitempty"`

	// Retry suggests reconnection interval in ms (optional, maps to "retry:" field)
	// Only included in event if > 0
	Retry int `json:"retry,omitempty"`

	// Comment sends a comment line before the event (optional)
	// Maps to ": comment" line
	Comment string `json:"comment,omitempty"`

	// Delay overrides the timing config for this specific event
	// Expressed in milliseconds
	Delay *int `json:"delay,omitempty"`
}

// EventGenerator configures dynamic event generation for continuous streams
type EventGenerator struct {
	// Type specifies the generator type
	// Supported: "sequence", "random", "template"
	Type string `json:"type"`

	// Count is the total number of events to generate (0 = unlimited)
	Count int `json:"count,omitempty"`

	// Sequence generates incrementing values
	Sequence *SequenceGenerator `json:"sequence,omitempty"`

	// Random generates random data based on schema
	Random *RandomGenerator `json:"random,omitempty"`

	// TemplateGen repeats a template with variable substitution
	TemplateGen *TemplateGenerator `json:"template,omitempty"`
}

// SequenceGenerator produces incrementing numeric events
type SequenceGenerator struct {
	Start     int    `json:"start"`
	Increment int    `json:"increment"`
	Format    string `json:"format,omitempty"` // e.g., "event-%d"
}

// RandomGenerator produces random data events
type RandomGenerator struct {
	// Schema defines the JSON structure with random placeholders
	// Supports: $random(min,max), $uuid, $timestamp, $pick(a,b,c)
	Schema map[string]interface{} `json:"schema"`
}

// TemplateGenerator repeats events from a list
type TemplateGenerator struct {
	// Events to cycle through
	Events []SSEEventDef `json:"events"`
	// Repeat cycles through events this many times (0 = forever)
	Repeat int `json:"repeat,omitempty"`
}

// TimingConfig controls event delivery timing
type TimingConfig struct {
	// FixedDelay sets a constant delay between events (ms)
	FixedDelay *int `json:"fixedDelay,omitempty"`

	// RandomDelay sets a random delay range between events (ms)
	RandomDelay *RandomDelayConfig `json:"randomDelay,omitempty"`

	// PerEventDelays sets specific delays for each event (ms)
	// Length should match number of events
	PerEventDelays []int `json:"perEventDelays,omitempty"`

	// Burst configures burst delivery mode
	Burst *BurstConfig `json:"burst,omitempty"`

	// InitialDelay before first event (ms)
	InitialDelay int `json:"initialDelay,omitempty"`
}

// RandomDelayConfig defines a random delay range
type RandomDelayConfig struct {
	Min int `json:"min"` // Minimum delay in ms
	Max int `json:"max"` // Maximum delay in ms
}

// BurstConfig defines burst delivery mode
type BurstConfig struct {
	// Count is number of events per burst
	Count int `json:"count"`
	// Interval is delay between events within a burst (ms)
	Interval int `json:"interval"`
	// Pause is delay between bursts (ms)
	Pause int `json:"pause"`
}

// LifecycleConfig controls connection behavior
type LifecycleConfig struct {
	// KeepaliveInterval in seconds (0 = disabled)
	// Sends ": keepalive" comment at this interval
	KeepaliveInterval int `json:"keepaliveInterval,omitempty"`

	// MaxEvents closes connection after this many events (0 = unlimited)
	MaxEvents int `json:"maxEvents,omitempty"`

	// Timeout closes connection after this duration of no events (seconds)
	// 0 = no timeout
	Timeout int `json:"timeout,omitempty"`

	// ConnectionTimeout is max connection duration regardless of activity (seconds)
	// 0 = no limit
	ConnectionTimeout int `json:"connectionTimeout,omitempty"`

	// Termination defines how the stream ends
	Termination TerminationConfig `json:"termination,omitempty"`

	// SimulateDisconnect triggers an abrupt disconnect after N events
	SimulateDisconnect *int `json:"simulateDisconnect,omitempty"`
}

// TerminationConfig defines how the stream ends
type TerminationConfig struct {
	// Type: "graceful", "abrupt", "error"
	Type string `json:"type,omitempty"`

	// FinalEvent sent before graceful close (optional)
	FinalEvent *SSEEventDef `json:"finalEvent,omitempty"`

	// ErrorEvent sent on error termination (optional)
	ErrorEvent *SSEEventDef `json:"errorEvent,omitempty"`

	// CloseDelay before closing connection after final event (ms)
	CloseDelay int `json:"closeDelay,omitempty"`
}

// ResumeConfig controls Last-Event-ID resumption
type ResumeConfig struct {
	// Enabled allows clients to resume with Last-Event-ID header
	Enabled bool `json:"enabled"`

	// BufferSize is max events to keep for resumption
	BufferSize int `json:"bufferSize,omitempty"`

	// MaxAge is max age of buffered events (seconds)
	MaxAge int `json:"maxAge,omitempty"`
}

// RateLimitConfig controls event delivery rate
type RateLimitConfig struct {
	// EventsPerSecond is the max rate of event delivery
	EventsPerSecond float64 `json:"eventsPerSecond"`

	// BurstSize allows temporary bursts above the rate
	BurstSize int `json:"burstSize,omitempty"`

	// Strategy defines behavior when limit is reached
	// "wait" (default), "drop", "error"
	Strategy string `json:"strategy,omitempty"`

	// Headers include rate limit headers in response
	Headers bool `json:"headers,omitempty"`
}

// SSEStream represents an active SSE connection
type SSEStream struct {
	// ID uniquely identifies this connection
	ID string `json:"id"`

	// MockID is the mock this stream is serving
	MockID string `json:"mockId"`

	// ClientIP is the client's IP address
	ClientIP string `json:"clientIp"`

	// UserAgent from the request
	UserAgent string `json:"userAgent,omitempty"`

	// StartTime when connection was established
	StartTime time.Time `json:"startTime"`

	// LastEventTime when last event was sent
	LastEventTime *time.Time `json:"lastEventTime,omitempty"`

	// EventsSent count of events delivered
	EventsSent int64 `json:"eventsSent"`

	// BytesSent total bytes sent
	BytesSent int64 `json:"bytesSent"`

	// LastEventID of the last sent event
	LastEventID string `json:"lastEventId,omitempty"`

	// ResumedFrom is the Last-Event-ID header from client (if any)
	ResumedFrom string `json:"resumedFrom,omitempty"`

	// Status of the connection
	Status StreamStatus `json:"status"`

	// Internal fields (not serialized)
	ctx     context.Context       `json:"-"`
	cancel  context.CancelFunc    `json:"-"`
	writer  http.ResponseWriter   `json:"-"`
	flusher http.Flusher          `json:"-"`
	config  *SSEConfig            `json:"-"`
	mu      sync.Mutex            `json:"-"`
	eventCh chan *SSEEventDef     `json:"-"`
	errCh   chan error            `json:"-"`
	doneCh  chan struct{}         `json:"-"`
}

// SSEConnectionManager tracks active SSE connections
type SSEConnectionManager struct {
	// connections maps stream ID to stream
	connections map[string]*SSEStream
	mu          sync.RWMutex

	// maxConnections limits total active connections (0 = unlimited)
	maxConnections int

	// metrics for observability
	totalConnections int64
	totalEventsSent  int64
	totalBytesSent   int64
	connectionErrors int64
}

// ConnectionStats provides aggregated statistics
type ConnectionStats struct {
	ActiveConnections int            `json:"activeConnections"`
	TotalConnections  int64          `json:"totalConnections"`
	TotalEventsSent   int64          `json:"totalEventsSent"`
	TotalBytesSent    int64          `json:"totalBytesSent"`
	ConnectionErrors  int64          `json:"connectionErrors"`
	ConnectionsByMock map[string]int `json:"connectionsByMock"`
}

// EventBuffer stores events for resumption
type EventBuffer struct {
	events  []BufferedEvent
	maxSize int
	maxAge  time.Duration
	mu      sync.RWMutex
}

// BufferedEvent wraps an event with metadata for resumption
type BufferedEvent struct {
	ID        string      `json:"id"`
	Event     SSEEventDef `json:"event"`
	Timestamp time.Time   `json:"timestamp"`
	Index     int64       `json:"index"`
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

// OpenAIChatTemplate parameters for OpenAI streaming responses
type OpenAIChatTemplate struct {
	// Tokens to stream (each becomes a delta event)
	Tokens []string `json:"tokens"`

	// Model name in response
	Model string `json:"model,omitempty"`

	// DelayPerToken in ms (default: 50)
	DelayPerToken int `json:"delayPerToken,omitempty"`

	// FinishReason: "stop", "length", "function_call"
	FinishReason string `json:"finishReason,omitempty"`

	// IncludeDone sends "data: [DONE]" at end
	IncludeDone bool `json:"includeDone,omitempty"`
}

// NotificationTemplate parameters for generic notification streams
type NotificationTemplate struct {
	// Messages to stream
	Messages []NotificationMessage `json:"messages"`

	// Interval between messages (ms)
	Interval int `json:"interval,omitempty"`

	// Loop back to start when done
	Loop bool `json:"loop,omitempty"`
}

// NotificationMessage represents a notification event
type NotificationMessage struct {
	Type    string      `json:"type"`
	Payload interface{} `json:"payload"`
}

// Admin API response types

// ListConnectionsResponse represents a list of SSE connections
type ListConnectionsResponse struct {
	Connections []SSEStreamInfo `json:"connections"`
	Stats       ConnectionStats `json:"stats"`
}

// SSEStreamInfo represents a single connection for API responses
type SSEStreamInfo struct {
	ID         string       `json:"id"`
	MockID     string       `json:"mockId"`
	MockName   string       `json:"mockName,omitempty"`
	ClientIP   string       `json:"clientIp"`
	StartTime  time.Time    `json:"startTime"`
	Duration   string       `json:"duration"`
	EventsSent int64        `json:"eventsSent"`
	Status     StreamStatus `json:"status"`
}

// CloseConnectionRequest represents a request to close an SSE connection
type CloseConnectionRequest struct {
	// Graceful sends final event before closing
	Graceful bool `json:"graceful,omitempty"`

	// FinalEvent to send before closing (optional)
	FinalEvent *SSEEventDef `json:"finalEvent,omitempty"`
}
