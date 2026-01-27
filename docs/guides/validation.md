# Request Validation

Mockd provides field-level validation for incoming requests, ensuring that your mock APIs behave like real APIs by rejecting malformed data. This guide covers how to configure validators for both stateful resources and HTTP mocks.

## Overview

Request validation allows you to:

- Enforce data types and constraints on incoming request bodies
- Return realistic error responses when validation fails
- Test client-side error handling without a real backend
- Document expected request formats alongside your mock definitions

Use validation when you want your mocks to reject invalid requests rather than accepting any data. This is especially useful for testing form validation, API client libraries, and error handling flows.

## Quick Start

Add a `validation` block to any stateful resource or HTTP mock:

```yaml
resources:
  - name: users
    seed:
      - id: 1
        email: "alice@example.com"
        age: 28
    validation:
      fields:
        email:
          type: string
          format: email
          required: true
        age:
          type: integer
          min: 0
          max: 150
```

With this configuration, requests with an invalid email or out-of-range age will receive a `400 Bad Request` response with detailed error information.

## Validation for Stateful Resources

Stateful resources support validation on create and update operations. The validation block sits at the resource level:

```yaml
resources:
  - name: products
    seed:
      - id: 1
        sku: "WIDGET-001"
        name: "Blue Widget"
        price: 29.99
    validation:
      mode: strict
      onCreate:
        fields:
          sku:
            type: string
            pattern: "^[A-Z]+-\\d{3}$"
            required: true
          name:
            type: string
            minLength: 1
            maxLength: 200
            required: true
          price:
            type: number
            min: 0
            required: true
      onUpdate:
        fields:
          name:
            type: string
            minLength: 1
            maxLength: 200
          price:
            type: number
            min: 0
```

When `onCreate` and `onUpdate` are omitted, the top-level `fields` apply to both operations. Use separate blocks when create and update have different requirements.

## Validation for HTTP Mocks

For HTTP mocks, add validation inside the request specification:

```yaml
mocks:
  - request:
      method: POST
      path: /api/contact
    validation:
      mode: strict
      fields:
        name:
          type: string
          required: true
        email:
          type: string
          format: email
          required: true
        message:
          type: string
          minLength: 10
          maxLength: 1000
    response:
      status: 200
      body:
        success: true
```

The validation runs before the response is generated. If validation fails, the configured error response is returned instead.

## Field Validators

### Types

Every field validator requires a `type` property:

| Type      | Description                          |
|-----------|--------------------------------------|
| `string`  | Text values                          |
| `number`  | Floating-point numbers               |
| `integer` | Whole numbers only                   |
| `boolean` | True or false                        |
| `array`   | Lists of values                      |
| `object`  | Nested objects                       |

```yaml
fields:
  username:
    type: string
  score:
    type: number
  count:
    type: integer
  active:
    type: boolean
  tags:
    type: array
  metadata:
    type: object
```

### String Validation

String fields support length constraints, patterns, and format validation:

```yaml
fields:
  username:
    type: string
    minLength: 3
    maxLength: 32
    pattern: "^[a-zA-Z0-9_]+$"
  email:
    type: string
    format: email
  website:
    type: string
    format: uri
  userId:
    type: string
    format: uuid
```

Available formats:

| Format     | Description                              |
|------------|------------------------------------------|
| `email`    | Valid email address                      |
| `uuid`     | UUID v4 format                           |
| `date`     | ISO 8601 date (YYYY-MM-DD)               |
| `datetime` | ISO 8601 datetime                        |
| `uri`      | Valid URI                                |
| `ipv4`     | IPv4 address                             |
| `ipv6`     | IPv6 address                             |
| `hostname` | Valid hostname                           |

### Number Validation

Number and integer fields support range constraints:

```yaml
fields:
  price:
    type: number
    min: 0
    max: 99999.99
  quantity:
    type: integer
    min: 1
    max: 100
  temperature:
    type: number
    exclusiveMin: -273.15
  rating:
    type: number
    min: 0
    exclusiveMax: 5
```

- `min` / `max`: Inclusive bounds
- `exclusiveMin` / `exclusiveMax`: Exclusive bounds

### Array Validation

Array fields can constrain length and validate items:

```yaml
fields:
  tags:
    type: array
    minItems: 1
    maxItems: 10
    uniqueItems: true
    items:
      type: string
      minLength: 1
      maxLength: 50
  scores:
    type: array
    items:
      type: integer
      min: 0
      max: 100
```

The `items` property defines a validator applied to each element in the array.

### Enum Validation

Restrict values to a predefined set:

```yaml
fields:
  status:
    type: string
    enum:
      - pending
      - approved
      - rejected
  priority:
    type: integer
    enum:
      - 1
      - 2
      - 3
```

### Required and Nullable

Control whether fields must be present and whether they accept null:

```yaml
fields:
  name:
    type: string
    required: true
  middleName:
    type: string
    nullable: true
  email:
    type: string
    required: true
    nullable: false
```

- `required: true` - Field must be present in the request
- `nullable: true` - Field may be explicitly set to null
- By default, fields are optional and non-nullable

## Nested Field Validation

Use dot notation to validate nested objects and array elements:

```yaml
fields:
  address.street:
    type: string
    required: true
  address.city:
    type: string
    required: true
  address.zipCode:
    type: string
    pattern: "^\\d{5}(-\\d{4})?$"
  items.sku:
    type: string
    required: true
  items.quantity:
    type: integer
    min: 1
```

For arrays, the path `items.sku` validates the `sku` field on every object in the `items` array. This works with deeply nested structures:

```yaml
fields:
  order.shipping.address.country:
    type: string
    enum:
      - US
      - CA
      - MX
```

## Validation Modes

Control how validation failures are handled with the `mode` property:

| Mode         | Behavior                                              |
|--------------|-------------------------------------------------------|
| `strict`     | Reject invalid requests with 400 error (default)      |
| `warn`       | Log warning but process the request normally          |
| `permissive` | Silently ignore validation errors                     |

```yaml
validation:
  mode: warn
  fields:
    email:
      type: string
      format: email
```

Use `warn` mode during development to identify validation issues without breaking functionality. Switch to `strict` for testing error handling paths.

## Error Response Format

When validation fails in strict mode, Mockd returns an RFC 7807 Problem Details response:

```json
{
  "type": "https://mockd.dev/errors/validation-error",
  "title": "Validation Error",
  "status": 400,
  "detail": "Request body failed validation",
  "instance": "/api/users",
  "errors": [
    {
      "field": "email",
      "message": "must be a valid email address",
      "value": "not-an-email"
    },
    {
      "field": "age",
      "message": "must be greater than or equal to 0",
      "value": -5
    }
  ]
}
```

The response includes:

- `type`: URI identifying the error type
- `title`: Human-readable error title
- `status`: HTTP status code
- `detail`: Explanation of what went wrong
- `instance`: The request path that failed
- `errors`: Array of individual field errors with the field name, error message, and rejected value

## Auto-Inference from Seed Data

When seed data is provided without explicit validation rules, Mockd can infer basic type validation:

```yaml
resources:
  - name: users
    validation:
      inferFromSeed: true
    seed:
      - id: 1
        email: "alice@example.com"
        age: 28
        active: true
```

This automatically creates validators based on the seed data types. Explicit field validators override inferred ones. Use this for quick prototyping, but define explicit validators for production mocks.

## Complete Examples

### Stateful Resource with Full Validation

```yaml
resources:
  - name: orders
    seed:
      - id: 1
        customerId: "cust_abc123"
        status: pending
        items:
          - sku: "WIDGET-001"
            quantity: 2
        shippingAddress:
          street: "123 Main St"
          city: "Springfield"
          zipCode: "12345"
    validation:
      mode: strict
      onCreate:
        fields:
          customerId:
            type: string
            pattern: "^cust_[a-z0-9]+$"
            required: true
          status:
            type: string
            enum:
              - pending
              - processing
              - shipped
              - delivered
          items:
            type: array
            minItems: 1
            required: true
          items.sku:
            type: string
            required: true
          items.quantity:
            type: integer
            min: 1
            required: true
          shippingAddress.street:
            type: string
            required: true
          shippingAddress.city:
            type: string
            required: true
          shippingAddress.zipCode:
            type: string
            pattern: "^\\d{5}$"
            required: true
      onUpdate:
        fields:
          status:
            type: string
            enum:
              - pending
              - processing
              - shipped
              - delivered
```

### HTTP Mock with Validation

```yaml
mocks:
  - request:
      method: POST
      path: /api/newsletter/subscribe
    validation:
      mode: strict
      fields:
        email:
          type: string
          format: email
          required: true
        firstName:
          type: string
          minLength: 1
          maxLength: 100
        preferences:
          type: object
        preferences.frequency:
          type: string
          enum:
            - daily
            - weekly
            - monthly
        preferences.topics:
          type: array
          items:
            type: string
          maxItems: 5
          uniqueItems: true
    response:
      status: 201
      body:
        subscribed: true
        message: "Successfully subscribed to newsletter"
```

## Next Steps

- [Stateful Mocking](./stateful-mocking.md) - Learn more about CRUD resources and state management
- [Configuration](../reference/configuration.md) - Explore all configuration options for your mock server
