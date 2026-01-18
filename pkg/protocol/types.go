package protocol

// Protocol identifies the protocol type.
type Protocol string

// Protocol constants for all supported and planned protocols.
const (
	// Currently supported protocols
	ProtocolGRPC      Protocol = "grpc"
	ProtocolMQTT      Protocol = "mqtt"
	ProtocolSSE       Protocol = "sse"
	ProtocolWebSocket Protocol = "websocket"
	ProtocolGraphQL   Protocol = "graphql"
	ProtocolSOAP      Protocol = "soap"

	// Future protocols
	ProtocolSocketIO Protocol = "socketio"
	ProtocolAMQP     Protocol = "amqp"
	ProtocolKafka    Protocol = "kafka"
	ProtocolNATS     Protocol = "nats"
	ProtocolRedis    Protocol = "redis"
	ProtocolJSONRPC  Protocol = "jsonrpc"
	ProtocolThrift   Protocol = "thrift"
)

// String returns the string representation of the protocol.
func (p Protocol) String() string {
	return string(p)
}

// Capability identifies optional features a handler supports.
// Use Metadata.HasCapability() to check if a handler supports a capability.
type Capability string

// Capability constants for all supported features.
const (
	// Connection capabilities
	CapabilityConnections      Capability = "connections"       // Manages persistent connections
	CapabilityConnectionGroups Capability = "connection_groups" // Supports connection grouping/rooms

	// Communication capabilities
	CapabilityBroadcast     Capability = "broadcast"     // Can broadcast to multiple recipients
	CapabilityPubSub        Capability = "pubsub"        // Supports publish/subscribe
	CapabilityStreaming     Capability = "streaming"     // Supports streaming data
	CapabilityBidirectional Capability = "bidirectional" // Supports bidirectional communication
	CapabilitySubscriptions Capability = "subscriptions" // Supports subscription management

	// Session capabilities
	CapabilitySessions        Capability = "sessions"        // Maintains session state
	CapabilitySessionResume   Capability = "session_resume"  // Can resume interrupted sessions
	CapabilityAcknowledgments Capability = "acknowledgments" // Supports message acknowledgments

	// Operational capabilities
	CapabilityRecording  Capability = "recording"  // Supports request/message recording
	CapabilityReplay     Capability = "replay"     // Can replay recorded sessions
	CapabilityMetrics    Capability = "metrics"    // Exposes operational metrics
	CapabilityMocking    Capability = "mocking"    // Supports mock responses
	CapabilityMatching   Capability = "matching"   // Supports request/message matching
	CapabilityTemplating Capability = "templating" // Supports response templating

	// Schema capabilities
	CapabilitySchemaValidation Capability = "schema_validation" // Validates against schema
	CapabilitySchemaIntrospect Capability = "schema_introspect" // Supports schema introspection
)

// String returns the string representation of the capability.
func (c Capability) String() string {
	return string(c)
}

// TransportType indicates the underlying transport mechanism.
type TransportType string

// TransportType constants for all supported transports.
const (
	TransportHTTP1     TransportType = "http1"
	TransportHTTP2     TransportType = "http2"
	TransportTCP       TransportType = "tcp"
	TransportWebSocket TransportType = "websocket"
)

// String returns the string representation of the transport type.
func (t TransportType) String() string {
	return string(t)
}

// ConnectionModel describes the connection lifecycle pattern.
type ConnectionModel string

// ConnectionModel constants for all supported models.
const (
	ConnectionModelStateless  ConnectionModel = "stateless"  // No state between requests
	ConnectionModelPersistent ConnectionModel = "persistent" // Long-lived connections
	ConnectionModelUpgrade    ConnectionModel = "upgrade"    // HTTP upgrade (WebSocket)
	ConnectionModelStandalone ConnectionModel = "standalone" // Independent TCP server
)

// String returns the string representation of the connection model.
func (c ConnectionModel) String() string {
	return string(c)
}

// CommunicationPattern describes the message flow pattern.
type CommunicationPattern string

// CommunicationPattern constants for all supported patterns.
const (
	PatternRequestResponse CommunicationPattern = "request_response"
	PatternServerPush      CommunicationPattern = "server_push"
	PatternBidirectional   CommunicationPattern = "bidirectional"
	PatternPubSub          CommunicationPattern = "pubsub"
	PatternStreaming       CommunicationPattern = "streaming"
)

// String returns the string representation of the communication pattern.
func (p CommunicationPattern) String() string {
	return string(p)
}
