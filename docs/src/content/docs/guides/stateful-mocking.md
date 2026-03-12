---
title: Stateful Mocking
description: Simulate real CRUD APIs where resources persist across requests with create, update, and delete operations.
---

Stateful mocking allows mockd to simulate real CRUD APIs where resources persist across requests. Create, update, and delete operations modify state that subsequent requests can observe.

:::note[Workspace Isolation]
Stateful resources are scoped to workspaces. Resources created in one workspace are independent from resources in other workspaces. Use `--workspace` to target a specific workspace.
:::

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
  - name: users
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
  - name: users
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
| `name` | Table name (used in extend bindings) | Required |
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

:::tip[POST-as-Update: When to use `patch` vs `update`]
Most REST APIs use PUT for full replacement and PATCH for partial updates. But some APIs (notably Stripe and Twilio) use **POST for both creates and updates**. When the POST endpoint uses partial-merge semantics (only fields present in the body are updated, others are preserved), use `action: patch`:

```yaml
extend:
  # Stripe uses POST to update customers — only sent fields change
  - mock: stripe.PostCustomersId
    table: customers
    action: patch     # NOT update — partial merge

  # Traditional REST API uses PUT for full replacement
  - mock: update-user
    table: users
    action: update    # Full replacement — missing fields are removed
```

**Rule of thumb:** Use `update` for PUT endpoints (full replace). Use `patch` for PATCH endpoints or POST endpoints that do partial updates.
:::

### Multiple Tables

```yaml
tables:
  - name: users
    seedData:
      - id: "1"
        name: "Alice"
  - name: posts
    seedData:
      - id: "1"
        title: "First Post"
  - name: comments
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
  - name: users
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
  - name: posts
    seedData:
      - id: "1"
        title: "First Post"
  - name: comments
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

## ID Strategies

Tables support five ID generation strategies, controlled by the `idStrategy` field. When a create request includes an ID in the body, that ID is used regardless of strategy.

| Strategy | `idStrategy` | Example Output | Description |
|----------|-------------|----------------|-------------|
| UUID (default) | `uuid` | `f47ac10b-58cc-4372-a567-0e02b2c3d479` | Standard UUID v4 |
| Prefix | `prefix` | `cus_a1b2c3d4e5f6g7h8` | Configurable prefix + 16 random hex characters (requires `idPrefix`) |
| ULID | `ulid` | `01HQJK5Y3N8RJZVP10XGBC94XR` | Universally Unique Lexicographically Sortable Identifier (time-sortable) |
| Sequence | `sequence` | `1`, `2`, `3` | Auto-incrementing integer starting from 1 |
| Short | `short` | `a1b2c3d4e5f6g7h8` | 16 random hex characters (no prefix) |

### Configuration

```yaml
tables:
  # UUID (default — no config needed)
  - name: users
    idField: id

  # Prefix — Stripe-style IDs like cus_abc123
  - name: customers
    idField: id
    idStrategy: prefix
    idPrefix: "cus_"

  # ULID — time-sortable
  - name: events
    idField: id
    idStrategy: ulid

  # Sequence — auto-incrementing integers
  - name: tickets
    idField: id
    idStrategy: sequence

  # Short — compact hex IDs
  - name: tokens
    idField: id
    idStrategy: short
```

When using `sequence`, the counter is reset to 0 on `POST /state/reset`. If seed data contains numeric IDs, the counter starts after the highest seed ID.

## Filtering, Sorting & Pagination

### Query Parameter Filtering

Filter by any field using exact-match query parameters. Any query parameter that isn't a [reserved parameter](#reserved-query-parameters) is treated as a field filter:

```bash
# Single field filter
GET /api/users?status=active

# Multiple fields (AND logic)
GET /api/users?status=active&role=admin

# Filter by ID
GET /api/users?id=123
```

### Bracket Notation (Nested Fields)

Filter on nested object fields using bracket notation:

```bash
# Filter by nested field
GET /api/users?metadata[tier]=premium

# Multiple levels of nesting
GET /api/users?address[country]=US
```

Bracket notation resolves against the stored data — `metadata[tier]` matches items where `data.metadata.tier` equals `"premium"`.

### Sorting

Sort results by any field:

```bash
GET /api/users?sort=name&order=asc
GET /api/users?sort=createdAt&order=desc
```

| Parameter | Description | Default |
|-----------|-------------|---------|
| `sort` | Field to sort by (`id`, `createdAt`, `updatedAt`, or any data field) | `createdAt` |
| `order` | Sort direction: `asc` or `desc` | `desc` |

Sorting supports string, numeric (int, int64, float64), and time comparisons. Unknown types fall back to string comparison.

### Offset-Based Pagination

```bash
GET /api/users?limit=10&offset=20
```

| Parameter | Description | Default |
|-----------|-------------|---------|
| `limit` | Maximum items to return | `100` |
| `offset` | Number of items to skip | `0` |

Response includes pagination metadata:

```json
{
  "data": [...],
  "meta": {
    "total": 45,
    "limit": 10,
    "offset": 20,
    "count": 10,
    "has_more": true
  }
}
```

### Cursor-Based Pagination

For Stripe-style APIs, use cursor-based pagination with `starting_after` and `ending_before`:

```bash
# Get next page after a specific item
GET /v1/customers?limit=10&starting_after=cus_123

# Get previous page before a specific item
GET /v1/customers?limit=10&ending_before=cus_456
```

| Parameter | Description |
|-----------|-------------|
| `starting_after` | Return items after the item with this ID (forward pagination) |
| `ending_before` | Return items before the item with this ID (backward pagination) |

Cursor pagination is mutually exclusive with `offset`. When a cursor parameter is present, `offset` is ignored. The response's `has_more` field indicates whether more items exist beyond the current page.

### Parent Field Filtering

For sub-resource tables (e.g., invoice line items under invoices), the `parentField` configuration automatically filters items by the parent ID from the URL path parameter:

```yaml
tables:
  - name: line_items
    parentField: invoice    # filters by this field

mocks:
  - id: list-line-items
    type: http
    http:
      matcher: { method: GET, path: /v1/invoices/{invoice}/lines }
      response: { statusCode: 200 }

extend:
  - mock: list-line-items
    table: line_items
    action: list
```

When a request hits `GET /v1/invoices/inv_123/lines`, mockd automatically filters `line_items` where `invoice == "inv_123"`.

### Reserved Query Parameters

These query parameters are reserved by mockd and are NOT treated as field filters:

| Category | Parameters |
|----------|------------|
| **Pagination** | `limit`, `offset`, `page`, `per_page`, `starting_after`, `ending_before`, `cursor`, `page_size`, `page_token` |
| **Sorting** | `sort`, `order`, `sort_by`, `order_by` |
| **Expansion** | `expand`, `expand[]`, `fields`, `include`, `exclude`, `select` |
| **Other** | `format`, `pretty`, `api_version`, `idempotency_key`, `request_id` |

## Relationships & Expand

Tables can define relationships between fields and other tables. When a client requests expansion via `?expand[]`, mockd looks up the related item by ID and inlines the full object in place of the string ID.

### Defining Relationships

Add a `relationships` map to a table, where each key is a field name and the value specifies the target table:

```yaml
tables:
  - name: customers
    idField: id
    idStrategy: prefix
    idPrefix: "cus_"
    seedData:
      - { id: "cus_123", name: "Jenny Rosen", email: "jenny@example.com" }

  - name: charges
    idField: id
    idStrategy: prefix
    idPrefix: "ch_"
    relationships:
      customer: { table: customers }
    seedData:
      - { id: "ch_456", amount: 2000, currency: "usd", customer: "cus_123" }
```

| Field | Type | Description |
|-------|------|-------------|
| `table` | string | Name of the target table to look up |
| `field` | string | Field in the target table to match against (default: the target table's `idField`) |

### Using ?expand[]

Expand fields on GET requests (both single-item and list endpoints):

```bash
# Without expand — customer is a string ID
GET /v1/charges/ch_456
# Response: {"id": "ch_456", "amount": 2000, "customer": "cus_123", ...}

# With expand — customer is inlined as the full object
GET /v1/charges/ch_456?expand[]=customer
# Response: {"id": "ch_456", "amount": 2000, "customer": {"id": "cus_123", "name": "Jenny Rosen", ...}, ...}
```

Two syntax styles are supported:

```bash
# Array-style (Stripe convention)
GET /v1/charges?expand[]=customer

# Comma-separated
GET /v1/charges?expand=customer,invoice
```

### Expand on List Endpoints

Expand is applied to every item in a list response:

```bash
GET /v1/charges?expand[]=customer
# Each charge in the response has its customer field expanded
```

### Graceful Degradation

- If a field has no defined relationship, the expand request for that field is silently ignored
- If the related item is not found (e.g., the referenced ID doesn't exist in the target table), the field is left as the original string ID
- If the field value is empty or nil, it's left as-is

### Real-World Example: Stripe Subscriptions

```yaml
tables:
  - name: subscriptions
    idStrategy: prefix
    idPrefix: "sub_"
    relationships:
      customer: { table: customers }
      latest_invoice: { table: invoices }
    seedData:
      - { id: "sub_123", customer: "cus_123", status: "active", latest_invoice: "in_789" }
```

```bash
# Expand multiple related objects
GET /v1/subscriptions/sub_123?expand[]=customer&expand[]=latest_invoice
```

## Form URL-Encoded Body Handling

When a request uses `Content-Type: application/x-www-form-urlencoded`, mockd automatically coerces form data into structured JSON. This is critical for SDK compatibility with APIs like Stripe and Twilio, which use form encoding for all requests.

### Type Coercion

String form values are automatically converted to their natural types:

| Form Value | Coerced To | Go Type |
|------------|-----------|---------|
| `"true"`, `"false"` | Boolean | `bool` |
| `"42"` | Integer | `int64` |
| `"3.14"` | Float | `float64` |
| `"inf"` | Null | `nil` |
| `"+15551234567"` | String (preserved) | `string` |

Values starting with `+` are NOT coerced to numbers — this preserves phone numbers like `+15551234567` that would otherwise be parsed as positive integers.

### Nested Object Expansion

Bracket-notation fields are converted to nested objects:

```
address[city]=New+York&address[state]=NY
```
becomes:
```json
{"address": {"city": "New York", "state": "NY"}}
```

### Array Coercion

Numeric-keyed bracket notation is converted to arrays:

```
items[0]=card&items[1]=bank_account
```
becomes:
```json
{"items": ["card", "bank_account"]}
```

This also works with nested objects inside arrays:

```
items[0][price]=price_123&items[1][price]=price_456
```
becomes:
```json
{"items": [{"price": "price_123"}, {"price": "price_456"}]}
```

### Why This Matters

Stripe and Twilio SDKs send all API requests as form-encoded bodies, not JSON. Without this coercion, a Stripe SDK call like:

```go
stripe.Customer.Create(&stripe.CustomerParams{
    Name: stripe.String("Jenny Rosen"),
})
```

Would arrive as `name=Jenny+Rosen` and be stored as a raw string instead of properly structured data. The coercion layer ensures mockd handles these SDKs transparently.

## Validation

Validate incoming requests before creating or updating resources. Validation ensures data integrity by checking field types, formats, constraints, and required fields.

### Quick Example

```yaml
tables:
  - name: users
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
  - name: users
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
  - name: users
    idField: id
    seedData:
      - id: "1"
        name: "Admin"
        role: "admin"
  - name: posts
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
  - name: users
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
  - name: accounts
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
| `list` | `resource`, `as`, `filter` | Query a resource for multiple items and store the result array |
| `validate` | `condition`, `errorMessage`, `errorStatus` | Check a boolean condition; halt with an error if false |

#### List Step

The `list` step queries a resource with optional filters and stores the result as an array in a named variable. This enables aggregation via [expr-lang](https://github.com/expr-lang/expr) builtins like `sum()`, `filter()`, `count()`, `map()`, and `reduce()`.

```yaml
steps:
  - type: list
    resource: transactions
    as: txns
    filter:
      accountId: "input.accountId"
      status: "'completed'"
  - type: set
    var: total
    value: "sum(txns, .amount)"
```

The `filter` field is a map of field name to expression. Each expression is evaluated against the operation context. Literal strings must be quoted inside the expression (e.g., `"'completed'"`). The list step returns all matching items (no pagination limit).

#### Validate Step

The `validate` step evaluates a boolean expression and halts the operation with an error if the condition is false. This enables business logic validation within custom operations.

```yaml
steps:
  - type: read
    resource: accounts
    id: "input.sourceId"
    as: source
  - type: validate
    condition: "source.balance >= input.amount"
    errorMessage: "Insufficient funds"
    errorStatus: 400
```

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `condition` | string | Required | Boolean expression — operation halts if false |
| `errorMessage` | string | `"validation failed: {condition}"` | Error message returned on failure |
| `errorStatus` | int | `400` | HTTP status code returned on failure |

### Expression Language

Steps use [expr-lang/expr](https://github.com/expr-lang/expr) for evaluating expressions. The environment includes:
- `input` — the request data
- Named variables from prior `read`/`create` steps
- Standard arithmetic, comparison, and string operators

### String Literals in Expressions

Custom operation expressions treat unquoted values as **variable references** and single-quoted values inside the YAML string as **string literals**. This distinction is critical:

```yaml
steps:
  - type: update
    resource: orders
    id: "input.orderId"
    set:
      # Variable reference — reads the value of pi.status
      status: "pi.status"

      # String literal — sets the value to the string "succeeded"
      status: '"succeeded"'

      # Null coalescing — uses input value or falls back to a literal string
      reason: 'input.cancellation_reason ?? "requested_by_customer"'
```

The YAML quoting rules:
- `"pi.status"` → evaluated as an expression (variable lookup)
- `'"succeeded"'` → the outer single quotes are YAML, inner double quotes make it an expr-lang string literal
- `'input.reason ?? "default"'` → expression with a string literal fallback

:::caution[Common mistake]
Writing `status: "succeeded"` (without inner quotes) will cause an expression evaluation error because expr-lang interprets `succeeded` as a variable name. Always use `'"literal"'` for string constants in expressions.
:::

### Common Expression Patterns

Custom operation expressions support the full [expr-lang/expr](https://github.com/expr-lang/expr) syntax. Here are the most commonly used patterns:

| Pattern | Example | Description |
|---------|---------|-------------|
| Variable access | `input.amount` | Read a field from input or a named variable |
| Nested access | `source.metadata.tier` | Dot-notation for nested fields |
| Arithmetic | `source.balance - input.amount` | Addition, subtraction, multiplication, division |
| Comparison | `source.balance >= input.amount` | `==`, `!=`, `>`, `<`, `>=`, `<=` |
| String literal | `'"succeeded"'` | Literal string value (note the quoting) |
| Boolean literal | `true`, `false` | Boolean constants |
| Numeric literal | `42`, `3.14` | Integer and float constants |
| Null coalescing | `input.reason ?? "requested_by_customer"` | Use left value if non-nil, otherwise right |
| String conversion | `string(input.amount)` | Convert a value to string |
| Array aggregation | `sum(txns, .amount)` | Sum a field across an array (from a `list` step) |
| Array filtering | `filter(txns, .status == "completed")` | Filter array items |
| Array count | `count(txns, .status == "pending")` | Count matching items |
| Array mapping | `map(txns, .amount)` | Extract a field from each item |

For the complete expression language reference, see the [expr-lang documentation](https://expr-lang.org/docs/language-definition).

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
  - path: ./stripe-openapi.yaml
    as: stripe
    format: openapi

tables:
  - name: customers
    seedData:
      - id: "cus_001"
        name: "Alice"
        email: "alice@example.com"

extend:
  - mock: stripe.ListCustomers
    table: customers
    action: list
  - mock: stripe.CreateCustomer
    table: customers
    action: create
  - mock: stripe.GetCustomer
    table: customers
    action: get
```

Imported mocks receive IDs prefixed with the namespace using dot notation (e.g., `stripe.ListCustomers`). The `extend` bindings wire those generated mocks to your local tables, creating a stateful digital twin of the imported API.

To discover the available operationIds after importing a spec, use `mockd list` on a running server:

```bash
# List all mocks including imported ones
mockd list

# Filter to see just the imported namespace
mockd list | grep "stripe\."
```

Each operationId comes directly from the OpenAPI spec's `operationId` field. For example, Stripe's spec defines `operationId: PostCustomers` on `POST /v1/customers`, which becomes `stripe.PostCustomers` with the `as: stripe` namespace.

## Response Transform Pipeline

When a mock has an extend binding, the response flows through a transform pipeline:

1. **Request arrives** and matches a mock via the standard matcher
2. **Extend binding** routes the request to the table's Bridge
3. **Bridge executes** the CRUD action (list, get, create, update, delete, custom)
4. **Result is serialized** as the response body (JSON for HTTP, XML for SOAP)
5. **Response headers and status code** from the mock definition are applied

The mock's `response.body` field is ignored when an extend binding is active — the table's data becomes the response. However, `response.statusCode` and `response.headers` are still respected.

## Response Transforms

Response transforms customize how stateful table data is shaped before it's returned to the client. Without transforms, mockd returns its standard format — items with `id`, `createdAt`, `updatedAt`, and a list envelope of `{"data": [...], "meta": {...}}`. With transforms, you can match the exact response shape of any API — Stripe, Twilio, GitHub, or your own.

This is the feature that makes digital twins possible: the same underlying CRUD data can be returned in Stripe's format, Twilio's format, or any other convention.

### Where Transforms Are Defined

Transforms can be set at two levels:

1. **Table-level default** — applies to all bindings for that table
2. **Binding-level override** — overrides the table default for a specific endpoint

The resolution order is: binding override > table default > no transform (raw data).

```yaml
tables:
  - name: customers
    response:           # table-level default
      timestamps:
        format: unix

extend:
  - mock: get-customer
    table: customers
    action: get         # uses the table default transform

  - mock: list-customers
    table: customers
    action: list
    response:           # binding-level override (replaces table default)
      timestamps:
        format: iso8601
      list:
        dataField: results
```

### Transform Execution Order

When an item is returned from a table, transforms are applied in this order:

1. **Rename** — field keys are renamed (stored data unchanged)
2. **Hide** — fields are removed from the response
3. **WrapAsList** — array fields are wrapped in list object envelopes
4. **Timestamps** — format conversion and/or key renaming for `createdAt`/`updatedAt`
5. **Inject** — static fields are added to the response (always present, can't be accidentally hidden or renamed)

This ordering is intentional: injected fields are added last so they are always present in the output regardless of hide/rename rules.

### Timestamps

Controls how `createdAt` and `updatedAt` fields appear in responses. mockd internally stores timestamps as RFC3339Nano strings; the transform converts them on output.

| Field | Type | Description |
|-------|------|-------------|
| `format` | string | Output format: `unix`, `iso8601`, `rfc3339` (default), `none` |
| `fields` | map | Rename timestamp keys (e.g., `createdAt` to `created`) |

**Formats:**

| Format | Output | Example |
|--------|--------|---------|
| `unix` | Integer epoch seconds | `1705312200` |
| `iso8601` | RFC3339 string | `"2024-01-15T10:30:00Z"` |
| `rfc3339` | RFC3339Nano string (default, no transform) | `"2024-01-15T10:30:00.000000000Z"` |
| `none` | Field removed entirely | _(field absent)_ |

**Example: Stripe-style unix timestamps renamed to `created`:**

```yaml
tables:
  - name: customers
    response:
      timestamps:
        format: unix
        fields:
          createdAt: created
          updatedAt: updated
```

A stored item like:
```json
{"id": "cus_123", "name": "Alice", "createdAt": "2024-01-15T10:30:00Z", "updatedAt": "2024-01-15T10:30:00Z"}
```

Is returned as:
```json
{"id": "cus_123", "name": "Alice", "created": 1705312200, "updated": 1705312200}
```

### Fields

Controls field-level modifications applied to every item response.

#### Inject

Adds static key-value pairs to every response. Values are literals — strings, numbers, booleans, nulls, or nested objects.

```yaml
response:
  fields:
    inject:
      object: customer
      livemode: false
      api_version: "2024-01-01"
```

Every item response from this table will include `"object": "customer"`, `"livemode": false`, and `"api_version": "2024-01-01"` regardless of what's stored in the table.

#### Hide

Removes fields from responses. The data is still stored in the table — it's just not returned to clients. Useful for hiding internal fields or auto-generated fields you don't want exposed.

```yaml
response:
  fields:
    hide:
      - updatedAt
      - _internalNotes
      - metadata
```

#### Rename

Changes field keys in responses without modifying stored data. Key is the original field name, value is the output field name.

```yaml
response:
  fields:
    rename:
      firstName: first_name
      lastName: last_name
      emailAddress: email
```

#### WrapAsList

Wraps specified array fields in list object envelopes. This is essential for APIs like Stripe that represent nested collections as list objects (`{object: "list", data: [...], has_more: false}`) instead of plain arrays.

```yaml
response:
  fields:
    wrapAsList:
      items:
        url: "/v1/subscriptions/{{id}}/items"
      lines:
        url: "/v1/invoices/{{id}}/lines"
```

The `url` field supports `{{fieldName}}` template substitution from the parent item. For example, if the parent item has `id: "sub_123"`, the URL becomes `/v1/subscriptions/sub_123/items`.

A stored array field like:
```json
{"id": "sub_123", "items": [{"price": "price_gold"}]}
```

Is returned as:
```json
{
  "id": "sub_123",
  "items": {
    "object": "list",
    "data": [{"price": "price_gold"}],
    "has_more": false,
    "url": "/v1/subscriptions/sub_123/items"
  }
}
```

Set the value to `null` (or omit the `url`) for a plain list wrapper without a URL.

### List Envelope

Controls the shape of list (collection) responses. By default, mockd returns `{"data": [...], "meta": {"total": N, "limit": 100, "offset": 0, "count": N}}`. The list transform lets you customize the envelope to match any API convention.

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `dataField` | string | `"data"` | Key for the items array |
| `extraFields` | map | `{}` | Static fields injected into the list envelope (not into items) |
| `metaFields` | map | `{}` | Rename pagination meta keys (`total`, `limit`, `offset`, `count`) |
| `hideMeta` | boolean | `false` | Omit pagination metadata entirely |

**Example: Stripe-style list envelope:**

Stripe lists look like `{"object": "list", "data": [...], "has_more": true, "url": "/v1/customers"}` with no separate meta object.

```yaml
response:
  list:
    dataField: data
    extraFields:
      object: list
      url: /v1/customers
      has_more: false
    hideMeta: true
```

The `has_more` field is special: when included in `extraFields`, its value is dynamically computed from the pagination state rather than using the static value. This means `has_more` will be `true` when there are more items beyond the current page and `false` otherwise.

All other `extraFields` values are static and passed through as-is, including `null` values. For example, Twilio-style pagination fields like `next_page_uri: null` are included in the response exactly as configured.

**Example: Custom meta field names:**

```yaml
response:
  list:
    metaFields:
      total: total_count
      count: page_size
```

Returns:
```json
{
  "data": [...],
  "meta": {
    "total_count": 45,
    "limit": 10,
    "offset": 0,
    "page_size": 10
  }
}
```

**Example: Twilio-style envelope:**

```yaml
response:
  list:
    dataField: results
    extraFields:
      page: 0
      page_size: 50
    metaFields:
      total: total
    hideMeta: false
```

### Verb Overrides

Customize the HTTP status code and response body for create and delete operations.

#### Create Override

By default, create operations return HTTP 201. Some APIs (like Stripe) return 200 for creates.

```yaml
response:
  create:
    status: 200
```

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `status` | integer | `201` | HTTP status code for create responses |

#### Delete Override

By default, delete operations return HTTP 204 with no body. Transforms let you customize the status code, return a response body, and optionally preserve the item (soft delete).

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `status` | integer | `204` | HTTP status code for delete responses |
| `body` | map | `nil` | Response body template (supports `{{item.fieldName}}` substitution) |
| `preserve` | boolean | `false` | When true, the item is NOT removed from the store (soft delete) |

**Example: Stripe-style soft delete with confirmation body:**

Stripe's DELETE endpoints return 200 with a JSON body confirming what was deleted, and the item remains accessible (soft delete):

```yaml
response:
  delete:
    status: 200
    preserve: true
    body:
      id: "{{item.id}}"
      object: customer
      deleted: true
```

When you `DELETE /v1/customers/cus_123`, this returns:

```json
{
  "id": "cus_123",
  "object": "customer",
  "deleted": true
}
```

The `{{item.fieldName}}` syntax substitutes values from the item being deleted. The item data is read before the delete action, so all fields are available for template substitution. Only string values in the body template are processed for `{{item.*}}` patterns — non-string values (booleans, numbers) are returned as-is.

When `preserve: true`, the item remains in the table after the delete response is sent. This is useful for APIs that use soft-delete semantics where deleted resources can still be retrieved.

### Error Transforms

Customize the shape of error responses to match your target API's error format. Without error transforms, mockd returns its standard format:

```json
{"error": "not found", "resource": "customers", "id": "cus_999", "statusCode": 404}
```

With error transforms, you can match any API's error convention.

| Field | Type | Description |
|-------|------|-------------|
| `wrap` | string | Nest the error object under a key (e.g., `"error"` produces `{"error": {...}}`) |
| `fields` | map | Map mockd error fields to custom field names |
| `inject` | map | Add static fields to every error response |
| `typeMap` | map | Map mockd error codes to custom type strings |
| `codeMap` | map | Map mockd error codes to custom code strings |

**Available source fields for `fields` mapping:** `message`, `code`, `type`, `resource`, `id`, `field`

**Available error codes for `typeMap` and `codeMap`:** `NOT_FOUND`, `CONFLICT`, `VALIDATION_ERROR`, `CAPACITY_EXCEEDED`, `INTERNAL_ERROR`

**Example: Stripe-style error format:**

```yaml
response:
  errors:
    wrap: error
    fields:
      message: message
      type: type
      code: code
    typeMap:
      NOT_FOUND: invalid_request_error
      CONFLICT: invalid_request_error
      VALIDATION_ERROR: invalid_request_error
      CAPACITY_EXCEEDED: api_error
      INTERNAL_ERROR: api_error
    codeMap:
      NOT_FOUND: resource_missing
      CONFLICT: resource_already_exists
      VALIDATION_ERROR: parameter_invalid
```

A 404 error for `GET /v1/customers/cus_nonexistent` returns:

```json
{
  "error": {
    "message": "not found",
    "type": "invalid_request_error",
    "code": "resource_missing"
  }
}
```

You can also inject static fields into every error:

```yaml
response:
  errors:
    wrap: error
    inject:
      doc_url: "https://docs.example.com/errors"
      request_log_url: "https://dashboard.example.com/logs"
```

### Complete Example: Stripe Digital Twin Transform

This is the full response transform used by the Stripe digital twin. It shows all transform features working together with YAML anchors for reuse across tables:

:::tip[YAML Anchors for Reuse]
Both the Stripe and Twilio digital twin configs use **YAML anchors** (`&name`) and **aliases** (`*name`) to define shared transforms once and reuse them across tables. This is standard YAML — not a mockd feature — but it's the recommended pattern for avoiding duplication in configs with multiple tables that share the same API conventions.
:::

The `x-` prefixed keys (like `x-stripe-timestamps`) are **ignored by mockd** — they exist solely as YAML anchor hosts. Any top-level key starting with `x-` is treated as a comment/extension and not validated or processed.

```yaml
# YAML anchors for reuse across tables
x-stripe-timestamps: &stripe-timestamps
  format: unix
  fields:
    createdAt: created
    updatedAt: updated

x-stripe-hide: &stripe-hide
  - updatedAt

x-stripe-errors: &stripe-errors
  wrap: error
  fields:
    message: message
    type: type
    code: code
  typeMap:
    NOT_FOUND: invalid_request_error
    CONFLICT: invalid_request_error
    VALIDATION_ERROR: invalid_request_error
    CAPACITY_EXCEEDED: api_error
    INTERNAL_ERROR: api_error
  codeMap:
    NOT_FOUND: resource_missing
    CONFLICT: resource_already_exists
    VALIDATION_ERROR: parameter_invalid

tables:
  - name: customers
    idField: id
    idStrategy: prefix
    idPrefix: "cus_"
    seedData:
      - { id: "cus_123", name: "Jenny Rosen", email: "jenny.rosen@example.com" }
    response:
      timestamps: *stripe-timestamps
      fields:
        inject:
          object: customer
          livemode: false
        hide: *stripe-hide
      list:
        dataField: data
        extraFields:
          object: list
          url: /v1/customers
          has_more: false
        hideMeta: true
      create:
        status: 200
      delete:
        status: 200
        preserve: true
        body:
          id: "{{item.id}}"
          object: customer
          deleted: true
      errors: *stripe-errors
```

With this transform, mockd responses are indistinguishable from the real Stripe API:

- `GET /v1/customers` returns `{"object":"list","data":[...],"has_more":false,"url":"/v1/customers"}`
- `GET /v1/customers/cus_123` returns `{"id":"cus_123","object":"customer","created":1705312200,"livemode":false,...}`
- `POST /v1/customers` returns 200 (not 201) with the created customer
- `DELETE /v1/customers/cus_123` returns 200 with `{"id":"cus_123","object":"customer","deleted":true}`
- `GET /v1/customers/cus_nonexistent` returns `{"error":{"type":"invalid_request_error","code":"resource_missing","message":"not found"}}`

### Table-Level vs Binding-Level Transforms

When you need different response shapes for different endpoints on the same table, use binding-level overrides:

```yaml
tables:
  - name: customers
    response:
      # Table default: full transform
      timestamps:
        format: unix
        fields: { createdAt: created }
      fields:
        inject: { object: customer }

extend:
  # Standard endpoints use the table default
  - { mock: get-customer, table: customers, action: get }
  - { mock: list-customers, table: customers, action: list }

  # This endpoint needs a different list envelope
  - mock: search-customers
    table: customers
    action: list
    response:
      timestamps:
        format: unix
        fields: { createdAt: created }
      fields:
        inject: { object: customer }
      list:
        dataField: data
        extraFields:
          object: search_result
          url: /v1/customers/search
          has_more: false
        hideMeta: true
```

Note that a binding-level override **replaces** the entire table default — it does not merge with it. If you want the same timestamp and field transforms, you must repeat them in the binding override.

## Custom Operations via Extend

Extend bindings support `action: custom` to trigger multi-step custom operations:

```yaml
tables:
  - name: accounts
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

For config-driven workflows, `mockd start` and `mockd serve` are interchangeable. `start` adds `--load` (directory loading) and `--watch` (auto-reload) flags. Both accept `--config` / `-c` for config files and `--detach` / `-d` for daemon mode.

## Next Steps

- [SOAP Mocking](/protocols/soap/) - Full SOAP protocol guide with WSDL import
- [Proxy Recording](/guides/proxy-recording/) - Record real API traffic
- [Admin API Reference](/reference/admin-api/) - State management endpoints
- [Configuration Reference](/reference/configuration/) - Full schema
