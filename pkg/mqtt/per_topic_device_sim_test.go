package mqtt_test

import (
	"context"
	"testing"
	"time"

	"github.com/getmockd/mockd/pkg/mqtt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPerTopicDeviceSimulation(t *testing.T) {
	// Start a test broker
	cfg := &mqtt.MQTTConfig{
		ID:      "test-per-topic",
		Port:    19001,
		Enabled: true,
	}

	broker, err := mqtt.NewBroker(cfg)
	require.NoError(t, err)
	require.NotNil(t, broker)

	err = broker.Start(context.Background())
	require.NoError(t, err)
	defer broker.Stop(context.Background(), 5*time.Second)

	// Create topic config with device simulation
	topics := []mqtt.TopicConfig{
		{
			Topic:  "sensors/{device_id}/temperature",
			QoS:    0,
			Retain: false,
			Messages: []mqtt.MessageConfig{
				{
					Payload:  `{"temp": {{ random.float(20, 30, 1) }}}`,
					Repeat:   true,
					Interval: "100ms",
				},
			},
			DeviceSimulation: &mqtt.DeviceSimulationSettings{
				Enabled:         true,
				DeviceCount:     3,
				DeviceIDPattern: "sensor-{n}",
			},
		},
	}

	// Create and start simulator
	sim := mqtt.NewSimulator(broker, topics, nil)
	require.NotNil(t, sim)

	sim.Start()
	defer sim.Stop()

	// Wait for some messages to be published
	time.Sleep(300 * time.Millisecond)

	// Check per-topic device simulation status
	statuses := sim.GetPerTopicDeviceSimulationStatus()
	require.NotNil(t, statuses)

	status, ok := statuses["sensors/{device_id}/temperature"]
	require.True(t, ok, "Expected status for topic pattern")

	assert.True(t, status.Running)
	assert.Equal(t, 3, status.DeviceCount)
	assert.Equal(t, 3, status.ActiveDevices)
	assert.Greater(t, status.TotalMessages, int64(0))
	assert.Len(t, status.Devices, 3)

	// Verify device IDs
	deviceIDs := make([]string, len(status.Devices))
	for i, d := range status.Devices {
		deviceIDs[i] = d.DeviceID
	}
	assert.Contains(t, deviceIDs, "sensor-1")
	assert.Contains(t, deviceIDs, "sensor-2")
	assert.Contains(t, deviceIDs, "sensor-3")
}

func TestValidateDeviceSimulationSettings(t *testing.T) {
	tests := []struct {
		name         string
		settings     *mqtt.DeviceSimulationSettings
		topicPattern string
		expectError  bool
	}{
		{
			name:         "nil settings",
			settings:     nil,
			topicPattern: "test/{device_id}/data",
			expectError:  true,
		},
		{
			name: "disabled settings",
			settings: &mqtt.DeviceSimulationSettings{
				Enabled: false,
			},
			topicPattern: "test/{device_id}/data",
			expectError:  false,
		},
		{
			name: "valid settings with {n}",
			settings: &mqtt.DeviceSimulationSettings{
				Enabled:         true,
				DeviceCount:     10,
				DeviceIDPattern: "device-{n}",
			},
			topicPattern: "test/{device_id}/data",
			expectError:  false,
		},
		{
			name: "valid settings with {id}",
			settings: &mqtt.DeviceSimulationSettings{
				Enabled:         true,
				DeviceCount:     10,
				DeviceIDPattern: "sensor-{id}",
			},
			topicPattern: "test/{device_id}/data",
			expectError:  false,
		},
		{
			name: "device count too low",
			settings: &mqtt.DeviceSimulationSettings{
				Enabled:         true,
				DeviceCount:     0,
				DeviceIDPattern: "device-{n}",
			},
			topicPattern: "test/{device_id}/data",
			expectError:  true,
		},
		{
			name: "device count too high",
			settings: &mqtt.DeviceSimulationSettings{
				Enabled:         true,
				DeviceCount:     1001,
				DeviceIDPattern: "device-{n}",
			},
			topicPattern: "test/{device_id}/data",
			expectError:  true,
		},
		{
			name: "missing placeholder in device ID pattern",
			settings: &mqtt.DeviceSimulationSettings{
				Enabled:         true,
				DeviceCount:     10,
				DeviceIDPattern: "device-fixed",
			},
			topicPattern: "test/{device_id}/data",
			expectError:  true,
		},
		{
			name: "missing {device_id} in topic pattern",
			settings: &mqtt.DeviceSimulationSettings{
				Enabled:         true,
				DeviceCount:     10,
				DeviceIDPattern: "device-{n}",
			},
			topicPattern: "test/data",
			expectError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := mqtt.ValidateDeviceSimulationSettings(tt.settings, tt.topicPattern)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
