# mockd

A high-performance, local-first HTTP/HTTPS mock server with CLI and Go API.

## Features

- **Local-First**: Zero external dependencies, works completely offline
- **High Performance**: Handles 1000+ concurrent requests with sub-2-second startup
- **CLI Tool**: Full command-line interface for managing mocks
- **Flexible Matching**: Match requests by method, path, headers, query params, and body
- **HTTPS Support**: Auto-generated self-signed certificates for secure testing
- **Admin API**: RESTful API for dynamic mock configuration
- **Stateful Mocking**: Simulate CRUD operations with persistent state
- **Proxy Recording**: Record real API traffic for replay
- **Request Logging**: Full request inspection for debugging
- **Shell Completion**: Bash, Zsh, and Fish completion support

## Installation

```bash
# Install with go install
go install github.com/getmockd/mockd/cmd/mockd@latest

# Or clone and build
git clone https://github.com/getmockd/mockd.git
cd mockd
go build -o mockd ./cmd/mockd
```

## Quick Start

```bash
# Start the mock server
mockd start

# Start with custom port and config
mockd start --port 3000 --config mocks.json

# Add a mock endpoint
mockd add --path /api/users --status 200 --body '{"users": []}'

# Add a mock with headers
mockd add -m POST --path /api/users -s 201 \
  -b '{"id": "new-user"}' \
  -H "Content-Type:application/json"

# List all mocks
mockd list

# Get mock details
mockd get <mock-id>

# Delete a mock
mockd delete <mock-id>

# Export configuration
mockd export -o mocks.json

# Import configuration
mockd import mocks.json

# View request logs
mockd logs

# Show effective configuration
mockd config

# Generate shell completion
mockd completion bash > /etc/bash_completion.d/mockd
```

## Environment Variables

Configure mockd via environment variables for CI/CD:

| Variable | Description | Default |
|----------|-------------|---------|
| `MOCKD_PORT` | HTTP server port | 8080 |
| `MOCKD_ADMIN_PORT` | Admin API port | 9090 |
| `MOCKD_ADMIN_URL` | Admin API URL for CLI | http://localhost:9090 |
| `MOCKD_CONFIG` | Path to config file | |
| `MOCKD_HTTPS_PORT` | HTTPS port (0=disabled) | 0 |
| `MOCKD_READ_TIMEOUT` | Read timeout seconds | 30 |
| `MOCKD_WRITE_TIMEOUT` | Write timeout seconds | 30 |
| `MOCKD_MAX_LOG_ENTRIES` | Max request log entries | 1000 |

## Configuration Files

mockd supports configuration files with the following precedence:

1. Command-line flags (highest priority)
2. Environment variables
3. Local config `.mockdrc.json` (current directory)
4. Global config `~/.config/mockd/config.json`
5. Default values

Example `.mockdrc.json`:
```json
{
  "port": 3000,
  "adminPort": 9090,
  "maxLogEntries": 500
}
```

## Go Library Usage

```bash
go get github.com/getmockd/mockd/pkg/engine
```

```go
package main

import (
    "log"
    "github.com/getmockd/mockd/pkg/engine"
    "github.com/getmockd/mockd/pkg/config"
)

func main() {
    // Create server configuration
    cfg := &config.ServerConfiguration{
        HTTPPort:  8080,
        AdminPort: 9090,
    }

    // Create and start the engine
    srv := engine.NewServer(cfg)

    // Add a mock
    srv.AddMock(&config.MockConfiguration{
        Name: "Get Users",
        Matcher: &config.RequestMatcher{
            Method: "GET",
            Path:   "/api/users",
        },
        Response: &config.ResponseDefinition{
            StatusCode: 200,
            Headers: map[string]string{
                "Content-Type": "application/json",
            },
            Body: `{"users": ["Alice", "Bob"]}`,
        },
    })

    if err := srv.Start(); err != nil {
        log.Fatalf("Failed to start server: %v", err)
    }
    defer srv.Stop()

    log.Println("Mock server running on :8080")
    log.Println("Admin API running on :9090")
    select {}
}
```

## Admin API

Create, update, and delete mocks at runtime:

```bash
# Create a mock
curl -X POST http://localhost:9090/mocks \
  -H "Content-Type: application/json" \
  -d '{
    "matcher": {"method": "GET", "path": "/api/health"},
    "response": {"statusCode": 200, "body": "{\"status\": \"ok\"}"}
  }'

# List all mocks
curl http://localhost:9090/mocks

# Delete a mock
curl -X DELETE http://localhost:9090/mocks/{id}
```

## Mock Configuration File

Load mocks from a JSON file:

```json
{
  "version": "1.0",
  "name": "My API Mocks",
  "mocks": [
    {
      "id": "get-users",
      "matcher": {"method": "GET", "path": "/api/users"},
      "response": {"statusCode": 200, "body": "[]"}
    }
  ]
}
```

## Documentation

The documentation site is built with MkDocs and deployed to GitHub Pages.

### Local Development

```bash
# Install mise if not already installed
# https://mise.jdx.dev/getting-started.html

# Trust the mise config and install tools
mise trust
mise install

# Install documentation dependencies
mise run docs-install

# Start local server with live reload
mise run docs-serve

# Build static site
mise run docs-build
```

The site will be available at `http://localhost:8000`.

### Documentation Structure

```
docs/
├── index.md              # Homepage
├── getting-started/      # Installation, quickstart, concepts
├── guides/               # Feature guides
├── reference/            # CLI, config, API reference
└── examples/             # Usage examples
```

### Adding New Pages

1. Create a new `.md` file in the appropriate directory
2. Add the page to `nav:` in `mkdocs.yml`
3. Update the `llmstxt` plugin sections if needed
4. Run `mkdocs serve` to preview

## Requirements

- Go 1.21+

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

## License

MIT License - see [LICENSE](LICENSE) for details.
