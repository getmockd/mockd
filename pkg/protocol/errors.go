package protocol

// Error is a simple error type for protocol errors.
// It allows defining sentinel errors as constants.
type Error string

// Error implements the error interface.
func (e Error) Error() string { return string(e) }

// Sentinel errors for common protocol operations.
// These errors should be used for consistent error handling across handlers.
var (
	// ErrNilHandler is returned when attempting to register a nil handler.
	ErrNilHandler = Error("handler cannot be nil")

	// ErrEmptyHandlerID is returned when a handler has an empty ID.
	ErrEmptyHandlerID = Error("handler ID cannot be empty")

	// ErrMissingID is returned when a handler has no ID in its metadata.
	// Handler IDs are required for registry registration.
	ErrMissingID = Error("handler ID is required")

	// ErrHandlerExists is returned when registering a handler with an ID
	// that is already registered in the registry.
	ErrHandlerExists = Error("handler with this ID already exists")

	// ErrHandlerNotFound is returned when looking up a handler by ID
	// that is not registered in the registry.
	ErrHandlerNotFound = Error("handler not found")

	// ErrAlreadyRunning is returned when attempting to start a handler
	// that is already running.
	ErrAlreadyRunning = Error("handler is already running")

	// ErrNotRunning is returned when attempting to stop a handler
	// that is not running.
	ErrNotRunning = Error("handler is not running")

	// ErrConnectionNotFound is returned when looking up a connection by ID
	// that does not exist.
	ErrConnectionNotFound = Error("connection not found")

	// ErrGroupNotFound is returned when looking up a connection group
	// that does not exist.
	ErrGroupNotFound = Error("group not found")

	// ErrTopicNotFound is returned when looking up a topic
	// that does not exist.
	ErrTopicNotFound = Error("topic not found")

	// ErrSubscriptionNotFound is returned when looking up a subscription
	// that does not exist.
	ErrSubscriptionNotFound = Error("subscription not found")

	// ErrQueueNotFound is returned when looking up a queue
	// that does not exist.
	ErrQueueNotFound = Error("queue not found")

	// ErrQueueExists is returned when creating a queue with a name
	// that already exists.
	ErrQueueExists = Error("queue already exists")

	// ErrServiceNotFound is returned when looking up an RPC service
	// that does not exist.
	ErrServiceNotFound = Error("service not found")

	// ErrMethodNotFound is returned when looking up an RPC method
	// that does not exist.
	ErrMethodNotFound = Error("method not found")

	// ErrRecordingNotEnabled is returned when attempting recording operations
	// on a handler that does not have recording enabled.
	ErrRecordingNotEnabled = Error("recording is not enabled")

	// ErrReplayNotFound is returned when looking up a replay session
	// that does not exist.
	ErrReplayNotFound = Error("replay session not found")

	// ErrInvalidMessage is returned when a message cannot be processed
	// due to invalid format or content.
	ErrInvalidMessage = Error("invalid message")

	// ErrTimeout is returned when an operation times out.
	ErrTimeout = Error("operation timed out")

	// ErrShutdown is returned when an operation is attempted on a handler
	// that is shutting down.
	ErrShutdown = Error("handler is shutting down")
)
