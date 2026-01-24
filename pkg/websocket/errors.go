package websocket

import "errors"

// Common errors for the websocket package.
var (
	// ErrConnectionClosed indicates the connection is closed.
	ErrConnectionClosed = errors.New("connection closed")
	// ErrConnectionNotFound indicates the connection was not found.
	ErrConnectionNotFound = errors.New("connection not found")
	// ErrEndpointNotFound indicates the endpoint was not found.
	ErrEndpointNotFound = errors.New("endpoint not found")
	// ErrMaxConnectionsReached indicates the maximum connections limit was reached.
	ErrMaxConnectionsReached = errors.New("maximum connections reached")
	// ErrEndpointDisabled indicates the endpoint is disabled.
	ErrEndpointDisabled = errors.New("endpoint is disabled")
	// ErrSubprotocolRequired indicates a subprotocol is required but not provided.
	ErrSubprotocolRequired = errors.New("subprotocol required")
	// ErrSubprotocolMismatch indicates the requested subprotocol is not supported.
	ErrSubprotocolMismatch = errors.New("subprotocol not supported")
	// ErrMessageTooLarge indicates the message exceeds the size limit.
	ErrMessageTooLarge = errors.New("message too large")
	// ErrInvalidResponseValue indicates an invalid response value type.
	ErrInvalidResponseValue = errors.New("invalid response value")
	// ErrUnknownResponseType indicates an unknown response type.
	ErrUnknownResponseType = errors.New("unknown response type")
	// ErrInvalidMatcherType indicates an invalid matcher type.
	ErrInvalidMatcherType = errors.New("invalid matcher type")
	// ErrInvalidScenarioStep indicates an invalid scenario step type.
	ErrInvalidScenarioStep = errors.New("invalid scenario step type")
	// ErrScenarioTimeout indicates the scenario timed out waiting for a message.
	ErrScenarioTimeout = errors.New("scenario timeout")
	// ErrGroupNotFound indicates the group was not found.
	ErrGroupNotFound = errors.New("group not found")
	// ErrAlreadyInGroup indicates the connection is already in the group.
	ErrAlreadyInGroup = errors.New("already in group")
	// ErrNotInGroup indicates the connection is not in the group.
	ErrNotInGroup = errors.New("not in group")
	// ErrTooManyGroups indicates the connection has joined too many groups.
	ErrTooManyGroups = errors.New("too many groups")
)
