---
title: WebSocket Mocking
description: Create mock WebSocket endpoints for real-time bidirectional communication testing with message matching, response templating, and scripted scenarios.
---

WebSocket mocking enables testing of real-time bidirectional communication without connecting to actual backend services. mockd provides full WebSocket support with message matching, response templating, and scripted scenarios.

## Overview

Use WebSocket mocks when you need to:

- Test chat applications, notifications, or live updates
- Simulate real-time data feeds (stock prices, sports scores, IoT sensors)
- Develop frontends before backend WebSocket services are ready
- Create reproducible test scenarios for bidirectional protocols
- Debug client-side WebSocket handling

## Quick Start

Create a minimal WebSocket mock:

```yaml
version: "1.0"

mocks:
  - id: simple-ws
    name: Simple WebSocket
    type: websocket
    websocket:
      path: /ws
      echoMode: true
```

Start the server and connect:

```bash
# Start mockd
mockd serve --config mockd.yaml

# Connect with the mockd CLI
mockd websocket connect ws://localhost:4280/ws

# Or use wscat
wscat -c ws://localhost:4280/ws
```

In echo mode, any message you send is echoed back.

## Configuration

### Full WebSocket Spec

```yaml
version: "1.0"

mocks:
  - id: ws-full-example
    name: Full WebSocket Example
    type: websocket
    enabled: true
    websocket:
      # Required: endpoint path
      path: /ws/chat

      # Subprotocol negotiation
      subprotocols:
        - chat.v1
        - chat.v2
      requireSubprotocol: true  # Reject connections without matching subprotocol

      # Connection limits
      maxMessageSize: 65536      # Maximum message size in bytes
      maxConnections: 100        # Maximum concurrent connections
      idleTimeout: "5m"          # Disconnect after inactivity

      # Echo mode: reflect messages back to sender
      echoMode: false

      # Heartbeat/keepalive
      heartbeat:
        enabled: true
        interval: "30s"   # Send ping every 30 seconds
        timeout: "10s"    # Disconnect if no pong within 10 seconds

      # Message matchers (evaluated in order)
      matchers:
        - match:
            type: exact
            value: "ping"
          response:
            type: text
            value: "pong"

      # Default response when no matcher matches
      defaultResponse:
        type: json
        value:
          error: "Unknown message"

      # Scripted scenario (optional)
      scenario:
        name: welcome-flow
        steps:
          - type: send
            message:
              type: json
              value: { "type": "welcome" }
```

### Configuration Reference

| Field | Type | Description |
|-------|------|-------------|
| `path` | string | Endpoint path (required) |
| `subprotocols` | string[] | Supported WebSocket subprotocols |
| `requireSubprotocol` | boolean | Reject connections without matching subprotocol |
| `maxMessageSize` | integer | Maximum message size in bytes |
| `maxConnections` | integer | Maximum concurrent connections |
| `idleTimeout` | duration | Connection idle timeout (e.g., "30s", "5m") |
| `echoMode` | boolean | Echo received messages back to client |
| `heartbeat` | object | Ping/pong keepalive configuration |
| `matchers` | array | Message matchers with responses |
| `defaultResponse` | object | Response when no matcher matches |
| `scenario` | object | Scripted message sequence |

## Message Matchers

Matchers define how to respond to incoming WebSocket messages. Each matcher has a `match` criteria and `response` configuration.

### Match Types

#### Exact Match

Match the entire message exactly:

```yaml
matchers:
  - match:
      type: exact
      value: "ping"
    response:
      type: text
      value: "pong"
```

#### Contains Match

Match if the message contains a substring:

```yaml
matchers:
  - match:
      type: contains
      value: "hello"
    response:
      type: text
      value: "Hello from mockd!"
```

#### Prefix Match

Match if the message starts with a prefix:

```yaml
matchers:
  - match:
      type: prefix
      value: "CMD:"
    response:
      type: text
      value: "Command received"
```

#### Suffix Match

Match if the message ends with a suffix:

```yaml
matchers:
  - match:
      type: suffix
      value: "?"
    response:
      type: text
      value: "That's a question!"
```

#### Regex Match

Match using a regular expression:

```yaml
matchers:
  - match:
      type: regex
      value: "user_\\d+"
    response:
      type: json
      value:
        matched: true
        pattern: "user ID"
```

#### JSON Path Match

Match specific fields in JSON messages using JSONPath:

```yaml
matchers:
  # Match messages where $.type equals "ping"
  - match:
      type: json
      path: "$.type"
      value: "ping"
    response:
      type: json
      value:
        type: "pong"
        timestamp: "{{now}}"

  # Match messages where $.action equals "subscribe"
  - match:
      type: json
      path: "$.action"
      value: "subscribe"
    response:
      type: json
      value:
        action: "subscribed"
        channel: "{{message.channel}}"
```

### Message Type Filter

Filter by WebSocket message type (text or binary):

```yaml
matchers:
  - match:
      type: exact
      value: "ping"
      messageType: text   # Only match text messages
    response:
      type: text
      value: "pong"

  - match:
      type: regex
      value: ".*"
      messageType: binary  # Only match binary messages
    response:
      type: text
      value: "Binary message received"
```

### No Response

Match without sending a response (useful for logging or scenario progression):

```yaml
matchers:
  - match:
      type: json
      path: "$.type"
      value: "heartbeat"
    noResponse: true
```

### Matcher Priority

Matchers are evaluated in order. The first matching rule wins:

```yaml
matchers:
  # Specific match first
  - match:
      type: exact
      value: "ping"
    response:
      type: text
      value: "pong"

  # Generic match later
  - match:
      type: regex
      value: ".*"
    response:
      type: text
      value: "Unknown command"
```

## Responses

### Response Types

#### Text Response

```yaml
response:
  type: text
  value: "Hello, World!"
```

#### JSON Response

```yaml
response:
  type: json
  value:
    status: "ok"
    timestamp: "{{now}}"
    data:
      message: "Welcome!"
```

#### Binary Response

```yaml
response:
  type: binary
  value: "SGVsbG8gV29ybGQh"  # Base64 encoded
```

### Response Delay

Add artificial latency:

```yaml
response:
  type: json
  value:
    status: "processed"
  delay: "500ms"
```

### Templated Responses

Use template expressions in responses:

```yaml
response:
  type: json
  value:
    id: "{{uuid}}"
    timestamp: "{{now}}"
    echo: "{{message}}"  # Echo the received message
```

Available template variables:

| Expression | Description |
|------------|-------------|
| `{{message}}` | The received message content |
| `{{now}}` | Current ISO timestamp |
| `{{timestamp}}` | Unix timestamp (seconds) |
| `{{uuid}}` | Random UUID |

### Default Response

Define a fallback for unmatched messages:

```yaml
websocket:
  path: /ws
  matchers:
    - match:
        type: exact
        value: "ping"
      response:
        type: text
        value: "pong"

  defaultResponse:
    type: json
    value:
      type: "error"
      message: "Unknown command"
      received: "{{message}}"
```

### Echo Mode

When `echoMode: true`, messages are echoed back to the client. This is useful for testing client message handling:

```yaml
websocket:
  path: /ws/echo
  echoMode: true
```

Echo mode works alongside matchers. Matchers are checked first; if none match and no default response is configured, the message is echoed.

## Scenarios

Scenarios enable scripted message sequences for complex testing flows.

### Basic Scenario

```yaml
websocket:
  path: /ws/onboarding
  scenario:
    name: welcome-flow
    steps:
      # Send welcome message immediately on connect
      - type: send
        message:
          type: json
          value:
            type: "welcome"
            message: "Connected to server"

      # Wait for client ready signal
      - type: wait
        match:
          type: json
          path: "$.type"
          value: "ready"
        timeout: "10s"

      # Send session info
      - type: send
        message:
          type: json
          value:
            type: "session_start"
            sessionId: "{{uuid}}"
```

### Scenario Step Types

#### Send Step

Send a message to the client:

```yaml
- type: send
  message:
    type: json
    value:
      event: "notification"
      data: "Hello!"
```

#### Wait Step

Wait for a client message:

```yaml
- type: wait
  match:
    type: json
    path: "$.type"
    value: "acknowledge"
  timeout: "5s"
  optional: false  # Fail if timeout expires (default)
```

#### Delay Step

Pause for a duration:

```yaml
- type: delay
  duration: "2s"
```

### Looping Scenarios

Repeat the scenario when it completes:

```yaml
scenario:
  name: heartbeat-loop
  loop: true
  steps:
    - type: delay
      duration: "5s"
    - type: send
      message:
        type: json
        value:
          type: "heartbeat"
          timestamp: "{{now}}"
```

### Reset on Reconnect

Reset scenario state when a client reconnects:

```yaml
scenario:
  name: tutorial
  resetOnReconnect: true
  steps:
    - type: send
      message:
        type: text
        value: "Welcome! Let's begin the tutorial..."
```

## Examples

### Chat Application

```yaml
version: "1.0"

mocks:
  - id: chat-room
    name: Chat Room
    type: websocket
    websocket:
      path: /ws/chat
      subprotocols:
        - chat
        - json
      maxMessageSize: 65536
      idleTimeout: "10m"
      maxConnections: 100

      heartbeat:
        enabled: true
        interval: "30s"
        timeout: "10s"

      matchers:
        # Join room
        - match:
            type: json
            path: "$.type"
            value: "join"
          response:
            type: json
            value:
              type: "joined"
              message: "Welcome to the chat room!"
              timestamp: "{{now}}"

        # Send message
        - match:
            type: json
            path: "$.type"
            value: "message"
          response:
            type: json
            value:
              type: "message_ack"
              id: "{{uuid}}"
              timestamp: "{{now}}"

        # Leave room
        - match:
            type: json
            path: "$.type"
            value: "leave"
          response:
            type: json
            value:
              type: "left"
              message: "Goodbye!"

        # Typing indicator
        - match:
            type: json
            path: "$.type"
            value: "typing"
          noResponse: true

        # Status command
        - match:
            type: exact
            value: "status"
          response:
            type: json
            value:
              type: "status"
              users: 42
              uptime: "{{timestamp}}"

        # Help command
        - match:
            type: exact
            value: "help"
          response:
            type: text
            value: |
              Available commands:
              - {"type": "join", "username": "..."}: Join chat
              - {"type": "message", "content": "..."}: Send message
              - {"type": "leave"}: Leave chat
              - status: Get server status
              - help: Show this help

      defaultResponse:
        type: json
        value:
          type: "error"
          message: "Unknown command. Type 'help' for available commands."
```

### Notification Service

```yaml
version: "1.0"

mocks:
  - id: notifications
    name: Push Notifications
    type: websocket
    websocket:
      path: /ws/notifications
      heartbeat:
        enabled: true
        interval: "30s"

      matchers:
        # Subscribe to channel
        - match:
            type: json
            path: "$.action"
            value: "subscribe"
          response:
            type: json
            value:
              action: "subscribed"
              channel: "{{message.channel}}"

        # Unsubscribe
        - match:
            type: json
            path: "$.action"
            value: "unsubscribe"
          response:
            type: json
            value:
              action: "unsubscribed"
              channel: "{{message.channel}}"

      # Send periodic notifications
      scenario:
        name: notification-stream
        loop: true
        steps:
          - type: delay
            duration: "10s"
          - type: send
            message:
              type: json
              value:
                type: "notification"
                id: "{{uuid}}"
                title: "New update available"
                body: "Check out the latest features"
                timestamp: "{{now}}"
```

### Real-Time Data Feed

```yaml
version: "1.0"

mocks:
  - id: stock-ticker
    name: Stock Ticker
    type: websocket
    websocket:
      path: /ws/stocks
      maxConnections: 1000

      matchers:
        # Subscribe to symbol
        - match:
            type: json
            path: "$.action"
            value: "subscribe"
          response:
            type: json
            value:
              action: "subscribed"
              symbol: "{{message.symbol}}"
              message: "You will receive updates for this symbol"

      # Simulate price updates
      scenario:
        name: price-updates
        loop: true
        steps:
          - type: delay
            duration: "1s"
          - type: send
            message:
              type: json
              value:
                type: "price_update"
                symbol: "MOCK"
                price: 123.45
                change: 1.23
                volume: 1000000
                timestamp: "{{now}}"
```

### GraphQL Subscriptions

```yaml
version: "1.0"

mocks:
  - id: graphql-ws
    name: GraphQL WebSocket
    type: websocket
    websocket:
      path: /graphql
      subprotocols:
        - graphql-ws
        - graphql-transport-ws
      requireSubprotocol: true

      matchers:
        # Connection init
        - match:
            type: json
            path: "$.type"
            value: "connection_init"
          response:
            type: json
            value:
              type: "connection_ack"

        # Subscribe
        - match:
            type: json
            path: "$.type"
            value: "subscribe"
          response:
            type: json
            value:
              type: "next"
              id: "{{message.id}}"
              payload:
                data:
                  onMessage:
                    id: "{{uuid}}"
                    content: "Subscription active"

        # Ping
        - match:
            type: json
            path: "$.type"
            value: "ping"
          response:
            type: json
            value:
              type: "pong"
```

## CLI Commands

mockd provides CLI tools for creating WebSocket mocks and interacting with WebSocket endpoints.

### Add a WebSocket Mock

Create WebSocket mocks directly from the command line using `mockd websocket add`:

```bash
# Echo mode — reflects messages back to sender
mockd websocket add --path /ws/echo --echo

# With a custom path
mockd websocket add --path /ws/chat --echo
```

Output:

```
Created mock: websocket_0b5ebb9fa569a655
  Type: websocket
  Path: /ws/echo
  Echo: enabled
```

#### Add Command Flags

| Flag | Description |
|------|-------------|
| `--path` | WebSocket endpoint path (required) |
| `--echo` | Enable echo mode (reflect messages back) |
| `--admin-url` | Admin API URL (default: `http://localhost:4290`) |

:::tip
For WebSocket mocks with matchers, scenarios, or complex configurations, use a YAML config file instead of the CLI. The CLI `add` command creates simple echo-mode endpoints for quick testing.
:::

### websocket connect

Start an interactive WebSocket session (REPL mode):

```bash
# Basic connection
mockd websocket connect ws://localhost:4280/ws

# With custom headers
mockd websocket connect -H "Authorization:Bearer token" ws://localhost:4280/ws

# With subprotocol
mockd websocket connect --subprotocol graphql-ws ws://localhost:4280/graphql

# JSON output format
mockd websocket connect --json ws://localhost:4280/ws
```

Flags:
- `-H, --header` - Custom headers (key:value), repeatable
- `--subprotocol` - WebSocket subprotocol
- `-t, --timeout` - Connection timeout (default: 30s)
- `--json` - Output messages in JSON format

### websocket send

Send a single message and exit:

```bash
# Send text message
mockd websocket send ws://localhost:4280/ws "hello"

# Send JSON message
mockd websocket send ws://localhost:4280/ws '{"action":"ping"}'

# Send from file
mockd websocket send ws://localhost:4280/ws @message.json

# With custom headers
mockd websocket send -H "Authorization:Bearer token" ws://localhost:4280/ws "hello"
```

Flags:
- `-H, --header` - Custom headers (key:value), repeatable
- `--subprotocol` - WebSocket subprotocol
- `-t, --timeout` - Connection timeout (default: 30s)
- `--json` - Output result in JSON format

### websocket listen

Stream incoming messages:

```bash
# Listen indefinitely
mockd websocket listen ws://localhost:4280/ws

# Listen for 10 messages then exit
mockd websocket listen -n 10 ws://localhost:4280/ws

# JSON output
mockd websocket listen --json ws://localhost:4280/ws

# With headers
mockd websocket listen -H "Authorization:Bearer token" ws://localhost:4280/ws
```

Flags:
- `-H, --header` - Custom headers (key:value), repeatable
- `--subprotocol` - WebSocket subprotocol
- `-t, --timeout` - Connection timeout (default: 30s)
- `-n, --count` - Number of messages to receive (0 = unlimited)
- `--json` - Output messages in JSON format

### websocket status

Show WebSocket mock status from the admin API:

```bash
# Default admin URL
mockd websocket status

# Custom admin URL
mockd websocket status --admin-url http://localhost:9091

# JSON output
mockd websocket status --json
```

Flags:
- `--admin-url` - Admin API base URL (default: http://localhost:4290)
- `--json` - Output in JSON format

## Testing WebSocket Mocks

### Using mockd CLI

```bash
# Start server
mockd serve --config mockd.yaml &

# Test echo mode
mockd websocket send ws://localhost:4280/ws/echo "test message"

# Interactive testing
mockd websocket connect ws://localhost:4280/ws/chat
> {"type": "join", "username": "testuser"}
< {"type": "joined", "message": "Welcome to the chat room!", "timestamp": "..."}
> help
< Available commands: ...
```

### Using wscat

```bash
# Install wscat
npm install -g wscat

# Connect and interact
wscat -c ws://localhost:4280/ws/chat
> ping
< pong
> {"type": "join", "username": "alice"}
< {"type": "joined", "message": "Welcome to the chat room!", "timestamp": "..."}
```

### Using curl (for connection testing)

```bash
# Verify WebSocket endpoint exists (returns 400 for non-upgrade requests)
curl -i http://localhost:4280/ws

# Test with upgrade headers
curl -i -N \
  -H "Connection: Upgrade" \
  -H "Upgrade: websocket" \
  -H "Sec-WebSocket-Version: 13" \
  -H "Sec-WebSocket-Key: $(openssl rand -base64 16)" \
  http://localhost:4280/ws
```

### Using websocat

```bash
# Install websocat
cargo install websocat

# Simple connection
websocat ws://localhost:4280/ws

# One-shot message
echo "ping" | websocat ws://localhost:4280/ws

# With subprotocol
websocat --protocol chat ws://localhost:4280/ws/chat
```

### Integration Tests (JavaScript)

```javascript
const WebSocket = require('ws');

describe('WebSocket Mock', () => {
  let ws;

  beforeEach((done) => {
    ws = new WebSocket('ws://localhost:4280/ws/chat');
    ws.on('open', done);
  });

  afterEach(() => {
    ws.close();
  });

  test('responds to ping with pong', (done) => {
    ws.on('message', (data) => {
      expect(data.toString()).toBe('pong');
      done();
    });
    ws.send('ping');
  });

  test('handles JSON messages', (done) => {
    ws.on('message', (data) => {
      const response = JSON.parse(data.toString());
      expect(response.type).toBe('joined');
      expect(response.message).toContain('Welcome');
      done();
    });
    ws.send(JSON.stringify({ type: 'join', username: 'testuser' }));
  });
});
```

### Integration Tests (Go)

```go
package main

import (
    "testing"
    "github.com/gorilla/websocket"
)

func TestWebSocketMock(t *testing.T) {
    conn, _, err := websocket.DefaultDialer.Dial("ws://localhost:4280/ws/chat", nil)
    if err != nil {
        t.Fatalf("Failed to connect: %v", err)
    }
    defer conn.Close()

    // Test ping/pong
    if err := conn.WriteMessage(websocket.TextMessage, []byte("ping")); err != nil {
        t.Fatalf("Failed to send: %v", err)
    }

    _, msg, err := conn.ReadMessage()
    if err != nil {
        t.Fatalf("Failed to read: %v", err)
    }

    if string(msg) != "pong" {
        t.Errorf("Expected 'pong', got '%s'", msg)
    }
}
```

### Integration Tests (Python)

```python
import pytest
import websocket
import json

def test_websocket_ping():
    ws = websocket.create_connection("ws://localhost:4280/ws/chat")
    ws.send("ping")
    result = ws.recv()
    assert result == "pong"
    ws.close()

def test_websocket_json():
    ws = websocket.create_connection("ws://localhost:4280/ws/chat")
    ws.send(json.dumps({"type": "join", "username": "testuser"}))
    result = json.loads(ws.recv())
    assert result["type"] == "joined"
    assert "Welcome" in result["message"]
    ws.close()
```

## Next Steps

- [Response Templating](/guides/response-templating) - Dynamic response values
- [Request Matching](/guides/request-matching) - HTTP request matching patterns
- [Stateful Mocking](/guides/stateful-mocking) - CRUD simulation with state
