package mqtt

import (
	"context"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestValidateMultiDeviceConfig(t *testing.T) {
	tests := []struct {
		name    string
		config  *MultiDeviceSimulationConfig
		wantErr bool
		errMsg  string
	}{
		{
			name:    "nil config",
			config:  nil,
			wantErr: true,
			errMsg:  "config cannot be nil",
		},
		{
			name: "device count too low",
			config: &MultiDeviceSimulationConfig{
				DeviceCount:     0,
				DeviceIDPattern: "device-{id}",
				TopicPattern:    "sensors/{device_id}/temp",
				IntervalMs:      1000,
			},
			wantErr: true,
			errMsg:  "deviceCount must be between 1 and 1000",
		},
		{
			name: "device count too high",
			config: &MultiDeviceSimulationConfig{
				DeviceCount:     1001,
				DeviceIDPattern: "device-{id}",
				TopicPattern:    "sensors/{device_id}/temp",
				IntervalMs:      1000,
			},
			wantErr: true,
			errMsg:  "deviceCount must be between 1 and 1000",
		},
		{
			name: "missing device ID pattern",
			config: &MultiDeviceSimulationConfig{
				DeviceCount:  5,
				TopicPattern: "sensors/{device_id}/temp",
				IntervalMs:   1000,
			},
			wantErr: true,
			errMsg:  "deviceIdPattern is required",
		},
		{
			name: "device ID pattern missing placeholder",
			config: &MultiDeviceSimulationConfig{
				DeviceCount:     5,
				DeviceIDPattern: "device-1",
				TopicPattern:    "sensors/{device_id}/temp",
				IntervalMs:      1000,
			},
			wantErr: true,
			errMsg:  "deviceIdPattern must contain {n}, {id}, or {index} placeholder",
		},
		{
			name: "missing topic pattern",
			config: &MultiDeviceSimulationConfig{
				DeviceCount:     5,
				DeviceIDPattern: "device-{id}",
				IntervalMs:      1000,
			},
			wantErr: true,
			errMsg:  "topicPattern is required",
		},
		{
			name: "topic pattern missing placeholder",
			config: &MultiDeviceSimulationConfig{
				DeviceCount:     5,
				DeviceIDPattern: "device-{id}",
				TopicPattern:    "sensors/temp",
				IntervalMs:      1000,
			},
			wantErr: true,
			errMsg:  "topicPattern must contain {device_id} placeholder",
		},
		{
			name: "interval too low",
			config: &MultiDeviceSimulationConfig{
				DeviceCount:     5,
				DeviceIDPattern: "device-{id}",
				TopicPattern:    "sensors/{device_id}/temp",
				IntervalMs:      50,
			},
			wantErr: true,
			errMsg:  "intervalMs must be at least 100ms",
		},
		{
			name: "invalid QoS",
			config: &MultiDeviceSimulationConfig{
				DeviceCount:     5,
				DeviceIDPattern: "device-{id}",
				TopicPattern:    "sensors/{device_id}/temp",
				IntervalMs:      1000,
				QoS:             3,
			},
			wantErr: true,
			errMsg:  "qos must be 0, 1, or 2",
		},
		{
			name: "valid config",
			config: &MultiDeviceSimulationConfig{
				DeviceCount:     5,
				DeviceIDPattern: "device-{id}",
				TopicPattern:    "sensors/{device_id}/temp",
				IntervalMs:      1000,
				QoS:             1,
			},
			wantErr: false,
		},
		{
			name: "valid config with payload template",
			config: &MultiDeviceSimulationConfig{
				DeviceCount:     10,
				DeviceIDPattern: "sensor-{id}",
				TopicPattern:    "devices/{device_id}/data",
				PayloadTemplate: `{"deviceId":"{{ device_id }}","temp":{{ random.float(20,30,1) }}}`,
				IntervalMs:      500,
				QoS:             0,
				Retain:          true,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateMultiDeviceConfig(tt.config)
			if tt.wantErr {
				if err == nil {
					t.Errorf("ValidateMultiDeviceConfig() expected error, got nil")
				} else if tt.errMsg != "" && !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("ValidateMultiDeviceConfig() error = %v, want error containing %q", err, tt.errMsg)
				}
			} else {
				if err != nil {
					t.Errorf("ValidateMultiDeviceConfig() unexpected error = %v", err)
				}
			}
		})
	}
}

func TestNewMultiDeviceSimulator(t *testing.T) {
	config := &MQTTConfig{
		Port:    18850,
		Enabled: true,
	}

	broker, err := NewBroker(config)
	if err != nil {
		t.Fatalf("NewBroker() error = %v", err)
	}

	tests := []struct {
		name    string
		config  *MultiDeviceSimulationConfig
		wantErr bool
	}{
		{
			name:    "nil config",
			config:  nil,
			wantErr: true,
		},
		{
			name: "valid config",
			config: &MultiDeviceSimulationConfig{
				DeviceCount:     5,
				DeviceIDPattern: "device-{id}",
				TopicPattern:    "sensors/{device_id}/temp",
				IntervalMs:      1000,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sim, err := NewMultiDeviceSimulator(broker, tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewMultiDeviceSimulator() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && sim == nil {
				t.Error("NewMultiDeviceSimulator() returned nil simulator without error")
			}
		})
	}
}

func TestMultiDeviceSimulator_StartStop(t *testing.T) {
	config := &MQTTConfig{
		Port:    18851,
		Enabled: true,
	}

	broker, err := NewBroker(config)
	if err != nil {
		t.Fatalf("NewBroker() error = %v", err)
	}

	if err := broker.Start(context.Background()); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	defer broker.Stop(context.Background(), 5*time.Second)

	// Wait for broker to be ready
	time.Sleep(100 * time.Millisecond)

	simConfig := &MultiDeviceSimulationConfig{
		DeviceCount:     3,
		DeviceIDPattern: "device-{id}",
		TopicPattern:    "test/{device_id}/data",
		IntervalMs:      100,
	}

	sim, err := NewMultiDeviceSimulator(broker, simConfig)
	if err != nil {
		t.Fatalf("NewMultiDeviceSimulator() error = %v", err)
	}

	// Test Start
	if err := sim.Start(); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	if !sim.IsRunning() {
		t.Error("IsRunning() = false, want true")
	}

	// Test double start
	if err := sim.Start(); err == nil {
		t.Error("Start() on running simulator should return error")
	}

	// Wait a bit for some messages
	time.Sleep(250 * time.Millisecond)

	// Check status
	status := sim.GetStatus()
	if !status.Running {
		t.Error("GetStatus().Running = false, want true")
	}
	if status.DeviceCount != 3 {
		t.Errorf("GetStatus().DeviceCount = %d, want 3", status.DeviceCount)
	}
	if status.TotalMessages < 3 {
		t.Errorf("GetStatus().TotalMessages = %d, want at least 3", status.TotalMessages)
	}

	// Test Stop
	sim.Stop()

	if sim.IsRunning() {
		t.Error("IsRunning() = true after Stop, want false")
	}
}

func TestMultiDeviceSimulator_DeviceIDs(t *testing.T) {
	config := &MQTTConfig{
		Port:    18852,
		Enabled: true,
	}

	broker, err := NewBroker(config)
	if err != nil {
		t.Fatalf("NewBroker() error = %v", err)
	}

	if err := broker.Start(context.Background()); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	defer broker.Stop(context.Background(), 5*time.Second)

	time.Sleep(100 * time.Millisecond)

	simConfig := &MultiDeviceSimulationConfig{
		DeviceCount:     5,
		DeviceIDPattern: "sensor-{id}",
		TopicPattern:    "devices/{device_id}/temperature",
		IntervalMs:      100,
	}

	sim, err := NewMultiDeviceSimulator(broker, simConfig)
	if err != nil {
		t.Fatalf("NewMultiDeviceSimulator() error = %v", err)
	}

	if err := sim.Start(); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	defer sim.Stop()

	time.Sleep(150 * time.Millisecond)

	// Verify device IDs and topics
	status := sim.GetStatus()
	expectedDevices := []struct {
		id    string
		topic string
	}{
		{"sensor-1", "devices/sensor-1/temperature"},
		{"sensor-2", "devices/sensor-2/temperature"},
		{"sensor-3", "devices/sensor-3/temperature"},
		{"sensor-4", "devices/sensor-4/temperature"},
		{"sensor-5", "devices/sensor-5/temperature"},
	}

	if len(status.Devices) != 5 {
		t.Fatalf("len(status.Devices) = %d, want 5", len(status.Devices))
	}

	for i, expected := range expectedDevices {
		if status.Devices[i].DeviceID != expected.id {
			t.Errorf("Devices[%d].DeviceID = %q, want %q", i, status.Devices[i].DeviceID, expected.id)
		}
		if status.Devices[i].Topic != expected.topic {
			t.Errorf("Devices[%d].Topic = %q, want %q", i, status.Devices[i].Topic, expected.topic)
		}
	}
}

func TestMultiDeviceSimulator_GetDeviceStatus(t *testing.T) {
	config := &MQTTConfig{
		Port:    18853,
		Enabled: true,
	}

	broker, err := NewBroker(config)
	if err != nil {
		t.Fatalf("NewBroker() error = %v", err)
	}

	if err := broker.Start(context.Background()); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	defer broker.Stop(context.Background(), 5*time.Second)

	time.Sleep(100 * time.Millisecond)

	simConfig := &MultiDeviceSimulationConfig{
		DeviceCount:     3,
		DeviceIDPattern: "device-{id}",
		TopicPattern:    "test/{device_id}/data",
		IntervalMs:      100,
	}

	sim, err := NewMultiDeviceSimulator(broker, simConfig)
	if err != nil {
		t.Fatalf("NewMultiDeviceSimulator() error = %v", err)
	}

	if err := sim.Start(); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	defer sim.Stop()

	time.Sleep(150 * time.Millisecond)

	// Get specific device status
	deviceStatus := sim.GetDeviceStatus("device-2")
	if deviceStatus == nil {
		t.Fatal("GetDeviceStatus(device-2) returned nil")
	}

	if deviceStatus.DeviceID != "device-2" {
		t.Errorf("DeviceID = %q, want device-2", deviceStatus.DeviceID)
	}
	if !deviceStatus.Connected {
		t.Error("Connected = false, want true")
	}
	if deviceStatus.MessageCount < 1 {
		t.Errorf("MessageCount = %d, want at least 1", deviceStatus.MessageCount)
	}

	// Try non-existent device
	noDevice := sim.GetDeviceStatus("device-999")
	if noDevice != nil {
		t.Error("GetDeviceStatus(device-999) should return nil")
	}
}

func TestMultiDeviceSimulator_MessagePublishing(t *testing.T) {
	config := &MQTTConfig{
		Port:    18854,
		Enabled: true,
	}

	broker, err := NewBroker(config)
	if err != nil {
		t.Fatalf("NewBroker() error = %v", err)
	}

	if err := broker.Start(context.Background()); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	defer broker.Stop(context.Background(), 5*time.Second)

	time.Sleep(100 * time.Millisecond)

	simConfig := &MultiDeviceSimulationConfig{
		DeviceCount:     3,
		DeviceIDPattern: "device-{id}",
		TopicPattern:    "sensors/{device_id}/temp",
		IntervalMs:      100,
	}

	sim, err := NewMultiDeviceSimulator(broker, simConfig)
	if err != nil {
		t.Fatalf("NewMultiDeviceSimulator() error = %v", err)
	}

	// Track messages received per topic
	var mu sync.Mutex
	receivedTopics := make(map[string]int)

	broker.Subscribe("sensors/+/temp", func(topic string, payload []byte) {
		mu.Lock()
		receivedTopics[topic]++
		mu.Unlock()
	})

	if err := sim.Start(); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	// Wait for messages
	time.Sleep(350 * time.Millisecond)

	sim.Stop()

	mu.Lock()
	// Should have received messages from all 3 devices
	expectedTopics := []string{
		"sensors/device-1/temp",
		"sensors/device-2/temp",
		"sensors/device-3/temp",
	}
	for _, topic := range expectedTopics {
		if count, ok := receivedTopics[topic]; !ok || count < 3 {
			t.Errorf("Topic %s received %d messages, want at least 3", topic, count)
		}
	}
	mu.Unlock()
}

func TestMultiDeviceSimulator_PayloadTemplate(t *testing.T) {
	config := &MQTTConfig{
		Port:    18855,
		Enabled: true,
	}

	broker, err := NewBroker(config)
	if err != nil {
		t.Fatalf("NewBroker() error = %v", err)
	}

	if err := broker.Start(context.Background()); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	defer broker.Stop(context.Background(), 5*time.Second)

	time.Sleep(100 * time.Millisecond)

	simConfig := &MultiDeviceSimulationConfig{
		DeviceCount:     2,
		DeviceIDPattern: "device-{id}",
		TopicPattern:    "test/{device_id}/data",
		PayloadTemplate: `{"deviceId":"{{ device_id }}","value":{{ random.int(1,100) }}}`,
		IntervalMs:      100,
	}

	sim, err := NewMultiDeviceSimulator(broker, simConfig)
	if err != nil {
		t.Fatalf("NewMultiDeviceSimulator() error = %v", err)
	}

	var mu sync.Mutex
	var receivedPayloads []string

	broker.Subscribe("test/+/data", func(topic string, payload []byte) {
		mu.Lock()
		receivedPayloads = append(receivedPayloads, string(payload))
		mu.Unlock()
	})

	if err := sim.Start(); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	time.Sleep(150 * time.Millisecond)

	sim.Stop()

	mu.Lock()
	if len(receivedPayloads) < 2 {
		t.Fatalf("Expected at least 2 payloads, got %d", len(receivedPayloads))
	}

	// Check that payloads contain device IDs
	hasDevice1 := false
	hasDevice2 := false
	for _, payload := range receivedPayloads {
		if strings.Contains(payload, "device-1") {
			hasDevice1 = true
		}
		if strings.Contains(payload, "device-2") {
			hasDevice2 = true
		}
	}
	mu.Unlock()

	if !hasDevice1 {
		t.Error("No payload contained device-1")
	}
	if !hasDevice2 {
		t.Error("No payload contained device-2")
	}
}

func TestMultiDeviceSimulator_ConcurrentPublishing(t *testing.T) {
	config := &MQTTConfig{
		Port:    18856,
		Enabled: true,
	}

	broker, err := NewBroker(config)
	if err != nil {
		t.Fatalf("NewBroker() error = %v", err)
	}

	if err := broker.Start(context.Background()); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	defer broker.Stop(context.Background(), 5*time.Second)

	time.Sleep(100 * time.Millisecond)

	// Use more devices to test concurrency
	simConfig := &MultiDeviceSimulationConfig{
		DeviceCount:     10,
		DeviceIDPattern: "device-{id}",
		TopicPattern:    "concurrent/{device_id}/test",
		IntervalMs:      100,
	}

	sim, err := NewMultiDeviceSimulator(broker, simConfig)
	if err != nil {
		t.Fatalf("NewMultiDeviceSimulator() error = %v", err)
	}

	var totalMessages int64

	broker.Subscribe("concurrent/#", func(topic string, payload []byte) {
		atomic.AddInt64(&totalMessages, 1)
	})

	if err := sim.Start(); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	// Wait for messages
	time.Sleep(350 * time.Millisecond)

	sim.Stop()

	// Should have received at least 30 messages (10 devices * 3 intervals)
	count := atomic.LoadInt64(&totalMessages)
	if count < 30 {
		t.Errorf("Received %d messages, want at least 30", count)
	}

	// Verify status shows all devices
	status := sim.GetStatus()
	if len(status.Devices) != 10 {
		t.Errorf("len(status.Devices) = %d, want 10", len(status.Devices))
	}
}

func TestMultiDeviceSimulator_MaxDevices(t *testing.T) {
	// Test with the maximum allowed devices (1000)
	// This is a lightweight test that just verifies creation works

	config := &MQTTConfig{
		Port:    18857,
		Enabled: true,
	}

	broker, err := NewBroker(config)
	if err != nil {
		t.Fatalf("NewBroker() error = %v", err)
	}

	simConfig := &MultiDeviceSimulationConfig{
		DeviceCount:     1000,
		DeviceIDPattern: "device-{id}",
		TopicPattern:    "mass/{device_id}/test",
		IntervalMs:      10000, // Long interval to avoid performance issues
	}

	sim, err := NewMultiDeviceSimulator(broker, simConfig)
	if err != nil {
		t.Fatalf("NewMultiDeviceSimulator() error = %v", err)
	}

	if sim == nil {
		t.Error("NewMultiDeviceSimulator() returned nil for 1000 devices")
	}
}
