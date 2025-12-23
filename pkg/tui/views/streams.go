package views

import (
	"fmt"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/getmockd/mockd/pkg/recording"
	"github.com/getmockd/mockd/pkg/tui/client"
	"github.com/getmockd/mockd/pkg/tui/styles"
)

// StreamsModel represents the stream recordings view state.
type StreamsModel struct {
	client  *client.Client
	width   int
	height  int
	loading bool
	err     error

	// Data
	recordings        []*recording.RecordingSummary
	selectedRecording *recording.StreamRecording

	// Filters
	protocolFilter string // "", "websocket", "sse"

	// UI state
	table         table.Model
	spinner       spinner.Model
	lastRefresh   time.Time
	statusMessage string
	cursor        int

	// Modal state
	showReplayModal bool
	replayMode      string // "pure", "synchronized", "triggered"
	replayScale     float64
}

// NewStreams creates a new streams model.
func NewStreams(adminClient *client.Client) StreamsModel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(styles.ColorPrimary)

	columns := []table.Column{
		{Title: "ID", Width: 12},
		{Title: "Protocol", Width: 10},
		{Title: "Path", Width: 30},
		{Title: "Frames", Width: 8},
		{Title: "Duration", Width: 10},
		{Title: "Size", Width: 10},
		{Title: "Recorded", Width: 16},
	}

	t := table.New(
		table.WithColumns(columns),
		table.WithFocused(true),
		table.WithHeight(10),
	)

	tableStyle := table.DefaultStyles()
	tableStyle.Header = tableStyle.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(styles.ColorBorder).
		BorderBottom(true).
		Bold(true).
		Foreground(styles.ColorPrimary)
	tableStyle.Selected = tableStyle.Selected.
		Foreground(styles.ColorPrimary).
		Background(lipgloss.Color("#3E4451")).
		Bold(true)
	t.SetStyles(tableStyle)

	return StreamsModel{
		client:      adminClient,
		loading:     true,
		spinner:     s,
		table:       t,
		replayMode:  "pure",
		replayScale: 1.0,
	}
}

// Init initializes the streams view (Bubbletea lifecycle).
func (m StreamsModel) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
		m.fetchStreamRecordings(),
		m.tickRefresh(),
	)
}

// Update handles messages (Bubbletea lifecycle).
func (m StreamsModel) Update(msg tea.Msg) (StreamsModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.table.SetHeight(msg.Height - 10)
		return m, nil

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case streamRecordingsLoadedMsg:
		m.loading = false
		m.recordings = msg.recordings
		m.lastRefresh = time.Now()
		m.updateTable()
		return m, nil

	case refreshTickMsg:
		// Auto-refresh every 5 seconds
		return m, tea.Batch(
			m.fetchStreamRecordings(),
			m.tickRefresh(),
		)

	case errMsg:
		m.loading = false
		m.err = msg.err
		return m, nil

	case streamRecordingDetailMsg:
		m.selectedRecording = msg.recording
		return m, nil

	case statusMsg:
		m.statusMessage = msg.message
		// Clear status after 3 seconds
		return m, tea.Tick(3*time.Second, func(t time.Time) tea.Msg {
			return clearStatusMsg{}
		})

	case clearStatusMsg:
		m.statusMessage = ""
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)
	}

	// Update table
	var cmd tea.Cmd
	m.table, cmd = m.table.Update(msg)
	return m, cmd
}

// handleKey processes keyboard input for streams-specific actions.
func (m StreamsModel) handleKey(msg tea.KeyMsg) (StreamsModel, tea.Cmd) {
	// If replay modal is open, handle modal keys
	if m.showReplayModal {
		return m.handleReplayModalKey(msg)
	}

	switch msg.String() {
	case "r":
		// Show replay modal
		if len(m.recordings) > 0 {
			m.showReplayModal = true
			m.statusMessage = "Select replay mode"
		}
		return m, nil

	case "x":
		// Export recording
		if len(m.recordings) > 0 {
			return m, m.exportRecording()
		}
		return m, nil

	case "c":
		// Convert to mock
		if len(m.recordings) > 0 {
			return m, m.convertRecording()
		}
		return m, nil

	case "d":
		// Delete recording
		if len(m.recordings) > 0 {
			return m, m.deleteRecording()
		}
		return m, nil

	case "1":
		// Filter: All
		m.protocolFilter = ""
		m.statusMessage = "Showing all recordings"
		return m, m.fetchStreamRecordings()

	case "2":
		// Filter: WebSocket
		m.protocolFilter = "websocket"
		m.statusMessage = "Showing WebSocket recordings"
		return m, m.fetchStreamRecordings()

	case "3":
		// Filter: SSE
		m.protocolFilter = "sse"
		m.statusMessage = "Showing SSE recordings"
		return m, m.fetchStreamRecordings()

	case "enter":
		// View details
		if len(m.recordings) > 0 {
			return m, m.fetchRecordingDetail()
		}
		return m, nil
	}

	return m, nil
}

// handleReplayModalKey handles keyboard input when replay modal is open.
func (m StreamsModel) handleReplayModalKey(msg tea.KeyMsg) (StreamsModel, tea.Cmd) {
	switch msg.String() {
	case "esc", "q":
		m.showReplayModal = false
		m.statusMessage = ""
		return m, nil

	case "1":
		m.replayMode = "pure"
		return m, nil

	case "2":
		m.replayMode = "synchronized"
		return m, nil

	case "3":
		m.replayMode = "triggered"
		return m, nil

	case "enter":
		// Start replay with selected mode
		m.showReplayModal = false
		return m, m.startReplay()

	case "+", "=":
		// Increase timing scale
		m.replayScale = min(10.0, m.replayScale+0.1)
		return m, nil

	case "-":
		// Decrease timing scale
		m.replayScale = max(0.1, m.replayScale-0.1)
		return m, nil
	}

	return m, nil
}

// View renders the streams view (Bubbletea lifecycle).
func (m StreamsModel) View() string {
	if m.loading && len(m.recordings) == 0 {
		return m.renderLoading()
	}

	if m.err != nil {
		return m.renderError()
	}

	// If replay modal is open, render it as overlay
	if m.showReplayModal {
		return m.renderStreamsView() + "\n" + m.renderReplayModal()
	}

	return m.renderStreamsView()
}

// renderLoading shows a loading spinner.
func (m StreamsModel) renderLoading() string {
	return fmt.Sprintf("\n  %s Loading stream recordings...\n", m.spinner.View())
}

// renderError shows an error message.
func (m StreamsModel) renderError() string {
	errorStyle := lipgloss.NewStyle().
		Foreground(styles.ColorError).
		Bold(true)

	return fmt.Sprintf("\n  %s\n\n  %s\n",
		errorStyle.Render("Error loading stream recordings:"),
		m.err.Error(),
	)
}

// renderStreamsView renders the full streams view.
func (m StreamsModel) renderStreamsView() string {
	sections := []string{
		m.renderHeader(),
		m.renderTable(),
	}

	// Only show details if we have room
	if m.selectedRecording != nil && len(m.recordings) < 5 {
		sections = append(sections, "", m.renderDetail())
	}

	if m.statusMessage != "" {
		sections = append(sections, "", m.renderStatusMessage())
	}

	return lipgloss.JoinVertical(lipgloss.Left, sections...)
}

// renderHeader renders the header with filter tabs.
func (m StreamsModel) renderHeader() string {
	titleStyle := styles.TitleStyle
	title := titleStyle.Render(fmt.Sprintf("Stream Recordings (%d)", len(m.recordings)))

	// Protocol filter tabs
	allStyle := lipgloss.NewStyle().Padding(0, 1)
	wsStyle := lipgloss.NewStyle().Padding(0, 1)
	sseStyle := lipgloss.NewStyle().Padding(0, 1)

	if m.protocolFilter == "" {
		allStyle = allStyle.Foreground(styles.ColorPrimary).Bold(true)
	} else {
		allStyle = allStyle.Foreground(styles.ColorMuted)
	}

	if m.protocolFilter == "websocket" {
		wsStyle = wsStyle.Foreground(styles.ColorPrimary).Bold(true)
	} else {
		wsStyle = wsStyle.Foreground(styles.ColorMuted)
	}

	if m.protocolFilter == "sse" {
		sseStyle = sseStyle.Foreground(styles.ColorPrimary).Bold(true)
	} else {
		sseStyle = sseStyle.Foreground(styles.ColorMuted)
	}

	tabs := lipgloss.JoinHorizontal(
		lipgloss.Top,
		allStyle.Render("[1] All"),
		wsStyle.Render("[2] WebSocket"),
		sseStyle.Render("[3] SSE"),
	)

	return lipgloss.JoinVertical(lipgloss.Left, title, tabs)
}

// renderTable renders the recordings table.
func (m StreamsModel) renderTable() string {
	return m.table.View()
}

// renderDetail renders the selected recording details.
func (m StreamsModel) renderDetail() string {
	if m.selectedRecording == nil {
		return ""
	}

	titleStyle := styles.TitleStyle
	title := titleStyle.Render("Recording Details")

	separator := lipgloss.NewStyle().Foreground(styles.ColorMuted).Render("─────────────────")

	lines := []string{
		title,
		separator,
		fmt.Sprintf("ID: %s", m.selectedRecording.ID),
		fmt.Sprintf("Protocol: %s", m.selectedRecording.Protocol),
		fmt.Sprintf("Path: %s", m.selectedRecording.Metadata.Path),
		fmt.Sprintf("Frames: %d", m.selectedRecording.Stats.FrameCount),
		fmt.Sprintf("Duration: %s", formatDuration(time.Duration(m.selectedRecording.Duration)*time.Millisecond)),
		fmt.Sprintf("Status: %s", m.selectedRecording.Status),
	}

	if !m.selectedRecording.StartTime.IsZero() {
		lines = append(lines, fmt.Sprintf("Recorded: %s", m.selectedRecording.StartTime.Format("2006-01-02 15:04:05")))
	}

	return lipgloss.JoinVertical(lipgloss.Left, lines...)
}

// renderReplayModal renders the replay mode selection modal.
func (m StreamsModel) renderReplayModal() string {
	modalStyle := styles.ModalStyle.
		Width(50).
		Align(lipgloss.Center)

	titleStyle := styles.ModalTitleStyle
	title := titleStyle.Render("Select Replay Mode")

	separator := lipgloss.NewStyle().Foreground(styles.ColorMuted).Render("─────────────────────────────────────────────────")

	// Mode options
	pureStyle := lipgloss.NewStyle()
	syncStyle := lipgloss.NewStyle()
	trigStyle := lipgloss.NewStyle()

	if m.replayMode == "pure" {
		pureStyle = pureStyle.Foreground(styles.ColorPrimary).Bold(true)
	} else {
		pureStyle = pureStyle.Foreground(styles.ColorForeground)
	}

	if m.replayMode == "synchronized" {
		syncStyle = syncStyle.Foreground(styles.ColorPrimary).Bold(true)
	} else {
		syncStyle = syncStyle.Foreground(styles.ColorForeground)
	}

	if m.replayMode == "triggered" {
		trigStyle = trigStyle.Foreground(styles.ColorPrimary).Bold(true)
	} else {
		trigStyle = trigStyle.Foreground(styles.ColorForeground)
	}

	options := []string{
		pureStyle.Render("[1] Pure - Replay with original timing"),
		syncStyle.Render("[2] Synchronized - Wait for client messages"),
		trigStyle.Render("[3] Triggered - Manual frame advancement"),
	}

	scaleInfo := fmt.Sprintf("Timing Scale: %.1fx [+/-]", m.replayScale)

	actions := lipgloss.NewStyle().Foreground(styles.ColorMuted).Render("[Enter] Start  [Esc] Cancel")

	content := lipgloss.JoinVertical(
		lipgloss.Left,
		title,
		separator,
		"",
		lipgloss.JoinVertical(lipgloss.Left, options...),
		"",
		scaleInfo,
		"",
		actions,
	)

	return modalStyle.Render(content)
}

// renderStatusMessage renders a temporary status message.
func (m StreamsModel) renderStatusMessage() string {
	msgStyle := lipgloss.NewStyle().
		Foreground(styles.ColorInfo).
		Italic(true)
	return msgStyle.Render(m.statusMessage)
}

// SetSize updates the streams view dimensions.
func (m *StreamsModel) SetSize(width, height int) {
	m.width = width
	m.height = height
	// Conservative height to avoid pushing content off screen
	tableHeight := height - 7
	if tableHeight < 5 {
		tableHeight = 5
	}
	if tableHeight > 15 {
		tableHeight = 15
	}
	m.table.SetHeight(tableHeight)
}

// updateTable updates the table with current recordings data.
func (m *StreamsModel) updateTable() {
	rows := make([]table.Row, 0, len(m.recordings))

	for _, rec := range m.recordings {
		size := formatBytes(rec.FileSize)
		duration := formatDuration(time.Duration(rec.Duration) * time.Millisecond)
		recorded := rec.StartTime.Format("2006-01-02 15:04")

		rows = append(rows, table.Row{
			truncate(rec.ID, 12),
			string(rec.Protocol),
			truncate(rec.Path, 30),
			fmt.Sprintf("%d", rec.FrameCount),
			duration,
			size,
			recorded,
		})
	}

	m.table.SetRows(rows)
}

// fetchStreamRecordings fetches stream recordings from the API.
func (m StreamsModel) fetchStreamRecordings() tea.Cmd {
	return func() tea.Msg {
		filter := &client.StreamRecordingFilter{
			Protocol: m.protocolFilter,
			Limit:    100,
		}

		recordings, err := m.client.ListStreamRecordings(filter)
		if err != nil {
			return errMsg{err: fmt.Errorf("fetch stream recordings: %w", err)}
		}

		return streamRecordingsLoadedMsg{recordings: recordings}
	}
}

// fetchRecordingDetail fetches details for the selected recording.
func (m StreamsModel) fetchRecordingDetail() tea.Cmd {
	return func() tea.Msg {
		if len(m.recordings) == 0 {
			return nil
		}

		selectedIdx := m.table.Cursor()
		if selectedIdx >= len(m.recordings) {
			return nil
		}

		rec := m.recordings[selectedIdx]
		detail, err := m.client.GetStreamRecording(rec.ID)
		if err != nil {
			return errMsg{err: fmt.Errorf("fetch recording detail: %w", err)}
		}

		return streamRecordingDetailMsg{recording: detail}
	}
}

// startReplay starts replaying the selected recording.
func (m StreamsModel) startReplay() tea.Cmd {
	return func() tea.Msg {
		if len(m.recordings) == 0 {
			return statusMsg{message: "No recording selected"}
		}

		selectedIdx := m.table.Cursor()
		if selectedIdx >= len(m.recordings) {
			return statusMsg{message: "No recording selected"}
		}

		rec := m.recordings[selectedIdx]
		sessionID, err := m.client.StartReplay(rec.ID, m.replayMode, m.replayScale, false, 300)
		if err != nil {
			return statusMsg{message: fmt.Sprintf("Failed to start replay: %v", err)}
		}

		return statusMsg{message: fmt.Sprintf("Started replay session: %s", sessionID)}
	}
}

// exportRecording exports the selected recording.
func (m StreamsModel) exportRecording() tea.Cmd {
	return func() tea.Msg {
		if len(m.recordings) == 0 {
			return statusMsg{message: "No recording selected"}
		}

		selectedIdx := m.table.Cursor()
		if selectedIdx >= len(m.recordings) {
			return statusMsg{message: "No recording selected"}
		}

		rec := m.recordings[selectedIdx]
		_, err := m.client.ExportStreamRecording(rec.ID)
		if err != nil {
			return statusMsg{message: fmt.Sprintf("Export failed: %v", err)}
		}

		return statusMsg{message: "Recording exported successfully"}
	}
}

// convertRecording converts the selected recording to a mock.
func (m StreamsModel) convertRecording() tea.Cmd {
	return func() tea.Msg {
		if len(m.recordings) == 0 {
			return statusMsg{message: "No recording selected"}
		}

		selectedIdx := m.table.Cursor()
		if selectedIdx >= len(m.recordings) {
			return statusMsg{message: "No recording selected"}
		}

		rec := m.recordings[selectedIdx]
		_, err := m.client.ConvertStreamRecording(rec.ID, nil)
		if err != nil {
			return statusMsg{message: fmt.Sprintf("Conversion failed: %v", err)}
		}

		return statusMsg{message: "Recording converted to mock"}
	}
}

// deleteRecording deletes the selected recording.
func (m StreamsModel) deleteRecording() tea.Cmd {
	return func() tea.Msg {
		if len(m.recordings) == 0 {
			return statusMsg{message: "No recording selected"}
		}

		selectedIdx := m.table.Cursor()
		if selectedIdx >= len(m.recordings) {
			return statusMsg{message: "No recording selected"}
		}

		rec := m.recordings[selectedIdx]
		err := m.client.DeleteStreamRecording(rec.ID)
		if err != nil {
			return statusMsg{message: fmt.Sprintf("Delete failed: %v", err)}
		}

		// Refresh the list after deletion
		m.fetchStreamRecordings()

		return statusMsg{message: "Recording deleted"}
	}
}

// tickRefresh returns a command that schedules the next refresh.
func (m StreamsModel) tickRefresh() tea.Cmd {
	return tea.Tick(5*time.Second, func(t time.Time) tea.Msg {
		return refreshTickMsg(t)
	})
}

// Messages

// streamRecordingsLoadedMsg contains loaded stream recordings.
type streamRecordingsLoadedMsg struct {
	recordings []*recording.RecordingSummary
}

// streamRecordingDetailMsg contains detailed recording data.
type streamRecordingDetailMsg struct {
	recording *recording.StreamRecording
}

// clearStatusMsg signals to clear the status message.
type clearStatusMsg struct{}

// statusMsg displays a temporary status message.
type statusMsg struct {
	message string
}

// Utility functions

// formatBytes formats bytes in a human-readable way.
func formatBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}

// min returns the minimum of two float64 values.
func min(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

// max returns the maximum of two float64 values.
func max(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}
