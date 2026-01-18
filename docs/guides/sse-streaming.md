# Server-Sent Events (SSE) and Streaming

mockd supports Server-Sent Events (SSE) and HTTP chunked transfer encoding for simulating streaming APIs like AI chat completions, real-time feeds, and large file downloads.

## Quick Start

### Basic SSE Mock

```json
{
  "id": "basic-sse",
  "matcher": { "method": "GET", "path": "/events" },
  "sse": {
    "events": [
      { "data": "Hello" },
      { "data": "World" }
    ],
    "timing": { "fixedDelay": 1000 }
  }
}
```

### OpenAI-Compatible Streaming

```json
{
  "id": "openai-mock",
  "matcher": { "method": "POST", "path": "/v1/chat/completions" },
  "sse": {
    "template": "openai-chat",
    "templateParams": {
      "tokens": ["Hello", "!", " How", " can", " I", " help", "?"],
      "model": "gpt-4",
      "finishReason": "stop",
      "includeDone": true,
      "delayPerToken": 50
    }
  }
}
```

## SSE Configuration

### Events

Define events to send to clients:

```json
{
  "sse": {
    "events": [
      {
        "type": "message",
        "data": "Event payload",
        "id": "event-1",
        "retry": 3000
      }
    ]
  }
}
```

| Field | Description |
|-------|-------------|
| `type` | Event type name (optional, for client filtering) |
| `data` | Event payload (string or JSON object) |
| `id` | Event ID (for Last-Event-ID resumption) |
| `retry` | Reconnection interval in milliseconds |
| `comment` | SSE comment (not delivered as event) |

### Timing Control

Control event delivery timing:

```json
{
  "sse": {
    "timing": {
      "initialDelay": 100,
      "fixedDelay": 500,
      "randomDelay": { "min": 100, "max": 500 },
      "burstMode": { "count": 5, "delay": 10, "pauseAfter": 1000 },
      "perEventDelays": [100, 200, 500]
    }
  }
}
```

| Field | Description |
|-------|-------------|
| `initialDelay` | Delay before first event (ms) |
| `fixedDelay` | Constant delay between events (ms) |
| `randomDelay` | Random delay range (min/max ms) |
| `burstMode` | Send events in bursts |
| `perEventDelays` | Specific delay for each event |

### Lifecycle Management

Control connection behavior:

```json
{
  "sse": {
    "lifecycle": {
      "keepaliveInterval": 15,
      "timeout": 300,
      "maxEvents": 100,
      "duration": 60,
      "gracefulShutdown": true,
      "terminationEvent": { "type": "close", "data": "Stream ended" }
    }
  }
}
```

| Field | Description |
|-------|-------------|
| `keepaliveInterval` | Keepalive ping interval (seconds, min 5) |
| `timeout` | Connection timeout (seconds) |
| `maxEvents` | Maximum events before closing |
| `duration` | Maximum stream duration (seconds) |
| `gracefulShutdown` | Send termination event before closing |
| `terminationEvent` | Event to send on graceful close |

### Rate Limiting

Control event rate:

```json
{
  "sse": {
    "rateLimit": {
      "eventsPerSecond": 10,
      "burstSize": 5
    }
  }
}
```

### Connection Resumption

Support Last-Event-ID header for reconnection:

```json
{
  "sse": {
    "resume": {
      "enabled": true,
      "bufferSize": 100
    }
  }
}
```

## Built-in Templates

### openai-chat

OpenAI Chat Completions streaming format:

```json
{
  "sse": {
    "template": "openai-chat",
    "templateParams": {
      "tokens": ["Hello", " World"],
      "model": "gpt-4",
      "finishReason": "stop",
      "includeDone": true,
      "delayPerToken": 50
    }
  }
}
```

### notification-stream

Real-time notification stream:

```json
{
  "sse": {
    "template": "notification-stream",
    "templateParams": {
      "notifications": [
        { "type": "alert", "message": "System update" }
      ],
      "includeTimestamp": true,
      "includeId": true,
      "eventType": "notification"
    }
  }
}
```

## Random Placeholders

Use placeholders in event data for dynamic values:

| Placeholder | Description | Example |
|-------------|-------------|---------|
| `$random:min:max` | Random integer | `$random:1:100` |
| `$uuid` | UUID v4 | `550e8400-e29b-41d4-a716-446655440000` |
| `$timestamp` | ISO 8601 timestamp | `2024-01-15T10:30:00Z` |
| `$pick:a,b,c` | Random choice | `$pick:red,green,blue` |

Example:
```json
{
  "data": {
    "id": "$uuid",
    "value": "$random:1:100",
    "status": "$pick:active,pending,complete",
    "timestamp": "$timestamp"
  }
}
```

## HTTP Chunked Transfer

For non-SSE streaming (file downloads, NDJSON):

### Basic Chunked Response

```json
{
  "chunked": {
    "data": "Large content to stream in chunks...",
    "chunkSize": 1024,
    "chunkDelay": 100
  }
}
```

### NDJSON Streaming

```json
{
  "chunked": {
    "format": "ndjson",
    "ndjsonItems": [
      { "id": 1, "name": "Alice" },
      { "id": 2, "name": "Bob" }
    ],
    "chunkDelay": 100
  }
}
```

## Admin API

### List Connections
```
GET /sse/connections
```

### Get Connection
```
GET /sse/connections/{id}
```

### Close Connection
```
DELETE /sse/connections/{id}
```

### Get Stats
```
GET /sse/stats
```

### Mock-Specific Operations
```
GET /mocks/{id}/sse/connections
DELETE /mocks/{id}/sse/connections
GET /mocks/{id}/sse/buffer
DELETE /mocks/{id}/sse/buffer
```

## Testing with curl

```bash
# Basic SSE
curl -N -H "Accept: text/event-stream" http://localhost:4280/events

# OpenAI streaming
curl -N -X POST \
  -H "Content-Type: application/json" \
  -H "Accept: text/event-stream" \
  -d '{"stream": true, "messages": [{"role": "user", "content": "Hi"}]}' \
  http://localhost:4280/v1/chat/completions

# Chunked download
curl -N http://localhost:4280/download/file

# NDJSON stream
curl -N http://localhost:4280/api/logs/stream
```

## Browser EventSource

```javascript
const source = new EventSource('/events');

source.onmessage = (event) => {
  console.log('Message:', event.data);
};

source.addEventListener('custom-type', (event) => {
  console.log('Custom event:', event.data);
});

source.onerror = (error) => {
  console.error('Error:', error);
};
```

## Examples

See `docs/examples/sse/` for complete configuration examples:

- `basic-sse.json` - Simple SSE events
- `typed-events.json` - Events with types and IDs
- `openai-chat.json` - OpenAI streaming mock
- `stock-ticker.json` - Real-time data feed
- `chunked-download.json` - Chunked file download
- `ndjson-stream.json` - NDJSON log streaming
