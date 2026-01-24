# MQTT Mocking

MQTT mocking enables testing of IoT devices, sensor networks, and real-time messaging systems without connecting to actual MQTT brokers. mockd provides a full-featured MQTT broker with configurable topics, authentication, QoS levels, and device simulation.

## Overview

Use MQTT mocks when you need to:

- Test IoT applications and device communication
- Simulate sensor networks and telemetry data
- Develop smart home or industrial automation systems
- Create reproducible test scenarios for MQTT clients
- Mock message queues and pub/sub patterns
- Test real-time notification systems

## Quick Start

Create a minimal MQTT broker mock:

```yaml
version: "1.0"

mocks:
  - id: mqtt-broker
    name: Simple MQTT Broker
    type: mqtt
    enabled: true
    mqtt:
      port: 1883
      topics:
        - topic: sensors/temperature
          qos: 1
          messages:
            - payload: '{"value": 22.5, "unit": "celsius"}'
              repeat: true
              interval: "5s"
```

Start the server and test:

```bash
# Start mockd
mockd serve --config mockd.yaml

# Subscribe to messages using mockd CLI
mockd mqtt subscribe sensors/temperature

# Or use mosquitto client
mosquitto_sub -h localhost -p 1883 -t "sensors/#"
```

## Configuration

### Full MQTT Spec

```yaml
version: "1.0"

mocks:
  - id: mqtt-full-example
    name: IoT MQTT Broker
    type: mqtt
    enabled: true
    mqtt:
      # Required: broker port
      port: 1883

      # TLS configuration (optional)
      tls:
        enabled: true
        certFile: ./certs/server.crt
        keyFile: ./certs/server.key

      # Authentication (optional)
      auth:
        enabled: true
        users:
          - username: device
            password: secret123
            acl:
              - topic: "sensors/#"
                access: write
              - topic: "commands/#"
                access: read
          - username: admin
            password: admin123
            acl:
              - topic: "#"
                access: readwrite

      # Topic configurations
      topics:
        - topic: sensors/temperature
          qos: 1
          retain: true
          messages:
            - payload: '{"value": 22.5}'
              interval: "5s"
              repeat: true

        - topic: commands/+/execute
          qos: 2
          onPublish:
            response:
              payload: '{"status": "executed"}'
            forward: responses/device

        - topic: devices/+/telemetry
          qos: 0
          deviceSimulation:
            enabled: true
            deviceCount: 10
            deviceIdPattern: "device_{index}"
```

### Configuration Reference

| Field | Type | Description |
|-------|------|-------------|
| `port` | integer | MQTT broker port (default: 1883) |
| `tls` | object | TLS/SSL configuration |
| `auth` | object | Authentication settings |
| `topics` | array | Topic configurations |

## Topics and Messages

Topics define how the broker handles message publishing and subscription.

### Basic Topic Configuration

```yaml
topics:
  - topic: sensors/temperature
    qos: 1
    retain: true
    messages:
      - payload: |
          {
            "deviceId": "temp_001",
            "value": 22.5,
            "unit": "celsius",
            "timestamp": "{{ now }}"
          }
        interval: "5s"
        repeat: true
```

### Topic Fields

| Field | Type | Description |
|-------|------|-------------|
| `topic` | string | Topic pattern (supports wildcards) |
| `qos` | integer | Quality of Service level (0, 1, or 2) |
| `retain` | boolean | Retain last message for new subscribers |
| `messages` | array | Predefined messages to publish |
| `onPublish` | object | Handler for received messages |
| `deviceSimulation` | object | Simulate multiple devices |

### Topic Wildcards

MQTT supports two wildcard characters for topic subscriptions:

```yaml
# Single-level wildcard (+)
# Matches exactly one topic level
topics:
  - topic: sensors/+/temperature  # matches sensors/room1/temperature
                                   # matches sensors/room2/temperature

# Multi-level wildcard (#)
# Matches any number of levels (must be last character)
topics:
  - topic: sensors/#              # matches sensors/temperature
                                   # matches sensors/humidity
                                   # matches sensors/room1/temp
```

### Message Configuration

Define messages to publish automatically on a topic:

```yaml
messages:
  - payload: '{"temperature": 22.5}'
    delay: "0s"         # Initial delay before first publish
    repeat: true        # Continuously publish at interval
    interval: "5s"      # Time between repeated publishes
```

Message fields:

| Field | Type | Description |
|-------|------|-------------|
| `payload` | string | Message content (string or JSON) |
| `delay` | duration | Initial delay before first publish |
| `repeat` | boolean | Continuously publish at interval |
| `interval` | duration | Time between repeated publishes |

### Templated Payloads

Use template expressions in message payloads:

```yaml
messages:
  - payload: |
      {
        "deviceId": "sensor_001",
        "temperature": {{ random.int(18, 28) }},
        "humidity": {{ random.int(40, 80) }},
        "timestamp": "{{ now }}"
      }
    interval: "5s"
    repeat: true
```

Available templates:

| Template | Description |
|----------|-------------|
| `{{ now }}` | Current ISO timestamp |
| `{{ uuid }}` | Random UUID |
| `{{ timestamp }}` | Unix timestamp |
| `{{ random.int(min, max) }}` | Random integer in range |

### Retained Messages

Retained messages are stored on the broker and delivered to new subscribers immediately:

```yaml
topics:
  - topic: devices/status
    retain: true
    messages:
      - payload: '{"status": "online", "uptime": 12345}'
```

When a client subscribes to `devices/status`, they immediately receive the last retained message.

## QoS Levels

MQTT defines three Quality of Service levels for message delivery:

### QoS 0 - At Most Once

Fire and forget. No acknowledgment required. Messages may be lost.

```yaml
topics:
  - topic: sensors/motion
    qos: 0
    messages:
      - payload: '{"detected": true}'
```

Use for:
- High-frequency sensor data where occasional loss is acceptable
- Non-critical notifications
- Status updates that will be superseded

### QoS 1 - At Least Once

Message delivered at least once. May have duplicates. Requires acknowledgment.

```yaml
topics:
  - topic: sensors/temperature
    qos: 1
    messages:
      - payload: '{"value": 22.5}'
```

Use for:
- Important sensor readings
- Command acknowledgments
- Data that should not be lost

### QoS 2 - Exactly Once

Message delivered exactly once. Highest overhead with four-step handshake.

```yaml
topics:
  - topic: commands/critical
    qos: 2
    onPublish:
      response:
        payload: '{"status": "executed"}'
```

Use for:
- Financial transactions
- Critical commands
- Messages where duplicates cause problems

## Publish Handlers

Respond to messages received on a topic using publish handlers.

### Basic Response

```yaml
topics:
  - topic: commands/device/+
    qos: 1
    onPublish:
      response:
        payload: '{"status": "acknowledged", "timestamp": "{{ now }}"}'
        delay: "100ms"
```

### Forward Messages

Forward received messages to another topic:

```yaml
topics:
  - topic: commands/device/+
    qos: 1
    onPublish:
      response:
        payload: '{"status": "acknowledged"}'
      forward: responses/device
```

### Handler Fields

| Field | Type | Description |
|-------|------|-------------|
| `response` | object | Message to publish in response |
| `response.payload` | string | Response message content |
| `response.delay` | duration | Delay before sending response |
| `forward` | string | Topic to forward the message to |

### Using Message Content in Responses

Reference the received message in your response:

```yaml
topics:
  - topic: commands/+/execute
    qos: 1
    onPublish:
      response:
        payload: |
          {
            "status": "executed",
            "command": "{{ message.payload }}",
            "executedAt": "{{ now }}"
          }
```

## Authentication

Configure username/password authentication with Access Control Lists (ACL).

### Enable Authentication

```yaml
mqtt:
  port: 1883
  auth:
    enabled: true
    users:
      - username: sensor-gateway
        password: gateway-secret
        acl:
          - topic: "sensors/#"
            access: readwrite
      - username: dashboard
        password: readonly-pass
        acl:
          - topic: "sensors/#"
            access: read
```

### User Configuration

| Field | Type | Description |
|-------|------|-------------|
| `username` | string | Login username |
| `password` | string | Login password |
| `acl` | array | Access control rules |

### ACL Rules

| Field | Type | Description |
|-------|------|-------------|
| `topic` | string | Topic pattern (supports wildcards) |
| `access` | string | Access level |

Access levels:

| Level | Description |
|-------|-------------|
| `read` | Subscribe only |
| `write` | Publish only |
| `readwrite` | Both subscribe and publish |

### Role-Based Access Example

```yaml
auth:
  enabled: true
  users:
    # IoT devices - can only publish sensor data
    - username: device
      password: device123
      acl:
        - topic: "sensors/#"
          access: write
        - topic: "commands/#"
          access: read

    # Monitoring dashboard - read-only access
    - username: monitor
      password: monitor123
      acl:
        - topic: "#"
          access: read

    # Admin - full access
    - username: admin
      password: admin123
      acl:
        - topic: "#"
          access: all
```

## TLS Configuration

Enable TLS/SSL for encrypted MQTT connections.

### Basic TLS Setup

```yaml
mqtt:
  port: 8883
  tls:
    enabled: true
    certFile: ./certs/server.crt
    keyFile: ./certs/server.key
```

### Generate Self-Signed Certificates

```bash
# Generate private key
openssl genrsa -out server.key 2048

# Generate self-signed certificate
openssl req -new -x509 -key server.key -out server.crt -days 365

# Connect with TLS
mosquitto_sub -h localhost -p 8883 --cafile server.crt -t "sensors/#"
```

## Device Simulation

Simulate multiple IoT devices publishing to topics.

### Configuration

```yaml
topics:
  - topic: devices/+/telemetry
    qos: 1
    deviceSimulation:
      enabled: true
      deviceCount: 100
      deviceIdPattern: "device_{index}"
```

This creates 100 virtual devices (`device_0` through `device_99`) each publishing to their own topic:
- `devices/device_0/telemetry`
- `devices/device_1/telemetry`
- ...
- `devices/device_99/telemetry`

### Device Simulation Fields

| Field | Type | Description |
|-------|------|-------------|
| `enabled` | boolean | Enable device simulation |
| `deviceCount` | integer | Number of virtual devices |
| `deviceIdPattern` | string | Pattern for device IDs (`{index}` replaced with number) |

### Simulating Device Fleet

```yaml
topics:
  # Temperature sensors
  - topic: sensors/temperature/+
    qos: 1
    deviceSimulation:
      enabled: true
      deviceCount: 50
      deviceIdPattern: "temp_sensor_{index}"
    messages:
      - payload: |
          {
            "deviceId": "{{deviceId}}",
            "temperature": {{ random.int(15, 35) }},
            "timestamp": "{{ now }}"
          }
        interval: "10s"
        repeat: true

  # Motion sensors
  - topic: sensors/motion/+
    qos: 0
    deviceSimulation:
      enabled: true
      deviceCount: 20
      deviceIdPattern: "motion_{index}"
```

## Examples

### IoT Sensor Network

```yaml
version: "1.0"

mocks:
  - id: iot-sensors
    name: IoT Sensor Network
    type: mqtt
    enabled: true
    mqtt:
      port: 1883

      auth:
        enabled: true
        users:
          - username: sensor
            password: sensor123
            acl:
              - topic: "sensors/#"
                access: write
          - username: gateway
            password: gateway123
            acl:
              - topic: "#"
                access: all

      topics:
        # Temperature sensor
        - topic: sensors/temperature
          qos: 1
          retain: true
          messages:
            - payload: |
                {
                  "deviceId": "temp_001",
                  "type": "temperature",
                  "value": {{ random.int(18, 28) }},
                  "unit": "celsius",
                  "timestamp": "{{ now }}"
                }
              interval: "5s"
              repeat: true

        # Humidity sensor
        - topic: sensors/humidity
          qos: 1
          retain: true
          messages:
            - payload: |
                {
                  "deviceId": "humid_001",
                  "type": "humidity",
                  "value": {{ random.int(40, 80) }},
                  "unit": "percent",
                  "timestamp": "{{ now }}"
                }
              interval: "5s"
              repeat: true

        # Motion sensor (less frequent)
        - topic: sensors/motion
          qos: 0
          messages:
            - payload: |
                {
                  "deviceId": "motion_001",
                  "type": "motion",
                  "detected": true,
                  "zone": "entrance",
                  "timestamp": "{{ now }}"
                }
              interval: "30s"
              repeat: true

        # Device status (retained)
        - topic: devices/status
          qos: 1
          retain: true
          messages:
            - payload: |
                {
                  "deviceId": "gateway_001",
                  "status": "online",
                  "uptime": {{ random.int(1000, 50000) }},
                  "firmware": "1.2.3",
                  "timestamp": "{{ now }}"
                }
              delay: "1s"
```

### Command and Response System

```yaml
version: "1.0"

mocks:
  - id: command-system
    name: Device Command System
    type: mqtt
    enabled: true
    mqtt:
      port: 1883

      topics:
        # Command topic - listens for commands and responds
        - topic: commands/device/+
          qos: 2
          onPublish:
            response:
              payload: |
                {
                  "status": "acknowledged",
                  "command": "{{ message.payload }}",
                  "executedAt": "{{ now }}",
                  "id": "{{ uuid }}"
                }
              delay: "50ms"
            forward: responses/device

        # Response topic for monitoring
        - topic: responses/device
          qos: 1
          retain: true

        # Alert topic with high QoS
        - topic: alerts/+
          qos: 2
          retain: true
```

### Smart Home System

```yaml
version: "1.0"

mocks:
  - id: smart-home
    name: Smart Home Hub
    type: mqtt
    enabled: true
    mqtt:
      port: 1883

      auth:
        enabled: true
        users:
          - username: homeassistant
            password: ha-secret
            acl:
              - topic: "#"
                access: all
          - username: light
            password: light123
            acl:
              - topic: "home/lights/#"
                access: readwrite
          - username: thermostat
            password: thermo123
            acl:
              - topic: "home/climate/#"
                access: readwrite

      topics:
        # Living room light
        - topic: home/lights/living_room/state
          qos: 1
          retain: true
          messages:
            - payload: '{"on": true, "brightness": 80, "color": "#ffffff"}'

        # Living room light commands
        - topic: home/lights/living_room/set
          qos: 1
          onPublish:
            response:
              payload: |
                {
                  "on": true,
                  "brightness": 80,
                  "updated": "{{ now }}"
                }
            forward: home/lights/living_room/state

        # Thermostat
        - topic: home/climate/thermostat/state
          qos: 1
          retain: true
          messages:
            - payload: |
                {
                  "current_temp": {{ random.int(18, 24) }},
                  "target_temp": 21,
                  "mode": "heat",
                  "humidity": {{ random.int(40, 60) }}
                }
              interval: "30s"
              repeat: true

        # Door sensor
        - topic: home/security/front_door
          qos: 1
          retain: true
          messages:
            - payload: '{"state": "closed", "battery": 95}'

        # Motion sensor
        - topic: home/security/motion/hallway
          qos: 0
          messages:
            - payload: |
                {
                  "motion": true,
                  "timestamp": "{{ now }}"
                }
              interval: "60s"
              repeat: true
```

### Notification Service

```yaml
version: "1.0"

mocks:
  - id: notifications
    name: Push Notification Service
    type: mqtt
    enabled: true
    mqtt:
      port: 1883

      topics:
        # User notifications
        - topic: notifications/user/+
          qos: 1
          messages:
            - payload: |
                {
                  "id": "{{ uuid }}",
                  "type": "info",
                  "title": "System Update",
                  "body": "New features are available",
                  "timestamp": "{{ now }}"
                }
              interval: "30s"
              repeat: true

        # Broadcast notifications
        - topic: notifications/broadcast
          qos: 1
          retain: true
          messages:
            - payload: |
                {
                  "id": "{{ uuid }}",
                  "type": "announcement",
                  "title": "Maintenance Notice",
                  "body": "Scheduled maintenance at midnight",
                  "priority": "high"
                }
              delay: "5s"

        # Notification acknowledgments
        - topic: notifications/ack/+
          qos: 1
          onPublish:
            response:
              payload: '{"status": "received", "timestamp": "{{ now }}"}'
```

## CLI Commands

mockd provides CLI tools for interacting with MQTT brokers.

### mqtt publish

Publish a message to an MQTT topic:

```bash
# Publish a simple message
mockd mqtt publish sensors/temperature "25.5"

# Publish with custom broker
mockd mqtt publish -b mqtt.example.com:1883 sensors/temp "25.5"

# Publish with authentication
mockd mqtt publish -u user -P pass sensors/temp "25.5"

# Publish with QoS 1 and retain
mockd mqtt publish --qos 1 --retain sensors/temp "25.5"

# Publish JSON payload
mockd mqtt publish sensors/data '{"temp": 25.5, "humidity": 60}'

# Publish from file
mockd mqtt publish sensors/config @config.json
```

Flags:

| Flag | Description |
|------|-------------|
| `-b, --broker` | MQTT broker address (default: localhost:1883) |
| `-u, --username` | MQTT username |
| `-P, --password` | MQTT password |
| `--qos` | QoS level 0, 1, or 2 (default: 0) |
| `--retain` | Retain message on broker |

### mqtt subscribe

Subscribe to a topic and print received messages:

```bash
# Subscribe to a topic
mockd mqtt subscribe sensors/temperature

# Subscribe with wildcard
mockd mqtt subscribe "sensors/#"

# Subscribe to single-level wildcard
mockd mqtt subscribe "sensors/+/temperature"

# Receive only 5 messages then exit
mockd mqtt subscribe -n 5 sensors/temperature

# Subscribe with timeout
mockd mqtt subscribe -t 30s sensors/temperature

# Subscribe with authentication
mockd mqtt subscribe -u user -P pass sensors/temperature

# Subscribe with QoS 1
mockd mqtt subscribe --qos 1 sensors/temperature
```

Flags:

| Flag | Description |
|------|-------------|
| `-b, --broker` | MQTT broker address (default: localhost:1883) |
| `-u, --username` | MQTT username |
| `-P, --password` | MQTT password |
| `--qos` | QoS level 0, 1, or 2 (default: 0) |
| `-n, --count` | Number of messages to receive (0 = unlimited) |
| `-t, --timeout` | Timeout duration (e.g., 30s, 5m) |

### mqtt status

Show MQTT broker status from the admin API:

```bash
# Default admin URL
mockd mqtt status

# Custom admin URL
mockd mqtt status --admin-url http://localhost:9091

# JSON output
mockd mqtt status --json
```

Flags:

| Flag | Description |
|------|-------------|
| `--admin-url` | Admin API base URL (default: http://localhost:4290) |
| `--json` | Output in JSON format |

## Testing

### Using mockd CLI

```bash
# Start server
mockd serve --config mockd.yaml &

# Subscribe to all sensors in background
mockd mqtt subscribe "sensors/#" &

# Publish test messages
mockd mqtt publish sensors/temperature '{"value": 25.5}'
mockd mqtt publish sensors/humidity '{"value": 65}'

# Test command/response
mockd mqtt subscribe responses/device &
mockd mqtt publish commands/device/001 '{"action": "restart"}'
```

### Using Mosquitto Clients

Install mosquitto clients:

```bash
# Ubuntu/Debian
apt install mosquitto-clients

# macOS
brew install mosquitto

# Subscribe to all sensors
mosquitto_sub -h localhost -p 1883 -t "sensors/#" -v

# Subscribe to specific topic
mosquitto_sub -h localhost -p 1883 -t "sensors/temperature"

# Publish a message
mosquitto_pub -h localhost -p 1883 -t "sensors/temperature" -m '{"value": 25.5}'

# Publish with QoS 1
mosquitto_pub -h localhost -p 1883 -t "sensors/temperature" -m '{"value": 25.5}' -q 1

# Subscribe with authentication
mosquitto_sub -h localhost -p 1883 -u device -P secret123 -t "sensors/#"

# Publish with retain flag
mosquitto_pub -h localhost -p 1883 -t "status" -m "online" -r
```

### Integration Tests (JavaScript)

```javascript
const mqtt = require('mqtt');

describe('MQTT Mock', () => {
  let client;

  beforeEach((done) => {
    client = mqtt.connect('mqtt://localhost:1883');
    client.on('connect', done);
  });

  afterEach(() => {
    client.end();
  });

  test('receives temperature readings', (done) => {
    client.subscribe('sensors/temperature', (err) => {
      expect(err).toBeNull();
    });

    client.on('message', (topic, message) => {
      const data = JSON.parse(message.toString());
      expect(topic).toBe('sensors/temperature');
      expect(data).toHaveProperty('value');
      expect(data).toHaveProperty('unit');
      done();
    });
  });

  test('publishes and receives message', (done) => {
    const testTopic = 'test/messages';
    const testMessage = { id: 1, text: 'hello' };

    client.subscribe(testTopic);

    client.on('message', (topic, message) => {
      const data = JSON.parse(message.toString());
      expect(data).toEqual(testMessage);
      done();
    });

    client.publish(testTopic, JSON.stringify(testMessage));
  });
});
```

### Integration Tests (Go)

```go
package main

import (
    "encoding/json"
    "testing"
    "time"

    mqtt "github.com/eclipse/paho.mqtt.golang"
)

func TestMQTTMock(t *testing.T) {
    opts := mqtt.NewClientOptions().
        AddBroker("tcp://localhost:1883").
        SetClientID("test-client")

    client := mqtt.NewClient(opts)
    if token := client.Connect(); token.Wait() && token.Error() != nil {
        t.Fatalf("Failed to connect: %v", token.Error())
    }
    defer client.Disconnect(250)

    // Subscribe and receive message
    received := make(chan []byte, 1)
    client.Subscribe("sensors/temperature", 1, func(c mqtt.Client, m mqtt.Message) {
        received <- m.Payload()
    })

    select {
    case msg := <-received:
        var data map[string]interface{}
        if err := json.Unmarshal(msg, &data); err != nil {
            t.Fatalf("Failed to parse message: %v", err)
        }
        if _, ok := data["value"]; !ok {
            t.Error("Expected 'value' field in message")
        }
    case <-time.After(10 * time.Second):
        t.Fatal("Timeout waiting for message")
    }
}
```

### Integration Tests (Python)

```python
import pytest
import paho.mqtt.client as mqtt
import json
import time

def test_mqtt_subscribe():
    received = []

    def on_message(client, userdata, msg):
        received.append(json.loads(msg.payload))

    client = mqtt.Client()
    client.on_message = on_message
    client.connect("localhost", 1883)
    client.subscribe("sensors/temperature")
    client.loop_start()

    # Wait for message
    time.sleep(6)
    client.loop_stop()
    client.disconnect()

    assert len(received) > 0
    assert "value" in received[0]

def test_mqtt_publish():
    received = []

    def on_message(client, userdata, msg):
        received.append(json.loads(msg.payload))

    client = mqtt.Client()
    client.on_message = on_message
    client.connect("localhost", 1883)
    client.subscribe("test/topic")
    client.loop_start()

    # Publish message
    client.publish("test/topic", json.dumps({"test": True}))
    time.sleep(1)

    client.loop_stop()
    client.disconnect()

    assert len(received) == 1
    assert received[0]["test"] == True
```

### Testing with Authentication

```bash
# Start server with auth enabled
mockd serve --config mockd-auth.yaml &

# Test valid credentials
mosquitto_sub -h localhost -p 1883 -u sensor -P sensor123 -t "sensors/#"

# Test invalid credentials (should fail)
mosquitto_sub -h localhost -p 1883 -u invalid -P wrong -t "sensors/#"

# Test ACL (device user can only write to sensors)
mosquitto_pub -h localhost -p 1883 -u device -P device123 -t "sensors/temp" -m "25"
mosquitto_sub -h localhost -p 1883 -u device -P device123 -t "commands/#"  # denied
```

### Testing QoS Levels

```bash
# QoS 0 - At most once
mosquitto_pub -h localhost -p 1883 -t "qos/test" -m "qos0" -q 0

# QoS 1 - At least once
mosquitto_pub -h localhost -p 1883 -t "qos/test" -m "qos1" -q 1

# QoS 2 - Exactly once
mosquitto_pub -h localhost -p 1883 -t "qos/test" -m "qos2" -q 2

# Subscribe with specific QoS
mosquitto_sub -h localhost -p 1883 -t "qos/test" -q 2
```

## Next Steps

- [Response Templating](response-templating.md) - Dynamic response values
- [WebSocket Mocking](websocket-mocking.md) - Real-time bidirectional communication
- [TLS/HTTPS](tls-https.md) - Secure connections
