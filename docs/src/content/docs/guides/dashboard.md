---
title: Web Dashboard
description: Built-in web dashboard for managing mocks visually
sidebar:
  order: 15
---

mockd includes an embedded web dashboard — a full-featured Svelte UI for creating, editing, and managing mocks across all supported protocols. No separate install required. If your binary includes the dashboard, open your browser and start working.

## Accessing the Dashboard

When mockd is running, open the admin port in your browser:

```
http://localhost:4290
```

No extra flags or configuration needed. If the binary was built with the dashboard included, it's served automatically from the admin API port (default **4290**).

Release builds, Docker images, and packages installed via Homebrew, apt, or rpm all include the dashboard out of the box.

## Features

### Mock Management

Create, edit, and delete mocks for all 7 protocols from a single interface:

- **HTTP** — REST endpoints with method, path, headers, status, and body
- **WebSocket** — Connection handlers and message patterns
- **GraphQL** — Query and mutation mocks with operation matching
- **gRPC** — Service and RPC method mocks
- **SOAP** — WSDL-based operation mocks
- **MQTT** — Topic subscriptions and message payloads
- **OAuth** — Token endpoints and authorization flows

### Tabbed Editor

A VS Code-style editing experience:

- **Tabs** — Open multiple mocks simultaneously, switch between them
- **Command palette** — Press `Ctrl+K` to search commands, mocks, and actions
- **Keyboard shortcuts** — Navigate and edit without touching the mouse

### Mock Tree

The left sidebar organizes your mocks in a tree view:

- Folder grouping by protocol or custom structure
- Search and sort across all mocks
- Context menus with rename, duplicate, copy ID, copy JSON, and delete

### Request Log Viewer

Inspect incoming traffic in real time:

- Sortable columns for method, path, status, latency, and timestamp
- Near-miss debugging — see which mocks almost matched a request and why they didn't
- Filter and search across logged requests

### Additional Views

- **Recording sessions** — Manage proxy recording sessions and captured traffic
- **Stateful resources** — View and edit CRUD resources created by stateful mocks
- **Custom operations** — Manage custom operation handlers
- **Engine status** — Monitor the running engine, active mocks, and resource usage
- **Settings** — Configure theme (light/dark) and connection settings
- **Workspaces** — Create, switch, and delete workspaces to organize mock sets
- **Import/Export** — Import from OpenAPI, Postman, HAR, WireMock, and other formats; export your mocks as YAML or JSON

## Architecture

The dashboard is a Svelte single-page application embedded directly into the mockd binary. It's served as static assets from the admin API — no separate backend or additional process.

The dashboard makes API calls to `http://localhost:4290/...` — the same admin API used by the CLI and MCP server. Everything the dashboard can do, the CLI and API can do too. The dashboard is a convenience layer, not a separate system.

## Running with Docker

The official Docker image includes the dashboard:

```bash
docker run -p 4280:4280 -p 4290:4290 ghcr.io/getmockd/mockd:latest
```

- Mock server: `http://localhost:4280`
- Dashboard: `http://localhost:4290`

## Building from Source

If you build mockd from source with `go build ./cmd/mockd`, the binary works fully — CLI, admin API, MCP server, and all protocols — but does not include the dashboard UI. When you visit the admin port in a browser, you'll see a plain-text fallback explaining that the dashboard is available in release builds.

To get the dashboard, use one of the pre-built options:

- **Homebrew:** `brew install getmockd/tap/mockd`
- **Docker:** `docker run ghcr.io/getmockd/mockd:latest`
- **Release binary:** Download from [GitHub Releases](https://github.com/getmockd/mockd/releases)
- **Install script:** `curl -fsSL https://get.mockd.io | sh`

The dashboard is free but not open source. Release binaries include the compiled dashboard assets embedded at build time.
