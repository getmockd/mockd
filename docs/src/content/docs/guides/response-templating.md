---
title: Response Templating
description: Create dynamic responses that include data from the incoming request, generate random values, or compute values at response time.
---

Response templating allows you to create dynamic responses that include data from the incoming request, generate random values, or compute values at response time.

:::note
Examples below show the contents of the `http` block within a mock definition. In a config file, wrap each example in the full mock structure:
```yaml
mocks:
  - id: my-mock
    type: http
    http:
      matcher: { method: POST, path: /api/example }
      response: { ... }
```
:::

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
      "isoTimestamp": "{{timestamp.iso}}",
      "unixTimestamp": "{{timestamp}}",
      "unixMs": "{{timestamp.unix_ms}}"
    }
  }
}
```

Output:
```json
{
  "timestamp": "2024-01-15T10:30:00-06:00",
  "isoTimestamp": "2024-01-15T16:30:00.123456789Z",
  "unixTimestamp": "1705315800",
  "unixMs": "1705315800123"
}
```

### Random Values

```json
{
  "response": {
    "body": {
      "id": "{{uuid}}",
      "shortId": "{{uuid.short}}",
      "randomInt": "{{random.int(1, 100)}}",
      "randomFloat": "{{random.float(0, 1)}}",
      "randomString": "{{random.string(8)}}"
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
      "fallback": "{{default request.query.name \"Anonymous\"}}"
    }
  }
}
```

## Default Values

Provide fallback values when a field is missing:

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

Both space-separated and parenthesized syntax work:

```json
{
  "response": {
    "body": {
      "name": "{{default(request.query.name, \"Anonymous\")}}"
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

## Faker Functions

Generate realistic sample data with 35 built-in faker types. Faker type names are **case-insensitive** — `faker.Name`, `faker.name`, and `faker.NAME` all work. Faker functions work in **all protocols** — HTTP, GraphQL, gRPC, SOAP, WebSocket, SSE, and MQTT response bodies.

```json
{
  "response": {
    "body": {
      "name": "{{faker.name}}",
      "email": "{{faker.email}}",
      "phone": "{{faker.phone}}",
      "company": "{{faker.company}}",
      "card": "{{faker.creditCard}}",
      "ip": "{{faker.ipv4}}",
      "job": "{{faker.jobTitle}}",
      "price": "{{faker.price}}"
    }
  }
}
```

### Basic

| Faker Type | Example Output |
|------------|---------------|
| `faker.name` | John Smith |
| `faker.firstName` | Alice |
| `faker.lastName` | Williams |
| `faker.email` | alice42@example.com |
| `faker.phone` | +1-555-123-4567 |
| `faker.company` | Acme Corp |
| `faker.address` | 123 Main St, New York, NY 10001 |
| `faker.word` | delta |
| `faker.sentence` | Lorem ipsum dolor sit amet. |
| `faker.words` | alpha gamma epsilon (3-5 random words) |
| `faker.words(n)` | alpha beta gamma (exactly n words) |
| `faker.boolean` | true |
| `faker.uuid` | a1b2c3d4-e5f6-7890-abcd-ef1234567890 |

### Internet

| Faker Type | Example Output |
|------------|---------------|
| `faker.ipv4` | 192.168.1.42 |
| `faker.ipv6` | 2001:0db8:85a3:0000:0000:8a2e:0370:7334 |
| `faker.macAddress` | 00:1A:2B:3C:4D:5E |
| `faker.userAgent` | Mozilla/5.0 (Windows NT 10.0; ...) |
| `faker.url` | https://example.com/api |

### Finance

| Faker Type | Example Output |
|------------|---------------|
| `faker.creditCard` | 4532015112830366 (Luhn-valid) |
| `faker.creditCardExp` | 08/28 (MM/YY, always future) |
| `faker.cvv` | 847 |
| `faker.currencyCode` | USD (ISO 4217) |
| `faker.currency` | US Dollar |
| `faker.iban` | GB29NWBK60161331926819 |
| `faker.price` | 49.99 |

### Commerce & Identity

| Faker Type | Example Output |
|------------|---------------|
| `faker.productName` | Ergonomic Steel Chair |
| `faker.color` | indigo |
| `faker.hexColor` | #4A90D9 |
| `faker.ssn` | 123-45-6789 |
| `faker.passport` | X12345678 |
| `faker.jobTitle` | Senior Software Engineer |

### Geo & Data

| Faker Type | Example Output |
|------------|---------------|
| `faker.latitude` | 37.774929 |
| `faker.longitude` | -122.419416 |
| `faker.slug` | ergonomic-steel-chair |
| `faker.mimeType` | application/json |
| `faker.fileExtension` | pdf |

:::tip
`faker.words(n)` is the only parameterized faker type. Use it to generate exactly *n* space-separated words: `{{faker.words(5)}}` → `alpha beta gamma delta epsilon`.

Faker type names are case-insensitive: `{{faker.firstName}}`, `{{faker.firstname}}`, and `{{faker.FIRSTNAME}}` all produce the same result.
:::

## Seeded (Deterministic) Responses

By default, faker functions and random values produce different output on every request. For deterministic testing, you can **seed** the random number generator so that the same request always produces the same response.

### Query Parameter Seeding

Add `?_mockd_seed=<number>` to any request:

```bash
# These two requests return identical faker output
curl "http://localhost:4280/api/users?_mockd_seed=42"
curl "http://localhost:4280/api/users?_mockd_seed=42"

# Different seed = different (but deterministic) output
curl "http://localhost:4280/api/users?_mockd_seed=99"
```

### Config-Level Seeding

Set the `seed` field on a response to make it always deterministic without query parameters:

```yaml
mocks:
  - id: deterministic-user
    type: http
    http:
      matcher:
        method: GET
        path: /api/test-user
      response:
        statusCode: 200
        seed: 42
        body: |
          {
            "name": "{{faker.name}}",
            "email": "{{faker.email}}",
            "id": "{{uuid}}"
          }
```

Every request to `/api/test-user` returns the same name, email, and UUID.

### Use Cases

- **Snapshot testing** — Compare responses against golden files
- **Flaky test elimination** — Same seed = same output = no randomness-induced failures
- **Reproducible bug reports** — Share the seed value to reproduce exact responses

:::tip
Config-level `seed` and query parameter `_mockd_seed` can be combined. The query parameter takes precedence if both are present.
:::

## Sequences

Generate auto-incrementing values (useful for IDs):

```yaml
response:
  statusCode: 200
  body: |
    {
      "id": "{{sequence("order-id")}}",
      "ticketNumber": "{{sequence("tickets", 1000)}}"
    }
```

The optional second argument sets the starting value (default: 1). Sequences persist for the lifetime of the server.

:::note
The `sequence()` function uses double quotes around the name. When writing body templates in YAML, use the literal block style (`body: |`) to avoid quote escaping issues.
:::

## Complete Example

```yaml
mocks:
  - id: create-order
    type: http
    http:
      matcher:
        method: POST
        path: /api/orders
      response:
        statusCode: 201
        headers:
          Content-Type: "application/json"
          Location: "/api/orders/{{uuid}}"
          X-Request-ID: "{{request.header.X-Request-ID}}"
        body: |
          {
            "id": "{{uuid}}",
            "status": "pending",
            "customer": {
              "name": "{{request.body.customer.name}}",
              "email": "{{lower request.body.customer.email}}"
            },
            "total": "{{request.body.total}}",
            "createdAt": "{{now}}"
          }
```

Request:
```bash
curl -X POST http://localhost:4280/api/orders \
  -H "Content-Type: application/json" \
  -H "X-Request-ID: req-123" \
  -d '{
    "customer": {"name": "Alice", "email": "ALICE@EXAMPLE.COM"},
    "total": 49.99
  }'
```

Response:
```json
{
  "id": "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
  "status": "pending",
  "customer": {
    "name": "Alice",
    "email": "alice@example.com"
  },
  "total": "49.99",
  "createdAt": "2026-02-24T10:30:00-06:00"
}
```

## Template Reference

| Expression | Description |
|------------|-------------|
| `{{request.method}}` | HTTP method |
| `{{request.path}}` | Request path |
| `{{request.url}}` | Full request URL |
| `{{request.pathParam.name}}` | Path parameter |
| `{{request.query.name}}` | Query parameter |
| `{{request.header.Name}}` | Request header |
| `{{request.body.field}}` | Body field (dot-nested) |
| `{{request.rawBody}}` | Raw request body string |
| `{{now}}` | Current timestamp (RFC3339) |
| `{{timestamp}}` | Unix timestamp (seconds) |
| `{{timestamp.iso}}` | ISO timestamp (RFC3339Nano UTC) |
| `{{timestamp.unix_ms}}` | Unix timestamp (milliseconds) |
| `{{uuid}}` | Random UUID |
| `{{uuid.short}}` | Short random ID (hex) |
| `{{random.int(min, max)}}` | Random integer in range (alias: `randomInt`) |
| `{{random.float(min, max)}}` | Random float in range (alias: `randomFloat`) |
| `{{random.string(length)}}` | Random alphanumeric string (alias: `randomString`) |
| `{{sequence("name")}}` | Auto-incrementing counter |
| `{{upper value}}` | Uppercase string |
| `{{lower value}}` | Lowercase string |
| `{{default value fallback}}` | Default if empty |
| `{{faker.name}}` | Random person name |
| `{{faker.email}}` | Random email address |
| `{{faker.creditCard}}` | Luhn-valid credit card number |
| `{{faker.ipv4}}` | Random IPv4 address |
| `{{faker.price}}` | Random price (e.g., 49.99) |
| `{{faker.*}}` | [35 types total](#faker-functions) |

## Next Steps

- [Request Matching](/guides/request-matching/) - Matching rules
- [Stateful Mocking](/guides/stateful-mocking/) - CRUD simulation
- [Configuration Reference](/reference/configuration/) - Full schema
