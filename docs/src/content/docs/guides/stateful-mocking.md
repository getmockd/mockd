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

Enable stateful mocking in your configuration:

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
# Create a user
curl -X POST http://localhost:4280/api/users \
  -H "Content-Type: application/json" \
  -d '{"name": "Alice", "email": "alice@example.com"}'
# Response: {"id": 1, "name": "Alice", "email": "alice@example.com"}

# List users - Alice is now in the list
curl http://localhost:4280/api/users
# Response: [{"id": 1, "name": "Alice", "email": "alice@example.com"}]

# Get single user
curl http://localhost:4280/api/users/1
# Response: {"id": 1, "name": "Alice", "email": "alice@example.com"}

# Update user
curl -X PUT http://localhost:4280/api/users/1 \
  -H "Content-Type: application/json" \
  -d '{"name": "Alice Smith", "email": "alice@example.com"}'
# Response: {"id": 1, "name": "Alice Smith", "email": "alice@example.com"}

# Delete user
curl -X DELETE http://localhost:4280/api/users/1
# Response: 204 No Content

# User is gone
curl http://localhost:4280/api/users/1
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
| `basePath` | URL path prefix for the resource | Required |
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

Response:
```json
{
  "id": 3,
  "name": "Charlie",
  "email": "charlie@example.com"
}
```

Status: `201 Created`

### Read Collection (GET)

```bash
GET /api/users
```

Response:
```json
[
  {"id": 1, "name": "Alice"},
  {"id": 2, "name": "Bob"},
  {"id": 3, "name": "Charlie"}
]
```

### Read Single (GET)

```bash
GET /api/users/2
```

Response:
```json
{"id": 2, "name": "Bob", "email": "bob@example.com"}
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
{"id": 2, "name": "Robert", "email": "robert@example.com"}
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
# Get state overview
GET /state

# Reset all state to seed data
POST /state/reset

# List all resources
GET /state/resources

# Get specific resource info
GET /state/resources/users

# Clear specific resource (remove all items)
DELETE /state/resources/users
```

## Combined with Static Mocks

Stateful resources work alongside traditional mocks:

```json
{
  "mocks": [
    {
      "request": {"method": "GET", "path": "/api/health"},
      "response": {"status": 200, "body": {"status": "ok"}}
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

## Next Steps

- [Proxy Recording](/guides/proxy-recording/) - Record real API traffic
- [Admin API Reference](/reference/admin-api/) - State management endpoints
- [Configuration Reference](/reference/configuration/) - Full schema
