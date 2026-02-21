package e2e_test

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"strconv"
	"testing"

	"github.com/getmockd/mockd/pkg/admin"
	"github.com/getmockd/mockd/pkg/admin/engineclient"
	"github.com/getmockd/mockd/pkg/config"
	"github.com/getmockd/mockd/pkg/engine"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestAPIIntegration handles the port of all api_*.bats tests to native Go
func TestAPIIntegration(t *testing.T) {
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

	client := &http.Client{}

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

	engineReq := func(method, path string, body []byte) *http.Response {
		req, _ := http.NewRequest(method, mockTargetURL+path, bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		resp, _ := client.Do(req)
		return resp
	}
	t.Run("Folders", func(t *testing.T) {
		resp := apiReq("GET", "/folders", nil)
		assert.Equal(t, 200, resp.StatusCode)

		resp = apiReq("POST", "/folders", []byte(`{"name": "Test Folder"}`))
		assert.Equal(t, 201, resp.StatusCode)
		
		var folder struct { ID string `json:"id"` }
		json.NewDecoder(resp.Body).Decode(&folder)
		
		resp = apiReq("GET", "/folders/"+folder.ID, nil)
		assert.Equal(t, 200, resp.StatusCode)
		
		resp = apiReq("DELETE", "/folders/"+folder.ID, nil)
		assert.Equal(t, 204, resp.StatusCode)
	})

	t.Run("Negative", func(t *testing.T) {
		resp := apiReq("POST", "/mocks", []byte(`not json`))
		assert.Equal(t, 400, resp.StatusCode)

		resp = apiReq("GET", "/mocks/nonexistent", nil)
		assert.Equal(t, 404, resp.StatusCode)
	})

	t.Run("HTTP Core Execution", func(t *testing.T) {
		apiReq("DELETE", "/mocks", nil)

		apiReq("POST", "/mocks", []byte(`{
			"type": "http",
			"name": "Body Matcher",
			"http": {
			  "matcher": {"method": "POST", "path": "/api/echo", "bodyContains": "hello"},
			  "response": {"statusCode": 200, "body": "{\"echoed\": true}"}
			}
		}`))

		resp := engineReq("POST", "/api/echo", []byte(`hello world`))
		assert.Equal(t, 200, resp.StatusCode)
		b, _ := ioutil.ReadAll(resp.Body)
		assert.Contains(t, string(b), "echoed")
		
		resp = engineReq("GET", "/api/no-such-endpoint", nil)
		assert.Equal(t, 404, resp.StatusCode)
	})

	t.Run("Mock Operations", func(t *testing.T) {
		apiReq("DELETE", "/mocks", nil)
		
		resp := apiReq("POST", "/mocks/bulk", []byte(`[
			{
			  "type": "http",
			  "name": "Bulk Mock 1",
			  "http": {
				"matcher": {"method": "GET", "path": "/api/bulk1"},
				"response": {"statusCode": 200}
			  }
			}
		]`))
		assert.True(t, resp.StatusCode == 200 || resp.StatusCode == 201)

		resp = engineReq("GET", "/api/bulk1", nil)
		assert.Equal(t, 200, resp.StatusCode)
	})
	
	t.Run("Proxy Contexts", func(t *testing.T) {
		resp := apiReq("GET", "/proxy/status", nil)
		assert.Equal(t, 200, resp.StatusCode)
		resp = apiReq("GET", "/proxy/filters", nil)
		assert.Equal(t, 200, resp.StatusCode)
	})

	t.Run("State Management", func(t *testing.T) {
		resp := apiReq("GET", "/state", nil)
		assert.Equal(t, 200, resp.StatusCode)
		resp = apiReq("GET", "/state/resources", nil)
		assert.Equal(t, 200, resp.StatusCode)
	})

	t.Run("Workspaces", func(t *testing.T) {
		resp := apiReq("POST", "/workspaces", []byte(`{"name": "test-ws"}`))
		assert.Equal(t, 201, resp.StatusCode)
		
		var ws struct { ID string `json:"id"` }
		json.NewDecoder(resp.Body).Decode(&ws)

		resp = apiReq("GET", "/workspaces/"+ws.ID, nil)
		assert.Equal(t, 200, resp.StatusCode)
		
		resp = apiReq("DELETE", "/workspaces/"+ws.ID, nil)
		assert.Equal(t, 204, resp.StatusCode)
	})

	t.Run("Metadata & Output Structures", func(t *testing.T) {
		endpoints := []string{"/formats", "/templates", "/openapi.json", "/openapi.yaml", "/insomnia.json", "/insomnia.yaml", "/grpc", "/mqtt", "/soap"}
		for _, ep := range endpoints {
			resp := apiReq("GET", ep, nil)
			require.Equal(t, 200, resp.StatusCode, "Endpoint %s failed", ep)
		}
	})
}
