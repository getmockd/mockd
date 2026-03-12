---
title: Core Concepts
description: Understanding the fundamental concepts of mockd
---

Understanding the fundamental concepts of mockd will help you create effective mocks for any scenario.

## Multi-Protocol Architecture

mockd is not just an HTTP mock server. It supports **seven protocols** from a single binary and configuration file:

| Protocol | Use Case | Default Port |
|----------|----------|-------------|
| **HTTP/HTTPS** | REST APIs, webhooks, file downloads | 4280 |
| **GraphQL** | GraphQL queries, mutations, subscriptions | 4280 (path-based) |
| **gRPC** | Protobuf-based RPC services | 50051 |
| **WebSocket** | Real-time bidirectional communication | 4280 (path-based) |
| **MQTT** | IoT devices, sensor networks, pub/sub | 1883 |
| **SOAP/WSDL** | Enterprise XML web services | 4280 (path-based) |
| **SSE** | Server-Sent Events, AI streaming | 4280 (path-based) |

HTTP, GraphQL, WebSocket, SOAP, and SSE share the HTTP port (4280) and are differentiated by path or content type. gRPC and MQTT run on their own ports.

All protocols are configured in the same YAML/JSON file using the `type` field:

```yaml
mocks:
  - id: rest-api
    type: http          # HTTP mock
    http: { ... }

  - id: graphql-api
    type: graphql       # GraphQL mock
    graphql: { ... }

  - id: grpc-service
    type: grpc          # gRPC mock
    grpc: { ... }

  - id: ws-endpoint
    type: websocket     # WebSocket mock
    websocket: { ... }

  - id: mqtt-broker
    type: mqtt          # MQTT mock
    mqtt: { ... }

  - id: soap-service
    type: soap          # SOAP mock
    soap: { ... }
```

## What is a Mock?

A **mock** is a rule that defines:

1. **Request Matcher** - Which incoming requests to match
2. **Response** - What to send back when matched

```yaml
- type: http
  http:
    matcher: { ... }   # Which requests to match
    response: { ... }  # What to send back
```

Each mock has a `type` (http, graphql, grpc, websocket, mqtt, soap, oauth) and a protocol-specific block. For HTTP mocks, when a request arrives, mockd checks each mock's matcher. The first match wins and its response is returned. Other protocols use protocol-specific matching (GraphQL operations, gRPC methods, MQTT topics, etc.).

## Request Matching

The request matcher defines criteria for matching incoming requests:

```json
{
  "matcher": {
    "method": "GET",
    "path": "/api/users",
    "headers": {
      "Authorization": "Bearer*"
    },
    "queryParams": {
      "page": "1"
    }
  }
}
```

### Matching Fields

| Field | Description | Matching Type |
|-------|-------------|---------------|
| `method` | HTTP method (GET, POST, etc.) | Exact match |
| `path` | URL path | Exact or `{param}` pattern |
| `pathPattern` | URL path regex | Full regex |
| `headers` | HTTP headers | Exact or glob (`*`) |
| `queryParams` | Query string parameters | Exact match |
| `bodyContains` | Request body substring | Substring match |

### Path Patterns

Paths can include dynamic segments:

```json
"/api/users/{id}"           // Matches /api/users/1, /api/users/abc
"/api/{resource}/{id}"      // Matches /api/posts/123
```

For full regex matching, use `pathPattern` instead of `path`:

```json
{
  "pathPattern": "/api/v[0-9]+/users/.*"
}
```

### Glob Matching

Headers support glob patterns with `*`:

```json
{
  "headers": {
    "Authorization": "Bearer*",
    "Content-Type": "*json*"
  }
}
```

Patterns: `prefix*` (starts with), `*suffix` (ends with), `*middle*` (contains).

See [Request Matching Guide](/guides/request-matching/) for complete details.

## Response Definition

The response defines what mockd sends back:

```json
{
  "response": {
    "statusCode": 200,
    "headers": {
      "Content-Type": "application/json"
    },
    "body": {
      "message": "Success"
    },
    "delayMs": 100
  }
}
```

### Response Fields

| Field | Description | Default |
|-------|-------------|---------|
| `statusCode` | HTTP status code | 200 |
| `headers` | Response headers | `{}` |
| `body` | Response body (string or JSON) | `""` |
| `delayMs` | Simulated latency (milliseconds) | `0` |
| `seed` | Deterministic seed for faker/random output | `0` (random) |

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

See [Response Templating Guide](/guides/response-templating/) for more.

## Mock Priority

When multiple mocks could match a request, mockd uses this priority:

1. **More specific paths win** - `/api/users/1` beats `/api/users/{id}`
2. **More matchers win** - Path + headers beats path only
3. **Order in config** - Earlier mocks win if priority is equal

## Configuration File

A complete configuration file:

```yaml
version: "1.0"
mocks:
  - id: list-users
    name: List users
    type: http
    http:
      matcher:
        method: GET
        path: /api/users
      response:
        statusCode: 200
        body: '{"users": []}'
```

### Top-Level Fields

| Field | Description | Required |
|-------|-------------|----------|
| `version` | Config version (`"1.0"`) | Yes |
| `mocks` | Array of mock definitions | Yes |
| `tables` | Named data stores (pure data, no routing) | No |
| `extend` | Bindings from mocks to tables | No |
| `imports` | External spec imports with namespacing | No |
| `statefulResources` | Low-level CRUD resources (prefer `tables` + `extend`) | No |

## Stateful Mocking

mockd can simulate stateful CRUD APIs where:

- POST creates resources
- GET retrieves resources
- PUT/PATCH updates resources
- DELETE removes resources

State persists across requests during the server session.

The stateful architecture uses two concepts:

- **Tables** — Pure data stores that hold seed data. No routing is attached.
- **Extend bindings** — Wire mock endpoints to tables with a specific action (list, get, create, update, delete, custom).

```yaml
tables:
  - name: users
    idField: id
    seedData: []

mocks:
  - id: list-users
    type: http
    http:
      matcher: { method: GET, path: /api/users }
      response: { statusCode: 200 }

extend:
  - mock: list-users
    table: users
    action: list
```

For quick prototyping, the CLI shortcut `mockd http add --path /api/users --stateful` creates a table + mocks + extend bindings in one step.

See [Stateful Mocking Guide](/guides/stateful-mocking/).

## Workspaces

Workspaces provide isolated environments within a single mockd instance. Each workspace has its own:
- **Mocks** — route definitions scoped to the workspace
- **Stateful resources** — independent data stores per workspace  
- **Request logs** — traffic filtered by workspace

The default workspace (empty string) contains everything not assigned to a specific workspace. Use `mockd workspace create` to create isolated environments, or pass `--workspace <id>` to scope any CLI command.

Workspaces are useful for:
- Running multiple API environments side-by-side (e.g., Stripe + Twilio)
- Test isolation (each test suite gets its own workspace)
- Team separation on shared instances

## Proxy Recording

mockd can act as a proxy to record real API traffic:

```bash
mockd proxy start
```

Recorded requests become mocks automatically.

See [Proxy Recording Guide](/guides/proxy-recording/).

## Ways to Interact with mockd

mockd provides several interfaces for managing mocks:

- **CLI** — The `mockd` command-line tool for creating, listing, and managing mocks from your terminal. See the [CLI Reference](/reference/cli/).
- **Admin API** — A RESTful HTTP API on port 4290 for runtime mock management, state control, and proxy operations. See the [Admin API Reference](/reference/admin-api/).
- **MCP Server** — A Model Context Protocol server that lets AI-powered editors (Claude Code, Cursor, Windsurf) create and manage mocks directly. See the [MCP Server Guide](/guides/mcp-server/).
- **Web Dashboard** — A built-in UI served from the admin port (http://localhost:4290) for visual mock management. Available in release builds. See the [Dashboard Guide](/guides/dashboard/).

## Next Steps

- **[Request Matching](/guides/request-matching/)** - Advanced matching techniques
- **[Response Templating](/guides/response-templating/)** - Dynamic responses
- **[Protocol Guides](/protocols/graphql/)** - GraphQL, gRPC, WebSocket, MQTT, SOAP, SSE
- **[CLI Reference](/reference/cli/)** - Command-line options
- **[Configuration Reference](/reference/configuration/)** - Full config schema
