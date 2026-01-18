package requestlog

// GRPCMeta contains gRPC-specific request metadata.
type GRPCMeta struct {
	// Service is the gRPC service name (e.g., "mypackage.MyService").
	Service string `json:"service"`

	// MethodName is the gRPC method name (e.g., "GetUser").
	MethodName string `json:"methodName"`

	// StreamType is the gRPC stream type (unary, server_stream, client_stream, bidi).
	StreamType string `json:"streamType,omitempty"`

	// StatusCode is the gRPC status code name (e.g., "OK", "NOT_FOUND").
	StatusCode string `json:"statusCode,omitempty"`

	// StatusMessage is the gRPC status message.
	StatusMessage string `json:"statusMessage,omitempty"`
}

// WebSocketMeta contains WebSocket-specific request metadata.
type WebSocketMeta struct {
	// ConnectionID is the WebSocket connection identifier.
	ConnectionID string `json:"connectionId"`

	// MessageType is the WebSocket message type (text, binary, ping, pong, close).
	MessageType string `json:"messageType"`

	// Direction is the message direction (inbound, outbound).
	Direction string `json:"direction"`

	// Subprotocol is the negotiated WebSocket subprotocol.
	Subprotocol string `json:"subprotocol,omitempty"`

	// CloseCode is the WebSocket close code (if closing).
	CloseCode int `json:"closeCode,omitempty"`
}

// SSEMeta contains SSE-specific request metadata.
type SSEMeta struct {
	// ConnectionID is the SSE connection identifier.
	ConnectionID string `json:"connectionId"`

	// EventType is the SSE event type (e.g., "message", "update").
	EventType string `json:"eventType,omitempty"`

	// EventID is the SSE event ID.
	EventID string `json:"eventId,omitempty"`

	// IsConnection indicates if this is a connection event (open/close) vs data event.
	IsConnection bool `json:"isConnection,omitempty"`

	// EventCount is the number of events sent (for connection close).
	EventCount int `json:"eventCount,omitempty"`
}

// MQTTMeta contains MQTT-specific request metadata.
type MQTTMeta struct {
	// ClientID is the MQTT client identifier.
	ClientID string `json:"clientId"`

	// Topic is the MQTT topic.
	Topic string `json:"topic"`

	// QoS is the MQTT QoS level (0, 1, 2).
	QoS int `json:"qos"`

	// Retain indicates if the message is retained.
	Retain bool `json:"retain,omitempty"`

	// Direction is the message direction (publish, subscribe, received).
	Direction string `json:"direction"`

	// MessageID is the MQTT message ID.
	MessageID uint16 `json:"messageId,omitempty"`
}

// SOAPMeta contains SOAP-specific request metadata.
type SOAPMeta struct {
	// Operation is the SOAP operation name.
	Operation string `json:"operation"`

	// SOAPAction is the SOAPAction header value.
	SOAPAction string `json:"soapAction,omitempty"`

	// SOAPVersion is the SOAP version (1.1 or 1.2).
	SOAPVersion string `json:"soapVersion"`

	// IsFault indicates if the response is a SOAP fault.
	IsFault bool `json:"isFault,omitempty"`

	// FaultCode is the SOAP fault code (if fault).
	FaultCode string `json:"faultCode,omitempty"`
}

// GraphQLMeta contains GraphQL-specific request metadata.
type GraphQLMeta struct {
	// OperationType is the GraphQL operation type (query, mutation, subscription).
	OperationType string `json:"operationType"`

	// OperationName is the GraphQL operation name (if named).
	OperationName string `json:"operationName,omitempty"`

	// Variables contains the GraphQL variables (JSON string).
	Variables string `json:"variables,omitempty"`

	// HasErrors indicates if the response contains errors.
	HasErrors bool `json:"hasErrors,omitempty"`

	// ErrorCount is the number of GraphQL errors in response.
	ErrorCount int `json:"errorCount,omitempty"`
}
