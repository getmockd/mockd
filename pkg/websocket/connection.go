package websocket

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	ws "github.com/coder/websocket"

	"github.com/getmockd/mockd/pkg/recording"
)

// Connection represents an active WebSocket connection.
type Connection struct {
	id            string
	endpointPath  string
	conn          *ws.Conn
	subprotocol   string
	connectedAt   time.Time
	lastMessageAt atomic.Value // time.Time
	messagesSent  atomic.Int64
	messagesRecv  atomic.Int64
	groups        map[string]struct{}
	metadata      map[string]interface{}
	scenarioState *ScenarioState

	endpoint      *Endpoint
	manager       *ConnectionManager
	recordingHook recording.WebSocketRecordingHook
	ctx           context.Context
	cancel        context.CancelFunc
	mu            sync.RWMutex
	sendMu        sync.RWMutex // Coordinates Send/Read/Ping with Close to prevent TOCTOU races
	closed        atomic.Bool
}

// NewConnection creates a new Connection wrapping a websocket.Conn.
func NewConnection(wsConn *ws.Conn, endpoint *Endpoint, subprotocol string, r *http.Request) *Connection {
	ctx, cancel := context.WithCancel(context.Background())

	c := &Connection{
		id:           GenerateConnectionID(),
		endpointPath: endpoint.Path(),
		conn:         wsConn,
		subprotocol:  subprotocol,
		connectedAt:  time.Now(),
		groups:       make(map[string]struct{}),
		metadata:     make(map[string]interface{}),
		endpoint:     endpoint,
		ctx:          ctx,
		cancel:       cancel,
	}

	// Capture metadata from request
	if r != nil {
		c.metadata["remoteAddr"] = r.RemoteAddr
		c.metadata["userAgent"] = r.UserAgent()
		if host := r.Host; host != "" {
			c.metadata["host"] = host
		}
	}

	c.lastMessageAt.Store(c.connectedAt)

	return c
}

// ID returns the unique connection ID.
func (c *Connection) ID() string {
	return c.id
}

// EndpointPath returns the endpoint path this connection belongs to.
func (c *Connection) EndpointPath() string {
	return c.endpointPath
}

// Subprotocol returns the negotiated subprotocol.
func (c *Connection) Subprotocol() string {
	return c.subprotocol
}

// ConnectedAt returns the connection establishment time.
func (c *Connection) ConnectedAt() time.Time {
	return c.connectedAt
}

// LastMessageAt returns the last message time.
func (c *Connection) LastMessageAt() time.Time {
	v := c.lastMessageAt.Load()
	if v == nil {
		return c.connectedAt
	}
	t, ok := v.(time.Time)
	if !ok {
		return c.connectedAt
	}
	return t
}

// MessagesSent returns the total messages sent.
func (c *Connection) MessagesSent() int64 {
	return c.messagesSent.Load()
}

// MessagesReceived returns the total messages received.
func (c *Connection) MessagesReceived() int64 {
	return c.messagesRecv.Load()
}

// Groups returns the groups this connection belongs to.
func (c *Connection) Groups() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	groups := make([]string, 0, len(c.groups))
	for g := range c.groups {
		groups = append(groups, g)
	}
	return groups
}

// GetGroups returns a copy of the groups the connection belongs to.
// This is thread-safe and returns a snapshot of the current groups.
func (c *Connection) GetGroups() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	groups := make([]string, 0, len(c.groups))
	for g := range c.groups {
		groups = append(groups, g)
	}
	return groups
}

// Metadata returns the connection metadata.
func (c *Connection) Metadata() map[string]interface{} {
	c.mu.RLock()
	defer c.mu.RUnlock()
	// Return copy
	meta := make(map[string]interface{}, len(c.metadata))
	for k, v := range c.metadata {
		meta[k] = v
	}
	return meta
}

// SetMetadata sets a metadata value.
func (c *Connection) SetMetadata(key string, value interface{}) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.metadata[key] = value
}

// Context returns the connection context.
func (c *Connection) Context() context.Context {
	return c.ctx
}

// Endpoint returns the endpoint this connection belongs to.
func (c *Connection) Endpoint() *Endpoint {
	return c.endpoint
}

// SetManager sets the connection manager.
func (c *Connection) SetManager(m *ConnectionManager) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.manager = m
}

// SetRecordingHook sets the recording hook for this connection.
// If set, messages will be recorded via the hook.
func (c *Connection) SetRecordingHook(hook recording.WebSocketRecordingHook) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.recordingHook = hook
}

// IsClosed returns whether the connection is closed.
func (c *Connection) IsClosed() bool {
	return c.closed.Load()
}

// Send sends a message to the client.
func (c *Connection) Send(msgType MessageType, data []byte) error {
	c.sendMu.RLock()
	defer c.sendMu.RUnlock()

	if c.closed.Load() {
		return ErrConnectionClosed
	}

	var wsType ws.MessageType
	switch msgType {
	case MessageText:
		wsType = ws.MessageText
	case MessageBinary:
		wsType = ws.MessageBinary
	default:
		wsType = ws.MessageText
	}

	err := c.conn.Write(c.ctx, wsType, data)
	if err != nil {
		return err
	}

	c.messagesSent.Add(1)
	c.lastMessageAt.Store(time.Now())

	// Record the sent message if recording hook is set
	c.mu.RLock()
	hook := c.recordingHook
	manager := c.manager
	c.mu.RUnlock()
	if hook != nil {
		frame := recording.NewWebSocketFrame(
			c.messagesSent.Load(),
			c.connectedAt,
			recording.DirectionServerToClient,
			convertMessageType(msgType),
			data,
		)
		_ = hook.OnFrame(frame)
	}

	// Log outbound message via manager
	if manager != nil {
		remoteAddr := ""
		if addr, ok := c.Metadata()["remoteAddr"].(string); ok {
			remoteAddr = addr
		}
		manager.LogMessageSent(c, msgType, data, remoteAddr)
	}

	return nil
}

// SendText sends a text message.
func (c *Connection) SendText(text string) error {
	return c.Send(MessageText, []byte(text))
}

// SendJSON sends a JSON message.
func (c *Connection) SendJSON(v interface{}) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return c.Send(MessageText, data)
}

// Read reads the next message from the connection.
// Returns the message type, data, and any error.
func (c *Connection) Read() (MessageType, []byte, error) {
	// Note: We don't take sendMu here because Read() blocks on I/O.
	// Close() will cancel the context, which will unblock Read().
	if c.closed.Load() {
		return 0, nil, ErrConnectionClosed
	}

	wsType, data, err := c.conn.Read(c.ctx)
	if err != nil {
		return 0, nil, err
	}

	c.messagesRecv.Add(1)
	c.lastMessageAt.Store(time.Now())

	var msgType MessageType
	switch wsType {
	case ws.MessageText:
		msgType = MessageText
	case ws.MessageBinary:
		msgType = MessageBinary
	default:
		msgType = MessageText
	}

	// Record the received message if recording hook is set
	c.mu.RLock()
	hook := c.recordingHook
	c.mu.RUnlock()
	if hook != nil {
		frame := recording.NewWebSocketFrame(
			c.messagesRecv.Load(),
			c.connectedAt,
			recording.DirectionClientToServer,
			convertMessageType(msgType),
			data,
		)
		_ = hook.OnFrame(frame)
	}

	return msgType, data, nil
}

// Close closes the connection with the given close code and reason.
func (c *Connection) Close(code CloseCode, reason string) error {
	c.sendMu.Lock()
	defer c.sendMu.Unlock()

	if c.closed.Swap(true) {
		return ErrConnectionClosed
	}

	c.cancel()

	// Notify recording hook of connection close and complete the recording
	c.mu.RLock()
	hook := c.recordingHook
	c.mu.RUnlock()
	if hook != nil {
		hook.OnClose(int(code), reason)
		// Complete the recording after close
		_ = hook.OnComplete()
	}

	// Close the websocket connection
	return c.conn.Close(ws.StatusCode(code), reason)
}

// CloseNormal closes the connection with normal closure.
func (c *Connection) CloseNormal() error {
	return c.Close(CloseNormalClosure, "")
}

// MaxGroupsPerConnection limits the number of groups a connection can join
// to prevent unbounded memory growth.
const MaxGroupsPerConnection = 100

// JoinGroup adds the connection to a group.
// This method is safe to call concurrently with ConnectionManager.JoinGroup.
// Returns ErrTooManyGroups if the connection is already in MaxGroupsPerConnection groups.
func (c *Connection) JoinGroup(group string) error {
	// Check and update connection state first
	c.mu.Lock()
	if _, exists := c.groups[group]; exists {
		c.mu.Unlock()
		return ErrAlreadyInGroup
	}
	if len(c.groups) >= MaxGroupsPerConnection {
		c.mu.Unlock()
		return ErrTooManyGroups
	}
	c.groups[group] = struct{}{}
	manager := c.manager
	connID := c.id
	c.mu.Unlock()

	// Notify manager outside of connection lock to avoid deadlock.
	// Lock ordering: always release connection lock before acquiring manager lock.
	if manager != nil {
		manager.addToGroup(connID, group)
	}

	return nil
}

// LeaveGroup removes the connection from a group.
// This method is safe to call concurrently with ConnectionManager.LeaveGroup.
func (c *Connection) LeaveGroup(group string) error {
	// Check and update connection state first
	c.mu.Lock()
	if _, exists := c.groups[group]; !exists {
		c.mu.Unlock()
		return ErrNotInGroup
	}
	delete(c.groups, group)
	manager := c.manager
	connID := c.id
	c.mu.Unlock()

	// Notify manager outside of connection lock to avoid deadlock.
	// Lock ordering: always release connection lock before acquiring manager lock.
	if manager != nil {
		manager.removeFromGroup(connID, group)
	}

	return nil
}

// InGroup returns whether the connection is in a group.
func (c *Connection) InGroup(group string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	_, exists := c.groups[group]
	return exists
}

// SetScenarioState sets the scenario state for this connection.
func (c *Connection) SetScenarioState(state *ScenarioState) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.scenarioState = state
}

// ScenarioState returns the current scenario state.
func (c *Connection) ScenarioState() *ScenarioState {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.scenarioState
}

// Info returns public information about this connection.
func (c *Connection) Info() *ConnectionInfo {
	c.mu.RLock()
	defer c.mu.RUnlock()

	info := &ConnectionInfo{
		ID:               c.id,
		EndpointPath:     c.endpointPath,
		Subprotocol:      c.subprotocol,
		ConnectedAt:      c.connectedAt,
		LastMessageAt:    c.LastMessageAt(),
		MessagesSent:     c.messagesSent.Load(),
		MessagesReceived: c.messagesRecv.Load(),
		Groups:           make([]string, 0, len(c.groups)),
		Metadata:         make(map[string]interface{}, len(c.metadata)),
	}

	for g := range c.groups {
		info.Groups = append(info.Groups, g)
	}

	for k, v := range c.metadata {
		info.Metadata[k] = v
	}

	if c.scenarioState != nil {
		info.ScenarioState = c.scenarioState.Info()
	}

	return info
}

// Ping sends a ping frame to the client.
func (c *Connection) Ping(ctx context.Context) error {
	c.sendMu.RLock()
	defer c.sendMu.RUnlock()

	if c.closed.Load() {
		return ErrConnectionClosed
	}
	return c.conn.Ping(ctx)
}
