package e2e_test

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/getmockd/mockd/pkg/admin"
	"github.com/getmockd/mockd/pkg/admin/engineclient"
	"github.com/getmockd/mockd/pkg/config"
	"github.com/getmockd/mockd/pkg/engine"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSSEProtocolIntegration(t *testing.T) {
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
	mockTargetURL := "http://localhost:" + strconv.Itoa(port)

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

	// Setup: Create SSE Mock
	resp1 := apiReq("POST", "/mocks", []byte(`{
		"type": "http",
		"name": "SSE Event Stream",
		"http": {
		  "matcher": {"method": "GET", "path": "/events"},
		  "sse": {
			"events": [
			  {"type": "message", "data": {"text": "hello"}, "id": "1"},
			  {"type": "message", "data": {"text": "world"}, "id": "2"}
			],
			"timing": {"fixedDelay": 10},
			"lifecycle": {"maxEvents": 2}
		  }
		}
	}`))
	resp1.Body.Close()

	// Create a typed-events SSE mock
	resp2 := apiReq("POST", "/mocks", []byte(`{
		"type": "http",
		"name": "SSE Typed Events",
		"http": {
		  "matcher": {"method": "GET", "path": "/typed-events"},
		  "sse": {
			"events": [
			  {"type": "notification", "data": {"msg": "alert"}, "id": "10"},
			  {"type": "heartbeat", "data": {"ts": 12345}, "id": "11"}
			],
			"timing": {"fixedDelay": 10},
			"lifecycle": {"maxEvents": 2}
		  }
		}
	}`))
	resp2.Body.Close()

	t.Run("Create SSE mock returns 201", func(t *testing.T) {
		resp := apiReq("POST", "/mocks", []byte(`{
			"type": "http",
			"name": "SSE Verify",
			"http": {
			  "matcher": {"method": "GET", "path": "/events-verify"},
			  "sse": {"events": [{"type": "message", "data": {"text": "test"}}], "lifecycle": {"maxEvents": 1}}
			}
		}`))
		require.Equal(t, 201, resp.StatusCode)

		var mock struct{ ID string `json:"id"` }
		json.NewDecoder(resp.Body).Decode(&mock)
		resp.Body.Close()

		respD := apiReq("DELETE", "/mocks/"+mock.ID, nil)
		respD.Body.Close()
	})

	// Helper to consume SSE stream
	consumeSSE := func(path string, lastEventID string) ([]string, string, error) {
		req, _ := http.NewRequest("GET", mockTargetURL+path, nil)
		req.Header.Set("Accept", "text/event-stream")
		if lastEventID != "" {
			req.Header.Set("Last-Event-ID", lastEventID)
		}

		// Use a client without tight timeout since SSE blocks
		sseClient := &http.Client{Timeout: 3 * time.Second}
		resp, err := sseClient.Do(req)
		if err != nil {
			return nil, "", err
		}
		defer resp.Body.Close()

		contentType := resp.Header.Get("Content-Type")

		var lines []string
		scanner := bufio.NewScanner(resp.Body)
		for scanner.Scan() {
			lines = append(lines, scanner.Text())
		}
		return lines, contentType, nil
	}

	t.Run("Received SSE events", func(t *testing.T) {
		lines, contentType, err := consumeSSE("/events", "")
		require.NoError(t, err)

		fullOutput := strings.Join(lines, "\n")
		assert.Contains(t, fullOutput, "hello")
		assert.Contains(t, contentType, "text/event-stream")
	})

	t.Run("Receives both event IDs", func(t *testing.T) {
		lines, _, err := consumeSSE("/events", "")
		require.NoError(t, err)

		fullOutput := strings.Join(lines, "\n")
		assert.Contains(t, fullOutput, "hello")
		assert.Contains(t, fullOutput, "world")
		assert.Contains(t, fullOutput, "id:1")
		assert.Contains(t, fullOutput, "id:2")
	})

	t.Run("Last-Event-ID reconnection resumes stream", func(t *testing.T) {
		lines, _, err := consumeSSE("/events", "1")
		require.NoError(t, err)

		fullOutput := strings.Join(lines, "\n")
		assert.Contains(t, fullOutput, "world")
	})

	t.Run("Wire Format Verification fields", func(t *testing.T) {
		lines, _, err := consumeSSE("/events", "")
		require.NoError(t, err)

		fullOutput := strings.Join(lines, "\n")
		assert.Contains(t, fullOutput, "event:")
		assert.Contains(t, fullOutput, "id:")
		assert.Contains(t, fullOutput, "data:")
	})

	t.Run("Typed events have correct event: type", func(t *testing.T) {
		lines, _, err := consumeSSE("/typed-events", "")
		require.NoError(t, err)

		fullOutput := strings.Join(lines, "\n")
		assert.Contains(t, fullOutput, "event:notification")
		assert.Contains(t, fullOutput, "event:heartbeat")
	})

	t.Run("Admin API test /sse/connections returns 200", func(t *testing.T) {
		resp := apiReq("GET", "/sse/connections", nil)
		resp.Body.Close()
		assert.Equal(t, 200, resp.StatusCode)
	})

	t.Run("Admin API test /sse/stats returns 200", func(t *testing.T) {
		resp := apiReq("GET", "/sse/stats", nil)
		resp.Body.Close()
		assert.Equal(t, 200, resp.StatusCode)
	})
}
