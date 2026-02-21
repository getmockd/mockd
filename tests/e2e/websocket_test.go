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
		req, err := http.NewRequest(method, urlStr, bytes.NewBuffer(body))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/json")
		resp, err := client.Do(req)
		require.NoError(t, err)

		if resp.StatusCode >= 400 {
			b, _ := ioutil.ReadAll(resp.Body)
			t.Logf("API Error %s %s -> %d : %s", method, urlStr, resp.StatusCode, string(b))
			resp.Body = ioutil.NopCloser(bytes.NewBuffer(b))
		}

		return resp
	}

	// Setup: Create WebSocket Mocks
	resp1 := apiReq("POST", "/mocks", []byte(`{
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
	resp1.Body.Close()

	resp2 := apiReq("POST", "/mocks", []byte(`{
		"type": "websocket",
		"name": "Echo Only WS",
		"websocket": {"path": "/ws/echo", "echoMode": true}
	}`))
	resp2.Body.Close()

	t.Run("Create WebSocket mock returns 201", func(t *testing.T) {
		resp := apiReq("POST", "/mocks", []byte(`{
			"type": "websocket",
			"name": "WS Verify",
			"websocket": {"path": "/ws/verify", "echoMode": true}
		}`))
		require.Equal(t, 201, resp.StatusCode)

		var mock struct {
			ID string `json:"id"`
		}
		json.NewDecoder(resp.Body).Decode(&mock)
		resp.Body.Close()

		respD := apiReq("DELETE", "/mocks/"+mock.ID, nil)
		respD.Body.Close()
	})

	t.Run("Handlers list returns 200", func(t *testing.T) {
		resp := apiReq("GET", "/handlers", nil)
		resp.Body.Close()
		assert.Equal(t, 200, resp.StatusCode)
	})

	t.Run("Echo mode returns sent message", func(t *testing.T) {
		c, resp, err := websocket.DefaultDialer.Dial(wsTargetURL+"/ws/echo", nil)
		if resp != nil {
			defer resp.Body.Close()
		}
		require.NoError(t, err)
		defer c.Close()

		err = c.WriteMessage(websocket.TextMessage, []byte("hello e2e"))
		require.NoError(t, err)

		_, message, err := c.ReadMessage()
		require.NoError(t, err)
		assert.Equal(t, "hello e2e", string(message))
	})

	t.Run("Matcher responds with pong for ping", func(t *testing.T) {
		c, resp, err := websocket.DefaultDialer.Dial(wsTargetURL+"/ws/test", nil)
		if resp != nil {
			defer resp.Body.Close()
		}
		require.NoError(t, err)
		defer c.Close()

		err = c.WriteMessage(websocket.TextMessage, []byte("ping"))
		require.NoError(t, err)

		_, message, err := c.ReadMessage()
		require.NoError(t, err)
		assert.Equal(t, "pong", string(message))
	})

	t.Run("Default response for unmatched message", func(t *testing.T) {
		c, resp, err := websocket.DefaultDialer.Dial(wsTargetURL+"/ws/test", nil)
		if resp != nil {
			defer resp.Body.Close()
		}
		require.NoError(t, err)
		defer c.Close()

		err = c.WriteMessage(websocket.TextMessage, []byte("something-random"))
		require.NoError(t, err)

		_, message, err := c.ReadMessage()
		require.NoError(t, err)
		assert.Equal(t, "unknown", string(message))
	})

	t.Run("Multiple messages over single connection", func(t *testing.T) {
		c, resp, err := websocket.DefaultDialer.Dial(wsTargetURL+"/ws/echo", nil)
		if resp != nil {
			defer resp.Body.Close()
		}
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
		if resp != nil {
			defer resp.Body.Close()
		}
		require.Error(t, err)
		if resp != nil {
			assert.Equal(t, 404, resp.StatusCode)
		}
	})
}
