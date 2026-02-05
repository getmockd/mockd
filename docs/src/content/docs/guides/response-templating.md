---
title: Response Templating
description: Create dynamic responses that include data from the incoming request, generate random values, or compute values at response time.
---

Response templating allows you to create dynamic responses that include data from the incoming request, generate random values, or compute values at response time.

## Template Syntax

Templates use double curly braces: `{{expression}}`

```json
{
  "response": {
    "body": {
      "message": "Hello, {{request.query.name}}"
    }
  }
}
```

Request: `GET /api/greet?name=Alice`
Response: `{"message": "Hello, Alice"}`

## Request Data

Access various parts of the incoming request.

### Path Parameters

```json
{
  "matcher": {
    "path": "/api/users/{id}"
  },
  "response": {
    "body": {
      "id": "{{request.pathParam.id}}",
      "url": "/api/users/{{request.pathParam.id}}"
    }
  }
}
```

### Query Parameters

```json
{
  "response": {
    "body": {
      "page": "{{request.query.page}}",
      "limit": "{{request.query.limit}}"
    }
  }
}
```

### Headers

```json
{
  "response": {
    "body": {
      "userAgent": "{{request.header.User-Agent}}",
      "correlationId": "{{request.header.X-Correlation-ID}}"
    }
  }
}
```

### Request Body

Access parsed request body (JSON):

```json
{
  "response": {
    "body": {
      "received": {
        "username": "{{request.body.username}}",
        "email": "{{request.body.email}}"
      },
      "status": "created"
    }
  }
}
```

Nested access:

```json
{
  "response": {
    "body": {
      "city": "{{request.body.address.city}}",
      "firstItem": "{{request.body.items[0].name}}"
    }
  }
}
```

### Request Metadata

```json
{
  "response": {
    "body": {
      "method": "{{request.method}}",
      "path": "{{request.path}}",
      "fullUrl": "{{request.url}}"
    }
  }
}
```

## Built-in Functions

### Timestamps

```json
{
  "response": {
    "body": {
      "timestamp": "{{now}}",
      "isoDate": "{{now.iso}}",
      "unixTimestamp": "{{timestamp}}"
    }
  }
}
```

Output:
```json
{
  "timestamp": "2024-01-15T10:30:00Z",
  "isoDate": "2024-01-15T10:30:00Z",
  "unixTimestamp": 1705315800
}
```

### Random Values

```json
{
  "response": {
    "body": {
      "id": "{{uuid}}",
      "randomFloat": "{{random.float 0 1}}"
    }
  }
}
```

### String Functions

```json
{
  "response": {
    "body": {
      "upper": "{{upper request.body.name}}",
      "lower": "{{lower request.body.email}}",
      "trim": "{{trim request.body.input}}"
    }
  }
}
```

## Conditional Logic

### If/Else

```json
{
  "response": {
    "body": {
      "status": "{{if request.query.premium}}premium{{else}}free{{end}}"
    }
  }
}
```

### Default Values

```json
{
  "response": {
    "body": {
      "page": "{{default request.query.page \"1\"}}",
      "limit": "{{default request.query.limit \"10\"}}"
    }
  }
}
```

## Loops and Arrays

### Repeating Elements

```json
{
  "response": {
    "body": {
      "items": "{{repeat 5 '{\"id\": {{index}}, \"name\": \"Item {{index}}\"}'}}"
    }
  }
}
```

### Array Length

```json
{
  "response": {
    "body": {
      "itemCount": "{{len request.body.items}}"
    }
  }
}
```

## Response Headers

Templates work in headers too:

```json
{
  "response": {
    "headers": {
      "X-Request-ID": "{{uuid}}",
      "X-Correlation-ID": "{{request.header.X-Correlation-ID}}",
      "Location": "/api/users/{{request.body.id}}"
    }
  }
}
```

## Escaping

### Literal Braces

To output literal `{{`, escape with backslash:

```json
{
  "body": "Template syntax uses \\{{expression}}"
}
```

### JSON in Templates

When embedding JSON, ensure proper escaping:

```json
{
  "response": {
    "body": "{{jsonEncode request.body}}"
  }
}
```

## Complete Example

```json
{
  "matcher": {
    "method": "POST",
    "path": "/api/orders"
  },
  "response": {
    "statusCode": 201,
    "headers": {
      "Content-Type": "application/json",
      "Location": "/api/orders/{{uuid}}",
      "X-Request-ID": "{{request.header.X-Request-ID}}"
    },
    "body": {
      "id": "{{uuid}}",
      "status": "pending",
      "customer": {
        "name": "{{request.body.customer.name}}",
        "email": "{{lower request.body.customer.email}}"
      },
      "items": "{{request.body.items}}",
      "itemCount": "{{len request.body.items}}",
      "total": "{{request.body.total}}",
      "createdAt": "{{now.iso}}",
      "estimatedDelivery": "{{now.addDays 3}}"
    }
  }
}
```

Request:
```bash
curl -X POST http://localhost:4280/api/orders \
  -H "Content-Type: application/json" \
  -H "X-Request-ID: req-123" \
  -d '{
    "customer": {"name": "Alice", "email": "ALICE@EXAMPLE.COM"},
    "items": [{"sku": "A1", "qty": 2}],
    "total": 49.99
  }'
```

Response:
```json
{
  "id": "abc123",
  "status": "pending",
  "customer": {
    "name": "Alice",
    "email": "alice@example.com"
  },
  "items": [{"sku": "A1", "qty": 2}],
  "itemCount": 1,
  "total": 49.99,
  "createdAt": "2024-01-15T10:30:00Z",
  "estimatedDelivery": "2024-01-18T10:30:00Z"
}
```

## Template Reference

| Expression | Description |
|------------|-------------|
| `{{request.method}}` | HTTP method |
| `{{request.path}}` | Request path |
| `{{request.pathParam.name}}` | Path parameter |
| `{{request.query.name}}` | Query parameter |
| `{{request.header.Name}}` | Request header |
| `{{request.body}}` | Full request body |
| `{{request.body.field}}` | Body field |
| `{{now}}` | Current ISO timestamp |
| `{{timestamp}}` | Unix timestamp (seconds) |
| `{{uuid}}` | Random UUID |
| `{{upper value}}` | Uppercase string |
| `{{lower value}}` | Lowercase string |
| `{{default value fallback}}` | Default if empty |
| `{{len array}}` | Array length |

## Next Steps

- [Request Matching](/guides/request-matching/) - Matching rules
- [Stateful Mocking](/guides/stateful-mocking/) - CRUD simulation
- [Configuration Reference](/reference/configuration/) - Full schema
