package tui

import (
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/getmockd/mockd/pkg/tui/client"
	"github.com/getmockd/mockd/pkg/tui/components"
	"github.com/getmockd/mockd/pkg/tui/styles"
	"github.com/getmockd/mockd/pkg/tui/views"
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

	// Admin API client
	adminClient *client.Client

	// Views
	dashboard views.DashboardModel
	mocks     views.MocksModel
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
	// Create admin client
	adminClient := client.NewDefaultClient()

	return model{
		keys:        DefaultKeyMap(),
		currentView: dashboardView,
		ready:       false,
		loading:     false,
		header:      components.NewHeader(),
		sidebar:     components.NewSidebar(),
		statusBar:   components.NewStatusBar(),
		help:        components.NewHelp(),
		adminClient: adminClient,
		dashboard:   views.NewDashboard(adminClient),
		mocks:       views.NewMocks(adminClient),
	}
}

// Init initializes the model (Bubbletea lifecycle)
func (m model) Init() tea.Cmd {
	// Return initial commands
	return tea.Batch(
		tea.EnterAltScreen,
		tickCmd(),
		m.dashboard.Init(),
		m.mocks.Init(),
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

		// Update view sizes
		contentWidth := msg.Width - 16 // Sidebar width
		contentHeight := msg.Height - 4
		m.dashboard.SetSize(contentWidth, contentHeight)
		m.mocks.SetSize(contentWidth, contentHeight)

		// Initialize status bar hints
		m.updateStatusBarHints()

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

	// Delegate to active view
	var cmd tea.Cmd
	switch m.currentView {
	case dashboardView:
		m.dashboard, cmd = m.dashboard.Update(msg)
		return m, cmd
	case mocksView:
		m.mocks, cmd = m.mocks.Update(msg)
		return m, cmd
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
		m.updateStatusBarHints()
		return m, nil

	case key.Matches(msg, m.keys.Mocks):
		m.currentView = mocksView
		m.sidebar.SetActive(1)
		m.updateStatusBarHints()
		return m, nil

	case key.Matches(msg, m.keys.Recordings):
		m.currentView = recordingsView
		m.sidebar.SetActive(2)
		m.updateStatusBarHints()
		return m, nil

	case key.Matches(msg, m.keys.Streams):
		m.currentView = streamsView
		m.sidebar.SetActive(3)
		m.updateStatusBarHints()
		return m, nil

	case key.Matches(msg, m.keys.Traffic):
		m.currentView = trafficView
		m.sidebar.SetActive(4)
		m.updateStatusBarHints()
		return m, nil

	case key.Matches(msg, m.keys.Connections):
		m.currentView = connectionsView
		m.sidebar.SetActive(5)
		m.updateStatusBarHints()
		return m, nil

	case key.Matches(msg, m.keys.Logs):
		m.currentView = logsView
		m.sidebar.SetActive(6)
		m.updateStatusBarHints()
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
		return m.dashboard.View()
	case mocksView:
		return m.mocks.View()
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

// updateStatusBarHints updates the status bar hints based on the current view.
func (m *model) updateStatusBarHints() {
	baseHints := []components.KeyHint{
		{Key: "?", Desc: "help"},
		{Key: "q", Desc: "quit"},
	}

	switch m.currentView {
	case dashboardView:
		m.statusBar.SetHints(append([]components.KeyHint{
			{Key: "s", Desc: "start/stop"},
			{Key: "r", Desc: "record"},
			{Key: "p", Desc: "proxy"},
		}, baseHints...))
	case mocksView:
		m.statusBar.SetHints(append([]components.KeyHint{
			{Key: "enter", Desc: "toggle"},
			{Key: "n", Desc: "new"},
			{Key: "e", Desc: "edit"},
			{Key: "d", Desc: "delete"},
			{Key: "/", Desc: "filter"},
		}, baseHints...))
	case trafficView:
		m.statusBar.SetHints(append([]components.KeyHint{
			{Key: "p", Desc: "pause"},
			{Key: "c", Desc: "clear"},
			{Key: "/", Desc: "filter"},
			{Key: "enter", Desc: "details"},
		}, baseHints...))
	default:
		m.statusBar.SetHints(baseHints)
	}
}

// tickCmd returns a command that sends a tick message every second
func tickCmd() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}
