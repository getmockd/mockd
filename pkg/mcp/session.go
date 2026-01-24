package mcp

import (
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"
)

// MCPSession represents a single client session.
type MCPSession struct {
	// ID is the unique session identifier.
	ID string

	// ProtocolVersion is the negotiated protocol version.
	ProtocolVersion string

	// ClientInfo contains information about the connected client.
	ClientInfo ClientInfo

	// Capabilities are the client-declared capabilities.
	Capabilities ClientCapabilities

	// State is the current session lifecycle state.
	State SessionState

	// Subscriptions tracks subscribed resource URIs.
	Subscriptions map[string]bool

	// EventChannel is the outbound event channel for SSE notifications.
	EventChannel chan *JSONRPCNotification

	// CreatedAt is when the session was created.
	CreatedAt time.Time

	// LastActiveAt is the timestamp of the last request.
	LastActiveAt time.Time

	mu sync.RWMutex
}

// NewSession creates a new session with a generated ID.
func NewSession() *MCPSession {
	return &MCPSession{
		ID:            generateSessionID(),
		State:         SessionStateNew,
		Subscriptions: make(map[string]bool),
		EventChannel:  make(chan *JSONRPCNotification, 100),
		CreatedAt:     time.Now(),
		LastActiveAt:  time.Now(),
	}
}

// generateSessionID creates a unique session ID.
func generateSessionID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		// Fallback to timestamp-based ID if crypto/rand fails
		return hex.EncodeToString([]byte(time.Now().String()))
	}
	return hex.EncodeToString(b)
}

// Touch updates the last active timestamp.
func (s *MCPSession) Touch() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.LastActiveAt = time.Now()
}

// IsExpired checks if the session has expired.
func (s *MCPSession) IsExpired(timeout time.Duration) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return time.Since(s.LastActiveAt) > timeout
}

// SetState updates the session state.
func (s *MCPSession) SetState(state SessionState) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.State = state
}

// GetState returns the current session state.
func (s *MCPSession) GetState() SessionState {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.State
}

// Subscribe adds a resource URI to subscriptions.
func (s *MCPSession) Subscribe(uri string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Subscriptions[uri] = true
}

// Unsubscribe removes a resource URI from subscriptions.
func (s *MCPSession) Unsubscribe(uri string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.Subscriptions, uri)
}

// IsSubscribed checks if the session is subscribed to a URI.
func (s *MCPSession) IsSubscribed(uri string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.Subscriptions[uri]
}

// GetSubscriptions returns all subscribed URIs.
func (s *MCPSession) GetSubscriptions() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	uris := make([]string, 0, len(s.Subscriptions))
	for uri := range s.Subscriptions {
		uris = append(uris, uri)
	}
	return uris
}

// SendNotification sends a notification to the session's event channel.
// Returns false if the channel is full or closed.
func (s *MCPSession) SendNotification(notif *JSONRPCNotification) bool {
	select {
	case s.EventChannel <- notif:
		return true
	default:
		return false
	}
}

// Close closes the session's event channel.
func (s *MCPSession) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.State = SessionStateExpired
	close(s.EventChannel)
}

// SessionManager manages all active MCP sessions.
type SessionManager struct {
	sessions map[string]*MCPSession
	config   *Config
	mu       sync.RWMutex
}

// NewSessionManager creates a new session manager.
func NewSessionManager(cfg *Config) *SessionManager {
	return &SessionManager{
		sessions: make(map[string]*MCPSession),
		config:   cfg,
	}
}

// Create creates a new session and adds it to the manager.
// Returns an error if the maximum session limit is reached.
func (m *SessionManager) Create() (*MCPSession, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if len(m.sessions) >= m.config.MaxSessions {
		// Try to clean up expired sessions first
		m.cleanupLocked()

		if len(m.sessions) >= m.config.MaxSessions {
			return nil, NewJSONRPCErrorWithMessage(
				ErrCodeInternalError,
				"maximum session limit reached",
				nil,
			)
		}
	}

	session := NewSession()
	m.sessions[session.ID] = session
	return session, nil
}

// Get retrieves a session by ID.
func (m *SessionManager) Get(id string) *MCPSession {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.sessions[id]
}

// Touch updates a session's last active timestamp.
func (m *SessionManager) Touch(id string) {
	m.mu.RLock()
	session := m.sessions[id]
	m.mu.RUnlock()

	if session != nil {
		session.Touch()
	}
}

// Delete removes a session by ID.
func (m *SessionManager) Delete(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if session, ok := m.sessions[id]; ok {
		session.Close() // Properly closes EventChannel and sets state
		delete(m.sessions, id)
	}
}

// Cleanup removes all expired sessions.
func (m *SessionManager) Cleanup() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.cleanupLocked()
}

// cleanupLocked removes expired sessions (must be called with lock held).
func (m *SessionManager) cleanupLocked() int {
	removed := 0
	for id, session := range m.sessions {
		if session.IsExpired(m.config.SessionTimeout) {
			session.Close() // Properly closes EventChannel and sets state
			delete(m.sessions, id)
			removed++
		}
	}
	return removed
}

// Count returns the number of active sessions.
func (m *SessionManager) Count() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.sessions)
}

// List returns all session IDs.
func (m *SessionManager) List() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	ids := make([]string, 0, len(m.sessions))
	for id := range m.sessions {
		ids = append(ids, id)
	}
	return ids
}

// Broadcast sends a notification to all sessions.
func (m *SessionManager) Broadcast(notif *JSONRPCNotification) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, session := range m.sessions {
		if session.GetState() == SessionStateReady {
			session.SendNotification(notif)
		}
	}
}

// BroadcastToSubscribers sends a notification to all sessions subscribed to a URI.
func (m *SessionManager) BroadcastToSubscribers(uri string, notif *JSONRPCNotification) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, session := range m.sessions {
		if session.GetState() == SessionStateReady && session.IsSubscribed(uri) {
			session.SendNotification(notif)
		}
	}
}

// StartCleanupRoutine starts a goroutine that periodically cleans up expired sessions.
func (m *SessionManager) StartCleanupRoutine(interval time.Duration, stop <-chan struct{}) {
	ticker := time.NewTicker(interval)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				m.Cleanup()
			case <-stop:
				return
			}
		}
	}()
}

// Close closes all sessions and cleans up resources.
func (m *SessionManager) Close() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for id, session := range m.sessions {
		session.Close() // Properly closes EventChannel and sets state
		delete(m.sessions, id)
	}
}
