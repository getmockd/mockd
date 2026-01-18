package engine

import (
	"sync"
	"time"

	"github.com/getmockd/mockd/pkg/requestlog"
)

// RequestLogger defines the interface for logging incoming requests.
type RequestLogger interface {
	// Log records a request log entry
	Log(entry *requestlog.Entry)
	// Get retrieves a log entry by ID
	Get(id string) *requestlog.Entry
	// List returns all log entries, optionally filtered
	List(filter *RequestLogFilter) []*requestlog.Entry
	// Clear removes all log entries
	Clear()
	// Count returns the number of log entries
	Count() int
}

// RequestLogFilter defines criteria for filtering request logs.
type RequestLogFilter struct {
	Protocol   string // Filter by protocol (http, grpc, websocket, sse, mqtt, soap, graphql)
	Method     string // Filter by HTTP method (or gRPC method, etc.)
	Path       string // Filter by path prefix (or topic pattern for MQTT)
	MatchedID  string // Filter by matched mock ID
	StatusCode int    // Filter by response status code
	HasError   *bool  // Filter by error presence
	Limit      int    // Maximum number of entries to return
	Offset     int    // Number of entries to skip

	// Protocol-specific filters
	GRPCService     string // Filter gRPC by service name
	MQTTTopic       string // Filter MQTT by topic (supports wildcards)
	MQTTClientID    string // Filter MQTT by client ID
	SOAPOperation   string // Filter SOAP by operation name
	GraphQLOpType   string // Filter GraphQL by operation type (query, mutation, subscription)
	WSConnectionID  string // Filter WebSocket by connection ID
	SSEConnectionID string // Filter SSE by connection ID
}

// LogSubscriber is a channel that receives new log entries.
type LogSubscriber chan *requestlog.Entry

// InMemoryRequestLogger implements RequestLogger with an in-memory circular buffer.
type InMemoryRequestLogger struct {
	entries     []*requestlog.Entry
	maxEntries  int
	mu          sync.RWMutex
	nextID      int64
	subscribers map[LogSubscriber]struct{}
	subMu       sync.RWMutex
}

// NewInMemoryRequestLogger creates a new InMemoryRequestLogger with the given capacity.
func NewInMemoryRequestLogger(maxEntries int) *InMemoryRequestLogger {
	if maxEntries <= 0 {
		maxEntries = 1000 // Default
	}
	return &InMemoryRequestLogger{
		entries:     make([]*requestlog.Entry, 0, maxEntries),
		maxEntries:  maxEntries,
		subscribers: make(map[LogSubscriber]struct{}),
	}
}

// Log records a request log entry.
func (l *InMemoryRequestLogger) Log(entry *requestlog.Entry) {
	if entry == nil {
		return
	}

	l.mu.Lock()

	// Generate ID if not set
	if entry.ID == "" {
		l.nextID++
		entry.ID = generateLogID(l.nextID)
	}

	// Set timestamp if not set
	if entry.Timestamp.IsZero() {
		entry.Timestamp = time.Now()
	}

	// Set default protocol if not set (backwards compatibility)
	if entry.Protocol == "" {
		entry.Protocol = requestlog.ProtocolHTTP
	}

	// FIFO eviction: remove oldest if at capacity
	if len(l.entries) >= l.maxEntries {
		l.entries = l.entries[1:]
	}

	l.entries = append(l.entries, entry)
	l.mu.Unlock()

	// Notify subscribers (non-blocking)
	l.subMu.RLock()
	for sub := range l.subscribers {
		select {
		case sub <- entry:
		default:
			// Drop if subscriber is slow
		}
	}
	l.subMu.RUnlock()
}

// Get retrieves a log entry by ID.
func (l *InMemoryRequestLogger) Get(id string) *requestlog.Entry {
	l.mu.RLock()
	defer l.mu.RUnlock()

	for _, entry := range l.entries {
		if entry.ID == id {
			return entry
		}
	}
	return nil
}

// List returns all log entries, optionally filtered.
func (l *InMemoryRequestLogger) List(filter *RequestLogFilter) []*requestlog.Entry {
	l.mu.RLock()
	defer l.mu.RUnlock()

	// Return in reverse order (newest first)
	result := make([]*requestlog.Entry, 0, len(l.entries))

	// Iterate in reverse order
	for i := len(l.entries) - 1; i >= 0; i-- {
		entry := l.entries[i]

		// Apply filters
		if filter != nil {
			if !matchesFilter(entry, filter) {
				continue
			}
		}

		result = append(result, entry)
	}

	// Apply offset and limit
	if filter != nil {
		if filter.Offset > 0 {
			if filter.Offset >= len(result) {
				return []*requestlog.Entry{}
			}
			result = result[filter.Offset:]
		}
		if filter.Limit > 0 && filter.Limit < len(result) {
			result = result[:filter.Limit]
		}
	}

	return result
}

// matchesFilter checks if an entry matches all filter criteria.
func matchesFilter(entry *requestlog.Entry, filter *RequestLogFilter) bool {
	// Protocol filter
	if filter.Protocol != "" && entry.Protocol != filter.Protocol {
		return false
	}

	// Common filters
	if filter.Method != "" && entry.Method != filter.Method {
		return false
	}
	if filter.Path != "" && !matchesPathPrefix(entry.Path, filter.Path) {
		return false
	}
	if filter.MatchedID != "" && entry.MatchedMockID != filter.MatchedID {
		return false
	}
	if filter.StatusCode != 0 && entry.ResponseStatus != filter.StatusCode {
		return false
	}
	if filter.HasError != nil {
		hasError := entry.Error != ""
		if *filter.HasError != hasError {
			return false
		}
	}

	// Protocol-specific filters
	if filter.GRPCService != "" {
		if entry.GRPC == nil || entry.GRPC.Service != filter.GRPCService {
			return false
		}
	}
	if filter.MQTTTopic != "" {
		if entry.MQTT == nil || !matchesMQTTTopic(filter.MQTTTopic, entry.MQTT.Topic) {
			return false
		}
	}
	if filter.MQTTClientID != "" {
		if entry.MQTT == nil || entry.MQTT.ClientID != filter.MQTTClientID {
			return false
		}
	}
	if filter.SOAPOperation != "" {
		if entry.SOAP == nil || entry.SOAP.Operation != filter.SOAPOperation {
			return false
		}
	}
	if filter.GraphQLOpType != "" {
		if entry.GraphQL == nil || entry.GraphQL.OperationType != filter.GraphQLOpType {
			return false
		}
	}
	if filter.WSConnectionID != "" {
		if entry.WebSocket == nil || entry.WebSocket.ConnectionID != filter.WSConnectionID {
			return false
		}
	}
	if filter.SSEConnectionID != "" {
		if entry.SSE == nil || entry.SSE.ConnectionID != filter.SSEConnectionID {
			return false
		}
	}

	return true
}

// matchesMQTTTopic checks if a topic matches an MQTT topic pattern (supports + and # wildcards).
func matchesMQTTTopic(pattern, topic string) bool {
	if pattern == topic {
		return true
	}
	if pattern == "#" {
		return true
	}

	patternParts := splitTopic(pattern)
	topicParts := splitTopic(topic)

	for i, part := range patternParts {
		if part == "#" {
			return true // # matches everything remaining
		}
		if i >= len(topicParts) {
			return false
		}
		if part == "+" {
			continue // + matches any single level
		}
		if part != topicParts[i] {
			return false
		}
	}

	return len(patternParts) == len(topicParts)
}

// splitTopic splits a topic string by '/'.
func splitTopic(topic string) []string {
	if topic == "" {
		return nil
	}
	result := make([]string, 0)
	start := 0
	for i := 0; i <= len(topic); i++ {
		if i == len(topic) || topic[i] == '/' {
			result = append(result, topic[start:i])
			start = i + 1
		}
	}
	return result
}

// Clear removes all log entries.
func (l *InMemoryRequestLogger) Clear() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.entries = make([]*requestlog.Entry, 0, l.maxEntries)
}

// Count returns the number of log entries.
func (l *InMemoryRequestLogger) Count() int {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return len(l.entries)
}

// ClearByMockID removes all log entries matching the given mock ID.
func (l *InMemoryRequestLogger) ClearByMockID(mockID string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	// Filter out entries matching the mock ID
	filtered := make([]*requestlog.Entry, 0, len(l.entries))
	for _, entry := range l.entries {
		if entry.MatchedMockID != mockID {
			filtered = append(filtered, entry)
		}
	}
	l.entries = filtered
}

// CountByMockID returns the number of log entries matching the given mock ID.
func (l *InMemoryRequestLogger) CountByMockID(mockID string) int {
	l.mu.RLock()
	defer l.mu.RUnlock()

	count := 0
	for _, entry := range l.entries {
		if entry.MatchedMockID == mockID {
			count++
		}
	}
	return count
}

// matchesPathPrefix checks if a path starts with the given prefix.
func matchesPathPrefix(path, prefix string) bool {
	if len(prefix) > len(path) {
		return false
	}
	return path[:len(prefix)] == prefix
}

// generateLogID generates a unique log entry ID.
func generateLogID(n int64) string {
	return "req-" + generateShortID(n)
}

// generateShortID generates a short ID from a number.
func generateShortID(n int64) string {
	const charset = "0123456789abcdefghijklmnopqrstuvwxyz"
	if n == 0 {
		return "0"
	}

	var result []byte
	for n > 0 {
		result = append([]byte{charset[n%36]}, result...)
		n /= 36
	}
	return string(result)
}

// Subscribe registers a subscriber to receive new log entries.
// Returns a channel that will receive entries and an unsubscribe function.
func (l *InMemoryRequestLogger) Subscribe() (LogSubscriber, func()) {
	ch := make(LogSubscriber, 100) // Buffer to prevent blocking

	l.subMu.Lock()
	l.subscribers[ch] = struct{}{}
	l.subMu.Unlock()

	unsubscribe := func() {
		l.subMu.Lock()
		delete(l.subscribers, ch)
		l.subMu.Unlock()
		close(ch)
	}

	return ch, unsubscribe
}
