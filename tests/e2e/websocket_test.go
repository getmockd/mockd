package e2e_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"
	"testing"
	"time"

	"github.com/getmockd/mockd/pkg/admin"
	"github.com/getmockd/mockd/pkg/admin/engineclient"
	"github.com/getmockd/mockd/pkg/config"
	"github.com/getmockd/mockd/pkg/engine"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWebSocketProtocolIntegration(t *testing.T) {
	port := getFreePort(t)
	controlPort := getFreePort(t)
	adminPort := getFreePort(t)

	cfg := &config.ServerConfiguration{
		HTTPPort:       port,
		ManagementPort: controlPort,
	}

	server := engine.NewServer(cfg)
	go func() {
		_ = server.Start()
	}()
	defer server.Stop()

	adminURL := "http://localhost:" + strconv.Itoa(adminPort)
	engineURL := "http://localhost:" + strconv.Itoa(controlPort)
	wsTargetURL := fmt.Sprintf("ws://localhost:%d", port)

	engClient := engineclient.New(engineURL)

	adminAPI := admin.NewAPI(adminPort,
		admin.WithLocalEngine(engineURL),
		admin.WithAPIKeyDisabled(),
		admin.WithDataDir(t.TempDir()),
	)
	adminAPI.SetLocalEngine(engClient)

	go func() {
		_ = adminAPI.Start()
	}()
	defer adminAPI.Stop()

	waitForServer(t, adminURL+"/health")
	waitForServer(t, engineURL+"/health")

	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	apiReq := func(method, path string, body []byte) *http.Response {
		urlStr := adminURL + path
		req, _ := http.NewRequest(method, urlStr, bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		resp, _ := client.Do(req)

		if resp.StatusCode >= 400 {
			b, _ := ioutil.ReadAll(resp.Body)
			t.Logf("API Error %s %s -> %d : %s", method, urlStr, resp.StatusCode, string(b))
			resp.Body = ioutil.NopCloser(bytes.NewBuffer(b))
		}

		return resp
	}

	// Setup: Create WebSocket Mocks
	apiReq("POST", "/mocks", []byte(`{
		"type": "websocket",
		"name": "Test WebSocket",
		"websocket": {
		  "path": "/ws/test",
		  "echoMode": true,
		  "matchers": [
			{
			  "match": {"type": "exact", "value": "ping"},
			  "response": {"type": "text", "value": "pong"}
			}
		  ],
		  "defaultResponse": {"type": "text", "value": "unknown"}
		}
	}`))

	apiReq("POST", "/mocks", []byte(`{
		"type": "websocket",
		"name": "Echo Only WS",
		"websocket": {"path": "/ws/echo", "echoMode": true}
	}`))

	t.Run("Create WebSocket mock returns 201", func(t *testing.T) {
		resp := apiReq("POST", "/mocks", []byte(`{
			"type": "websocket",
			"name": "WS Verify",
			"websocket": {"path": "/ws/verify", "echoMode": true}
		}`))
		require.Equal(t, 201, resp.StatusCode)

		var mock struct{ ID string `json:"id"` }
		json.NewDecoder(resp.Body).Decode(&mock)

		apiReq("DELETE", "/mocks/"+mock.ID, nil)
	})

	t.Run("Handlers list returns 200", func(t *testing.T) {
		resp := apiReq("GET", "/handlers", nil)
		assert.Equal(t, 200, resp.StatusCode)
	})

	t.Run("Echo mode returns sent message", func(t *testing.T) {
		c, _, err := websocket.DefaultDialer.Dial(wsTargetURL+"/ws/echo", nil)
		require.NoError(t, err)
		defer c.Close()

		err = c.WriteMessage(websocket.TextMessage, []byte("hello e2e"))
		require.NoError(t, err)

		_, message, err := c.ReadMessage()
		require.NoError(t, err)
		assert.Equal(t, "hello e2e", string(message))
	})

	t.Run("Matcher responds with pong for ping", func(t *testing.T) {
		c, _, err := websocket.DefaultDialer.Dial(wsTargetURL+"/ws/test", nil)
		require.NoError(t, err)
		defer c.Close()

		err = c.WriteMessage(websocket.TextMessage, []byte("ping"))
		require.NoError(t, err)

		_, message, err := c.ReadMessage()
		require.NoError(t, err)
		assert.Equal(t, "pong", string(message))
	})

	t.Run("Default response for unmatched message", func(t *testing.T) {
		c, _, err := websocket.DefaultDialer.Dial(wsTargetURL+"/ws/test", nil)
		require.NoError(t, err)
		defer c.Close()

		err = c.WriteMessage(websocket.TextMessage, []byte("something-random"))
		require.NoError(t, err)

		_, message, err := c.ReadMessage()
		require.NoError(t, err)
		assert.Equal(t, "unknown", string(message))
	})

	t.Run("Multiple messages over single connection", func(t *testing.T) {
		c, _, err := websocket.DefaultDialer.Dial(wsTargetURL+"/ws/echo", nil)
		require.NoError(t, err)
		defer c.Close()

		err = c.WriteMessage(websocket.TextMessage, []byte("msg1"))
		require.NoError(t, err)
		_, message1, _ := c.ReadMessage()
		assert.Equal(t, "msg1", string(message1))

		err = c.WriteMessage(websocket.TextMessage, []byte("msg2"))
		require.NoError(t, err)
		_, message2, _ := c.ReadMessage()
		assert.Equal(t, "msg2", string(message2))
	})

	t.Run("Connection to non-existent WS path fails", func(t *testing.T) {
		_, resp, err := websocket.DefaultDialer.Dial(wsTargetURL+"/ws/no-such-path", nil)
		require.Error(t, err)
		if resp != nil {
			assert.Equal(t, 404, resp.StatusCode)
		}
	})
}
