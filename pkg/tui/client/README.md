# TUI Admin API Client

This package provides a Go HTTP client for the mockd Admin API. It's designed for use by the TUI but can be used standalone.

## Features

- **Health Operations**: Check server health and status
- **Mock Operations**: CRUD operations for mock endpoints
- **Traffic Operations**: View and manage request logs
- **Recording Operations**: Manage HTTP recordings
- **Stream Recording Operations**: Manage WebSocket/SSE recordings
- **Replay Operations**: Control stream recording playback
- **Proxy Operations**: Check proxy status

## Usage

### Basic Setup

```go
import "github.com/getmockd/mockd/pkg/tui/client"

// Create client pointing to default Admin API (localhost:9090)
c := client.NewDefaultClient()

// Or specify custom URL
c := client.NewClient("http://localhost:9090")
```

### Health Check

```go
health, err := c.GetHealth()
if err != nil {
    log.Fatal(err)
}
fmt.Printf("Status: %s, Uptime: %d seconds\n", health.Status, health.Uptime)
```

### Mock Operations

```go
// List all mocks
mocks, err := c.ListMocks()

// List only enabled mocks
mocks, err := c.ListMocksFiltered(true)

// Get specific mock
mock, err := c.GetMock("mock-id")

// Create new mock
newMock := &config.MockConfiguration{
    Matcher: &config.RequestMatcher{
        Method: "GET",
        Path:   "/api/users",
    },
    Response: &config.ResponseDefinition{
        Status: 200,
        Body:   `{"users": []}`,
    },
    Enabled: true,
}
created, err := c.CreateMock(newMock)

// Update mock
updated, err := c.UpdateMock("mock-id", mock)

// Toggle mock enabled/disabled
toggled, err := c.ToggleMock("mock-id", true)

// Delete mock
err := c.DeleteMock("mock-id")
```

### Traffic Operations

```go
// Get all request logs
entries, err := c.GetTraffic(nil)

// Get filtered request logs
filter := &client.RequestLogFilter{
    Method: "GET",
    Path:   "/api",
    Limit:  100,
}
entries, err := c.GetTraffic(filter)

// Get specific request
entry, err := c.GetRequest("request-id")

// Clear all traffic
err := c.ClearTraffic()
```

### HTTP Recording Operations

```go
// List recordings
recordings, err := c.ListRecordings(nil)

// List with filter
filter := &client.RecordingFilter{
    SessionID: "session-123",
    Method:    "POST",
    Limit:     50,
}
recordings, err := c.ListRecordings(filter)

// Get specific recording
rec, err := c.GetRecording("recording-id")

// Delete recording
err := c.DeleteRecording("recording-id")

// Export recordings
data, err := c.ExportRecording("session-id", []string{"rec-1", "rec-2"})

// Convert recordings to mocks
mockIDs, err := c.ConvertRecording("session-id", nil, true, true)
```

### Stream Recording Operations

```go
// List stream recordings (WebSocket/SSE)
recordings, err := c.ListStreamRecordings(nil)

// List with filter
filter := &client.StreamRecordingFilter{
    Protocol: "websocket",
    Limit:    100,
}
recordings, err := c.ListStreamRecordings(filter)

// Get specific stream recording
rec, err := c.GetStreamRecording("recording-id")

// Delete stream recording
err := c.DeleteStreamRecording("recording-id")

// Export stream recording
data, err := c.ExportStreamRecording("recording-id")

// Convert stream recording to mock
opts := &admin.ConvertRecordingRequest{
    SimplifyTiming:        ptrBool(true),
    MinDelay:              100,
    MaxDelay:              5000,
    IncludeClientMessages: ptrBool(false),
}
result, err := c.ConvertStreamRecording("recording-id", opts)
```

### Replay Operations

```go
// Start replay in pure mode
sessionID, err := c.StartReplay("recording-id", "pure", 1.0, false, 0)

// Start replay in synchronized mode
sessionID, err := c.StartReplay("recording-id", "synchronized", 1.0, true, 30000)

// Get replay status
status, err := c.GetReplayStatus(sessionID)
fmt.Printf("Frame %d/%d\n", status.CurrentFrame, status.TotalFrames)

// List all replay sessions
sessions, err := c.ListReplaySessions()

// Stop replay
err := c.StopReplay(sessionID)
```

### Proxy Operations

```go
// Get proxy status
status, err := c.GetProxyStatus()
if status.Running {
    fmt.Printf("Proxy running on port %d in %s mode\n", status.Port, status.Mode)
}
```

### Error Handling

The client returns typed errors from the Admin API:

```go
mock, err := c.GetMock("nonexistent")
if err != nil {
    // Error format: "error_code: error message"
    // e.g., "not_found: Mock not found"
    fmt.Println(err)
}
```

## Configuration

The client uses a 10-second timeout by default. You can customize the underlying HTTP client if needed:

```go
c := client.NewDefaultClient()
c.httpClient.Timeout = 30 * time.Second
```

## Testing

Run the test suite:

```bash
go test -v ./pkg/tui/client/...
```

The tests use `httptest` to mock the Admin API responses.
