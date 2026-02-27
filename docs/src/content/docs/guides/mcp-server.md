---
title: MCP Server
description: Use mockd from AI-powered editors like Cursor, Windsurf, and Claude Code via the Model Context Protocol
---

mockd includes a built-in [Model Context Protocol](https://modelcontextprotocol.io/) (MCP) server with 16 tools for creating, managing, and debugging mocks directly from AI-powered editors.

## Quick Start

```bash
# Start mockd with MCP support (stdio transport)
mockd mcp

# Or enable MCP alongside the mock server (HTTP transport)
mockd serve --mcp
```

## Editor Setup

### Claude Code / Claude Desktop

Add to your MCP config (`~/.claude/claude_code_config.json` or Claude Desktop settings):

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

### Cursor

Add to `.cursor/mcp.json` in your project root:

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

### Windsurf

Add to your MCP configuration:

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

## Available Tools (16)

mockd's MCP server exposes 16 tools organized by function:

### Mock Management

| Tool | Actions | Description |
|------|---------|-------------|
| `manage_mock` | list, get, create, update, delete, toggle | Full CRUD for mock endpoints across all 7 protocols |

### Import & Export

| Tool | Description |
|------|-------------|
| `import_mocks` | Import from OpenAPI, Postman, HAR, WireMock, cURL, or mockd YAML/JSON |
| `export_mocks` | Export all mocks as YAML or JSON |

### Observability

| Tool | Description |
|------|-------------|
| `get_server_status` | Server health, ports, uptime, and statistics |
| `get_request_logs` | View captured request/response logs with protocol filtering |
| `clear_request_logs` | Clear all logs for test isolation |

### Chaos Engineering

| Tool | Description |
|------|-------------|
| `get_chaos_config` | View current chaos fault injection settings |
| `set_chaos_config` | Configure latency, error rates, bandwidth throttling, or apply named profiles |
| `reset_chaos_stats` | Reset injection statistics counters |

### Mock Verification

| Tool | Description |
|------|-------------|
| `verify_mock` | Assert a mock was called the expected number of times |
| `get_mock_invocations` | View detailed request/response pairs for a specific mock |
| `reset_verification` | Clear verification data for test isolation |

### Stateful Resources

| Tool | Actions | Description |
|------|---------|-------------|
| `manage_state` | overview, add_resource, list_items, get_item, create_item, reset | Manage CRUD collections that persist data across requests |

### Custom Operations

| Tool | Actions | Description |
|------|---------|-------------|
| `manage_custom_operation` | list, get, register, delete, execute | Multi-step operations on stateful resources |

### Session Management

| Tool | Actions | Description |
|------|---------|-------------|
| `manage_context` | get, switch | Switch between mockd server contexts (multi-environment) |
| `manage_workspace` | list, switch | Switch between isolated workspace configurations |

## Example Workflow

Here's what a typical AI-assisted workflow looks like:

1. **Create mocks** — Ask your AI editor: "Create a REST API for a todo app with GET, POST, PUT, DELETE"
2. **Send test traffic** — The AI calls `get_request_logs` to verify traffic is flowing
3. **Verify behavior** — Use `verify_mock` to assert the right endpoints were called
4. **Inject chaos** — Apply the `flaky` profile with `set_chaos_config` to test resilience
5. **Manage state** — Use `manage_state` to create, inspect, and reset CRUD resources
6. **Export** — Save the configuration with `export_mocks` for version control

## MCP Resources

The MCP server also exposes two resources for AI context:

| Resource URI | Description |
|-------------|-------------|
| `mock://chaos` | Current chaos configuration (read-only) |
| `mock://verification/{mockId}` | Verification data for a specific mock |

## Transports

mockd supports two MCP transports:

| Transport | Command | Use Case |
|-----------|---------|----------|
| **stdio** | `mockd mcp` | Editor integration (recommended) |
| **HTTP** | `mockd serve --mcp` | Remote access, shared server |

The stdio transport is recommended for local editor integration. The HTTP transport runs alongside the mock server and is useful when the mockd server is running on a remote machine or in a container.

## Next Steps

- [Chaos Engineering](/guides/chaos-engineering/) — Fault injection and chaos profiles
- [Mock Verification](/guides/mock-verification/) — Verify mock call counts and invocations
- [Stateful Mocking](/guides/stateful-mocking/) — CRUD simulation and custom operations
- [Import & Export](/guides/import-export/) — Bring existing API definitions
