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
	dashboard   views.DashboardModel
	mocks       views.MocksModel
	mockForm    views.MockFormModel
	traffic     views.TrafficModel
	streams     views.StreamsModel
	proxy       views.ProxyModel
	connections views.ConnectionsModel
	logs        views.LogsModel

	// Application state
	ready         bool
	loading       bool
	err           error
	statusMessage string
	lastUpdate    time.Time
}

// newModel creates a new root model with default client
func newModel() model {
	return newModelWithClient(client.NewDefaultClient())
}

// newModelWithClient creates a new root model with custom admin client
func newModelWithClient(adminClient *client.Client) model {
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
		mockForm:    views.NewMockForm(adminClient),
		traffic:     views.NewTraffic(adminClient),
		streams:     views.NewStreams(adminClient),
		proxy:       views.NewProxy(adminClient),
		connections: views.NewConnections(adminClient),
		logs:        views.NewLogs(adminClient),
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
		m.mockForm.Init(),
		m.traffic.Init(),
		m.streams.Init(),
		m.proxy.Init(),
		m.connections.Init(),
		m.logs.Init(),
	)
}

// Update handles messages (Bubbletea lifecycle)
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		// Try global keys first, then fall through to view if not handled
		if handled, cmd := m.handleGlobalKeys(msg); handled {
			return m, cmd
		}
		// Key not handled globally, delegate to active view
		var cmd tea.Cmd
		switch m.currentView {
		case dashboardView:
			m.dashboard, cmd = m.dashboard.Update(msg)
		case mocksView:
			m.mocks, cmd = m.mocks.Update(msg)
		case recordingsView:
			m.mockForm, cmd = m.mockForm.Update(msg)
		case streamsView:
			m.streams, cmd = m.streams.Update(msg)
		case trafficView:
			m.traffic, cmd = m.traffic.Update(msg)
		case connectionsView:
			m.connections, cmd = m.connections.Update(msg)
		case logsView:
			m.logs, cmd = m.logs.Update(msg)
		}
		return m, cmd

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
		m.mockForm.SetSize(contentWidth, contentHeight)
		m.traffic.SetSize(contentWidth, contentHeight)
		m.streams.SetSize(contentWidth, contentHeight)
		m.proxy.SetSize(contentWidth, contentHeight)
		m.connections.SetSize(contentWidth, contentHeight)
		m.logs.SetSize(contentWidth, contentHeight)

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
	case recordingsView:
		m.mockForm, cmd = m.mockForm.Update(msg)
		return m, cmd
	case streamsView:
		m.streams, cmd = m.streams.Update(msg)
		return m, cmd
	case trafficView:
		m.traffic, cmd = m.traffic.Update(msg)
		return m, cmd
	case connectionsView:
		m.connections, cmd = m.connections.Update(msg)
		return m, cmd
	case logsView:
		m.logs, cmd = m.logs.Update(msg)
		return m, cmd
	}

	return m, nil
}

// handleGlobalKeys processes global keyboard shortcuts, returns (handled bool, cmd)
func (m *model) handleGlobalKeys(msg tea.KeyMsg) (bool, tea.Cmd) {
	// Global key handling
	switch {
	case key.Matches(msg, m.keys.Quit):
		return true, tea.Quit

	case key.Matches(msg, m.keys.Help):
		m.help.Toggle()
		return true, nil

	case key.Matches(msg, m.keys.Dashboard):
		m.currentView = dashboardView
		m.sidebar.SetActive(0)
		m.updateStatusBarHints()
		return true, nil

	case key.Matches(msg, m.keys.Mocks):
		m.currentView = mocksView
		m.sidebar.SetActive(1)
		m.updateStatusBarHints()
		// Trigger data refresh when switching to view
		return true, m.mocks.Init()

	case key.Matches(msg, m.keys.Recordings):
		m.currentView = recordingsView
		m.sidebar.SetActive(2)
		m.updateStatusBarHints()
		return true, nil

	case key.Matches(msg, m.keys.Streams):
		m.currentView = streamsView
		m.sidebar.SetActive(3)
		m.updateStatusBarHints()
		return true, nil

	case key.Matches(msg, m.keys.Traffic):
		m.currentView = trafficView
		m.sidebar.SetActive(4)
		m.updateStatusBarHints()
		return true, nil

	case key.Matches(msg, m.keys.Connections):
		m.currentView = connectionsView
		m.sidebar.SetActive(5)
		m.updateStatusBarHints()
		return true, nil

	case key.Matches(msg, m.keys.Logs):
		m.currentView = logsView
		m.sidebar.SetActive(6)
		m.updateStatusBarHints()
		return true, nil
	}

	// Key not handled
	return false, nil
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
		return m.proxy.View()
	case streamsView:
		return m.streams.View()
	case trafficView:
		return m.traffic.View()
	case connectionsView:
		return m.connections.View()
	case logsView:
		return m.logs.View()
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
	case recordingsView:
		m.statusBar.SetHints(append([]components.KeyHint{
			{Key: "s", Desc: "start/stop"},
			{Key: "m", Desc: "mode"},
			{Key: "t", Desc: "target"},
		}, baseHints...))
	case streamsView:
		m.statusBar.SetHints(append([]components.KeyHint{
			{Key: "enter", Desc: "details"},
			{Key: "d", Desc: "delete"},
			{Key: "r", Desc: "replay"},
		}, baseHints...))
	case trafficView:
		m.statusBar.SetHints(append([]components.KeyHint{
			{Key: "p", Desc: "pause"},
			{Key: "c", Desc: "clear"},
			{Key: "/", Desc: "filter"},
			{Key: "enter", Desc: "details"},
		}, baseHints...))
	case connectionsView:
		m.statusBar.SetHints(append([]components.KeyHint{
			{Key: "k", Desc: "kill"},
			{Key: "f", Desc: "filter"},
		}, baseHints...))
	case logsView:
		m.statusBar.SetHints(append([]components.KeyHint{
			{Key: "c", Desc: "clear"},
			{Key: "l", Desc: "level"},
			{Key: "/", Desc: "search"},
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
