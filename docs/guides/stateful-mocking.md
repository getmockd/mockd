# Stateful Mocking

Stateful mocking allows mockd to simulate real CRUD APIs where resources persist across requests. Create, update, and delete operations modify state that subsequent requests can observe.

## Overview

Traditional mocks return static responses. Stateful mocking maintains an in-memory store that:

- **POST** creates new resources
- **GET** retrieves current resources
- **PUT/PATCH** updates existing resources
- **DELETE** removes resources

Changes persist for the lifetime of the server session.

## Quick Start

Enable stateful mocking in your configuration:

```json
{
  "stateful": {
    "resources": {
      "users": {
        "collection": "/api/users",
        "item": "/api/users/{id}",
        "idField": "id",
        "autoId": true
      }
    }
  }
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

| Field | Description | Default |
|-------|-------------|---------|
| `collection` | Path for list/create operations | Required |
| `item` | Path for single item operations | Required |
| `idField` | Field name for resource ID | `"id"` |
| `autoId` | Auto-generate IDs on create | `true` |

### Multiple Resources

```json
{
  "stateful": {
    "resources": {
      "users": {
        "collection": "/api/users",
        "item": "/api/users/{id}"
      },
      "posts": {
        "collection": "/api/posts",
        "item": "/api/posts/{id}"
      },
      "comments": {
        "collection": "/api/posts/{postId}/comments",
        "item": "/api/posts/{postId}/comments/{id}"
      }
    }
  }
}
```

### Initial Data (Seeding)

Pre-populate resources:

```json
{
  "stateful": {
    "resources": {
      "users": {
        "collection": "/api/users",
        "item": "/api/users/{id}",
        "seed": [
          {"id": 1, "name": "Alice", "email": "alice@example.com"},
          {"id": 2, "name": "Bob", "email": "bob@example.com"}
        ]
      }
    }
  }
}
```

### Seed from File

```json
{
  "stateful": {
    "resources": {
      "users": {
        "collection": "/api/users",
        "item": "/api/users/{id}",
        "seedFile": "./data/users.json"
      }
    }
  }
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

### Partial Update (PATCH)

Update specific fields:

```bash
PATCH /api/users/2
Content-Type: application/json

{"email": "bob.new@example.com"}
```

Response:
```json
{"id": 2, "name": "Bob", "email": "bob.new@example.com"}
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
  "stateful": {
    "resources": {
      "posts": {
        "collection": "/api/posts",
        "item": "/api/posts/{id}"
      },
      "comments": {
        "collection": "/api/posts/{postId}/comments",
        "item": "/api/posts/{postId}/comments/{id}",
        "parentRef": "postId"
      }
    }
  }
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

```bash
GET /api/users?name=Alice
GET /api/users?status=active
```

Enable filtering:

```json
{
  "stateful": {
    "resources": {
      "users": {
        "collection": "/api/users",
        "item": "/api/users/{id}",
        "filtering": true
      }
    }
  }
}
```

### Pagination

```bash
GET /api/users?page=2&limit=10
```

Enable pagination:

```json
{
  "stateful": {
    "resources": {
      "users": {
        "collection": "/api/users",
        "item": "/api/users/{id}",
        "pagination": {
          "pageParam": "page",
          "limitParam": "limit",
          "defaultLimit": 20
        }
      }
    }
  }
}
```

Response includes pagination metadata:

```json
{
  "data": [...],
  "pagination": {
    "page": 2,
    "limit": 10,
    "total": 45,
    "totalPages": 5
  }
}
```

## State Persistence

By default, state exists only in memory and resets when the server stops.

### File Persistence

Save state to disk:

```json
{
  "stateful": {
    "persistence": {
      "enabled": true,
      "file": "./mockd-state.json",
      "saveInterval": "30s"
    }
  }
}
```

State is loaded on startup and saved periodically.

## Admin API

Manage state via the admin API:

```bash
# Get current state
GET /state

# Reset all state
DELETE /state

# Reset specific resource
DELETE /state/users

# Import state
POST /state
{"users": [...], "posts": [...]}
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

Static mocks take priority when matched.

## Complete Example

```json
{
  "server": {
    "port": 4280
  },
  "stateful": {
    "resources": {
      "users": {
        "collection": "/api/users",
        "item": "/api/users/{id}",
        "idField": "id",
        "autoId": true,
        "seed": [
          {"id": 1, "name": "Admin", "role": "admin"}
        ]
      },
      "posts": {
        "collection": "/api/posts",
        "item": "/api/posts/{id}",
        "filtering": true,
        "pagination": {
          "defaultLimit": 10
        }
      }
    },
    "persistence": {
      "enabled": true,
      "file": "./state.json"
    }
  }
}
```

## Next Steps

- [Proxy Recording](proxy-recording.md) - Record real API traffic
- [Admin API Reference](../reference/admin-api.md) - State management endpoints
- [Configuration Reference](../reference/configuration.md) - Full schema
