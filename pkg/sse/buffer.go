package sse

import (
	"sync"
	"time"
)

// NewEventBuffer creates a new event buffer for resumption support.
func NewEventBuffer(maxSize int, maxAgeSeconds int) *EventBuffer {
	if maxSize <= 0 {
		maxSize = DefaultBufferSize
	}

	var maxAge time.Duration
	if maxAgeSeconds > 0 {
		maxAge = time.Duration(maxAgeSeconds) * time.Second
	}

	return &EventBuffer{
		events:  make([]BufferedEvent, 0, maxSize),
		maxSize: maxSize,
		maxAge:  maxAge,
	}
}

// Add adds an event to the buffer.
func (b *EventBuffer) Add(event BufferedEvent) {
	b.mu.Lock()
	defer b.mu.Unlock()

	// Clean up old events first
	b.cleanupLocked()

	// If buffer is full, remove oldest
	if len(b.events) >= b.maxSize {
		b.events = b.events[1:]
	}

	b.events = append(b.events, event)
}

// GetEventsAfterID returns all events after the specified ID.
func (b *EventBuffer) GetEventsAfterID(id string) []BufferedEvent {
	b.mu.RLock()
	defer b.mu.RUnlock()

	// Find the index of the event with the given ID
	foundIndex := -1
	for i, e := range b.events {
		if e.ID == id {
			foundIndex = i
			break
		}
	}

	if foundIndex == -1 {
		// ID not found, return empty
		return nil
	}

	// Return events after the found index
	if foundIndex+1 >= len(b.events) {
		return nil
	}

	// Make a copy to avoid data races
	result := make([]BufferedEvent, len(b.events)-foundIndex-1)
	copy(result, b.events[foundIndex+1:])
	return result
}

// GetEventsAfterIndex returns all events after the specified index.
func (b *EventBuffer) GetEventsAfterIndex(index int64) []BufferedEvent {
	b.mu.RLock()
	defer b.mu.RUnlock()

	result := make([]BufferedEvent, 0)
	for _, e := range b.events {
		if e.Index > index {
			result = append(result, e)
		}
	}
	return result
}

// GetEvent returns an event by ID.
func (b *EventBuffer) GetEvent(id string) *BufferedEvent {
	b.mu.RLock()
	defer b.mu.RUnlock()

	for _, e := range b.events {
		if e.ID == id {
			return &e
		}
	}
	return nil
}

// GetLatest returns the most recent events up to count.
func (b *EventBuffer) GetLatest(count int) []BufferedEvent {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if count <= 0 || len(b.events) == 0 {
		return nil
	}

	start := len(b.events) - count
	if start < 0 {
		start = 0
	}

	result := make([]BufferedEvent, len(b.events)-start)
	copy(result, b.events[start:])
	return result
}

// Size returns the current number of buffered events.
func (b *EventBuffer) Size() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return len(b.events)
}

// Clear removes all events from the buffer.
func (b *EventBuffer) Clear() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.events = b.events[:0]
}

// Cleanup removes expired events.
func (b *EventBuffer) Cleanup() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.cleanupLocked()
}

// cleanupLocked removes expired events (must be called with lock held).
func (b *EventBuffer) cleanupLocked() {
	if b.maxAge <= 0 {
		return
	}

	cutoff := time.Now().Add(-b.maxAge)
	newEvents := make([]BufferedEvent, 0, len(b.events))
	for _, e := range b.events {
		if e.Timestamp.After(cutoff) {
			newEvents = append(newEvents, e)
		}
	}
	b.events = newEvents
}

// Stats returns buffer statistics.
func (b *EventBuffer) Stats() BufferStats {
	b.mu.RLock()
	defer b.mu.RUnlock()

	stats := BufferStats{
		Size:     len(b.events),
		Capacity: b.maxSize,
	}

	if len(b.events) > 0 {
		stats.OldestTimestamp = b.events[0].Timestamp
		stats.NewestTimestamp = b.events[len(b.events)-1].Timestamp
		stats.OldestID = b.events[0].ID
		stats.NewestID = b.events[len(b.events)-1].ID
	}

	return stats
}

// BufferStats contains buffer statistics.
type BufferStats struct {
	Size            int       `json:"size"`
	Capacity        int       `json:"capacity"`
	OldestTimestamp time.Time `json:"oldestTimestamp,omitempty"`
	NewestTimestamp time.Time `json:"newestTimestamp,omitempty"`
	OldestID        string    `json:"oldestId,omitempty"`
	NewestID        string    `json:"newestId,omitempty"`
}

// EventBufferPool manages a pool of event buffers.
type EventBufferPool struct {
	buffers map[string]*EventBuffer
	mu      sync.RWMutex
	config  BufferPoolConfig
}

// BufferPoolConfig configures the buffer pool.
type BufferPoolConfig struct {
	DefaultSize   int
	DefaultMaxAge int
}

// NewEventBufferPool creates a new buffer pool.
func NewEventBufferPool(config BufferPoolConfig) *EventBufferPool {
	if config.DefaultSize <= 0 {
		config.DefaultSize = DefaultBufferSize
	}
	return &EventBufferPool{
		buffers: make(map[string]*EventBuffer),
		config:  config,
	}
}

// GetOrCreate gets or creates a buffer for the given key.
func (p *EventBufferPool) GetOrCreate(key string) *EventBuffer {
	p.mu.Lock()
	defer p.mu.Unlock()

	if buffer, ok := p.buffers[key]; ok {
		return buffer
	}

	buffer := NewEventBuffer(p.config.DefaultSize, p.config.DefaultMaxAge)
	p.buffers[key] = buffer
	return buffer
}

// Get returns a buffer if it exists.
func (p *EventBufferPool) Get(key string) *EventBuffer {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.buffers[key]
}

// Remove removes a buffer.
func (p *EventBufferPool) Remove(key string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	delete(p.buffers, key)
}

// Clear removes all buffers.
func (p *EventBufferPool) Clear() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.buffers = make(map[string]*EventBuffer)
}

// Keys returns all buffer keys.
func (p *EventBufferPool) Keys() []string {
	p.mu.RLock()
	defer p.mu.RUnlock()

	keys := make([]string, 0, len(p.buffers))
	for k := range p.buffers {
		keys = append(keys, k)
	}
	return keys
}

// CleanupAll runs cleanup on all buffers.
func (p *EventBufferPool) CleanupAll() {
	p.mu.RLock()
	defer p.mu.RUnlock()

	for _, buffer := range p.buffers {
		buffer.Cleanup()
	}
}
