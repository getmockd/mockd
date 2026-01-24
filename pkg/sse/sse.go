// Package sse provides Server-Sent Events (SSE) and HTTP streaming support for the mock server.
// It implements SSE per the W3C specification and supports HTTP chunked transfer encoding.
package sse

import (
	"errors"
)

// SSE-related constants per W3C specification
const (
	// ContentTypeEventStream is the MIME type for SSE responses
	ContentTypeEventStream = "text/event-stream"

	// ContentTypeNDJSON is the MIME type for newline-delimited JSON
	ContentTypeNDJSON = "application/x-ndjson"

	// DefaultKeepaliveInterval is the default keepalive interval in seconds
	DefaultKeepaliveInterval = 15

	// DefaultBufferSize is the default event buffer size for resumption
	DefaultBufferSize = 100

	// DefaultMaxEvents is the default maximum events per connection (0 = unlimited)
	DefaultMaxEvents = 0

	// DefaultRetryMs is the default retry interval in milliseconds
	DefaultRetryMs = 3000

	// MaxEventDataSize is the maximum size of event data in bytes
	MaxEventDataSize = 1 << 20 // 1MB
)

// SSE field prefixes per W3C specification
const (
	fieldEvent   = "event:"
	fieldData    = "data:"
	fieldID      = "id:"
	fieldRetry   = "retry:"
	fieldComment = ":"
)

// StreamStatus represents the status of an SSE stream.
type StreamStatus string

const (
	StreamStatusConnecting StreamStatus = "connecting"
	StreamStatusActive     StreamStatus = "active"
	StreamStatusPaused     StreamStatus = "paused"
	StreamStatusClosing    StreamStatus = "closing"
	StreamStatusClosed     StreamStatus = "closed"
)

// Termination types
const (
	TerminationGraceful = "graceful"
	TerminationAbrupt   = "abrupt"
	TerminationError    = "error"
)

// Generator types
const (
	GeneratorSequence = "sequence"
	GeneratorRandom   = "random"
	GeneratorTemplate = "template"
)

// Rate limit strategies
const (
	RateLimitStrategyWait  = "wait"
	RateLimitStrategyDrop  = "drop"
	RateLimitStrategyError = "error"
)

// Backpressure strategies
const (
	BackpressureBuffer = "buffer"
	BackpressureDrop   = "drop"
	BackpressureBlock  = "block"
)

// Built-in template names
const (
	TemplateOpenAIChat         = "openai-chat"
	TemplateNotificationStream = "notification-stream"
)

// Errors
var (
	// ErrStreamClosed indicates the stream has been closed
	ErrStreamClosed = errors.New("sse: stream closed")

	// ErrClientDisconnected indicates the client has disconnected
	ErrClientDisconnected = errors.New("sse: client disconnected")

	// ErrMaxEventsReached indicates the maximum event count was reached
	ErrMaxEventsReached = errors.New("sse: maximum events reached")

	// ErrConnectionTimeout indicates the connection timed out
	ErrConnectionTimeout = errors.New("sse: connection timeout")

	// ErrRateLimited indicates the stream is being rate limited
	ErrRateLimited = errors.New("sse: rate limited")

	// ErrBufferFull indicates the event buffer is full
	ErrBufferFull = errors.New("sse: buffer full")

	// ErrInvalidConfig indicates invalid SSE configuration
	ErrInvalidConfig = errors.New("sse: invalid configuration")

	// ErrEventTooLarge indicates the event data exceeds size limit
	ErrEventTooLarge = errors.New("sse: event data too large")

	// ErrInvalidEventID indicates an invalid event ID
	ErrInvalidEventID = errors.New("sse: invalid event ID")

	// ErrTemplateNotFound indicates the requested template was not found
	ErrTemplateNotFound = errors.New("sse: template not found")

	// ErrFlusherNotSupported indicates the response writer doesn't support flushing
	ErrFlusherNotSupported = errors.New("sse: flusher not supported")

	// ErrMaxConnectionsReached indicates the maximum connection count was reached
	ErrMaxConnectionsReached = errors.New("sse: maximum connections reached")
)
