package protocol

import "time"

// ConnectionManager handlers maintain persistent connections.
// Implement this interface for protocols with long-lived connections
// like WebSocket, MQTT, gRPC streams, etc.
//
// Example implementation:
//
//	func (h *MyHandler) ConnectionCount() int {
//	    h.mu.RLock()
//	    defer h.mu.RUnlock()
//	    return len(h.connections)
//	}
//
//	func (h *MyHandler) ListConnections() []protocol.ConnectionInfo {
//	    h.mu.RLock()
//	    defer h.mu.RUnlock()
//	    conns := make([]protocol.ConnectionInfo, 0, len(h.connections))
//	    for _, c := range h.connections {
//	        conns = append(conns, c.Info())
//	    }
//	    return conns
//	}
type ConnectionManager interface {
	// ConnectionCount returns the number of active connections.
	ConnectionCount() int

	// ListConnections returns information about all active connections.
	ListConnections() []ConnectionInfo

	// GetConnection returns information about a specific connection.
	// Returns ErrConnectionNotFound if the connection does not exist.
	GetConnection(id string) (*ConnectionInfo, error)

	// CloseConnection closes a specific connection with the given reason.
	// Returns ErrConnectionNotFound if the connection does not exist.
	CloseConnection(id string, reason string) error

	// CloseAllConnections closes all connections with the given reason.
	// Returns the number of connections that were closed.
	CloseAllConnections(reason string) int
}

// ConnectionInfo provides information about a connection.
// This struct is returned by ConnectionManager methods and used
// by the Admin API to display connection details.
type ConnectionInfo struct {
	// ID is the unique identifier for this connection.
	ID string `json:"id"`

	// RemoteAddr is the client's network address.
	RemoteAddr string `json:"remoteAddr"`

	// ConnectedAt is when the connection was established.
	ConnectedAt time.Time `json:"connectedAt"`

	// LastActivity is when the last message was sent or received.
	LastActivity time.Time `json:"lastActivity"`

	// BytesSent is the total bytes sent to this client.
	BytesSent int64 `json:"bytesSent"`

	// BytesReceived is the total bytes received from this client.
	BytesReceived int64 `json:"bytesReceived"`

	// Metadata holds protocol-specific connection information.
	// Examples: MQTT client ID, WebSocket subprotocol, etc.
	Metadata map[string]any `json:"metadata,omitempty"`
}

// GroupableConnections handlers support grouping connections (rooms/channels).
// This extends ConnectionManager with group operations, useful for
// protocols that support rooms (Socket.IO) or channels (WebSocket).
//
// Example implementation:
//
//	func (h *MyHandler) JoinGroup(connID, group string) error {
//	    h.mu.Lock()
//	    defer h.mu.Unlock()
//	    conn, ok := h.connections[connID]
//	    if !ok {
//	        return protocol.ErrConnectionNotFound
//	    }
//	    if _, ok := h.groups[group]; !ok {
//	        h.groups[group] = make(map[string]bool)
//	    }
//	    h.groups[group][connID] = true
//	    conn.groups[group] = true
//	    return nil
//	}
type GroupableConnections interface {
	ConnectionManager

	// JoinGroup adds a connection to a group.
	// Returns ErrConnectionNotFound if the connection does not exist.
	JoinGroup(connID, group string) error

	// LeaveGroup removes a connection from a group.
	// Returns ErrConnectionNotFound if the connection does not exist.
	// Returns ErrGroupNotFound if the group does not exist.
	LeaveGroup(connID, group string) error

	// ListGroups returns all group names.
	ListGroups() []string

	// ListGroupConnections returns connection IDs in a group.
	// Returns empty slice if the group does not exist.
	ListGroupConnections(group string) []string
}
