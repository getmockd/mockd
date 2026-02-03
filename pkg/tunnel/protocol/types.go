// Package protocol defines the wire protocol for QUIC relay communication.
//
// IMPORTANT: This file is the source of truth for the wire protocol.
// The relay server at mockd-relay/internal/protocol/types.go must be kept in sync.
// To sync: cp pkg/tunnel/protocol/types.go ../mockd-relay/internal/protocol/types.go
package protocol

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
)

// ProtocolVersion is the protocol version.
const ProtocolVersion uint8 = 1

// StreamType identifies the type of data being transmitted on a QUIC stream.
type StreamType uint8

const (
	// StreamTypeControl is for control messages (auth, ping, config).
	StreamTypeControl StreamType = 0
	// StreamTypeHTTP is for HTTP request/response streams (half-duplex).
	StreamTypeHTTP StreamType = 1
	// StreamTypeMQTT is for native MQTT connection passthrough (bidirectional).
	StreamTypeMQTT StreamType = 2
	// StreamTypeGRPC is for gRPC streams over HTTP/2 (bidirectional).
	StreamTypeGRPC StreamType = 3
	// StreamTypeWebSocket is for WebSocket frame passthrough (bidirectional).
	StreamTypeWebSocket StreamType = 4
)

func (t StreamType) String() string {
	switch t {
	case StreamTypeControl:
		return "control"
	case StreamTypeHTTP:
		return "http"
	case StreamTypeMQTT:
		return "mqtt"
	case StreamTypeGRPC:
		return "grpc"
	case StreamTypeWebSocket:
		return "websocket"
	default:
		return fmt.Sprintf("unknown(%d)", t)
	}
}

// IsBidirectional returns true if this stream type uses bidirectional byte
// bridging after the initial metadata exchange (as opposed to half-duplex
// request/response like StreamTypeHTTP).
func (t StreamType) IsBidirectional() bool {
	switch t {
	case StreamTypeMQTT, StreamTypeGRPC, StreamTypeWebSocket:
		return true
	default:
		return false
	}
}

// StreamHeader is the header sent at the beginning of each QUIC stream.
// Binary format:
//   - Version: 1 byte
//   - Type: 1 byte
//   - Flags: 1 byte
//   - Reserved: 1 byte
//   - MetadataLen: 4 bytes (big-endian)
//   - Metadata: variable (JSON)
type StreamHeader struct {
	Version  uint8
	Type     StreamType
	Flags    uint8
	Metadata []byte
}

// Header flags.
const (
	FlagNone uint8 = 0
	// FlagBidirectional indicates the stream remains open in both directions
	// after the initial metadata exchange. Neither side should close its write
	// direction to signal "body done" — instead, both sides bridge bytes
	// concurrently until one side closes or the connection ends.
	FlagBidirectional uint8 = 1 << 0
	// FlagTrailer marks a StreamHeader as carrying HTTP trailer metadata.
	// Sent after the body's end-of-body sentinel in the gRPC forwarding path.
	FlagTrailer uint8 = 1 << 1
)

// EncodeHeader writes a stream header to the writer.
func EncodeHeader(w io.Writer, h *StreamHeader) error {
	// Fixed header: 8 bytes
	buf := make([]byte, 8)
	buf[0] = h.Version
	buf[1] = byte(h.Type)
	buf[2] = h.Flags
	buf[3] = 0 // reserved
	binary.BigEndian.PutUint32(buf[4:8], uint32(len(h.Metadata)))

	if _, err := w.Write(buf); err != nil {
		return fmt.Errorf("write header: %w", err)
	}

	if len(h.Metadata) > 0 {
		if _, err := w.Write(h.Metadata); err != nil {
			return fmt.Errorf("write metadata: %w", err)
		}
	}

	return nil
}

// DecodeHeader reads a stream header from the reader.
func DecodeHeader(r io.Reader) (*StreamHeader, error) {
	// Fixed header: 8 bytes
	buf := make([]byte, 8)
	if _, err := io.ReadFull(r, buf); err != nil {
		return nil, fmt.Errorf("read header: %w", err)
	}

	h := &StreamHeader{
		Version: buf[0],
		Type:    StreamType(buf[1]),
		Flags:   buf[2],
	}

	metadataLen := binary.BigEndian.Uint32(buf[4:8])
	if metadataLen > 0 {
		if metadataLen > 1024*64 { // 64KB max
			return nil, fmt.Errorf("metadata too large: %d bytes", metadataLen)
		}
		h.Metadata = make([]byte, metadataLen)
		if _, err := io.ReadFull(r, h.Metadata); err != nil {
			return nil, fmt.Errorf("read metadata: %w", err)
		}
	}

	return h, nil
}

// WriteBodyChunk writes a length-prefixed body chunk to w.
// A nil or empty data slice writes the end-of-body sentinel (length 0).
func WriteBodyChunk(w io.Writer, data []byte) error {
	var buf [4]byte
	binary.BigEndian.PutUint32(buf[:], uint32(len(data)))
	if _, err := w.Write(buf[:]); err != nil {
		return fmt.Errorf("write chunk length: %w", err)
	}
	if len(data) > 0 {
		if _, err := w.Write(data); err != nil {
			return fmt.Errorf("write chunk data: %w", err)
		}
	}
	return nil
}

// ReadBodyChunk reads a length-prefixed body chunk from r.
// Returns (nil, nil) when the end-of-body sentinel (length 0) is read.
func ReadBodyChunk(r io.Reader) ([]byte, error) {
	var buf [4]byte
	if _, err := io.ReadFull(r, buf[:]); err != nil {
		return nil, fmt.Errorf("read chunk length: %w", err)
	}
	n := binary.BigEndian.Uint32(buf[:])
	if n == 0 {
		return nil, nil // end-of-body sentinel
	}
	if n > 4*1024*1024 { // 4MB max chunk sanity guard
		return nil, fmt.Errorf("chunk too large: %d bytes", n)
	}
	data := make([]byte, n)
	if _, err := io.ReadFull(r, data); err != nil {
		return nil, fmt.Errorf("read chunk data: %w", err)
	}
	return data, nil
}

// HTTPMetadata contains metadata for HTTP request/response streams.
// Used for StreamTypeHTTP, StreamTypeGRPC, and StreamTypeWebSocket
// (which all begin with an HTTP request).
type HTTPMetadata struct {
	// Request fields
	Method string              `json:"method,omitempty"`
	Path   string              `json:"path,omitempty"`
	Host   string              `json:"host,omitempty"`
	Header map[string][]string `json:"header,omitempty"`

	// Response fields
	StatusCode int                 `json:"status_code,omitempty"`
	Trailer    map[string][]string `json:"trailer,omitempty"`
}

// EncodeHTTPMetadata encodes HTTP metadata to JSON bytes.
func EncodeHTTPMetadata(m *HTTPMetadata) ([]byte, error) {
	return json.Marshal(m)
}

// DecodeHTTPMetadata decodes HTTP metadata from JSON bytes.
func DecodeHTTPMetadata(data []byte) (*HTTPMetadata, error) {
	var m HTTPMetadata
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	return &m, nil
}

// MQTTMetadata contains metadata for native MQTT connection streams.
type MQTTMetadata struct {
	// BrokerName identifies which local MQTT broker to connect to.
	// Maps to the subdomain prefix: {broker}.{session}.tunnel.mockd.io
	BrokerName string `json:"broker_name,omitempty"`

	// ClientID is the MQTT client identifier from the CONNECT packet.
	ClientID string `json:"client_id,omitempty"`
}

// EncodeMQTTMetadata encodes MQTT metadata to JSON bytes.
func EncodeMQTTMetadata(m *MQTTMetadata) ([]byte, error) {
	return json.Marshal(m)
}

// DecodeMQTTMetadata decodes MQTT metadata from JSON bytes.
func DecodeMQTTMetadata(data []byte) (*MQTTMetadata, error) {
	var m MQTTMetadata
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	return &m, nil
}

// ControlMessage types
const (
	ControlTypeAuth       = "auth"
	ControlTypeAuthOK     = "auth_ok"
	ControlTypeAuthError  = "auth_error"
	ControlTypePing       = "ping"
	ControlTypePong       = "pong"
	ControlTypeDisconnect = "disconnect"
	ControlTypeGoaway     = "goaway"
)

// ControlMessage is sent on the control stream (stream 0).
type ControlMessage struct {
	Type    string `json:"type"`
	Payload any    `json:"payload,omitempty"`
}

// AuthPayload is the payload for auth messages.
type AuthPayload struct {
	Token      string         `json:"token"`
	LocalPort  int            `json:"local_port"`
	Protocols  []ProtocolPort `json:"protocols,omitempty"`
	TunnelAuth *TunnelAuth    `json:"tunnel_auth,omitempty"`
}

// ProtocolPort declares a protocol and local port that the agent wants to expose.
type ProtocolPort struct {
	// Type is the protocol: "http", "grpc", "websocket", "mqtt".
	Type string `json:"type"`

	// Port is the local port the protocol is listening on.
	Port int `json:"port"`

	// Name is an optional identifier for multi-instance protocols (e.g., MQTT broker name).
	// Used for subdomain routing: {name}.{session}.tunnel.mockd.io
	Name string `json:"name,omitempty"`
}

// TunnelAuth configures authentication for incoming requests to a tunnel URL.
// In relay-terminated mode, the relay enforces this. In E2E mode, the agent enforces it.
type TunnelAuth struct {
	// Type is the auth mode: "none", "token", "basic", "ip".
	Type string `json:"type"`

	// Token is the secret value for type=token.
	Token string `json:"token,omitempty"`

	// TokenHeader is the HTTP header name to check for the token.
	// Default: "X-Tunnel-Token". Configurable to avoid conflicts with mock headers.
	TokenHeader string `json:"token_header,omitempty"`

	// Username is the username for type=basic.
	Username string `json:"username,omitempty"`

	// Password is the password for type=basic.
	Password string `json:"password,omitempty"`

	// AllowedIPs is a list of CIDR ranges for type=ip (e.g., ["10.0.0.0/8", "192.168.1.0/24"]).
	AllowedIPs []string `json:"allowed_ips,omitempty"`
}

// DefaultTokenHeader is the default HTTP header name for tunnel token auth.
const DefaultTokenHeader = "X-Tunnel-Token"

// EffectiveTokenHeader returns the token header name, falling back to the default.
func (a *TunnelAuth) EffectiveTokenHeader() string {
	if a.TokenHeader != "" {
		return a.TokenHeader
	}
	return DefaultTokenHeader
}

// AuthOKPayload is the payload for successful auth response.
type AuthOKPayload struct {
	SessionID string `json:"session_id"`
	Subdomain string `json:"subdomain"`
	PublicURL string `json:"public_url"`
}

// AuthErrorPayload is the payload for failed auth response.
type AuthErrorPayload struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// GoawayPayload is the payload for GOAWAY control messages.
// Sent by the relay when it is shutting down gracefully.
type GoawayPayload struct {
	// Reason describes why the relay is shutting down ("shutdown", "deploy", "maintenance").
	Reason string `json:"reason"`

	// DrainTimeout is the maximum time the relay will wait for in-flight requests
	// to complete before force-closing, in milliseconds.
	DrainTimeoutMs int64 `json:"drain_timeout_ms"`

	// Message is a human-readable message for logging.
	Message string `json:"message,omitempty"`
}

// EncodeControlMessage encodes a control message to JSON bytes.
func EncodeControlMessage(msg *ControlMessage) ([]byte, error) {
	return json.Marshal(msg)
}

// DecodeControlMessage decodes a control message from JSON bytes.
func DecodeControlMessage(data []byte) (*ControlMessage, error) {
	var msg ControlMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		return nil, err
	}
	return &msg, nil
}
