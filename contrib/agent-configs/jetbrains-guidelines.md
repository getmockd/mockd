# mockd Guidelines for JetBrains AI Assistant

> Add this content to your JetBrains AI Assistant project-level prompt or custom instructions.

## Project Context

This project uses **mockd** for API mocking. mockd is a multi-protocol mock server supporting HTTP, GraphQL, gRPC, WebSocket, MQTT, SSE, and SOAP from a single binary.

## MCP Server

mockd includes an MCP server with 18 tools for AI-driven mock management. If MCP is available, configure it:

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

### Available MCP Tools

| Tool | Description |
|------|-------------|
| `manage_mock` | Create, list, get, update, delete, or toggle mocks |
| `manage_context` | Get server context for current workspace |
| `manage_workspace` | Switch between named workspaces |
| `verify_mock` | Assert a mock was called N times |
| `get_mock_invocations` | See what requests hit a mock |
| `reset_verification` | Clear verification counters |
| `set_chaos_config` | Inject latency, errors, chaos profiles, stateful faults |
| `get_chaos_config` | View current chaos configuration |
| `reset_chaos_stats` | Reset chaos counters |
| `get_stateful_faults` | View circuit breaker / retry-after / degradation state |
| `manage_circuit_breaker` | Trip or reset circuit breakers manually |
| `get_request_logs` | View captured traffic |
| `clear_request_logs` | Clear request log history |
| `manage_state` | CRUD operations on stateful resources |
| `manage_custom_operation` | Manage custom operations for stateful resources |
| `import_mocks` | Import from OpenAPI, Postman, HAR, WireMock, cURL, WSDL, Mockoon |
| `export_mocks` | Export mocks as YAML/JSON |
| `get_server_status` | Check server health and version |

## Important: Ports

- Mock server runs on **port 4280** (not 8080)
- Admin API runs on **port 4290** (not 8081)
- Never suggest 8080 or 8081 as mockd ports

## CLI Commands

```bash
# Lifecycle
mockd start                        # Start background daemon
mockd start -c mocks.yaml          # Start with config file
mockd stop                         # Stop daemon
mockd engine start                 # Headless engine for CI

# Create mocks
mockd add http --path /api/users --status 200 --body '{"users":[]}'
mockd add http --path /api/users --stateful   # Auto-generates CRUD

# Protocol-specific
mockd add graphql --path /graphql --operation GetUser --body '...'
mockd add grpc --path /api.UserService --rpc-method GetUser --body '...'
mockd add websocket --path /ws --message '{"event":"connected"}'
mockd add mqtt --topic sensors/temp --payload '{"temp":22.5}'
mockd add sse --path /events --event-type update --body '{"ts":"{{now}}"}'
mockd add soap --path /ws --operation GetUser --response '<User>...</User>'

# Inspection
mockd list                         # List all mocks
mockd logs --requests              # View request logs

# Verification (CI-friendly, exit code 1 on failure)
mockd verify check <id> --exactly 3
mockd verify check <id> --at-least 1

# Chaos engineering
mockd chaos apply flaky            # 30% error rate
mockd chaos apply slow-api         # 200-800ms latency
mockd chaos apply offline          # 100% 503 errors
mockd chaos disable

# Import/export
mockd import openapi.yaml
mockd import postman-collection.json
mockd export --format yaml > backup.yaml
```

## Configuration Format

mockd config files use YAML or JSON. Key structure:

```yaml
mocks:
  - id: get-users          # Optional, auto-generated
    type: http             # Optional, auto-generated
    http:
      matcher:
        method: GET
        path: /api/users
      response:
        statusCode: 200
        headers:
          Content-Type: application/json
        body: '[{"id":"{{uuid}}","name":"{{faker.name}}"}]'

statefulResources:
  - name: users
    basePath: /api/users
    idField: id
    seedData:
      - id: "1"
        name: "Alice"
        email: "alice@example.com"
```

## Response Templates

Use `{{...}}` in response bodies for dynamic data:

- **Identity**: `{{faker.name}}`, `{{faker.email}}`, `{{faker.phone}}`, `{{faker.username}}`
- **Finance**: `{{faker.creditCard}}`, `{{faker.iban}}`, `{{faker.price}}`
- **Internet**: `{{faker.ipv4}}`, `{{faker.ipv6}}`, `{{faker.userAgent}}`, `{{faker.url}}`
- **IDs**: `{{uuid}}`, `{{uuid.short}}`
- **Time**: `{{now}}`, `{{timestamp}}`
- **Numbers**: `{{random.Int 1 100}}`, `{{random.Float 0.0 1.0}}`
- **Text**: `{{faker.words(5)}}`, `{{faker.slug}}`, `{{faker.paragraph}}`
- **Request echo**: `{{request.Method}}`, `{{request.Path}}`, `{{request.Header "X-Id"}}`
- **Sequences**: `{{sequence("counter", 1)}}`

## Stateful Resource Responses

List endpoints on stateful resources return paginated envelopes:

```json
{
  "data": [{ "id": "1", "name": "Alice" }],
  "meta": { "total": 1, "limit": 20, "offset": 0, "count": 1 }
}
```

## Common Mistakes to Avoid

1. Using port 8080 instead of 4280
2. Forgetting `type: http` and `http:` wrapper in config (OK to omit — auto-inferred)
3. Using `mockd logs` instead of `mockd logs --requests`
4. Expecting bare arrays from stateful list endpoints (they return paginated envelopes)
5. Sending Ctrl+C to stop a daemon (use `mockd stop`)
