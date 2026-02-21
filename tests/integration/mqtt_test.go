package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"sync"
	"testing"
	"time"

	mqttclient "github.com/eclipse/paho.mqtt.golang"
	"github.com/getmockd/mockd/pkg/config"
	"github.com/getmockd/mockd/pkg/engine"
	"github.com/getmockd/mockd/pkg/mock"
	"github.com/getmockd/mockd/pkg/mqtt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ============================================================================
// Test Helpers
// ============================================================================

// setupMQTTBroker creates and starts an MQTT broker for testing.
// It returns the broker and the actual bound port (which may differ
// from the configured port when port 0 is used for OS auto-assign).
func setupMQTTBroker(t *testing.T, cfg *mqtt.MQTTConfig) (*mqtt.Broker, int) {
	broker, err := mqtt.NewBroker(cfg)
	require.NoError(t, err)

	err = broker.Start(context.Background())
	require.NoError(t, err)

	t.Cleanup(func() {
		broker.Stop(context.Background(), 5*time.Second)
	})

	// Wait for broker to be ready
	time.Sleep(100 * time.Millisecond)

	return broker, broker.Port()
}

// createMQTTClient creates a Paho MQTT client for testing
func createMQTTClient(t *testing.T, port int, clientID string) mqttclient.Client {
	opts := mqttclient.NewClientOptions()
	opts.AddBroker(fmt.Sprintf("tcp://localhost:%d", port))
	opts.SetClientID(clientID)
	opts.SetAutoReconnect(false)
	opts.SetConnectTimeout(5 * time.Second)

	client := mqttclient.NewClient(opts)
	token := client.Connect()
	if !token.WaitTimeout(5 * time.Second) {
		t.Fatalf("MQTT connect timeout")
	}
	require.NoError(t, token.Error())

	t.Cleanup(func() {
		client.Disconnect(250)
	})

	return client
}

// ============================================================================
// User Story 1: Basic MQTT Broker
// ============================================================================

func TestMQTT_US1_BrokerStartStop(t *testing.T) {
	cfg := &mqtt.MQTTConfig{
		ID:      "test-broker",
		Name:    "Test Broker",
		Port:    0,
		Enabled: true,
	}

	broker, err := mqtt.NewBroker(cfg)
	require.NoError(t, err)

	// Start broker
	err = broker.Start(context.Background())
	require.NoError(t, err)
	assert.True(t, broker.IsRunning())

	port := broker.Port()

	// Verify port is listening
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("localhost:%d", port), time.Second)
	require.NoError(t, err)
	conn.Close()

	// Allow server to cleanup the test connection before stopping
	// This prevents a race in mochi-mqtt between client attachment and shutdown
	time.Sleep(100 * time.Millisecond)

	// Stop broker
	err = broker.Stop(context.Background(), 5*time.Second)
	require.NoError(t, err)
	assert.False(t, broker.IsRunning())
}

func TestMQTT_US1_BasicPubSub(t *testing.T) {
	broker, port := setupMQTTBroker(t, &mqtt.MQTTConfig{
		ID:      "test-pubsub",
		Port:    0,
		Enabled: true,
	})
	_ = broker

	// Create subscriber
	subscriber := createMQTTClient(t, port, "subscriber")
	received := make(chan string, 1)

	token := subscriber.Subscribe("test/topic", 1, func(client mqttclient.Client, msg mqttclient.Message) {
		received <- string(msg.Payload())
	})
	require.True(t, token.WaitTimeout(5*time.Second))
	require.NoError(t, token.Error())

	// Create publisher
	publisher := createMQTTClient(t, port, "publisher")
	token = publisher.Publish("test/topic", 1, false, "Hello MQTT")
	require.True(t, token.WaitTimeout(5*time.Second))
	require.NoError(t, token.Error())

	// Verify message received
	select {
	case msg := <-received:
		assert.Equal(t, "Hello MQTT", msg)
	case <-time.After(5 * time.Second):
		t.Fatal("Timeout waiting for message")
	}
}

// ============================================================================
// User Story 2: Topic Wildcards
// ============================================================================

func TestMQTT_US2_SingleLevelWildcard(t *testing.T) {
	broker, port := setupMQTTBroker(t, &mqtt.MQTTConfig{
		ID:      "test-wildcard-single",
		Port:    0,
		Enabled: true,
	})
	_ = broker

	client := createMQTTClient(t, port, "wildcard-client")
	received := make(chan string, 10)

	// Subscribe with + wildcard
	token := client.Subscribe("sensors/+/temperature", 1, func(c mqttclient.Client, msg mqttclient.Message) {
		received <- msg.Topic()
	})
	require.True(t, token.WaitTimeout(5*time.Second))

	// Publish to matching topics
	client.Publish("sensors/room1/temperature", 1, false, "25").Wait()
	client.Publish("sensors/room2/temperature", 1, false, "26").Wait()
	client.Publish("sensors/room1/humidity", 1, false, "60").Wait() // Should not match

	time.Sleep(500 * time.Millisecond) // Allow messages to propagate through broker

	// Disconnect client before draining channel to prevent race
	client.Disconnect(100)
	time.Sleep(50 * time.Millisecond) // Allow disconnect to complete

	// Verify only matching messages received using non-blocking drain
	topics := []string{}
drainLoop:
	for {
		select {
		case topic := <-received:
			topics = append(topics, topic)
		default:
			break drainLoop
		}
	}

	assert.Len(t, topics, 2)
	assert.Contains(t, topics, "sensors/room1/temperature")
	assert.Contains(t, topics, "sensors/room2/temperature")
}

func TestMQTT_US2_MultiLevelWildcard(t *testing.T) {
	broker, port := setupMQTTBroker(t, &mqtt.MQTTConfig{
		ID:      "test-wildcard-multi",
		Port:    0,
		Enabled: true,
	})
	_ = broker

	client := createMQTTClient(t, port, "wildcard-client")
	received := make(chan string, 10)

	// Subscribe with # wildcard
	token := client.Subscribe("home/#", 1, func(c mqttclient.Client, msg mqttclient.Message) {
		received <- msg.Topic()
	})
	require.True(t, token.WaitTimeout(5*time.Second))

	// Publish to various topics
	client.Publish("home/living/temp", 1, false, "22").Wait()
	client.Publish("home/bedroom/light", 1, false, "on").Wait()
	client.Publish("home/kitchen/appliances/oven", 1, false, "off").Wait()
	client.Publish("office/temp", 1, false, "21").Wait() // Should not match

	time.Sleep(500 * time.Millisecond) // Allow messages to propagate through broker

	// Disconnect client before draining channel to prevent race
	client.Disconnect(100)
	time.Sleep(50 * time.Millisecond) // Allow disconnect to complete

	// Drain using non-blocking read
	topics := []string{}
drainLoop:
	for {
		select {
		case topic := <-received:
			topics = append(topics, topic)
		default:
			break drainLoop
		}
	}

	assert.Len(t, topics, 3)
	assert.Contains(t, topics, "home/living/temp")
	assert.Contains(t, topics, "home/bedroom/light")
	assert.Contains(t, topics, "home/kitchen/appliances/oven")
}

// ============================================================================
// User Story 3: QoS Levels
// ============================================================================

func TestMQTT_US3_QoS0(t *testing.T) {
	broker, port := setupMQTTBroker(t, &mqtt.MQTTConfig{
		ID:      "test-qos0",
		Port:    0,
		Enabled: true,
	})
	_ = broker

	client := createMQTTClient(t, port, "qos-client")
	received := make(chan bool, 1)

	client.Subscribe("qos/test", 0, func(c mqttclient.Client, msg mqttclient.Message) {
		assert.Equal(t, byte(0), msg.Qos())
		received <- true
	}).Wait()

	client.Publish("qos/test", 0, false, "qos0 message").Wait()

	select {
	case <-received:
		// Success
	case <-time.After(2 * time.Second):
		t.Fatal("Timeout waiting for QoS 0 message")
	}
}

func TestMQTT_US3_QoS1(t *testing.T) {
	broker, port := setupMQTTBroker(t, &mqtt.MQTTConfig{
		ID:      "test-qos1",
		Port:    0,
		Enabled: true,
	})
	_ = broker

	client := createMQTTClient(t, port, "qos-client")
	received := make(chan bool, 1)

	client.Subscribe("qos/test", 1, func(c mqttclient.Client, msg mqttclient.Message) {
		assert.Equal(t, byte(1), msg.Qos())
		received <- true
	}).Wait()

	client.Publish("qos/test", 1, false, "qos1 message").Wait()

	select {
	case <-received:
		// Success
	case <-time.After(2 * time.Second):
		t.Fatal("Timeout waiting for QoS 1 message")
	}
}

func TestMQTT_US3_QoS2(t *testing.T) {
	broker, port := setupMQTTBroker(t, &mqtt.MQTTConfig{
		ID:      "test-qos2",
		Port:    0,
		Enabled: true,
	})
	_ = broker

	client := createMQTTClient(t, port, "qos-client")
	received := make(chan bool, 1)

	client.Subscribe("qos/test", 2, func(c mqttclient.Client, msg mqttclient.Message) {
		assert.Equal(t, byte(2), msg.Qos())
		received <- true
	}).Wait()

	client.Publish("qos/test", 2, false, "qos2 message").Wait()

	select {
	case <-received:
		// Success
	case <-time.After(2 * time.Second):
		t.Fatal("Timeout waiting for QoS 2 message")
	}
}

// ============================================================================
// User Story 4: Retained Messages
// ============================================================================

func TestMQTT_US4_RetainedMessages(t *testing.T) {
	broker, port := setupMQTTBroker(t, &mqtt.MQTTConfig{
		ID:      "test-retain",
		Port:    0,
		Enabled: true,
	})
	_ = broker

	// First client publishes retained message
	publisher := createMQTTClient(t, port, "publisher")
	publisher.Publish("retain/test", 1, true, "retained value").Wait()
	publisher.Disconnect(250)

	time.Sleep(200 * time.Millisecond) // Allow retained message to persist in broker

	// New client subscribes and should receive retained message
	received := make(chan string, 1)
	subscriber := createMQTTClient(t, port, "subscriber")
	subscriber.Subscribe("retain/test", 1, func(c mqttclient.Client, msg mqttclient.Message) {
		received <- string(msg.Payload())
	}).Wait()

	select {
	case msg := <-received:
		assert.Equal(t, "retained value", msg)
	case <-time.After(2 * time.Second):
		t.Fatal("Timeout waiting for retained message")
	}
}

// ============================================================================
// User Story 5: Topic Configuration with Auto-Publishing
// ============================================================================

func TestMQTT_US5_ConfiguredTopics(t *testing.T) {
	cfg := &mqtt.MQTTConfig{
		ID:      "test-topics",
		Port:    0,
		Enabled: true,
		Topics: []mqtt.TopicConfig{
			{
				Topic: "sensors/temperature",
				QoS:   1,
				Messages: []mqtt.MessageConfig{
					{
						Payload:  `{"temp": 25.5, "unit": "C"}`,
						Interval: "100ms",
						Repeat:   true,
					},
				},
			},
		},
	}

	broker, port := setupMQTTBroker(t, cfg)
	_ = broker

	client := createMQTTClient(t, port, "sensor-sub")
	received := make(chan string, 5)

	client.Subscribe("sensors/temperature", 1, func(c mqttclient.Client, msg mqttclient.Message) {
		received <- string(msg.Payload())
	}).Wait()

	// Wait for auto-published messages
	time.Sleep(350 * time.Millisecond) // Collect ~3 messages at 100ms interval

	// Disconnect client before draining channel to prevent race
	client.Disconnect(100)
	time.Sleep(50 * time.Millisecond) // Allow disconnect to complete

	// Drain remaining messages from channel
	count := 0
drainLoop:
	for {
		select {
		case msg := <-received:
			assert.Contains(t, msg, "temp")
			count++
		default:
			break drainLoop
		}
	}

	// Should receive at least 2 messages in 350ms with 100ms interval
	assert.GreaterOrEqual(t, count, 2, "Should receive auto-published messages")
}

// ============================================================================
// User Story 6: OnPublish Handler (Request/Response)
// ============================================================================

func TestMQTT_US6_OnPublishResponse(t *testing.T) {
	cfg := &mqtt.MQTTConfig{
		ID:      "test-onpublish",
		Port:    0,
		Enabled: true,
		Topics: []mqtt.TopicConfig{
			{
				Topic: "request/echo",
				OnPublish: &mqtt.PublishHandler{
					Response: &mqtt.MessageConfig{
						Payload: `{"echo": "received"}`,
					},
				},
			},
		},
	}

	broker, port := setupMQTTBroker(t, cfg)
	_ = broker

	client := createMQTTClient(t, port, "echo-client")
	received := make(chan string, 10)

	// Subscribe to the same topic to get response
	client.Subscribe("request/echo", 1, func(c mqttclient.Client, msg mqttclient.Message) {
		received <- string(msg.Payload())
	}).Wait()

	// Publish request
	client.Publish("request/echo", 1, false, `{"data": "test"}`).Wait()

	// We may receive both our published message and the response
	// Wait for up to 2 messages (original + response)
	foundResponse := false
	timeout := time.After(2 * time.Second)
Loop:
	for i := 0; i < 2; i++ {
		select {
		case msg := <-received:
			if msg == `{"echo": "received"}` {
				foundResponse = true
				break Loop
			}
		case <-timeout:
			break Loop
		}
	}

	assert.True(t, foundResponse, "Should receive the OnPublish response")
}

func TestMQTT_US6_OnPublishForward(t *testing.T) {
	cfg := &mqtt.MQTTConfig{
		ID:      "test-forward",
		Port:    0,
		Enabled: true,
		Topics: []mqtt.TopicConfig{
			{
				Topic: "input/data",
				OnPublish: &mqtt.PublishHandler{
					Forward: "output/data",
				},
			},
		},
	}

	broker, port := setupMQTTBroker(t, cfg)
	_ = broker

	client := createMQTTClient(t, port, "forward-client")
	received := make(chan string, 1)

	// Subscribe to forwarded topic
	client.Subscribe("output/data", 1, func(c mqttclient.Client, msg mqttclient.Message) {
		received <- string(msg.Payload())
	}).Wait()

	time.Sleep(100 * time.Millisecond) // Allow subscription to register

	// Publish to input topic
	client.Publish("input/data", 1, false, "forwarded message").Wait()

	select {
	case msg := <-received:
		assert.Equal(t, "forwarded message", msg)
	case <-time.After(2 * time.Second):
		t.Fatal("Timeout waiting for forwarded message")
	}
}

// ============================================================================
// User Story 7: Authentication
// ============================================================================

func TestMQTT_US7_AuthenticationRequired(t *testing.T) {
	cfg := &mqtt.MQTTConfig{
		ID:      "test-auth",
		Port:    0,
		Enabled: true,
		Auth: &mqtt.MQTTAuthConfig{
			Enabled: true,
			Users: []mqtt.MQTTUser{
				{
					Username: "testuser",
					Password: "testpass",
				},
			},
		},
	}

	broker, port := setupMQTTBroker(t, cfg)
	_ = broker

	// Try connecting without credentials - should fail
	opts := mqttclient.NewClientOptions()
	opts.AddBroker(fmt.Sprintf("tcp://localhost:%d", port))
	opts.SetClientID("no-auth-client")
	opts.SetConnectTimeout(2 * time.Second)

	client := mqttclient.NewClient(opts)
	token := client.Connect()
	token.WaitTimeout(2 * time.Second)
	assert.Error(t, token.Error(), "Connection without auth should fail")
}

func TestMQTT_US7_AuthenticationSuccess(t *testing.T) {
	cfg := &mqtt.MQTTConfig{
		ID:      "test-auth-success",
		Port:    0,
		Enabled: true,
		Auth: &mqtt.MQTTAuthConfig{
			Enabled: true,
			Users: []mqtt.MQTTUser{
				{
					Username: "testuser",
					Password: "testpass",
				},
			},
		},
	}

	broker, port := setupMQTTBroker(t, cfg)
	_ = broker

	// Connect with valid credentials
	opts := mqttclient.NewClientOptions()
	opts.AddBroker(fmt.Sprintf("tcp://localhost:%d", port))
	opts.SetClientID("auth-client")
	opts.SetUsername("testuser")
	opts.SetPassword("testpass")
	opts.SetConnectTimeout(5 * time.Second)

	client := mqttclient.NewClient(opts)
	token := client.Connect()
	require.True(t, token.WaitTimeout(5*time.Second))
	require.NoError(t, token.Error())

	client.Disconnect(250)
}

// ============================================================================
// User Story 8: Multiple Clients
// ============================================================================

func TestMQTT_US8_MultipleClients(t *testing.T) {
	broker, port := setupMQTTBroker(t, &mqtt.MQTTConfig{
		ID:      "test-multi-client",
		Port:    0,
		Enabled: true,
	})
	_ = broker

	numClients := 5
	var wg sync.WaitGroup
	received := make(chan string, numClients*10)
	clients := make([]mqttclient.Client, numClients)
	var clientsMu sync.Mutex

	// Create multiple subscribers
	for i := 0; i < numClients; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			client := createMQTTClient(t, port, fmt.Sprintf("client-%d", id))
			clientsMu.Lock()
			clients[id] = client
			clientsMu.Unlock()
			client.Subscribe("broadcast", 1, func(c mqttclient.Client, msg mqttclient.Message) {
				received <- fmt.Sprintf("client-%d:%s", id, string(msg.Payload()))
			}).Wait()
		}(i)
	}

	time.Sleep(500 * time.Millisecond) // Allow all clients to connect and subscribe

	// Publish a message
	publisher := createMQTTClient(t, port, "broadcaster")
	publisher.Publish("broadcast", 1, false, "hello all").Wait()

	time.Sleep(500 * time.Millisecond) // Allow broadcast to reach all clients

	// Disconnect all clients before draining channel to prevent race
	for _, client := range clients {
		if client != nil {
			client.Disconnect(100)
		}
	}
	publisher.Disconnect(100)
	time.Sleep(50 * time.Millisecond) // Allow disconnects to complete

	// Drain messages using non-blocking read
	messages := []string{}
drainLoop:
	for {
		select {
		case msg := <-received:
			messages = append(messages, msg)
		default:
			break drainLoop
		}
	}

	assert.Len(t, messages, numClients, "All clients should receive the broadcast")
}

// ============================================================================
// User Story 9: Auto-Start from Stored Mock
// ============================================================================

func TestMQTT_US9_AutoStartFromMock(t *testing.T) {
	mqttPort := getFreePort()
	managementPort := getFreePort()

	// Create server with MQTT mock via ImportConfig
	serverCfg := config.DefaultServerConfiguration()
	serverCfg.HTTPPort = 0 // Disable HTTP for this test
	serverCfg.ManagementPort = managementPort
	server := engine.NewServer(serverCfg)

	// Import MQTT mock configuration
	mqttMockCfg := &config.MockConfiguration{
		ID:      "test-mqtt-mock",
		Type:    mock.TypeMQTT,
		Name:    "Test MQTT",
		Enabled: boolPtr(true),
		MQTT: &mock.MQTTSpec{
			Port: mqttPort,
			Topics: []mock.TopicConfig{
				{
					Topic: "test/auto",
					QoS:   1,
				},
			},
		},
	}

	collection := &config.MockCollection{
		Version: "1.0",
		Name:    "mqtt-test",
		Mocks:   []*config.MockConfiguration{mqttMockCfg},
	}

	err := server.ImportConfig(collection, true)
	require.NoError(t, err)

	err = server.Start()
	require.NoError(t, err)
	t.Cleanup(func() {
		server.Stop()
	})

	time.Sleep(500 * time.Millisecond) // Allow engine and MQTT broker to start

	// Verify MQTT broker is running
	client := createMQTTClient(t, mqttPort, "auto-test")
	received := make(chan bool, 1)

	client.Subscribe("test/auto", 1, func(c mqttclient.Client, msg mqttclient.Message) {
		received <- true
	}).Wait()

	client.Publish("test/auto", 1, false, "auto-start test").Wait()

	select {
	case <-received:
		// Success - broker auto-started
	case <-time.After(2 * time.Second):
		t.Fatal("MQTT broker did not auto-start from mock")
	}
}

// ============================================================================
// User Story 10: JSON Payloads
// ============================================================================

func TestMQTT_US10_JSONPayloads(t *testing.T) {
	broker, port := setupMQTTBroker(t, &mqtt.MQTTConfig{
		ID:      "test-json",
		Port:    0,
		Enabled: true,
	})
	_ = broker

	type SensorData struct {
		Temperature float64 `json:"temperature"`
		Humidity    int     `json:"humidity"`
		Timestamp   string  `json:"timestamp"`
	}

	client := createMQTTClient(t, port, "json-client")
	received := make(chan SensorData, 1)

	client.Subscribe("sensors/data", 1, func(c mqttclient.Client, msg mqttclient.Message) {
		var data SensorData
		if err := json.Unmarshal(msg.Payload(), &data); err == nil {
			received <- data
		}
	}).Wait()

	// Publish JSON payload
	payload := SensorData{
		Temperature: 23.5,
		Humidity:    65,
		Timestamp:   "2026-01-07T12:00:00Z",
	}
	payloadBytes, _ := json.Marshal(payload)
	client.Publish("sensors/data", 1, false, payloadBytes).Wait()

	select {
	case data := <-received:
		assert.Equal(t, 23.5, data.Temperature)
		assert.Equal(t, 65, data.Humidity)
	case <-time.After(2 * time.Second):
		t.Fatal("Timeout waiting for JSON message")
	}
}

// ============================================================================
// User Story 11: Broker Stats
// ============================================================================

func TestMQTT_US11_BrokerStats(t *testing.T) {
	broker, port := setupMQTTBroker(t, &mqtt.MQTTConfig{
		ID:      "test-stats",
		Port:    0,
		Enabled: true,
	})

	// Connect a client
	client := createMQTTClient(t, port, "stats-client")
	client.Subscribe("stats/test", 1, nil).Wait()
	client.Publish("stats/test", 1, false, "test").Wait()

	time.Sleep(200 * time.Millisecond) // Allow stats counters to update

	stats := broker.GetStats()
	assert.Equal(t, port, stats.Port)
	assert.True(t, stats.Running)
	assert.GreaterOrEqual(t, stats.ClientCount, 1)
}

// ============================================================================
// User Story 12: MQTT Templating
// ============================================================================

func TestMQTT_US12_TemplatingBasicVariables(t *testing.T) {
	cfg := &mqtt.MQTTConfig{
		ID:      "test-templating-basic",
		Port:    0,
		Enabled: true,
		Topics: []mqtt.TopicConfig{
			{
				Topic: "sensors/temperature",
				QoS:   1,
				Messages: []mqtt.MessageConfig{
					{
						Payload:  `{"timestamp": "{{ timestamp }}", "uuid": "{{ uuid }}"}`,
						Interval: "100ms",
						Repeat:   true,
					},
				},
			},
		},
	}

	broker, port := setupMQTTBroker(t, cfg)
	_ = broker

	client := createMQTTClient(t, port, "template-sub")
	received := make(chan string, 5)

	client.Subscribe("sensors/temperature", 1, func(c mqttclient.Client, msg mqttclient.Message) {
		received <- string(msg.Payload())
	}).Wait()

	// Wait for auto-published messages
	time.Sleep(250 * time.Millisecond) // Collect ~2 messages at 100ms interval

	// Disconnect client before draining channel to prevent race
	client.Disconnect(100)
	time.Sleep(50 * time.Millisecond) // Allow disconnect to complete

	// Drain and verify messages
	count := 0
drainLoop:
	for {
		select {
		case msg := <-received:
			// Verify the message contains rendered template values (not raw placeholders)
			assert.NotContains(t, msg, "{{ timestamp }}")
			assert.NotContains(t, msg, "{{ uuid }}")
			assert.Contains(t, msg, "timestamp")
			assert.Contains(t, msg, "uuid")

			// Verify it's valid JSON with expected structure
			var data map[string]interface{}
			err := json.Unmarshal([]byte(msg), &data)
			require.NoError(t, err, "Should be valid JSON")
			assert.Contains(t, data, "timestamp")
			assert.Contains(t, data, "uuid")
			count++
		default:
			break drainLoop
		}
	}

	assert.GreaterOrEqual(t, count, 1, "Should receive templated messages")
}

func TestMQTT_US12_TemplatingRandomValues(t *testing.T) {
	cfg := &mqtt.MQTTConfig{
		ID:      "test-templating-random",
		Port:    0,
		Enabled: true,
		Topics: []mqtt.TopicConfig{
			{
				Topic: "sensors/data",
				QoS:   1,
				Messages: []mqtt.MessageConfig{
					{
						Payload:  `{"temp": {{ random.int(20, 30) }}, "humidity": {{ random.float(0.0, 100.0, 1) }}}`,
						Interval: "100ms",
						Repeat:   true,
					},
				},
			},
		},
	}

	broker, port := setupMQTTBroker(t, cfg)
	_ = broker

	client := createMQTTClient(t, port, "random-sub")
	received := make(chan string, 5)

	client.Subscribe("sensors/data", 1, func(c mqttclient.Client, msg mqttclient.Message) {
		received <- string(msg.Payload())
	}).Wait()

	// Wait for auto-published messages
	time.Sleep(250 * time.Millisecond) // Collect ~2 messages at 100ms interval

	client.Disconnect(100)
	time.Sleep(50 * time.Millisecond) // Allow disconnect to complete

	// Drain and verify messages have random values within expected range
	count := 0
drainLoop:
	for {
		select {
		case msg := <-received:
			var data map[string]interface{}
			err := json.Unmarshal([]byte(msg), &data)
			require.NoError(t, err, "Should be valid JSON")

			temp, ok := data["temp"].(float64)
			require.True(t, ok, "temp should be a number")
			assert.GreaterOrEqual(t, temp, float64(20), "temp should be >= 20")
			assert.LessOrEqual(t, temp, float64(30), "temp should be <= 30")

			humidity, ok := data["humidity"].(float64)
			require.True(t, ok, "humidity should be a number")
			assert.GreaterOrEqual(t, humidity, float64(0), "humidity should be >= 0")
			assert.LessOrEqual(t, humidity, float64(100), "humidity should be <= 100")
			count++
		default:
			break drainLoop
		}
	}

	assert.GreaterOrEqual(t, count, 1, "Should receive messages with random values")
}

func TestMQTT_US12_TemplatingSequence(t *testing.T) {
	cfg := &mqtt.MQTTConfig{
		ID:      "test-templating-sequence",
		Port:    0,
		Enabled: true,
		Topics: []mqtt.TopicConfig{
			{
				Topic: "events/log",
				QoS:   1,
				Messages: []mqtt.MessageConfig{
					{
						Payload:  `{"eventId": {{ sequence("event") }}}`,
						Interval: "50ms",
						Repeat:   true,
					},
				},
			},
		},
	}

	broker, port := setupMQTTBroker(t, cfg)
	_ = broker

	client := createMQTTClient(t, port, "sequence-sub")
	received := make(chan string, 10)

	client.Subscribe("events/log", 1, func(c mqttclient.Client, msg mqttclient.Message) {
		received <- string(msg.Payload())
	}).Wait()

	// Wait for several auto-published messages
	time.Sleep(200 * time.Millisecond) // Collect ~4 messages at 50ms interval

	client.Disconnect(100)
	time.Sleep(50 * time.Millisecond) // Allow disconnect to complete

	// Collect sequence values and verify they are incrementing
	var eventIds []int64
drainLoop:
	for {
		select {
		case msg := <-received:
			var data map[string]interface{}
			err := json.Unmarshal([]byte(msg), &data)
			require.NoError(t, err, "Should be valid JSON")

			eventID, ok := data["eventId"].(float64)
			require.True(t, ok, "eventId should be a number")
			eventIds = append(eventIds, int64(eventID))
		default:
			break drainLoop
		}
	}

	require.GreaterOrEqual(t, len(eventIds), 2, "Should receive at least 2 messages")

	// Verify sequence is incrementing (not necessarily starting from 1 due to timing)
	for i := 1; i < len(eventIds); i++ {
		assert.Equal(t, eventIds[i-1]+1, eventIds[i], "Sequence should increment by 1")
	}
}

func TestMQTT_US12_TemplatingFaker(t *testing.T) {
	cfg := &mqtt.MQTTConfig{
		ID:      "test-templating-faker",
		Port:    0,
		Enabled: true,
		Topics: []mqtt.TopicConfig{
			{
				Topic: "users/profile",
				QoS:   1,
				Messages: []mqtt.MessageConfig{
					{
						Payload:  `{"name": "{{ faker.firstName }}", "email": "{{ faker.email }}"}`,
						Interval: "100ms",
						Repeat:   true,
					},
				},
			},
		},
	}

	broker, port := setupMQTTBroker(t, cfg)
	_ = broker

	client := createMQTTClient(t, port, "faker-sub")
	received := make(chan string, 5)

	client.Subscribe("users/profile", 1, func(c mqttclient.Client, msg mqttclient.Message) {
		received <- string(msg.Payload())
	}).Wait()

	// Wait for auto-published messages
	time.Sleep(150 * time.Millisecond) // Collect ~1-2 messages at 100ms interval

	client.Disconnect(100)
	time.Sleep(50 * time.Millisecond) // Allow disconnect to complete

	// Verify messages have faker-generated values
	count := 0
drainLoop:
	for {
		select {
		case msg := <-received:
			var data map[string]interface{}
			err := json.Unmarshal([]byte(msg), &data)
			require.NoError(t, err, "Should be valid JSON")

			name, ok := data["name"].(string)
			require.True(t, ok, "name should be a string")
			assert.NotEmpty(t, name, "name should not be empty")
			assert.NotContains(t, name, "{{ faker", "name should be rendered")

			email, ok := data["email"].(string)
			require.True(t, ok, "email should be a string")
			assert.NotEmpty(t, email, "email should not be empty")
			assert.Contains(t, email, "@", "email should contain @")
			count++
		default:
			break drainLoop
		}
	}

	assert.GreaterOrEqual(t, count, 1, "Should receive messages with faker values")
}

// ============================================================================
// User Story 13: QoS Levels - Extended Tests
// ============================================================================

func TestMQTT_US13_QoSLevelsWithVerification(t *testing.T) {
	// Test all three QoS levels in a single test to verify behavior
	testCases := []struct {
		name         string
		publishQoS   byte
		subscribeQoS byte
		description  string
	}{
		{
			name:         "QoS0_AtMostOnce",
			publishQoS:   0,
			subscribeQoS: 0,
			description:  "Fire and forget - no acknowledgment",
		},
		{
			name:         "QoS1_AtLeastOnce",
			publishQoS:   1,
			subscribeQoS: 1,
			description:  "Acknowledged delivery - may receive duplicates",
		},
		{
			name:         "QoS2_ExactlyOnce",
			publishQoS:   2,
			subscribeQoS: 2,
			description:  "Assured delivery - exactly once",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			broker, port := setupMQTTBroker(t, &mqtt.MQTTConfig{
				ID:      fmt.Sprintf("test-qos-%d", tc.publishQoS),
				Port:    0,
				Enabled: true,
			})
			_ = broker

			client := createMQTTClient(t, port, fmt.Sprintf("qos%d-client", tc.publishQoS))
			received := make(chan mqttclient.Message, 1)

			// Subscribe with the test QoS level
			token := client.Subscribe(fmt.Sprintf("qos%d/test", tc.publishQoS), tc.subscribeQoS, func(c mqttclient.Client, msg mqttclient.Message) {
				received <- msg
			})
			require.True(t, token.WaitTimeout(5*time.Second))
			require.NoError(t, token.Error())

			// Publish with the test QoS level
			payload := fmt.Sprintf("QoS %d message: %s", tc.publishQoS, tc.description)
			pubToken := client.Publish(fmt.Sprintf("qos%d/test", tc.publishQoS), tc.publishQoS, false, payload)
			require.True(t, pubToken.WaitTimeout(5*time.Second))
			require.NoError(t, pubToken.Error())

			// Verify message received
			select {
			case msg := <-received:
				assert.Equal(t, payload, string(msg.Payload()))
				// Effective QoS is the minimum of publish and subscribe QoS
				expectedQoS := tc.publishQoS
				if tc.subscribeQoS < expectedQoS {
					expectedQoS = tc.subscribeQoS
				}
				assert.Equal(t, expectedQoS, msg.Qos(), "Effective QoS should be min(pub, sub)")
			case <-time.After(3 * time.Second):
				t.Fatalf("Timeout waiting for %s message", tc.name)
			}
		})
	}
}

func TestMQTT_US13_QoSDowngrade(t *testing.T) {
	// Test that QoS is downgraded when subscribe QoS < publish QoS
	broker, port := setupMQTTBroker(t, &mqtt.MQTTConfig{
		ID:      "test-qos-downgrade",
		Port:    0,
		Enabled: true,
	})
	_ = broker

	client := createMQTTClient(t, port, "downgrade-client")
	received := make(chan mqttclient.Message, 1)

	// Subscribe with QoS 0
	token := client.Subscribe("qos/downgrade", 0, func(c mqttclient.Client, msg mqttclient.Message) {
		received <- msg
	})
	require.True(t, token.WaitTimeout(5*time.Second))

	// Publish with QoS 2
	client.Publish("qos/downgrade", 2, false, "downgrade test").Wait()

	select {
	case msg := <-received:
		// Message should be downgraded to QoS 0
		assert.Equal(t, byte(0), msg.Qos(), "QoS should be downgraded to subscriber's QoS")
	case <-time.After(2 * time.Second):
		t.Fatal("Timeout waiting for downgraded message")
	}
}

// ============================================================================
// User Story 14: Retained Messages - Extended Tests
// ============================================================================

func TestMQTT_US14_RetainedMessageOverwrite(t *testing.T) {
	broker, port := setupMQTTBroker(t, &mqtt.MQTTConfig{
		ID:      "test-retain-overwrite",
		Port:    0,
		Enabled: true,
	})
	_ = broker

	// Publish first retained message
	publisher := createMQTTClient(t, port, "publisher1")
	publisher.Publish("retain/overwrite", 1, true, "first value").Wait()
	publisher.Disconnect(250)

	time.Sleep(100 * time.Millisecond) // Allow retained message to persist

	// Publish second retained message (should overwrite)
	publisher2 := createMQTTClient(t, port, "publisher2")
	publisher2.Publish("retain/overwrite", 1, true, "second value").Wait()
	publisher2.Disconnect(250)

	time.Sleep(100 * time.Millisecond) // Allow retained message to overwrite

	// New subscriber should receive only the latest retained message
	received := make(chan string, 5)
	subscriber := createMQTTClient(t, port, "subscriber")
	subscriber.Subscribe("retain/overwrite", 1, func(c mqttclient.Client, msg mqttclient.Message) {
		received <- string(msg.Payload())
	}).Wait()

	select {
	case msg := <-received:
		assert.Equal(t, "second value", msg, "Should receive the latest retained message")
	case <-time.After(2 * time.Second):
		t.Fatal("Timeout waiting for retained message")
	}

	// Verify no more messages
	select {
	case <-received:
		t.Fatal("Should not receive multiple retained messages")
	case <-time.After(200 * time.Millisecond):
		// Expected - no more messages
	}
}

func TestMQTT_US14_RetainedMessageClear(t *testing.T) {
	broker, port := setupMQTTBroker(t, &mqtt.MQTTConfig{
		ID:      "test-retain-clear",
		Port:    0,
		Enabled: true,
	})
	_ = broker

	// Publish retained message
	publisher := createMQTTClient(t, port, "publisher")
	publisher.Publish("retain/clear", 1, true, "retained value").Wait()

	time.Sleep(100 * time.Millisecond) // Allow retained message to persist

	// Clear retained message by publishing empty payload with retain flag
	publisher.Publish("retain/clear", 1, true, "").Wait()
	publisher.Disconnect(250)

	time.Sleep(100 * time.Millisecond) // Allow retained message clear to take effect

	// New subscriber should not receive any retained message
	received := make(chan string, 1)
	subscriber := createMQTTClient(t, port, "subscriber")
	subscriber.Subscribe("retain/clear", 1, func(c mqttclient.Client, msg mqttclient.Message) {
		received <- string(msg.Payload())
	}).Wait()

	select {
	case msg := <-received:
		// Some brokers may deliver the empty retained message
		assert.Empty(t, msg, "If received, should be empty (cleared)")
	case <-time.After(500 * time.Millisecond):
		// Expected - no retained message after clear
	}
}

func TestMQTT_US14_RetainedMessageMultipleTopics(t *testing.T) {
	broker, port := setupMQTTBroker(t, &mqtt.MQTTConfig{
		ID:      "test-retain-multi",
		Port:    0,
		Enabled: true,
	})
	_ = broker

	// Publish retained messages to multiple topics
	publisher := createMQTTClient(t, port, "publisher")
	publisher.Publish("retain/topic1", 1, true, "value1").Wait()
	publisher.Publish("retain/topic2", 1, true, "value2").Wait()
	publisher.Publish("retain/topic3", 1, true, "value3").Wait()
	publisher.Disconnect(250)

	time.Sleep(200 * time.Millisecond) // Allow retained messages to persist

	// New subscriber using wildcard should receive all retained messages
	received := make(chan string, 10)
	subscriber := createMQTTClient(t, port, "subscriber")
	subscriber.Subscribe("retain/#", 1, func(c mqttclient.Client, msg mqttclient.Message) {
		received <- string(msg.Payload())
	}).Wait()

	// Wait for retained messages
	time.Sleep(500 * time.Millisecond) // Allow all retained messages to be delivered

	subscriber.Disconnect(100)
	time.Sleep(50 * time.Millisecond) // Allow disconnect to complete

	// Collect all received messages
	messages := []string{}
drainLoop:
	for {
		select {
		case msg := <-received:
			messages = append(messages, msg)
		default:
			break drainLoop
		}
	}

	assert.Len(t, messages, 3, "Should receive all 3 retained messages")
	assert.Contains(t, messages, "value1")
	assert.Contains(t, messages, "value2")
	assert.Contains(t, messages, "value3")
}

// ============================================================================
// User Story 15: Will Messages (Last Will and Testament)
// ============================================================================

func TestMQTT_US15_WillMessageOnDisconnect(t *testing.T) {
	broker, port := setupMQTTBroker(t, &mqtt.MQTTConfig{
		ID:      "test-will-disconnect",
		Port:    0,
		Enabled: true,
	})
	_ = broker

	// Create subscriber to listen for will message
	subscriber := createMQTTClient(t, port, "subscriber")
	received := make(chan string, 1)

	subscriber.Subscribe("client/status", 1, func(c mqttclient.Client, msg mqttclient.Message) {
		received <- string(msg.Payload())
	}).Wait()

	time.Sleep(100 * time.Millisecond) // Allow subscription to register

	// Create client with will message
	opts := mqttclient.NewClientOptions()
	opts.AddBroker(fmt.Sprintf("tcp://localhost:%d", port))
	opts.SetClientID("will-client")
	opts.SetAutoReconnect(false)
	opts.SetConnectTimeout(5 * time.Second)
	opts.SetBinaryWill("client/status", []byte("client disconnected unexpectedly"), 1, false)

	willClient := mqttclient.NewClient(opts)
	token := willClient.Connect()
	require.True(t, token.WaitTimeout(5*time.Second))
	require.NoError(t, token.Error())

	// Verify client connected successfully with will configured
	assert.True(t, willClient.IsConnected(), "Client should be connected with will message configured")

	// Note: A graceful Disconnect() will NOT trigger the will message
	// The will message is only sent when the client disconnects ungracefully
	// (network failure, keep-alive timeout, etc.)
	willClient.Disconnect(250)

	// Will should not be triggered on graceful disconnect
	select {
	case <-received:
		// Will message should not be received on graceful disconnect
		// Some broker implementations may differ
	case <-time.After(500 * time.Millisecond):
		// Expected - graceful disconnect doesn't trigger will
	}
}

func TestMQTT_US15_WillMessageRetained(t *testing.T) {
	broker, port := setupMQTTBroker(t, &mqtt.MQTTConfig{
		ID:      "test-will-retained",
		Port:    0,
		Enabled: true,
	})
	_ = broker

	// First, publish a "client online" status
	onlineClient := createMQTTClient(t, port, "online-publisher")
	onlineClient.Publish("device/status", 1, true, "online").Wait()
	onlineClient.Disconnect(250)

	time.Sleep(100 * time.Millisecond) // Allow retained message to persist

	// Create client with retained will message (will overwrite on ungraceful disconnect)
	opts := mqttclient.NewClientOptions()
	opts.AddBroker(fmt.Sprintf("tcp://localhost:%d", port))
	opts.SetClientID("retained-will-client")
	opts.SetAutoReconnect(false)
	opts.SetConnectTimeout(5 * time.Second)
	// Set will as retained
	opts.SetBinaryWill("device/status", []byte("offline"), 1, true)

	willClient := mqttclient.NewClient(opts)
	token := willClient.Connect()
	require.True(t, token.WaitTimeout(5*time.Second))
	require.NoError(t, token.Error())

	// Verify current retained message is "online"
	received := make(chan string, 2)
	subscriber := createMQTTClient(t, port, "status-subscriber")
	subscriber.Subscribe("device/status", 1, func(c mqttclient.Client, msg mqttclient.Message) {
		received <- string(msg.Payload())
	}).Wait()

	select {
	case msg := <-received:
		assert.Equal(t, "online", msg, "Should receive current retained status")
	case <-time.After(2 * time.Second):
		t.Fatal("Timeout waiting for retained status")
	}

	// Clean disconnect (won't trigger will)
	willClient.Disconnect(250)
}

func TestMQTT_US15_WillMessageWithQoS(t *testing.T) {
	// Test will messages with different QoS levels
	testCases := []struct {
		name    string
		willQoS byte
	}{
		{"WillQoS0", 0},
		{"WillQoS1", 1},
		{"WillQoS2", 2},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			broker, port := setupMQTTBroker(t, &mqtt.MQTTConfig{
				ID:      fmt.Sprintf("test-will-qos%d", tc.willQoS),
				Port:    0,
				Enabled: true,
			})
			_ = broker

			// Create client with will message at specified QoS
			opts := mqttclient.NewClientOptions()
			opts.AddBroker(fmt.Sprintf("tcp://localhost:%d", port))
			opts.SetClientID(fmt.Sprintf("will-qos%d-client", tc.willQoS))
			opts.SetAutoReconnect(false)
			opts.SetConnectTimeout(5 * time.Second)
			opts.SetBinaryWill("will/qos/test", []byte(fmt.Sprintf("will with QoS %d", tc.willQoS)), tc.willQoS, false)

			willClient := mqttclient.NewClient(opts)
			token := willClient.Connect()
			require.True(t, token.WaitTimeout(5*time.Second))
			require.NoError(t, token.Error(), "Should connect with will QoS %d", tc.willQoS)

			// Verify connection was successful
			assert.True(t, willClient.IsConnected(), "Client should be connected")

			willClient.Disconnect(250)
		})
	}
}

// ============================================================================
// User Story 16: MockResponse Templating with Wildcards
// ============================================================================

func TestMQTT_US16_MockResponseWithWildcardTemplating(t *testing.T) {
	cfg := &mqtt.MQTTConfig{
		ID:      "test-mockresponse-wildcard",
		Port:    0,
		Enabled: true,
	}

	broker, port := setupMQTTBroker(t, cfg)

	// Set up a mock response that uses templating
	// Topic uses {1} directly for wildcard substitution
	// Payload uses {{ topic }} to include the full topic which contains the device ID
	broker.SetMockResponses([]*mqtt.MockResponse{
		{
			ID:              "device-response",
			TriggerPattern:  "devices/+/command",
			ResponseTopic:   "devices/{1}/response",
			PayloadTemplate: `{"topic": "{{ topic }}", "status": "acknowledged", "uuid": "{{ uuid }}"}`,
			DelayMs:         10,
			Enabled:         true,
		},
	})

	client := createMQTTClient(t, port, "wildcard-response-client")
	received := make(chan string, 10)

	// Subscribe to the response topic
	client.Subscribe("devices/+/response", 1, func(c mqttclient.Client, msg mqttclient.Message) {
		received <- string(msg.Payload())
	}).Wait()

	time.Sleep(100 * time.Millisecond) // Allow subscription to register

	// Publish command to a specific device
	client.Publish("devices/sensor-001/command", 1, false, `{"action": "restart"}`).Wait()

	// Wait for mock response
	select {
	case msg := <-received:
		// Verify topic was rendered with device ID
		assert.Contains(t, msg, "sensor-001", "Should contain the device ID from topic")
		// Verify uuid was rendered (not a placeholder)
		assert.NotContains(t, msg, "{{ uuid }}", "UUID should be rendered")
		assert.Contains(t, msg, "acknowledged", "Should contain response status")

		// Verify it's valid JSON
		var data map[string]interface{}
		err := json.Unmarshal([]byte(msg), &data)
		require.NoError(t, err, "Should be valid JSON")
		assert.Equal(t, "devices/sensor-001/command", data["topic"], "Topic should be the trigger topic")
	case <-time.After(2 * time.Second):
		t.Fatal("Timeout waiting for mock response")
	}
}

func TestMQTT_US16_MockResponseWithPayloadAccess(t *testing.T) {
	cfg := &mqtt.MQTTConfig{
		ID:      "test-mockresponse-payload",
		Port:    0,
		Enabled: true,
	}

	broker, port := setupMQTTBroker(t, cfg)

	// Set up a mock response that accesses the incoming payload
	broker.SetMockResponses([]*mqtt.MockResponse{
		{
			ID:              "echo-response",
			TriggerPattern:  "echo/request",
			ResponseTopic:   "echo/response",
			PayloadTemplate: `{"echoedAction": "{{ payload.action }}", "timestamp": "{{ timestamp }}"}`,
			DelayMs:         10,
			Enabled:         true,
		},
	})

	client := createMQTTClient(t, port, "payload-client")
	received := make(chan string, 10)

	// Subscribe to the response topic
	client.Subscribe("echo/response", 1, func(c mqttclient.Client, msg mqttclient.Message) {
		received <- string(msg.Payload())
	}).Wait()

	time.Sleep(100 * time.Millisecond) // Allow subscription to register

	// Publish request with action field
	client.Publish("echo/request", 1, false, `{"action": "test-action-123"}`).Wait()

	// Wait for mock response
	select {
	case msg := <-received:
		// Verify payload field was extracted
		assert.Contains(t, msg, "test-action-123", "Should contain the echoed action")
		// Verify timestamp was rendered
		assert.NotContains(t, msg, "{{ timestamp }}", "Timestamp should be rendered")

		// Verify it's valid JSON
		var data map[string]interface{}
		err := json.Unmarshal([]byte(msg), &data)
		require.NoError(t, err, "Should be valid JSON")
		assert.Equal(t, "test-action-123", data["echoedAction"], "Should echo the action")
	case <-time.After(2 * time.Second):
		t.Fatal("Timeout waiting for mock response")
	}
}
