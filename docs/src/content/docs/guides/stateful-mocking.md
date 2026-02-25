---
title: Stateful Mocking
description: Simulate real CRUD APIs where resources persist across requests with create, update, and delete operations.
---

Stateful mocking allows mockd to simulate real CRUD APIs where resources persist across requests. Create, update, and delete operations modify state that subsequent requests can observe.

## Overview

Traditional mocks return static responses. Stateful mocking maintains an in-memory store that:

- **POST** creates new resources
- **GET** retrieves current resources  
- **PUT** replaces existing resources
- **DELETE** removes resources

Changes persist for the lifetime of the server session.

## Quick Start

Create a stateful resource using the CLI or configuration file:

```bash
# Via CLI (creates the resource on a running server)
mockd stateful add users --path /api/users

# Or for a bridge-only resource (accessible via SOAP/GraphQL/gRPC but not HTTP)
mockd stateful add users
```

Or define it in your configuration file:

```json
{
  "statefulResources": [
    {
      "name": "users",
      "basePath": "/api/users",
      "idField": "id"
    }
  ]
}
```

Start the server and interact:

```bash
# Create a user (returns 201 with auto-generated UUID id)
curl -X POST http://localhost:4280/api/users \
  -H "Content-Type: application/json" \
  -d '{"name": "Alice", "email": "alice@example.com"}'
# Response: {"id": "a1b2c3d4-...", "name": "Alice", "email": "alice@example.com", ...}

# List users - returns paginated response
curl http://localhost:4280/api/users
# Response: {"data": [...], "meta": {"total": 1, "limit": 100, "offset": 0, "count": 1}}

# Get single user by ID
curl http://localhost:4280/api/users/a1b2c3d4-...
# Response: {"id": "a1b2c3d4-...", "name": "Alice", "email": "alice@example.com"}

# Update user
curl -X PUT http://localhost:4280/api/users/a1b2c3d4-... \
  -H "Content-Type: application/json" \
  -d '{"name": "Alice Smith", "email": "alice@example.com"}'

# Delete user
curl -X DELETE http://localhost:4280/api/users/a1b2c3d4-...
# Response: 204 No Content

# User is gone
curl http://localhost:4280/api/users/a1b2c3d4-...
# Response: 404 Not Found
```

## Configuration

### Basic Resource

```json
{
  "statefulResources": [
    {
      "name": "users",
      "basePath": "/api/users"
    }
  ]
}
```

| Field | Description | Default |
|-------|-------------|---------|
| `name` | Unique resource name | Required |
| `basePath` | URL path prefix for HTTP REST endpoints (omit for bridge-only) | `""` |
| `idField` | Field name for resource ID | `"id"` |
| `parentField` | Parent FK field for nested resources | - |
| `seedData` | Initial data array | `[]` |

### Multiple Resources

```json
{
  "statefulResources": [
    {
      "name": "users",
      "basePath": "/api/users"
    },
    {
      "name": "posts",
      "basePath": "/api/posts"
    },
    {
      "name": "comments",
      "basePath": "/api/posts/:postId/comments",
      "parentField": "postId"
    }
  ]
}
```

### Initial Data (Seeding)

Pre-populate resources:

```json
{
  "statefulResources": [
    {
      "name": "users",
      "basePath": "/api/users",
      "seedData": [
        {"id": "1", "name": "Alice", "email": "alice@example.com"},
        {"id": "2", "name": "Bob", "email": "bob@example.com"}
      ]
    }
  ]
}
```

## CRUD Operations

### Create (POST)

```bash
POST /api/users
Content-Type: application/json

{"name": "Charlie", "email": "charlie@example.com"}
```

Response (`201 Created`):
```json
{
  "id": "f47ac10b-58cc-4372-a567-0e02b2c3d479",
  "name": "Charlie",
  "email": "charlie@example.com",
  "createdAt": "2024-01-15T10:30:00Z",
  "updatedAt": "2024-01-15T10:30:00Z"
}
```

IDs are auto-generated as UUIDs unless the request body includes an `id` field.

### Read Collection (GET)

```bash
GET /api/users
```

Response (paginated):
```json
{
  "data": [
    {"id": "1", "name": "Alice"},
    {"id": "2", "name": "Bob"}
  ],
  "meta": {
    "total": 2,
    "limit": 100,
    "offset": 0,
    "count": 2
  }
}
```

### Read Single (GET)

```bash
GET /api/users/2
```

Response:
```json
{"id": "2", "name": "Bob", "email": "bob@example.com"}
```

Not found:
```bash
GET /api/users/999
```
Response: `404 Not Found`

### Update (PUT)

Replace entire resource:

```bash
PUT /api/users/2
Content-Type: application/json

{"name": "Robert", "email": "robert@example.com"}
```

Response:
```json
{"id": "2", "name": "Robert", "email": "robert@example.com"}
```

### Delete (DELETE)

```bash
DELETE /api/users/2
```

Response: `204 No Content`

## Nested Resources

Handle parent-child relationships:

```json
{
  "statefulResources": [
    {
      "name": "posts",
      "basePath": "/api/posts"
    },
    {
      "name": "comments",
      "basePath": "/api/posts/:postId/comments",
      "parentField": "postId"
    }
  ]
}
```

Comments are scoped to their parent post:

```bash
# Get comments for post 1
GET /api/posts/1/comments

# Create comment on post 1
POST /api/posts/1/comments
{"text": "Great post!"}
```

## Filtering and Pagination

### Query Filtering

Filter by any field using query parameters:

```bash
GET /api/users?name=Alice
GET /api/users?status=active
```

Filtering is always enabled.

### Pagination

Use offset-based pagination:

```bash
GET /api/users?limit=10&offset=20
GET /api/users?sort=name&order=asc
```

Query parameters:

| Parameter | Description | Default |
|-----------|-------------|---------|
| `limit` | Maximum items to return | 100 |
| `offset` | Items to skip | 0 |
| `sort` | Field to sort by | `createdAt` |
| `order` | Sort direction: "asc" or "desc" | `desc` |

Response includes pagination metadata:

```json
{
  "data": [...],
  "meta": {
    "total": 45,
    "limit": 10,
    "offset": 20,
    "count": 10
  }
}
```

## Validation

Validate incoming requests before creating or updating resources. Validation ensures data integrity by checking field types, formats, constraints, and required fields.

### Quick Example

```yaml
statefulResources:
  - name: users
    basePath: /api/users
    validation:
      mode: strict
      fields:
        email:
          type: string
          required: true
          format: email
        username:
          type: string
          required: true
          minLength: 3
          maxLength: 30
          pattern: "^[a-z][a-z0-9_]*$"
        age:
          type: integer
          min: 0
          max: 150
        role:
          type: string
          enum: [admin, user, guest]
```

### Validation Modes

| Mode | Behavior |
|------|----------|
| `strict` | Reject request on any validation failure (default) |
| `warn` | Log warnings but allow request through |
| `permissive` | Only fail on critical errors (missing required fields) |

### Nested Fields

Validate nested object fields using dot notation:

```yaml
fields:
  "address.city":
    type: string
    required: true
  "items.sku":
    type: string
    pattern: "^SKU-[A-Z0-9]+$"
```

For nested objects, array validation, formats, patterns, and more, see the [Validation Guide](/guides/validation/).

## State Lifetime

State exists only in memory and resets when the server stops. Use seed data to pre-populate resources on startup.

## Admin API

Manage state via the admin API:

```bash
# Get state overview (resource list, item counts)
curl http://localhost:4290/state

# Reset all resources to seed data
curl -X POST http://localhost:4290/state/reset

# List all registered resources
curl http://localhost:4290/state/resources

# Get specific resource info
curl http://localhost:4290/state/resources/users

# Reset a specific resource to its seed data
curl -X POST http://localhost:4290/state/resources/users/reset

# Clear all items from a resource (does NOT restore seed data)
curl -X DELETE http://localhost:4290/state/resources/users

# List items in a resource
curl http://localhost:4290/state/resources/users/items

# Create an item via admin API
curl -X POST http://localhost:4290/state/resources/users/items \
  -H "Content-Type: application/json" \
  -d '{"name": "Charlie", "email": "charlie@example.com"}'
```

## Combined with Static Mocks

Stateful resources work alongside traditional mocks:

```json
{
  "mocks": [
    {
      "id": "health-check",
      "type": "http",
      "http": {
        "matcher": {"method": "GET", "path": "/api/health"},
        "response": {"statusCode": 200, "body": "{\"status\": \"ok\"}"}
      }
    }
  ],
  "statefulResources": [
    {
      "name": "users",
      "basePath": "/api/users"
    }
  ]
}
```

Static mocks take priority when matched.

## Complete Example

```json
{
  "server": {
    "port": 4280
  },
  "statefulResources": [
    {
      "name": "users",
      "basePath": "/api/users",
      "idField": "id",
      "seedData": [
        {"id": "1", "name": "Admin", "role": "admin"}
      ]
    },
    {
      "name": "posts",
      "basePath": "/api/posts"
    }
  ]
}
```

## Multi-Protocol State Sharing

Stateful resources are **protocol-agnostic**. The same in-memory store backs HTTP REST, SOAP, and other protocol handlers. Data created by one protocol is immediately visible to all others.

### SOAP + REST Sharing

```yaml
version: "1.0"

statefulResources:
  - name: users
    basePath: /api/users

mocks:
  - type: soap
    soap:
      path: /soap/UserService
      operations:
        GetUser:
          statefulResource: users
          statefulAction: get
        CreateUser:
          statefulResource: users
          statefulAction: create
```

```bash
# Create via REST
curl -X POST http://localhost:4280/api/users \
  -H "Content-Type: application/json" \
  -d '{"name": "Alice"}'

# Retrieve via SOAP — same data store!
curl -X POST http://localhost:4280/soap/UserService \
  -H "SOAPAction: GetUser" -H "Content-Type: text/xml" \
  -d '<soap:Envelope xmlns:soap="http://schemas.xmlsoap.org/soap/envelope/">
        <soap:Body><GetUser><Id>USER_ID_HERE</Id></GetUser></soap:Body>
      </soap:Envelope>'
```

This is especially useful for testing systems that use REST internally but expose SOAP externally (or vice versa).

## Custom Operations

Custom operations compose reads, writes, and expression-evaluated transforms against stateful resources. They enable complex mock scenarios that span multiple resources.

### Example: Fund Transfer

```yaml
statefulResources:
  - name: accounts
    basePath: /api/accounts
    seedData:
      - { id: "acct-1", owner: "Alice", balance: 1000 }
      - { id: "acct-2", owner: "Bob", balance: 500 }

customOperations:
  - name: TransferFunds
    steps:
      - type: read
        resource: accounts
        id: "input.sourceId"
        as: source
      - type: read
        resource: accounts
        id: "input.destId"
        as: dest
      - type: update
        resource: accounts
        id: "input.sourceId"
        set:
          balance: "source.balance - input.amount"
      - type: update
        resource: accounts
        id: "input.destId"
        set:
          balance: "dest.balance + input.amount"
    response:
      status: '"completed"'
      newSourceBalance: "source.balance - input.amount"
      newDestBalance: "dest.balance + input.amount"
```

### Step Types

| Step | Fields | Description |
|------|--------|-------------|
| `read` | `resource`, `id`, `as` | Read an item, store in named variable |
| `create` | `resource`, `set`, `as` | Create an item with expression fields |
| `update` | `resource`, `id`, `set` | Update an item with expression fields |
| `delete` | `resource`, `id` | Delete an item |
| `set` | `var`, `value` | Set a context variable to an expression |

### Expression Language

Steps use [expr-lang/expr](https://github.com/expr-lang/expr) for evaluating expressions. The environment includes:
- `input` — the request data
- Named variables from prior `read`/`create` steps
- Standard arithmetic, comparison, and string operators

### Using with SOAP

Wire custom operations from SOAP operation configs:

```yaml
mocks:
  - type: soap
    soap:
      path: /soap/BankService
      operations:
        TransferFunds:
          soapAction: "http://bank.example.com/TransferFunds"
          statefulResource: TransferFunds
          statefulAction: custom
```

## Next Steps

- [SOAP Mocking](/protocols/soap/) - Full SOAP protocol guide with WSDL import
- [Proxy Recording](/guides/proxy-recording/) - Record real API traffic
- [Admin API Reference](/reference/admin-api/) - State management endpoints
- [Configuration Reference](/reference/configuration/) - Full schema
