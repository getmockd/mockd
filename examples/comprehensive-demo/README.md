# mockd Comprehensive Feature Demos

This directory contains comprehensive demo configurations showcasing all mockd features.

## Demo Files

| File | Mocks | Description |
|------|-------|-------------|
| `http-features-demo.json` | 60 | HTTP mocking: methods, path params, query params, headers, body matching, delays, templates |
| `http-sse-features-demo.json` | 32 | Server-Sent Events: basic streams, intervals, AI-style streaming, conditional events |
| `ws-features-demo.json` | 30 | WebSocket: echo, scenarios, heartbeat, subprotocols, binary messages |
| `graphql-features-demo.json` | 20 | GraphQL: schemas, resolvers, mutations, subscriptions, errors, introspection |
| `grpc-features-demo.json` | 13 | gRPC: unary/streaming RPCs, error codes, metadata matching, delays, reflection |
| `soap-features-demo.json` | 31 | SOAP 1.1/1.2: operations, faults, XPath matching, WSDL generation |

## Quick Start

### Load All Demos

```bash
# Start mockd
mockd serve

# Load all demo mocks (in another terminal)
./load-demos.sh
```

### Load Individual Demo

```bash
# Load just HTTP demos
mockd import http-features-demo.json

# Load just GraphQL demos  
mockd import graphql-features-demo.json
```

### Export to Insomnia

After loading mocks, export to Insomnia for easy testing:

```bash
# Get your API key
API_KEY=$(cat ~/.local/share/mockd/admin-api-key)

# Open in Insomnia (use URL import)
# http://localhost:4290/insomnia.yaml?api_key=$API_KEY
```

## Feature Highlights by Protocol

### HTTP Features
- Path parameters: `/users/{id}`, `/api/{version}/items/{item_id}`
- Regex path matching: `/products/[0-9]+`
- Query parameter matching and templating
- Header matching (exact, contains, regex)
- Body matching (contains, equals, JSONPath, regex)
- Response delays and timeouts
- Template functions: `{{uuid}}`, `{{timestamp}}`, `{{random 1 100}}`
- Conditional responses based on request data
- All HTTP methods (GET, POST, PUT, DELETE, PATCH, OPTIONS)

### SSE Features  
- Basic event streams with multiple events
- Interval-based streaming (every N seconds)
- AI-style token streaming (simulates LLM responses)
- Conditional events based on query params
- Event IDs and retry hints
- Keep-alive comments

### WebSocket Features
- Echo mode for testing
- Message pattern matching (exact, regex, JSON)
- Scenarios (multi-step conversations)
- Heartbeat/ping-pong
- Subprotocol negotiation
- Binary message support
- Broadcast simulation

### GraphQL Features
- Schema definition with types, queries, mutations
- Custom resolvers with static and dynamic data
- Subscriptions (WebSocket-based)
- GraphQL errors and partial responses
- Introspection control
- N+1 query simulation
- Pagination patterns

### gRPC Features
- Unary RPC mocking
- Server streaming
- Client streaming  
- Bidirectional streaming
- All gRPC error codes
- Metadata/header matching
- Response delays
- Server reflection (for Insomnia/grpcurl discovery)

### SOAP Features
- SOAP 1.1 and 1.2 envelopes
- Multiple operations per endpoint
- SOAP faults (client/server errors)
- XPath request matching
- WSDL generation
- Complex nested XML types

## Testing with curl

### HTTP
```bash
curl http://localhost:4280/api/users
curl http://localhost:4280/api/users/123
curl -X POST http://localhost:4280/api/users -d '{"name":"John"}'
```

### SSE
```bash
curl -N http://localhost:4280/sse/basic
curl -N http://localhost:4280/sse/ai-stream
```

### GraphQL
```bash
curl -X POST http://localhost:4280/graphql \
  -H "Content-Type: application/json" \
  -d '{"query": "{ users { id name } }"}'
```

### gRPC
```bash
grpcurl -plaintext localhost:50051 list
grpcurl -plaintext -d '{"id": "1"}' localhost:50051 demo.UserService/GetUser
```

### SOAP
```bash
curl -X POST http://localhost:4280/soap/users \
  -H "Content-Type: text/xml" \
  -d '<soap:Envelope xmlns:soap="http://schemas.xmlsoap.org/soap/envelope/">
        <soap:Body><GetUser><id>123</id></GetUser></soap:Body>
      </soap:Envelope>'
```

## Notes

- gRPC mocks use ports 50051-50063 (one per demo config)
- Proto file for gRPC: `demo.proto` (must exist for gRPC mocks to work)
- WebSocket connections: `ws://localhost:4280/ws/*`
- SSE streams require `-N` flag with curl to disable buffering
