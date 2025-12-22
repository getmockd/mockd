package components

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/getmockd/mockd/pkg/tui/styles"
)

// KeyHint represents a key binding hint
type KeyHint struct {
	Key  string
	Desc string
}

// StatusBarModel represents the status bar component
type StatusBarModel struct {
	width   int
	hints   []KeyHint
	message string
}

// NewStatusBar creates a new status bar component
func NewStatusBar() StatusBarModel {
	return StatusBarModel{
		width: 80,
		hints: []KeyHint{
			{Key: "?", Desc: "help"},
			{Key: "q", Desc: "quit"},
		},
		message: "",
	}
}

// SetWidth updates the status bar width
func (s *StatusBarModel) SetWidth(width int) {
	s.width = width
}

// SetHints updates the key hints
func (s *StatusBarModel) SetHints(hints []KeyHint) {
	s.hints = hints
}

// SetMessage sets a temporary status message
func (s *StatusBarModel) SetMessage(message string) {
	s.message = message
}

// ClearMessage clears the status message
func (s *StatusBarModel) ClearMessage() {
	s.message = ""
}

// View renders the status bar
func (s StatusBarModel) View() string {
	// Build hints string
	var hintParts []string
	for _, hint := range s.hints {
		key := styles.StatusBarKeyStyle.Render("[" + hint.Key + "]")
		desc := styles.StatusBarValueStyle.Render(hint.Desc)
		hintParts = append(hintParts, key+" "+desc)
	}
	hintsStr := strings.Join(hintParts, "  ")

	// If there's a message, show it on the left, hints on the right
	var content string
	if s.message != "" {
		messageStyle := styles.StatusBarValueStyle.Bold(true)
		message := messageStyle.Render(s.message)

		// Calculate spacing
		messageWidth := lipgloss.Width(message)
		hintsWidth := lipgloss.Width(hintsStr)
		padding := s.width - messageWidth - hintsWidth - 2
		if padding < 1 {
			padding = 1
		}

		content = lipgloss.JoinHorizontal(
			lipgloss.Top,
			message,
			strings.Repeat(" ", padding),
			hintsStr,
		)
	} else {
		// Just show hints, right-aligned
		hintsWidth := lipgloss.Width(hintsStr)
		padding := s.width - hintsWidth - 1
		if padding < 0 {
			padding = 0
		}
		content = strings.Repeat(" ", padding) + hintsStr
	}

	return styles.StatusBarStyle.
		Width(s.width).
		Render(content)
}
