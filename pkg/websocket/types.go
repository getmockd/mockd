package websocket

import (
	"encoding/base64"
	"encoding/json"
	"time"
)

// MessageType represents the type of WebSocket message.
type MessageType int

const (
	// MessageText indicates a UTF-8 encoded text message.
	MessageText MessageType = 1
	// MessageBinary indicates a binary message.
	MessageBinary MessageType = 2
)

// String returns the string representation of the message type.
func (t MessageType) String() string {
	switch t {
	case MessageText:
		return "text"
	case MessageBinary:
		return "binary"
	default:
		return "unknown"
	}
}

// CloseCode represents a WebSocket close status code per RFC 6455.
type CloseCode int

const (
	// CloseNormalClosure indicates a normal closure (1000).
	CloseNormalClosure CloseCode = 1000
	// CloseGoingAway indicates the endpoint is going away (1001).
	CloseGoingAway CloseCode = 1001
	// CloseProtocolError indicates a protocol error (1002).
	CloseProtocolError CloseCode = 1002
	// CloseUnsupportedData indicates unsupported data type (1003).
	CloseUnsupportedData CloseCode = 1003
	// CloseNoStatusReceived indicates no status code was received (1005).
	CloseNoStatusReceived CloseCode = 1005
	// CloseAbnormalClosure indicates abnormal closure (1006).
	CloseAbnormalClosure CloseCode = 1006
	// CloseInvalidPayload indicates invalid UTF-8 in text message (1007).
	CloseInvalidPayload CloseCode = 1007
	// ClosePolicyViolation indicates a policy violation (1008).
	ClosePolicyViolation CloseCode = 1008
	// CloseMessageTooBig indicates message is too large (1009).
	CloseMessageTooBig CloseCode = 1009
	// CloseMandatoryExtension indicates missing mandatory extension (1010).
	CloseMandatoryExtension CloseCode = 1010
	// CloseInternalError indicates internal server error (1011).
	CloseInternalError CloseCode = 1011
	// CloseServiceRestart indicates service restart (1012).
	CloseServiceRestart CloseCode = 1012
	// CloseTryAgainLater indicates try again later (1013).
	CloseTryAgainLater CloseCode = 1013
	// CloseTLSHandshake indicates TLS handshake failure (1015).
	CloseTLSHandshake CloseCode = 1015
)

// String returns a human-readable description of the close code.
func (c CloseCode) String() string {
	switch c {
	case CloseNormalClosure:
		return "normal closure"
	case CloseGoingAway:
		return "going away"
	case CloseProtocolError:
		return "protocol error"
	case CloseUnsupportedData:
		return "unsupported data"
	case CloseNoStatusReceived:
		return "no status received"
	case CloseAbnormalClosure:
		return "abnormal closure"
	case CloseInvalidPayload:
		return "invalid payload"
	case ClosePolicyViolation:
		return "policy violation"
	case CloseMessageTooBig:
		return "message too big"
	case CloseMandatoryExtension:
		return "mandatory extension"
	case CloseInternalError:
		return "internal error"
	case CloseServiceRestart:
		return "service restart"
	case CloseTryAgainLater:
		return "try again later"
	case CloseTLSHandshake:
		return "TLS handshake"
	default:
		return "unknown"
	}
}

// MessageResponse defines a response to send for a matched message.
type MessageResponse struct {
	// Type is the message type: "text", "binary", or "json".
	Type string `json:"type"`
	// Value is the response content.
	// For "text": string
	// For "binary": base64-encoded string
	// For "json": object that will be marshaled
	Value interface{} `json:"value"`
	// Delay is the wait time before sending (optional).
	Delay Duration `json:"delay,omitempty"`
}

// GetData returns the response data as bytes.
func (r *MessageResponse) GetData() ([]byte, MessageType, error) {
	switch r.Type {
	case "text":
		s, ok := r.Value.(string)
		if !ok {
			return nil, MessageText, ErrInvalidResponseValue
		}
		return []byte(s), MessageText, nil
	case "json":
		data, err := json.Marshal(r.Value)
		if err != nil {
			return nil, MessageText, err
		}
		return data, MessageText, nil
	case "binary":
		s, ok := r.Value.(string)
		if !ok {
			return nil, MessageBinary, ErrInvalidResponseValue
		}
		// Value should be base64 encoded for binary - decode it
		decoded, err := base64.StdEncoding.DecodeString(s)
		if err != nil {
			// Fall back to raw bytes for backward compatibility
			// (allows non-base64 strings if they were stored that way)
			return []byte(s), MessageBinary, nil
		}
		return decoded, MessageBinary, nil
	default:
		return nil, MessageText, ErrUnknownResponseType
	}
}

// Duration is a time.Duration that marshals/unmarshals as a string.
type Duration time.Duration

// MarshalJSON marshals the duration as a string.
func (d Duration) MarshalJSON() ([]byte, error) {
	return json.Marshal(time.Duration(d).String())
}

// UnmarshalJSON unmarshals a duration string.
func (d *Duration) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		// Try as integer (milliseconds)
		var ms int64
		if err := json.Unmarshal(data, &ms); err != nil {
			return err
		}
		*d = Duration(time.Duration(ms) * time.Millisecond)
		return nil
	}
	if s == "" {
		*d = 0
		return nil
	}
	parsed, err := time.ParseDuration(s)
	if err != nil {
		return err
	}
	*d = Duration(parsed)
	return nil
}

// Duration returns the underlying time.Duration.
func (d Duration) Duration() time.Duration {
	return time.Duration(d)
}

// ConnectionInfo represents public information about a connection.
type ConnectionInfo struct {
	ID               string                 `json:"id"`
	EndpointPath     string                 `json:"endpointPath"`
	Subprotocol      string                 `json:"subprotocol,omitempty"`
	ConnectedAt      time.Time              `json:"connectedAt"`
	LastMessageAt    time.Time              `json:"lastMessageAt,omitempty"`
	MessagesSent     int64                  `json:"messagesSent"`
	MessagesReceived int64                  `json:"messagesReceived"`
	Groups           []string               `json:"groups,omitempty"`
	Metadata         map[string]interface{} `json:"metadata,omitempty"`
	ScenarioState    *ScenarioStateInfo     `json:"scenarioState,omitempty"`
}

// ScenarioStateInfo represents public information about scenario state.
type ScenarioStateInfo struct {
	Name        string    `json:"name,omitempty"`
	CurrentStep int       `json:"currentStep"`
	TotalSteps  int       `json:"totalSteps"`
	Completed   bool      `json:"completed"`
	StartedAt   time.Time `json:"startedAt,omitempty"`
}

// EndpointInfo represents public information about an endpoint.
type EndpointInfo struct {
	Path               string   `json:"path"`
	Subprotocols       []string `json:"subprotocols,omitempty"`
	ConnectionCount    int      `json:"connectionCount"`
	MaxConnections     int      `json:"maxConnections"`
	HasScenario        bool     `json:"hasScenario"`
	ScenarioName       string   `json:"scenarioName,omitempty"`
	RequireSubprotocol bool     `json:"requireSubprotocol,omitempty"`
	MatcherCount       int      `json:"matcherCount,omitempty"`
	HeartbeatEnabled   bool     `json:"heartbeatEnabled,omitempty"`
	HeartbeatInterval  string   `json:"heartbeatInterval,omitempty"`
	MaxMessageSize     int64    `json:"maxMessageSize,omitempty"`
	IdleTimeout        string   `json:"idleTimeout,omitempty"`
	Enabled            bool     `json:"enabled"`
	// Organization fields
	ParentID    string  `json:"parentId,omitempty"`
	MetaSortKey float64 `json:"metaSortKey,omitempty"`
}

// Stats represents aggregate WebSocket statistics.
type Stats struct {
	TotalConnections      int            `json:"totalConnections"`
	TotalEndpoints        int            `json:"totalEndpoints"`
	TotalMessagesSent     int64          `json:"totalMessagesSent"`
	TotalMessagesReceived int64          `json:"totalMessagesReceived"`
	ConnectionsByEndpoint map[string]int `json:"connectionsByEndpoint"`
	Uptime                string         `json:"uptime"`
}

// BroadcastResult represents the result of a broadcast operation.
type BroadcastResult struct {
	Broadcast  bool   `json:"broadcast"`
	Recipients int    `json:"recipients"`
	Endpoint   string `json:"endpoint,omitempty"`
	Group      string `json:"group,omitempty"`
}
