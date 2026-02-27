---
title: MCP Server
description: Use mockd from AI-powered editors like Cursor, Windsurf, and Claude Code via the Model Context Protocol
---

mockd includes a built-in [Model Context Protocol](https://modelcontextprotocol.io/) (MCP) server with 16 tools for creating, managing, and debugging mocks directly from AI-powered editors.

## What is MCP?

The Model Context Protocol (MCP) is an open standard from [Anthropic](https://anthropic.com) that lets AI assistants interact with external tools and data sources. Instead of copy-pasting curl commands or switching between your editor and terminal, your AI assistant talks to mockd directly — creating mocks, inspecting traffic, injecting chaos, and verifying behavior in a single conversation.

mockd's MCP server is built on the official [mcp-go SDK](https://github.com/mark3labs/mcp-go) and exposes mockd's full capabilities as structured tools that any MCP-compatible client can discover and invoke.

:::note
mockd is the **only** API mocking tool with a built-in MCP server. No competitor (WireMock, Postman, Mockoon, MSW, Microcks) offers this.
:::

## Quick Start

```bash
# Start mockd with MCP support (stdio transport)
# Auto-starts a background daemon if no server is running — zero setup needed.
mockd mcp

# Or enable MCP alongside the mock server (HTTP transport)
mockd serve --mcp
```

## Zero-Setup: Auto-Start Daemon

When you run `mockd mcp`, it automatically handles server lifecycle:

1. **Already running?** Connects to the existing mockd server (via PID file or default URL)
2. **Nothing running?** Auto-starts a background daemon (`mockd start --detach --no-auth`)
3. **Daemon is shared** — it survives the MCP session, so multiple AI assistants (e.g., two Claude windows) share the same server and mocks persist across sessions

Stop the daemon with `mockd stop` when you're done.

### Project-Scoped Isolation

Use `--data-dir` to start a separate daemon per project, avoiding conflicts when working across multiple codebases:

```json
{
  "mcpServers": {
    "mockd": {
      "command": "mockd",
      "args": ["mcp", "--data-dir", "./mockd-data"]
    }
  }
}
```

Project daemons run on different ports (14280/14290 by default) and store their PID file inside the data directory. Multiple sessions in the same project share the same daemon.

### `mockd mcp` Flags

| Flag | Description | Default |
|------|-------------|---------|
| `--admin-url` | Connect to a specific admin API URL (skips auto-start) | |
| `--data-dir` | Project-scoped data directory (starts separate daemon) | |
| `--config` | Config file to load on daemon startup | |
| `--port` | Mock server port for project daemon | `4280` (or `14280` with `--data-dir`) |
| `--admin-port` | Admin API port for project daemon | `4290` (or `14290` with `--data-dir`) |
| `--log-level` | Log level for stderr output | `warn` |

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

Add to `~/.codeium/windsurf/mcp_config.json`:

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

:::tip[Prerequisites]
mockd must be installed and in your `PATH`. Verify with `mockd version`. If you installed via Docker, MCP stdio transport won't work — use the binary install (`brew install getmockd/tap/mockd` or `curl -sSL https://get.mockd.io | sh`).
:::

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

## Example: Creating a Mock via MCP

When you ask your AI editor "Create an endpoint that returns a list of users," the AI calls the `manage_mock` tool behind the scenes:

**Tool call** (`manage_mock` with action `create`):
```json
{
  "action": "create",
  "type": "http",
  "http": {
    "matcher": { "method": "GET", "path": "/api/users" },
    "response": {
      "statusCode": 200,
      "headers": { "Content-Type": "application/json" },
      "body": "[{\"id\":1,\"name\":\"{{faker.name}}\",\"email\":\"{{faker.email}}\"}]"
    }
  }
}
```

**Tool response:**
```json
{
  "action": "created",
  "id": "http_a1b2c3d4",
  "message": "Created http mock"
}
```

The AI can then verify it works by calling `verify_mock` after sending test traffic, or inject chaos with `set_chaos_config` to test your app's error handling — all without leaving the editor.

## Typical Workflow

Here's what a full AI-assisted development session looks like:

1. **Create mocks** — "Create a REST API for a todo app with GET, POST, PUT, DELETE"
2. **Send test traffic** — The AI calls `get_request_logs` to verify traffic is flowing
3. **Verify behavior** — `verify_mock` asserts the right endpoints were called the right number of times
4. **Inject chaos** — Apply the `flaky` profile with `set_chaos_config` to test resilience
5. **Manage state** — `manage_state` creates CRUD collections, seeds data, resets between tests
6. **Export** — `export_mocks` saves the full configuration for version control

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

## CLI Equivalents

Every MCP tool has a corresponding CLI command. Use these interchangeably:

| MCP Tool | CLI Equivalent |
|----------|---------------|
| `manage_mock` (create) | `mockd add http --path /api/users --status 200` |
| `verify_mock` | `mockd verify check <mock-id> --exactly 3` |
| `get_mock_invocations` | `mockd verify invocations <mock-id>` |
| `set_chaos_config` (profile) | `mockd chaos apply flaky` |
| `get_request_logs` | `mockd logs --requests` |
| `import_mocks` | `mockd import openapi.yaml` |
| `export_mocks` | `mockd export --format yaml` |

See the [CLI Reference](/reference/cli/) for the full command list, including [`mockd mcp`](/reference/cli/#mockd-mcp) and [`mockd verify`](/reference/cli/#mockd-verify).

## Next Steps

- [Chaos Engineering](/guides/chaos-engineering/) — Fault injection and chaos profiles
- [Mock Verification](/guides/mock-verification/) — Verify mock call counts and invocations
- [Stateful Mocking](/guides/stateful-mocking/) — CRUD simulation and custom operations
- [Import & Export](/guides/import-export/) — Bring existing API definitions
