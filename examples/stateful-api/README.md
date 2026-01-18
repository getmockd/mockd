# Stateful API Example

This example demonstrates mockd's stateful mocking capabilities, allowing you to simulate a complete CRUD API with persistent in-memory state.

## Features Demonstrated

- **Stateful Resources**: Users, Products, and Orders with full CRUD operations
- **Nested Resources**: Orders scoped to users (`/api/users/:userId/orders`)
- **Seed Data**: Pre-populated data that loads on server start
- **Filtering & Pagination**: Query parameters for filtering and paginating results
- **State Management**: Reset state via Admin API

## Quick Start

```bash
# Start the server with this configuration
mockd start --config examples/stateful-api/config.json

# Or run in the background
mockd start --config examples/stateful-api/config.json &
```

## API Endpoints

### Users (`/api/users`)

```bash
# List all users
curl http://localhost:4280/api/users

# Get a specific user
curl http://localhost:4280/api/users/user-001

# Create a new user
curl -X POST http://localhost:4280/api/users \
  -H "Content-Type: application/json" \
  -d '{"name": "New User", "email": "new@example.com", "role": "user"}'

# Update a user
curl -X PUT http://localhost:4280/api/users/user-001 \
  -H "Content-Type: application/json" \
  -d '{"name": "Alice Updated", "email": "alice@example.com", "role": "admin"}'

# Delete a user
curl -X DELETE http://localhost:4280/api/users/user-003
```

### Products (`/api/products`)

```bash
# List all products
curl http://localhost:4280/api/products

# Filter by category
curl "http://localhost:4280/api/products?category=electronics"

# Sort by price (descending)
curl "http://localhost:4280/api/products?sort=price&order=desc"

# Pagination
curl "http://localhost:4280/api/products?limit=2&offset=0"
```

### Orders (Nested under Users)

```bash
# List orders for a specific user
curl http://localhost:4280/api/users/user-001/orders

# Create an order for a user (userId is auto-set from path)
curl -X POST http://localhost:4280/api/users/user-001/orders \
  -H "Content-Type: application/json" \
  -d '{"productId": "prod-001", "quantity": 3, "status": "pending"}'

# Get a specific order
curl http://localhost:4280/api/users/user-001/orders/order-001
```

## Admin API

### State Overview

```bash
# Get state overview
curl http://localhost:4290/state

# List all stateful resources
curl http://localhost:4290/state/resources

# Get details for a specific resource
curl http://localhost:4290/state/resources/users
```

### State Reset

```bash
# Reset all resources to seed data
curl -X POST http://localhost:4290/state/reset

# Reset only a specific resource
curl -X POST "http://localhost:4290/state/reset?resource=users"

# Clear a resource (removes all items, no seed data restore)
curl -X DELETE http://localhost:4290/state/resources/products
```

## Response Format

### Collection Response (Paginated)

```json
{
  "data": [
    {
      "id": "user-001",
      "name": "Alice Johnson",
      "email": "alice@example.com",
      "createdAt": "2025-01-01T00:00:00Z",
      "updatedAt": "2025-01-01T00:00:00Z"
    }
  ],
  "meta": {
    "total": 3,
    "limit": 100,
    "offset": 0,
    "count": 3
  }
}
```

### Single Resource Response

```json
{
  "id": "user-001",
  "name": "Alice Johnson",
  "email": "alice@example.com",
  "role": "admin",
  "active": true,
  "createdAt": "2025-01-01T00:00:00Z",
  "updatedAt": "2025-01-01T00:00:00Z"
}
```

## Configuration Structure

```json
{
  "statefulResources": [
    {
      "name": "resourceName",
      "basePath": "/api/path",
      "idField": "id",
      "parentField": "parentId",
      "seedData": [...]
    }
  ]
}
```

| Field | Description |
|-------|-------------|
| `name` | Unique resource identifier |
| `basePath` | URL path prefix (supports `:param` for nested resources) |
| `idField` | Field name for item ID (default: "id") |
| `parentField` | Field name for parent FK in nested resources |
| `seedData` | Initial data loaded on startup/reset |
