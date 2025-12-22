package views

import (
	"fmt"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/getmockd/mockd/pkg/admin"
	"github.com/getmockd/mockd/pkg/config"
	"github.com/getmockd/mockd/pkg/tui/client"
	"github.com/getmockd/mockd/pkg/tui/styles"
)

// DashboardModel represents the dashboard view state.
type DashboardModel struct {
	client  *client.Client
	width   int
	height  int
	loading bool
	err     error

	// Server status
	health        *admin.HealthResponse
	proxyStatus   *admin.ProxyStatusResponse
	mocks         []*config.MockConfiguration
	recentTraffic []*config.RequestLogEntry

	// Counts
	activeMocks   int
	disabledMocks int
	totalRequests int
	activeConns   int

	// UI state
	spinner       spinner.Model
	lastRefresh   time.Time
	statusMessage string
	recording     bool
}

// NewDashboard creates a new dashboard model.
func NewDashboard(adminClient *client.Client) DashboardModel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(styles.ColorPrimary)

	return DashboardModel{
		client:  adminClient,
		loading: true,
		spinner: s,
	}
}

// Init initializes the dashboard (Bubbletea lifecycle).
func (m DashboardModel) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
		m.fetchDashboardData(),
		m.tickRefresh(),
	)
}

// Update handles messages (Bubbletea lifecycle).
func (m DashboardModel) Update(msg tea.Msg) (DashboardModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case dashboardDataMsg:
		m.loading = false
		m.health = msg.health
		m.proxyStatus = msg.proxyStatus
		m.mocks = msg.mocks
		m.recentTraffic = msg.recentTraffic
		m.lastRefresh = time.Now()

		// Calculate counts
		m.activeMocks = 0
		m.disabledMocks = 0
		for _, mock := range m.mocks {
			if mock.Enabled {
				m.activeMocks++
			} else {
				m.disabledMocks++
			}
		}
		m.totalRequests = len(m.recentTraffic)

		return m, nil

	case refreshTickMsg:
		// Auto-refresh every 5 seconds
		return m, tea.Batch(
			m.fetchDashboardData(),
			m.tickRefresh(),
		)

	case errMsg:
		m.loading = false
		m.err = msg.err
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)
	}

	return m, nil
}

// handleKey processes keyboard input for dashboard-specific actions.
func (m DashboardModel) handleKey(msg tea.KeyMsg) (DashboardModel, tea.Cmd) {
	switch msg.String() {
	case "s":
		// TODO: Start/stop server action
		m.statusMessage = "Server control not yet implemented"
		return m, nil

	case "r":
		// TODO: Toggle recording
		m.recording = !m.recording
		m.statusMessage = fmt.Sprintf("Recording: %v", m.recording)
		return m, nil

	case "p":
		// Navigate to proxy view (handled by parent model)
		return m, nil
	}

	return m, nil
}

// View renders the dashboard (Bubbletea lifecycle).
func (m DashboardModel) View() string {
	if m.loading && m.health == nil {
		return m.renderLoading()
	}

	if m.err != nil {
		return m.renderError()
	}

	return m.renderDashboard()
}

// renderLoading shows a loading spinner.
func (m DashboardModel) renderLoading() string {
	return fmt.Sprintf("\n  %s Loading dashboard data...\n", m.spinner.View())
}

// renderError shows an error message.
func (m DashboardModel) renderError() string {
	errorStyle := lipgloss.NewStyle().
		Foreground(styles.ColorError).
		Bold(true)

	return fmt.Sprintf("\n  %s\n\n  %s\n",
		errorStyle.Render("Error loading dashboard:"),
		m.err.Error(),
	)
}

// renderDashboard renders the full dashboard view.
func (m DashboardModel) renderDashboard() string {
	sections := []string{
		m.renderServerStatus(),
		"",
		m.renderQuickStats(),
		"",
		m.renderRecentActivity(),
	}

	if m.statusMessage != "" {
		sections = append(sections, "", m.renderStatusMessage())
	}

	return lipgloss.JoinVertical(lipgloss.Left, sections...)
}

// renderServerStatus renders the server status section.
func (m DashboardModel) renderServerStatus() string {
	titleStyle := styles.TitleStyle
	title := titleStyle.Render("Server Status")

	// Server running indicator
	serverStatus := "● Server Running"
	if m.health == nil || m.health.Status != "ok" {
		serverStatus = "○ Server Offline"
	}
	statusStyle := lipgloss.NewStyle().Foreground(styles.ColorSuccess)
	statusLine := statusStyle.Render(serverStatus)

	// Build status lines
	separator := lipgloss.NewStyle().Foreground(styles.ColorMuted).Render("─────────────")

	lines := []string{
		title,
		separator,
		statusLine,
		fmt.Sprintf("Admin API: :9090"),
	}

	// Add uptime if available
	if m.health != nil {
		uptime := time.Duration(m.health.Uptime) * time.Second
		lines = append(lines, fmt.Sprintf("Uptime: %s", formatDuration(uptime)))
	}

	// Add proxy status
	if m.proxyStatus != nil && m.proxyStatus.Running {
		recordingStatus := ""
		if m.proxyStatus.Mode == "record" {
			recordingStatus = " (recording)"
		}
		lines = append(lines,
			fmt.Sprintf("Proxy: Running on :%d%s", m.proxyStatus.Port, recordingStatus),
		)
	} else {
		lines = append(lines, "Proxy: Inactive")
	}

	return lipgloss.JoinVertical(lipgloss.Left, lines...)
}

// renderQuickStats renders the quick stats section.
func (m DashboardModel) renderQuickStats() string {
	titleStyle := styles.TitleStyle
	title := titleStyle.Render("Quick Stats")

	separator := lipgloss.NewStyle().Foreground(styles.ColorMuted).Render("───────────")

	lines := []string{
		title,
		separator,
		fmt.Sprintf("Mocks: %d active, %d disabled", m.activeMocks, m.disabledMocks),
		fmt.Sprintf("Recordings: %d", 0), // TODO: Get real recording count
		fmt.Sprintf("Active connections: %d", m.activeConns),
	}

	return lipgloss.JoinVertical(lipgloss.Left, lines...)
}

// renderRecentActivity renders the recent activity section.
func (m DashboardModel) renderRecentActivity() string {
	titleStyle := styles.TitleStyle
	title := titleStyle.Render("Recent Activity")

	separator := lipgloss.NewStyle().Foreground(styles.ColorMuted).Render("───────────────")

	lines := []string{
		title,
		separator,
	}

	if len(m.recentTraffic) == 0 {
		lines = append(lines, "No recent activity")
	} else {
		// Show last 5 requests
		count := len(m.recentTraffic)
		if count > 5 {
			count = 5
		}

		for i := 0; i < count; i++ {
			entry := m.recentTraffic[i]
			timestamp := entry.Timestamp.Format("15:04:05")

			// Format status with color
			statusStr := fmt.Sprintf("%d", entry.ResponseStatus)
			statusStyle := lipgloss.NewStyle()
			if entry.ResponseStatus >= 200 && entry.ResponseStatus < 300 {
				statusStyle = statusStyle.Foreground(styles.ColorSuccess)
			} else if entry.ResponseStatus >= 400 {
				statusStyle = statusStyle.Foreground(styles.ColorError)
			}

			// Format duration
			durationStr := formatDuration(time.Duration(entry.DurationMs) * time.Millisecond)

			// Build line
			line := fmt.Sprintf("%s  %-6s %-30s %s  %6s",
				timestamp,
				entry.Method,
				truncate(entry.Path, 30),
				statusStyle.Render(statusStr),
				durationStr,
			)

			lines = append(lines, line)
		}
	}

	return lipgloss.JoinVertical(lipgloss.Left, lines...)
}

// renderStatusMessage renders a temporary status message.
func (m DashboardModel) renderStatusMessage() string {
	msgStyle := lipgloss.NewStyle().
		Foreground(styles.ColorInfo).
		Italic(true)
	return msgStyle.Render(m.statusMessage)
}

// SetSize updates the dashboard dimensions.
func (m *DashboardModel) SetSize(width, height int) {
	m.width = width
	m.height = height
}

// fetchDashboardData fetches all dashboard data from the API.
func (m DashboardModel) fetchDashboardData() tea.Cmd {
	return func() tea.Msg {
		// Fetch health
		health, err := m.client.GetHealth()
		if err != nil {
			return errMsg{err: fmt.Errorf("fetch health: %w", err)}
		}

		// Fetch proxy status
		proxyStatus, err := m.client.GetProxyStatus()
		if err != nil {
			// Proxy status might not be available, don't fail
			proxyStatus = nil
		}

		// Fetch mocks
		mocks, err := m.client.ListMocks()
		if err != nil {
			return errMsg{err: fmt.Errorf("fetch mocks: %w", err)}
		}

		// Fetch recent traffic (last 10 requests)
		traffic, err := m.client.GetTraffic(&client.RequestLogFilter{
			Limit: 10,
		})
		if err != nil {
			// Traffic might not be available yet
			traffic = nil
		}

		return dashboardDataMsg{
			health:        health,
			proxyStatus:   proxyStatus,
			mocks:         mocks,
			recentTraffic: traffic,
		}
	}
}

// tickRefresh returns a command that schedules the next refresh.
func (m DashboardModel) tickRefresh() tea.Cmd {
	return tea.Tick(5*time.Second, func(t time.Time) tea.Msg {
		return refreshTickMsg(t)
	})
}

// Messages

// dashboardDataMsg contains all dashboard data.
type dashboardDataMsg struct {
	health        *admin.HealthResponse
	proxyStatus   *admin.ProxyStatusResponse
	mocks         []*config.MockConfiguration
	recentTraffic []*config.RequestLogEntry
}

// refreshTickMsg signals it's time to refresh dashboard data.
type refreshTickMsg time.Time

// errMsg wraps errors from async operations.
type errMsg struct {
	err error
}

// Utility functions

// formatDuration formats a duration in a human-readable way.
func formatDuration(d time.Duration) string {
	if d < time.Microsecond {
		return fmt.Sprintf("%dns", d.Nanoseconds())
	}
	if d < time.Millisecond {
		return fmt.Sprintf("%dµs", d.Microseconds())
	}
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	if d < time.Minute {
		return fmt.Sprintf("%.1fs", d.Seconds())
	}
	if d < time.Hour {
		return fmt.Sprintf("%.1fm", d.Minutes())
	}
	return fmt.Sprintf("%.1fh", d.Hours())
}

// truncate truncates a string to the specified length.
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}
