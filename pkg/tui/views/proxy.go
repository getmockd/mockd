package views

import (
	"fmt"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/getmockd/mockd/pkg/admin"
	"github.com/getmockd/mockd/pkg/tui/client"
	"github.com/getmockd/mockd/pkg/tui/styles"
)

// ProxyModel represents the proxy view state.
type ProxyModel struct {
	client  *client.Client
	width   int
	height  int
	loading bool
	err     error

	// Data
	status *admin.ProxyStatusResponse

	// UI state
	spinner       spinner.Model
	lastRefresh   time.Time
	statusMessage string

	// Input state
	showTargetInput bool
	targetInput     textinput.Model
}

// NewProxy creates a new proxy model.
func NewProxy(adminClient *client.Client) ProxyModel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(styles.ColorPrimary)

	ti := textinput.New()
	ti.Placeholder = "https://api.example.com"
	ti.CharLimit = 200
	ti.Width = 50

	return ProxyModel{
		client:      adminClient,
		loading:     true,
		spinner:     s,
		targetInput: ti,
	}
}

// Init initializes the proxy view (Bubbletea lifecycle).
func (m ProxyModel) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
		m.fetchProxyStatus(),
		m.tickRefresh(),
	)
}

// Update handles messages (Bubbletea lifecycle).
func (m ProxyModel) Update(msg tea.Msg) (ProxyModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case proxyStatusLoadedMsg:
		m.loading = false
		m.status = msg.status
		m.lastRefresh = time.Now()
		return m, nil

	case refreshTickMsg:
		// Auto-refresh every 3 seconds
		return m, tea.Batch(
			m.fetchProxyStatus(),
			m.tickRefresh(),
		)

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

	// If target input is active, update it
	if m.showTargetInput {
		var cmd tea.Cmd
		m.targetInput, cmd = m.targetInput.Update(msg)
		return m, cmd
	}

	return m, nil
}

// handleKey processes keyboard input for proxy-specific actions.
func (m ProxyModel) handleKey(msg tea.KeyMsg) (ProxyModel, tea.Cmd) {
	// If target input is shown, handle input-specific keys
	if m.showTargetInput {
		switch msg.String() {
		case "enter":
			// TODO: Update proxy target
			newTarget := m.targetInput.Value()
			m.showTargetInput = false
			m.statusMessage = fmt.Sprintf("Proxy target updated to: %s", newTarget)
			return m, nil

		case "esc":
			m.showTargetInput = false
			m.statusMessage = "Target change cancelled"
			return m, nil
		}
		return m, nil
	}

	// Regular proxy view keys
	switch msg.String() {
	case "s":
		// Start/stop proxy
		return m, m.toggleProxy()

	case "t":
		// Change target - show input
		m.showTargetInput = true
		m.targetInput.Focus()
		return m, nil

	case "r":
		// Toggle recording
		return m, m.toggleRecording()

	case "m":
		// Change mode (pass-through/record/mock)
		return m, m.cycleMode()
	}

	return m, nil
}

// View renders the proxy view (Bubbletea lifecycle).
func (m ProxyModel) View() string {
	if m.loading && m.status == nil {
		return m.renderLoading()
	}

	if m.err != nil {
		return m.renderError()
	}

	// If target input is shown, render it as overlay
	if m.showTargetInput {
		return m.renderProxyView() + "\n\n" + m.renderTargetInput()
	}

	return m.renderProxyView()
}

// renderLoading shows a loading spinner.
func (m ProxyModel) renderLoading() string {
	return fmt.Sprintf("\n  %s Loading proxy status...\n", m.spinner.View())
}

// renderError shows an error message.
func (m ProxyModel) renderError() string {
	errorStyle := lipgloss.NewStyle().
		Foreground(styles.ColorError).
		Bold(true)

	return fmt.Sprintf("\n  %s\n\n  %s\n",
		errorStyle.Render("Error loading proxy status:"),
		m.err.Error(),
	)
}

// renderProxyView renders the full proxy view.
func (m ProxyModel) renderProxyView() string {
	sections := []string{
		m.renderHeader(),
		"",
		m.renderStatus(),
	}

	if m.statusMessage != "" {
		sections = append(sections, "", m.renderStatusMessage())
	}

	return lipgloss.JoinVertical(lipgloss.Left, sections...)
}

// renderHeader renders the header.
func (m ProxyModel) renderHeader() string {
	titleStyle := styles.TitleStyle
	title := titleStyle.Render("Proxy Control")

	separator := lipgloss.NewStyle().Foreground(styles.ColorMuted).Render("─────────────")

	return lipgloss.JoinVertical(lipgloss.Left, title, separator)
}

// renderStatus renders the proxy status information.
func (m ProxyModel) renderStatus() string {
	if m.status == nil {
		return "No proxy status available"
	}

	// Running status
	runningIndicator := "○ Stopped"
	runningColor := styles.ColorMuted
	if m.status.Running {
		runningIndicator = "● Running"
		runningColor = styles.ColorSuccess
	}
	statusLine := lipgloss.NewStyle().Foreground(runningColor).Render(runningIndicator)

	lines := []string{
		statusLine,
	}

	if m.status.Running {
		lines = append(lines,
			fmt.Sprintf("Port: %d", m.status.Port),
			fmt.Sprintf("Mode: %s", m.status.Mode),
		)

		// Recording status
		recordingStatus := "Recording: OFF"
		if m.status.Mode == "record" {
			recordingStatus = "Recording: ON"
		}
		recordingStyle := lipgloss.NewStyle()
		if m.status.Mode == "record" {
			recordingStyle = recordingStyle.Foreground(styles.ColorWarning).Bold(true)
		} else {
			recordingStyle = recordingStyle.Foreground(styles.ColorMuted)
		}
		lines = append(lines, recordingStyle.Render(recordingStatus))

		// Recording count
		if m.status.RecordingCount > 0 {
			lines = append(lines, fmt.Sprintf("Recordings: %d", m.status.RecordingCount))
		}

		// Uptime
		if m.status.Uptime > 0 {
			uptime := time.Duration(m.status.Uptime) * time.Second
			lines = append(lines, fmt.Sprintf("Uptime: %s", formatDuration(uptime)))
		}
	} else {
		lines = append(lines, "Proxy is not running")
	}

	return lipgloss.JoinVertical(lipgloss.Left, lines...)
}

// renderTargetInput renders the target URL input overlay.
func (m ProxyModel) renderTargetInput() string {
	modalStyle := styles.ModalStyle.
		Width(60).
		Align(lipgloss.Left)

	titleStyle := styles.ModalTitleStyle
	title := titleStyle.Render("Change Proxy Target")

	separator := lipgloss.NewStyle().Foreground(styles.ColorMuted).Render("────────────────────────────────────────────────────────")

	label := lipgloss.NewStyle().Foreground(styles.ColorInfo).Render("Target URL:")

	actions := lipgloss.NewStyle().Foreground(styles.ColorMuted).Render("[Enter] Save  [Esc] Cancel")

	content := lipgloss.JoinVertical(
		lipgloss.Left,
		title,
		separator,
		"",
		label,
		m.targetInput.View(),
		"",
		actions,
	)

	return modalStyle.Render(content)
}

// renderStatusMessage renders a temporary status message.
func (m ProxyModel) renderStatusMessage() string {
	msgStyle := lipgloss.NewStyle().
		Foreground(styles.ColorInfo).
		Italic(true)
	return msgStyle.Render(m.statusMessage)
}

// SetSize updates the proxy view dimensions.
func (m *ProxyModel) SetSize(width, height int) {
	m.width = width
	m.height = height
}

// fetchProxyStatus fetches proxy status from the API.
func (m ProxyModel) fetchProxyStatus() tea.Cmd {
	return func() tea.Msg {
		status, err := m.client.GetProxyStatus()
		if err != nil {
			return errMsg{err: fmt.Errorf("fetch proxy status: %w", err)}
		}

		return proxyStatusLoadedMsg{status: status}
	}
}

// toggleProxy starts or stops the proxy.
func (m ProxyModel) toggleProxy() tea.Cmd {
	return func() tea.Msg {
		// TODO: Implement proxy start/stop via admin API
		// For now, just show a message
		if m.status != nil && m.status.Running {
			return statusMsg{message: "Proxy stop not yet implemented"}
		}
		return statusMsg{message: "Proxy start not yet implemented"}
	}
}

// toggleRecording toggles recording mode.
func (m ProxyModel) toggleRecording() tea.Cmd {
	return func() tea.Msg {
		// TODO: Implement recording toggle via admin API
		if m.status != nil && m.status.Mode == "record" {
			return statusMsg{message: "Recording disabled"}
		}
		return statusMsg{message: "Recording enabled"}
	}
}

// cycleMode cycles through proxy modes.
func (m ProxyModel) cycleMode() tea.Cmd {
	return func() tea.Msg {
		// TODO: Implement mode cycling via admin API
		// Cycle: pass-through -> record -> mock -> pass-through
		return statusMsg{message: "Mode cycling not yet implemented"}
	}
}

// tickRefresh returns a command that schedules the next refresh.
func (m ProxyModel) tickRefresh() tea.Cmd {
	return tea.Tick(3*time.Second, func(t time.Time) tea.Msg {
		return refreshTickMsg(t)
	})
}

// Messages

// proxyStatusLoadedMsg contains loaded proxy status.
type proxyStatusLoadedMsg struct {
	status *admin.ProxyStatusResponse
}
