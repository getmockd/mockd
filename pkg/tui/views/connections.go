package views

import (
	"fmt"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/getmockd/mockd/pkg/sse"
	"github.com/getmockd/mockd/pkg/tui/client"
	"github.com/getmockd/mockd/pkg/tui/styles"
	"github.com/getmockd/mockd/pkg/websocket"
)

// Connection represents a unified view of WS or SSE connection.
type Connection struct {
	ID        string
	Type      string // "ws" or "sse"
	Path      string
	ClientIP  string
	Duration  time.Duration
	Messages  int
	Status    string
	Recording bool
}

// ConnectionsModel represents the connections view state.
type ConnectionsModel struct {
	client  *client.Client
	width   int
	height  int
	loading bool
	err     error

	// Data
	wsConnections  []websocket.ConnectionInfo
	sseConnections []sse.SSEStreamInfo
	connections    []Connection // Unified view

	// Filters
	typeFilter string // "", "ws", "sse"

	// UI state
	table         table.Model
	spinner       spinner.Model
	lastRefresh   time.Time
	statusMessage string
	cursor        int

	// Message input state
	showMessageInput bool
	messageInput     textinput.Model
}

// NewConnections creates a new connections model.
func NewConnections(adminClient *client.Client) ConnectionsModel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(styles.ColorPrimary)

	columns := []table.Column{
		{Title: "ID", Width: 12},
		{Title: "Type", Width: 6},
		{Title: "Path", Width: 30},
		{Title: "Client IP", Width: 16},
		{Title: "Duration", Width: 10},
		{Title: "Messages", Width: 10},
		{Title: "Status", Width: 10},
	}

	t := table.New(
		table.WithColumns(columns),
		table.WithFocused(true),
		table.WithHeight(15),
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

	mi := textinput.New()
	mi.Placeholder = "Enter message to send..."
	mi.CharLimit = 500
	mi.Width = 50

	return ConnectionsModel{
		client:       adminClient,
		loading:      true,
		spinner:      s,
		table:        t,
		messageInput: mi,
	}
}

// Init initializes the connections view (Bubbletea lifecycle).
func (m ConnectionsModel) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
		m.fetchConnections(),
		m.tickRefresh(),
	)
}

// Update handles messages (Bubbletea lifecycle).
func (m ConnectionsModel) Update(msg tea.Msg) (ConnectionsModel, tea.Cmd) {
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

	case connectionsLoadedMsg:
		m.loading = false
		m.wsConnections = msg.wsConnections
		m.sseConnections = msg.sseConnections
		m.lastRefresh = time.Now()
		m.mergeConnections()
		m.updateTable()
		return m, nil

	case refreshTickMsg:
		// Auto-refresh every 2 seconds for live connections
		return m, tea.Batch(
			m.fetchConnections(),
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

	// Update table or message input
	if m.showMessageInput {
		var cmd tea.Cmd
		m.messageInput, cmd = m.messageInput.Update(msg)
		return m, cmd
	}

	var cmd tea.Cmd
	m.table, cmd = m.table.Update(msg)
	return m, cmd
}

// handleKey processes keyboard input for connections-specific actions.
func (m ConnectionsModel) handleKey(msg tea.KeyMsg) (ConnectionsModel, tea.Cmd) {
	// If message input is shown, handle input-specific keys
	if m.showMessageInput {
		switch msg.String() {
		case "enter":
			message := m.messageInput.Value()
			m.showMessageInput = false
			m.messageInput.SetValue("")
			return m, m.sendMessage(message)

		case "esc":
			m.showMessageInput = false
			m.messageInput.SetValue("")
			m.statusMessage = "Message cancelled"
			return m, nil
		}
		return m, nil
	}

	// Regular connections view keys
	switch msg.String() {
	case "d":
		// Disconnect selected connection
		if len(m.connections) > 0 {
			return m, m.disconnectConnection()
		}
		return m, nil

	case "m":
		// Send message to connection
		if len(m.connections) > 0 {
			m.showMessageInput = true
			m.messageInput.Focus()
			return m, nil
		}
		return m, nil

	case "r":
		// Toggle recording for connection
		if len(m.connections) > 0 {
			return m, m.toggleConnectionRecording()
		}
		return m, nil

	case "1":
		// Filter: All
		m.typeFilter = ""
		m.statusMessage = "Showing all connections"
		m.mergeConnections()
		m.updateTable()
		return m, nil

	case "2":
		// Filter: WebSocket
		m.typeFilter = "ws"
		m.statusMessage = "Showing WebSocket connections"
		m.mergeConnections()
		m.updateTable()
		return m, nil

	case "3":
		// Filter: SSE
		m.typeFilter = "sse"
		m.statusMessage = "Showing SSE connections"
		m.mergeConnections()
		m.updateTable()
		return m, nil
	}

	return m, nil
}

// View renders the connections view (Bubbletea lifecycle).
func (m ConnectionsModel) View() string {
	if m.loading && len(m.connections) == 0 {
		return m.renderLoading()
	}

	if m.err != nil {
		return m.renderError()
	}

	// If message input is shown, render it as overlay
	if m.showMessageInput {
		return m.renderConnectionsView() + "\n\n" + m.renderMessageInput()
	}

	return m.renderConnectionsView()
}

// renderLoading shows a loading spinner.
func (m ConnectionsModel) renderLoading() string {
	return fmt.Sprintf("\n  %s Loading connections...\n", m.spinner.View())
}

// renderError shows an error message.
func (m ConnectionsModel) renderError() string {
	errorStyle := lipgloss.NewStyle().
		Foreground(styles.ColorError).
		Bold(true)

	return fmt.Sprintf("\n  %s\n\n  %s\n",
		errorStyle.Render("Error loading connections:"),
		m.err.Error(),
	)
}

// renderConnectionsView renders the full connections view.
func (m ConnectionsModel) renderConnectionsView() string {
	sections := []string{
		m.renderHeader(),
		"",
		m.renderTable(),
	}

	if m.statusMessage != "" {
		sections = append(sections, "", m.renderStatusMessage())
	}

	return lipgloss.JoinVertical(lipgloss.Left, sections...)
}

// renderHeader renders the header with filter tabs.
func (m ConnectionsModel) renderHeader() string {
	titleStyle := styles.TitleStyle
	title := titleStyle.Render(fmt.Sprintf("Active Connections (%d)", len(m.connections)))

	// Type filter tabs
	allStyle := lipgloss.NewStyle().Padding(0, 1)
	wsStyle := lipgloss.NewStyle().Padding(0, 1)
	sseStyle := lipgloss.NewStyle().Padding(0, 1)

	if m.typeFilter == "" {
		allStyle = allStyle.Foreground(styles.ColorPrimary).Bold(true)
	} else {
		allStyle = allStyle.Foreground(styles.ColorMuted)
	}

	if m.typeFilter == "ws" {
		wsStyle = wsStyle.Foreground(styles.ColorPrimary).Bold(true)
	} else {
		wsStyle = wsStyle.Foreground(styles.ColorMuted)
	}

	if m.typeFilter == "sse" {
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

// renderTable renders the connections table.
func (m ConnectionsModel) renderTable() string {
	if len(m.connections) == 0 {
		return lipgloss.NewStyle().
			Foreground(styles.ColorMuted).
			Italic(true).
			Render("No active connections")
	}
	return m.table.View()
}

// renderMessageInput renders the message input overlay.
func (m ConnectionsModel) renderMessageInput() string {
	modalStyle := styles.ModalStyle.
		Width(60).
		Align(lipgloss.Left)

	titleStyle := styles.ModalTitleStyle
	title := titleStyle.Render("Send Message")

	separator := lipgloss.NewStyle().Foreground(styles.ColorMuted).Render("────────────────────────────────────────────────────────")

	label := lipgloss.NewStyle().Foreground(styles.ColorInfo).Render("Message:")

	actions := lipgloss.NewStyle().Foreground(styles.ColorMuted).Render("[Enter] Send  [Esc] Cancel")

	content := lipgloss.JoinVertical(
		lipgloss.Left,
		title,
		separator,
		"",
		label,
		m.messageInput.View(),
		"",
		actions,
	)

	return modalStyle.Render(content)
}

// renderStatusMessage renders a temporary status message.
func (m ConnectionsModel) renderStatusMessage() string {
	msgStyle := lipgloss.NewStyle().
		Foreground(styles.ColorInfo).
		Italic(true)
	return msgStyle.Render(m.statusMessage)
}

// SetSize updates the connections view dimensions.
func (m *ConnectionsModel) SetSize(width, height int) {
	m.width = width
	m.height = height
	m.table.SetHeight(height - 10)
}

// mergeConnections merges WS and SSE connections into unified list.
func (m *ConnectionsModel) mergeConnections() {
	m.connections = make([]Connection, 0)

	// Add WebSocket connections
	if m.typeFilter == "" || m.typeFilter == "ws" {
		for _, wsConn := range m.wsConnections {
			duration := time.Since(wsConn.ConnectedAt)

			m.connections = append(m.connections, Connection{
				ID:       wsConn.ID,
				Type:     "ws",
				Path:     wsConn.EndpointPath,
				ClientIP: "", // Not available in ConnectionInfo
				Duration: duration,
				Messages: int(wsConn.MessagesReceived + wsConn.MessagesSent),
				Status:   "active",
			})
		}
	}

	// Add SSE connections
	if m.typeFilter == "" || m.typeFilter == "sse" {
		for _, sseConn := range m.sseConnections {
			duration := time.Since(sseConn.StartTime)

			m.connections = append(m.connections, Connection{
				ID:       sseConn.ID,
				Type:     "sse",
				Path:     "", // SSE doesn't expose path in the info
				ClientIP: sseConn.ClientIP,
				Duration: duration,
				Messages: int(sseConn.EventsSent),
				Status:   string(sseConn.Status),
			})
		}
	}
}

// updateTable updates the table with current connections data.
func (m *ConnectionsModel) updateTable() {
	rows := make([]table.Row, 0, len(m.connections))

	for _, conn := range m.connections {
		duration := formatDuration(conn.Duration)

		rows = append(rows, table.Row{
			truncate(conn.ID, 12),
			conn.Type,
			truncate(conn.Path, 30),
			conn.ClientIP,
			duration,
			fmt.Sprintf("%d", conn.Messages),
			conn.Status,
		})
	}

	m.table.SetRows(rows)
}

// fetchConnections fetches both WS and SSE connections from the API.
func (m ConnectionsModel) fetchConnections() tea.Cmd {
	return func() tea.Msg {
		// Fetch WebSocket connections
		wsResp, err := m.client.ListWSConnections("")
		var wsConnections []websocket.ConnectionInfo
		if err == nil && wsResp != nil {
			wsConnections = wsResp.Connections
		}

		// Fetch SSE connections
		sseResp, err := m.client.ListSSEConnections()
		var sseConnections []sse.SSEStreamInfo
		if err == nil && sseResp != nil {
			sseConnections = sseResp.Connections
		}

		return connectionsLoadedMsg{
			wsConnections:  wsConnections,
			sseConnections: sseConnections,
		}
	}
}

// disconnectConnection disconnects the selected connection.
func (m ConnectionsModel) disconnectConnection() tea.Cmd {
	return func() tea.Msg {
		if len(m.connections) == 0 {
			return statusMsg{message: "No connection selected"}
		}

		selectedIdx := m.table.Cursor()
		if selectedIdx >= len(m.connections) {
			return statusMsg{message: "No connection selected"}
		}

		conn := m.connections[selectedIdx]

		var err error
		if conn.Type == "ws" {
			err = m.client.DisconnectWS(conn.ID)
		} else {
			err = m.client.CloseSSEConnection(conn.ID)
		}

		if err != nil {
			return statusMsg{message: fmt.Sprintf("Disconnect failed: %v", err)}
		}

		return statusMsg{message: fmt.Sprintf("Disconnected %s connection %s", conn.Type, conn.ID)}
	}
}

// sendMessage sends a message to the selected connection.
func (m ConnectionsModel) sendMessage(message string) tea.Cmd {
	return func() tea.Msg {
		if len(m.connections) == 0 {
			return statusMsg{message: "No connection selected"}
		}

		selectedIdx := m.table.Cursor()
		if selectedIdx >= len(m.connections) {
			return statusMsg{message: "No connection selected"}
		}

		conn := m.connections[selectedIdx]

		// Only WebSocket supports sending messages
		if conn.Type != "ws" {
			return statusMsg{message: "Can only send messages to WebSocket connections"}
		}

		err := m.client.SendWSMessage(conn.ID, message)
		if err != nil {
			return statusMsg{message: fmt.Sprintf("Send failed: %v", err)}
		}

		return statusMsg{message: "Message sent"}
	}
}

// toggleConnectionRecording toggles recording for the selected connection.
func (m ConnectionsModel) toggleConnectionRecording() tea.Cmd {
	return func() tea.Msg {
		// TODO: Implement recording toggle via admin API
		return statusMsg{message: "Recording toggle not yet implemented"}
	}
}

// tickRefresh returns a command that schedules the next refresh.
func (m ConnectionsModel) tickRefresh() tea.Cmd {
	return tea.Tick(2*time.Second, func(t time.Time) tea.Msg {
		return refreshTickMsg(t)
	})
}

// Messages

// connectionsLoadedMsg contains loaded connection data.
type connectionsLoadedMsg struct {
	wsConnections  []websocket.ConnectionInfo
	sseConnections []sse.SSEStreamInfo
}
