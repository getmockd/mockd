package requestlog

// Logger is the minimal interface for logging request entries.
// Protocol handlers (gRPC, MQTT, SOAP, etc.) accept this interface to log requests.
// This allows them to work with any implementation that can record entries,
// whether it's an in-memory store, persistent database, or forwarding to a remote admin.
type Logger interface {
	Log(entry *Entry)
}

// Store defines the interface for request history storage.
// Implementations store request/response entries for user inspection via the Admin API.
// Store embeds Logger, so any Store implementation can be used where Logger is expected.
type Store interface {
	Logger

	// Get retrieves a log entry by ID.
	Get(id string) *Entry

	// List returns all log entries, optionally filtered.
	List(filter *Filter) []*Entry

	// Clear removes all log entries.
	Clear()

	// Count returns the number of log entries.
	Count() int
}

// Filter defines criteria for filtering request logs.
type Filter struct {
	// Protocol filters by protocol (http, grpc, websocket, sse, mqtt, soap, graphql).
	Protocol string

	// Method filters by HTTP method (or gRPC method, etc.).
	Method string

	// Path filters by path prefix (or topic pattern for MQTT).
	Path string

	// MatchedID filters by matched mock ID.
	MatchedID string

	// StatusCode filters by response status code.
	StatusCode int

	// HasError filters by error presence.
	HasError *bool

	// Limit is the maximum number of entries to return.
	Limit int

	// Offset is the number of entries to skip.
	Offset int

	// Protocol-specific filters

	// GRPCService filters gRPC by service name.
	GRPCService string

	// MQTTTopic filters MQTT by topic (supports wildcards).
	MQTTTopic string

	// MQTTClientID filters MQTT by client ID.
	MQTTClientID string

	// SOAPOperation filters SOAP by operation name.
	SOAPOperation string

	// GraphQLOpType filters GraphQL by operation type (query, mutation, subscription).
	GraphQLOpType string

	// WSConnectionID filters WebSocket by connection ID.
	WSConnectionID string

	// SSEConnectionID filters SSE by connection ID.
	SSEConnectionID string
}

// Subscriber is a channel that receives new log entries.
// Used for real-time updates in streaming APIs.
type Subscriber chan *Entry

// SubscribableStore extends Store with subscription support for real-time updates.
type SubscribableStore interface {
	Store

	// Subscribe registers a subscriber to receive new log entries.
	// Returns a channel that will receive entries and an unsubscribe function.
	Subscribe() (Subscriber, func())
}

// ExtendedStore provides additional query methods beyond the basic Store interface.
type ExtendedStore interface {
	Store

	// ClearByMockID removes all log entries matching the given mock ID.
	ClearByMockID(mockID string)

	// CountByMockID returns the number of log entries matching the given mock ID.
	CountByMockID(mockID string) int
}
