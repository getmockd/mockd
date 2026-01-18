# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [1.0.0] - 2026-01-17

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

### Notes

- Initial public release
- Licensed under Apache 2.0

[Unreleased]: https://github.com/getmockd/mockd/compare/v1.0.0...HEAD
[1.0.0]: https://github.com/getmockd/mockd/releases/tag/v1.0.0
