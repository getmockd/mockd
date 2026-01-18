package mqtt

import (
	"context"
	"sync"
	"testing"
	"time"
)

func TestNewBroker(t *testing.T) {
	tests := []struct {
		name    string
		config  *MQTTConfig
		wantErr bool
	}{
		{
			name:    "nil config",
			config:  nil,
			wantErr: true,
		},
		{
			name: "valid config with default port",
			config: &MQTTConfig{
				Enabled: true,
			},
			wantErr: false,
		},
		{
			name: "valid config with custom port",
			config: &MQTTConfig{
				Port:    1884,
				Enabled: true,
			},
			wantErr: false,
		},
		{
			name: "config with auth",
			config: &MQTTConfig{
				Port:    1885,
				Enabled: true,
				Auth: &MQTTAuthConfig{
					Enabled: true,
					Users: []MQTTUser{
						{
							Username: "testuser",
							Password: "testpass",
						},
					},
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			broker, err := NewBroker(tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewBroker() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && broker == nil {
				t.Error("NewBroker() returned nil broker without error")
			}
		})
	}
}

func TestBroker_StartStop(t *testing.T) {
	config := &MQTTConfig{
		Port:    18831,
		Enabled: true,
	}

	broker, err := NewBroker(config)
	if err != nil {
		t.Fatalf("NewBroker() error = %v", err)
	}

	// Test Start
	if err := broker.Start(context.Background()); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	if !broker.IsRunning() {
		t.Error("IsRunning() = false, want true")
	}

	// Test double start
	if err := broker.Start(context.Background()); err == nil {
		t.Error("Start() on running broker should return error")
	}

	// Test Stop
	if err := broker.Stop(context.Background(), 5*time.Second); err != nil {
		t.Errorf("Stop() error = %v", err)
	}

	if broker.IsRunning() {
		t.Error("IsRunning() = true after Stop, want false")
	}

	// Test double stop (should be safe)
	if err := broker.Stop(context.Background(), 5*time.Second); err != nil {
		t.Errorf("Stop() on stopped broker error = %v", err)
	}
}

func TestBroker_InternalSubscription(t *testing.T) {
	config := &MQTTConfig{
		Port:    18832,
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

	// Wait for server to be ready
	time.Sleep(100 * time.Millisecond)

	var received []byte
	var mu sync.Mutex
	done := make(chan struct{})

	broker.Subscribe("test/topic", func(topic string, payload []byte) {
		mu.Lock()
		received = payload
		mu.Unlock()
		close(done)
	})

	// Publish a message
	testPayload := []byte(`{"test": "data"}`)
	if err := broker.Publish("test/topic", testPayload, 0, false); err != nil {
		t.Errorf("Publish() error = %v", err)
	}

	// Wait for message or timeout
	select {
	case <-done:
		mu.Lock()
		if string(received) != string(testPayload) {
			t.Errorf("received = %s, want %s", string(received), string(testPayload))
		}
		mu.Unlock()
	case <-time.After(2 * time.Second):
		t.Error("Timed out waiting for message")
	}

	// Test unsubscribe
	broker.Unsubscribe("test/topic")
}

func TestBroker_PublishNotRunning(t *testing.T) {
	config := &MQTTConfig{
		Port:    18833,
		Enabled: true,
	}

	broker, err := NewBroker(config)
	if err != nil {
		t.Fatalf("NewBroker() error = %v", err)
	}

	// Publish without starting should fail
	err = broker.Publish("test/topic", []byte("test"), 0, false)
	if err == nil {
		t.Error("Publish() on non-running broker should return error")
	}
}

func TestBroker_GetStats(t *testing.T) {
	config := &MQTTConfig{
		Port:    18834,
		Enabled: true,
		Auth: &MQTTAuthConfig{
			Enabled: true,
		},
		TLS: &MQTTTLSConfig{
			Enabled: false,
		},
		Topics: []TopicConfig{
			{Topic: "test/topic"},
		},
	}

	broker, err := NewBroker(config)
	if err != nil {
		t.Fatalf("NewBroker() error = %v", err)
	}

	stats := broker.GetStats()

	if stats.Running {
		t.Error("Running should be false before Start()")
	}
	if stats.Port != 18834 {
		t.Errorf("Port = %d, want 18834", stats.Port)
	}
	if !stats.AuthEnabled {
		t.Error("AuthEnabled should be true")
	}
	if stats.TLSEnabled {
		t.Error("TLSEnabled should be false")
	}
	if stats.TopicCount != 1 {
		t.Errorf("TopicCount = %d, want 1", stats.TopicCount)
	}
}

func TestMatchTopic(t *testing.T) {
	tests := []struct {
		pattern string
		topic   string
		want    bool
	}{
		// Exact matches
		{"sensors/temp", "sensors/temp", true},
		{"sensors/temp", "sensors/humidity", false},

		// Single-level wildcard (+)
		{"sensors/+/temp", "sensors/room1/temp", true},
		{"sensors/+/temp", "sensors/room1/humidity", false},
		{"+/temp", "sensors/temp", true},
		{"sensors/+", "sensors/temp", true},

		// Multi-level wildcard (#)
		{"sensors/#", "sensors/temp", true},
		{"sensors/#", "sensors/room1/temp", true},
		{"sensors/#", "sensors/room1/room2/temp", true},
		{"#", "anything/goes/here", true},

		// Mixed wildcards
		{"sensors/+/#", "sensors/room1/temp", true},
		{"sensors/+/#", "sensors/room1/temp/value", true},

		// Edge cases
		{"sensors/temp", "sensors/temp/value", false},
		{"sensors/temp/value", "sensors/temp", false},
	}

	for _, tt := range tests {
		t.Run(tt.pattern+"_"+tt.topic, func(t *testing.T) {
			if got := matchTopic(tt.pattern, tt.topic); got != tt.want {
				t.Errorf("matchTopic(%q, %q) = %v, want %v", tt.pattern, tt.topic, got, tt.want)
			}
		})
	}
}

func TestCheckAccess(t *testing.T) {
	tests := []struct {
		access string
		write  bool
		want   bool
	}{
		{"readwrite", true, true},
		{"readwrite", false, true},
		{"read", true, false},
		{"read", false, true},
		{"write", true, true},
		{"write", false, false},
		{"invalid", true, false},
		{"invalid", false, false},
	}

	for _, tt := range tests {
		t.Run(tt.access, func(t *testing.T) {
			if got := checkAccess(tt.access, tt.write); got != tt.want {
				t.Errorf("checkAccess(%q, %v) = %v, want %v", tt.access, tt.write, got, tt.want)
			}
		})
	}
}

func TestSimulator(t *testing.T) {
	config := &MQTTConfig{
		Port:    18835,
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

	// Wait for server to be ready
	time.Sleep(100 * time.Millisecond)

	topics := []TopicConfig{
		{
			Topic: "test/simulator",
			Messages: []MessageConfig{
				{
					Payload:  `{"test": "value"}`,
					Interval: "100ms",
					Repeat:   true,
				},
			},
		},
	}

	simulator := NewSimulator(broker, topics, nil)

	var receivedCount int
	var mu sync.Mutex

	broker.Subscribe("test/simulator", func(topic string, payload []byte) {
		mu.Lock()
		receivedCount++
		mu.Unlock()
	})

	simulator.Start()

	// Wait for a few messages
	time.Sleep(350 * time.Millisecond)

	simulator.Stop()

	mu.Lock()
	count := receivedCount
	mu.Unlock()

	// Should have received at least 3 messages (initial + 2 repeats)
	if count < 3 {
		t.Errorf("received %d messages, want at least 3", count)
	}
}

func TestSimulator_WithDelay(t *testing.T) {
	config := &MQTTConfig{
		Port:    18836,
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

	// Wait for server to be ready
	time.Sleep(100 * time.Millisecond)

	topics := []TopicConfig{
		{
			Topic: "test/delayed",
			Messages: []MessageConfig{
				{
					Payload: `{"delayed": true}`,
					Delay:   "200ms",
					Repeat:  false,
				},
			},
		},
	}

	simulator := NewSimulator(broker, topics, nil)

	var received bool
	var mu sync.Mutex
	done := make(chan struct{})

	broker.Subscribe("test/delayed", func(topic string, payload []byte) {
		mu.Lock()
		received = true
		mu.Unlock()
		close(done)
	})

	start := time.Now()
	simulator.Start()
	defer simulator.Stop()

	select {
	case <-done:
		elapsed := time.Since(start)
		if elapsed < 200*time.Millisecond {
			t.Errorf("message received too early: %v", elapsed)
		}
		mu.Lock()
		if !received {
			t.Error("message not marked as received")
		}
		mu.Unlock()
	case <-time.After(1 * time.Second):
		t.Error("timed out waiting for delayed message")
	}
}

func TestAuthHook(t *testing.T) {
	authConfig := &MQTTAuthConfig{
		Enabled: true,
		Users: []MQTTUser{
			{
				Username: "validuser",
				Password: "validpass",
				ACL: []ACLRule{
					{Topic: "allowed/#", Access: "readwrite"},
					{Topic: "readonly/#", Access: "read"},
					{Topic: "writeonly/#", Access: "write"},
				},
			},
		},
	}

	hook := NewAuthHook(authConfig)

	// Test ID
	if hook.ID() != "auth-hook" {
		t.Errorf("ID() = %s, want auth-hook", hook.ID())
	}

	// Test Provides - use actual constants from mochi-mqtt
	// OnConnectAuthenticate = 4, OnACLCheck = 5
	if !hook.Provides(4) { // OnConnectAuthenticate
		t.Error("Provides(OnConnectAuthenticate) should return true")
	}
	if !hook.Provides(5) { // OnACLCheck
		t.Error("Provides(OnACLCheck) should return true")
	}
}

func TestMessageHook(t *testing.T) {
	config := &MQTTConfig{
		Port:    18837,
		Enabled: true,
	}

	broker, err := NewBroker(config)
	if err != nil {
		t.Fatalf("NewBroker() error = %v", err)
	}

	hook := NewMessageHook(broker)

	// Test ID
	if hook.ID() != "message-hook" {
		t.Errorf("ID() = %s, want message-hook", hook.ID())
	}
}

func TestDeviceSimulator(t *testing.T) {
	config := &MQTTConfig{
		Port:    18838,
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

	// Wait for server to be ready
	time.Sleep(100 * time.Millisecond)

	device := NewDeviceSimulator(broker, "device1", "sensor", "devices", 100*time.Millisecond)
	device.AddGenerator("temp", NewStaticGenerator(`{"temp": 22.5}`))
	device.AddGenerator("humidity", NewStaticGenerator(`{"humidity": 65}`))

	var tempCount, humidityCount int
	var mu sync.Mutex

	broker.Subscribe("devices/device1/temp", func(topic string, payload []byte) {
		mu.Lock()
		tempCount++
		mu.Unlock()
	})

	broker.Subscribe("devices/device1/humidity", func(topic string, payload []byte) {
		mu.Lock()
		humidityCount++
		mu.Unlock()
	})

	device.Start()
	time.Sleep(350 * time.Millisecond)
	device.Stop()

	mu.Lock()
	if tempCount < 3 {
		t.Errorf("tempCount = %d, want at least 3", tempCount)
	}
	if humidityCount < 3 {
		t.Errorf("humidityCount = %d, want at least 3", humidityCount)
	}
	mu.Unlock()
}

func TestStaticGenerator(t *testing.T) {
	gen := NewStaticGenerator(`{"test": "value"}`)
	result := gen.Generate()

	if string(result) != `{"test": "value"}` {
		t.Errorf("Generate() = %s, want {\"test\": \"value\"}", string(result))
	}
}

func TestTemplateGenerator(t *testing.T) {
	gen := NewTemplateGenerator(`{"value": 42}`, nil)
	result := gen.Generate()

	if string(result) != `{"value": 42}` {
		t.Errorf("Generate() = %s, want {\"value\": 42}", string(result))
	}
}
