package websocket

import (
	"sync"
	"time"

	"github.com/getmockd/mockd/pkg/template"
)

// EndpointConfig defines the configuration for a WebSocket endpoint.
type EndpointConfig struct {
	// Path is the URL path for WebSocket upgrade (e.g., "/ws/chat").
	Path string `json:"path"`
	// Subprotocols lists supported subprotocols for negotiation.
	Subprotocols []string `json:"subprotocols,omitempty"`
	// RequireSubprotocol rejects connections without a matching subprotocol.
	RequireSubprotocol bool `json:"requireSubprotocol,omitempty"`
	// Matchers contains message matching rules for conditional responses.
	Matchers []*MatcherConfig `json:"matchers,omitempty"`
	// DefaultResponse is sent when no matcher matches.
	DefaultResponse *MessageResponse `json:"defaultResponse,omitempty"`
	// Scenario defines a scripted message sequence.
	Scenario *ScenarioConfig `json:"scenario,omitempty"`
	// Heartbeat configures ping/pong keepalive.
	Heartbeat *HeartbeatConfig `json:"heartbeat,omitempty"`
	// MaxMessageSize is the maximum message size in bytes (default: 65536).
	MaxMessageSize int64 `json:"maxMessageSize,omitempty"`
	// IdleTimeout closes connections after inactivity (default: 0 = disabled).
	IdleTimeout Duration `json:"idleTimeout,omitempty"`
	// MaxConnections limits concurrent connections (default: 0 = unlimited).
	MaxConnections int `json:"maxConnections,omitempty"`
	// EchoMode enables automatic echo of received messages (default: true if no matchers/scenario).
	EchoMode *bool `json:"echoMode,omitempty"`
	// SkipOriginVerify skips verification of the Origin header during WebSocket handshake.
	// Default: true (allows any origin for development/testing convenience).
	// Set to false to enforce that Origin matches the Host header.
	SkipOriginVerify *bool `json:"skipOriginVerify,omitempty"`
}

// HeartbeatConfig configures WebSocket ping/pong keepalive.
type HeartbeatConfig struct {
	// Enabled enables heartbeat pings.
	Enabled bool `json:"enabled"`
	// Interval is the time between pings (default: 30s).
	Interval Duration `json:"interval,omitempty"`
	// Timeout is the maximum wait for pong response (default: 10s).
	Timeout Duration `json:"timeout,omitempty"`
}

// DefaultEndpointConfig returns an EndpointConfig with sensible defaults.
func DefaultEndpointConfig() *EndpointConfig {
	return &EndpointConfig{
		MaxMessageSize: 65536, // 64KB
		MaxConnections: 0,     // unlimited
	}
}

// Endpoint represents a WebSocket endpoint that can accept connections.
type Endpoint struct {
	path               string
	subprotocols       []string
	requireSubprotocol bool
	matchers           []*Matcher
	defaultResponse    *MessageResponse
	scenario           *Scenario
	heartbeat          *HeartbeatConfig
	maxMessageSize     int64
	idleTimeout        time.Duration
	maxConnections     int
	echoMode           bool
	skipOriginVerify   bool
	enabled            bool

	manager        *ConnectionManager
	connections    map[string]*Connection
	templateEngine *template.Engine
	mu             sync.RWMutex
}

// NewEndpoint creates a new WebSocket endpoint with the given configuration.
func NewEndpoint(cfg *EndpointConfig) (*Endpoint, error) {
	if cfg == nil {
		cfg = DefaultEndpointConfig()
	}

	// Set defaults
	maxMsgSize := cfg.MaxMessageSize
	if maxMsgSize <= 0 {
		maxMsgSize = 65536
	}

	// Determine echo mode
	echoMode := true
	if cfg.EchoMode != nil {
		echoMode = *cfg.EchoMode
	} else if len(cfg.Matchers) > 0 || cfg.Scenario != nil {
		// Disable echo mode if matchers or scenario are configured
		echoMode = false
	}

	// Determine skipOriginVerify (default: true for dev-friendly behavior)
	skipOriginVerify := true
	if cfg.SkipOriginVerify != nil {
		skipOriginVerify = *cfg.SkipOriginVerify
	}

	e := &Endpoint{
		path:               cfg.Path,
		subprotocols:       cfg.Subprotocols,
		requireSubprotocol: cfg.RequireSubprotocol,
		defaultResponse:    cfg.DefaultResponse,
		heartbeat:          cfg.Heartbeat,
		maxMessageSize:     maxMsgSize,
		idleTimeout:        cfg.IdleTimeout.Duration(),
		maxConnections:     cfg.MaxConnections,
		echoMode:           echoMode,
		skipOriginVerify:   skipOriginVerify,
		enabled:            true, // Endpoints are enabled by default
		connections:        make(map[string]*Connection),
	}

	// Compile matchers
	for _, mc := range cfg.Matchers {
		m, err := NewMatcher(mc)
		if err != nil {
			return nil, err
		}
		e.matchers = append(e.matchers, m)
	}

	// Load scenario
	if cfg.Scenario != nil {
		s, err := NewScenario(cfg.Scenario)
		if err != nil {
			return nil, err
		}
		e.scenario = s
	}

	return e, nil
}

// Path returns the endpoint path.
func (e *Endpoint) Path() string {
	return e.path
}

// Subprotocols returns the supported subprotocols.
func (e *Endpoint) Subprotocols() []string {
	return e.subprotocols
}

// RequireSubprotocol returns whether a subprotocol is required.
func (e *Endpoint) RequireSubprotocol() bool {
	return e.requireSubprotocol
}

// MaxMessageSize returns the maximum message size.
func (e *Endpoint) MaxMessageSize() int64 {
	return e.maxMessageSize
}

// IdleTimeout returns the idle timeout duration.
func (e *Endpoint) IdleTimeout() time.Duration {
	return e.idleTimeout
}

// MaxConnections returns the maximum connections limit.
func (e *Endpoint) MaxConnections() int {
	return e.maxConnections
}

// EchoMode returns whether echo mode is enabled.
func (e *Endpoint) EchoMode() bool {
	return e.echoMode
}

// SkipOriginVerify returns whether origin verification should be skipped.
func (e *Endpoint) SkipOriginVerify() bool {
	return e.skipOriginVerify
}

// Enabled returns whether the endpoint is enabled.
func (e *Endpoint) Enabled() bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.enabled
}

// SetEnabled sets the enabled state of the endpoint.
func (e *Endpoint) SetEnabled(enabled bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.enabled = enabled
}

// Heartbeat returns the heartbeat configuration.
func (e *Endpoint) Heartbeat() *HeartbeatConfig {
	return e.heartbeat
}

// Scenario returns the scenario configuration.
func (e *Endpoint) Scenario() *Scenario {
	return e.scenario
}

// SetManager sets the connection manager for this endpoint.
func (e *Endpoint) SetManager(m *ConnectionManager) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.manager = m
}

// Manager returns the connection manager.
func (e *Endpoint) Manager() *ConnectionManager {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.manager
}

// SetTemplateEngine sets the template engine for processing response templates.
func (e *Endpoint) SetTemplateEngine(engine *template.Engine) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.templateEngine = engine
}

// CanAccept returns whether a new connection can be accepted.
// Deprecated: Use TryAccept for atomic check-and-add to avoid TOCTOU races.
func (e *Endpoint) CanAccept() bool {
	if e.maxConnections <= 0 {
		return true
	}
	e.mu.RLock()
	defer e.mu.RUnlock()
	return len(e.connections) < e.maxConnections
}

// TryAccept atomically checks whether a new connection can be accepted and,
// if so, adds it to the endpoint. Returns true if the connection was added.
// This avoids the TOCTOU race between CanAccept() and AddConnection().
func (e *Endpoint) TryAccept(conn *Connection) bool {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.maxConnections > 0 && len(e.connections) >= e.maxConnections {
		return false
	}
	e.connections[conn.ID()] = conn
	return true
}

// ConnectionCount returns the number of active connections.
func (e *Endpoint) ConnectionCount() int {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return len(e.connections)
}

// AddConnection adds a connection to this endpoint.
func (e *Endpoint) AddConnection(conn *Connection) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.connections[conn.ID()] = conn
}

// RemoveConnection removes a connection from this endpoint.
func (e *Endpoint) RemoveConnection(id string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	delete(e.connections, id)
}

// GetConnection returns a connection by ID.
func (e *Endpoint) GetConnection(id string) *Connection {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.connections[id]
}

// Connections returns all connections for this endpoint.
func (e *Endpoint) Connections() []*Connection {
	e.mu.RLock()
	defer e.mu.RUnlock()
	conns := make([]*Connection, 0, len(e.connections))
	for _, c := range e.connections {
		conns = append(conns, c)
	}
	return conns
}

// NegotiateSubprotocol selects the best subprotocol from client preferences.
// Returns empty string if no match and subprotocol is not required.
func (e *Endpoint) NegotiateSubprotocol(clientProtocols []string) (string, error) {
	if len(e.subprotocols) == 0 {
		// No server subprotocols configured
		if e.requireSubprotocol {
			return "", ErrSubprotocolRequired
		}
		return "", nil
	}

	// Find first matching subprotocol
	for _, serverProto := range e.subprotocols {
		for _, clientProto := range clientProtocols {
			if serverProto == clientProto {
				return serverProto, nil
			}
		}
	}

	// No match found
	if e.requireSubprotocol {
		return "", ErrSubprotocolMismatch
	}
	return "", nil
}

// MatchMessage finds a response for the given message.
// Returns nil if no matcher matches.
func (e *Endpoint) MatchMessage(msgType MessageType, data []byte) *MessageResponse {
	for _, m := range e.matchers {
		if m.Match(msgType, data) {
			return m.Response()
		}
	}
	return e.defaultResponse
}

// Info returns public information about this endpoint.
func (e *Endpoint) Info() *EndpointInfo {
	e.mu.RLock()
	defer e.mu.RUnlock()

	info := &EndpointInfo{
		Path:               e.path,
		Subprotocols:       e.subprotocols,
		ConnectionCount:    len(e.connections),
		MaxConnections:     e.maxConnections,
		HasScenario:        e.scenario != nil,
		RequireSubprotocol: e.requireSubprotocol,
		MatcherCount:       len(e.matchers),
		MaxMessageSize:     e.maxMessageSize,
		Enabled:            e.enabled,
	}

	if e.scenario != nil {
		info.ScenarioName = e.scenario.Name()
	}

	if e.heartbeat != nil && e.heartbeat.Enabled {
		info.HeartbeatEnabled = true
		info.HeartbeatInterval = e.heartbeat.Interval.Duration().String()
	}

	if e.idleTimeout > 0 {
		info.IdleTimeout = e.idleTimeout.String()
	}

	return info
}

// Broadcast sends a message to all connections on this endpoint.
func (e *Endpoint) Broadcast(msgType MessageType, data []byte) int {
	e.mu.RLock()
	conns := make([]*Connection, 0, len(e.connections))
	for _, c := range e.connections {
		conns = append(conns, c)
	}
	e.mu.RUnlock()

	sent := 0
	for _, conn := range conns {
		if err := conn.Send(msgType, data); err == nil {
			sent++
		}
	}
	return sent
}
