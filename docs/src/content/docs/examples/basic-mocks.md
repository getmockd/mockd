---
title: Basic Mocks Examples
description: Simple examples to get started with mockd request/response mocking.
---

Simple examples to get started with mockd request/response mocking. Each config below can be saved as a YAML file and loaded with `mockd serve --config mocks.yaml`.

## Hello World

The simplest possible mock:

```yaml
version: "1.0"
mocks:
  - id: hello
    type: http
    http:
      matcher:
        method: GET
        path: /hello
      response:
        statusCode: 200
        body: "Hello, World!"
```

Test:

```bash
curl http://localhost:4280/hello
# Hello, World!
```

## JSON Response

Return JSON data:

```yaml
version: "1.0"
mocks:
  - id: get-user
    type: http
    http:
      matcher:
        method: GET
        path: /api/user
      response:
        statusCode: 200
        headers:
          Content-Type: application/json
        body: '{"id": 1, "name": "Alice", "email": "alice@example.com", "roles": ["user", "admin"]}'
```

## Multiple Endpoints

Mock a simple API:

```yaml
version: "1.0"
mocks:
  - id: list-products
    name: List products
    type: http
    http:
      matcher:
        method: GET
        path: /api/products
      response:
        statusCode: 200
        body: '{"products": [{"id": 1, "name": "Widget", "price": 9.99}, {"id": 2, "name": "Gadget", "price": 19.99}]}'

  - id: get-product
    name: Get product
    type: http
    http:
      matcher:
        method: GET
        path: /api/products/1
      response:
        statusCode: 200
        body: '{"id": 1, "name": "Widget", "price": 9.99, "description": "A useful widget"}'

  - id: product-not-found
    name: Product not found
    type: http
    http:
      matcher:
        method: GET
        path: /api/products/999
      response:
        statusCode: 404
        body: '{"error": "Product not found"}'
```

## Path Parameters

Match dynamic paths:

```yaml
version: "1.0"
mocks:
  - id: get-user-by-id
    type: http
    http:
      matcher:
        method: GET
        path: /api/users/{id}
      response:
        statusCode: 200
        body: '{"id": "{{request.pathParam.id}}", "name": "User {{request.pathParam.id}}"}'
```

Test:

```bash
curl http://localhost:4280/api/users/42
# {"id": "42", "name": "User 42"}

curl http://localhost:4280/api/users/abc
# {"id": "abc", "name": "User abc"}
```

## Query Parameters

Match and use query params:

```yaml
version: "1.0"
mocks:
  - id: search
    type: http
    http:
      matcher:
        method: GET
        path: /api/search
      response:
        statusCode: 200
        body: '{"query": "{{request.query.q}}", "results": []}'
```

This mock matches any GET to `/api/search` regardless of query parameters. The template `{{request.query.q}}` extracts the `q` parameter from the request.

Test:

```bash
curl "http://localhost:4280/api/search?q=hello"
# {"query": "hello", "results": []}
```

## Header Matching

Require specific headers:

```yaml
version: "1.0"
mocks:
  - id: authenticated
    name: Authenticated request
    type: http
    http:
      priority: 10
      matcher:
        method: GET
        path: /api/protected
        headers:
          Authorization: "Bearer valid-token"
      response:
        statusCode: 200
        body: '{"message": "Access granted"}'

  - id: unauthorized
    name: Unauthorized
    type: http
    http:
      matcher:
        method: GET
        path: /api/protected
      response:
        statusCode: 401
        body: '{"error": "Unauthorized"}'
```

Test:

```bash
curl http://localhost:4280/api/protected
# {"error": "Unauthorized"}

curl -H "Authorization: Bearer valid-token" http://localhost:4280/api/protected
# {"message": "Access granted"}
```

## POST with Body

Handle POST requests:

```yaml
version: "1.0"
mocks:
  - id: create-user
    type: http
    http:
      matcher:
        method: POST
        path: /api/users
      response:
        statusCode: 201
        headers:
          Location: "/api/users/{{uuid}}"
        body: '{"id": "{{uuid}}", "name": "{{request.body.name}}", "email": "{{request.body.email}}", "createdAt": "{{now}}"}'
```

Test:

```bash
curl -X POST http://localhost:4280/api/users \
  -H "Content-Type: application/json" \
  -d '{"name": "Bob", "email": "bob@example.com"}'
# {"id": "x7k9m2", "name": "Bob", "email": "bob@example.com", "createdAt": "2024-01-15T10:30:00Z"}
```

## Simulated Delay

Add latency to responses:

```yaml
version: "1.0"
mocks:
  - id: slow-endpoint
    type: http
    http:
      matcher:
        method: GET
        path: /api/slow
      response:
        statusCode: 200
        delayMs: 2000
        body: '{"message": "Finally!"}'
```

## Error Responses

Mock various error scenarios:

```yaml
version: "1.0"
mocks:
  - id: error-400
    type: http
    http:
      matcher:
        method: GET
        path: /api/error/400
      response:
        statusCode: 400
        body: '{"error": "Bad Request", "message": "Invalid parameters"}'

  - id: error-500
    type: http
    http:
      matcher:
        method: GET
        path: /api/error/500
      response:
        statusCode: 500
        body: '{"error": "Internal Server Error", "message": "Something went wrong"}'

  - id: error-503
    type: http
    http:
      matcher:
        method: GET
        path: /api/error/503
      response:
        statusCode: 503
        headers:
          Retry-After: "30"
        body: '{"error": "Service Unavailable"}'
```

## File-Based Response

Load response body from file:

```yaml
version: "1.0"
mocks:
  - id: large-data
    type: http
    http:
      matcher:
        method: GET
        path: /api/large-data
      response:
        statusCode: 200
        headers:
          Content-Type: application/json
        bodyFile: ./responses/large-data.json
```

## Complete Example

A realistic API mock:

```yaml
version: "1.0"
mocks:
  - id: health-check
    name: Health check
    type: http
    http:
      matcher:
        method: GET
        path: /health
      response:
        statusCode: 200
        body: '{"status": "ok"}'

  - id: list-users
    name: List users
    type: http
    http:
      matcher:
        method: GET
        path: /api/v1/users
      response:
        statusCode: 200
        body: '{"data": [{"id": 1, "name": "Alice"}, {"id": 2, "name": "Bob"}], "meta": {"total": 2, "page": 1}}'

  - id: get-user-by-id
    name: Get user by ID
    type: http
    http:
      matcher:
        method: GET
        path: /api/v1/users/{id}
      response:
        statusCode: 200
        body: '{"id": "{{request.pathParam.id}}", "name": "User {{request.pathParam.id}}", "email": "user{{request.pathParam.id}}@example.com"}'

  - id: create-user
    name: Create user
    type: http
    http:
      matcher:
        method: POST
        path: /api/v1/users
      response:
        statusCode: 201
        body: '{"id": "{{uuid}}", "name": "{{request.body.name}}", "createdAt": "{{now}}"}'

  - id: delete-user
    name: Delete user
    type: http
    http:
      matcher:
        method: DELETE
        path: /api/v1/users/{id}
      response:
        statusCode: 204
```

## Next Steps

- [CRUD API Example](/examples/crud-api) - Stateful CRUD simulation
- [Integration Testing](/examples/integration-testing) - Using mocks in tests
- [Request Matching](/guides/request-matching) - Advanced matching
