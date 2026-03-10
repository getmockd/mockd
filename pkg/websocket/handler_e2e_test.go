package websocket

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	gorillaWs "github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// dialWS is a test helper that dials a WebSocket endpoint on the given test server.
// It returns the gorilla connection, the HTTP response, and any error.
func dialWS(t *testing.T, ts *httptest.Server, path string) (*gorillaWs.Conn, *http.Response, error) {
	t.Helper()
	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + path
	return gorillaWs.DefaultDialer.Dial(wsURL, nil)
}

// setupHandler creates a ConnectionManager, registers the given endpoints,
// creates a WebSocketHandler, and starts an httptest.Server.
// The caller must call ts.Close() when done.
func setupHandler(t *testing.T, endpoints ...*Endpoint) (*httptest.Server, *ConnectionManager) {
	t.Helper()
	manager := NewConnectionManager()
	for _, ep := range endpoints {
		manager.RegisterEndpoint(ep)
	}
	handler := NewWebSocketHandler(manager)
	ts := httptest.NewServer(handler)
	return ts, manager
}

func TestHandlerE2E_EchoMode(t *testing.T) {
	// Echo mode is the default when no matchers are configured.
	// Send a text message and expect the same text back.
	endpoint, err := NewEndpoint(&EndpointConfig{
		Path: "/ws/echo",
	})
	require.NoError(t, err)

	ts, _ := setupHandler(t, endpoint)
	defer ts.Close()

	conn, _, err := dialWS(t, ts, "/ws/echo")
	require.NoError(t, err)
	defer conn.Close()

	// Send a message
	err = conn.WriteMessage(gorillaWs.TextMessage, []byte("hello"))
	require.NoError(t, err)

	// Read the echoed response
	msgType, msg, err := conn.ReadMessage()
	require.NoError(t, err)
	assert.Equal(t, gorillaWs.TextMessage, msgType)
	assert.Equal(t, "hello", string(msg))

	// Verify with a second message
	err = conn.WriteMessage(gorillaWs.TextMessage, []byte("world"))
	require.NoError(t, err)

	msgType, msg, err = conn.ReadMessage()
	require.NoError(t, err)
	assert.Equal(t, gorillaWs.TextMessage, msgType)
	assert.Equal(t, "world", string(msg))
}

func TestHandlerE2E_ExactMatcher(t *testing.T) {
	// Create an endpoint with an exact matcher: "ping" → "pong".
	endpoint, err := NewEndpoint(&EndpointConfig{
		Path: "/ws/matcher",
		Matchers: []*MatcherConfig{
			{
				Match:    &MatchCriteria{Type: "exact", Value: "ping"},
				Response: &MessageResponse{Type: "text", Value: "pong"},
			},
		},
	})
	require.NoError(t, err)

	ts, _ := setupHandler(t, endpoint)
	defer ts.Close()

	conn, _, err := dialWS(t, ts, "/ws/matcher")
	require.NoError(t, err)
	defer conn.Close()

	err = conn.WriteMessage(gorillaWs.TextMessage, []byte("ping"))
	require.NoError(t, err)

	msgType, msg, err := conn.ReadMessage()
	require.NoError(t, err)
	assert.Equal(t, gorillaWs.TextMessage, msgType)
	assert.Equal(t, "pong", string(msg))
}

func TestHandlerE2E_ContainsMatcher(t *testing.T) {
	// Matcher with type "contains" value "help" should match "can you help me".
	endpoint, err := NewEndpoint(&EndpointConfig{
		Path: "/ws/contains",
		Matchers: []*MatcherConfig{
			{
				Match:    &MatchCriteria{Type: "contains", Value: "help"},
				Response: &MessageResponse{Type: "text", Value: "How can I assist you?"},
			},
		},
	})
	require.NoError(t, err)

	ts, _ := setupHandler(t, endpoint)
	defer ts.Close()

	conn, _, err := dialWS(t, ts, "/ws/contains")
	require.NoError(t, err)
	defer conn.Close()

	err = conn.WriteMessage(gorillaWs.TextMessage, []byte("can you help me"))
	require.NoError(t, err)

	msgType, msg, err := conn.ReadMessage()
	require.NoError(t, err)
	assert.Equal(t, gorillaWs.TextMessage, msgType)
	assert.Equal(t, "How can I assist you?", string(msg))
}

func TestHandlerE2E_DefaultResponse(t *testing.T) {
	// Endpoint with matchers + DefaultResponse.
	// An unmatched message should return the default response.
	endpoint, err := NewEndpoint(&EndpointConfig{
		Path: "/ws/default",
		Matchers: []*MatcherConfig{
			{
				Match:    &MatchCriteria{Type: "exact", Value: "ping"},
				Response: &MessageResponse{Type: "text", Value: "pong"},
			},
		},
		DefaultResponse: &MessageResponse{
			Type:  "text",
			Value: "unknown command",
		},
	})
	require.NoError(t, err)

	ts, _ := setupHandler(t, endpoint)
	defer ts.Close()

	conn, _, err := dialWS(t, ts, "/ws/default")
	require.NoError(t, err)
	defer conn.Close()

	// Send an unmatched message
	err = conn.WriteMessage(gorillaWs.TextMessage, []byte("something random"))
	require.NoError(t, err)

	msgType, msg, err := conn.ReadMessage()
	require.NoError(t, err)
	assert.Equal(t, gorillaWs.TextMessage, msgType)
	assert.Equal(t, "unknown command", string(msg))

	// Verify the specific matcher still works
	err = conn.WriteMessage(gorillaWs.TextMessage, []byte("ping"))
	require.NoError(t, err)

	msgType, msg, err = conn.ReadMessage()
	require.NoError(t, err)
	assert.Equal(t, gorillaWs.TextMessage, msgType)
	assert.Equal(t, "pong", string(msg))
}

func TestHandlerE2E_NonWebSocketRequest(t *testing.T) {
	// A plain HTTP GET (no upgrade headers) should return 400.
	endpoint, err := NewEndpoint(&EndpointConfig{
		Path: "/ws/test",
	})
	require.NoError(t, err)

	ts, _ := setupHandler(t, endpoint)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/ws/test")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	assert.Contains(t, string(body), "WebSocket upgrade required")
}

func TestHandlerE2E_UnknownPath(t *testing.T) {
	// Dialing a path with no endpoint registered should fail with 404.
	endpoint, err := NewEndpoint(&EndpointConfig{
		Path: "/ws/registered",
	})
	require.NoError(t, err)

	ts, _ := setupHandler(t, endpoint)
	defer ts.Close()

	conn, resp, err := dialWS(t, ts, "/ws/not-registered")
	if conn != nil {
		conn.Close()
	}
	require.Error(t, err)
	require.NotNil(t, resp, "expected HTTP response from failed dial")
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func TestHandlerE2E_DisabledEndpoint(t *testing.T) {
	// A disabled endpoint should return 503.
	endpoint, err := NewEndpoint(&EndpointConfig{
		Path: "/ws/disabled",
	})
	require.NoError(t, err)

	endpoint.SetEnabled(false)

	ts, _ := setupHandler(t, endpoint)
	defer ts.Close()

	conn, resp, err := dialWS(t, ts, "/ws/disabled")
	if conn != nil {
		conn.Close()
	}
	require.Error(t, err)
	require.NotNil(t, resp, "expected HTTP response from failed dial")
	assert.Equal(t, http.StatusServiceUnavailable, resp.StatusCode)
}

func TestHandlerE2E_MaxConnections(t *testing.T) {
	// With MaxConnections: 1, the first connection succeeds and the second
	// should be rejected with 503.
	endpoint, err := NewEndpoint(&EndpointConfig{
		Path:           "/ws/limited",
		MaxConnections: 1,
	})
	require.NoError(t, err)

	ts, _ := setupHandler(t, endpoint)
	defer ts.Close()

	// First connection should succeed
	conn1, _, err := dialWS(t, ts, "/ws/limited")
	require.NoError(t, err)

	// Second connection should be rejected
	conn2, resp, err := dialWS(t, ts, "/ws/limited")
	if conn2 != nil {
		conn2.Close()
	}
	require.Error(t, err)
	require.NotNil(t, resp, "expected HTTP response from rejected dial")
	assert.Equal(t, http.StatusServiceUnavailable, resp.StatusCode)

	// Close the first connection
	conn1.Close()

	// Wait briefly for the server to process the disconnect and free the slot.
	// The server-side handleConnection goroutine needs time to run its deferred
	// cleanup (RemoveConnection) after detecting the closed connection.
	require.Eventually(t, func() bool {
		return endpoint.ConnectionCount() == 0
	}, 2*time.Second, 50*time.Millisecond, "expected connection count to reach 0 after close")

	// Now a new connection should succeed
	conn3, _, err := dialWS(t, ts, "/ws/limited")
	require.NoError(t, err)
	defer conn3.Close()

	// Verify the new connection works with echo
	err = conn3.WriteMessage(gorillaWs.TextMessage, []byte("after-reconnect"))
	require.NoError(t, err)

	msgType, msg, err := conn3.ReadMessage()
	require.NoError(t, err)
	assert.Equal(t, gorillaWs.TextMessage, msgType)
	assert.Equal(t, "after-reconnect", string(msg))
}
