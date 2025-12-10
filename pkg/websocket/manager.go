package websocket

import (
	"sync"
	"sync/atomic"
	"time"
)

// ConnectionManager manages all WebSocket connections across endpoints.
type ConnectionManager struct {
	connections map[string]*Connection      // ID -> Connection
	byEndpoint  map[string]map[string]bool  // endpoint path -> set of connection IDs
	byGroup     map[string]map[string]bool  // group name -> set of connection IDs
	endpoints   map[string]*Endpoint        // path -> Endpoint

	totalMsgSent atomic.Int64
	totalMsgRecv atomic.Int64
	startTime    time.Time

	mu sync.RWMutex
}

// NewConnectionManager creates a new ConnectionManager.
func NewConnectionManager() *ConnectionManager {
	return &ConnectionManager{
		connections: make(map[string]*Connection),
		byEndpoint:  make(map[string]map[string]bool),
		byGroup:     make(map[string]map[string]bool),
		endpoints:   make(map[string]*Endpoint),
		startTime:   time.Now(),
	}
}

// RegisterEndpoint registers an endpoint with the manager.
func (m *ConnectionManager) RegisterEndpoint(e *Endpoint) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.endpoints[e.Path()] = e
	e.SetManager(m)

	if m.byEndpoint[e.Path()] == nil {
		m.byEndpoint[e.Path()] = make(map[string]bool)
	}
}

// UnregisterEndpoint removes an endpoint from the manager.
func (m *ConnectionManager) UnregisterEndpoint(path string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.endpoints, path)
	// Note: connections remain until they close
}

// GetEndpoint returns an endpoint by path.
func (m *ConnectionManager) GetEndpoint(path string) *Endpoint {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.endpoints[path]
}

// Endpoints returns all registered endpoints.
func (m *ConnectionManager) Endpoints() []*Endpoint {
	m.mu.RLock()
	defer m.mu.RUnlock()

	eps := make([]*Endpoint, 0, len(m.endpoints))
	for _, e := range m.endpoints {
		eps = append(eps, e)
	}
	return eps
}

// Add registers a new connection.
func (m *ConnectionManager) Add(conn *Connection) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.connections[conn.ID()] = conn
	conn.SetManager(m)

	// Add to endpoint mapping
	if m.byEndpoint[conn.EndpointPath()] == nil {
		m.byEndpoint[conn.EndpointPath()] = make(map[string]bool)
	}
	m.byEndpoint[conn.EndpointPath()][conn.ID()] = true
}

// Remove unregisters a connection and cleans up groups.
func (m *ConnectionManager) Remove(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	conn, exists := m.connections[id]
	if !exists {
		return
	}

	// Track stats before removal
	m.totalMsgSent.Add(conn.MessagesSent())
	m.totalMsgRecv.Add(conn.MessagesReceived())

	// Remove from endpoint mapping
	if eps, ok := m.byEndpoint[conn.EndpointPath()]; ok {
		delete(eps, id)
	}

	// Remove from all groups
	for group := range conn.groups {
		if grp, ok := m.byGroup[group]; ok {
			delete(grp, id)
			if len(grp) == 0 {
				delete(m.byGroup, group)
			}
		}
	}

	delete(m.connections, id)
}

// Get returns a connection by ID.
func (m *ConnectionManager) Get(id string) *Connection {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.connections[id]
}

// ListAll returns all connection IDs.
func (m *ConnectionManager) ListAll() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	ids := make([]string, 0, len(m.connections))
	for id := range m.connections {
		ids = append(ids, id)
	}
	return ids
}

// ListByEndpoint returns connection IDs for an endpoint.
func (m *ConnectionManager) ListByEndpoint(path string) []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	ids := make([]string, 0)
	if eps, ok := m.byEndpoint[path]; ok {
		for id := range eps {
			ids = append(ids, id)
		}
	}
	return ids
}

// ListByGroup returns connection IDs in a group.
func (m *ConnectionManager) ListByGroup(group string) []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	ids := make([]string, 0)
	if grp, ok := m.byGroup[group]; ok {
		for id := range grp {
			ids = append(ids, id)
		}
	}
	return ids
}

// Count returns the total connection count.
func (m *ConnectionManager) Count() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.connections)
}

// CountByEndpoint returns the connection count for an endpoint.
func (m *ConnectionManager) CountByEndpoint(path string) int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if eps, ok := m.byEndpoint[path]; ok {
		return len(eps)
	}
	return 0
}

// Broadcast sends a message to all connections on an endpoint.
func (m *ConnectionManager) Broadcast(path string, msgType MessageType, data []byte) int {
	m.mu.RLock()
	var ids []string
	if eps, ok := m.byEndpoint[path]; ok {
		ids = make([]string, 0, len(eps))
		for id := range eps {
			ids = append(ids, id)
		}
	}
	m.mu.RUnlock()

	sent := 0
	for _, id := range ids {
		m.mu.RLock()
		conn := m.connections[id]
		m.mu.RUnlock()

		if conn != nil && !conn.IsClosed() {
			if err := conn.Send(msgType, data); err == nil {
				sent++
			}
		}
	}
	return sent
}

// BroadcastToGroup sends a message to all connections in a group.
func (m *ConnectionManager) BroadcastToGroup(group string, msgType MessageType, data []byte) int {
	m.mu.RLock()
	var ids []string
	if grp, ok := m.byGroup[group]; ok {
		ids = make([]string, 0, len(grp))
		for id := range grp {
			ids = append(ids, id)
		}
	}
	m.mu.RUnlock()

	sent := 0
	for _, id := range ids {
		m.mu.RLock()
		conn := m.connections[id]
		m.mu.RUnlock()

		if conn != nil && !conn.IsClosed() {
			if err := conn.Send(msgType, data); err == nil {
				sent++
			}
		}
	}
	return sent
}

// JoinGroup adds a connection to a group.
func (m *ConnectionManager) JoinGroup(connID, group string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	conn, exists := m.connections[connID]
	if !exists {
		return ErrConnectionNotFound
	}

	// Add to connection's groups
	conn.mu.Lock()
	if _, exists := conn.groups[group]; exists {
		conn.mu.Unlock()
		return ErrAlreadyInGroup
	}
	conn.groups[group] = struct{}{}
	conn.mu.Unlock()

	// Add to manager's group mapping
	if m.byGroup[group] == nil {
		m.byGroup[group] = make(map[string]bool)
	}
	m.byGroup[group][connID] = true

	return nil
}

// LeaveGroup removes a connection from a group.
func (m *ConnectionManager) LeaveGroup(connID, group string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	conn, exists := m.connections[connID]
	if !exists {
		return ErrConnectionNotFound
	}

	// Remove from connection's groups
	conn.mu.Lock()
	if _, exists := conn.groups[group]; !exists {
		conn.mu.Unlock()
		return ErrNotInGroup
	}
	delete(conn.groups, group)
	conn.mu.Unlock()

	// Remove from manager's group mapping
	if grp, ok := m.byGroup[group]; ok {
		delete(grp, connID)
		if len(grp) == 0 {
			delete(m.byGroup, group)
		}
	}

	return nil
}

// addToGroup is called by Connection.JoinGroup (internal use).
func (m *ConnectionManager) addToGroup(connID, group string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.byGroup[group] == nil {
		m.byGroup[group] = make(map[string]bool)
	}
	m.byGroup[group][connID] = true
}

// removeFromGroup is called by Connection.LeaveGroup (internal use).
func (m *ConnectionManager) removeFromGroup(connID, group string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if grp, ok := m.byGroup[group]; ok {
		delete(grp, connID)
		if len(grp) == 0 {
			delete(m.byGroup, group)
		}
	}
}

// Stats returns aggregate statistics.
func (m *ConnectionManager) Stats() *Stats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Calculate current stats
	var totalSent, totalRecv int64
	for _, conn := range m.connections {
		totalSent += conn.MessagesSent()
		totalRecv += conn.MessagesReceived()
	}

	// Add historical stats
	totalSent += m.totalMsgSent.Load()
	totalRecv += m.totalMsgRecv.Load()

	// Connection counts by endpoint
	byEndpoint := make(map[string]int)
	for path, eps := range m.byEndpoint {
		byEndpoint[path] = len(eps)
	}

	return &Stats{
		TotalConnections:      len(m.connections),
		TotalEndpoints:        len(m.endpoints),
		TotalMessagesSent:     totalSent,
		TotalMessagesReceived: totalRecv,
		ConnectionsByEndpoint: byEndpoint,
		Uptime:                time.Since(m.startTime).String(),
	}
}

// Close closes all connections and cleans up.
func (m *ConnectionManager) Close() {
	m.mu.Lock()
	conns := make([]*Connection, 0, len(m.connections))
	for _, conn := range m.connections {
		conns = append(conns, conn)
	}
	m.mu.Unlock()

	// Close all connections
	for _, conn := range conns {
		conn.Close(CloseGoingAway, "server shutdown")
	}

	m.mu.Lock()
	m.connections = make(map[string]*Connection)
	m.byEndpoint = make(map[string]map[string]bool)
	m.byGroup = make(map[string]map[string]bool)
	m.mu.Unlock()
}

// DisconnectByID closes a specific connection.
func (m *ConnectionManager) DisconnectByID(id string, code CloseCode, reason string) error {
	m.mu.RLock()
	conn := m.connections[id]
	m.mu.RUnlock()

	if conn == nil {
		return ErrConnectionNotFound
	}

	return conn.Close(code, reason)
}

// SendToConnection sends a message to a specific connection.
func (m *ConnectionManager) SendToConnection(id string, msgType MessageType, data []byte) error {
	m.mu.RLock()
	conn := m.connections[id]
	m.mu.RUnlock()

	if conn == nil {
		return ErrConnectionNotFound
	}

	return conn.Send(msgType, data)
}

// GetConnectionInfo returns info for a specific connection.
func (m *ConnectionManager) GetConnectionInfo(id string) (*ConnectionInfo, error) {
	m.mu.RLock()
	conn := m.connections[id]
	m.mu.RUnlock()

	if conn == nil {
		return nil, ErrConnectionNotFound
	}

	return conn.Info(), nil
}

// ListConnectionInfos returns info for all connections.
func (m *ConnectionManager) ListConnectionInfos(endpointFilter, groupFilter string) []*ConnectionInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var conns []*Connection

	if endpointFilter != "" {
		if eps, ok := m.byEndpoint[endpointFilter]; ok {
			for id := range eps {
				if conn := m.connections[id]; conn != nil {
					conns = append(conns, conn)
				}
			}
		}
	} else if groupFilter != "" {
		if grp, ok := m.byGroup[groupFilter]; ok {
			for id := range grp {
				if conn := m.connections[id]; conn != nil {
					conns = append(conns, conn)
				}
			}
		}
	} else {
		for _, conn := range m.connections {
			conns = append(conns, conn)
		}
	}

	infos := make([]*ConnectionInfo, 0, len(conns))
	for _, conn := range conns {
		infos = append(infos, conn.Info())
	}
	return infos
}

// ListEndpointInfos returns info for all endpoints.
func (m *ConnectionManager) ListEndpointInfos() []*EndpointInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()

	infos := make([]*EndpointInfo, 0, len(m.endpoints))
	for _, e := range m.endpoints {
		infos = append(infos, e.Info())
	}
	return infos
}
