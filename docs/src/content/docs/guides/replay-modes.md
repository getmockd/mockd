---
title: Replay Modes
description: Understand mockd's three replay modes for stream recordings - Pure, Synchronized, and Triggered - and when to use each.
---

mockd supports three replay modes for stream recordings, each suited to different testing scenarios. Understanding when to use each mode helps you create more effective and reliable tests.

## Overview

| Mode | Timing | Client Input | Best For |
|------|--------|--------------|----------|
| **Pure** | Original delays preserved | Ignored | Demo, playback, timing verification |
| **Synchronized** | Waits for client messages | Required to proceed | Protocol compliance, request-response flows |
| **Triggered** | Manual/API control | Optional | Step debugging, integration tests |

## Pure Mode

Replays server messages with original timing, ignoring any client input.

### How It Works

1. Recording starts playing immediately on connection
2. Delays between frames match the original recording (scaled by `timingScale`)
3. Client messages are ignored - playback continues regardless
4. Completes when all server frames have been sent

### Use Cases

- **Demos**: Show realistic streaming behavior
- **Timing verification**: Ensure client handles message timing correctly
- **Load testing**: Generate predictable server traffic patterns
- **UI development**: Develop against a predictable stream

### Configuration

```bash
# Via Admin API
curl -X POST http://localhost:4290/stream-recordings/01HXYZ123456/replay \
  -H "Content-Type: application/json" \
  -d '{
    "mode": "pure",
    "timingScale": 1.0
  }'
```

```json
{
  "mode": "pure",
  "timingScale": 1.0
}
```

### Options

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `timingScale` | float | 1.0 | Speed multiplier. 0.5 = 2x speed, 2.0 = half speed |

### Example: 2x Speed Playback

```bash
curl -X POST http://localhost:4290/stream-recordings/01HXYZ123456/replay \
  -d '{"mode": "pure", "timingScale": 0.5}'
```

### Pause/Resume

Pure mode supports pause and resume:

```bash
# Pause
curl -X POST http://localhost:4290/replay/SESSION_ID/pause

# Resume
curl -X POST http://localhost:4290/replay/SESSION_ID/resume
```

## Synchronized Mode

Waits for expected client messages before sending server responses. Ensures the client follows the recorded conversation flow.

### How It Works

1. Recording analyzes client-to-server and server-to-client frame order
2. When a client message is expected, replay pauses and waits
3. Client must send a message (optionally matching expected content)
4. After receiving client message, server responses are sent
5. Timeout occurs if client doesn't respond within limit

### Use Cases

- **Protocol compliance testing**: Verify client sends correct messages
- **Integration testing**: Test full request-response flows
- **Conversation flow testing**: Ensure proper message sequencing
- **State machine validation**: Test stateful WebSocket protocols

### Configuration

```bash
curl -X POST http://localhost:4290/stream-recordings/01HXYZ123456/replay \
  -H "Content-Type: application/json" \
  -d '{
    "mode": "synchronized",
    "strictMatching": false,
    "timeout": 30000
  }'
```

```json
{
  "mode": "synchronized",
  "strictMatching": false,
  "timeout": 30000
}
```

### Options

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `strictMatching` | bool | false | Require exact message content match |
| `timeout` | int | 30000 | Max ms to wait for client message |

### Strict vs Loose Matching

**Loose matching** (default): Any client message advances the replay. Useful when message content varies but sequence matters.

**Strict matching**: Client message must exactly match recorded content. Useful for protocol compliance verification.

```bash
# Strict matching - client must send exact messages
curl -X POST http://localhost:4290/stream-recordings/01HXYZ123456/replay \
  -d '{"mode": "synchronized", "strictMatching": true}'
```

### Timeout Handling

If the client doesn't send a message within the timeout:

- Replay session enters error state
- `ErrMatchTimeout` error is returned
- Session can be stopped and restarted

### Example Flow

Given a recording with this sequence:
```
1. [s2c] Welcome message
2. [c2s] Client login
3. [s2c] Login success
4. [c2s] Client request data
5. [s2c] Data response
```

Replay behavior:
1. Server sends "Welcome message" immediately
2. Replay waits for client to send any message
3. Client sends login -> Server sends "Login success"
4. Replay waits for client
5. Client sends request -> Server sends "Data response"
6. Replay completes

## Triggered Mode

Manual control over frame advancement via API. Each frame or batch of frames is sent only when explicitly triggered.

### How It Works

1. Recording loads but doesn't start sending
2. Optionally sends first frame on connect (`autoAdvanceOnConnect`)
3. Waits for `/replay/{id}/advance` API calls
4. Each advance sends one or more frames
5. Completes when all frames have been sent

### Use Cases

- **Step debugging**: Examine state after each message
- **Integration test assertions**: Assert between each message
- **Controlled scenarios**: Precise control over message timing
- **Interactive demos**: Manual control for presentations

### Configuration

```bash
curl -X POST http://localhost:4290/stream-recordings/01HXYZ123456/replay \
  -H "Content-Type: application/json" \
  -d '{
    "mode": "triggered",
    "autoAdvanceOnConnect": true
  }'
```

```json
{
  "mode": "triggered",
  "autoAdvanceOnConnect": true
}
```

### Options

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `autoAdvanceOnConnect` | bool | false | Send first frame immediately on connect |

### Advancing Playback

```bash
# Advance 1 frame (default)
curl -X POST http://localhost:4290/replay/SESSION_ID/advance

# Advance N frames
curl -X POST http://localhost:4290/replay/SESSION_ID/advance \
  -d '{"count": 5}'

# Advance until specific content
curl -X POST http://localhost:4290/replay/SESSION_ID/advance \
  -d '{"until": "{\"type\":\"complete\"}"}'
```

### Advance Response

```json
{
  "framesSent": 1,
  "currentFrame": 5,
  "totalFrames": 42,
  "status": "waiting",
  "complete": false
}
```

### Example: Integration Test

```javascript
// Start triggered replay
const { replayId } = await fetch('/stream-recordings/REC_ID/replay', {
  method: 'POST',
  body: JSON.stringify({ mode: 'triggered' })
}).then(r => r.json());

// Connect WebSocket
const ws = new WebSocket('ws://localhost:4280/ws/chat');

// Advance and assert
await fetch(`/replay/${replayId}/advance`, { method: 'POST' });
const msg1 = await nextMessage(ws);
expect(msg1.type).toBe('welcome');

await fetch(`/replay/${replayId}/advance`, { method: 'POST' });
const msg2 = await nextMessage(ws);
expect(msg2.type).toBe('ready');

// Clean up
await fetch(`/replay/${replayId}`, { method: 'DELETE' });
```

## Replay Status

Check replay session status:

```bash
curl http://localhost:4290/replay/SESSION_ID
```

```json
{
  "id": "01REPLAY123456",
  "recordingId": "01HXYZ123456",
  "status": "playing",
  "currentFrame": 15,
  "totalFrames": 42,
  "framesSent": 15,
  "startedAt": "2024-01-15T10:30:00Z",
  "elapsedMs": 5432,
  "config": {
    "mode": "pure",
    "timingScale": 1.0
  }
}
```

### Status Values

| Status | Description |
|--------|-------------|
| `pending` | Session created, not yet started |
| `playing` | Actively sending frames |
| `waiting` | Waiting for client input (synchronized) or trigger (triggered) |
| `paused` | Paused (pure mode only) |
| `complete` | All frames sent |
| `aborted` | Session stopped early |

## Managing Replay Sessions

### List Active Sessions

```bash
curl http://localhost:4290/replay
```

### Stop Replay

```bash
curl -X DELETE http://localhost:4290/replay/SESSION_ID
```

## Replay API Reference

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/stream-recordings/{id}/replay` | Start replay for recording |
| GET | `/replay` | List active replay sessions |
| GET | `/replay/{id}` | Get replay session status |
| DELETE | `/replay/{id}` | Stop replay session |
| POST | `/replay/{id}/advance` | Advance triggered replay |
| POST | `/replay/{id}/pause` | Pause pure mode replay |
| POST | `/replay/{id}/resume` | Resume paused replay |

## Mode Comparison

### When to Use Each Mode

```
Need original timing?
├── Yes → Pure Mode
│   └── Want to control speed? → Use timingScale
└── No
    ├── Need to verify client messages? → Synchronized Mode
    │   └── Need exact content match? → strictMatching: true
    └── Need step-by-step control? → Triggered Mode
        └── Want first frame auto-sent? → autoAdvanceOnConnect: true
```

### Performance Characteristics

| Mode | CPU Usage | Memory | Network |
|------|-----------|--------|---------|
| Pure | Low | Low | Steady stream |
| Synchronized | Low (waiting) | Low | Bursty |
| Triggered | Minimal | Low | On-demand |

## Common Patterns

### Speed Testing

Test how client handles fast message streams:

```bash
# 10x speed
curl -X POST http://localhost:4290/stream-recordings/REC_ID/replay \
  -d '{"mode": "pure", "timingScale": 0.1}'
```

### Protocol Verification

Ensure client follows correct message sequence:

```bash
curl -X POST http://localhost:4290/stream-recordings/REC_ID/replay \
  -d '{"mode": "synchronized", "strictMatching": true, "timeout": 5000}'
```

### Debugging Session

Step through messages one at a time:

```bash
# Start triggered replay
REPLAY_ID=$(curl -s -X POST http://localhost:4290/stream-recordings/REC_ID/replay \
  -d '{"mode": "triggered"}' | jq -r '.id')

# Step through
while true; do
  result=$(curl -s -X POST "http://localhost:4290/replay/$REPLAY_ID/advance")
  echo "$result" | jq
  
  if [ "$(echo $result | jq -r '.complete')" = "true" ]; then
    break
  fi
  
  read -p "Press enter to advance..."
done
```

## Next Steps

- [Stream Recording](/guides/stream-recording/) - Recording WebSocket and SSE streams
- [SSE Streaming](/guides/sse-streaming/) - SSE mock configuration
- [Admin API](/reference/admin-api/) - Stream recording API endpoints
