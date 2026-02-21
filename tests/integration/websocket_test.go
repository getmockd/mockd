package integration

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	ws "github.com/coder/websocket"
	"github.com/getmockd/mockd/pkg/config"
	"github.com/getmockd/mockd/pkg/engine"
	"github.com/getmockd/mockd/pkg/mock"
	"github.com/getmockd/mockd/pkg/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ============================================================================
// Test Helpers
// ============================================================================

func setupTestServer(t *testing.T, wsEndpoints []*config.WebSocketEndpointConfig) *httptest.Server {
	cfg := config.DefaultServerConfiguration()
	srv := engine.NewServer(cfg)

	// Import WebSocket endpoints via collection
	if len(wsEndpoints) > 0 {
		collection := &config.MockCollection{
			Version:            "1.0",
			Name:               "ws-test",
			WebSocketEndpoints: wsEndpoints,
		}
		err := srv.ImportConfig(collection, true)
		require.NoError(t, err)
	}

	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(func() {
		ts.Close()
	})

	return ts
}

func connectWS(t *testing.T, url string) *ws.Conn {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, resp, err := ws.Dial(ctx, url, nil)
	if resp != nil && resp.Body != nil {
		resp.Body.Close()
	}
	require.NoError(t, err)
	t.Cleanup(func() {
		conn.Close(ws.StatusNormalClosure, "test cleanup")
	})

	return conn
}

func sendText(t *testing.T, conn *ws.Conn, msg string) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := conn.Write(ctx, ws.MessageText, []byte(msg))
	require.NoError(t, err)
}

func readText(t *testing.T, conn *ws.Conn) string {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	msgType, data, err := conn.Read(ctx)
	require.NoError(t, err)
	assert.Equal(t, ws.MessageText, msgType)
	return string(data)
}

// ============================================================================
// User Story 1: Basic WebSocket Connection and Message Echo
// ============================================================================

func TestWS_US1_BasicConnection(t *testing.T) {
	ts := setupTestServer(t, []*config.WebSocketEndpointConfig{
		{Path: "/ws/echo"},
	})

	// Convert http:// to ws://
	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/ws/echo"
	conn := connectWS(t, wsURL)

	// Connection should be established (test passes if we got here)
	assert.NotNil(t, conn)
}

func TestWS_US1_TextMessageEcho(t *testing.T) {
	echoMode := true
	ts := setupTestServer(t, []*config.WebSocketEndpointConfig{
		{Path: "/ws/echo", EchoMode: &echoMode},
	})

	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/ws/echo"
	conn := connectWS(t, wsURL)

	// Send message
	sendText(t, conn, "hello")

	// Should receive echo
	response := readText(t, conn)
	assert.Equal(t, "hello", response)
}

func TestWS_US1_BinaryMessageEcho(t *testing.T) {
	echoMode := true
	ts := setupTestServer(t, []*config.WebSocketEndpointConfig{
		{Path: "/ws/echo", EchoMode: &echoMode},
	})

	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/ws/echo"
	conn := connectWS(t, wsURL)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Send binary message
	binaryData := []byte{0x00, 0x01, 0x02, 0x03, 0xFF}
	err := conn.Write(ctx, ws.MessageBinary, binaryData)
	require.NoError(t, err)

	// Should receive binary echo
	msgType, data, err := conn.Read(ctx)
	require.NoError(t, err)
	assert.Equal(t, ws.MessageBinary, msgType)
	assert.Equal(t, binaryData, data)
}

func TestWS_US1_CloseFrame(t *testing.T) {
	ts := setupTestServer(t, []*config.WebSocketEndpointConfig{
		{Path: "/ws/echo"},
	})

	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/ws/echo"

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, resp, err := ws.Dial(ctx, wsURL, nil)
	if resp != nil && resp.Body != nil {
		resp.Body.Close()
	}
	require.NoError(t, err)

	// Close the connection
	err = conn.Close(ws.StatusNormalClosure, "client closing")
	assert.NoError(t, err)
}

func TestWS_US1_NonWebSocketRequest(t *testing.T) {
	ts := setupTestServer(t, []*config.WebSocketEndpointConfig{
		{Path: "/ws/echo"},
	})

	// Make regular HTTP request to WS endpoint
	// Without Upgrade header, this will fall through to mock matching which returns 404
	resp, err := http.Get(ts.URL + "/ws/echo")
	require.NoError(t, err)
	defer resp.Body.Close()

	// Should return 404 (no mock match) since it's not a WS upgrade request
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func TestWS_US1_EndpointNotFound(t *testing.T) {
	ts := setupTestServer(t, []*config.WebSocketEndpointConfig{
		{Path: "/ws/echo"},
	})

	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/ws/nonexistent"

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, resp, err := ws.Dial(ctx, wsURL, nil)
	if resp != nil && resp.Body != nil {
		resp.Body.Close()
	}
	assert.Error(t, err) // Should fail to connect
}

// ============================================================================
// User Story 2: Message Matching and Conditional Responses
// ============================================================================

func TestWS_US2_ExactMatch(t *testing.T) {
	ts := setupTestServer(t, []*config.WebSocketEndpointConfig{
		{
			Path: "/ws/chat",
			Matchers: []*mock.WSMatcherConfig{
				{
					Match:    &mock.WSMatchCriteria{Type: "exact", Value: "ping"},
					Response: &mock.WSMessageResponse{Type: "text", Value: "pong"},
				},
			},
		},
	})

	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/ws/chat"
	conn := connectWS(t, wsURL)

	sendText(t, conn, "ping")
	response := readText(t, conn)
	assert.Equal(t, "pong", response)
}

func TestWS_US2_RegexMatch(t *testing.T) {
	ts := setupTestServer(t, []*config.WebSocketEndpointConfig{
		{
			Path: "/ws/chat",
			Matchers: []*mock.WSMatcherConfig{
				{
					Match:    &mock.WSMatchCriteria{Type: "regex", Value: "^hello.*"},
					Response: &mock.WSMessageResponse{Type: "text", Value: "hi there!"},
				},
			},
		},
	})

	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/ws/chat"
	conn := connectWS(t, wsURL)

	sendText(t, conn, "hello world")
	response := readText(t, conn)
	assert.Equal(t, "hi there!", response)
}

func TestWS_US2_JSONMatch(t *testing.T) {
	ts := setupTestServer(t, []*config.WebSocketEndpointConfig{
		{
			Path: "/ws/api",
			Matchers: []*mock.WSMatcherConfig{
				{
					Match:    &mock.WSMatchCriteria{Type: "json", Path: "$.type", Value: "subscribe"},
					Response: &mock.WSMessageResponse{Type: "json", Value: map[string]interface{}{"status": "subscribed"}},
				},
			},
		},
	})

	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/ws/api"
	conn := connectWS(t, wsURL)

	sendText(t, conn, `{"type": "subscribe", "channel": "news"}`)
	response := readText(t, conn)
	assert.Contains(t, response, "subscribed")
}

func TestWS_US2_DefaultResponse(t *testing.T) {
	ts := setupTestServer(t, []*config.WebSocketEndpointConfig{
		{
			Path: "/ws/chat",
			Matchers: []*mock.WSMatcherConfig{
				{
					Match:    &mock.WSMatchCriteria{Type: "exact", Value: "ping"},
					Response: &mock.WSMessageResponse{Type: "text", Value: "pong"},
				},
			},
			DefaultResponse: &mock.WSMessageResponse{Type: "text", Value: "unknown command"},
		},
	})

	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/ws/chat"
	conn := connectWS(t, wsURL)

	sendText(t, conn, "something else")
	response := readText(t, conn)
	assert.Equal(t, "unknown command", response)
}

func TestWS_US2_FirstMatchWins(t *testing.T) {
	ts := setupTestServer(t, []*config.WebSocketEndpointConfig{
		{
			Path: "/ws/chat",
			Matchers: []*mock.WSMatcherConfig{
				{
					Match:    &mock.WSMatchCriteria{Type: "exact", Value: "test"},
					Response: &mock.WSMessageResponse{Type: "text", Value: "first"},
				},
				{
					Match:    &mock.WSMatchCriteria{Type: "contains", Value: "test"},
					Response: &mock.WSMessageResponse{Type: "text", Value: "second"},
				},
			},
		},
	})

	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/ws/chat"
	conn := connectWS(t, wsURL)

	sendText(t, conn, "test")
	response := readText(t, conn)
	assert.Equal(t, "first", response) // First matcher wins
}

// ============================================================================
// User Story 3: Message Scenarios and Sequences
// ============================================================================

func TestWS_US3_ScenarioSendsOnConnect(t *testing.T) {
	ts := setupTestServer(t, []*config.WebSocketEndpointConfig{
		{
			Path: "/ws/welcome",
			Scenario: &mock.WSScenarioConfig{
				Name: "welcome",
				Steps: []mock.WSScenarioStepConfig{
					{Type: "send", Message: &mock.WSMessageResponse{Type: "text", Value: "Welcome!"}},
				},
			},
		},
	})

	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/ws/welcome"
	conn := connectWS(t, wsURL)

	// Should receive welcome message
	response := readText(t, conn)
	assert.Equal(t, "Welcome!", response)
}

func TestWS_US3_ScenarioWithDelay(t *testing.T) {
	ts := setupTestServer(t, []*config.WebSocketEndpointConfig{
		{
			Path: "/ws/delayed",
			Scenario: &mock.WSScenarioConfig{
				Name: "delayed",
				Steps: []mock.WSScenarioStepConfig{
					{Type: "send", Message: &mock.WSMessageResponse{Type: "text", Value: "first"}},
					{Type: "wait", Duration: "100ms"},
					{Type: "send", Message: &mock.WSMessageResponse{Type: "text", Value: "second"}},
				},
			},
		},
	})

	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/ws/delayed"
	conn := connectWS(t, wsURL)

	start := time.Now()

	// Should receive first message
	first := readText(t, conn)
	assert.Equal(t, "first", first)

	// Should receive second message after delay
	second := readText(t, conn)
	assert.Equal(t, "second", second)

	elapsed := time.Since(start)
	assert.GreaterOrEqual(t, elapsed, 100*time.Millisecond)
}

// ============================================================================
// User Story 4: Connection State Management
// ============================================================================

func TestWS_US4_MaxConnections(t *testing.T) {
	ts := setupTestServer(t, []*config.WebSocketEndpointConfig{
		{Path: "/ws/limited", MaxConnections: 2},
	})

	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/ws/limited"

	ctx := context.Background()

	// Connect first two clients
	conn1, resp1, err := ws.Dial(ctx, wsURL, nil)
	if resp1 != nil && resp1.Body != nil {
		resp1.Body.Close()
	}
	require.NoError(t, err)
	defer conn1.Close(ws.StatusNormalClosure, "")

	// Small wait to ensure connection is registered
	time.Sleep(50 * time.Millisecond)

	conn2, resp2, err := ws.Dial(ctx, wsURL, nil)
	if resp2 != nil && resp2.Body != nil {
		resp2.Body.Close()
	}
	require.NoError(t, err)
	defer conn2.Close(ws.StatusNormalClosure, "")

	// Small wait to ensure connection is registered
	time.Sleep(50 * time.Millisecond)

	// Third connection should fail with 503 Service Unavailable
	ctx3, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, resp3, err := ws.Dial(ctx3, wsURL, nil)
	if resp3 != nil && resp3.Body != nil {
		resp3.Body.Close()
	}
	assert.Error(t, err) // Should be rejected
}

// ============================================================================
// User Story 5: Subprotocol Negotiation
// ============================================================================

func TestWS_US5_SubprotocolNegotiation(t *testing.T) {
	ts := setupTestServer(t, []*config.WebSocketEndpointConfig{
		{
			Path:         "/ws/proto",
			Subprotocols: []string{"json", "xml"},
		},
	})

	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/ws/proto"

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, resp, err := ws.Dial(ctx, wsURL, &ws.DialOptions{
		Subprotocols: []string{"json", "xml"},
	})
	require.NoError(t, err)
	defer func() {
		if resp != nil && resp.Body != nil {
			resp.Body.Close()
		}
		conn.Close(ws.StatusNormalClosure, "")
	}()

	// Server should select "json" (first match)
	assert.Equal(t, "json", resp.Header.Get("Sec-WebSocket-Protocol"))
}

func TestWS_US5_RequireSubprotocol(t *testing.T) {
	ts := setupTestServer(t, []*config.WebSocketEndpointConfig{
		{
			Path:               "/ws/required",
			Subprotocols:       []string{"graphql-ws"},
			RequireSubprotocol: true,
		},
	})

	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/ws/required"

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Connect without subprotocol - should fail
	_, resp, err := ws.Dial(ctx, wsURL, nil)
	if resp != nil && resp.Body != nil {
		resp.Body.Close()
	}
	assert.Error(t, err)
}

// ============================================================================
// User Story 6: Heartbeat and Ping/Pong
// ============================================================================

func TestWS_US6_ServerPing(t *testing.T) {
	ts := setupTestServer(t, []*config.WebSocketEndpointConfig{
		{
			Path: "/ws/heartbeat",
			Heartbeat: &mock.WSHeartbeatConfig{
				Enabled:  true,
				Interval: "100ms",
				Timeout:  "50ms",
			},
		},
	})

	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/ws/heartbeat"
	conn := connectWS(t, wsURL)

	// Wait a bit for heartbeat to occur
	time.Sleep(150 * time.Millisecond)

	// Connection should still be alive (pong is automatic)
	err := conn.Write(context.Background(), ws.MessageText, []byte("test"))
	assert.NoError(t, err)
}

// ============================================================================
// Unit Tests for Matcher
// ============================================================================

func TestMatcher_Exact(t *testing.T) {
	ts := setupTestServer(t, []*config.WebSocketEndpointConfig{
		{
			Path: "/ws/test",
			Matchers: []*mock.WSMatcherConfig{
				{
					Match:    &mock.WSMatchCriteria{Type: "exact", Value: "test"},
					Response: &mock.WSMessageResponse{Type: "text", Value: "matched"},
				},
			},
		},
	})

	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/ws/test"
	conn := connectWS(t, wsURL)

	// Exact match
	sendText(t, conn, "test")
	assert.Equal(t, "matched", readText(t, conn))
}

func TestMatcher_Contains(t *testing.T) {
	ts := setupTestServer(t, []*config.WebSocketEndpointConfig{
		{
			Path: "/ws/test",
			Matchers: []*mock.WSMatcherConfig{
				{
					Match:    &mock.WSMatchCriteria{Type: "contains", Value: "needle"},
					Response: &mock.WSMessageResponse{Type: "text", Value: "found"},
				},
			},
		},
	})

	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/ws/test"
	conn := connectWS(t, wsURL)

	sendText(t, conn, "haystack needle haystack")
	assert.Equal(t, "found", readText(t, conn))
}

// ============================================================================
// Admin API Tests for WebSocket
// ============================================================================

func TestWS_AdminAPI_ListConnections(t *testing.T) {
	echoMode := true
	ts := setupTestServer(t, []*config.WebSocketEndpointConfig{
		{Path: "/ws/test", EchoMode: &echoMode},
	})

	// Connect a WebSocket client
	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/ws/test"
	conn := connectWS(t, wsURL)

	// Give it a moment to register
	time.Sleep(50 * time.Millisecond)

	// Now check via Admin API (on same server for simplicity)
	// In real usage, this would be on the admin port
	// For this test, we verify the manager has the connection
	manager := getWSManager(t, ts)
	require.NotNil(t, manager)

	infos := manager.ListConnectionInfos("", "")
	assert.Equal(t, 1, len(infos))
	assert.Equal(t, "/ws/test", infos[0].EndpointPath)

	// Close the connection
	conn.Close(ws.StatusNormalClosure, "")
	time.Sleep(50 * time.Millisecond)

	// Should be gone
	infos = manager.ListConnectionInfos("", "")
	assert.Equal(t, 0, len(infos))
}

func TestWS_AdminAPI_SendMessage(t *testing.T) {
	ts := setupTestServer(t, []*config.WebSocketEndpointConfig{
		{Path: "/ws/test"},
	})

	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/ws/test"
	conn := connectWS(t, wsURL)
	time.Sleep(50 * time.Millisecond)

	manager := getWSManager(t, ts)
	infos := manager.ListConnectionInfos("", "")
	require.Equal(t, 1, len(infos))

	connID := infos[0].ID

	// Send a message via manager
	err := manager.SendToConnection(connID, websocket.MessageText, []byte("admin message"))
	require.NoError(t, err)

	// Should receive the message
	msg := readText(t, conn)
	assert.Equal(t, "admin message", msg)
}

func TestWS_AdminAPI_Disconnect(t *testing.T) {
	ts := setupTestServer(t, []*config.WebSocketEndpointConfig{
		{Path: "/ws/test"},
	})

	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/ws/test"

	ctx := context.Background()
	conn, resp, err := ws.Dial(ctx, wsURL, nil)
	if resp != nil && resp.Body != nil {
		resp.Body.Close()
	}
	require.NoError(t, err)

	time.Sleep(50 * time.Millisecond)

	manager := getWSManager(t, ts)
	infos := manager.ListConnectionInfos("", "")
	require.Equal(t, 1, len(infos))

	connID := infos[0].ID

	// Disconnect via manager
	err = manager.DisconnectByID(connID, websocket.CloseGoingAway, "admin kicked")
	require.NoError(t, err)

	// Try to read - should get error (connection closed)
	_, _, err = conn.Read(ctx)
	assert.Error(t, err)
}

func TestWS_AdminAPI_Broadcast(t *testing.T) {
	echoMode := false
	ts := setupTestServer(t, []*config.WebSocketEndpointConfig{
		{Path: "/ws/broadcast", EchoMode: &echoMode},
	})

	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/ws/broadcast"

	// Connect two clients
	conn1 := connectWS(t, wsURL)
	conn2 := connectWS(t, wsURL)
	time.Sleep(50 * time.Millisecond)

	manager := getWSManager(t, ts)

	// Broadcast to endpoint
	sent := manager.BroadcastToEndpoint("/ws/broadcast", websocket.MessageText, []byte("broadcast!"))
	assert.Equal(t, 2, sent)

	// Both should receive
	assert.Equal(t, "broadcast!", readText(t, conn1))
	assert.Equal(t, "broadcast!", readText(t, conn2))
}

func TestWS_AdminAPI_Stats(t *testing.T) {
	echoMode := true
	ts := setupTestServer(t, []*config.WebSocketEndpointConfig{
		{Path: "/ws/stats", EchoMode: &echoMode},
	})

	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/ws/stats"
	conn := connectWS(t, wsURL)

	// Send some messages
	sendText(t, conn, "one")
	readText(t, conn)
	sendText(t, conn, "two")
	readText(t, conn)

	time.Sleep(50 * time.Millisecond)

	manager := getWSManager(t, ts)
	stats := manager.WebSocketStats()

	assert.Equal(t, 1, stats.TotalConnections)
	assert.Equal(t, 1, stats.TotalEndpoints)
	// Messages sent/received should be tracked
}

func TestWS_AdminAPI_Groups(t *testing.T) {
	echoMode := false
	ts := setupTestServer(t, []*config.WebSocketEndpointConfig{
		{Path: "/ws/groups", EchoMode: &echoMode},
	})

	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/ws/groups"

	conn1 := connectWS(t, wsURL)
	time.Sleep(50 * time.Millisecond) // Wait for first connection to register

	conn2 := connectWS(t, wsURL)
	time.Sleep(50 * time.Millisecond) // Wait for second connection to register

	manager := getWSManager(t, ts)
	infos := manager.ListConnectionInfos("", "")
	require.Equal(t, 2, len(infos))

	// Add ALL connections to the group - this avoids order dependency issues
	for _, info := range infos {
		err := manager.JoinGroup(info.ID, "room:test")
		require.NoError(t, err)
	}

	// Broadcast to group - both should receive
	sent := manager.BroadcastToGroupRaw("room:test", websocket.MessageText, []byte("group msg"))
	assert.Equal(t, 2, sent)

	// Small delay to ensure messages are in-flight
	time.Sleep(50 * time.Millisecond)

	// Both connections should receive the message
	msg1 := readText(t, conn1)
	assert.Equal(t, "group msg", msg1)

	msg2 := readText(t, conn2)
	assert.Equal(t, "group msg", msg2)
}

// Helper to get WebSocketManager from test server
func getWSManager(t *testing.T, ts *httptest.Server) *websocket.ConnectionManager {
	// The test server has a Handler that's the engine handler
	// We need to access the WebSocketManager
	// This is a bit hacky for tests - in production you'd use the Admin API

	// Get underlying handler
	handler := ts.Config.Handler
	if handler == nil {
		t.Fatal("handler is nil")
	}

	// Type assert to engine.Handler
	engineHandler, ok := handler.(*engine.Handler)
	if !ok {
		t.Fatal("handler is not *engine.Handler")
	}

	return engineHandler.WebSocketManager()
}

// ============================================================================
// Full Server Integration Tests (with middleware chain)
// ============================================================================
// These tests use srv.Start() instead of httptest.NewServer(srv.Handler())
// to ensure the full middleware chain is tested, including metrics middleware.

func TestWS_FullServer_WithMiddleware(t *testing.T) {
	// This test ensures WebSocket works through the full middleware chain.
	// It specifically tests the http.Hijacker interface implementation in
	// metricsResponseWriter which was previously missing and caused HTTP 501.

	port := getFreePort()
	require.Greater(t, port, 0, "failed to get free port")
	cfg := config.DefaultServerConfiguration()
	cfg.HTTPPort = port
	cfg.ManagementPort = getFreePort()

	srv := engine.NewServer(cfg)

	// Import a WebSocket endpoint
	echoMode := true
	collection := &config.MockCollection{
		Version: "1.0",
		Name:    "ws-middleware-test",
		WebSocketEndpoints: []*config.WebSocketEndpointConfig{
			{Path: "/ws/echo", EchoMode: &echoMode},
		},
	}
	err := srv.ImportConfig(collection, true)
	require.NoError(t, err)

	// Start the FULL server (with middleware chain)
	err = srv.Start()
	require.NoError(t, err)
	t.Cleanup(func() {
		srv.Stop()
	})

	// Wait for server to be ready
	waitForReady(t, srv.ManagementPort())

	// Connect via WebSocket - this goes through the FULL middleware chain
	wsURL := "ws://localhost:" + itoa(port) + "/ws/echo"
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, resp, err := ws.Dial(ctx, wsURL, nil)
	if resp != nil && resp.Body != nil {
		defer resp.Body.Close()
	}
	if err != nil {
		if resp != nil {
			t.Fatalf("WebSocket connection failed with status %d: %v", resp.StatusCode, err)
		}
		t.Fatalf("WebSocket connection failed: %v", err)
	}
	defer conn.Close(ws.StatusNormalClosure, "test complete")

	// Verify echo works
	testMsg := "hello through middleware"
	err = conn.Write(ctx, ws.MessageText, []byte(testMsg))
	require.NoError(t, err)

	msgType, data, err := conn.Read(ctx)
	require.NoError(t, err)
	assert.Equal(t, ws.MessageText, msgType)
	assert.Equal(t, testMsg, string(data))
}

// itoa converts int to string without importing strconv.
func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	if i < 0 {
		return "-" + itoa(-i)
	}
	digits := make([]byte, 0, 10)
	for i > 0 {
		digits = append(digits, byte('0'+i%10))
		i /= 10
	}
	for left, right := 0, len(digits)-1; left < right; left, right = left+1, right-1 {
		digits[left], digits[right] = digits[right], digits[left]
	}
	return string(digits)
}
