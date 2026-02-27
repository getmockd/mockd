# mockd — AI Agent Instructions

> Copy this file to `.claude/mockd.md` in your project root for Claude Code,
> or adapt for other AI assistants.

## What is mockd?

mockd is a multi-protocol API mock server. It mocks HTTP, GraphQL, gRPC, WebSocket, MQTT, SSE, and SOAP from a single binary. It has a built-in MCP server with 16 tools — you can create, manage, and debug mocks directly via MCP without CLI commands.

## MCP Server

mockd has an MCP server. If it's configured in your MCP settings, use the MCP tools directly:

- `manage_mock` — Create, list, get, update, delete, or toggle mocks (action parameter)
- `verify_mock` — Assert a mock was called N times
- `get_mock_invocations` — See what requests hit a mock
- `set_chaos_config` — Inject latency, errors, or apply chaos profiles (slow-api, flaky, offline, etc.)
- `get_request_logs` — See all captured traffic
- `manage_state` — CRUD stateful resources
- `import_mocks` — Import from OpenAPI, Postman, HAR, WireMock, cURL, or YAML
- `export_mocks` — Export all mocks as YAML/JSON

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
```

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
- `{{request.Method}}`, `{{request.Path}}`, `{{request.Header "X-Id"}}` — Echo request data
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
4. For stateful CRUD, use `mockd add http --path /api/users --stateful` or configure `statefulResources` in YAML
5. Request logs: `mockd logs --requests` (not `mockd logs`)
6. Stop daemon: `mockd stop` (not Ctrl+C on a daemon)
