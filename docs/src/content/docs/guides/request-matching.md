---
title: Request Matching
description: Learn how mockd evaluates matchers to determine which mock responds to incoming HTTP requests.
---

Request matching determines which mock responds to an incoming HTTP request. mockd evaluates matchers in order and returns the first matching response.

> **Note:** In JSON/YAML configuration files, the actual field name is `matcher`, not `request`. The examples below use the simplified format for clarity.

## Basic Matching

### Method Matching

Match specific HTTP methods:

```json
{
  "request": {
    "method": "GET"
  }
}
```

Supported methods: `GET`, `POST`, `PUT`, `PATCH`, `DELETE`, `HEAD`, `OPTIONS`

### Path Matching

Exact path match:

```json
{
  "request": {
    "path": "/api/users"
  }
}
```

## Path Parameters

Capture dynamic path segments using curly braces:

```json
{
  "request": {
    "path": "/api/users/{id}"
  }
}
```

This matches:
- `/api/users/1`
- `/api/users/abc`
- `/api/users/123-456`

Access captured values in responses:

```json
{
  "response": {
    "body": {
      "userId": "{{request.pathParam.id}}"
    }
  }
}
```

### Multiple Path Parameters

```json
{
  "request": {
    "path": "/api/{resource}/{id}/comments/{commentId}"
  }
}
```

Matches `/api/posts/5/comments/12` with:
- `resource = "posts"`
- `id = "5"`
- `commentId = "12"`

### Greedy Path Matching

Match remaining path segments with `.*`:

```json
{
  "request": {
    "path": "/api/files/{filepath:.*}"
  }
}
```

Matches `/api/files/documents/2024/report.pdf`

### Regex Path Matching (pathPattern)

Use full regex patterns for advanced path matching with `pathPattern`:

```json
{
  "matcher": {
    "pathPattern": "^/api/users/\\d+$"
  }
}
```

This matches `/api/users/123` but not `/api/users/abc`.

#### Named Capture Groups

Extract path segments into named variables:

```json
{
  "matcher": {
    "pathPattern": "^/api/(?P<resource>\\w+)/(?P<id>\\d+)$"
  }
}
```

Matches `/api/users/456` with captures:
- `resource = "users"`
- `id = "456"`

#### Common Regex Patterns

| Pattern | Description |
|---------|-------------|
| `^/api/users/\\d+$` | Numeric ID only |
| `^/api/(users\|products)/\\d+$` | Multiple resource types |
| `^/api/orders/[0-9a-f-]{36}$` | UUID format |
| `^/api/items/[\\w-]+$` | Slugs with alphanumeric and dashes |

## Query Parameter Matching

Match requests with specific query parameters:

```json
{
  "request": {
    "path": "/api/users",
    "query": {
      "page": "1",
      "limit": "10"
    }
  }
}
```

### Optional Query Parameters

Only specified parameters are required. Additional parameters are ignored:

```json
{
  "request": {
    "path": "/api/search",
    "query": {
      "q": "test"
    }
  }
}
```

Matches both:
- `/api/search?q=test`
- `/api/search?q=test&page=1&extra=value`

## Header Matching

Match requests with specific headers:

```json
{
  "request": {
    "headers": {
      "Content-Type": "application/json",
      "X-API-Key": "secret123"
    }
  }
}
```

### Wildcard Header Matching

Use `*` wildcards for flexible header matching:

```json
{
  "request": {
    "headers": {
      "Authorization": "Bearer *",
      "Accept": "application/*"
    }
  }
}
```

Supported patterns:
- `prefix*` - matches values starting with prefix
- `*suffix` - matches values ending with suffix
- `*contains*` - matches values containing the substring

### Case Sensitivity

Header names are case-insensitive (per HTTP spec), but values are case-sensitive:

```json
{
  "request": {
    "headers": {
      "content-type": "application/json"
    }
  }
}
```

Matches `Content-Type: application/json` and `CONTENT-TYPE: application/json`

## Body Matching

Match requests with specific body content.

### JSON Body Matching

Match exact JSON structure:

```json
{
  "request": {
    "body": {
      "username": "alice",
      "action": "login"
    }
  }
}
```

### Partial JSON Matching

Use `bodyContains` for partial matching:

```json
{
  "request": {
    "bodyContains": {
      "username": "alice"
    }
  }
}
```

Matches any request body that contains `"username": "alice"`, ignoring other fields.

### JSON Path Matching

Match specific paths in JSON:

```json
{
  "request": {
    "bodyMatch": {
      "$.user.email": ".*@example\\.com",
      "$.items[0].quantity": "[1-9][0-9]*"
    }
  }
}
```

### String Body Matching

Match raw body content:

```json
{
  "request": {
    "bodyString": "<xml>.*</xml>"
  }
}
```

### Regex Body Matching (bodyPattern)

Use full regex patterns for body matching with `bodyPattern`:

```json
{
  "matcher": {
    "bodyPattern": "\"email\":\\s*\"[^\"]+@example\\.com\""
  }
}
```

This matches any JSON body containing an email field ending with `@example.com`.

#### Useful Body Patterns

| Pattern | Description |
|---------|-------------|
| `"status":\\s*"(pending\|approved)"` | Match status values |
| `[0-9a-f]{8}-[0-9a-f]{4}-` | Contains UUID |
| `(?i)error` | Contains "error" (case-insensitive) |
| `(?s)start.*end` | Multiline matching |

### JSONPath Body Matching (bodyJsonPath)

Match specific JSON fields using JSONPath expressions:

```json
{
  "matcher": {
    "bodyJsonPath": {
      "$.user.name": "John",
      "$.items[0].quantity": 5,
      "$.status": "active"
    }
  }
}
```

This matches requests where:
- `$.user.name` equals "John"
- `$.items[0].quantity` equals 5
- `$.status` equals "active"

#### JSONPath Syntax

| Expression | Description |
|------------|-------------|
| `$.field` | Root-level field |
| `$.user.name` | Nested field |
| `$.items[0]` | Array index |
| `$.items[*].id` | Any array element |
| `$..name` | Recursive descent |

#### Existence Checks

Check if a field exists (or doesn't exist):

```json
{
  "matcher": {
    "bodyJsonPath": {
      "$.token": {"exists": true},
      "$.deleted": {"exists": false}
    }
  }
}
```

#### Type Support

JSONPath matching supports:
- Strings: `"$.name": "John"`
- Numbers: `"$.age": 30`
- Booleans: `"$.active": true`
- Null: `"$.deleted": null`

## Combining Matchers

Combine multiple matchers for precise matching:

```json
{
  "request": {
    "method": "POST",
    "path": "/api/users/{id}/comments",
    "headers": {
      "Content-Type": "application/json",
      "Authorization": "Bearer .*"
    },
    "query": {
      "notify": "true"
    },
    "bodyContains": {
      "text": ".*"
    }
  }
}
```

All conditions must match for the mock to respond.

## Priority and Ordering

When multiple mocks could match, mockd uses this priority:

### 1. Specificity

More specific matches win:

```json
// This wins for /api/users/1
{ "path": "/api/users/1" }

// This wins for /api/users/2, /api/users/abc, etc.
{ "path": "/api/users/{id}" }
```

### 2. Number of Matchers

Mocks with more conditions win:

```json
// Less specific (matches any GET /api/users)
{ "method": "GET", "path": "/api/users" }

// More specific (matches only with Authorization header)
{ "method": "GET", "path": "/api/users", "headers": { "Authorization": ".*" } }
```

### 3. Configuration Order

When priority is equal, earlier mocks in the config file win.

## Matching Examples

### API Key Required

```json
{
  "request": {
    "path": "/api/.*",
    "headers": {
      "X-API-Key": "valid-key-123"
    }
  },
  "response": { "status": 200 }
}
```

### Content Negotiation

```json
[
  {
    "request": {
      "path": "/api/data",
      "headers": { "Accept": "application/xml" }
    },
    "response": {
      "headers": { "Content-Type": "application/xml" },
      "body": "<data>...</data>"
    }
  },
  {
    "request": {
      "path": "/api/data",
      "headers": { "Accept": "application/json" }
    },
    "response": {
      "headers": { "Content-Type": "application/json" },
      "body": { "data": "..." }
    }
  }
]
```

## Next Steps

- [Response Templating](/guides/response-templating/) - Dynamic responses
- [Stateful Mocking](/guides/stateful-mocking/) - CRUD simulation
- [Configuration Reference](/reference/configuration/) - Full schema
