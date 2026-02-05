---
title: Basic Mocks Examples
description: Simple examples to get started with mockd request/response mocking.
---

Simple examples to get started with mockd request/response mocking.

## Hello World

The simplest possible mock:

```json
{
  "mocks": [
    {
      "request": {
        "method": "GET",
        "path": "/hello"
      },
      "response": {
        "status": 200,
        "body": "Hello, World!"
      }
    }
  ]
}
```

Test:

```bash
curl http://localhost:4280/hello
# Hello, World!
```

## JSON Response

Return JSON data:

```json
{
  "mocks": [
    {
      "request": {
        "method": "GET",
        "path": "/api/user"
      },
      "response": {
        "status": 200,
        "headers": {
          "Content-Type": "application/json"
        },
        "body": {
          "id": 1,
          "name": "Alice",
          "email": "alice@example.com",
          "roles": ["user", "admin"]
        }
      }
    }
  ]
}
```

## Multiple Endpoints

Mock a simple API:

```json
{
  "mocks": [
    {
      "name": "List products",
      "request": {
        "method": "GET",
        "path": "/api/products"
      },
      "response": {
        "status": 200,
        "body": {
          "products": [
            {"id": 1, "name": "Widget", "price": 9.99},
            {"id": 2, "name": "Gadget", "price": 19.99}
          ]
        }
      }
    },
    {
      "name": "Get product",
      "request": {
        "method": "GET",
        "path": "/api/products/1"
      },
      "response": {
        "status": 200,
        "body": {
          "id": 1,
          "name": "Widget",
          "price": 9.99,
          "description": "A useful widget"
        }
      }
    },
    {
      "name": "Product not found",
      "request": {
        "method": "GET",
        "path": "/api/products/999"
      },
      "response": {
        "status": 404,
        "body": {
          "error": "Product not found"
        }
      }
    }
  ]
}
```

## Path Parameters

Match dynamic paths:

```json
{
  "mocks": [
    {
      "request": {
        "method": "GET",
        "path": "/api/users/{id}"
      },
      "response": {
        "status": 200,
        "body": {
          "id": "{{request.pathParam.id}}",
          "name": "User {{request.pathParam.id}}"
        }
      }
    }
  ]
}
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

```json
{
  "mocks": [
    {
      "request": {
        "method": "GET",
        "path": "/api/search",
        "query": {
          "q": ".*"
        }
      },
      "response": {
        "status": 200,
        "body": {
          "query": "{{request.query.q}}",
          "results": []
        }
      }
    }
  ]
}
```

Test:

```bash
curl "http://localhost:4280/api/search?q=hello"
# {"query": "hello", "results": []}
```

## Header Matching

Require specific headers:

```json
{
  "mocks": [
    {
      "name": "Authenticated request",
      "request": {
        "method": "GET",
        "path": "/api/protected",
        "headers": {
          "Authorization": "Bearer valid-token"
        }
      },
      "response": {
        "status": 200,
        "body": {"message": "Access granted"}
      }
    },
    {
      "name": "Unauthorized",
      "request": {
        "method": "GET",
        "path": "/api/protected"
      },
      "response": {
        "status": 401,
        "body": {"error": "Unauthorized"}
      }
    }
  ]
}
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

```json
{
  "mocks": [
    {
      "request": {
        "method": "POST",
        "path": "/api/users"
      },
      "response": {
        "status": 201,
        "headers": {
          "Location": "/api/users/{{uuid}}"
        },
        "body": {
          "id": "{{uuid}}",
          "name": "{{request.body.name}}",
          "email": "{{request.body.email}}",
          "createdAt": "{{now}}"
        }
      }
    }
  ]
}
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

```json
{
  "mocks": [
    {
      "request": {
        "method": "GET",
        "path": "/api/slow"
      },
      "response": {
        "status": 200,
        "delay": "2s",
        "body": {"message": "Finally!"}
      }
    }
  ]
}
```

## Error Responses

Mock various error scenarios:

```json
{
  "mocks": [
    {
      "request": {
        "method": "GET",
        "path": "/api/error/400"
      },
      "response": {
        "status": 400,
        "body": {
          "error": "Bad Request",
          "message": "Invalid parameters"
        }
      }
    },
    {
      "request": {
        "method": "GET",
        "path": "/api/error/500"
      },
      "response": {
        "status": 500,
        "body": {
          "error": "Internal Server Error",
          "message": "Something went wrong"
        }
      }
    },
    {
      "request": {
        "method": "GET",
        "path": "/api/error/503"
      },
      "response": {
        "status": 503,
        "headers": {
          "Retry-After": "30"
        },
        "body": {
          "error": "Service Unavailable"
        }
      }
    }
  ]
}
```

## File-Based Response

Load response body from file:

```json
{
  "mocks": [
    {
      "request": {
        "method": "GET",
        "path": "/api/large-data"
      },
      "response": {
        "status": 200,
        "headers": {
          "Content-Type": "application/json"
        },
        "bodyFile": "./responses/large-data.json"
      }
    }
  ]
}
```

## Complete Example

A realistic API mock:

```json
{
  "server": {
    "port": 4280
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
      "name": "List users",
      "request": {
        "method": "GET",
        "path": "/api/v1/users"
      },
      "response": {
        "status": 200,
        "body": {
          "data": [
            {"id": 1, "name": "Alice"},
            {"id": 2, "name": "Bob"}
          ],
          "meta": {
            "total": 2,
            "page": 1
          }
        }
      }
    },
    {
      "name": "Get user by ID",
      "request": {
        "method": "GET",
        "path": "/api/v1/users/{id}"
      },
      "response": {
        "status": 200,
        "body": {
          "id": "{{request.pathParam.id}}",
          "name": "User {{request.pathParam.id}}",
          "email": "user{{request.pathParam.id}}@example.com"
        }
      }
    },
    {
      "name": "Create user",
      "request": {
        "method": "POST",
        "path": "/api/v1/users"
      },
      "response": {
        "status": 201,
        "body": {
          "id": "{{uuid}}",
          "name": "{{request.body.name}}",
          "createdAt": "{{now}}"
        }
      }
    },
    {
      "name": "Delete user",
      "request": {
        "method": "DELETE",
        "path": "/api/v1/users/{id}"
      },
      "response": {
        "status": 204
      }
    }
  ]
}
```

## Next Steps

- [CRUD API Example](/examples/crud-api) - Stateful CRUD simulation
- [Integration Testing](/examples/integration-testing) - Using mocks in tests
- [Request Matching](/guides/request-matching) - Advanced matching
