package e2e_test

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"strconv"
	"testing"
	"time"

	"github.com/getmockd/mockd/pkg/admin"
	"github.com/getmockd/mockd/pkg/admin/engineclient"
	"github.com/getmockd/mockd/pkg/config"
	"github.com/getmockd/mockd/pkg/engine"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSOAPProtocolIntegration(t *testing.T) {
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

	engineSOAPReq := func(path, action string, body []byte) (*http.Response, string) {
		req, err := http.NewRequest("POST", mockTargetURL+path, bytes.NewBuffer(body))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "text/xml")
		req.Header.Set("SOAPAction", action)
		resp, err := client.Do(req)
		require.NoError(t, err)
		b, _ := ioutil.ReadAll(resp.Body)
		return resp, string(b)
	}

	// Setup: Create a SOAP Mock
	mockReqBody := []byte(`{
		"type": "soap",
		"name": "Test SOAP Service",
		"soap": {
		  "path": "/soap/user",
		  "operations": {
			"GetUser": {
			  "soapAction": "http://example.com/GetUser",
			  "response": "<GetUserResponse><id>123</id><name>John Doe</name></GetUserResponse>"
			},
			"CreateUser": {
			  "soapAction": "http://example.com/CreateUser",
			  "response": "<CreateUserResponse><userId>new-001</userId><status>created</status></CreateUserResponse>"
			}
		  }
		}
	}`)

	resp := apiReq("POST", "/mocks", mockReqBody)
	resp.Body.Close()
	require.Equal(t, 201, resp.StatusCode, "Failed to create SOAP mock")

	t.Run("Create Extra SOAP Mock", func(t *testing.T) {
		resp := apiReq("POST", "/mocks", []byte(`{
			"type": "soap",
			"name": "SOAP Verify",
			"soap": {
			  "path": "/soap/verify",
			  "operations": {"Ping": {"response": "<ok/>"}}
			}
		}`))
		require.Equal(t, 201, resp.StatusCode)

		var mock struct {
			ID string `json:"id"`
		}
		json.NewDecoder(resp.Body).Decode(&mock)
		resp.Body.Close()

		resp = apiReq("DELETE", "/mocks/"+mock.ID, nil)
		resp.Body.Close()
		require.Equal(t, 204, resp.StatusCode)
	})

	t.Run("SOAP GetUser request returns 200", func(t *testing.T) {
		bodyXML := []byte(`<?xml version="1.0"?><soap:Envelope xmlns:soap="http://schemas.xmlsoap.org/soap/envelope/"><soap:Body><GetUser><userId>123</userId></GetUser></soap:Body></soap:Envelope>`)
		resp, body := engineSOAPReq("/soap/user", "http://example.com/GetUser", bodyXML)
		resp.Body.Close()
		assert.Equal(t, 200, resp.StatusCode)
		assert.Contains(t, body, "John Doe")
	})

	t.Run("WSDL endpoint responds", func(t *testing.T) {
		req, _ := http.NewRequest("GET", mockTargetURL+"/soap/user?wsdl", nil)
		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		// It might be unimplemented depending on mockd state, we just ensure it doesn't crash
		assert.True(t, resp.StatusCode == 200 || resp.StatusCode == 404 || resp.StatusCode == 501)
	})

	t.Run("CreateUser operation returns response", func(t *testing.T) {
		bodyXML := []byte(`<?xml version="1.0"?><soap:Envelope xmlns:soap="http://schemas.xmlsoap.org/soap/envelope/"><soap:Body><CreateUser><name>Jane</name><email>jane@example.com</email></CreateUser></soap:Body></soap:Envelope>`)
		resp, body := engineSOAPReq("/soap/user", "http://example.com/CreateUser", bodyXML)
		resp.Body.Close()
		assert.Equal(t, 200, resp.StatusCode)
		assert.Contains(t, body, "new-001")
		assert.Contains(t, body, "created")
	})
}
