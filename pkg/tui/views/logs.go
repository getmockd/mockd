package views

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/getmockd/mockd/pkg/tui/client"
	"github.com/getmockd/mockd/pkg/tui/styles"
)

// LogLevel represents log severity levels.
type LogLevel int

const (
	LogLevelDebug LogLevel = iota
	LogLevelInfo
	LogLevelWarn
	LogLevelError
)

// LogEntry represents a single log entry.
type LogEntry struct {
	Timestamp time.Time
	Level     LogLevel
	Message   string
	Source    string
}

// LogsModel represents the logs view state.
type LogsModel struct {
	client  *client.Client
	width   int
	height  int
	loading bool
	err     error

	// Data
	logs     []LogEntry
	filtered []LogEntry

	// Filters
	levelFilter LogLevel // Minimum level to show
	searchTerm  string

	// UI state
	viewport      viewport.Model
	spinner       spinner.Model
	lastRefresh   time.Time
	statusMessage string
	paused        bool

	// Search input state
	showSearch  bool
	searchInput textinput.Model
}

// NewLogs creates a new logs model.
func NewLogs(adminClient *client.Client) LogsModel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(styles.ColorPrimary)

	vp := viewport.New(80, 15)
	vp.Style = lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(styles.ColorBorder)

	si := textinput.New()
	si.Placeholder = "Search logs..."
	si.CharLimit = 100
	si.Width = 50

	return LogsModel{
		client:      adminClient,
		loading:     false, // Logs are generated locally, no loading needed
		spinner:     s,
		viewport:    vp,
		searchInput: si,
		levelFilter: LogLevelDebug, // Show all levels by default
		logs:        make([]LogEntry, 0),
	}
}

// Init initializes the logs view (Bubbletea lifecycle).
func (m LogsModel) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
		m.tickRefresh(),
	)
}

// Update handles messages (Bubbletea lifecycle).
func (m LogsModel) Update(msg tea.Msg) (LogsModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.SetSize(msg.Width, msg.Height)
		return m, nil

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case refreshTickMsg:
		// Auto-refresh every second if not paused
		if !m.paused {
			m.generateMockLogs()
			m.filterLogs()
			m.updateViewport()
		}
		return m, m.tickRefresh()

	case errMsg:
		m.loading = false
		m.err = msg.err
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

	// Update search input if active
	if m.showSearch {
		var cmd tea.Cmd
		m.searchInput, cmd = m.searchInput.Update(msg)
		// Update search term and filter
		m.searchTerm = m.searchInput.Value()
		m.filterLogs()
		m.updateViewport()
		return m, cmd
	}

	// Update viewport
	var cmd tea.Cmd
	m.viewport, cmd = m.viewport.Update(msg)
	return m, cmd
}

// handleKey processes keyboard input for logs-specific actions.
func (m LogsModel) handleKey(msg tea.KeyMsg) (LogsModel, tea.Cmd) {
	// If search is shown, handle search-specific keys
	if m.showSearch {
		switch msg.String() {
		case "enter", "esc":
			m.showSearch = false
			m.searchInput.Blur()
			return m, nil
		}
		return m, nil
	}

	// Regular logs view keys
	switch msg.String() {
	case "p":
		// Pause/resume
		m.paused = !m.paused
		if m.paused {
			m.statusMessage = "Logs paused"
		} else {
			m.statusMessage = "Logs resumed"
		}
		return m, nil

	case "c":
		// Clear logs
		m.logs = make([]LogEntry, 0)
		m.filtered = make([]LogEntry, 0)
		m.statusMessage = "Logs cleared"
		m.updateViewport()
		return m, nil

	case "/":
		// Show search
		m.showSearch = true
		m.searchInput.Focus()
		return m, nil

	case "1":
		// Filter: Debug and above
		m.levelFilter = LogLevelDebug
		m.statusMessage = "Showing all log levels"
		m.filterLogs()
		m.updateViewport()
		return m, nil

	case "2":
		// Filter: Info and above
		m.levelFilter = LogLevelInfo
		m.statusMessage = "Showing info, warn, error"
		m.filterLogs()
		m.updateViewport()
		return m, nil

	case "3":
		// Filter: Warn and above
		m.levelFilter = LogLevelWarn
		m.statusMessage = "Showing warn, error"
		m.filterLogs()
		m.updateViewport()
		return m, nil

	case "4":
		// Filter: Error only
		m.levelFilter = LogLevelError
		m.statusMessage = "Showing error only"
		m.filterLogs()
		m.updateViewport()
		return m, nil
	}

	return m, nil
}

// View renders the logs view (Bubbletea lifecycle).
func (m LogsModel) View() string {
	if m.err != nil {
		return m.renderError()
	}

	// If search is shown, render it as overlay
	if m.showSearch {
		return m.renderLogsView() + "\n\n" + m.renderSearchInput()
	}

	return m.renderLogsView()
}

// renderError shows an error message.
func (m LogsModel) renderError() string {
	errorStyle := lipgloss.NewStyle().
		Foreground(styles.ColorError).
		Bold(true)

	return fmt.Sprintf("\n  %s\n\n  %s\n",
		errorStyle.Render("Error in logs view:"),
		m.err.Error(),
	)
}

// renderLogsView renders the full logs view.
func (m LogsModel) renderLogsView() string {
	sections := []string{
		m.renderHeader(),
		m.viewport.View(),
	}

	if m.statusMessage != "" {
		sections = append(sections, "", m.renderStatusMessage())
	}

	return lipgloss.JoinVertical(lipgloss.Left, sections...)
}

// renderHeader renders the header with level filter tabs.
func (m LogsModel) renderHeader() string {
	titleStyle := styles.TitleStyle
	pausedIndicator := ""
	if m.paused {
		pausedIndicator = " [PAUSED]"
	}
	title := titleStyle.Render(fmt.Sprintf("Application Logs (%d)%s", len(m.filtered), pausedIndicator))

	// Level filter tabs
	debugStyle := lipgloss.NewStyle().Padding(0, 1)
	infoStyle := lipgloss.NewStyle().Padding(0, 1)
	warnStyle := lipgloss.NewStyle().Padding(0, 1)
	errorStyle := lipgloss.NewStyle().Padding(0, 1)

	if m.levelFilter == LogLevelDebug {
		debugStyle = debugStyle.Foreground(styles.ColorPrimary).Bold(true)
	} else {
		debugStyle = debugStyle.Foreground(styles.ColorMuted)
	}

	if m.levelFilter == LogLevelInfo {
		infoStyle = infoStyle.Foreground(styles.ColorPrimary).Bold(true)
	} else {
		infoStyle = infoStyle.Foreground(styles.ColorMuted)
	}

	if m.levelFilter == LogLevelWarn {
		warnStyle = warnStyle.Foreground(styles.ColorPrimary).Bold(true)
	} else {
		warnStyle = warnStyle.Foreground(styles.ColorMuted)
	}

	if m.levelFilter == LogLevelError {
		errorStyle = errorStyle.Foreground(styles.ColorPrimary).Bold(true)
	} else {
		errorStyle = errorStyle.Foreground(styles.ColorMuted)
	}

	tabs := lipgloss.JoinHorizontal(
		lipgloss.Top,
		debugStyle.Render("[1] Debug"),
		infoStyle.Render("[2] Info"),
		warnStyle.Render("[3] Warn"),
		errorStyle.Render("[4] Error"),
	)

	return lipgloss.JoinVertical(lipgloss.Left, title, tabs)
}

// renderSearchInput renders the search input overlay.
func (m LogsModel) renderSearchInput() string {
	modalStyle := styles.ModalStyle.
		Width(60).
		Align(lipgloss.Left)

	titleStyle := styles.ModalTitleStyle
	title := titleStyle.Render("Search Logs")

	separator := lipgloss.NewStyle().Foreground(styles.ColorMuted).Render("────────────────────────────────────────────────────────")

	label := lipgloss.NewStyle().Foreground(styles.ColorInfo).Render("Search:")

	actions := lipgloss.NewStyle().Foreground(styles.ColorMuted).Render("[Enter/Esc] Close")

	content := lipgloss.JoinVertical(
		lipgloss.Left,
		title,
		separator,
		"",
		label,
		m.searchInput.View(),
		"",
		actions,
	)

	return modalStyle.Render(content)
}

// renderStatusMessage renders a temporary status message.
func (m LogsModel) renderStatusMessage() string {
	msgStyle := lipgloss.NewStyle().
		Foreground(styles.ColorInfo).
		Italic(true)
	return msgStyle.Render(m.statusMessage)
}

// SetSize updates the logs view dimensions.
func (m *LogsModel) SetSize(width, height int) {
	m.width = width
	m.height = height
	m.viewport.Width = width - 4
	// Very conservative height - header takes ~3 lines, viewport borders take ~2
	viewportHeight := height - 10
	if viewportHeight < 8 {
		viewportHeight = 8
	}
	if viewportHeight > 12 {
		viewportHeight = 12
	}
	m.viewport.Height = viewportHeight
	m.updateViewport()
}

// filterLogs filters logs based on level and search term.
func (m *LogsModel) filterLogs() {
	m.filtered = make([]LogEntry, 0)

	for _, log := range m.logs {
		// Filter by level
		if log.Level < m.levelFilter {
			continue
		}

		// Filter by search term
		if m.searchTerm != "" {
			if !strings.Contains(strings.ToLower(log.Message), strings.ToLower(m.searchTerm)) &&
				!strings.Contains(strings.ToLower(log.Source), strings.ToLower(m.searchTerm)) {
				continue
			}
		}

		m.filtered = append(m.filtered, log)
	}
}

// updateViewport updates the viewport content with filtered logs.
func (m *LogsModel) updateViewport() {
	var lines []string

	for _, log := range m.filtered {
		timestamp := log.Timestamp.Format("15:04:05.000")

		var levelStyle lipgloss.Style
		var levelStr string
		switch log.Level {
		case LogLevelDebug:
			levelStyle = lipgloss.NewStyle().Foreground(styles.ColorMuted)
			levelStr = "DEBUG"
		case LogLevelInfo:
			levelStyle = lipgloss.NewStyle().Foreground(styles.ColorInfo)
			levelStr = "INFO "
		case LogLevelWarn:
			levelStyle = lipgloss.NewStyle().Foreground(styles.ColorWarning)
			levelStr = "WARN "
		case LogLevelError:
			levelStyle = lipgloss.NewStyle().Foreground(styles.ColorError)
			levelStr = "ERROR"
		}

		line := fmt.Sprintf("%s %s [%s] %s",
			timestamp,
			levelStyle.Render(levelStr),
			log.Source,
			log.Message,
		)

		lines = append(lines, line)
	}

	content := strings.Join(lines, "\n")
	m.viewport.SetContent(content)

	// Auto-scroll to bottom for new logs
	if !m.paused {
		m.viewport.GotoBottom()
	}
}

// generateMockLogs generates sample log entries for demonstration.
func (m *LogsModel) generateMockLogs() {
	// In a real implementation, this would fetch logs from the admin API
	// or from a log buffer. For now, we'll generate mock logs.

	now := time.Now()

	// Limit log buffer to 1000 entries
	if len(m.logs) > 1000 {
		m.logs = m.logs[len(m.logs)-900:]
	}

	// Generate a few random log entries
	samples := []LogEntry{
		{Timestamp: now, Level: LogLevelInfo, Message: "Request received: GET /api/users", Source: "engine"},
		{Timestamp: now, Level: LogLevelDebug, Message: "Matching request against 12 mocks", Source: "matcher"},
		{Timestamp: now, Level: LogLevelInfo, Message: "Mock matched: user-list", Source: "matcher"},
		{Timestamp: now, Level: LogLevelWarn, Message: "Response latency: 150ms (threshold: 100ms)", Source: "engine"},
	}

	// Add a sample log every few ticks
	if len(m.logs)%3 == 0 && len(m.logs) < 100 {
		m.logs = append(m.logs, samples[len(m.logs)%len(samples)])
	}
}

// tickRefresh returns a command that schedules the next refresh.
func (m LogsModel) tickRefresh() tea.Cmd {
	return tea.Tick(1*time.Second, func(t time.Time) tea.Msg {
		return refreshTickMsg(t)
	})
}
