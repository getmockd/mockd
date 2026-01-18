# Core Concepts

Understanding the fundamental concepts of mockd will help you create effective mocks for any scenario.

## What is a Mock?

A **mock** is a rule that defines:

1. **Request Matcher** - Which incoming requests to match
2. **Response** - What to send back when matched

```json
{
  "request": { ... },   // Request matcher
  "response": { ... }   // Response definition
}
```

When an HTTP request arrives, mockd checks each mock's request matcher. The first match wins and its response is returned.

## Request Matching

The request matcher defines criteria for matching incoming requests:

```json
{
  "request": {
    "method": "GET",
    "path": "/api/users",
    "headers": {
      "Authorization": "Bearer .*"
    },
    "query": {
      "page": "1"
    }
  }
}
```

### Matching Fields

| Field | Description | Matching Type |
|-------|-------------|---------------|
| `method` | HTTP method (GET, POST, etc.) | Exact match |
| `path` | URL path | Exact or pattern |
| `headers` | HTTP headers | Exact or regex |
| `query` | Query string parameters | Exact or regex |
| `body` | Request body | JSON matching |

### Path Patterns

Paths can include dynamic segments:

```json
"/api/users/{id}"           // Matches /api/users/1, /api/users/abc
"/api/{resource}/{id}"      // Matches /api/posts/123
"/api/files/{path:.*}"      // Matches /api/files/a/b/c (greedy)
```

### Regex Matching

Headers and query params support regex:

```json
{
  "headers": {
    "Authorization": "Bearer [a-zA-Z0-9]+"
  }
}
```

See [Request Matching Guide](../guides/request-matching.md) for complete details.

## Response Definition

The response defines what mockd sends back:

```json
{
  "response": {
    "status": 200,
    "headers": {
      "Content-Type": "application/json"
    },
    "body": {
      "message": "Success"
    },
    "delay": "100ms"
  }
}
```

### Response Fields

| Field | Description | Default |
|-------|-------------|---------|
| `status` | HTTP status code | 200 |
| `headers` | Response headers | `{}` |
| `body` | Response body (string or JSON) | `""` |
| `delay` | Simulated latency | `0ms` |

### Body Types

The body can be:

- **JSON object/array** - Automatically serialized
- **String** - Sent as-is
- **File reference** - Load from file

```json
// JSON body
"body": {"users": []}

// String body
"body": "<html>Hello</html>"

// File reference
"body": "@./responses/users.json"
```

## Response Templating

Responses can use templates to include request data:

```json
{
  "response": {
    "body": {
      "received_id": "{{request.pathParam.id}}",
      "timestamp": "{{now}}"
    }
  }
}
```

### Available Variables

| Variable | Description |
|----------|-------------|
| `request.method` | HTTP method |
| `request.path` | Request path |
| `request.pathParam.{name}` | Path parameter value |
| `request.query.{name}` | Query parameter value |
| `request.header.{name}` | Header value |
| `request.body` | Parsed request body |
| `now` | Current timestamp |
| `uuid` | Random UUID |

See [Response Templating Guide](../guides/response-templating.md) for more.

## Mock Priority

When multiple mocks could match a request, mockd uses this priority:

1. **More specific paths win** - `/api/users/1` beats `/api/users/{id}`
2. **More matchers win** - Path + headers beats path only
3. **Order in config** - Earlier mocks win if priority is equal

## Configuration File

A complete configuration file:

```json
{
  "server": {
    "port": 4280,
    "host": "localhost"
  },
  "mocks": [
    {
      "name": "List users",
      "request": {
        "method": "GET",
        "path": "/api/users"
      },
      "response": {
        "status": 200,
        "body": {"users": []}
      }
    }
  ]
}
```

### Top-Level Fields

| Field | Description | Required |
|-------|-------------|----------|
| `server` | Server configuration | No |
| `mocks` | Array of mock definitions | Yes |

## Stateful Mocking

mockd can simulate stateful CRUD APIs where:

- POST creates resources
- GET retrieves resources
- PUT/PATCH updates resources
- DELETE removes resources

State persists across requests during the server session.

```json
{
  "stateful": {
    "resources": {
      "users": {
        "collection": "/api/users",
        "item": "/api/users/{id}"
      }
    }
  }
}
```

See [Stateful Mocking Guide](../guides/stateful-mocking.md).

## Proxy Recording

mockd can act as a proxy to record real API traffic:

```bash
mockd proxy --target https://api.example.com --record
```

Recorded requests become mocks automatically.

See [Proxy Recording Guide](../guides/proxy-recording.md).

## Next Steps

- **[Request Matching](../guides/request-matching.md)** - Advanced matching techniques
- **[Response Templating](../guides/response-templating.md)** - Dynamic responses
- **[CLI Reference](../reference/cli.md)** - Command-line options
- **[Configuration Reference](../reference/configuration.md)** - Full config schema
