# mockd Rules for Windsurf

> Copy this file to `.windsurfrules` in your project root, or add to your Windsurf workspace rules.

## mockd Overview

mockd is a multi-protocol API mock server (HTTP, GraphQL, gRPC, WebSocket, MQTT, SSE, SOAP). It runs as a single binary with a built-in admin API and MCP server.

## MCP Integration

mockd has an MCP server with 18 tools. Configure in your Windsurf MCP settings:

```json
{
  "mcpServers": {
    "mockd": {
      "command": "mockd",
      "args": ["mcp"]
    }
  }
}
```

The MCP server auto-starts a background daemon if no mockd server is running.

### MCP Tools

- `manage_mock` — Create, list, get, update, delete, or toggle mocks
- `manage_context` — Get server context for current workspace
- `manage_workspace` — Switch between named workspaces
- `verify_mock` — Assert a mock was called N times
- `get_mock_invocations` — See what requests hit a mock
- `reset_verification` — Clear verification counters
- `set_chaos_config` — Inject latency, errors, chaos profiles, or configure stateful faults (circuit breaker, retry-after, progressive degradation, chunked dribble)
- `get_chaos_config` — View current chaos configuration
- `reset_chaos_stats` — Reset chaos counters
- `get_stateful_faults` — View circuit breaker, retry-after, and progressive degradation state
- `manage_circuit_breaker` — Trip or reset circuit breakers manually
- `get_request_logs` — See all captured traffic
- `clear_request_logs` — Clear request log history
- `manage_state` — CRUD operations on stateful resources
- `manage_custom_operation` — Manage custom operations for stateful resources
- `import_mocks` — Import from OpenAPI, Postman, HAR, WireMock, cURL, WSDL, Mockoon, or YAML
- `export_mocks` — Export mocks as YAML/JSON
- `get_server_status` — Check server health and version

## Ports

- Mock server: **4280** (never use 8080)
- Admin API: **4290** (never use 8081)

## CLI Quick Reference

```bash
# Start/stop
mockd start                        # Background daemon
mockd start -c mocks.yaml          # With config
mockd stop                         # Stop daemon
mockd engine start                 # Headless engine (CI mode)

# Create mocks
mockd add http --path /api/users --status 200 --body '{"users":[]}'
mockd add http --path /api/users --stateful   # Auto CRUD

# Other protocols
mockd add graphql --path /graphql --operation GetUser --body '...'
mockd add grpc --path /api.UserService --rpc-method GetUser --body '...'
mockd add websocket --path /ws --message '{"event":"connected"}'
mockd add mqtt --topic sensors/temp --payload '{"temp":{{faker.latitude}}}'
mockd add sse --path /events --event-type update --body '{"ts":"{{now}}"}'
mockd add soap --path /ws --operation GetUser --response '<User>...</User>'

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
mockd export --format yaml > backup.yaml
```

## Config Format (YAML)

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
        body: '[{"id":"{{uuid}}","name":"{{faker.name}}"}]'
```

## Template Functions

- `{{uuid}}`, `{{uuid.short}}` — UUIDs
- `{{faker.name}}`, `{{faker.email}}`, `{{faker.phone}}` — Identity (35 faker types total)
- `{{faker.creditCard}}`, `{{faker.iban}}`, `{{faker.price}}` — Finance
- `{{faker.ipv4}}`, `{{faker.ipv6}}`, `{{faker.userAgent}}` — Internet
- `{{now}}`, `{{timestamp}}` — Time
- `{{random.Int 1 100}}`, `{{random.Float 0.0 1.0}}` — Random numbers
- `{{random.Element "a" "b" "c"}}` — Random choice
- `{{request.Method}}`, `{{request.Path}}`, `{{request.Header "X-Id"}}` — Echo request
- `{{sequence("counter", 1)}}` — Auto-incrementing counter

## Key Rules

1. Always use ports 4280/4290, never 8080
2. `id` and `type` are auto-generated if omitted in config files
3. Admin API: `POST /mocks` with `type` + protocol wrapper (`http: { matcher: ..., response: ... }`)
4. Stateful CRUD: `mockd add http --path /api/users --stateful` or `statefulResources` in config
5. Request logs: `mockd logs --requests` (not `mockd logs`)
6. Stop daemon: `mockd stop` (not Ctrl+C on a daemon)
7. List responses from stateful resources are paginated: `{"data": [...], "meta": {"total", "limit", "offset", "count"}}`
