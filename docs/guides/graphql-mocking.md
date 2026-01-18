# GraphQL Mocking

GraphQL mocking enables you to create mock GraphQL endpoints for testing GraphQL clients. Configure queries, mutations, subscriptions, and introspection with flexible resolvers.

## Overview

mockd's GraphQL support includes:

- **Schema validation** - Define schemas inline or from files
- **Query/Mutation resolvers** - Return mock data for operations
- **Argument matching** - Conditional responses based on arguments
- **Introspection** - Full introspection support for tooling
- **Subscriptions** - WebSocket-based real-time data streaming
- **Template support** - Dynamic responses with variables

## Quick Start

Create a minimal GraphQL mock:

```yaml
version: "1.0"

mocks:
  - id: my-graphql-api
    name: User API
    type: graphql
    enabled: true
    graphql:
      path: /graphql
      introspection: true
      schema: |
        type Query {
          users: [User!]!
          user(id: ID!): User
        }
        type User {
          id: ID!
          name: String!
          email: String
        }

      resolvers:
        Query.users:
          response:
            - id: "1"
              name: "Alice"
              email: "alice@example.com"
            - id: "2"
              name: "Bob"
              email: "bob@example.com"

        Query.user:
          response:
            id: "1"
            name: "Alice"
            email: "alice@example.com"
```

Start the server and test:

```bash
# Start mockd
mockd serve --config mockd.yaml

# Query users
curl -X POST http://localhost:4280/graphql \
  -H "Content-Type: application/json" \
  -d '{"query": "{ users { id name } }"}'

# Response:
# {"data":{"users":[{"id":"1","name":"Alice"},{"id":"2","name":"Bob"}]}}
```

## Configuration

### Full Configuration Reference

```yaml
mocks:
  - id: graphql-endpoint
    name: My GraphQL API
    type: graphql
    enabled: true
    graphql:
      # Endpoint path (required)
      path: /graphql

      # Schema definition - use either inline or file
      schema: |
        type Query {
          # Schema SDL here
        }
      # OR
      schemaFile: ./schemas/api.graphql

      # Enable introspection queries (default: false)
      introspection: true

      # Resolver configurations
      resolvers:
        Query.fieldName:
          response: # Response data
          delay: "100ms" # Optional delay
          match: # Optional argument matching
            args:
              id: "123"
          error: # Return an error instead
            message: "Error message"
            path: ["fieldName"]
            extensions:
              code: ERROR_CODE

      # Subscription configurations (WebSocket)
      subscriptions:
        messageAdded:
          events:
            - data: { id: "1", text: "Hello" }
            - data: { id: "2", text: "World" }
              delay: "1s"
          timing:
            fixedDelay: "500ms"
            repeat: true
```

### Configuration Fields

| Field | Type | Description |
|-------|------|-------------|
| `path` | string | GraphQL endpoint path (e.g., `/graphql`) |
| `schema` | string | Inline GraphQL SDL schema |
| `schemaFile` | string | Path to external `.graphql` schema file |
| `introspection` | boolean | Enable `__schema` and `__type` queries |
| `resolvers` | map | Resolver configurations by field path |
| `subscriptions` | map | Subscription configurations by field name |

## Schema Definition

Define your GraphQL schema either inline or in an external file.

### Inline Schema

```yaml
graphql:
  schema: |
    type Query {
      user(id: ID!): User
      users(status: String): [User!]!
    }

    type Mutation {
      createUser(input: CreateUserInput!): User!
      updateUser(id: ID!, input: UpdateUserInput!): User
      deleteUser(id: ID!): Boolean!
    }

    type User {
      id: ID!
      email: String!
      name: String!
      role: Role!
      status: Status!
      createdAt: String!
      profile: Profile
    }

    type Profile {
      bio: String
      avatar: String
      location: String
    }

    input CreateUserInput {
      email: String!
      name: String!
      role: Role
    }

    input UpdateUserInput {
      email: String
      name: String
      role: Role
    }

    enum Role {
      ADMIN
      USER
      GUEST
    }

    enum Status {
      ACTIVE
      INACTIVE
      PENDING
    }
```

### External Schema File

```yaml
graphql:
  schemaFile: ./schemas/api.graphql
```

Create `schemas/api.graphql`:

```graphql
type Query {
  user(id: ID!): User
  users: [User!]!
}

type User {
  id: ID!
  name: String!
  email: String
}
```

### Schema Validation

Validate your schema before starting the server:

```bash
mockd graphql validate schema.graphql
```

Output:

```
Schema valid: schema.graphql
  Types: 5
  Queries: 2
  Mutations: 3
```

## Resolvers

Resolvers define how GraphQL fields return mock data. Use the format `Type.field` to specify resolvers.

### Basic Resolver

Return static data for a query:

```yaml
resolvers:
  Query.users:
    response:
      - id: "user_001"
        email: "alice@example.com"
        name: "Alice Smith"
        role: "ADMIN"
      - id: "user_002"
        email: "bob@example.com"
        name: "Bob Johnson"
        role: "USER"

  Query.user:
    response:
      id: "user_001"
      email: "alice@example.com"
      name: "Alice Smith"
      role: "ADMIN"
      profile:
        bio: "Platform administrator"
        location: "San Francisco, CA"
```

### Mutation Resolvers

```yaml
resolvers:
  Mutation.createUser:
    response:
      id: "user_new"
      email: "newuser@example.com"
      name: "New User"
      role: "USER"
      status: "PENDING"
      createdAt: "{{now}}"

  Mutation.updateUser:
    response:
      id: "user_001"
      email: "alice.updated@example.com"
      name: "Alice Smith (Updated)"
      updatedAt: "{{now}}"

  Mutation.deleteUser:
    response: true
```

### Response Delay

Simulate network latency:

```yaml
resolvers:
  Query.users:
    response:
      - id: "1"
        name: "Alice"
    delay: 500ms

  Query.slowQuery:
    response: { status: "completed" }
    delay: 2s
```

### Dynamic Responses with Templates

Use template expressions in responses:

```yaml
resolvers:
  Mutation.createUser:
    response:
      id: "{{uuid}}"
      name: "{{request.body.variables.name}}"
      email: "{{request.body.variables.email}}"
      createdAt: "{{now}}"

  Query.user:
    response:
      id: "{{args.id}}"
      name: "User {{args.id}}"
```

Available templates:

| Template | Description |
|----------|-------------|
| `{{uuid}}` | Random UUID |
| `{{now}}` | Current ISO timestamp |
| `{{timestamp}}` | Unix timestamp |
| `{{args.fieldName}}` | Argument value from query |
| `{{request.body.variables.name}}` | Variable from request |

## Argument Matching

Return different responses based on query arguments.

### Match Specific Arguments

```yaml
resolvers:
  Query.user:
    match:
      args:
        id: "123"
    response:
      id: "123"
      name: "John Doe"
      email: "john@example.com"
```

### Multiple Resolvers with Different Matches

Define multiple resolver entries by using resolver lists (when you need conditional matching, configure multiple mocks):

```yaml
mocks:
  # Resolver for specific user
  - id: graphql-user-123
    type: graphql
    enabled: true
    graphql:
      path: /graphql
      schema: |
        type Query { user(id: ID!): User }
        type User { id: ID!, name: String!, email: String }
      resolvers:
        Query.user:
          match:
            args:
              id: "123"
          response:
            id: "123"
            name: "Admin User"
            email: "admin@example.com"
```

### Error on Specific Arguments

Return an error for certain inputs:

```yaml
resolvers:
  Mutation.deleteUser:
    match:
      args:
        id: "nonexistent"
    error:
      message: "User not found"
      path: ["deleteUser"]
      extensions:
        code: NOT_FOUND
```

## Introspection

When introspection is enabled, mockd responds to `__schema` and `__type` queries based on your schema definition.

### Enable Introspection

```yaml
graphql:
  introspection: true
```

### Test Introspection

```bash
# Query schema types
curl -X POST http://localhost:4280/graphql \
  -H "Content-Type: application/json" \
  -d '{"query": "{ __schema { types { name } } }"}'

# Query specific type
curl -X POST http://localhost:4280/graphql \
  -H "Content-Type: application/json" \
  -d '{"query": "{ __type(name: \"User\") { name fields { name type { name } } } }"}'
```

Introspection enables GraphQL tooling:

- GraphQL IDEs (GraphiQL, Apollo Studio)
- Code generators
- Schema documentation tools
- Client libraries with auto-completion

### Disable for Production-like Testing

```yaml
graphql:
  introspection: false
```

Queries to `__schema` will return:

```json
{"errors":[{"message":"introspection is disabled"}]}
```

## Variables and Arguments

Handle GraphQL variables in queries and mutations.

### Query with Variables

```graphql
query GetUser($id: ID!) {
  user(id: $id) {
    id
    name
    email
  }
}
```

Request:

```bash
curl -X POST http://localhost:4280/graphql \
  -H "Content-Type: application/json" \
  -d '{
    "query": "query GetUser($id: ID!) { user(id: $id) { id name } }",
    "variables": {"id": "123"}
  }'
```

### Named Operations

```graphql
query ListUsers {
  users {
    id
    name
  }
}

query GetUser($id: ID!) {
  user(id: $id) {
    id
    name
    email
  }
}
```

Specify operation name:

```bash
curl -X POST http://localhost:4280/graphql \
  -H "Content-Type: application/json" \
  -d '{
    "query": "query ListUsers { users { id name } } query GetUser($id: ID!) { user(id: $id) { id name email } }",
    "operationName": "GetUser",
    "variables": {"id": "123"}
  }'
```

### Using Variables in Resolvers

Reference argument values in responses:

```yaml
resolvers:
  Query.user:
    response:
      id: "{{args.id}}"
      name: "User {{args.id}}"
      fetchedAt: "{{now}}"
```

## Subscriptions

GraphQL subscriptions stream real-time data over WebSocket connections.

### Configuration

```yaml
graphql:
  schema: |
    type Query {
      _: String
    }
    type Subscription {
      messageAdded(channel: String!): Message
      notifications: Notification
    }
    type Message {
      id: ID!
      text: String!
      timestamp: String!
    }
    type Notification {
      id: ID!
      type: String!
      message: String!
    }

  subscriptions:
    messageAdded:
      events:
        - data:
            id: "1"
            text: "Hello from mockd!"
            timestamp: "2024-01-15T10:00:00Z"
        - data:
            id: "2"
            text: "Another message"
            timestamp: "2024-01-15T10:00:01Z"
          delay: "1s"
      timing:
        fixedDelay: "500ms"
        repeat: false

    notifications:
      events:
        - data:
            id: "n1"
            type: "info"
            message: "System update available"
      timing:
        fixedDelay: "2s"
        repeat: true
```

### Timing Options

| Field | Description | Example |
|-------|-------------|---------|
| `fixedDelay` | Fixed delay between events | `"500ms"`, `"2s"` |
| `randomDelay` | Random delay range | `"100ms-500ms"` |
| `repeat` | Repeat events after sequence ends | `true`/`false` |

### Event-specific Delays

```yaml
subscriptions:
  messageAdded:
    events:
      - data: { id: "1", text: "Immediate" }
      - data: { id: "2", text: "After 1 second" }
        delay: "1s"
      - data: { id: "3", text: "After 2 more seconds" }
        delay: "2s"
```

### Variable Substitution

Use subscription arguments in event data:

```yaml
subscriptions:
  messageAdded:
    events:
      - data:
          id: "1"
          channel: "{{args.channel}}"
          text: "Message in {{vars.channel}}"
```

### WebSocket Protocols

mockd supports both GraphQL WebSocket protocols:

- **graphql-transport-ws** (modern) - Recommended
- **graphql-ws** / **subscriptions-transport-ws** (legacy)

Connect with your client:

```javascript
// Apollo Client
import { GraphQLWsLink } from '@apollo/client/link/subscriptions';
import { createClient } from 'graphql-ws';

const wsLink = new GraphQLWsLink(
  createClient({
    url: 'ws://localhost:4280/graphql',
  })
);
```

## Error Responses

Configure error responses for testing error handling.

### Simple Error

```yaml
resolvers:
  Query.restrictedData:
    error:
      message: "Unauthorized access"
```

Response:

```json
{
  "data": null,
  "errors": [{
    "message": "Unauthorized access"
  }]
}
```

### Error with Path and Extensions

```yaml
resolvers:
  Mutation.deleteUser:
    match:
      args:
        id: "protected"
    error:
      message: "Cannot delete protected user"
      path: ["deleteUser"]
      extensions:
        code: FORBIDDEN
        userId: "protected"
        reason: "System account"
```

Response:

```json
{
  "data": {"deleteUser": null},
  "errors": [{
    "message": "Cannot delete protected user",
    "path": ["deleteUser"],
    "extensions": {
      "code": "FORBIDDEN",
      "userId": "protected",
      "reason": "System account"
    }
  }]
}
```

## Examples

### E-Commerce API

```yaml
mocks:
  - id: ecommerce-graphql
    name: E-Commerce API
    type: graphql
    enabled: true
    graphql:
      path: /graphql
      introspection: true
      schema: |
        type Query {
          products(category: String, limit: Int): [Product!]!
          product(id: ID!): Product
          cart: Cart
          orders: [Order!]!
        }

        type Mutation {
          addToCart(productId: ID!, quantity: Int!): Cart!
          checkout: Order!
        }

        type Product {
          id: ID!
          name: String!
          price: Float!
          category: String!
          inStock: Boolean!
        }

        type Cart {
          id: ID!
          items: [CartItem!]!
          total: Float!
        }

        type CartItem {
          product: Product!
          quantity: Int!
        }

        type Order {
          id: ID!
          items: [CartItem!]!
          total: Float!
          status: String!
          createdAt: String!
        }

      resolvers:
        Query.products:
          response:
            - id: "prod_001"
              name: "Wireless Headphones"
              price: 79.99
              category: "Electronics"
              inStock: true
            - id: "prod_002"
              name: "Running Shoes"
              price: 129.99
              category: "Sports"
              inStock: true
            - id: "prod_003"
              name: "Coffee Maker"
              price: 49.99
              category: "Home"
              inStock: false

        Query.product:
          response:
            id: "{{args.id}}"
            name: "Product {{args.id}}"
            price: 99.99
            category: "General"
            inStock: true

        Query.cart:
          response:
            id: "cart_001"
            items:
              - product:
                  id: "prod_001"
                  name: "Wireless Headphones"
                  price: 79.99
                quantity: 2
            total: 159.98

        Mutation.addToCart:
          response:
            id: "cart_001"
            items:
              - product:
                  id: "{{args.productId}}"
                  name: "Added Product"
                  price: 99.99
                quantity: "{{args.quantity}}"
            total: 199.98

        Mutation.checkout:
          response:
            id: "order_{{uuid}}"
            items: []
            total: 159.98
            status: "CONFIRMED"
            createdAt: "{{now}}"
          delay: 500ms
```

### User Authentication API

```yaml
mocks:
  - id: auth-graphql
    name: Auth API
    type: graphql
    enabled: true
    graphql:
      path: /graphql
      introspection: true
      schema: |
        type Query {
          me: User
        }

        type Mutation {
          login(email: String!, password: String!): AuthPayload!
          register(input: RegisterInput!): AuthPayload!
          refreshToken(token: String!): AuthPayload!
        }

        type User {
          id: ID!
          email: String!
          name: String!
          role: String!
        }

        type AuthPayload {
          token: String!
          refreshToken: String!
          user: User!
          expiresAt: String!
        }

        input RegisterInput {
          email: String!
          password: String!
          name: String!
        }

      resolvers:
        Query.me:
          response:
            id: "user_current"
            email: "user@example.com"
            name: "Current User"
            role: "USER"

        Mutation.login:
          response:
            token: "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."
            refreshToken: "refresh_{{uuid}}"
            user:
              id: "user_001"
              email: "{{args.email}}"
              name: "Authenticated User"
              role: "USER"
            expiresAt: "{{now}}"
          delay: 200ms

        Mutation.register:
          response:
            token: "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."
            refreshToken: "refresh_{{uuid}}"
            user:
              id: "user_{{uuid}}"
              email: "{{request.body.variables.input.email}}"
              name: "{{request.body.variables.input.name}}"
              role: "USER"
            expiresAt: "{{now}}"
```

### Real-time Chat with Subscriptions

```yaml
mocks:
  - id: chat-graphql
    name: Chat API
    type: graphql
    enabled: true
    graphql:
      path: /graphql
      introspection: true
      schema: |
        type Query {
          messages(roomId: ID!): [Message!]!
          rooms: [Room!]!
        }

        type Mutation {
          sendMessage(roomId: ID!, text: String!): Message!
          createRoom(name: String!): Room!
        }

        type Subscription {
          messageAdded(roomId: ID!): Message
          userTyping(roomId: ID!): TypingIndicator
        }

        type Message {
          id: ID!
          roomId: ID!
          text: String!
          sender: User!
          timestamp: String!
        }

        type Room {
          id: ID!
          name: String!
          members: [User!]!
        }

        type User {
          id: ID!
          name: String!
          avatar: String
        }

        type TypingIndicator {
          userId: ID!
          userName: String!
          isTyping: Boolean!
        }

      resolvers:
        Query.messages:
          response:
            - id: "msg_001"
              roomId: "{{args.roomId}}"
              text: "Welcome to the room!"
              sender:
                id: "user_system"
                name: "System"
              timestamp: "2024-01-15T10:00:00Z"

        Query.rooms:
          response:
            - id: "room_general"
              name: "General"
              members:
                - id: "user_001"
                  name: "Alice"
            - id: "room_random"
              name: "Random"
              members: []

        Mutation.sendMessage:
          response:
            id: "msg_{{uuid}}"
            roomId: "{{args.roomId}}"
            text: "{{args.text}}"
            sender:
              id: "user_current"
              name: "You"
            timestamp: "{{now}}"

      subscriptions:
        messageAdded:
          events:
            - data:
                id: "msg_live_001"
                roomId: "{{args.roomId}}"
                text: "Someone joined the room"
                sender:
                  id: "user_002"
                  name: "Bob"
                timestamp: "2024-01-15T10:01:00Z"
            - data:
                id: "msg_live_002"
                roomId: "{{args.roomId}}"
                text: "Hello everyone!"
                sender:
                  id: "user_002"
                  name: "Bob"
                timestamp: "2024-01-15T10:01:05Z"
              delay: "2s"
          timing:
            fixedDelay: "1s"

        userTyping:
          events:
            - data:
                userId: "user_002"
                userName: "Bob"
                isTyping: true
            - data:
                userId: "user_002"
                userName: "Bob"
                isTyping: false
              delay: "2s"
          timing:
            fixedDelay: "5s"
            repeat: true
```

## CLI Commands

### Validate Schema

Validate a GraphQL schema file:

```bash
mockd graphql validate schema.graphql
```

```
Schema valid: schema.graphql
  Types: 8
  Queries: 3
  Mutations: 4
```

### Execute Query

Execute a query against a running GraphQL endpoint:

```bash
# Simple query
mockd graphql query http://localhost:4280/graphql "{ users { id name } }"

# Query with variables
mockd graphql query http://localhost:4280/graphql \
  "query GetUser(\$id: ID!) { user(id: \$id) { name } }" \
  -v '{"id": "123"}'

# Query from file
mockd graphql query http://localhost:4280/graphql @query.graphql

# With custom headers
mockd graphql query http://localhost:4280/graphql "{ me { name } }" \
  -H "Authorization:Bearer token123"

# Specify operation name
mockd graphql query http://localhost:4280/graphql @operations.graphql \
  -o GetUserById \
  -v '{"id": "456"}'
```

### Query Command Options

| Flag | Description |
|------|-------------|
| `-v, --variables` | JSON string of variables |
| `-o, --operation` | Operation name for multi-operation documents |
| `-H, --header` | Additional headers (`key:value,key2:value2`) |
| `--pretty` | Pretty print output (default: true) |

### Initialize GraphQL Template

Create a new project with GraphQL configuration:

```bash
mockd init --template graphql-api
```

This generates a complete GraphQL mock configuration with:

- User type with CRUD operations
- Query and Mutation types
- Sample resolvers
- Introspection enabled

## Testing Tips

### Test Query Parsing

Validate queries against your schema before running tests:

```bash
# Check if query is valid against schema
curl -X POST http://localhost:4280/graphql \
  -H "Content-Type: application/json" \
  -d '{"query": "{ invalidField }"}'

# Response shows validation error:
# {"errors":[{"message":"validation error: ..."}]}
```

### Test Error Handling

Configure error responses to test client error handling:

```yaml
resolvers:
  Query.users:
    error:
      message: "Service temporarily unavailable"
      extensions:
        code: SERVICE_UNAVAILABLE
        retryAfter: 30
```

### Test with Different Content Types

mockd supports multiple content types:

```bash
# application/json (default)
curl -X POST http://localhost:4280/graphql \
  -H "Content-Type: application/json" \
  -d '{"query": "{ users { id } }"}'

# application/graphql
curl -X POST http://localhost:4280/graphql \
  -H "Content-Type: application/graphql" \
  -d '{ users { id } }'

# GET request with query parameters
curl "http://localhost:4280/graphql?query=%7B%20users%20%7B%20id%20%7D%20%7D"
```

### Test Latency Simulation

Use delays to test timeout handling:

```yaml
resolvers:
  Query.slowQuery:
    response: { status: "ok" }
    delay: 5s  # Test client timeout behavior
```

### Test with GraphQL Clients

Use your favorite GraphQL client or IDE:

- **GraphiQL** - In-browser IDE
- **Apollo Studio** - Full-featured GraphQL IDE
- **Postman** - API testing with GraphQL support
- **Insomnia** - REST and GraphQL client

All support introspection for auto-completion when `introspection: true`.

## Next Steps

- [Response Templating](response-templating.md) - Dynamic response values
- [Request Matching](request-matching.md) - HTTP-level matching
- [Configuration Reference](../reference/configuration.md) - Full configuration schema
