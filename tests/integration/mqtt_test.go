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

// getFreeMQTTPort returns an available port for MQTT testing
func getFreeMQTTPort(t *testing.T) int {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	port := l.Addr().(*net.TCPAddr).Port
	l.Close()
	return port
}

// setupMQTTBroker creates and starts an MQTT broker for testing
func setupMQTTBroker(t *testing.T, cfg *mqtt.MQTTConfig) *mqtt.Broker {
	broker, err := mqtt.NewBroker(cfg)
	require.NoError(t, err)

	err = broker.Start(context.Background())
	require.NoError(t, err)

	t.Cleanup(func() {
		broker.Stop(context.Background(), 5*time.Second)
	})

	// Wait for broker to be ready
	time.Sleep(100 * time.Millisecond)

	return broker
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
	port := getFreeMQTTPort(t)

	cfg := &mqtt.MQTTConfig{
		ID:      "test-broker",
		Name:    "Test Broker",
		Port:    port,
		Enabled: true,
	}

	broker, err := mqtt.NewBroker(cfg)
	require.NoError(t, err)

	// Start broker
	err = broker.Start(context.Background())
	require.NoError(t, err)
	assert.True(t, broker.IsRunning())

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
	port := getFreeMQTTPort(t)
	broker := setupMQTTBroker(t, &mqtt.MQTTConfig{
		ID:      "test-pubsub",
		Port:    port,
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
	port := getFreeMQTTPort(t)
	broker := setupMQTTBroker(t, &mqtt.MQTTConfig{
		ID:      "test-wildcard-single",
		Port:    port,
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

	time.Sleep(500 * time.Millisecond)

	// Disconnect client before draining channel to prevent race
	client.Disconnect(100)
	time.Sleep(50 * time.Millisecond)

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
	port := getFreeMQTTPort(t)
	broker := setupMQTTBroker(t, &mqtt.MQTTConfig{
		ID:      "test-wildcard-multi",
		Port:    port,
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

	time.Sleep(500 * time.Millisecond)

	// Disconnect client before draining channel to prevent race
	client.Disconnect(100)
	time.Sleep(50 * time.Millisecond)

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
	port := getFreeMQTTPort(t)
	broker := setupMQTTBroker(t, &mqtt.MQTTConfig{
		ID:      "test-qos0",
		Port:    port,
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
	port := getFreeMQTTPort(t)
	broker := setupMQTTBroker(t, &mqtt.MQTTConfig{
		ID:      "test-qos1",
		Port:    port,
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
	port := getFreeMQTTPort(t)
	broker := setupMQTTBroker(t, &mqtt.MQTTConfig{
		ID:      "test-qos2",
		Port:    port,
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
	port := getFreeMQTTPort(t)
	broker := setupMQTTBroker(t, &mqtt.MQTTConfig{
		ID:      "test-retain",
		Port:    port,
		Enabled: true,
	})
	_ = broker

	// First client publishes retained message
	publisher := createMQTTClient(t, port, "publisher")
	publisher.Publish("retain/test", 1, true, "retained value").Wait()
	publisher.Disconnect(250)

	time.Sleep(200 * time.Millisecond)

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
	port := getFreeMQTTPort(t)

	cfg := &mqtt.MQTTConfig{
		ID:      "test-topics",
		Port:    port,
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

	broker := setupMQTTBroker(t, cfg)
	_ = broker

	client := createMQTTClient(t, port, "sensor-sub")
	received := make(chan string, 5)

	client.Subscribe("sensors/temperature", 1, func(c mqttclient.Client, msg mqttclient.Message) {
		received <- string(msg.Payload())
	}).Wait()

	// Wait for auto-published messages
	time.Sleep(350 * time.Millisecond)

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
	port := getFreeMQTTPort(t)

	cfg := &mqtt.MQTTConfig{
		ID:      "test-onpublish",
		Port:    port,
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

	broker := setupMQTTBroker(t, cfg)
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
	port := getFreeMQTTPort(t)

	cfg := &mqtt.MQTTConfig{
		ID:      "test-forward",
		Port:    port,
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

	broker := setupMQTTBroker(t, cfg)
	_ = broker

	client := createMQTTClient(t, port, "forward-client")
	received := make(chan string, 1)

	// Subscribe to forwarded topic
	client.Subscribe("output/data", 1, func(c mqttclient.Client, msg mqttclient.Message) {
		received <- string(msg.Payload())
	}).Wait()

	time.Sleep(100 * time.Millisecond)

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
	port := getFreeMQTTPort(t)

	cfg := &mqtt.MQTTConfig{
		ID:      "test-auth",
		Port:    port,
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

	broker := setupMQTTBroker(t, cfg)
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
	port := getFreeMQTTPort(t)

	cfg := &mqtt.MQTTConfig{
		ID:      "test-auth-success",
		Port:    port,
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

	broker := setupMQTTBroker(t, cfg)
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
	port := getFreeMQTTPort(t)
	broker := setupMQTTBroker(t, &mqtt.MQTTConfig{
		ID:      "test-multi-client",
		Port:    port,
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

	time.Sleep(500 * time.Millisecond)

	// Publish a message
	publisher := createMQTTClient(t, port, "broadcaster")
	publisher.Publish("broadcast", 1, false, "hello all").Wait()

	time.Sleep(500 * time.Millisecond)

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
	mqttPort := getFreeMQTTPort(t)
	managementPort := getFreePort()

	// Create server with MQTT mock via ImportConfig
	serverCfg := config.DefaultServerConfiguration()
	serverCfg.HTTPPort = 0 // Disable HTTP for this test
	serverCfg.ManagementPort = managementPort
	server := engine.NewServer(serverCfg)

	// Import MQTT mock configuration
	mqttMockCfg := &config.MockConfiguration{
		ID:      "test-mqtt-mock",
		Type:    mock.MockTypeMQTT,
		Name:    "Test MQTT",
		Enabled: true,
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

	time.Sleep(500 * time.Millisecond)

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
	port := getFreeMQTTPort(t)
	broker := setupMQTTBroker(t, &mqtt.MQTTConfig{
		ID:      "test-json",
		Port:    port,
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
	port := getFreeMQTTPort(t)
	broker := setupMQTTBroker(t, &mqtt.MQTTConfig{
		ID:      "test-stats",
		Port:    port,
		Enabled: true,
	})

	// Connect a client
	client := createMQTTClient(t, port, "stats-client")
	client.Subscribe("stats/test", 1, nil).Wait()
	client.Publish("stats/test", 1, false, "test").Wait()

	time.Sleep(200 * time.Millisecond)

	stats := broker.GetStats()
	assert.Equal(t, port, stats.Port)
	assert.True(t, stats.Running)
	assert.GreaterOrEqual(t, stats.ClientCount, 1)
}
