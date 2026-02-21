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

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/getmockd/mockd/pkg/admin"
	"github.com/getmockd/mockd/pkg/admin/engineclient"
	"github.com/getmockd/mockd/pkg/config"
	"github.com/getmockd/mockd/pkg/engine"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMQTTProtocolIntegration(t *testing.T) {
	port := getFreePort(t)
	controlPort := getFreePort(t)
	adminPort := getFreePort(t)
	mqttPort := getFreePort(t)

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

	// Setup: Create an MQTT Mock
	mockReqBody := []byte(fmt.Sprintf(`{
		"type": "mqtt",
		"name": "e2e-mqtt-test",
		"mqtt": {
		  "port": %d,
		  "topics": [
			{
			  "topic": "sensors/temp",
			  "messages": [
				{"payload": "{\"temp\": 72, \"unit\": \"F\"}"}
			  ]
			},
			{
			  "topic": "sensors/+/data",
			  "messages": [
				{"payload": "{\"reading\": 42}"}
			  ]
			},
			{
			  "topic": "test/retained",
			  "messages": []
			}
		  ]
		}
	}`, mqttPort))

	resp := apiReq("POST", "/mocks", mockReqBody)
	require.Equal(t, 201, resp.StatusCode, "Failed to create MQTT mock")

	var createdMock struct {
		ID string `json:"id"`
	}
	json.NewDecoder(resp.Body).Decode(&createdMock)
	resp.Body.Close()
	mockID := createdMock.ID

	// Wait for the MQTT broker to spin up
	// The mockd internals use a TCP listener, so let's dial it until successful
	time.Sleep(500 * time.Millisecond)

	// Helper to connect an MQTT client
	connectMQTT := func(clientID string, port int, username, password string) (mqtt.Client, error) {
		opts := mqtt.NewClientOptions()
		opts.AddBroker(fmt.Sprintf("tcp://127.0.0.1:%d", port))
		opts.SetClientID(clientID)
		opts.SetCleanSession(true)
		if username != "" {
			opts.SetUsername(username)
			opts.SetPassword(password)
		}
		opts.SetConnectTimeout(2 * time.Second)

		c := mqtt.NewClient(opts)
		token := c.Connect()
		if token.Wait() && token.Error() != nil {
			return nil, token.Error()
		}
		return c, nil
	}

	t.Run("Create MQTT mock returns 201", func(t *testing.T) {
		resp := apiReq("GET", "/mocks/"+mockID, nil)
		resp.Body.Close()
		assert.Equal(t, 200, resp.StatusCode)
	})

	t.Run("Broker accepts publish connection", func(t *testing.T) {
		c, err := connectMQTT("pub-test", mqttPort, "", "")
		require.NoError(t, err)
		defer c.Disconnect(250)

		token := c.Publish("test/ping", 0, false, "hello")
		token.Wait()
		require.NoError(t, token.Error())
	})

	t.Run("Subscribe receives temp message", func(t *testing.T) {
		msgChan := make(chan string, 1)

		c, err := connectMQTT("sub-test-temp", mqttPort, "", "")
		require.NoError(t, err)
		defer c.Disconnect(250)

		token := c.Subscribe("sensors/temp", 0, func(client mqtt.Client, msg mqtt.Message) {
			msgChan <- string(msg.Payload())
		})
		token.Wait()
		require.NoError(t, token.Error())

		// Depending on whether it's auto-published upon subscription or if we manually publish
		// Let's trigger it manually just in case
		c.Publish("sensors/temp", 0, false, `{"temp": 72, "unit": "F"}`)

		select {
		case msg := <-msgChan:
			assert.Contains(t, msg, "temp")
		case <-time.After(2 * time.Second):
			t.Fatal("Timeout waiting for message on sensors/temp")
		}
	})

	t.Run("Wildcard subscription receives messages", func(t *testing.T) {
		msgChan := make(chan string, 1)

		c, err := connectMQTT("sub-test-wildcard", mqttPort, "", "")
		require.NoError(t, err)
		defer c.Disconnect(250)

		token := c.Subscribe("sensors/#", 0, func(client mqtt.Client, msg mqtt.Message) {
			msgChan <- string(msg.Payload())
		})
		token.Wait()
		require.NoError(t, token.Error())

		c.Publish("sensors/temp", 0, false, `{"temp": 99}`)

		select {
		case msg := <-msgChan:
			assert.Contains(t, msg, "temp")
		case <-time.After(2 * time.Second):
			t.Fatal("Timeout waiting for wildcard message")
		}
	})

	t.Run("Single-level wildcard receives messages", func(t *testing.T) {
		msgChan := make(chan string, 1)

		c, err := connectMQTT("sub-test-single-wildcard", mqttPort, "", "")
		require.NoError(t, err)
		defer c.Disconnect(250)

		token := c.Subscribe("sensors/+/data", 0, func(client mqtt.Client, msg mqtt.Message) {
			msgChan <- string(msg.Payload())
		})
		token.Wait()
		require.NoError(t, token.Error())

		c.Publish("sensors/livingroom/data", 0, false, `{"reading": 42}`)

		select {
		case msg := <-msgChan:
			assert.Contains(t, msg, "reading")
		case <-time.After(2 * time.Second):
			t.Fatal("Timeout waiting for single-level wildcard message")
		}
	})

	t.Run("Retained message delivered to late subscriber", func(t *testing.T) {
		pubClient, err := connectMQTT("pub-retained", mqttPort, "", "")
		require.NoError(t, err)

		token := pubClient.Publish("test/retained", 0, true, `{"retained": true}`)
		token.Wait()
		require.NoError(t, token.Error())
		pubClient.Disconnect(250)

		// Wait a bit
		time.Sleep(100 * time.Millisecond)

		msgChan := make(chan string, 1)
		subClient, err := connectMQTT("sub-retained", mqttPort, "", "")
		require.NoError(t, err)

		token = subClient.Subscribe("test/retained", 0, func(client mqtt.Client, msg mqtt.Message) {
			msgChan <- string(msg.Payload())
		})
		token.Wait()
		require.NoError(t, token.Error())
		defer subClient.Disconnect(250)

		select {
		case msg := <-msgChan:
			assert.Contains(t, msg, "retained")
		case <-time.After(2 * time.Second):
			t.Fatal("Timeout waiting for retained message")
		}
	})

	t.Run("Auth broker rejects wrong credentials", func(t *testing.T) {
		authPort := getFreePort(t)
		authMockReqBody := []byte(fmt.Sprintf(`{
			"type": "mqtt",
			"name": "e2e-mqtt-auth",
			"mqtt": {
			  "port": %d,
			  "auth": {
				"enabled": true,
				"users": [
				  {"username": "sensor", "password": "s3cret"}
				]
			  },
			  "topics": [
				{"topic": "auth/data", "messages": [{"payload": "ok"}]}
			  ]
			}
		}`, authPort))

		resp := apiReq("POST", "/mocks", authMockReqBody)
		require.Equal(t, 201, resp.StatusCode)

		var authMock struct {
			ID string `json:"id"`
		}
		json.NewDecoder(resp.Body).Decode(&authMock)
		resp.Body.Close()

		time.Sleep(500 * time.Millisecond) // Allow broker to settle

		_, err := connectMQTT("auth-fail", authPort, "sensor", "wrongpass")
		require.Error(t, err)

		c, err := connectMQTT("auth-success", authPort, "sensor", "s3cret")
		require.NoError(t, err)
		c.Disconnect(250)

		respD := apiReq("DELETE", "/mocks/"+authMock.ID, nil)
		respD.Body.Close()
	})

	t.Run("Toggle mock disabled stops broker", func(t *testing.T) {
		resp1 := apiReq("POST", "/mocks/"+mockID+"/toggle", []byte(`{"enabled": false}`))
		resp1.Body.Close()
		time.Sleep(500 * time.Millisecond) // Allow broker to shut down

		_, err := connectMQTT("test-disabled", mqttPort, "", "")
		require.Error(t, err, "Broker should be unreachable when disabled")

		resp2 := apiReq("POST", "/mocks/"+mockID+"/toggle", []byte(`{"enabled": true}`))
		resp2.Body.Close()
		time.Sleep(500 * time.Millisecond) // Allow broker to restart
	})

	t.Run("Delete MQTT mock shuts down broker", func(t *testing.T) {
		respD2 := apiReq("DELETE", "/mocks/"+mockID, nil)
		respD2.Body.Close()
		time.Sleep(500 * time.Millisecond) // Allow broker to shut down

		_, err := connectMQTT("test-deleted", mqttPort, "", "")
		require.Error(t, err, "Broker should be unreachable after deletion")
	})
}
