package protocol

// Message represents a generic protocol message.
// Used for broadcasting and pub/sub operations across different protocols.
type Message struct {
	// Type is an optional message type identifier.
	// Interpretation is protocol-specific (e.g., WebSocket opcode, MQTT packet type).
	Type string `json:"type,omitempty"`

	// Data is the message payload.
	Data []byte `json:"data"`

	// Headers holds optional message headers/metadata.
	// Interpretation is protocol-specific.
	Headers map[string]string `json:"headers,omitempty"`
}

// Broadcaster can send messages to multiple recipients.
// Implement this interface for protocols that support broadcasting
// like WebSocket, SSE, etc.
//
// Example implementation:
//
//	func (h *MyHandler) Broadcast(msg protocol.Message) (int, error) {
//	    h.mu.RLock()
//	    defer h.mu.RUnlock()
//	    sent := 0
//	    for _, conn := range h.connections {
//	        if err := conn.Send(msg.Data); err == nil {
//	            sent++
//	        }
//	    }
//	    return sent, nil
//	}
type Broadcaster interface {
	// Broadcast sends a message to all connections.
	// Returns the number of connections the message was sent to.
	Broadcast(msg Message) (sent int, err error)

	// BroadcastTo sends a message to specific connections.
	// Connections that don't exist are silently skipped.
	// Returns the number of connections the message was sent to.
	BroadcastTo(connIDs []string, msg Message) (sent int, err error)
}

// GroupBroadcaster can broadcast to connection groups.
// This extends Broadcaster with group-based broadcasting, useful for
// protocols that support rooms/channels.
//
// Example implementation:
//
//	func (h *MyHandler) BroadcastToGroup(group string, msg protocol.Message) (int, error) {
//	    h.mu.RLock()
//	    connIDs := h.groups[group]
//	    h.mu.RUnlock()
//	    return h.BroadcastTo(connIDs, msg)
//	}
type GroupBroadcaster interface {
	Broadcaster

	// BroadcastToGroup sends a message to all connections in a group.
	// Returns 0 if the group does not exist or is empty.
	BroadcastToGroup(group string, msg Message) (sent int, err error)
}
