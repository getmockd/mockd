package mqtt

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestConditionalResponseIntegration tests the full flow of conditional responses
func TestConditionalResponseIntegration(t *testing.T) {
	// Create and start a broker
	config := &MQTTConfig{
		ID:      "integration-test",
		Port:    19883, // Use a non-standard port to avoid conflicts
		Enabled: true,
	}
	broker, err := NewBroker(config)
	require.NoError(t, err)

	err = broker.Start(context.Background())
	require.NoError(t, err)
	defer broker.Stop(context.Background(), 5*time.Second)

	// Give broker time to start
	time.Sleep(100 * time.Millisecond)

	// Set up conditional response: if command == "on", respond with powered state
	condResp := &ConditionalResponse{
		ID:             "test-conditional",
		Name:           "Device Control",
		TriggerPattern: "devices/+/command",
		Rules: []ConditionalRule{
			{
				ID:              "rule-on",
				Name:            "Power On",
				Priority:        1,
				Condition:       "$.command == 'on'",
				ResponseTopic:   "devices/{1}/status",
				PayloadTemplate: `{"state": "powered", "command": "{{ payload.command }}"}`,
				Enabled:         true,
			},
			{
				ID:              "rule-off",
				Name:            "Power Off",
				Priority:        2,
				Condition:       "$.command == 'off'",
				ResponseTopic:   "devices/{1}/status",
				PayloadTemplate: `{"state": "standby", "command": "{{ payload.command }}"}`,
				Enabled:         true,
			},
			{
				ID:              "rule-high-temp",
				Name:            "High Temperature Alert",
				Priority:        0, // Highest priority
				Condition:       "$.temperature > 30",
				ResponseTopic:   "devices/{1}/alert",
				PayloadTemplate: `{"alert": "high_temperature", "value": {{ payload.temperature }}}`,
				Enabled:         true,
			},
		},
		DefaultResponse: &MockResponse{
			ID:              "default",
			ResponseTopic:   "devices/{1}/status",
			PayloadTemplate: `{"state": "unknown", "error": "unrecognized command"}`,
			Enabled:         true,
		},
		Enabled: true,
	}

	broker.conditionalResponseHandler.AddConditionalResponse(condResp)

	// Set up message capture
	var capturedMessages []struct {
		Topic   string
		Payload map[string]any
	}
	var mu sync.Mutex

	broker.Subscribe("devices/#", func(topic string, payload []byte) {
		var p map[string]any
		json.Unmarshal(payload, &p)
		mu.Lock()
		capturedMessages = append(capturedMessages, struct {
			Topic   string
			Payload map[string]any
		}{topic, p})
		mu.Unlock()
	})

	// Test case 1: Command "on" should trigger power on response
	t.Run("command_on_triggers_powered_response", func(t *testing.T) {
		capturedMessages = nil

		payload := []byte(`{"command": "on"}`)
		err := broker.Publish("devices/device1/command", payload, 0, false)
		require.NoError(t, err)

		// Wait for response
		time.Sleep(200 * time.Millisecond)

		mu.Lock()
		defer mu.Unlock()

		// Should have the original command and the response
		require.GreaterOrEqual(t, len(capturedMessages), 1)

		// Find the status response
		var found bool
		for _, msg := range capturedMessages {
			if msg.Topic == "devices/device1/status" && msg.Payload["state"] == "powered" {
				found = true
				assert.Equal(t, "on", msg.Payload["command"])
				break
			}
		}
		assert.True(t, found, "Expected powered status response")
	})

	// Test case 2: Command "off" should trigger standby response
	t.Run("command_off_triggers_standby_response", func(t *testing.T) {
		capturedMessages = nil

		payload := []byte(`{"command": "off"}`)
		err := broker.Publish("devices/device2/command", payload, 0, false)
		require.NoError(t, err)

		time.Sleep(200 * time.Millisecond)

		mu.Lock()
		defer mu.Unlock()

		var found bool
		for _, msg := range capturedMessages {
			if msg.Topic == "devices/device2/status" && msg.Payload["state"] == "standby" {
				found = true
				assert.Equal(t, "off", msg.Payload["command"])
				break
			}
		}
		assert.True(t, found, "Expected standby status response")
	})

	// Test case 3: High temperature triggers alert (highest priority)
	t.Run("high_temp_triggers_alert", func(t *testing.T) {
		capturedMessages = nil

		// This matches both "command == 'on'" (not really) and "temperature > 30"
		// But temperature check should win due to priority 0
		payload := []byte(`{"temperature": 35, "command": "on"}`)
		err := broker.Publish("devices/sensor1/command", payload, 0, false)
		require.NoError(t, err)

		time.Sleep(200 * time.Millisecond)

		mu.Lock()
		defer mu.Unlock()

		var found bool
		for _, msg := range capturedMessages {
			if msg.Topic == "devices/sensor1/alert" {
				found = true
				assert.Equal(t, "high_temperature", msg.Payload["alert"])
				break
			}
		}
		assert.True(t, found, "Expected high temperature alert")
	})

	// Test case 4: Unrecognized command triggers default response
	t.Run("unknown_command_triggers_default", func(t *testing.T) {
		capturedMessages = nil

		payload := []byte(`{"command": "reset"}`)
		err := broker.Publish("devices/device3/command", payload, 0, false)
		require.NoError(t, err)

		time.Sleep(200 * time.Millisecond)

		mu.Lock()
		defer mu.Unlock()

		var found bool
		for _, msg := range capturedMessages {
			if msg.Topic == "devices/device3/status" && msg.Payload["state"] == "unknown" {
				found = true
				assert.Contains(t, msg.Payload["error"], "unrecognized")
				break
			}
		}
		assert.True(t, found, "Expected default response for unknown command")
	})

	// Test case 5: Non-matching topic should not trigger
	t.Run("non_matching_topic_no_response", func(t *testing.T) {
		capturedMessages = nil

		payload := []byte(`{"command": "on"}`)
		err := broker.Publish("other/topic", payload, 0, false)
		require.NoError(t, err)

		time.Sleep(200 * time.Millisecond)

		mu.Lock()
		defer mu.Unlock()

		// Should only have the original message, no conditional response
		for _, msg := range capturedMessages {
			assert.NotContains(t, msg.Topic, "status")
			assert.NotContains(t, msg.Topic, "alert")
		}
	})

	// Test case 6: Disabled conditional response should not trigger
	t.Run("disabled_response_no_trigger", func(t *testing.T) {
		// Disable the conditional response
		resp := broker.conditionalResponseHandler.GetConditionalResponse("test-conditional")
		require.NotNil(t, resp)
		resp.Enabled = false
		broker.conditionalResponseHandler.UpdateConditionalResponse(resp)

		capturedMessages = nil

		payload := []byte(`{"command": "on"}`)
		err := broker.Publish("devices/device4/command", payload, 0, false)
		require.NoError(t, err)

		time.Sleep(200 * time.Millisecond)

		mu.Lock()
		defer mu.Unlock()

		// Should have no status/alert responses since conditional is disabled
		for _, msg := range capturedMessages {
			if msg.Topic == "devices/device4/status" || msg.Topic == "devices/device4/alert" {
				t.Error("Expected no conditional response when disabled")
			}
		}

		// Re-enable for other tests
		resp.Enabled = true
		broker.conditionalResponseHandler.UpdateConditionalResponse(resp)
	})
}

// TestConditionalResponseWithDelay tests that delays are respected
func TestConditionalResponseWithDelay(t *testing.T) {
	config := &MQTTConfig{
		ID:      "delay-test",
		Port:    19884,
		Enabled: true,
	}
	broker, err := NewBroker(config)
	require.NoError(t, err)

	err = broker.Start(context.Background())
	require.NoError(t, err)
	defer broker.Stop(context.Background(), 5*time.Second)

	time.Sleep(100 * time.Millisecond)

	// Set up conditional response with 500ms delay
	condResp := &ConditionalResponse{
		ID:             "delayed-response",
		TriggerPattern: "test/#",
		Rules: []ConditionalRule{
			{
				ID:              "delayed-rule",
				Condition:       "$.value == true",
				ResponseTopic:   "test/response",
				PayloadTemplate: `{"received": true}`,
				DelayMs:         500,
				Enabled:         true,
			},
		},
		Enabled: true,
	}
	broker.conditionalResponseHandler.AddConditionalResponse(condResp)

	var responseReceived bool
	var responseTime time.Time
	var mu sync.Mutex

	broker.Subscribe("test/response", func(topic string, payload []byte) {
		mu.Lock()
		responseReceived = true
		responseTime = time.Now()
		mu.Unlock()
	})

	startTime := time.Now()
	err = broker.Publish("test/trigger", []byte(`{"value": true}`), 0, false)
	require.NoError(t, err)

	// Check immediately - should not have response yet
	time.Sleep(200 * time.Millisecond)
	mu.Lock()
	assert.False(t, responseReceived, "Response should not be received before delay")
	mu.Unlock()

	// Wait for delay to pass
	time.Sleep(500 * time.Millisecond)
	mu.Lock()
	assert.True(t, responseReceived, "Response should be received after delay")
	elapsed := responseTime.Sub(startTime)
	assert.GreaterOrEqual(t, elapsed.Milliseconds(), int64(450), "Response should be delayed by at least 450ms")
	mu.Unlock()
}

// TestConditionalResponsePriorityOrdering verifies priority ordering
func TestConditionalResponsePriorityOrdering(t *testing.T) {
	config := &MQTTConfig{
		ID:      "priority-test",
		Port:    19885,
		Enabled: true,
	}
	broker, err := NewBroker(config)
	require.NoError(t, err)

	err = broker.Start(context.Background())
	require.NoError(t, err)
	defer broker.Stop(context.Background(), 5*time.Second)

	time.Sleep(100 * time.Millisecond)

	// Both rules match the same condition, but priority should determine which fires
	condResp := &ConditionalResponse{
		ID:             "priority-test",
		TriggerPattern: "priority/#",
		Rules: []ConditionalRule{
			{
				ID:              "low-priority",
				Priority:        100,
				Condition:       "$.match == true",
				ResponseTopic:   "priority/response",
				PayloadTemplate: `{"winner": "low"}`,
				Enabled:         true,
			},
			{
				ID:              "high-priority",
				Priority:        1,
				Condition:       "$.match == true",
				ResponseTopic:   "priority/response",
				PayloadTemplate: `{"winner": "high"}`,
				Enabled:         true,
			},
			{
				ID:              "medium-priority",
				Priority:        50,
				Condition:       "$.match == true",
				ResponseTopic:   "priority/response",
				PayloadTemplate: `{"winner": "medium"}`,
				Enabled:         true,
			},
		},
		Enabled: true,
	}
	broker.conditionalResponseHandler.AddConditionalResponse(condResp)

	var responses []map[string]any
	var mu sync.Mutex

	broker.Subscribe("priority/response", func(topic string, payload []byte) {
		var p map[string]any
		json.Unmarshal(payload, &p)
		mu.Lock()
		responses = append(responses, p)
		mu.Unlock()
	})

	err = broker.Publish("priority/test", []byte(`{"match": true}`), 0, false)
	require.NoError(t, err)

	time.Sleep(200 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()

	// Should only have one response (first matching rule)
	require.Len(t, responses, 1)
	// And it should be from the high priority rule
	assert.Equal(t, "high", responses[0]["winner"])
}
