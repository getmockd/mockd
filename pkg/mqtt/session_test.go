package mqtt

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// TestPanelSession — subscription management
// ---------------------------------------------------------------------------

func TestTestPanelSession_AddSubscription(t *testing.T) {
	s := NewTestPanelSession("broker-1")

	s.AddSubscription("sensors/temp")
	s.AddSubscription("sensors/humidity")

	assert.Equal(t, []string{"sensors/temp", "sensors/humidity"}, s.Subscriptions)
}

func TestTestPanelSession_AddSubscription_Dedup(t *testing.T) {
	s := NewTestPanelSession("broker-1")

	s.AddSubscription("sensors/temp")
	s.AddSubscription("sensors/temp") // duplicate

	assert.Len(t, s.Subscriptions, 1)
}

func TestTestPanelSession_RemoveSubscription(t *testing.T) {
	s := NewTestPanelSession("broker-1")

	s.AddSubscription("sensors/temp")
	s.AddSubscription("sensors/humidity")
	s.RemoveSubscription("sensors/temp")

	assert.Equal(t, []string{"sensors/humidity"}, s.Subscriptions)
}

func TestTestPanelSession_RemoveSubscription_NotPresent(t *testing.T) {
	s := NewTestPanelSession("broker-1")
	s.AddSubscription("sensors/temp")

	s.RemoveSubscription("does-not-exist")
	assert.Len(t, s.Subscriptions, 1)
}

// ---------------------------------------------------------------------------
// TestPanelSession — IsSubscribed (uses MQTT wildcard matching)
// ---------------------------------------------------------------------------

func TestTestPanelSession_IsSubscribed_Exact(t *testing.T) {
	s := NewTestPanelSession("broker-1")
	s.AddSubscription("sensors/temp")

	assert.True(t, s.IsSubscribed("sensors/temp"))
	assert.False(t, s.IsSubscribed("sensors/humidity"))
}

func TestTestPanelSession_IsSubscribed_Wildcards(t *testing.T) {
	s := NewTestPanelSession("broker-1")
	s.AddSubscription("sensors/+/data")
	s.AddSubscription("home/#")

	assert.True(t, s.IsSubscribed("sensors/room1/data"))
	assert.False(t, s.IsSubscribed("sensors/room1/command"))
	assert.True(t, s.IsSubscribed("home/living/light"))
	assert.True(t, s.IsSubscribed("home/kitchen"))
}

func TestTestPanelSession_IsSubscribed_NoSubscriptions(t *testing.T) {
	s := NewTestPanelSession("broker-1")
	assert.False(t, s.IsSubscribed("any/topic"))
}

// ---------------------------------------------------------------------------
// TestPanelSession — message history
// ---------------------------------------------------------------------------

func TestTestPanelSession_AddAndGetMessages(t *testing.T) {
	s := NewTestPanelSession("broker-1")

	for i := 0; i < 5; i++ {
		s.AddMessage(MQTTMessage{
			ID:    string(rune('a' + i)),
			Topic: "test",
		})
	}

	msgs := s.GetMessages(0)
	assert.Len(t, msgs, 5)

	// GetMessages with limit returns most recent.
	msgs = s.GetMessages(2)
	assert.Len(t, msgs, 2)
	// Last two IDs should be d, e  (rune(100), rune(101))
	assert.Equal(t, string(rune('d')), msgs[0].ID)
	assert.Equal(t, string(rune('e')), msgs[1].ID)
}

func TestTestPanelSession_MessageHistory_TrimAtMax(t *testing.T) {
	s := NewTestPanelSession("broker-1")

	// Add MaxMessageHistory + 50 messages.
	total := MaxMessageHistory + 50
	for i := 0; i < total; i++ {
		s.AddMessage(MQTTMessage{
			ID:    "msg",
			Topic: "test",
		})
	}

	msgs := s.GetMessages(0)
	assert.Len(t, msgs, MaxMessageHistory)
}

func TestTestPanelSession_ClearHistory(t *testing.T) {
	s := NewTestPanelSession("broker-1")

	s.AddMessage(MQTTMessage{ID: "m1", Topic: "test"})
	s.AddMessage(MQTTMessage{ID: "m2", Topic: "test"})
	s.ClearHistory()

	assert.Empty(t, s.GetMessages(0))
}

// ---------------------------------------------------------------------------
// TestPanelSession — listener channels
// ---------------------------------------------------------------------------

func TestTestPanelSession_SubscribeReceivesMessages(t *testing.T) {
	s := NewTestPanelSession("broker-1")
	ch := s.Subscribe()

	msg := MQTTMessage{ID: "m1", Topic: "sensors/temp", Payload: `{"temp":22}`}
	s.AddMessage(msg)

	select {
	case got := <-ch:
		assert.Equal(t, "m1", got.ID)
		assert.Equal(t, "sensors/temp", got.Topic)
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for message on listener channel")
	}
}

func TestTestPanelSession_Unsubscribe(t *testing.T) {
	s := NewTestPanelSession("broker-1")
	ch := s.Subscribe()

	s.Unsubscribe(ch)

	// Channel should be closed after unsubscribe.
	_, open := <-ch
	assert.False(t, open, "channel should be closed after Unsubscribe")
}

func TestTestPanelSession_MultipleListeners(t *testing.T) {
	s := NewTestPanelSession("broker-1")
	ch1 := s.Subscribe()
	ch2 := s.Subscribe()

	msg := MQTTMessage{ID: "m1", Topic: "test"}
	s.AddMessage(msg)

	for _, ch := range []chan MQTTMessage{ch1, ch2} {
		select {
		case got := <-ch:
			assert.Equal(t, "m1", got.ID)
		case <-time.After(time.Second):
			t.Fatal("timed out waiting for message on listener")
		}
	}
}

func TestTestPanelSession_CloseListeners(t *testing.T) {
	s := NewTestPanelSession("broker-1")
	ch1 := s.Subscribe()
	ch2 := s.Subscribe()

	s.CloseListeners()

	_, open1 := <-ch1
	_, open2 := <-ch2
	assert.False(t, open1)
	assert.False(t, open2)
}

// ---------------------------------------------------------------------------
// TestPanelSession — LastActivity updated
// ---------------------------------------------------------------------------

func TestTestPanelSession_LastActivityUpdated(t *testing.T) {
	s := NewTestPanelSession("broker-1")
	initial := s.LastActivity

	time.Sleep(5 * time.Millisecond)
	s.AddSubscription("new/topic")
	assert.True(t, s.LastActivity.After(initial), "AddSubscription should update LastActivity")

	prev := s.LastActivity
	time.Sleep(5 * time.Millisecond)
	s.RemoveSubscription("new/topic")
	assert.True(t, s.LastActivity.After(prev), "RemoveSubscription should update LastActivity")

	prev = s.LastActivity
	time.Sleep(5 * time.Millisecond)
	s.AddMessage(MQTTMessage{ID: "m1"})
	assert.True(t, s.LastActivity.After(prev), "AddMessage should update LastActivity")

	prev = s.LastActivity
	time.Sleep(5 * time.Millisecond)
	s.ClearHistory()
	assert.True(t, s.LastActivity.After(prev), "ClearHistory should update LastActivity")
}

// ---------------------------------------------------------------------------
// SessionManager — create / get / delete
// ---------------------------------------------------------------------------

func TestSessionManager_CreateAndGetSession(t *testing.T) {
	sm := NewSessionManager()

	s := sm.CreateSession("broker-1")
	require.NotNil(t, s)
	assert.NotEmpty(t, s.ID)
	assert.Equal(t, "broker-1", s.BrokerID)

	got := sm.GetSession(s.ID)
	require.NotNil(t, got)
	assert.Equal(t, s.ID, got.ID)
}

func TestSessionManager_GetSession_NotFound(t *testing.T) {
	sm := NewSessionManager()
	assert.Nil(t, sm.GetSession("nonexistent"))
}

func TestSessionManager_DeleteSession(t *testing.T) {
	sm := NewSessionManager()
	s := sm.CreateSession("broker-1")

	// Attach a listener so we can verify it gets closed on delete.
	ch := s.Subscribe()

	sm.DeleteSession(s.ID)

	assert.Nil(t, sm.GetSession(s.ID))

	// Listener channel should have been closed.
	_, open := <-ch
	assert.False(t, open, "listener channel should be closed after session deletion")
}

func TestSessionManager_DeleteSession_NonExistent(t *testing.T) {
	sm := NewSessionManager()
	// Should not panic.
	sm.DeleteSession("does-not-exist")
}

// ---------------------------------------------------------------------------
// SessionManager — GetBrokerSessions
// ---------------------------------------------------------------------------

func TestSessionManager_GetBrokerSessions(t *testing.T) {
	sm := NewSessionManager()

	s1 := sm.CreateSession("broker-1")
	s2 := sm.CreateSession("broker-1")
	_ = sm.CreateSession("broker-2")

	sessions := sm.GetBrokerSessions("broker-1")
	require.Len(t, sessions, 2)
	ids := []string{sessions[0].ID, sessions[1].ID}
	assert.Contains(t, ids, s1.ID)
	assert.Contains(t, ids, s2.ID)
}

func TestSessionManager_GetBrokerSessions_Empty(t *testing.T) {
	sm := NewSessionManager()
	assert.Empty(t, sm.GetBrokerSessions("nonexistent"))
}

func TestSessionManager_DeleteSession_RemovesFromBrokerIndex(t *testing.T) {
	sm := NewSessionManager()

	s1 := sm.CreateSession("broker-1")
	s2 := sm.CreateSession("broker-1")

	sm.DeleteSession(s1.ID)

	sessions := sm.GetBrokerSessions("broker-1")
	require.Len(t, sessions, 1)
	assert.Equal(t, s2.ID, sessions[0].ID)
}

// ---------------------------------------------------------------------------
// SessionManager — NotifyMessage
// ---------------------------------------------------------------------------

func TestSessionManager_NotifyMessage(t *testing.T) {
	sm := NewSessionManager()
	s := sm.CreateSession("broker-1")
	s.AddSubscription("sensors/#")

	ch := s.Subscribe()

	msg := MQTTMessage{
		ID:    "m1",
		Topic: "sensors/temp",
	}
	sm.NotifyMessage("broker-1", msg)

	select {
	case got := <-ch:
		assert.Equal(t, "m1", got.ID)
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for NotifyMessage")
	}
}

func TestSessionManager_NotifyMessage_NotSubscribed(t *testing.T) {
	sm := NewSessionManager()
	s := sm.CreateSession("broker-1")
	s.AddSubscription("other/topic")

	ch := s.Subscribe()

	msg := MQTTMessage{
		ID:    "m1",
		Topic: "sensors/temp",
	}
	sm.NotifyMessage("broker-1", msg)

	select {
	case <-ch:
		t.Fatal("session should not receive message for non-subscribed topic")
	case <-time.After(100 * time.Millisecond):
		// expected — no message
	}
}

func TestSessionManager_NotifyMessage_DifferentBroker(t *testing.T) {
	sm := NewSessionManager()
	s := sm.CreateSession("broker-1")
	s.AddSubscription("#") // subscribe to everything

	ch := s.Subscribe()

	msg := MQTTMessage{ID: "m1", Topic: "test"}
	sm.NotifyMessage("broker-2", msg) // different broker

	select {
	case <-ch:
		t.Fatal("session on broker-1 should not receive messages from broker-2")
	case <-time.After(100 * time.Millisecond):
		// expected
	}
}

// ---------------------------------------------------------------------------
// SessionManager — CleanupStaleSessions
// ---------------------------------------------------------------------------

func TestSessionManager_CleanupStaleSessions(t *testing.T) {
	sm := NewSessionManager()

	// Create two sessions. Make one stale by setting its LastActivity far in the past.
	active := sm.CreateSession("broker-1")
	stale := sm.CreateSession("broker-1")

	staleCh := stale.Subscribe()

	// Manually set stale session's LastActivity to 2 hours ago.
	stale.mu.Lock()
	stale.LastActivity = time.Now().Add(-2 * time.Hour)
	stale.mu.Unlock()

	removed := sm.CleanupStaleSessions(1 * time.Hour)
	assert.Equal(t, 1, removed)

	// Active session should still be accessible.
	assert.NotNil(t, sm.GetSession(active.ID))

	// Stale session should be gone.
	assert.Nil(t, sm.GetSession(stale.ID))

	// Stale session's listener should have been closed.
	_, open := <-staleCh
	assert.False(t, open, "stale session listener should be closed after cleanup")

	// Broker index should only contain the active session.
	sessions := sm.GetBrokerSessions("broker-1")
	require.Len(t, sessions, 1)
	assert.Equal(t, active.ID, sessions[0].ID)
}

func TestSessionManager_CleanupStaleSessions_NoneStale(t *testing.T) {
	sm := NewSessionManager()
	sm.CreateSession("broker-1")
	sm.CreateSession("broker-1")

	removed := sm.CleanupStaleSessions(1 * time.Hour)
	assert.Equal(t, 0, removed)
}

func TestSessionManager_CleanupStaleSessions_AllStale(t *testing.T) {
	sm := NewSessionManager()

	s1 := sm.CreateSession("broker-1")
	s2 := sm.CreateSession("broker-2")

	s1.mu.Lock()
	s1.LastActivity = time.Now().Add(-3 * time.Hour)
	s1.mu.Unlock()

	s2.mu.Lock()
	s2.LastActivity = time.Now().Add(-3 * time.Hour)
	s2.mu.Unlock()

	removed := sm.CleanupStaleSessions(1 * time.Hour)
	assert.Equal(t, 2, removed)
	assert.Empty(t, sm.GetBrokerSessions("broker-1"))
	assert.Empty(t, sm.GetBrokerSessions("broker-2"))
}
