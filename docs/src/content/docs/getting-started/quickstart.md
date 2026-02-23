---
title: Quickstart
description: Get your first mock API running in under 5 minutes
---

Get your first mock API running in under 5 minutes.

## Prerequisites

- mockd installed ([Installation Guide](/getting-started/installation/))
- A terminal
- curl or any HTTP client

## Option A: CLI-First (No Config File)

The fastest way to start — add mocks directly from the command line.

### Start an Empty Server

```bash
mockd start -d
```

This starts mockd in the background on port 4280 (mock server) and 4290 (admin API).

### Add a Mock

```bash
mockd add http --path /api/hello --body '{"message": "Hello, World!"}'
```

Output:

```
Created mock: http_abc123
  Type: http
  Method: GET
  Path:   /api/hello
  Status: 200
```

### Test It

```bash
curl http://localhost:4280/api/hello
```

Response:

```json
{"message": "Hello, World!"}
```

### Add More Mocks

```bash
# POST endpoint
mockd add http -m POST --path /api/users --status 201 \
  --body '{"id": 3, "message": "User created"}'

# Endpoint with delay
mockd add http --path /api/slow --delay 500 \
  --body '{"message": "This took a while"}'

# List what you've created
mockd list
```

---

## Option B: YAML Config File

For version-controlled, reproducible mock setups.

### Create a Config File

Create `mockd.yaml`:

```yaml
version: "1.0"

mocks:
  - id: hello-world
    name: Hello World Endpoint
    type: http
    enabled: true
    http:
      matcher:
        method: GET
        path: /api/hello
      response:
        statusCode: 200
        headers:
          Content-Type: application/json
        body: '{"message": "Hello, World!"}'
```

### Start the Server

```bash
mockd serve --config mockd.yaml
```

You should see output like:

```
mockd server starting...
Listening on http://localhost:4280
Admin API on http://localhost:4290
Loaded 1 mock(s) from mockd.yaml
```

### Test Your Mock

```bash
curl http://localhost:4280/api/hello
```

Response:

```json
{"message": "Hello, World!"}
```

---

## Option C: Initialize a Project

Use `mockd init` to scaffold a starter configuration:

```bash
mockd init
```

This creates a `mockd.yaml` with example mocks you can customize. Then start with:

```bash
mockd serve
```

---

## Adding More Mocks

Expand your YAML config with a realistic REST API:

```yaml
version: "1.0"

mocks:
  - id: get-users
    name: Get Users List
    type: http
    enabled: true
    http:
      matcher:
        method: GET
        path: /api/users
      response:
        statusCode: 200
        body: |
          {
            "users": [
              {"id": 1, "name": "Alice", "email": "alice@example.com"},
              {"id": 2, "name": "Bob", "email": "bob@example.com"}
            ]
          }

  - id: get-user-by-id
    name: Get User by ID
    type: http
    enabled: true
    http:
      matcher:
        method: GET
        path: /api/users/{id}
      response:
        statusCode: 200
        body: |
          {"id": "{{request.pathParam.id}}", "name": "Dynamic User"}

  - id: create-user
    name: Create New User
    type: http
    enabled: true
    http:
      matcher:
        method: POST
        path: /api/users
      response:
        statusCode: 201
        body: '{"id": 3, "message": "User created"}'
```

Restart the server (Ctrl+C to stop, then start again):

```bash
mockd serve --config mockd.yaml
```

Test the endpoints:

```bash
# List users
curl http://localhost:4280/api/users

# Get single user (dynamic path parameter)
curl http://localhost:4280/api/users/42

# Create user
curl -X POST http://localhost:4280/api/users
```

---

## Using Path Parameters

Match dynamic path segments:

```yaml
http:
  matcher:
    method: GET
    path: /api/users/{id}
  response:
    statusCode: 200
    body: '{"id": "{{request.pathParam.id}}", "name": "User {{request.pathParam.id}}"}'
```

This matches `/api/users/1`, `/api/users/abc`, etc.

---

## Adding Delays

Simulate network latency:

```yaml
http:
  matcher:
    method: GET
    path: /api/slow
  response:
    statusCode: 200
    delayMs: 500
    body: '{"message": "This took a while"}'
```

---

## Changing the Port

Use a different port:

```bash
mockd serve --config mockd.yaml --port 3000
```

---

## Beyond HTTP

mockd isn't just for HTTP. Add other protocol mocks to the same config:

```yaml
version: "1.0"

mocks:
  # HTTP mock
  - id: api-hello
    type: http
    http:
      matcher: { method: GET, path: /api/hello }
      response: { statusCode: 200, body: '{"msg": "hello"}' }

  # WebSocket mock
  - id: ws-echo
    type: websocket
    websocket:
      path: /ws
      echoMode: true

  # GraphQL mock
  - id: graphql-api
    type: graphql
    graphql:
      path: /graphql
      schema: |
        type Query { hello: String }
      resolvers:
        Query.hello:
          response: "world"
```

```bash
# Start everything
mockd serve --config mockd.yaml

# Test HTTP
curl http://localhost:4280/api/hello

# Test GraphQL
curl -X POST http://localhost:4280/graphql \
  -H "Content-Type: application/json" \
  -d '{"query": "{ hello }"}'
```

## What's Next?

- **[Core Concepts](/getting-started/concepts/)** - Understand mocks, matching, and responses
- **[Request Matching](/guides/request-matching/)** - Advanced matching patterns
- **[Stateful Mocking](/guides/stateful-mocking/)** - Simulate CRUD APIs
- **[Protocol Guides](/protocols/graphql/)** - GraphQL, gRPC, WebSocket, MQTT, SOAP, SSE
- **[CLI Reference](/reference/cli/)** - All command-line options
