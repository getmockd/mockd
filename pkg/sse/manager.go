package sse

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/getmockd/mockd/pkg/metrics"
)

// NewConnectionManager creates a new SSE connection manager.
func NewConnectionManager(maxConnections int) *SSEConnectionManager {
	return &SSEConnectionManager{
		connections:    make(map[string]*SSEStream),
		maxConnections: maxConnections,
	}
}

// Register adds a new connection to the manager.
func (m *SSEConnectionManager) Register(stream *SSEStream) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check connection limit
	if m.maxConnections > 0 && len(m.connections) >= m.maxConnections {
		m.connectionErrors++
		return ErrMaxConnectionsReached
	}

	m.connections[stream.ID] = stream
	m.totalConnections++

	// Update metrics
	if metrics.ActiveConnections != nil {
		if vec, err := metrics.ActiveConnections.WithLabels("sse"); err == nil {
			vec.Inc()
		}
	}

	return nil
}

// Deregister removes a connection from the manager.
func (m *SSEConnectionManager) Deregister(streamID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if stream, ok := m.connections[streamID]; ok {
		// Cancel the stream context if not already cancelled
		if stream.cancel != nil {
			stream.cancel()
		}
		delete(m.connections, streamID)

		// Update metrics
		if metrics.ActiveConnections != nil {
			if vec, err := metrics.ActiveConnections.WithLabels("sse"); err == nil {
				vec.Dec()
			}
		}
	}
}

// Get returns a connection by ID.
func (m *SSEConnectionManager) Get(streamID string) *SSEStream {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.connections[streamID]
}

// GetConnections returns all active connections.
func (m *SSEConnectionManager) GetConnections() []*SSEStream {
	m.mu.RLock()
	defer m.mu.RUnlock()

	streams := make([]*SSEStream, 0, len(m.connections))
	for _, stream := range m.connections {
		streams = append(streams, stream)
	}
	return streams
}

// GetConnectionsByMock returns all connections for a specific mock.
func (m *SSEConnectionManager) GetConnectionsByMock(mockID string) []*SSEStream {
	m.mu.RLock()
	defer m.mu.RUnlock()

	streams := make([]*SSEStream, 0)
	for _, stream := range m.connections {
		if stream.MockID == mockID {
			streams = append(streams, stream)
		}
	}
	return streams
}

// Count returns the number of active connections.
func (m *SSEConnectionManager) Count() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.connections)
}

// CountByMock returns the number of connections for a specific mock.
func (m *SSEConnectionManager) CountByMock(mockID string) int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	count := 0
	for _, stream := range m.connections {
		if stream.MockID == mockID {
			count++
		}
	}
	return count
}

// Close gracefully closes a connection by ID.
func (m *SSEConnectionManager) Close(streamID string, graceful bool, finalEvent *SSEEventDef) error {
	m.mu.Lock()
	stream, ok := m.connections[streamID]
	m.mu.Unlock()

	if !ok {
		return ErrStreamClosed
	}

	// Signal the stream to close
	if stream.cancel != nil {
		stream.cancel()
	}

	return nil
}

// CloseAll closes all connections.
func (m *SSEConnectionManager) CloseAll() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, stream := range m.connections {
		if stream.cancel != nil {
			stream.cancel()
		}
	}

	// Clear the map
	m.connections = make(map[string]*SSEStream)
}

// CloseByMock closes all connections for a specific mock.
func (m *SSEConnectionManager) CloseByMock(mockID string) int {
	m.mu.Lock()
	defer m.mu.Unlock()

	count := 0
	for id, stream := range m.connections {
		if stream.MockID == mockID {
			if stream.cancel != nil {
				stream.cancel()
			}
			delete(m.connections, id)
			count++
		}
	}
	return count
}

// Stats returns aggregated statistics.
func (m *SSEConnectionManager) Stats() ConnectionStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := ConnectionStats{
		ActiveConnections: len(m.connections),
		TotalConnections:  m.totalConnections,
		TotalEventsSent:   m.totalEventsSent,
		TotalBytesSent:    m.totalBytesSent,
		ConnectionErrors:  m.connectionErrors,
		ConnectionsByMock: make(map[string]int),
	}

	for _, stream := range m.connections {
		stats.ConnectionsByMock[stream.MockID]++
	}

	return stats
}

// GetConnectionInfo returns connection info for API responses.
func (m *SSEConnectionManager) GetConnectionInfo() []SSEStreamInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()

	info := make([]SSEStreamInfo, 0, len(m.connections))
	for _, stream := range m.connections {
		info = append(info, SSEStreamInfo{
			ID:         stream.ID,
			MockID:     stream.MockID,
			ClientIP:   stream.ClientIP,
			StartTime:  stream.StartTime,
			Duration:   formatDuration(time.Since(stream.StartTime)),
			EventsSent: stream.EventsSent,
			Status:     stream.Status,
		})
	}
	return info
}

// recordEventSent records that an event was sent.
func (m *SSEConnectionManager) recordEventSent(bytes int64) {
	atomic.AddInt64(&m.totalEventsSent, 1)
	atomic.AddInt64(&m.totalBytesSent, bytes)
}

// formatDuration formats a duration as a human-readable string.
func formatDuration(d time.Duration) string {
	if d < time.Second {
		return formatMillis(d.Milliseconds()) + "ms"
	}
	if d < time.Minute {
		return formatFloat(d.Seconds()) + "s"
	}
	if d < time.Hour {
		return formatFloat(d.Minutes()) + "m"
	}
	return formatFloat(d.Hours()) + "h"
}

// formatMillis formats milliseconds.
func formatMillis(ms int64) string {
	return formatInt64(ms)
}

// formatFloat formats a float with 1 decimal place.
func formatFloat(f float64) string {
	whole := int64(f)
	frac := int64((f - float64(whole)) * 10)
	if frac == 0 {
		return formatInt64(whole)
	}
	return formatInt64(whole) + "." + formatInt64(frac)
}

// SSEConnectionManagerOption is a functional option for the connection manager.
type SSEConnectionManagerOption func(*SSEConnectionManager)

// WithMaxConnections sets the maximum number of connections.
func WithMaxConnections(max int) SSEConnectionManagerOption {
	return func(m *SSEConnectionManager) {
		m.maxConnections = max
	}
}

// ConnectionManagerBuilder is a builder for SSEConnectionManager.
type ConnectionManagerBuilder struct {
	maxConnections int
}

// NewConnectionManagerBuilder creates a new builder.
func NewConnectionManagerBuilder() *ConnectionManagerBuilder {
	return &ConnectionManagerBuilder{
		maxConnections: 0, // unlimited by default
	}
}

// MaxConnections sets the maximum connections.
func (b *ConnectionManagerBuilder) MaxConnections(max int) *ConnectionManagerBuilder {
	b.maxConnections = max
	return b
}

// Build creates the connection manager.
func (b *ConnectionManagerBuilder) Build() *SSEConnectionManager {
	return NewConnectionManager(b.maxConnections)
}

// StreamIterator provides iteration over streams.
type StreamIterator struct {
	streams []*SSEStream
	index   int
	mu      sync.RWMutex
}

// NewStreamIterator creates an iterator over connections.
func (m *SSEConnectionManager) NewIterator() *StreamIterator {
	return &StreamIterator{
		streams: m.GetConnections(),
		index:   0,
	}
}

// Next returns the next stream.
func (it *StreamIterator) Next() *SSEStream {
	it.mu.Lock()
	defer it.mu.Unlock()

	if it.index >= len(it.streams) {
		return nil
	}
	stream := it.streams[it.index]
	it.index++
	return stream
}

// HasNext returns whether there are more streams.
func (it *StreamIterator) HasNext() bool {
	it.mu.RLock()
	defer it.mu.RUnlock()
	return it.index < len(it.streams)
}

// Reset resets the iterator.
func (it *StreamIterator) Reset() {
	it.mu.Lock()
	defer it.mu.Unlock()
	it.index = 0
}
