# Admin API Reference

The Admin API provides runtime management of the mockd server.

## Overview

mockd uses a **three-port architecture** separating the control plane from the data plane:

| Port | Default | Purpose |
|------|---------|---------|
| **Mock Server** | `4280` | Data plane - serves your mock endpoints |
| **Admin API** | `4290` | Control plane - management and configuration |
| **Engine Control** | `4281` | Internal - Admin-to-Engine communication |

```bash
mockd start --port 4280 --admin-port 4290
# Mocks at:  http://localhost:4280/api/users
# Admin at:  http://localhost:4290/mocks
```

The Engine Control port (`4281`) is used internally for communication between the Admin API and the mock engine. In most cases, you don't need to interact with it directly.

Base URL: `http://localhost:4290` (or your configured `--admin-port`)

## Authentication

The admin API has no authentication by default. In production, consider:

- Binding to localhost only (`--host localhost`)
- Using a firewall
- Running admin port on internal network only

---

## Endpoints

### Health

#### GET /health

Check server health.

**Response:**

```json
{
  "status": "ok",
  "uptime": 3600
}
```

---

### Ports

#### GET /ports

List all ports in use by mockd, grouped by component.

**Response:**

```json
{
  "ports": [
    {
      "port": 4290,
      "protocol": "HTTP",
      "component": "Admin API",
      "status": "running"
    },
    {
      "port": 4280,
      "protocol": "HTTP",
      "component": "Mock Engine",
      "status": "running"
    },
    {
      "port": 1883,
      "protocol": "MQTT",
      "component": "MQTT Broker",
      "status": "running"
    },
    {
      "port": 50051,
      "protocol": "gRPC",
      "component": "gRPC Server",
      "status": "running"
    }
  ]
}
```

The response includes ports for:
- Admin API (HTTP)
- Mock Engine (HTTP/HTTPS)
- Protocol handlers (gRPC, MQTT, WebSocket, SSE, GraphQL, SOAP)

**Note:** Ports with TLS enabled will include `"tls": true` in the response.

---

### Mock Management

#### GET /mocks

List all configured mocks.

**Response:**

```json
{
  "mocks": [
    {
      "id": "abc123",
      "name": "Get users",
      "matcher": {
        "method": "GET",
        "path": "/api/users"
      },
      "response": {
        "statusCode": 200,
        "body": "[]"
      },
      "enabled": true,
      "priority": 0,
      "createdAt": "2024-01-15T10:30:00Z",
      "updatedAt": "2024-01-15T10:30:00Z"
    }
  ],
  "count": 1
}
```

#### GET /mocks/{id}

Get a specific mock by ID.

#### POST /mocks

Add a new mock at runtime.

**Request:**

```json
{
  "name": "Get users",
  "matcher": {
    "method": "GET",
    "path": "/api/users"
  },
  "response": {
    "statusCode": 200,
    "headers": {"Content-Type": "application/json"},
    "body": "[{\"id\": 1, \"name\": \"Alice\"}]"
  }
}
```

**Response:** Returns the created mock with generated ID (HTTP 201).

**Port Sharing for gRPC and MQTT:**

When creating a gRPC or MQTT mock on a port that's already in use by another mock of the **same protocol** in the **same workspace**, the new services/topics are **merged** into the existing mock instead of creating a new one. This mirrors real-world behavior where a single gRPC server serves multiple services and a single MQTT broker handles multiple topics.

**Merge Response (HTTP 200):**

```json
{
  "action": "merged",
  "message": "Merged into existing gRPC server on port 50051",
  "targetMockId": "grpc_abc123",
  "addedServices": ["myapp.HealthService/Check"],
  "totalServices": ["myapp.UserService/GetUser", "myapp.HealthService/Check"],
  "mock": { ... }
}
```

**Conflict cases (HTTP 409):**
- Different protocols on the same port (e.g., gRPC on an MQTT port)
- Service/method already exists (gRPC) or topic already exists (MQTT)
- Different workspaces trying to use the same port

#### PUT /mocks/{id}

Update an existing mock.

#### DELETE /mocks/{id}

Remove a mock.

#### POST /mocks/{id}/toggle

Toggle a mock's enabled state.

---

### Mock Verification

#### GET /mocks/{id}/verify

Get verification status for a mock (call count, timestamps).

**Response:**

```json
{
  "mockId": "abc123",
  "callCount": 5,
  "firstCalledAt": "2024-01-15T10:30:00Z",
  "lastCalledAt": "2024-01-15T10:35:00Z"
}
```

#### POST /mocks/{id}/verify

Verify mock was called expected number of times.

**Request:**

```json
{
  "atLeast": 1,
  "atMost": 10,
  "exactly": null
}
```

**Response:**

```json
{
  "success": true,
  "message": "Mock called 5 times, expected at least 1"
}
```

#### GET /mocks/{id}/invocations

List all invocations of a mock.

**Response:**

```json
{
  "invocations": [
    {
      "timestamp": "2024-01-15T10:30:00Z",
      "method": "GET",
      "path": "/api/users",
      "headers": {"User-Agent": "curl/7.68.0"},
      "body": ""
    }
  ],
  "count": 5
}
```

#### DELETE /mocks/{id}/invocations

Reset verification data for a specific mock.

#### DELETE /verify

Reset all verification data for all mocks.

---

### State Management (Stateful Resources)

#### GET /state

Get stateful resource overview.

**Response:**

```json
{
  "resources": 2,
  "totalItems": 15,
  "resourceList": [
    {"name": "users", "itemCount": 10},
    {"name": "posts", "itemCount": 5}
  ]
}
```

#### POST /state/reset

Reset all stateful resources to seed data.

#### GET /state/resources

List all stateful resources.

#### GET /state/resources/{name}

Get all items in a specific resource.

#### DELETE /state/resources/{name}

Clear a specific resource (reset to seed data).

---

### Request History

#### GET /requests

Get recent request history.

**Query Parameters:**

| Parameter | Description | Default |
|-----------|-------------|---------|
| `limit` | Max requests to return | `100` |
| `offset` | Pagination offset | `0` |
| `path` | Filter by path pattern | |
| `method` | Filter by HTTP method | |
| `matched` | Filter by matched mock ID | |

**Response:**

```json
{
  "requests": [
    {
      "id": "req-123",
      "timestamp": "2024-01-15T10:30:00Z",
      "method": "GET",
      "path": "/api/users",
      "matchedMockID": "abc123",
      "responseStatus": 200,
      "durationMs": 5
    }
  ],
  "total": 150
}
```

#### GET /requests/{id}

Get details of a specific request including headers, body, and response.

#### DELETE /requests

Clear request history.

---

### Proxy Management

#### GET /proxy/status

Get current proxy status.

**Response:**

```json
{
  "running": true,
  "port": 8888,
  "mode": "record",
  "sessionId": "session-123"
}
```

#### POST /proxy/start

Start the MITM proxy.

**Request:**

```json
{
  "port": 8888,
  "mode": "record",
  "sessionName": "my-session"
}
```

#### POST /proxy/stop

Stop the proxy.

#### PUT /proxy/mode

Change proxy mode.

**Request:**

```json
{
  "mode": "passthrough"
}
```

Modes: `record`, `passthrough`, `playback`

#### GET /proxy/filters

Get current recording filters.

#### PUT /proxy/filters

Update recording filters.

---

### CA Certificate (HTTPS Interception)

#### GET /proxy/ca

Check if CA certificate exists.

**Response:**

```json
{
  "exists": true,
  "path": "/path/to/ca.crt",
  "fingerprint": "AB:CD:EF:...",
  "expiresAt": "2034-01-15T00:00:00Z",
  "organization": "mockd Local CA"
}
```

#### POST /proxy/ca

Generate a new CA certificate.

**Request:**

```json
{
  "caPath": "/path/to/store/ca"
}
```

#### GET /proxy/ca/download

Download the CA certificate (PEM format).

---

### Recording Sessions

#### GET /sessions

List all recording sessions.

#### POST /sessions

Create a new recording session.

#### GET /sessions/{id}

Get session details.

#### DELETE /sessions/{id}

Delete a session.

---

### Recordings

#### GET /recordings

List all HTTP recordings.

**Query Parameters:**

| Parameter | Description |
|-----------|-------------|
| `sessionId` | Filter by session |
| `method` | Filter by HTTP method |
| `path` | Filter by path pattern |

#### GET /recordings/{id}

Get a specific recording.

#### DELETE /recordings/{id}

Delete a recording.

#### DELETE /recordings

Clear all recordings.

#### POST /recordings/convert

Convert recordings to mocks.

**Request:**

```json
{
  "sessionId": "session-123",
  "deduplicate": true,
  "includeHeaders": false
}
```

**Response:**

```json
{
  "mockIds": ["mock-1", "mock-2"],
  "count": 2
}
```

#### POST /recordings/export

Export recordings to JSON.

**Request:**

```json
{
  "format": "json",
  "sessionId": "session-123"
}
```

---

### Configuration

#### GET /config

Export current mock configuration.

#### POST /config

Import mock configuration.

**Request:**

```json
{
  "config": {
    "version": "1.0",
    "mocks": [...]
  },
  "replace": false
}
```

---

### SSE Management

#### GET /sse/connections

List active SSE connections.

#### GET /sse/connections/{id}

Get SSE connection details.

#### DELETE /sse/connections/{id}

Close an SSE connection.

#### GET /sse/stats

Get SSE statistics.

---

### WebSocket Management

#### GET /admin/ws/connections

List active WebSocket connections.

#### GET /admin/ws/connections/{id}

Get connection details.

#### DELETE /admin/ws/connections/{id}

Close a WebSocket connection.

#### POST /admin/ws/connections/{id}/send

Send a message to a specific connection.

**Request:**

```json
{
  "type": "text",
  "data": "Hello from server"
}
```

#### POST /admin/ws/broadcast

Broadcast message to all connections.

#### GET /admin/ws/endpoints

List configured WebSocket endpoints.

#### GET /admin/ws/stats

Get WebSocket statistics.

---

### Stream Recordings (WebSocket/SSE)

#### GET /stream-recordings

List stream recordings.

#### GET /stream-recordings/{id}

Get stream recording details.

#### DELETE /stream-recordings/{id}

Delete a stream recording.

#### POST /stream-recordings/{id}/export

Export stream recording.

#### POST /stream-recordings/{id}/convert

Convert stream recording to mock.

#### POST /stream-recordings/{id}/replay

Start replaying a stream recording.

#### GET /replay

List active replay sessions.

#### GET /replay/{id}

Get replay session status.

#### DELETE /replay/{id}

Stop a replay session.

---

### gRPC Recording

#### GET /grpc

List all registered gRPC servers.

**Response:**

```json
{
  "servers": [
    {
      "id": "grpc-server-1",
      "address": ":50051",
      "running": true,
      "recordingEnabled": false
    }
  ],
  "count": 1
}
```

#### GET /grpc/{id}/status

Get gRPC server status.

#### POST /grpc/{id}/record/start

Start recording gRPC calls on a server.

**Response:**

```json
{
  "message": "Recording enabled for gRPC server 'grpc-server-1'",
  "enabled": true
}
```

#### POST /grpc/{id}/record/stop

Stop recording gRPC calls.

#### GET /grpc-recordings

List gRPC recordings.

**Query Parameters:**

| Parameter | Description |
|-----------|-------------|
| `service` | Filter by service name |
| `method` | Filter by method name |
| `streamType` | Filter by stream type (unary, server_stream, client_stream, bidi) |
| `hasError` | Filter by error presence (true/false) |
| `limit` | Max recordings to return |
| `offset` | Pagination offset |

#### GET /grpc-recordings/{id}

Get a specific gRPC recording.

#### DELETE /grpc-recordings/{id}

Delete a gRPC recording.

#### DELETE /grpc-recordings

Clear all gRPC recordings.

#### GET /grpc-recordings/stats

Get gRPC recording statistics.

#### POST /grpc-recordings/convert

Convert gRPC recordings to mock config.

**Request:**

```json
{
  "recordingIds": ["grpc-abc123"],
  "service": "mypackage.MyService",
  "includeMetadata": true,
  "includeDelay": false,
  "deduplicate": true
}
```

#### POST /grpc-recordings/{id}/convert

Convert a single gRPC recording to mock config.

#### POST /grpc-recordings/export

Export all gRPC recordings as JSON.

---

### MQTT Recording

#### GET /mqtt

List all registered MQTT brokers.

**Response:**

```json
{
  "brokers": [
    {
      "id": "mqtt-broker-1",
      "port": 1883,
      "running": true,
      "recordingEnabled": false
    }
  ],
  "count": 1
}
```

#### GET /mqtt/{id}/status

Get MQTT broker status.

#### POST /mqtt/{id}/record/start

Start recording MQTT messages.

#### POST /mqtt/{id}/record/stop

Stop recording MQTT messages.

#### GET /mqtt-recordings

List MQTT recordings.

**Query Parameters:**

| Parameter | Description |
|-----------|-------------|
| `topicPattern` | Filter by topic (supports MQTT wildcards + and #) |
| `clientId` | Filter by client ID |
| `direction` | Filter by direction (publish/subscribe) |
| `limit` | Max recordings to return |
| `offset` | Pagination offset |

#### GET /mqtt-recordings/{id}

Get a specific MQTT recording.

#### DELETE /mqtt-recordings/{id}

Delete an MQTT recording.

#### DELETE /mqtt-recordings

Clear all MQTT recordings.

#### GET /mqtt-recordings/stats

Get MQTT recording statistics.

#### POST /mqtt-recordings/convert

Convert MQTT recordings to mock config.

**Request:**

```json
{
  "recordingIds": ["mqtt-abc123"],
  "topicPattern": "sensors/#",
  "deduplicate": true,
  "includeQoS": true,
  "includeRetain": true
}
```

#### POST /mqtt-recordings/{id}/convert

Convert a single MQTT recording to mock config.

#### POST /mqtt-recordings/export

Export all MQTT recordings as JSON.

---

### SOAP Recording

#### GET /soap

List all registered SOAP handlers.

**Response:**

```json
{
  "handlers": [
    {
      "id": "soap-handler-1",
      "path": "/soap/service",
      "recordingEnabled": false
    }
  ],
  "count": 1
}
```

#### GET /soap/{id}/status

Get SOAP handler status.

#### POST /soap/{id}/record/start

Start recording SOAP requests.

#### POST /soap/{id}/record/stop

Stop recording SOAP requests.

#### GET /soap-recordings

List SOAP recordings.

**Query Parameters:**

| Parameter | Description |
|-----------|-------------|
| `endpoint` | Filter by endpoint path |
| `operation` | Filter by operation name |
| `soapAction` | Filter by SOAPAction header |
| `hasFault` | Filter by fault presence (true/false) |
| `limit` | Max recordings to return |
| `offset` | Pagination offset |

#### GET /soap-recordings/{id}

Get a specific SOAP recording.

#### DELETE /soap-recordings/{id}

Delete a SOAP recording.

#### DELETE /soap-recordings

Clear all SOAP recordings.

#### GET /soap-recordings/stats

Get SOAP recording statistics.

#### POST /soap-recordings/convert

Convert SOAP recordings to mock config.

**Request:**

```json
{
  "recordingIds": ["soap-abc123"],
  "endpoint": "/soap/service",
  "operation": "GetUser",
  "deduplicate": true,
  "includeDelay": false,
  "preserveFaults": true
}
```

#### POST /soap-recordings/{id}/convert

Convert a single SOAP recording to mock config.

#### POST /soap-recordings/export

Export all SOAP recordings as JSON.

---

### Chaos Injection

#### GET /chaos

Get current chaos configuration.

**Response:**

```json
{
  "enabled": true,
  "latency": {
    "enabled": true,
    "minMs": 100,
    "maxMs": 500
  },
  "errorRate": {
    "enabled": true,
    "percentage": 10,
    "statusCode": 500
  }
}
```

#### PUT /chaos

Update chaos configuration.

**Request:**

```json
{
  "enabled": true,
  "latency": {
    "enabled": true,
    "minMs": 50,
    "maxMs": 200
  }
}
```

---

## Error Responses

All errors return a consistent format:

```json
{
  "error": "error_code",
  "message": "Human readable message"
}
```

### Common Error Codes

| Code | HTTP Status | Description |
|------|-------------|-------------|
| `not_found` | 404 | Resource not found |
| `invalid_json` | 400 | Invalid JSON in request |
| `validation_error` | 400 | Validation failed |
| `missing_field` | 400 | Required field missing |

---

## Examples

### Reset State Before Tests

```bash
curl -X POST http://localhost:4290/state/reset
```

### Add Mock at Runtime

```bash
curl -X POST http://localhost:4290/mocks \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Test endpoint",
    "matcher": {"method": "GET", "path": "/api/test"},
    "response": {"statusCode": 200, "body": "{\"test\": true}"}
  }'
```

### Check Request History

```bash
curl "http://localhost:4290/requests?limit=10&path=/api/users"
```

### Start Proxy Recording

```bash
curl -X POST http://localhost:4290/proxy/start \
  -H "Content-Type: application/json" \
  -d '{"port": 8888, "mode": "record", "sessionName": "test-session"}'
```

### Convert Recordings to Mocks

```bash
curl -X POST http://localhost:4290/recordings/convert \
  -H "Content-Type: application/json" \
  -d '{"deduplicate": true}'
```

## See Also

- [CLI Reference](cli.md) - Command-line options
- [Configuration Reference](configuration.md) - Config file format
- [Stateful Mocking](../guides/stateful-mocking.md) - State management
