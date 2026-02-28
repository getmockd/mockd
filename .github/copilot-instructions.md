# mockd Instructions for GitHub Copilot

> Copy this to `.github/copilot-instructions.md` in your project root.

## mockd Mock Server

This project uses mockd for API mocking. mockd is a multi-protocol mock server supporting HTTP, GraphQL, gRPC, WebSocket, MQTT, SSE, and SOAP from a single binary.

### MCP Server

mockd includes an MCP server with 18 tools. Configure in `.github/copilot-mcp.json`:

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

MCP tools available:

- `manage_mock` — Create, list, get, update, delete, or toggle mocks
- `manage_context` — Get server context for current workspace
- `manage_workspace` — Switch between named workspaces
- `verify_mock` — Assert a mock was called N times
- `get_mock_invocations` — See what requests hit a mock
- `reset_verification` — Clear verification counters
- `set_chaos_config` — Inject latency, errors, chaos profiles, stateful faults
- `get_chaos_config` — View current chaos configuration
- `reset_chaos_stats` — Reset chaos counters
- `get_stateful_faults` — View circuit breaker / retry-after / degradation state
- `manage_circuit_breaker` — Trip or reset circuit breakers
- `get_request_logs` — View captured traffic
- `clear_request_logs` — Clear request log history
- `manage_state` — CRUD operations on stateful resources
- `manage_custom_operation` — Manage custom operations for stateful resources
- `import_mocks` — Import from OpenAPI, Postman, HAR, WireMock, cURL, WSDL, Mockoon
- `export_mocks` — Export mocks as YAML/JSON
- `get_server_status` — Check server health and version

### Ports

- Mock server: **4280** (not 8080)
- Admin API: **4290** (not 8081)

### CLI Commands

```bash
mockd start                          # Start daemon
mockd start -c mocks.yaml            # Start with config
mockd stop                           # Stop daemon
mockd engine start                   # Headless engine for CI
mockd add http --path /api/users --status 200 --body '{"data":[]}'
mockd add http --path /api/users --stateful   # Auto CRUD
mockd list                           # List mocks
mockd logs --requests                # View traffic
mockd verify check <id> --exactly 3  # Assert call count
mockd chaos apply flaky              # Inject errors
mockd chaos disable                  # Stop chaos
mockd import openapi.yaml            # Import specs
mockd export --format yaml           # Export config
```

### Config File (YAML)

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

statefulResources:
  - name: users
    basePath: /api/users
    idField: id
    seedData:
      - id: "1"
        name: "Alice"
```

### Template Functions

`{{uuid}}`, `{{faker.name}}`, `{{faker.email}}`, `{{faker.creditCard}}`, `{{faker.ipv4}}`, `{{now}}`, `{{random.Int 1 100}}`, `{{request.Method}}`, `{{request.Path}}`, `{{faker.words(5)}}`, `{{sequence("counter", 1)}}`

### Key Rules

1. Always use ports 4280/4290, never 8080
2. `id` and `type` are auto-generated if omitted in config
3. Use `mockd logs --requests` for request logs
4. Use `mockd stop` to stop the daemon
5. Stateful list responses are paginated: `{"data": [...], "meta": {...}}`
