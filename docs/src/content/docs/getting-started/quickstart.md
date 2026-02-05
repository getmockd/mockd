---
title: Quickstart
description: Get your first mock API running in under 5 minutes
---

Get your first mock API running in under 5 minutes.

## Prerequisites

- mockd installed ([Installation Guide](/getting-started/installation/))
- A terminal
- curl or any HTTP client

## Step 1: Create a Mock Configuration

Create a file called `mocks.json` in your current directory:

```json
{
  "mocks": [
    {
      "id": "hello-world",
      "name": "Hello World Endpoint",
      "enabled": true,
      "matcher": {
        "method": "GET",
        "path": "/api/hello"
      },
      "response": {
        "statusCode": 200,
        "headers": {
          "Content-Type": "application/json"
        },
        "body": "{\"message\": \"Hello, World!\"}"
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
Listening on http://localhost:4280
Loaded 1 mock(s) from mocks.json
```

## Step 3: Test Your Mock

Open a new terminal and make a request:

```bash
curl http://localhost:4280/api/hello
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
      "id": "get-users",
      "name": "Get Users List",
      "enabled": true,
      "matcher": {
        "method": "GET",
        "path": "/api/users"
      },
      "response": {
        "statusCode": 200,
        "headers": {
          "Content-Type": "application/json"
        },
        "body": "{\"users\": [{\"id\": 1, \"name\": \"Alice\", \"email\": \"alice@example.com\"}, {\"id\": 2, \"name\": \"Bob\", \"email\": \"bob@example.com\"}]}"
      }
    },
    {
      "id": "get-user-1",
      "name": "Get User 1",
      "enabled": true,
      "matcher": {
        "method": "GET",
        "path": "/api/users/1"
      },
      "response": {
        "statusCode": 200,
        "headers": {
          "Content-Type": "application/json"
        },
        "body": "{\"id\": 1, \"name\": \"Alice\", \"email\": \"alice@example.com\"}"
      }
    },
    {
      "id": "create-user",
      "name": "Create New User",
      "enabled": true,
      "matcher": {
        "method": "POST",
        "path": "/api/users"
      },
      "response": {
        "statusCode": 201,
        "headers": {
          "Content-Type": "application/json"
        },
        "body": "{\"id\": 3, \"message\": \"User created\"}"
      }
    },
    {
      "id": "user-not-found",
      "name": "User Not Found",
      "enabled": true,
      "matcher": {
        "method": "GET",
        "path": "/api/users/999"
      },
      "response": {
        "statusCode": 404,
        "headers": {
          "Content-Type": "application/json"
        },
        "body": "{\"error\": \"User not found\"}"
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
curl http://localhost:4280/api/users

# Get single user
curl http://localhost:4280/api/users/1

# Create user
curl -X POST http://localhost:4280/api/users

# Not found
curl http://localhost:4280/api/users/999
```

---

## Using Path Parameters

Match dynamic path segments with patterns:

```json
{
  "id": "get-user-by-id",
  "name": "Get User by ID",
  "enabled": true,
  "matcher": {
    "method": "GET",
    "path": "/api/users/*"
  },
  "response": {
    "statusCode": 200,
    "headers": {
      "Content-Type": "application/json"
    },
    "body": "{\"id\": \"dynamic\", \"name\": \"Dynamic User\"}"
  }
}
```

---

## Adding Delays

Simulate network latency:

```json
{
  "id": "slow-endpoint",
  "name": "Slow Endpoint",
  "enabled": true,
  "matcher": {
    "method": "GET",
    "path": "/api/slow"
  },
  "response": {
    "statusCode": 200,
    "headers": {
      "Content-Type": "application/json"
    },
    "delayMs": 500,
    "body": "{\"message\": \"This took a while\"}"
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

- **[Core Concepts](/getting-started/concepts/)** - Understand mocks, matching, and responses
- **[Request Matching](/guides/request-matching/)** - Advanced matching patterns
- **[Stateful Mocking](/guides/stateful-mocking/)** - Simulate CRUD APIs
- **[CLI Reference](/reference/cli/)** - All command-line options
