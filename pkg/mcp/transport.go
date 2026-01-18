package mcp

import (
	"fmt"
	"net/http"
	"strings"
	"sync/atomic"
	"time"
)

// HTTP headers used by MCP protocol.
const (
	HeaderSessionID       = "Mcp-Session-Id"
	HeaderProtocolVersion = "MCP-Protocol-Version"
	HeaderLastEventID     = "Last-Event-ID"
	HeaderContentType     = "Content-Type"
	HeaderAccept          = "Accept"
	HeaderOrigin          = "Origin"
)

// Content types.
const (
	ContentTypeJSON        = "application/json"
	ContentTypeEventStream = "text/event-stream"
)

// SSEWriter handles writing Server-Sent Events.
type SSEWriter struct {
	w        http.ResponseWriter
	flusher  http.Flusher
	eventID  atomic.Int64
	closed   bool
}

// NewSSEWriter creates a new SSE writer.
func NewSSEWriter(w http.ResponseWriter) (*SSEWriter, error) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		return nil, fmt.Errorf("response writer does not support flushing")
	}

	return &SSEWriter{
		w:       w,
		flusher: flusher,
	}, nil
}

// WriteHeaders sets the necessary headers for SSE.
func (s *SSEWriter) WriteHeaders() {
	s.w.Header().Set(HeaderContentType, ContentTypeEventStream)
	s.w.Header().Set("Cache-Control", "no-cache")
	s.w.Header().Set("Connection", "keep-alive")
	s.w.Header().Set("X-Accel-Buffering", "no") // Disable nginx buffering
}

// WriteEvent writes an SSE event.
func (s *SSEWriter) WriteEvent(event *SSEEvent) error {
	if s.closed {
		return fmt.Errorf("writer is closed")
	}

	var sb strings.Builder

	// Event ID
	if event.ID != "" {
		sb.WriteString("id: ")
		sb.WriteString(event.ID)
		sb.WriteByte('\n')
	} else {
		// Auto-generate ID
		id := s.eventID.Add(1)
		sb.WriteString("id: ")
		sb.WriteString(formatInt64(id))
		sb.WriteByte('\n')
	}

	// Event type
	if event.Event != "" {
		sb.WriteString("event: ")
		sb.WriteString(event.Event)
		sb.WriteByte('\n')
	}

	// Retry hint
	if event.Retry > 0 {
		sb.WriteString("retry: ")
		sb.WriteString(formatInt64(int64(event.Retry)))
		sb.WriteByte('\n')
	}

	// Data (handle multiline)
	lines := strings.Split(event.Data, "\n")
	for _, line := range lines {
		sb.WriteString("data: ")
		sb.WriteString(line)
		sb.WriteByte('\n')
	}

	// End with blank line
	sb.WriteByte('\n')

	_, err := s.w.Write([]byte(sb.String()))
	if err != nil {
		return err
	}

	s.flusher.Flush()
	return nil
}

// WriteComment writes an SSE comment (keepalive).
func (s *SSEWriter) WriteComment(comment string) error {
	if s.closed {
		return fmt.Errorf("writer is closed")
	}

	_, err := fmt.Fprintf(s.w, ": %s\n\n", comment)
	if err != nil {
		return err
	}

	s.flusher.Flush()
	return nil
}

// WriteKeepalive writes a keepalive comment.
func (s *SSEWriter) WriteKeepalive() error {
	return s.WriteComment("keepalive")
}

// Close marks the writer as closed.
func (s *SSEWriter) Close() {
	s.closed = true
}

// formatInt64 formats an int64 as string without using fmt.
func formatInt64(n int64) string {
	if n == 0 {
		return "0"
	}

	negative := n < 0
	if negative {
		n = -n
	}

	var digits [20]byte
	i := len(digits)

	for n > 0 {
		i--
		digits[i] = byte('0' + n%10)
		n /= 10
	}

	if negative {
		i--
		digits[i] = '-'
	}

	return string(digits[i:])
}

// TransportConfig holds transport-specific configuration.
type TransportConfig struct {
	// KeepaliveInterval is the interval for sending keepalive comments.
	KeepaliveInterval time.Duration

	// ReadTimeout is the timeout for reading requests.
	ReadTimeout time.Duration

	// WriteTimeout is the timeout for writing responses.
	WriteTimeout time.Duration
}

// DefaultTransportConfig returns default transport configuration.
func DefaultTransportConfig() *TransportConfig {
	return &TransportConfig{
		KeepaliveInterval: 30 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
	}
}

// RequestInfo extracts information from an HTTP request relevant to MCP.
type RequestInfo struct {
	SessionID       string
	ProtocolVersion string
	LastEventID     string
	ContentType     string
	Accept          string
	Origin          string
}

// ExtractRequestInfo extracts MCP-relevant information from a request.
func ExtractRequestInfo(r *http.Request) *RequestInfo {
	return &RequestInfo{
		SessionID:       r.Header.Get(HeaderSessionID),
		ProtocolVersion: r.Header.Get(HeaderProtocolVersion),
		LastEventID:     r.Header.Get(HeaderLastEventID),
		ContentType:     r.Header.Get(HeaderContentType),
		Accept:          r.Header.Get(HeaderAccept),
		Origin:          r.Header.Get(HeaderOrigin),
	}
}

// WantsSSE checks if the request wants an SSE stream.
func (ri *RequestInfo) WantsSSE() bool {
	return strings.Contains(ri.Accept, ContentTypeEventStream)
}

// WantsJSON checks if the request wants JSON response.
func (ri *RequestInfo) WantsJSON() bool {
	return strings.Contains(ri.Accept, ContentTypeJSON) || ri.Accept == "" || ri.Accept == "*/*"
}
