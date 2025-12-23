// Package components provides reusable TUI components for the mockd TUI
package components

import (
	"github.com/charmbracelet/lipgloss"
	"github.com/getmockd/mockd/pkg/tui/styles"
)

// TabBarModel manages the tab bar navigation component
type TabBarModel struct {
	tabs      []string    // View names
	activeTab int         // Currently active tab index (0-6)
	width     int         // Available width for rendering
	tabBounds []TabBounds // Click detection regions (for mouse support)
}

// TabBounds tracks clickable regions for mouse support
type TabBounds struct {
	startX int // Left edge X coordinate (inclusive)
	endX   int // Right edge X coordinate (exclusive)
}

// NewTabBar creates a new tab bar component with default views
func NewTabBar() TabBarModel {
	return TabBarModel{
		tabs: []string{
			"Dashboard",
			"Mocks",
			"Recordings",
			"Streams",
			"Traffic",
			"Connections",
			"Logs",
		},
		activeTab: 0,
		width:     0,
		tabBounds: make([]TabBounds, 0, 7),
	}
}

// SetActive sets the currently active tab
func (t *TabBarModel) SetActive(index int) {
	if index >= 0 && index < len(t.tabs) {
		t.activeTab = index
	}
}

// SetWidth sets the available width for tab bar rendering
func (t *TabBarModel) SetWidth(width int) {
	t.width = width
}

// GetTabAt determines which tab was clicked at screen coordinates
// Returns tab index (0-6) if click was on a tab, -1 otherwise
func (t *TabBarModel) GetTabAt(x, y int) int {
	// Tab bar is at row 1 (below header at row 0)
	if y != 1 {
		return -1
	}

	// Check bounds
	if len(t.tabBounds) == 0 {
		return -1
	}

	for i, bounds := range t.tabBounds {
		if x >= bounds.startX && x < bounds.endX {
			return i
		}
	}

	return -1
}

// View renders the tab bar as a string for display
func (t *TabBarModel) View() string {
	// Clear bounds for new render
	t.tabBounds = make([]TabBounds, 0, len(t.tabs))

	var renderedTabs []string
	currentX := 0

	// Render each tab
	for i, tabName := range t.tabs {
		var style lipgloss.Style

		// Determine style based on active state
		if i == t.activeTab {
			style = styles.TabActiveStyle
		} else {
			style = styles.TabInactiveStyle
		}

		// Handle border connections for first/last tabs
		border, _, _, _, _ := style.GetBorder()
		isFirst := i == 0
		isLast := i == len(t.tabs)-1
		isActive := i == t.activeTab

		if isFirst && isActive {
			border.BottomLeft = "│"
		} else if isFirst && !isActive {
			border.BottomLeft = "├"
		} else if isLast && isActive {
			border.BottomRight = "│"
		} else if isLast && !isActive {
			border.BottomRight = "┤"
		}

		style = style.Border(border)

		// Render tab
		rendered := style.Render(tabName)
		tabWidth := lipgloss.Width(rendered)

		// Track bounds for mouse click detection
		t.tabBounds = append(t.tabBounds, TabBounds{
			startX: currentX,
			endX:   currentX + tabWidth,
		})

		currentX += tabWidth
		renderedTabs = append(renderedTabs, rendered)
	}

	// Join tabs horizontally
	tabRow := lipgloss.JoinHorizontal(lipgloss.Bottom, renderedTabs...)

	// Create bottom border across full width
	bottomBorder := styles.TabBarStyle.
		Width(t.width).
		Render(tabRow)

	return bottomBorder
}
