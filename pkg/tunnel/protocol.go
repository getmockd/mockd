package tunnel

import (
	"encoding/json"
)

// Message types for tunnel protocol.
const (
	MessageTypeRequest    = "request"
	MessageTypeResponse   = "response"
	MessageTypePing       = "ping"
	MessageTypePong       = "pong"
	MessageTypeError      = "error"
	MessageTypeConnected  = "connected"
	MessageTypeDisconnect = "disconnect"
)

// TunnelMessage represents messages between relay and client.
type TunnelMessage struct {
	Type    string            `json:"type"`              // "request", "response", "ping", "pong", "error", "connected", "disconnect"
	ID      string            `json:"id"`                // Request correlation ID
	Method  string            `json:"method,omitempty"`  // HTTP method (for requests)
	Path    string            `json:"path,omitempty"`    // Request path
	Headers map[string]string `json:"headers,omitempty"` // HTTP headers
	Body    []byte            `json:"body,omitempty"`    // Request/response body
	Status  int               `json:"status,omitempty"`  // Response status code
	Error   string            `json:"error,omitempty"`   // Error message
}

// ConnectedMessage is received from the relay after successful connection.
type ConnectedMessage struct {
	Type      string `json:"type"`       // Always "connected"
	SessionID string `json:"session_id"` // Session identifier
	PublicURL string `json:"public_url"` // The public URL for this tunnel
	Subdomain string `json:"subdomain"`  // Assigned subdomain
}

// ErrorMessage is received when an error occurs.
type ErrorMessage struct {
	Type    string `json:"type"`    // Always "error"
	ID      string `json:"id"`      // Request ID if applicable
	Code    string `json:"code"`    // Error code
	Message string `json:"message"` // Human-readable error message
}

// NewResponseMessage creates a new response message.
func NewResponseMessage(id string, status int, headers map[string]string, body []byte) *TunnelMessage {
	return &TunnelMessage{
		Type:    MessageTypeResponse,
		ID:      id,
		Status:  status,
		Headers: headers,
		Body:    body,
	}
}

// NewErrorMessage creates a new error message.
func NewErrorMessage(id, code, message string) *TunnelMessage {
	return &TunnelMessage{
		Type:  MessageTypeError,
		ID:    id,
		Error: code + ": " + message,
	}
}

// NewPongMessage creates a pong message in response to a ping.
func NewPongMessage(pingID string) *TunnelMessage {
	return &TunnelMessage{
		Type: MessageTypePong,
		ID:   pingID,
	}
}

// Encode serializes a message to JSON bytes.
func (m *TunnelMessage) Encode() ([]byte, error) {
	return json.Marshal(m)
}

// DecodeMessage deserializes a JSON message.
func DecodeMessage(data []byte) (*TunnelMessage, error) {
	var msg TunnelMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		return nil, err
	}
	return &msg, nil
}

// DecodeConnectedMessage deserializes a connected message.
func DecodeConnectedMessage(data []byte) (*ConnectedMessage, error) {
	var msg ConnectedMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		return nil, err
	}
	return &msg, nil
}
