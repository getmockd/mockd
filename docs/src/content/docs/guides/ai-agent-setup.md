---
title: AI Agent Setup
description: Pre-built configuration templates for Claude Code, Cursor, GitHub Copilot, and other AI assistants
---

mockd ships with agent configuration templates that teach AI assistants how to use mockd effectively. These templates provide your AI editor with the correct ports, commands, config syntax, and patterns — so you can say "create an API mock" and it just works.

## Why Agent Config Templates?

AI assistants like Claude Code, Cursor, and GitHub Copilot read project-level instruction files to understand your tools and conventions. Without a mockd config, the AI might:

- Use port 8080 instead of 4280
- Generate invalid mock configs
- Miss useful features like `--stateful`, chaos profiles, or verification

With a config file, the AI knows mockd's full capabilities and generates correct commands on the first try.

## Quick Setup

### Claude Code

Copy `CLAUDE.md` to your project:

```bash
mkdir -p .claude
curl -sSL https://raw.githubusercontent.com/getmockd/mockd/main/contrib/agent-configs/CLAUDE.md \
  -o .claude/mockd.md
```

Claude Code reads all `.md` files in `.claude/` automatically.

### Cursor

Copy `cursor-rules.md` to your project:

```bash
mkdir -p .cursor/rules
curl -sSL https://raw.githubusercontent.com/getmockd/mockd/main/contrib/agent-configs/cursor-rules.md \
  -o .cursor/rules/mockd.md
```

Cursor reads all `.md` files in `.cursor/rules/` automatically.

### GitHub Copilot

Append the mockd instructions to your Copilot config:

```bash
curl -sSL https://raw.githubusercontent.com/getmockd/mockd/main/contrib/agent-configs/copilot-instructions.md \
  >> .github/copilot-instructions.md
```

## Combining with MCP

For the best experience, combine an agent config template with the [MCP server](/guides/mcp-server/). The config template teaches the AI about mockd concepts and syntax, while MCP gives it direct tool access to create and manage mocks without running CLI commands.

**Claude Code** — Add MCP + config:
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

Plus `.claude/mockd.md` in your project root.

**Cursor** — Add MCP + rules:

`.cursor/mcp.json`:
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

Plus `.cursor/rules/mockd.md` in your project root.

## What's In the Templates

Each template includes:

| Section | Purpose |
|---------|---------|
| **Ports** | Correct defaults: 4280 (mock), 4290 (admin) |
| **CLI Reference** | Create, list, delete, import, export, verify, chaos commands |
| **Config Format** | Valid YAML structure with `type` + protocol wrapper |
| **Template Functions** | 35 faker types (case-insensitive), UUID, timestamps, request echo, random values |
| **Matching Rules** | Path patterns, header globs, body matchers |
| **MCP Tools** | All 16 tools with action parameters |

## Customizing

The templates are starting points. You can customize them for your project:

```markdown
## Project-Specific Mocks

This project mocks the Payment API (Stripe-like):
- Base path: /api/v1/payments
- Auth: Bearer token in Authorization header
- Always return idempotency-key in response headers

When creating payment mocks, use:
- faker.creditCard for card numbers (Luhn-valid)
- faker.currencyCode for currencies (ISO 4217)
- faker.price for amounts
```

## Templates Location

All templates live in the mockd repository:

```
mockd/contrib/agent-configs/
  CLAUDE.md               # Claude Code / Claude Desktop
  cursor-rules.md         # Cursor
  copilot-instructions.md # GitHub Copilot
```

Browse them on GitHub: [github.com/getmockd/mockd/tree/main/contrib/agent-configs](https://github.com/getmockd/mockd/tree/main/contrib/agent-configs)
