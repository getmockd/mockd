---
title: Configuration Reference
description: Complete reference for mockd configuration files, including all mock types, server settings, and validation options.
---

Complete reference for mockd configuration files.

## File Format

mockd supports YAML and JSON configuration files. The `version` field is required.

```bash
mockd serve --config mocks.yaml
mockd serve --config mocks.json
```

## Top-Level Structure

```yaml
version: "1.0"

mocks:
  - id: string
    name: string
    type: http | websocket | graphql | grpc | mqtt | soap | oauth
    enabled: boolean
    http: { ... }      # if type: http
    websocket: { ... } # if type: websocket
    graphql: { ... }   # if type: graphql
    grpc: { ... }      # if type: grpc
    mqtt: { ... }      # if type: mqtt
    soap: { ... }      # if type: soap
    oauth: { ... }     # if type: oauth

serverConfig: { ... }       # Optional server settings
statefulResources: [ ... ]  # Optional CRUD resources
customOperations: [ ... ]   # Optional multi-step operations
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `version` | string | Yes | Config version (e.g., `"1.0"`) |
| `mocks` | array | Yes | Mock definitions |
| `serverConfig` | object | No | Server configuration |
| `statefulResources` | array | No | Stateful CRUD resources |
| `customOperations` | array | No | Multi-step custom operations with expression evaluation |

---

## Mock Definition

All mock types share common fields:

```yaml
mocks:
  - id: unique-mock-id
    name: "Human-readable name"
    description: "Optional description"
    type: http
    enabled: true
    parentId: ""           # Folder ID (optional)
    metaSortKey: 0         # Sort order (optional)
    http: { ... }          # Type-specific configuration
```

### Common Fields

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `id` | string | No | Auto-generated | Unique identifier |
| `type` | string | No | Inferred | Mock type: `http`, `websocket`, `graphql`, `grpc`, `mqtt`, `soap`, `oauth` |
| `name` | string | No | | Human-readable name |
| `description` | string | No | | Longer description |
| `enabled` | boolean | No | `true` | Whether mock is active |
| `parentId` | string | No | | Folder ID for organization |
| `metaSortKey` | number | No | | Manual ordering within folder |

---

## HTTP Mock

HTTP mocks match incoming requests and return configured responses.

```yaml
mocks:
  - id: get-users
    name: Get Users
    type: http
    enabled: true
    http:
      priority: 0
      matcher:
        method: GET
        path: /api/users
        headers:
          Authorization: "Bearer *"
        queryParams:
          status: active
      response:
        statusCode: 200
        headers:
          Content-Type: application/json
        body: '{"users": []}'
        delayMs: 100
```

### HTTP Spec Fields

| Field | Type | Description |
|-------|------|-------------|
| `priority` | integer | Match priority (higher = matches first) |
| `matcher` | object | Request matching criteria |
| `response` | object | Response definition |
| `sse` | object | Server-Sent Events config (instead of response) |
| `chunked` | object | Chunked transfer config (instead of response) |
| `validation` | object | Request validation ([see Validation](#validation)) |

### HTTP Matcher

| Field | Type | Description |
|-------|------|-------------|
| `method` | string | HTTP method (GET, POST, PUT, DELETE, PATCH, etc.) |
| `path` | string | URL path (supports `{param}` syntax for path parameters) |
| `pathPattern` | string | Regex pattern for URL path |
| `headers` | map | Header matchers (exact match or glob patterns with `*`) |
| `queryParams` | map | Query parameter matchers (exact match) |
| `bodyContains` | string | Body must contain this string |
| `bodyEquals` | string | Body must equal this string exactly |
| `bodyPattern` | string | Body must match this regex pattern |
| `bodyJsonPath` | map | JSONPath matchers (path: expected value) |
| `mtls` | object | mTLS client certificate matching |

### Path Patterns

```yaml
# Exact match
path: /api/users

# Path parameters
path: /api/users/{id}
path: /api/{resource}/{id}

# Greedy path parameter (matches multiple segments)
path: /api/files/{path:.*}

# Regex pattern
pathPattern: "/api/users/[0-9]+"
```

### HTTP Response

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `statusCode` | integer | `200` | HTTP status code |
| `headers` | map | `{}` | Response headers |
| `body` | string | `""` | Response body (supports templates) |
| `bodyFile` | string | | Load body from file path |
| `delayMs` | integer | `0` | Response delay in milliseconds |

### mTLS Matching

```yaml
matcher:
  mtls:
    cn: "client.example.com"     # Common Name pattern
    ou: "Engineering"            # Organizational Unit pattern
    o: "Example Corp"            # Organization pattern
    san:
      dns: "*.example.com"       # DNS SAN pattern
      email: "*@example.com"     # Email SAN pattern
      ip: "10.0.0.*"             # IP SAN pattern
```

### SSE (Server-Sent Events)

```yaml
http:
  matcher:
    method: GET
    path: /events
  sse:
    events:
      - type: update
        data: '{"status": "connected"}'
        id: "1"
      - type: update
        data: '{"status": "processing"}'
        delay: 1000
    timing:
      fixedDelay: 1000          # ms between events
      initialDelay: 0           # ms before first event
    lifecycle:
      maxEvents: 10             # max events before closing
      timeout: 60000            # connection timeout ms
      keepaliveInterval: 15     # keepalive interval in seconds
    resume:
      enabled: true             # support Last-Event-ID
      bufferSize: 100           # events to buffer
```

### Chunked Transfer

```yaml
http:
  matcher:
    method: GET
    path: /stream
  chunked:
    chunkSize: 1024         # bytes per chunk
    chunkDelay: 100         # ms between chunks
    data: "..."             # data to stream
    dataFile: ./large.json  # or load from file
    format: ndjson          # optional: ndjson format
    ndjsonItems:            # for ndjson format
      - {"id": 1}
      - {"id": 2}
```

---

## WebSocket Mock

WebSocket mocks handle bidirectional message communication.

```yaml
mocks:
  - id: chat-ws
    name: Chat WebSocket
    type: websocket
    enabled: true
    websocket:
      path: /ws/chat
      subprotocols:
        - chat
        - json
      requireSubprotocol: false
      echoMode: true
      maxMessageSize: 65536
      idleTimeout: "5m"
      maxConnections: 100
      heartbeat:
        enabled: true
        interval: "30s"
        timeout: "10s"
      matchers:
        - match:
            type: exact
            value: "ping"
          response:
            type: text
            value: "pong"
        - match:
            type: json
            path: "$.type"
            value: "join"
          response:
            type: json
            value:
              type: "joined"
              message: "Welcome!"
      defaultResponse:
        type: json
        value:
          type: "echo"
          message: "{{message}}"
```

### WebSocket Spec Fields

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `path` | string | Required | WebSocket upgrade path |
| `subprotocols` | array | `[]` | Supported subprotocols |
| `requireSubprotocol` | boolean | `false` | Require matching subprotocol |
| `echoMode` | boolean | `false` | Echo received messages |
| `maxMessageSize` | integer | `65536` | Max message size (bytes) |
| `idleTimeout` | string | | Connection idle timeout |
| `maxConnections` | integer | `0` | Max concurrent connections (0 = unlimited) |
| `heartbeat` | object | | Ping/pong keepalive config |
| `matchers` | array | `[]` | Message matching rules |
| `defaultResponse` | object | | Response when no matcher matches |
| `scenario` | object | | Scripted message sequence |

### WebSocket Match Criteria

| Field | Type | Description |
|-------|------|-------------|
| `type` | string | Match type: `exact`, `contains`, `regex`, `json` |
| `value` | string | Value to match |
| `path` | string | JSONPath for `json` type |
| `messageType` | string | Filter by message type: `text`, `binary` |

### WebSocket Message Response

| Field | Type | Description |
|-------|------|-------------|
| `type` | string | Response type: `text`, `json`, `binary` |
| `value` | any | Response content (string or object for json) |
| `delay` | string | Delay before sending (e.g., "100ms") |

### WebSocket Heartbeat

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `enabled` | boolean | `false` | Enable ping/pong |
| `interval` | string | `"30s"` | Ping interval |
| `timeout` | string | `"10s"` | Pong timeout |

### WebSocket Scenario

```yaml
websocket:
  scenario:
    name: "onboarding"
    loop: false
    resetOnReconnect: true
    steps:
      - type: send
        message:
          type: json
          value: {"type": "welcome"}
      - type: wait
        duration: "1s"
      - type: expect
        match:
          type: json
          path: "$.type"
          value: "ready"
        timeout: "10s"
        optional: false
```

---

## GraphQL Mock

GraphQL mocks provide a full GraphQL API endpoint.

```yaml
mocks:
  - id: graphql-api
    name: GraphQL API
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
          email: String!
        }
      resolvers:
        Query.users:
          response:
            - id: "1"
              name: "Alice"
              email: "alice@example.com"
        Query.user:
          response:
            id: "1"
            name: "Alice"
            email: "alice@example.com"
```

### GraphQL Spec Fields

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `path` | string | Required | GraphQL endpoint path |
| `schema` | string | | Inline SDL schema |
| `schemaFile` | string | | Path to .graphql schema file |
| `introspection` | boolean | `false` | Enable introspection queries |
| `resolvers` | map | `{}` | Field resolver configurations |
| `subscriptions` | map | `{}` | Subscription configurations |

### GraphQL Resolvers

Resolvers are keyed by `Type.field`:

```yaml
resolvers:
  Query.users:
    response:
      - { id: "1", name: "Alice" }
      - { id: "2", name: "Bob" }
    delay: "100ms"
  
  Query.user:
    # Match specific arguments
    match:
      args:
        id: "1"
    response:
      id: "1"
      name: "Alice"
  
  Mutation.createUser:
    response:
      id: "{{uuid}}"
      name: "New User"
    
  Query.error:
    error:
      message: "Something went wrong"
      path: ["error"]
      extensions:
        code: "INTERNAL_ERROR"
```

---

## gRPC Mock

gRPC mocks provide a gRPC service endpoint.

```yaml
mocks:
  - id: grpc-greeter
    name: Greeter Service
    type: grpc
    enabled: true
    grpc:
      port: 50051
      reflection: true
      protoFile: |
        syntax = "proto3";
        package helloworld;
        
        service Greeter {
          rpc SayHello (HelloRequest) returns (HelloReply) {}
          rpc SayHelloStream (HelloRequest) returns (stream HelloReply) {}
        }
        
        message HelloRequest {
          string name = 1;
        }
        
        message HelloReply {
          string message = 1;
        }
      services:
        helloworld.Greeter:
          methods:
            SayHello:
              response:
                message: "Hello, World!"
            SayHelloStream:
              responses:
                - message: "Hello 1"
                - message: "Hello 2"
                - message: "Hello 3"
              streamDelay: "500ms"
```

### gRPC Spec Fields

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `port` | integer | Required | gRPC server port |
| `protoFile` | string | | Inline proto definition |
| `protoFiles` | array | | Paths to .proto files |
| `importPaths` | array | | Proto import paths |
| `reflection` | boolean | `false` | Enable gRPC reflection |
| `services` | map | `{}` | Service configurations |

### gRPC Method Config

```yaml
services:
  package.Service:
    methods:
      MethodName:
        response:           # Single response
          field: value
        responses:          # Multiple responses (streaming)
          - { field: value1 }
          - { field: value2 }
        delay: "100ms"      # Response delay
        streamDelay: "50ms" # Delay between stream messages
        match:              # Request matching
          metadata:
            authorization: "Bearer *"
          request:
            field: expected_value
        error:
          code: "NOT_FOUND"
          message: "Resource not found"
          details:
            type: "ErrorInfo"
```

---

## MQTT Mock

MQTT mocks provide an MQTT broker.

```yaml
mocks:
  - id: mqtt-broker
    name: IoT MQTT Broker
    type: mqtt
    enabled: true
    mqtt:
      port: 1883
      tls:
        enabled: false
        certFile: ./certs/mqtt.crt
        keyFile: ./certs/mqtt.key
      auth:
        enabled: false
        users:
          - username: device
            password: secret123
            acl:
              - topic: "sensors/#"
                access: publish
              - topic: "commands/#"
                access: subscribe
      topics:
        - topic: sensors/temperature
          qos: 1
          retain: true
          messages:
            - payload: '{"temp": 22, "unit": "celsius"}'
              interval: "5s"
              repeat: true
        - topic: commands/device/+
          qos: 1
          onPublish:
            response:
              payload: '{"status": "ack"}'
            forward: responses/device
```

### MQTT Spec Fields

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `port` | integer | Required | MQTT broker port |
| `tls` | object | | TLS configuration |
| `auth` | object | | Authentication configuration |
| `topics` | array | `[]` | Topic configurations |

### MQTT Topic Config

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `topic` | string | Required | Topic pattern (supports `+` and `#` wildcards) |
| `qos` | integer | `0` | Quality of Service (0, 1, 2) |
| `retain` | boolean | `false` | Retain last message |
| `messages` | array | | Messages to publish |
| `onPublish` | object | | Handler for received messages |
| `deviceSimulation` | object | | Simulate multiple devices |

### MQTT Message Config

| Field | Type | Description |
|-------|------|-------------|
| `payload` | string | Message payload (supports templates) |
| `delay` | string | Initial delay before sending |
| `interval` | string | Repeat interval |
| `repeat` | boolean | Whether to repeat |

---

## SOAP Mock

SOAP mocks provide SOAP/WSDL service endpoints.

```yaml
mocks:
  - id: soap-service
    name: Calculator Service
    type: soap
    enabled: true
    soap:
      path: /soap/calculator
      wsdlFile: ./calculator.wsdl  # or inline with wsdl:
      operations:
        Add:
          soapAction: "http://example.com/Add"
          response: |
            <AddResponse>
              <AddResult>{{xpath://Add/a}}</AddResult>
            </AddResponse>
          delay: "50ms"
          match:
            xpath:
              "//a": "10"
          fault:
            code: "Server.InvalidInput"
            message: "Invalid input provided"
            detail: "<errorCode>1001</errorCode>"
```

### SOAP Spec Fields

| Field | Type | Description |
|-------|------|-------------|
| `path` | string | SOAP endpoint path |
| `wsdl` | string | Inline WSDL definition |
| `wsdlFile` | string | Path to WSDL file |
| `operations` | map | Operation configurations |

### SOAP Operation Config

| Field | Type | Description |
|-------|------|-------------|
| `soapAction` | string | SOAPAction header value |
| `response` | string | XML response body |
| `delay` | string | Response delay |
| `match` | object | XPath-based request matching |
| `fault` | object | SOAP fault response |
| `statefulResource` | string | Name of stateful resource for CRUD operations |
| `statefulAction` | string | CRUD action: `get`, `list`, `create`, `update`, `patch`, `delete`, `custom` |

> **Note:** When `statefulResource` is set, the operation gets its response from the stateful resource — `response` and `fault` fields are not required. `statefulResource` and `statefulAction` must be set together.

---

## Custom Operations

Custom operations compose reads, writes, and expression-evaluated transforms against stateful resources.

```yaml
version: "1.0"

customOperations:
  - name: TransferFunds
    steps:
      - type: read
        resource: accounts
        id: "input.sourceId"
        as: source
      - type: read
        resource: accounts
        id: "input.destId"
        as: dest
      - type: update
        resource: accounts
        id: "input.sourceId"
        set:
          balance: "source.balance - input.amount"
      - type: update
        resource: accounts
        id: "input.destId"
        set:
          balance: "dest.balance + input.amount"
    response:
      status: '"completed"'
```

### Custom Operation Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `name` | string | Yes | Unique operation name |
| `steps` | array | Yes | Ordered sequence of steps |
| `response` | map | No | Field → expression map for building the result |

### Step Config

| Field | Type | Description |
|-------|------|-------------|
| `type` | string | Step type: `read`, `create`, `update`, `delete`, `set` |
| `resource` | string | Stateful resource name (for read/create/update/delete) |
| `id` | string | Expression resolving to item ID (for read/update/delete) |
| `as` | string | Variable name to store the result (for read/create) |
| `set` | map | Field → expression map (for create/update) |
| `var` | string | Variable name (for set steps) |
| `value` | string | Expression value (for set steps) |

Expressions use [expr-lang/expr](https://github.com/expr-lang/expr) syntax. The environment includes `input` (request data) and variables from prior steps.

---

## Server Configuration

Server settings can be included in the config file.

> **Note:** Port settings (`httpPort`, `httpsPort`, `adminPort`) from config files are currently overridden by CLI flags. Use `--port` and `--admin-port` flags to set ports:
> ```bash
> mockd serve --config myconfig.yaml --port 4280 --admin-port 4290
> ```

```yaml
version: "1.0"

serverConfig:
  httpPort: 4280
  httpsPort: 4283
  adminPort: 4290
  logRequests: true
  maxLogEntries: 1000
  maxBodySize: 10485760     # 10MB
  readTimeout: 30           # seconds
  writeTimeout: 30          # seconds
  tls:
    enabled: false
    certFile: ./certs/server.crt
    keyFile: ./certs/server.key
    autoGenerateCert: true
  mtls:
    enabled: false
    clientAuth: "require-and-verify"
    caCertFile: ./certs/ca.crt
    allowedCNs:
      - "client.example.com"

mocks: [...]
```

### Server Config Fields

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `httpPort` | integer | `4280` | HTTP server port (0 = disabled) |
| `httpsPort` | integer | `0` | HTTPS server port (0 = disabled) |
| `adminPort` | integer | `4290` | Admin API port |
| `managementPort` | integer | `4281` | Engine management API port (internal) |
| `logRequests` | boolean | `true` | Enable request logging |
| `maxLogEntries` | integer | `1000` | Max log entries to retain |
| `maxBodySize` | integer | `10485760` | Max request body size (bytes) |
| `readTimeout` | integer | `30` | HTTP read timeout (seconds) |
| `writeTimeout` | integer | `30` | HTTP write timeout (seconds) |

The `managementPort` is used for internal communication between the Admin API and the mock engine. In standalone mode, you typically don't need to configure this.

### TLS Configuration

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `enabled` | boolean | `false` | Enable TLS/HTTPS |
| `certFile` | string | | Path to certificate file |
| `keyFile` | string | | Path to private key file |
| `autoGenerateCert` | boolean | `false` | Auto-generate self-signed cert |

### mTLS Configuration

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `enabled` | boolean | `false` | Enable mTLS |
| `clientAuth` | string | `"none"` | Client auth mode |
| `caCertFile` | string | | CA certificate file |
| `caCertFiles` | array | | Multiple CA certificate files |
| `allowedCNs` | array | | Allowed client Common Names |
| `allowedOUs` | array | | Allowed Organizational Units |

Client auth modes:
- `none` - No client certificate requested
- `request` - Client certificate requested but not required
- `require` - Client certificate required but not verified
- `verify-if-given` - Verify client certificate if provided
- `require-and-verify` - Require and verify client certificate

### CORS Configuration

Configure Cross-Origin Resource Sharing for the mock server.

```yaml
serverConfig:
  cors:
    enabled: true
    allowOrigins:
      - "http://localhost:3000"
      - "https://app.example.com"
    allowMethods:
      - GET
      - POST
      - PUT
      - DELETE
      - OPTIONS
    allowHeaders:
      - Content-Type
      - Authorization
      - X-Requested-With
    exposeHeaders:
      - X-Request-ID
    allowCredentials: false
    maxAge: 86400
```

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `enabled` | boolean | `true` | Enable CORS handling |
| `allowOrigins` | array | `["http://localhost:*"]` | Allowed origins (use `["*"]` for any) |
| `allowMethods` | array | `[GET, POST, PUT, PATCH, DELETE, OPTIONS, HEAD]` | Allowed HTTP methods |
| `allowHeaders` | array | `[Content-Type, Authorization, X-Requested-With, Accept, Origin]` | Allowed request headers |
| `exposeHeaders` | array | `[]` | Headers browsers can access |
| `allowCredentials` | boolean | `false` | Allow credentials (cannot use with `*` origin) |
| `maxAge` | integer | `86400` | Preflight cache duration (seconds) |

**Default behavior:** When not configured, mockd allows requests from localhost origins only. This is secure for local development while preventing cross-origin attacks.

**Wildcard origins:**

```yaml
cors:
  allowOrigins: ["*"]  # Allow any origin (not recommended for production)
```

**Note:** When `allowCredentials: true`, you cannot use wildcard origins.

### Rate Limiting Configuration

Configure rate limiting for the mock server.

```yaml
serverConfig:
  rateLimit:
    enabled: true
    requestsPerSecond: 1000
    burstSize: 2000
    trustedProxies:
      - "10.0.0.0/8"
      - "172.16.0.0/12"
```

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `enabled` | boolean | `false` | Enable rate limiting |
| `requestsPerSecond` | float | `1000` | Requests per second limit |
| `burstSize` | integer | `2000` | Maximum burst size (token bucket) |
| `trustedProxies` | array | `[]` | CIDR ranges for trusted proxies |

**How it works:** Rate limiting uses a token bucket algorithm. The bucket fills at `requestsPerSecond` rate up to `burstSize` tokens. Each request consumes one token.

**Trusted proxies:** When set, mockd trusts `X-Forwarded-For` headers from these IP ranges for accurate client IP detection.

**Example: Strict rate limiting for load testing:**

```yaml
serverConfig:
  rateLimit:
    enabled: true
    requestsPerSecond: 100
    burstSize: 150
```

---

## Stateful Resources

Stateful resources provide automatic CRUD operations with in-memory state. Resource definitions and seed data are persisted to the admin file store, but runtime data (created/updated/deleted items) is held in memory and resets to seed data on restart.

```yaml
version: "1.0"

statefulResources:
  - name: users
    basePath: /api/users
    idField: id
    parentField: ""
    seedData:
      - id: "1"
        name: "Alice"
        email: "alice@example.com"
      - id: "2"
        name: "Bob"
        email: "bob@example.com"

mocks: []
```

### Stateful Resource Fields

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `name` | string | Required | Resource name (e.g., "users") |
| `basePath` | string | Required | Base URL path (e.g., "/api/users") |
| `idField` | string | `"id"` | Field name for resource ID |
| `parentField` | string | | Parent FK field for nested resources |
| `seedData` | array | `[]` | Initial data to load |
| `validation` | object | | Validation rules ([see Validation](#validation)) |

### Generated Endpoints

For a resource with `basePath: /api/users`:

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/users` | List all resources |
| POST | `/api/users` | Create resource |
| GET | `/api/users/{id}` | Get resource by ID |
| PUT | `/api/users/{id}` | Replace resource |
| PATCH | `/api/users/{id}` | Update resource |
| DELETE | `/api/users/{id}` | Delete resource |

### Validation

Stateful resources and HTTP mocks support field-level request validation.

#### StatefulValidation

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `mode` | string | `"strict"` | Validation mode: `strict`, `warn`, `permissive` |
| `auto` | boolean | `false` | Auto-infer rules from seed data |
| `required` | array | `[]` | Required field names (shared) |
| `fields` | map | `{}` | Field validators (shared) |
| `pathParams` | map | `{}` | Path parameter validators |
| `onCreate` | object | | Create-specific validation |
| `onUpdate` | object | | Update-specific validation |
| `schema` | object | | Inline JSON Schema |
| `schemaRef` | string | | Path to JSON Schema file |

#### RequestValidation (for HTTP mocks)

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `mode` | string | `"strict"` | Validation mode: `strict`, `warn`, `permissive` |
| `failStatus` | integer | `400` | HTTP status code for failures |
| `required` | array | `[]` | Required field names |
| `fields` | map | `{}` | Field validators |
| `pathParams` | map | `{}` | Path parameter validators |
| `queryParams` | map | `{}` | Query parameter validators |
| `headers` | map | `{}` | Header validators |
| `schema` | object | | Inline JSON Schema |
| `schemaRef` | string | | Path to JSON Schema file |

#### FieldValidator

| Field | Type | Description |
|-------|------|-------------|
| `type` | string | Expected type: `string`, `number`, `integer`, `boolean`, `array`, `object` |
| `required` | boolean | Field must be present |
| `nullable` | boolean | Allow null values |
| `minLength` | integer | Minimum string length |
| `maxLength` | integer | Maximum string length |
| `pattern` | string | Regex pattern for strings |
| `format` | string | Format: `email`, `uuid`, `date`, `datetime`, `uri`, `ipv4`, `ipv6`, `hostname` |
| `min` | number | Minimum value (inclusive) |
| `max` | number | Maximum value (inclusive) |
| `exclusiveMin` | number | Minimum value (exclusive) |
| `exclusiveMax` | number | Maximum value (exclusive) |
| `minItems` | integer | Minimum array items |
| `maxItems` | integer | Maximum array items |
| `uniqueItems` | boolean | Array items must be unique |
| `items` | object | FieldValidator for array items |
| `enum` | array | Allowed values |
| `properties` | map | Nested object validators |
| `message` | string | Custom error message |

#### Nested Fields

Use dot notation for nested object fields:

```yaml
fields:
  "address.city":
    type: string
    required: true
  "address.zipCode":
    type: string
    pattern: "^[0-9]{5}$"
  "items.sku":
    type: string
    required: true
```

For arrays, the field after the dot applies to each array item:
- `items.sku` validates the `sku` field in each item of the `items` array

See the [Validation Guide](/guides/validation) for comprehensive examples.

---

## Template Variables

Response bodies support template variables:

```yaml
body: |
  {
    "id": "{{request.pathParam.id}}",
    "query": "{{request.query.search}}",
    "header": "{{request.header.Authorization}}",
    "body": {{request.body}},
    "field": "{{jsonPath request.body '$.field'}}",
    "timestamp": "{{now}}",
    "unix": "{{timestamp}}",
    "uuid": "{{uuid}}",
    "random": {{randomInt 1 100}}
  }
```

### Available Variables

| Variable | Description |
|----------|-------------|
| `{{request.method}}` | HTTP method |
| `{{request.path}}` | Request path |
| `{{request.url}}` | Full URL |
| `{{request.pathParam.name}}` | Path parameter value |
| `{{request.query.name}}` | Query parameter value |
| `{{request.header.Name}}` | Request header value |
| `{{request.body}}` | Full request body (raw) |
| `{{jsonPath request.body '$.path'}}` | JSONPath extraction |
| `{{now}}` | ISO 8601 timestamp |
| `{{timestamp}}` | Unix timestamp (seconds) |
| `{{uuid}}` | Random UUID |
| `{{randomInt min max}}` | Random integer |
| `{{randomFloat min max}}` | Random float |

See `mockd help templating` for the complete list.

---

## Complete Example

```yaml
version: "1.0"

serverConfig:
  httpPort: 4280
  adminPort: 4290
  logRequests: true

mocks:
  # HTTP mock
  - id: health-check
    name: Health Check
    type: http
    enabled: true
    http:
      matcher:
        method: GET
        path: /health
      response:
        statusCode: 200
        body: '{"status": "ok"}'

  # HTTP with path parameters
  - id: get-user
    name: Get User
    type: http
    enabled: true
    http:
      matcher:
        method: GET
        path: /api/users/{id}
      response:
        statusCode: 200
        headers:
          Content-Type: application/json
        body: |
          {
            "id": "{{request.pathParam.id}}",
            "name": "User {{request.pathParam.id}}"
          }

  # WebSocket
  - id: ws-echo
    name: Echo WebSocket
    type: websocket
    enabled: true
    websocket:
      path: /ws/echo
      echoMode: true

  # GraphQL
  - id: graphql
    name: GraphQL API
    type: graphql
    enabled: true
    graphql:
      path: /graphql
      introspection: true
      schema: |
        type Query {
          hello: String!
        }
      resolvers:
        Query.hello:
          response: "Hello, World!"

statefulResources:
  - name: posts
    basePath: /api/posts
    seedData:
      - id: "1"
        title: "First Post"
        content: "Hello, World!"
```

## See Also

- [CLI Reference](/reference/cli) - Command-line options
- [Request Matching](/guides/request-matching) - Matching patterns
- `mockd help config` - Built-in configuration help
- `mockd help templating` - Template variable reference
- `mockd init --template list` - Available templates
