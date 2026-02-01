package sse

import (
	"context"
	"testing"
	"time"
)

func TestConnectionManager_Register(t *testing.T) {
	manager := NewConnectionManager(100)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	stream := &SSEStream{
		ID:        "test-1",
		MockID:    "mock-1",
		ClientIP:  "127.0.0.1",
		StartTime: time.Now(),
		Status:    StreamStatusActive,
		ctx:       ctx,
		cancel:    cancel,
	}

	err := manager.Register(stream)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if manager.Count() != 1 {
		t.Errorf("expected count 1, got %d", manager.Count())
	}
}

func TestConnectionManager_Deregister(t *testing.T) {
	manager := NewConnectionManager(100)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	stream := &SSEStream{
		ID:     "test-1",
		MockID: "mock-1",
		ctx:    ctx,
		cancel: cancel,
	}

	_ = manager.Register(stream)
	manager.Deregister("test-1")

	if manager.Count() != 0 {
		t.Errorf("expected count 0, got %d", manager.Count())
	}
}

func TestConnectionManager_Get(t *testing.T) {
	manager := NewConnectionManager(100)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	stream := &SSEStream{
		ID:     "test-1",
		MockID: "mock-1",
		ctx:    ctx,
		cancel: cancel,
	}

	_ = manager.Register(stream)

	// Get existing
	found := manager.Get("test-1")
	if found == nil {
		t.Fatal("expected to find stream")
		return
	}
	if found.ID != "test-1" {
		t.Errorf("expected ID 'test-1', got %q", found.ID)
	}

	// Get non-existing
	notFound := manager.Get("non-existing")
	if notFound != nil {
		t.Error("expected nil for non-existing stream")
	}
}

func TestConnectionManager_MaxConnections(t *testing.T) {
	manager := NewConnectionManager(2) // Limit to 2

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Register first two
	for i := 0; i < 2; i++ {
		stream := &SSEStream{
			ID:     formatStreamID(int64(i)),
			MockID: "mock-1",
			ctx:    ctx,
			cancel: cancel,
		}
		err := manager.Register(stream)
		if err != nil {
			t.Fatalf("unexpected error registering stream %d: %v", i, err)
		}
	}

	// Third should fail
	stream := &SSEStream{
		ID:     "test-3",
		MockID: "mock-1",
		ctx:    ctx,
		cancel: cancel,
	}
	err := manager.Register(stream)
	if err != ErrMaxConnectionsReached {
		t.Errorf("expected ErrMaxConnectionsReached, got %v", err)
	}
}

func TestConnectionManager_GetConnections(t *testing.T) {
	manager := NewConnectionManager(100)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Register multiple
	for i := 0; i < 3; i++ {
		stream := &SSEStream{
			ID:     formatStreamID(int64(i)),
			MockID: "mock-1",
			ctx:    ctx,
			cancel: cancel,
		}
		_ = manager.Register(stream)
	}

	connections := manager.GetConnections()
	if len(connections) != 3 {
		t.Errorf("expected 3 connections, got %d", len(connections))
	}
}

func TestConnectionManager_GetConnectionsByMock(t *testing.T) {
	manager := NewConnectionManager(100)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Register for mock-1
	for i := 0; i < 2; i++ {
		stream := &SSEStream{
			ID:     formatStreamID(int64(i)),
			MockID: "mock-1",
			ctx:    ctx,
			cancel: cancel,
		}
		_ = manager.Register(stream)
	}

	// Register for mock-2
	stream := &SSEStream{
		ID:     "test-mock2",
		MockID: "mock-2",
		ctx:    ctx,
		cancel: cancel,
	}
	_ = manager.Register(stream)

	// Get mock-1 connections
	mock1Conns := manager.GetConnectionsByMock("mock-1")
	if len(mock1Conns) != 2 {
		t.Errorf("expected 2 connections for mock-1, got %d", len(mock1Conns))
	}

	// Get mock-2 connections
	mock2Conns := manager.GetConnectionsByMock("mock-2")
	if len(mock2Conns) != 1 {
		t.Errorf("expected 1 connection for mock-2, got %d", len(mock2Conns))
	}
}

func TestConnectionManager_CountByMock(t *testing.T) {
	manager := NewConnectionManager(100)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Register for mock-1
	for i := 0; i < 3; i++ {
		stream := &SSEStream{
			ID:     formatStreamID(int64(i)),
			MockID: "mock-1",
			ctx:    ctx,
			cancel: cancel,
		}
		_ = manager.Register(stream)
	}

	count := manager.CountByMock("mock-1")
	if count != 3 {
		t.Errorf("expected count 3, got %d", count)
	}

	count = manager.CountByMock("mock-2")
	if count != 0 {
		t.Errorf("expected count 0 for mock-2, got %d", count)
	}
}

func TestConnectionManager_Close(t *testing.T) {
	manager := NewConnectionManager(100)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	stream := &SSEStream{
		ID:     "test-1",
		MockID: "mock-1",
		ctx:    ctx,
		cancel: cancel,
	}
	_ = manager.Register(stream)

	err := manager.Close("test-1", true, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Try to close non-existing
	err = manager.Close("non-existing", true, nil)
	if err != ErrStreamClosed {
		t.Errorf("expected ErrStreamClosed, got %v", err)
	}
}

func TestConnectionManager_CloseAll(t *testing.T) {
	manager := NewConnectionManager(100)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Register multiple
	for i := 0; i < 5; i++ {
		stream := &SSEStream{
			ID:     formatStreamID(int64(i)),
			MockID: "mock-1",
			ctx:    ctx,
			cancel: cancel,
		}
		_ = manager.Register(stream)
	}

	manager.CloseAll()

	if manager.Count() != 0 {
		t.Errorf("expected count 0 after CloseAll, got %d", manager.Count())
	}
}

func TestConnectionManager_CloseByMock(t *testing.T) {
	manager := NewConnectionManager(100)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Register for mock-1
	for i := 0; i < 3; i++ {
		stream := &SSEStream{
			ID:     formatStreamID(int64(i)),
			MockID: "mock-1",
			ctx:    ctx,
			cancel: cancel,
		}
		_ = manager.Register(stream)
	}

	// Register for mock-2
	stream := &SSEStream{
		ID:     "mock2-stream",
		MockID: "mock-2",
		ctx:    ctx,
		cancel: cancel,
	}
	_ = manager.Register(stream)

	// Close mock-1 connections
	closed := manager.CloseByMock("mock-1")
	if closed != 3 {
		t.Errorf("expected 3 closed, got %d", closed)
	}

	if manager.Count() != 1 {
		t.Errorf("expected count 1, got %d", manager.Count())
	}

	if manager.CountByMock("mock-2") != 1 {
		t.Error("expected mock-2 connection to still exist")
	}
}

func TestConnectionManager_Stats(t *testing.T) {
	manager := NewConnectionManager(100)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Register for different mocks
	for i := 0; i < 2; i++ {
		stream := &SSEStream{
			ID:     formatStreamID(int64(i)),
			MockID: "mock-1",
			ctx:    ctx,
			cancel: cancel,
		}
		_ = manager.Register(stream)
	}

	stream := &SSEStream{
		ID:     "mock2-stream",
		MockID: "mock-2",
		ctx:    ctx,
		cancel: cancel,
	}
	_ = manager.Register(stream)

	stats := manager.Stats()

	if stats.ActiveConnections != 3 {
		t.Errorf("expected 3 active connections, got %d", stats.ActiveConnections)
	}
	if stats.TotalConnections != 3 {
		t.Errorf("expected 3 total connections, got %d", stats.TotalConnections)
	}
	if stats.ConnectionsByMock["mock-1"] != 2 {
		t.Errorf("expected 2 connections for mock-1, got %d", stats.ConnectionsByMock["mock-1"])
	}
	if stats.ConnectionsByMock["mock-2"] != 1 {
		t.Errorf("expected 1 connection for mock-2, got %d", stats.ConnectionsByMock["mock-2"])
	}
}

func TestConnectionManager_GetConnectionInfo(t *testing.T) {
	manager := NewConnectionManager(100)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	now := time.Now()
	stream := &SSEStream{
		ID:         "test-1",
		MockID:     "mock-1",
		ClientIP:   "192.168.1.1",
		StartTime:  now,
		EventsSent: 42,
		Status:     StreamStatusActive,
		ctx:        ctx,
		cancel:     cancel,
	}
	_ = manager.Register(stream)

	info := manager.GetConnectionInfo()
	if len(info) != 1 {
		t.Fatalf("expected 1 info entry, got %d", len(info))
	}

	if info[0].ID != "test-1" {
		t.Errorf("expected ID 'test-1', got %q", info[0].ID)
	}
	if info[0].MockID != "mock-1" {
		t.Errorf("expected MockID 'mock-1', got %q", info[0].MockID)
	}
	if info[0].ClientIP != "192.168.1.1" {
		t.Errorf("expected ClientIP '192.168.1.1', got %q", info[0].ClientIP)
	}
	if info[0].EventsSent != 42 {
		t.Errorf("expected EventsSent 42, got %d", info[0].EventsSent)
	}
	if info[0].Status != StreamStatusActive {
		t.Errorf("expected status Active, got %v", info[0].Status)
	}
}

func TestConnectionManager_ConcurrentAccess(t *testing.T) {
	manager := NewConnectionManager(1000)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Concurrent registrations
	done := make(chan bool, 100)
	for i := 0; i < 100; i++ {
		go func(id int) {
			stream := &SSEStream{
				ID:     formatStreamID(int64(id)),
				MockID: "mock-1",
				ctx:    ctx,
				cancel: cancel,
			}
			_ = manager.Register(stream)
			done <- true
		}(i)
	}

	// Wait for all
	for i := 0; i < 100; i++ {
		<-done
	}

	if manager.Count() != 100 {
		t.Errorf("expected count 100, got %d", manager.Count())
	}

	// Concurrent deregistrations
	for i := 0; i < 100; i++ {
		go func(id int) {
			manager.Deregister(formatStreamID(int64(id)))
			done <- true
		}(i)
	}

	// Wait for all
	for i := 0; i < 100; i++ {
		<-done
	}

	if manager.Count() != 0 {
		t.Errorf("expected count 0, got %d", manager.Count())
	}
}

func TestStreamIterator(t *testing.T) {
	manager := NewConnectionManager(100)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Register some streams
	for i := 0; i < 5; i++ {
		stream := &SSEStream{
			ID:     formatStreamID(int64(i)),
			MockID: "mock-1",
			ctx:    ctx,
			cancel: cancel,
		}
		_ = manager.Register(stream)
	}

	// Use iterator
	iter := manager.NewIterator()
	count := 0
	for iter.HasNext() {
		stream := iter.Next()
		if stream == nil {
			t.Error("expected non-nil stream")
		}
		count++
	}

	if count != 5 {
		t.Errorf("expected to iterate 5 streams, got %d", count)
	}

	// Test reset
	iter.Reset()
	if !iter.HasNext() {
		t.Error("expected HasNext to be true after reset")
	}
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		duration time.Duration
		expected string
	}{
		{500 * time.Millisecond, "500ms"},
		{1500 * time.Millisecond, "1.5s"},
		{30 * time.Second, "30s"},
		{90 * time.Second, "1.5m"},
		{30 * time.Minute, "30m"},
		{90 * time.Minute, "1.5h"},
		{2 * time.Hour, "2h"},
	}

	for _, tc := range tests {
		result := formatDuration(tc.duration)
		if result != tc.expected {
			t.Errorf("formatDuration(%v) = %q, expected %q", tc.duration, result, tc.expected)
		}
	}
}
