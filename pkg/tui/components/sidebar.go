package components

import (
	"github.com/charmbracelet/lipgloss"
	"github.com/getmockd/mockd/pkg/tui/styles"
)

// MenuItem represents a sidebar menu item
type MenuItem struct {
	Key    string
	Label  string
	Active bool
}

// SidebarModel represents the sidebar component
type SidebarModel struct {
	items  []MenuItem
	active int
	height int
}

// NewSidebar creates a new sidebar component
func NewSidebar() SidebarModel {
	items := []MenuItem{
		{Key: "1", Label: "Dashboard"},
		{Key: "2", Label: "Mocks"},
		{Key: "3", Label: "Recordings"},
		{Key: "4", Label: "Streams"},
		{Key: "5", Label: "Traffic"},
		{Key: "6", Label: "Connections"},
		{Key: "7", Label: "Logs"},
	}

	return SidebarModel{
		items:  items,
		active: 0,
		height: 20,
	}
}

// SetActive sets the active menu item by index
func (s *SidebarModel) SetActive(index int) {
	if index >= 0 && index < len(s.items) {
		s.active = index
	}
}

// SetHeight updates the sidebar height
func (s *SidebarModel) SetHeight(height int) {
	s.height = height
}

// View renders the sidebar
func (s SidebarModel) View() string {
	var items []string

	for i, item := range s.items {
		var style lipgloss.Style
		var prefix string

		if i == s.active {
			style = styles.SidebarItemActiveStyle
			prefix = "▶ "
		} else {
			style = styles.SidebarItemStyle
			prefix = "  "
		}

		label := prefix + item.Label
		items = append(items, style.Render(label))
	}

	// Join all items vertically
	content := lipgloss.JoinVertical(lipgloss.Left, items...)

	// Apply sidebar container style with height
	return styles.SidebarStyle.
		Height(s.height).
		Render(content)
}
