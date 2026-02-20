# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- `--cors-origins` flag on `serve` command
- `--rate-limit` flag on `serve` command
- `--no-persist` flag on `serve` command
- `--watch` flag on `serve` command
- `--match-body-contains` flag on `add` command
- `--path-pattern` flag on `add` command
- `--type oauth` for `add` command with `--issuer`, `--client-id`, `--client-secret`, `--oauth-user`, `--oauth-password`
- `{{random.string(N)}}` template function
- `{{mtls.san.ip}}` and `{{mtls.san.uri}}` template variables
- `{{sequence("name", start)}}` works in all contexts (not just MQTT)
- MCP stateful tools now work through admin API
- Stateful item-level CRUD admin endpoints
- 20+ new tests (chaos, template, mTLS)

### Fixed

- SSE template expressions now resolve (was returning literal strings)
- OpenAPI body validation errors now include field paths
- Health endpoint zero timestamp in Docker (startTime race condition)
- Install script version display uses installed binary instead of PATH lookup
- Content-Type auto-detection: JSON bodies get `application/json` instead of `text/plain`
- `bodyFile` relative path resolution (resolves relative to config file directory)
- Validation mode "warn" now adds warning headers instead of blocking
- Validation mode "permissive" now skips validation entirely
- Stateful capacity error returns 507 instead of 500
- Chaos probability values clamped to [0.0, 1.0]
- Chaos per-path rules now properly preempt global rules
- SSE rate limit headers now sent when configured
- Unknown CLI command now shows helpful error with available commands
- Port range validation (0-65535) prevents misleading OS errors
- `--match-query` now accepts both `key=value` and `key:value` formats
- `{{default}}` template function now properly resolves context values

### Changed

- Template engine `New()` always initializes a SequenceStore

## [0.2.9] - 2026-02-19

### Added

- CLI UX improvements: `mockd add` upserts by method+path, `mockd list -w`, `mockd delete --path/--method/--yes`, `mockd rm` alias
- README overhauled

### Changed

- Homebrew tap renamed to `getmockd/homebrew-tap`
- Helm chart bumped

## [0.2.8] - 2026-02-15

### Fixed

- 5 test fixes (envelope unwrapping, SOAP WSDL, E2E import)

### Notes

- 53 commits pushed to origin, all CI green
- First public push

## [0.2.7] - 2026-02-12

### Fixed

- Validation body double-read elimination (halves peak memory)
- 9 P3 cosmetic fixes: MCP version wiring, timestamps, seed IDs, export options, variable shadow, log IDs, modulo bias, Insomnia export

### Added

- Protocol interface documentation
- Template engine boundary documentation

## [0.2.6] - 2026-02-12

### Added

- 59 recording tests (WebSocket, SOAP handler, SOAP converter)
- 39 tests across various subsystems

### Fixed

- 12 P3/cosmetic fixes and dead code cleanup

### Notes

- Marketing audit on mockd-ui

## [0.2.5] - 2026-02-12

### Fixed

- 22 bug fixes across admin, portability, CLI, MCP, tracing, stateful

### Added

- 67 new tests (chaos, portability, stateful, CLI config)

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

[Unreleased]: https://github.com/getmockd/mockd/compare/v0.2.9...HEAD
[0.2.9]: https://github.com/getmockd/mockd/compare/v0.2.8...v0.2.9
[0.2.8]: https://github.com/getmockd/mockd/compare/v0.2.7...v0.2.8
[0.2.7]: https://github.com/getmockd/mockd/compare/v0.2.6...v0.2.7
[0.2.6]: https://github.com/getmockd/mockd/compare/v0.2.5...v0.2.6
[0.2.5]: https://github.com/getmockd/mockd/compare/v0.2.4...v0.2.5
[0.2.4]: https://github.com/getmockd/mockd/compare/v0.2.0...v0.2.4
[0.2.0]: https://github.com/getmockd/mockd/compare/v0.1.0...v0.2.0
[0.1.0]: https://github.com/getmockd/mockd/releases/tag/v0.1.0
