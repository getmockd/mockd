package mqtt

import (
	"sync"
	"time"

	"github.com/google/uuid"
)

const (
	// MaxMessageHistory is the maximum number of messages to keep in history
	MaxMessageHistory = 1000
)

// MQTTMessage represents a message in the test panel history
type MQTTMessage struct {
	ID             string    `json:"id"`
	Timestamp      time.Time `json:"timestamp"`
	Topic          string    `json:"topic"`
	Payload        string    `json:"payload"`
	PayloadFormat  string    `json:"payloadFormat"` // "json", "text", "hex", "base64"
	QoS            int       `json:"qos"`
	Retain         bool      `json:"retain"`
	Direction      string    `json:"direction"` // "outgoing" or "incoming"
	ClientID       string    `json:"clientId,omitempty"`
	IsMockResponse bool      `json:"isMockResponse"`
	MockResponseID string    `json:"mockResponseId,omitempty"`
}

// TestPanelSession represents an active test panel session
type TestPanelSession struct {
	ID             string        `json:"id"`
	BrokerID       string        `json:"brokerId"`
	Subscriptions  []string      `json:"subscriptions"`
	MessageHistory []MQTTMessage `json:"messageHistory"`
	CreatedAt      time.Time     `json:"createdAt"`
	LastActivity   time.Time     `json:"lastActivity"`
	mu             sync.RWMutex
	listeners      []chan MQTTMessage
}

// NewTestPanelSession creates a new test panel session
func NewTestPanelSession(brokerID string) *TestPanelSession {
	return &TestPanelSession{
		ID:             uuid.New().String(),
		BrokerID:       brokerID,
		Subscriptions:  make([]string, 0),
		MessageHistory: make([]MQTTMessage, 0),
		CreatedAt:      time.Now(),
		LastActivity:   time.Now(),
		listeners:      make([]chan MQTTMessage, 0),
	}
}

// AddSubscription adds a topic subscription to the session
func (s *TestPanelSession) AddSubscription(topic string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Check if already subscribed
	for _, sub := range s.Subscriptions {
		if sub == topic {
			return
		}
	}
	s.Subscriptions = append(s.Subscriptions, topic)
	s.LastActivity = time.Now()
}

// RemoveSubscription removes a topic subscription
func (s *TestPanelSession) RemoveSubscription(topic string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i, sub := range s.Subscriptions {
		if sub == topic {
			s.Subscriptions = append(s.Subscriptions[:i], s.Subscriptions[i+1:]...)
			s.LastActivity = time.Now()
			return
		}
	}
}

// AddMessage adds a message to the session history
func (s *TestPanelSession) AddMessage(msg MQTTMessage) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Add message to history
	s.MessageHistory = append(s.MessageHistory, msg)

	// Trim history if it exceeds max
	if len(s.MessageHistory) > MaxMessageHistory {
		s.MessageHistory = s.MessageHistory[len(s.MessageHistory)-MaxMessageHistory:]
	}

	s.LastActivity = time.Now()

	// Notify all listeners
	for _, listener := range s.listeners {
		select {
		case listener <- msg:
		default:
			// Listener buffer full, skip
		}
	}
}

// GetMessages returns the message history
func (s *TestPanelSession) GetMessages(limit int) []MQTTMessage {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if limit <= 0 || limit > len(s.MessageHistory) {
		limit = len(s.MessageHistory)
	}

	// Return most recent messages
	start := len(s.MessageHistory) - limit
	result := make([]MQTTMessage, limit)
	copy(result, s.MessageHistory[start:])
	return result
}

// ClearHistory clears the message history
func (s *TestPanelSession) ClearHistory() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.MessageHistory = make([]MQTTMessage, 0)
	s.LastActivity = time.Now()
}

// Subscribe adds a listener channel for new messages
func (s *TestPanelSession) Subscribe() chan MQTTMessage {
	s.mu.Lock()
	defer s.mu.Unlock()

	ch := make(chan MQTTMessage, 100)
	s.listeners = append(s.listeners, ch)
	return ch
}

// Unsubscribe removes a listener channel
func (s *TestPanelSession) Unsubscribe(ch chan MQTTMessage) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i, listener := range s.listeners {
		if listener == ch {
			s.listeners = append(s.listeners[:i], s.listeners[i+1:]...)
			close(ch)
			return
		}
	}
}

// IsSubscribed checks if a topic matches any of the session's subscriptions
func (s *TestPanelSession) IsSubscribed(topic string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, sub := range s.Subscriptions {
		if matchTopic(sub, topic) {
			return true
		}
	}
	return false
}

// SessionManager manages active test panel sessions
type SessionManager struct {
	sessions map[string]*TestPanelSession // sessionID -> session
	byBroker map[string][]string          // brokerID -> sessionIDs
	mu       sync.RWMutex
}

// NewSessionManager creates a new session manager
func NewSessionManager() *SessionManager {
	return &SessionManager{
		sessions: make(map[string]*TestPanelSession),
		byBroker: make(map[string][]string),
	}
}

// CreateSession creates a new test panel session for a broker
func (m *SessionManager) CreateSession(brokerID string) *TestPanelSession {
	m.mu.Lock()
	defer m.mu.Unlock()

	session := NewTestPanelSession(brokerID)
	m.sessions[session.ID] = session
	m.byBroker[brokerID] = append(m.byBroker[brokerID], session.ID)
	return session
}

// GetSession returns a session by ID
func (m *SessionManager) GetSession(sessionID string) *TestPanelSession {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.sessions[sessionID]
}

// DeleteSession removes a session
func (m *SessionManager) DeleteSession(sessionID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	session, exists := m.sessions[sessionID]
	if !exists {
		return
	}

	// Remove from byBroker map
	brokerSessions := m.byBroker[session.BrokerID]
	for i, id := range brokerSessions {
		if id == sessionID {
			m.byBroker[session.BrokerID] = append(brokerSessions[:i], brokerSessions[i+1:]...)
			break
		}
	}

	// Close all listeners
	session.mu.Lock()
	for _, listener := range session.listeners {
		close(listener)
	}
	session.listeners = nil
	session.mu.Unlock()

	delete(m.sessions, sessionID)
}

// GetBrokerSessions returns all sessions for a broker
func (m *SessionManager) GetBrokerSessions(brokerID string) []*TestPanelSession {
	m.mu.RLock()
	defer m.mu.RUnlock()

	sessionIDs := m.byBroker[brokerID]
	sessions := make([]*TestPanelSession, 0, len(sessionIDs))
	for _, id := range sessionIDs {
		if session, exists := m.sessions[id]; exists {
			sessions = append(sessions, session)
		}
	}
	return sessions
}

// NotifyMessage notifies all sessions for a broker about a new message
func (m *SessionManager) NotifyMessage(brokerID string, msg MQTTMessage) {
	sessions := m.GetBrokerSessions(brokerID)
	for _, session := range sessions {
		if session.IsSubscribed(msg.Topic) {
			session.AddMessage(msg)
		}
	}
}

// CleanupStaleSessions removes sessions that have been inactive for longer than maxAge.
// This prevents memory leaks from abandoned sessions with unclosed listener channels.
func (m *SessionManager) CleanupStaleSessions(maxAge time.Duration) int {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	removed := 0

	for sessionID, session := range m.sessions {
		session.mu.RLock()
		lastActivity := session.LastActivity
		brokerID := session.BrokerID
		session.mu.RUnlock()

		if now.Sub(lastActivity) > maxAge {
			// Close all listeners to prevent leaks
			session.mu.Lock()
			for _, listener := range session.listeners {
				close(listener)
			}
			session.listeners = nil
			session.mu.Unlock()

			// Remove from byBroker map
			brokerSessions := m.byBroker[brokerID]
			for i, id := range brokerSessions {
				if id == sessionID {
					m.byBroker[brokerID] = append(brokerSessions[:i], brokerSessions[i+1:]...)
					break
				}
			}

			delete(m.sessions, sessionID)
			removed++
		}
	}

	return removed
}

// CloseListeners closes all listeners for a session without removing it.
// This is useful when a client disconnects but may reconnect.
func (s *TestPanelSession) CloseListeners() {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, listener := range s.listeners {
		close(listener)
	}
	s.listeners = nil
}
