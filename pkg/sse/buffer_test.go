package sse

import (
	"sync"
	"testing"
	"time"
)

func TestEventBuffer_Add(t *testing.T) {
	buffer := NewEventBuffer(5, 0)

	event := BufferedEvent{
		ID:        "1",
		Event:     SSEEventDef{Data: "test"},
		Timestamp: time.Now(),
		Index:     1,
	}

	buffer.Add(event)

	if buffer.Size() != 1 {
		t.Errorf("expected size 1, got %d", buffer.Size())
	}
}

func TestEventBuffer_SizeLimit(t *testing.T) {
	buffer := NewEventBuffer(3, 0)

	for i := 0; i < 5; i++ {
		buffer.Add(BufferedEvent{
			ID:        string(rune('a' + i)),
			Timestamp: time.Now(),
			Index:     int64(i),
		})
	}

	if buffer.Size() != 3 {
		t.Errorf("expected size 3 (max), got %d", buffer.Size())
	}

	// Should have the last 3 events (c, d, e)
	events := buffer.GetLatest(10)
	if len(events) != 3 {
		t.Fatalf("expected 3 events, got %d", len(events))
	}
	if events[0].ID != "c" {
		t.Errorf("expected first event 'c', got %q", events[0].ID)
	}
}

func TestEventBuffer_GetEventsAfterID(t *testing.T) {
	buffer := NewEventBuffer(10, 0)

	for i := 0; i < 5; i++ {
		buffer.Add(BufferedEvent{
			ID:        string(rune('a' + i)),
			Event:     SSEEventDef{Data: "test"},
			Timestamp: time.Now(),
			Index:     int64(i),
		})
	}

	events := buffer.GetEventsAfterID("b")
	// Events after 'b' are: c, d, e (3 events)
	if len(events) != 3 {
		t.Fatalf("expected 3 events after 'b', got %d", len(events))
	}
	if events[0].ID != "c" {
		t.Errorf("expected first event 'c', got %q", events[0].ID)
	}
	if events[1].ID != "d" {
		t.Errorf("expected second event 'd', got %q", events[1].ID)
	}
	if events[2].ID != "e" {
		t.Errorf("expected third event 'e', got %q", events[2].ID)
	}
}

func TestEventBuffer_GetEventsAfterID_NotFound(t *testing.T) {
	buffer := NewEventBuffer(10, 0)

	buffer.Add(BufferedEvent{ID: "a"})
	buffer.Add(BufferedEvent{ID: "b"})

	events := buffer.GetEventsAfterID("z")
	if events != nil {
		t.Errorf("expected nil for non-existent ID, got %v", events)
	}
}

func TestEventBuffer_GetEventsAfterID_LastEvent(t *testing.T) {
	buffer := NewEventBuffer(10, 0)

	buffer.Add(BufferedEvent{ID: "a"})
	buffer.Add(BufferedEvent{ID: "b"})

	events := buffer.GetEventsAfterID("b")
	if events != nil {
		t.Errorf("expected nil for last event ID, got %v", events)
	}
}

func TestEventBuffer_GetEventsAfterIndex(t *testing.T) {
	buffer := NewEventBuffer(10, 0)

	for i := 0; i < 5; i++ {
		buffer.Add(BufferedEvent{
			ID:    string(rune('a' + i)),
			Index: int64(i + 10),
		})
	}

	events := buffer.GetEventsAfterIndex(11)
	if len(events) != 3 {
		t.Fatalf("expected 3 events after index 11, got %d", len(events))
	}
}

func TestEventBuffer_GetEvent(t *testing.T) {
	buffer := NewEventBuffer(10, 0)

	buffer.Add(BufferedEvent{ID: "a", Event: SSEEventDef{Data: "alpha"}})
	buffer.Add(BufferedEvent{ID: "b", Event: SSEEventDef{Data: "beta"}})

	event := buffer.GetEvent("b")
	if event == nil {
		t.Fatal("expected to find event 'b'")
		return
	}
	if event.Event.Data != "beta" {
		t.Errorf("expected data 'beta', got %v", event.Event.Data)
	}

	event = buffer.GetEvent("z")
	if event != nil {
		t.Errorf("expected nil for non-existent event, got %v", event)
	}
}

func TestEventBuffer_GetLatest(t *testing.T) {
	buffer := NewEventBuffer(10, 0)

	for i := 0; i < 5; i++ {
		buffer.Add(BufferedEvent{
			ID:    string(rune('a' + i)),
			Index: int64(i),
		})
	}

	events := buffer.GetLatest(3)
	if len(events) != 3 {
		t.Fatalf("expected 3 events, got %d", len(events))
	}
	if events[0].ID != "c" {
		t.Errorf("expected first 'c', got %q", events[0].ID)
	}
	if events[2].ID != "e" {
		t.Errorf("expected last 'e', got %q", events[2].ID)
	}

	// Request more than available
	events = buffer.GetLatest(100)
	if len(events) != 5 {
		t.Errorf("expected all 5 events, got %d", len(events))
	}
}

func TestEventBuffer_GetLatest_Empty(t *testing.T) {
	buffer := NewEventBuffer(10, 0)

	events := buffer.GetLatest(5)
	if events != nil {
		t.Errorf("expected nil for empty buffer, got %v", events)
	}
}

func TestEventBuffer_GetLatest_ZeroCount(t *testing.T) {
	buffer := NewEventBuffer(10, 0)
	buffer.Add(BufferedEvent{ID: "a"})

	events := buffer.GetLatest(0)
	if events != nil {
		t.Errorf("expected nil for zero count, got %v", events)
	}
}

func TestEventBuffer_Clear(t *testing.T) {
	buffer := NewEventBuffer(10, 0)

	for i := 0; i < 5; i++ {
		buffer.Add(BufferedEvent{ID: string(rune('a' + i))})
	}

	if buffer.Size() != 5 {
		t.Errorf("expected size 5, got %d", buffer.Size())
	}

	buffer.Clear()

	if buffer.Size() != 0 {
		t.Errorf("expected size 0 after clear, got %d", buffer.Size())
	}
}

func TestEventBuffer_AgeExpiration(t *testing.T) {
	buffer := NewEventBuffer(10, 1) // 1 second max age

	// Add an event
	buffer.Add(BufferedEvent{
		ID:        "old",
		Timestamp: time.Now().Add(-2 * time.Second), // 2 seconds old
	})

	// Add a fresh event
	buffer.Add(BufferedEvent{
		ID:        "new",
		Timestamp: time.Now(),
	})

	// Cleanup should remove old event
	buffer.Cleanup()

	if buffer.Size() != 1 {
		t.Errorf("expected 1 event after cleanup, got %d", buffer.Size())
	}

	event := buffer.GetEvent("old")
	if event != nil {
		t.Error("expected old event to be removed")
	}

	event = buffer.GetEvent("new")
	if event == nil {
		t.Error("expected new event to remain")
	}
}

func TestEventBuffer_Stats(t *testing.T) {
	buffer := NewEventBuffer(10, 0)

	now := time.Now()
	buffer.Add(BufferedEvent{ID: "a", Timestamp: now.Add(-1 * time.Minute)})
	buffer.Add(BufferedEvent{ID: "b", Timestamp: now.Add(-30 * time.Second)})
	buffer.Add(BufferedEvent{ID: "c", Timestamp: now})

	stats := buffer.Stats()

	if stats.Size != 3 {
		t.Errorf("expected size 3, got %d", stats.Size)
	}
	if stats.Capacity != 10 {
		t.Errorf("expected capacity 10, got %d", stats.Capacity)
	}
	if stats.OldestID != "a" {
		t.Errorf("expected oldest 'a', got %q", stats.OldestID)
	}
	if stats.NewestID != "c" {
		t.Errorf("expected newest 'c', got %q", stats.NewestID)
	}
}

func TestEventBuffer_Stats_Empty(t *testing.T) {
	buffer := NewEventBuffer(10, 0)
	stats := buffer.Stats()

	if stats.Size != 0 {
		t.Errorf("expected size 0, got %d", stats.Size)
	}
	if stats.OldestID != "" {
		t.Errorf("expected empty oldest ID, got %q", stats.OldestID)
	}
}

func TestEventBuffer_Concurrent(t *testing.T) {
	buffer := NewEventBuffer(100, 0)
	var wg sync.WaitGroup

	// Concurrent writes
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			for j := 0; j < 10; j++ {
				buffer.Add(BufferedEvent{
					ID:        string(rune('a' + n)),
					Timestamp: time.Now(),
				})
			}
		}(i)
	}

	// Concurrent reads
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 20; j++ {
				_ = buffer.Size()
				_ = buffer.GetLatest(5)
				_ = buffer.Stats()
			}
		}()
	}

	wg.Wait()

	if buffer.Size() > 100 {
		t.Errorf("buffer exceeded max size: %d", buffer.Size())
	}
}

func TestEventBuffer_DefaultSize(t *testing.T) {
	buffer := NewEventBuffer(0, 0) // 0 should use default

	// Should use DefaultBufferSize
	for i := 0; i < DefaultBufferSize+10; i++ {
		buffer.Add(BufferedEvent{ID: string(rune(i))})
	}

	if buffer.Size() != DefaultBufferSize {
		t.Errorf("expected default size %d, got %d", DefaultBufferSize, buffer.Size())
	}
}

// EventBufferPool tests

func TestEventBufferPool_GetOrCreate(t *testing.T) {
	pool := NewEventBufferPool(BufferPoolConfig{
		DefaultSize:   50,
		DefaultMaxAge: 60,
	})

	buffer1 := pool.GetOrCreate("mock-1")
	if buffer1 == nil {
		t.Fatal("expected buffer to be created")
	}

	buffer2 := pool.GetOrCreate("mock-1")
	if buffer1 != buffer2 {
		t.Error("expected same buffer for same key")
	}

	buffer3 := pool.GetOrCreate("mock-2")
	if buffer3 == buffer1 {
		t.Error("expected different buffer for different key")
	}
}

func TestEventBufferPool_Get(t *testing.T) {
	pool := NewEventBufferPool(BufferPoolConfig{})

	buffer := pool.Get("nonexistent")
	if buffer != nil {
		t.Error("expected nil for non-existent key")
	}

	pool.GetOrCreate("exists")
	buffer = pool.Get("exists")
	if buffer == nil {
		t.Error("expected buffer for existing key")
	}
}

func TestEventBufferPool_Remove(t *testing.T) {
	pool := NewEventBufferPool(BufferPoolConfig{})

	pool.GetOrCreate("test")
	pool.Remove("test")

	buffer := pool.Get("test")
	if buffer != nil {
		t.Error("expected nil after remove")
	}
}

func TestEventBufferPool_Clear(t *testing.T) {
	pool := NewEventBufferPool(BufferPoolConfig{})

	pool.GetOrCreate("a")
	pool.GetOrCreate("b")
	pool.GetOrCreate("c")

	pool.Clear()

	keys := pool.Keys()
	if len(keys) != 0 {
		t.Errorf("expected 0 keys after clear, got %d", len(keys))
	}
}

func TestEventBufferPool_Keys(t *testing.T) {
	pool := NewEventBufferPool(BufferPoolConfig{})

	pool.GetOrCreate("a")
	pool.GetOrCreate("b")
	pool.GetOrCreate("c")

	keys := pool.Keys()
	if len(keys) != 3 {
		t.Errorf("expected 3 keys, got %d", len(keys))
	}
}

func TestEventBufferPool_CleanupAll(t *testing.T) {
	pool := NewEventBufferPool(BufferPoolConfig{
		DefaultSize:   10,
		DefaultMaxAge: 1,
	})

	buffer1 := pool.GetOrCreate("a")
	buffer1.Add(BufferedEvent{
		ID:        "old",
		Timestamp: time.Now().Add(-2 * time.Second),
	})

	buffer2 := pool.GetOrCreate("b")
	buffer2.Add(BufferedEvent{
		ID:        "new",
		Timestamp: time.Now(),
	})

	pool.CleanupAll()

	if buffer1.Size() != 0 {
		t.Errorf("expected buffer1 to be empty after cleanup, got %d", buffer1.Size())
	}
	if buffer2.Size() != 1 {
		t.Errorf("expected buffer2 to have 1 event, got %d", buffer2.Size())
	}
}

func TestEventBufferPool_DefaultConfig(t *testing.T) {
	pool := NewEventBufferPool(BufferPoolConfig{}) // Empty config

	buffer := pool.GetOrCreate("test")

	// Should use DefaultBufferSize
	for i := 0; i < DefaultBufferSize+10; i++ {
		buffer.Add(BufferedEvent{ID: string(rune(i))})
	}

	if buffer.Size() != DefaultBufferSize {
		t.Errorf("expected default size %d, got %d", DefaultBufferSize, buffer.Size())
	}
}
