---
title: CLI Reference
description: Complete reference for the mockd command-line interface, including all commands, flags, and usage examples.
---

Complete reference for the mockd command-line interface.

## Global Flags

These flags apply to all commands:

| Flag | Description |
|------|-------------|
| `-h, --help` | Show help message |
| `-v, --version` | Show version information |
| `--admin-url` | Admin API base URL (default: http://localhost:4290) |
| `--json` | Output command results in JSON format |

---

## Autocomplete

mockd leverages Cobra to generate autocomplete scripts for popular shells.

### mockd completion

Generate the autocompletion script for the specified shell.

```bash
mockd completion [command]
```

**Commands:**

| Command | Description |
|---------|-------------|
| `bash` | Generate the autocompletion script for bash |
| `fish` | Generate the autocompletion script for fish |
| `powershell` | Generate the autocompletion script for powershell |
| `zsh` | Generate the autocompletion script for zsh |

**Examples:**

```bash
# Load bash completion in the current shell
source <(mockd completion bash)

# Load zsh completion in the current shell
source <(mockd completion zsh)

# Configure bash completion permanently (Linux)
mockd completion bash > /etc/bash_completion.d/mockd
```

---

## Server Commands

### mockd serve

Start the mock server (default command).

```bash
mockd serve [flags]
```

**Flags:**

| Flag | Short | Description | Default |
|------|-------|-------------|---------|
| `--port` | `-p` | HTTP server port | `4280` |
| `--admin-port` | `-a` | Admin API port | `4290` |
| `--config` | `-c` | Path to mock configuration file | |
| `--https-port` | | HTTPS server port (0 = disabled) | `0` |
| `--read-timeout` | | Read timeout in seconds | `30` |
| `--write-timeout` | | Write timeout in seconds | `30` |
| `--request-timeout` | | Request timeout in seconds (sets both read and write timeout) | `0` |
| `--max-log-entries` | | Maximum request log entries | `1000` |
| `--max-connections` | | Maximum concurrent HTTP connections (0 = unlimited) | `0` |
| `--auto-cert` | | Auto-generate TLS certificate | `true` |

**CORS Flags:**

| Flag | Description | Default |
|------|-------------|---------|
| `--cors-origins` | Comma-separated CORS allowed origins (e.g., `'*'` or `'https://app.example.com'`) | |

**Rate Limiting Flags:**

| Flag | Description | Default |
|------|-------------|---------|
| `--rate-limit` | Rate limit in requests per second (0 = disabled) | `0` |

**Persistence Flags:**

| Flag | Description | Default |
|------|-------------|---------|
| `--no-persist` | Disable persistent storage (mocks are lost on restart) | `false` |

**Storage Flags:**

| Flag | Description | Default |
|------|-------------|---------|
| `--data-dir` | Data directory for persistent storage | `~/.local/share/mockd` |
| `--no-auth` | Disable API key authentication on admin API | `false` |

**TLS Flags:**

| Flag | Description |
|------|-------------|
| `--tls-cert` | Path to TLS certificate file |
| `--tls-key` | Path to TLS private key file |
| `--tls-auto` | Auto-generate self-signed certificate |

**mTLS Flags:**

| Flag | Description |
|------|-------------|
| `--mtls-enabled` | Enable mTLS client certificate validation |
| `--mtls-client-auth` | Client auth mode (none, request, require, verify-if-given, require-and-verify) |
| `--mtls-ca` | Path to CA certificate for client validation |
| `--mtls-allowed-cns` | Comma-separated list of allowed Common Names |

**Audit Flags:**

| Flag | Description |
|------|-------------|
| `--audit-enabled` | Enable audit logging |
| `--audit-file` | Path to audit log file |
| `--audit-level` | Log level (debug, info, warn, error) |

**Runtime Mode Flags (register with control plane):**

| Flag | Description | Default |
|------|-------------|---------|
| `--register` | Register with control plane as a runtime | |
| `--control-plane` | Control plane URL | `https://api.mockd.io` |
| `--token` | Runtime token (or MOCKD_RUNTIME_TOKEN env var) | |
| `--name` | Runtime name (required with --register) | |
| `--labels` | Runtime labels (key=value,key2=value2) | |

**Pull Mode Flags:**

| Flag | Description |
|------|-------------|
| `--pull` | mockd:// URI to pull and serve |
| `--cache` | Local cache directory for pulled mocks |

**GraphQL Flags:**

| Flag | Description | Default |
|------|-------------|---------|
| `--graphql-schema` | Path to GraphQL schema file | |
| `--graphql-path` | GraphQL endpoint path | `/graphql` |

**OAuth Flags:**

| Flag | Description |
|------|-------------|
| `--oauth-enabled` | Enable OAuth provider |
| `--oauth-issuer` | OAuth issuer URL |
| `--oauth-port` | OAuth server port |

**Chaos Flags:**

| Flag | Description |
|------|-------------|
| `--chaos-enabled` | Enable chaos injection |
| `--chaos-latency` | Add random latency (e.g., "10ms-100ms") |
| `--chaos-error-rate` | Error rate (0.0-1.0) |

**MCP Flags:**

| Flag | Description | Default |
|------|-------------|---------|
| `--mcp` | Enable MCP (Model Context Protocol) HTTP server | `false` |
| `--mcp-port` | MCP server port | `9091` |
| `--mcp-allow-remote` | Allow remote MCP connections (default: localhost only) | `false` |

**Validation Flags:**

| Flag | Description | Default |
|------|-------------|---------|
| `--validate-spec` | Path to OpenAPI spec for request validation | |
| `--validate-fail` | Fail on validation error | `false` |

**Daemon Flags:**

| Flag | Short | Description | Default |
|------|-------|-------------|---------|
| `--detach` | `-d` | Run server in background (daemon mode) | |
| `--pid-file` | | Path to PID file | `~/.mockd/mockd.pid` |

**Logging Flags:**

| Flag | Description | Default |
|------|-------------|---------|
| `--log-level` | Log level (debug, info, warn, error) | `info` |
| `--log-format` | Log format (text, json) | `text` |
| `--loki-endpoint` | Loki endpoint for log aggregation | |

**Tracing Flags:**

| Flag | Description | Default |
|------|-------------|---------|
| `--otlp-endpoint` | OTLP HTTP endpoint for distributed tracing (e.g., http://localhost:4318/v1/traces) | |
| `--trace-sampler` | Trace sampling ratio (0.0-1.0) | `1.0` |

**Examples:**

```bash
# Start with defaults
mockd serve

# Start with config file on custom port
mockd serve --config mocks.json --port 3000

# Register as a runtime
mockd serve --register --name ci-runner-1 --token $MOCKD_RUNTIME_TOKEN

# Pull and serve from cloud
mockd serve --pull mockd://acme/payment-api

# Start with TLS using certificate files
mockd serve --tls-cert server.crt --tls-key server.key --https-port 8443

# Start with mTLS enabled
mockd serve --mtls-enabled --mtls-ca ca.crt --tls-cert server.crt --tls-key server.key

# Start with audit logging
mockd serve --audit-enabled --audit-file audit.log --audit-level debug

# Start in daemon/background mode
  mockd serve -d

  # Start with distributed tracing (send traces to Jaeger)
  mockd serve --otlp-endpoint http://localhost:4318/v1/traces

  # Start with JSON structured logging
  mockd serve --log-level debug --log-format json

  # Send logs to Loki for aggregation
  mockd serve --loki-endpoint http://localhost:3100/loki/api/v1/push

# Allow CORS from any origin
mockd serve --cors-origins '*'

# Allow CORS from specific origins
mockd serve --cors-origins 'https://app.example.com,https://admin.example.com'

# Rate limit to 100 requests per second
mockd serve --rate-limit 100

# Ephemeral mode — mocks are lost on restart
mockd serve --no-persist

# Watch config file for changes and auto-reload
# Combine: ephemeral server with CORS and rate limiting
mockd serve --no-persist --cors-origins '*' --rate-limit 50 --config mocks.yaml
```

---

### mockd start

Start the mock server (alias for serve with additional directory loading features).

```bash
mockd start [flags]
```

**Flags:**

| Flag | Short | Description | Default |
|------|-------|-------------|---------|
| `--port` | `-p` | HTTP server port | `4280` |
| `--admin-port` | `-a` | Admin API port | `4290` |
| `--config` | `-c` | Path to mock configuration file | |
| `--load` | | Load mocks from directory | |
| `--validate` | | Validate files before serving (with --load) | |
| `--watch` | | Watch for file changes and auto-reload (with --load) | `false` |
| `--engine-name` | | Name for this engine when registering with admin | |
| `--admin-url` | | Admin server URL to register with (enables engine mode) | |
| `--https-port` | | HTTPS server port (0 = disabled) | `0` |
| `--read-timeout` | | Read timeout in seconds | `30` |
| `--write-timeout` | | Write timeout in seconds | `30` |
| `--max-log-entries` | | Maximum request log entries | `1000` |
| `--auto-cert` | | Auto-generate TLS certificate | `true` |
| `--detach` | `-d` | Run server in background (daemon mode) | `false` |
| `--pid-file` | | Path to PID file | `~/.mockd/mockd.pid` |
| `--log-level` | | Log level (debug, info, warn, error) | `info` |
| `--log-format` | | Log format (text, json) | `text` |

Also supports all TLS, mTLS, Audit, GraphQL, gRPC, OAuth, MQTT, Chaos, Validation, and Storage flags from `serve`.

**Examples:**

```bash
# Start with defaults
mockd start

# Start with config file on custom port
mockd start --config mocks.json --port 3000

# Start with HTTPS enabled
mockd start --https-port 8443

# Load mocks from directory
mockd start --load ./mocks/

# Validate mocks before serving
mockd start --load ./mocks/ --validate
```

---

### mockd stop

Stop a running mockd server.

```bash
mockd stop [component] [flags]
```

**Arguments:**

| Argument | Description |
|----------|-------------|
| `component` | Optional component to stop: "admin" or "engine" (if not specified, stops all) |

**Flags:**

| Flag | Short | Description | Default |
|------|-------|-------------|---------|
| `--pid-file` | | Path to PID file | `~/.mockd/mockd.pid` |
| `--force` | `-f` | Send SIGKILL instead of SIGTERM | |
| `--timeout` | | Timeout in seconds to wait for shutdown | `10` |

**Examples:**

```bash
# Stop all components
mockd stop

# Force stop
mockd stop --force

# Stop with custom PID file
mockd stop --pid-file /tmp/mockd.pid

# Stop with longer timeout
mockd stop --timeout 30
```

---

### mockd status

Show status of running mockd server.

```bash
mockd status [flags]
```

**Flags:**

| Flag | Description | Default |
|------|-------------|---------|
| `--pid-file` | Path to PID file | `~/.mockd/mockd.pid` |
| `--json` | Output in JSON format | |

**Examples:**

```bash
# Check server status
mockd status

# Output as JSON
mockd status --json

# Use custom PID file
mockd status --pid-file /tmp/mockd.pid
```

---

### mockd up

Start local admins and engines defined in `mockd.yaml`. Validates the project configuration, starts servers, and bootstraps workspaces seamlessly.

```bash
mockd up [flags]
```

**Flags:**

| Flag | Short | Description | Default |
|------|-------|-------------|---------|
| `--config` | `-f` | Config file path (can be specified multiple times) | |
| `--detach` | `-d` | Run in background (daemon mode) | `false` |
| `--log-level` | | Log level (debug, info, warn, error) | `info` |

**Examples:**

```bash
mockd up
mockd up -f custom-mockd.yaml -d
```

---

### mockd down

Stop all services started by a previous `mockd up` command.

```bash
mockd down [flags]
```

**Flags:**

| Flag | Description | Default |
|------|-------------|---------|
| `--pid-file` | Path to PID file | `~/.mockd/mockd.pid` |
| `--timeout` | Shutdown timeout duration | `30s` |

---

### mockd ports

Show all ports in use by the running mockd server.

```bash
mockd ports [flags]
```

**Flags:**

| Flag | Short | Description | Default |
|------|-------|-------------|---------|
| `--pid-file` | | Path to PID file | `~/.mockd/mockd.pid` |
| `--admin-port` | `-a` | Admin API port to query | `4290` |
| `--json` | | Output in JSON format | |

**Output:**

The command displays a table of all ports with their protocol, component, and status:

```
PORT    PROTOCOL   COMPONENT       STATUS
------- ---------- --------------- --------
1883    MQTT       MQTT Broker     running
4280    HTTP       Mock Engine     running
4290    HTTP       Admin API       running
50051   gRPC       gRPC Server     running
```

**Examples:**

```bash
# Show all ports
mockd ports

# Output as JSON
mockd ports --json

# Query a different admin port
mockd ports --admin-port 8090
```

---

### mockd init

Create a starter mockd configuration file.

```bash
mockd init [flags]
```

**Flags:**

| Flag | Short | Description | Default |
|------|-------|-------------|---------|
| `--force` | | Overwrite existing config file | |
| `--output` | `-o` | Output filename | `mockd.yaml` |
| `--format` | | Output format: yaml or json | inferred from filename |
| `--interactive` | `-i` | Interactive mode - prompts for configuration | |
| `--template` | `-t` | Template to use | `default` |

**Templates:**

| Template | Description |
|----------|-------------|
| `default` | Basic HTTP mocks (hello, echo, health) |
| `crud` | Full REST CRUD API for resources |
| `websocket-chat` | Chat room WebSocket endpoint with echo |
| `graphql-api` | GraphQL API with User CRUD resolvers |
| `grpc-service` | gRPC Greeter service with reflection |
| `mqtt-iot` | MQTT broker with IoT sensor topics |

**Examples:**

```bash
# Create default mockd.yaml
mockd init

# List available templates
mockd init --template list

# Use CRUD API template
mockd init --template crud

# Use WebSocket template with custom output
mockd init -t websocket-chat -o websocket.yaml

# Interactive setup
mockd init -i

# Create with custom filename
mockd init -o my-mocks.yaml

# Create JSON config
mockd init --format json -o mocks.json

# Overwrite existing config
mockd init --force
```

---

## Mock Creation Commands

The `mockd` CLI organizes mock creation by protocol. Each protocol has its own `<protocol> add` subcommand.

By default, if a mock already exists with the same method and path (or equivalent identifiers), it is **updated in place** (upsert behavior). The command prints "Updated mock: ..." when an existing mock is modified and "Created mock: ..." when a new mock is created.

:::note[Upsert by Default]
Use `--allow-duplicate` if you intentionally need multiple mocks on the same route (e.g., with different matchers like headers or query parameters).
:::

:::tip[Interactive Forms]
If you omit **required flags** for complex mock types (like `mockd grpc add` without a `--proto`), mockd will gracefully fall back to an interactive terminal form using `charmbracelet/huh` to walk you through the configuration step-by-step!
:::

### Global Add Flags

The following flags apply to **all** `<protocol> add` commands:

| Flag | Short | Description | Default |
|------|-------|-------------|---------|
| `--name` | `-n` | Mock display name | |
| `--allow-duplicate` | | Create a second mock even if one already exists on the same route | `false` |
| `--admin-url` | | Admin API base URL | `http://localhost:4290` |
| `--json` | | Output in JSON format | |

---

### mockd http add

Add or update an HTTP or SSE mock endpoint.

```bash
mockd http add [flags]
```

**HTTP Flags:**

| Flag | Short | Description | Default |
|------|-------|-------------|---------|
| `--method` | `-m` | HTTP method to match | `GET` |
| `--path` | | URL path to match | Required |
| `--status` | `-s` | Response status code | `200` |
| `--body` | `-b` | Response body | |
| `--body-file` | | Read response body from file | |
| `--header` | `-H` | Response header (key:value), repeatable | |
| `--match-header` | | Required request header (key:value), repeatable | |
| `--match-query` | | Required query param (key=value or key:value), repeatable | |
| `--match-body-contains` | | Match requests whose body contains this string | |
| `--path-pattern` | | Regex path pattern for matching (alternative to `--path`) | |
| `--priority` | | Mock priority (higher = matched first) | |
| `--delay` | | Response delay in milliseconds | |
| `--stateful-operation` | | Wire to a custom stateful operation (e.g., TransferFunds) | |

**SSE Flags (for streaming):**

| Flag | Description | Default |
|------|-------------|---------|
| `--sse` | Enable SSE streaming response | |
| `--sse-event` | SSE event (type:data), repeatable | |
| `--sse-delay` | Delay between events in milliseconds | `100` |
| `--sse-template` | Built-in template: openai-chat, notification-stream | |
| `--sse-repeat` | Repeat events N times (0 = infinite) | `1` |
| `--sse-keepalive` | Keepalive interval in milliseconds (0 = disabled) | `0` |

**Examples:**
```bash
mockd http add --path /api/users --status 200 --body '[{"id":1}]'
mockd http add -m POST --path /api/users -s 201 -b '{"created": true}'
mockd http add --path /events --sse --sse-event 'connected:{"status":"ok"}'
mockd http add -m POST --path /api/transfer --stateful-operation TransferFunds
```

---

### mockd websocket add

Add or update a WebSocket mock endpoint.

```bash
mockd websocket add [flags]
```

**WebSocket Flags:**

| Flag | Description | Default |
|------|-------------|---------|
| `--path` | WebSocket path | Required |
| `--message` | Default response message (JSON) | |
| `--echo` | Enable echo mode | |

**Examples:**
```bash
mockd websocket add --path /ws/chat --echo
mockd websocket add --path /ws/events --message '{"type": "connected"}'
```

---

### mockd graphql add

Add or update a GraphQL mock endpoint.

```bash
mockd graphql add [flags]
```

**GraphQL Flags:**

| Flag | Description | Default |
|------|-------------|---------|
| `--path` | GraphQL endpoint path | `/graphql` |
| `--operation` | Operation name | Required |
| `--op-type` | Operation type: query or mutation | `query` |
| `--response` | JSON response data | |

**Examples:**
```bash
mockd graphql add --operation getUser --response '{"data": {"user": {"id": "1"}}}'
```

---

### mockd graphql validate

Validate a GraphQL schema file.

```bash
mockd graphql validate <schema-file>
```

---

### mockd graphql query

Execute a query against a GraphQL endpoint.

```bash
mockd graphql query <endpoint> <query> [flags]
```

**Flags:**

| Flag | Short | Description | Default |
|------|-------|-------------|---------|
| `--header` | `-H` | Additional headers (key:value,key2:value2) | |
| `--pretty` | | Pretty print output | `true` |

**Examples:**
```bash
mockd graphql query http://localhost:4280/graphql 'query { getUser { id name } }'
```

---

### mockd grpc add

Add or update a gRPC mock endpoint.

```bash
mockd grpc add [flags]
```

**gRPC Flags:**

| Flag | Description | Default |
|------|-------------|---------|
| `--proto` | Path to .proto file (required, repeatable) | |
| `--proto-path` | Import path for proto dependencies (repeatable) | |
| `--service` | Service name, e.g., myapp.UserService (required) | |
| `--rpc-method` | RPC method name (required) | |
| `--response` | JSON response data | |
| `--grpc-port` | gRPC server port | `50051` |

**Examples:**
```bash
mockd grpc add --proto ./user.proto --service myapp.UserService --rpc-method GetUser --response '{"id": "1"}'
```

---

### mockd grpc call

Call a gRPC endpoint directly from the CLI.

```bash
mockd grpc call <endpoint> <service/method> <json-body> [flags]
```

**Flags:**

| Flag | Short | Description | Default |
|------|-------|-------------|---------|
| `--metadata` | `-m` | gRPC metadata (key:value) | |
| `--plaintext` | | Use plaintext connection | `true` |
| `--pretty` | | Pretty print JSON output | `true` |

**Examples:**
```bash
mockd grpc call localhost:50051 myapp.UserService/GetUser '{"id": "1"}'
```

---

### mockd mqtt add

Add or update an MQTT mock endpoint.

```bash
mockd mqtt add [flags]
```

**MQTT Flags:**

| Flag | Description | Default |
|------|-------------|---------|
| `--topic` | Topic pattern | Required |
| `--payload` | Response payload | |
| `--qos` | QoS level: 0, 1, or 2 | `0` |
| `--mqtt-port` | MQTT broker port (required) | |

**Examples:**
```bash
mockd mqtt add --topic sensors/temperature --payload '{"temp": 72.5}' --qos 1
```

---

### mockd mqtt publish

Publish a message to an MQTT topic.

```bash
mockd mqtt publish <broker> <topic> <message> [flags]
```

**Flags:**

| Flag | Short | Description | Default |
|------|-------|-------------|---------|
| `--message` | `-m` | Message to publish (alternative to positional arg) | |
| `--qos` | `-q` | QoS level (0, 1, 2) | `0` |
| `--retain` | `-r` | Retain message | `false` |
| `--username` | `-u` | MQTT username | |
| `--password` | `-P` | MQTT password | |

---

### mockd mqtt subscribe

Subscribe to an MQTT topic and print messages to the console.

```bash
mockd mqtt subscribe <broker> <topic> [flags]
```

**Flags:**

| Flag | Short | Description | Default |
|------|-------|-------------|---------|
| `--qos` | `-q` | QoS level (0, 1, 2) | `0` |
| `--username` | `-u` | MQTT username | |
| `--password` | `-P` | MQTT password | |
| `--count` | `-c` | Number of messages to receive before exiting (0 = infinite) | `0` |

---

### mockd mqtt status

Show the current mockd MQTT broker status.

```bash
mockd mqtt status [flags]
```

**Flags:**

| Flag | Description | Default |
|------|-------------|---------|
| `--admin-url` | Admin API base URL | `http://localhost:4290` |

---

### mockd soap add

Add or update a SOAP mock endpoint.

```bash
mockd soap add [flags]
```

**SOAP Flags:**

| Flag | Description | Default |
|------|-------------|---------|
| `--path` | SOAP endpoint path | `/soap` |
| `--operation` | SOAP operation name | Required |
| `--soap-action` | SOAPAction header value | |
| `--response` | XML response body | |
| `--stateful-resource` | Stateful resource name (e.g., `users`) | |
| `--stateful-action` | Stateful action: `list`, `get`, `create`, `update`, `delete`, `custom` | |

> `--stateful-resource` and `--stateful-action` must be used together. When set, the operation reads/writes the named stateful resource instead of returning a canned response.

**Examples:**
```bash
# Canned response
mockd soap add --operation GetWeather --soap-action "http://example.com/GetWeather" --response '<GetWeatherResponse><Temperature>72</Temperature></GetWeatherResponse>'

# Stateful: list all users from the "users" resource
mockd soap add --path /soap --action GetUsers --stateful-resource users --stateful-action list

# Stateful: get a single user by ID
mockd soap add --path /soap --action GetUser --stateful-resource users --stateful-action get
```

---

### mockd soap import

Generate SOAP mock configurations from a WSDL file.

```bash
mockd soap import <wsdl-file> [flags]
```

**Flags:**

| Flag | Short | Description | Default |
|------|-------|-------------|---------|
| `--stateful` | | Enable stateful CRUD heuristics | `false` |
| `--output` | `-o` | Output file path | stdout |
| `--format` | `-f` | Output format (yaml/json) | `yaml` |

**Examples:**
```bash
# Generate static SOAP mocks from WSDL
mockd soap import service.wsdl

# Generate stateful mocks (auto-detects CRUD operations)
mockd soap import service.wsdl --stateful

# Save to a file
mockd soap import service.wsdl --stateful -o soap-mocks.yaml

# Output as JSON
mockd soap import service.wsdl --format json
```

When `--stateful` is enabled, the importer detects CRUD patterns in operation names (Get, List, Create, Update, Delete) and generates both `statefulResources` definitions and SOAP operations pre-wired with `statefulResource`/`statefulAction` fields.

---

### mockd soap validate

Validate a WSDL file against standard schema rules.

```bash
mockd soap validate <wsdl-file>
```

---

### mockd soap call

Execute a SOAP call against an endpoint.

```bash
mockd soap call <endpoint> <action> <body> [flags]
```

**Flags:**

| Flag | Short | Description | Default |
|------|-------|-------------|---------|
| `--header` | `-H` | Additional headers (key:value,key2:value2) | |
| `--pretty` | | Pretty print output | `true` |
| `--action` | | SOAPAction header value | |
| `--soap12` | | Use SOAP 1.2 envelope format | `false` |
| `--timeout` | | Request timeout in seconds | `30` |

**Examples:**
```bash
mockd soap call http://localhost:4280/soap "http://example.com/GetWeather" '<soapenv:Envelope>...</soapenv:Envelope>'

# SOAP 1.2 with custom action
mockd soap call --soap12 --action "http://example.com/GetWeather" http://localhost:4280/soap GetWeather
```

---

### mockd oauth add

Add or update an OAuth/OIDC mock provider. This creates a full OAuth/OIDC mock server with standard endpoints including `/.well-known/openid-configuration`, `/token`, `/authorize`, `/userinfo`, and `/jwks`.

```bash
mockd oauth add [flags]
```

**OAuth Flags:**

| Flag | Description | Default |
|------|-------------|---------|
| `--issuer` | OAuth issuer URL | `http://localhost:4280` |
| `--client-id` | OAuth client ID | `test-client` |
| `--client-secret` | OAuth client secret | `test-secret` |
| `--oauth-user` | Test username | `testuser` |
| `--oauth-password` | Test password | `password` |

**Examples:**
```bash
mockd oauth add
mockd oauth add --name "Auth Server" --issuer http://localhost:4280/auth --client-id my-app --client-secret s3cret --oauth-user admin --oauth-password admin123
```

---

### mockd oauth list

List all OAuth mocks.

```bash
mockd oauth list [flags]
```

---

### mockd oauth get

Get details of a specific OAuth mock.

```bash
mockd oauth get <id> [flags]
```

---

### mockd oauth delete

Delete an OAuth mock.

```bash
mockd oauth delete <id> [flags]
```

---

### mockd oauth status

Show the OAuth provider status.

```bash
mockd oauth status [flags]
```

---

### mockd stateful

Manage stateful CRUD resources. Stateful resources provide in-memory data stores that can be shared across protocols (HTTP REST, SOAP, GraphQL, gRPC, etc.).

```bash
mockd stateful [command]
```

**Commands:**

| Command | Description |
|---------|-------------|
| `add` | Create a stateful CRUD resource |
| `list` | List all stateful resources |
| `reset` | Reset a stateful resource to seed data |

---

### mockd stateful add

Create a new stateful CRUD resource. By default, resources are "bridge-only" (accessible via SOAP, GraphQL, gRPC, etc. but without HTTP REST endpoints). Use `--path` to also expose HTTP REST endpoints.

```bash
mockd stateful add <name> [flags]
```

**Flags:**

| Flag | Description | Default |
|------|-------------|---------|
| `--path` | URL base path for HTTP REST endpoints (omit for bridge-only) | |
| `--id-field` | Custom ID field name | `id` |

**Examples:**

```bash
# Create with HTTP REST endpoints
mockd stateful add users --path /api/users
# Endpoints: GET/POST /api/users, GET/PUT/DELETE /api/users/{id}

# Bridge-only (no HTTP endpoints, accessible via SOAP/GraphQL/gRPC)
mockd stateful add products

# Custom ID field
mockd stateful add orders --path /api/orders --id-field orderId
```

---

### mockd stateful list

List all stateful resources and their item counts.

```bash
mockd stateful list [flags]
```

**Flags:**

| Flag | Description | Default |
|------|-------------|---------|
| `--limit` | Maximum items to show per resource | `100` |
| `--offset` | Skip this many items | `0` |
| `--sort` | Sort field | |
| `--order` | Sort order (`asc` or `desc`) | |

**Example output:**

```
Stateful Resources (3):

NAME        BASE PATH       ITEMS  SEED  ID FIELD
----        ---------       -----  ----  --------
users       /api/users      5      3     id
products    /api/products   12     10    id
orders      (bridge-only)   0      0     orderId

Total items across all resources: 17
```

---

### mockd stateful reset

Reset a stateful resource to its initial seed data state. All current items are removed and replaced with the original seed data (if any).

```bash
mockd stateful reset <name> [flags]
```

**Examples:**

```bash
# Reset users to seed data
mockd stateful reset users

# Reset with JSON output
mockd stateful reset products --json
```

---

### mockd stateful custom

Manage custom multi-step operations that run against stateful resources. Custom operations define a pipeline of steps (read, create, update, delete, set) with expression-based logic using [expr-lang/expr](https://github.com/expr-lang/expr). They can be invoked via CLI, REST API, SOAP, or any protocol that supports the stateful bridge.

```bash
mockd stateful custom [command]
```

**Commands:**

| Command | Description |
|---------|-------------|
| `list` | List all registered custom operations |
| `get` | Show details of a custom operation |
| `add` | Register a new custom operation |
| `validate` | Validate a custom operation definition (no writes) |
| `run` | Execute a custom operation |
| `delete` | Delete a custom operation |

---

### mockd stateful custom list

List all registered custom operations.

```bash
mockd stateful custom list [flags]
```

**Example:**

```bash
mockd stateful custom list
mockd stateful custom list --json
```

---

### mockd stateful custom get

Show details of a custom operation including its steps and response template.

```bash
mockd stateful custom get <name> [flags]
```

**Example:**

```bash
mockd stateful custom get TransferFunds
mockd stateful custom get TransferFunds --json
```

---

### mockd stateful custom add

Register a new custom operation from a file or inline definition.

```bash
mockd stateful custom add [flags]
```

**Flags:**

| Flag | Description | Default |
|------|-------------|---------|
| `--file` | Path to YAML/JSON file containing the operation definition | |
| `--definition` | Inline JSON operation definition | |

**Examples:**

```bash
# From a YAML file
mockd stateful custom validate --file transfer.yaml
mockd stateful custom add --file transfer.yaml

# Inline JSON definition
mockd stateful custom add --definition '{
  "name": "TransferFunds",
  "consistency": "atomic",
  "steps": [
    {"type": "read", "resource": "accounts", "id": "input.sourceId", "as": "source"},
    {"type": "read", "resource": "accounts", "id": "input.destId", "as": "dest"},
    {"type": "update", "resource": "accounts", "id": "input.sourceId", "set": {"balance": "source.balance - input.amount"}},
    {"type": "update", "resource": "accounts", "id": "input.destId", "set": {"balance": "dest.balance + input.amount"}}
  ],
  "response": {"status": "\"completed\""}
}'
```

**Operation Definition Format:**

```yaml
name: TransferFunds
consistency: atomic
steps:
  - type: read
    resource: accounts
    id: "input.sourceId"
    as: source
  - type: read
    resource: accounts
    id: "input.destId"
    as: dest
  - type: set
    as: total
    value: "source.balance + dest.balance"
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
  total: "string(total)"
```

**Step Types:**

| Type | Description | Required Fields |
|------|-------------|-----------------|
| `read` | Read a single item from a resource | `resource`, `id`, `as` |
| `create` | Create a new item in a resource | `resource`, `set` |
| `update` | Update an existing item in a resource | `resource`, `id`, `set` |
| `delete` | Delete an item from a resource | `resource`, `id` |
| `set` | Set a computed value in the expression context | `var`, `value` |

> All `id`, `value`, and `set` field values are **expr expressions** evaluated against an environment containing `input` (the request data) and all previously computed variables from `as` and `set.var`.

---

### mockd stateful custom validate

Validate a custom operation definition locally before registering it. This command performs preflight checks and does not mutate server state.

```bash
mockd stateful custom validate [flags]
```

**Flags:**

| Flag | Description | Default |
|------|-------------|---------|
| `--file` | Path to YAML/JSON file containing the operation definition | |
| `--definition` | Inline JSON operation definition | |
| `--input` | Inline JSON input example for expression compile checks | |
| `--input-file` | Path to JSON file containing input example | |
| `--fixtures-file` | Path to JSON/YAML fixtures file for runtime expression checks | |
| `--check-resources` | Verify referenced stateful resources exist on the running admin/engine | `false` |
| `--check-expressions-runtime` | Evaluate expressions with sample input/fixtures (no writes) | `false` |
| `--strict` | Treat validation warnings as errors | `false` |

**Examples:**

```bash
# Validate a YAML definition locally (no writes)
mockd stateful custom validate --file transfer.yaml

# Validate with example input to catch expression/env issues
mockd stateful custom validate --file transfer.yaml \
  --input '{"sourceId":"acct-1","destId":"acct-2","amount":100}'

# Validate and verify referenced stateful resources exist on the running engine
mockd stateful custom validate --file transfer.yaml --check-resources

# Runtime-check expressions with sample input + fixtures (no writes)
mockd stateful custom validate --file transfer.yaml \
  --input '{"sourceId":"acct-1","destId":"acct-2","amount":100}' \
  --check-expressions-runtime \
  --fixtures-file transfer-fixtures.json

# Fail on warnings (e.g., empty update/create set maps)
mockd stateful custom validate --file transfer.yaml --strict
```

**Fixtures file (optional, recommended for `--check-expressions-runtime`):**

```json
{
  "resources": {
    "accounts": {
      "acct-1": { "id": "acct-1", "balance": 500 },
      "acct-2": { "id": "acct-2", "balance": 200 }
    }
  },
  "vars": {
    "source": { "id": "acct-1", "balance": 500 }
  }
}
```

If fixtures are missing for `read`/`update` aliases, validation still runs using synthetic placeholders and emits warnings (or fails under `--strict`).

---

### mockd stateful custom run

Execute a registered custom operation with the given input.

```bash
mockd stateful custom run <name> [flags]
```

**Flags:**

| Flag | Description | Default |
|------|-------------|---------|
| `--input` | Inline JSON input for the operation | |
| `--input-file` | Path to JSON file containing operation input | |

**Examples:**

```bash
# Run with inline input
mockd stateful custom run TransferFunds --input '{"sourceId":"acct-1","destId":"acct-2","amount":100}'

# Run with input from file
mockd stateful custom run TransferFunds --input-file transfer-input.json

# Run with no input
mockd stateful custom run TransferFunds
```

---

### mockd stateful custom delete

Delete a registered custom operation.

```bash
mockd stateful custom delete <name> [flags]
```

**Example:**

```bash
mockd stateful custom delete TransferFunds
```

---

### mockd new

Create mocks from templates.

```bash
mockd new [flags]
```

**Flags:**

| Flag | Short | Description | Default |
|------|-------|-------------|---------|
| `--template` | `-t` | Template: blank, crud, auth, pagination, errors | `blank` |
| `--name` | `-n` | Collection name | |
| `--output` | `-o` | Output file | stdout |
| `--resource` | | Resource name (for crud/pagination templates) | |

**Templates:**

| Template | Description |
|----------|-------------|
| `blank` | Empty mock collection |
| `crud` | REST CRUD endpoints (GET list, GET one, POST, PUT, DELETE) |
| `auth` | Authentication flow (login, logout, refresh, me) |
| `pagination` | List endpoints with cursor/offset pagination |
| `errors` | Common HTTP error responses (400, 401, 403, 404, 500) |

**Examples:**

```bash
# Create a blank collection
mockd new -t blank -o mocks.yaml

# Create CRUD endpoints for users
mockd new -t crud --resource users -o users.yaml

# Create auth endpoints
mockd new -t auth -n "Auth API" -o auth.yaml
```

---

### mockd list

List all configured mocks.

```bash
mockd list [flags]
```

**Flags:**

| Flag | Short | Description | Default |
|------|-------|-------------|---------|
| `--config` | `-c` | List mocks from config file (no server needed) | |
| `--type` | `-t` | Filter by type: http, websocket, graphql, grpc, mqtt, soap, oauth | |
| `--no-truncate` | `-w` | Show full IDs and paths without truncation | `false` |
| `--admin-url` | | Admin API base URL | `http://localhost:4290` |
| `--json` | | Output in JSON format | |

**Examples:**

```bash
# List all mocks from running server
mockd list

# List mocks from config file (no server needed)
mockd list --config mockd.yaml

# List only WebSocket mocks
mockd list --type websocket

# Show full IDs and paths (useful for copy-pasting IDs into delete commands)
mockd list --no-truncate

# List as JSON
mockd list --json

# List from remote server
mockd list --admin-url http://remote:4290
```

---

### mockd get

Get details of a specific mock.

```bash
mockd get <mock-id> [flags]
```

**Arguments:**

| Argument | Description |
|----------|-------------|
| `mock-id` | ID of the mock to retrieve (required) |

**Flags:**

| Flag | Description | Default |
|------|-------|-------------|---------|
| `--admin-url` | Admin API base URL | `http://localhost:4290` |
| `--json` | Output in JSON format | |

**Examples:**

```bash
# Get mock details
mockd get abc123

# Get as JSON
mockd get abc123 --json
```

---

### mockd delete

Delete mocks by ID, ID prefix, or path. Also available as `mockd remove` and `mockd rm`.

```bash
mockd delete [<mock-id>] [flags]
```

**Arguments:**

| Argument | Description |
|----------|-------------|
| `mock-id` | Full or prefix of the mock ID. Supports prefix matching — if the prefix uniquely identifies one mock, it is deleted. If the prefix matches multiple mocks, all matches are shown and you are asked to be more specific. |

**Flags:**

| Flag | Short | Description | Default |
|------|-------|-------------|---------|
| `--path` | | Delete mocks matching a URL path | |
| `--method` | | Filter by HTTP method (used with `--path`) | |
| `--yes` | `-y` | Skip confirmation when multiple mocks match | `false` |
| `--admin-url` | | Admin API base URL | `http://localhost:4290` |

**Examples:**

```bash
# Delete by exact ID
mockd delete http_abc123def456

# Delete by ID prefix (must uniquely match one mock)
mockd delete http_abc
# Output: Deleted mock: http_abc123def456

# If prefix is ambiguous, shows matches and asks for more specificity
mockd delete http_a
# Output:
#   Multiple mocks match prefix "http_a":
#     http_abc123def456  GET  /api/users
#     http_aef789012345  POST /api/users
#   Please provide a more specific prefix.

# Delete all mocks on a path
mockd delete --path /api/users

# Delete only GET mocks on a path
mockd delete --path /api/users --method GET

# Skip confirmation when deleting multiple mocks
mockd delete --path /api/users -y

# Using aliases
mockd remove http_abc123
mockd rm http_abc123
```

---

### mockd remove / mockd rm

Hidden aliases for `mockd delete`. All flags and arguments are identical.

```bash
mockd remove [<mock-id>] [flags]
mockd rm [<mock-id>] [flags]
```

See [mockd delete](#mockd-delete) for full documentation.

---

## Import/Export Commands

### mockd import

Import mocks from various sources and formats. Imported configurations may include `statefulResources` definitions, which are persisted to the admin file store and survive restarts. Runtime data for stateful resources is in-memory only and resets to seed data on restart.

```bash
mockd import <source> [flags]
```

**Arguments:**

| Argument | Description |
|----------|-------------|
| `source` | Path to file, or cURL command (in quotes) |

**Flags:**

| Flag | Short | Description | Default |
|------|-------|-------------|---------|
| `--format` | `-f` | Force format (auto-detected if omitted) | |
| `--replace` | | Replace all existing mocks | merge |
| `--dry-run` | | Preview import without saving | |
| `--include-static` | | Include static assets (for HAR imports) | |
| `--admin-url` | | Admin API base URL | `http://localhost:4290` |

**Supported Formats:**

| Format | Description |
|--------|-------------|
| `mockd` | Mockd native format (YAML/JSON) |
| `openapi` | OpenAPI 3.x or Swagger 2.0 |
| `postman` | Postman Collection v2.x |
| `har` | HTTP Archive (browser recordings) |
| `wiremock` | WireMock JSON mappings |
| `curl` | cURL command |
| `wsdl` | WSDL 1.1 service definition (generates SOAP mocks) |

**Examples:**

```bash
# Import from OpenAPI spec (auto-detected)
mockd import openapi.yaml

# Import from Postman collection
mockd import collection.json -f postman

# Import from HAR file including static assets
mockd import recording.har --include-static

# Import from cURL command
mockd import "curl -X POST https://api.example.com/users -H 'Content-Type: application/json' -d '{\"name\": \"test\"}'"

# Preview import without saving
mockd import openapi.yaml --dry-run

# Replace all mocks with imported ones
mockd import mocks.yaml --replace
```

---

### mockd export

Export current mocks to file.

```bash
mockd export [flags]
```

**Flags:**

| Flag | Short | Description | Default |
|------|-------|-------------|---------|
| `--output` | `-o` | Output file | stdout |
| `--name` | `-n` | Collection name | `exported-config` |
| `--format` | `-f` | Output format: mockd, openapi | `mockd` |
| `--version` | | Version tag for the export | |
| `--admin-url` | | Admin API base URL | `http://localhost:4290` |

**Formats:**

| Format | Description |
|--------|-------------|
| `mockd` | Mockd native format (YAML/JSON) - recommended for portability |
| `openapi` | OpenAPI 3.x specification - for API documentation |

**Examples:**

```bash
# Export to stdout as YAML
mockd export

# Export to JSON file
mockd export -o mocks.json

# Export to YAML file
mockd export -o mocks.yaml

# Export as OpenAPI specification
mockd export -f openapi -o api.yaml

# Export with custom name
mockd export -n "My API Mocks" -o mocks.yaml
```

---

### mockd convert

Convert recorded API traffic directly into `mockd` mock configurations.
Reads recordings from disk (written by `mockd proxy start`) and outputs mock configurations that can be piped or imported with `mockd import`.

```bash
mockd convert [flags]
```

**Flags:**

| Flag | Short | Description | Default |
|------|-------|-------------|---------|
| `--session` | `-s` | Session name or directory | `latest` |
| `--file` | `-f` | Path to a specific recording file or directory | |
| `--recordings-dir` | | Base recordings directory override | |
| `--include-hosts` | | Comma-separated host patterns to include (e.g. `api.*.com`) | |
| `--path-filter` | | Glob pattern to filter paths (e.g., `/api/*`) | |
| `--method` | | Comma-separated HTTP methods (e.g., `GET,POST`) | |
| `--status` | | Status code filter (e.g., `2xx`, `200,201`) | |
| `--smart-match` | | Convert dynamic path segments like `/users/123` to `/users/{id}` | `false` |
| `--duplicates` | | Duplicate handling strategy: `first`, `last`, `all` | `first` |
| `--include-headers` | | Include request headers in mock matchers | `false` |
| `--check-sensitive` | | Check for sensitive data in recordings and show warnings | `true` |
| `--output` | `-o` | Output file path (default is stdout) | |

**Examples:**

```bash
# Convert the latest proxy recording session into mocks
mockd convert

# Convert a named session with smart path parameter matching
mockd convert --session stripe-api --smart-match

# Convert only specific host traffic, targeting only GET/POST methods
mockd convert --include-hosts "api.stripe.com" --method GET,POST

# Convert a specific recording JSON file
mockd convert --file ./my-recordings/rec_abc123.json

# Pipe converted JSON directly into the server using 'mockd import'
mockd convert --session my-api --smart-match | mockd import "curl -X POST -d @-"
```

---

## Logging Commands

### mockd logs

View request logs.

```bash
mockd logs [flags]
```

**Flags:**

| Flag | Short | Description | Default |
|------|-------|-------------|---------|
| `--protocol` | | Filter by protocol (http, grpc, mqtt, soap, graphql, websocket, sse) | |
| `--method` | `-m` | Filter by HTTP method | |
| `--path` | `-p` | Filter by path (substring match) | |
| `--matched` | | Show only matched requests | |
| `--unmatched` | | Show only unmatched requests | |
| `--limit` | `-n` | Number of entries to show | `20` |
| `--verbose` | | Show headers and body | |
| `--clear` | | Clear all logs | |
| `--follow` | `-f` | Stream logs in real-time (like tail -f) | |
| `--admin-url` | | Admin API base URL | `http://localhost:4290` |
| `--json` | | Output in JSON format | |

**Examples:**

```bash
# Show recent logs
mockd logs

# Show last 50 entries
mockd logs -n 50

# Filter by method
mockd logs -m POST

# Filter by protocol
mockd logs --protocol grpc

# Show verbose output
mockd logs --verbose

# Stream logs in real-time
mockd logs --follow

# Clear logs
mockd logs --clear
```

---

## Configuration Commands

### mockd config

Show effective configuration.

```bash
mockd config [flags]
```

**Flags:**

| Flag | Description |
|------|-------------|
| `--json` | Output in JSON format |

**Examples:**

```bash
mockd config
mockd config --json
```

---

### mockd doctor

Diagnose common setup issues and validate configuration.

```bash
mockd doctor [flags]
```

**Flags:**

| Flag | Short | Description | Default |
|------|-------|-------------|---------|
| `--config` | | Path to config file to validate | |
| `--port` | `-p` | Mock server port to check | `4280` |
| `--admin-port` | `-a` | Admin API port to check | `4290` |

**Examples:**

```bash
# Run all checks with defaults
mockd doctor

# Validate a specific config file
mockd doctor --config mocks.yaml

# Check custom ports
mockd doctor -p 3000 -a 3001
```

---

## Context Commands

### mockd context

Manage contexts (admin server + workspace pairs). Similar to kubectl contexts, allows quick switching between different mockd deployments (local, staging, CI, cloud, etc.).

```bash
mockd context [command]
```

**Subcommands:**

| Command | Description |
|---------|-------------|
| (no command) | Show current context |
| `show` | Show current context (same as no command) |
| `use <name>` | Switch to a different context |
| `add <name>` | Add a new context |
| `list` | List all contexts |
| `remove <name>` | Remove a context |

**Configuration:**

Contexts are stored in `~/.config/mockd/contexts.yaml`. A default "local" context pointing to `http://localhost:4290` is created automatically.

---

### mockd context show

Show the current context.

```bash
mockd context
mockd context show
```

**Output includes:**
- Current context name
- Admin URL
- Workspace (if set)
- Description (if set)
- Environment variable overrides (if active)

---

### mockd context use

Switch to a different context.

```bash
mockd context use <name>
```

**Examples:**

```bash
# Switch to staging context
mockd context use staging

# Switch back to local
mockd context use local
```

---

### mockd context add

Add a new context.

```bash
mockd context add <name> [flags]
```

**Flags:**

| Flag | Short | Description |
|------|-------|-------------|
| `--admin-url` | `-u` | Admin API URL (e.g., http://localhost:4290) |
| `--workspace` | `-w` | Default workspace for this context |
| `--description` | `-d` | Description for this context |
| `--token` | `-t` | Auth token for cloud/enterprise deployments |
| `--tls-insecure` | | Skip TLS certificate verification (for self-signed certs) |
| `--use` | | Switch to this context after adding |
| `--json` | | Output in JSON format |

**Examples:**

```bash
# Add with flags
mockd context add -u https://staging.example.com:4290 staging

# Add interactively (will prompt for URL)
mockd context add production

# Add and switch to it
mockd context add -u http://dev-server:4290 --use dev

# Add with auth token for cloud deployment
mockd context add -u https://api.mockd.io -t YOUR_TOKEN --use cloud

# Add with workspace preset
mockd context add -u https://staging:4290 -w my-workspace staging
```

**Security Notes:**
- URLs with embedded credentials (`http://user:pass@host`) are rejected for security
- Auth tokens are stored in the config file but masked in JSON output
- Config file is created with 0600 permissions (owner read/write only)

---

### mockd context list

List all contexts.

```bash
mockd context list [flags]
```

**Flags:**

| Flag | Description |
|------|-------------|
| `--json` | Output in JSON format |

**Examples:**

```bash
# List all contexts
mockd context list

# Output:
# CURRENT  NAME     ADMIN URL                      WORKSPACE  DESCRIPTION
# *        local    http://localhost:4290          -          Local mockd server
#          staging  https://staging.example.com    ws-123     Staging server
#          cloud    https://api.mockd.io           -          Cloud deployment

# JSON output (tokens are masked)
mockd context list --json
```

---

### mockd context remove

Remove a context.

```bash
mockd context remove <name> [flags]
```

**Flags:**

| Flag | Short | Description |
|------|-------|-------------|
| `--force` | `-f` | Force removal without confirmation |

**Examples:**

```bash
# Remove with confirmation prompt
mockd context remove old-server

# Remove without confirmation
mockd context remove old-server --force
```

**Note:** The current context cannot be removed. Switch to another context first.

---

## Workspace Commands

### mockd workspace

Manage workspaces for organizing mocks. Workspaces allow logical grouping and isolation of mocks within a mockd server.

```bash
mockd workspace [command]
```

**Subcommands:**

| Command | Description |
|---------|-------------|
| (no command) | Show current workspace |
| `list` | List all workspaces |
| `use <id>` | Switch to a workspace |
| `create` | Create a new workspace |
| `delete <id>` | Delete a workspace |
| `clear` | Clear workspace selection (use all mocks) |

---

### mockd workspace show

Show the current workspace.

```bash
mockd workspace
mockd workspace show
```

---

### mockd workspace list

List all workspaces from the server.

```bash
mockd workspace list [flags]
```

**Flags:**

| Flag | Description |
|------|-------------|
| `--json` | Output in JSON format |

---

### mockd workspace use

Switch to a workspace.

```bash
mockd workspace use <id>
```

**Examples:**

```bash
mockd workspace use ws-123
mockd workspace use "my-project"
```

---

### mockd workspace create

Create a new workspace.

```bash
mockd workspace create [flags]
```

**Flags:**

| Flag | Short | Description |
|------|-------|-------------|
| `--name` | `-n` | Workspace name |
| `--description` | `-d` | Workspace description |
| `--use` | | Switch to this workspace after creating |

**Examples:**

```bash
# Create and switch to new workspace
mockd workspace create -n "my-project" --use

# Create with description
mockd workspace create -n "api-tests" -d "Mocks for API integration tests"
```

---

### mockd workspace delete

Delete a workspace.

```bash
mockd workspace delete <id> [flags]
```

**Flags:**

| Flag | Short | Description |
|------|-------|-------------|
| `--force` | `-f` | Force deletion without confirmation |

---

### mockd workspace clear

Clear the workspace selection to use all mocks.

```bash
mockd workspace clear
```

---

## Proxy Commands

### mockd proxy

Manage the MITM proxy for recording API traffic.

```bash
mockd proxy <subcommand> [flags]
```

**Subcommands:**

- `start` - Start the proxy server
- `stop` - Stop the proxy server
- `status` - Show proxy server status
- `mode` - Get or set proxy mode
- `ca` - Manage CA certificate

---

#### mockd proxy start

Start the MITM proxy server.

```bash
mockd proxy start [flags]
```

**Flags:**

| Flag | Short | Description | Default |
|------|-------|-------------|---------|
| `--port` | `-p` | Proxy server port | `8888` |
| `--mode` | `-m` | Proxy mode: record or passthrough | `record` |
| `--session` | `-s` | Recording session name | |
| `--ca-path` | | Path to CA certificate directory | |
| `--include` | | Comma-separated path patterns to include | |
| `--exclude` | | Comma-separated path patterns to exclude | |
| `--include-hosts` | | Comma-separated host patterns to include | |
| `--exclude-hosts` | | Comma-separated host patterns to exclude | |

**Examples:**

```bash
# Start proxy in record mode
mockd proxy start

# Start with custom port and session
mockd proxy start --port 9000 --session my-session

# Start with filters
mockd proxy start --include "/api/*" --exclude "/api/health"
```

---

:::note
The proxy runs in the foreground and is stopped with **Ctrl+C**. There are no separate `proxy stop`, `proxy status`, or `proxy mode` CLI commands. These operations are available via the [Admin API](/reference/admin-api/) when the proxy is started through the admin server.
:::

---

#### mockd proxy ca

Manage CA certificate for HTTPS interception.

```bash
mockd proxy ca <subcommand> [flags]
```

**Subcommands:**

- `export` - Export CA certificate for trust installation
- `generate` - Generate a new CA certificate

---

##### mockd proxy ca export

```bash
mockd proxy ca export [flags]
```

**Flags:**

| Flag | Short | Description | Default |
|------|-------|-------------|---------|
| `--output` | `-o` | Output file path | stdout |
| `--ca-path` | | Path to CA certificate directory | |

**Examples:**

```bash
# Export to stdout
mockd proxy ca export

# Export to file
mockd proxy ca export -o ca.crt
```

---

##### mockd proxy ca generate

```bash
mockd proxy ca generate [flags]
```

**Flags:**

| Flag | Description |
|------|-------------|
| `--ca-path` | Path to CA certificate directory (required) |

**Examples:**

```bash
mockd proxy ca generate --ca-path ~/.mockd/ca
```

---

## Recordings Commands

### mockd recordings

Manage recorded API traffic.

```bash
mockd recordings <subcommand> [flags]
```

**Subcommands:**

- `sessions` - List all recording sessions
- `list` - List requests from a session
- `export` - Export recordings to JSON
- `import` - Import recordings from JSON
- `clear` - Clear all recordings

---

#### mockd recordings sessions

List all recording sessions.

```bash
mockd recordings sessions [flags]
```

**Flags:**

| Flag | Description |
|------|-------------|
| `--recordings-dir` | Base recordings directory override |

---

#### mockd recordings list

List all recorded API requests in a session.

```bash
mockd recordings list [flags]
```

**Flags:**

| Flag | Short | Description | Default |
|------|-------|-------------|---------|
| `--session` | `-s` | Session name or directory | `latest` |
| `--recordings-dir` | | Base recordings directory override | |
| `--method` | | Filter by HTTP method | |
| `--host` | | Filter by request host | |
| `--limit` | | Maximum number of recordings to show | `0` (all) |

Also supports the global `--json` flag to output as JSON.

**Examples:**

```bash
# List all recordings from latest session
mockd recordings list

# List only GET requests
mockd recordings list --method GET

# List as JSON
mockd recordings list --json
```

---

#### mockd recordings export

Export recordings to JSON format.

```bash
mockd recordings export [flags]
```

**Flags:**

| Flag | Short | Description | Default |
|------|-------|-------------|---------|
| `--session` | | Export specific session | |
| `--output` | `-o` | Output file path | stdout |

**Examples:**

```bash
# Export all recordings to stdout
mockd recordings export

# Export specific session to file
mockd recordings export --session my-session -o recordings.json
```

---

#### mockd recordings import

Import recordings from JSON format.

```bash
mockd recordings import [flags]
```

**Flags:**

| Flag | Short | Description |
|------|-------|-------------|
| `--input` | `-i` | Input file path (required) |

**Examples:**

```bash
mockd recordings import -i recordings.json
```

---

#### mockd recordings clear

Clear all recordings.

```bash
mockd recordings clear [flags]
```

**Flags:**

| Flag | Short | Description |
|------|-------|-------------|
| `--force` | `-f` | Skip confirmation |

**Examples:**

```bash
mockd recordings clear
mockd recordings clear --force
```

---

### mockd convert

Convert recorded API traffic to mock definitions (standalone command).

```bash
mockd convert [flags]
```

**Flags:**

| Flag | Short | Description | Default |
|------|-------|-------------|---------|
| `--recording` | | Convert a single recording by ID | |
| `--session` | | Convert all recordings from a session (use "latest" for most recent) | |
| `--path-filter` | | Glob pattern to filter paths (e.g., /api/*) | |
| `--method` | | Comma-separated HTTP methods (e.g., GET,POST) | |
| `--status` | | Status code filter (e.g., 2xx, 200,201) | |
| `--smart-match` | | Convert dynamic path segments like /users/123 to /users/{id} | |
| `--duplicates` | | Duplicate handling strategy: first, last, all | `first` |
| `--include-headers` | | Include request headers in mock matchers | |
| `--check-sensitive` | | Check for sensitive data and show warnings | `true` |
| `--output` | `-o` | Output file path | stdout |

**Examples:**

```bash
# Convert a single recording
mockd convert --recording abc123

# Convert latest session with smart matching
mockd convert --session latest --smart-match

# Convert session filtering only GET requests to /api/*
mockd convert --session my-session --path-filter '/api/*' --method GET

# Convert and save to file
mockd convert --session latest -o mocks.json

# Convert only successful responses
mockd convert --session latest --status 2xx
```

---

## Stream Recordings Commands

### mockd stream-recordings

Manage WebSocket and SSE stream recordings.

```bash
mockd stream-recordings <subcommand> [flags]
```

**Subcommands:**

- `list, ls` - List all stream recordings
- `show, get` - Show details of a specific recording
- `delete, rm` - Delete a recording
- `export` - Export a recording to JSON
- `convert` - Convert a recording to mock config
- `stats` - Show storage statistics
- `vacuum` - Remove soft-deleted recordings
- `sessions` - List active recording sessions

---

#### mockd stream-recordings list

List all WebSocket and SSE stream recordings.

```bash
mockd stream-recordings list [flags]
```

**Flags:**

| Flag | Description | Default |
|------|-------------|---------|
| `--protocol` | Filter by protocol (websocket, sse) | |
| `--path` | Filter by path prefix | |
| `--status` | Filter by status (complete, incomplete, recording) | |
| `--json` | Output as JSON | |
| `--limit` | Maximum number of recordings to show | `20` |
| `--offset` | Offset for pagination | |
| `--sort` | Sort by field: startTime, name, size | `startTime` |
| `--order` | Sort order: asc, desc | `desc` |
| `--include-deleted` | Include soft-deleted recordings | |

**Examples:**

```bash
# List all recordings
mockd stream-recordings list

# List only WebSocket recordings
mockd stream-recordings list --protocol websocket

# List as JSON
mockd stream-recordings list --json

# Paginate results
mockd stream-recordings list --limit 10 --offset 20
```

---

#### mockd stream-recordings show

Show details of a specific stream recording.

```bash
mockd stream-recordings show <id> [flags]
```

**Flags:**

| Flag | Description |
|------|-------------|
| `--json` | Output as JSON |

**Examples:**

```bash
mockd stream-recordings show 01ABCDEF123456
mockd stream-recordings show 01ABCDEF123456 --json
```

---

#### mockd stream-recordings delete

Delete a stream recording.

```bash
mockd stream-recordings delete <id> [flags]
```

**Flags:**

| Flag | Short | Description |
|------|-------|-------------|
| `--force` | `-f` | Skip confirmation |
| `--permanent` | | Permanently delete (not soft-delete) |

**Examples:**

```bash
mockd stream-recordings delete 01ABCDEF123456
mockd stream-recordings delete 01ABCDEF123456 --force
mockd stream-recordings delete 01ABCDEF123456 --permanent
```

---

#### mockd stream-recordings export

Export a stream recording to JSON format.

```bash
mockd stream-recordings export <id> [flags]
```

**Flags:**

| Flag | Short | Description | Default |
|------|-------|-------------|---------|
| `--output` | `-o` | Output file path | stdout |

**Examples:**

```bash
mockd stream-recordings export 01ABCDEF123456
mockd stream-recordings export 01ABCDEF123456 -o recording.json
```

---

#### mockd stream-recordings convert

Convert a stream recording to a mock configuration.

```bash
mockd stream-recordings convert <id> [flags]
```

**Flags:**

| Flag | Short | Description | Default |
|------|-------|-------------|---------|
| `--output` | `-o` | Output file path | stdout |
| `--simplify-timing` | | Normalize timing to reduce noise | |
| `--min-delay` | | Minimum delay to preserve in ms | `10` |
| `--max-delay` | | Maximum delay in ms | `5000` |
| `--include-client` | | Include client messages as expect steps | `true` |
| `--deduplicate` | | Remove consecutive duplicate messages | |

**Examples:**

```bash
mockd stream-recordings convert 01ABCDEF123456
mockd stream-recordings convert 01ABCDEF123456 --simplify-timing
mockd stream-recordings convert 01ABCDEF123456 -o scenario.json
```

---

#### mockd stream-recordings stats

Show storage statistics for stream recordings.

```bash
mockd stream-recordings stats [flags]
```

**Flags:**

| Flag | Description |
|------|-------------|
| `--json` | Output as JSON |

---

#### mockd stream-recordings vacuum

Permanently remove soft-deleted recordings.

```bash
mockd stream-recordings vacuum [flags]
```

**Flags:**

| Flag | Short | Description |
|------|-------|-------------|
| `--force` | `-f` | Skip confirmation |

**Examples:**

```bash
mockd stream-recordings vacuum
mockd stream-recordings vacuum --force
```

---

#### mockd stream-recordings sessions

List active recording sessions.

```bash
mockd stream-recordings sessions [flags]
```

**Flags:**

| Flag | Description |
|------|-------------|
| `--json` | Output as JSON |

---

## AI Commands

### mockd generate

Generate mock configurations using AI.

```bash
mockd generate [flags]
```

**Flags:**

| Flag | Short | Description | Default |
|------|-------|-------------|---------|
| `--input` | `-i` | Input OpenAPI spec file | |
| `--prompt` | `-p` | Natural language description for generation | |
| `--output` | `-o` | Output file | stdout |
| `--ai` | | Enable AI-powered data generation | |
| `--provider` | | AI provider (openai, anthropic, ollama) | |
| `--model` | | AI model to use | |
| `--dry-run` | | Preview generation without saving | |
| `--admin-url` | | Admin API base URL | `http://localhost:4290` |

**Environment Variables:**

| Variable | Description |
|----------|-------------|
| `MOCKD_AI_PROVIDER` | Default AI provider |
| `MOCKD_AI_API_KEY` | API key for the provider |
| `MOCKD_AI_MODEL` | Default model |
| `MOCKD_AI_ENDPOINT` | Custom endpoint (for Ollama) |

**Examples:**

```bash
# Generate mocks from OpenAPI spec with AI enhancement
mockd generate --ai --input openapi.yaml -o mocks.yaml

# Generate mocks from natural language description
mockd generate --ai --prompt "user management API with CRUD operations"

# Generate mocks using specific provider
mockd generate --ai --provider openai --prompt "payment processing API"

# Preview what would be generated
mockd generate --ai --prompt "blog API" --dry-run
```

---

### mockd enhance

Enhance existing mocks with AI-generated response data.

```bash
mockd enhance [flags]
```

**Flags:**

| Flag | Description |
|------|-------------|
| `--ai` | Enable AI-powered enhancement (required) |
| `--provider` | AI provider (openai, anthropic, ollama) |
| `--model` | AI model to use |
| `--admin-url` | Admin API base URL (default: http://localhost:4290) |

**Environment Variables:**

| Variable | Description |
|----------|-------------|
| `MOCKD_AI_PROVIDER` | Default AI provider |
| `MOCKD_AI_API_KEY` | API key for the provider |
| `MOCKD_AI_MODEL` | Default model |
| `MOCKD_AI_ENDPOINT` | Custom endpoint (for Ollama) |

**Examples:**

```bash
# Enhance all mocks with AI-generated data
mockd enhance --ai

# Use specific provider
mockd enhance --ai --provider anthropic
```

---

## GraphQL Commands

### mockd graphql

Manage and test GraphQL endpoints.

```bash
mockd graphql <subcommand> [flags]
```

**Subcommands:**

- `validate` - Validate a GraphQL schema file
- `query` - Execute a query against a GraphQL endpoint

---

#### mockd graphql validate

Validate a GraphQL schema file.

```bash
mockd graphql validate <schema-file>
```

**Arguments:**

| Argument | Description |
|----------|-------------|
| `schema-file` | Path to the GraphQL schema file (.graphql or .gql) |

**Examples:**

```bash
# Validate a schema file
mockd graphql validate schema.graphql

# Validate with full path
mockd graphql validate ./schemas/api.graphql
```

---

#### mockd graphql query

Execute a GraphQL query against an endpoint.

```bash
mockd graphql query <endpoint> <query> [flags]
```

**Arguments:**

| Argument | Description |
|----------|-------------|
| `endpoint` | GraphQL endpoint URL (e.g., http://localhost:4280/graphql) |
| `query` | GraphQL query string or @filename |

**Flags:**

| Flag | Short | Description | Default |
|------|-------|-------------|---------|
| `--variables` | `-v` | JSON string of variables | |
| `--operation` | `-o` | Operation name for multi-operation documents | |
| `--header` | `-H` | Additional headers (key:value,key2:value2) | |
| `--pretty` | | Pretty print output | `true` |

**Examples:**

```bash
# Simple query
mockd graphql query http://localhost:4280/graphql "{ users { id name } }"

# Query with variables
mockd graphql query http://localhost:4280/graphql \
  "query GetUser($id: ID!) { user(id: $id) { name } }" \
  -v '{"id": "123"}'

# Query from file
mockd graphql query http://localhost:4280/graphql @query.graphql

# With custom headers
mockd graphql query http://localhost:4280/graphql "{ me { name } }" \
  -H "Authorization:Bearer token123"
```

---

## Chaos Engineering Commands

### mockd chaos

Manage chaos injection for fault testing.

```bash
mockd chaos <subcommand> [flags]
```

**Subcommands:**

- `enable` - Enable chaos injection
- `disable` - Disable chaos injection
- `status` - Show current chaos configuration
- `profiles` - List available chaos profiles
- `apply` - Apply a named chaos profile

---

#### mockd chaos enable

Enable chaos injection on the running mock server.

```bash
mockd chaos enable [flags]
```

**Flags:**

| Flag | Short | Description | Default |
|------|-------|-------------|---------|
| `--admin-url` | | Admin API base URL | `http://localhost:4290` |
| `--latency` | `-l` | Add random latency (e.g., "10ms-100ms") | |
| `--error-rate` | `-e` | Error rate (0.0-1.0) | |
| `--error-code` | | HTTP error code to return | `500` |
| `--path` | `-p` | Path pattern to apply chaos to (regex) | |
| `--probability` | | Probability of applying chaos | `1.0` |

**Examples:**

```bash
# Enable random latency
mockd chaos enable --latency "50ms-200ms"

# Enable error injection with 10% rate
mockd chaos enable --error-rate 0.1 --error-code 503

# Apply chaos only to specific paths
mockd chaos enable --latency "100ms-500ms" --path "/api/.*"

# Combine latency and errors
mockd chaos enable --latency "10ms-50ms" --error-rate 0.05
```

---

#### mockd chaos disable

Disable chaos injection on the running mock server.

```bash
mockd chaos disable [flags]
```

**Flags:**

| Flag | Description | Default |
|------|-------------|---------|
| `--admin-url` | Admin API base URL | `http://localhost:4290` |

**Examples:**

```bash
mockd chaos disable
```

---

#### mockd chaos status

Show current chaos configuration.

```bash
mockd chaos status [flags]
```

**Flags:**

| Flag | Description | Default |
|------|-------------|---------|
| `--admin-url` | Admin API base URL | `http://localhost:4290` |
| `--json` | Output in JSON format | |

**Examples:**

```bash
mockd chaos status
mockd chaos status --json
```

---

#### mockd chaos profiles

List all available chaos profiles. Profiles are pre-built chaos configurations for common failure scenarios.

```bash
mockd chaos profiles [flags]
```

**Flags:**

| Flag | Description | Default |
|------|-------------|---------|
| `--admin-url` | Admin API base URL | `http://localhost:4290` |
| `--json` | Output in JSON format (includes full config for each profile) | |

**Examples:**

```bash
# List all profiles
mockd chaos profiles

# List with full configuration details
mockd chaos profiles --json
```

**Output:**

```
Available chaos profiles:

  degraded        Partially degraded service
  dns-flaky       Intermittent DNS resolution failures
  flaky           Unreliable service with random errors
  mobile-3g       Mobile 3G network conditions
  offline         Service completely down
  overloaded      Overloaded server under heavy load
  rate-limited    Rate-limited API
  satellite       Satellite internet simulation
  slow-api        Simulates slow upstream API
  timeout         Connection timeout simulation

Apply a profile with: mockd chaos apply <profile-name>
```

---

#### mockd chaos apply

Apply a named chaos profile. This replaces the current chaos configuration with the profile's settings.

```bash
mockd chaos apply <profile-name> [flags]
```

**Arguments:**

| Argument | Description |
|----------|-------------|
| `profile-name` | Name of the chaos profile to apply (see `mockd chaos profiles`) |

**Flags:**

| Flag | Description | Default |
|------|-------------|---------|
| `--admin-url` | Admin API base URL | `http://localhost:4290` |

**Examples:**

```bash
# Simulate a flaky service (20% errors, 0-100ms latency)
mockd chaos apply flaky

# Simulate mobile network conditions (300-800ms, 50KB/s, 2% errors)
mockd chaos apply mobile-3g

# Take the service completely offline (100% 503 errors)
mockd chaos apply offline

# Disable when done
mockd chaos disable
```

---

## Verification Commands

### mockd verify

Verify mock call counts and inspect invocations. Useful for integration testing where you need to prove your code makes the correct API calls.

```bash
mockd verify <subcommand> [flags]
```

**Subcommands:**

| Command | Description |
|---------|-------------|
| `status` | Show call count and last-called time for a mock |
| `check` | Assert that a mock was called the expected number of times |
| `invocations` | List recorded request details for a mock |
| `reset` | Clear verification data (call counts and invocation history) |

---

#### mockd verify status

Show the call count and last-called timestamp for a specific mock.

```bash
mockd verify status <mock-id> [flags]
```

**Flags:**

| Flag | Description | Default |
|------|-------------|---------|
| `--admin-url` | Admin API base URL | `http://localhost:4290` |
| `--json` | Output in JSON format | |

**Examples:**

```bash
# Check how many times a mock was called
mockd verify status http_abc123

# Output:
# Mock: http_abc123
#   Call count: 5
#   Last called: 2026-02-26 19:30:45

# JSON output
mockd verify status http_abc123 --json
```

---

#### mockd verify check

Assert call count expectations for a mock. Returns exit code 0 on pass and exit code 1 on failure — suitable for CI scripts and test automation.

At least one assertion flag is required.

```bash
mockd verify check <mock-id> [flags]
```

**Flags:**

| Flag | Description | Default |
|------|-------------|---------|
| `--exactly` | Assert mock was called exactly N times | |
| `--at-least` | Assert mock was called at least N times | |
| `--at-most` | Assert mock was called at most N times | |
| `--never` | Assert mock was never called | |
| `--admin-url` | Admin API base URL | `http://localhost:4290` |
| `--json` | Output in JSON format | |

**Examples:**

```bash
# Assert exactly 3 calls
mockd verify check http_abc123 --exactly 3
# PASS: called exactly 3 time(s) (called 3 time(s))

# Assert at least 1 call
mockd verify check http_abc123 --at-least 1
# PASS: called at least 1 time(s) (called 5 time(s))

# Assert never called (useful for negative testing)
mockd verify check http_abc123 --never
# FAIL: expected never called but was called 5 time(s)

# Assert a range (combine flags)
mockd verify check http_abc123 --at-least 1 --at-most 10

# Use in CI — non-zero exit code on failure
mockd verify check http_abc123 --exactly 1 || echo "Verification failed!"
```

---

#### mockd verify invocations

List all recorded invocation details (method, path, timestamp, body) for a specific mock.

```bash
mockd verify invocations <mock-id> [flags]
```

**Flags:**

| Flag | Description | Default |
|------|-------------|---------|
| `--admin-url` | Admin API base URL | `http://localhost:4290` |
| `--json` | Output in JSON format | |

**Examples:**

```bash
# List all invocations for a mock
mockd verify invocations http_abc123

# Output:
# Mock: http_abc123 (3 invocation(s))
#
#   [1] 19:30:45.123 GET /api/users at http_abc123
#   [2] 19:31:02.456 POST /api/users at http_abc123
#       Body: {"name":"Alice","email":"alice@example.com"}
#   [3] 19:31:15.789 GET /api/users at http_abc123

# JSON output (full request details)
mockd verify invocations http_abc123 --json
```

---

#### mockd verify reset

Clear verification data (call counts and invocation history) for a specific mock or all mocks. Use between test runs for isolation.

```bash
mockd verify reset [mock-id] [flags]
```

**Flags:**

| Flag | Description | Default |
|------|-------------|---------|
| `--all` | Reset verification data for all mocks | |
| `--admin-url` | Admin API base URL | `http://localhost:4290` |
| `--json` | Output in JSON format | |

**Examples:**

```bash
# Reset a specific mock
mockd verify reset http_abc123
# Verification data cleared for mock: http_abc123

# Reset all mocks (useful between test suites)
mockd verify reset --all
# All verification data cleared
```

---

## See Also

- [Configuration Reference](/reference/configuration) - Config file format
- [Admin API Reference](/reference/admin-api) - Runtime management API
