package engine

import (
	"sync"
	"time"

	"github.com/getmockd/mockd/pkg/config"
)

// RequestLogger defines the interface for logging incoming requests.
type RequestLogger interface {
	// Log records a request log entry
	Log(entry *config.RequestLogEntry)
	// Get retrieves a log entry by ID
	Get(id string) *config.RequestLogEntry
	// List returns all log entries, optionally filtered
	List(filter *RequestLogFilter) []*config.RequestLogEntry
	// Clear removes all log entries
	Clear()
	// Count returns the number of log entries
	Count() int
}

// RequestLogFilter defines criteria for filtering request logs.
type RequestLogFilter struct {
	Method     string // Filter by HTTP method
	Path       string // Filter by path prefix
	MatchedID  string // Filter by matched mock ID
	StatusCode int    // Filter by response status code
	Limit      int    // Maximum number of entries to return
	Offset     int    // Number of entries to skip
}

// InMemoryRequestLogger implements RequestLogger with an in-memory circular buffer.
type InMemoryRequestLogger struct {
	entries    []*config.RequestLogEntry
	maxEntries int
	mu         sync.RWMutex
	nextID     int64
}

// NewInMemoryRequestLogger creates a new InMemoryRequestLogger with the given capacity.
func NewInMemoryRequestLogger(maxEntries int) *InMemoryRequestLogger {
	if maxEntries <= 0 {
		maxEntries = 1000 // Default
	}
	return &InMemoryRequestLogger{
		entries:    make([]*config.RequestLogEntry, 0, maxEntries),
		maxEntries: maxEntries,
	}
}

// Log records a request log entry.
func (l *InMemoryRequestLogger) Log(entry *config.RequestLogEntry) {
	if entry == nil {
		return
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	// Generate ID if not set
	if entry.ID == "" {
		l.nextID++
		entry.ID = generateLogID(l.nextID)
	}

	// Set timestamp if not set
	if entry.Timestamp.IsZero() {
		entry.Timestamp = time.Now()
	}

	// FIFO eviction: remove oldest if at capacity
	if len(l.entries) >= l.maxEntries {
		l.entries = l.entries[1:]
	}

	l.entries = append(l.entries, entry)
}

// Get retrieves a log entry by ID.
func (l *InMemoryRequestLogger) Get(id string) *config.RequestLogEntry {
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
func (l *InMemoryRequestLogger) List(filter *RequestLogFilter) []*config.RequestLogEntry {
	l.mu.RLock()
	defer l.mu.RUnlock()

	// Return in reverse order (newest first)
	result := make([]*config.RequestLogEntry, 0, len(l.entries))

	// Iterate in reverse order
	for i := len(l.entries) - 1; i >= 0; i-- {
		entry := l.entries[i]

		// Apply filters
		if filter != nil {
			if filter.Method != "" && entry.Method != filter.Method {
				continue
			}
			if filter.Path != "" && !matchesPathPrefix(entry.Path, filter.Path) {
				continue
			}
			if filter.MatchedID != "" && entry.MatchedMockID != filter.MatchedID {
				continue
			}
			if filter.StatusCode != 0 && entry.ResponseStatus != filter.StatusCode {
				continue
			}
		}

		result = append(result, entry)
	}

	// Apply offset and limit
	if filter != nil {
		if filter.Offset > 0 {
			if filter.Offset >= len(result) {
				return []*config.RequestLogEntry{}
			}
			result = result[filter.Offset:]
		}
		if filter.Limit > 0 && filter.Limit < len(result) {
			result = result[:filter.Limit]
		}
	}

	return result
}

// Clear removes all log entries.
func (l *InMemoryRequestLogger) Clear() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.entries = make([]*config.RequestLogEntry, 0, l.maxEntries)
}

// Count returns the number of log entries.
func (l *InMemoryRequestLogger) Count() int {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return len(l.entries)
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
