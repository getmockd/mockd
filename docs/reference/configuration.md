# Configuration Reference

Complete reference for mockd configuration files.

## File Format

mockd supports JSON and YAML configuration files:

```bash
mockd start --config mocks.json
mockd start --config mocks.yaml
```

## Top-Level Structure

```json
{
  "server": { ... },
  "mocks": [ ... ],
  "stateful": { ... },
  "proxy": { ... }
}
```

| Field | Type | Description |
|-------|------|-------------|
| `server` | object | Server configuration |
| `mocks` | array | Mock definitions |
| `stateful` | object | Stateful resource configuration |
| `proxy` | object | Proxy configuration |

---

## Server Configuration

```json
{
  "server": {
    "port": 8080,
    "host": "localhost",
    "adminEnabled": true,
    "adminPort": 8081,
    "tls": {
      "enabled": false,
      "port": 8443,
      "certFile": "./certs/server.crt",
      "keyFile": "./certs/server.key",
      "minVersion": "1.2"
    },
    "cors": {
      "enabled": true,
      "origins": ["*"],
      "methods": ["GET", "POST", "PUT", "DELETE"],
      "headers": ["Content-Type", "Authorization"]
    },
    "logging": {
      "level": "info",
      "format": "text",
      "requestLogging": true
    }
  }
}
```

### Server Fields

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `port` | integer | `8080` | Main server port |
| `host` | string | `localhost` | Bind address |
| `adminEnabled` | boolean | `true` | Enable admin API |
| `adminPort` | integer | `8081` | Admin API port |

### TLS Configuration

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `enabled` | boolean | `false` | Enable HTTPS |
| `port` | integer | `8443` | HTTPS port (if different from main) |
| `certFile` | string | | Path to certificate file |
| `keyFile` | string | | Path to private key file |
| `minVersion` | string | `"1.2"` | Minimum TLS version |
| `clientAuth` | string | `"none"` | Client auth mode |
| `clientCAs` | array | `[]` | Client CA certificate files |

### CORS Configuration

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `enabled` | boolean | `true` | Enable CORS |
| `origins` | array | `["*"]` | Allowed origins |
| `methods` | array | All | Allowed methods |
| `headers` | array | Common | Allowed headers |
| `credentials` | boolean | `false` | Allow credentials |

---

## Mock Definition

```json
{
  "mocks": [
    {
      "name": "Get users",
      "description": "Returns list of users",
      "priority": 0,
      "request": {
        "method": "GET",
        "path": "/api/users",
        "headers": {},
        "query": {},
        "body": null
      },
      "response": {
        "status": 200,
        "headers": {
          "Content-Type": "application/json"
        },
        "body": { "users": [] },
        "delay": "0ms"
      }
    }
  ]
}
```

### Mock Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `name` | string | No | Human-readable name |
| `description` | string | No | Description of mock |
| `priority` | integer | No | Match priority (lower = higher) |
| `request` | object | Yes | Request matcher |
| `response` | object | Yes | Response definition |

### Request Matcher

| Field | Type | Description |
|-------|------|-------------|
| `method` | string | HTTP method (GET, POST, etc.) |
| `path` | string | URL path (exact or pattern) |
| `headers` | object | Header matchers (key: regex) |
| `query` | object | Query param matchers (key: regex) |
| `body` | any | Exact body match (JSON) |
| `bodyContains` | object | Partial body match |
| `bodyMatch` | object | JSONPath matchers |
| `bodyString` | string | Raw body regex |

### Response Definition

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `status` | integer | `200` | HTTP status code |
| `headers` | object | `{}` | Response headers |
| `body` | any | `""` | Response body |
| `bodyFile` | string | | Load body from file |
| `delay` | string | `"0ms"` | Response delay |

### Path Patterns

```json
"/api/users"              // Exact match
"/api/users/{id}"         // Path parameter
"/api/{resource}/{id}"    // Multiple parameters
"/api/files/{path:.*}"    // Greedy parameter
```

### Body Types

```json
// JSON object
"body": {"users": []}

// String
"body": "<html>Hello</html>"

// File reference
"bodyFile": "./responses/users.json"

// Template
"body": {"id": "{{request.pathParams.id}}"}
```

---

## Stateful Configuration

```json
{
  "stateful": {
    "enabled": true,
    "resources": {
      "users": {
        "collection": "/api/users",
        "item": "/api/users/{id}",
        "idField": "id",
        "autoId": true,
        "seed": [],
        "seedFile": null,
        "filtering": false,
        "pagination": null
      }
    },
    "persistence": {
      "enabled": false,
      "file": "./mockd-state.json",
      "saveInterval": "30s"
    }
  }
}
```

### Resource Configuration

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `collection` | string | Required | Collection endpoint path |
| `item` | string | Required | Item endpoint path |
| `idField` | string | `"id"` | ID field name |
| `autoId` | boolean | `true` | Auto-generate IDs |
| `seed` | array | `[]` | Initial data |
| `seedFile` | string | | Load seed from file |
| `filtering` | boolean | `false` | Enable query filtering |
| `pagination` | object | `null` | Pagination settings |
| `parentRef` | string | | Parent resource field |

### Pagination Configuration

```json
{
  "pagination": {
    "enabled": true,
    "pageParam": "page",
    "limitParam": "limit",
    "defaultLimit": 20,
    "maxLimit": 100
  }
}
```

### Persistence Configuration

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `enabled` | boolean | `false` | Enable persistence |
| `file` | string | `"./mockd-state.json"` | State file path |
| `saveInterval` | string | `"30s"` | Auto-save interval |

---

## Proxy Configuration

```json
{
  "proxy": {
    "target": "https://api.example.com",
    "port": 8080,
    "record": false,
    "playback": false,
    "recordingsDir": "./recordings",
    "recordFormat": "json",
    "stripHeaders": [],
    "rewriteHost": true,
    "tls": {
      "caCertFile": null,
      "caKeyFile": null,
      "certCacheDir": "./certs/generated"
    },
    "recordFilter": {
      "includePaths": [],
      "excludePaths": [],
      "methods": [],
      "statusCodes": []
    },
    "redact": {
      "bodyFields": [],
      "replacement": "[REDACTED]"
    }
  }
}
```

### Proxy Fields

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `target` | string | Required | Upstream API URL |
| `port` | integer | `8080` | Proxy port |
| `record` | boolean | `false` | Enable recording |
| `playback` | boolean | `false` | Enable playback |
| `recordingsDir` | string | `"./recordings"` | Recordings path |
| `recordFormat` | string | `"json"` | Format (json, yaml) |
| `stripHeaders` | array | `[]` | Headers to strip |
| `rewriteHost` | boolean | `true` | Rewrite Host header |

---

## Template Variables

Available in response bodies and headers:

| Variable | Description |
|----------|-------------|
| `{{request.method}}` | HTTP method |
| `{{request.path}}` | Request path |
| `{{request.url}}` | Full URL |
| `{{request.pathParams.name}}` | Path parameter |
| `{{request.query.name}}` | Query parameter |
| `{{request.headers.Name}}` | Request header |
| `{{request.body}}` | Full request body |
| `{{request.body.field}}` | Body field |
| `{{now}}` | ISO timestamp |
| `{{now.unix}}` | Unix timestamp |
| `{{now.unixMilli}}` | Unix millis |
| `{{uuid}}` | Random UUID |
| `{{uuid.short}}` | Short ID |
| `{{random.int min max}}` | Random integer |
| `{{random.float min max}}` | Random float |

---

## Complete Example

```json
{
  "server": {
    "port": 8080,
    "host": "localhost",
    "cors": {
      "enabled": true,
      "origins": ["http://localhost:3000"]
    }
  },
  "mocks": [
    {
      "name": "Health check",
      "request": {
        "method": "GET",
        "path": "/health"
      },
      "response": {
        "status": 200,
        "body": {"status": "ok"}
      }
    },
    {
      "name": "Get user by ID",
      "request": {
        "method": "GET",
        "path": "/api/users/{id}"
      },
      "response": {
        "status": 200,
        "body": {
          "id": "{{request.pathParams.id}}",
          "name": "User {{request.pathParams.id}}"
        }
      }
    }
  ],
  "stateful": {
    "resources": {
      "posts": {
        "collection": "/api/posts",
        "item": "/api/posts/{id}",
        "seed": [
          {"id": 1, "title": "First Post"}
        ]
      }
    }
  }
}
```

## See Also

- [CLI Reference](cli.md) - Command-line options
- [JSON Schema Reference](json-schema.md) - Validation schema
- [Request Matching](../guides/request-matching.md) - Matching patterns
