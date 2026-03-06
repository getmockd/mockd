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

The dashboard is a single Svelte frontend codebase with two delivery mechanisms:

1. **Embedded web dashboard** (production) — The compiled frontend is embedded into the Go binary at build time and served from the admin API. The dashboard communicates with mockd over the same port it's served from.

2. **Wails desktop app** (development) — The same frontend runs as a native desktop application via Wails during development.

In production, the dashboard makes API calls to `http://localhost:4290/...` — the same admin API used by the CLI and MCP server. There is no separate backend; the dashboard is a static frontend talking to the existing admin API.

## Running with Docker

The official Docker image includes the dashboard:

```bash
docker run -p 4280:4280 -p 4290:4290 ghcr.io/getmockd/mockd:latest
```

- Mock server: `http://localhost:4280`
- Dashboard: `http://localhost:4290`

## Building from Source

To build mockd with the dashboard included, use the `dashboard` build tag:

```bash
go build -tags dashboard ./cmd/mockd
```

This requires the pre-built frontend distribution in `pkg/admin/dashboard/dist/`. You don't need to build the frontend yourself — the release CI automatically downloads the compiled frontend assets from the `mockd-desktop` releases.

Without the `-tags dashboard` flag, the binary builds normally but serves a plain-text fallback instead of the dashboard UI when you visit the admin port in a browser.
