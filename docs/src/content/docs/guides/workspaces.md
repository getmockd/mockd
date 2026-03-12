---
title: Workspaces
description: Isolate mocks, state, and logs with workspaces for multi-tenant testing, environment separation, and parallel test runs.
---

Workspaces provide isolated environments within a single mockd instance. Each workspace has its own mocks, stateful resources, request logs, and custom operations — completely independent from other workspaces.

## When to Use Workspaces

- **Multi-tenant testing** — simulate different customer environments side by side
- **Environment isolation** — separate dev, staging, and QA mock configurations
- **Parallel test runs** — each test suite gets its own workspace so tests don't interfere
- **Team collaboration** — teammates work on different API mocks without conflicts

## Creating Workspaces

### CLI

```bash
# Create a workspace
mockd workspace create -n "Payment API" -d "Stripe mock environment"

# Create and switch to it immediately
mockd workspace create -n "Payment API" --use
```

### MCP

```json
{ "action": "create", "name": "Payment API" }
```

### Admin API

```bash
curl -X POST http://localhost:4290/workspaces \
  -H "Content-Type: application/json" \
  -d '{"name": "Payment API", "description": "Stripe mock environment"}'
```

## Switching Workspaces

Once created, switch to a workspace so all subsequent commands target it:

```bash
# Switch to a workspace (persists in context config)
mockd workspace use ws_abc123

# Verify current workspace
mockd workspace show
```

The active workspace is saved in `~/.config/mockd/contexts.yaml` and persists across terminal sessions.

## Workspace Scoping

When a workspace is active, all operations are scoped to it:

| Resource | Scoped? | Details |
|----------|---------|---------|
| Mocks | Yes | Each workspace has its own set of mocks |
| Stateful tables/resources | Yes | Data stores are independent per workspace |
| Request logs | Yes | Logs are filtered by workspace |
| Custom operations | Yes | Operations are registered per workspace |
| Import/Export | Yes | Imports go into the active workspace; exports come from it |
| Chaos config | **No** | Chaos injection is global across all workspaces |

## The `--workspace` Flag

Instead of switching the persistent workspace, you can scope a single command with the `--workspace` flag:

```bash
# List mocks in a specific workspace without switching
mockd list --workspace ws_abc123

# Import into a specific workspace
mockd import openapi.yaml --workspace ws_abc123

# Export from a specific workspace
mockd export --workspace ws_abc123 -o mocks.yaml

# View logs for a specific workspace
mockd logs --workspace ws_abc123
```

This flag is available on every CLI command as a global flag.

## Environment Variable

Set `MOCKD_WORKSPACE` to scope all commands to a workspace without using `--workspace` on every call:

```bash
export MOCKD_WORKSPACE=ws_abc123
mockd list          # scoped to ws_abc123
mockd import ...    # scoped to ws_abc123
```

### Resolution Order

When multiple sources specify a workspace, mockd uses this priority:

1. `--workspace` flag (highest priority)
2. `MOCKD_WORKSPACE` environment variable
3. Context config (`mockd workspace use`)
4. Default workspace (no workspace — global scope)

## Listing and Managing Workspaces

```bash
# List all workspaces
mockd workspace list

# Output:
# CURRENT  ID                  NAME            BASE PATH  TYPE    DESCRIPTION
# *        ws_abc123           Payment API     /          local   Stripe mock environment
#          ws_def456           Comms API       /          local   Twilio mock environment

# Delete a workspace
mockd workspace delete ws_def456

# Force delete (skip confirmation)
mockd workspace delete ws_def456 --force

# Clear workspace selection (revert to default)
mockd workspace clear
```

## Practical Examples

### Parallel Test Suites

Give each test suite its own workspace so tests can run concurrently without mock collisions:

```bash
# In test setup
WORKSPACE_ID=$(mockd workspace create -n "test-suite-$RANDOM" --json | jq -r '.id')

# Run tests scoped to this workspace
MOCKD_WORKSPACE=$WORKSPACE_ID pytest tests/

# Teardown
mockd workspace delete $WORKSPACE_ID --force
```

### Multi-API Development

When building against multiple third-party APIs, use separate workspaces to keep configurations clean:

```bash
# Stripe mocks
mockd workspace create -n "Stripe" --use
mockd import stripe-openapi.yaml

# Switch to Twilio
mockd workspace create -n "Twilio" --use
mockd import twilio-openapi.yaml

# Switch between them as needed
mockd workspace use ws_abc123  # back to Stripe
```

### MCP Workflow

AI assistants can manage workspaces via the `manage_workspace` MCP tool:

```json
// List workspaces
{ "action": "list" }

// Create a workspace
{ "action": "create", "name": "Integration Tests" }

// Switch to a workspace
{ "action": "switch", "id": "ws_abc123" }
```

All subsequent `manage_mock`, `manage_state`, and `import_mocks` calls are automatically scoped to the active workspace.

## Limitations

- **Chaos config is global.** Chaos fault injection (latency, error rates, circuit breakers) applies to all workspaces. You cannot configure chaos per workspace.
- **Persistence does not round-trip workspace context.** On server restart, persisted mocks retain their `workspaceId` field, but the active workspace selection (set via `mockd workspace use`) resets. Re-select with `mockd workspace use` after restart.
- **Port conflicts.** If workspaces contain gRPC or MQTT mocks that bind to the same port, only one can be active at a time.

## See Also

- [CLI Reference: workspace commands](/reference/cli/#mockd-workspace)
- [Admin API: workspace endpoints](/reference/admin-api/#get-workspaces)
- [Stateful Mocking](/guides/stateful-mocking/) — stateful resources are workspace-scoped
- [MCP Server](/guides/mcp-server/) — `manage_workspace` tool
