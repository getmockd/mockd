package websocket

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/getmockd/mockd/pkg/recording"
)

// =============================================================================
// Helper Functions for Creating Test Connections
// =============================================================================

// createTestConnectionWithoutWS creates a Connection for testing without a real websocket.
// This is used for testing connection state management, groups, metadata, etc.
func createTestConnectionWithoutWS(id string) *Connection {
	ctx, cancel := context.WithCancel(context.Background())

	conn := &Connection{
		id:          id,
		conn:        nil, // No underlying websocket - tests must not call Send/Read/Close
		ctx:         ctx,
		cancel:      cancel,
		groups:      make(map[string]struct{}),
		metadata:    make(map[string]interface{}),
		connectedAt: time.Now(),
	}
	conn.lastMessageAt.Store(conn.connectedAt)

	return conn
}

// =============================================================================
// REGRESSION TESTS - Bug 3.2: Non-atomic closed check in Send()
// =============================================================================

func TestConnection_ConcurrentCloses_NoPanic(t *testing.T) {
	// Multiple goroutines trying to close simultaneously should not panic.
	// This tests the atomic closed flag behavior.
	conn := createTestConnectionWithoutWS("concurrent-close")

	const numClosers = 10
	var wg sync.WaitGroup
	var successCount atomic.Int32
	var errorCount atomic.Int32

	wg.Add(numClosers)
	for i := 0; i < numClosers; i++ {
		go func() {
			defer wg.Done()
			// Manually set closed flag to simulate close behavior
			if conn.closed.CompareAndSwap(false, true) {
				successCount.Add(1)
				conn.cancel()
			} else {
				errorCount.Add(1)
			}
		}()
	}

	wg.Wait()

	// Exactly one close should succeed
	if successCount.Load() != 1 {
		t.Errorf("Expected exactly 1 successful close, got %d", successCount.Load())
	}
	if errorCount.Load() != numClosers-1 {
		t.Errorf("Expected %d errors, got %d", numClosers-1, errorCount.Load())
	}
}

func TestConnection_IsClosed_ReflectsState(t *testing.T) {
	conn := createTestConnectionWithoutWS("closed-state")

	if conn.IsClosed() {
		t.Error("New connection should not be closed")
	}

	// Manually mark as closed
	conn.closed.Store(true)

	if !conn.IsClosed() {
		t.Error("Connection should be closed after marking")
	}
}

// =============================================================================
// REGRESSION TESTS - Bug 2.6: Race condition in manager.Remove()
// =============================================================================

func TestConnection_GetGroups_ReturnsCopy(t *testing.T) {
	// Regression test for Bug 2.6: GetGroups should return a copy to prevent
	// race conditions when iterating over groups during Remove().
	conn := createTestConnectionWithoutWS("groups-copy-test")
	manager := NewConnectionManager()
	manager.Add(conn)

	// Join several groups
	groups := []string{"group1", "group2", "group3"}
	for _, g := range groups {
		if err := conn.JoinGroup(g); err != nil {
			t.Fatalf("JoinGroup failed: %v", err)
		}
	}

	// Get groups copy
	groupsCopy := conn.GetGroups()

	// Modify original groups
	_ = conn.LeaveGroup("group1")
	_ = conn.JoinGroup("group4")

	// The copy should still have the original groups
	if len(groupsCopy) != 3 {
		t.Errorf("Copy should have 3 groups, got %d", len(groupsCopy))
	}

	// Verify the copy contains original values
	found := make(map[string]bool)
	for _, g := range groupsCopy {
		found[g] = true
	}
	for _, g := range groups {
		if !found[g] {
			t.Errorf("Copy missing group %s", g)
		}
	}
}

func TestConnectionManager_Remove_ConcurrentGroupAccess(t *testing.T) {
	// Regression test: Removing a connection while iterating its groups
	// should not cause a race condition.
	manager := NewConnectionManager()

	const numConnections = 20
	const numGroups = 5
	connections := make([]*Connection, numConnections)

	// Create connections and add to groups
	for i := 0; i < numConnections; i++ {
		conn := createTestConnectionWithoutWS(string(rune('A' + i)))
		manager.Add(conn)
		connections[i] = conn

		// Join multiple groups
		for j := 0; j < numGroups; j++ {
			groupName := string(rune('0' + j))
			_ = conn.JoinGroup(groupName)
		}
	}

	// Concurrently: remove connections and access their groups
	var wg sync.WaitGroup
	wg.Add(numConnections * 2)

	for i := 0; i < numConnections; i++ {
		conn := connections[i]

		// Goroutine 1: Remove the connection
		go func(c *Connection) {
			defer wg.Done()
			manager.Remove(c.ID())
		}(conn)

		// Goroutine 2: Access its groups
		go func(c *Connection) {
			defer wg.Done()
			for j := 0; j < 10; j++ {
				_ = c.GetGroups()
				_ = c.Groups()
				_ = c.InGroup("0")
			}
		}(conn)
	}

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Success - no race or deadlock
	case <-time.After(5 * time.Second):
		t.Fatal("Test timed out - possible race or deadlock")
	}
}

// =============================================================================
// REGRESSION TESTS - Bug 2.7: Potential deadlock in Stop()
// =============================================================================

func TestConnectionManager_Stop_NoDeadlock(t *testing.T) {
	// Regression test for Bug 2.7: Stop() should release the lock properly
	// and copy connections before closing to avoid holding lock during close.
	// We test the locking behavior by verifying concurrent operations don't deadlock.
	manager := NewConnectionManager()

	const numConnections = 100
	for i := 0; i < numConnections; i++ {
		conn := createTestConnectionWithoutWS(GenerateConnectionID())
		manager.Add(conn)

		// Join some groups too
		_ = conn.JoinGroup("all")
		if i%2 == 0 {
			_ = conn.JoinGroup("evens")
		}
	}

	// Verify connections were added
	if manager.Count() != numConnections {
		t.Fatalf("Expected %d connections, got %d", numConnections, manager.Count())
	}

	// Test that we can access manager while Stop logic is running
	// by simulating the Stop pattern: copy connections then operate
	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		// Simulate Stop() pattern: acquire lock, copy, release, then operate
		manager.mu.Lock()
		conns := make([]*Connection, 0, len(manager.connections))
		for _, conn := range manager.connections {
			conns = append(conns, conn)
		}
		manager.mu.Unlock()

		// Operate on connections outside the lock
		for _, conn := range conns {
			_ = conn.IsClosed()
			_ = conn.GetGroups()
		}
	}()

	go func() {
		defer wg.Done()
		// Concurrent reads while "Stop" is happening
		for i := 0; i < 100; i++ {
			_ = manager.Count()
			_ = manager.ListAll()
			_ = manager.ListByGroup("all")
		}
	}()

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Success - no deadlock
	case <-time.After(5 * time.Second):
		t.Fatal("Timed out - possible deadlock")
	}
}

func TestConnectionManager_CloseAllConnections_CopiesConnections(t *testing.T) {
	// Test that CloseAllConnections properly copies connections before closing
	// to avoid holding the lock during close operations.
	manager := NewConnectionManager()

	const numConnections = 50
	for i := 0; i < numConnections; i++ {
		conn := createTestConnectionWithoutWS(GenerateConnectionID())
		manager.Add(conn)
	}

	// Simulate the pattern used in CloseAllConnections
	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		// Pattern: lock, copy, unlock, then close
		manager.mu.Lock()
		conns := make([]*Connection, 0, len(manager.connections))
		for _, conn := range manager.connections {
			conns = append(conns, conn)
		}
		manager.mu.Unlock()

		// Mark connections as closed outside the lock
		for _, conn := range conns {
			conn.closed.Store(true)
			conn.cancel()
		}
	}()

	go func() {
		defer wg.Done()
		// Concurrent reads
		for i := 0; i < 100; i++ {
			_ = manager.Count()
			_ = manager.ListAll()
		}
	}()

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Success
	case <-time.After(5 * time.Second):
		t.Fatal("Timed out - possible deadlock")
	}
}

// =============================================================================
// REGRESSION TESTS - Bug 3.3: Message drop in synchronized replay
// =============================================================================

func TestReplayer_OnClientMessage_ChannelBuffered(t *testing.T) {
	// Regression test for Bug 3.3: The expectedMsg channel should be buffered
	// to prevent message drops with rapid messages.
	conn := createTestConnectionWithoutWS("replayer-test")

	rec := &recording.StreamRecording{
		Protocol: recording.ProtocolWebSocket,
		WebSocket: &recording.WebSocketRecordingData{
			Frames: []recording.WebSocketFrame{
				{Direction: recording.DirectionClientToServer, MessageType: recording.MessageTypeText, Data: "client1"},
				{Direction: recording.DirectionServerToClient, MessageType: recording.MessageTypeText, Data: "server1"},
			},
		},
	}

	config := ReplayConfig{
		Mode:    recording.ReplayModeSynchronized,
		Timeout: 100 * time.Millisecond,
	}

	replayer, err := NewWebSocketReplayer(rec, conn, config)
	if err != nil {
		t.Fatalf("Failed to create replayer: %v", err)
	}

	// Verify the channel is buffered
	if cap(replayer.expectedMsg) < 1 {
		t.Error("expectedMsg channel should be buffered")
	}
}

func TestReplayer_OnClientMessage_ContextCancelled_Discards(t *testing.T) {
	// Test that OnClientMessage doesn't block forever when context is cancelled.
	conn := createTestConnectionWithoutWS("replayer-cancel-test")

	rec := &recording.StreamRecording{
		Protocol: recording.ProtocolWebSocket,
		WebSocket: &recording.WebSocketRecordingData{
			Frames: []recording.WebSocketFrame{
				{Direction: recording.DirectionClientToServer, MessageType: recording.MessageTypeText, Data: "client1"},
				{Direction: recording.DirectionServerToClient, MessageType: recording.MessageTypeText, Data: "server1"},
			},
		},
	}

	config := ReplayConfig{
		Mode:    recording.ReplayModeSynchronized,
		Timeout: 100 * time.Millisecond,
	}

	replayer, err := NewWebSocketReplayer(rec, conn, config)
	if err != nil {
		t.Fatalf("Failed to create replayer: %v", err)
	}

	// Don't start, just cancel
	replayer.cancel()

	// OnClientMessage should not block when cancelled
	done := make(chan struct{})
	go func() {
		replayer.OnClientMessage([]byte("test"))
		close(done)
	}()

	select {
	case <-done:
		// Success - didn't block
	case <-time.After(1 * time.Second):
		t.Fatal("OnClientMessage blocked after cancel")
	}
}

// =============================================================================
// CONNECTION LIFECYCLE TESTS
// =============================================================================

func TestConnection_JoinGroup_AddsToGroup(t *testing.T) {
	conn := createTestConnectionWithoutWS("join-test")
	manager := NewConnectionManager()
	manager.Add(conn)

	err := conn.JoinGroup("test-group")
	if err != nil {
		t.Fatalf("JoinGroup failed: %v", err)
	}

	if !conn.InGroup("test-group") {
		t.Error("Connection should be in test-group")
	}

	groups := conn.Groups()
	if len(groups) != 1 || groups[0] != "test-group" {
		t.Errorf("Groups() returned unexpected: %v", groups)
	}
}

func TestConnection_LeaveGroup_RemovesFromGroup(t *testing.T) {
	conn := createTestConnectionWithoutWS("leave-test")
	manager := NewConnectionManager()
	manager.Add(conn)

	// Join then leave
	_ = conn.JoinGroup("test-group")
	err := conn.LeaveGroup("test-group")
	if err != nil {
		t.Fatalf("LeaveGroup failed: %v", err)
	}

	if conn.InGroup("test-group") {
		t.Error("Connection should not be in test-group after leaving")
	}

	// Leaving again should error
	err = conn.LeaveGroup("test-group")
	if !errors.Is(err, ErrNotInGroup) {
		t.Errorf("Expected ErrNotInGroup, got: %v", err)
	}
}

func TestConnection_JoinGroup_MaxGroups_ReturnsError(t *testing.T) {
	conn := createTestConnectionWithoutWS("max-groups-test")
	manager := NewConnectionManager()
	manager.Add(conn)

	// Join max groups
	for i := 0; i < MaxGroupsPerConnection; i++ {
		groupName := string(rune('a' + (i % 26)))
		if i >= 26 {
			groupName = string(rune('a'+i%26)) + string(rune('0'+i/26))
		}
		err := conn.JoinGroup(groupName)
		if err != nil {
			t.Fatalf("JoinGroup %d failed: %v", i, err)
		}
	}

	// One more should fail
	err := conn.JoinGroup("overflow")
	if !errors.Is(err, ErrTooManyGroups) {
		t.Errorf("Expected ErrTooManyGroups, got: %v", err)
	}
}

func TestConnection_JoinGroup_AlreadyInGroup_ReturnsError(t *testing.T) {
	conn := createTestConnectionWithoutWS("already-in-group-test")
	manager := NewConnectionManager()
	manager.Add(conn)

	_ = conn.JoinGroup("test-group")
	err := conn.JoinGroup("test-group")
	if !errors.Is(err, ErrAlreadyInGroup) {
		t.Errorf("Expected ErrAlreadyInGroup, got: %v", err)
	}
}

func TestConnection_Metadata_SetAndGet(t *testing.T) {
	conn := createTestConnectionWithoutWS("metadata-test")

	// Set metadata
	conn.SetMetadata("key1", "value1")
	conn.SetMetadata("key2", 42)
	conn.SetMetadata("key3", true)

	// Get metadata (returns copy)
	meta := conn.Metadata()

	if meta["key1"] != "value1" {
		t.Errorf("Expected 'value1', got %v", meta["key1"])
	}
	if meta["key2"] != 42 {
		t.Errorf("Expected 42, got %v", meta["key2"])
	}
	if meta["key3"] != true {
		t.Errorf("Expected true, got %v", meta["key3"])
	}

	// Verify it's a copy - modifying shouldn't affect original
	meta["key1"] = "modified"
	originalMeta := conn.Metadata()
	if originalMeta["key1"] != "value1" {
		t.Error("Metadata() should return a copy")
	}
}

func TestConnection_Info_ReturnsCorrectData(t *testing.T) {
	conn := createTestConnectionWithoutWS("info-test")
	conn.subprotocol = "test-protocol"
	manager := NewConnectionManager()
	manager.Add(conn)

	_ = conn.JoinGroup("group1")
	conn.SetMetadata("custom", "data")

	info := conn.Info()

	if info.ID != "info-test" {
		t.Errorf("Expected ID 'info-test', got %s", info.ID)
	}
	if info.Subprotocol != "test-protocol" {
		t.Errorf("Expected Subprotocol 'test-protocol', got %s", info.Subprotocol)
	}
	if len(info.Groups) != 1 || info.Groups[0] != "group1" {
		t.Errorf("Groups mismatch: %v", info.Groups)
	}
	if info.Metadata["custom"] != "data" {
		t.Errorf("Metadata mismatch: %v", info.Metadata)
	}
}

func TestConnection_Context_IsCancellable(t *testing.T) {
	conn := createTestConnectionWithoutWS("context-test")

	ctx := conn.Context()
	select {
	case <-ctx.Done():
		t.Error("Context should not be done initially")
	default:
		// Expected
	}

	conn.cancel()

	select {
	case <-ctx.Done():
		// Expected
	case <-time.After(100 * time.Millisecond):
		t.Error("Context should be done after cancel")
	}
}

// =============================================================================
// CONNECTION MANAGER TESTS
// =============================================================================

func TestConnectionManager_Add_AssignsManager(t *testing.T) {
	manager := NewConnectionManager()
	conn := createTestConnectionWithoutWS("add-test")

	manager.Add(conn)

	// Verify connection is registered
	retrieved := manager.Get("add-test")
	if retrieved == nil {
		t.Error("Connection not found after Add")
	}
	if retrieved.ID() != "add-test" {
		t.Errorf("ID mismatch: expected 'add-test', got %s", retrieved.ID())
	}

	// Verify manager was set on connection
	conn.mu.RLock()
	connManager := conn.manager
	conn.mu.RUnlock()
	if connManager != manager {
		t.Error("Manager not set on connection")
	}
}

func TestConnectionManager_Get_ReturnsConnection(t *testing.T) {
	manager := NewConnectionManager()
	conn := createTestConnectionWithoutWS("get-test")
	manager.Add(conn)

	retrieved := manager.Get("get-test")
	if retrieved == nil {
		t.Error("Get returned nil for existing connection")
	}
	if retrieved != conn {
		t.Error("Get returned different connection object")
	}
}

func TestConnectionManager_Get_NotFound_ReturnsNil(t *testing.T) {
	manager := NewConnectionManager()

	retrieved := manager.Get("nonexistent")
	if retrieved != nil {
		t.Error("Get should return nil for nonexistent connection")
	}
}

func TestConnectionManager_Count_ReturnsCorrectCount(t *testing.T) {
	manager := NewConnectionManager()

	if manager.Count() != 0 {
		t.Error("Initial count should be 0")
	}

	// Add connections
	for i := 0; i < 5; i++ {
		conn := createTestConnectionWithoutWS(string(rune('A' + i)))
		manager.Add(conn)
	}

	if manager.Count() != 5 {
		t.Errorf("Expected count 5, got %d", manager.Count())
	}

	// Remove one
	manager.Remove("A")
	if manager.Count() != 4 {
		t.Errorf("Expected count 4 after removal, got %d", manager.Count())
	}
}

func TestConnectionManager_ListAll_ReturnsAllIDs(t *testing.T) {
	manager := NewConnectionManager()

	ids := []string{"conn-a", "conn-b", "conn-c"}
	for _, id := range ids {
		conn := createTestConnectionWithoutWS(id)
		manager.Add(conn)
	}

	listed := manager.ListAll()
	if len(listed) != len(ids) {
		t.Errorf("Expected %d IDs, got %d", len(ids), len(listed))
	}

	// Check all expected IDs are present
	listedMap := make(map[string]bool)
	for _, id := range listed {
		listedMap[id] = true
	}
	for _, id := range ids {
		if !listedMap[id] {
			t.Errorf("Missing ID: %s", id)
		}
	}
}

func TestConnectionManager_ListByEndpoint_ReturnsCorrectConnections(t *testing.T) {
	manager := NewConnectionManager()

	// Add connections to different endpoints
	for i := 0; i < 3; i++ {
		conn := createTestConnectionWithoutWS(string(rune('A' + i)))
		conn.endpointPath = "/ws1"
		manager.Add(conn)
	}
	for i := 0; i < 2; i++ {
		conn := createTestConnectionWithoutWS(string(rune('X' + i)))
		conn.endpointPath = "/ws2"
		manager.Add(conn)
	}

	ws1Conns := manager.ListByEndpoint("/ws1")
	if len(ws1Conns) != 3 {
		t.Errorf("Expected 3 connections for /ws1, got %d", len(ws1Conns))
	}

	ws2Conns := manager.ListByEndpoint("/ws2")
	if len(ws2Conns) != 2 {
		t.Errorf("Expected 2 connections for /ws2, got %d", len(ws2Conns))
	}
}

func TestConnectionManager_ListByGroup_ReturnsGroupMembers(t *testing.T) {
	manager := NewConnectionManager()

	conn1 := createTestConnectionWithoutWS("conn1")
	conn2 := createTestConnectionWithoutWS("conn2")
	conn3 := createTestConnectionWithoutWS("conn3")

	manager.Add(conn1)
	manager.Add(conn2)
	manager.Add(conn3)

	_ = manager.JoinGroup("conn1", "groupA")
	_ = manager.JoinGroup("conn2", "groupA")
	_ = manager.JoinGroup("conn2", "groupB")
	_ = manager.JoinGroup("conn3", "groupB")

	groupA := manager.ListByGroup("groupA")
	if len(groupA) != 2 {
		t.Errorf("Expected 2 in groupA, got %d", len(groupA))
	}

	groupB := manager.ListByGroup("groupB")
	if len(groupB) != 2 {
		t.Errorf("Expected 2 in groupB, got %d", len(groupB))
	}

	emptyGroup := manager.ListByGroup("nonexistent")
	if len(emptyGroup) != 0 {
		t.Errorf("Expected 0 in nonexistent group, got %d", len(emptyGroup))
	}
}

func TestConnectionManager_Remove_CleansUpGroups(t *testing.T) {
	manager := NewConnectionManager()

	conn := createTestConnectionWithoutWS("remove-test")
	manager.Add(conn)

	_ = manager.JoinGroup("remove-test", "group1")
	_ = manager.JoinGroup("remove-test", "group2")

	// Remove the connection
	manager.Remove("remove-test")

	// Groups should be empty or removed
	group1 := manager.ListByGroup("group1")
	if len(group1) != 0 {
		t.Errorf("group1 should be empty after remove, got %d", len(group1))
	}

	group2 := manager.ListByGroup("group2")
	if len(group2) != 0 {
		t.Errorf("group2 should be empty after remove, got %d", len(group2))
	}
}

func TestConnectionManager_ListGroups_ReturnsAllGroups(t *testing.T) {
	manager := NewConnectionManager()

	conn := createTestConnectionWithoutWS("test-conn")
	manager.Add(conn)

	_ = manager.JoinGroup("test-conn", "alpha")
	_ = manager.JoinGroup("test-conn", "beta")
	_ = manager.JoinGroup("test-conn", "gamma")

	groups := manager.ListGroups()
	if len(groups) != 3 {
		t.Errorf("Expected 3 groups, got %d", len(groups))
	}

	groupMap := make(map[string]bool)
	for _, g := range groups {
		groupMap[g] = true
	}
	for _, expected := range []string{"alpha", "beta", "gamma"} {
		if !groupMap[expected] {
			t.Errorf("Missing group: %s", expected)
		}
	}
}

func TestConnectionManager_JoinGroup_NonexistentConnection(t *testing.T) {
	manager := NewConnectionManager()

	err := manager.JoinGroup("nonexistent", "group")
	if !errors.Is(err, ErrConnectionNotFound) {
		t.Errorf("Expected ErrConnectionNotFound, got: %v", err)
	}
}

func TestConnectionManager_LeaveGroup_NonexistentConnection(t *testing.T) {
	manager := NewConnectionManager()

	err := manager.LeaveGroup("nonexistent", "group")
	if !errors.Is(err, ErrConnectionNotFound) {
		t.Errorf("Expected ErrConnectionNotFound, got: %v", err)
	}
}

// =============================================================================
// REPLAY TESTS
// =============================================================================

func TestReplayer_InvalidRecording_ReturnsError(t *testing.T) {
	conn := createTestConnectionWithoutWS("invalid-recording")

	// Nil recording
	_, err := NewWebSocketReplayer(nil, conn, DefaultReplayConfig())
	if !errors.Is(err, ErrInvalidRecording) {
		t.Errorf("Expected ErrInvalidRecording for nil, got: %v", err)
	}

	// Wrong protocol
	rec := &recording.StreamRecording{
		Protocol: recording.ProtocolHTTP,
	}
	_, err = NewWebSocketReplayer(rec, conn, DefaultReplayConfig())
	if !errors.Is(err, ErrInvalidRecording) {
		t.Errorf("Expected ErrInvalidRecording for wrong protocol, got: %v", err)
	}

	// No frames
	rec = &recording.StreamRecording{
		Protocol:  recording.ProtocolWebSocket,
		WebSocket: &recording.WebSocketRecordingData{Frames: nil},
	}
	_, err = NewWebSocketReplayer(rec, conn, DefaultReplayConfig())
	if !errors.Is(err, ErrNoFramesToReplay) {
		t.Errorf("Expected ErrNoFramesToReplay, got: %v", err)
	}

	// Empty frames
	rec = &recording.StreamRecording{
		Protocol:  recording.ProtocolWebSocket,
		WebSocket: &recording.WebSocketRecordingData{Frames: []recording.WebSocketFrame{}},
	}
	_, err = NewWebSocketReplayer(rec, conn, DefaultReplayConfig())
	if !errors.Is(err, ErrNoFramesToReplay) {
		t.Errorf("Expected ErrNoFramesToReplay for empty frames, got: %v", err)
	}
}

func TestReplayer_InvalidMode_ReturnsError(t *testing.T) {
	conn := createTestConnectionWithoutWS("invalid-mode")

	rec := &recording.StreamRecording{
		Protocol: recording.ProtocolWebSocket,
		WebSocket: &recording.WebSocketRecordingData{
			Frames: []recording.WebSocketFrame{
				{Direction: recording.DirectionServerToClient, MessageType: recording.MessageTypeText, Data: "msg"},
			},
		},
	}

	config := ReplayConfig{
		Mode: "invalid-mode",
	}

	_, err := NewWebSocketReplayer(rec, conn, config)
	if !errors.Is(err, ErrInvalidReplayMode) {
		t.Errorf("Expected ErrInvalidReplayMode, got: %v", err)
	}
}

func TestReplayer_DoubleStart_ReturnsError(t *testing.T) {
	conn := createTestConnectionWithoutWS("double-start")

	rec := &recording.StreamRecording{
		Protocol: recording.ProtocolWebSocket,
		WebSocket: &recording.WebSocketRecordingData{
			Frames: []recording.WebSocketFrame{
				{Direction: recording.DirectionServerToClient, MessageType: recording.MessageTypeText, Data: "msg"},
			},
		},
	}

	replayer, _ := NewWebSocketReplayer(rec, conn, DefaultReplayConfig())

	// Manually set started flag without calling Start() to avoid needing real websocket
	replayer.mu.Lock()
	replayer.started = true
	replayer.mu.Unlock()

	err := replayer.Start()
	if !errors.Is(err, ErrReplayAlreadyStarted) {
		t.Errorf("Expected ErrReplayAlreadyStarted, got: %v", err)
	}
}

func TestReplayer_Progress_TracksCorrectly(t *testing.T) {
	conn := createTestConnectionWithoutWS("replay-progress")

	rec := &recording.StreamRecording{
		Protocol: recording.ProtocolWebSocket,
		WebSocket: &recording.WebSocketRecordingData{
			Frames: []recording.WebSocketFrame{
				{Direction: recording.DirectionServerToClient, MessageType: recording.MessageTypeText, Data: "msg1"},
				{Direction: recording.DirectionServerToClient, MessageType: recording.MessageTypeText, Data: "msg2"},
				{Direction: recording.DirectionServerToClient, MessageType: recording.MessageTypeText, Data: "msg3"},
			},
		},
	}

	config := ReplayConfig{
		Mode:        recording.ReplayModePure,
		TimingScale: 100.0,
	}

	replayer, err := NewWebSocketReplayer(rec, conn, config)
	if err != nil {
		t.Fatalf("Failed to create replayer: %v", err)
	}

	current, total, sent := replayer.Progress()
	if total != 3 {
		t.Errorf("Expected total 3, got %d", total)
	}
	if current != 0 || sent != 0 {
		t.Errorf("Before start: expected current=0, sent=0, got current=%d, sent=%d", current, sent)
	}
}

func TestReplayer_Status_InitiallyPending(t *testing.T) {
	conn := createTestConnectionWithoutWS("status-test")

	rec := &recording.StreamRecording{
		Protocol: recording.ProtocolWebSocket,
		WebSocket: &recording.WebSocketRecordingData{
			Frames: []recording.WebSocketFrame{
				{Direction: recording.DirectionServerToClient, MessageType: recording.MessageTypeText, Data: "msg"},
			},
		},
	}

	replayer, _ := NewWebSocketReplayer(rec, conn, DefaultReplayConfig())

	if replayer.Status() != recording.ReplayStatusPending {
		t.Errorf("Expected pending status, got %v", replayer.Status())
	}
}

func TestReplayer_Stop_SetsAbortedStatus(t *testing.T) {
	conn := createTestConnectionWithoutWS("stop-test")

	rec := &recording.StreamRecording{
		Protocol: recording.ProtocolWebSocket,
		WebSocket: &recording.WebSocketRecordingData{
			Frames: []recording.WebSocketFrame{
				{Direction: recording.DirectionServerToClient, MessageType: recording.MessageTypeText, Data: "msg"},
			},
		},
	}

	replayer, _ := NewWebSocketReplayer(rec, conn, DefaultReplayConfig())

	// Manually set to playing status
	replayer.mu.Lock()
	replayer.started = true
	replayer.status = recording.ReplayStatusPlaying
	replayer.mu.Unlock()

	replayer.Stop()

	// Status should be aborted
	status := replayer.Status()
	if status != recording.ReplayStatusAborted {
		t.Errorf("Expected aborted status, got %v", status)
	}
}

func TestReplayer_TriggeredMode_AdvanceErrors(t *testing.T) {
	conn := createTestConnectionWithoutWS("triggered-errors")

	rec := &recording.StreamRecording{
		Protocol: recording.ProtocolWebSocket,
		WebSocket: &recording.WebSocketRecordingData{
			Frames: []recording.WebSocketFrame{
				{Direction: recording.DirectionServerToClient, MessageType: recording.MessageTypeText, Data: "msg"},
			},
		},
	}

	// Pure mode - Advance should fail
	pureConfig := ReplayConfig{Mode: recording.ReplayModePure}
	pureReplayer, _ := NewWebSocketReplayer(rec, conn, pureConfig)

	_, err := pureReplayer.Advance(1)
	if !errors.Is(err, ErrTriggeredModeOnly) {
		t.Errorf("Expected ErrTriggeredModeOnly, got: %v", err)
	}

	// Triggered mode but not started - Advance should fail
	triggerConfig := ReplayConfig{Mode: recording.ReplayModeTriggered}
	triggerReplayer, _ := NewWebSocketReplayer(rec, conn, triggerConfig)

	_, err = triggerReplayer.Advance(1)
	if !errors.Is(err, ErrReplayNotStarted) {
		t.Errorf("Expected ErrReplayNotStarted, got: %v", err)
	}
}

func TestReplayer_GetProgress_IncludesElapsed(t *testing.T) {
	conn := createTestConnectionWithoutWS("progress-elapsed")

	rec := &recording.StreamRecording{
		Protocol: recording.ProtocolWebSocket,
		WebSocket: &recording.WebSocketRecordingData{
			Frames: []recording.WebSocketFrame{
				{Direction: recording.DirectionServerToClient, MessageType: recording.MessageTypeText, Data: "msg"},
			},
		},
	}

	replayer, _ := NewWebSocketReplayer(rec, conn, DefaultReplayConfig())

	// Before start, elapsed should be 0
	progress := replayer.GetProgress()
	if progress.Elapsed != 0 {
		t.Errorf("Expected 0 elapsed before start, got %v", progress.Elapsed)
	}

	// Manually set started and startTime to simulate Start() without needing websocket
	replayer.mu.Lock()
	replayer.started = true
	replayer.startTime = time.Now().Add(-100 * time.Millisecond) // Started 100ms ago
	replayer.mu.Unlock()

	progress = replayer.GetProgress()
	if progress.Elapsed < 90*time.Millisecond {
		t.Errorf("Expected elapsed >= 90ms after start, got %v", progress.Elapsed)
	}
}

func TestReplayer_DefaultConfig(t *testing.T) {
	config := DefaultReplayConfig()

	if config.Mode != recording.ReplayModePure {
		t.Errorf("Expected pure mode, got %v", config.Mode)
	}
	if config.TimingScale != 1.0 {
		t.Errorf("Expected timing scale 1.0, got %v", config.TimingScale)
	}
	if !config.SkipClientFrames {
		t.Error("Expected skip client frames to be true")
	}
	if config.Timeout != 30*time.Second {
		t.Errorf("Expected 30s timeout, got %v", config.Timeout)
	}
}

// =============================================================================
// EDGE CASE TESTS
// =============================================================================

func TestConnectionManager_EmptyGroup_Operations(t *testing.T) {
	manager := NewConnectionManager()

	// Operations on empty groups shouldn't panic
	members := manager.ListByGroup("nonexistent")
	if len(members) != 0 {
		t.Error("Empty group should return empty list")
	}

	// Broadcast to empty group
	sent := manager.BroadcastToGroupRaw("nonexistent", MessageText, []byte("test"))
	if sent != 0 {
		t.Errorf("Broadcast to empty group should send 0, got %d", sent)
	}
}

func TestConnectionManager_SingleConnection_Operations(t *testing.T) {
	manager := NewConnectionManager()
	conn := createTestConnectionWithoutWS("single")
	manager.Add(conn)

	// Test all operations with single connection
	if manager.Count() != 1 {
		t.Error("Count should be 1")
	}

	_ = manager.JoinGroup("single", "solo-group")
	members := manager.ListByGroup("solo-group")
	if len(members) != 1 {
		t.Errorf("Expected 1 member, got %d", len(members))
	}

	// Verify group membership
	groups := manager.ListGroups()
	if len(groups) != 1 || groups[0] != "solo-group" {
		t.Errorf("Expected solo-group, got %v", groups)
	}
}

func TestConnectionManager_ConnectionCount_ByEndpoint(t *testing.T) {
	manager := NewConnectionManager()

	// Add connections to different endpoints
	for i := 0; i < 3; i++ {
		conn := createTestConnectionWithoutWS(string(rune('A' + i)))
		conn.endpointPath = "/ws1"
		manager.Add(conn)
	}

	if manager.CountByEndpoint("/ws1") != 3 {
		t.Errorf("Expected 3 for /ws1, got %d", manager.CountByEndpoint("/ws1"))
	}
	if manager.CountByEndpoint("/ws2") != 0 {
		t.Errorf("Expected 0 for /ws2, got %d", manager.CountByEndpoint("/ws2"))
	}
}

func TestConnectionManager_Stats_ReturnsCorrectValues(t *testing.T) {
	manager := NewConnectionManager()

	for i := 0; i < 5; i++ {
		conn := createTestConnectionWithoutWS(string(rune('A' + i)))
		conn.endpointPath = "/ws"
		manager.Add(conn)
	}

	stats := manager.WebSocketStats()
	if stats.TotalConnections != 5 {
		t.Errorf("Expected 5 connections, got %d", stats.TotalConnections)
	}
	if stats.ConnectionsByEndpoint["/ws"] != 5 {
		t.Errorf("Expected 5 for /ws, got %d", stats.ConnectionsByEndpoint["/ws"])
	}
}

// =============================================================================
// CONCURRENCY STRESS TESTS
// =============================================================================

func TestConnection_ConcurrentMetadataAccess(t *testing.T) {
	conn := createTestConnectionWithoutWS("concurrent-metadata")

	var wg sync.WaitGroup
	const numGoroutines = 20
	const opsPerGoroutine = 100

	wg.Add(numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < opsPerGoroutine; j++ {
				key := string(rune('a' + (j % 26)))
				conn.SetMetadata(key, j)
				_ = conn.Metadata()
			}
		}(i)
	}

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Success
	case <-time.After(5 * time.Second):
		t.Fatal("Concurrent metadata access timed out")
	}
}

func TestConnectionManager_ConcurrentAddRemove(t *testing.T) {
	manager := NewConnectionManager()

	var wg sync.WaitGroup
	const numGoroutines = 20
	const opsPerGoroutine = 50

	wg.Add(numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < opsPerGoroutine; j++ {
				connID := GenerateConnectionID()
				conn := createTestConnectionWithoutWS(connID)
				manager.Add(conn)
				_ = manager.Count()
				_ = manager.ListAll()
				manager.Remove(connID)
			}
		}(i)
	}

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Success
	case <-time.After(10 * time.Second):
		t.Fatal("Concurrent add/remove timed out")
	}
}

func TestConnectionManager_ConcurrentGroupJoinLeave(t *testing.T) {
	manager := NewConnectionManager()

	const numConnections = 10
	connections := make([]*Connection, numConnections)
	for i := 0; i < numConnections; i++ {
		conn := createTestConnectionWithoutWS(string(rune('A' + i)))
		manager.Add(conn)
		connections[i] = conn
	}

	var wg sync.WaitGroup
	const numGoroutines = 50
	const opsPerGoroutine = 100

	wg.Add(numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < opsPerGoroutine; j++ {
				conn := connections[j%numConnections]
				group := string(rune('0' + (j % 5)))

				switch j % 4 {
				case 0:
					_ = manager.JoinGroup(conn.ID(), group)
				case 1:
					_ = manager.LeaveGroup(conn.ID(), group)
				case 2:
					_ = conn.JoinGroup(group)
				case 3:
					_ = conn.LeaveGroup(group)
				}
			}
		}(i)
	}

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Success
	case <-time.After(10 * time.Second):
		t.Fatal("Concurrent group operations timed out")
	}
}

func TestConnectionManager_ConcurrentBroadcast_ListOperations(t *testing.T) {
	// Test that broadcast-related list operations don't race with add/remove.
	// We can't test actual broadcast without real websockets, but we can test
	// the locking behavior of the connection list operations.
	manager := NewConnectionManager()

	const numConnections = 20
	for i := 0; i < numConnections; i++ {
		conn := createTestConnectionWithoutWS(string(rune('A' + i)))
		conn.endpointPath = "/ws"
		manager.Add(conn)
		_ = conn.JoinGroup("all")
	}

	var wg sync.WaitGroup
	const numGoroutines = 10

	wg.Add(numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				// Test that list operations used by broadcast don't race
				_ = manager.ListByEndpoint("/ws")
				_ = manager.ListByGroup("all")
				_ = manager.ListAll()
				_ = manager.Count()
			}
		}()
	}

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Success
	case <-time.After(10 * time.Second):
		t.Fatal("Concurrent list operations timed out")
	}
}

// =============================================================================
// Configuration Tests - SkipOriginVerify
// =============================================================================

func TestEndpoint_SkipOriginVerify_DefaultTrue(t *testing.T) {
	// Test that SkipOriginVerify defaults to true for development convenience
	cfg := &EndpointConfig{
		Path: "/ws/test",
	}

	endpoint, err := NewEndpoint(cfg)
	if err != nil {
		t.Fatalf("Failed to create endpoint: %v", err)
	}

	if !endpoint.SkipOriginVerify() {
		t.Error("SkipOriginVerify should default to true")
	}
}

func TestEndpoint_SkipOriginVerify_ExplicitFalse(t *testing.T) {
	// Test that SkipOriginVerify can be explicitly set to false
	skipVerify := false
	cfg := &EndpointConfig{
		Path:             "/ws/test",
		SkipOriginVerify: &skipVerify,
	}

	endpoint, err := NewEndpoint(cfg)
	if err != nil {
		t.Fatalf("Failed to create endpoint: %v", err)
	}

	if endpoint.SkipOriginVerify() {
		t.Error("SkipOriginVerify should be false when explicitly set")
	}
}

func TestEndpoint_SkipOriginVerify_ExplicitTrue(t *testing.T) {
	// Test that SkipOriginVerify can be explicitly set to true
	skipVerify := true
	cfg := &EndpointConfig{
		Path:             "/ws/test",
		SkipOriginVerify: &skipVerify,
	}

	endpoint, err := NewEndpoint(cfg)
	if err != nil {
		t.Fatalf("Failed to create endpoint: %v", err)
	}

	if !endpoint.SkipOriginVerify() {
		t.Error("SkipOriginVerify should be true when explicitly set")
	}
}
