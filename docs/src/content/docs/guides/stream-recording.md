---
title: Stream Recording
description: Record WebSocket and SSE streams with full timing fidelity, then replay them as mocks for testing.
---

Record WebSocket and SSE (Server-Sent Events) streams with full timing fidelity, then replay them as mocks. Perfect for capturing real-time API behavior and creating reproducible test fixtures.

## Overview

Stream recording captures:

- **WebSocket**: Bidirectional message streams with frame types (text, binary, ping/pong, close)
- **SSE**: Server-sent event streams with event types, IDs, and retry hints

Each recording preserves:
- Message content and encoding
- Precise timing between frames
- Connection metadata (path, headers, query params)
- Protocol-specific details (close codes, subprotocols)

## Quick Start

### 1. Start mockd

```bash
mockd serve --config mocks.json
```

### 2. Start Recording via Admin API

```bash
# Start recording WebSocket traffic on a specific path
curl -X POST http://localhost:4290/stream-recordings/start \
  -H "Content-Type: application/json" \
  -d '{"protocol": "websocket", "path": "/ws/chat", "name": "chat-session"}'
```

### 3. Generate Traffic

Connect to your WebSocket or SSE endpoint through mockd:

```bash
# WebSocket
wscat -c ws://localhost:4280/ws/chat

# SSE
curl -N http://localhost:4280/events
```

### 4. Stop Recording and View

```bash
# Stop recording
curl -X POST http://localhost:4290/stream-recordings/{id}/stop

# List recordings via CLI
mockd stream-recordings list
```

```
ID            PROTOCOL   PATH           STATUS    FRAMES  DURATION  SIZE
01HXYZ123456  websocket  /ws/chat       complete  42      12.3s     8.2 KB
01HXYZ789012  sse        /events        complete  15      5.1s      2.1 KB
```

### 5. Convert to Mock

```bash
# Convert to mock config
mockd stream-recordings convert 01HXYZ123456 -o chat-scenario.json

# Start with the mock
mockd serve --config chat-scenario.json
```

## Recording via Admin API

### Start Recording

```bash
curl -X POST http://localhost:4290/stream-recordings/start \
  -H "Content-Type: application/json" \
  -d '{
    "protocol": "websocket",
    "path": "/ws/chat",
    "name": "chat-session-1"
  }'
```

Response:
```json
{
  "sessionId": "01HXYZ123456789ABCDEF",
  "recordingId": "01HXYZ123456789ABCDEF"
}
```

### Stop Recording

```bash
curl -X POST http://localhost:4290/stream-recordings/01HXYZ123456/stop
```

### List Recordings

```bash
# All recordings
curl http://localhost:4290/stream-recordings

# Filter by protocol
curl "http://localhost:4290/stream-recordings?protocol=websocket"

# Filter by path
curl "http://localhost:4290/stream-recordings?path=/ws/chat"
```

### Get Recording Details

```bash
curl http://localhost:4290/stream-recordings/01HXYZ123456
```

### Export Recording

```bash
curl -X POST http://localhost:4290/stream-recordings/01HXYZ123456/export \
  -o recording.json
```

### Convert to Mock

```bash
curl -X POST http://localhost:4290/stream-recordings/01HXYZ123456/convert \
  -H "Content-Type: application/json" \
  -d '{"simplifyTiming": true}' \
  -o mock-config.json
```

## CLI Commands

### List Recordings

```bash
# Basic list
mockd stream-recordings list

# Filter by protocol
mockd stream-recordings list --protocol websocket

# Filter by status
mockd stream-recordings list --status complete

# JSON output
mockd stream-recordings list --json

# Pagination
mockd stream-recordings list --limit 10 --offset 20
```

### Show Recording Details

```bash
mockd stream-recordings show 01HXYZ123456

# JSON output
mockd stream-recordings show 01HXYZ123456 --json
```

Output:
```
ID:          01HXYZ123456789ABCDEF
Name:        chat-session-1
Protocol:    websocket
Path:        /ws/chat
Status:      complete
Started:     2024-01-15T10:30:00Z
Ended:       2024-01-15T10:30:12Z
Duration:    12.3s
Frames:      42
File Size:   8.2 KB

WebSocket Details:
  Text Frames:   38
  Binary Frames: 2
  Ping/Pong:     2
  Close Code:    1000
```

### Export Recording

```bash
# Export to stdout
mockd stream-recordings export 01HXYZ123456

# Export to file
mockd stream-recordings export 01HXYZ123456 -o recording.json
```

### Convert to Mock Config

```bash
# Basic conversion
mockd stream-recordings convert 01HXYZ123456

# With timing normalization
mockd stream-recordings convert 01HXYZ123456 --simplify-timing

# Output to file
mockd stream-recordings convert 01HXYZ123456 -o scenario.json

# Fine-tune timing
mockd stream-recordings convert 01HXYZ123456 \
  --simplify-timing \
  --min-delay 50 \
  --max-delay 2000
```

### Delete Recording

```bash
# Soft delete (recoverable)
mockd stream-recordings delete 01HXYZ123456

# Skip confirmation
mockd stream-recordings delete 01HXYZ123456 --force

# Permanent delete
mockd stream-recordings delete 01HXYZ123456 --permanent
```

### Storage Management

```bash
# View storage stats
mockd stream-recordings stats

# Permanently remove soft-deleted recordings
mockd stream-recordings vacuum
```

### Active Sessions

```bash
# List recordings in progress
mockd stream-recordings sessions
```

## Recording Format

Recordings are stored as JSON files with this structure:

```json
{
  "id": "01HXYZ123456789ABCDEF",
  "version": "1.0",
  "name": "chat-session-1",
  "protocol": "websocket",
  "status": "complete",
  "startTime": "2024-01-15T10:30:00Z",
  "endTime": "2024-01-15T10:30:12Z",
  "duration": 12345,
  "metadata": {
    "path": "/ws/chat",
    "headers": {"Authorization": "[REDACTED]"},
    "query": {"room": "general"},
    "source": "manual"
  },
  "websocket": {
    "subprotocol": "chat.v1",
    "frames": [
      {
        "direction": "s2c",
        "type": "text",
        "data": "{\"type\":\"welcome\"}",
        "timestamp": "2024-01-15T10:30:00.100Z",
        "relativeMs": 100
      },
      {
        "direction": "c2s",
        "type": "text",
        "data": "{\"type\":\"join\",\"room\":\"general\"}",
        "timestamp": "2024-01-15T10:30:00.250Z",
        "relativeMs": 250
      }
    ],
    "closeCode": 1000,
    "closeReason": "Normal closure"
  },
  "stats": {
    "frameCount": 42,
    "bytesSent": 1200,
    "bytesReceived": 800,
    "fileSizeBytes": 8432
  }
}
```

### Frame Direction

- `c2s` - Client to server (sent by client)
- `s2c` - Server to client (sent by server)

### Message Types (WebSocket)

- `text` - UTF-8 text frame
- `binary` - Binary frame (base64 encoded in JSON)
- `ping` - Ping control frame
- `pong` - Pong control frame
- `close` - Close control frame

### SSE Recording Structure

```json
{
  "protocol": "sse",
  "sse": {
    "events": [
      {
        "type": "message",
        "data": "Hello world",
        "id": "evt-001",
        "timestamp": "2024-01-15T10:30:00.100Z",
        "relativeMs": 100
      }
    ]
  }
}
```

## Sensitive Data Handling

By default, mockd redacts sensitive headers:

- `Authorization`
- `Cookie` / `Set-Cookie`
- `X-API-Key`
- `X-Auth-Token`

### Custom Redaction

Configure in `mockd.yaml`:

```yaml
recording:
  filterHeaders:
    - Authorization
    - X-Custom-Secret
  filterBodyKeys:
    - password
    - secret
    - $.user.ssn
  redactValue: "[REDACTED]"
```

## Storage Configuration

```yaml
recording:
  dataDir: ~/.local/share/mockd/recordings
  maxBytes: 524288000  # 500MB
  warnPercent: 80      # Warn at 80% capacity
```

Storage location defaults:
- Linux: `~/.local/share/mockd/recordings/`
- macOS: `~/Library/Application Support/mockd/recordings/`
- Windows: `%LOCALAPPDATA%\mockd\recordings\`

## Use Cases

### Capture Production Behavior

Record real WebSocket traffic from production to create accurate test fixtures:

```bash
# Start mockd and the proxy
mockd serve

# Start recording via Admin API
curl -X POST http://localhost:4290/stream-recordings/start \
  -H "Content-Type: application/json" \
  -d '{"protocol": "websocket", "path": "/ws", "name": "prod-traffic"}'

# Run your app (configure to use mockd as WebSocket proxy)
WS_URL=ws://localhost:4280/ws npm test

# Stop recording and convert to mocks
curl -X POST http://localhost:4290/stream-recordings/{id}/stop
mockd stream-recordings convert {id} -o fixtures/chat.json
```

### Integration Testing

Create reproducible stream mocks for CI/CD:

```bash
# Start mockd with recorded scenario
mockd start --config fixtures/chat.json

# Run tests against mock
npm test
```

### Debug Timing Issues

Analyze frame timing in recordings:

```bash
mockd stream-recordings show 01HXYZ123456 --json | jq '.websocket.frames[] | {relativeMs, direction, type}'
```

## Admin API Reference

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/stream-recordings` | List recordings |
| GET | `/stream-recordings/stats` | Storage statistics |
| GET | `/stream-recordings/sessions` | Active recording sessions |
| POST | `/stream-recordings/start` | Start recording |
| POST | `/stream-recordings/vacuum` | Remove soft-deleted |
| GET | `/stream-recordings/{id}` | Get recording |
| DELETE | `/stream-recordings/{id}` | Delete recording |
| POST | `/stream-recordings/{id}/stop` | Stop recording session |
| POST | `/stream-recordings/{id}/export` | Export as JSON |
| POST | `/stream-recordings/{id}/convert` | Convert to mock config |

## Next Steps

- [Replay Modes](/guides/replay-modes/) - Learn about Pure, Synchronized, and Triggered replay
- [SSE Streaming](/guides/sse-streaming/) - SSE mock configuration
- [Admin API](/reference/admin-api/) - Stream recording API endpoints
