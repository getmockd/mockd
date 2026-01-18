// Package recording provides frame types for stream recordings.
package recording

import (
	"encoding/base64"
	"time"
)

// WebSocketFrame represents a single WebSocket message in a recording.
type WebSocketFrame struct {
	// Sequence is a monotonic sequence number.
	Sequence int64 `json:"seq"`

	// Timestamp is the absolute time when the frame was recorded.
	Timestamp time.Time `json:"ts"`

	// RelativeMs is milliseconds from recording start.
	RelativeMs int64 `json:"rel"`

	// Direction indicates client-to-server or server-to-client.
	Direction Direction `json:"dir"`

	// MessageType is the WebSocket frame type.
	MessageType MessageType `json:"type"`

	// Data is the frame content (UTF-8 for text, base64 for binary).
	Data string `json:"data"`

	// DataEncoding indicates how Data is encoded.
	DataEncoding DataEncoding `json:"encoding"`

	// DataSize is the original size in bytes.
	DataSize int `json:"size"`

	// CloseCode is present for close frames.
	CloseCode *int `json:"closeCode,omitempty"`

	// CloseReason is present for close frames.
	CloseReason *string `json:"closeReason,omitempty"`
}

// NewWebSocketFrame creates a new WebSocket frame for recording.
func NewWebSocketFrame(seq int64, startTime time.Time, dir Direction, msgType MessageType, data []byte) WebSocketFrame {
	now := time.Now()
	frame := WebSocketFrame{
		Sequence:    seq,
		Timestamp:   now,
		RelativeMs:  now.Sub(startTime).Milliseconds(),
		Direction:   dir,
		MessageType: msgType,
		DataSize:    len(data),
	}

	// Encode data based on message type
	if msgType == MessageTypeBinary {
		frame.Data = base64.StdEncoding.EncodeToString(data)
		frame.DataEncoding = DataEncodingBase64
	} else {
		frame.Data = string(data)
		frame.DataEncoding = DataEncodingUTF8
	}

	return frame
}

// NewWebSocketCloseFrame creates a close frame for recording.
func NewWebSocketCloseFrame(seq int64, startTime time.Time, dir Direction, code int, reason string) WebSocketFrame {
	now := time.Now()
	return WebSocketFrame{
		Sequence:     seq,
		Timestamp:    now,
		RelativeMs:   now.Sub(startTime).Milliseconds(),
		Direction:    dir,
		MessageType:  MessageTypeClose,
		Data:         reason,
		DataEncoding: DataEncodingUTF8,
		DataSize:     len(reason),
		CloseCode:    &code,
		CloseReason:  &reason,
	}
}

// GetData returns the decoded data bytes.
func (f *WebSocketFrame) GetData() ([]byte, error) {
	if f.DataEncoding == DataEncodingBase64 {
		return base64.StdEncoding.DecodeString(f.Data)
	}
	return []byte(f.Data), nil
}

// SSEEvent represents a single SSE event in a recording.
type SSEEvent struct {
	// Sequence is a monotonic sequence number.
	Sequence int64 `json:"seq"`

	// Timestamp is the absolute time when the event was recorded.
	Timestamp time.Time `json:"ts"`

	// RelativeMs is milliseconds from first event.
	RelativeMs int64 `json:"rel"`

	// EventType is the SSE event type (event: field).
	EventType string `json:"event,omitempty"`

	// Data is the SSE data field.
	Data string `json:"data"`

	// ID is the SSE event ID (id: field).
	ID string `json:"id,omitempty"`

	// Retry is the SSE retry field in milliseconds.
	Retry *int `json:"retry,omitempty"`

	// Comment is any SSE comment preceding the event.
	Comment string `json:"comment,omitempty"`

	// DataSize is the size of Data in bytes.
	DataSize int `json:"size"`
}

// NewSSEEvent creates a new SSE event for recording.
func NewSSEEvent(seq int64, firstEventTime time.Time, eventType, data, id string, retry *int) SSEEvent {
	now := time.Now()
	relMs := int64(0)
	if !firstEventTime.IsZero() {
		relMs = now.Sub(firstEventTime).Milliseconds()
	}

	return SSEEvent{
		Sequence:   seq,
		Timestamp:  now,
		RelativeMs: relMs,
		EventType:  eventType,
		Data:       data,
		ID:         id,
		Retry:      retry,
		DataSize:   len(data),
	}
}

// WebSocketRecordingData contains WebSocket-specific recording data.
type WebSocketRecordingData struct {
	// ConnectedAt is when the WebSocket connection was established.
	ConnectedAt time.Time `json:"connectedAt"`

	// DisconnectedAt is when the connection closed.
	DisconnectedAt *time.Time `json:"disconnectedAt,omitempty"`

	// CloseCode is the WebSocket close code.
	CloseCode *int `json:"closeCode,omitempty"`

	// CloseReason is the WebSocket close reason.
	CloseReason *string `json:"closeReason,omitempty"`

	// Frames contains all recorded WebSocket frames.
	Frames []WebSocketFrame `json:"frames"`
}

// SSERecordingData contains SSE-specific recording data.
type SSERecordingData struct {
	// RequestBody is the request body for POST requests.
	RequestBody string `json:"requestBody,omitempty"`

	// RequestBodyB64 is base64-encoded request body if binary.
	RequestBodyB64 string `json:"requestBodyB64,omitempty"`

	// ResponseStatus is the HTTP response status code.
	ResponseStatus int `json:"responseStatus"`

	// ResponseHeaders are the response headers.
	ResponseHeaders map[string]string `json:"responseHeaders,omitempty"`

	// StreamStartedAt is when the first event was received.
	StreamStartedAt time.Time `json:"streamStartedAt"`

	// StreamEndedAt is when the stream ended.
	StreamEndedAt *time.Time `json:"streamEndedAt,omitempty"`

	// Events contains all recorded SSE events.
	Events []SSEEvent `json:"events"`
}

// HTTPRecordingData contains HTTP-specific recording data.
// This wraps the existing Recording type for unified storage.
type HTTPRecordingData struct {
	// Request is the recorded HTTP request.
	Request HTTPRecordedRequest `json:"request"`

	// Response is the recorded HTTP response.
	Response HTTPRecordedResponse `json:"response"`

	// RequestedAt is when the request was made.
	RequestedAt time.Time `json:"requestedAt"`

	// RespondedAt is when the response was received.
	RespondedAt time.Time `json:"respondedAt"`

	// DurationMs is the request duration in milliseconds.
	DurationMs int64 `json:"durationMs"`
}

// HTTPRecordedRequest is the recorded HTTP request.
type HTTPRecordedRequest struct {
	Method   string            `json:"method"`
	URL      string            `json:"url"`
	Path     string            `json:"path"`
	Host     string            `json:"host"`
	Headers  map[string]string `json:"headers,omitempty"`
	Body     string            `json:"body,omitempty"`
	BodyB64  string            `json:"bodyB64,omitempty"`
	BodySize int               `json:"bodySize"`
}

// HTTPRecordedResponse is the recorded HTTP response.
type HTTPRecordedResponse struct {
	StatusCode int               `json:"statusCode"`
	Status     string            `json:"status"`
	Headers    map[string]string `json:"headers,omitempty"`
	Body       string            `json:"body,omitempty"`
	BodyB64    string            `json:"bodyB64,omitempty"`
	BodySize   int               `json:"bodySize"`
}
