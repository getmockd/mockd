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

### CLI Shortcut (Quick Prototyping)

The fastest way to get a stateful CRUD API running:

```bash
# Creates a data store + HTTP CRUD mocks in one step
mockd http add --path /api/users --stateful
```

Or create the resource manually:

```bash
mockd stateful add users
```

### Config File (Production)

For production configs, use **tables** (pure data stores) and **extend** (explicit bindings from mocks to tables):

```yaml
version: "1.0"

tables:
  users:
    idField: id

mocks:
  - id: list-users
    type: http
    http:
      matcher: { method: GET, path: /api/users }
      response: { statusCode: 200 }

  - id: create-user
    type: http
    http:
      matcher: { method: POST, path: /api/users }
      response: { statusCode: 201 }

  - id: get-user
    type: http
    http:
      matcher: { method: GET, path: /api/users/{id} }
      response: { statusCode: 200 }

  - id: update-user
    type: http
    http:
      matcher: { method: PUT, path: /api/users/{id} }
      response: { statusCode: 200 }

  - id: delete-user
    type: http
    http:
      matcher: { method: DELETE, path: /api/users/{id} }
      response: { statusCode: 200 }

extend:
  - mock: list-users
    table: users
    action: list
  - mock: create-user
    table: users
    action: create
  - mock: get-user
    table: users
    action: get
  - mock: update-user
    table: users
    action: update
  - mock: delete-user
    table: users
    action: delete
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

### Tables

Tables are pure data stores — they hold seed data and a schema but have no HTTP routing attached. Routing is handled by extend bindings.

```yaml
tables:
  users:
    idField: id
    seedData:
      - id: "1"
        name: "Alice"
        email: "alice@example.com"
      - id: "2"
        name: "Bob"
        email: "bob@example.com"
```

| Field | Description | Default |
|-------|-------------|---------|
| `idField` | Field name for resource ID | `"id"` |
| `seedData` | Initial data array | `[]` |

### Extend Bindings

Each extend binding connects a mock endpoint to a table with a specific action:

```yaml
extend:
  - mock: list-users       # references mock id
    table: users            # references table name
    action: list            # CRUD action
```

| Field | Description | Required |
|-------|-------------|----------|
| `mock` | ID of the mock to bind | Yes |
| `table` | Name of the table | Yes |
| `action` | `list`, `get`, `create`, `update`, `patch`, `delete`, `custom` | Yes |
| `operation` | Custom operation name (when `action: custom`) | No |

### Multiple Tables

```yaml
tables:
  users:
    seedData:
      - id: "1"
        name: "Alice"
  posts:
    seedData:
      - id: "1"
        title: "First Post"
  comments:
    idField: id
    # parentField not needed — parent scoping is handled by mock path params

mocks:
  - id: list-users
    type: http
    http:
      matcher: { method: GET, path: /api/users }
      response: { statusCode: 200 }
  - id: list-posts
    type: http
    http:
      matcher: { method: GET, path: /api/posts }
      response: { statusCode: 200 }

extend:
  - mock: list-users
    table: users
    action: list
  - mock: list-posts
    table: posts
    action: list
```

### Seed Data

Pre-populate tables:

```yaml
tables:
  users:
    seedData:
      - id: "1"
        name: "Alice"
        email: "alice@example.com"
      - id: "2"
        name: "Bob"
        email: "bob@example.com"
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

Handle parent-child relationships using tables with extend bindings:

```yaml
tables:
  posts:
    seedData:
      - id: "1"
        title: "First Post"
  comments:
    seedData: []

mocks:
  - id: list-comments
    type: http
    http:
      matcher: { method: GET, path: /api/posts/{postId}/comments }
      response: { statusCode: 200 }
  - id: create-comment
    type: http
    http:
      matcher: { method: POST, path: /api/posts/{postId}/comments }
      response: { statusCode: 201 }

extend:
  - mock: list-comments
    table: comments
    action: list
  - mock: create-comment
    table: comments
    action: create
```

Comments are scoped to their parent post via the path parameter:

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
tables:
  users:
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

Tables and extend bindings work alongside traditional static mocks:

```yaml
version: "1.0"

tables:
  users:
    seedData:
      - id: "1"
        name: "Alice"

mocks:
  - id: health-check
    type: http
    http:
      matcher: { method: GET, path: /api/health }
      response: { statusCode: 200, body: '{"status": "ok"}' }

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

Static mocks without extend bindings return their configured response. Mocks with extend bindings route through the stateful table.

## Complete Example

```yaml
version: "1.0"

serverConfig:
  httpPort: 4280
  adminPort: 4290

tables:
  users:
    idField: id
    seedData:
      - id: "1"
        name: "Admin"
        role: "admin"
  posts:
    seedData: []

mocks:
  - id: list-users
    type: http
    http:
      matcher: { method: GET, path: /api/users }
      response: { statusCode: 200 }
  - id: create-user
    type: http
    http:
      matcher: { method: POST, path: /api/users }
      response: { statusCode: 201 }
  - id: get-user
    type: http
    http:
      matcher: { method: GET, path: /api/users/{id} }
      response: { statusCode: 200 }
  - id: list-posts
    type: http
    http:
      matcher: { method: GET, path: /api/posts }
      response: { statusCode: 200 }
  - id: create-post
    type: http
    http:
      matcher: { method: POST, path: /api/posts }
      response: { statusCode: 201 }

extend:
  - { mock: list-users,  table: users, action: list }
  - { mock: create-user, table: users, action: create }
  - { mock: get-user,    table: users, action: get }
  - { mock: list-posts,  table: posts, action: list }
  - { mock: create-post, table: posts, action: create }
```

## Multi-Protocol State Sharing

Stateful resources are **protocol-agnostic**. The same in-memory store backs HTTP REST, SOAP, and other protocol handlers. Data created by one protocol is immediately visible to all others.

### SOAP + REST Sharing

```yaml
version: "1.0"

tables:
  users:
    seedData:
      - id: "1"
        name: "Alice"

mocks:
  - id: create-user
    type: http
    http:
      matcher: { method: POST, path: /api/users }
      response: { statusCode: 201 }

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

extend:
  - mock: create-user
    table: users
    action: create
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

SOAP operations use `statefulResource` and `statefulAction` directly on the operation config (that hasn't changed). REST endpoints use the tables+extend pattern. Both share the same underlying data store.

This is especially useful for testing systems that use REST internally but expose SOAP externally (or vice versa).

## Custom Operations

Custom operations compose reads, writes, and expression-evaluated transforms against stateful resources. They enable complex mock scenarios that span multiple resources.

### Example: Fund Transfer

```yaml
tables:
  accounts:
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

### Consistency Modes

Custom operations support two consistency modes:

| Mode | Description |
|------|-------------|
| `best_effort` (default) | Steps execute sequentially. If a step fails, prior state changes persist. |
| `atomic` | Steps execute sequentially. If a step fails, all prior state changes from this operation are rolled back. |

```yaml
customOperations:
  - name: TransferFunds
    consistency: atomic
    steps:
      # ...
```

:::caution[Atomic limitations]
**Atomic provides rollback-on-failure within a single operation.** It does NOT provide isolation across concurrent requests — other requests may observe intermediate state during execution. This is a mock server, not a database transaction engine.
:::

### Validating Operations Offline

Use `mockd stateful custom validate` to check operation definitions before registering them:

```bash
# Compile-check all expressions
mockd stateful custom validate --file transfer.yaml

# Evaluate expressions with sample input (preflight confidence, not a guarantee)
mockd stateful custom validate --file transfer.yaml \
  --input '{"sourceId":"acct-1","destId":"acct-2","amount":100}' \
  --check-expressions-runtime

# Verify referenced resources exist on the running server
mockd stateful custom validate --file transfer.yaml --check-resources
```

The `--check-expressions-runtime` flag provides **preflight confidence** by evaluating expressions with sample input and optional fixture data. It does not guarantee runtime success — actual resource data may differ from fixtures.

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

When a SOAP request arrives for the `TransferFunds` operation, the handler extracts the SOAP body as a map, passes it as input to the Bridge, executes the custom operation steps, and serializes the result back as an XML SOAP response.

### Using with HTTP Mocks

Any HTTP mock can trigger a custom operation by setting the `statefulOperation` field instead of a static `response`:

```yaml
mocks:
  - id: transfer-endpoint
    type: http
    http:
      matcher:
        method: POST
        path: /api/transfer
      statefulOperation: TransferFunds
```

When a `POST /api/transfer` request arrives, the JSON request body becomes the operation's `input`, the custom operation steps execute, and the result is returned as a JSON response. This allows HTTP endpoints to run the same multi-step logic as SOAP operations — sharing both the operation definition and the underlying stateful data.

**Example usage (CLI):**

```bash
# Register the custom operation
mockd stateful custom validate --file transfer.yaml --check-resources
# Optional stronger preflight (sample input + runtime expression checks, no writes)
mockd stateful custom validate --file transfer.yaml \
  --input '{"sourceId":"acct-1","destId":"acct-2","amount":100}' \
  --check-expressions-runtime \
  --fixtures-file transfer-fixtures.json
mockd stateful custom add --file transfer.yaml

# Create the HTTP mock wired to the operation
mockd add http --method POST --path /api/transfer --stateful-operation TransferFunds

# Call it
curl -X POST http://localhost:4280/api/transfer \
  -H "Content-Type: application/json" \
  -d '{"sourceId":"acct-1","destId":"acct-2","amount":100}'
```

**Example usage (YAML config):**

```yaml
customOperations:
  - name: TransferFunds
    consistency: atomic
    steps:
      - type: read
        resource: accounts
        id: "input.sourceId"
        as: source
      # ... more steps ...
    response:
      status: '"completed"'

mocks:
  - type: http
    http:
      matcher:
        method: POST
        path: /api/transfer
      statefulOperation: TransferFunds
```

### Using with the CLI

Custom operations can be executed directly from the CLI without any protocol handler:

```bash
mockd stateful custom validate --file transfer.yaml --input '{"sourceId":"acct-1","destId":"acct-2","amount":100}'
mockd stateful custom validate --file transfer.yaml --input '{"sourceId":"acct-1","destId":"acct-2","amount":100}' --check-expressions-runtime --fixtures-file transfer-fixtures.json
mockd stateful custom run TransferFunds --input '{"sourceId":"acct-1","destId":"acct-2","amount":100}'
```

This is useful for testing, scripting, and AI agent workflows where you want to manipulate stateful data through defined business logic without making HTTP/SOAP requests.

### Using with the Admin API

Custom operations are also accessible via the admin REST API:

```bash
# List all operations
curl http://localhost:4290/state/operations

# Execute an operation
curl -X POST http://localhost:4290/state/operations/TransferFunds/execute \
  -H "Content-Type: application/json" \
  -d '{"sourceId":"acct-1","destId":"acct-2","amount":100}'
```

## Importing Specs and Binding to Tables

Use `imports` to load external API specs (OpenAPI, WSDL) and bind the generated mocks to tables:

```yaml
version: "1.0"

imports:
  - spec: ./stripe-openapi.yaml
    namespace: stripe
    format: openapi

tables:
  customers:
    seedData:
      - id: "cus_001"
        name: "Alice"
        email: "alice@example.com"

extend:
  - mock: stripe/list-customers
    table: customers
    action: list
  - mock: stripe/create-customer
    table: customers
    action: create
  - mock: stripe/get-customer
    table: customers
    action: get
```

Imported mocks receive IDs prefixed with the namespace (e.g., `stripe/list-customers`). The `extend` bindings wire those generated mocks to your local tables, creating a stateful digital twin of the imported API.

## Response Transform Pipeline

When a mock has an extend binding, the response flows through a transform pipeline:

1. **Request arrives** and matches a mock via the standard matcher
2. **Extend binding** routes the request to the table's Bridge
3. **Bridge executes** the CRUD action (list, get, create, update, delete, custom)
4. **Result is serialized** as the response body (JSON for HTTP, XML for SOAP)
5. **Response headers and status code** from the mock definition are applied

The mock's `response.body` field is ignored when an extend binding is active — the table's data becomes the response. However, `response.statusCode` and `response.headers` are still respected.

## Custom Operations via Extend

Extend bindings support `action: custom` to trigger multi-step custom operations:

```yaml
tables:
  accounts:
    seedData:
      - { id: "acct-1", owner: "Alice", balance: 1000 }
      - { id: "acct-2", owner: "Bob", balance: 500 }

customOperations:
  - name: TransferFunds
    consistency: atomic
    steps:
      - type: read
        resource: accounts
        id: "input.sourceId"
        as: source
      - type: update
        resource: accounts
        id: "input.sourceId"
        set: { balance: "source.balance - input.amount" }
    response:
      status: '"completed"'

mocks:
  - id: transfer
    type: http
    http:
      matcher: { method: POST, path: /api/transfer }
      response: { statusCode: 200 }

extend:
  - mock: transfer
    table: accounts
    action: custom
    operation: TransferFunds
```

## CLI vs Config: When to Use Each

| Approach | Use Case |
|----------|----------|
| `mockd http add --stateful` | Quick prototyping, one-off testing |
| `mockd stateful add` + manual mocks | Interactive exploration |
| `tables` + `extend` in config | Production configs, version-controlled setups |
| `imports` + `extend` | Digital twins of third-party APIs |

The CLI `--stateful` shortcut is equivalent to creating a table + mocks + extend bindings in a config file. Use it for speed; use config files for reproducibility.

## Next Steps

- [SOAP Mocking](/protocols/soap/) - Full SOAP protocol guide with WSDL import
- [Proxy Recording](/guides/proxy-recording/) - Record real API traffic
- [Admin API Reference](/reference/admin-api/) - State management endpoints
- [Configuration Reference](/reference/configuration/) - Full schema
