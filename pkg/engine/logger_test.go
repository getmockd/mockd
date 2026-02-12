package engine

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/getmockd/mockd/pkg/requestlog"
)

func TestInMemoryRequestLogger_Log(t *testing.T) {
	logger := NewInMemoryRequestLogger(100)

	entry := &requestlog.Entry{
		Method:         "GET",
		Path:           "/api/test",
		ResponseStatus: 200,
	}

	logger.Log(entry)

	assert.Equal(t, 1, logger.Count())
	assert.NotEmpty(t, entry.ID)
	assert.False(t, entry.Timestamp.IsZero())
}

func TestInMemoryRequestLogger_Get(t *testing.T) {
	logger := NewInMemoryRequestLogger(100)

	entry := &requestlog.Entry{
		Method: "GET",
		Path:   "/api/test",
	}
	logger.Log(entry)

	retrieved := logger.Get(entry.ID)
	require.NotNil(t, retrieved)
	assert.Equal(t, entry.Path, retrieved.Path)
}

func TestInMemoryRequestLogger_GetNotFound(t *testing.T) {
	logger := NewInMemoryRequestLogger(100)

	retrieved := logger.Get("nonexistent")
	assert.Nil(t, retrieved)
}

func TestInMemoryRequestLogger_List(t *testing.T) {
	logger := NewInMemoryRequestLogger(100)

	// Log multiple entries
	for i := 0; i < 5; i++ {
		logger.Log(&requestlog.Entry{
			Method: "GET",
			Path:   "/api/test",
		})
	}

	entries := logger.List(nil)
	assert.Len(t, entries, 5)

	// Verify reverse order (newest first)
	for i := 0; i < len(entries)-1; i++ {
		assert.True(t, entries[i].Timestamp.After(entries[i+1].Timestamp) ||
			entries[i].Timestamp.Equal(entries[i+1].Timestamp))
	}
}

func TestInMemoryRequestLogger_ListWithFilter(t *testing.T) {
	logger := NewInMemoryRequestLogger(100)

	// Log mixed entries
	logger.Log(&requestlog.Entry{Method: "GET", Path: "/api/users"})
	logger.Log(&requestlog.Entry{Method: "POST", Path: "/api/users"})
	logger.Log(&requestlog.Entry{Method: "GET", Path: "/api/orders"})

	// Filter by method
	entries := logger.List(&requestlog.Filter{Method: "GET"})
	assert.Len(t, entries, 2)

	// Filter by path prefix
	entries = logger.List(&requestlog.Filter{Path: "/api/users"})
	assert.Len(t, entries, 2)

	// Combined filter
	entries = logger.List(&requestlog.Filter{Method: "GET", Path: "/api/users"})
	assert.Len(t, entries, 1)
}

func TestInMemoryRequestLogger_ListWithLimit(t *testing.T) {
	logger := NewInMemoryRequestLogger(100)

	for i := 0; i < 10; i++ {
		logger.Log(&requestlog.Entry{Method: "GET", Path: "/api/test"})
	}

	entries := logger.List(&requestlog.Filter{Limit: 3})
	assert.Len(t, entries, 3)
}

func TestInMemoryRequestLogger_ListWithOffset(t *testing.T) {
	logger := NewInMemoryRequestLogger(100)

	for i := 0; i < 10; i++ {
		logger.Log(&requestlog.Entry{Method: "GET", Path: "/api/test"})
	}

	entries := logger.List(&requestlog.Filter{Offset: 3})
	assert.Len(t, entries, 7)
}

func TestInMemoryRequestLogger_Clear(t *testing.T) {
	logger := NewInMemoryRequestLogger(100)

	for i := 0; i < 5; i++ {
		logger.Log(&requestlog.Entry{Method: "GET"})
	}
	assert.Equal(t, 5, logger.Count())

	logger.Clear()
	assert.Equal(t, 0, logger.Count())
}

func TestInMemoryRequestLogger_FIFOEviction(t *testing.T) {
	logger := NewInMemoryRequestLogger(3)

	// Log more than capacity
	logger.Log(&requestlog.Entry{Method: "GET", Path: "/first"})
	time.Sleep(1 * time.Millisecond)
	logger.Log(&requestlog.Entry{Method: "GET", Path: "/second"})
	time.Sleep(1 * time.Millisecond)
	logger.Log(&requestlog.Entry{Method: "GET", Path: "/third"})
	time.Sleep(1 * time.Millisecond)
	logger.Log(&requestlog.Entry{Method: "GET", Path: "/fourth"})

	assert.Equal(t, 3, logger.Count())

	entries := logger.List(nil)
	// Newest first
	assert.Equal(t, "/fourth", entries[0].Path)
	assert.Equal(t, "/third", entries[1].Path)
	assert.Equal(t, "/second", entries[2].Path)
	// First should be evicted
}

func TestInMemoryRequestLogger_FilterByMatchedID(t *testing.T) {
	logger := NewInMemoryRequestLogger(100)

	logger.Log(&requestlog.Entry{Method: "GET", MatchedMockID: "mock-1"})
	logger.Log(&requestlog.Entry{Method: "GET", MatchedMockID: "mock-2"})
	logger.Log(&requestlog.Entry{Method: "GET", MatchedMockID: ""}) // no match

	entries := logger.List(&requestlog.Filter{MatchedID: "mock-1"})
	assert.Len(t, entries, 1)
}

func TestInMemoryRequestLogger_FilterByStatusCode(t *testing.T) {
	logger := NewInMemoryRequestLogger(100)

	logger.Log(&requestlog.Entry{Method: "GET", ResponseStatus: 200})
	logger.Log(&requestlog.Entry{Method: "GET", ResponseStatus: 404})
	logger.Log(&requestlog.Entry{Method: "GET", ResponseStatus: 200})

	entries := logger.List(&requestlog.Filter{StatusCode: 200})
	assert.Len(t, entries, 2)
}

func TestInMemoryRequestLogger_NilEntry(t *testing.T) {
	logger := NewInMemoryRequestLogger(100)

	logger.Log(nil)
	assert.Equal(t, 0, logger.Count())
}

func TestInMemoryRequestLogger_DefaultCapacity(t *testing.T) {
	logger := NewInMemoryRequestLogger(0)
	assert.NotNil(t, logger)

	// Should use default capacity
	for i := 0; i < 100; i++ {
		logger.Log(&requestlog.Entry{Method: "GET"})
	}
	assert.Equal(t, 100, logger.Count())
}

func TestInMemoryRequestLogger_ConcurrentAccess(t *testing.T) {
	logger := NewInMemoryRequestLogger(100)
	done := make(chan bool)

	// Writer goroutine
	go func() {
		for i := 0; i < 50; i++ {
			logger.Log(&requestlog.Entry{Method: "GET"})
		}
		done <- true
	}()

	// Reader goroutine
	go func() {
		for i := 0; i < 50; i++ {
			_ = logger.List(nil)
			_ = logger.Count()
		}
		done <- true
	}()

	<-done
	<-done

	// Should not panic
	assert.GreaterOrEqual(t, logger.Count(), 0)
}

func TestInMemoryRequestLogger_ClearByMockID(t *testing.T) {
	logger := NewInMemoryRequestLogger(100)

	// Log entries for different mocks
	logger.Log(&requestlog.Entry{Method: "GET", MatchedMockID: "mock-1"})
	logger.Log(&requestlog.Entry{Method: "GET", MatchedMockID: "mock-1"})
	logger.Log(&requestlog.Entry{Method: "GET", MatchedMockID: "mock-2"})
	logger.Log(&requestlog.Entry{Method: "GET", MatchedMockID: ""}) // unmatched

	assert.Equal(t, 4, logger.Count())

	// Clear only mock-1 entries
	logger.ClearByMockID("mock-1")

	assert.Equal(t, 2, logger.Count())

	// Verify mock-1 entries are gone
	entries := logger.List(&requestlog.Filter{MatchedID: "mock-1"})
	assert.Len(t, entries, 0)

	// Verify mock-2 entries remain
	entries = logger.List(&requestlog.Filter{MatchedID: "mock-2"})
	assert.Len(t, entries, 1)
}

func TestInMemoryRequestLogger_CountByMockID(t *testing.T) {
	logger := NewInMemoryRequestLogger(100)

	// Log entries for different mocks
	logger.Log(&requestlog.Entry{Method: "GET", MatchedMockID: "mock-1"})
	logger.Log(&requestlog.Entry{Method: "GET", MatchedMockID: "mock-1"})
	logger.Log(&requestlog.Entry{Method: "GET", MatchedMockID: "mock-1"})
	logger.Log(&requestlog.Entry{Method: "GET", MatchedMockID: "mock-2"})

	assert.Equal(t, 3, logger.CountByMockID("mock-1"))
	assert.Equal(t, 1, logger.CountByMockID("mock-2"))
	assert.Equal(t, 0, logger.CountByMockID("mock-3"))
}

func TestInMemoryRequestLogger_Subscribe(t *testing.T) {
	logger := NewInMemoryRequestLogger(100)

	// Subscribe before logging
	sub, unsubscribe := logger.Subscribe()
	defer unsubscribe()

	// Log an entry
	entry := &requestlog.Entry{Method: "GET", Path: "/api/test"}
	logger.Log(entry)

	// Should receive the entry
	select {
	case received := <-sub:
		assert.Equal(t, entry.Path, received.Path)
		assert.NotEmpty(t, received.ID)
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Expected to receive entry from subscriber")
	}
}

func TestInMemoryRequestLogger_SubscribeMultiple(t *testing.T) {
	logger := NewInMemoryRequestLogger(100)

	// Create two subscribers
	sub1, unsub1 := logger.Subscribe()
	defer unsub1()
	sub2, unsub2 := logger.Subscribe()
	defer unsub2()

	// Log an entry
	entry := &requestlog.Entry{Method: "POST", Path: "/api/users"}
	logger.Log(entry)

	// Both should receive the entry
	for _, sub := range []requestlog.Subscriber{sub1, sub2} {
		select {
		case received := <-sub:
			assert.Equal(t, entry.Path, received.Path)
		case <-time.After(100 * time.Millisecond):
			t.Fatal("Expected to receive entry from subscriber")
		}
	}
}

func TestInMemoryRequestLogger_Unsubscribe(t *testing.T) {
	logger := NewInMemoryRequestLogger(100)

	sub, unsubscribe := logger.Subscribe()

	// Unsubscribe
	unsubscribe()

	// Log an entry
	logger.Log(&requestlog.Entry{Method: "GET", Path: "/api/test"})

	// Channel should be closed, not receiving new entries
	select {
	case _, ok := <-sub:
		assert.False(t, ok, "Channel should be closed after unsubscribe")
	case <-time.After(100 * time.Millisecond):
		// Expected - channel is closed and doesn't block
	}
}
