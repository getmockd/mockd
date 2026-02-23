---
title: gRPC Mocking
description: Create mock gRPC services for testing gRPC clients with protobuf support, all RPC types, request matching, and server reflection.
---

gRPC mocking enables you to create mock gRPC services for testing gRPC clients. Configure unary RPCs, streaming methods, and server reflection with protobuf definitions.

## Overview

mockd's gRPC support includes:

- **Protobuf support** - Use `.proto` files to define your service schema
- **All RPC types** - Unary, server streaming, client streaming, and bidirectional
- **Request matching** - Conditional responses based on metadata and request fields
- **Server reflection** - Enable tooling discovery with `grpcurl` and gRPC UI
- **Error simulation** - Return gRPC status codes with detailed error messages
- **Template support** - Dynamic responses with variables

## Quick Start

Create a minimal gRPC mock. First, create your proto file `protos/greeter.proto`:

```protobuf
syntax = "proto3";
package helloworld;

service Greeter {
  rpc SayHello (HelloRequest) returns (HelloReply) {}
}

message HelloRequest {
  string name = 1;
}

message HelloReply {
  string message = 1;
}
```

Then create your mockd configuration:

```yaml
version: "1.0"

mocks:
  - id: my-grpc-service
    name: Greeter Service
    type: grpc
    enabled: true
    grpc:
      port: 50051
      protoFile: ./protos/greeter.proto
      reflection: true
      services:
        helloworld.Greeter:
          methods:
            SayHello:
              response:
                message: "Hello, World!"
```

Start the server and test:

```bash
# Start mockd
mockd serve --config mockd.yaml

# List services (requires grpcurl)
grpcurl -plaintext localhost:50051 list

# Call SayHello
grpcurl -plaintext -d '{"name": "World"}' \
  localhost:50051 helloworld.Greeter/SayHello

# Response:
# {
#   "message": "Hello, World!"
# }
```

## Configuration

### Full Configuration Reference

```yaml
mocks:
  - id: grpc-endpoint
    name: My gRPC Service
    type: grpc
    enabled: true
    grpc:
      # gRPC server port (required)
      port: 50051

      # Proto file path (required)
      protoFile: ./protos/service.proto

      # Multiple proto files (alternative to protoFile)
      protoFiles:
        - ./protos/service.proto
        - ./protos/messages.proto

      # Import paths for proto dependencies
      importPaths:
        - ./protos
        - ./vendor/googleapis

      # Enable gRPC server reflection (default: false)
      reflection: true

      # Service and method configurations
      services:
        package.ServiceName:
          methods:
            MethodName:
              response: # Single response (unary/server streaming)
              responses: # Multiple responses (streaming)
              delay: "100ms" # Response delay
              streamDelay: "50ms" # Delay between stream messages
              match: # Conditional matching
                metadata:
                  key: "value"
                request:
                  field: "value"
              error: # Return gRPC error
                code: NOT_FOUND
                message: "Resource not found"
```

### Configuration Fields

| Field | Type | Description |
|-------|------|-------------|
| `port` | int | gRPC server port |
| `protoFile` | string | Path to a single `.proto` file |
| `protoFiles` | []string | Paths to multiple `.proto` files |
| `importPaths` | []string | Additional proto import paths |
| `reflection` | boolean | Enable gRPC server reflection |
| `services` | map | Service and method configurations |

## Proto File Configuration

mockd requires protobuf definitions to validate and parse messages. Proto files must be provided as file paths.

### Single Proto File

```yaml
grpc:
  protoFile: ./protos/service.proto
```

Create `protos/service.proto`:

```protobuf
syntax = "proto3";

package users;

service UserService {
  rpc GetUser (GetUserRequest) returns (User) {}
  rpc ListUsers (ListUsersRequest) returns (stream User) {}
}

message GetUserRequest {
  string id = 1;
}

message ListUsersRequest {
  int32 page_size = 1;
}

message User {
  string id = 1;
  string name = 2;
  string email = 3;
}
```

### Multiple Proto Files

When your service spans multiple proto files:

```yaml
grpc:
  protoFiles:
    - ./protos/service.proto
    - ./protos/messages.proto
    - ./protos/common.proto
```

### Import Paths

Configure import paths for proto dependencies:

```yaml
grpc:
  protoFile: ./protos/service.proto
  importPaths:
    - ./protos
    - ./vendor/googleapis
    - ./third_party/protobuf
```

This allows proto files to import other definitions:

```protobuf
import "google/protobuf/timestamp.proto";
import "common/types.proto";
```

## Service Definition

Configure responses for each service and method using the fully qualified service name (package.ServiceName).

### Basic Service Configuration

```yaml
services:
  helloworld.Greeter:
    methods:
      SayHello:
        response:
          message: "Hello!"

  users.UserService:
    methods:
      GetUser:
        response:
          id: "123"
          name: "John Doe"
          email: "john@example.com"
```

### Multiple Methods

```yaml
services:
  users.UserService:
    methods:
      GetUser:
        response:
          id: "123"
          name: "John Doe"
          email: "john@example.com"

      CreateUser:
        response:
          id: "{{uuid}}"
          name: "New User"
          created_at: "{{now}}"
        delay: "100ms"

      DeleteUser:
        response:
          success: true
```

## Method Responses

### Unary RPC (Single Request, Single Response)

The most common RPC type - one request, one response:

```yaml
services:
  UserService:
    methods:
      GetUser:
        response:
          id: "123"
          name: "John Doe"
          email: "john@example.com"
          role: "ADMIN"
```

### Server Streaming (Single Request, Multiple Responses)

Return multiple messages in response to a single request:

```yaml
services:
  UserService:
    methods:
      ListUsers:
        responses:
          - id: "1"
            name: "Alice"
            email: "alice@example.com"
          - id: "2"
            name: "Bob"
            email: "bob@example.com"
          - id: "3"
            name: "Carol"
            email: "carol@example.com"
        streamDelay: "100ms"
```

### Client Streaming (Multiple Requests, Single Response)

Receive multiple messages and return a single response:

```yaml
services:
  UserService:
    methods:
      BatchCreate:
        response:
          count: 3
          success: true
```

### Bidirectional Streaming

Both client and server send multiple messages:

```yaml
services:
  ChatService:
    methods:
      Chat:
        responses:
          - type: "ack"
            message: "Received"
          - type: "ack"
            message: "Processed"
        streamDelay: "50ms"
```

### Response Delay

Simulate network latency:

```yaml
services:
  UserService:
    methods:
      GetUser:
        response:
          id: "123"
          name: "John"
        delay: "500ms"

      SlowOperation:
        response:
          status: "completed"
        delay: "2s"
```

### Stream Delay

Control timing between streamed messages:

```yaml
services:
  UserService:
    methods:
      ListUsers:
        responses:
          - id: "1"
            name: "Alice"
          - id: "2"
            name: "Bob"
          - id: "3"
            name: "Carol"
        streamDelay: "200ms"  # 200ms between each message
```

### Dynamic Responses with Templates

Use template expressions in responses:

```yaml
services:
  UserService:
    methods:
      CreateUser:
        response:
          id: "{{uuid}}"
          name: "New User"
          created_at: "{{now}}"
          timestamp: "{{timestamp}}"

      GetUser:
        response:
          id: "user_123"
          name: "Dynamic User"
          fetched_at: "{{now}}"
```

Available templates:

| Template | Description |
|----------|-------------|
| `{{uuid}}` | Random UUID |
| `{{now}}` | Current ISO timestamp |
| `{{timestamp}}` | Unix timestamp |

## Request Matching

Return different responses based on metadata or request field values.

### Metadata Matching

Match requests based on gRPC metadata (headers):

```yaml
services:
  UserService:
    methods:
      GetUser:
        match:
          metadata:
            authorization: "Bearer token123"
            x-request-id: "req-*"
        response:
          id: "123"
          name: "Authenticated User"
```

Metadata matching supports:
- Exact match: `authorization: "Bearer token123"`
- Wildcard match: `x-request-id: "req-*"`

### Request Field Matching

Match based on message field values:

```yaml
services:
  UserService:
    methods:
      GetUser:
        match:
          request:
            id: "123"
        response:
          id: "123"
          name: "John Doe"
          email: "john@example.com"
```

### Combined Matching

Match both metadata and request fields:

```yaml
services:
  UserService:
    methods:
      GetUser:
        match:
          metadata:
            authorization: "Bearer valid-token"
          request:
            id: "123"
        response:
          id: "123"
          name: "John Doe"
          email: "john@example.com"
```

### Multiple Match Conditions

Configure multiple mocks with different match conditions for the same method:

```yaml
mocks:
  # Match specific user
  - id: grpc-user-123
    type: grpc
    enabled: true
    grpc:
      port: 50051
      protoFile: ./user.proto
      services:
        UserService:
          methods:
            GetUser:
              match:
                request:
                  id: "123"
              response:
                id: "123"
                name: "Admin User"

  # Match not found case
  - id: grpc-user-not-found
    type: grpc
    enabled: true
    grpc:
      port: 50051
      protoFile: ./user.proto
      services:
        UserService:
          methods:
            GetUser:
              match:
                request:
                  id: "999"
              error:
                code: NOT_FOUND
                message: "User with ID 999 not found"
```

## gRPC Errors

Return gRPC status codes with detailed error information.

### Basic Error

```yaml
services:
  UserService:
    methods:
      GetUser:
        error:
          code: NOT_FOUND
          message: "User not found"
```

### Error with Details

```yaml
services:
  UserService:
    methods:
      GetUser:
        error:
          code: NOT_FOUND
          message: "User not found"
          details:
            type: "UserError"
            user_id: "123"
            reason: "No user exists with this ID"
```

### Common gRPC Status Codes

| Code | Description |
|------|-------------|
| `OK` | Success |
| `CANCELLED` | Operation cancelled |
| `UNKNOWN` | Unknown error |
| `INVALID_ARGUMENT` | Invalid argument provided |
| `DEADLINE_EXCEEDED` | Timeout exceeded |
| `NOT_FOUND` | Resource not found |
| `ALREADY_EXISTS` | Resource already exists |
| `PERMISSION_DENIED` | Permission denied |
| `RESOURCE_EXHAUSTED` | Resource exhausted (rate limit) |
| `FAILED_PRECONDITION` | Precondition failed |
| `ABORTED` | Operation aborted |
| `OUT_OF_RANGE` | Out of range |
| `UNIMPLEMENTED` | Not implemented |
| `INTERNAL` | Internal error |
| `UNAVAILABLE` | Service unavailable |
| `DATA_LOSS` | Data loss |
| `UNAUTHENTICATED` | Not authenticated |

### Conditional Errors

Return errors based on request conditions:

```yaml
services:
  UserService:
    methods:
      GetUser:
        match:
          request:
            id: "forbidden"
        error:
          code: PERMISSION_DENIED
          message: "Access to this user is forbidden"
          details:
            user_id: "forbidden"
            required_role: "ADMIN"
```

## Reflection Support

Enable gRPC server reflection to allow tooling to discover services and methods at runtime.

### Enable Reflection

```yaml
grpc:
  reflection: true
```

### Benefits of Reflection

With reflection enabled, clients can:

- Discover available services and methods
- Get message type information
- Use tools like `grpcurl` without proto files
- Enable IDE auto-completion

### Testing Reflection

```bash
# List all services
grpcurl -plaintext localhost:50051 list

# Output:
# grpc.reflection.v1alpha.ServerReflection
# helloworld.Greeter

# Describe a service
grpcurl -plaintext localhost:50051 describe helloworld.Greeter

# Output:
# helloworld.Greeter is a service:
# service Greeter {
#   rpc SayHello ( .helloworld.HelloRequest ) returns ( .helloworld.HelloReply );
# }

# Describe a message
grpcurl -plaintext localhost:50051 describe helloworld.HelloRequest
```

### Disable for Production-like Testing

```yaml
grpc:
  reflection: false
```

Without reflection, clients need proto files to make requests.

## Examples

### User Service

Create `protos/users.proto`:

```protobuf
syntax = "proto3";

package users;

service UserService {
  rpc GetUser (GetUserRequest) returns (User) {}
  rpc ListUsers (ListUsersRequest) returns (stream User) {}
  rpc CreateUser (CreateUserRequest) returns (User) {}
  rpc UpdateUser (UpdateUserRequest) returns (User) {}
  rpc DeleteUser (DeleteUserRequest) returns (DeleteUserResponse) {}
}

message GetUserRequest { string id = 1; }
message ListUsersRequest { int32 page_size = 1; string page_token = 2; }
message CreateUserRequest { string name = 1; string email = 2; string role = 3; }
message UpdateUserRequest { string id = 1; string name = 2; string email = 3; }
message DeleteUserRequest { string id = 1; }
message DeleteUserResponse { bool success = 1; }
message User {
  string id = 1;
  string name = 2;
  string email = 3;
  string role = 4;
  string created_at = 5;
  string updated_at = 6;
}
```

Then configure in `mockd.yaml`:

```yaml
version: "1.0"

mocks:
  - id: user-grpc-service
    name: User Service
    type: grpc
    enabled: true
    grpc:
      port: 50051
      protoFile: ./protos/users.proto
      reflection: true
      services:
        users.UserService:
          methods:
            GetUser:
              response:
                id: "user_001"
                name: "John Doe"
                email: "john@example.com"
                role: "USER"
                created_at: "2024-01-15T10:00:00Z"
              delay: "50ms"

            ListUsers:
              responses:
                - id: "user_001"
                  name: "Alice Smith"
                  email: "alice@example.com"
                  role: "ADMIN"
                - id: "user_002"
                  name: "Bob Johnson"
                  email: "bob@example.com"
                  role: "USER"
              streamDelay: "100ms"

            CreateUser:
              response:
                id: "{{uuid}}"
                name: "New User"
                email: "new@example.com"
                role: "USER"
                created_at: "{{now}}"

            DeleteUser:
              response:
                success: true
```

### Chat Service with Bidirectional Streaming

Create `protos/chat.proto`:

```protobuf
syntax = "proto3";

package chat;

service ChatService {
  rpc SendMessage (ChatMessage) returns (ChatAck) {}
  rpc StreamMessages (ChatRoom) returns (stream ChatMessage) {}
  rpc Chat (stream ChatMessage) returns (stream ChatMessage) {}
}

message ChatRoom { string room_id = 1; }
message ChatMessage {
  string id = 1;
  string room_id = 2;
  string sender = 3;
  string text = 4;
  string timestamp = 5;
}
message ChatAck { string message_id = 1; bool delivered = 2; }
```

Then configure:

```yaml
version: "1.0"

mocks:
  - id: chat-grpc-service
    name: Chat Service
    type: grpc
    enabled: true
    grpc:
      port: 50052
      protoFile: ./protos/chat.proto
      reflection: true
      services:
        chat.ChatService:
          methods:
            SendMessage:
              response:
                message_id: "{{uuid}}"
                delivered: true
              delay: "20ms"

            StreamMessages:
              responses:
                - id: "msg_001"
                  room_id: "general"
                  sender: "system"
                  text: "Welcome to the chat!"
                - id: "msg_002"
                  room_id: "general"
                  sender: "alice"
                  text: "Hello everyone!"
              streamDelay: "500ms"

            Chat:
              responses:
                - id: "echo_001"
                  sender: "bot"
                  text: "Message received"
              streamDelay: "100ms"
```

### Order Service with Error Handling

Create `protos/orders.proto`:

```protobuf
syntax = "proto3";

package orders;

service OrderService {
  rpc GetOrder (GetOrderRequest) returns (Order) {}
  rpc CreateOrder (CreateOrderRequest) returns (Order) {}
  rpc CancelOrder (CancelOrderRequest) returns (CancelOrderResponse) {}
}

message GetOrderRequest { string order_id = 1; }
message CreateOrderRequest { string customer_id = 1; repeated OrderItem items = 2; }
message OrderItem { string product_id = 1; int32 quantity = 2; }
message CancelOrderRequest { string order_id = 1; string reason = 2; }
message CancelOrderResponse { bool success = 1; string message = 2; }
message Order {
  string id = 1;
  string customer_id = 2;
  repeated OrderItem items = 3;
  string status = 4;
  double total = 5;
  string created_at = 6;
}
```

Then configure with multiple match conditions:

```yaml
version: "1.0"

mocks:
  # Successful order lookup
  - id: order-grpc-success
    name: Order Service - Success
    type: grpc
    enabled: true
    grpc:
      port: 50053
      protoFile: ./protos/orders.proto
      reflection: true
      services:
        orders.OrderService:
          methods:
            GetOrder:
              match:
                request:
                  order_id: "order_123"
              response:
                id: "order_123"
                customer_id: "cust_001"
                status: "CONFIRMED"
                total: 99.99

            CreateOrder:
              response:
                id: "{{uuid}}"
                customer_id: "cust_001"
                status: "PENDING"
                created_at: "{{now}}"
              delay: "200ms"

  # Order not found
  - id: order-grpc-not-found
    name: Order Service - Not Found
    type: grpc
    enabled: true
    grpc:
      port: 50053
      protoFile: ./protos/orders.proto
      services:
        orders.OrderService:
          methods:
            GetOrder:
              match:
                request:
                  order_id: "nonexistent"
              error:
                code: NOT_FOUND
                message: "Order not found"

  # Cancel order - permission denied
  - id: order-grpc-permission-denied
    name: Order Service - Permission Denied
    type: grpc
    enabled: true
    grpc:
      port: 50053
      protoFile: ./protos/orders.proto
      services:
        orders.OrderService:
          methods:
            CancelOrder:
              match:
                request:
                  order_id: "shipped_order"
              error:
                code: FAILED_PRECONDITION
                message: "Cannot cancel shipped order"
                details:
                  order_id: "shipped_order"
                  status: "SHIPPED"
```

## CLI Commands

### Add a gRPC Mock

Create gRPC mocks directly from the command line using `mockd grpc add`:

```bash
# Basic unary RPC
mockd grpc add --proto ./protos/greeter.proto \
  --service helloworld.Greeter \
  --rpc-method SayHello \
  --response '{"message": "Hello!"}'

# With custom gRPC port
mockd grpc add --proto ./protos/users.proto \
  --service users.UserService \
  --rpc-method GetUser \
  --response '{"id": "123", "name": "John Doe"}' \
  --grpc-port 50052
```

Output:

```
Created mock: grpc_1dc8695005df8cde
  Type: grpc
  Added:
    - helloworld.Greeter/SayHello
  Total services:
    - helloworld.Greeter/SayHello
```

When adding multiple methods to the same port, mocks are automatically merged:

```bash
# Add another method to the same service
mockd grpc add --proto ./protos/users.proto \
  --service users.UserService \
  --rpc-method CreateUser \
  --response '{"id": "new_1", "name": "New User"}'

# Output:
# Merged into mock: grpc_1dc8695005df8cde
#   Type: grpc
#   Added:
#     - users.UserService/CreateUser
#   Total services:
#     - users.UserService/GetUser
#     - users.UserService/CreateUser
```

#### Add Command Flags

| Flag | Description |
|------|-------------|
| `--proto` | Path to `.proto` file (required) |
| `--service` | Fully qualified service name (e.g., `package.Service`) |
| `--rpc-method` | RPC method name |
| `--response` | JSON response body |
| `--grpc-port` | gRPC server port (default: 50051) |
| `--admin-url` | Admin API URL (default: `http://localhost:4290`) |

### List Services and Methods

Inspect a proto file to see available services and methods:

```bash
# List services from a proto file
mockd grpc list api.proto

# With import path
mockd grpc list api.proto -I ./proto
```

Output:

```
Services in api.proto:

  users.UserService
    - GetUser (GetUserRequest) returns (User)
    - ListUsers (ListUsersRequest) returns (stream User)
    - CreateUser (CreateUserRequest) returns (User)
```

### Call a gRPC Method

Test gRPC endpoints directly from the CLI:

```bash
# Call a unary method
mockd grpc call localhost:50051 users.UserService/GetUser '{"id": "123"}'

# With metadata
mockd grpc call localhost:50051 users.UserService/GetUser '{"id": "123"}' \
  -m "authorization:Bearer token123"

# Request body from file
mockd grpc call localhost:50051 users.UserService/CreateUser @request.json

# Plaintext mode (no TLS)
mockd grpc call localhost:50051 users.UserService/GetUser '{"id": "123"}' --plaintext
```

### Call Command Options

| Flag | Description |
|------|-------------|
| `-m, --metadata` | gRPC metadata as `key:value,key2:value2` |
| `--plaintext` | Use plaintext (no TLS, default: true) |
| `--pretty` | Pretty print output (default: true) |

### Initialize gRPC Template

Create a new project with gRPC configuration:

```bash
mockd init --template grpc-service
```

This generates a complete gRPC mock configuration with:

- Greeter service with multiple RPC types
- Unary, server streaming, client streaming, and bidirectional examples
- Reflection enabled
- Sample proto definition

## Testing with grpcurl

[grpcurl](https://github.com/fullstorydev/grpcurl) is a command-line tool for interacting with gRPC servers.

### Installation

```bash
# macOS
brew install grpcurl

# Linux (download from releases)
curl -sSL https://github.com/fullstorydev/grpcurl/releases/download/v1.8.9/grpcurl_1.8.9_linux_x86_64.tar.gz | tar xz

# Go install
go install github.com/fullstorydev/grpcurl/cmd/grpcurl@latest
```

### List Services

```bash
# With reflection enabled
grpcurl -plaintext localhost:50051 list

# Output:
# grpc.reflection.v1alpha.ServerReflection
# helloworld.Greeter
```

### Describe Service

```bash
grpcurl -plaintext localhost:50051 describe helloworld.Greeter

# Output:
# helloworld.Greeter is a service:
# service Greeter {
#   rpc SayHello ( .helloworld.HelloRequest ) returns ( .helloworld.HelloReply );
#   rpc SayHelloStream ( .helloworld.HelloRequest ) returns ( stream .helloworld.HelloReply );
# }
```

### Call Unary Method

```bash
grpcurl -plaintext \
  -d '{"name": "World"}' \
  localhost:50051 helloworld.Greeter/SayHello

# Output:
# {
#   "message": "Hello, World!"
# }
```

### Call Server Streaming Method

```bash
grpcurl -plaintext \
  -d '{"name": "World"}' \
  localhost:50051 helloworld.Greeter/SayHelloStream

# Output (multiple messages):
# {
#   "message": "Hello! (1/3)"
# }
# {
#   "message": "Hello again! (2/3)"
# }
# {
#   "message": "Hello one more time! (3/3)"
# }
```

### With Metadata

```bash
grpcurl -plaintext \
  -H "authorization: Bearer token123" \
  -H "x-request-id: req-001" \
  -d '{"id": "123"}' \
  localhost:50051 users.UserService/GetUser
```

### Without Reflection

If reflection is disabled, provide the proto file:

```bash
grpcurl -plaintext \
  -proto ./protos/service.proto \
  -d '{"name": "World"}' \
  localhost:50051 helloworld.Greeter/SayHello
```

### From File

```bash
# Save request to file
echo '{"name": "World"}' > request.json

# Call with file input
grpcurl -plaintext \
  -d @ \
  localhost:50051 helloworld.Greeter/SayHello < request.json
```

## Testing Tips

### Test All RPC Types

Ensure your client handles all streaming types correctly:

```yaml
services:
  TestService:
    methods:
      # Unary
      UnaryMethod:
        response: { status: "ok" }

      # Server streaming
      ServerStream:
        responses:
          - { seq: 1 }
          - { seq: 2 }
          - { seq: 3 }
        streamDelay: "100ms"

      # Client streaming
      ClientStream:
        response:
          count: 5
          processed: true

      # Bidirectional
      BidiStream:
        responses:
          - { echo: "received" }
        streamDelay: "50ms"
```

### Test Error Handling

Verify your client handles gRPC errors properly:

```yaml
services:
  UserService:
    methods:
      # Test not found
      GetUser:
        match:
          request:
            id: "nonexistent"
        error:
          code: NOT_FOUND
          message: "User not found"

      # Test validation error
      CreateUser:
        match:
          request:
            email: ""
        error:
          code: INVALID_ARGUMENT
          message: "Email is required"

      # Test authentication error
      DeleteUser:
        error:
          code: UNAUTHENTICATED
          message: "Authentication required"
```

### Test Timeouts

Use delays to test client timeout behavior:

```yaml
services:
  SlowService:
    methods:
      SlowMethod:
        response: { status: "completed" }
        delay: "30s"  # Will trigger client timeout
```

### Test with Different Clients

mockd works with any gRPC client:

- **grpcurl** - Command-line testing
- **BloomRPC** - GUI client for gRPC
- **Postman** - API testing with gRPC support
- **gRPC UI** - Web-based gRPC client
- **Your application** - Native gRPC clients in any language

## Next Steps

- [Response Templating](/guides/response-templating) - Dynamic response values
- [Request Matching](/guides/request-matching) - Advanced matching patterns
- [Configuration Reference](/reference/configuration) - Full configuration schema
