package websocket

import (
	"sync"
	"testing"
	"time"
)

func TestConnectionManager_JoinLeaveGroup(t *testing.T) {
	manager := NewConnectionManager()

	// Create a mock connection
	conn := &Connection{
		id:     "test-conn-1",
		groups: make(map[string]struct{}),
	}
	conn.manager = manager

	// Add connection to manager
	manager.Add(conn)

	// Test JoinGroup
	err := manager.JoinGroup(conn.id, "room1")
	if err != nil {
		t.Fatalf("JoinGroup failed: %v", err)
	}

	// Verify connection is in group
	if !conn.InGroup("room1") {
		t.Error("Connection should be in group 'room1'")
	}

	// Test duplicate join
	err = manager.JoinGroup(conn.id, "room1")
	if err != ErrAlreadyInGroup {
		t.Errorf("Expected ErrAlreadyInGroup, got %v", err)
	}

	// Test LeaveGroup
	err = manager.LeaveGroup(conn.id, "room1")
	if err != nil {
		t.Fatalf("LeaveGroup failed: %v", err)
	}

	// Verify connection left group
	if conn.InGroup("room1") {
		t.Error("Connection should not be in group 'room1'")
	}

	// Test leaving a group not in
	err = manager.LeaveGroup(conn.id, "room1")
	if err != ErrNotInGroup {
		t.Errorf("Expected ErrNotInGroup, got %v", err)
	}
}

func TestConnectionManager_JoinGroupNonexistentConnection(t *testing.T) {
	manager := NewConnectionManager()

	err := manager.JoinGroup("nonexistent", "room1")
	if err != ErrConnectionNotFound {
		t.Errorf("Expected ErrConnectionNotFound, got %v", err)
	}
}

func TestConnection_JoinLeaveGroup(t *testing.T) {
	manager := NewConnectionManager()

	conn := &Connection{
		id:     "test-conn-2",
		groups: make(map[string]struct{}),
	}
	conn.manager = manager
	manager.Add(conn)

	// Test join via connection method
	err := conn.JoinGroup("room2")
	if err != nil {
		t.Fatalf("JoinGroup via connection failed: %v", err)
	}

	if !conn.InGroup("room2") {
		t.Error("Connection should be in group 'room2'")
	}

	// Test duplicate join via connection
	err = conn.JoinGroup("room2")
	if err != ErrAlreadyInGroup {
		t.Errorf("Expected ErrAlreadyInGroup, got %v", err)
	}

	// Test leave via connection method
	err = conn.LeaveGroup("room2")
	if err != nil {
		t.Fatalf("LeaveGroup via connection failed: %v", err)
	}

	if conn.InGroup("room2") {
		t.Error("Connection should not be in group 'room2'")
	}
}

func TestConnectionManager_ConcurrentGroupOperations(t *testing.T) {
	// This test verifies that concurrent group operations don't cause deadlock.
	// Run with -race flag to detect race conditions.
	manager := NewConnectionManager()

	// Create multiple connections
	const numConnections = 10
	connections := make([]*Connection, numConnections)
	for i := 0; i < numConnections; i++ {
		conn := &Connection{
			id:     string(rune('A' + i)),
			groups: make(map[string]struct{}),
		}
		conn.manager = manager
		connections[i] = conn
		manager.Add(conn)
	}

	const numGroups = 5
	groups := make([]string, numGroups)
	for i := 0; i < numGroups; i++ {
		groups[i] = string(rune('0' + i))
	}

	// Run concurrent operations that could deadlock if locking is wrong
	const numGoroutines = 50
	const opsPerGoroutine = 100
	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	// Timeout channel to detect deadlock
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < opsPerGoroutine; j++ {
				conn := connections[j%numConnections]
				group := groups[j%numGroups]

				// Alternate between different join/leave patterns
				switch j % 4 {
				case 0:
					// Join via manager
					_ = manager.JoinGroup(conn.id, group)
				case 1:
					// Leave via manager
					_ = manager.LeaveGroup(conn.id, group)
				case 2:
					// Join via connection
					_ = conn.JoinGroup(group)
				case 3:
					// Leave via connection
					_ = conn.LeaveGroup(group)
				}
			}
		}(i)
	}

	// Wait with timeout
	select {
	case <-done:
		// Success - no deadlock
	case <-time.After(10 * time.Second):
		t.Fatal("Test timed out - possible deadlock detected")
	}
}

func TestConnectionManager_ConcurrentJoinFromBothSides(t *testing.T) {
	// Test specifically the case where manager.JoinGroup and conn.JoinGroup
	// are called simultaneously for the same connection/group.
	manager := NewConnectionManager()

	conn := &Connection{
		id:     "concurrent-test",
		groups: make(map[string]struct{}),
	}
	conn.manager = manager
	manager.Add(conn)

	const iterations = 100
	for i := 0; i < iterations; i++ {
		group := "test-group"

		// Reset state
		conn.mu.Lock()
		delete(conn.groups, group)
		conn.mu.Unlock()
		manager.mu.Lock()
		delete(manager.byGroup, group)
		manager.mu.Unlock()

		// Try to join from both sides simultaneously
		var wg sync.WaitGroup
		wg.Add(2)

		go func() {
			defer wg.Done()
			manager.JoinGroup(conn.id, group)
		}()

		go func() {
			defer wg.Done()
			conn.JoinGroup(group)
		}()

		done := make(chan struct{})
		go func() {
			wg.Wait()
			close(done)
		}()

		select {
		case <-done:
			// Success
		case <-time.After(time.Second):
			t.Fatalf("Iteration %d: Deadlock detected during concurrent join", i)
		}

		// Connection should be in group (may have been added once or twice)
		if !conn.InGroup(group) {
			t.Errorf("Iteration %d: Connection should be in group after concurrent joins", i)
		}
	}
}

func TestConnectionManager_BroadcastToGroup(t *testing.T) {
	manager := NewConnectionManager()

	// Create connections - we can't send real messages without a WebSocket,
	// but we can verify the group membership logic works
	conn1 := &Connection{
		id:     "conn1",
		groups: make(map[string]struct{}),
	}
	conn1.manager = manager

	conn2 := &Connection{
		id:     "conn2",
		groups: make(map[string]struct{}),
	}
	conn2.manager = manager

	manager.Add(conn1)
	manager.Add(conn2)

	// Join conn1 to room1
	_ = manager.JoinGroup("conn1", "room1")

	// Join conn2 to room1 and room2
	_ = manager.JoinGroup("conn2", "room1")
	_ = manager.JoinGroup("conn2", "room2")

	// Verify group membership via manager's internal state
	manager.mu.RLock()
	room1Members := manager.byGroup["room1"]
	room2Members := manager.byGroup["room2"]
	manager.mu.RUnlock()

	if len(room1Members) != 2 {
		t.Errorf("Expected 2 members in room1, got %d", len(room1Members))
	}
	if !room1Members["conn1"] || !room1Members["conn2"] {
		t.Error("room1 should contain both conn1 and conn2")
	}

	if len(room2Members) != 1 {
		t.Errorf("Expected 1 member in room2, got %d", len(room2Members))
	}
	if !room2Members["conn2"] {
		t.Error("room2 should contain conn2")
	}
}
