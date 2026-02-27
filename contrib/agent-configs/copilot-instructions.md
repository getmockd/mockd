# mockd Instructions for GitHub Copilot

> Append this to `.github/copilot-instructions.md` in your project root.

## mockd Mock Server

This project uses mockd for API mocking. mockd is a multi-protocol mock server supporting HTTP, GraphQL, gRPC, WebSocket, MQTT, SSE, and SOAP.

### Ports
- Mock server: **4280** (not 8080)
- Admin API: **4290** (not 8081)

### CLI Commands
```bash
mockd start                          # Start daemon
mockd start -c mocks.yaml            # Start with config
mockd stop                           # Stop daemon
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
```

### Template Functions
`{{uuid}}`, `{{faker.name}}`, `{{faker.email}}`, `{{faker.creditCard}}`, `{{faker.ipv4}}`, `{{now}}`, `{{random.Int 1 100}}`, `{{request.Method}}`, `{{request.Path}}`, `{{faker.words(5)}}`

### Rules
1. Always use ports 4280/4290
2. `id` and `type` are auto-generated if omitted
3. Use `mockd logs --requests` for request logs
4. Use `mockd stop` to stop the daemon
