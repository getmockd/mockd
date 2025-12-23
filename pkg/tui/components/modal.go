package components

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/getmockd/mockd/pkg/tui/styles"
)

// ModalModel represents a confirmation modal dialog.
type ModalModel struct {
	title   string
	message string
	visible bool
	width   int
	height  int

	// Callback when confirmed
	onConfirm func() tea.Msg
	onCancel  func() tea.Msg

	// UI state
	selectedButton int // 0 = Yes, 1 = No
}

// NewModal creates a new modal dialog.
func NewModal() ModalModel {
	return ModalModel{
		visible:        false,
		selectedButton: 0,
	}
}

// Show displays the modal with a title and message.
func (m *ModalModel) Show(title, message string, onConfirm, onCancel func() tea.Msg) {
	m.title = title
	m.message = message
	m.visible = true
	m.selectedButton = 0
	m.onConfirm = onConfirm
	m.onCancel = onCancel
}

// Hide hides the modal.
func (m *ModalModel) Hide() {
	m.visible = false
}

// IsVisible returns whether the modal is visible.
func (m ModalModel) IsVisible() bool {
	return m.visible
}

// SetSize sets the modal dimensions.
func (m *ModalModel) SetSize(width, height int) {
	m.width = width
	m.height = height
}

// Update handles messages.
func (m ModalModel) Update(msg tea.Msg) (ModalModel, tea.Cmd) {
	if !m.visible {
		return m, nil
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "left", "h", "shift+tab":
			m.selectedButton = 0
			return m, nil

		case "right", "l", "tab":
			m.selectedButton = 1
			return m, nil

		case "enter":
			if m.selectedButton == 0 {
				// Confirmed
				m.visible = false
				if m.onConfirm != nil {
					return m, func() tea.Msg { return m.onConfirm() }
				}
			} else {
				// Cancelled
				m.visible = false
				if m.onCancel != nil {
					return m, func() tea.Msg { return m.onCancel() }
				}
			}
			return m, nil

		case "esc":
			// Cancel on escape
			m.visible = false
			if m.onCancel != nil {
				return m, func() tea.Msg { return m.onCancel() }
			}
			return m, nil
		}
	}

	return m, nil
}

// View renders the modal.
func (m ModalModel) View() string {
	if !m.visible {
		return ""
	}

	// Modal styles
	modalStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(styles.ColorPrimary).
		Padding(1, 2).
		Width(50).
		Align(lipgloss.Center)

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(styles.ColorPrimary).
		Align(lipgloss.Center)

	messageStyle := lipgloss.NewStyle().
		Foreground(styles.ColorForeground).
		Align(lipgloss.Center).
		MarginTop(1).
		MarginBottom(2)

	buttonStyle := lipgloss.NewStyle().
		Padding(0, 2).
		Margin(0, 1)

	selectedButtonStyle := buttonStyle.
		Background(styles.ColorPrimary).
		Foreground(styles.ColorBackground).
		Bold(true)

	unselectedButtonStyle := buttonStyle.
		Background(styles.ColorMuted).
		Foreground(styles.ColorForeground)

	// Render buttons
	yesButton := "Yes"
	noButton := "No"

	if m.selectedButton == 0 {
		yesButton = selectedButtonStyle.Render(yesButton)
		noButton = unselectedButtonStyle.Render(noButton)
	} else {
		yesButton = unselectedButtonStyle.Render(yesButton)
		noButton = selectedButtonStyle.Render(noButton)
	}

	buttons := lipgloss.JoinHorizontal(lipgloss.Center, yesButton, noButton)

	// Compose modal content
	content := lipgloss.JoinVertical(
		lipgloss.Center,
		titleStyle.Render(m.title),
		messageStyle.Render(m.message),
		buttons,
	)

	modal := modalStyle.Render(content)

	// Center the modal in the screen
	// Create a blank canvas
	lines := strings.Split(modal, "\n")
	modalHeight := len(lines)
	modalWidth := lipgloss.Width(modal)

	// Calculate centering position
	verticalPadding := (m.height - modalHeight) / 2
	horizontalPadding := (m.width - modalWidth) / 2

	if verticalPadding < 0 {
		verticalPadding = 0
	}
	if horizontalPadding < 0 {
		horizontalPadding = 0
	}

	// Position the modal
	positioned := lipgloss.Place(
		m.width,
		m.height,
		lipgloss.Center,
		lipgloss.Center,
		modal,
	)

	return positioned
}
