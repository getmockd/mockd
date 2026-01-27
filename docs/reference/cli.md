# CLI Reference

Complete reference for the mockd command-line interface.

## Global Flags

These flags apply to all commands:

| Flag | Description |
|------|-------------|
| `-h, --help` | Show help message |
| `-v, --version` | Show version information |

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
| `--max-log-entries` | | Maximum request log entries | `1000` |
| `--auto-cert` | | Auto-generate TLS certificate | `true` |

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
| `--watch` | | Watch for file changes (with --load) | |
| `--validate` | | Validate files before serving (with --load) | |
| `--https-port` | | HTTPS server port (0 = disabled) | `0` |
| `--read-timeout` | | Read timeout in seconds | `30` |
| `--write-timeout` | | Write timeout in seconds | `30` |
| `--max-log-entries` | | Maximum request log entries | `1000` |
| `--auto-cert` | | Auto-generate TLS certificate | `true` |

Also supports all TLS, mTLS, Audit, GraphQL, gRPC, OAuth, MQTT, Chaos, and Validation flags from `serve`.

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

# Load with hot reload
mockd start --load ./mocks/ --watch

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

## Mock Management Commands

### mockd add

Add a new mock endpoint.

```bash
mockd add [flags]
```

**Global Flags:**

| Flag | Short | Description | Default |
|------|-------|-------------|---------|
| `--type` | `-t` | Mock type: http, websocket, graphql, grpc, mqtt, soap | `http` |
| `--name` | `-n` | Mock display name | |
| `--admin-url` | | Admin API base URL | `http://localhost:4290` |
| `--json` | | Output in JSON format | |

**HTTP Flags (`--type http`):**

| Flag | Short | Description | Default |
|------|-------|-------------|---------|
| `--method` | `-m` | HTTP method to match | `GET` |
| `--path` | | URL path to match | Required |
| `--status` | `-s` | Response status code | `200` |
| `--body` | `-b` | Response body | |
| `--body-file` | | Read response body from file | |
| `--header` | `-H` | Response header (key:value), repeatable | |
| `--match-header` | | Required request header (key:value), repeatable | |
| `--match-query` | | Required query param (key:value), repeatable | |
| `--priority` | | Mock priority (higher = matched first) | |
| `--delay` | | Response delay in milliseconds | |

**SSE Flags (HTTP with streaming):**

| Flag | Description | Default |
|------|-------------|---------|
| `--sse` | Enable SSE streaming response | |
| `--sse-event` | SSE event (type:data), repeatable | |
| `--sse-delay` | Delay between events in milliseconds | `100` |
| `--sse-template` | Built-in template: openai-chat, notification-stream | |
| `--sse-repeat` | Repeat events N times (0 = infinite) | `1` |
| `--sse-keepalive` | Keepalive interval in milliseconds (0 = disabled) | `0` |

**WebSocket Flags (`--type websocket`):**

| Flag | Description | Default |
|------|-------------|---------|
| `--path` | WebSocket path | Required |
| `--message` | Default response message (JSON) | |
| `--echo` | Enable echo mode | |

**GraphQL Flags (`--type graphql`):**

| Flag | Description | Default |
|------|-------------|---------|
| `--path` | GraphQL endpoint path | `/graphql` |
| `--operation` | Operation name | Required |
| `--op-type` | Operation type: query or mutation | `query` |
| `--response` | JSON response data | |

**gRPC Flags (`--type grpc`):**

| Flag | Description | Default |
|------|-------------|---------|
| `--proto` | Path to .proto file (required, repeatable) | |
| `--proto-path` | Import path for proto dependencies (repeatable) | |
| `--service` | Service name, e.g., myapp.UserService (required) | |
| `--rpc-method` | RPC method name (required) | |
| `--response` | JSON response data | |
| `--grpc-port` | gRPC server port | `50051` |

**MQTT Flags (`--type mqtt`):**

| Flag | Description | Default |
|------|-------------|---------|
| `--topic` | Topic pattern | Required |
| `--payload` | Response payload | |
| `--qos` | QoS level: 0, 1, or 2 | `0` |
| `--mqtt-port` | MQTT broker port (required) | |

**SOAP Flags (`--type soap`):**

| Flag | Description | Default |
|------|-------------|---------|
| `--path` | SOAP endpoint path | `/soap` |
| `--operation` | SOAP operation name | Required |
| `--soap-action` | SOAPAction header value | |
| `--response` | XML response body | |

**Examples:**

```bash
# Simple HTTP GET mock
mockd add --path /api/users --status 200 --body '[]'

# HTTP POST with JSON response
mockd add -m POST --path /api/users -s 201 \
  -b '{"id": "new-id", "created": true}' \
  -H "Content-Type:application/json"

# WebSocket mock with echo mode
mockd add --type websocket --path /ws/chat --echo

# WebSocket mock with default response
mockd add --type websocket --path /ws/events \
  --message '{"type": "connected", "status": "ok"}'

# GraphQL query mock
mockd add --type graphql --operation getUser \
  --response '{"data": {"user": {"id": "1", "name": "Alice"}}}'

# GraphQL mutation mock
mockd add --type graphql --operation createUser --op-type mutation \
  --response '{"data": {"createUser": {"id": "new-id"}}}'

# gRPC mock
mockd add --type grpc --service greeter.Greeter --rpc-method SayHello \
  --response '{"message": "Hello, World!"}'

# MQTT mock
mockd add --type mqtt --topic sensors/temperature --payload '{"temp": 72.5}' --qos 1

# SOAP mock
mockd add --type soap --operation GetWeather --soap-action "http://example.com/GetWeather" \
  --response '<GetWeatherResponse><Temperature>72</Temperature></GetWeatherResponse>'

# SSE streaming mock with custom events
mockd add --path /events --sse \
  --sse-event 'connected:{"status":"ok"}' \
  --sse-event 'update:{"count":1}' \
  --sse-event 'update:{"count":2}' \
  --sse-delay 500

# SSE with OpenAI-compatible streaming template
mockd add -m POST --path /v1/chat/completions --sse --sse-template openai-chat

# SSE with infinite keepalive ping
mockd add --path /stream --sse \
  --sse-event 'ping:{}' --sse-delay 1000 --sse-repeat 0
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
| `--type` | `-t` | Filter by type: http, websocket, graphql, grpc, mqtt, soap | |
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
|------|-------------|---------|
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

Delete a mock by ID.

```bash
mockd delete <mock-id> [flags]
```

**Arguments:**

| Argument | Description |
|----------|-------------|
| `mock-id` | ID of the mock to delete (required) |

**Flags:**

| Flag | Description | Default |
|------|-------------|---------|
| `--admin-url` | Admin API base URL | `http://localhost:4290` |

**Examples:**

```bash
mockd delete abc123
```

---

## Import/Export Commands

### mockd import

Import mocks from various sources and formats.

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
| `--config` | `-c` | Path to config file to validate | |
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

#### mockd proxy stop

Stop the running proxy server.

```bash
mockd proxy stop
```

---

#### mockd proxy status

Show the current proxy server status.

```bash
mockd proxy status
```

---

#### mockd proxy mode

Get or set the proxy operating mode.

```bash
mockd proxy mode [mode]
```

**Arguments:**

| Argument | Description |
|----------|-------------|
| `mode` | New mode: record or passthrough (optional) |

**Examples:**

```bash
# Get current mode
mockd proxy mode

# Set mode to passthrough
mockd proxy mode passthrough
```

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

- `list` - List all recordings
- `convert` - Convert recordings to mock definitions
- `export` - Export recordings to JSON
- `import` - Import recordings from JSON
- `clear` - Clear all recordings

---

#### mockd recordings list

List all recorded API requests.

```bash
mockd recordings list [flags]
```

**Flags:**

| Flag | Description |
|------|-------------|
| `--session` | Filter by session ID |
| `--method` | Filter by HTTP method |
| `--path` | Filter by request path |
| `--json` | Output as JSON |
| `--limit` | Maximum number of recordings to show |

**Examples:**

```bash
# List all recordings
mockd recordings list

# List only GET requests
mockd recordings list --method GET

# List as JSON
mockd recordings list --json
```

---

#### mockd recordings convert

Convert recordings to mock definitions.

```bash
mockd recordings convert [flags]
```

**Flags:**

| Flag | Short | Description | Default |
|------|-------|-------------|---------|
| `--session` | | Filter by session ID | |
| `--deduplicate` | | Remove duplicate request patterns | `true` |
| `--include-headers` | | Include request headers in matchers | |
| `--output` | `-o` | Output file path | stdout |

**Examples:**

```bash
# Convert all recordings to mocks
mockd recordings convert

# Convert specific session with deduplication
mockd recordings convert --session my-session --deduplicate

# Save to file
mockd recordings convert -o mocks.json
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

## gRPC Commands

### mockd grpc

Manage and test gRPC endpoints.

```bash
mockd grpc <subcommand> [flags]
```

**Subcommands:**

- `list` - List services and methods from a proto file
- `call` - Call a gRPC method

---

#### mockd grpc list

List services and methods defined in a proto file.

```bash
mockd grpc list <proto-file> [flags]
```

**Arguments:**

| Argument | Description |
|----------|-------------|
| `proto-file` | Path to the .proto file |

**Flags:**

| Flag | Short | Description |
|------|-------|-------------|
| `--import` | `-I` | Import path for proto includes |

**Examples:**

```bash
# List services from a proto file
mockd grpc list api.proto

# With import path
mockd grpc list api.proto -I ./proto
```

---

#### mockd grpc call

Call a gRPC method on an endpoint.

```bash
mockd grpc call <endpoint> <service/method> <json-body> [flags]
```

**Arguments:**

| Argument | Description |
|----------|-------------|
| `endpoint` | gRPC server address (e.g., localhost:50051) |
| `service/method` | Full service and method name (e.g., package.Service/Method) |
| `json-body` | JSON request body or @filename |

**Flags:**

| Flag | Short | Description | Default |
|------|-------|-------------|---------|
| `--metadata` | `-m` | gRPC metadata as key:value,key2:value2 | |
| `--plaintext` | | Use plaintext (no TLS) | `true` |
| `--pretty` | | Pretty print output | `true` |

**Examples:**

```bash
# Call a method
mockd grpc call localhost:50051 greet.Greeter/SayHello '{"name": "World"}'

# With metadata
mockd grpc call localhost:50051 greet.Greeter/SayHello '{"name": "World"}' \
  -m "authorization:Bearer token123"

# Request from file
mockd grpc call localhost:50051 greet.Greeter/SayHello @request.json
```

---

## MQTT Commands

### mockd mqtt

Publish and subscribe to MQTT messages for testing.

```bash
mockd mqtt <subcommand> [flags]
```

**Subcommands:**

- `publish` - Publish a message to a topic
- `subscribe` - Subscribe to a topic and print messages
- `status` - Show MQTT broker status

---

#### mockd mqtt publish

Publish a message to an MQTT topic.

```bash
mockd mqtt publish [flags] <topic> <message>
```

**Arguments:**

| Argument | Description |
|----------|-------------|
| `topic` | MQTT topic to publish to |
| `message` | Message payload (or @filename for file content) |

**Flags:**

| Flag | Short | Description | Default |
|------|-------|-------------|---------|
| `--broker` | `-b` | MQTT broker address | `localhost:1883` |
| `--username` | `-u` | MQTT username | |
| `--password` | `-P` | MQTT password | |
| `--qos` | | QoS level 0, 1, or 2 | `0` |
| `--retain` | | Retain message on broker | |

**Examples:**

```bash
# Publish a simple message
mockd mqtt publish sensors/temperature "25.5"

# Publish with custom broker
mockd mqtt publish -b mqtt.example.com:1883 sensors/temp "25.5"

# Publish with authentication
mockd mqtt publish -u user -P pass sensors/temp "25.5"

# Publish with QoS 1 and retain
mockd mqtt publish --qos 1 --retain sensors/temp "25.5"

# Publish from file
mockd mqtt publish sensors/config @config.json
```

---

#### mockd mqtt subscribe

Subscribe to an MQTT topic and print received messages.

```bash
mockd mqtt subscribe [flags] <topic>
```

**Arguments:**

| Argument | Description |
|----------|-------------|
| `topic` | MQTT topic to subscribe to (supports wildcards: +, #) |

**Flags:**

| Flag | Short | Description | Default |
|------|-------|-------------|---------|
| `--broker` | `-b` | MQTT broker address | `localhost:1883` |
| `--username` | `-u` | MQTT username | |
| `--password` | `-P` | MQTT password | |
| `--qos` | | QoS level 0, 1, or 2 | `0` |
| `--count` | `-n` | Number of messages to receive (0 = unlimited) | |
| `--timeout` | `-t` | Timeout duration (e.g., 30s, 5m) | |

**Examples:**

```bash
# Subscribe to a topic
mockd mqtt subscribe sensors/temperature

# Subscribe with wildcard
mockd mqtt subscribe "sensors/#"

# Receive only 5 messages
mockd mqtt subscribe -n 5 sensors/temperature

# Subscribe with timeout
mockd mqtt subscribe -t 30s sensors/temperature

# Subscribe with authentication
mockd mqtt subscribe -u user -P pass sensors/temperature
```

---

#### mockd mqtt status

Show MQTT broker status.

```bash
mockd mqtt status [flags]
```

**Flags:**

| Flag | Description | Default |
|------|-------------|---------|
| `--admin-url` | Admin API base URL | `http://localhost:4290` |
| `--json` | Output in JSON format | |

**Examples:**

```bash
mockd mqtt status
mockd mqtt status --json
```

---

## WebSocket Commands

### mockd websocket

Interact with WebSocket endpoints for testing.

```bash
mockd websocket <subcommand> [flags]
```

**Subcommands:**

- `connect` - Interactive WebSocket client (REPL mode)
- `send` - Send a single message and exit
- `listen` - Stream incoming messages
- `status` - Show WebSocket handler status

---

#### mockd websocket connect

Start an interactive WebSocket client session (REPL mode).

```bash
mockd websocket connect [flags] <url>
```

**Arguments:**

| Argument | Description |
|----------|-------------|
| `url` | WebSocket URL (e.g., ws://localhost:4280/ws) |

**Flags:**

| Flag | Short | Description | Default |
|------|-------|-------------|---------|
| `--header` | `-H` | Custom headers (key:value), repeatable | |
| `--subprotocol` | | WebSocket subprotocol | |
| `--timeout` | `-t` | Connection timeout | `30s` |
| `--json` | | Output messages in JSON format | |

**Examples:**

```bash
# Connect to a WebSocket endpoint
mockd websocket connect ws://localhost:4280/ws

# Connect with custom headers
mockd websocket connect -H "Authorization:Bearer token" ws://localhost:4280/ws

# Connect with subprotocol
mockd websocket connect --subprotocol graphql-ws ws://localhost:4280/graphql
```

---

#### mockd websocket send

Send a single message to a WebSocket endpoint and exit.

```bash
mockd websocket send [flags] <url> <message>
```

**Arguments:**

| Argument | Description |
|----------|-------------|
| `url` | WebSocket URL (e.g., ws://localhost:4280/ws) |
| `message` | Message to send (or @filename for file content) |

**Flags:**

| Flag | Short | Description | Default |
|------|-------|-------------|---------|
| `--header` | `-H` | Custom headers (key:value), repeatable | |
| `--subprotocol` | | WebSocket subprotocol | |
| `--timeout` | `-t` | Connection timeout | `30s` |
| `--json` | | Output result in JSON format | |

**Examples:**

```bash
# Send a simple message
mockd websocket send ws://localhost:4280/ws "hello"

# Send JSON message
mockd websocket send ws://localhost:4280/ws '{"action":"ping"}'

# Send with custom headers
mockd websocket send -H "Authorization:Bearer token" ws://localhost:4280/ws "hello"

# Send message from file
mockd websocket send ws://localhost:4280/ws @message.json
```

---

#### mockd websocket listen

Listen for incoming WebSocket messages and print them.

```bash
mockd websocket listen [flags] <url>
```

**Arguments:**

| Argument | Description |
|----------|-------------|
| `url` | WebSocket URL (e.g., ws://localhost:4280/ws) |

**Flags:**

| Flag | Short | Description | Default |
|------|-------|-------------|---------|
| `--header` | `-H` | Custom headers (key:value), repeatable | |
| `--subprotocol` | | WebSocket subprotocol | |
| `--timeout` | `-t` | Connection timeout | `30s` |
| `--count` | `-n` | Number of messages to receive (0 = unlimited) | |
| `--json` | | Output messages in JSON format | |

**Examples:**

```bash
# Listen to all messages
mockd websocket listen ws://localhost:4280/ws

# Listen for 10 messages then exit
mockd websocket listen -n 10 ws://localhost:4280/ws

# Listen with JSON output
mockd websocket listen --json ws://localhost:4280/ws

# Listen with custom headers
mockd websocket listen -H "Authorization:Bearer token" ws://localhost:4280/ws
```

---

#### mockd websocket status

Show WebSocket mock status from the admin API.

```bash
mockd websocket status [flags]
```

**Flags:**

| Flag | Description | Default |
|------|-------------|---------|
| `--admin-url` | Admin API base URL | `http://localhost:4290` |
| `--json` | Output in JSON format | |

**Examples:**

```bash
mockd websocket status
mockd websocket status --json
mockd websocket status --admin-url http://localhost:9091
```

---

## SOAP Commands

### mockd soap

Manage and test SOAP web services.

```bash
mockd soap <subcommand> [flags]
```

**Subcommands:**

- `validate` - Validate a WSDL file
- `call` - Call a SOAP operation

---

#### mockd soap validate

Validate a WSDL file.

```bash
mockd soap validate <wsdl-file>
```

**Arguments:**

| Argument | Description |
|----------|-------------|
| `wsdl-file` | Path to the WSDL file |

**Examples:**

```bash
# Validate a WSDL file
mockd soap validate service.wsdl

# Validate with full path
mockd soap validate ./wsdl/calculator.wsdl
```

---

#### mockd soap call

Call a SOAP operation on an endpoint.

```bash
mockd soap call <endpoint> <operation> [flags]
```

**Arguments:**

| Argument | Description |
|----------|-------------|
| `endpoint` | SOAP service endpoint URL |
| `operation` | Operation name (used in SOAP body if no body provided) |

**Flags:**

| Flag | Short | Description | Default |
|------|-------|-------------|---------|
| `--action` | `-a` | SOAPAction header value | |
| `--body` | `-b` | SOAP body content (XML) or @filename | |
| `--header` | `-H` | Additional headers (key:value,key2:value2) | |
| `--soap12` | | Use SOAP 1.2 | SOAP 1.1 |
| `--pretty` | | Pretty print output | `true` |
| `--timeout` | | Request timeout in seconds | `30` |

**Examples:**

```bash
# Call an operation with auto-generated body
mockd soap call http://localhost:4280/soap GetUser

# Call with SOAPAction header
mockd soap call http://localhost:4280/soap GetUser \
  -a "http://example.com/GetUser"

# Call with custom body
mockd soap call http://localhost:4280/soap GetUser \
  -b '<GetUser xmlns="http://example.com/"><id>123</id></GetUser>'

# Call with body from file
mockd soap call http://localhost:4280/soap GetUser -b @request.xml

# Use SOAP 1.2
mockd soap call http://localhost:4280/soap GetUser --soap12
```

---

## Template Commands

### mockd templates

Manage mock templates from the official template library.

```bash
mockd templates <command> [flags]
```

**Commands:**

- `list` - List available templates
- `add` - Download and import a template

---

#### mockd templates list

List available templates from the template library.

```bash
mockd templates list [flags]
```

**Flags:**

| Flag | Short | Description |
|------|-------|-------------|
| `--category` | `-c` | Filter by category: protocols, services, patterns |
| `--base-url` | | Custom templates repository URL |

**Examples:**

```bash
mockd templates list
mockd templates list -c protocols
```

---

#### mockd templates add

Download and import a template from the template library.

```bash
mockd templates add <template-id> [flags]
```

**Arguments:**

| Argument | Description |
|----------|-------------|
| `template-id` | Template identifier (e.g., services/openai/chat-completions) |

**Flags:**

| Flag | Short | Description | Default |
|------|-------|-------------|---------|
| `--output` | `-o` | Save to file instead of importing to running server | |
| `--dry-run` | | Preview template content without importing | |
| `--admin-url` | | Admin API base URL | `http://localhost:4290` |
| `--base-url` | | Custom templates repository URL | |

**Examples:**

```bash
# Import template to running server
mockd templates add services/openai/chat-completions

# Save template to file
mockd templates add protocols/websocket/chat -o websocket.yaml

# Preview template
mockd templates add services/stripe/webhooks --dry-run
```

---

## Cloud Commands

### mockd tunnel

Expose local mocks via secure cloud tunnel.

```bash
mockd tunnel [flags]
mockd tunnel status
mockd tunnel stop
```

**Supported Protocols:**

The tunnel relays HTTP-based traffic over port 443:

| Protocol | Supported | Notes |
|----------|-----------|-------|
| HTTP/HTTPS | Yes | Full support |
| WebSocket | Yes | Upgrade handled automatically |
| SSE | Yes | Streaming responses work |
| GraphQL | Yes | Runs over HTTP |
| SOAP | Yes | Runs over HTTP |
| gRPC | Planned | Considering gRPC-web proxy approach |
| MQTT | Planned | Considering TCP-over-WebSocket approach |

> **Note**: TCP-based protocols (native gRPC, MQTT) require dedicated ports and are being evaluated for future releases based on community interest. See [Sharing Mocks](../guides/sharing-mocks.md) for alternatives like ngrok TCP tunnels.

**Flags:**

| Flag | Short | Description | Default |
|------|-------|-------------|---------|
| `--port` | `-p` | HTTP server port | `4280` |
| `--admin-port` | | Admin API port | `4290` |
| `--config` | `-c` | Path to mock configuration file | |
| `--relay` | | Relay server URL | `wss://relay.mockd.io/ws` |
| `--token` | | Authentication token (or MOCKD_TOKEN env var) | |
| `--subdomain` | `-s` | Requested subdomain (auto-assigned if empty) | |
| `--domain` | | Custom domain (must be verified in cloud dashboard) | |

**Authentication Flags:**

| Flag | Description |
|------|-------------|
| `--auth-token` | Require this token in X-Auth-Token header |
| `--auth-basic` | Require HTTP Basic Auth (format: user:pass) |
| `--allow-ips` | Allow only these IPs (comma-separated CIDR or IP) |

**Subcommands:**

| Command | Description |
|---------|-------------|
| `status` | Show current tunnel status and metrics |
| `stop` | Stop the running tunnel |

**Environment Variables:**

| Variable | Description |
|----------|-------------|
| `MOCKD_TOKEN` | Authentication token (alternative to --token flag) |

**Examples:**

```bash
# Start tunnel with auto-assigned subdomain
mockd tunnel --token YOUR_TOKEN

# Start tunnel with custom subdomain
mockd tunnel --token YOUR_TOKEN --subdomain my-api

# Start tunnel with config file
mockd tunnel --config mocks.json --token YOUR_TOKEN

# Start tunnel with custom domain (must verify in cloud dashboard first)
mockd tunnel --token YOUR_TOKEN --domain mocks.acme.com

# Protect tunnel with token authentication
mockd tunnel --token YOUR_TOKEN --auth-token secret123

# Protect tunnel with HTTP Basic Auth
mockd tunnel --token YOUR_TOKEN --auth-basic admin:password

# Restrict tunnel access by IP
mockd tunnel --token YOUR_TOKEN --allow-ips "10.0.0.0/8,192.168.1.0/24"

# Check tunnel status
mockd tunnel status
```

---

#### mockd tunnel status

Show the current tunnel connection status.

```bash
mockd tunnel status [flags]
```

**Flags:**

| Flag | Description | Default |
|------|-------------|---------|
| `--admin` | Admin API address | `http://localhost:4290` |

---

#### mockd tunnel stop

Stop the running tunnel connection.

```bash
mockd tunnel stop [flags]
```

**Flags:**

| Flag | Description | Default |
|------|-------------|---------|
| `--admin` | Admin API address | `http://localhost:4290` |

---

## Utility Commands

### mockd completion

Generate shell completion scripts.

```bash
mockd completion <shell>
```

**Arguments:**

| Argument | Description |
|----------|-------------|
| `shell` | Target shell (bash, zsh, fish) |

**Supported Shells:** bash, zsh, fish

**Examples:**

```bash
# Bash (add to ~/.bashrc or /etc/bash_completion.d/)
mockd completion bash > /etc/bash_completion.d/mockd
# Or for user install:
mockd completion bash >> ~/.bashrc

# Zsh (add to fpath)
mockd completion zsh > "${fpath[1]}/_mockd"
# Or for Oh My Zsh:
mockd completion zsh > ~/.oh-my-zsh/completions/_mockd

# Fish
mockd completion fish > ~/.config/fish/completions/mockd.fish
```

---

### mockd version

Display version information.

```bash
mockd version [flags]
```

**Flags:**

| Flag | Description |
|------|-------------|
| `--json` | Output in JSON format |

**Examples:**

```bash
mockd version
mockd version --json
```

---

---

## Environment Variables

| Variable | Description | Equivalent Flag |
|----------|-------------|-----------------|
| `MOCKD_ADMIN_URL` | Default admin URL (overrides context) | `--admin-url` |
| `MOCKD_CONTEXT` | Override current context | `--context` |
| `MOCKD_WORKSPACE` | Override current workspace | `--workspace` |
| `MOCKD_PORT` | Default mock port | `--port` |
| `MOCKD_ADMIN_PORT` | Default admin port | `--admin-port` |
| `MOCKD_TOKEN` | Cloud authentication token | `--token` |
| `MOCKD_RUNTIME_TOKEN` | Runtime token for control plane | `--token` |
| `MOCKD_AI_PROVIDER` | Default AI provider | `--provider` |
| `MOCKD_AI_API_KEY` | API key for AI provider | |
| `MOCKD_AI_MODEL` | Default AI model | `--model` |
| `MOCKD_AI_ENDPOINT` | Custom AI endpoint (for Ollama) | |

**Priority Order:**

For admin URL: explicit flag > `MOCKD_ADMIN_URL` > context config > default
For workspace: explicit flag > `MOCKD_WORKSPACE` > context config > none
For context: explicit flag > `MOCKD_CONTEXT` > saved current context

---

## Exit Codes

| Code | Description |
|------|-------------|
| `0` | Success |
| `1` | General error |
| `2` | Configuration error |

---

## Additional Help Topics

```bash
mockd help config        # Configuration file format
mockd help matching      # Request matching patterns
mockd help templating    # Template variable reference
mockd help formats       # Import/export formats
mockd help websocket     # WebSocket mock configuration
mockd help graphql       # GraphQL mock configuration
mockd help grpc          # gRPC mock configuration
mockd help mqtt          # MQTT broker configuration
mockd help soap          # SOAP/WSDL mock configuration
mockd help sse           # Server-Sent Events configuration
```

---

## See Also

- [Configuration Reference](configuration.md) - Config file format
- [Admin API Reference](admin-api.md) - Runtime management API
- [Quickstart](../getting-started/quickstart.md) - Getting started guide
