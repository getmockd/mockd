package views

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/getmockd/mockd/pkg/config"
	"github.com/getmockd/mockd/pkg/tui/client"
	"github.com/getmockd/mockd/pkg/tui/styles"
)

const (
	maxRequests        = 100
	trafficRefreshRate = 1 * time.Second
	pausedRefreshRate  = 10 * time.Second // Still check for updates when paused, but less frequently
)

// TrafficModel represents the traffic view state.
type TrafficModel struct {
	client  *client.Client
	width   int
	height  int
	loading bool
	err     error

	// Request log data
	requests      []*config.RequestLogEntry
	lastRequestID string // Track the last request ID to detect new ones

	// Table component
	table table.Model

	// Detail view
	detailOpen      bool
	detailViewport  viewport.Model
	selectedRequest *config.RequestLogEntry

	// Filter
	filterActive bool
	filterInput  textinput.Model
	filterText   string

	// State
	paused        bool
	lastRefresh   time.Time
	statusMessage string
}

// NewTraffic creates a new traffic model.
func NewTraffic(adminClient *client.Client) TrafficModel {
	// Create table
	columns := []table.Column{
		{Title: "Time", Width: 10},
		{Title: "Method", Width: 7},
		{Title: "Path", Width: 35},
		{Title: "Status", Width: 6},
		{Title: "Duration", Width: 10},
		{Title: "Mock", Width: 20},
	}

	t := table.New(
		table.WithColumns(columns),
		table.WithFocused(true),
		table.WithHeight(10),
	)

	s := table.DefaultStyles()
	s.Header = s.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(styles.ColorBorder).
		BorderBottom(true).
		Foreground(styles.ColorPrimary).
		Bold(true)
	s.Selected = s.Selected.
		Foreground(styles.ColorPrimary).
		Background(lipgloss.Color("#3E4451")).
		Bold(true)
	t.SetStyles(s)

	// Create filter input
	filterInput := textinput.New()
	filterInput.Placeholder = "Filter by method, path, or status..."
	filterInput.CharLimit = 100
	filterInput.Width = 50

	// Create detail viewport
	detailViewport := viewport.New(80, 20)

	return TrafficModel{
		client:         adminClient,
		loading:        true,
		table:          t,
		filterInput:    filterInput,
		detailViewport: detailViewport,
		requests:       make([]*config.RequestLogEntry, 0, maxRequests),
	}
}

// Init initializes the traffic view (Bubbletea lifecycle).
func (m TrafficModel) Init() tea.Cmd {
	return tea.Batch(
		m.fetchTrafficData(),
		m.tickRefresh(),
	)
}

// Update handles messages (Bubbletea lifecycle).
func (m TrafficModel) Update(msg tea.Msg) (TrafficModel, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.updateTableSize()
		return m, nil

	case trafficDataMsg:
		m.loading = false
		m.err = nil

		// Update requests
		if len(msg.requests) > 0 {
			// Check if we have new requests
			if len(m.requests) == 0 || msg.requests[0].ID != m.lastRequestID {
				// Merge new requests with existing ones
				m.requests = m.mergeRequests(msg.requests)

				// Update last request ID
				if len(m.requests) > 0 {
					m.lastRequestID = m.requests[0].ID
				}

				// Update table rows
				m.updateTableRows()
			}
		}

		m.lastRefresh = time.Now()
		return m, nil

	case trafficRefreshTickMsg:
		// Auto-refresh (unless paused)
		if !m.paused {
			cmds = append(cmds, m.fetchTrafficData())
		}
		cmds = append(cmds, m.tickRefresh())
		return m, tea.Batch(cmds...)

	case trafficErrMsg:
		m.loading = false
		m.err = msg.err
		return m, nil

	case tea.KeyMsg:
		// Handle detail view key events first
		if m.detailOpen {
			switch msg.String() {
			case "esc", "enter", "q":
				m.detailOpen = false
				return m, nil
			default:
				// Let viewport handle scrolling
				m.detailViewport, cmd = m.detailViewport.Update(msg)
				return m, cmd
			}
		}

		// Handle filter input events
		if m.filterActive {
			switch msg.String() {
			case "enter", "esc":
				m.filterActive = false
				m.filterInput.Blur()
				if msg.String() == "enter" {
					m.filterText = m.filterInput.Value()
					m.updateTableRows()
				}
				return m, nil
			default:
				m.filterInput, cmd = m.filterInput.Update(msg)
				return m, cmd
			}
		}

		// Handle main view key events
		return m.handleKey(msg)
	}

	// Update table (for navigation)
	if !m.filterActive && !m.detailOpen {
		m.table, cmd = m.table.Update(msg)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

// handleKey processes keyboard input for traffic-specific actions.
func (m TrafficModel) handleKey(msg tea.KeyMsg) (TrafficModel, tea.Cmd) {
	switch msg.String() {
	case "p":
		// Toggle pause/resume
		m.paused = !m.paused
		if m.paused {
			m.statusMessage = "Traffic paused"
		} else {
			m.statusMessage = "Traffic resumed"
			// Fetch immediately when resuming
			return m, m.fetchTrafficData()
		}
		return m, nil

	case "c":
		// Clear all traffic
		return m, m.clearTraffic()

	case "/":
		// Activate filter
		m.filterActive = true
		m.filterInput.Focus()
		return m, textinput.Blink

	case "enter":
		// View request details
		cursor := m.table.Cursor()
		if cursor >= 0 && cursor < len(m.requests) {
			// Apply filter to get the actual request
			filtered := m.filterRequests(m.requests)
			if cursor < len(filtered) {
				m.selectedRequest = filtered[cursor]
				m.detailOpen = true
				m.renderRequestDetail()
			}
		}
		return m, nil
	}

	return m, nil
}

// View renders the traffic view (Bubbletea lifecycle).
func (m TrafficModel) View() string {
	if m.detailOpen {
		return m.renderDetailOverlay()
	}

	if m.loading && len(m.requests) == 0 {
		return m.renderLoading()
	}

	return m.renderTrafficView()
}

// renderLoading shows a loading message.
func (m TrafficModel) renderLoading() string {
	return "\n  Loading traffic data...\n"
}

// renderTrafficView renders the main traffic view.
func (m TrafficModel) renderTrafficView() string {
	sections := []string{}

	// Title
	titleStyle := styles.TitleStyle
	title := titleStyle.Render("Traffic Log")

	// Status info
	statusParts := []string{}
	if m.paused {
		statusParts = append(statusParts, lipgloss.NewStyle().Foreground(styles.ColorWarning).Render("PAUSED"))
	} else {
		statusParts = append(statusParts, lipgloss.NewStyle().Foreground(styles.ColorSuccess).Render("LIVE"))
	}

	statusParts = append(statusParts, fmt.Sprintf("%d requests", len(m.requests)))

	if m.filterText != "" {
		filtered := m.filterRequests(m.requests)
		statusParts = append(statusParts, fmt.Sprintf("(%d filtered)", len(filtered)))
	}

	statusInfo := lipgloss.NewStyle().Foreground(styles.ColorMuted).Render(strings.Join(statusParts, " • "))

	header := lipgloss.JoinHorizontal(lipgloss.Top, title, "  ", statusInfo)
	sections = append(sections, header)

	separator := lipgloss.NewStyle().Foreground(styles.ColorMuted).Render(strings.Repeat("─", m.width-4))
	sections = append(sections, separator)

	// Filter input (if active)
	if m.filterActive {
		filterLabel := lipgloss.NewStyle().Foreground(styles.ColorInfo).Render("Filter: ")
		sections = append(sections, filterLabel+m.filterInput.View())
	} else if m.filterText != "" {
		filterLabel := lipgloss.NewStyle().Foreground(styles.ColorMuted).Render("Filter: ")
		filterValue := lipgloss.NewStyle().Foreground(styles.ColorInfo).Render(m.filterText)
		clearHint := lipgloss.NewStyle().Foreground(styles.ColorMuted).Render(" (/ to change)")
		sections = append(sections, filterLabel+filterValue+clearHint)
	}

	// Table
	sections = append(sections, m.table.View())

	// Error message
	if m.err != nil {
		errorStyle := lipgloss.NewStyle().Foreground(styles.ColorError)
		sections = append(sections, "", errorStyle.Render(fmt.Sprintf("Error: %s", m.err.Error())))
	}

	// Status message
	if m.statusMessage != "" {
		msgStyle := lipgloss.NewStyle().Foreground(styles.ColorInfo).Italic(true)
		sections = append(sections, "", msgStyle.Render(m.statusMessage))
	}

	// Help
	helpStyle := lipgloss.NewStyle().Foreground(styles.ColorMuted)
	helpText := "[↑/↓] Navigate • [Enter] Details • [p] Pause/Resume • [c] Clear • [/] Filter"
	sections = append(sections, "", helpStyle.Render(helpText))

	return lipgloss.JoinVertical(lipgloss.Left, sections...)
}

// renderDetailOverlay renders the request detail overlay.
func (m TrafficModel) renderDetailOverlay() string {
	if m.selectedRequest == nil {
		return "No request selected"
	}

	// Render the detail content in the viewport
	content := m.detailViewport.View()

	// Wrap in a modal style
	modal := styles.ModalStyle.
		Width(m.width - 10).
		Height(m.height - 6).
		Render(content)

	// Title
	title := styles.ModalTitleStyle.
		Width(m.width - 10).
		Render("Request Details")

	// Help
	helpStyle := lipgloss.NewStyle().Foreground(styles.ColorMuted).Align(lipgloss.Center)
	help := helpStyle.Width(m.width - 10).Render("[↑/↓] Scroll • [Esc/Enter] Close")

	return lipgloss.JoinVertical(lipgloss.Center, title, modal, help)
}

// renderRequestDetail renders the selected request details into the viewport.
func (m *TrafficModel) renderRequestDetail() {
	if m.selectedRequest == nil {
		m.detailViewport.SetContent("No request selected")
		return
	}

	req := m.selectedRequest
	sections := []string{}

	// Request info
	infoStyle := lipgloss.NewStyle().Foreground(styles.ColorPrimary).Bold(true)
	sections = append(sections, infoStyle.Render("Request"))
	sections = append(sections, fmt.Sprintf("Time:     %s", req.Timestamp.Format("2006-01-02 15:04:05.000")))
	sections = append(sections, fmt.Sprintf("Method:   %s", req.Method))
	sections = append(sections, fmt.Sprintf("Path:     %s", req.Path))
	if req.QueryString != "" {
		sections = append(sections, fmt.Sprintf("Query:    %s", req.QueryString))
	}
	sections = append(sections, fmt.Sprintf("Client:   %s", req.RemoteAddr))
	sections = append(sections, "")

	// Request headers
	sections = append(sections, infoStyle.Render("Request Headers"))
	if len(req.Headers) > 0 {
		for name, values := range req.Headers {
			for _, value := range values {
				sections = append(sections, fmt.Sprintf("%s: %s", name, value))
			}
		}
	} else {
		sections = append(sections, lipgloss.NewStyle().Foreground(styles.ColorMuted).Render("No headers"))
	}
	sections = append(sections, "")

	// Request body
	sections = append(sections, infoStyle.Render("Request Body"))
	if req.Body != "" {
		// Try to format as JSON
		formatted := m.formatJSON(req.Body)
		sections = append(sections, formatted)
		if req.BodySize > len(req.Body) {
			sections = append(sections, lipgloss.NewStyle().Foreground(styles.ColorWarning).Render(fmt.Sprintf("(truncated, original size: %d bytes)", req.BodySize)))
		}
	} else {
		sections = append(sections, lipgloss.NewStyle().Foreground(styles.ColorMuted).Render("Empty body"))
	}
	sections = append(sections, "")

	// Response info
	sections = append(sections, infoStyle.Render("Response"))
	statusColor := m.getStatusColor(req.ResponseStatus)
	statusStyle := lipgloss.NewStyle().Foreground(statusColor).Bold(true)
	sections = append(sections, fmt.Sprintf("Status:   %s", statusStyle.Render(fmt.Sprintf("%d", req.ResponseStatus))))
	sections = append(sections, fmt.Sprintf("Duration: %s", formatDuration(time.Duration(req.DurationMs)*time.Millisecond)))

	if req.MatchedMockID != "" {
		sections = append(sections, fmt.Sprintf("Mock:     %s", req.MatchedMockID))
	} else {
		sections = append(sections, lipgloss.NewStyle().Foreground(styles.ColorMuted).Render("Mock:     No match"))
	}

	content := lipgloss.JoinVertical(lipgloss.Left, sections...)
	m.detailViewport.SetContent(content)
}

// formatJSON attempts to format a string as pretty-printed JSON.
func (m TrafficModel) formatJSON(s string) string {
	var obj interface{}
	if err := json.Unmarshal([]byte(s), &obj); err != nil {
		// Not JSON, return as-is
		return s
	}

	formatted, err := json.MarshalIndent(obj, "", "  ")
	if err != nil {
		return s
	}

	return string(formatted)
}

// getStatusColor returns the appropriate color for a status code.
func (m TrafficModel) getStatusColor(status int) lipgloss.Color {
	if status >= 200 && status < 300 {
		return styles.ColorSuccess
	} else if status >= 300 && status < 400 {
		return styles.ColorWarning
	} else if status >= 400 {
		return styles.ColorError
	}
	return styles.ColorForeground
}

// SetSize updates the traffic view dimensions.
func (m *TrafficModel) SetSize(width, height int) {
	m.width = width
	m.height = height
	m.updateTableSize()
	m.detailViewport.Width = width - 14
	m.detailViewport.Height = height - 12
}

// updateTableSize updates the table dimensions based on current size.
func (m *TrafficModel) updateTableSize() {
	if m.height > 0 {
		// Reserve space for title, separator, filter, help, and padding
		tableHeight := m.height - 10
		if m.filterActive || m.filterText != "" {
			tableHeight -= 1
		}
		if tableHeight < 5 {
			tableHeight = 5
		}
		m.table.SetHeight(tableHeight)
	}

	if m.width > 0 {
		// Update column widths to fit
		availableWidth := m.width - 6 // Account for borders and padding

		// Fixed widths
		timeWidth := 10
		methodWidth := 7
		statusWidth := 6
		durationWidth := 10
		mockWidth := 20

		// Remaining width for path
		pathWidth := availableWidth - timeWidth - methodWidth - statusWidth - durationWidth - mockWidth - 10
		if pathWidth < 20 {
			pathWidth = 20
		}

		columns := []table.Column{
			{Title: "Time", Width: timeWidth},
			{Title: "Method", Width: methodWidth},
			{Title: "Path", Width: pathWidth},
			{Title: "Status", Width: statusWidth},
			{Title: "Duration", Width: durationWidth},
			{Title: "Mock", Width: mockWidth},
		}
		m.table.SetColumns(columns)
	}
}

// updateTableRows updates the table rows from the current requests.
func (m *TrafficModel) updateTableRows() {
	// Apply filter
	filtered := m.filterRequests(m.requests)

	// Convert to table rows
	rows := make([]table.Row, 0, len(filtered))
	for _, req := range filtered {
		// Format time
		timeStr := req.Timestamp.Format("15:04:05")

		// Format status with color (we'll use the value as-is, color is handled by rendering)
		statusStr := fmt.Sprintf("%d", req.ResponseStatus)

		// Format duration
		durationStr := formatDuration(time.Duration(req.DurationMs) * time.Millisecond)

		// Format mock ID
		mockStr := req.MatchedMockID
		if mockStr == "" {
			mockStr = "-"
		} else {
			mockStr = truncate(mockStr, 20)
		}

		// Truncate path to fit
		pathStr := truncate(req.Path, 35)

		row := table.Row{
			timeStr,
			req.Method,
			pathStr,
			statusStr,
			durationStr,
			mockStr,
		}
		rows = append(rows, row)
	}

	m.table.SetRows(rows)
}

// filterRequests filters requests based on the current filter text.
func (m TrafficModel) filterRequests(requests []*config.RequestLogEntry) []*config.RequestLogEntry {
	if m.filterText == "" {
		return requests
	}

	filterLower := strings.ToLower(m.filterText)
	filtered := make([]*config.RequestLogEntry, 0)

	for _, req := range requests {
		// Check method
		if strings.Contains(strings.ToLower(req.Method), filterLower) {
			filtered = append(filtered, req)
			continue
		}

		// Check path
		if strings.Contains(strings.ToLower(req.Path), filterLower) {
			filtered = append(filtered, req)
			continue
		}

		// Check status
		statusStr := fmt.Sprintf("%d", req.ResponseStatus)
		if strings.Contains(statusStr, filterLower) {
			filtered = append(filtered, req)
			continue
		}
	}

	return filtered
}

// mergeRequests merges new requests with existing ones, maintaining order and limit.
func (m TrafficModel) mergeRequests(newRequests []*config.RequestLogEntry) []*config.RequestLogEntry {
	// Create a map of existing request IDs for deduplication
	existingIDs := make(map[string]bool)
	for _, req := range m.requests {
		existingIDs[req.ID] = true
	}

	// Add new requests that don't exist yet
	merged := make([]*config.RequestLogEntry, 0, maxRequests)

	// Add new requests first (they're newer)
	for _, req := range newRequests {
		if !existingIDs[req.ID] {
			merged = append(merged, req)
		}
	}

	// Add existing requests
	merged = append(merged, m.requests...)

	// Limit to max requests
	if len(merged) > maxRequests {
		merged = merged[:maxRequests]
	}

	return merged
}

// fetchTrafficData fetches traffic data from the API.
func (m TrafficModel) fetchTrafficData() tea.Cmd {
	return func() tea.Msg {
		// Fetch recent traffic
		traffic, err := m.client.GetTraffic(&client.RequestLogFilter{
			Limit: maxRequests,
		})
		if err != nil {
			return trafficErrMsg{err: fmt.Errorf("fetch traffic: %w", err)}
		}

		return trafficDataMsg{
			requests: traffic,
		}
	}
}

// clearTraffic clears all traffic logs.
func (m TrafficModel) clearTraffic() tea.Cmd {
	return func() tea.Msg {
		if err := m.client.ClearTraffic(); err != nil {
			return trafficErrMsg{err: fmt.Errorf("clear traffic: %w", err)}
		}

		// Clear local state
		return trafficClearedMsg{}
	}
}

// tickRefresh returns a command that schedules the next refresh.
func (m TrafficModel) tickRefresh() tea.Cmd {
	refreshRate := trafficRefreshRate
	if m.paused {
		refreshRate = pausedRefreshRate
	}

	return tea.Tick(refreshRate, func(t time.Time) tea.Msg {
		return trafficRefreshTickMsg(t)
	})
}

// Messages

// trafficDataMsg contains traffic data.
type trafficDataMsg struct {
	requests []*config.RequestLogEntry
}

// trafficRefreshTickMsg signals it's time to refresh traffic data.
type trafficRefreshTickMsg time.Time

// trafficErrMsg wraps errors from async operations.
type trafficErrMsg struct {
	err error
}

// trafficClearedMsg indicates traffic was cleared successfully.
type trafficClearedMsg struct{}
