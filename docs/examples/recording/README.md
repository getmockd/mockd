# Stream Recording Examples

Example recordings demonstrating WebSocket and SSE stream capture formats.

## Files

| File | Protocol | Description |
|------|----------|-------------|
| `websocket-chat.json` | WebSocket | Bidirectional chat with auth, messages, ping/pong |
| `sse-notifications.json` | SSE | Real-time notification stream with typed events |
| `openai-streaming.json` | SSE | OpenAI-compatible chat completion streaming |

## Usage

### Load as Mock

```bash
# Convert recording to mock config
mockd stream-recordings convert 01EXAMPLE_CHAT_RECORDING -o chat-mock.json

# Start mockd with the mock
mockd start --config chat-mock.json
```

### Import Recording

Copy a recording file to your mockd recordings directory:

```bash
cp websocket-chat.json ~/.config/mockd/recordings/
```

### Use in Tests

Reference these examples to understand the recording format when writing tests or building tooling around mockd recordings.

## Recording Format

See [Stream Recording Guide](../../guides/stream-recording.md) for full format documentation.

### Key Fields

```json
{
  "formatVersion": "1.0",
  "id": "ULID",
  "protocol": "websocket|sse",
  "status": "complete|incomplete|recording",
  "metadata": {
    "path": "/ws/endpoint",
    "headers": {},
    "query": {}
  },
  "websocket": { "frames": [...] },
  "sse": { "events": [...] },
  "stats": { "frameCount": 0 }
}
```

### Frame/Event Fields

**WebSocket Frame:**
```json
{
  "direction": "c2s|s2c",
  "type": "text|binary|ping|pong|close",
  "data": "content",
  "relativeMs": 100,
  "timestamp": "2024-01-15T10:30:00Z"
}
```

**SSE Event:**
```json
{
  "type": "message",
  "data": "content",
  "id": "evt-001",
  "relativeMs": 100,
  "timestamp": "2024-01-15T10:30:00Z"
}
```
