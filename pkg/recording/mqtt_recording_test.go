package recording

import (
	"strings"
	"testing"
)

func TestNewMQTTRecording(t *testing.T) {
	topic := "sensor/temperature"
	payload := []byte(`{"value": 23.5}`)
	qos := 1
	retain := true
	clientID := "sensor-001"
	direction := MQTTDirectionPublish

	rec := NewMQTTRecording(topic, payload, qos, retain, clientID, direction)

	if rec.ID == "" {
		t.Error("Expected ID to be set")
	}
	if !strings.HasPrefix(rec.ID, "mqtt-") {
		t.Errorf("Expected ID to have 'mqtt-' prefix, got '%s'", rec.ID)
	}
	if rec.Timestamp.IsZero() {
		t.Error("Expected timestamp to be set")
	}
	if rec.Topic != topic {
		t.Errorf("Expected topic '%s', got '%s'", topic, rec.Topic)
	}
	if string(rec.Payload) != string(payload) {
		t.Errorf("Expected payload '%s', got '%s'", string(payload), string(rec.Payload))
	}
	if rec.QoS != qos {
		t.Errorf("Expected QoS %d, got %d", qos, rec.QoS)
	}
	if rec.Retain != retain {
		t.Errorf("Expected retain %v, got %v", retain, rec.Retain)
	}
	if rec.ClientID != clientID {
		t.Errorf("Expected clientID '%s', got '%s'", clientID, rec.ClientID)
	}
	if rec.Direction != direction {
		t.Errorf("Expected direction '%s', got '%s'", direction, rec.Direction)
	}
}

func TestNewMQTTRecordingWithSubscribeDirection(t *testing.T) {
	rec := NewMQTTRecording("topic/test", []byte("data"), 0, false, "client-1", MQTTDirectionSubscribe)

	if rec.Direction != MQTTDirectionSubscribe {
		t.Errorf("Expected direction '%s', got '%s'", MQTTDirectionSubscribe, rec.Direction)
	}
}

func TestNewMQTTRecordingWithEmptyPayload(t *testing.T) {
	rec := NewMQTTRecording("topic/empty", []byte{}, 0, false, "client-1", MQTTDirectionPublish)

	if rec.Payload == nil {
		t.Error("Expected payload to be empty slice, not nil")
	}
	if len(rec.Payload) != 0 {
		t.Errorf("Expected payload length 0, got %d", len(rec.Payload))
	}
}

func TestNewMQTTRecordingWithNilPayload(t *testing.T) {
	rec := NewMQTTRecording("topic/nil", nil, 0, false, "client-1", MQTTDirectionPublish)

	if rec.Payload != nil {
		t.Errorf("Expected payload to be nil, got %v", rec.Payload)
	}
}

func TestNewMQTTRecordingQoSLevels(t *testing.T) {
	testCases := []struct {
		name string
		qos  int
	}{
		{"QoS 0 - At most once", 0},
		{"QoS 1 - At least once", 1},
		{"QoS 2 - Exactly once", 2},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			rec := NewMQTTRecording("topic", []byte("msg"), tc.qos, false, "client", MQTTDirectionPublish)
			if rec.QoS != tc.qos {
				t.Errorf("Expected QoS %d, got %d", tc.qos, rec.QoS)
			}
		})
	}
}

func TestGenerateMQTTID(t *testing.T) {
	id := generateMQTTID()

	if id == "" {
		t.Error("Expected ID to be non-empty")
	}
	if !strings.HasPrefix(id, "mqtt-") {
		t.Errorf("Expected ID to have 'mqtt-' prefix, got '%s'", id)
	}
}

func TestGenerateMQTTIDUniqueness(t *testing.T) {
	ids := make(map[string]bool)
	count := 1000

	for i := 0; i < count; i++ {
		id := generateMQTTID()
		if ids[id] {
			t.Errorf("Duplicate ID generated: %s", id)
		}
		ids[id] = true
	}

	if len(ids) != count {
		t.Errorf("Expected %d unique IDs, got %d", count, len(ids))
	}
}

func TestGenerateMQTTIDFormat(t *testing.T) {
	id := generateMQTTID()

	// Format should be: mqtt-xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx
	// Prefix "mqtt-" + 5 segments separated by dashes
	parts := strings.Split(id, "-")
	if len(parts) != 6 {
		t.Errorf("Expected 6 parts (mqtt + 5 segments), got %d parts: %s", len(parts), id)
	}
	if parts[0] != "mqtt" {
		t.Errorf("Expected first part to be 'mqtt', got '%s'", parts[0])
	}
}

func TestMQTTRecordingSetMessageID(t *testing.T) {
	rec := NewMQTTRecording("topic", []byte("data"), 1, false, "client", MQTTDirectionPublish)

	// Initial value should be 0
	if rec.MessageID != 0 {
		t.Errorf("Expected initial MessageID to be 0, got %d", rec.MessageID)
	}

	// Set and verify
	rec.SetMessageID(12345)
	if rec.MessageID != 12345 {
		t.Errorf("Expected MessageID 12345, got %d", rec.MessageID)
	}

	// Test max uint16 value
	rec.SetMessageID(65535)
	if rec.MessageID != 65535 {
		t.Errorf("Expected MessageID 65535, got %d", rec.MessageID)
	}
}

func TestMQTTRecordingSetMessageIDZero(t *testing.T) {
	rec := NewMQTTRecording("topic", []byte("data"), 1, false, "client", MQTTDirectionPublish)
	rec.SetMessageID(100)
	rec.SetMessageID(0)

	if rec.MessageID != 0 {
		t.Errorf("Expected MessageID 0, got %d", rec.MessageID)
	}
}

func TestMQTTRecordingPayloadString(t *testing.T) {
	testCases := []struct {
		name     string
		payload  []byte
		expected string
	}{
		{"JSON payload", []byte(`{"key": "value"}`), `{"key": "value"}`},
		{"Plain text", []byte("Hello, MQTT!"), "Hello, MQTT!"},
		{"Empty payload", []byte{}, ""},
		{"Unicode", []byte("你好世界"), "你好世界"},
		{"With newlines", []byte("line1\nline2\nline3"), "line1\nline2\nline3"},
		{"Binary-like", []byte{0x48, 0x65, 0x6c, 0x6c, 0x6f}, "Hello"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			rec := NewMQTTRecording("topic", tc.payload, 0, false, "client", MQTTDirectionPublish)
			result := rec.PayloadString()
			if result != tc.expected {
				t.Errorf("Expected '%s', got '%s'", tc.expected, result)
			}
		})
	}
}

func TestMQTTRecordingPayloadStringNil(t *testing.T) {
	rec := NewMQTTRecording("topic", nil, 0, false, "client", MQTTDirectionPublish)
	result := rec.PayloadString()

	if result != "" {
		t.Errorf("Expected empty string for nil payload, got '%s'", result)
	}
}

func TestMQTTRecordingIsPublish(t *testing.T) {
	testCases := []struct {
		name      string
		direction MQTTDirection
		expected  bool
	}{
		{"Publish direction", MQTTDirectionPublish, true},
		{"Subscribe direction", MQTTDirectionSubscribe, false},
		{"Empty direction", MQTTDirection(""), false},
		{"Unknown direction", MQTTDirection("unknown"), false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			rec := &MQTTRecording{Direction: tc.direction}
			result := rec.IsPublish()
			if result != tc.expected {
				t.Errorf("Expected IsPublish() to be %v for direction '%s', got %v", tc.expected, tc.direction, result)
			}
		})
	}
}

func TestMQTTRecordingIsSubscribe(t *testing.T) {
	testCases := []struct {
		name      string
		direction MQTTDirection
		expected  bool
	}{
		{"Subscribe direction", MQTTDirectionSubscribe, true},
		{"Publish direction", MQTTDirectionPublish, false},
		{"Empty direction", MQTTDirection(""), false},
		{"Unknown direction", MQTTDirection("unknown"), false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			rec := &MQTTRecording{Direction: tc.direction}
			result := rec.IsSubscribe()
			if result != tc.expected {
				t.Errorf("Expected IsSubscribe() to be %v for direction '%s', got %v", tc.expected, tc.direction, result)
			}
		})
	}
}

func TestMQTTRecordingIsPublishAndIsSubscribeMutuallyExclusive(t *testing.T) {
	publishRec := NewMQTTRecording("topic", []byte("data"), 0, false, "client", MQTTDirectionPublish)
	subscribeRec := NewMQTTRecording("topic", []byte("data"), 0, false, "client", MQTTDirectionSubscribe)

	if publishRec.IsPublish() && publishRec.IsSubscribe() {
		t.Error("Publish recording should not be both publish and subscribe")
	}
	if subscribeRec.IsPublish() && subscribeRec.IsSubscribe() {
		t.Error("Subscribe recording should not be both publish and subscribe")
	}
	if !publishRec.IsPublish() {
		t.Error("Publish recording should return true for IsPublish()")
	}
	if !subscribeRec.IsSubscribe() {
		t.Error("Subscribe recording should return true for IsSubscribe()")
	}
}

func TestMQTTDirectionConstants(t *testing.T) {
	if MQTTDirectionPublish != "publish" {
		t.Errorf("Expected MQTTDirectionPublish to be 'publish', got '%s'", MQTTDirectionPublish)
	}
	if MQTTDirectionSubscribe != "subscribe" {
		t.Errorf("Expected MQTTDirectionSubscribe to be 'subscribe', got '%s'", MQTTDirectionSubscribe)
	}
}

func TestMQTTRecordingFieldsDirectAccess(t *testing.T) {
	rec := &MQTTRecording{
		ID:        "mqtt-test-id",
		Topic:     "devices/sensor/temp",
		Payload:   []byte(`{"temp": 25.5}`),
		QoS:       2,
		Retain:    true,
		ClientID:  "device-001",
		Direction: MQTTDirectionPublish,
		MessageID: 42,
	}

	if rec.ID != "mqtt-test-id" {
		t.Errorf("Expected ID 'mqtt-test-id', got '%s'", rec.ID)
	}
	if rec.Topic != "devices/sensor/temp" {
		t.Errorf("Expected Topic 'devices/sensor/temp', got '%s'", rec.Topic)
	}
	if rec.PayloadString() != `{"temp": 25.5}` {
		t.Errorf("Expected PayloadString '%s', got '%s'", `{"temp": 25.5}`, rec.PayloadString())
	}
	if rec.QoS != 2 {
		t.Errorf("Expected QoS 2, got %d", rec.QoS)
	}
	if !rec.Retain {
		t.Error("Expected Retain to be true")
	}
	if rec.ClientID != "device-001" {
		t.Errorf("Expected ClientID 'device-001', got '%s'", rec.ClientID)
	}
	if rec.MessageID != 42 {
		t.Errorf("Expected MessageID 42, got %d", rec.MessageID)
	}
}

func TestMQTTRecordingWithSpecialTopics(t *testing.T) {
	testCases := []struct {
		name  string
		topic string
	}{
		{"Root topic", "/"},
		{"Multi-level", "home/living-room/temperature"},
		{"With numbers", "sensor/123/data"},
		{"Single level wildcard pattern", "sensors/+/temperature"},
		{"Multi level wildcard pattern", "sensors/#"},
		{"Dollar sign topic", "$SYS/broker/uptime"},
		{"Empty topic", ""},
		{"Unicode topic", "传感器/温度"},
		{"With spaces", "my topic/test"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			rec := NewMQTTRecording(tc.topic, []byte("data"), 0, false, "client", MQTTDirectionPublish)
			if rec.Topic != tc.topic {
				t.Errorf("Expected topic '%s', got '%s'", tc.topic, rec.Topic)
			}
		})
	}
}

func TestMQTTRecordingWithLargePayload(t *testing.T) {
	// Create a 1MB payload
	largePayload := make([]byte, 1024*1024)
	for i := range largePayload {
		largePayload[i] = byte(i % 256)
	}

	rec := NewMQTTRecording("topic/large", largePayload, 1, false, "client", MQTTDirectionPublish)

	if len(rec.Payload) != len(largePayload) {
		t.Errorf("Expected payload length %d, got %d", len(largePayload), len(rec.Payload))
	}
}

func TestMQTTRecordingFilterDefaults(t *testing.T) {
	filter := MQTTRecordingFilter{}

	if filter.TopicPattern != "" {
		t.Errorf("Expected empty TopicPattern, got '%s'", filter.TopicPattern)
	}
	if filter.ClientID != "" {
		t.Errorf("Expected empty ClientID, got '%s'", filter.ClientID)
	}
	if filter.Direction != "" {
		t.Errorf("Expected empty Direction, got '%s'", filter.Direction)
	}
	if filter.Limit != 0 {
		t.Errorf("Expected Limit 0, got %d", filter.Limit)
	}
	if filter.Offset != 0 {
		t.Errorf("Expected Offset 0, got %d", filter.Offset)
	}
}

func TestMQTTRecordingFilterWithValues(t *testing.T) {
	filter := MQTTRecordingFilter{
		TopicPattern: "sensor/+/temperature",
		ClientID:     "device-001",
		Direction:    MQTTDirectionPublish,
		Limit:        100,
		Offset:       50,
	}

	if filter.TopicPattern != "sensor/+/temperature" {
		t.Errorf("Expected TopicPattern 'sensor/+/temperature', got '%s'", filter.TopicPattern)
	}
	if filter.ClientID != "device-001" {
		t.Errorf("Expected ClientID 'device-001', got '%s'", filter.ClientID)
	}
	if filter.Direction != MQTTDirectionPublish {
		t.Errorf("Expected Direction '%s', got '%s'", MQTTDirectionPublish, filter.Direction)
	}
	if filter.Limit != 100 {
		t.Errorf("Expected Limit 100, got %d", filter.Limit)
	}
	if filter.Offset != 50 {
		t.Errorf("Expected Offset 50, got %d", filter.Offset)
	}
}
