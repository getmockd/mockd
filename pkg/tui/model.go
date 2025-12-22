package tui

import (
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/getmockd/mockd/pkg/tui/components"
	"github.com/getmockd/mockd/pkg/tui/styles"
)

// model is the root application model
type model struct {
	// Window dimensions
	width  int
	height int

	// Current active view
	currentView viewState

	// Global keybindings
	keys KeyMap

	// Components
	header    components.HeaderModel
	sidebar   components.SidebarModel
	statusBar components.StatusBarModel
	help      components.HelpModel

	// Views (placeholders for now)
	// dashboard   dashboardModel
	// mocks       mocksModel
	// recordings  recordingsModel
	// streams     streamsModel
	// traffic     trafficModel
	// connections connectionsModel
	// logs        logsModel

	// Application state
	ready         bool
	loading       bool
	err           error
	statusMessage string
	lastUpdate    time.Time
}

// newModel creates a new root model
func newModel() model {
	return model{
		keys:        DefaultKeyMap(),
		currentView: dashboardView,
		ready:       false,
		loading:     false,
		header:      components.NewHeader(),
		sidebar:     components.NewSidebar(),
		statusBar:   components.NewStatusBar(),
		help:        components.NewHelp(),
	}
}

// Init initializes the model (Bubbletea lifecycle)
func (m model) Init() tea.Cmd {
	// Return initial commands
	return tea.Batch(
		tea.EnterAltScreen,
		tickCmd(),
	)
}

// Update handles messages (Bubbletea lifecycle)
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return m.handleKeyPress(msg)

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.ready = true

		// Update component sizes
		m.header.SetWidth(msg.Width)
		m.sidebar.SetHeight(msg.Height - 4) // Account for header and statusbar
		m.statusBar.SetWidth(msg.Width)
		m.help.SetSize(msg.Width, msg.Height)

		return m, nil

	case tickMsg:
		m.lastUpdate = time.Time(msg)
		// Schedule next tick
		return m, tickCmd()

	case errMsg:
		m.err = msg.err
		m.loading = false
		return m, nil

	case statusMsg:
		m.statusMessage = msg.message
		m.loading = false
		return m, nil

	case viewSwitchMsg:
		m.currentView = msg.view
		return m, nil
	}

	return m, nil
}

// handleKeyPress processes keyboard input
func (m model) handleKeyPress(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Global key handling
	switch {
	case key.Matches(msg, m.keys.Quit):
		return m, tea.Quit

	case key.Matches(msg, m.keys.Help):
		m.help.Toggle()
		return m, nil

	case key.Matches(msg, m.keys.Dashboard):
		m.currentView = dashboardView
		m.sidebar.SetActive(0)
		return m, nil

	case key.Matches(msg, m.keys.Mocks):
		m.currentView = mocksView
		m.sidebar.SetActive(1)
		return m, nil

	case key.Matches(msg, m.keys.Recordings):
		m.currentView = recordingsView
		m.sidebar.SetActive(2)
		return m, nil

	case key.Matches(msg, m.keys.Streams):
		m.currentView = streamsView
		m.sidebar.SetActive(3)
		return m, nil

	case key.Matches(msg, m.keys.Traffic):
		m.currentView = trafficView
		m.sidebar.SetActive(4)
		return m, nil

	case key.Matches(msg, m.keys.Connections):
		m.currentView = connectionsView
		m.sidebar.SetActive(5)
		return m, nil

	case key.Matches(msg, m.keys.Logs):
		m.currentView = logsView
		m.sidebar.SetActive(6)
		return m, nil
	}

	// TODO: Delegate to active view

	return m, nil
}

// View renders the UI (Bubbletea lifecycle)
func (m model) View() string {
	if !m.ready {
		return "Initializing mockd TUI..."
	}

	// For now, return a simple placeholder
	// Will be replaced with proper layout rendering
	return m.renderLayout()
}

// renderLayout composes the full UI layout
func (m model) renderLayout() string {
	// If help is visible, render it as an overlay
	if m.help.IsVisible() {
		// Render normal layout first
		base := m.renderMainLayout()
		// Overlay help on top
		return base + "\n" + m.help.View()
	}

	return m.renderMainLayout()
}

// renderMainLayout renders the main UI layout (header, sidebar, content, statusbar)
func (m model) renderMainLayout() string {
	// Render header
	header := m.header.View()

	// Render sidebar
	sidebar := m.sidebar.View()

	// Render content area based on current view
	content := m.renderContent()

	// Render status bar
	statusBar := m.statusBar.View()

	// Calculate dimensions for content area
	sidebarWidth := lipgloss.Width(sidebar)
	contentWidth := m.width - sidebarWidth - 2 // Account for borders/padding
	contentHeight := m.height - 4              // Account for header and statusbar

	// Style content area
	contentStyled := styles.ContentStyle.
		Width(contentWidth).
		Height(contentHeight).
		Render(content)

	// Compose main area (sidebar + content)
	mainArea := lipgloss.JoinHorizontal(
		lipgloss.Top,
		sidebar,
		contentStyled,
	)

	// Compose full layout
	return lipgloss.JoinVertical(
		lipgloss.Left,
		header,
		mainArea,
		statusBar,
	)
}

// renderContent renders the current view's content
func (m model) renderContent() string {
	switch m.currentView {
	case dashboardView:
		return m.renderDashboard()
	case mocksView:
		return "Mocks View\n\nManage mock endpoints here.\n\nPress 'n' to create a new mock."
	case recordingsView:
		return "Recordings View\n\nView HTTP recordings here."
	case streamsView:
		return "Streams View\n\nManage WebSocket and SSE recordings."
	case trafficView:
		return "Traffic View\n\nLive request log will appear here."
	case connectionsView:
		return "Connections View\n\nActive WebSocket and SSE connections."
	case logsView:
		return "Logs View\n\nApplication logs will appear here."
	default:
		return "Unknown View"
	}
}

// renderDashboard renders the dashboard view
func (m model) renderDashboard() string {
	title := styles.TitleStyle.Render("Dashboard")

	status := lipgloss.NewStyle().
		Foreground(styles.ColorSuccess).
		Render("● Server Running")

	info := `
Server Status
─────────────
Mock Server: Running on :8080
Admin API: :9090
Proxy: Inactive

Quick Stats
───────────
Mocks: 0 active, 0 disabled
Recordings: 0
Active Connections: 0

Recent Activity
───────────────
No recent activity

`

	return lipgloss.JoinVertical(
		lipgloss.Left,
		title,
		"",
		status,
		info,
	)
}

// tickCmd returns a command that sends a tick message every second
func tickCmd() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}
