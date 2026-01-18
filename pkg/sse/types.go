package sse

import (
	"context"
	"net/http"
	"sync"
	"time"

	"github.com/getmockd/mockd/pkg/mock"
)

// Config type aliases - canonical definitions are in pkg/mock/types.go
type (
	// SSEConfig defines the configuration for an SSE streaming endpoint
	SSEConfig = mock.SSEConfig

	// SSEEventDef defines a single event in the stream
	SSEEventDef = mock.SSEEventDef

	// EventGenerator configures dynamic event generation for continuous streams
	EventGenerator = mock.SSEEventGenerator

	// SequenceGenerator produces incrementing numeric events
	SequenceGenerator = mock.SSESequenceGenerator

	// RandomGenerator produces random data events
	RandomGenerator = mock.SSERandomGenerator

	// TemplateGenerator repeats events from a list
	TemplateGenerator = mock.SSETemplateGenerator

	// TimingConfig controls event delivery timing
	TimingConfig = mock.SSETimingConfig

	// RandomDelayConfig defines a random delay range
	RandomDelayConfig = mock.SSERandomDelayConfig

	// BurstConfig defines burst delivery mode
	BurstConfig = mock.SSEBurstConfig

	// LifecycleConfig controls connection behavior
	LifecycleConfig = mock.SSELifecycleConfig

	// TerminationConfig defines how the stream ends
	TerminationConfig = mock.SSETerminationConfig

	// ResumeConfig controls Last-Event-ID resumption
	ResumeConfig = mock.SSEResumeConfig

	// RateLimitConfig controls event delivery rate
	RateLimitConfig = mock.SSERateLimitConfig

	// ChunkedConfig configures HTTP chunked transfer encoding
	ChunkedConfig = mock.ChunkedConfig
)

// SSEStream represents an active SSE connection
type SSEStream struct {
	// ID uniquely identifies this connection
	ID string `json:"id"`

	// MockID is the mock this stream is serving
	MockID string `json:"mockId"`

	// Path is the endpoint path for this stream
	Path string `json:"path"`

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
	ctx      context.Context     `json:"-"`
	cancel   context.CancelFunc  `json:"-"`
	writer   http.ResponseWriter `json:"-"`
	flusher  http.Flusher        `json:"-"`
	config   *SSEConfig          `json:"-"`
	mu       sync.Mutex          `json:"-"`
	eventCh  chan *SSEEventDef   `json:"-"`
	errCh    chan error          `json:"-"`
	doneCh   chan struct{}       `json:"-"`
	recorder *StreamRecorder     `json:"-"`
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
