# Admin API Reference

The admin API provides runtime management of the mockd server. By default, it runs on port 8081.

## Overview

Base URL: `http://localhost:8081/__admin`

All endpoints are prefixed with `/__admin` to avoid conflicts with mocked routes.

## Authentication

The admin API has no authentication by default. In production, consider:

- Binding to localhost only (`--host localhost`)
- Using a firewall
- Enabling authentication (see configuration)

---

## Endpoints

### Health & Status

#### GET /__admin/health

Check server health.

**Response:**

```json
{
  "status": "healthy",
  "uptime": "2h15m30s",
  "version": "0.1.0"
}
```

#### GET /__admin/status

Get detailed server status.

**Response:**

```json
{
  "status": "running",
  "uptime": "2h15m30s",
  "version": "0.1.0",
  "config": {
    "port": 8080,
    "mocksCount": 15,
    "statefulResources": ["users", "posts"]
  },
  "stats": {
    "requestsTotal": 1250,
    "requestsMatched": 1200,
    "requestsUnmatched": 50,
    "avgResponseTime": "5ms"
  }
}
```

---

### Mock Management

#### GET /__admin/mocks

List all configured mocks.

**Response:**

```json
{
  "mocks": [
    {
      "id": "mock-1",
      "name": "Get users",
      "request": {
        "method": "GET",
        "path": "/api/users"
      },
      "matchCount": 45
    }
  ]
}
```

#### GET /__admin/mocks/:id

Get a specific mock by ID.

**Response:**

```json
{
  "id": "mock-1",
  "name": "Get users",
  "request": {
    "method": "GET",
    "path": "/api/users"
  },
  "response": {
    "status": 200,
    "body": {"users": []}
  },
  "matchCount": 45
}
```

#### POST /__admin/mocks

Add a new mock at runtime.

**Request:**

```json
{
  "name": "New mock",
  "request": {
    "method": "GET",
    "path": "/api/new"
  },
  "response": {
    "status": 200,
    "body": {"message": "hello"}
  }
}
```

**Response:**

```json
{
  "id": "mock-16",
  "name": "New mock",
  "message": "Mock added successfully"
}
```

#### PUT /__admin/mocks/:id

Update an existing mock.

**Request:**

```json
{
  "response": {
    "status": 201,
    "body": {"message": "updated"}
  }
}
```

**Response:**

```json
{
  "id": "mock-16",
  "message": "Mock updated successfully"
}
```

#### DELETE /__admin/mocks/:id

Remove a mock.

**Response:**

```json
{
  "message": "Mock deleted successfully"
}
```

---

### State Management

#### GET /__admin/state

Get all stateful resource data.

**Response:**

```json
{
  "users": [
    {"id": 1, "name": "Alice"},
    {"id": 2, "name": "Bob"}
  ],
  "posts": [
    {"id": 1, "title": "First post", "authorId": 1}
  ]
}
```

#### GET /__admin/state/:resource

Get data for a specific resource.

**Response:**

```json
[
  {"id": 1, "name": "Alice"},
  {"id": 2, "name": "Bob"}
]
```

#### POST /__admin/state

Import state data.

**Request:**

```json
{
  "users": [
    {"id": 1, "name": "Charlie"},
    {"id": 2, "name": "Diana"}
  ]
}
```

**Response:**

```json
{
  "message": "State imported successfully",
  "resources": {
    "users": 2
  }
}
```

#### DELETE /__admin/state

Reset all state.

**Response:**

```json
{
  "message": "All state reset successfully"
}
```

#### DELETE /__admin/state/:resource

Reset specific resource state.

**Response:**

```json
{
  "message": "Resource 'users' reset successfully"
}
```

---

### Request History

#### GET /__admin/requests

Get recent request history.

**Query Parameters:**

| Parameter | Description | Default |
|-----------|-------------|---------|
| `limit` | Max requests to return | `100` |
| `path` | Filter by path pattern | |
| `method` | Filter by HTTP method | |
| `matched` | Filter by match status | |

**Response:**

```json
{
  "requests": [
    {
      "id": "req-123",
      "timestamp": "2024-01-15T10:30:00Z",
      "method": "GET",
      "path": "/api/users",
      "headers": {...},
      "matched": true,
      "mockId": "mock-1",
      "responseStatus": 200,
      "responseTime": "5ms"
    }
  ],
  "total": 150
}
```

#### GET /__admin/requests/:id

Get details of a specific request.

**Response:**

```json
{
  "id": "req-123",
  "timestamp": "2024-01-15T10:30:00Z",
  "request": {
    "method": "GET",
    "path": "/api/users",
    "headers": {
      "Accept": "application/json"
    },
    "query": {},
    "body": null
  },
  "matched": true,
  "mockId": "mock-1",
  "response": {
    "status": 200,
    "headers": {...},
    "body": {"users": []}
  },
  "responseTime": "5ms"
}
```

#### DELETE /__admin/requests

Clear request history.

**Response:**

```json
{
  "message": "Request history cleared"
}
```

---

### Recordings

#### GET /__admin/recordings

List recorded requests (proxy mode).

**Response:**

```json
{
  "recordings": [
    {
      "id": "rec-abc123",
      "filename": "GET_api_users_abc123.json",
      "method": "GET",
      "path": "/api/users",
      "recordedAt": "2024-01-15T10:30:00Z"
    }
  ]
}
```

#### DELETE /__admin/recordings/:id

Delete a recording.

**Response:**

```json
{
  "message": "Recording deleted successfully"
}
```

---

### Configuration

#### GET /__admin/config

Get current configuration.

**Response:**

```json
{
  "server": {
    "port": 8080,
    "host": "localhost"
  },
  "mocksCount": 15,
  "statefulEnabled": true
}
```

#### POST /__admin/config/reload

Reload configuration from file.

**Response:**

```json
{
  "message": "Configuration reloaded",
  "mocksCount": 16
}
```

---

### Control

#### POST /__admin/shutdown

Gracefully shutdown the server.

**Request:**

```json
{
  "timeout": "30s"
}
```

**Response:**

```json
{
  "message": "Shutdown initiated"
}
```

---

## Error Responses

All errors return a consistent format:

```json
{
  "error": "Error message here",
  "code": "ERROR_CODE",
  "details": {}
}
```

### Error Codes

| Code | HTTP Status | Description |
|------|-------------|-------------|
| `NOT_FOUND` | 404 | Resource not found |
| `BAD_REQUEST` | 400 | Invalid request |
| `VALIDATION_ERROR` | 422 | Validation failed |
| `INTERNAL_ERROR` | 500 | Server error |

---

## Examples

### Reset State Before Tests

```bash
curl -X DELETE http://localhost:8081/__admin/state
curl -X POST http://localhost:8081/__admin/state \
  -H "Content-Type: application/json" \
  -d '{"users": [{"id": 1, "name": "Test User"}]}'
```

### Add Mock at Runtime

```bash
curl -X POST http://localhost:8081/__admin/mocks \
  -H "Content-Type: application/json" \
  -d '{
    "request": {"method": "GET", "path": "/api/test"},
    "response": {"status": 200, "body": {"test": true}}
  }'
```

### Check Request History

```bash
curl "http://localhost:8081/__admin/requests?limit=10&path=/api/users"
```

### Reload Configuration

```bash
curl -X POST http://localhost:8081/__admin/config/reload
```

## See Also

- [CLI Reference](cli.md) - Command-line options
- [Configuration Reference](configuration.md) - Config file format
- [Stateful Mocking](../guides/stateful-mocking.md) - State management
