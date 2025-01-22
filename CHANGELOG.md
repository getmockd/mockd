# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

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

[Unreleased]: https://github.com/getmockd/mockd/compare/v0.2.0...HEAD
[0.2.0]: https://github.com/getmockd/mockd/compare/v0.1.0...v0.2.0
[0.1.0]: https://github.com/getmockd/mockd/releases/tag/v0.1.0
