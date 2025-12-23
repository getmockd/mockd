package views

import (
	"fmt"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/getmockd/mockd/pkg/config"
	"github.com/getmockd/mockd/pkg/tui/client"
	"github.com/getmockd/mockd/pkg/tui/components"
	"github.com/getmockd/mockd/pkg/tui/styles"
)

// ViewMode represents the current view mode.
type ViewMode int

const (
	ViewModeList ViewMode = iota
	ViewModeForm
	ViewModeConfirmDelete
)

// MocksModel represents the mocks view state.
type MocksModel struct {
	client *client.Client
	width  int
	height int

	// View mode
	viewMode ViewMode

	// List view
	table         table.Model
	mocks         []*config.MockConfiguration
	filterInput   textinput.Model
	filterActive  bool
	loading       bool
	err           error
	spinner       spinner.Model
	lastRefresh   time.Time
	selectedMock  *config.MockConfiguration
	statusMessage string

	// Form view
	mockForm MockFormModel

	// Modal
	modal components.ModalModel

	// Delete confirmation
	mockToDelete *config.MockConfiguration
}

// NewMocks creates a new mocks view.
func NewMocks(adminClient *client.Client) MocksModel {
	// Create table
	columns := []table.Column{
		{Title: "Enabled", Width: 8},
		{Title: "Method", Width: 8},
		{Title: "Path", Width: 30},
		{Title: "Status", Width: 8},
		{Title: "Name", Width: 25},
	}

	t := table.New(
		table.WithColumns(columns),
		table.WithFocused(true),
		table.WithHeight(20),
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

	// Create filter input
	filterInput := textinput.New()
	filterInput.Placeholder = "Filter by path or name..."
	filterInput.Width = 40

	// Create spinner
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(styles.ColorPrimary)

	return MocksModel{
		client:       adminClient,
		viewMode:     ViewModeList,
		table:        t,
		filterInput:  filterInput,
		filterActive: false,
		loading:      true,
		spinner:      s,
		mocks:        []*config.MockConfiguration{},
		mockForm:     NewMockForm(adminClient),
		modal:        components.NewModal(),
	}
}

// SetSize sets the view dimensions.
func (m *MocksModel) SetSize(width, height int) {
	m.width = width
	m.height = height

	// Update table height - be conservative to avoid pushing content off screen
	// Account for: title (1) + spacing (2) + filter hint (1) + spacing (1) + table header (2)
	tableHeight := height - 7
	if tableHeight < 5 {
		tableHeight = 5
	}
	if tableHeight > 15 {
		tableHeight = 15 // Cap max table height
	}
	m.table.SetHeight(tableHeight)

	// Update form size
	m.mockForm.SetSize(width, height)

	// Update modal size
	m.modal.SetSize(width, height)
}

// IsInFormMode returns true if the view is in form mode.
func (m *MocksModel) IsInFormMode() bool {
	return m.viewMode == ViewModeForm
}

// Init initializes the view.
func (m MocksModel) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
		m.fetchMocks(),
	)
}

// Update handles messages.
func (m MocksModel) Update(msg tea.Msg) (MocksModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.SetSize(msg.Width, msg.Height)
		return m, nil

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case mocksLoadedMsg:
		m.loading = false
		m.mocks = msg.mocks
		m.lastRefresh = time.Now()
		m.updateTable()
		return m, nil

	case mockToggledMsg:
		m.statusMessage = fmt.Sprintf("Mock %s %s", msg.mock.Name, enabledStatus(msg.mock.Enabled))
		m.loading = true
		return m, m.fetchMocks()

	case mockDeletedMsg:
		m.statusMessage = "Mock deleted successfully"
		m.mockToDelete = nil
		m.loading = true
		return m, m.fetchMocks()

	case mockFormSubmittedMsg:
		if m.mockForm.mode == MockFormModeCreate {
			m.statusMessage = "Mock created successfully"
		} else {
			m.statusMessage = "Mock updated successfully"
		}
		m.viewMode = ViewModeList
		m.mockForm.Reset()
		m.loading = true
		return m, m.fetchMocks()

	case mockFormCancelledMsg:
		m.viewMode = ViewModeList
		m.mockForm.Reset()
		return m, nil

	case mockFormErrorMsg:
		m.statusMessage = fmt.Sprintf("Error: %v", msg.err)
		return m, nil

	case confirmDeleteMsg:
		// User confirmed deletion
		m.modal.Hide()
		if m.mockToDelete != nil {
			return m, m.deleteMock(m.mockToDelete)
		}
		return m, nil

	case cancelDeleteMsg:
		// User cancelled deletion
		m.modal.Hide()
		m.mockToDelete = nil
		return m, nil

	case mocksErrorMsg:
		m.loading = false
		m.err = msg.err
		m.statusMessage = fmt.Sprintf("Error: %v", msg.err)
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)
	}

	// Delegate to sub-components based on view mode
	var cmd tea.Cmd
	switch m.viewMode {
	case ViewModeList:
		if m.filterActive {
			m.filterInput, cmd = m.filterInput.Update(msg)
			m.applyFilter()
			return m, cmd
		}

		m.table, cmd = m.table.Update(msg)
		m.updateSelectedMock()
		return m, cmd

	case ViewModeForm:
		m.mockForm, cmd = m.mockForm.Update(msg)
		return m, cmd

	case ViewModeConfirmDelete:
		m.modal, cmd = m.modal.Update(msg)
		return m, cmd
	}

	return m, nil
}

// handleKey processes keyboard input.
func (m MocksModel) handleKey(msg tea.KeyMsg) (MocksModel, tea.Cmd) {
	// Handle modal keys first
	if m.modal.IsVisible() {
		var cmd tea.Cmd
		m.modal, cmd = m.modal.Update(msg)
		return m, cmd
	}

	// Handle filter input keys
	if m.filterActive {
		switch msg.String() {
		case "enter", "esc":
			m.filterActive = false
			m.filterInput.Blur()
			return m, nil
		default:
			var cmd tea.Cmd
			m.filterInput, cmd = m.filterInput.Update(msg)
			m.applyFilter()
			return m, cmd
		}
	}

	// Handle view-specific keys (only for list mode)
	// Form and modal modes are handled via delegation in Update()
	if m.viewMode == ViewModeList {
		return m.handleListKeys(msg)
	}

	// For form mode, delegate to form
	if m.viewMode == ViewModeForm {
		var cmd tea.Cmd
		m.mockForm, cmd = m.mockForm.Update(msg)
		return m, cmd
	}

	// For other modes, return early to let Update() delegation handle it
	return m, nil
}

// handleListKeys handles keyboard input in list view.
func (m MocksModel) handleListKeys(msg tea.KeyMsg) (MocksModel, tea.Cmd) {
	switch msg.String() {
	case "enter":
		// Toggle enabled/disabled
		if m.selectedMock != nil {
			return m, m.toggleMock(m.selectedMock)
		}
		return m, nil

	case "n":
		// New mock
		m.viewMode = ViewModeForm
		m.mockForm.SetMode(MockFormModeCreate, nil)
		return m, m.mockForm.Init()

	case "e":
		// Edit mock
		if m.selectedMock != nil {
			m.viewMode = ViewModeForm
			m.mockForm.SetMode(MockFormModeEdit, m.selectedMock)
			return m, m.mockForm.Init()
		}
		return m, nil

	case "d":
		// Delete mock
		if m.selectedMock != nil {
			m.mockToDelete = m.selectedMock
			m.modal.Show(
				"Delete Mock",
				fmt.Sprintf("Are you sure you want to delete '%s'?", m.selectedMock.Name),
				func() tea.Msg { return confirmDeleteMsg{} },
				func() tea.Msg { return cancelDeleteMsg{} },
			)
			return m, nil
		}
		return m, nil

	case "/":
		// Activate filter
		m.filterActive = true
		m.filterInput.Focus()
		return m, nil

	case "r":
		// Refresh
		m.loading = true
		return m, m.fetchMocks()
	}

	return m, nil
}

// View renders the view.
func (m MocksModel) View() string {
	// Show modal overlay if visible
	if m.modal.IsVisible() {
		return m.renderList() + "\n" + m.modal.View()
	}

	switch m.viewMode {
	case ViewModeList:
		return m.renderList()
	case ViewModeForm:
		return m.mockForm.View()
	default:
		return m.renderList()
	}
}

// renderList renders the list view with improved lipgloss layouts.
func (m MocksModel) renderList() string {
	// Title section
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(styles.ColorPrimary).
		MarginBottom(1)
	title := titleStyle.Render(fmt.Sprintf("Mocks (%d)", len(m.mocks)))

	// Filter section
	var filterSection string
	if m.filterActive {
		filterSection = lipgloss.JoinHorizontal(
			lipgloss.Left,
			styles.FormLabelStyle.Render("Filter:"),
			" ",
			m.filterInput.View(),
		)
	} else {
		filterHintStyle := lipgloss.NewStyle().Foreground(styles.ColorMuted)
		filterSection = filterHintStyle.Render("Press / to filter")
	}

	// Loading state
	if m.loading && len(m.mocks) == 0 {
		loadingStyle := lipgloss.NewStyle().
			Foreground(styles.ColorPrimary).
			MarginTop(2)
		loading := lipgloss.JoinHorizontal(
			lipgloss.Left,
			m.spinner.View(),
			" Loading mocks...",
		)
		return lipgloss.JoinVertical(
			lipgloss.Left,
			title,
			filterSection,
			loadingStyle.Render(loading),
		)
	}

	// Error state
	if m.err != nil {
		errorStyle := lipgloss.NewStyle().
			Foreground(styles.ColorError).
			MarginTop(2)
		return lipgloss.JoinVertical(
			lipgloss.Left,
			title,
			filterSection,
			errorStyle.Render(fmt.Sprintf("Error: %v", m.err)),
		)
	}

	// Build main content sections
	sections := []string{title, filterSection, m.table.View()}

	// Add details panel if we have room and a selection
	if m.selectedMock != nil && len(m.mocks) < 5 {
		detailPanel := lipgloss.NewStyle().
			MarginTop(1).
			Render(m.renderMockDetail())
		sections = append(sections, detailPanel)
	}

	// Add status message if present
	if m.statusMessage != "" {
		statusStyle := lipgloss.NewStyle().
			Foreground(styles.ColorSuccess).
			MarginTop(1)
		sections = append(sections, statusStyle.Render(m.statusMessage))
	}

	return lipgloss.JoinVertical(lipgloss.Left, sections...)
}

// renderMockDetail renders the detail panel for the selected mock with improved layout.
func (m MocksModel) renderMockDetail() string {
	mock := m.selectedMock

	detailTitleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(styles.ColorInfo).
		MarginBottom(1)

	labelStyle := lipgloss.NewStyle().
		Foreground(styles.ColorMuted).
		Width(12).
		Align(lipgloss.Right)

	valueStyle := lipgloss.NewStyle().
		Foreground(styles.ColorForeground)

	// Helper to create detail rows
	detailRow := func(label, value string) string {
		return lipgloss.JoinHorizontal(
			lipgloss.Top,
			labelStyle.Render(label+":"),
			" ",
			valueStyle.Render(value),
		)
	}

	// Build detail sections
	var sections []string

	sections = append(sections, detailTitleStyle.Render("Mock Details"))
	sections = append(sections, detailRow("ID", mock.ID))
	sections = append(sections, detailRow("Name", mock.Name))
	sections = append(sections, detailRow("Enabled", enabledStatus(mock.Enabled)))

	// Response details
	if mock.Response != nil {
		sections = append(sections, detailRow("Status", fmt.Sprintf("%d", mock.Response.StatusCode)))

		if mock.Response.DelayMs > 0 {
			sections = append(sections, detailRow("Delay", fmt.Sprintf("%dms", mock.Response.DelayMs)))
		}

		if len(mock.Response.Headers) > 0 {
			sections = append(sections, labelStyle.Render("Headers:"))
			for k, v := range mock.Response.Headers {
				headerRow := lipgloss.NewStyle().
					MarginLeft(2).
					Render(detailRow(k, v))
				sections = append(sections, headerRow)
			}
		}

		if mock.Response.Body != "" {
			bodyPreview := mock.Response.Body
			if len(bodyPreview) > 100 {
				bodyPreview = bodyPreview[:100] + "..."
			}
			sections = append(sections, detailRow("Body", bodyPreview))
		}
	}

	// Use a border for the detail panel
	detailPanel := lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(styles.ColorBorder).
		Padding(1, 2).
		Render(lipgloss.JoinVertical(lipgloss.Left, sections...))

	return detailPanel
}

// fetchMocks fetches the list of mocks from the API.
func (m MocksModel) fetchMocks() tea.Cmd {
	return func() tea.Msg {
		mocks, err := m.client.ListMocks()
		if err != nil {
			return mocksErrorMsg{err: err}
		}
		return mocksLoadedMsg{mocks: mocks}
	}
}

// toggleMock toggles a mock's enabled state.
func (m MocksModel) toggleMock(mock *config.MockConfiguration) tea.Cmd {
	return func() tea.Msg {
		result, err := m.client.ToggleMock(mock.ID, !mock.Enabled)
		if err != nil {
			return mocksErrorMsg{err: err}
		}
		return mockToggledMsg{mock: result}
	}
}

// deleteMock deletes a mock.
func (m MocksModel) deleteMock(mock *config.MockConfiguration) tea.Cmd {
	return func() tea.Msg {
		err := m.client.DeleteMock(mock.ID)
		if err != nil {
			return mocksErrorMsg{err: err}
		}
		return mockDeletedMsg{}
	}
}

// updateTable updates the table rows based on current mocks.
func (m *MocksModel) updateTable() {
	rows := []table.Row{}
	for _, mock := range m.mocks {
		enabled := "✗"
		if mock.Enabled {
			enabled = "✓"
		}

		method := mock.Matcher.Method
		path := mock.Matcher.Path
		status := ""
		name := mock.Name

		if mock.Response != nil {
			status = fmt.Sprintf("%d", mock.Response.StatusCode)
		}

		// Truncate path if too long
		if len(path) > 30 {
			path = path[:27] + "..."
		}

		// Truncate name if too long
		if len(name) > 25 {
			name = name[:22] + "..."
		}

		rows = append(rows, table.Row{enabled, method, path, status, name})
	}

	m.table.SetRows(rows)
	m.updateSelectedMock()
}

// updateSelectedMock updates the selected mock based on table cursor.
func (m *MocksModel) updateSelectedMock() {
	cursor := m.table.Cursor()
	if cursor >= 0 && cursor < len(m.mocks) {
		m.selectedMock = m.mocks[cursor]
	} else {
		m.selectedMock = nil
	}
}

// applyFilter filters the mocks based on the filter input.
func (m *MocksModel) applyFilter() {
	// For now, just update the table with all mocks
	// TODO: Implement actual filtering logic
	m.updateTable()
}

// enabledStatus returns a string representation of enabled status.
func enabledStatus(enabled bool) string {
	if enabled {
		return "enabled"
	}
	return "disabled"
}

// Messages

type mocksLoadedMsg struct {
	mocks []*config.MockConfiguration
}

type mockToggledMsg struct {
	mock *config.MockConfiguration
}

type mockDeletedMsg struct{}

type confirmDeleteMsg struct{}

type cancelDeleteMsg struct{}

type mocksErrorMsg struct {
	err error
}
