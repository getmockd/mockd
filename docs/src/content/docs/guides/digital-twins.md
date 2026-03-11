---
title: Building a Digital Twin
description: Step-by-step guide to building a fully stateful, SDK-compatible mock of any third-party API.
---

## What is a Digital Twin?

A digital twin is a local mock server that mimics a third-party API with full stateful behavior — creates persist, updates modify state, deletes remove items. Unlike static mocks that return canned responses, digital twins let you run your real SDK test suites against localhost. mockd can twin any API that has an OpenAPI spec, turning hundreds of endpoints into a working local replica in minutes.

## Prerequisites

- **mockd installed** — via `go install github.com/getmockd/mockd/cmd/mockd@latest` or [binary download](https://github.com/getmockd/mockd/releases)
- **An OpenAPI spec** for your target API (Stripe: [github.com/stripe/openapi](https://github.com/stripe/openapi), Twilio: [github.com/twilio/twilio-oai](https://github.com/twilio/twilio-oai))
- **Familiarity with YAML**

## Step 1: Get the API Spec

Download the OpenAPI spec for your target API. We'll use Stripe as our running example.

```bash
# Stripe publishes their spec publicly
curl -o stripe.yaml https://raw.githubusercontent.com/stripe/openapi/master/openapi/spec3.yaml

# Or if you already have a local copy
ls stripe.yaml
```

Any valid OpenAPI 3.x spec works. If your target API doesn't publish one, you can record traffic with `mockd record` — see the [Import & Export guide](/guides/import-export/).

## Step 2: Import the Spec

Create a `mockd.yaml` configuration file. Start with just the import:

```yaml
version: "1.0"

imports:
  - path: stripe.yaml
    as: stripe
```

- **`path`** — relative path to the OpenAPI spec file
- **`as`** — namespace prefix; all operationIds get prefixed as `stripe.{operationId}`, preventing collisions if you import multiple APIs

mockd now serves all 587 endpoints with auto-generated schema-based responses. Everything works, but stateless. Try it:

```bash
mockd start -c mockd.yaml --no-auth -d
mockd list | head -20
# You'll see: stripe.PostCustomers, stripe.GetCustomersId, etc.
curl http://localhost:4280/v1/customers
mockd stop
```

You now have a working mock of the entire Stripe API. Let's make the important parts stateful.

## Step 3: Identify Endpoints to Make Stateful

Most endpoints can stay as static mocks — you only need to bind the ones your code actually calls. Use `extend` to override specific imported endpoints with stateful behavior while leaving the rest untouched. Find the operationIds:

```bash
# List all imported mocks
mockd list | grep "stripe\."

# Common pattern: operationIds follow {Method}{Resource}
# POST /v1/customers             → PostCustomers
# GET /v1/customers              → GetCustomers
# GET /v1/customers/{customer}   → GetCustomersCustomer
# POST /v1/customers/{customer}  → PostCustomersCustomer (update!)
# DELETE /v1/customers/{customer} → DeleteCustomersCustomer
```

For a typical integration, you might only need 5-10 stateful endpoints out of hundreds.

## Step 4: Design Your Tables

Tables are pure data stores — each one holds rows for a single resource type:

```yaml
tables:
  - name: customers
    idField: id
    idStrategy: prefix
    idPrefix: "cus_"
    seedData:
      - id: "cus_test"
        name: "Test Customer"
        email: "test@example.com"
        created: 1705312200
```

Key decisions:

- **`idStrategy`** — Stripe uses prefixed IDs (`cus_`, `pi_`, `sub_`), so use `prefix`. Other options: `uuid`, `sequential`, `nanoid`
- **`idPrefix`** — Match the real API's ID format so SDK validations pass
- **`seedData`** — Pre-populate with test fixtures. IDs in seed data are used as-is

## Step 5: Write Extend Bindings

Connect imported endpoints to your tables. Each `extend` entry overrides one imported mock with a table-backed action:

```yaml
extend:
  # List customers
  - mock: stripe.GetCustomers
    table: customers
    action: list

  # Create a customer
  - mock: stripe.PostCustomers
    table: customers
    action: create

  # Get a single customer
  - mock: stripe.GetCustomersCustomer
    table: customers
    action: get

  # Update a customer (Stripe uses POST, not PUT!)
  - mock: stripe.PostCustomersCustomer
    table: customers
    action: patch

  # Delete a customer
  - mock: stripe.DeleteCustomersCustomer
    table: customers
    action: delete
```

:::caution[POST for Updates]
Stripe uses POST for both creates and updates. When the endpoint updates an existing resource (POST to a URL with an ID), use `action: patch` — this merges only the sent fields into the existing item. Using `action: update` would replace the entire item (PUT semantics), wiping out fields not included in the request.
:::

Actions: `list` (all rows), `create` (insert + auto-ID), `get` (fetch by ID), `patch` (partial merge), `update` (full replace), `delete` (remove by ID).

## Step 6: Add Response Transforms

Without transforms, mockd returns its default format. Response transforms let you match the target API's conventions exactly — timestamps, envelopes, error shapes, and more:

```yaml
tables:
  - name: customers
    idField: id
    idStrategy: prefix
    idPrefix: "cus_"
    seedData:
      - id: "cus_test"
        name: "Test Customer"
        email: "test@example.com"
    response:
      # Timestamps as unix epoch, renamed to match Stripe's field names
      timestamps:
        format: unix
        fields:
          createdAt: created
          updatedAt: updated

      # Add object type field, hide internal tracking fields
      fields:
        inject:
          object: customer
          livemode: false
        hide:
          - updatedAt

      # Stripe wraps lists in an envelope with metadata
      list:
        dataField: data
        extraFields:
          object: list
          url: /v1/customers
          has_more: false
        hideMeta: true

      # Stripe returns 200 for creates (not 201)
      create:
        status: 200

      # Stripe soft-delete returns 200 with a confirmation body
      delete:
        status: 200
        preserve: true
        body:
          id: "{{item.id}}"
          object: customer
          deleted: true

      # Match Stripe's error envelope and type system
      errors:
        wrap: error
        fields:
          message: message
          type: type
          code: code
        typeMap:
          NOT_FOUND: invalid_request_error
          CONFLICT: invalid_request_error
          VALIDATION_ERROR: invalid_request_error
        codeMap:
          NOT_FOUND: resource_missing
          CONFLICT: resource_already_exists
```

Each section targets a specific convention: **timestamps** (unix epoch + renamed fields), **fields** (inject `object`/`livemode`, hide internals), **list** (Stripe's `{object, data, has_more, url}` envelope), **create** (200 instead of 201), **delete** (soft-delete with confirmation body), and **errors** (Stripe's error envelope so SDKs parse them correctly).

:::tip[YAML Anchors]
When you have multiple tables, use YAML anchors to define shared transforms once. Keys starting with `x-` are ignored by mockd and work as anchor hosts:

```yaml
x-stripe-defaults: &stripe-defaults
  timestamps:
    format: unix
    fields: { createdAt: created, updatedAt: updated }
  errors:
    wrap: error
    # ...

tables:
  - name: customers
    response:
      <<: *stripe-defaults
      fields: { inject: { object: customer } }
  - name: products
    response:
      <<: *stripe-defaults
      fields: { inject: { object: product } }
```

See the [complete Stripe sample](https://github.com/getmockd/mockd-samples/tree/main/third-party-apis/stripe-api) for this pattern across 9 tables.
:::

## Step 7: Test It

Start the server and exercise the full CRUD lifecycle:

```bash
mockd start -c mockd.yaml --no-auth -d

# Create
curl -X POST http://localhost:4280/v1/customers \
  -d "name=Jenny Rosen" -d "email=jenny@example.com"
# → {"id":"cus_a1b2c3...","object":"customer","created":1705312200,...}

# List
curl http://localhost:4280/v1/customers
# → {"object":"list","data":[...],"has_more":false,"url":"/v1/customers"}

# Get
curl http://localhost:4280/v1/customers/cus_test

# Update (partial merge)
curl -X POST http://localhost:4280/v1/customers/cus_test -d "name=Updated Name"

# Verify update persisted
curl http://localhost:4280/v1/customers/cus_test
# → {"id":"cus_test","name":"Updated Name","email":"test@example.com",...}

# Delete
curl -X DELETE http://localhost:4280/v1/customers/cus_test
# → {"id":"cus_test","object":"customer","deleted":true}

# Confirm gone
curl http://localhost:4280/v1/customers/cus_test
# → {"error":{"type":"invalid_request_error","code":"resource_missing",...}}

mockd stop
```

## Step 8: Test with Your SDK

The real payoff — point your SDK at the digital twin and run your actual test suite. No mocking libraries, no interface swapping. Your production code talks HTTP to what looks like Stripe.

### Stripe SDK (Go)

```go
import (
    "strings"
    "testing"
    "github.com/stripe/stripe-go/v82"
    "github.com/stripe/stripe-go/v82/customer"
)

func init() {
    stripe.Key = "sk_test_fake"
    stripe.SetBackend(stripe.APIBackend, stripe.GetBackendWithConfig(
        stripe.APIBackend,
        &stripe.BackendConfig{
            URL: stripe.String("http://localhost:4280"),
        },
    ))
}

func TestCreateCustomer(t *testing.T) {
    c, err := customer.New(&stripe.CustomerParams{
        Name:  stripe.String("Test User"),
        Email: stripe.String("test@example.com"),
    })
    if err != nil {
        t.Fatal(err)
    }
    if !strings.HasPrefix(c.ID, "cus_") {
        t.Errorf("expected cus_ prefix, got %s", c.ID)
    }
}
```

### Stripe SDK (Python)

```python
import stripe

stripe.api_key = "sk_test_fake"
stripe.api_base = "http://localhost:4280"

customer = stripe.Customer.create(name="Test User", email="test@example.com")
assert customer.id.startswith("cus_")

updated = stripe.Customer.modify(customer.id, name="New Name")
assert updated.name == "New Name"
```

### Stripe SDK (Node.js)

```javascript
const stripe = require('stripe')('sk_test_fake', {
  apiVersion: '2024-04-10',
  host: 'localhost',
  port: 4280,
  protocol: 'http',
});

const customer = await stripe.customers.create({
  name: 'Test User',
  email: 'test@example.com',
});
console.log(customer.id); // cus_a1b2c3d4...
```

:::note[No Auth]
Use `--no-auth` when starting mockd to skip API key validation. The SDK still sends its key, but mockd won't reject it.
:::

:::note[Form Encoding]
Stripe SDKs send requests as `application/x-www-form-urlencoded`, not JSON. mockd handles form-encoded bodies automatically — no extra configuration needed.
:::

## Adding Custom Operations

For endpoints that aren't simple CRUD — like confirming a payment intent or capturing a charge — use custom operations:

```yaml
customOperations:
  - name: ConfirmPaymentIntent
    steps:
      - type: read
        resource: payment_intents
        id: "input.intent"
        as: pi
      - type: update
        resource: payment_intents
        id: "input.intent"
        set:
          status: '"succeeded"'
          amount_received: "pi.amount"
    response:
      id: "pi.id"
      object: '"payment_intent"'
      status: '"succeeded"'
      amount: "pi.amount"
      amount_received: "pi.amount"

extend:
  - mock: stripe.PostPaymentIntentsIntentConfirm
    table: payment_intents
    action: custom
    operation: ConfirmPaymentIntent
```

Note the quoting: `'"succeeded"'` is a string literal (outer quotes are YAML, inner quotes mark it as a literal value). `"pi.amount"` without inner quotes is a reference to a field on the `pi` variable from the `read` step. Custom operations can chain reads and writes across multiple tables for complex workflows.

## Complete Working Example

```bash
# Clone the samples repository
git clone https://github.com/getmockd/mockd-samples.git

# Run the Stripe digital twin (9 tables, 44 bindings, 8 custom operations)
mockd start -c mockd-samples/third-party-apis/stripe-api/mockd.yaml --no-auth

# Run the Twilio digital twin (7 tables, 30 bindings)
mockd start -c mockd-samples/third-party-apis/twilio-api/mockd.yaml --no-auth
```

The Stripe sample passes 49/49 tests from the official `stripe-go` SDK. The Twilio sample passes 13/13 from `twilio-go`. These prove the pattern generalizes across different API styles.

## Next Steps

- **[Stateful Mocking Guide](/guides/stateful-mocking/)** — Complete reference for tables, extend, response transforms, and custom operations
- **[Configuration Reference](/reference/configuration/)** — Full schema documentation for `mockd.yaml`
- **[Import & Export](/guides/import-export/)** — Importing from OpenAPI, WSDL, Postman collections, and more
