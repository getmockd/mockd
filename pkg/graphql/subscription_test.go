package graphql

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"
)

const subscriptionTestSchema = `
type Query {
	user(id: ID!): User
}

type Subscription {
	messageAdded(channel: String!): Message
	userPresence(userId: ID!): PresenceEvent
	countdown(from: Int!): Int
}

type User {
	id: ID!
	name: String!
}

type Message {
	id: ID!
	text: String!
	channel: String!
}

type PresenceEvent {
	userId: ID!
	status: String!
	timestamp: String!
}
`

func newTestSubscriptionHandler(t *testing.T, config *GraphQLConfig) *SubscriptionHandler {
	schema, err := ParseSchema(subscriptionTestSchema)
	if err != nil {
		t.Fatalf("ParseSchema() error = %v", err)
	}

	return NewSubscriptionHandler(schema, config)
}

func setupTestWebSocketServer(t *testing.T, handler *SubscriptionHandler) *httptest.Server {
	ts := httptest.NewServer(handler)
	t.Cleanup(func() {
		ts.Close()
	})
	return ts
}

func connectWS(t *testing.T, url string, subprotocol string) *websocket.Conn {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	wsURL := "ws" + strings.TrimPrefix(url, "http")

	var opts *websocket.DialOptions
	if subprotocol != "" {
		opts = &websocket.DialOptions{
			Subprotocols: []string{subprotocol},
		}
	}

	conn, _, err := websocket.Dial(ctx, wsURL, opts)
	if err != nil {
		t.Fatalf("websocket.Dial() error = %v", err)
	}

	t.Cleanup(func() {
		conn.Close(websocket.StatusNormalClosure, "test cleanup")
	})

	return conn
}

func sendWSMessage(t *testing.T, conn *websocket.Conn, msg *wsMessage) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	if err := conn.Write(ctx, websocket.MessageText, data); err != nil {
		t.Fatalf("conn.Write() error = %v", err)
	}
}

func readWSMessage(t *testing.T, conn *websocket.Conn) *wsMessage {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	msgType, data, err := conn.Read(ctx)
	if err != nil {
		t.Fatalf("conn.Read() error = %v", err)
	}

	if msgType != websocket.MessageText {
		t.Fatalf("expected text message, got %v", msgType)
	}

	var msg wsMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	return &msg
}

func readWSMessageWithTimeout(t *testing.T, conn *websocket.Conn, timeout time.Duration) (*wsMessage, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	msgType, data, err := conn.Read(ctx)
	if err != nil {
		return nil, err
	}

	if msgType != websocket.MessageText {
		t.Fatalf("expected text message, got %v", msgType)
	}

	var msg wsMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	return &msg, nil
}

// ============================================================================
// Connection Tests
// ============================================================================

func TestSubscriptionHandler_ConnectionInit(t *testing.T) {
	config := &GraphQLConfig{
		Subscriptions: map[string]SubscriptionConfig{
			"messageAdded": {
				Events: []EventConfig{
					{Data: map[string]interface{}{"id": "1", "text": "hello"}},
				},
			},
		},
	}

	handler := newTestSubscriptionHandler(t, config)
	ts := setupTestWebSocketServer(t, handler)

	conn := connectWS(t, ts.URL, "graphql-transport-ws")

	// Send connection_init
	sendWSMessage(t, conn, &wsMessage{Type: msgTypeConnectionInit})

	// Should receive connection_ack
	resp := readWSMessage(t, conn)
	if resp.Type != msgTypeConnectionAck {
		t.Errorf("expected connection_ack, got %s", resp.Type)
	}
}

func TestSubscriptionHandler_LegacyProtocol_ConnectionInit(t *testing.T) {
	config := &GraphQLConfig{
		Subscriptions: map[string]SubscriptionConfig{
			"messageAdded": {
				Events: []EventConfig{
					{Data: map[string]interface{}{"id": "1", "text": "hello"}},
				},
			},
		},
	}

	handler := newTestSubscriptionHandler(t, config)
	ts := setupTestWebSocketServer(t, handler)

	conn := connectWS(t, ts.URL, "graphql-ws")

	// Send connection_init
	sendWSMessage(t, conn, &wsMessage{Type: msgTypeConnectionInit})

	// Should receive connection_ack
	resp := readWSMessage(t, conn)
	if resp.Type != msgTypeConnectionAck {
		t.Errorf("expected connection_ack, got %s", resp.Type)
	}

	// Legacy protocol should also send keep-alive
	resp2 := readWSMessage(t, conn)
	if resp2.Type != msgTypeConnectionKeepAlive {
		t.Errorf("expected ka (keep-alive), got %s", resp2.Type)
	}
}

func TestSubscriptionHandler_PingPong(t *testing.T) {
	config := &GraphQLConfig{}

	handler := newTestSubscriptionHandler(t, config)
	ts := setupTestWebSocketServer(t, handler)

	conn := connectWS(t, ts.URL, "graphql-transport-ws")

	// Init connection first
	sendWSMessage(t, conn, &wsMessage{Type: msgTypeConnectionInit})
	readWSMessage(t, conn) // connection_ack

	// Send ping
	pingPayload, _ := json.Marshal(map[string]string{"test": "data"})
	sendWSMessage(t, conn, &wsMessage{Type: msgTypePing, Payload: pingPayload})

	// Should receive pong with same payload
	resp := readWSMessage(t, conn)
	if resp.Type != msgTypePong {
		t.Errorf("expected pong, got %s", resp.Type)
	}

	// Verify payload is echoed back
	var payload map[string]string
	if err := json.Unmarshal(resp.Payload, &payload); err == nil {
		if payload["test"] != "data" {
			t.Errorf("expected payload to be echoed back")
		}
	}
}

// ============================================================================
// Subscription Tests
// ============================================================================

func TestSubscriptionHandler_BasicSubscription(t *testing.T) {
	config := &GraphQLConfig{
		Subscriptions: map[string]SubscriptionConfig{
			"messageAdded": {
				Events: []EventConfig{
					{Data: map[string]interface{}{"id": "1", "text": "hello", "channel": "general"}},
					{Data: map[string]interface{}{"id": "2", "text": "world", "channel": "general"}},
				},
			},
		},
	}

	handler := newTestSubscriptionHandler(t, config)
	ts := setupTestWebSocketServer(t, handler)

	conn := connectWS(t, ts.URL, "graphql-transport-ws")

	// Init connection
	sendWSMessage(t, conn, &wsMessage{Type: msgTypeConnectionInit})
	readWSMessage(t, conn) // connection_ack

	// Subscribe
	payload, _ := json.Marshal(subscribePayload{
		Query: `subscription { messageAdded(channel: "general") { id text channel } }`,
	})
	sendWSMessage(t, conn, &wsMessage{
		ID:      "sub-1",
		Type:    msgTypeSubscribe,
		Payload: payload,
	})

	// Should receive first event
	event1 := readWSMessage(t, conn)
	if event1.Type != msgTypeNext {
		t.Errorf("expected next, got %s", event1.Type)
	}
	if event1.ID != "sub-1" {
		t.Errorf("expected id sub-1, got %s", event1.ID)
	}

	// Should receive second event
	event2 := readWSMessage(t, conn)
	if event2.Type != msgTypeNext {
		t.Errorf("expected next, got %s", event2.Type)
	}

	// Should receive complete
	complete := readWSMessage(t, conn)
	if complete.Type != msgTypeComplete {
		t.Errorf("expected complete, got %s", complete.Type)
	}
	if complete.ID != "sub-1" {
		t.Errorf("expected id sub-1, got %s", complete.ID)
	}
}

func TestSubscriptionHandler_LegacyProtocol_Subscription(t *testing.T) {
	config := &GraphQLConfig{
		Subscriptions: map[string]SubscriptionConfig{
			"messageAdded": {
				Events: []EventConfig{
					{Data: map[string]interface{}{"id": "1", "text": "hello"}},
				},
			},
		},
	}

	handler := newTestSubscriptionHandler(t, config)
	ts := setupTestWebSocketServer(t, handler)

	conn := connectWS(t, ts.URL, "graphql-ws")

	// Init connection
	sendWSMessage(t, conn, &wsMessage{Type: msgTypeConnectionInit})
	readWSMessage(t, conn) // connection_ack
	readWSMessage(t, conn) // ka

	// Subscribe using "start" (legacy)
	payload, _ := json.Marshal(subscribePayload{
		Query: `subscription { messageAdded(channel: "general") { id text } }`,
	})
	sendWSMessage(t, conn, &wsMessage{
		ID:      "sub-1",
		Type:    msgTypeStart,
		Payload: payload,
	})

	// Should receive data event (legacy uses "data" instead of "next")
	event := readWSMessage(t, conn)
	if event.Type != msgTypeData {
		t.Errorf("expected data, got %s", event.Type)
	}

	// Should receive complete
	complete := readWSMessage(t, conn)
	if complete.Type != msgTypeComplete {
		t.Errorf("expected complete, got %s", complete.Type)
	}
}

func TestSubscriptionHandler_WithVariables(t *testing.T) {
	config := &GraphQLConfig{
		Subscriptions: map[string]SubscriptionConfig{
			"messageAdded": {
				Events: []EventConfig{
					{Data: map[string]interface{}{
						"id":      "1",
						"text":    "hello from {{args.channel}}",
						"channel": "{{args.channel}}",
					}},
				},
			},
		},
	}

	handler := newTestSubscriptionHandler(t, config)
	ts := setupTestWebSocketServer(t, handler)

	conn := connectWS(t, ts.URL, "graphql-transport-ws")

	// Init connection
	sendWSMessage(t, conn, &wsMessage{Type: msgTypeConnectionInit})
	readWSMessage(t, conn) // connection_ack

	// Subscribe with variables
	payload, _ := json.Marshal(subscribePayload{
		Query:     `subscription Sub($channel: String!) { messageAdded(channel: $channel) { id text channel } }`,
		Variables: map[string]interface{}{"channel": "tech"},
	})
	sendWSMessage(t, conn, &wsMessage{
		ID:      "sub-1",
		Type:    msgTypeSubscribe,
		Payload: payload,
	})

	// Should receive event with substituted variables
	event := readWSMessage(t, conn)
	if event.Type != msgTypeNext {
		t.Errorf("expected next, got %s", event.Type)
	}

	var eventPayload map[string]interface{}
	if err := json.Unmarshal(event.Payload, &eventPayload); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	data, ok := eventPayload["data"].(map[string]interface{})
	if !ok {
		t.Fatal("expected data in payload")
	}

	if data["channel"] != "tech" {
		t.Errorf("expected channel to be 'tech', got %v", data["channel"])
	}

	if !strings.Contains(data["text"].(string), "tech") {
		t.Errorf("expected text to contain 'tech', got %v", data["text"])
	}
}

func TestSubscriptionHandler_SubscriptionWithPrefixLookup(t *testing.T) {
	config := &GraphQLConfig{
		Subscriptions: map[string]SubscriptionConfig{
			"Subscription.messageAdded": {
				Events: []EventConfig{
					{Data: map[string]interface{}{"id": "1", "text": "from prefixed config"}},
				},
			},
		},
	}

	handler := newTestSubscriptionHandler(t, config)
	ts := setupTestWebSocketServer(t, handler)

	conn := connectWS(t, ts.URL, "graphql-transport-ws")

	// Init and subscribe
	sendWSMessage(t, conn, &wsMessage{Type: msgTypeConnectionInit})
	readWSMessage(t, conn) // connection_ack

	payload, _ := json.Marshal(subscribePayload{
		Query: `subscription { messageAdded(channel: "test") { id text } }`,
	})
	sendWSMessage(t, conn, &wsMessage{
		ID:      "sub-1",
		Type:    msgTypeSubscribe,
		Payload: payload,
	})

	// Should receive event
	event := readWSMessage(t, conn)
	if event.Type != msgTypeNext {
		t.Errorf("expected next, got %s", event.Type)
	}
}

// ============================================================================
// Timing Tests
// ============================================================================

func TestSubscriptionHandler_FixedDelay(t *testing.T) {
	config := &GraphQLConfig{
		Subscriptions: map[string]SubscriptionConfig{
			"messageAdded": {
				Events: []EventConfig{
					{Data: map[string]interface{}{"id": "1"}},
					{Data: map[string]interface{}{"id": "2"}},
				},
				Timing: &TimingConfig{
					FixedDelay: "100ms",
				},
			},
		},
	}

	handler := newTestSubscriptionHandler(t, config)
	ts := setupTestWebSocketServer(t, handler)

	conn := connectWS(t, ts.URL, "graphql-transport-ws")

	// Init and subscribe
	sendWSMessage(t, conn, &wsMessage{Type: msgTypeConnectionInit})
	readWSMessage(t, conn) // connection_ack

	payload, _ := json.Marshal(subscribePayload{
		Query: `subscription { messageAdded(channel: "test") { id } }`,
	})

	start := time.Now()
	sendWSMessage(t, conn, &wsMessage{
		ID:      "sub-1",
		Type:    msgTypeSubscribe,
		Payload: payload,
	})

	// Read both events
	readWSMessage(t, conn) // first event
	readWSMessage(t, conn) // second event (should be delayed)

	elapsed := time.Since(start)

	// Should take at least 100ms due to delay between events
	if elapsed < 100*time.Millisecond {
		t.Errorf("expected at least 100ms delay, got %v", elapsed)
	}
}

func TestSubscriptionHandler_EventSpecificDelay(t *testing.T) {
	config := &GraphQLConfig{
		Subscriptions: map[string]SubscriptionConfig{
			"messageAdded": {
				Events: []EventConfig{
					{Data: map[string]interface{}{"id": "1"}, Delay: "50ms"},
					{Data: map[string]interface{}{"id": "2"}, Delay: "100ms"},
				},
			},
		},
	}

	handler := newTestSubscriptionHandler(t, config)
	ts := setupTestWebSocketServer(t, handler)

	conn := connectWS(t, ts.URL, "graphql-transport-ws")

	// Init and subscribe
	sendWSMessage(t, conn, &wsMessage{Type: msgTypeConnectionInit})
	readWSMessage(t, conn) // connection_ack

	payload, _ := json.Marshal(subscribePayload{
		Query: `subscription { messageAdded(channel: "test") { id } }`,
	})

	start := time.Now()
	sendWSMessage(t, conn, &wsMessage{
		ID:      "sub-1",
		Type:    msgTypeSubscribe,
		Payload: payload,
	})

	// Read both events
	readWSMessage(t, conn) // first event (after 50ms)
	readWSMessage(t, conn) // second event (after additional 100ms)

	elapsed := time.Since(start)

	// Should take at least 150ms (50ms + 100ms)
	if elapsed < 150*time.Millisecond {
		t.Errorf("expected at least 150ms total delay, got %v", elapsed)
	}
}

func TestSubscriptionHandler_Repeat(t *testing.T) {
	config := &GraphQLConfig{
		Subscriptions: map[string]SubscriptionConfig{
			"messageAdded": {
				Events: []EventConfig{
					{Data: map[string]interface{}{"id": "1"}},
				},
				Timing: &TimingConfig{
					Repeat: true,
				},
			},
		},
	}

	handler := newTestSubscriptionHandler(t, config)
	ts := setupTestWebSocketServer(t, handler)

	conn := connectWS(t, ts.URL, "graphql-transport-ws")

	// Init and subscribe
	sendWSMessage(t, conn, &wsMessage{Type: msgTypeConnectionInit})
	readWSMessage(t, conn) // connection_ack

	payload, _ := json.Marshal(subscribePayload{
		Query: `subscription { messageAdded(channel: "test") { id } }`,
	})
	sendWSMessage(t, conn, &wsMessage{
		ID:      "sub-1",
		Type:    msgTypeSubscribe,
		Payload: payload,
	})

	// Should receive multiple events due to repeat
	receivedCount := 0
	for i := 0; i < 3; i++ {
		msg, err := readWSMessageWithTimeout(t, conn, 500*time.Millisecond)
		if err != nil {
			break
		}
		if msg.Type == msgTypeNext {
			receivedCount++
		}
	}

	if receivedCount < 2 {
		t.Errorf("expected at least 2 repeated events, got %d", receivedCount)
	}
}

// ============================================================================
// Unsubscribe Tests
// ============================================================================

func TestSubscriptionHandler_Unsubscribe(t *testing.T) {
	config := &GraphQLConfig{
		Subscriptions: map[string]SubscriptionConfig{
			"messageAdded": {
				Events: []EventConfig{
					{Data: map[string]interface{}{"id": "1"}, Delay: "100ms"},
					{Data: map[string]interface{}{"id": "2"}, Delay: "100ms"},
					{Data: map[string]interface{}{"id": "3"}, Delay: "100ms"},
				},
			},
		},
	}

	handler := newTestSubscriptionHandler(t, config)
	ts := setupTestWebSocketServer(t, handler)

	conn := connectWS(t, ts.URL, "graphql-transport-ws")

	// Init and subscribe
	sendWSMessage(t, conn, &wsMessage{Type: msgTypeConnectionInit})
	readWSMessage(t, conn) // connection_ack

	payload, _ := json.Marshal(subscribePayload{
		Query: `subscription { messageAdded(channel: "test") { id } }`,
	})
	sendWSMessage(t, conn, &wsMessage{
		ID:      "sub-1",
		Type:    msgTypeSubscribe,
		Payload: payload,
	})

	// Read first event
	event1 := readWSMessage(t, conn)
	if event1.Type != msgTypeNext {
		t.Errorf("expected next, got %s", event1.Type)
	}

	// Unsubscribe (modern protocol uses "complete")
	sendWSMessage(t, conn, &wsMessage{
		ID:   "sub-1",
		Type: msgTypeComplete,
	})

	// May receive a complete message from the server, but should not receive more data events
	// Read any remaining messages
	dataEventsAfterUnsubscribe := 0
	for i := 0; i < 5; i++ {
		msg, err := readWSMessageWithTimeout(t, conn, 200*time.Millisecond)
		if err != nil {
			break // Timeout is expected
		}
		if msg.Type == msgTypeNext {
			dataEventsAfterUnsubscribe++
		}
		// Complete messages are expected, ignore them
	}

	// Should not receive any more data events after unsubscribe
	if dataEventsAfterUnsubscribe > 0 {
		t.Errorf("received %d data events after unsubscribe", dataEventsAfterUnsubscribe)
	}
}

func TestSubscriptionHandler_LegacyUnsubscribe(t *testing.T) {
	config := &GraphQLConfig{
		Subscriptions: map[string]SubscriptionConfig{
			"messageAdded": {
				Events: []EventConfig{
					{Data: map[string]interface{}{"id": "1"}, Delay: "100ms"},
					{Data: map[string]interface{}{"id": "2"}, Delay: "100ms"},
				},
			},
		},
	}

	handler := newTestSubscriptionHandler(t, config)
	ts := setupTestWebSocketServer(t, handler)

	conn := connectWS(t, ts.URL, "graphql-ws")

	// Init and subscribe
	sendWSMessage(t, conn, &wsMessage{Type: msgTypeConnectionInit})
	readWSMessage(t, conn) // connection_ack
	readWSMessage(t, conn) // ka

	payload, _ := json.Marshal(subscribePayload{
		Query: `subscription { messageAdded(channel: "test") { id } }`,
	})
	sendWSMessage(t, conn, &wsMessage{
		ID:      "sub-1",
		Type:    msgTypeStart,
		Payload: payload,
	})

	// Read first event
	readWSMessage(t, conn)

	// Unsubscribe using "stop" (legacy)
	sendWSMessage(t, conn, &wsMessage{
		ID:   "sub-1",
		Type: msgTypeStop,
	})

	// May receive a complete message from the server, but should not receive more data events
	dataEventsAfterStop := 0
	for i := 0; i < 5; i++ {
		msg, err := readWSMessageWithTimeout(t, conn, 200*time.Millisecond)
		if err != nil {
			break // Timeout is expected
		}
		if msg.Type == msgTypeData {
			dataEventsAfterStop++
		}
		// Complete messages are expected, ignore them
	}

	// Should not receive any more data events after stop
	if dataEventsAfterStop > 0 {
		t.Errorf("received %d data events after stop", dataEventsAfterStop)
	}
}

// ============================================================================
// Error Handling Tests
// ============================================================================

func TestSubscriptionHandler_UnknownSubscription(t *testing.T) {
	config := &GraphQLConfig{
		Subscriptions: map[string]SubscriptionConfig{},
	}

	handler := newTestSubscriptionHandler(t, config)
	ts := setupTestWebSocketServer(t, handler)

	conn := connectWS(t, ts.URL, "graphql-transport-ws")

	// Init and subscribe to non-existent subscription
	sendWSMessage(t, conn, &wsMessage{Type: msgTypeConnectionInit})
	readWSMessage(t, conn) // connection_ack

	payload, _ := json.Marshal(subscribePayload{
		Query: `subscription { nonExistent { id } }`,
	})
	sendWSMessage(t, conn, &wsMessage{
		ID:      "sub-1",
		Type:    msgTypeSubscribe,
		Payload: payload,
	})

	// Should receive error
	resp := readWSMessage(t, conn)
	if resp.Type != msgTypeError {
		t.Errorf("expected error, got %s", resp.Type)
	}
}

func TestSubscriptionHandler_MissingSubscriptionID(t *testing.T) {
	config := &GraphQLConfig{
		Subscriptions: map[string]SubscriptionConfig{
			"messageAdded": {
				Events: []EventConfig{
					{Data: map[string]interface{}{"id": "1"}},
				},
			},
		},
	}

	handler := newTestSubscriptionHandler(t, config)
	ts := setupTestWebSocketServer(t, handler)

	conn := connectWS(t, ts.URL, "graphql-transport-ws")

	// Init and subscribe without ID
	sendWSMessage(t, conn, &wsMessage{Type: msgTypeConnectionInit})
	readWSMessage(t, conn) // connection_ack

	payload, _ := json.Marshal(subscribePayload{
		Query: `subscription { messageAdded(channel: "test") { id } }`,
	})
	sendWSMessage(t, conn, &wsMessage{
		ID:      "", // Missing ID
		Type:    msgTypeSubscribe,
		Payload: payload,
	})

	// Should receive error
	resp := readWSMessage(t, conn)
	if resp.Type != msgTypeError {
		t.Errorf("expected error, got %s", resp.Type)
	}
}

func TestSubscriptionHandler_DuplicateSubscriptionID(t *testing.T) {
	config := &GraphQLConfig{
		Subscriptions: map[string]SubscriptionConfig{
			"messageAdded": {
				Events: []EventConfig{
					{Data: map[string]interface{}{"id": "1"}, Delay: "500ms"},
					{Data: map[string]interface{}{"id": "2"}, Delay: "500ms"},
				},
			},
		},
	}

	handler := newTestSubscriptionHandler(t, config)
	ts := setupTestWebSocketServer(t, handler)

	conn := connectWS(t, ts.URL, "graphql-transport-ws")

	// Init connection
	sendWSMessage(t, conn, &wsMessage{Type: msgTypeConnectionInit})
	readWSMessage(t, conn) // connection_ack

	payload, _ := json.Marshal(subscribePayload{
		Query: `subscription { messageAdded(channel: "test") { id } }`,
	})

	// First subscription
	sendWSMessage(t, conn, &wsMessage{
		ID:      "sub-1",
		Type:    msgTypeSubscribe,
		Payload: payload,
	})

	// Try to use same ID again
	sendWSMessage(t, conn, &wsMessage{
		ID:      "sub-1",
		Type:    msgTypeSubscribe,
		Payload: payload,
	})

	// First message might be an event or error, depends on timing
	// Read messages until we get an error
	foundError := false
	for i := 0; i < 3; i++ {
		msg, err := readWSMessageWithTimeout(t, conn, 200*time.Millisecond)
		if err != nil {
			break
		}
		if msg.Type == msgTypeError {
			foundError = true
			break
		}
	}

	if !foundError {
		t.Error("expected error for duplicate subscription ID")
	}
}

// ============================================================================
// Connection Management Tests
// ============================================================================

func TestSubscriptionHandler_ConnectionCount(t *testing.T) {
	config := &GraphQLConfig{}

	handler := newTestSubscriptionHandler(t, config)
	ts := setupTestWebSocketServer(t, handler)

	// Initial count should be 0
	if count := handler.ConnectionCount(); count != 0 {
		t.Errorf("expected 0 connections, got %d", count)
	}

	// Connect first client
	conn1 := connectWS(t, ts.URL, "graphql-transport-ws")
	sendWSMessage(t, conn1, &wsMessage{Type: msgTypeConnectionInit})
	readWSMessage(t, conn1)

	// Wait for connection to be registered
	time.Sleep(50 * time.Millisecond)

	if count := handler.ConnectionCount(); count != 1 {
		t.Errorf("expected 1 connection, got %d", count)
	}

	// Connect second client
	conn2 := connectWS(t, ts.URL, "graphql-transport-ws")
	sendWSMessage(t, conn2, &wsMessage{Type: msgTypeConnectionInit})
	readWSMessage(t, conn2)

	time.Sleep(50 * time.Millisecond)

	if count := handler.ConnectionCount(); count != 2 {
		t.Errorf("expected 2 connections, got %d", count)
	}
}

func TestSubscriptionHandler_SubscriptionCount(t *testing.T) {
	config := &GraphQLConfig{
		Subscriptions: map[string]SubscriptionConfig{
			"messageAdded": {
				Events: []EventConfig{
					{Data: map[string]interface{}{"id": "1"}, Delay: "1s"},
				},
				Timing: &TimingConfig{Repeat: true},
			},
		},
	}

	handler := newTestSubscriptionHandler(t, config)
	ts := setupTestWebSocketServer(t, handler)

	conn := connectWS(t, ts.URL, "graphql-transport-ws")
	sendWSMessage(t, conn, &wsMessage{Type: msgTypeConnectionInit})
	readWSMessage(t, conn) // connection_ack

	// Initial subscription count should be 0
	if count := handler.SubscriptionCount(); count != 0 {
		t.Errorf("expected 0 subscriptions, got %d", count)
	}

	// Subscribe
	payload, _ := json.Marshal(subscribePayload{
		Query: `subscription { messageAdded(channel: "test") { id } }`,
	})
	sendWSMessage(t, conn, &wsMessage{
		ID:      "sub-1",
		Type:    msgTypeSubscribe,
		Payload: payload,
	})

	time.Sleep(50 * time.Millisecond)

	if count := handler.SubscriptionCount(); count != 1 {
		t.Errorf("expected 1 subscription, got %d", count)
	}

	// Add another subscription
	sendWSMessage(t, conn, &wsMessage{
		ID:      "sub-2",
		Type:    msgTypeSubscribe,
		Payload: payload,
	})

	time.Sleep(50 * time.Millisecond)

	if count := handler.SubscriptionCount(); count != 2 {
		t.Errorf("expected 2 subscriptions, got %d", count)
	}
}

func TestSubscriptionHandler_CloseAll(t *testing.T) {
	config := &GraphQLConfig{
		Subscriptions: map[string]SubscriptionConfig{
			"messageAdded": {
				Events: []EventConfig{
					{Data: map[string]interface{}{"id": "1"}, Delay: "1s"},
				},
				Timing: &TimingConfig{Repeat: true},
			},
		},
	}

	handler := newTestSubscriptionHandler(t, config)
	ts := setupTestWebSocketServer(t, handler)

	// Connect and subscribe
	conn := connectWS(t, ts.URL, "graphql-transport-ws")
	sendWSMessage(t, conn, &wsMessage{Type: msgTypeConnectionInit})
	readWSMessage(t, conn)

	payload, _ := json.Marshal(subscribePayload{
		Query: `subscription { messageAdded(channel: "test") { id } }`,
	})
	sendWSMessage(t, conn, &wsMessage{
		ID:      "sub-1",
		Type:    msgTypeSubscribe,
		Payload: payload,
	})

	time.Sleep(50 * time.Millisecond)

	// Close all connections
	handler.CloseAll("server shutdown")

	time.Sleep(50 * time.Millisecond)

	// Connection count should be 0
	if count := handler.ConnectionCount(); count != 0 {
		t.Errorf("expected 0 connections after CloseAll, got %d", count)
	}
}

// ============================================================================
// Concurrent Tests
// ============================================================================

func TestSubscriptionHandler_ConcurrentSubscriptions(t *testing.T) {
	config := &GraphQLConfig{
		Subscriptions: map[string]SubscriptionConfig{
			"messageAdded": {
				Events: []EventConfig{
					{Data: map[string]interface{}{"id": "1"}},
					{Data: map[string]interface{}{"id": "2"}},
				},
			},
		},
	}

	handler := newTestSubscriptionHandler(t, config)
	ts := setupTestWebSocketServer(t, handler)

	const numClients = 5
	var wg sync.WaitGroup
	errors := make(chan error, numClients)

	for i := 0; i < numClients; i++ {
		wg.Add(1)
		go func(clientID int) {
			defer wg.Done()

			conn := connectWS(t, ts.URL, "graphql-transport-ws")
			sendWSMessage(t, conn, &wsMessage{Type: msgTypeConnectionInit})
			readWSMessage(t, conn) // connection_ack

			payload, _ := json.Marshal(subscribePayload{
				Query: `subscription { messageAdded(channel: "test") { id } }`,
			})
			sendWSMessage(t, conn, &wsMessage{
				ID:      "sub-1",
				Type:    msgTypeSubscribe,
				Payload: payload,
			})

			// Should receive events
			for j := 0; j < 2; j++ {
				msg, err := readWSMessageWithTimeout(t, conn, 5*time.Second)
				if err != nil {
					errors <- err
					return
				}
				if msg.Type != msgTypeNext {
					errors <- nil // Unexpected message type
					return
				}
			}

			// Should receive complete
			msg, err := readWSMessageWithTimeout(t, conn, 5*time.Second)
			if err != nil || msg.Type != msgTypeComplete {
				errors <- err
				return
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	for err := range errors {
		if err != nil {
			t.Errorf("concurrent client error: %v", err)
		}
	}
}

// ============================================================================
// Helper Function Tests
// ============================================================================

func TestParseRandomDelay(t *testing.T) {
	h := &SubscriptionHandler{}

	tests := []struct {
		name     string
		input    string
		minDelay time.Duration
		maxDelay time.Duration
	}{
		{"valid range", "100ms-500ms", 100 * time.Millisecond, 500 * time.Millisecond},
		{"same values", "100ms-100ms", 100 * time.Millisecond, 100 * time.Millisecond},
		{"invalid format", "invalid", 0, 0},
		{"empty string", "", 0, 0},
		{"single value", "100ms", 0, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			delay := h.parseRandomDelay(tt.input)
			if tt.minDelay == 0 && tt.maxDelay == 0 {
				if delay != 0 {
					t.Errorf("expected 0 for invalid input, got %v", delay)
				}
			} else {
				if delay < tt.minDelay || delay > tt.maxDelay {
					t.Errorf("delay %v not in range [%v, %v]", delay, tt.minDelay, tt.maxDelay)
				}
			}
		})
	}
}

func TestParseSubscriptionQuery(t *testing.T) {
	h := &SubscriptionHandler{}

	tests := []struct {
		name          string
		query         string
		inputVars     map[string]interface{}
		expectedField string
		expectError   bool
	}{
		{
			name:          "simple subscription",
			query:         `subscription { messageAdded { id } }`,
			expectedField: "messageAdded",
		},
		{
			name:          "named subscription",
			query:         `subscription OnMessage { messageAdded { id } }`,
			expectedField: "messageAdded",
		},
		{
			name:          "subscription with args",
			query:         `subscription { messageAdded(channel: "test") { id } }`,
			expectedField: "messageAdded",
		},
		{
			name:          "subscription with variables",
			query:         `subscription Sub($channel: String!) { messageAdded(channel: $channel) { id } }`,
			inputVars:     map[string]interface{}{"channel": "test"},
			expectedField: "messageAdded",
		},
		{
			name:        "invalid query",
			query:       `query { user { id } }`,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			field, _, err := h.parseSubscriptionQuery(tt.query, tt.inputVars)
			if tt.expectError {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}
			if field != tt.expectedField {
				t.Errorf("expected field %q, got %q", tt.expectedField, field)
			}
		})
	}
}

func TestApplyVariables(t *testing.T) {
	h := &SubscriptionHandler{}

	tests := []struct {
		name     string
		data     interface{}
		vars     map[string]interface{}
		expected interface{}
	}{
		{
			name:     "string substitution",
			data:     "Hello {{args.name}}!",
			vars:     map[string]interface{}{"name": "World"},
			expected: "Hello World!",
		},
		{
			name:     "vars prefix substitution",
			data:     "Channel: {{vars.channel}}",
			vars:     map[string]interface{}{"channel": "general"},
			expected: "Channel: general",
		},
		{
			name: "map substitution",
			data: map[string]interface{}{
				"text": "Message for {{args.user}}",
			},
			vars: map[string]interface{}{"user": "Alice"},
			expected: map[string]interface{}{
				"text": "Message for Alice",
			},
		},
		{
			name:     "no variables",
			data:     "Static text",
			vars:     map[string]interface{}{},
			expected: "Static text",
		},
		{
			name:     "nil data",
			data:     nil,
			vars:     map[string]interface{}{"x": "y"},
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := h.applyVariables(tt.data, tt.vars)

			// Compare as JSON for map types
			expectedJSON, _ := json.Marshal(tt.expected)
			resultJSON, _ := json.Marshal(result)

			if string(expectedJSON) != string(resultJSON) {
				t.Errorf("expected %v, got %v", string(expectedJSON), string(resultJSON))
			}
		})
	}
}
