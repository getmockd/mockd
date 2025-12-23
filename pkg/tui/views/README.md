# TUI Views

This package contains all view implementations for the mockd TUI.

## Implemented Views

### Dashboard (`dashboard.go`)

The dashboard view provides a real-time overview of the mockd server status.

**Features:**
- Real server health status from Admin API
- Proxy status display (running/inactive, mode, port)
- Mock statistics (active/disabled counts)
- Recent traffic display (last 5 requests)
- Auto-refresh every 5 seconds
- Loading spinner while fetching data
- Error handling and display

**Key Bindings:**
- `s` - Start/stop server (placeholder)
- `r` - Toggle recording (placeholder)
- `p` - Navigate to proxy view

**Data Sources:**
- `GET /health` - Server status and uptime
- `GET /proxy/status` - Proxy configuration
- `GET /mocks` - List of all mocks
- `GET /requests?limit=10` - Recent traffic

**Implementation:**
- Uses Bubbletea's Elm architecture
- Integrates with `pkg/tui/client` for API calls
- Refreshes data every 5 seconds automatically
- Handles loading and error states gracefully

**Testing:**
- 11 unit tests covering all major functionality
- Tests for initialization, data updates, error handling
- Tests for utility functions (formatDuration, truncate)

## View Structure

Each view implements the Bubbletea model interface:

```go
type ViewModel interface {
    Init() tea.Cmd
    Update(tea.Msg) (ViewModel, tea.Cmd)
    View() string
}
```

## Data Flow

1. View initialization triggers API calls via `pkg/tui/client`
2. API responses are converted to custom messages
3. Update function handles messages and updates model state
4. View function renders the current state

## Adding New Views

To add a new view:

1. Create a new file in this directory (e.g., `myview.go`)
2. Implement the Bubbletea model interface
3. Define custom messages for your view
4. Add the view to the root model in `pkg/tui/model.go`
5. Update the view routing in `renderContent()`
6. Add tests in `myview_test.go`

## Utilities

Common utility functions used across views:

- `formatDuration(d time.Duration) string` - Format durations for display
- `truncate(s string, maxLen int) string` - Truncate long strings with ellipsis
