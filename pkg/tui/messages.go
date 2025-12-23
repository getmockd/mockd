package tui

import "time"

// Message types for the Bubbletea update loop

// errMsg wraps errors from async operations
type errMsg struct {
	err error
}

func (e errMsg) Error() string {
	return e.err.Error()
}

// tickMsg is sent on a timer for periodic updates
type tickMsg time.Time

// windowSizeMsg is sent when the terminal is resized
type windowSizeMsg struct {
	width  int
	height int
}

// viewState represents the current active view
type viewState int

const (
	dashboardView viewState = iota
	mocksView
	recordingsView
	streamsView
	trafficView
	connectionsView
	logsView
)

// viewSwitchMsg signals a view change
type viewSwitchMsg struct {
	view viewState
}

// statusMsg displays a temporary status message
type statusMsg struct {
	message string
	isError bool
}

// Data loaded messages

// mocksLoadedMsg contains loaded mock data
type mocksLoadedMsg struct {
	mocks []MockSummary
}

// recordingsLoadedMsg contains loaded recording data
type recordingsLoadedMsg struct {
	recordings []RecordingSummary
}

// streamsLoadedMsg contains loaded stream recording data
type streamsLoadedMsg struct {
	streams []StreamRecordingSummary
}

// trafficLoadedMsg contains loaded traffic/request log data
type trafficLoadedMsg struct {
	entries []RequestLogEntry
}

// connectionsLoadedMsg contains active connections data
type connectionsLoadedMsg struct {
	connections []ConnectionInfo
}

// statusLoadedMsg contains server status data
type statusLoadedMsg struct {
	serverRunning bool
	serverPort    int
	adminPort     int
	proxyActive   bool
	recording     bool
}

// Action result messages

// mockCreatedMsg signals successful mock creation
type mockCreatedMsg struct {
	id string
}

// mockUpdatedMsg signals successful mock update
type mockUpdatedMsg struct {
	id string
}

// mockDeletedMsg signals successful mock deletion
type mockDeletedMsg struct {
	id string
}

// mockToggledMsg signals successful mock enable/disable toggle
type mockToggledMsg struct {
	id      string
	enabled bool
}

// Temporary data structures (will be replaced with actual types from pkg/admin)

// MockSummary represents a mock endpoint
type MockSummary struct {
	ID          string
	Name        string
	Method      string
	Path        string
	Status      int
	Enabled     bool
	Priority    int
	Hits        int
	Description string
}

// RecordingSummary represents a recorded HTTP session
type RecordingSummary struct {
	ID        string
	Method    string
	Path      string
	Status    int
	Timestamp time.Time
	Duration  time.Duration
}

// StreamRecordingSummary represents a WebSocket/SSE recording
type StreamRecordingSummary struct {
	ID        string
	Protocol  string // "ws" or "sse"
	Path      string
	Frames    int
	Duration  time.Duration
	Size      int64
	Timestamp time.Time
}

// RequestLogEntry represents a single request in the traffic log
type RequestLogEntry struct {
	ID        string
	Timestamp time.Time
	Method    string
	Path      string
	Status    int
	Duration  time.Duration
	MockID    string
	MockName  string
}

// ConnectionInfo represents an active WebSocket or SSE connection
type ConnectionInfo struct {
	ID         string
	Type       string // "ws" or "sse"
	Path       string
	Duration   time.Duration
	Messages   int
	Recording  bool
	RemoteAddr string
}
