# mockd Rules for Cursor

> Copy this file to `.cursor/rules/mockd.md` in your project root.

## mockd Overview

mockd is a multi-protocol API mock server (HTTP, GraphQL, gRPC, WebSocket, MQTT, SSE, SOAP). It runs as a single binary with an admin API.

## MCP Integration

mockd has an MCP server with 18 tools. Configure in `.cursor/mcp.json`:

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

## Ports

- Mock server: **4280** (never use 8080)
- Admin API: **4290** (never use 8081)

## Common Patterns

### Create a mock endpoint
```bash
mockd add http --path /api/users --status 200 --body '{"users":[]}'
```

### Create CRUD endpoints (auto-generates GET, POST, PUT, DELETE)
```bash
mockd add http --path /api/users --stateful
```

### Start with a config file
```bash
mockd start -c mocks.yaml
```

### Verify mocks in CI
```bash
mockd verify check <mock-id> --exactly 3 || exit 1
```

### Inject chaos for resilience testing
```bash
mockd chaos apply flaky    # 30% errors
mockd chaos apply offline  # 100% 503
mockd chaos disable
```

## Config Format

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

- `{{uuid}}` — UUID v4
- `{{faker.name}}`, `{{faker.email}}`, `{{faker.phone}}` — Identity
- `{{faker.creditCard}}`, `{{faker.iban}}`, `{{faker.price}}` — Finance
- `{{now}}`, `{{timestamp}}` — Time
- `{{request.Method}}`, `{{request.Path}}` — Echo request
- `{{random.Int 1 100}}` — Random numbers
- `{{faker.words(5)}}` — Parameterized

## Tables + Extend (Stateful Config)

The recommended way to add stateful CRUD to imported API specs in config files:

```yaml
version: "1.0"
imports:
  - path: api-spec.yaml
    as: api
tables:
  - name: users
    idField: id
extend:
  - { mock: api.ListUsers, table: users, action: list }
  - { mock: api.CreateUser, table: users, action: create }
  - { mock: api.GetUser, table: users, action: get }
```

Tables are pure data stores (no routing, no basePath). Extend binds imported mocks to table CRUD actions. Custom operations use `action: custom` + `operation: OpName`.

## Key Rules

1. Ports 4280 (mock) and 4290 (admin) — never 8080
2. `id` and `type` auto-generated if omitted in config
3. Admin API: `POST /mocks` with `type` + protocol wrapper
4. Stateful CRUD: `mockd add http --path /api/users --stateful` for quick prototyping, or tables+extend in config (recommended)
5. Logs: `mockd logs --requests`
6. Stop: `mockd stop`
