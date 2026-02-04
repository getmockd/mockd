# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.2.4] - 2026-02-04

### Added

- **MCP server overhaul** — 19 tools across all protocols with session-scoped context switching, `switch_context` for multi-environment workflows, and both stdio and HTTP transports (`mockd mcp` / `mockd serve --mcp`)
- **OpenRouter AI provider** — First-class support via `MOCKD_AI_PROVIDER=openrouter` with google/gemini-2.5-flash default. Access 200+ models through a single API key
- **AI mock validation pipeline** — Generated mocks are now validated before export, with invalid mocks skipped and warnings printed
- **Multi-protocol QUIC tunnel** (`mockd tunnel-quic`) — Expose local mocks to the internet through a single QUIC connection. All 7 protocols tunneled through port 443 with zero configuration
- **gRPC tunnel support** — Native HTTP/2 with trailer forwarding via streaming chunked body framing
- **MQTT tunnel support** — MQTT connections routed via TLS ALPN on port 443
- **WebSocket tunnel support** — Bidirectional WebSocket frames proxied through QUIC
- **Tunnel authentication** — Protect tunnel URLs with token auth, HTTP Basic Auth, or IP allowlists
- **Field-level validation** — Validate request bodies with type checking, constraints, patterns, and formats
- **OAuth token introspection** (`POST /introspect`) — RFC 7662 compliant
- **YAML export for proxy recordings**
- **SSE rate limiting**

### Changed

- AI generation prompt now requests `{param}` path syntax (not Express-style `:param`)
- AI provider default `maxTokens` increased from 500 to 4096 to prevent truncated responses
- Unified admin API create endpoint now validates mocks before storing
- Chaos API `ErrorRateConfig` uses `statusCodes` array and `defaultCode`
- `/engines` endpoint includes local engine in response

### Fixed

- **MQTT broker shutdown deadlock** — `Stop()` held the broker mutex while `server.Close()` triggered hook callbacks that also needed the mutex. Fixed with atomic stopping flag and lock release before close
- **AI code fence parsing** — LLMs wrapping JSON in markdown fences now stripped before parsing
- **Express-style path params** — AI-generated `:param` paths automatically normalized to `{param}`
- **Graceful degradation** — MCP tools and CLI commands now return actionable error messages when mockd server is unreachable
- **CVE patches** — Upgraded quic-go v0.49→v0.57 and x/crypto v0.44→v0.47
- Chaos injection CLI no longer nests config under a `global` key the API ignores

### Removed

- **gRPC recording** — Only recorded traffic from mockd's own server. External service recording deferred pending community demand
- Duplicate docs workflow file

### Security

- quic-go and x/crypto upgraded to resolve known CVEs

## [0.2.0] - 2026-01-21

### Added

- gRPC/MQTT port merging: automatically merge services/topics when creating mocks on the same port
- Port conflict detection with actionable error messages
- `mockd ports` command to list all ports in use
- CLI merge output shows added and total services/topics
- Metrics path normalization for UUIDs, MongoDB ObjectIDs, and numeric IDs
- Shared test helpers for port allocation stability

### Changed

- Version reset to 0.2.0 to reflect pre-release status
- Improved CLI help text for gRPC and MQTT flags (documents merge behavior)

### Fixed

- CLI handling of merge responses (HTTP 200 vs 201)
- Bulk create and update handlers properly detect merge targets as conflicts
- Integration test port allocation stability

## [0.1.0] - 2026-01-17

### Added

- Multi-protocol mock server support: HTTP, WebSocket, gRPC, MQTT, SSE, GraphQL, SOAP
- CLI with 30+ commands for mock management
- Admin API for runtime mock configuration
- Proxy recording mode for capturing real API traffic
- Stateful mocking for simulating CRUD operations
- Chaos engineering features: latency injection, error rates, timeouts
- mTLS support with certificate matching
- OpenTelemetry tracing integration
- Prometheus metrics endpoint
- OAuth mock provider for testing auth flows
- MCP server for AI agent integration
- Shell completion support for bash, zsh, and fish
- Import/export support for OpenAPI, Postman, WireMock, HAR, and cURL formats
- Docker container support
- Helm chart for Kubernetes deployment
- kubectl-style context management for switching between mockd deployments
- Workspace CLI commands for organizing mocks into logical groups

### Security

- Config file permissions restricted to `0600` (owner read/write only)
- Config directory permissions restricted to `0700`
- Auth tokens masked in JSON output

### Notes

- Initial public release (pre-1.0)
- Licensed under Apache 2.0

[Unreleased]: https://github.com/getmockd/mockd/compare/v0.2.4...HEAD
[0.2.4]: https://github.com/getmockd/mockd/compare/v0.2.0...v0.2.4
[0.2.0]: https://github.com/getmockd/mockd/compare/v0.1.0...v0.2.0
[0.1.0]: https://github.com/getmockd/mockd/releases/tag/v0.1.0
