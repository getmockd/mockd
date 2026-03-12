# mockd — AI Agent Instructions

> Copy this file to `.claude/mockd.md` in your project root for Claude Code,
> or adapt for other AI assistants.

## What is mockd?

mockd is a multi-protocol API mock server. It mocks HTTP, GraphQL, gRPC, WebSocket, MQTT, SSE, and SOAP from a single binary. It has a built-in MCP server with 18 tools — you can create, manage, and debug mocks directly via MCP without CLI commands.

## MCP Server

mockd has an MCP server. If it's configured in your MCP settings, use the MCP tools directly:

- `manage_mock` — Create, list, get, update, delete, or toggle mocks (action parameter)
- `manage_context` — View or switch admin server context (action: get/switch; name for switch)
- `manage_workspace` — List, switch, or create workspaces (action: list/switch/create; id, name)
- `import_mocks` — Import from OpenAPI, Postman, HAR, WireMock, cURL, WSDL, Mockoon, or YAML
- `export_mocks` — Export all mocks as YAML/JSON
- `get_server_status` — Health check, ports, and statistics (no params)
- `get_request_logs` — See all captured traffic (limit, offset, method, pathPrefix, mockId, protocol, unmatchedOnly)
- `clear_request_logs` — Remove all captured request/response logs (no params)
- `get_chaos_config` — Current chaos fault injection config and statistics (no params)
- `set_chaos_config` — Inject latency, errors, apply chaos profiles, or configure advanced rules (circuit breaker, retry-after, progressive degradation)
- `reset_chaos_stats` — Reset chaos counters to zero without changing config (no params)
- `get_stateful_faults` — View circuit breaker, retry-after, and progressive degradation state
- `manage_circuit_breaker` — Trip or reset circuit breakers manually (action: trip/reset; key)
- `verify_mock` — Assert a mock was called N times (id, expected_count, at_least, at_most)
- `get_mock_invocations` — See what requests hit a mock (id, limit)
- `reset_verification` — Clear invocation records and counters for one mock or all (id optional, omit for all)
- `manage_state` — CRUD stateful resources (action: overview/add_resource/list_items/get_item/create_item/reset/delete_resource; resource, item_id, data, seed_data, etc.)
- `manage_custom_operation` — Manage custom operations on stateful resources (action: list/get/register/delete/execute; name, definition, input)

## Ports

- Mock server: **4280** (not 8080)
- Admin API: **4290** (not 8081)
- With `--data-dir`: 14280/14290

## CLI Quick Reference

```bash
# Start/stop
mockd start                        # Background daemon
mockd start -c mocks.yaml          # With config
mockd stop                         # Stop daemon

# Create mocks
mockd add http --path /api/users --status 200 --body '{"users":[]}'
mockd add http --path /api/users --method POST --status 201 --body '{"id":"{{uuid}}"}'
mockd add http --path /api/users --stateful   # Auto-creates CRUD endpoints

# Other protocols
mockd add graphql --path /graphql --operation GetUser --status 200 --body '...'
mockd add grpc --path /api.UserService --rpc-method GetUser --status 200 --body '...'
mockd add websocket --path /ws --message '{"event":"connected"}'
mockd add mqtt --topic sensors/temp --payload '{"temp":{{faker.latitude}}}'
mockd add sse --path /events --event-type update --body '{"ts":"{{now}}"}'
mockd add soap --path /ws --operation GetUser --status 200 --response '<User>...</User>'

# Manage
mockd list                         # List all mocks
mockd delete <id>                  # Delete a mock
mockd update <id> --status 201     # Update response status
mockd update <id> --body '{"ok":true}'  # Update response body
mockd update <id> --table users --bind list  # Bind to stateful table
mockd update <id> --delay 500      # Add response delay
mockd update <id> --enabled false  # Disable a mock
mockd update <id> --name "My Mock" # Set display name
mockd update <id> --table users --bind custom --operation VerifyUser
mockd logs --requests              # View request logs

# Verify (CI-friendly, exit code 1 on failure)
mockd verify check <id> --exactly 3
mockd verify check <id> --at-least 1
mockd verify check <id> --never

# Chaos
mockd chaos apply flaky            # 30% error rate
mockd chaos apply slow-api         # 200-800ms latency
mockd chaos apply offline          # 100% 503 errors
mockd chaos disable

# Import/export
mockd import openapi.yaml
mockd import postman-collection.json
cat mocks.yaml | mockd import
mockd export --format yaml > backup.yaml

# Workspaces
mockd workspace list              # List all workspaces
mockd workspace create -n "Name"  # Create a workspace
mockd workspace use <id>          # Switch to a workspace
mockd workspace delete <id>       # Delete a workspace
mockd workspace clear             # Clear workspace selection
```

All CLI commands accept `--workspace <id>` as a global flag to scope operations to a specific workspace.

## Workspaces

All CLI commands accept `--workspace <id>` to scope operations to a specific workspace.
Environment variable: `MOCKD_WORKSPACE`. Resolution order: flag > env > context config > default.

Stateful resources, request logs, and import/export are all workspace-scoped.
Chaos config is global (not workspace-scoped).

## Config File Format (YAML)

```yaml
mocks:
  - id: get-users
    type: http
    http:
      matcher:
        method: GET
        path: /api/users
      response:
        statusCode: 200
        headers:
          Content-Type: application/json
        body: |
          [{"id":"{{uuid}}","name":"{{faker.name}}","email":"{{faker.email}}"}]

  - id: create-user
    type: http
    http:
      matcher:
        method: POST
        path: /api/users
      response:
        statusCode: 201
        body: |
          {"id":"{{uuid}}","created":true}
```

## Tables + Extend (Stateful Config)

The recommended way to add stateful CRUD to imported API specs in config files:

```yaml
# Tables + Extend (recommended for imported specs)
version: "1.0"
imports:
  - path: api-spec.yaml
    as: api
tables:
  - name: users
    idField: id
    seedData:
      - { id: "1", name: "Alice", email: "alice@example.com" }
extend:
  - { mock: api.ListUsers, table: users, action: list }
  - { mock: api.CreateUser, table: users, action: create }
  - { mock: api.GetUser, table: users, action: get }
  - { mock: api.UpdateUser, table: users, action: update }
  - { mock: api.DeleteUser, table: users, action: delete }
```

- **tables**: Pure data stores — no routing, no basePath. Just a name, optional idField, and optional seedData.
- **extend**: Binds imported mock endpoints to table CRUD actions (`list`, `get`, `create`, `update`, `delete`).
- **Custom operations**: Use `action: custom` + `operation: OpName` for non-CRUD actions (e.g., confirm, capture, cancel).
- The default list response format is `{"data":[...],"meta":{...}}`. Use response transforms to customize the envelope.

### Stateful Bindings via MCP / API / CLI

You can also create stateful bindings at runtime without a config file:

**MCP** — use `extend` on `manage_mock`:
```
1. manage_state: {"action":"add_resource","resource":"users"}
2. manage_mock:  {"action":"create","type":"http","http":{"matcher":{"method":"GET","path":"/api/users"}},"extend":{"table":"users","action":"list"}}
3. manage_mock:  {"action":"create","type":"http","http":{"matcher":{"method":"POST","path":"/api/users"}},"extend":{"table":"users","action":"create"}}
4. manage_mock:  {"action":"create","type":"http","http":{"matcher":{"method":"GET","path":"/api/users/{id}"}},"extend":{"table":"users","action":"get"}}
```

**CLI** — use `--table` and `--bind`:
```bash
mockd add http --path /api/users --table users --bind list
mockd add http --path /api/users --method POST --table users --bind create
mockd add http --path /api/users/{id} --table users --bind get
mockd add http --path /api/users/{id} --method PUT --table users --bind update
mockd add http --path /api/users/{id} --method DELETE --table users --bind delete
mockd add http --path /api/users/{id}/verify --method POST --table users --bind custom --operation VerifyUser
```

Actions: `list`, `get`, `create`, `update`, `patch`, `delete`, `custom`. For `custom`, also provide `--operation` (CLI) or `"operation"` (MCP).

## Template Functions

Use `{{...}}` in response bodies:

- `{{uuid}}`, `{{uuid.short}}` — UUIDs
- `{{faker.name}}`, `{{faker.email}}`, `{{faker.phone}}` — Identity
- `{{faker.creditCard}}`, `{{faker.iban}}`, `{{faker.price}}` — Finance
- `{{faker.ipv4}}`, `{{faker.ipv6}}`, `{{faker.userAgent}}` — Internet
- `{{faker.latitude}}`, `{{faker.longitude}}` — Geo
- `{{faker.words(5)}}`, `{{faker.slug}}` — Text
- `{{now}}`, `{{timestamp}}` — Time
- `{{random.Int 1 100}}`, `{{random.Float 0.0 1.0}}` — Random numbers
- `{{random.Element "a" "b" "c"}}` — Random choice
- `{{request.Method}}`, `{{request.Path}}`, `{{request.header.X-Id}}` — Echo request data (case-insensitive)
- `{{request.PathParam.id}}` — Path parameter from `{id}` in path
- `{{request.query.page}}` — Query parameter value
- `{{request.rawBody}}` — Raw request body
- `{{sequence("counter", 1)}}` — Auto-incrementing counter

## Matching

- **Path**: Exact match or `{param}` patterns (e.g., `/api/users/{id}`)
- **Path regex**: Use `pathPattern` field (e.g., `^/api/v[0-9]+/.*`)
- **Headers**: Glob patterns (`Authorization: Bearer*`)
- **Query params**: Exact match only
- **Body**: `bodyContains` (substring), `bodyEquals` (exact), `bodyPattern` (regex)

## Key Rules

1. Always use ports 4280/4290, never 8080
2. The `id` and `type` fields are auto-generated if omitted in config files
3. Admin API expects `type` + protocol wrapper (e.g., `type: http` + `http: { matcher: ..., response: ... }`)
4. For stateful CRUD, use `mockd add http --path /api/users --stateful` for quick prototyping, or configure tables+extend in YAML config files (recommended)
5. Request logs: `mockd logs --requests` (not `mockd logs`)
6. Stop daemon: `mockd stop` (not Ctrl+C on a daemon)
