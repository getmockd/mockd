# Testing the TUI Admin API Client

## Unit Tests

Run the unit tests (no server required):

```bash
go test -v ./pkg/tui/client/...
```

All unit tests use `httptest` to mock the Admin API, so they don't require a running mockd server.

## Integration Tests

Integration tests require a running mockd server with Admin API on `localhost:9090`.

### Setup

1. Start mockd in one terminal:
```bash
go run ./cmd/mockd
```

2. Run integration tests in another terminal:
```bash
go test -tags=integration -v ./pkg/tui/client/...
```

### What Integration Tests Cover

- **Health Check**: Verifies server is reachable and responds correctly
- **Mock Operations**: Full CRUD cycle (create, read, update, delete, toggle)
- **Traffic Operations**: Fetching request logs with filters
- **Proxy Status**: Querying proxy state
- **Stream Recordings**: Listing and filtering stream recordings

### Running Specific Integration Tests

```bash
# Run only health check test
go test -tags=integration -v -run TestIntegration_HealthCheck ./pkg/tui/client/...

# Run only mock operations test
go test -tags=integration -v -run TestIntegration_MockOperations ./pkg/tui/client/...
```

## Manual Testing

You can also test the client manually using a simple Go program:

```go
package main

import (
	"fmt"
	"log"
	
	"github.com/getmockd/mockd/pkg/tui/client"
)

func main() {
	c := client.NewDefaultClient()
	
	// Test health
	health, err := c.GetHealth()
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Server: %s (uptime: %ds)\n", health.Status, health.Uptime)
	
	// List mocks
	mocks, err := c.ListMocks()
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Found %d mocks\n", len(mocks))
	
	// Get proxy status
	status, err := c.GetProxyStatus()
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Proxy running: %t\n", status.Running)
}
```

Save as `test_client.go` and run:
```bash
go run test_client.go
```

## Test Coverage

Check test coverage:

```bash
go test -cover ./pkg/tui/client/...
```

Generate detailed coverage report:

```bash
go test -coverprofile=coverage.out ./pkg/tui/client/...
go tool cover -html=coverage.out
```

## Troubleshooting

### "Admin API not available"

If integration tests are skipped with this message:
1. Verify mockd is running: `curl http://localhost:9090/health`
2. Check the port is correct (default is 9090)
3. Make sure no firewall is blocking the connection

### "Mock not found" errors

This usually means:
1. The mock was already deleted
2. There's a timing issue - add a small delay
3. The server was restarted between operations

### Connection refused

1. Verify Admin API port: check mockd startup logs
2. Try using a custom URL: `client.NewClient("http://localhost:CUSTOM_PORT")`
3. Check if Admin API is enabled in mockd configuration
