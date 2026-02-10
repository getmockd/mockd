# mockd Architecture

This document provides an overview of mockd's architecture for contributors and maintainers.

## High-Level Overview

mockd follows a two-server architecture separating concerns between mock serving and management:

```
                    ┌─────────────────────────────────────────────────────────────────┐
                    │                        MOCKD ARCHITECTURE                        │
                    ├─────────────────────────────────────────────────────────────────┤
                    │                                                                  │
                    │  ┌──────────────────────────────────────────────────────────┐   │
                    │  │                    Admin Server (:4290)                   │   │
                    │  │            (Public HTTP API - User-facing)                │   │
                    │  │                                                           │   │
                    │  │  Package: pkg/admin/                                      │   │
                    │  │  Role: CRUD mocks, view logs, configure engine            │   │
                    │  └──────────────────────────────────────────────────────────┘   │
                    │                              │                                   │
                    │                              │ engineclient.Client               │
                    │                              │ (HTTP client for mgmt API)        │
                    │                              ▼                                   │
                    │  ┌──────────────────────────────────────────────────────────┐   │
                    │  │                       Engine                              │   │
                    │  │                                                           │   │
                    │  │   ┌─────────────────────┐  ┌───────────────────────────┐ │   │
                    │  │   │   Mock Server       │  │   Management API          │ │   │
                    │  │   │   (:4280)           │  │   (Internal Port)         │ │   │
                    │  │   │                     │  │                           │ │   │
                    │  │   │   DATA PLANE        │  │   CONTROL PLANE           │ │   │
                    │  │   │   Serves mocks      │  │   Runtime configuration   │ │   │
                    │  │   └─────────────────────┘  └───────────────────────────┘ │   │
                    │  └──────────────────────────────────────────────────────────┘   │
                    └─────────────────────────────────────────────────────────────────┘
```

### Design Principles

- **Local-First**: All data stored locally by default (XDG-compliant directories)
- **Zero Dependencies**: Single binary, no external services required
- **Protocol Agnostic**: Unified mock type with protocol-specific specs
- **Extensible**: Clean interfaces for adding protocols, storage backends, and audit writers

## Directory Structure

```
mockd/
├── cmd/mockd/              # CLI entry point
│   └── main.go             # Command routing to pkg/cli
├── pkg/                    # Public packages
│   ├── admin/              # Admin API server (management interface)
│   │   ├── routes.go       # HTTP route registration
│   │   ├── handlers.go     # Core mock CRUD handlers
│   │   └── *_handlers.go   # Protocol-specific handlers
│   ├── engine/             # Core mock server engine
│   │   ├── server.go       # HTTP/HTTPS server lifecycle
│   │   ├── handler.go      # Request routing and response
│   │   ├── matcher.go      # Mock selection logic
│   │   └── api/            # Internal management API
│   ├── mock/               # Unified mock types
│   │   └── types.go        # Mock, HTTPSpec, WebSocketSpec, etc.
│   ├── protocol/           # Protocol handler interface
│   │   └── handler.go      # Handler interface definition
│   ├── store/              # Storage abstraction
│   │   ├── store.go        # Store interface and config
│   │   ├── interfaces.go   # MockStore, WorkspaceStore, etc.
│   │   └── file/           # File-based implementation
│   ├── cli/                # CLI command implementations
│   ├── websocket/          # WebSocket protocol support
│   ├── grpc/               # gRPC protocol support
│   ├── mqtt/               # MQTT broker support
│   ├── graphql/            # GraphQL protocol support
│   ├── soap/               # SOAP/WSDL protocol support
│   ├── sse/                # Server-Sent Events support
│   ├── audit/              # Audit logging infrastructure
│   ├── proxy/              # MITM proxy for recording
│   ├── recording/          # Traffic recording
│   ├── portability/        # Import/export formats
│   ├── oauth/              # OAuth mock provider
│   ├── chaos/              # Chaos injection
│   ├── template/           # Response templating
│   ├── tracing/            # OpenTelemetry tracing
│   ├── metrics/            # Prometheus metrics
│   └── ...
├── internal/               # Private packages
│   ├── matching/           # Request matching algorithms
│   ├── cliconfig/          # CLI configuration
│   ├── id/                 # ID generation utilities
│   └── storage/            # Internal storage helpers
├── tests/                  # Integration tests
│   ├── integration/        # Full server integration tests
│   ├── performance/        # Performance benchmarks
│   ├── fixtures/           # Test data files
│   └── unit/               # Unit test helpers
└── benchmarks/             # Performance benchmarking suite
```

## Key Concepts

### Mock Lifecycle

```
1. CREATE                    2. MATCH                      3. RESPOND
┌──────────────┐            ┌──────────────┐             ┌──────────────┐
│ Admin API    │            │ Engine       │             │ Engine       │
│ POST /mocks  │ ─────────▶ │ Receives     │ ──────────▶ │ Sends mock   │
│              │            │ HTTP request │             │ response     │
└──────────────┘            └──────────────┘             └──────────────┘
       │                           │                            │
       ▼                           ▼                            ▼
┌──────────────┐            ┌──────────────┐             ┌──────────────┐
│ Store        │            │ Matcher      │             │ Template     │
│ Persists to  │            │ Scores all   │             │ Renders      │
│ disk         │            │ mocks        │             │ response     │
└──────────────┘            └──────────────┘             └──────────────┘
```

### Unified Mock Type

All mock types share a common structure (`pkg/mock/types.go`):

```go
type Mock struct {
    ID          string    `json:"id"`
    Type        Type      `json:"type"`        // http, websocket, grpc, mqtt, etc.
    Name        string    `json:"name"`
    Enabled     bool      `json:"enabled"`
    WorkspaceID string    `json:"workspaceId"`
    
    // Type-specific specs - exactly one populated based on Type
    HTTP      *HTTPSpec      `json:"http,omitempty"`
    WebSocket *WebSocketSpec `json:"websocket,omitempty"`
    GraphQL   *GraphQLSpec   `json:"graphql,omitempty"`
    GRPC      *GRPCSpec      `json:"grpc,omitempty"`
    SOAP      *SOAPSpec      `json:"soap,omitempty"`
    MQTT      *MQTTSpec      `json:"mqtt,omitempty"`
    OAuth     *OAuthSpec     `json:"oauth,omitempty"`
}
```

### Request Matching Flow

The matcher (`internal/matching/matcher.go`) uses a scoring system:

1. **Score Calculation**: Each matching criterion adds points
2. **Required vs Optional**: Method and path must match; headers/body add score
3. **Priority Tiebreaker**: Equal scores resolved by mock priority
4. **Captures**: Regex captures from `pathPattern` available in templates

```go
// Scoring weights (internal/matching/scores.go)
ScoreMethod       = 10   // Method matches
ScorePathExact    = 15   // Exact path matches
ScorePathPattern  = 14   // Regex path matches
ScorePathNamedParams = 12 // Named parameter path matches
ScorePathWildcard = 10   // Wildcard path matches
ScoreHeader       = 10   // Each header match
ScoreQueryParam   = 5    // Each query param match
ScoreBodyEquals   = 25   // Exact body match
ScoreBodyPattern  = 22   // Body regex pattern match
ScoreBodyContains = 20   // Body contains string
ScoreBodyNoCriteria = 1  // No body criteria specified
ScoreJSONPathCondition = 15 // Per matched JSONPath condition
```

### Protocol Handler Interface

All protocol handlers implement (`pkg/protocol/handler.go`):

```go
type Handler interface {
    // Metadata returns handler identification and capabilities
    Metadata() Metadata
    
    // Lifecycle management
    Start(ctx context.Context) error
    Stop(ctx context.Context, timeout time.Duration) error
    
    // Health monitoring
    Health(ctx context.Context) HealthStatus
}
```

### Storage Abstraction

The store interface (`pkg/store/store.go`) abstracts persistence:

```go
type Store interface {
    Open(ctx context.Context) error
    Close() error
    
    Workspaces() WorkspaceStore
    Mocks() MockStore
    Folders() FolderStore
    Recordings() RecordingStore
    RequestLog() RequestLogStore
    Preferences() PreferencesStore
    
    Begin(ctx context.Context) (Transaction, error)
    Sync(ctx context.Context) error
}
```

Storage backends:
- `file`: JSON files in XDG directories (default)
- `sqlite`: Embedded SQLite database
- `memory`: In-memory for testing

## Extension Points

### Adding a New Protocol

1. **Define the spec** in `pkg/mock/types.go`:
   ```go
   type MyProtocolSpec struct {
       Port     int    `json:"port"`
       // ... protocol-specific fields
   }
   ```

2. **Add to Type constants**:
   ```go
   TypeMyProtocol Type = "myprotocol"
   ```

3. **Create handler package** at `pkg/myprotocol/`:
   - Implement `protocol.Handler` interface
   - Handle mock matching for your protocol
   - Implement recording support if applicable

4. **Register in engine** (`pkg/engine/protocol_manager.go`):
   ```go
   pm.RegisterHandler(myprotocol.NewHandler(config))
   ```

5. **Add Admin API endpoints** in `pkg/admin/`:
   - Recording management handlers
   - Status/health endpoints

### Adding a New Storage Backend

1. **Implement Store interface** in `pkg/store/`:
   ```go
   type PostgresStore struct { ... }
   
   func (s *PostgresStore) Mocks() MockStore { ... }
   // ... implement all interface methods
   ```

2. **Register backend** in store factory:
   ```go
   case BackendPostgres:
       return NewPostgresStore(cfg.ConnectionString)
   ```

### Adding Audit Logging

The audit system (`pkg/audit/`) supports custom writers:

1. **Implement Logger interface**:
   ```go
   type MyAuditWriter struct { ... }
   
   func (w *MyAuditWriter) Log(entry AuditEntry) error { ... }
   func (w *MyAuditWriter) Close() error { ... }
   ```

2. **Register via registry** (`pkg/audit/registry.go`):
   ```go
   audit.RegisterWriter("siem", func(cfg Config) (Logger, error) {
       return NewSIEMWriter(cfg), nil
   })
   ```

3. **Enable in config**:
   ```yaml
   audit:
     enabled: true
     writers:
       - type: siem
         endpoint: https://siem.example.com
   ```

## Observability

### Metrics (Prometheus)

The `pkg/metrics` package exposes Prometheus metrics at `/metrics` on the admin port:

| Metric | Type | Description |
|--------|------|-------------|
| `mockd_requests_total` | Counter | Total requests by method, path, status |
| `mockd_request_duration_seconds` | Histogram | Request latency distribution |
| `mockd_match_hits_total` | Counter | Mock match hits by mock_id |
| `mockd_match_misses_total` | Counter | Requests that didn't match any mock |

### Tracing (OpenTelemetry)

The `pkg/tracing` package provides OTLP-compatible distributed tracing:

```bash
# Enable tracing via CLI
mockd serve --otlp-endpoint http://localhost:4318/v1/traces

# With sampling (10% of traces)
mockd serve --otlp-endpoint http://localhost:4318/v1/traces --trace-sampler 0.1
```

Traces include HTTP attributes (`http.method`, `http.url`, `http.status_code`, etc.) and integrate with any OTLP-compatible backend (Jaeger, Tempo, Honeycomb, etc.).

### Structured Logging

JSON-formatted structured logs with component tags:

```bash
mockd serve --log-level debug --log-format json
```

Log output includes `component` (engine, admin) and `subcomponent` (handler) fields for filtering.

### Observability Stack

See `observability/` for Docker Compose configurations:
- `docker-compose.observability.yml` - Full stack (mockd + Prometheus + Jaeger + Grafana)
- `docker-compose.observability-local.yml` - Observability only (for local mockd development)

## Testing Strategy

### Unit Tests

Located alongside source files (`*_test.go`):

```bash
# Run all unit tests
go test ./pkg/...

# Run specific package tests
go test ./pkg/engine/...
go test ./internal/matching/...

# With coverage
go test -cover ./pkg/...
```

### Integration Tests

Located in `tests/integration/`:

```bash
# Run all integration tests
go test ./tests/integration/...

# Run specific test
go test ./tests/integration/ -run TestHTTPMocking

# With verbose output
go test -v ./tests/integration/...
```

Key integration test files:
- `http_test.go` - Basic HTTP mocking
- `https_test.go` - TLS/mTLS support
- `websocket_test.go` - WebSocket protocol
- `grpc_test.go` - gRPC protocol
- `mqtt_test.go` - MQTT broker
- `proxy_test.go` - MITM proxy recording
- `stateful_test.go` - Stateful mock behavior

### Performance Tests

Located in `tests/performance/` and `benchmarks/`:

```bash
# Run Go benchmarks
go test -bench=. ./...

# Run k6 load tests
k6 run benchmarks/k6/load_test.js

# Full benchmark suite
./benchmarks/run_all.sh
```

### Test Fixtures

Test data in `tests/fixtures/`:
- `protos/` - gRPC proto files
- `wsdl/` - SOAP WSDL files
- `certs/` - Test certificates
- `mocks/` - Sample mock configurations

## Configuration

### Default Ports

| Service | Port | Purpose |
|---------|------|---------|
| Mock Server | 4280 | Serves mock responses |
| Admin API | 4290 | Management interface |
| Engine Control | 4281 | Internal engine communication (not user-facing) |

### Data Directories (XDG)

| Platform | Config | Data | Cache |
|----------|--------|------|-------|
| Linux | `~/.config/mockd` | `~/.local/share/mockd` | `~/.cache/mockd` |
| macOS | `~/Library/Preferences/mockd` | `~/Library/Application Support/mockd` | `~/Library/Caches/mockd` |
| Windows | `%APPDATA%/mockd` | `%LOCALAPPDATA%/mockd` | `%LOCALAPPDATA%/mockd/cache` |

## Key Files Reference

| File | Purpose |
|------|---------|
| `cmd/mockd/main.go` | CLI entry point and command routing |
| `pkg/engine/server.go` | Mock server lifecycle management |
| `pkg/engine/handler.go` | HTTP request handling and mock serving |
| `pkg/engine/matcher.go` | Mock selection and scoring |
| `pkg/admin/routes.go` | Admin API route definitions |
| `pkg/mock/types.go` | Unified mock type definitions |
| `pkg/protocol/handler.go` | Protocol handler interface |
| `pkg/store/store.go` | Storage interface definitions |
| `internal/matching/matcher.go` | Request matching algorithms |

## Further Reading

- `docs/` - User documentation (Astro Starlight)
- `CONTRIBUTING.md` - Contribution guidelines
- `ROADMAP.md` - Project roadmap
