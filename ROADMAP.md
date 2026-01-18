# Mockd Roadmap

This document outlines the planned development roadmap for mockd. It is a living document and community input is welcome via [GitHub Discussions](https://github.com/getmockd/mockd/discussions).

## v1.0.0 (Current Release)

The initial release includes a comprehensive feature set for multi-protocol API mocking:

- **Protocol Support**: HTTP, WebSocket, gRPC, MQTT, SSE, GraphQL, SOAP
- **CLI Interface**: Full command-line interface for all operations
- **Admin API**: RESTful API for programmatic mock management
- **Proxy Recording**: Record real API traffic and replay as mocks
- **Stateful Mocking**: Maintain state across requests for realistic scenarios
- **Chaos Engineering**: Fault injection, latency simulation, and error responses
- **OpenTelemetry Tracing**: Distributed tracing integration for observability
- **MCP Server**: Model Context Protocol server for AI agent integration

## v1.1.0 (Planned)

Focus on API improvements and protocol enhancements:

- **API Versioning**: Add `/v1/` prefix to Admin API endpoints
- **Pagination**: Implement pagination for list endpoints
- **GraphQL Query Validation**: Validate incoming queries against schema
- **WebSocket Binary Encoding**: Fix binary message encoding issues
- **gRPC Error Details**: Support for rich error details in gRPC responses
- **Windows Installation**: Improved installer and PATH configuration for Windows

## v1.2.0 (Planned)

Focus on user experience and advanced features:

- **Desktop Application**: Native desktop app using Electron or Tauri
- **Web UI**: Browser-based interface for mock management
- **OpenAPI Request Validation**: Validate incoming requests against OpenAPI specs during mocking
- **HTTP/2 Support**: Full HTTP/2 protocol support

## v2.0.0 (Future)

Major feature additions and ecosystem expansion:

- **Contract Testing Integration**: Pact provider verification support
- **VS Code Extension**: IDE integration for mock development
- **Template Gallery**: Pre-built templates for common APIs (Stripe, Twilio, AWS, etc.)
- **MQTT v5 Full Support**: Complete MQTT v5 protocol implementation

---

*Have ideas or feature requests? Join the discussion at [GitHub Discussions](https://github.com/getmockd/mockd/discussions).*
