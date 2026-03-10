package mqtt

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// ResponseHandler CRUD helpers
// ---------------------------------------------------------------------------

func TestResponseHandler_SetAndGetResponses(t *testing.T) {
	config := &MQTTConfig{Port: 0, Enabled: true}
	broker, err := NewBroker(config)
	require.NoError(t, err)

	h := NewResponseHandler(broker)

	responses := []*MockResponse{
		{ID: "r1", TriggerPattern: "a/+", Enabled: true},
		{ID: "r2", TriggerPattern: "b/#", Enabled: false},
	}
	h.SetResponses(responses)

	got := h.GetResponses()
	require.Len(t, got, 2)
	assert.Equal(t, "r1", got[0].ID)
	assert.Equal(t, "r2", got[1].ID)

	// GetResponses returns a copy of the slice (not the backing array),
	// so appending to the returned slice does not affect internal state.
	got = append(got, &MockResponse{ID: "r3"})
	assert.Len(t, h.GetResponses(), 2, "appending to returned slice must not grow internal list")
}

func TestResponseHandler_AddResponse(t *testing.T) {
	config := &MQTTConfig{Port: 0, Enabled: true}
	broker, err := NewBroker(config)
	require.NoError(t, err)

	h := NewResponseHandler(broker)

	h.AddResponse(&MockResponse{ID: "r1", TriggerPattern: "x/y", Enabled: true})
	h.AddResponse(&MockResponse{ID: "r2", TriggerPattern: "x/z", Enabled: true})

	assert.Len(t, h.GetResponses(), 2)
}

func TestResponseHandler_RemoveResponse(t *testing.T) {
	config := &MQTTConfig{Port: 0, Enabled: true}
	broker, err := NewBroker(config)
	require.NoError(t, err)

	h := NewResponseHandler(broker)
	h.SetResponses([]*MockResponse{
		{ID: "r1", TriggerPattern: "a/b", Enabled: true},
		{ID: "r2", TriggerPattern: "c/d", Enabled: true},
		{ID: "r3", TriggerPattern: "e/f", Enabled: true},
	})

	h.RemoveResponse("r2")
	got := h.GetResponses()
	require.Len(t, got, 2)
	assert.Equal(t, "r1", got[0].ID)
	assert.Equal(t, "r3", got[1].ID)

	// Removing a non-existent ID is a no-op.
	h.RemoveResponse("does-not-exist")
	assert.Len(t, h.GetResponses(), 2)
}

// ---------------------------------------------------------------------------
// FindMatchingResponses
// ---------------------------------------------------------------------------

func TestFindMatchingResponses_ExactMatch(t *testing.T) {
	config := &MQTTConfig{Port: 0, Enabled: true}
	broker, err := NewBroker(config)
	require.NoError(t, err)

	h := NewResponseHandler(broker)
	h.SetResponses([]*MockResponse{
		{ID: "r1", TriggerPattern: "sensors/temp", Enabled: true},
		{ID: "r2", TriggerPattern: "sensors/humidity", Enabled: true},
	})

	matches := h.FindMatchingResponses("sensors/temp")
	require.Len(t, matches, 1)
	assert.Equal(t, "r1", matches[0].Response.ID)
	assert.Equal(t, "sensors/temp", matches[0].Topic)
	assert.Empty(t, matches[0].Wildcards)
}

func TestFindMatchingResponses_SingleLevelWildcard(t *testing.T) {
	config := &MQTTConfig{Port: 0, Enabled: true}
	broker, err := NewBroker(config)
	require.NoError(t, err)

	h := NewResponseHandler(broker)
	h.SetResponses([]*MockResponse{
		{ID: "r1", TriggerPattern: "devices/+/command", Enabled: true},
	})

	matches := h.FindMatchingResponses("devices/thermostat/command")
	require.Len(t, matches, 1)
	assert.Equal(t, "r1", matches[0].Response.ID)
	assert.Equal(t, []string{"thermostat"}, matches[0].Wildcards)

	// Non-matching topic
	matches = h.FindMatchingResponses("devices/thermostat/data")
	assert.Empty(t, matches)
}

func TestFindMatchingResponses_MultiLevelWildcard(t *testing.T) {
	config := &MQTTConfig{Port: 0, Enabled: true}
	broker, err := NewBroker(config)
	require.NoError(t, err)

	h := NewResponseHandler(broker)
	h.SetResponses([]*MockResponse{
		{ID: "r1", TriggerPattern: "home/#", Enabled: true},
	})

	matches := h.FindMatchingResponses("home/living/light/brightness")
	require.Len(t, matches, 1)
	assert.Equal(t, []string{"living/light/brightness"}, matches[0].Wildcards)
}

func TestFindMatchingResponses_DisabledSkipped(t *testing.T) {
	config := &MQTTConfig{Port: 0, Enabled: true}
	broker, err := NewBroker(config)
	require.NoError(t, err)

	h := NewResponseHandler(broker)
	h.SetResponses([]*MockResponse{
		{ID: "r1", TriggerPattern: "test/#", Enabled: false},
		{ID: "r2", TriggerPattern: "test/#", Enabled: true},
	})

	matches := h.FindMatchingResponses("test/topic")
	require.Len(t, matches, 1)
	assert.Equal(t, "r2", matches[0].Response.ID)
}

func TestFindMatchingResponses_MultipleMatches(t *testing.T) {
	config := &MQTTConfig{Port: 0, Enabled: true}
	broker, err := NewBroker(config)
	require.NoError(t, err)

	h := NewResponseHandler(broker)
	h.SetResponses([]*MockResponse{
		{ID: "r1", TriggerPattern: "sensors/+/temp", Enabled: true},
		{ID: "r2", TriggerPattern: "sensors/#", Enabled: true},
		{ID: "r3", TriggerPattern: "other/topic", Enabled: true},
	})

	matches := h.FindMatchingResponses("sensors/room1/temp")
	require.Len(t, matches, 2)

	ids := []string{matches[0].Response.ID, matches[1].Response.ID}
	assert.Contains(t, ids, "r1")
	assert.Contains(t, ids, "r2")
}

func TestFindMatchingResponses_NoMatch(t *testing.T) {
	config := &MQTTConfig{Port: 0, Enabled: true}
	broker, err := NewBroker(config)
	require.NoError(t, err)

	h := NewResponseHandler(broker)
	h.SetResponses([]*MockResponse{
		{ID: "r1", TriggerPattern: "sensors/temp", Enabled: true},
	})

	matches := h.FindMatchingResponses("other/topic")
	assert.Empty(t, matches)
}

// ---------------------------------------------------------------------------
// HandlePublish — integration test with a running broker
// ---------------------------------------------------------------------------

func TestHandlePublish_TriggersResponseOnAnotherTopic(t *testing.T) {
	config := &MQTTConfig{
		Port:    18950,
		Enabled: true,
	}
	broker, err := NewBroker(config)
	require.NoError(t, err)

	require.NoError(t, broker.Start(context.Background()))
	defer broker.Stop(context.Background(), 5*time.Second)
	time.Sleep(100 * time.Millisecond)

	broker.responseHandler.SetResponses([]*MockResponse{
		{
			ID:              "resp1",
			TriggerPattern:  "commands/+/request",
			ResponseTopic:   "commands/{1}/response",
			PayloadTemplate: `{"status":"ok"}`,
			Enabled:         true,
		},
	})

	var received []byte
	var mu sync.Mutex
	done := make(chan struct{})

	broker.Subscribe("commands/device1/response", func(topic string, payload []byte) {
		mu.Lock()
		received = payload
		mu.Unlock()
		close(done)
	})
	defer broker.Unsubscribe("commands/device1/response")

	// Trigger by publishing to the request topic.
	broker.responseHandler.HandlePublish("commands/device1/request", []byte(`{}`), "client1")

	select {
	case <-done:
		mu.Lock()
		assert.JSONEq(t, `{"status":"ok"}`, string(received))
		mu.Unlock()
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for mock response")
	}
}

func TestHandlePublish_DisabledResponseNotTriggered(t *testing.T) {
	config := &MQTTConfig{
		Port:    18951,
		Enabled: true,
	}
	broker, err := NewBroker(config)
	require.NoError(t, err)

	require.NoError(t, broker.Start(context.Background()))
	defer broker.Stop(context.Background(), 5*time.Second)
	time.Sleep(100 * time.Millisecond)

	broker.responseHandler.SetResponses([]*MockResponse{
		{
			ID:              "resp-disabled",
			TriggerPattern:  "test/topic",
			ResponseTopic:   "test/topic/response",
			PayloadTemplate: `{"msg":"should not arrive"}`,
			Enabled:         false,
		},
	})

	received := make(chan struct{}, 1)
	broker.Subscribe("test/topic/response", func(topic string, payload []byte) {
		received <- struct{}{}
	})
	defer broker.Unsubscribe("test/topic/response")

	broker.responseHandler.HandlePublish("test/topic", []byte(`{}`), "client1")

	select {
	case <-received:
		t.Fatal("disabled response should not have been triggered")
	case <-time.After(300 * time.Millisecond):
		// expected — nothing received
	}
}

func TestHandlePublish_DefaultResponseTopicSuffix(t *testing.T) {
	config := &MQTTConfig{
		Port:    18952,
		Enabled: true,
	}
	broker, err := NewBroker(config)
	require.NoError(t, err)

	require.NoError(t, broker.Start(context.Background()))
	defer broker.Stop(context.Background(), 5*time.Second)
	time.Sleep(100 * time.Millisecond)

	// ResponseTopic is empty → defaults to triggerTopic + "/response"
	broker.responseHandler.SetResponses([]*MockResponse{
		{
			ID:              "resp-default",
			TriggerPattern:  "sensors/temp",
			ResponseTopic:   "",
			PayloadTemplate: `{"ack":true}`,
			Enabled:         true,
		},
	})

	var received []byte
	var mu sync.Mutex
	done := make(chan struct{})

	broker.Subscribe("sensors/temp/response", func(topic string, payload []byte) {
		mu.Lock()
		received = payload
		mu.Unlock()
		close(done)
	})
	defer broker.Unsubscribe("sensors/temp/response")

	broker.responseHandler.HandlePublish("sensors/temp", []byte(`{}`), "client1")

	select {
	case <-done:
		mu.Lock()
		assert.JSONEq(t, `{"ack":true}`, string(received))
		mu.Unlock()
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for default-topic response")
	}
}

// ---------------------------------------------------------------------------
// executeResponse — delay
// ---------------------------------------------------------------------------

func TestExecuteResponse_Delay(t *testing.T) {
	config := &MQTTConfig{
		Port:    18953,
		Enabled: true,
	}
	broker, err := NewBroker(config)
	require.NoError(t, err)

	require.NoError(t, broker.Start(context.Background()))
	defer broker.Stop(context.Background(), 5*time.Second)
	time.Sleep(100 * time.Millisecond)

	broker.responseHandler.SetResponses([]*MockResponse{
		{
			ID:              "resp-delay",
			TriggerPattern:  "delay/trigger",
			ResponseTopic:   "delay/result",
			PayloadTemplate: `{"delayed":true}`,
			DelayMs:         200,
			Enabled:         true,
		},
	})

	done := make(chan struct{})
	start := time.Now()

	broker.Subscribe("delay/result", func(topic string, payload []byte) {
		close(done)
	})
	defer broker.Unsubscribe("delay/result")

	broker.responseHandler.HandlePublish("delay/trigger", []byte(`{}`), "client1")

	select {
	case <-done:
		elapsed := time.Since(start)
		assert.GreaterOrEqual(t, elapsed, 180*time.Millisecond, "response should be delayed ~200ms")
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for delayed response")
	}
}

// ---------------------------------------------------------------------------
// executeResponse — loop prevention
// ---------------------------------------------------------------------------

func TestExecuteResponse_LoopPrevention(t *testing.T) {
	config := &MQTTConfig{
		Port:    18954,
		Enabled: true,
	}
	broker, err := NewBroker(config)
	require.NoError(t, err)

	require.NoError(t, broker.Start(context.Background()))
	defer broker.Stop(context.Background(), 5*time.Second)
	time.Sleep(100 * time.Millisecond)

	// Simulate a loop scenario: manually mark the response topic as active
	// before calling HandlePublish.
	broker.markMockResponseTopic("loop/output")

	broker.responseHandler.SetResponses([]*MockResponse{
		{
			ID:              "resp-loop",
			TriggerPattern:  "loop/input",
			ResponseTopic:   "loop/output",
			PayloadTemplate: `{"should":"not publish"}`,
			Enabled:         true,
		},
	})

	received := make(chan struct{}, 1)
	broker.Subscribe("loop/output", func(topic string, payload []byte) {
		received <- struct{}{}
	})
	defer broker.Unsubscribe("loop/output")

	broker.responseHandler.HandlePublish("loop/input", []byte(`{}`), "client1")

	select {
	case <-received:
		t.Fatal("loop-prevented response should not have been published")
	case <-time.After(300 * time.Millisecond):
		// expected — loop was blocked
	}

	// Clean up
	broker.unmarkMockResponseTopic("loop/output")
}

// ---------------------------------------------------------------------------
// executeResponse — wildcard substitution in response topic
// ---------------------------------------------------------------------------

func TestExecuteResponse_WildcardSubstitution(t *testing.T) {
	config := &MQTTConfig{
		Port:    18955,
		Enabled: true,
	}
	broker, err := NewBroker(config)
	require.NoError(t, err)

	require.NoError(t, broker.Start(context.Background()))
	defer broker.Stop(context.Background(), 5*time.Second)
	time.Sleep(100 * time.Millisecond)

	broker.responseHandler.SetResponses([]*MockResponse{
		{
			ID:              "resp-wildcard",
			TriggerPattern:  "+/+/command",
			ResponseTopic:   "{1}/{2}/status",
			PayloadTemplate: `{"from":"{1}","device":"{2}"}`,
			Enabled:         true,
		},
	})

	var receivedTopic string
	var mu sync.Mutex
	done := make(chan struct{})

	// The response should land on "floor1/thermostat/status"
	broker.Subscribe("floor1/thermostat/status", func(topic string, payload []byte) {
		mu.Lock()
		receivedTopic = topic
		mu.Unlock()
		close(done)
	})
	defer broker.Unsubscribe("floor1/thermostat/status")

	broker.responseHandler.HandlePublish("floor1/thermostat/command", []byte(`{}`), "client1")

	select {
	case <-done:
		mu.Lock()
		assert.Equal(t, "floor1/thermostat/status", receivedTopic)
		mu.Unlock()
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for wildcard-substituted response")
	}
}
