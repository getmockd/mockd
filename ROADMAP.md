# Mockd Roadmap

This document outlines the planned development roadmap for mockd. It is a living document and community input is welcome via [GitHub Discussions](https://github.com/getmockd/mockd/discussions).

## v0.3.x (Current)

Pre-release version with all core protocols, cloud tunneling, and full Cobra CLI:

- **7 Protocol Support**: HTTP, WebSocket, gRPC, MQTT, SSE, GraphQL, SOAP
- **Full Cobra CLI**: All commands use `spf13/cobra` with `charmbracelet/huh` interactive forms
- **Cloud Tunnel**: `mockd tunnel-quic` exposes local mocks to the internet via QUIC relay — all protocols tunneled through port 443
- **Multi-Protocol Tunneling**: gRPC (native HTTP/2), WebSocket (bidirectional), MQTT (ALPN routing), SSE streaming — all working through the tunnel
- **Tunnel Authentication**: Token, HTTP Basic Auth, and IP allowlist protection for tunnel URLs
- **CLI Interface**: Full command-line interface with 30+ commands
- **Admin API**: RESTful API for programmatic mock management
- **Proxy Recording**: Record real HTTP/HTTPS traffic and replay as mocks
- **Stateful Mocking**: Maintain state across requests for realistic scenarios
- **Chaos Engineering**: Fault injection, latency simulation, and error responses
- **OpenTelemetry Tracing**: Distributed tracing integration for observability
- **MCP Server**: Model Context Protocol server for AI agent integration
- **Port Merging**: Automatic merging of gRPC services/MQTT topics on same port
- **Native Go Test Suites**: E2E, integration, and performance tests (no BATS dependency)

## v1.0.0 (Planned)

Stable release with production-ready features:

- **API Versioning**: Add `/v1/` prefix to Admin API endpoints
- **Pagination**: Implement pagination for list endpoints
- **Custom Subdomains**: Persistent tunnel subdomains for paid tiers
- **GraphQL Query Validation**: Validate incoming queries against schema
- **gRPC Error Details**: Support for rich error details in gRPC responses
- **Windows Installation**: Improved installer and PATH configuration for Windows

## v1.2.0 (Planned)

Focus on user experience and advanced features:

- **Web UI**: Browser-based interface for mock management at mockd.io
- **Desktop Application**: Native desktop app
- **OpenAPI Request Validation**: Validate incoming requests against OpenAPI specs during mocking
- **WebSocket Fallback Tunnel**: Automatic fallback when QUIC/UDP is blocked

## v2.0.0 (Future)

Major feature additions and ecosystem expansion:

- **Contract Testing Integration**: Pact provider verification support
- **VS Code Extension**: IDE integration for mock development
- **Template Gallery**: Pre-built templates for common APIs (Stripe, Twilio, AWS, etc.)
- **MQTT v5 Full Support**: Complete MQTT v5 protocol implementation
- **End-to-End Tunnel Encryption**: Private CA for zero-trust tunnel security

---

*Have ideas or feature requests? Join the discussion at [GitHub Discussions](https://github.com/getmockd/mockd/discussions).*
