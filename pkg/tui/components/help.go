package components

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/getmockd/mockd/pkg/tui/styles"
)

// HelpModel represents the help overlay component
type HelpModel struct {
	width   int
	height  int
	visible bool
}

// NewHelp creates a new help component
func NewHelp() HelpModel {
	return HelpModel{
		width:   80,
		height:  24,
		visible: false,
	}
}

// SetSize updates the help overlay size
func (h *HelpModel) SetSize(width, height int) {
	h.width = width
	h.height = height
}

// Toggle toggles the help visibility
func (h *HelpModel) Toggle() {
	h.visible = !h.visible
}

// SetVisible sets the help visibility
func (h *HelpModel) SetVisible(visible bool) {
	h.visible = visible
}

// IsVisible returns whether the help is visible
func (h HelpModel) IsVisible() bool {
	return h.visible
}

// View renders the help overlay
func (h HelpModel) View() string {
	if !h.visible {
		return ""
	}

	// Help content
	sections := []string{
		styles.ModalTitleStyle.Render("mockd TUI - Keyboard Shortcuts"),
		"",
		"Navigation",
		"──────────",
		"  1-7         Switch between views",
		"  ↑/k, ↓/j    Move up/down in lists",
		"  g, G        Go to top/bottom",
		"  PgUp/PgDn   Page up/down",
		"  Enter       Select item",
		"  Esc         Go back/cancel",
		"",
		"Global Actions",
		"──────────────",
		"  ?           Toggle this help",
		"  q, Ctrl+C   Quit",
		"  Ctrl+R      Refresh data",
		"",
		"View-Specific",
		"─────────────",
		"  n           New item (Mocks view)",
		"  e           Edit item",
		"  d           Delete item",
		"  /           Filter/search",
		"  p           Pause/resume (Traffic view)",
		"  r           Toggle recording",
		"",
		styles.HelpStyle.Render("Press ? or Esc to close"),
	}

	content := strings.Join(sections, "\n")

	// Calculate modal dimensions
	modalWidth := 60
	modalHeight := len(sections) + 4

	// Center the modal
	modal := styles.ModalStyle.
		Width(modalWidth).
		Height(modalHeight).
		Render(content)

	// Place it in the center of the screen
	return lipgloss.Place(
		h.width,
		h.height,
		lipgloss.Center,
		lipgloss.Center,
		modal,
	)
}
