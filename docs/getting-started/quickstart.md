# Quickstart

Get your first mock API running in under 5 minutes.

## Prerequisites

- mockd installed ([Installation Guide](installation.md))
- A terminal
- curl or any HTTP client

## Step 1: Create a Mock Configuration

Create a file called `mocks.json` in your current directory:

```json
{
  "mocks": [
    {
      "request": {
        "method": "GET",
        "path": "/api/hello"
      },
      "response": {
        "status": 200,
        "headers": {
          "Content-Type": "application/json"
        },
        "body": {
          "message": "Hello, World!"
        }
      }
    }
  ]
}
```

## Step 2: Start the Mock Server

```bash
mockd start --config mocks.json
```

You should see output like:

```
mockd server starting...
Listening on http://localhost:8080
Loaded 1 mock(s) from mocks.json
```

## Step 3: Test Your Mock

Open a new terminal and make a request:

```bash
curl http://localhost:8080/api/hello
```

Response:

```json
{"message": "Hello, World!"}
```

Congratulations! You've created your first mock API.

---

## Adding More Mocks

Let's add a more realistic API. Update `mocks.json`:

```json
{
  "mocks": [
    {
      "request": {
        "method": "GET",
        "path": "/api/users"
      },
      "response": {
        "status": 200,
        "headers": {
          "Content-Type": "application/json"
        },
        "body": {
          "users": [
            {"id": 1, "name": "Alice", "email": "alice@example.com"},
            {"id": 2, "name": "Bob", "email": "bob@example.com"}
          ]
        }
      }
    },
    {
      "request": {
        "method": "GET",
        "path": "/api/users/1"
      },
      "response": {
        "status": 200,
        "headers": {
          "Content-Type": "application/json"
        },
        "body": {
          "id": 1,
          "name": "Alice",
          "email": "alice@example.com"
        }
      }
    },
    {
      "request": {
        "method": "POST",
        "path": "/api/users"
      },
      "response": {
        "status": 201,
        "headers": {
          "Content-Type": "application/json"
        },
        "body": {
          "id": 3,
          "message": "User created"
        }
      }
    },
    {
      "request": {
        "method": "GET",
        "path": "/api/users/999"
      },
      "response": {
        "status": 404,
        "headers": {
          "Content-Type": "application/json"
        },
        "body": {
          "error": "User not found"
        }
      }
    }
  ]
}
```

Restart the server (Ctrl+C to stop, then start again):

```bash
mockd start --config mocks.json
```

Test the new endpoints:

```bash
# List users
curl http://localhost:8080/api/users

# Get single user
curl http://localhost:8080/api/users/1

# Create user
curl -X POST http://localhost:8080/api/users

# Not found
curl http://localhost:8080/api/users/999
```

---

## Using Path Parameters

Match dynamic path segments with patterns:

```json
{
  "request": {
    "method": "GET",
    "path": "/api/users/{id}"
  },
  "response": {
    "status": 200,
    "body": {
      "id": "{{request.pathParams.id}}",
      "name": "Dynamic User"
    }
  }
}
```

---

## Adding Delays

Simulate network latency:

```json
{
  "request": {
    "method": "GET",
    "path": "/api/slow"
  },
  "response": {
    "status": 200,
    "delay": "500ms",
    "body": {
      "message": "This took a while"
    }
  }
}
```

---

## Changing the Port

Use a different port:

```bash
mockd start --config mocks.json --port 3000
```

Or in the config:

```json
{
  "server": {
    "port": 3000
  },
  "mocks": [...]
}
```

---

## What's Next?

- **[Core Concepts](concepts.md)** - Understand mocks, matching, and responses
- **[Request Matching](../guides/request-matching.md)** - Advanced matching patterns
- **[Stateful Mocking](../guides/stateful-mocking.md)** - Simulate CRUD APIs
- **[CLI Reference](../reference/cli.md)** - All command-line options
